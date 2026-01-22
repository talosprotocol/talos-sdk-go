package domain

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

type Signer struct {
	PrivateKey *ecdsa.PrivateKey
	KID        string
}

type JWSHeader struct {
	Alg  string   `json:"alg"`
	KID  string   `json:"kid"`
	B64  bool     `json:"b64"`
	Crit []string `json:"crit"`
}

func (s *Signer) SignBody(body []byte) (string, error) {
	if s.PrivateKey == nil {
		return "", fmt.Errorf("private key is required")
	}

	header := JWSHeader{
		Alg:  "ES256",
		KID:  s.KID,
		B64:  false,
		Crit: []string{"b64"},
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	protected := base64.RawURLEncoding.EncodeToString(headerJSON)
	signingInput := protected + "." + string(body)

	hashed := sha256.Sum256([]byte(signingInput))
	r, ss, err := ecdsa.Sign(rand.Reader, s.PrivateKey, hashed[:])
	if err != nil {
		return "", err
	}

	curveOrder := s.PrivateKey.Curve.Params().N
	byteLen := (curveOrder.BitLen() + 7) / 8

	rawSig := make([]byte, 2*byteLen)
	rBytes := r.Bytes()
	sBytes := ss.Bytes()

	copy(rawSig[byteLen-len(rBytes):byteLen], rBytes)
	copy(rawSig[2*byteLen-len(sBytes):], sBytes)

	sigB64 := base64.RawURLEncoding.EncodeToString(rawSig)
	return protected + ".." + sigB64, nil
}

func VerifyBody(publicKey *ecdsa.PublicKey, body []byte, jws string) error {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWS format: expected 3 parts")
	}

	protectedB64 := parts[0]
	payloadB64 := parts[1]
	signatureB64 := parts[2]

	if payloadB64 != "" {
		return fmt.Errorf("invalid detached JWS: payload must be empty")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(protectedB64)
	if err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	}

	var header JWSHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return fmt.Errorf("failed to unmarshal header: %w", err)
	}

	if header.Alg != "ES256" {
		return fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}
	if header.B64 {
		return fmt.Errorf("invalid UCP signature: b64 must be false")
	}

	signingInput := protectedB64 + "." + string(body)
	hashed := sha256.Sum256([]byte(signingInput))

	sigBytes, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	if len(sigBytes) != 64 {
		return fmt.Errorf("invalid signature length")
	}

	rr := new(big.Int).SetBytes(sigBytes[:32])
	ss := new(big.Int).SetBytes(sigBytes[32:])

	if ecdsa.Verify(publicKey, hashed[:], rr, ss) {
		return nil
	}
	return fmt.Errorf("signature verification failed")
}
