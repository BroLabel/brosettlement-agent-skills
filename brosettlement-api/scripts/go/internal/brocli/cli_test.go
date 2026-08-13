package brocli

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "brosettlement mpc initialize --confirm") {
		t.Fatalf("help does not describe guarded MPC initialization: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "brosettlement update [--auto]") {
		t.Fatalf("help does not describe CLI updates: %s", stdout.String())
	}
}

func TestAPIMutationRequiresConfirmationBeforeCredentials(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"api", "POST", "/api/v1/wallets"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "pass --confirm") {
		t.Fatalf("missing confirmation error: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "BROSETTLEMENT_API_KEY_ID") {
		t.Fatalf("credentials were accessed before confirmation: %s", stderr.String())
	}
}

func TestMPCInitializeUsesCurrentStagingRequestShape(t *testing.T) {
	configureTestCredentials(t)
	var requestErr string
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			requestErr = err.Error()
		}
		checks := []struct {
			ok      bool
			message string
		}{
			{request.Method == http.MethodPost, "method is not POST"},
			{request.URL.RequestURI() == "/api/v1/mpc/initialize", "target changed"},
			{string(body) == "{}", "body is not the exact empty JSON object"},
			{request.Header.Get("Content-Type") == "application/json", "content type is not JSON"},
			{request.Header.Get("X-Api-Body-Hash") == "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a", "body hash does not match exact {} bytes"},
			{request.Header.Get("X-Idempotency-Key") == "init-test", "idempotency key changed"},
		}
		for _, check := range checks {
			if !check.ok && requestErr == "" {
				requestErr = check.message
			}
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"status":"PROVISIONING"}`)),
		}, nil
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"api", "POST", "/api/v1/mpc/initialize",
		"--base-url", "https://example.test",
		"--idempotency-key", "init-test",
		"--confirm",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if requestErr != "" {
		t.Fatal(requestErr)
	}
	if !strings.Contains(stdout.String(), `"statusCode": 201`) ||
		!strings.Contains(stdout.String(), `"status": "PROVISIONING"`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestCommandsFetchesAndFiltersCurrentContract(t *testing.T) {
	useRoundTripper(t, func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
			"info":{"title":"BroSettlement Integration API","version":"1.0"},
			"paths":{
				"/api/v1/wallets":{"get":{"summary":"List wallets","operationId":"Wallets_list","tags":["Wallets"]}},
				"/api/v1/assets":{"get":{"summary":"List assets","operationId":"Assets_list","tags":["Assets"]}}
			}
		}`)),
		}, nil
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"commands", "wallets", "--json", "--swagger-json", "https://example.test/openapi.json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "/api/v1/wallets") || strings.Contains(stdout.String(), "/api/v1/assets") {
		t.Fatalf("filter returned wrong commands: %s", stdout.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func useRoundTripper(t *testing.T, function roundTripFunc) {
	t.Helper()
	previous := newHTTPClient
	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{Transport: function, Timeout: timeout}
	}
	t.Cleanup(func() { newHTTPClient = previous })
}

func configureTestCredentials(t *testing.T) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "private.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("BROSETTLEMENT_API_KEY_ID", "11111111-2222-4333-8444-555555555555")
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", keyPath)
}
