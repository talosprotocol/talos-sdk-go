package canonical

import (
	"encoding/json"
)

// Marshal returns the canonical JSON encoding of v.
// It ensures keys are sorted alphabetically and there is no whitespace.
func Marshal(v interface{}) ([]byte, error) {
	// First marshal to bytes to handle struct tags etc.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Unmarshal into interface{} to lose struct ordering and get generic map/slice
	var generic interface{}
	if err := json.Unmarshal(b, &generic); err != nil {
		return nil, err
	}

	// Marshaling a map[string]interface{} guarantees sorted keys in Go.
	// encoding/json output has no whitespace by default.
	return json.Marshal(generic)
}
