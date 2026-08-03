package brocli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
)

// Version and Commit are injected into release builds with -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
)

type versionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func runVersion(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "Print JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("version does not accept positional arguments")
	}

	info := versionInfo{Version: Version, Commit: Commit, OS: runtime.GOOS, Arch: runtime.GOARCH}
	if *asJSON {
		encoded, err := json.Marshal(info)
		if err != nil {
			return fmt.Errorf("encode version: %w", err)
		}
		fmt.Fprintln(stdout, string(encoded))
		return nil
	}
	fmt.Fprintf(stdout, "brosettlement %s (%s) %s/%s\n", info.Version, info.Commit, info.OS, info.Arch)
	return nil
}
