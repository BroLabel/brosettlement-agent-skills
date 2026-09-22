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

const assetHelp = `Usage:
  brosettlement asset show --chain CHAIN --asset ASSET

Returns one exact asset catalog match, including decimals, token contract,
deposit/withdrawal availability, and required confirmations.`

type assetShowOutput struct {
	State      string                 `json:"state"`
	Chain      string                 `json:"chain"`
	Asset      string                 `json:"asset"`
	Definition map[string]interface{} `json:"definition"`
	Pages      int                    `json:"pages"`
	StatusCode int                    `json:"statusCode"`
	RequestID  string                 `json:"requestId,omitempty"`
	DurationMS int64                  `json:"durationMs"`
}

func runAsset(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(stdout, assetHelp)
		return errHelp
	}
	if strings.ToLower(args[0]) != "show" {
		return fmt.Errorf("unknown asset command %q", args[0])
	}
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	baseURL := environment.apiBaseURL
	chain := ""
	asset := ""
	timeout := 30 * time.Second
	flags := flag.NewFlagSet("asset show", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&baseURL, "base-url", environment.apiBaseURL, "BroSettlement API base URL")
	flags.StringVar(&chain, "chain", "", "Exact blockchain identifier")
	flags.StringVar(&asset, "asset", "", "Exact asset symbol")
	flags.DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected asset show arguments: %s", strings.Join(flags.Args(), " "))
	}
	chain = strings.TrimSpace(chain)
	asset = strings.TrimSpace(asset)
	if chain == "" {
		return fmt.Errorf("--chain is required")
	}
	if asset == "" {
		return fmt.Errorf("--asset is required")
	}
	if timeout <= 0 {
		return fmt.Errorf("--timeout must be greater than zero")
	}
	if err := validateAPIBaseURL(baseURL); err != nil {
		return err
	}
	query := url.Values{"asset": {asset}, "chain": {chain}, "limit": {"100"}}
	client := newFastPathHTTPClient(timeout)
	start := time.Now()
	matches := make([]map[string]interface{}, 0, 1)
	var lastResponse apiOutput
	pages := 0
	for page := 1; ; page++ {
		if page > 20 {
			return fmt.Errorf("asset show exceeded 20 cursor pages")
		}
		target := "/api/v1/assets?" + query.Encode()
		response, requestErr := sendSignedAPIRequest(client, baseURL, http.MethodGet, target, nil, "", false)
		if requestErr != nil {
			return requestErr
		}
		lastResponse = response
		pages = page
		if response.StatusCode != http.StatusOK {
			if outputErr := writeJSON(stdout, response, "asset show"); outputErr != nil {
				return outputErr
			}
			return fmt.Errorf("asset show: API returned HTTP %d", response.StatusCode)
		}
		items, parseErr := responseItems(response.Body)
		if parseErr != nil {
			return fmt.Errorf("asset show: %w", parseErr)
		}
		for _, item := range items {
			if objectString(item, "chain") == chain && objectString(item, "asset") == asset {
				matches = append(matches, item)
			}
		}
		responseObject, _ := response.Body.(map[string]interface{})
		nextCursor, _ := responseObject["nextCursor"].(string)
		if nextCursor == "" {
			break
		}
		query.Set("cursor", nextCursor)
	}
	if len(matches) != 1 {
		return fmt.Errorf("asset show found %d exact matches; expected exactly one", len(matches))
	}
	return writeJSON(stdout, assetShowOutput{
		State: "resolved", Chain: chain, Asset: asset, Definition: matches[0], Pages: pages,
		StatusCode: lastResponse.StatusCode, RequestID: lastResponse.RequestID,
		DurationMS: time.Since(start).Milliseconds(),
	}, "asset show")
}
