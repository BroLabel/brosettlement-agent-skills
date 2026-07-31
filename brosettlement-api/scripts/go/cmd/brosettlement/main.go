package main

import (
	"os"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/brocli"
)

func main() {
	os.Exit(brocli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
