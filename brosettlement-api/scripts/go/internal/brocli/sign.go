package brocli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/broauth"
)

type signOutput struct {
	CanonicalRequestTarget string            `json:"canonicalRequestTarget"`
	Headers                map[string]string `json:"headers"`
	Body                   string            `json:"body,omitempty"`
}

func runSign(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: brosettlement sign METHOD TARGET [--body-file FILE] [--idempotency-key KEY] [--timestamp UNIX_SECONDS] [--nonce VALUE]")
		fmt.Fprintln(stdout, "Print signed REST headers without sending a request.")
		return errHelp
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: brosettlement sign METHOD TARGET [options]")
	}

	method := strings.ToUpper(args[0])
	target := args[1]
	if !strings.HasPrefix(target, "/") {
		return fmt.Errorf("target must be an exact request target beginning with /")
	}

	var bodyFile, idempotencyKey, timestamp, nonce string
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&bodyFile, "body-file", "", "File containing exact request body bytes")
	flags.StringVar(&idempotencyKey, "idempotency-key", "", "Stable logical-operation key")
	flags.StringVar(&timestamp, "timestamp", "", "Unsigned Unix timestamp for deterministic signing")
	flags.StringVar(&nonce, "nonce", "", "16-128 character nonce for deterministic signing")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected sign arguments: %s", strings.Join(flags.Args(), " "))
	}

	normalizedIdempotencyKey, err := broauth.NormalizeExplicitIdempotencyKey(method, target, idempotencyKey)
	if err != nil {
		return fmt.Errorf("%s %s %w", method, target, err)
	}
	idempotencyKey = normalizedIdempotencyKey

	body := []byte{}
	if bodyFile != "" {
		body, err = os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf("read body file: %w", err)
		}
	}
	exactEmptyJSONObject := broauth.RequiresExactEmptyJSONObject(method, target)
	if exactEmptyJSONObject {
		if bodyFile == "" {
			body = []byte("{}")
		} else if !bytes.Equal(body, []byte("{}")) {
			return fmt.Errorf("%s requires the exact two-byte JSON body {}", target)
		}
	}

	headers, generatedNonce, err := broauth.RESTHeadersWithInputs(method, target, body, timestamp, nonce)
	if err != nil {
		return fmt.Errorf("sign request: %w", err)
	}
	if len(body) > 0 {
		headers.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		headers.Set("X-Idempotency-Key", idempotencyKey)
	} else if broauth.RequiresIdempotency(method, target) {
		headers.Set("X-Idempotency-Key", "req-"+generatedNonce)
	}

	output := signOutput{
		CanonicalRequestTarget: target,
		Headers: map[string]string{
			"X-Api-Key-Id":    headers.Get("X-Api-Key-Id"),
			"X-Api-Timestamp": headers.Get("X-Api-Timestamp"),
			"X-Api-Nonce":     headers.Get("X-Api-Nonce"),
			"X-Api-Signature": headers.Get("X-Api-Signature"),
		},
	}
	for _, name := range []string{"X-Api-Body-Hash", "Content-Type", "X-Idempotency-Key"} {
		if value := headers.Get(name); value != "" {
			output.Headers[name] = value
		}
	}
	if exactEmptyJSONObject {
		output.Body = "{}"
	}
	return writeJSON(stdout, output, "signed headers")
}
