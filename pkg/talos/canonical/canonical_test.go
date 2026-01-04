package canonical_test

import (
	"testing"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/canonical"
)

func TestGolden(t *testing.T) {
	// Golden test cases
	cases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple_map",
			input:    map[string]interface{}{"b": 2, "a": 1},
			expected: `{"a":1,"b":2}`,
		},
		{
			name:     "nested_map",
			input:    map[string]interface{}{"x": map[string]int{"z": 3, "y": 2}, "a": 1},
			expected: `{"a":1,"x":{"y":2,"z":3}}`,
		},
		{
			name:     "mixed_types",
			input:    map[string]interface{}{"id": 123, "active": true, "name": "test"},
			expected: `{"active":true,"id":123,"name":"test"}`,
		},
		{
			name:     "empty_map",
			input:    map[string]interface{}{},
			expected: `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonical.Marshal(tc.input)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if string(got) != tc.expected {
				t.Errorf("got %s, want %s", string(got), tc.expected)
			}
		})
	}
}

func TestPermutation(t *testing.T) {
	// Permutation test: Different insertion orders must produce identical output

	// Map 1: Insert A then B
	m1 := make(map[string]int)
	m1["a"] = 1
	m1["b"] = 2

	// Map 2: Insert B then A
	m2 := make(map[string]int)
	m2["b"] = 2
	m2["a"] = 1

	b1, err := canonical.Marshal(m1)
	if err != nil {
		t.Fatalf("m1 marshal failed: %v", err)
	}

	b2, err := canonical.Marshal(m2)
	if err != nil {
		t.Fatalf("m2 marshal failed: %v", err)
	}

	if string(b1) != string(b2) {
		t.Errorf("Permutation failed: %s != %s", string(b1), string(b2))
	}

	// Complex nested permutation
	// Nest 1: x:{z,y}
	n1 := map[string]interface{}{
		"x": map[string]int{"z": 3, "y": 2},
	}
	// Nest 2: x:{y,z}
	n2 := map[string]interface{}{
		"x": map[string]int{"y": 2, "z": 3},
	}

	nb1, _ := canonical.Marshal(n1)
	nb2, _ := canonical.Marshal(n2)

	if string(nb1) != string(nb2) {
		t.Errorf("Nested permutation failed: %s != %s", string(nb1), string(nb2))
	}
}
