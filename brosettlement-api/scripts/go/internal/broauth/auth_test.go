package broauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRESTHeadersSignsMPCInitializeEmptyJSONObject(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "private.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	const keyID = "11111111-2222-4333-8444-555555555555"
	t.Setenv(apiKeyIDEnv, keyID)
	t.Setenv(privateKeyFileEnv, keyPath)

	headers, _, err := RESTHeaders("POST", "/api/v1/mpc/initialize", []byte("{}"))
	if err != nil {
		t.Fatalf("RESTHeaders: %v", err)
	}

	const expectedBodyHash = "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
	if got := headers.Get("X-Api-Body-Hash"); got != expectedBodyHash {
		t.Fatalf("unexpected X-Api-Body-Hash: %q", got)
	}

	canonical := strings.Join([]string{
		"POST",
		"/api/v1/mpc/initialize",
		expectedBodyHash,
		headers.Get("X-Api-Timestamp"),
		headers.Get("X-Api-Nonce"),
		keyID,
	}, "\n")
	signature, err := base64.StdEncoding.DecodeString(headers.Get("X-Api-Signature"))
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(publicKey, []byte(canonical), signature) {
		t.Fatal("signature does not contain the hash of the exact {} body")
	}
	if !RequiresExactEmptyJSONObject("POST", "/api/v1/mpc/initialize") {
		t.Fatal("MPC initialize must use the exact empty JSON object")
	}
}

func TestRESTHeadersKeepsBodyHashEmptyForBodylessGet(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "private.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	t.Setenv(apiKeyIDEnv, "11111111-2222-4333-8444-555555555555")
	t.Setenv(privateKeyFileEnv, keyPath)

	headers, _, err := RESTHeaders("GET", "/api/v1/mpc/status", nil)
	if err != nil {
		t.Fatalf("RESTHeaders: %v", err)
	}
	if got := headers.Get("X-Api-Body-Hash"); got != "" {
		t.Fatalf("unexpected X-Api-Body-Hash: %q", got)
	}
}

func TestNormalizeExplicitTransactionIdempotencyKey(t *testing.T) {
	normalized, err := NormalizeExplicitIdempotencyKey(
		"POST",
		"/api/v1/transactions?lang=en",
		"  withdrawal-test-1  ",
	)
	if err != nil {
		t.Fatalf("NormalizeExplicitIdempotencyKey: %v", err)
	}
	if normalized != "withdrawal-test-1" {
		t.Fatalf("normalized key = %q, want withdrawal-test-1", normalized)
	}
}

func TestRejectsInvalidExplicitTransactionIdempotencyKeys(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: "   "},
		{name: "non ASCII", value: "withdrawal-☃"},
		{name: "too long", value: strings.Repeat("a", 129)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NormalizeExplicitIdempotencyKey(
				"POST",
				"/api/v1/transactions",
				test.value,
			); err == nil {
				t.Fatalf("NormalizeExplicitIdempotencyKey(%q) succeeded, want error", test.value)
			}
		})
	}
}

func TestAccountCreationRequiresNormalizedStableKey(t *testing.T) {
	const target = "/api/v1/ledger/accounts?lang=en"
	if !RequiresIdempotency("POST", target) {
		t.Fatal("account creation requires an idempotency key")
	}
	key, err := NormalizeExplicitIdempotencyKey("post", target, "  account-create-1  ")
	if err != nil || key != "account-create-1" {
		t.Fatalf("normalized key = %q, error = %v", key, err)
	}
	if _, err := NormalizeExplicitIdempotencyKey("POST", target, ""); err == nil {
		t.Fatal("account creation accepted a missing key")
	}
	key, err = NormalizeExplicitIdempotencyKey("POST", target, strings.Repeat("a", 128))
	if err != nil || len(key) != 128 {
		t.Fatalf("128-byte key was rejected: %v", err)
	}
	if _, err := NormalizeExplicitIdempotencyKey("GET", target, ""); err != nil {
		t.Fatalf("read-only account request requires a key: %v", err)
	}
}
