package wallet

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/crypto"
	"github.com/talosprotocol/talos-sdk-go/pkg/talos/errors"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// EncodeBase58 encodes bytes to base58 (Bitcoin style).
func EncodeBase58(input []byte) string {
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append(result, base58Alphabet[mod.Int64()])
	}

	// Leading zeros
	for _, b := range input {
		if b != 0 {
			break
		}
		result = append(result, base58Alphabet[0])
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// Wallet represents a Talos identity.
type Wallet struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	name       string
}

// Generate creates a new wallet securely.
// Name is optional.
func Generate(name string) (*Wallet, error) {
	pub, priv, err := crypto.GenerateKey()
	if err != nil {
		return nil, errors.New(errors.CodeCryptoError, "failed to generate key", errors.WithCause(err))
	}
	return &Wallet{
		privateKey: priv,
		publicKey:  pub,
		name:       name,
	}, nil
}

// FromSeed creates a wallet from a 32-byte seed.
func FromSeed(seed []byte, name string) (*Wallet, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, errors.New(errors.CodeInvalidInput, fmt.Sprintf("seed must be %d bytes", ed25519.SeedSize))
	}
	pub, priv, err := crypto.FromSeed(seed)
	if err != nil {
		return nil, errors.New(errors.CodeCryptoError, "failed to derive key from seed", errors.WithCause(err))
	}
	return &Wallet{
		privateKey: priv,
		publicKey:  pub,
		name:       name,
	}, nil
}

// PublicKey returns the public key bytes.
func (w *Wallet) PublicKey() []byte {
	return []byte(w.publicKey)
}

// Address returns the hex-encoded SHA256 hash of the public key.
func (w *Wallet) Address() string {
	hash := crypto.SHA256(w.PublicKey())
	return hex.EncodeToString(hash)
}

// DID returns the did:key identifier.
// Format: did:key:z + base58(0xed01 + pubkey)
func (w *Wallet) DID() string {
	prefix := []byte{0xed, 0x01}
	input := append(prefix, w.PublicKey()...)
	return "did:key:z" + EncodeBase58(input)
}

// Sign signs a message.
func (w *Wallet) Sign(message []byte) []byte {
	return crypto.Sign(w.privateKey, message)
}

// Verify verifies a signature.
func Verify(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	// crypto.Verify handles generic keys, but we pass bytes here
	return crypto.Verify(ed25519.PublicKey(publicKey), message, signature)
}

// Name returns the wallet name.
func (w *Wallet) Name() string {
	return w.name
}
