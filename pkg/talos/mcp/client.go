package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var Version = "dev"

const (
	DefaultTimeout          = 30 * time.Second
	DefaultMaxResponseBytes = 10 * 1024 * 1024 // 10MB
	MaxErrorSnippetBytes    = 16 * 1024        // 16KB
)

type InvokeMode string

const (
	InvokeModeSync  InvokeMode = "sync"
	InvokeModeAsync InvokeMode = "async"
)

// Doer matches http.Client.Do
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type McpClient struct {
	BaseURL          string
	APIKey           string
	HTTPClient       Doer
	MaxResponseBytes int64
}

type APIError struct {
	Status      int    `json:"-"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	RequestID   string `json:"request_id"`
	BodySnippet string `json:"-"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("talos mcp error [status=%d, code=%s]: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("talos mcp http error [status=%d]: %s", e.Status, e.BodySnippet)
}

type Server struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Tags      map[string]string `json:"tags"`
}

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Option func(*McpClient)

func WithHTTPClient(client Doer) Option {
	return func(c *McpClient) {
		c.HTTPClient = client
	}
}

func WithMaxResponseBytes(limit int64) Option {
	return func(c *McpClient) {
		c.MaxResponseBytes = limit
	}
}

func NewClient(baseURL string, apiKey string, opts ...Option) *McpClient {
	c := &McpClient{
		BaseURL:          strings.TrimRight(baseURL, "/"),
		APIKey:           apiKey,
		HTTPClient:       &http.Client{Timeout: DefaultTimeout},
		MaxResponseBytes: DefaultMaxResponseBytes,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *McpClient) ListServers(ctx context.Context) ([]Server, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := c.BaseURL + "/v1/mcp/servers"
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result struct {
		Servers []Server `json:"servers"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, c.MaxResponseBytes)).Decode(&result); err != nil {
		return nil, err
	}

	return result.Servers, nil
}

func (c *McpClient) ListTools(ctx context.Context, serverID string) ([]Tool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := c.BaseURL + "/v1/mcp/servers/" + pathEscape(serverID) + "/tools"
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, c.MaxResponseBytes)).Decode(&result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

func (c *McpClient) setHeaders(req *http.Request, requestID string) {
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "talos-sdk-go/"+Version)
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
}

func (c *McpClient) handleError(resp *http.Response) error {
	apiErr := &APIError{
		Status:    resp.StatusCode,
		RequestID: resp.Header.Get("X-Request-Id"),
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, MaxErrorSnippetBytes))

	// Try to parse as JSON error
	var errWrap struct {
		Error *APIError `json:"error"`
	}
	if err := json.Unmarshal(body, &errWrap); err == nil && errWrap.Error != nil {
		apiErr.Code = errWrap.Error.Code
		apiErr.Message = errWrap.Error.Message
		if errWrap.Error.RequestID != "" {
			apiErr.RequestID = errWrap.Error.RequestID
		}
	} else {
		apiErr.BodySnippet = string(body)
		if apiErr.Code == "" {
			apiErr.Code = fmt.Sprintf("http_%d", resp.StatusCode)
		}
	}

	return apiErr
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func pathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), ":", "%3A")
}

type ToolCallRequest struct {
	ServerID  string     `json:"server_id"`
	ToolName  string     `json:"tool_name"`
	Input     any        `json:"input"`
	RequestID string     `json:"request_id,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	Mode      InvokeMode `json:"mode,omitempty"`
}

type ToolCallResponse struct {
	RequestID string          `json:"request_id"`
	Output    json.RawMessage `json:"output"`
	TimingMS  int             `json:"timing_ms"`
	AuditRef  string          `json:"audit_ref"`
}

func (r *ToolCallResponse) DecodeOutput(v any) error {
	return json.Unmarshal(r.Output, v)
}

func (c *McpClient) CallTool(ctx context.Context, serverID, toolName string, input any, requestID, sessionID string) (*ToolCallResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if requestID == "" {
		requestID = generateRequestID()
	}

	// Manual deterministic URL assembly to avoid JoinPath normalization surprises
	base := strings.TrimRight(c.BaseURL, "/")
	endpoint := base + "/v1/mcp/servers/" + pathEscape(serverID) + "/tools/" + pathEscape(toolName) + ":call"

	bodyObj := ToolCallRequest{
		ServerID:  serverID,
		ToolName:  toolName,
		Input:     input,
		RequestID: requestID,
		SessionID: sessionID,
		Mode:      InvokeModeSync,
	}

	bodyBytes, err := json.Marshal(bodyObj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, requestID)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result ToolCallResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, c.MaxResponseBytes)).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
