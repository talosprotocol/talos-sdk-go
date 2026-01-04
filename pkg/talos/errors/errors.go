package errors

import (
	"fmt"
)

// TalosErrorCode represents standard Talos error codes.
type TalosErrorCode string

const (
	// Authorization Errors
	CodeDenied            TalosErrorCode = "TALOS_DENIED"
	CodeInvalidCapability TalosErrorCode = "TALOS_INVALID_CAPABILITY"

	// Protocol Errors
	CodeProtocolMismatch TalosErrorCode = "TALOS_PROTOCOL_MISMATCH"
	CodeFrameInvalid     TalosErrorCode = "TALOS_FRAME_INVALID"

	// Crypto Errors
	CodeCryptoError  TalosErrorCode = "TALOS_CRYPTO_ERROR"
	CodeInvalidInput TalosErrorCode = "TALOS_INVALID_INPUT"

	// Transport Errors
	CodeTransportTimeout TalosErrorCode = "TALOS_TRANSPORT_TIMEOUT"
	CodeTransportError   TalosErrorCode = "TALOS_TRANSPORT_ERROR"
)

// TalosError is the canonical error type for Talos SDK.
type TalosError struct {
	Code      TalosErrorCode
	Message   string
	Details   map[string]interface{}
	RequestID string
	Cause     error
}

func (e *TalosError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *TalosError) Unwrap() error {
	return e.Cause
}

// New creates a new TalosError.
func New(code TalosErrorCode, message string, opts ...Option) *TalosError {
	err := &TalosError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(err)
	}
	return err
}

// Option configures a TalosError.
type Option func(*TalosError)

func WithDetails(details map[string]interface{}) Option {
	return func(e *TalosError) {
		for k, v := range details {
			e.Details[k] = v
		}
	}
}

func WithRequestID(id string) Option {
	return func(e *TalosError) {
		e.RequestID = id
	}
}

func WithCause(cause error) Option {
	return func(e *TalosError) {
		e.Cause = cause
	}
}
