package brocli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAccountCommandsRejectInvalidKeysBeforeCredentials(t *testing.T) {
	t.Setenv("BROSETTLEMENT_API_KEY_ID", "")
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", "")
	commands := [][]string{
		{"account", "create", "--name", "Treasury", "--external-id", "treasury-1", "--confirm"},
		{"api", "POST", "/api/v1/ledger/accounts?lang=en", "--confirm"},
		{"sign", "POST", "/api/v1/ledger/accounts"},
	}
	keys := []struct {
		name, value, want string
	}{
		{"missing", "", "requires an explicit stable --idempotency-key"},
		{"blank", "   ", "requires an explicit stable --idempotency-key"},
		{"non ASCII", "account-☃", "printable ASCII"},
		{"control", "account\x00key", "printable ASCII"},
		{"too long", strings.Repeat("a", 129), "at most 128"},
	}
	for _, command := range commands {
		for _, key := range keys {
			t.Run(strings.Join(command[:2], " ")+"/"+key.name, func(t *testing.T) {
				args := append([]string(nil), command...)
				if key.name != "missing" {
					args = append(args, "--idempotency-key", key.value)
				}
				var stdout, stderr bytes.Buffer
				if code := Run(args, &stdout, &stderr); code != 1 {
					t.Fatalf("Run returned %d, want validation failure", code)
				}
				if !strings.Contains(stderr.String(), key.want) || strings.Contains(stderr.String(), "BROSETTLEMENT_API_KEY_ID") {
					t.Fatalf("expected key validation before credentials, got %s", stderr.String())
				}
			})
		}
	}
}

func TestAccountCreateExplicitRetryKeepsKeyAndBodyWithFreshNonce(t *testing.T) {
	configureTestCredentials(t)
	posts, gets := 0, 0
	firstNonce := ""
	const currentAccount = `{"id":"account-1","name":"Renamed","externalId":"updated-id","metadata":{"updated":true}}`
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			posts++
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if request.URL.Path != "/api/v1/ledger/accounts" || string(body) != `{"name":"Treasury","externalId":"treasury-1"}` {
				t.Fatalf("unexpected account request: %s %s", request.URL, body)
			}
			if request.Header.Get("X-Idempotency-Key") != "account-create-1" || request.GetBody != nil {
				t.Fatal("account create lost its stable key or enabled transport replay")
			}
			nonce := request.Header.Get("X-Api-Nonce")
			if nonce == "" || nonce == firstNonce {
				t.Fatal("each attempt must have a fresh signing nonce")
			}
			if posts == 1 {
				firstNonce = nonce
				return nil, errors.New("connection reset after write")
			}
			if posts != 2 {
				t.Fatalf("unexpected automatic POST retry %d", posts)
			}
			return jsonHTTPResponse(http.StatusCreated, currentAccount), nil
		}
		gets++
		if request.Header.Get("X-Idempotency-Key") != "" {
			t.Fatal("read-back must not carry a mutation key")
		}
		if gets == 1 {
			if request.URL.Query().Get("externalId") != "treasury-1" {
				t.Fatal("unknown outcome must reconcile the original external ID")
			}
			return jsonHTTPResponse(http.StatusOK, `{"items":[],"nextCursor":null}`), nil
		}
		if gets != 2 || request.URL.Path != "/api/v1/ledger/accounts/account-1" {
			t.Fatalf("unexpected read-back: %s", request.URL)
		}
		return jsonHTTPResponse(http.StatusOK, currentAccount), nil
	})

	args := []string{"account", "create", "--name", "Treasury", "--external-id", "treasury-1", "--idempotency-key", "  account-create-1  ", "--confirm"}
	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("first Run returned %d: %s", code, stderr.String())
	}
	var first accountCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &first)
	if posts != 1 || gets != 1 || !first.OutcomeUnknown {
		t.Fatalf("unknown outcome triggered a retry: posts=%d gets=%d output=%#v", posts, gets, first)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("explicit retry returned %d: %s", code, stderr.String())
	}
	var replay accountCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &replay)
	if posts != 2 || gets != 2 || replay.State != "created" || replay.VerificationPending || replay.AccountID != "account-1" {
		t.Fatalf("replay did not verify current account data: %#v", replay)
	}
	if transactionStringField(replay.CreateResponse.Body, "name") != "Renamed" {
		t.Fatal("replay lost the current account data")
	}
}

func TestAccountCreateIdempotencyConflictStopsWithoutReconciliation(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Method != http.MethodPost || requests != 1 {
			t.Fatal("idempotency conflict must not trigger reconciliation or mutation retry")
		}
		return jsonHTTPResponse(http.StatusConflict, `{"code":"IDEMPOTENCY_CONFLICT"}`), nil
	})
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"account", "create", "--name", "Treasury", "--external-id", "treasury-1", "--idempotency-key", "account-create-1", "--confirm"}, &stdout, &stderr); code != 1 {
		t.Fatalf("Run returned %d, want conflict failure: %s", code, stderr.String())
	}
	var output accountCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 1 || output.Accepted || output.Reconciled || output.OutcomeUnknown || output.State != "create_failed" {
		t.Fatalf("unexpected conflict outcome: %#v", output)
	}
}

func TestAccountCreateRejectsReadBackOfDifferentAccount(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonHTTPResponse(http.StatusCreated, `{"id":"account-1","name":"Treasury","externalId":"treasury-1"}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"account-2","name":"Treasury","externalId":"treasury-1"}`), nil
	})
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"account", "create", "--name", "Treasury", "--external-id", "treasury-1", "--idempotency-key", "account-create-1", "--confirm"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output accountCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 2 || !output.Accepted || !output.VerificationPending || output.State != "verification_pending" {
		t.Fatalf("mismatched read-back was accepted: %#v", output)
	}
}
