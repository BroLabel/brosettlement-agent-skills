package main

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

type output struct {
	StatusCode int         `json:"statusCode"`
	RequestID  string      `json:"requestId,omitempty"`
	Body       interface{} `json:"body,omitempty"`
}

func main() {
	baseURL := flag.String("base-url", defaultBaseURL, "BroSettlement API base URL")
	method := flag.String("method", http.MethodGet, "HTTP method")
	target := flag.String("target", "", "Exact request target, including query string")
	bodyFile := flag.String("body-file", "", "File containing the exact request body bytes")
	idempotencyKey := flag.String("idempotency-key", "", "Explicit idempotency key")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP timeout")
	flag.Parse()

	if *target == "" || !strings.HasPrefix(*target, "/") {
		fail("-target must be an exact request target beginning with /")
	}
	body := []byte{}
	var err error
	if *bodyFile != "" {
		body, err = os.ReadFile(*bodyFile)
		if err != nil {
			fail("read body file: %v", err)
		}
	}
	explicitEmptyFormBody := broauth.RequiresExplicitEmptyFormBody(*method, *target)
	if explicitEmptyFormBody && *bodyFile != "" {
		fail("%s requires an explicit zero-length form body; omit -body-file", *target)
	}

	headers, nonce, err := broauth.RESTHeaders(*method, *target, body)
	if err != nil {
		fail("sign request: %v", err)
	}
	if len(body) > 0 {
		headers.Set("Content-Type", "application/json")
	} else if explicitEmptyFormBody {
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if *idempotencyKey != "" {
		headers.Set("X-Idempotency-Key", *idempotencyKey)
	} else if broauth.RequiresIdempotency(*method, *target) {
		headers.Set("X-Idempotency-Key", "req-"+nonce)
	}

	requestURL := strings.TrimRight(*baseURL, "/") + *target
	request, err := http.NewRequest(strings.ToUpper(*method), requestURL, bytes.NewReader(body))
	if err != nil {
		fail("create request: %v", err)
	}
	request.Header = headers

	response, err := (&http.Client{Timeout: *timeout}).Do(request)
	if err != nil {
		fail("send request: %v", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		fail("read response: %v", err)
	}

	var parsedBody interface{}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &parsedBody); err != nil {
			parsedBody = string(responseBody)
		}
	}
	result := output{
		StatusCode: response.StatusCode,
		RequestID:  response.Header.Get("X-Request-Id"),
		Body:       parsedBody,
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail("encode output: %v", err)
	}
	fmt.Println(string(encoded))
	if response.StatusCode >= http.StatusBadRequest {
		os.Exit(1)
	}
}

func fail(format string, values ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
