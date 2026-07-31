#!/bin/sh

set -eu

usage() {
  printf '%s\n' "Usage: ./install.sh --target /absolute/path/to/agent-skills [--update]"
}

manifest_for() {
  directory=$1
  output=$2
  (
    cd "$directory"
    find . -type f \
      ! -name '.brosettlement-installed' \
      ! -name '.brosettlement-manifest' \
      -exec cksum {} \; | LC_ALL=C sort
  ) > "$output"
}

write_metadata() {
  directory=$1
  skill=$2
  printf '%s\n' "$skill" > "$directory/.brosettlement-installed"
  manifest_for "$directory" "$directory/.brosettlement-manifest"
}

verify_installation() {
  directory=$1
  skill=$2

  [ ! -L "$directory" ] || return 1
  [ -f "$directory/.brosettlement-installed" ] || return 1
  [ "$(cat "$directory/.brosettlement-installed")" = "$skill" ] || return 1
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
update=false
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
    --update)
      update=true
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
if [ -z "$target" ]; then
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

if [ "$target" = "/" ]; then
  printf '%s\n' "Refusing to use the filesystem root as --target" >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
skills="brosettlement-onboarding brosettlement-api"

mkdir -p "$target"
target=$(CDPATH= cd -- "$target" && pwd)

# Validate every destination before changing any of them.
for skill in $skills; do
  if [ ! -f "$script_dir/$skill/SKILL.md" ]; then
    printf '%s\n' "Missing $skill/SKILL.md in repository" >&2
    exit 1
  fi

  destination="$target/$skill"
  if [ -e "$destination" ] || [ -L "$destination" ]; then
    if [ "$update" != true ]; then
      printf '%s\n' "Refusing to overwrite existing $destination" >&2
      printf '%s\n' "Use --update only for an unchanged installation created by this installer." >&2
      exit 1
    fi
    if ! verify_installation "$destination" "$skill"; then
      printf '%s\n' "Refusing to update unrecognized or modified installation: $destination" >&2
      exit 1
    fi
  fi
done

for skill in $skills; do
  source_directory="$script_dir/$skill"
  destination="$target/$skill"

  if [ ! -e "$destination" ]; then
    cp -R "$source_directory" "$destination"
    write_metadata "$destination" "$skill"
    printf '%s\n' "Installed $skill into $destination"
    continue
  fi

  stage=$(mktemp -d "$target/.brosettlement-install.XXXXXX")
  cp -R "$source_directory" "$stage/$skill"
  write_metadata "$stage/$skill" "$skill"
  mv "$destination" "$stage/previous"
  if mv "$stage/$skill" "$destination"; then
    rm -rf "$stage"
    printf '%s\n' "Updated $skill in $destination"
  else
    mv "$stage/previous" "$destination"
    rm -rf "$stage"
    printf '%s\n' "Failed to update $skill; restored the previous installation" >&2
    exit 1
  fi
done
