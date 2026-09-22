package brocli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWithdrawHelpDescribesOneShotCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"withdraw", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	for _, expected := range []string{
		"--wallet-id ID",
		"--asset ASSET",
		"--to ADDRESS",
		"--amount-atomic AMOUNT",
		"--idempotency-key KEY",
		"--client-reference REF",
		"--fee-limit-sun AMOUNT",
		"--confirm",
		"exactly one create request",
		"does not probe, poll, retry, or open",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("withdraw help does not contain %q:\n%s", expected, stdout.String())
		}
	}
}

func TestWithdrawValidatesBeforeCredentialsOrHTTP(t *testing.T) {
	t.Setenv("BROSETTLEMENT_API_KEY_ID", "")
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", "")

	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name: "confirmation",
			args: []string{
				"withdraw", "--wallet-id", "wallet-1", "--asset", "USDT",
				"--to", "destination", "--amount-atomic", "1",
				"--idempotency-key", "withdraw-1",
			},
			expected: "pass --confirm",
		},
		{
			name: "amount",
			args: []string{
				"withdraw", "--wallet-id", "wallet-1", "--asset", "USDT",
				"--to", "destination", "--amount-atomic", "1.5",
				"--idempotency-key", "withdraw-1", "--confirm",
			},
			expected: "--amount-atomic must be a positive base-10 integer",
		},
		{
			name: "idempotency",
			args: []string{
				"withdraw", "--wallet-id", "wallet-1", "--asset", "USDT",
				"--to", "destination", "--amount-atomic", "1", "--confirm",
			},
			expected: "requires an explicit stable --idempotency-key",
		},
		{
			name: "base URL",
			args: []string{
				"withdraw", "--wallet-id", "wallet-1", "--asset", "USDT",
				"--to", "destination", "--amount-atomic", "1",
				"--idempotency-key", "withdraw-1", "--base-url", "not-a-url", "--confirm",
			},
			expected: "--base-url must be an absolute HTTP(S) URL",
		},
		{
			name: "untrusted API origin",
			args: []string{
				"withdraw", "--wallet-id", "wallet-1", "--asset", "USDT",
				"--to", "destination", "--amount-atomic", "1",
				"--idempotency-key", "withdraw-1", "--base-url", "https://example.test", "--confirm",
			},
			expected: "--base-url must be the BroSettlement production or staging API origin",
		},
	}

	previous := newHTTPClient
	httpClientCalls := 0
	newHTTPClient = func(timeout time.Duration) *http.Client {
		httpClientCalls++
		return &http.Client{Timeout: timeout}
	}
	t.Cleanup(func() { newHTTPClient = previous })

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(test.args, &stdout, &stderr); code != 1 {
				t.Fatalf("Run returned %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), test.expected) {
				t.Fatalf("error does not contain %q: %s", test.expected, stderr.String())
			}
			if strings.Contains(stderr.String(), "BROSETTLEMENT_API_KEY_ID") ||
				strings.Contains(stderr.String(), "BROSETTLEMENT_API_PRIVATE_KEY_FILE") {
				t.Fatalf("credentials were accessed before validation: %s", stderr.String())
			}
		})
	}
	if httpClientCalls != 0 {
		t.Fatalf("validation created %d HTTP clients, want 0", httpClientCalls)
	}
}

func TestWithdrawSendsDeterministicCreateThenOneEscapedVerification(t *testing.T) {
	configureTestCredentials(t)

	const expectedBody = `{"clientReference":"payout-42","walletId":"wallet-1","asset":"USDT","toAddress":"TZ-destination","amountAtomic":"6000000","chainParams":{"tron":{"feeLimitSun":"30000000"}}}`
	expectedBodyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(expectedBody)))
	requests := 0
	clientFactoryCalls := 0
	var createNonce, createSignature, verificationNonce, verificationSignature string
	var requestErr string
	previous := newHTTPClient
	newHTTPClient = func(timeout time.Duration) *http.Client {
		clientFactoryCalls++
		return &http.Client{
			Timeout: timeout,
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					body, err := io.ReadAll(request.Body)
					if err != nil {
						requestErr = "read create body: " + err.Error()
					}
					checks := []struct {
						ok      bool
						message string
					}{
						{request.Method == http.MethodPost, "create method is not POST"},
						{request.URL.String() == productionAPIBaseURL+"/api/v1/transactions", "create URL changed"},
						{string(body) == expectedBody, "create body is not deterministic"},
						{request.Header.Get("X-Api-Body-Hash") == expectedBodyHash, "create body hash does not match the transmitted bytes"},
						{request.Header.Get("X-Idempotency-Key") == "withdrawal-42", "idempotency key changed"},
						{request.Header.Get("Content-Type") == "application/json", "create content type is not JSON"},
						{request.GetBody == nil, "mutation transport replay is enabled"},
					}
					createNonce = request.Header.Get("X-Api-Nonce")
					createSignature = request.Header.Get("X-Api-Signature")
					for _, check := range checks {
						if !check.ok && requestErr == "" {
							requestErr = check.message
						}
					}
					return jsonHTTPResponse(http.StatusCreated, `{"id":"tx/id?one","status":"PENDING"}`), nil
				case 2:
					checks := []struct {
						ok      bool
						message string
					}{
						{request.Method == http.MethodGet, "verification method is not GET"},
						{request.URL.RequestURI() == "/api/v1/transactions/tx%2Fid%3Fone", "transaction ID was not path-escaped"},
						{request.Header.Get("X-Idempotency-Key") == "", "verification reused mutation idempotency key"},
						{request.Header.Get("X-Api-Body-Hash") == "", "bodyless verification unexpectedly has a body hash"},
					}
					verificationNonce = request.Header.Get("X-Api-Nonce")
					verificationSignature = request.Header.Get("X-Api-Signature")
					for _, check := range checks {
						if !check.ok && requestErr == "" {
							requestErr = check.message
						}
					}
					return jsonHTTPResponse(http.StatusOK, `{"id":"tx/id?one","status":"PENDING"}`), nil
				default:
					t.Fatalf("unexpected request %d: %s %s", requests, request.Method, request.URL)
					return nil, nil
				}
			}),
		}
	}
	t.Cleanup(func() { newHTTPClient = previous })

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"withdraw",
		"--wallet-id", "wallet-1",
		"--asset", "USDT",
		"--to", "TZ-destination",
		"--amount-atomic", "6000000",
		"--idempotency-key", "withdrawal-42",
		"--client-reference", "payout-42",
		"--fee-limit-sun", "30000000",
		"--base-url", productionAPIBaseURL,
		"--timeout", "5s",
		"--confirm",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if requestErr != "" {
		t.Fatal(requestErr)
	}
	if requests != 2 {
		t.Fatalf("withdrawal sent %d requests, want one POST and one GET", requests)
	}
	if clientFactoryCalls != 1 {
		t.Fatalf("withdrawal created %d HTTP clients, want one shared client", clientFactoryCalls)
	}
	if createNonce == "" || createSignature == "" || verificationNonce == "" || verificationSignature == "" {
		t.Fatal("create or verification request is missing signed authentication headers")
	}
	if createNonce == verificationNonce || createSignature == verificationSignature {
		t.Fatal("verification request did not use a fresh nonce and signature")
	}

	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if !result.Accepted || !result.VerificationPending {
		t.Fatalf("unexpected accepted/pending result: %#v", result)
	}
	if result.State != "verification_pending" || result.TransactionID != "tx/id?one" {
		t.Fatalf("unexpected state or transaction ID: %#v", result)
	}
	if result.CreateResponse == nil || result.CreateResponse.StatusCode != http.StatusCreated {
		t.Fatalf("missing create response: %#v", result.CreateResponse)
	}
	if result.VerificationResponse == nil || result.VerificationResponse.StatusCode != http.StatusOK {
		t.Fatalf("missing verification response: %#v", result.VerificationResponse)
	}
	if strings.Contains(stdout.String(), "11111111-2222-4333-8444-555555555555") ||
		strings.Contains(stdout.String(), createSignature) || strings.Contains(stdout.String(), verificationSignature) {
		t.Fatalf("withdrawal output exposed authentication material: %s", stdout.String())
	}
}

func TestWithdrawDoesNotVerifyWhenCreateFails(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected request after failed create: %s %s", request.Method, request.URL)
		}
		return jsonHTTPResponse(http.StatusForbidden, `{"code":"INSUFFICIENT_SCOPE"}`), nil
	})

	var stdout, stderr bytes.Buffer
	code := Run(validWithdrawArgs(), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	if requests != 1 {
		t.Fatalf("failed create sent %d requests, want exactly one POST", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if result.Accepted || result.VerificationPending || result.State != "create_failed" {
		t.Fatalf("unexpected create failure result: %#v", result)
	}
	if result.CreateResponse == nil || result.CreateResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("missing create error response: %#v", result.CreateResponse)
	}
	if result.VerificationResponse != nil {
		t.Fatalf("failed create unexpectedly has verification response: %#v", result.VerificationResponse)
	}
}

func TestWithdrawRefusesRedirectWithoutSendingASecondRequest(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		response := jsonHTTPResponse(http.StatusTemporaryRedirect, "")
		response.Header.Set("Location", "https://example.test/relay")
		return response, nil
	})

	var stdout, stderr bytes.Buffer
	if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 1 {
		t.Fatalf("Run returned %d, want 1 for a refused redirect", code)
	}
	if requests != 1 {
		t.Fatalf("redirect caused %d requests, want exactly one POST", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if result.Accepted || result.OutcomeUnknown || result.State != "create_failed" {
		t.Fatalf("unexpected redirect result: %#v", result)
	}
}

func TestWithdrawTransportErrorLeavesCreateOutcomeUnknown(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("connection reset after write")
	})

	var stdout, stderr bytes.Buffer
	if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d for an uncertain create outcome: %s", code, stderr.String())
	}
	if requests != 1 {
		t.Fatalf("uncertain create sent %d requests, want exactly one POST", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if result.Accepted || !result.OutcomeUnknown || !result.VerificationPending || result.State != "create_outcome_unknown" {
		t.Fatalf("unexpected transport-error result: %#v", result)
	}
}

func TestWithdrawResponseReadErrorLeavesCreateOutcomeUnknown(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusCreated,
			Header:     make(http.Header),
			Body:       io.NopCloser(errorReader{}),
		}, nil
	})

	var stdout, stderr bytes.Buffer
	if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d for an unreadable create response: %s", code, stderr.String())
	}
	if requests != 1 {
		t.Fatalf("unreadable create response sent %d requests, want exactly one POST", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if result.Accepted || !result.OutcomeUnknown || !result.VerificationPending || result.State != "create_outcome_unknown" {
		t.Fatalf("unexpected read-error result: %#v", result)
	}
}

func TestWithdrawServerErrorLeavesCreateOutcomeUnknown(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return jsonHTTPResponse(http.StatusServiceUnavailable, `{"code":"SERVICE_UNAVAILABLE"}`), nil
	})

	var stdout, stderr bytes.Buffer
	if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d for an uncertain server response: %s", code, stderr.String())
	}
	if requests != 1 {
		t.Fatalf("server error sent %d requests, want exactly one POST", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if result.Accepted || !result.OutcomeUnknown || !result.VerificationPending || result.State != "create_outcome_unknown" {
		t.Fatalf("unexpected server-error result: %#v", result)
	}
	if result.CreateResponse == nil || result.CreateResponse.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("missing server error response: %#v", result.CreateResponse)
	}
}

func TestWithdrawAcceptedWithoutIDDoesNotVerifyOrFail(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return jsonHTTPResponse(http.StatusCreated, `{"status":"PENDING"}`), nil
	})

	var stdout, stderr bytes.Buffer
	if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if requests != 1 {
		t.Fatalf("accepted response without ID sent %d requests, want one POST and no GET", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if !result.Accepted || !result.VerificationPending || result.State != "accepted_missing_transaction_id" {
		t.Fatalf("unexpected missing-ID result: %#v", result)
	}
}

func TestWithdrawUnexpectedSuccessfulCreateDoesNotVerifyOrFail(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return jsonHTTPResponse(http.StatusAccepted, `{"id":"transaction-1","status":"PENDING"}`), nil
	})

	var stdout, stderr bytes.Buffer
	if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d after a 2xx create response: %s", code, stderr.String())
	}
	if requests != 1 {
		t.Fatalf("unexpected 2xx create sent %d requests, want one POST and no GET", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if !result.Accepted || !result.VerificationPending || result.State != "accepted_unexpected_create_status" {
		t.Fatalf("unexpected non-201 success result: %#v", result)
	}
}

func TestWithdrawGETForbiddenRemainsAcceptedAndReturnsSuccess(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonHTTPResponse(http.StatusCreated, `{"id":"transaction-1","status":"PENDING"}`), nil
		}
		return jsonHTTPResponse(http.StatusForbidden, `{"code":"INSUFFICIENT_SCOPE"}`), nil
	})

	var stdout, stderr bytes.Buffer
	if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d after accepted POST and forbidden GET: %s", code, stderr.String())
	}
	if requests != 2 {
		t.Fatalf("withdrawal sent %d requests, want one POST and one GET", requests)
	}
	result := decodeSingleWithdrawOutput(t, stdout.Bytes())
	if !result.Accepted || !result.VerificationPending || result.State != "verification_pending" {
		t.Fatalf("unexpected forbidden-verification result: %#v", result)
	}
	if result.VerificationResponse == nil || result.VerificationResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("missing forbidden verification response: %#v", result.VerificationResponse)
	}
}

func TestWithdrawMalformedOrMismatchedVerificationRemainsAccepted(t *testing.T) {
	tests := []struct {
		name             string
		verificationBody string
	}{
		{name: "mismatched ID", verificationBody: `{"id":"other-transaction","status":"PENDING"}`},
		{name: "missing status", verificationBody: `{"id":"transaction-1"}`},
		{name: "unknown status", verificationBody: `{"id":"transaction-1","status":"QUEUED"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configureTestCredentials(t)
			requests := 0
			useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return jsonHTTPResponse(http.StatusCreated, `{"id":"transaction-1","status":"PENDING"}`), nil
				}
				return jsonHTTPResponse(http.StatusOK, test.verificationBody), nil
			})

			var stdout, stderr bytes.Buffer
			if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
				t.Fatalf("Run returned %d after an accepted POST: %s", code, stderr.String())
			}
			if requests != 2 {
				t.Fatalf("withdrawal sent %d requests, want one POST and one GET", requests)
			}
			result := decodeSingleWithdrawOutput(t, stdout.Bytes())
			if !result.Accepted || !result.VerificationPending || result.State != "verification_error" {
				t.Fatalf("unexpected malformed-verification result: %#v", result)
			}
			if result.Error == nil || result.Error.Stage != "verification" {
				t.Fatalf("missing verification error: %#v", result.Error)
			}
		})
	}
}

func TestWithdrawReportsEveryDocumentedTransactionStatus(t *testing.T) {
	tests := []struct {
		status              string
		state               string
		verificationPending bool
	}{
		{status: "PENDING", state: "verification_pending", verificationPending: true},
		{status: "CONFIRMED", state: "confirmed"},
		{status: "FAILED", state: "failed"},
		{status: "FAILED_ON_CHAIN", state: "failed_on_chain"},
	}

	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			configureTestCredentials(t)
			requests := 0
			useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return jsonHTTPResponse(http.StatusCreated, `{"id":"transaction-1","status":"PENDING"}`), nil
				}
				return jsonHTTPResponse(
					http.StatusOK,
					fmt.Sprintf(`{"id":"transaction-1","status":%q}`, test.status),
				), nil
			})

			var stdout, stderr bytes.Buffer
			if code := Run(validWithdrawArgs(), &stdout, &stderr); code != 0 {
				t.Fatalf("Run returned %d: %s", code, stderr.String())
			}
			result := decodeSingleWithdrawOutput(t, stdout.Bytes())
			if result.TransactionStatus != test.status || result.State != test.state ||
				result.VerificationPending != test.verificationPending {
				t.Fatalf("unexpected result for %s: %#v", test.status, result)
			}
		})
	}
}

func validWithdrawArgs() []string {
	return []string{
		"withdraw",
		"--wallet-id", "wallet-1",
		"--asset", "USDT",
		"--to", "TZ-destination",
		"--amount-atomic", "6000000",
		"--idempotency-key", "withdrawal-42",
		"--base-url", productionAPIBaseURL,
		"--confirm",
	}
}

func jsonHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func decodeSingleWithdrawOutput(t *testing.T, data []byte) withdrawOutput {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var result withdrawOutput
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode withdrawal output: %v\n%s", err, data)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("withdrawal output contains more than one JSON value: %v\n%s", err, data)
	}
	return result
}

type errorReader struct{}

func (errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("response stream failed")
}
