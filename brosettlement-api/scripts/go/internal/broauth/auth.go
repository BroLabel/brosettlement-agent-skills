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
	if !strings.HasPrefix(requestTarget, "/") {
		return nil, "", errors.New("request target must begin with /")
	}
	credentials, err := LoadCredentials()
	if err != nil {
		return nil, "", err
	}
	nonce, err := randomNonce()
	if err != nil {
		return nil, "", err
	}

	method = strings.ToUpper(strings.TrimSpace(method))
	timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())
	bodyDigest := sha256.Sum256(body)
	bodyHash := ""
	if len(body) > 0 {
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
