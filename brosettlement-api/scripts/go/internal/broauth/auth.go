package broauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	apiKeyIDEnv       = "BROSETTLEMENT_API_KEY_ID"
	privateKeyFileEnv = "BROSETTLEMENT_API_PRIVATE_KEY_FILE"
)

type Credentials struct {
	APIKeyID   string
	PrivateKey ed25519.PrivateKey
}

func LoadCredentials() (Credentials, error) {
	keyID := strings.TrimSpace(os.Getenv(apiKeyIDEnv))
	keyPath := strings.TrimSpace(os.Getenv(privateKeyFileEnv))
	if keyID == "" {
		return Credentials{}, fmt.Errorf("set %s", apiKeyIDEnv)
	}
	if keyPath == "" {
		return Credentials{}, fmt.Errorf("set %s", privateKeyFileEnv)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return Credentials{}, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return Credentials{}, errors.New("private key file does not contain PEM data")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return Credentials{}, fmt.Errorf("parse PKCS#8 private key: %w", err)
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return Credentials{}, errors.New("private key is not Ed25519")
	}
	return Credentials{APIKeyID: keyID, PrivateKey: privateKey}, nil
}

func randomNonce() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func RESTHeaders(method, requestTarget string, body []byte) (http.Header, string, error) {
	return RESTHeadersWithInputs(method, requestTarget, body, "", "")
}

func RESTHeadersWithInputs(
	method string,
	requestTarget string,
	body []byte,
	timestamp string,
	nonce string,
) (http.Header, string, error) {
	if !strings.HasPrefix(requestTarget, "/") {
		return nil, "", errors.New("request target must begin with /")
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return nil, "", errors.New("method is required")
	}
	if timestamp != "" && !validUnixSeconds(timestamp) {
		return nil, "", errors.New("timestamp must be unsigned Unix seconds")
	}
	if nonce != "" && !validNonce(nonce) {
		return nil, "", errors.New("nonce must contain 16-128 letters, digits, dots, underscores, tildes, or hyphens")
	}
	credentials, err := LoadCredentials()
	if err != nil {
		return nil, "", err
	}
	if nonce == "" {
		nonce, err = randomNonce()
		if err != nil {
			return nil, "", err
		}
	}

	if timestamp == "" {
		timestamp = fmt.Sprintf("%d", time.Now().UTC().Unix())
	}
	bodyDigest := sha256.Sum256(body)
	bodyHash := ""
	if len(body) > 0 || RequiresBodyHash(method, requestTarget) {
		bodyHash = hex.EncodeToString(bodyDigest[:])
	}
	canonical := strings.Join([]string{
		method,
		requestTarget,
		bodyHash,
		timestamp,
		nonce,
		credentials.APIKeyID,
	}, "\n")
	signature := ed25519.Sign(credentials.PrivateKey, []byte(canonical))

	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("X-Api-Key-Id", credentials.APIKeyID)
	headers.Set("X-Api-Timestamp", timestamp)
	headers.Set("X-Api-Nonce", nonce)
	headers.Set("X-Api-Signature", base64.StdEncoding.EncodeToString(signature))
	if bodyHash != "" {
		headers.Set("X-Api-Body-Hash", bodyHash)
	}
	return headers, nonce, nil
}

func validUnixSeconds(value string) bool {
	if value == "0" {
		return true
	}
	if value == "" || value[0] < '1' || value[0] > '9' {
		return false
	}
	for index := 1; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validNonce(value string) bool {
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._~-", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func SignedWebSocketURL(rawURL string) (string, error) {
	credentials, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse WebSocket URL: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return "", errors.New("WebSocket URL must use ws or wss")
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	nonce, err := randomNonce()
	if err != nil {
		return "", err
	}
	timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())
	canonical := strings.Join([]string{"WS_CONNECT", path, timestamp, nonce}, "\n")
	signature := ed25519.Sign(credentials.PrivateKey, []byte(canonical))

	query := parsed.Query()
	query.Set("x-api-key-id", credentials.APIKeyID)
	query.Set("x-api-timestamp", timestamp)
	query.Set("x-api-nonce", nonce)
	query.Set("x-api-signature", base64.StdEncoding.EncodeToString(signature))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func RequiresIdempotency(method, requestTarget string) bool {
	if strings.ToUpper(method) != http.MethodPost {
		return false
	}
	path := requestPath(requestTarget)
	if path == "/api/v1/mpc/initialize" ||
		path == "/api/v1/wallets" ||
		path == "/api/v1/transactions" {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/co-signer/intents/") &&
		strings.HasSuffix(path, "/claim") {
		return true
	}
	return strings.HasPrefix(path, "/api/v1/co-signer/sessions/") &&
		strings.HasSuffix(path, "/messages")
}

func RequiresExplicitIdempotencyKey(method, requestTarget string) bool {
	if strings.ToUpper(method) != http.MethodPost {
		return false
	}
	path := requestPath(requestTarget)
	return path == "/api/v1/transactions" || path == "/api/v1/wallets"
}

func NormalizeExplicitIdempotencyKey(method, requestTarget, value string) (string, error) {
	if !RequiresExplicitIdempotencyKey(method, requestTarget) {
		return value, nil
	}
	return NormalizeStableIdempotencyKey(value)
}

func NormalizeStableIdempotencyKey(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", errors.New("requires an explicit stable --idempotency-key")
	}
	if len(normalized) > 128 {
		return "", errors.New("--idempotency-key must be at most 128 ASCII bytes")
	}
	for index := 0; index < len(normalized); index++ {
		if normalized[index] < 0x20 || normalized[index] > 0x7e {
			return "", errors.New("--idempotency-key must contain only printable ASCII characters")
		}
	}
	return normalized, nil
}

func RequiresBodyHash(method, requestTarget string) bool {
	if strings.ToUpper(method) != http.MethodPost {
		return false
	}
	path := requestPath(requestTarget)
	if path == "/api/v1/mpc/initialize" ||
		path == "/api/v1/wallets" ||
		path == "/api/v1/ledger/accounts" ||
		path == "/api/v1/transactions" {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/co-signer/intents/") &&
		strings.HasSuffix(path, "/result") {
		return true
	}
	return strings.HasPrefix(path, "/api/v1/co-signer/sessions/") &&
		strings.HasSuffix(path, "/messages")
}

func RequiresExactEmptyJSONObject(method, requestTarget string) bool {
	return strings.ToUpper(method) == http.MethodPost &&
		requestPath(requestTarget) == "/api/v1/mpc/initialize"
}

func requestPath(requestTarget string) string {
	if parsed, err := url.ParseRequestURI(requestTarget); err == nil {
		return parsed.Path
	}
	if before, _, ok := strings.Cut(requestTarget, "?"); ok {
		return before
	}
	return requestTarget
}
