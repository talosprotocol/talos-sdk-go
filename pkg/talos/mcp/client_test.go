package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCallTool_HappyPath(t *testing.T) {
	serverID := "test-server"
	toolName := "echo-tool"
	requestID := "req-123"
	sessionID := "sess-456"
	input := map[string]any{"msg": "hello"}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Method
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		// Verify Path & Escaping
		expectedPath := "/v1/mcp/servers/test-server/tools/echo-tool:call"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify Headers
		if r.Header.Get("Authorization") != "Bearer sk-test-key" {
			t.Errorf("Wrong Authorization header")
		}
		if r.Header.Get("X-Request-Id") != requestID {
			t.Errorf("Wrong X-Request-Id header, got %s", r.Header.Get("X-Request-Id"))
		}

		// Verify Body
		var req ToolCallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode body: %v", err)
		}
		if req.ServerID != serverID || req.ToolName != toolName || req.RequestID != requestID {
			t.Errorf("Body content mismatch")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", requestID)
		_ = json.NewEncoder(w).Encode(ToolCallResponse{
			RequestID: requestID,
			Output:    json.RawMessage(`{"received": "hello"}`),
			TimingMS:  10,
			AuditRef:  "audit-abc",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "sk-test-key")
	resp, err := client.CallTool(context.Background(), serverID, toolName, input, requestID, sessionID)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if resp.RequestID != requestID {
		t.Errorf("Response request ID mismatch")
	}

	var out struct {
		Received string `json:"received"`
	}
	if err := resp.DecodeOutput(&out); err != nil {
		t.Errorf("DecodeOutput failed: %v", err)
	}
	if out.Received != "hello" {
		t.Errorf("Output content mismatch")
	}
}

func TestCallTool_ErrorParsing(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		respBody     string
		expectedCode string
		snippetCheck string
	}{
		{
			name:         "JSON Error",
			status:       403,
			respBody:     `{"error": {"code": "POLICY_DENIED", "message": "unauthorized"}}`,
			expectedCode: "POLICY_DENIED",
		},
		{
			name:         "Raw Error",
			status:       502,
			respBody:     "Bad Gateway - Proxy error",
			expectedCode: "http_502",
			snippetCheck: "Bad Gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.respBody))
			}))
			defer ts.Close()

			client := NewClient(ts.URL, "sk-key")
			_, err := client.CallTool(context.TODO(), "s", "t", nil, "", "")
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			apiErr, ok := err.(*APIError)
			if !ok {
				t.Fatalf("Expected *APIError, got %T", err)
			}

			if apiErr.Status != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, apiErr.Status)
			}
			if apiErr.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, apiErr.Code)
			}
			if tt.snippetCheck != "" && !strings.Contains(apiErr.BodySnippet, tt.snippetCheck) {
				t.Errorf("Snippet %q not found in %q", tt.snippetCheck, apiErr.BodySnippet)
			}
		})
	}
}

func TestCallTool_Safety(t *testing.T) {
	t.Run("Response Size Limit", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			largeBody := strings.Repeat("A", 100) // Small enough for test, but we'll set limit lower
			_, _ = w.Write([]byte(largeBody))
		}))
		defer ts.Close()

		client := NewClient(ts.URL, "sk", WithMaxResponseBytes(10))
		_, err := client.CallTool(context.TODO(), "s", "t", nil, "", "")
		if err == nil {
			t.Fatal("Expected size error, got nil")
		}
	})

	t.Run("URL Escaping", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expected := "/v1/mcp/servers/svc%2F1/tools/echo%3A2:call"
			if r.URL.EscapedPath() != expected {
				t.Errorf("Expected escaped path %s, got %s", expected, r.URL.EscapedPath())
			}
		}))
		defer ts.Close()

		client := NewClient(ts.URL, "sk")
		_, _ = client.CallTool(context.TODO(), "svc/1", "echo:2", nil, "", "")
	})
}

func TestCallTool_Context(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(ToolCallResponse{})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "sk")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "s", "t", nil, "", "")
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context error, got %v", err)
	}
}
