package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Identity represents a Talos identity (Ed25519 keypair).
type Identity struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// Generate creates a new random identity.
func Generate() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return &Identity{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// Sign signs a message using the private key.
func (i *Identity) Sign(message []byte) []byte {
	return ed25519.Sign(i.PrivateKey, message)
}

// Address returns the Base64Url encoding of the public key.
func (i *Identity) Address() string {
	return base64.RawURLEncoding.EncodeToString(i.PublicKey)
}

// Verify verifies a signature against the public key.
func (i *Identity) Verify(message, signature []byte) bool {
	return ed25519.Verify(i.PublicKey, message, signature)
}
