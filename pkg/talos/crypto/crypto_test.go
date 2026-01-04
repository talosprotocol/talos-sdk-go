package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("expected pub key length %d, got %d", ed25519.PublicKeySize, len(pub))
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("expected priv key length %d, got %d", ed25519.PrivateKeySize, len(priv))
	}
}

func TestFromSeed(t *testing.T) {
	tests := []struct {
		name      string
		seed      []byte
		expectErr bool
	}{
		{
			name:      "Valid Seed",
			seed:      make([]byte, ed25519.SeedSize),
			expectErr: false,
		},
		{
			name:      "Invalid Seed Length",
			seed:      make([]byte, 31),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, priv, err := FromSeed(tt.seed)
			if (err != nil) != tt.expectErr {
				t.Errorf("FromSeed() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr {
				if len(pub) != ed25519.PublicKeySize {
					t.Errorf("expected pub key length %d, got %d", ed25519.PublicKeySize, len(pub))
				}
				if len(priv) != ed25519.PrivateKeySize {
					t.Errorf("expected priv key length %d, got %d", ed25519.PrivateKeySize, len(priv))
				}
			}
		})
	}
}

func TestSignAndVerify(t *testing.T) {
	pub, priv, _ := GenerateKey()
	msg := []byte("hello world")

	sig := Sign(priv, msg)
	if len(sig) != ed25519.SignatureSize {
		t.Errorf("expected signature length %d, got %d", ed25519.SignatureSize, len(sig))
	}

	if !Verify(pub, msg, sig) {
		t.Error("Verify failed for valid signature")
	}

	if Verify(pub, []byte("wrong message"), sig) {
		t.Error("Verify passed for wrong message")
	}
}

func TestSHA256(t *testing.T) {
	data := []byte("hello")
	hash := SHA256(data)
	if len(hash) != 32 {
		t.Errorf("expected hash length 32, got %d", len(hash))
	}
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hex.EncodeToString(hash) != expected {
		t.Errorf("expected hash %s, got %x", expected, hash)
	}
}
