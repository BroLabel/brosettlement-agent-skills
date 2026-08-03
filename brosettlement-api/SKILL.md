---
name: brosettlement-api
description: Build, inspect, test, and troubleshoot BroSettlement Integration API clients from the current staging Swagger contract. Use when a user asks what API commands are available, needs endpoint or schema details, wants a signed REST request, needs the WebSocket event listener, implements idempotent wallet or transaction operations, or diagnoses BroSettlement API authentication and response errors.
---

# BroSettlement API

Use the current Swagger as the source of truth:

- Swagger UI: https://brosettlement-staging-api.brolabel.io/swagger-integration#/
- Swagger JSON: https://brosettlement-staging-api.brolabel.io/swagger-integration-json

These URLs are **staging only**. Do not describe them as production and do not invent a production
URL. Continue using staging until the skill owner replaces the links with confirmed production
URLs.

Read [references/api.md](references/api.md) before designing or changing an integration.

## Credential protocol

Prefer an existing API key. Ask only for:

- the API Key ID;
- the absolute path to the matching Ed25519 private PEM already stored on the user's machine;
- confirmation that the key has the required scope and that all required settings shown on the
  API-key page are complete.

Never ask the user to paste a private key, password, JWT, TOTP code, or session token into chat.
Do not ask for the user's outbound public IP or call an IP-discovery service; direct the user to
the current network and allowlist instructions shown on the API-key page.
Never log in to Console or call API-key CRUD endpoints to create, edit, rotate, or revoke a key.
If a new key is required, generate a local Ed25519 pair only with the required confirmation,
show only the public key, and give the user manual Console instructions. Wait until the user
confirms that key creation, scopes, allowlist, and active status are complete.

Load existing credentials at runtime through `BROSETTLEMENT_API_KEY_ID` and
`BROSETTLEMENT_API_PRIVATE_KEY_FILE`. Do not copy credential values into source files, prompts,
generated examples, or shell history.

## Workflow

1. Prepare the bundled CLI and run its automatic version gate as described below. This may update
   only the compiled CLI executable; never update `SKILL.md`, references, scripts, or sibling skills.
2. Default to staging and state that explicitly.
3. Fetch the current Swagger JSON before answering endpoint, command, field, enum, scope, or error-schema questions.
4. Identify the exact operation, request schema, response schema, required scope, authentication headers, body-hash requirement, and idempotency requirement.
5. Resolve credentials through the credential protocol and verify prerequisites without printing secrets.
6. Prepare a redacted request plan: environment, method, exact target, scope, body source, idempotency behavior, and expected success response.
7. Run a safe read-only authentication probe before the first mutation when an applicable read scope exists.
8. Serialize the request body exactly once and hash the exact bytes that will be sent.
9. Build the canonical string with the exact request target, including the raw query string.
10. Ask for confirmation immediately before create, withdrawal, MPC initialization, signing, or another state-changing request.
11. Sign and send the confirmed request once with all required headers.
12. Validate the HTTP status and parse errors using the documented error schema.
13. Verify the resulting resource or lifecycle through a read endpoint and, when relevant, WebSocket events.
14. For uncertain outcomes, read the resource or status before retrying. Reuse the same idempotency key only for the identical logical request.
15. Report the operation, target environment, status, identifiers, and verification result without exposing secrets.

## Canonical signing invariants

For REST, use exactly six newline-separated fields:
`METHOD`, `EXACT_REQUEST_TARGET`, `BODY_HASH`, `TIMESTAMP`, `NONCE`, and `API_KEY_ID`.

- Preserve the raw query string exactly as sent; never strip everything after `?`.
- Include `API_KEY_ID` as the sixth line; never use the obsolete five-line format.
- Sign the same serialized body bytes that the HTTP client sends.
- Keep `X-Idempotency-Key` outside the canonical string.
- Never reuse a nonce.
- Treat `WS_CONNECT` as a separate four-line WebSocket canonical; never reuse REST signing logic.

If the bundled Go client cannot be used, implement these same invariants in the user's language
and verify a fixed timestamp/nonce test vector locally before any live mutation. Do not copy a
signing algorithm from another skill or document unless it matches the current staging Swagger
and the compatibility exceptions in this skill.

## Use the bundled CLI

Use the unified Go CLI as the default execution surface. On the first request, or when the binary
is missing, build it from the bundled source:

```bash
./scripts/build-cli.sh
```

At the start of every request that activates this skill, run exactly one automatic update check
before any BroSettlement API operation:

```bash
./scripts/go/bin/brosettlement update --auto
./scripts/go/bin/brosettlement version
```

The updater accepts only published `cli-vMAJOR.MINOR.PATCH` GitHub Releases from the official
repository, selects the current OS/architecture binary, verifies `checksums.txt` and any GitHub
SHA-256 asset digest, verifies the downloaded CLI-reported version, and atomically replaces only
the current executable. It must never pull, clone, rewrite, or update skill content.

If GitHub is unavailable, no published CLI release exists yet, the platform is unsupported, the
checksum fails, or the executable directory is not writable, report the skipped update briefly
and continue with the installed CLI when it supports the required command. Never weaken checksum
or source validation to make an update succeed.

Keep `cmd/list-commands`, `cmd/api-request`, and `cmd/ws-listener` only as legacy-compatible entry
points. Do not assemble signatures with ad hoc shell commands when the unified CLI is available.

## Answer API command questions

Use the Go command lister whenever the user asks what is available or asks for an operation by
topic. It fetches Swagger on every run.

```bash
./scripts/go/bin/brosettlement commands
./scripts/go/bin/brosettlement commands wallets
./scripts/go/bin/brosettlement commands "ledger balance" --json
```

Return matching HTTP methods, paths, and Swagger summaries. Then inspect the selected operation in
Swagger JSON before generating payloads or code. Do not answer from the bundled endpoint snapshot
when live staging Swagger is reachable.

## Send signed REST requests

Prefer the Go request client:

```bash
export BROSETTLEMENT_API_KEY_ID="<uuid>"
export BROSETTLEMENT_API_PRIVATE_KEY_FILE="/secure/path/private.pem"

./scripts/go/bin/brosettlement api GET '/api/v1/wallets'

./scripts/go/bin/brosettlement api POST '/api/v1/wallets' \
  --body-file /secure/path/create-wallet.json \
  --confirm
```

The client signs the exact target and body bytes, adds required empty-body hashes and idempotency
keys for the operations currently documented by Swagger, sends the request, and prints a
structured response. `GET`, `HEAD`, and `OPTIONS` run directly. Every other method requires
`--confirm`, which must be supplied only after the active agent has shown the redacted plan and
received explicit user confirmation.

For staging `POST /api/v1/mpc/initialize`, follow the verified server-compatible exception even
though Swagger marks `X-Api-Body-Hash` as required:

- send an explicit zero-length body with `Content-Type: application/x-www-form-urlencoded`;
- keep the canonical `BODY_HASH` line empty;
- omit `X-Api-Body-Hash`;
- never send `{}` or a body file.

This exact form returned HTTP `201` in staging. Do not retry alternative body/hash combinations
after an uncertain outcome; check `GET /api/v1/mpc/status` first and reuse the same idempotency
key only for the same logical initialization.

Use the guarded convenience commands during onboarding:

```bash
./scripts/go/bin/brosettlement mpc status
./scripts/go/bin/brosettlement mpc initialize \
  --idempotency-key '<stable-key-for-this-initialization>' \
  --confirm
```

The dependency-free Node.js header generator remains available when only signing headers are
needed:

Use `scripts/sign-request.mjs` to produce request headers from an exact method, request target, and optional body file:

```bash
BROSETTLEMENT_API_KEY_ID="<uuid>" \
BROSETTLEMENT_API_PRIVATE_KEY_FILE="/secure/path/private.pem" \
node scripts/sign-request.mjs \
  --method POST \
  --target /api/v1/wallets \
  --body-file /tmp/request.json \
  --idempotency-key "<stable-key-for-this-logical-request>"
```

Send the same bytes from `--body-file`; reformatting JSON after signing invalidates the body hash and signature.
If a documented operation requires idempotency and `--idempotency-key` is omitted, the generator
adds `req-<nonce>`. Pass an explicit stable key when preparing a request that may be retried.

## Listen to WebSocket events

Use the Go listener:

```bash
export BROSETTLEMENT_API_KEY_ID="<uuid>"
export BROSETTLEMENT_API_PRIVATE_KEY_FILE="/secure/path/private.pem"

./scripts/go/bin/brosettlement websocket listen \
  --log-path ./brosettlement_ws_listener.log
```

For a bounded smoke test:

```bash
./scripts/go/bin/brosettlement websocket listen --stop-after 30s
```

The listener uses the separate `WS_CONNECT` canonical string, reconnects after failures, and
writes structured JSON lines to stdout and a protected local log. Never log the signed WebSocket
URL because its query contains authentication material.

## Safety rules

- Never invent endpoints, fields, scopes, network identifiers, or status values.
- Never expose or request a private key when a public key, key ID, signature, or redacted diagnostic is sufficient.
- Never request a password, JWT, TOTP code, or authenticated Console session to manage API keys.
- Never create, edit, rotate, or revoke API keys for the user; API-key management is user-only.
- Never log canonical strings when they may contain sensitive query parameters.
- Treat staging and mainnet as distinct targets. Use staging unless the user explicitly authorizes a mainnet operation.
- Treat create, withdrawal, MPC initialization, and signing actions as state-changing. Confirm the intended resource and environment before executing them.
- Do not retry a mutation with a new idempotency key after an unknown outcome until the existing outcome has been checked.
- Do not claim success from an HTTP request alone; verify the resulting resource or terminal lifecycle status.

## WebSocket

REST and WebSocket signing formats differ. Use the WebSocket canonical format in the API reference and require `websockets:read`. Consume events idempotently by event ID, tolerate duplicates and reconnects, persist cursors when supported, and reconcile events against REST resources and ledger records.
