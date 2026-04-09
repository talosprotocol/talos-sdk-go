package a2a

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	TalosAttestationExtension    = "https://talosprotocol.com/extensions/a2a/attestation/v1"
	TalosSecureChannelsExtension = "https://talosprotocol.com/extensions/a2a/secure-channels/v1"
	TalosCompatJSONRPCExtension  = "https://talosprotocol.com/extensions/a2a/compat-jsonrpc/v0"
	defaultTimeoutSeconds        = 30
)

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	BaseURL    string
	APIToken   string
	HTTPClient Doer
}

type Option func(*Client)

type HTTPError struct {
	StatusCode int
	Payload    any
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http %d: %v", e.StatusCode, e.Payload)
}

type JSONRPCError struct {
	Code    int
	Message string
	Data    map[string]any
}

func (e *JSONRPCError) Error() string {
	return e.Message
}

type MessageOptions struct {
	MessageID     string
	TaskID        string
	ContextID     string
	Configuration map[string]any
	Metadata      map[string]any
}

type TaskOptions struct {
	HistoryLength    *int
	IncludeArtifacts bool
}

type ListTasksOptions struct {
	TaskOptions
	ContextID string
	Status    string
	PageSize  *int
	PageToken string
}

type PushNotificationConfigOptions struct {
	URL            string
	Token          string
	Authentication map[string]any
	ConfigID       string
}

type StreamHandler func(map[string]any) error

type StreamResult struct {
	Event map[string]any
	Err   error
}

func WithHTTPClient(client Doer) Option {
	return func(c *Client) {
		c.HTTPClient = client
	}
}

func NewClient(baseURL, apiToken string, opts ...Option) *Client {
	client := &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIToken:   apiToken,
		HTTPClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func (c *Client) GetAgentCard(ctx context.Context) (map[string]any, error) {
	return c.requestObject(ctx, http.MethodGet, "/.well-known/agent-card.json", nil)
}

func (c *Client) GetExtendedAgentCard(ctx context.Context) (map[string]any, error) {
	return c.requestObject(ctx, http.MethodGet, "/extendedAgentCard", nil)
}

func (c *Client) GetAuthenticatedExtendedAgentCard(ctx context.Context) (map[string]any, error) {
	return c.RPC(ctx, "GetExtendedAgentCard", nil)
}

func SupportedInterfaces(card map[string]any) []map[string]any {
	values, ok := card["supportedInterfaces"].([]any)
	if !ok {
		return nil
	}
	interfaces := make([]map[string]any, 0, len(values))
	for _, candidate := range values {
		if item, ok := candidate.(map[string]any); ok {
			interfaces = append(interfaces, item)
		}
	}
	return interfaces
}

func ExtensionURIs(card map[string]any) []string {
	capabilities, ok := card["capabilities"].(map[string]any)
	if !ok {
		return nil
	}
	extensions, ok := capabilities["extensions"].([]any)
	if !ok {
		return nil
	}
	uris := make([]string, 0, len(extensions))
	for _, candidate := range extensions {
		item, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if uri, ok := item["uri"].(string); ok {
			uris = append(uris, uri)
		}
	}
	return uris
}

func SupportsExtension(card map[string]any, uri string) bool {
	for _, candidate := range ExtensionURIs(card) {
		if candidate == uri {
			return true
		}
	}
	return false
}

func SupportsTalosSecureChannels(card map[string]any) bool {
	return SupportsExtension(card, TalosSecureChannelsExtension)
}

func SupportsTalosAttestation(card map[string]any) bool {
	return SupportsExtension(card, TalosAttestationExtension)
}

func SupportsTalosCompatJSONRPC(card map[string]any) bool {
	return SupportsExtension(card, TalosCompatJSONRPCExtension)
}

func (c *Client) SendMessage(ctx context.Context, text string, options MessageOptions) (map[string]any, error) {
	params := map[string]any{
		"message": c.message(text, options),
	}
	if options.Configuration != nil {
		params["configuration"] = options.Configuration
	}
	return c.RPC(ctx, "SendMessage", params)
}

func (c *Client) SendStreamingMessage(ctx context.Context, text string, options MessageOptions) ([]map[string]any, error) {
	params := map[string]any{
		"message": c.message(text, options),
	}
	if options.Configuration != nil {
		params["configuration"] = options.Configuration
	}
	return c.Stream(ctx, "SendStreamingMessage", params)
}

func (c *Client) SendStreamingMessageChan(
	ctx context.Context,
	text string,
	options MessageOptions,
) <-chan StreamResult {
	params := map[string]any{
		"message": c.message(text, options),
	}
	if options.Configuration != nil {
		params["configuration"] = options.Configuration
	}
	return c.StreamChan(ctx, "SendStreamingMessage", params)
}

func (c *Client) SendStreamingMessageEach(
	ctx context.Context,
	text string,
	options MessageOptions,
	handler StreamHandler,
) error {
	params := map[string]any{
		"message": c.message(text, options),
	}
	if options.Configuration != nil {
		params["configuration"] = options.Configuration
	}
	return c.StreamEach(ctx, "SendStreamingMessage", params, handler)
}

func (c *Client) GetTask(ctx context.Context, taskID string, options TaskOptions) (map[string]any, error) {
	return c.RPC(ctx, "GetTask", c.taskParams(taskID, options))
}

func (c *Client) CancelTask(ctx context.Context, taskID string, options TaskOptions) (map[string]any, error) {
	return c.RPC(ctx, "CancelTask", c.taskParams(taskID, options))
}

func (c *Client) ListTasks(ctx context.Context, options ListTasksOptions) (map[string]any, error) {
	params := map[string]any{
		"includeArtifacts": options.IncludeArtifacts,
	}
	if options.ContextID != "" {
		params["contextId"] = options.ContextID
	}
	if options.Status != "" {
		params["status"] = options.Status
	}
	if options.PageSize != nil {
		params["pageSize"] = *options.PageSize
	}
	if options.PageToken != "" {
		params["pageToken"] = options.PageToken
	}
	if options.HistoryLength != nil {
		params["historyLength"] = *options.HistoryLength
	}
	return c.RPC(ctx, "ListTasks", params)
}

func (c *Client) SetTaskPushNotificationConfig(
	ctx context.Context,
	taskID string,
	options PushNotificationConfigOptions,
) (map[string]any, error) {
	params := map[string]any{
		"taskId": taskID,
		"id":     firstNonEmpty(options.ConfigID, c.newID("push")),
		"url":    options.URL,
	}
	if options.Token != "" {
		params["token"] = options.Token
	}
	if options.Authentication != nil {
		params["authentication"] = options.Authentication
	}
	return c.RPC(ctx, "CreateTaskPushNotificationConfig", params)
}

func (c *Client) GetTaskPushNotificationConfig(
	ctx context.Context,
	taskID string,
	configID string,
) (map[string]any, error) {
	return c.RPC(ctx, "GetTaskPushNotificationConfig", map[string]any{"taskId": taskID, "id": configID})
}

func (c *Client) ListTaskPushNotificationConfigs(ctx context.Context, taskID string) (map[string]any, error) {
	return c.RPC(ctx, "ListTaskPushNotificationConfigs", map[string]any{"taskId": taskID})
}

func (c *Client) DeleteTaskPushNotificationConfig(
	ctx context.Context,
	taskID string,
	configID string,
) (map[string]any, error) {
	return c.RPC(ctx, "DeleteTaskPushNotificationConfig", map[string]any{"taskId": taskID, "id": configID})
}

func (c *Client) SubscribeToTask(ctx context.Context, taskID string, options TaskOptions) ([]map[string]any, error) {
	return c.Stream(ctx, "SubscribeToTask", c.taskParams(taskID, options))
}

func (c *Client) SubscribeToTaskChan(
	ctx context.Context,
	taskID string,
	options TaskOptions,
) <-chan StreamResult {
	return c.StreamChan(ctx, "SubscribeToTask", c.taskParams(taskID, options))
}

func (c *Client) SubscribeToTaskEach(
	ctx context.Context,
	taskID string,
	options TaskOptions,
	handler StreamHandler,
) error {
	return c.StreamEach(ctx, "SubscribeToTask", c.taskParams(taskID, options), handler)
}

func (c *Client) RPC(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.newID("rpc"),
		"method":  method,
		"params":  mapOrEmpty(params),
	}
	response, err := c.requestObject(ctx, http.MethodPost, "/rpc", payload)
	if err != nil {
		return nil, err
	}
	return extractResult(response)
}

func (c *Client) Stream(ctx context.Context, method string, params map[string]any) ([]map[string]any, error) {
	results := make([]map[string]any, 0, 4)
	err := c.StreamEach(ctx, method, params, func(event map[string]any) error {
		results = append(results, event)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) StreamChan(ctx context.Context, method string, params map[string]any) <-chan StreamResult {
	if ctx == nil {
		ctx = context.Background()
	}

	results := make(chan StreamResult)
	go func() {
		defer close(results)

		err := c.StreamEach(ctx, method, params, func(event map[string]any) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case results <- StreamResult{Event: event}:
				return nil
			}
		})
		if err == nil {
			return
		}

		select {
		case <-ctx.Done():
		case results <- StreamResult{Err: err}:
		}
	}()
	return results
}

func (c *Client) StreamEach(ctx context.Context, method string, params map[string]any, handler StreamHandler) error {
	if handler == nil {
		return fmt.Errorf("stream handler must not be nil")
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.newID("stream"),
		"method":  method,
		"params":  mapOrEmpty(params),
	}
	response, err := c.requestStream(ctx, http.MethodPost, "/rpc", payload)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck

	return consumeSSEEvents(response.Body, handler)
}

func (c *Client) requestObject(
	ctx context.Context,
	method string,
	path string,
	payload map[string]any,
) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var payload any
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			payload = string(responseBody)
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Payload: payload}
	}

	var result map[string]any
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) requestStream(
	ctx context.Context,
	method string,
	path string,
	payload map[string]any,
) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close() //nolint:errcheck
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		var payload any
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			payload = string(responseBody)
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Payload: payload}
	}
	return resp, nil
}

func extractResult(payload map[string]any) (map[string]any, error) {
	if rawError, ok := payload["error"].(map[string]any); ok {
		code := 0
		if value, ok := rawError["code"].(float64); ok {
			code = int(value)
		}
		data, _ := rawError["data"].(map[string]any)
		return nil, &JSONRPCError{
			Code:    code,
			Message: stringValue(rawError["message"]),
			Data:    data,
		}
	}

	result, ok := payload["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected A2A response: %v", payload)
	}
	return result, nil
}

func parseSSEEvents(reader io.Reader) ([]map[string]any, error) {
	results := make([]map[string]any, 0, 4)
	err := consumeSSEEvents(reader, func(event map[string]any) error {
		results = append(results, event)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func consumeSSEEvents(reader io.Reader, handler StreamHandler) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		rawPayload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if rawPayload == "" {
			continue
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
			return err
		}
		result, err := extractResult(payload)
		if err != nil {
			return err
		}
		if err := handler(result); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (c *Client) message(text string, options MessageOptions) map[string]any {
	message := map[string]any{
		"messageId": firstNonEmpty(options.MessageID, c.newID("msg")),
		"role":      "user",
		"parts":     []map[string]string{{"text": text}},
	}
	if options.TaskID != "" {
		message["taskId"] = options.TaskID
	}
	if options.ContextID != "" {
		message["contextId"] = options.ContextID
	}
	if options.Metadata != nil {
		message["metadata"] = options.Metadata
	}
	return message
}

func (c *Client) taskParams(taskID string, options TaskOptions) map[string]any {
	params := map[string]any{
		"id":               taskID,
		"includeArtifacts": options.IncludeArtifacts,
	}
	if options.HistoryLength != nil {
		params["historyLength"] = *options.HistoryLength
	}
	return params
}

func (c *Client) newID(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func mapOrEmpty(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func firstNonEmpty(primary string, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
