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
	if !strings.Contains(stdout.String(), "production by default") ||
		!strings.Contains(stdout.String(), "BROSETTLEMENT_ENVIRONMENT=production|staging") {
		t.Fatalf("help does not describe environment selection: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "automatic HTTP transport replay is disabled") {
		t.Fatalf("help does not describe single-attempt mutation behavior: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "stop after 30s by default") ||
		!strings.Contains(stdout.String(), "--follow is explicitly unbounded") {
		t.Fatalf("help does not describe bounded WebSocket behavior: %s", stdout.String())
	}
}

func TestSelectedEnvironmentDefaultsToProduction(t *testing.T) {
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "")
	environment, err := selectedEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if environment.name != "production" ||
		environment.apiBaseURL != "https://brosettlement-api.brolabel.io" ||
		environment.swaggerJSON != "https://brosettlement-api.brolabel.io/swagger-integration-json" ||
		environment.webSocketURL != "wss://brosettlement-api.brolabel.io/v1/ws" {
		t.Fatalf("unexpected production endpoints: %#v", environment)
	}
}

func TestSelectedEnvironmentSupportsExplicitStaging(t *testing.T) {
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "staging")
	environment, err := selectedEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if environment.name != "staging" ||
		environment.apiBaseURL != "https://brosettlement-staging-api.brolabel.io" ||
		environment.swaggerJSON != "https://brosettlement-staging-api.brolabel.io/swagger-integration-json" ||
		environment.webSocketURL != "wss://brosettlement-staging-api.brolabel.io/v1/ws" {
		t.Fatalf("unexpected staging endpoints: %#v", environment)
	}
}

func TestSelectedEnvironmentRejectsUnknownValue(t *testing.T) {
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "sandbox")
	if _, err := selectedEnvironment(); err == nil {
		t.Fatal("expected unsupported environment error")
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

func TestTransactionMutationRequiresExplicitIdempotencyBeforeCredentials(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"api", "POST", "/api/v1/transactions",
		"--confirm",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires an explicit stable --idempotency-key") {
		t.Fatalf("missing stable idempotency error: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "BROSETTLEMENT_API_KEY_ID") {
		t.Fatalf("credentials were accessed before idempotency validation: %s", stderr.String())
	}
}

func TestTransactionMutationUsesExplicitIdempotencyKeyOnce(t *testing.T) {
	configureTestCredentials(t)
	bodyPath := filepath.Join(t.TempDir(), "transaction.json")
	if err := os.WriteFile(bodyPath, []byte(`{"walletId":"wallet-test","amountAtomic":"15000000"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	var idempotencyKey string
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requestCount++
		idempotencyKey = request.Header.Get("X-Idempotency-Key")
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"code":"INSUFFICIENT_SCOPE"}`)),
		}, nil
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"api", "POST", "/api/v1/transactions",
		"--body-file", bodyPath,
		"--idempotency-key", "withdrawal-test-1",
		"--confirm",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1 for HTTP 403", code)
	}
	if requestCount != 1 {
		t.Fatalf("transaction mutation sent %d requests, want exactly 1", requestCount)
	}
	if idempotencyKey != "withdrawal-test-1" {
		t.Fatalf("transaction idempotency key = %q, want withdrawal-test-1", idempotencyKey)
	}
	if !strings.Contains(stdout.String(), `"code": "INSUFFICIENT_SCOPE"`) {
		t.Fatalf("structured API error missing from output: %s", stdout.String())
	}
}

func TestWalletMutationUsesProductionAndDoesNotRetry(t *testing.T) {
	configureTestCredentials(t)
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "")
	bodyPath := filepath.Join(t.TempDir(), "wallet.json")
	if err := os.WriteFile(bodyPath, []byte(`{"accountId":"account-test","network":"TRON_NILE"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	var requestURL, idempotencyKey, requestBody string
	var requestBodyPresent, transportReplayDisabled bool
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requestCount++
		requestURL = request.URL.String()
		idempotencyKey = request.Header.Get("X-Idempotency-Key")
		requestBodyPresent = request.Body != nil && request.Body != http.NoBody
		transportReplayDisabled = request.GetBody == nil
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		requestBody = string(body)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"code":"TEMPORARY_UNAVAILABLE"}`)),
		}, nil
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"api", "POST", "/api/v1/wallets",
		"--body-file", bodyPath,
		"--idempotency-key", "wallet-create-test-1",
		"--confirm",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1 for HTTP 503", code)
	}
	if requestCount != 1 {
		t.Fatalf("wallet mutation sent %d requests, want exactly 1", requestCount)
	}
	if requestURL != "https://brosettlement-api.brolabel.io/api/v1/wallets" {
		t.Fatalf("wallet mutation used %q, want production API", requestURL)
	}
	if requestBody != `{"accountId":"account-test","network":"TRON_NILE"}` {
		t.Fatalf("wallet mutation changed request body: %q", requestBody)
	}
	if idempotencyKey != "wallet-create-test-1" {
		t.Fatalf("wallet mutation idempotency key = %q, want wallet-create-test-1", idempotencyKey)
	}
	if !requestBodyPresent {
		t.Fatal("wallet mutation request body is absent")
	}
	if !transportReplayDisabled {
		t.Fatal("wallet mutation allows automatic HTTP transport replay: GetBody is non-nil")
	}
}

func TestGenericWalletMutationRequiresExplicitStableKey(t *testing.T) {
	t.Setenv("BROSETTLEMENT_API_KEY_ID", "")
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", "")
	bodyPath := filepath.Join(t.TempDir(), "wallet.json")
	if err := os.WriteFile(bodyPath, []byte(`{"accountId":"account-test","chain":"tron:nile"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"api", "POST", "/api/v1/wallets",
		"--body-file", bodyPath,
		"--confirm",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires an explicit stable --idempotency-key") {
		t.Fatalf("missing stable key error: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "BROSETTLEMENT_API_KEY_ID") {
		t.Fatalf("credentials were accessed before idempotency validation: %s", stderr.String())
	}
}

func TestWebSocketOptionsDefaultToBoundedThirtySeconds(t *testing.T) {
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "")
	environment, err := selectedEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	options, err := parseWebSocketOptions(nil, environment, &stderr)
	if err != nil {
		t.Fatalf("parseWebSocketOptions returned error: %v", err)
	}
	if options.follow {
		t.Fatal("default WebSocket options unexpectedly enable unbounded follow mode")
	}
	if options.stopAfter != 30*time.Second {
		t.Fatalf("default WebSocket stop-after is %s, want 30s", options.stopAfter)
	}
}

func TestWebSocketFollowIsExplicitAndMutuallyExclusiveWithStopAfter(t *testing.T) {
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "")
	environment, err := selectedEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	options, err := parseWebSocketOptions([]string{"--follow"}, environment, &stderr)
	if err != nil {
		t.Fatalf("parseWebSocketOptions returned error: %v", err)
	}
	if !options.follow {
		t.Fatal("--follow did not enable unbounded listener mode")
	}

	_, err = parseWebSocketOptions([]string{"--follow", "--stop-after", "45s"}, environment, &stderr)
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("combined --follow and --stop-after error = %v", err)
	}
}

func TestWebSocketRejectsNonPositiveStopAfter(t *testing.T) {
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "")
	environment, err := selectedEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	_, err = parseWebSocketOptions([]string{"--stop-after", "0s"}, environment, &stderr)
	if err == nil || !strings.Contains(err.Error(), "must be greater than zero") {
		t.Fatalf("zero --stop-after error = %v", err)
	}
}

func TestAPIRequestUsesExplicitStagingEnvironment(t *testing.T) {
	configureTestCredentials(t)
	t.Setenv("BROSETTLEMENT_ENVIRONMENT", "staging")
	requestCount := 0
	var requestURL string
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requestCount++
		requestURL = request.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"items":[]}`)),
		}, nil
	})

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"api", "GET", "/api/v1/wallets"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if requestCount != 1 {
		t.Fatalf("API read sent %d requests, want exactly 1", requestCount)
	}
	if requestURL != "https://brosettlement-staging-api.brolabel.io/api/v1/wallets" {
		t.Fatalf("API read used %q, want staging API", requestURL)
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
