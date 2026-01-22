package domain

import (
	"fmt"
	"strings"
)

// Dict represents an RFC 8941 Dictionary.
type Dict map[string]Item

// Item represents a Structured Field Item with Parameters.
type Item struct {
	Value  interface{}
	Params map[string]interface{}
}

// EncodeDict encodes a Dict into an RFC 8941 Structured Field value.
func EncodeDict(d Dict) (string, error) {
	var sb strings.Builder
	first := true

	for key, item := range d {
		if !first {
			sb.WriteString(", ")
		}
		first = false

		if err := validateKey(key); err != nil {
			return "", err
		}
		sb.WriteString(key)

		if item.Value != nil {
			if b, ok := item.Value.(bool); ok && b {
				// skip
			} else {
				sb.WriteString("=")
				valStr, err := encodeValue(item.Value)
				if err != nil {
					return "", err
				}
				sb.WriteString(valStr)
			}
		}

		for pKey, pVal := range item.Params {
			sb.WriteString(";")
			if err := validateKey(pKey); err != nil {
				return "", err
			}
			sb.WriteString(pKey)
			if b, ok := pVal.(bool); ok && b {
				// skip
			} else {
				sb.WriteString("=")
				valStr, err := encodeValue(pVal)
				if err != nil {
					return "", err
				}
				sb.WriteString(valStr)
			}
		}
	}

	return sb.String(), nil
}

func validateKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("empty key")
	}
	for _, r := range key {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == '*') {
			return fmt.Errorf("invalid character in key: %c", r)
		}
	}
	return nil
}

func encodeValue(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return "\"" + strings.ReplaceAll(strings.ReplaceAll(val, "\\", "\\\\"), "\"", "\\\"") + "\"", nil
	case bool:
		if val {
			return "?1", nil
		}
		return "?0", nil
	case int, int64:
		return fmt.Sprintf("%d", val), nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}
