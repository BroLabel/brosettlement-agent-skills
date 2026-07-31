#!/bin/sh

set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/brosettlement-lifecycle.XXXXXX")
trap 'rm -rf "$test_root"' EXIT HUP INT TERM
target="$test_root/skills"

"$repo/install.sh" --target "$target"
test -f "$target/brosettlement-onboarding/.brosettlement-installed"
test -f "$target/brosettlement-api/.brosettlement-manifest"

"$repo/install.sh" --target "$target" --update

"$repo/uninstall.sh" --target "$target" --all
test -d "$target/brosettlement-onboarding"
test -d "$target/brosettlement-api"

printf '%s\n' "local change" >> "$target/brosettlement-api/SKILL.md"
if "$repo/install.sh" --target "$target" --update 2>/dev/null; then
  printf '%s\n' "Update unexpectedly replaced a modified skill" >&2
  exit 1
fi
if "$repo/uninstall.sh" --target "$target" --all --confirm 2>/dev/null; then
  printf '%s\n' "Uninstall unexpectedly removed a modified skill" >&2
  exit 1
fi
test -d "$target/brosettlement-onboarding"
test -d "$target/brosettlement-api"

"$repo/uninstall.sh" --target "$target" --all --force-modified --confirm
test ! -e "$target/brosettlement-onboarding"
test ! -e "$target/brosettlement-api"

"$repo/install.sh" --target "$target"
"$repo/uninstall.sh" --target "$target" --skill brosettlement-onboarding --confirm
test ! -e "$target/brosettlement-onboarding"
test -d "$target/brosettlement-api"

printf '%s\n' "Install/update/uninstall lifecycle tests passed"
