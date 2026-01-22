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
