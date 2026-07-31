package brocli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const defaultSwaggerJSON = "https://brosettlement-staging-api.brolabel.io/swagger-integration-json"

var httpMethods = map[string]bool{
	"delete": true, "get": true, "head": true, "options": true,
	"patch": true, "post": true, "put": true,
}

type specification struct {
	Info struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

type operation struct {
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	OperationID string   `json:"operationId"`
	Tags        []string `json:"tags"`
}

type command struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Summary     string   `json:"summary,omitempty"`
	OperationID string   `json:"operationId,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func runCommands(args []string, stdout, stderr io.Writer) error {
	queryParts := make([]string, 0)
	flagArgs := make([]string, 0)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || len(flagArgs) > 0 {
			flagArgs = append(flagArgs, arg)
		} else {
			queryParts = append(queryParts, arg)
		}
	}
	flags := flag.NewFlagSet("commands", flag.ContinueOnError)
	flags.SetOutput(stderr)
	specURL := flags.String("swagger-json", defaultSwaggerJSON, "Swagger/OpenAPI JSON URL")
	asJSON := flags.Bool("json", false, "Print JSON")
	timeout := flags.Duration("timeout", 30*time.Second, "HTTP timeout")
	if err := flags.Parse(flagArgs); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected command arguments: %s", strings.Join(flags.Args(), " "))
	}
	query := strings.Join(queryParts, " ")
	request, err := http.NewRequest(http.MethodGet, *specURL, nil)
	if err != nil {
		return fmt.Errorf("create Swagger request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := newHTTPClient(*timeout).Do(request)
	if err != nil {
		return fmt.Errorf("fetch Swagger: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch Swagger: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read Swagger: %w", err)
	}
	var spec specification
	if err := json.Unmarshal(body, &spec); err != nil {
		return fmt.Errorf("parse Swagger: %w", err)
	}
	if spec.Info.Title != "BroSettlement Integration API" {
		return fmt.Errorf("unexpected Swagger title %q", spec.Info.Title)
	}

	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	commands := make([]command, 0)
	for path, pathItem := range spec.Paths {
		for method, rawOperation := range pathItem {
			method = strings.ToLower(method)
			if !httpMethods[method] {
				continue
			}
			var details operation
			if err := json.Unmarshal(rawOperation, &details); err != nil {
				return fmt.Errorf("parse %s %s: %w", method, path, err)
			}
			searchable := strings.ToLower(strings.Join([]string{
				method, path, details.Summary, details.Description,
				details.OperationID, strings.Join(details.Tags, " "),
			}, " "))
			if !containsAll(searchable, terms) {
				continue
			}
			commands = append(commands, command{
				Method: strings.ToUpper(method), Path: path, Summary: details.Summary,
				OperationID: details.OperationID, Tags: details.Tags,
			})
		}
	}
	sort.Slice(commands, func(left, right int) bool {
		if commands[left].Path == commands[right].Path {
			return commands[left].Method < commands[right].Method
		}
		return commands[left].Path < commands[right].Path
	})
	if *asJSON {
		result := map[string]interface{}{
			"source": *specURL, "title": spec.Info.Title, "version": spec.Info.Version,
			"query": query, "commands": commands,
		}
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("encode output: %w", err)
		}
		fmt.Fprintln(stdout, string(encoded))
	} else {
		fmt.Fprintf(stdout, "%s v%s — %s\n", spec.Info.Title, spec.Info.Version, *specURL)
		for _, item := range commands {
			fmt.Fprintf(stdout, "%-7s %-64s %s\n", item.Method, item.Path, item.Summary)
		}
	}
	if len(commands) == 0 {
		return fmt.Errorf("no API commands matched %q", query)
	}
	return nil
}

func containsAll(searchable string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(searchable, term) {
			return false
		}
	}
	return true
}
