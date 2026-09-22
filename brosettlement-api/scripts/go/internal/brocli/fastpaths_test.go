package brocli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRootHelpListsFastPaths(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	for _, expected := range []string{
		"brosettlement account create|show",
		"brosettlement wallet create|show|resolve",
		"brosettlement asset show",
		"brosettlement transaction status|wait",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("root help does not contain %q:\n%s", expected, stdout.String())
		}
	}
}

func TestWalletCreateRequiresExplicitStableKeyBeforeCredentials(t *testing.T) {
	t.Setenv("BROSETTLEMENT_API_KEY_ID", "")
	t.Setenv("BROSETTLEMENT_API_PRIVATE_KEY_FILE", "")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"wallet", "create", "--account-id", "account-1", "--chain", "tron:nile", "--confirm",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires an explicit stable --idempotency-key") {
		t.Fatalf("missing stable key error: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "BROSETTLEMENT_API_KEY_ID") {
		t.Fatalf("credentials were accessed before validation: %s", stderr.String())
	}
}

func TestWalletCreateSendsOnePOSTThenVerifiesExactWallet(t *testing.T) {
	configureTestCredentials(t)
	expectedBody := `{"chain":"tron:nile","accountId":"account-1"}`
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(expectedBody)))
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if request.Method != http.MethodPost || request.URL.RequestURI() != "/api/v1/wallets" {
				t.Fatalf("unexpected create request: %s %s", request.Method, request.URL)
			}
			if string(body) != expectedBody || request.Header.Get("X-Api-Body-Hash") != expectedHash {
				t.Fatalf("wallet create body/hash mismatch: %s", body)
			}
			if request.Header.Get("X-Idempotency-Key") != "wallet-create-1" || request.GetBody != nil {
				t.Fatal("wallet create lost its stable key or enabled transport replay")
			}
			return jsonHTTPResponse(http.StatusCreated, `{"id":"wallet/1","chain":"tron:nile","accountId":"account-1","address":"TNile","status":"ACTIVE"}`), nil
		case 2:
			if request.Method != http.MethodGet || request.URL.RequestURI() != "/api/v1/wallets/wallet%2F1" {
				t.Fatalf("unexpected verification request: %s %s", request.Method, request.URL.RequestURI())
			}
			if request.Header.Get("X-Idempotency-Key") != "" {
				t.Fatal("wallet read-back reused the mutation idempotency key")
			}
			return jsonHTTPResponse(http.StatusOK, `{"id":"wallet/1","chain":"tron:nile","accountId":"account-1","address":"TNile","status":"ACTIVE"}`), nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, nil
		}
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"wallet", "create",
		"--account-id", "account-1",
		"--chain", "tron:nile",
		"--idempotency-key", "wallet-create-1",
		"--confirm",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if requests != 2 {
		t.Fatalf("wallet create sent %d requests, want one POST and one GET", requests)
	}
	var output walletCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if !output.Accepted || output.OutcomeUnknown || output.VerificationPending || output.State != "active" || output.WalletID != "wallet/1" {
		t.Fatalf("unexpected wallet result: %#v", output)
	}
}

func TestWalletCreateDoesNotPollUndocumentedStatus(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Method == http.MethodPost {
			return jsonHTTPResponse(http.StatusCreated, `{"id":"wallet-1"}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"wallet-1","chain":"tron:nile","accountId":"account-1","address":"TNile","status":"PROVISIONING"}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"wallet", "create", "--account-id", "account-1", "--chain", "tron:nile",
		"--idempotency-key", "wallet-create-1", "--confirm",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output walletCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 2 || output.VerificationAttempts != 1 || !output.VerificationPending || output.State != "verification_pending" {
		t.Fatalf("wallet create polled or misreported an undocumented status: requests=%d output=%#v", requests, output)
	}
	if output.Error == nil || !strings.Contains(output.Error.Message, "no polling was attempted") {
		t.Fatalf("missing no-poll explanation: %#v", output.Error)
	}
}

func TestWalletCreateReportsDocumentedTerminalNonActiveStates(t *testing.T) {
	for _, testCase := range []struct {
		status string
		state  string
	}{
		{status: "DISABLED", state: "disabled"},
		{status: "ARCHIVED", state: "archived"},
	} {
		t.Run(testCase.status, func(t *testing.T) {
			configureTestCredentials(t)
			requests := 0
			useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
				requests++
				if request.Method == http.MethodPost {
					return jsonHTTPResponse(http.StatusCreated, `{"id":"wallet-1"}`), nil
				}
				body := fmt.Sprintf(`{"id":"wallet-1","chain":"tron:nile","accountId":"account-1","address":"TNile","status":%q}`, testCase.status)
				return jsonHTTPResponse(http.StatusOK, body), nil
			})

			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"wallet", "create", "--account-id", "account-1", "--chain", "tron:nile",
				"--idempotency-key", "wallet-create-1", "--confirm",
			}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run returned %d: %s", code, stderr.String())
			}
			var output walletCreateOutput
			decodeOneJSON(t, stdout.Bytes(), &output)
			if requests != 2 || output.State != testCase.state || output.VerificationPending || output.Error == nil {
				t.Fatalf("unexpected terminal wallet result: requests=%d output=%#v", requests, output)
			}
		})
	}
}

func TestWalletCreateRejectsMismatchedReadBack(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Method == http.MethodPost {
			return jsonHTTPResponse(http.StatusCreated, `{"id":"wallet-1"}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"wallet-1","chain":"tron:nile","accountId":"account-2","address":"TNile","status":"ACTIVE"}`), nil
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"wallet", "create", "--account-id", "account-1", "--chain", "tron:nile",
		"--idempotency-key", "wallet-create-1", "--confirm",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output walletCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 2 || output.State != "verification_error" || !output.VerificationPending || output.Error == nil {
		t.Fatalf("unexpected mismatched wallet result: requests=%d output=%#v", requests, output)
	}
}

func TestWalletCreateUnknownOutcomeNeverRepeatsPOST(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return jsonHTTPResponse(http.StatusServiceUnavailable, `{"code":"TEMPORARY_UNAVAILABLE"}`), nil
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"wallet", "create", "--account-id", "account-1", "--chain", "tron:nile",
		"--idempotency-key", "wallet-create-1", "--confirm",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d for unknown outcome: %s", code, stderr.String())
	}
	var output walletCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 1 || !output.OutcomeUnknown || !output.VerificationPending || output.State != "create_outcome_unknown" {
		t.Fatalf("unexpected unknown wallet outcome: requests=%d output=%#v", requests, output)
	}
}

func TestAccountCreatePreservesExplicitEmptyMetadataAndReadsBack(t *testing.T) {
	configureTestCredentials(t)
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	if err := os.WriteFile(metadataPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != `{"name":"Treasury","externalId":"treasury-1","metadata":{}}` {
				t.Fatalf("unexpected account body: %s", body)
			}
			if request.Header.Get("X-Idempotency-Key") != "" || request.GetBody != nil {
				t.Fatal("account create sent an unsupported idempotency key or enabled replay")
			}
			return jsonHTTPResponse(http.StatusCreated, `{"id":"account/1","name":"Treasury","externalId":"treasury-1","metadata":{}}`), nil
		}
		if request.URL.RequestURI() != "/api/v1/ledger/accounts/account%2F1" {
			t.Fatalf("unexpected account read-back target: %s", request.URL.RequestURI())
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"account/1","name":"Treasury","externalId":"treasury-1","metadata":{}}`), nil
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"account", "create", "--name", "Treasury", "--external-id", "treasury-1",
		"--metadata-file", metadataPath, "--confirm",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output accountCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if !output.Accepted || !output.Created || output.Reconciled || output.State != "created" || output.AccountID != "account/1" {
		t.Fatalf("unexpected account result: %#v", output)
	}
}

func TestAccountCreateReconcilesDocumentedExternalIDConflict(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return jsonHTTPResponse(http.StatusConflict, `{"code":"ACCOUNT_EXTERNAL_ID_CONFLICT"}`), nil
		case 2:
			values, err := url.ParseQuery(request.URL.RawQuery)
			if err != nil || values.Get("externalId") != "treasury-1" || values.Get("limit") != "2" {
				t.Fatalf("unexpected reconciliation query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"items":[{"id":"account-1","name":"Treasury","externalId":"treasury-1","metadata":null}],"nextCursor":null}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusOK, `{"id":"account-1","name":"Treasury","externalId":"treasury-1","metadata":null}`), nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, nil
		}
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"account", "create", "--name", "Treasury", "--external-id", "treasury-1", "--confirm"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output accountCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if output.Accepted || output.Created || !output.Reconciled || output.State != "reconciled_existing" || output.AccountID != "account-1" {
		t.Fatalf("unexpected reconciled account result: %#v", output)
	}
}

func TestAccountShowReadsExactEscapedID(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Method != http.MethodGet || request.URL.RequestURI() != "/api/v1/ledger/accounts/account%2F1" {
			t.Fatalf("unexpected account show request: %s %s", request.Method, request.URL.RequestURI())
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"account/1","name":"Treasury","externalId":"treasury-1"}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"account", "show", "--account-id", "account/1"}, &stdout, &stderr)
	if code != 0 || requests != 1 {
		t.Fatalf("account show failed: code=%d requests=%d stderr=%s", code, requests, stderr.String())
	}
	var output apiOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if output.StatusCode != http.StatusOK || transactionStringField(output.Body, "id") != "account/1" {
		t.Fatalf("unexpected account show output: %#v", output)
	}
}

func TestAccountShowRejectsMismatchedID(t *testing.T) {
	configureTestCredentials(t)
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusOK, `{"id":"account-2","name":"Other"}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"account", "show", "--account-id", "account-1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "response account id does not match") {
		t.Fatalf("missing account mismatch error: %s", stderr.String())
	}
}

func TestAccountCreateUnknownOutcomeNeverRepeatsPOST(t *testing.T) {
	configureTestCredentials(t)
	posts, gets := 0, 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			posts++
			return nil, errors.New("connection reset after write")
		}
		gets++
		return jsonHTTPResponse(http.StatusOK, `{"items":[],"nextCursor":null}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"account", "create", "--name", "Treasury", "--external-id", "treasury-1", "--confirm"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d for unknown outcome: %s", code, stderr.String())
	}
	var output accountCreateOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if posts != 1 || gets != 1 || !output.OutcomeUnknown || output.State != "create_outcome_unknown" {
		t.Fatalf("unexpected unknown account outcome: posts=%d gets=%d output=%#v", posts, gets, output)
	}
}

func TestWalletResolvePaginatesBeforeExactSelection(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonHTTPResponse(http.StatusOK, `{"items":[{"id":"partial","address":"TExactSuffix","chain":"tron:nile"}],"nextCursor":"next page"}`), nil
		}
		if request.URL.Query().Get("cursor") != "next page" {
			t.Fatalf("missing cursor on page two: %s", request.URL.RawQuery)
		}
		return jsonHTTPResponse(http.StatusOK, `{"items":[{"id":"wallet-1","address":"TExact","chain":"tron:nile"}],"nextCursor":null}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"wallet", "resolve", "--address", "TExact", "--chain", "tron:nile"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output walletResolveOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 2 || objectString(output.Wallet, "id") != "wallet-1" {
		t.Fatalf("unexpected resolved wallet: requests=%d output=%#v", requests, output)
	}
}

func TestWalletResolveRejectsExactMatchWithoutID(t *testing.T) {
	configureTestCredentials(t)
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusOK, `{"items":[{"address":"TExact","chain":"tron:nile"}],"nextCursor":null}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"wallet", "resolve", "--address", "TExact", "--chain", "tron:nile"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "without a wallet id") {
		t.Fatalf("wallet resolve accepted a match without id: code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestWalletShowFetchesIndependentViewsInOneInvocation(t *testing.T) {
	configureTestCredentials(t)
	var mutex sync.Mutex
	paths := map[string]int{}
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		mutex.Lock()
		paths[request.URL.Path]++
		mutex.Unlock()
		switch {
		case strings.HasSuffix(request.URL.Path, "/balances"):
			return jsonHTTPResponse(http.StatusOK, `{"items":[],"nextCursor":null}`), nil
		case strings.HasSuffix(request.URL.Path, "/ledger-entries"):
			return jsonHTTPResponse(http.StatusOK, `{"items":[],"nextCursor":null}`), nil
		default:
			return jsonHTTPResponse(http.StatusOK, `{"id":"wallet-1","accountId":"account-1","chain":"tron:nile","address":"TWallet","status":"ACTIVE"}`), nil
		}
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"wallet", "show", "--wallet-id", "wallet-1", "--asset", "USDT", "--entries", "10"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output walletShowOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if output.Partial || output.Wallet == nil || output.Balances == nil || output.LedgerEntries == nil || len(paths) != 3 {
		t.Fatalf("unexpected wallet show output: paths=%v output=%#v", paths, output)
	}
}

func TestWalletShowRejectsMismatchedWalletBody(t *testing.T) {
	configureTestCredentials(t)
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/balances"), strings.HasSuffix(request.URL.Path, "/ledger-entries"):
			return jsonHTTPResponse(http.StatusOK, `{"items":[],"nextCursor":null}`), nil
		default:
			return jsonHTTPResponse(http.StatusOK, `{"id":"another-wallet","accountId":"account-1","chain":"tron:nile","address":"TWallet","status":"ACTIVE"}`), nil
		}
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"wallet", "show", "--wallet-id", "wallet-1"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("wallet show accepted a mismatched wallet body: %s", stdout.String())
	}
	var output walletShowOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if !output.Partial || output.State != "partial" || !strings.Contains(output.Errors["wallet"], "does not match") {
		t.Fatalf("unexpected wallet mismatch output: %#v", output)
	}
}

func TestAssetShowPaginatesAndReturnsArchivedDefinition(t *testing.T) {
	configureTestCredentials(t)
	requests := 0
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonHTTPResponse(http.StatusOK, `{"items":[],"nextCursor":"cursor-2"}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"items":[{"chain":"tron:nile","asset":"USDT","status":"ARCHIVED","decimals":6,"depositsEnabled":false,"withdrawalsEnabled":false}],"nextCursor":null}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"asset", "show", "--chain", "tron:nile", "--asset", "USDT"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output assetShowOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 2 || objectString(output.Definition, "status") != "ARCHIVED" {
		t.Fatalf("unexpected asset output: requests=%d output=%#v", requests, output)
	}
}

func TestTransactionWaitStopsAtTerminalStatus(t *testing.T) {
	configureTestCredentials(t)
	previousSleep := fastPathSleep
	fastPathSleep = func(time.Duration) {}
	t.Cleanup(func() { fastPathSleep = previousSleep })
	requests := 0
	var nonces []string
	useRoundTripper(t, func(request *http.Request) (*http.Response, error) {
		requests++
		nonces = append(nonces, request.Header.Get("X-Api-Nonce"))
		if request.URL.RequestURI() != "/api/v1/transactions/tx%2F1" {
			t.Fatalf("transaction id was not escaped: %s", request.URL.RequestURI())
		}
		if requests == 1 {
			return jsonHTTPResponse(http.StatusOK, `{"id":"tx/1","status":"PENDING"}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"tx/1","status":"CONFIRMED"}`), nil
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"transaction", "wait", "--id", "tx/1", "--timeout", "30s", "--poll-interval", "1s",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	var output transactionReadOutput
	decodeOneJSON(t, stdout.Bytes(), &output)
	if requests != 2 || !output.Terminal || output.VerificationPending || output.Status != "CONFIRMED" || output.State != "confirmed" {
		t.Fatalf("unexpected transaction wait output: %#v", output)
	}
	if nonces[0] == "" || nonces[0] == nonces[1] {
		t.Fatalf("poll requests did not use fresh nonces: %v", nonces)
	}
}

func decodeOneJSON(t *testing.T, data []byte, target interface{}) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, data)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("output contains more than one JSON value: %v\n%s", err, data)
	}
}
