package domain

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/gowebpki/jcs"
)

type Ap2MerchantAuthorization struct{}

func (a *Ap2MerchantAuthorization) Verify(publicKey *ecdsa.PublicKey, payload interface{}, jws string) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	canonical, err := jcs.Transform(payloadJSON)
	if err != nil {
		return fmt.Errorf("JCS transformation failed: %w", err)
	}

	parts := strings.Split(jws, "..")
	if len(parts) != 2 {
		return fmt.Errorf("invalid AP2 JWS format: expected 2 parts separated by '..'")
	}

	protectedB64 := parts[0]
	signatureB64 := parts[1]

	payloadB64 := base64.RawURLEncoding.EncodeToString(canonical)
	signingInput := protectedB64 + "." + payloadB64

	hashed := sha256.Sum256([]byte(signingInput))

	sigBytes, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	if len(sigBytes) != 64 {
		return fmt.Errorf("invalid signature length: %d", len(sigBytes))
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	ss := new(big.Int).SetBytes(sigBytes[32:])

	if ecdsa.Verify(publicKey, hashed[:], r, ss) {
		return nil
	}

	return fmt.Errorf("AP2 signature verification failed")
}
