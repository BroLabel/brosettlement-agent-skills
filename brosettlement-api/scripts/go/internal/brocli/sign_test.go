package brocli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignPrintsDeterministicMPCHeadersWithoutSending(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "private.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type: "PRIVATE KEY", Bytes: keyBytes,
	}), 0o600); err != nil {
		t.Fatal(err)
	}

	const keyID = "11111111-2222-4333-8444-555555555555"
	const timestamp = "1750000000"
	const nonce = "nonce-for-test-0001"
	const bodyHash = "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
	t.Setenv("BROSETTLEMENT_API_KEY_ID", keyID)
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", keyPath)

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"sign", "POST", "/api/v1/mpc/initialize",
		"--timestamp", timestamp,
		"--nonce", nonce,
		"--idempotency-key", "mpc-init-test-1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}

	var output signOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if output.CanonicalRequestTarget != "/api/v1/mpc/initialize" || output.Body != "{}" {
		t.Fatalf("unexpected output: %#v", output)
	}
	if output.Headers["X-Api-Body-Hash"] != bodyHash {
		t.Fatalf("body hash = %q, want %q", output.Headers["X-Api-Body-Hash"], bodyHash)
	}
	if output.Headers["Content-Type"] != "application/json" {
		t.Fatalf("content type = %q", output.Headers["Content-Type"])
	}
	if output.Headers["X-Idempotency-Key"] != "mpc-init-test-1" {
		t.Fatalf("idempotency key = %q", output.Headers["X-Idempotency-Key"])
	}

	canonical := strings.Join([]string{
		"POST", "/api/v1/mpc/initialize", bodyHash, timestamp, nonce, keyID,
	}, "\n")
	signature, err := base64.StdEncoding.DecodeString(output.Headers["X-Api-Signature"])
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(publicKey, []byte(canonical), signature) {
		t.Fatal("signature does not match the canonical request")
	}
}

func TestSignValidatesDeterministicInputsBeforeCredentials(t *testing.T) {
	t.Setenv("BROSETTLEMENT_API_KEY_ID", "")
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", "")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "timestamp",
			args: []string{"sign", "GET", "/api/v1/wallets", "--timestamp", "0123"},
			want: "timestamp must be unsigned Unix seconds",
		},
		{
			name: "nonce",
			args: []string{"sign", "GET", "/api/v1/wallets", "--nonce", "too-short"},
			want: "nonce must contain 16-128",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(test.args, &stdout, &stderr); code != 1 {
				t.Fatalf("Run returned %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.want)
			}
		})
	}
}

func TestSignRequiresStableWalletIdempotencyKeyBeforeCredentials(t *testing.T) {
	t.Setenv("BROSETTLEMENT_API_KEY_ID", "")
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", "")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sign", "POST", "/api/v1/wallets"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires an explicit stable --idempotency-key") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "BROSETTLEMENT_API_KEY_ID") {
		t.Fatalf("credentials were accessed before idempotency validation: %s", stderr.String())
	}
}
