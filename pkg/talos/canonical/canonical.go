package canonical

import (
	"encoding/json"
	"fmt"

	"github.com/gowebpki/jcs"
)

// Marshal returns the RFC 8785 canonical JSON encoding of v.
func Marshal(v interface{}) ([]byte, error) {
	// JCS requires generic map/slice input usually, or struct logic.
	// gowebpki/jcs Transform usually takes interface{}.
	// Let's check signature.
	// If v is a struct, we might need to round-trip to map first OR the lib handles it.
	// Standard JCS libs often work on the JSON model (map[string]interface{}), not structs directly.
	// Safe approach: json.Marshal -> Unmarshal(interface{}) -> JCS.Transform.

	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json marshal failed: %w", err)
	}

	// jcs.Transform (gowebpki) takes raw JSON bytes and returns canonical bytes.
	// It parses the JSON internally, sorts keys, and handles serializations.
	return jcs.Transform(raw)
}
