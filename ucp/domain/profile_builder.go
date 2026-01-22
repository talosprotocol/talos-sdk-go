package domain

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type PlatformProfileBuilder struct {
	ProfileURL string
	PublicKeys map[string]*ecdsa.PublicKey
}

func (b *PlatformProfileBuilder) Build() (*PlatformProfile, error) {
	keys := make([]JWK, 0, len(b.PublicKeys))
	for kid, pub := range b.PublicKeys {
		jwk, err := PublicKeyToJWK(pub)
		if err != nil {
			return nil, err
		}
		if kid == "" || strings.HasPrefix(kid, "auto:") {
			thumb, _ := JWKThumbprint(jwk)
			jwk.Kid = thumb
		} else {
			jwk.Kid = kid
		}
		jwk.Use = "sig"
		jwk.Alg = "ES256"
		keys = append(keys, jwk)
	}

	p := &PlatformProfile{
		Ucp:         VersionInfo{Version: "2026-01-11"},
		SigningKeys: keys,
		Services: PlatformServices{
			Platform: PlatformService{
				Profile: PlatformProfileEndpoint{
					URL: b.ProfileURL,
				},
			},
		},
	}
	return p, nil
}

func PublicKeyToJWK(pub *ecdsa.PublicKey) (JWK, error) {
	if pub.Curve.Params().Name != "P-256" {
		return JWK{}, fmt.Errorf("only P-256 (ES256) is supported for UCP")
	}

	byteLen := (pub.Curve.Params().N.BitLen() + 7) / 8
	xBytes := pub.X.Bytes()
	yBytes := pub.Y.Bytes()

	x := make([]byte, byteLen)
	y := make([]byte, byteLen)
	copy(x[byteLen-len(xBytes):], xBytes)
	copy(y[byteLen-len(yBytes):], yBytes)

	return JWK{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(x),
		Y:   base64.RawURLEncoding.EncodeToString(y),
	}, nil
}

func JWKThumbprint(j JWK) (string, error) {
	m := map[string]string{
		"crv": j.Crv,
		"kty": j.Kty,
		"x":   j.X,
		"y":   j.Y,
	}

	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)
	return base64.RawURLEncoding.EncodeToString(hash[:]), nil
}
