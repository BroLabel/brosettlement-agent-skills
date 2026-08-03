#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
go_dir="$script_dir/go"
output_dir="$go_dir/bin"
output_name=brosettlement
case "$(go env GOOS)" in
  windows) output_name=brosettlement.exe ;;
esac

version=${BROSETTLEMENT_CLI_BUILD_VERSION:-0.0.0-dev}
commit=${BROSETTLEMENT_CLI_BUILD_COMMIT:-bundled}
ldflags="-s -w -X github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/brocli.Version=$version -X github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/brocli.Commit=$commit"

mkdir -p "$output_dir"
(
  cd "$go_dir"
  go build -trimpath -ldflags "$ldflags" -o "$output_dir/$output_name" ./cmd/brosettlement
)
printf '%s\n' "$output_dir/$output_name"
