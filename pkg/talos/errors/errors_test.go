package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestErrors(t *testing.T) {
	baseErr := errors.New("root cause")

	val := New(CodeInvalidInput, "bad input",
		WithDetails(map[string]interface{}{"field": "age"}),
		WithRequestID("req-123"),
		WithCause(baseErr),
	)

	if val.Code != CodeInvalidInput {
		t.Errorf("expected code %s, got %s", CodeInvalidInput, val.Code)
	}

	if val.RequestID != "req-123" {
		t.Errorf("expected request ID req-123, got %s", val.RequestID)
	}

	if val.Unwrap() != baseErr {
		t.Error("Unwrap expected baseErr")
	}

	msg := val.Error()
	if !strings.Contains(msg, "bad input") {
		t.Error("Error string should contain message")
	}
	if !strings.Contains(msg, "root cause") {
		t.Error("Error string should contain cause")
	}
}
