package version

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/canonical"
)

func TestVersionExportsArePresent(t *testing.T) {
	t.Parallel()

	if SDK_VERSION != "1.0.0" {
		t.Fatalf("SDK_VERSION = %q, want %q", SDK_VERSION, "1.0.0")
	}
	if SUPPORTED_PROTOCOL_RANGE != [2]string{"1.0", "1.x"} {
		t.Fatalf("SUPPORTED_PROTOCOL_RANGE = %v, want %v", SUPPORTED_PROTOCOL_RANGE, [2]string{"1.0", "1.x"})
	}
}

func TestContractManifestHashMatchesCanonicalManifest(t *testing.T) {
	t.Parallel()

	manifestPath := findContractManifest(t)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read contract manifest: %v", err)
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode contract manifest: %v", err)
	}

	canonicalBytes, err := canonical.Marshal(payload)
	if err != nil {
		t.Fatalf("canonicalize contract manifest: %v", err)
	}

	digest := sha256.Sum256(canonicalBytes)
	expected := base64.RawURLEncoding.EncodeToString(digest[:])

	if CONTRACT_MANIFEST_HASH != expected {
		t.Fatalf("CONTRACT_MANIFEST_HASH = %q, want %q", CONTRACT_MANIFEST_HASH, expected)
	}
	if containsColon(CONTRACT_MANIFEST_HASH) {
		t.Fatalf("CONTRACT_MANIFEST_HASH should be base64url without prefix, got %q", CONTRACT_MANIFEST_HASH)
	}
}

func findContractManifest(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	dir := filepath.Dir(filename)
	for {
		candidate := filepath.Join(dir, "contracts", "sdk", "contract_manifest.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatal("could not find contracts/sdk/contract_manifest.json")
	return ""
}

func containsColon(value string) bool {
	for _, r := range value {
		if r == ':' {
			return true
		}
	}
	return false
}
