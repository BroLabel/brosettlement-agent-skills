package brocli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/broauth"
)

const (
	withdrawCreateTarget = "/api/v1/transactions"
	withdrawHelp         = `Usage: brosettlement withdraw --wallet-id ID --asset ASSET --to ADDRESS --amount-atomic AMOUNT --idempotency-key KEY --confirm [options]

Required:
  --wallet-id ID           Source BroSettlement wallet ID
  --asset ASSET            Asset symbol, for example USDT
  --to ADDRESS             Destination blockchain address
  --amount-atomic AMOUNT   Positive base-10 amount in the asset's smallest unit
  --idempotency-key KEY    Stable key for this exact logical withdrawal
  --confirm                Confirm this state-changing request

Optional:
  --client-reference REF   Unique client reference (1-128 characters)
  --fee-limit-sun AMOUNT   Positive base-10 TRON fee limit in sun
  --base-url URL           BroSettlement API base URL
  --timeout DURATION       Shared HTTP client timeout (default 30s)

The command sends exactly one create request. After HTTP 201 with a transaction ID,
it performs exactly one immediate read-back. It does not probe, poll, retry, or open
a WebSocket. Automatic HTTP transport replay of the mutation is disabled.`
)

type withdrawOptions struct {
	baseURL         string
	walletID        string
	asset           string
	toAddress       string
	amountAtomic    string
	idempotencyKey  string
	clientReference string
	feeLimitSun     string
	timeout         time.Duration
	confirmed       bool
}

type withdrawRequest struct {
	ClientReference string                      `json:"clientReference,omitempty"`
	WalletID        string                      `json:"walletId"`
	Asset           string                      `json:"asset"`
	ToAddress       string                      `json:"toAddress"`
	AmountAtomic    string                      `json:"amountAtomic"`
	ChainParams     *withdrawRequestChainParams `json:"chainParams,omitempty"`
}

type withdrawRequestChainParams struct {
	Tron withdrawRequestTronParams `json:"tron"`
}

type withdrawRequestTronParams struct {
	FeeLimitSun string `json:"feeLimitSun"`
}

type withdrawDurations struct {
	Create       int64 `json:"create"`
	Verification int64 `json:"verification"`
	Total        int64 `json:"total"`
}

type withdrawError struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type withdrawOutput struct {
	Accepted             bool              `json:"accepted"`
	OutcomeUnknown       bool              `json:"outcomeUnknown"`
	VerificationPending  bool              `json:"verificationPending"`
	State                string            `json:"state"`
	TransactionID        string            `json:"transactionId,omitempty"`
	TransactionStatus    string            `json:"transactionStatus,omitempty"`
	CreateResponse       *apiOutput        `json:"createResponse,omitempty"`
	VerificationResponse *apiOutput        `json:"verificationResponse,omitempty"`
	Error                *withdrawError    `json:"error,omitempty"`
	DurationMS           withdrawDurations `json:"durationMs"`
}

type withdrawOutcomeUnknownError struct {
	err error
}

func (err *withdrawOutcomeUnknownError) Error() string {
	return err.err.Error()
}

func (err *withdrawOutcomeUnknownError) Unwrap() error {
	return err.err
}

func runWithdraw(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, withdrawHelp)
		return errHelp
	}

	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	options := withdrawOptions{
		baseURL: environment.apiBaseURL,
		timeout: 30 * time.Second,
	}
	flags := flag.NewFlagSet("withdraw", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&options.walletID, "wallet-id", "", "Source BroSettlement wallet ID")
	flags.StringVar(&options.asset, "asset", "", "Asset symbol")
	flags.StringVar(&options.toAddress, "to", "", "Destination blockchain address")
	flags.StringVar(&options.amountAtomic, "amount-atomic", "", "Positive base-10 amount in atomic units")
	flags.StringVar(&options.idempotencyKey, "idempotency-key", "", "Stable logical-operation key")
	flags.StringVar(&options.clientReference, "client-reference", "", "Unique client reference")
	flags.StringVar(&options.feeLimitSun, "fee-limit-sun", "", "Positive base-10 TRON fee limit in sun")
	flags.DurationVar(&options.timeout, "timeout", 30*time.Second, "Shared HTTP client timeout")
	flags.BoolVar(&options.confirmed, "confirm", false, "Confirm this withdrawal")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected withdraw arguments: %s", strings.Join(flags.Args(), " "))
	}

	body, err := validateAndMarshalWithdraw(&options)
	if err != nil {
		return err
	}
	return executeWithdraw(options, body, stdout)
}

func validateAndMarshalWithdraw(options *withdrawOptions) ([]byte, error) {
	if !options.confirmed {
		return nil, fmt.Errorf("withdrawal may change state; review the request and pass --confirm")
	}
	required := []struct {
		name  string
		value string
	}{
		{"--wallet-id", options.walletID},
		{"--asset", options.asset},
		{"--to", options.toAddress},
		{"--amount-atomic", options.amountAtomic},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			return nil, fmt.Errorf("%s is required", item.name)
		}
	}
	if err := validatePositiveDecimal("--amount-atomic", options.amountAtomic); err != nil {
		return nil, err
	}
	if options.feeLimitSun != "" {
		if err := validatePositiveDecimal("--fee-limit-sun", options.feeLimitSun); err != nil {
			return nil, err
		}
	}
	if options.clientReference != "" && (!utf8.ValidString(options.clientReference) || utf8.RuneCountInString(options.clientReference) > 128) {
		return nil, fmt.Errorf("--client-reference must be 1-128 valid UTF-8 characters")
	}
	if options.timeout <= 0 {
		return nil, fmt.Errorf("--timeout must be greater than zero")
	}
	if err := validateAPIBaseURL(options.baseURL); err != nil {
		return nil, err
	}
	normalizedIdempotencyKey, err := broauth.NormalizeExplicitIdempotencyKey(
		http.MethodPost,
		withdrawCreateTarget,
		options.idempotencyKey,
	)
	if err != nil {
		return nil, fmt.Errorf("POST %s %w", withdrawCreateTarget, err)
	}
	options.idempotencyKey = normalizedIdempotencyKey

	payload := withdrawRequest{
		ClientReference: options.clientReference,
		WalletID:        options.walletID,
		Asset:           options.asset,
		ToAddress:       options.toAddress,
		AmountAtomic:    options.amountAtomic,
	}
	if options.feeLimitSun != "" {
		payload.ChainParams = &withdrawRequestChainParams{
			Tron: withdrawRequestTronParams{FeeLimitSun: options.feeLimitSun},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode withdrawal body: %w", err)
	}
	return body, nil
}

func validatePositiveDecimal(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return fmt.Errorf("%s must be a positive base-10 integer", name)
		}
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok || parsed.Sign() <= 0 {
		return fmt.Errorf("%s must be a positive base-10 integer", name)
	}
	return nil
}

func validateAPIBaseURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("--base-url must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("--base-url must not contain credentials, a path, query, or fragment")
	}
	origin := strings.TrimSuffix(parsed.String(), "/")
	if origin != productionAPIBaseURL && origin != stagingAPIBaseURL {
		return fmt.Errorf("--base-url must be the BroSettlement production or staging API origin")
	}
	return nil
}

func executeWithdraw(options withdrawOptions, body []byte, stdout io.Writer) error {
	totalStart := time.Now()
	result := withdrawOutput{State: "creating"}
	client := newHTTPClient(options.timeout)
	// Redirects would turn one logical call into additional HTTP requests.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	createStart := time.Now()
	createResponse, err := sendWithdrawAPIRequest(
		client,
		options.baseURL,
		http.MethodPost,
		withdrawCreateTarget,
		body,
		options.idempotencyKey,
		true,
	)
	result.DurationMS.Create = time.Since(createStart).Milliseconds()
	if err != nil {
		result.Error = &withdrawError{Stage: "create", Message: err.Error()}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		var outcomeUnknown *withdrawOutcomeUnknownError
		if errors.As(err, &outcomeUnknown) {
			result.OutcomeUnknown = true
			result.VerificationPending = true
			result.State = "create_outcome_unknown"
			return writeWithdrawOutput(stdout, result)
		}
		result.State = "create_error"
		if outputErr := writeWithdrawOutput(stdout, result); outputErr != nil {
			return outputErr
		}
		return fmt.Errorf("create withdrawal: %w", err)
	}
	result.CreateResponse = &createResponse
	if createResponse.StatusCode != http.StatusCreated {
		if createResponse.StatusCode >= http.StatusOK && createResponse.StatusCode < http.StatusMultipleChoices {
			result.Accepted = true
			result.VerificationPending = true
			result.State = "accepted_unexpected_create_status"
			result.TransactionID = transactionStringField(createResponse.Body, "id")
			if status := transactionStringField(createResponse.Body, "status"); isKnownTransactionStatus(status) {
				result.TransactionStatus = status
			}
			result.Error = &withdrawError{
				Stage:   "verification",
				Message: fmt.Sprintf("create was accepted with unexpected HTTP %d; read-back was not attempted", createResponse.StatusCode),
			}
			result.DurationMS.Total = time.Since(totalStart).Milliseconds()
			return writeWithdrawOutput(stdout, result)
		}
		if createResponse.StatusCode >= http.StatusInternalServerError {
			result.OutcomeUnknown = true
			result.VerificationPending = true
			result.State = "create_outcome_unknown"
			result.Error = &withdrawError{
				Stage:   "create",
				Message: fmt.Sprintf("API returned HTTP %d; the create outcome is unknown and the request must not be repeated blindly", createResponse.StatusCode),
			}
			result.DurationMS.Total = time.Since(totalStart).Milliseconds()
			return writeWithdrawOutput(stdout, result)
		}
		result.State = "create_failed"
		result.Error = &withdrawError{
			Stage:   "create",
			Message: fmt.Sprintf("API returned HTTP %d", createResponse.StatusCode),
		}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		if outputErr := writeWithdrawOutput(stdout, result); outputErr != nil {
			return outputErr
		}
		return fmt.Errorf("create withdrawal: API returned HTTP %d", createResponse.StatusCode)
	}

	result.Accepted = true
	transactionID := transactionStringField(createResponse.Body, "id")
	result.TransactionID = transactionID
	if status := transactionStringField(createResponse.Body, "status"); isKnownTransactionStatus(status) {
		result.TransactionStatus = status
	}
	if strings.TrimSpace(transactionID) == "" {
		result.VerificationPending = true
		result.State = "accepted_missing_transaction_id"
		result.Error = &withdrawError{
			Stage:   "verification",
			Message: "accepted create response did not contain a non-empty transaction id",
		}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		return writeWithdrawOutput(stdout, result)
	}

	verificationTarget := "/api/v1/transactions/" + url.PathEscape(transactionID)
	verificationStart := time.Now()
	verificationResponse, err := sendWithdrawAPIRequest(
		client,
		options.baseURL,
		http.MethodGet,
		verificationTarget,
		nil,
		"",
		false,
	)
	result.DurationMS.Verification = time.Since(verificationStart).Milliseconds()
	if err != nil {
		result.VerificationPending = true
		result.State = "verification_error"
		result.Error = &withdrawError{Stage: "verification", Message: err.Error()}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		return writeWithdrawOutput(stdout, result)
	}

	result.VerificationResponse = &verificationResponse
	if verificationResponse.StatusCode != http.StatusOK {
		result.VerificationPending = true
		result.State = "verification_pending"
		result.Error = &withdrawError{
			Stage:   "verification",
			Message: fmt.Sprintf("API returned HTTP %d", verificationResponse.StatusCode),
		}
	} else {
		status, verificationErr := validateWithdrawVerificationBody(
			verificationResponse.Body,
			transactionID,
		)
		if verificationErr != nil {
			result.VerificationPending = true
			result.State = "verification_error"
			result.Error = &withdrawError{Stage: "verification", Message: verificationErr.Error()}
		} else {
			result.TransactionStatus = status
			switch status {
			case "PENDING":
				result.VerificationPending = true
				result.State = "verification_pending"
			case "CONFIRMED":
				result.State = "confirmed"
			case "FAILED":
				result.State = "failed"
			case "FAILED_ON_CHAIN":
				result.State = "failed_on_chain"
			}
		}
	}
	result.DurationMS.Total = time.Since(totalStart).Milliseconds()
	return writeWithdrawOutput(stdout, result)
}

func sendWithdrawAPIRequest(
	client *http.Client,
	baseURL string,
	method string,
	target string,
	body []byte,
	idempotencyKey string,
	mutation bool,
) (apiOutput, error) {
	headers, _, err := broauth.RESTHeaders(method, target, body)
	if err != nil {
		return apiOutput{}, fmt.Errorf("sign request: %w", err)
	}
	if len(body) > 0 {
		headers.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		headers.Set("X-Idempotency-Key", idempotencyKey)
	}

	requestURL := strings.TrimRight(baseURL, "/") + target
	request, err := http.NewRequest(method, requestURL, bytes.NewReader(body))
	if err != nil {
		return apiOutput{}, fmt.Errorf("create request: %w", err)
	}
	if mutation {
		// A non-nil body with no GetBody prevents net/http from replaying this
		// state-changing request after a reused-connection failure.
		request.GetBody = nil
		if request.Body == nil || request.Body == http.NoBody {
			request.Body = io.NopCloser(bytes.NewReader(nil))
		}
	}
	request.Header = headers

	response, err := client.Do(request)
	if err != nil {
		return apiOutput{}, &withdrawOutcomeUnknownError{
			err: fmt.Errorf("send request: %w", err),
		}
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return apiOutput{}, &withdrawOutcomeUnknownError{
			err: fmt.Errorf("read response: %w", err),
		}
	}

	var parsedBody interface{}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &parsedBody); err != nil {
			parsedBody = string(responseBody)
		}
	}
	return apiOutput{
		StatusCode: response.StatusCode,
		RequestID:  response.Header.Get("X-Request-Id"),
		Body:       parsedBody,
	}, nil
}

func transactionStringField(body interface{}, key string) string {
	object, ok := body.(map[string]interface{})
	if !ok {
		return ""
	}
	value, ok := object[key].(string)
	if !ok {
		return ""
	}
	return value
}

func validateWithdrawVerificationBody(body interface{}, transactionID string) (string, error) {
	verifiedID := transactionStringField(body, "id")
	if strings.TrimSpace(verifiedID) == "" {
		return "", fmt.Errorf("verification response did not contain a non-empty transaction id")
	}
	if verifiedID != transactionID {
		return "", fmt.Errorf("verification response transaction id does not match the accepted transaction")
	}
	status := transactionStringField(body, "status")
	switch status {
	case "PENDING", "CONFIRMED", "FAILED", "FAILED_ON_CHAIN":
		return status, nil
	case "":
		return "", fmt.Errorf("verification response did not contain a transaction status")
	default:
		return "", fmt.Errorf("verification response returned unknown transaction status %q", status)
	}
}

func isKnownTransactionStatus(status string) bool {
	switch status {
	case "PENDING", "CONFIRMED", "FAILED", "FAILED_ON_CHAIN":
		return true
	default:
		return false
	}
}

func writeWithdrawOutput(stdout io.Writer, result withdrawOutput) error {
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode withdrawal output: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, string(encoded)); err != nil {
		return fmt.Errorf("write withdrawal output: %w", err)
	}
	return nil
}
