package brocli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultReleaseAPIURL = "https://api.github.com/repos/BroLabel/brosettlement-agent-skills/releases?per_page=20"

var (
	releaseAPIURL       = defaultReleaseAPIURL
	updateGOOS          = runtime.GOOS
	updateGOARCH        = runtime.GOARCH
	updateExecutable    = os.Executable
	updateHTTPClient    = func(timeout time.Duration) *http.Client { return &http.Client{Timeout: timeout} }
	verifyDownloadedCLI = verifyCLIExecutable
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type githubRelease struct {
	TagName    string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

func runUpdate(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	auto := flags.Bool("auto", false, "Download and install an available verified update")
	timeout := flags.Duration("timeout", 30*time.Second, "GitHub request timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("update does not accept positional arguments")
	}

	client := updateHTTPClient(*timeout)
	release, err := fetchLatestRelease(client, releaseAPIURL)
	if err != nil {
		return fmt.Errorf("check GitHub release: %w", err)
	}
	latest, err := versionFromTag(release.TagName)
	if err != nil {
		return err
	}
	newer, err := isNewerVersion(latest, Version)
	if err != nil {
		return err
	}
	if !newer {
		fmt.Fprintf(stdout, "brosettlement CLI %s is up to date\n", Version)
		return nil
	}
	if !*auto {
		fmt.Fprintf(stdout, "brosettlement CLI %s is available (installed: %s)\n", latest, Version)
		return nil
	}

	assetName, err := platformAssetName(updateGOOS, updateGOARCH)
	if err != nil {
		return err
	}
	binaryAsset, ok := findReleaseAsset(release.Assets, assetName)
	if !ok {
		return fmt.Errorf("release %s does not contain %s", release.TagName, assetName)
	}
	checksumsAsset, ok := findReleaseAsset(release.Assets, "checksums.txt")
	if !ok {
		return fmt.Errorf("release %s does not contain checksums.txt", release.TagName)
	}

	checksums, err := downloadReleaseAsset(client, checksumsAsset)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyAssetDigest(checksums, checksumsAsset.Digest); err != nil {
		return fmt.Errorf("verify checksums asset: %w", err)
	}
	expectedChecksum, err := checksumForAsset(checksums, assetName)
	if err != nil {
		return err
	}
	binary, err := downloadReleaseAsset(client, binaryAsset)
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	if err := verifyChecksum(binary, expectedChecksum); err != nil {
		return fmt.Errorf("verify %s: %w", assetName, err)
	}
	if err := verifyAssetDigest(binary, binaryAsset.Digest); err != nil {
		return fmt.Errorf("verify GitHub asset digest: %w", err)
	}

	executablePath, err := updateExecutable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executablePath); resolveErr == nil {
		executablePath = resolved
	}
	if err := installCLIUpdate(executablePath, binary, latest); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Updated brosettlement CLI from %s to %s\n", Version, latest)
	return nil
}

func fetchLatestRelease(client *http.Client, apiURL string) (githubRelease, error) {
	request, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := client.Do(request)
	if err != nil {
		return githubRelease{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&releases); err != nil {
		return githubRelease{}, fmt.Errorf("decode release: %w", err)
	}
	for _, release := range releases {
		if !release.Draft && !release.Prerelease && strings.HasPrefix(release.TagName, "cli-v") {
			return release, nil
		}
	}
	return githubRelease{}, fmt.Errorf("no published cli-v release found")
}

func downloadReleaseAsset(client *http.Client, asset releaseAsset) ([]byte, error) {
	if !strings.HasPrefix(asset.BrowserDownloadURL, "https://github.com/BroLabel/brosettlement-agent-skills/releases/download/") &&
		!strings.HasPrefix(asset.BrowserDownloadURL, "http://127.0.0.1:") {
		return nil, fmt.Errorf("refusing unexpected asset URL %q", asset.BrowserDownloadURL)
	}
	request, err := http.NewRequest(http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asset server returned HTTP %d", response.StatusCode)
	}
	const maximumAssetSize = 100 << 20
	limited := io.LimitReader(response.Body, maximumAssetSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maximumAssetSize {
		return nil, fmt.Errorf("asset exceeds 100 MiB limit")
	}
	return data, nil
}

func installCLIUpdate(executablePath string, binary []byte, expectedVersion string) error {
	directory := filepath.Dir(executablePath)
	candidate, err := os.CreateTemp(directory, ".brosettlement-update-*")
	if err != nil {
		return fmt.Errorf("create update beside executable: %w", err)
	}
	candidatePath := candidate.Name()
	defer os.Remove(candidatePath)
	if _, err := candidate.Write(binary); err != nil {
		candidate.Close()
		return fmt.Errorf("write update: %w", err)
	}
	if err := candidate.Sync(); err != nil {
		candidate.Close()
		return fmt.Errorf("sync update: %w", err)
	}
	if err := candidate.Close(); err != nil {
		return fmt.Errorf("close update: %w", err)
	}
	mode := os.FileMode(0o755)
	if current, statErr := os.Stat(executablePath); statErr == nil {
		mode = current.Mode().Perm()
	}
	if err := os.Chmod(candidatePath, mode); err != nil {
		return fmt.Errorf("make update executable: %w", err)
	}
	if err := verifyDownloadedCLI(candidatePath, expectedVersion); err != nil {
		return fmt.Errorf("verify downloaded CLI: %w", err)
	}

	backupPath := executablePath + ".previous"
	_ = os.Remove(backupPath)
	if err := os.Rename(executablePath, backupPath); err != nil {
		return fmt.Errorf("prepare executable replacement: %w", err)
	}
	if err := os.Rename(candidatePath, executablePath); err != nil {
		_ = os.Rename(backupPath, executablePath)
		return fmt.Errorf("install update; previous executable restored: %w", err)
	}
	_ = os.Remove(backupPath)
	return nil
}

func verifyCLIExecutable(path, expectedVersion string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "version", "--json").Output()
	if err != nil {
		return err
	}
	var info versionInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return err
	}
	if info.Version != expectedVersion {
		return fmt.Errorf("binary reports version %q, expected %q", info.Version, expectedVersion)
	}
	return nil
}

func platformAssetName(goos, goarch string) (string, error) {
	if goos != "darwin" && goos != "linux" && goos != "windows" {
		return "", fmt.Errorf("automatic CLI updates are not supported on %s/%s", goos, goarch)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", fmt.Errorf("automatic CLI updates are not supported on %s/%s", goos, goarch)
	}
	name := "brosettlement-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

func findReleaseAsset(assets []releaseAsset, name string) (releaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func checksumForAsset(checksums []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			if len(fields[0]) != sha256.Size*2 {
				return "", fmt.Errorf("invalid checksum length for %s", assetName)
			}
			if _, err := hex.DecodeString(fields[0]); err != nil {
				return "", fmt.Errorf("invalid checksum for %s", assetName)
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt does not contain %s", assetName)
}

func verifyChecksum(data []byte, expected string) error {
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if actual != strings.ToLower(expected) {
		return fmt.Errorf("SHA-256 mismatch: got %s", actual)
	}
	return nil
}

func verifyAssetDigest(data []byte, digest string) error {
	if digest == "" {
		return nil
	}
	algorithm, expected, found := strings.Cut(digest, ":")
	if !found || algorithm != "sha256" {
		return fmt.Errorf("unsupported asset digest %q", digest)
	}
	return verifyChecksum(data, expected)
}

func versionFromTag(tag string) (string, error) {
	if !strings.HasPrefix(tag, "cli-v") {
		return "", fmt.Errorf("latest release tag %q is not a CLI release", tag)
	}
	version := strings.TrimPrefix(tag, "cli-v")
	if _, err := parseSemanticVersion(version); err != nil {
		return "", fmt.Errorf("invalid CLI release tag %q: %w", tag, err)
	}
	return version, nil
}

func isNewerVersion(latest, current string) (bool, error) {
	latestParts, err := parseSemanticVersion(latest)
	if err != nil {
		return false, fmt.Errorf("invalid latest version: %w", err)
	}
	if current == "dev" || current == "unknown" || strings.HasSuffix(current, "-dev") {
		return true, nil
	}
	currentParts, err := parseSemanticVersion(current)
	if err != nil {
		return false, fmt.Errorf("invalid installed version %q: %w", current, err)
	}
	for index := range latestParts {
		if latestParts[index] != currentParts[index] {
			return latestParts[index] > currentParts[index], nil
		}
	}
	return false, nil
}

func parseSemanticVersion(version string) ([3]int, error) {
	version = strings.TrimPrefix(version, "v")
	if base, _, found := strings.Cut(version, "-"); found {
		version = base
	}
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("expected MAJOR.MINOR.PATCH")
	}
	var parsed [3]int
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return [3]int{}, fmt.Errorf("invalid numeric component %q", part)
		}
		parsed[index] = value
	}
	return parsed, nil
}
