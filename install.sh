#!/bin/sh

set -eu

usage() {
  printf '%s\n' "Usage: ./install.sh --target /absolute/path/to/agent-skills"
}

target=""
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

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
skills="brosettlement-onboarding brosettlement-api"

for skill in $skills; do
  if [ ! -f "$script_dir/$skill/SKILL.md" ]; then
    printf '%s\n' "Missing $skill/SKILL.md in repository" >&2
    exit 1
  fi
  if [ -e "$target/$skill" ]; then
    printf '%s\n' "Refusing to overwrite existing $target/$skill" >&2
    exit 1
  fi
done

mkdir -p "$target"
for skill in $skills; do
  cp -R "$script_dir/$skill" "$target/$skill"
done

printf '%s\n' "Installed BroSettlement skills into $target"
