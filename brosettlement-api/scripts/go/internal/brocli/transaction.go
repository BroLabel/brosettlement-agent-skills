package brocli

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const transactionHelp = `Usage:
  brosettlement transaction status --id ID
  brosettlement transaction wait --id ID [--timeout 30s] [--poll-interval 2s]

status performs one signed GET. wait performs sequential signed GET requests,
uses a fresh nonce for every request, and stops at CONFIRMED, FAILED,
FAILED_ON_CHAIN, or the bounded timeout. It never creates or retries a transaction.`

type transactionReadOutput struct {
	TransactionID       string         `json:"transactionId"`
	Status              string         `json:"status,omitempty"`
	Terminal            bool           `json:"terminal"`
	VerificationPending bool           `json:"verificationPending"`
	TimedOut            bool           `json:"timedOut"`
	State               string         `json:"state"`
	Attempts            int            `json:"attempts"`
	Response            *apiOutput     `json:"response,omitempty"`
	Error               *fastPathError `json:"error,omitempty"`
	DurationMS          int64          `json:"durationMs"`
}

func runTransaction(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, transactionHelp)
		return errHelp
	}
	switch strings.ToLower(args[0]) {
	case "status", "get":
		return runTransactionRead(args[0], args[1:], stdout, stderr, false)
	case "wait":
		return runTransactionRead(args[0], args[1:], stdout, stderr, true)
	default:
		return fmt.Errorf("unknown transaction command %q", args[0])
	}
}

func runTransactionRead(command string, args []string, stdout, stderr io.Writer, wait bool) error {
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	baseURL := environment.apiBaseURL
	transactionID := ""
	requestTimeout := 15 * time.Second
	waitTimeout := 30 * time.Second
	pollInterval := 2 * time.Second
	flags := flag.NewFlagSet("transaction "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&transactionID, "id", "", "Transaction ID")
	flags.DurationVar(&requestTimeout, "request-timeout", 15*time.Second, "Per-request HTTP timeout")
	if wait {
		flags.DurationVar(&waitTimeout, "timeout", 30*time.Second, "Bounded total wait duration (maximum 2m)")
		flags.DurationVar(&pollInterval, "poll-interval", 2*time.Second, "Polling interval (minimum 1s)")
	} else {
		flags.DurationVar(&requestTimeout, "timeout", 15*time.Second, "HTTP timeout")
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected transaction %s arguments: %s", command, strings.Join(flags.Args(), " "))
	}
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return fmt.Errorf("--id is required")
	}
	if requestTimeout <= 0 {
		return fmt.Errorf("--request-timeout must be greater than zero")
	}
	if wait {
		if waitTimeout <= 0 || waitTimeout > 2*time.Minute {
			return fmt.Errorf("--timeout must be greater than zero and no more than 2m")
		}
		if pollInterval < time.Second {
			return fmt.Errorf("--poll-interval must be at least 1s")
		}
	}
	if err := validateAPIBaseURL(baseURL); err != nil {
		return err
	}
	return executeTransactionRead(baseURL, transactionID, requestTimeout, wait, waitTimeout, pollInterval, stdout)
}

func executeTransactionRead(
	baseURL string,
	transactionID string,
	requestTimeout time.Duration,
	wait bool,
	waitTimeout time.Duration,
	pollInterval time.Duration,
	stdout io.Writer,
) error {
	start := time.Now()
	result := transactionReadOutput{
		TransactionID: transactionID,
		State:         "reading",
	}
	client := newFastPathHTTPClient(requestTimeout)
	target := "/api/v1/transactions/" + url.PathEscape(transactionID)
	deadline := start.Add(waitTimeout)
	if wait && waitTimeout < client.Timeout {
		client.Timeout = waitTimeout
	}

	for {
		if wait && result.Attempts > 0 {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				result.TimedOut = true
				result.State = "wait_timeout"
				result.DurationMS = time.Since(start).Milliseconds()
				return writeJSON(stdout, result, "transaction wait")
			}
			if remaining < client.Timeout {
				client.Timeout = remaining
			}
		}
		result.Attempts++
		response, err := sendSignedAPIRequest(client, baseURL, http.MethodGet, target, nil, "", false)
		if err != nil {
			result.State = "read_error"
			result.Error = &fastPathError{Stage: "read", Message: err.Error()}
			result.DurationMS = time.Since(start).Milliseconds()
			if outputErr := writeJSON(stdout, result, "transaction status"); outputErr != nil {
				return outputErr
			}
			return err
		}
		result.Response = &response
		if response.StatusCode != http.StatusOK {
			result.State = "read_failed"
			result.Error = &fastPathError{Stage: "read", Message: fmt.Sprintf("API returned HTTP %d", response.StatusCode)}
			result.DurationMS = time.Since(start).Milliseconds()
			if outputErr := writeJSON(stdout, result, "transaction status"); outputErr != nil {
				return outputErr
			}
			return fmt.Errorf("transaction read: API returned HTTP %d", response.StatusCode)
		}
		verifiedID := transactionStringField(response.Body, "id")
		if verifiedID != transactionID {
			result.State = "read_error"
			result.Error = &fastPathError{Stage: "read", Message: "response transaction id does not match the requested transaction"}
			result.DurationMS = time.Since(start).Milliseconds()
			if outputErr := writeJSON(stdout, result, "transaction status"); outputErr != nil {
				return outputErr
			}
			return fmt.Errorf("transaction response id mismatch")
		}
		result.Status = transactionStringField(response.Body, "status")
		result.VerificationPending = false
		switch result.Status {
		case "CONFIRMED":
			result.Terminal = true
			result.State = "confirmed"
		case "FAILED":
			result.Terminal = true
			result.State = "failed"
		case "FAILED_ON_CHAIN":
			result.Terminal = true
			result.State = "failed_on_chain"
		case "PENDING":
			result.VerificationPending = true
			result.State = "pending"
		default:
			result.State = "read_error"
			result.Error = &fastPathError{Stage: "read", Message: fmt.Sprintf("unknown transaction status %q", result.Status)}
			result.DurationMS = time.Since(start).Milliseconds()
			if outputErr := writeJSON(stdout, result, "transaction status"); outputErr != nil {
				return outputErr
			}
			return fmt.Errorf("unknown transaction status %q", result.Status)
		}
		if result.Terminal || !wait {
			result.DurationMS = time.Since(start).Milliseconds()
			return writeJSON(stdout, result, "transaction status")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			result.TimedOut = true
			result.State = "wait_timeout"
			result.DurationMS = time.Since(start).Milliseconds()
			return writeJSON(stdout, result, "transaction wait")
		}
		sleepFor := pollInterval
		if sleepFor > remaining {
			sleepFor = remaining
		}
		fastPathSleep(sleepFor)
	}
}
