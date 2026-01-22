package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/gowebpki/jcs"
)

func TestSignAndVerifyBody(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	signer := &Signer{
		PrivateKey: priv,
		KID:        "test-key-1",
	}

	body := []byte(`{"hello":"world"}`)
	jws, err := signer.SignBody(body)
	if err != nil {
		t.Fatalf("SignBody failed: %v", err)
	}

	// Verify
	err = VerifyBody(&priv.PublicKey, body, jws)
	if err != nil {
		t.Errorf("VerifyBody failed: %v", err)
	}

	// Negative: tampered body
	err = VerifyBody(&priv.PublicKey, []byte(`{"hello":"tampered"}`), jws)
	if err == nil {
		t.Error("VerifyBody should have failed for tampered body")
	}
}

func TestAp2Verify(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Manual AP2 token production for test
	payload := map[string]string{"foo": "bar"}
	payloadJSON, _ := json.Marshal(payload)
	canonical, _ := jcs.Transform(payloadJSON)

	header := JWSHeader{Alg: "ES256", KID: "key-1"}
	headerJSON, _ := json.Marshal(header)
	protected := base64.RawURLEncoding.EncodeToString(headerJSON)

	payloadB64 := base64.RawURLEncoding.EncodeToString(canonical)
	signingInput := protected + "." + payloadB64
	hashed := sha256.Sum256([]byte(signingInput))

	r, s, _ := ecdsa.Sign(rand.Reader, priv, hashed[:])

	rawSig := make([]byte, 64)
	copy(rawSig[32-len(r.Bytes()):32], r.Bytes())
	copy(rawSig[64-len(s.Bytes()):], s.Bytes())
	sigB64 := base64.RawURLEncoding.EncodeToString(rawSig)

	jws := protected + ".." + sigB64 // Detached

	ap2 := &Ap2MerchantAuthorization{}
	err = ap2.Verify(&priv.PublicKey, payload, jws)
	if err != nil {
		t.Errorf("Ap2 Verify failed: %v", err)
	}
}

func TestEncodeDict(t *testing.T) {
	tests := []struct {
		name    string
		dict    Dict
		want    string
		wantErr bool
	}{
		{
			name: "single profile",
			dict: Dict{
				"profile": Item{Value: "https://talos.network/profile"},
			},
			want: `profile="https://talos.network/profile"`,
		},
		{
			name: "boolean true",
			dict: Dict{
				"active": Item{Value: true},
			},
			want: "active",
		},
		{
			name: "invalid key",
			dict: Dict{
				"Invalid": Item{Value: "val"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeDict(tt.dict)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeDict() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EncodeDict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeaders_ToMap(t *testing.T) {
	h := &RequestHeaders{
		RequestID:      "req-123",
		IdempotencyKey: "idem-456",
		AgentProfile:   "https://example.com/profile",
	}

	m, err := h.ToMap()
	if err != nil {
		t.Fatalf("ToMap() failed: %v", err)
	}

	if m["Request-Id"] != "req-123" {
		t.Errorf("wrong Request-Id: %s", m["Request-Id"])
	}
	if m["Idempotency-Key"] != "idem-456" {
		t.Errorf("wrong Idempotency-Key: %s", m["Idempotency-Key"])
	}
	if m["UCP-Agent"] != `profile="https://example.com/profile"` {
		t.Errorf("wrong UCP-Agent: %s", m["UCP-Agent"])
	}
}
