package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/wallet"
)

type TestVector struct {
	TestID   string                 `json:"test_id"`
	Inputs   map[string]interface{} `json:"inputs"`
	Expected map[string]interface{} `json:"expected"`
}

type VectorFile struct {
	Vectors []TestVector `json:"vectors"`
}

func main() {
	var vectors []TestVector

	// Vector 1: Standard Signature
	seedHex := "0000000000000000000000000000000000000000000000000000000000000000"
	seedBytes, _ := hex.DecodeString(seedHex)
	w, _ := wallet.FromSeed(seedBytes, "")

	msg := "hello from go"
	sig := w.Sign([]byte(msg))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	vectors = append(vectors, TestVector{
		TestID: "go_sign_1",
		Inputs: map[string]interface{}{
			"seed_hex":     seedHex,
			"message_utf8": msg,
		},
		Expected: map[string]interface{}{
			"did":                 w.DID(),
			"signature_base64url": sigB64,
			"verify":              true,
		},
	})

	// Vector 2: Another seed
	seedHex2 := "0101010101010101010101010101010101010101010101010101010101010101"
	seedBytes2, _ := hex.DecodeString(seedHex2)
	w2, _ := wallet.FromSeed(seedBytes2, "")

	msg2 := "interop test"
	sig2 := w2.Sign([]byte(msg2))
	sigB64_2 := base64.RawURLEncoding.EncodeToString(sig2)

	vectors = append(vectors, TestVector{
		TestID: "go_sign_2",
		Inputs: map[string]interface{}{
			"seed_hex":     seedHex2,
			"message_utf8": msg2,
		},
		Expected: map[string]interface{}{
			"did":                 w2.DID(),
			"signature_base64url": sigB64_2,
			"verify":              true,
		},
	})

	// Output
	vf := VectorFile{Vectors: vectors}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(vf); err != nil {
		panic(err)
	}
}
