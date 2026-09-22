package brocli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/broauth"
)

const walletHelp = `Usage:
  brosettlement wallet create --account-id ID --chain CHAIN --idempotency-key KEY --confirm
  brosettlement wallet show --wallet-id ID [--asset ASSET] [--entries N]
  brosettlement wallet resolve --address ADDRESS [--chain CHAIN]

create sends exactly one idempotent POST and performs one immediate read-back.
It never retries the POST or polls an undocumented wallet lifecycle state.
show fetches wallet details, balances, and recent ledger entries in parallel.
resolve requires one exact address match; the API's partial match is never trusted alone.`

type walletCreateOptions struct {
	baseURL        string
	accountID      string
	chain          string
	idempotencyKey string
	timeout        time.Duration
	confirmed      bool
}

type walletCreateRequest struct {
	Chain     string `json:"chain"`
	AccountID string `json:"accountId"`
}

type fastPathError struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type resourceDurations struct {
	Create       int64 `json:"create,omitempty"`
	Verification int64 `json:"verification,omitempty"`
	Total        int64 `json:"total"`
}

type walletCreateOutput struct {
	Accepted             bool              `json:"accepted"`
	OutcomeUnknown       bool              `json:"outcomeUnknown"`
	VerificationPending  bool              `json:"verificationPending"`
	State                string            `json:"state"`
	WalletID             string            `json:"walletId,omitempty"`
	WalletStatus         string            `json:"walletStatus,omitempty"`
	VerificationAttempts int               `json:"verificationAttempts,omitempty"`
	CreateResponse       *apiOutput        `json:"createResponse,omitempty"`
	VerificationResponse *apiOutput        `json:"verificationResponse,omitempty"`
	Error                *fastPathError    `json:"error,omitempty"`
	DurationMS           resourceDurations `json:"durationMs"`
}

type walletShowOutput struct {
	State         string            `json:"state"`
	WalletID      string            `json:"walletId"`
	Partial       bool              `json:"partial"`
	RequestCount  int               `json:"requestCount"`
	Wallet        *apiOutput        `json:"wallet,omitempty"`
	Balances      *apiOutput        `json:"balances,omitempty"`
	LedgerEntries *apiOutput        `json:"ledgerEntries,omitempty"`
	Errors        map[string]string `json:"errors,omitempty"`
	DurationMS    int64             `json:"durationMs"`
}

type walletResolveOutput struct {
	State      string                 `json:"state"`
	Address    string                 `json:"address"`
	Chain      string                 `json:"chain,omitempty"`
	Wallet     map[string]interface{} `json:"wallet"`
	Pages      int                    `json:"pages"`
	StatusCode int                    `json:"statusCode"`
	RequestID  string                 `json:"requestId,omitempty"`
	DurationMS int64                  `json:"durationMs"`
}

func runWallet(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, walletHelp)
		return errHelp
	}
	switch strings.ToLower(args[0]) {
	case "create":
		return runWalletCreate(args[1:], stdout, stderr)
	case "show":
		return runWalletShow(args[1:], stdout, stderr)
	case "resolve":
		return runWalletResolve(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown wallet command %q", args[0])
	}
}

func runWalletCreate(args []string, stdout, stderr io.Writer) error {
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	options := walletCreateOptions{
		baseURL: environment.apiBaseURL,
		timeout: 30 * time.Second,
	}
	flags := flag.NewFlagSet("wallet create", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&options.accountID, "account-id", "", "Ledger account ID")
	flags.StringVar(&options.chain, "chain", "", "Blockchain identifier: tron:mainnet or tron:nile")
	flags.StringVar(&options.idempotencyKey, "idempotency-key", "", "Stable key for this logical wallet creation")
	flags.DurationVar(&options.timeout, "timeout", 30*time.Second, "Per-request HTTP timeout")
	flags.BoolVar(&options.confirmed, "confirm", false, "Confirm wallet creation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected wallet create arguments: %s", strings.Join(flags.Args(), " "))
	}
	body, err := validateAndMarshalWalletCreate(&options)
	if err != nil {
		return err
	}
	return executeWalletCreate(options, body, stdout)
}

func validateAndMarshalWalletCreate(options *walletCreateOptions) ([]byte, error) {
	if !options.confirmed {
		return nil, fmt.Errorf("wallet creation may change state; review the request and pass --confirm")
	}
	options.accountID = strings.TrimSpace(options.accountID)
	if options.accountID == "" {
		return nil, fmt.Errorf("--account-id is required")
	}
	options.chain = strings.TrimSpace(options.chain)
	if options.chain != "tron:mainnet" && options.chain != "tron:nile" {
		return nil, fmt.Errorf("--chain must be tron:mainnet or tron:nile")
	}
	if options.timeout <= 0 {
		return nil, fmt.Errorf("--timeout must be greater than zero")
	}
	if err := validateAPIBaseURL(options.baseURL); err != nil {
		return nil, err
	}
	normalized, err := broauth.NormalizeStableIdempotencyKey(options.idempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("POST /api/v1/wallets %w", err)
	}
	options.idempotencyKey = normalized
	encoded, err := json.Marshal(walletCreateRequest{Chain: options.chain, AccountID: options.accountID})
	if err != nil {
		return nil, fmt.Errorf("encode wallet body: %w", err)
	}
	return encoded, nil
}

func executeWalletCreate(options walletCreateOptions, body []byte, stdout io.Writer) error {
	totalStart := time.Now()
	result := walletCreateOutput{State: "creating"}
	client := newFastPathHTTPClient(options.timeout)

	createStart := time.Now()
	createResponse, err := sendSignedAPIRequest(
		client, options.baseURL, http.MethodPost, "/api/v1/wallets", body,
		options.idempotencyKey, true,
	)
	result.DurationMS.Create = time.Since(createStart).Milliseconds()
	if err != nil {
		result.Error = &fastPathError{Stage: "create", Message: err.Error()}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		var outcomeUnknown *requestOutcomeUnknownError
		if errors.As(err, &outcomeUnknown) {
			result.OutcomeUnknown = true
			result.VerificationPending = true
			result.State = "create_outcome_unknown"
			return writeJSON(stdout, result, "wallet create")
		}
		result.State = "create_error"
		if outputErr := writeJSON(stdout, result, "wallet create"); outputErr != nil {
			return outputErr
		}
		return fmt.Errorf("create wallet: %w", err)
	}
	result.CreateResponse = &createResponse
	if createResponse.StatusCode != http.StatusCreated {
		if createResponse.StatusCode >= http.StatusInternalServerError {
			result.OutcomeUnknown = true
			result.VerificationPending = true
			result.State = "create_outcome_unknown"
			result.Error = &fastPathError{Stage: "create", Message: fmt.Sprintf("API returned HTTP %d; do not repeat the POST blindly", createResponse.StatusCode)}
			result.DurationMS.Total = time.Since(totalStart).Milliseconds()
			return writeJSON(stdout, result, "wallet create")
		}
		if createResponse.StatusCode >= http.StatusOK && createResponse.StatusCode < http.StatusMultipleChoices {
			result.Accepted = true
			result.VerificationPending = true
			result.State = "accepted_unexpected_create_status"
			result.WalletID = transactionStringField(createResponse.Body, "id")
			result.Error = &fastPathError{Stage: "verification", Message: fmt.Sprintf("create was accepted with unexpected HTTP %d; read-back was not attempted", createResponse.StatusCode)}
			result.DurationMS.Total = time.Since(totalStart).Milliseconds()
			return writeJSON(stdout, result, "wallet create")
		}
		result.State = "create_failed"
		result.Error = &fastPathError{Stage: "create", Message: fmt.Sprintf("API returned HTTP %d", createResponse.StatusCode)}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		if outputErr := writeJSON(stdout, result, "wallet create"); outputErr != nil {
			return outputErr
		}
		return fmt.Errorf("create wallet: API returned HTTP %d", createResponse.StatusCode)
	}

	result.Accepted = true
	result.WalletID = transactionStringField(createResponse.Body, "id")
	if result.WalletID == "" {
		result.VerificationPending = true
		result.State = "accepted_missing_wallet_id"
		result.Error = &fastPathError{Stage: "verification", Message: "accepted create response did not contain a non-empty wallet id"}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		return writeJSON(stdout, result, "wallet create")
	}

	verificationStart := time.Now()
	result.VerificationAttempts = 1
	verificationTarget := "/api/v1/wallets/" + url.PathEscape(result.WalletID)
	verificationResponse, verificationErr := sendSignedAPIRequest(
		client, options.baseURL, http.MethodGet, verificationTarget, nil, "", false,
	)
	if verificationErr != nil {
		result.VerificationPending = true
		result.State = "verification_error"
		result.Error = &fastPathError{Stage: "verification", Message: verificationErr.Error()}
	} else {
		result.VerificationResponse = &verificationResponse
		switch {
		case verificationResponse.StatusCode != http.StatusOK:
			result.VerificationPending = true
			result.State = "verification_pending"
			result.Error = &fastPathError{Stage: "verification", Message: fmt.Sprintf("API returned HTTP %d", verificationResponse.StatusCode)}
		case transactionStringField(verificationResponse.Body, "id") != result.WalletID:
			result.VerificationPending = true
			result.State = "verification_error"
			result.Error = &fastPathError{Stage: "verification", Message: "read-back wallet id does not match the accepted wallet"}
		default:
			verifiedObject, ok := verificationResponse.Body.(map[string]interface{})
			if !ok || objectString(verifiedObject, "accountId") != options.accountID ||
				objectString(verifiedObject, "chain") != options.chain ||
				strings.TrimSpace(objectString(verifiedObject, "address")) == "" {
				result.VerificationPending = true
				result.State = "verification_error"
				result.Error = &fastPathError{Stage: "verification", Message: "read-back wallet does not match the requested account and chain or has no address"}
				break
			}
			result.WalletStatus = transactionStringField(verificationResponse.Body, "status")
			switch result.WalletStatus {
			case "ACTIVE":
				result.State = "active"
				result.VerificationPending = false
			case "DISABLED":
				result.State = "disabled"
				result.VerificationPending = false
				result.Error = &fastPathError{Stage: "verification", Message: "wallet is terminal but not active"}
			case "ARCHIVED":
				result.State = "archived"
				result.VerificationPending = false
				result.Error = &fastPathError{Stage: "verification", Message: "wallet is terminal but not active"}
			default:
				result.State = "verification_pending"
				result.VerificationPending = true
				result.Error = &fastPathError{Stage: "verification", Message: fmt.Sprintf("wallet returned undocumented status %q; no polling was attempted", result.WalletStatus)}
			}
		}
	}
	result.DurationMS.Verification = time.Since(verificationStart).Milliseconds()
	result.DurationMS.Total = time.Since(totalStart).Milliseconds()
	return writeJSON(stdout, result, "wallet create")
}

func runWalletShow(args []string, stdout, stderr io.Writer) error {
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	baseURL := environment.apiBaseURL
	walletID := ""
	asset := ""
	entries := 20
	timeout := 30 * time.Second
	flags := flag.NewFlagSet("wallet show", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&walletID, "wallet-id", "", "BroSettlement wallet ID")
	flags.StringVar(&asset, "asset", "", "Optional exact asset filter")
	flags.IntVar(&entries, "entries", 20, "Number of recent ledger entries to return (0-100)")
	flags.DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected wallet show arguments: %s", strings.Join(flags.Args(), " "))
	}
	walletID = strings.TrimSpace(walletID)
	if walletID == "" {
		return fmt.Errorf("--wallet-id is required")
	}
	if entries < 0 || entries > 100 {
		return fmt.Errorf("--entries must be between 0 and 100")
	}
	if timeout <= 0 {
		return fmt.Errorf("--timeout must be greater than zero")
	}
	if err := validateAPIBaseURL(baseURL); err != nil {
		return err
	}

	start := time.Now()
	client := newFastPathHTTPClient(timeout)
	escapedID := url.PathEscape(walletID)
	type stage struct {
		name     string
		target   string
		response *apiOutput
		err      error
	}
	stages := []*stage{
		{name: "wallet", target: "/api/v1/wallets/" + escapedID},
	}
	balanceQuery := url.Values{"limit": {"100"}}
	if strings.TrimSpace(asset) != "" {
		balanceQuery.Set("asset", strings.TrimSpace(asset))
	}
	stages = append(stages, &stage{name: "balances", target: "/api/v1/wallets/" + escapedID + "/balances?" + balanceQuery.Encode()})
	if entries > 0 {
		entryQuery := url.Values{"limit": {fmt.Sprintf("%d", entries)}}
		if strings.TrimSpace(asset) != "" {
			entryQuery.Set("asset", strings.TrimSpace(asset))
		}
		stages = append(stages, &stage{name: "ledgerEntries", target: "/api/v1/wallets/" + escapedID + "/ledger-entries?" + entryQuery.Encode()})
	}

	var waitGroup sync.WaitGroup
	for _, current := range stages {
		waitGroup.Add(1)
		go func(current *stage) {
			defer waitGroup.Done()
			response, requestErr := sendSignedAPIRequest(client, baseURL, http.MethodGet, current.target, nil, "", false)
			current.response = &response
			if requestErr != nil {
				current.err = requestErr
				return
			}
			if response.StatusCode != http.StatusOK {
				current.err = fmt.Errorf("API returned HTTP %d", response.StatusCode)
				return
			}
			switch current.name {
			case "wallet":
				object, ok := response.Body.(map[string]interface{})
				if !ok {
					current.err = fmt.Errorf("wallet response body is not an object")
					return
				}
				if objectString(object, "id") != walletID {
					current.err = fmt.Errorf("wallet response id does not match the requested wallet")
					return
				}
				for _, requiredField := range []string{"accountId", "chain", "address", "status"} {
					if strings.TrimSpace(objectString(object, requiredField)) == "" {
						current.err = fmt.Errorf("wallet response is missing %s", requiredField)
						return
					}
				}
			case "balances", "ledgerEntries":
				if _, parseErr := responseItems(response.Body); parseErr != nil {
					current.err = parseErr
				}
			}
		}(current)
	}
	waitGroup.Wait()

	result := walletShowOutput{State: "complete", WalletID: walletID, RequestCount: len(stages), Errors: map[string]string{}, DurationMS: time.Since(start).Milliseconds()}
	for _, current := range stages {
		switch current.name {
		case "wallet":
			result.Wallet = current.response
		case "balances":
			result.Balances = current.response
		case "ledgerEntries":
			result.LedgerEntries = current.response
		}
		if current.err != nil {
			result.Errors[current.name] = current.err.Error()
		}
	}
	if len(result.Errors) == 0 {
		result.Errors = nil
	} else {
		result.Partial = true
		result.State = "partial"
	}
	if err := writeJSON(stdout, result, "wallet show"); err != nil {
		return err
	}
	if result.Partial {
		return fmt.Errorf("wallet show completed with partial errors")
	}
	return nil
}

func runWalletResolve(args []string, stdout, stderr io.Writer) error {
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	baseURL := environment.apiBaseURL
	address := ""
	chain := ""
	timeout := 30 * time.Second
	flags := flag.NewFlagSet("wallet resolve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&address, "address", "", "Exact blockchain address")
	flags.StringVar(&chain, "chain", "", "Optional exact blockchain identifier")
	flags.DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected wallet resolve arguments: %s", strings.Join(flags.Args(), " "))
	}
	address = strings.TrimSpace(address)
	chain = strings.TrimSpace(chain)
	if address == "" {
		return fmt.Errorf("--address is required")
	}
	if timeout <= 0 {
		return fmt.Errorf("--timeout must be greater than zero")
	}
	if err := validateAPIBaseURL(baseURL); err != nil {
		return err
	}
	query := url.Values{"addressContains": {address}, "limit": {"100"}}
	if chain != "" {
		query.Set("chain", chain)
	}
	client := newFastPathHTTPClient(timeout)
	start := time.Now()
	matches := make([]map[string]interface{}, 0, 1)
	var lastResponse apiOutput
	pages := 0
	for page := 1; ; page++ {
		if page > 20 {
			return fmt.Errorf("wallet resolve exceeded 20 cursor pages; narrow the chain filter")
		}
		target := "/api/v1/wallets?" + query.Encode()
		response, requestErr := sendSignedAPIRequest(client, baseURL, http.MethodGet, target, nil, "", false)
		if requestErr != nil {
			return requestErr
		}
		lastResponse = response
		pages = page
		if response.StatusCode != http.StatusOK {
			if outputErr := writeJSON(stdout, response, "wallet resolve"); outputErr != nil {
				return outputErr
			}
			return fmt.Errorf("wallet resolve: API returned HTTP %d", response.StatusCode)
		}
		items, parseErr := responseItems(response.Body)
		if parseErr != nil {
			return fmt.Errorf("wallet resolve: %w", parseErr)
		}
		for _, item := range items {
			if objectString(item, "address") != address {
				continue
			}
			if chain != "" && objectString(item, "chain") != chain {
				continue
			}
			if strings.TrimSpace(objectString(item, "id")) == "" {
				return fmt.Errorf("wallet resolve received an exact match without a wallet id")
			}
			matches = append(matches, item)
		}
		responseObject, _ := response.Body.(map[string]interface{})
		nextCursor, _ := responseObject["nextCursor"].(string)
		if nextCursor == "" {
			break
		}
		query.Set("cursor", nextCursor)
	}
	if len(matches) != 1 {
		return fmt.Errorf("wallet resolve found %d exact matches; expected exactly one", len(matches))
	}
	return writeJSON(stdout, walletResolveOutput{
		State: "resolved", Address: address, Chain: chain, Wallet: matches[0], Pages: pages,
		StatusCode: lastResponse.StatusCode, RequestID: lastResponse.RequestID,
		DurationMS: time.Since(start).Milliseconds(),
	}, "wallet resolve")
}
