---
name: brosettlement-api
description: Build, inspect, test, and troubleshoot BroSettlement Integration API clients from the current production or staging Swagger contract. Use when a user asks what API commands are available, needs endpoint or schema details, wants a signed REST request, needs the WebSocket event listener, implements idempotent wallet or transaction operations, or diagnoses BroSettlement API authentication and response errors.
---

# BroSettlement API

Use the current Swagger for the selected environment as the source of truth. Production is the
default:

- Production Swagger UI: https://brosettlement-api.brolabel.io/swagger-integration
- Production Swagger JSON: https://brosettlement-api.brolabel.io/swagger-integration-json
- Staging Swagger UI: https://brosettlement-staging-api.brolabel.io/swagger-integration#/
- Staging Swagger JSON: https://brosettlement-staging-api.brolabel.io/swagger-integration-json

Use production unless the user explicitly selects staging or the calling onboarding skill derives
staging from `app-staging.brolabel.io`. Never mix credentials, Swagger, REST, or WebSocket endpoints
between environments.

Read [references/api.md](references/api.md) before designing or changing an integration.

## Credential protocol

Prefer an existing API key. Ask only for:

- the API Key ID;
- the absolute path to the matching Ed25519 private PEM already stored on the user's machine;
- confirmation that the key has the required scopes and that all required settings shown on the
  API-key page are complete only when the key is new, its access configuration changed, or a
  current API response proves that access is missing. Do not request duplicate scope confirmation
  after successful operations with the same key and environment already establish usable access.

Never ask the user to paste a private key, password, JWT, TOTP code, or session token into chat.
Do not ask for the user's outbound public IP or call an IP-discovery service; direct the user to
the current network and allowlist instructions shown on the API-key page.
Never log in to Console or call API-key CRUD endpoints to create, edit, rotate, or revoke a key.
If a new key is required, generate a local Ed25519 pair only with the required confirmation,
show only the public key, and give the user manual Console instructions. Wait until the user
confirms that key creation, scopes, allowlist, and active status are complete.

Load existing credentials at runtime through `BROSETTLEMENT_API_KEY_ID` and
`BROSETTLEMENT_API_PRIVATE_KEY_FILE`. Select endpoints with
`BROSETTLEMENT_ENVIRONMENT=production|staging`; an unset value means production. Do not copy
credential values into source files, prompts, generated examples, or shell history.

## Scope reuse and withdrawal fast path

Reuse successful operations for the same API Key ID and environment as session evidence. Account
and wallet creation plus read-back already establish authentication and applicable wallet access;
do not repeat generic auth, wallet, balance, transaction-list, Co-Signer, or MPC probes merely to
predict withdrawal scopes.

An exact user instruction that resolves the source wallet, network, asset, amount, and destination
authorizes that one withdrawal. Prefer the guarded `brosettlement withdraw` command, pass
`--confirm`, require an explicit stable `--idempotency-key`, and submit once without a
permission-probe GET or duplicate question. The command performs the accepted create and exactly
one immediate read-back in a single CLI invocation; do not repeat either request outside it. On
`403 INSUFFICIENT_SCOPE`,
request only the write scope named by current Swagger and stop. Treat `IP_NOT_ALLOWED` as an
allowlist issue. After an accepted create, report its sanitized response and ID, then verify only
that ID with one immediate GET. If it is non-terminal or the GET returns `403 INSUFFICIENT_SCOPE`,
report **withdrawal accepted; verification pending** and stop; request the read scope only for the
403 case. Never resubmit an accepted withdrawal, poll synchronously, or block on WebSocket
verification. See [references/api.md](references/api.md#scope-evidence-and-withdrawal-execution)
for the complete error and retry rules.

## Workflow

1. Prepare the bundled CLI and run its automatic version gate as described below. Run this gate at
   most once per top-level user turn. When this skill is called by `$brosettlement-onboarding`, run
   it once for the continuous onboarding session and reuse the result for every account, wallet,
   status, and verification call in that session. This may update only the compiled CLI executable;
   never update `SKILL.md`, references, scripts, or sibling skills.
2. Default to production and state that explicitly. Use staging only when it was explicitly
   selected or derived from the staging Console hostname.
3. Fetch the current Swagger JSON once for the selected environment and reuse that exact document
   for all endpoint, field, enum, scope, and error-schema decisions in the same user turn or
   continuous onboarding session. Fetch it again only if the environment changes, the user asks
   for a refresh, or the server reports a contract mismatch. When command discovery is necessary,
   the reduced `commands` lister may make one separate discovery fetch before the single full-schema
   fetch; skip discovery when the exact target is already known. Never refetch once per endpoint.
   A released, successfully version-gated `brosettlement withdraw` command is the reviewed contract
   implementation for its fixed create-plus-read operation. When its required inputs are already
   resolved, invoke it directly without a `commands` call or a new Swagger download. Fetch Swagger
   only if the command is unavailable, an input requires genuine contract discovery, or the server
   returns a contract/access error that must be interpreted before further action.
4. Identify the exact operation, request schema, response schema, required scope, authentication headers, body-hash requirement, and idempotency requirement.
5. Resolve credentials through the credential protocol and verify prerequisites without printing secrets.
6. Prepare a redacted request plan: environment, method, exact target, scope, body source,
   idempotency behavior, and expected success response. Keep it internal or summarize it in one
   sentence when a calling skill defines an authorized tutorial flow; show the full plan when the
   user requests it or before other mutations.
7. Run one safe read-only authentication probe before the first mutation when an applicable read
   scope exists and no successful operation in the current session already establishes access.
   Reuse a successful probe and subsequent successful operations for the same API key and
   environment throughout the user turn or continuous onboarding session. Probe again only if the
   credential, environment, or relevant access configuration changes, or an authentication/access
   error invalidates the result. Never add a probe solely to predict whether a mutation-only scope
   exists. After the user fixes a scope named by `INSUFFICIENT_SCOPE`, resume the identical failed
   operation directly; do not insert an unrelated authentication probe.
8. Serialize the request body exactly once and hash the exact bytes that will be sent.
9. Build the canonical string with the exact request target, including the raw query string.
10. Ask for confirmation immediately before a state-changing request unless a calling skill has
    already captured explicit, narrowly scoped standing authorization. In particular,
    `$brosettlement-onboarding` may authorize exactly one tutorial ledger account and one linked
    **testnet blockchain wallet** against either the production or staging API environment. Under
    that standing authorization, pass the CLI's required `--confirm` flag without asking another
    yes/no question. This exception never covers MPC initialization, withdrawal, signing, a
    mainnet blockchain wallet or operation, destructive actions, or additional resources. The
    production API environment alone is not mainnet activity. Separately, the user's current
    explicit instruction to perform one exact withdrawal is immediate authorization for that
    withdrawal; do not turn the CLI safeguard into a duplicate confirmation question.
11. Sign and send the confirmed request once with all required headers.
12. Validate the HTTP status and parse errors using the documented error schema.
13. Verify the resulting resource or lifecycle through a read endpoint and, when relevant, WebSocket events.
14. For uncertain outcomes, read the resource or status before retrying. Reuse the same idempotency key only for the identical logical request.
15. Report the operation, target environment, status, sanitized API response, identifiers, and
    verification result without exposing secrets. After a create, read the resource back and state
    success explicitly only when verification succeeds.

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
signing algorithm from another skill or document unless it matches the selected environment's
current Swagger and the current operation-specific requirements in this skill.

## Use the bundled CLI

Use the unified Go CLI as the default execution surface. On the first request, or when the binary
is missing, build it from the bundled source:

```bash
./scripts/build-cli.sh
```

Run exactly one automatic update check before the first BroSettlement API operation in a top-level
user turn. When `$brosettlement-onboarding` calls this skill repeatedly, treat the continuous
onboarding flow as one session: run the gate once, remember that it succeeded or was safely skipped,
and do not rerun it before each API call:

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

### Present commands to the user

When teaching or returning a reusable command in chat, use the portable `@brosettlement` command
surface instead of exposing the skill's internal executable path. Present commands in this form:

```text
# Full command list
@brosettlement commands

# Search by topic
@brosettlement commands wallets
@brosettlement commands "ledger balance" --json

# Signed REST request
@brosettlement api GET '/api/v1/wallets'

# One withdrawal plus one immediate verification read
@brosettlement withdraw --wallet-id '<wallet-id>' --asset USDT \
  --to '<destination>' --amount-atomic '6000000' \
  --idempotency-key '<stable-key>' --confirm

# MPC status
@brosettlement mpc status

# WebSocket
@brosettlement websocket listen --stop-after 30s
```

Treat `@brosettlement` as the user-facing invocation handled by the installed skill. When actually
executing the operation, resolve it to the bundled verified CLI at
`./scripts/go/bin/brosettlement` (or the equivalent absolute installed path). Show the native path
only for direct-shell troubleshooting, builds, updates, or when the user's agent does not support
the `@brosettlement` command surface.

## Answer API command questions

Use the Go command lister whenever the user asks what is available or asks for an operation by
topic. The command fetches Swagger on every invocation and returns only reduced command metadata,
so invoke it at most once and reuse its result when several operations are handled in the same user
turn or continuous onboarding session. Fetch the complete Swagger document once afterward only
when exact schemas, scopes, or error details are needed. Skip the lister entirely when the exact
target is already known. Do not run it separately for the tutorial account create, account
read-back, wallet create, and wallet read-back.

```bash
./scripts/go/bin/brosettlement commands
./scripts/go/bin/brosettlement commands wallets
./scripts/go/bin/brosettlement commands "ledger balance" --json
```

Return matching HTTP methods, paths, and Swagger summaries. Then inspect the selected operation in
Swagger JSON before generating payloads or code. Do not answer from the bundled endpoint snapshot
when the selected environment's live Swagger is reachable.

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
`--confirm`. Supply it only after explicit authorization: either immediate user confirmation or
narrow standing authorization defined by the calling skill. The onboarding exception covers one
tutorial ledger account and one linked **testnet blockchain wallet** in either API environment.
Pass `--confirm` automatically under that authorization; do not convert the CLI flag into another
user question. It does not remove the CLI safeguard or authorize any other mutation, especially a
mainnet blockchain wallet or operation.

For `POST /api/v1/mpc/initialize`, follow the current selected-environment operation contract
exactly. The verified production and staging contracts currently require:

- send the exact two-byte JSON body `{}` with `Content-Type: application/json`;
- set `X-Api-Body-Hash` and the canonical `BODY_HASH` line to
  `44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a`;
- never send a zero-length body or reformat the payload.

The guarded `mpc initialize` command supplies `{}` automatically. Do not retry alternative
body/hash combinations after an uncertain outcome; check `GET /api/v1/mpc/status` first and
reuse the same idempotency key only for the same logical initialization.

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
For `POST /api/v1/transactions`, `--idempotency-key` is mandatory so a rejected or uncertain
withdrawal can be reconciled safely without generating a different key. For other documented
idempotent operations, the generator adds `req-<nonce>` when the option is omitted. Pass an
explicit stable key whenever an operation may need a controlled retry.

### Fast withdrawal command

For an exact authorized withdrawal whose wallet ID and atomic amount are already resolved, use one
guarded invocation instead of separate shell or tool calls:

```text
@brosettlement withdraw --wallet-id '<wallet-id>' --asset USDT \
  --to '<destination>' --amount-atomic '6000000' \
  --idempotency-key '<stable-key>' --confirm
```

Add `--client-reference '<reference>'` only when the caller needs one. For TRON, add
`--fee-limit-sun '<sun>'` only when that exact fee limit was resolved from the current operation
context. The command serializes one `CreateTransactionDto`, sends it once, extracts the returned
transaction ID, and performs one immediate `GET /api/v1/transactions/{id}` with the same HTTP
client. It returns the create response, verification response or error, and per-stage durations as
one JSON document. It never probes permissions, balances, wallets, Co-Signer, MPC, or WebSocket;
never polls; and never retries the mutation.

The guarded command accepts only the official production or staging API origin; do not relay a
signed withdrawal through a custom host. If transport failure, response-read failure, or HTTP 5xx
makes the create result uncertain, it returns `state: "create_outcome_unknown"` with
`outcomeUnknown: true` and a non-retry exit. Do not repeat the POST blindly; reconcile the stable
idempotency key and transaction state first.

If the create is accepted but verification is non-terminal or unavailable, treat the mutation as
accepted and report **withdrawal accepted; verification pending**. Never repeat the POST. Resolve
an address to the already-known wallet ID and convert the display amount to `amountAtomic` from
trusted session or API asset metadata before invoking the command. If either value is genuinely
unknown, perform only the minimum read needed to resolve it; that resolution is not a permission
probe.

## Listen to WebSocket events

Use the Go listener. Every listener started synchronously while answering a user must be bounded;
include `--stop-after` (normally `30s`, and no more than `2m` for an interactive check):

```bash
export BROSETTLEMENT_API_KEY_ID="<uuid>"
export BROSETTLEMENT_API_PRIVATE_KEY_FILE="/secure/path/private.pem"

./scripts/go/bin/brosettlement websocket listen \
  --log-path ./brosettlement_ws_listener.log \
  --stop-after 30s
```

For a bounded smoke test:

```bash
./scripts/go/bin/brosettlement websocket listen --stop-after 30s
```

The listener uses the separate `WS_CONNECT` canonical string, reconnects after failures, and
writes structured JSON lines to stdout and a protected local log. It stops after `30s` by default.
Never run an unbounded listener as a synchronous verification step. Pass `--follow` only when the
user explicitly asks for a long-running listener and it is launched as a separately managed
background process. Never log the signed WebSocket URL because its query contains authentication
material.

## Safety rules

- Never invent endpoints, fields, scopes, network identifiers, or status values.
- Never expose or request a private key when a public key, key ID, signature, or redacted diagnostic is sufficient.
- Never request a password, JWT, TOTP code, or authenticated Console session to manage API keys.
- Never create, edit, rotate, or revoke API keys for the user; API-key management is user-only.
- Never log canonical strings when they may contain sensitive query parameters.
- Treat production and staging as distinct API environments, and testnet and mainnet as distinct
  blockchain targets. Production is the default API environment, but mainnet activity still
  requires explicit authorization.
- Treat create, withdrawal, MPC initialization, and signing actions as state-changing. Confirm the
  intended resource and environment before executing them, while accepting an unambiguous explicit
  instruction in the current user message as that confirmation. Do not ask the same yes/no question
  again.
- Do not retry a mutation with a new idempotency key after an unknown outcome until the existing outcome has been checked.
- Do not claim success from an HTTP request alone; verify the resulting resource or terminal lifecycle status.

## WebSocket

REST and WebSocket signing formats differ. Use the WebSocket canonical format in the API reference and require `websockets:read`. Consume events idempotently by event ID, tolerate duplicates and reconnects, persist cursors when supported, and reconcile events against REST resources and ledger records.
