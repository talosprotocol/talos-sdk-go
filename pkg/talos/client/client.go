package client

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// TalosError represents a typed API error.
type TalosError struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func (e *TalosError) Error() string {
	return fmt.Sprintf("API Error %d: %s (req_id: %s)", e.Code, e.Message, e.RequestID)
}

type GatewayClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *GatewayClient) GetResource(path string) (string, error) {
	url := fmt.Sprintf("%s/%s", c.BaseURL, path)
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", &TalosError{
			Code:      resp.StatusCode,
			Message:   string(body), // Simplified text body for now
			RequestID: resp.Header.Get("x-request-id"),
		}
	}

	return string(body), nil
}

// StreamResource returns a channel of events (lines).
// Caller must close the returned channel or cancellation context to stop?
// Ideally, return a ReadCloser.
func (c *GatewayClient) StreamResource(path string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/%s", c.BaseURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use a client with no timeout for streaming
	streamClient := *c.HTTP
	streamClient.Timeout = 0

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close() //nolint:errcheck
		return nil, &TalosError{
			Code:    resp.StatusCode,
			Message: "Stream Error",
		}
	}

	return resp.Body, nil
}
