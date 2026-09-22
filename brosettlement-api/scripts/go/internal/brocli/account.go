package brocli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

const accountHelp = `Usage:
  brosettlement account create --name NAME --external-id ID --confirm [--metadata-file FILE]
  brosettlement account show --account-id ID

create sends exactly one POST and never retries it. Because account creation has
no server idempotency contract, --external-id is mandatory and is used only to
reconcile a conflict or uncertain outcome through a safe exact lookup.`

type accountCreateOptions struct {
	baseURL      string
	name         string
	externalID   string
	metadataFile string
	timeout      time.Duration
	confirmed    bool
}

type accountCreateRequest struct {
	Name       string          `json:"name"`
	ExternalID string          `json:"externalId"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

type accountCreateOutput struct {
	Accepted               bool             `json:"accepted"`
	Created                bool             `json:"created"`
	Reconciled             bool             `json:"reconciled"`
	OutcomeUnknown         bool             `json:"outcomeUnknown"`
	VerificationPending    bool             `json:"verificationPending"`
	State                  string           `json:"state"`
	AccountID              string           `json:"accountId,omitempty"`
	ExternalID             string           `json:"externalId"`
	CreateResponse         *apiOutput       `json:"createResponse,omitempty"`
	VerificationResponse   *apiOutput       `json:"verificationResponse,omitempty"`
	ReconciliationResponse *apiOutput       `json:"reconciliationResponse,omitempty"`
	Error                  *fastPathError   `json:"error,omitempty"`
	DurationMS             accountDurations `json:"durationMs"`
}

type accountDurations struct {
	Create         int64 `json:"create,omitempty"`
	Reconciliation int64 `json:"reconciliation,omitempty"`
	Verification   int64 `json:"verification,omitempty"`
	Total          int64 `json:"total"`
}

func runAccount(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, accountHelp)
		return errHelp
	}
	switch strings.ToLower(args[0]) {
	case "create":
		return runAccountCreate(args[1:], stdout, stderr)
	case "show":
		return runAccountShow(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown account command %q", args[0])
	}
}

func runAccountCreate(args []string, stdout, stderr io.Writer) error {
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	options := accountCreateOptions{baseURL: environment.apiBaseURL, timeout: 30 * time.Second}
	flags := flag.NewFlagSet("account create", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&options.name, "name", "", "Ledger account name")
	flags.StringVar(&options.externalID, "external-id", "", "Stable external account identifier")
	flags.StringVar(&options.metadataFile, "metadata-file", "", "Optional JSON object file")
	flags.DurationVar(&options.timeout, "timeout", 30*time.Second, "Per-request HTTP timeout")
	flags.BoolVar(&options.confirmed, "confirm", false, "Confirm ledger account creation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected account create arguments: %s", strings.Join(flags.Args(), " "))
	}
	body, err := validateAndMarshalAccountCreate(&options)
	if err != nil {
		return err
	}
	return executeAccountCreate(options, body, stdout)
}

func validateAndMarshalAccountCreate(options *accountCreateOptions) ([]byte, error) {
	if !options.confirmed {
		return nil, fmt.Errorf("account creation may change state; review the request and pass --confirm")
	}
	options.name = strings.TrimSpace(options.name)
	if options.name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	options.externalID = strings.TrimSpace(options.externalID)
	if options.externalID == "" {
		return nil, fmt.Errorf("--external-id is required for safe reconciliation")
	}
	if utf8.RuneCountInString(options.externalID) > 128 {
		return nil, fmt.Errorf("--external-id must be at most 128 characters")
	}
	if options.timeout <= 0 {
		return nil, fmt.Errorf("--timeout must be greater than zero")
	}
	if err := validateAPIBaseURL(options.baseURL); err != nil {
		return nil, err
	}
	payload := accountCreateRequest{Name: options.name, ExternalID: options.externalID}
	if options.metadataFile != "" {
		data, err := os.ReadFile(options.metadataFile)
		if err != nil {
			return nil, fmt.Errorf("read metadata file: %w", err)
		}
		if len(data) > 16*1024 {
			return nil, fmt.Errorf("metadata file must not exceed 16 KiB")
		}
		var metadata map[string]interface{}
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("metadata file must contain one JSON object: %w", err)
		}
		if metadata == nil {
			return nil, fmt.Errorf("metadata file must contain one JSON object")
		}
		payload.Metadata, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("normalize metadata object: %w", err)
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode account body: %w", err)
	}
	return encoded, nil
}

func executeAccountCreate(options accountCreateOptions, body []byte, stdout io.Writer) error {
	totalStart := time.Now()
	result := accountCreateOutput{State: "creating", ExternalID: options.externalID}
	client := newFastPathHTTPClient(options.timeout)
	var requested accountCreateRequest
	if err := json.Unmarshal(body, &requested); err != nil {
		return fmt.Errorf("decode deterministic account body: %w", err)
	}
	createStart := time.Now()
	createResponse, createErr := sendSignedAPIRequest(
		client, options.baseURL, http.MethodPost, "/api/v1/ledger/accounts", body, "", true,
	)
	result.DurationMS.Create = time.Since(createStart).Milliseconds()
	if createErr == nil {
		result.CreateResponse = &createResponse
	}

	unknownCreate := false
	if createErr != nil {
		var outcomeUnknown *requestOutcomeUnknownError
		unknownCreate = errors.As(createErr, &outcomeUnknown)
		if !unknownCreate {
			result.State = "create_error"
			result.Error = &fastPathError{Stage: "create", Message: createErr.Error()}
			result.DurationMS.Total = time.Since(totalStart).Milliseconds()
			if outputErr := writeJSON(stdout, result, "account create"); outputErr != nil {
				return outputErr
			}
			return fmt.Errorf("create account: %w", createErr)
		}
	} else if createResponse.StatusCode >= http.StatusInternalServerError {
		unknownCreate = true
	} else if createResponse.StatusCode == http.StatusConflict && transactionStringField(createResponse.Body, "code") == "ACCOUNT_EXTERNAL_ID_CONFLICT" {
		// An existing exact externalId may represent a previously completed
		// logical operation. Resolve it rather than repeating the mutation.
	} else if createResponse.StatusCode >= http.StatusOK && createResponse.StatusCode < http.StatusMultipleChoices && createResponse.StatusCode != http.StatusCreated {
		result.Accepted = true
		result.VerificationPending = true
		result.State = "accepted_unexpected_create_status"
		result.Error = &fastPathError{Stage: "verification", Message: fmt.Sprintf("create was accepted with unexpected HTTP %d; read-back was not attempted", createResponse.StatusCode)}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		return writeJSON(stdout, result, "account create")
	} else if createResponse.StatusCode != http.StatusCreated {
		result.State = "create_failed"
		result.Error = &fastPathError{Stage: "create", Message: fmt.Sprintf("API returned HTTP %d", createResponse.StatusCode)}
		result.DurationMS.Total = time.Since(totalStart).Milliseconds()
		if outputErr := writeJSON(stdout, result, "account create"); outputErr != nil {
			return outputErr
		}
		return fmt.Errorf("create account: API returned HTTP %d", createResponse.StatusCode)
	}

	if createErr == nil && createResponse.StatusCode == http.StatusCreated {
		result.Accepted = true
		result.Created = true
		result.AccountID = transactionStringField(createResponse.Body, "id")
		if result.AccountID != "" {
			verificationStart := time.Now()
			verificationResponse, verificationErr := readAccountByID(client, options.baseURL, result.AccountID)
			result.DurationMS.Verification = time.Since(verificationStart).Milliseconds()
			if verificationErr == nil {
				result.VerificationResponse = &verificationResponse
			}
			if verificationErr == nil && verificationResponse.StatusCode == http.StatusOK &&
				accountResponseMatches(verificationResponse.Body, result.AccountID, requested) {
				result.State = "created"
				result.DurationMS.Total = time.Since(totalStart).Milliseconds()
				return writeJSON(stdout, result, "account create")
			}
			result.VerificationPending = true
			result.State = "verification_pending"
			if verificationErr != nil {
				result.Error = &fastPathError{Stage: "verification", Message: verificationErr.Error()}
			} else {
				result.VerificationResponse = &verificationResponse
				result.Error = &fastPathError{Stage: "verification", Message: fmt.Sprintf("API returned HTTP %d or a mismatched account id", verificationResponse.StatusCode)}
			}
			result.DurationMS.Total = time.Since(totalStart).Milliseconds()
			return writeJSON(stdout, result, "account create")
		}
	}

	// Missing IDs, external-id conflicts, transport failures, and 5xx responses
	// are reconciled with one exact read. The POST is never repeated.
	reconciliationStart := time.Now()
	reconciliationResponse, account, reconciliationErr := reconcileAccountByExternalID(
		client, options.baseURL, requested,
	)
	result.DurationMS.Reconciliation = time.Since(reconciliationStart).Milliseconds()
	result.ReconciliationResponse = &reconciliationResponse
	if reconciliationErr == nil && account != nil {
		result.AccountID = objectString(account, "id")
		verificationStart := time.Now()
		verificationResponse, verificationErr := readAccountByID(client, options.baseURL, result.AccountID)
		result.DurationMS.Verification = time.Since(verificationStart).Milliseconds()
		result.VerificationResponse = &verificationResponse
		if verificationErr == nil && verificationResponse.StatusCode == http.StatusOK &&
			accountResponseMatches(verificationResponse.Body, result.AccountID, requested) {
			result.Reconciled = true
			if result.Created {
				result.State = "created_reconciled"
			} else {
				result.State = "reconciled_existing"
			}
			result.DurationMS.Total = time.Since(totalStart).Milliseconds()
			return writeJSON(stdout, result, "account create")
		}
		if verificationErr != nil {
			reconciliationErr = verificationErr
		} else {
			reconciliationErr = fmt.Errorf("reconciled account read-back did not match the requested account")
		}
	}

	result.VerificationPending = true
	result.DurationMS.Total = time.Since(totalStart).Milliseconds()
	if result.Accepted {
		result.State = "accepted_missing_account_id"
		message := "accepted create response did not contain a usable account id and exact external-id reconciliation did not complete"
		if reconciliationErr != nil {
			message += ": " + reconciliationErr.Error()
		}
		result.Error = &fastPathError{Stage: "verification", Message: message}
		return writeJSON(stdout, result, "account create")
	}
	if unknownCreate {
		result.OutcomeUnknown = true
		result.State = "create_outcome_unknown"
		message := "create outcome is unknown and exact external-id reconciliation did not find one account"
		if createErr != nil {
			message = createErr.Error() + "; " + message
		}
		if reconciliationErr != nil {
			message += ": " + reconciliationErr.Error()
		}
		result.Error = &fastPathError{Stage: "reconciliation", Message: message}
		return writeJSON(stdout, result, "account create")
	}
	result.State = "conflict_unresolved"
	result.Error = &fastPathError{Stage: "reconciliation", Message: reconciliationErr.Error()}
	if outputErr := writeJSON(stdout, result, "account create"); outputErr != nil {
		return outputErr
	}
	return fmt.Errorf("account conflict could not be reconciled")
}

func reconcileAccountByExternalID(client *http.Client, baseURL string, requested accountCreateRequest) (apiOutput, map[string]interface{}, error) {
	query := url.Values{"externalId": {requested.ExternalID}, "limit": {"2"}}
	target := "/api/v1/ledger/accounts?" + query.Encode()
	response, err := sendSignedAPIRequest(client, baseURL, http.MethodGet, target, nil, "", false)
	if err != nil {
		return apiOutput{}, nil, err
	}
	if response.StatusCode != http.StatusOK {
		return response, nil, fmt.Errorf("API returned HTTP %d", response.StatusCode)
	}
	items, err := responseItems(response.Body)
	if err != nil {
		return response, nil, err
	}
	responseObject, _ := response.Body.(map[string]interface{})
	if nextCursor, _ := responseObject["nextCursor"].(string); nextCursor != "" {
		return response, nil, fmt.Errorf("external-id lookup returned an additional page; refusing an ambiguous reconciliation")
	}
	matches := make([]map[string]interface{}, 0, 1)
	for _, item := range items {
		if accountObjectMatches(item, "", requested) {
			matches = append(matches, item)
		}
	}
	if len(matches) != 1 {
		return response, nil, fmt.Errorf("found %d exact external-id matches; expected exactly one", len(matches))
	}
	return response, matches[0], nil
}

func accountResponseMatches(body interface{}, expectedID string, requested accountCreateRequest) bool {
	object, ok := body.(map[string]interface{})
	return ok && accountObjectMatches(object, expectedID, requested)
}

func accountObjectMatches(object map[string]interface{}, expectedID string, requested accountCreateRequest) bool {
	if expectedID != "" && objectString(object, "id") != expectedID {
		return false
	}
	if objectString(object, "name") != requested.Name || objectString(object, "externalId") != requested.ExternalID {
		return false
	}
	if len(requested.Metadata) == 0 {
		return true
	}
	var expected interface{}
	if err := json.Unmarshal(requested.Metadata, &expected); err != nil {
		return false
	}
	actual, present := object["metadata"]
	return present && reflect.DeepEqual(actual, expected)
}

func readAccountByID(client *http.Client, baseURL, accountID string) (apiOutput, error) {
	target := "/api/v1/ledger/accounts/" + url.PathEscape(accountID)
	return sendSignedAPIRequest(client, baseURL, http.MethodGet, target, nil, "", false)
}

func runAccountShow(args []string, stdout, stderr io.Writer) error {
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	baseURL := environment.apiBaseURL
	accountID := ""
	timeout := 30 * time.Second
	flags := flag.NewFlagSet("account show", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&accountID, "account-id", "", "Ledger account ID")
	flags.DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected account show arguments: %s", strings.Join(flags.Args(), " "))
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("--account-id is required")
	}
	if timeout <= 0 {
		return fmt.Errorf("--timeout must be greater than zero")
	}
	if err := validateAPIBaseURL(baseURL); err != nil {
		return err
	}
	response, err := readAccountByID(newFastPathHTTPClient(timeout), baseURL, accountID)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		if err := writeJSON(stdout, response, "account show"); err != nil {
			return err
		}
		return fmt.Errorf("account show: API returned HTTP %d", response.StatusCode)
	}
	object, ok := response.Body.(map[string]interface{})
	if !ok || objectString(object, "id") != accountID {
		if err := writeJSON(stdout, response, "account show"); err != nil {
			return err
		}
		return fmt.Errorf("account show: response account id does not match the requested account")
	}
	return writeJSON(stdout, response, "account show")
}
