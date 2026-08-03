package brocli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

const usage = `BroSettlement Integration API CLI (staging by default)

Usage:
  brosettlement version [--json]
  brosettlement update [--auto]
  brosettlement commands [QUERY] [--json]
  brosettlement api METHOD TARGET [--body-file FILE] [--idempotency-key KEY] [--confirm]
  brosettlement mpc status
  brosettlement mpc initialize --confirm [--idempotency-key KEY]
  brosettlement websocket listen [--log-path FILE] [--stop-after DURATION]

Credentials:
  BROSETTLEMENT_API_KEY_ID
  BROSETTLEMENT_API_PRIVATE_KEY_FILE

Run "brosettlement <command> --help" for command options.
`

var errHelp = errors.New("help requested")

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	var err error
	switch strings.ToLower(args[0]) {
	case "version":
		err = runVersion(args[1:], stdout, stderr)
	case "update":
		err = runUpdate(args[1:], stdout, stderr)
	case "commands":
		err = runCommands(args[1:], stdout, stderr)
	case "api":
		err = runAPI(args[1:], stdout, stderr)
	case "mpc":
		err = runMPC(args[1:], stdout, stderr)
	case "websocket", "ws":
		err = runWebSocket(args[1:], stdout, stderr)
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if errors.Is(err, errHelp) || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "brosettlement: %v\n", err)
		return 1
	}
	return 0
}
