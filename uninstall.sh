#!/bin/sh

set -eu

usage() {
  printf '%s\n' "Usage: ./uninstall.sh --target /absolute/path/to/agent-skills (--skill NAME | --all) [--confirm] [--force-modified]"
}

manifest_for() {
  directory=$1
  output=$2
  (
    cd "$directory"
    find . -type f \
      ! -name '.brosettlement-installed' \
      ! -name '.brosettlement-manifest' \
      ! -path './scripts/go/bin/*' \
      -exec cksum {} \; | LC_ALL=C sort
  ) > "$output"
}

is_unchanged_installation() {
  directory=$1
  [ -f "$directory/.brosettlement-manifest" ] || return 1

  current_manifest=$(mktemp "${TMPDIR:-/tmp}/brosettlement-manifest.XXXXXX")
  manifest_for "$directory" "$current_manifest"
  if cmp -s "$directory/.brosettlement-manifest" "$current_manifest"; then
    rm -f "$current_manifest"
    return 0
  fi

  rm -f "$current_manifest"
  return 1
}

target=""
skills=""
confirm=false
force_modified=false

while [ "$#" -gt 0 ]; do
  case "$1" in
    --target)
      if [ "$#" -lt 2 ]; then
        usage >&2
        exit 2
      fi
      target=$2
      shift 2
      ;;
    --skill)
      if [ "$#" -lt 2 ]; then
        usage >&2
        exit 2
      fi
      case "$2" in
        brosettlement-onboarding|brosettlement-api) skills=$2 ;;
        *)
          printf '%s\n' "Unknown skill: $2" >&2
          exit 2
          ;;
      esac
      shift 2
      ;;
    --all)
      skills="brosettlement-onboarding brosettlement-api"
      shift
      ;;
    --confirm)
      confirm=true
      shift
      ;;
    --force-modified)
      force_modified=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      printf '%s\n' "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done
if [ -z "$target" ] || [ -z "$skills" ]; then
  usage >&2
  exit 2
fi

case "$target" in
  /*) ;;
  *)
    printf '%s\n' "--target must be an absolute path" >&2
    exit 2
    ;;
esac

if [ "$target" = "/" ] || [ ! -d "$target" ]; then
  printf '%s\n' "--target must be an existing skills directory and cannot be /" >&2
  exit 2
fi

target=$(CDPATH= cd -- "$target" && pwd)

# Validate every destination before removing any of them.
for skill in $skills; do
  destination="$target/$skill"
  if [ -L "$destination" ] || [ ! -d "$destination" ]; then
    printf '%s\n' "Not an installed skill directory: $destination" >&2
    exit 1
  fi
  if [ ! -f "$destination/.brosettlement-installed" ] || \
     [ "$(cat "$destination/.brosettlement-installed")" != "$skill" ]; then
    printf '%s\n' "Refusing to remove an unrecognized installation: $destination" >&2
    exit 1
  fi
  if ! is_unchanged_installation "$destination"; then
    if [ "$force_modified" != true ]; then
      printf '%s\n' "Refusing to remove modified installation: $destination" >&2
      printf '%s\n' "Review it first, then rerun with --force-modified --confirm if removal is intended." >&2
      exit 1
    fi
    printf '%s\n' "WARNING: modified files will be removed from $destination" >&2
  fi
done

printf '%s\n' "BroSettlement skills selected for removal:"
for skill in $skills; do
  printf '  %s\n' "$target/$skill"
done

if [ "$confirm" != true ]; then
  printf '%s\n' "Preview only. Rerun with --confirm to remove these directories."
  exit 0
fi

for skill in $skills; do
  destination="$target/$skill"
  rm -rf -- "$destination"
  printf '%s\n' "Removed $destination"
done
