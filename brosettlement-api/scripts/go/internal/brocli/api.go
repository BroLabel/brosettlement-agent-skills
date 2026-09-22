package brocli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/broauth"
)

var newHTTPClient = func(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

type apiOptions struct {
	baseURL        string
	method         string
	target         string
	bodyFile       string
	idempotencyKey string
	timeout        time.Duration
	confirmed      bool
}

type apiOutput struct {
	StatusCode int         `json:"statusCode"`
	RequestID  string      `json:"requestId,omitempty"`
	Body       interface{} `json:"body,omitempty"`
}

func runAPI(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: brosettlement api METHOD TARGET [--body-file FILE] [--idempotency-key KEY] [--confirm]")
		fmt.Fprintln(stdout, "Non-read-only methods require --confirm.")
		fmt.Fprintln(stdout, "POST /api/v1/wallets and POST /api/v1/transactions require an explicit stable --idempotency-key.")
		fmt.Fprintln(stdout, "Non-read-only methods are sent once; automatic HTTP transport replay is disabled.")
		return errHelp
	}
	if len(args) < 2 {
		return errorsForAPIUsage()
	}
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	options := apiOptions{method: strings.ToUpper(args[0]), target: args[1]}
	flags := flag.NewFlagSet("api", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&options.bodyFile, "body-file", "", "File containing exact request body bytes")
	flags.StringVar(&options.idempotencyKey, "idempotency-key", "", "Stable logical-operation key")
	flags.DurationVar(&options.timeout, "timeout", 30*time.Second, "HTTP timeout")
	flags.BoolVar(&options.confirmed, "confirm", false, "Confirm a state-changing request")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected API arguments: %s", strings.Join(flags.Args(), " "))
	}
	return executeAPI(options, stdout)
}

func runMPC(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, "Usage: brosettlement mpc status | brosettlement mpc initialize --confirm [--idempotency-key KEY]")
		return errHelp
	}
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	switch strings.ToLower(args[0]) {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("mpc status does not accept additional arguments")
		}
		return executeAPI(apiOptions{
			baseURL: environment.apiBaseURL,
			method:  http.MethodGet,
			target:  "/api/v1/mpc/status",
			timeout: 30 * time.Second,
		}, stdout)
	case "initialize":
		options := apiOptions{
			baseURL: environment.apiBaseURL,
			method:  http.MethodPost,
			target:  "/api/v1/mpc/initialize",
			timeout: 30 * time.Second,
		}
		flags := flag.NewFlagSet("mpc initialize", flag.ContinueOnError)
		flags.SetOutput(stderr)
		flags.StringVar(&options.baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
		flags.StringVar(&options.idempotencyKey, "idempotency-key", "", "Stable logical-operation key")
		flags.DurationVar(&options.timeout, "timeout", 30*time.Second, "HTTP timeout")
		flags.BoolVar(&options.confirmed, "confirm", false, "Confirm MPC initialization")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected MPC initialize arguments: %s", strings.Join(flags.Args(), " "))
		}
		return executeAPI(options, stdout)
	default:
		return fmt.Errorf("unknown MPC command %q", args[0])
	}
}

func executeAPI(options apiOptions, stdout io.Writer) error {
	if options.baseURL == "" {
		environment, err := selectedEnvironment()
		if err != nil {
			return err
		}
		options.baseURL = environment.apiBaseURL
	}
	if options.timeout == 0 {
		options.timeout = 30 * time.Second
	}
	if options.target == "" || !strings.HasPrefix(options.target, "/") {
		return fmt.Errorf("target must be an exact request target beginning with /")
	}
	if !isReadOnlyMethod(options.method) && !options.confirmed {
		return fmt.Errorf("%s %s may change state; review the request and pass --confirm", options.method, options.target)
	}
	normalizedIdempotencyKey, err := broauth.NormalizeExplicitIdempotencyKey(
		options.method,
		options.target,
		options.idempotencyKey,
	)
	if err != nil {
		return fmt.Errorf("%s %s %w", options.method, options.target, err)
	}
	options.idempotencyKey = normalizedIdempotencyKey

	body := []byte{}
	if options.bodyFile != "" {
		var err error
		body, err = os.ReadFile(options.bodyFile)
		if err != nil {
			return fmt.Errorf("read body file: %w", err)
		}
	}
	exactEmptyJSONObject := broauth.RequiresExactEmptyJSONObject(options.method, options.target)
	if exactEmptyJSONObject {
		if options.bodyFile == "" {
			body = []byte("{}")
		} else if !bytes.Equal(body, []byte("{}")) {
			return fmt.Errorf("%s requires the exact two-byte JSON body {}", options.target)
		}
	}

	result, err := sendSignedAPIRequest(
		newHTTPClient(options.timeout),
		options.baseURL,
		strings.ToUpper(options.method),
		options.target,
		body,
		options.idempotencyKey,
		!isReadOnlyMethod(options.method),
	)
	if err != nil {
		return err
	}
	if err := writeJSON(stdout, result, "API"); err != nil {
		return err
	}
	if result.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("API returned HTTP %d", result.StatusCode)
	}
	return nil
}

func isReadOnlyMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func errorsForAPIUsage() error {
	return fmt.Errorf("usage: brosettlement api METHOD TARGET [options]")
}
