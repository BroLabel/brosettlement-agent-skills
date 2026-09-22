package brocli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/broauth"
)

type requestOutcomeUnknownError struct {
	err error
}

var fastPathSleep = time.Sleep

func (err *requestOutcomeUnknownError) Error() string {
	return err.err.Error()
}

func (err *requestOutcomeUnknownError) Unwrap() error {
	return err.err
}

func newFastPathHTTPClient(timeoutDuration time.Duration) *http.Client {
	client := newHTTPClient(timeoutDuration)
	// High-level commands never follow redirects. A redirect could leak signed
	// headers or turn one logical operation into additional HTTP requests.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

func sendSignedAPIRequest(
	client *http.Client,
	baseURL string,
	method string,
	target string,
	body []byte,
	idempotencyKey string,
	mutation bool,
) (apiOutput, error) {
	headers, nonce, err := broauth.RESTHeaders(method, target, body)
	if err != nil {
		return apiOutput{}, fmt.Errorf("sign request: %w", err)
	}
	if len(body) > 0 {
		headers.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		headers.Set("X-Idempotency-Key", idempotencyKey)
	} else if broauth.RequiresIdempotency(method, target) {
		headers.Set("X-Idempotency-Key", "req-"+nonce)
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
		if mutation {
			return apiOutput{}, &requestOutcomeUnknownError{
				err: fmt.Errorf("send request: %w", err),
			}
		}
		return apiOutput{}, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		if mutation {
			return apiOutput{}, &requestOutcomeUnknownError{
				err: fmt.Errorf("read response: %w", err),
			}
		}
		return apiOutput{}, fmt.Errorf("read response: %w", err)
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

func writeJSON(stdout io.Writer, value interface{}, label string) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s output: %w", label, err)
	}
	if _, err := fmt.Fprintln(stdout, string(encoded)); err != nil {
		return fmt.Errorf("write %s output: %w", label, err)
	}
	return nil
}

func responseItems(body interface{}) ([]map[string]interface{}, error) {
	object, ok := body.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("response body is not an object")
	}
	rawItems, ok := object["items"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("response body does not contain an items array")
	}
	items := make([]map[string]interface{}, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("response items contain a non-object value")
		}
		items = append(items, item)
	}
	return items, nil
}

func objectString(object map[string]interface{}, key string) string {
	value, _ := object[key].(string)
	return value
}
