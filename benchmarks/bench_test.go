package benchmarks

import (
	"crypto/ed25519"
	"testing"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/canonical"
	"github.com/talosprotocol/talos-sdk-go/pkg/talos/crypto"
	"github.com/talosprotocol/talos-sdk-go/pkg/talos/wallet"
)

func BenchmarkWalletGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := wallet.Generate("BenchmarkUser")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWalletFromSeed(b *testing.B) {
	seed := make([]byte, ed25519.SeedSize)
	for i := 0; i < b.N; i++ {
		_, err := wallet.FromSeed(seed, "BenchmarkUser")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCryptoSign64B(b *testing.B) {
	_, priv, _ := crypto.GenerateKey()
	msg := make([]byte, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.Sign(priv, msg)
	}
}

func BenchmarkCryptoSign10KB(b *testing.B) {
	_, priv, _ := crypto.GenerateKey()
	msg := make([]byte, 10*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.Sign(priv, msg)
	}
}

func BenchmarkCryptoVerify(b *testing.B) {
	pub, priv, _ := crypto.GenerateKey()
	msg := make([]byte, 64)
	sig := crypto.Sign(priv, msg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.Verify(pub, msg, sig)
	}
}

func BenchmarkSHA256(b *testing.B) {
	data := []byte("hello world")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.SHA256(data)
	}
}

func BenchmarkCanonicalJSON(b *testing.B) {
	data := map[string]interface{}{
		"foo": "bar",
		"baz": 123,
		"inner": map[string]interface{}{
			"a": "b",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := canonical.Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
