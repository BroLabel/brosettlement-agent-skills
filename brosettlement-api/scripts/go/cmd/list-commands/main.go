package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const defaultSwaggerJSON = "https://brosettlement-staging-api.brolabel.io/swagger-integration-json"

var httpMethods = map[string]bool{
	"delete":  true,
	"get":     true,
	"head":    true,
	"options": true,
	"patch":   true,
	"post":    true,
	"put":     true,
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

func main() {
	specURL := flag.String("swagger-json", defaultSwaggerJSON, "Swagger/OpenAPI JSON URL")
	query := flag.String("query", "", "Filter by method, path, summary, operation ID, description, or tag")
	asJSON := flag.Bool("json", false, "Print JSON")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP timeout")
	flag.Parse()

	if *query == "" && flag.NArg() > 0 {
		*query = strings.Join(flag.Args(), " ")
	}

	request, err := http.NewRequest(http.MethodGet, *specURL, nil)
	if err != nil {
		fail("create Swagger request: %v", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := (&http.Client{Timeout: *timeout}).Do(request)
	if err != nil {
		fail("fetch Swagger: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fail("fetch Swagger: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fail("read Swagger: %v", err)
	}

	var spec specification
	if err := json.Unmarshal(body, &spec); err != nil {
		fail("parse Swagger: %v", err)
	}
	if spec.Info.Title != "BroSettlement Integration API" {
		fail("unexpected Swagger title %q", spec.Info.Title)
	}

	terms := strings.Fields(strings.ToLower(strings.TrimSpace(*query)))
	commands := make([]command, 0)
	for path, pathItem := range spec.Paths {
		for method, rawOperation := range pathItem {
			method = strings.ToLower(method)
			if !httpMethods[method] {
				continue
			}
			var details operation
			if err := json.Unmarshal(rawOperation, &details); err != nil {
				fail("parse %s %s: %v", method, path, err)
			}
			searchable := strings.ToLower(strings.Join([]string{
				method,
				path,
				details.Summary,
				details.Description,
				details.OperationID,
				strings.Join(details.Tags, " "),
			}, " "))
			if !containsAll(searchable, terms) {
				continue
			}
			commands = append(commands, command{
				Method:      strings.ToUpper(method),
				Path:        path,
				Summary:     details.Summary,
				OperationID: details.OperationID,
				Tags:        details.Tags,
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
			"source":   *specURL,
			"title":    spec.Info.Title,
			"version":  spec.Info.Version,
			"query":    *query,
			"commands": commands,
		}
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fail("encode output: %v", err)
		}
		fmt.Println(string(encoded))
	} else {
		fmt.Printf("%s v%s — %s\n", spec.Info.Title, spec.Info.Version, *specURL)
		for _, command := range commands {
			fmt.Printf("%-7s %-64s %s\n", command.Method, command.Path, command.Summary)
		}
	}
	if len(commands) == 0 {
		os.Exit(1)
	}
}

func containsAll(searchable string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(searchable, term) {
			return false
		}
	}
	return true
}

func fail(format string, values ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
