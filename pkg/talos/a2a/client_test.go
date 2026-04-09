package a2a

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeDoer struct {
	handle func(*http.Request) (*http.Response, error)
}

func (f fakeDoer) Do(req *http.Request) (*http.Response, error) {
	return f.handle(req)
}

func jsonResponse(status int, payload map[string]any) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}

func TestClientGetAgentCardAndExtensions(t *testing.T) {
	card := map[string]any{
		"name": "test-agent",
		"supportedInterfaces": []map[string]any{
			{"transport": "https", "url": "https://example.test/rpc"},
		},
		"capabilities": map[string]any{
			"extensions": []map[string]any{
				{"uri": TalosSecureChannelsExtension},
				{"uri": TalosAttestationExtension},
			},
		},
	}

	client := NewClient("https://example.test", "sk-test", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/.well-known/agent-card.json" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
				t.Fatalf("unexpected authorization header %q", got)
			}
			return jsonResponse(http.StatusOK, card), nil
		},
	}))
	got, err := client.GetAgentCard(context.Background())
	if err != nil {
		t.Fatalf("GetAgentCard failed: %v", err)
	}

	if len(SupportedInterfaces(got)) != 1 {
		t.Fatalf("supported interface count mismatch: %v", SupportedInterfaces(got))
	}
	if !SupportsTalosSecureChannels(got) {
		t.Fatalf("expected secure channels extension")
	}
	if !SupportsTalosAttestation(got) {
		t.Fatalf("expected attestation extension")
	}
	if SupportsTalosCompatJSONRPC(got) {
		t.Fatalf("did not expect compat jsonrpc extension")
	}
}

func TestClientRPCUsesCanonicalMethods(t *testing.T) {
	methods := make([]string, 0, 2)

	client := NewClient("https://example.test", "sk-test", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/rpc" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
				t.Fatalf("unexpected authorization header %q", got)
			}

			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			methods = append(methods, payload["method"].(string))

			return jsonResponse(http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      payload["id"],
				"result": map[string]any{
					"ok": true,
				},
			}), nil
		},
	}))
	if _, err := client.GetAuthenticatedExtendedAgentCard(context.Background()); err != nil {
		t.Fatalf("GetAuthenticatedExtendedAgentCard failed: %v", err)
	}
	if _, err := client.SendMessage(context.Background(), "hello", MessageOptions{}); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(methods) != 2 {
		t.Fatalf("unexpected method count %d", len(methods))
	}
	if methods[0] != "GetExtendedAgentCard" {
		t.Fatalf("unexpected first method %q", methods[0])
	}
	if methods[1] != "SendMessage" {
		t.Fatalf("unexpected second method %q", methods[1])
	}
}

func TestClientStreamingMethodsParseSSEEvents(t *testing.T) {
	methods := make([]string, 0, 2)

	client := NewClient("https://example.test", "sk-test", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/rpc" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			if got := r.Header.Get("Accept"); got != "text/event-stream" {
				t.Fatalf("unexpected accept header %q", got)
			}

			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			methods = append(methods, payload["method"].(string))

			body := strings.Join([]string{
				"data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-1\",\"result\":{\"index\":1}}",
				"",
				"data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-2\",\"result\":{\"index\":2}}",
				"",
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}))

	events, err := client.SendStreamingMessage(context.Background(), "hello", MessageOptions{})
	if err != nil {
		t.Fatalf("SendStreamingMessage failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("unexpected send stream event count %d", len(events))
	}

	taskEvents, err := client.SubscribeToTask(context.Background(), "task-1", TaskOptions{})
	if err != nil {
		t.Fatalf("SubscribeToTask failed: %v", err)
	}
	if len(taskEvents) != 2 {
		t.Fatalf("unexpected subscribe event count %d", len(taskEvents))
	}

	if methods[0] != "SendStreamingMessage" {
		t.Fatalf("unexpected first stream method %q", methods[0])
	}
	if methods[1] != "SubscribeToTask" {
		t.Fatalf("unexpected second stream method %q", methods[1])
	}
}

func TestClientStreamingHandlersProcessEventsIncrementally(t *testing.T) {
	client := NewClient("https://example.test", "sk-test", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			body := strings.Join([]string{
				"data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-1\",\"result\":{\"index\":1}}",
				"",
				"data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-2\",\"result\":{\"index\":2}}",
				"",
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}))

	var seen []int
	err := client.SendStreamingMessageEach(context.Background(), "hello", MessageOptions{}, func(event map[string]any) error {
		seen = append(seen, int(event["index"].(float64)))
		return nil
	})
	if err != nil {
		t.Fatalf("SendStreamingMessageEach failed: %v", err)
	}
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("unexpected incremental events: %v", seen)
	}
}

func TestClientStreamingChannelsYieldEventsAndErrors(t *testing.T) {
	okClient := NewClient("https://example.test", "sk-test", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			body := strings.Join([]string{
				"data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-1\",\"result\":{\"index\":1}}",
				"",
				"data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-2\",\"result\":{\"index\":2}}",
				"",
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}))

	var seen []int
	for item := range okClient.SendStreamingMessageChan(context.Background(), "hello", MessageOptions{}) {
		if item.Err != nil {
			t.Fatalf("unexpected stream error: %v", item.Err)
		}
		seen = append(seen, int(item.Event["index"].(float64)))
	}
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("unexpected channel events: %v", seen)
	}

	errClient := NewClient("https://example.test", "", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			body := "data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-err\",\"error\":{\"code\":-32000,\"message\":\"stream failed\",\"data\":{\"reason\":\"denied\"}}}\n\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}))

	var streamErr error
	for item := range errClient.SubscribeToTaskChan(context.Background(), "task-1", TaskOptions{}) {
		if item.Err != nil {
			streamErr = item.Err
		}
	}
	if streamErr == nil {
		t.Fatal("expected channel stream error")
	}

	rpcErr, ok := streamErr.(*JSONRPCError)
	if !ok {
		t.Fatalf("expected JSONRPCError, got %T", streamErr)
	}
	if rpcErr.Code != -32000 {
		t.Fatalf("unexpected code %d", rpcErr.Code)
	}
}

func TestClientStreamJSONRPCError(t *testing.T) {
	client := NewClient("https://example.test", "", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			body := "data: {\"jsonrpc\":\"2.0\",\"id\":\"evt-err\",\"error\":{\"code\":-32000,\"message\":\"stream failed\",\"data\":{\"reason\":\"denied\"}}}\n\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}))

	_, err := client.SubscribeToTask(context.Background(), "task-1", TaskOptions{})
	if err == nil {
		t.Fatal("expected stream error")
	}

	rpcErr, ok := err.(*JSONRPCError)
	if !ok {
		t.Fatalf("expected JSONRPCError, got %T", err)
	}
	if rpcErr.Code != -32000 {
		t.Fatalf("unexpected code %d", rpcErr.Code)
	}
}

func TestClientJSONRPCError(t *testing.T) {
	client := NewClient("https://example.test", "", WithHTTPClient(fakeDoer{
		handle: func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      "rpc-test",
				"error": map[string]any{
					"code":    -32603,
					"message": "rpc failed",
					"data": map[string]any{
						"reason": "denied",
					},
				},
			}), nil
		},
	}))
	_, err := client.RPC(context.Background(), "GetTask", map[string]any{"id": "task-1"})
	if err == nil {
		t.Fatal("expected RPC error")
	}

	rpcErr, ok := err.(*JSONRPCError)
	if !ok {
		t.Fatalf("expected JSONRPCError, got %T", err)
	}
	if rpcErr.Code != -32603 {
		t.Fatalf("unexpected code %d", rpcErr.Code)
	}
	if rpcErr.Data["reason"] != "denied" {
		t.Fatalf("unexpected error data: %v", rpcErr.Data)
	}
}
