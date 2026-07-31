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

func TestRESTHeadersSignsMPCInitializeWithoutBodyHash(t *testing.T) {
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

	headers, _, err := RESTHeaders("POST", "/api/v1/mpc/initialize", nil)
	if err != nil {
		t.Fatalf("RESTHeaders: %v", err)
	}

	if got := headers.Get("X-Api-Body-Hash"); got != "" {
		t.Fatalf("unexpected X-Api-Body-Hash: %q", got)
	}

	canonical := strings.Join([]string{
		"POST",
		"/api/v1/mpc/initialize",
		"",
		headers.Get("X-Api-Timestamp"),
		headers.Get("X-Api-Nonce"),
		keyID,
	}, "\n")
	signature, err := base64.StdEncoding.DecodeString(headers.Get("X-Api-Signature"))
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(publicKey, []byte(canonical), signature) {
		t.Fatal("signature does not preserve the empty canonical body-hash line")
	}
	if !RequiresExplicitEmptyFormBody("POST", "/api/v1/mpc/initialize") {
		t.Fatal("MPC initialize must use an explicit zero-length form body")
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
