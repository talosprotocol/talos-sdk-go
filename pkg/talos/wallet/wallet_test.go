package wallet

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	w, err := Generate("Alice")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if w.Name() != "Alice" {
		t.Errorf("expected name Alice, got %s", w.Name())
	}
	if len(w.PublicKey()) != 32 {
		t.Errorf("expected pub key length 32, got %d", len(w.PublicKey()))
	}
}

func TestFromSeed(t *testing.T) {
	seed := make([]byte, 32)
	w, err := FromSeed(seed, "Bob")
	if err != nil {
		t.Fatalf("FromSeed failed: %v", err)
	}
	if w.Name() != "Bob" {
		t.Errorf("expected name Bob, got %s", w.Name())
	}

	// Test Invalid Seed
	_, err = FromSeed(make([]byte, 31), "Bob")
	if err == nil {
		t.Error("expected error for invalid seed length, got nil")
	}
}

func TestAddressAndDID(t *testing.T) {
	seed := make([]byte, 32) // all zeros
	w, _ := FromSeed(seed, "Test")

	// Verify consistent address
	addr := w.Address()
	if len(addr) != 64 {
		t.Errorf("expected address length 64 (hex), got %d", len(addr))
	}

	// Verify DID prefix
	did := w.DID()
	if did[:9] != "did:key:z" {
		t.Errorf("expected did prefix did:key:z, got %s", did)
	}
}

func TestSign(t *testing.T) {
	w, _ := Generate("Signer")
	msg := []byte("data")
	sig := w.Sign(msg)

	if len(sig) != 64 {
		t.Errorf("expected signature length 64, got %d", len(sig))
	}

	if !Verify(w.PublicKey(), msg, sig) {
		t.Error("Verify failed")
	}

	// Test Verify failure
	if Verify(make([]byte, 32), msg, sig) {
		t.Error("Verify passed with wrong key")
	}
}

func TestEncodeBase58(t *testing.T) {
	input := []byte("hello world")
	encoded := EncodeBase58(input)
	if encoded == "" {
		t.Error("encoded string is empty")
	}
}
