package brocli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunVersionJSON(t *testing.T) {
	previousVersion, previousCommit := Version, Commit
	Version, Commit = "1.2.3", "abcdef0"
	t.Cleanup(func() { Version, Commit = previousVersion, previousCommit })

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version":"1.2.3"`) ||
		!strings.Contains(stdout.String(), `"commit":"abcdef0"`) {
		t.Fatalf("unexpected version output: %s", stdout.String())
	}
}

func TestUpdateAutoReplacesOnlyExecutable(t *testing.T) {
	target := filepath.Join(t.TempDir(), "brosettlement")
	if err := os.WriteFile(target, []byte("old-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBinary := []byte("new-cli")
	client := newReleaseClient("cli-v1.1.0", newBinary, false)
	configureUpdaterTest(t, client, target)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"update", "--auto"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	updated, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != string(newBinary) {
		t.Fatalf("executable was not updated: %q", updated)
	}
	if !strings.Contains(stdout.String(), "from 1.0.0 to 1.1.0") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestUpdateRejectsChecksumMismatch(t *testing.T) {
	target := filepath.Join(t.TempDir(), "brosettlement")
	if err := os.WriteFile(target, []byte("old-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := newReleaseClient("cli-v1.1.0", []byte("new-cli"), true)
	configureUpdaterTest(t, client, target)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"update", "--auto"}, &stdout, &stderr); code != 1 {
		t.Fatalf("Run returned %d, want 1", code)
	}
	current, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "old-cli" {
		t.Fatalf("checksum failure changed executable: %q", current)
	}
}

func TestUpdateCheckDoesNotReplaceExecutable(t *testing.T) {
	target := filepath.Join(t.TempDir(), "brosettlement")
	if err := os.WriteFile(target, []byte("old-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := newReleaseClient("cli-v1.1.0", []byte("new-cli"), false)
	configureUpdaterTest(t, client, target)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"update"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run returned %d: %s", code, stderr.String())
	}
	current, _ := os.ReadFile(target)
	if string(current) != "old-cli" {
		t.Fatalf("check-only update changed executable: %q", current)
	}
	if !strings.Contains(stdout.String(), "1.1.0 is available") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func configureUpdaterTest(t *testing.T, client *http.Client, executable string) {
	t.Helper()
	previousURL := releaseAPIURL
	previousVersion := Version
	previousGOOS, previousGOARCH := updateGOOS, updateGOARCH
	previousExecutable := updateExecutable
	previousClient := updateHTTPClient
	previousVerify := verifyDownloadedCLI
	releaseAPIURL = "http://127.0.0.1:80/latest"
	Version = "1.0.0"
	updateGOOS, updateGOARCH = "linux", "amd64"
	updateExecutable = func() (string, error) { return executable, nil }
	updateHTTPClient = func(timeout time.Duration) *http.Client {
		client.Timeout = timeout
		return client
	}
	verifyDownloadedCLI = func(_ string, expected string) error {
		if expected != "1.1.0" {
			return fmt.Errorf("unexpected candidate version %s", expected)
		}
		return nil
	}
	t.Cleanup(func() {
		releaseAPIURL = previousURL
		Version = previousVersion
		updateGOOS, updateGOARCH = previousGOOS, previousGOARCH
		updateExecutable = previousExecutable
		updateHTTPClient = previousClient
		verifyDownloadedCLI = previousVerify
	})
}

func newReleaseClient(tag string, binary []byte, badChecksum bool) *http.Client {
	checksum := fmt.Sprintf("%x", sha256.Sum256(binary))
	if badChecksum {
		checksum = strings.Repeat("0", 64)
	}
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		switch request.URL.Path {
		case "/latest":
			body = fmt.Sprintf(`[{"tag_name":%q,"assets":[`+
				`{"name":"brosettlement-linux-amd64","browser_download_url":%q},`+
				`{"name":"checksums.txt","browser_download_url":%q}]}]`,
				tag, "http://127.0.0.1:80/brosettlement-linux-amd64", "http://127.0.0.1:80/checksums.txt")
		case "/brosettlement-linux-amd64":
			body = string(binary)
		case "/checksums.txt":
			body = fmt.Sprintf("%s  brosettlement-linux-amd64\n", checksum)
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("not found"))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}
