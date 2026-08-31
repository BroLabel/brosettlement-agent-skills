package brocli

import (
	"fmt"
	"os"
	"strings"
)

const (
	productionAPIBaseURL   = "https://brosettlement-api.brolabel.io"
	productionSwaggerJSON  = "https://brosettlement-api.brolabel.io/swagger-integration-json"
	productionWebSocketURL = "wss://brosettlement-api.brolabel.io/v1/ws"

	stagingAPIBaseURL   = "https://brosettlement-staging-api.brolabel.io"
	stagingSwaggerJSON  = "https://brosettlement-staging-api.brolabel.io/swagger-integration-json"
	stagingWebSocketURL = "wss://brosettlement-staging-api.brolabel.io/v1/ws"
)

type environmentEndpoints struct {
	name         string
	apiBaseURL   string
	swaggerJSON  string
	webSocketURL string
}

func selectedEnvironment() (environmentEndpoints, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("BROSETTLEMENT_ENVIRONMENT")))
	switch value {
	case "", "production", "prod":
		return environmentEndpoints{
			name: "production", apiBaseURL: productionAPIBaseURL,
			swaggerJSON: productionSwaggerJSON, webSocketURL: productionWebSocketURL,
		}, nil
	case "staging", "stage":
		return environmentEndpoints{
			name: "staging", apiBaseURL: stagingAPIBaseURL,
			swaggerJSON: stagingSwaggerJSON, webSocketURL: stagingWebSocketURL,
		}, nil
	default:
		return environmentEndpoints{}, fmt.Errorf(
			"unsupported BROSETTLEMENT_ENVIRONMENT %q; use production or staging", value,
		)
	}
}
