package brocli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/broauth"
)

const defaultBaseURL = "https://brosettlement-staging-api.brolabel.io"

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
		return errHelp
	}
	if len(args) < 2 {
		return errorsForAPIUsage()
	}
	options := apiOptions{method: strings.ToUpper(args[0]), target: args[1]}
	flags := flag.NewFlagSet("api", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.baseURL, "base-url", defaultBaseURL, "BroSettlement API base URL")
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
	switch strings.ToLower(args[0]) {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("mpc status does not accept additional arguments")
		}
		return executeAPI(apiOptions{
			baseURL: defaultBaseURL,
			method:  http.MethodGet,
			target:  "/api/v1/mpc/status",
			timeout: 30 * time.Second,
		}, stdout)
	case "initialize":
		options := apiOptions{
			baseURL: defaultBaseURL,
			method:  http.MethodPost,
			target:  "/api/v1/mpc/initialize",
			timeout: 30 * time.Second,
		}
		flags := flag.NewFlagSet("mpc initialize", flag.ContinueOnError)
		flags.SetOutput(stderr)
		flags.StringVar(&options.baseURL, "base-url", defaultBaseURL, "BroSettlement API base URL")
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
		options.baseURL = defaultBaseURL
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

	body := []byte{}
	if options.bodyFile != "" {
		var err error
		body, err = os.ReadFile(options.bodyFile)
		if err != nil {
			return fmt.Errorf("read body file: %w", err)
		}
	}
	explicitEmptyFormBody := broauth.RequiresExplicitEmptyFormBody(options.method, options.target)
	if explicitEmptyFormBody && options.bodyFile != "" {
		return fmt.Errorf("%s requires an explicit zero-length form body; omit --body-file", options.target)
	}

	headers, nonce, err := broauth.RESTHeaders(options.method, options.target, body)
	if err != nil {
		return fmt.Errorf("sign request: %w", err)
	}
	if len(body) > 0 {
		headers.Set("Content-Type", "application/json")
	} else if explicitEmptyFormBody {
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if options.idempotencyKey != "" {
		headers.Set("X-Idempotency-Key", options.idempotencyKey)
	} else if broauth.RequiresIdempotency(options.method, options.target) {
		headers.Set("X-Idempotency-Key", "req-"+nonce)
	}

	requestURL := strings.TrimRight(options.baseURL, "/") + options.target
	request, err := http.NewRequest(strings.ToUpper(options.method), requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header = headers
	response, err := newHTTPClient(options.timeout).Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var parsedBody interface{}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &parsedBody); err != nil {
			parsedBody = string(responseBody)
		}
	}
	result := apiOutput{
		StatusCode: response.StatusCode,
		RequestID:  response.Header.Get("X-Request-Id"),
		Body:       parsedBody,
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	fmt.Fprintln(stdout, string(encoded))
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("API returned HTTP %d", response.StatusCode)
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
