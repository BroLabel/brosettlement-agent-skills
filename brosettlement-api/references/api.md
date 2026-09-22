# BroSettlement Integration API reference

## Current source of truth

- Production Swagger UI: https://brosettlement-api.brolabel.io/swagger-integration
- Production OpenAPI: https://brosettlement-api.brolabel.io/swagger-integration-json
- Staging Swagger UI: https://brosettlement-staging-api.brolabel.io/swagger-integration#/
- Staging OpenAPI: https://brosettlement-staging-api.brolabel.io/swagger-integration-json
- API title: `BroSettlement Integration API`
- API version: `1.0`
- Production base URL (default): `https://brosettlement-api.brolabel.io`
- Staging base URL: `https://brosettlement-staging-api.brolabel.io`
- REST prefix: `/api/v1`
- Production endpoint and OpenAPI transaction metadata verified: 2026-09-22

The CLI defaults to production. Set `BROSETTLEMENT_ENVIRONMENT=staging` only for the staging
environment. Fetch the selected environment's OpenAPI document whenever current fields, commands,
enums, required scopes, body-hash rules, idempotency rules, or error schemas matter. Treat this
file as workflow guidance, not a replacement for the schema. Never combine one environment's
credentials with another environment's endpoints.

Within one top-level user turn, fetch that complete OpenAPI document once and reuse it for every
operation. When `$brosettlement-onboarding` is the caller, reuse the same selected-environment
document for the continuous onboarding session. Refresh only after an environment change, an
explicit refresh request, or a server response that demonstrates contract drift; do not fetch once
per endpoint. If the exact target is unknown, the reduced `commands` lister may perform one
separate discovery fetch before the single full-document fetch. Skip discovery when the target is
already known. A released, version-gated high-level CLI command is a reviewed implementation of
its fixed contract and may run directly when all required inputs are resolved. This includes
`withdraw` and the account, wallet, asset, and transaction commands advertised by the installed
CLI. Fetch live OpenAPI afterward only when the command is unavailable, an input genuinely
requires contract discovery, or an API error demonstrates that the reviewed contract must be
rechecked.

## REST authentication

Required headers:

| Header | Contract |
|---|---|
| `X-Api-Key-Id` | Lowercase RFC 4122 API key UUID. |
| `X-Api-Timestamp` | Minimal unsigned Unix timestamp in UTC seconds. |
| `X-Api-Nonce` | Unique 16–128 character value matching `[A-Za-z0-9._~-]`. |
| `X-Api-Signature` | Padded RFC 4648 Base64 Ed25519 signature, exactly 88 characters. |
| `X-Api-Body-Hash` | SHA-256 lowercase hex digest of the exact raw body when body bytes are sent. |

Canonical string:

```text
METHOD
EXACT_REQUEST_TARGET
BODY_HASH
TIMESTAMP
NONCE
API_KEY_ID
```

Use uppercase method and preserve the exact raw path and query. For requests without body bytes,
keep the third canonical line empty and omit `X-Api-Body-Hash`.

For `POST /api/v1/mpc/initialize`, send the exact two-byte JSON body `{}`. Set
`Content-Type: application/json`, `X-Api-Body-Hash`, and the canonical `BODY_HASH` line to
`44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a`. A zero-length body is not
valid for this operation.

## Idempotency

The current OpenAPI requires `X-Idempotency-Key` for:

- `POST /api/v1/transactions`
- `POST /api/v1/wallets`
- `POST /api/v1/mpc/initialize`
- `POST /api/v1/co-signer/intents/{intentId}/claim`
- `POST /api/v1/co-signer/sessions/{sessionId}/messages`

Use one unique key per logical operation and retain it until the outcome is known. After a timeout
or another unknown outcome, inspect the existing resource or status before any retry. If a retry is
still justified, send identical request bytes with the same key. Never reuse a key with different
parameters or body bytes.

## Endpoint index

### Assets

- `GET /api/v1/assets`

### Ledger accounts

- `GET /api/v1/ledger/accounts`
- `POST /api/v1/ledger/accounts`
- `GET /api/v1/ledger/accounts/{accountId}`
- `GET /api/v1/ledger/accounts/{accountId}/wallets`
- `GET /api/v1/ledger/accounts/{accountId}/balances`
- `GET /api/v1/ledger/accounts/{accountId}/transactions`

### Wallets

- `POST /api/v1/wallets`
- `GET /api/v1/wallets`
- `GET /api/v1/wallets/{walletId}`
- `GET /api/v1/wallets/{walletId}/balances`
- `GET /api/v1/wallets/{walletId}/ledger-entries`

### Transactions

- `POST /api/v1/transactions`
- `GET /api/v1/transactions`
- `GET /api/v1/transactions/{id}`

### MPC

- `POST /api/v1/mpc/initialize`
- `GET /api/v1/mpc/status`

### Co-Signer protocol

- `GET /api/v1/co-signer/intents/pending`
- `POST /api/v1/co-signer/intents/{intentId}/claim`
- `POST /api/v1/co-signer/intents/{intentId}/result`
- `POST /api/v1/co-signer/sessions/{sessionId}/messages`
- `GET /api/v1/co-signer/sessions/{sessionId}/messages`

### Audit

- `GET /api/v1/orgs/{orgId}/audit-logs`

## WebSocket

- URL: `wss://<host>/v1/ws`
- Required scope: `websockets:read`
- Canonical string:

```text
WS_CONNECT
/v1/ws
TIMESTAMP
NONCE
```

Use the current OpenAPI description and public documentation for the complete handshake fields and event contracts.

## Integration checks

- Use `amountAtomic` for transaction creation when required by the current schema.
- Read `postedBalance`, `reservedBalance`, and `availableBalance` as distinct values.
- Treat transaction status as a lifecycle; verify terminal state and on-chain details.
- Consume event IDs idempotently and reconcile events with REST resources and ledger entries.
- Inspect the documented `PublicErrorResponseDto` instead of matching only free-form messages.

## Scope evidence and withdrawal execution

The production Integration OpenAPI verified on 2026-09-22 does not expose a signed endpoint that
returns the current API key's complete scope set. It defines `withdrawals:create` for
`POST /api/v1/transactions` and `transactions:read` for transaction read endpoints. A successful
read proves only the scope required by that read; it cannot prove a mutation-only scope.

Within the same API-key/environment session, preserve evidence from successful operations. Account
and wallet creation plus their read-backs already prove working authentication, allowlist handling,
and the relevant account/wallet access. Do not repeat those GET requests or a generic authentication
probe solely because the next operation is a withdrawal.

For an exact withdrawal explicitly requested by the user:

1. Reuse the known API environment, key, source wallet, and established session checkpoints.
2. Treat the exact request as authorization and invoke `brosettlement withdraw` once with the
   resolved wallet ID, asset, destination, atomic amount, stable idempotency key, and `--confirm`;
   do not preflight `transactions:read`. The command owns the single POST and immediate GET.
3. On `403 INSUFFICIENT_SCOPE`, identify `withdrawals:create` from the current Swagger, give the
   user manual Console instructions, and stop. After the user confirms the access change, retry
   only the identical request with the same key; do not insert another authentication probe.
4. On `403 IP_NOT_ALLOWED`, direct the user to the key's allowlist settings instead of requesting a
   scope.
5. On an accepted create, report its sanitized response and ID, then make one immediate read of that
   transaction by ID. If it is non-terminal, report **withdrawal accepted; verification pending**,
   return the retrieval command, and stop without polling. If the GET receives
   `403 INSUFFICIENT_SCOPE`, use the same pending wording and request only `transactions:read`.
   Do not resubmit the transaction. WebSocket observation is optional and must never delay this
   result.
6. If the create outcome is unknown, inspect the returned transaction ID first when one is
   available. When no ID or other lookup handle was returned, the only safe fallback is one replay
   of the exact same target and body bytes with the same idempotency key; never generate a new key.

If a future selected-environment Swagger exposes a signed current-key introspection endpoint, use
it once and compare its returned scopes with the operation requirements instead of probing
unrelated business endpoints.

## Bundled Go CLI

Run from `scripts/go`:

| Command | Purpose |
|---|---|
| `go run ./cmd/brosettlement commands [QUERY] [--json]` | Fetch Swagger and list or search current API operations. |
| `go run ./cmd/brosettlement api METHOD TARGET [options]` | Sign and send an exact REST request; mutations require `--confirm`. |
| `go run ./cmd/brosettlement account create\|show [options]` | Create one account with external-ID reconciliation, or read one account. |
| `go run ./cmd/brosettlement wallet create\|show\|resolve [options]` | Create and verify a wallet, read its views in parallel, or resolve an exact address. |
| `go run ./cmd/brosettlement asset show --chain CHAIN --asset ASSET` | Resolve exact asset metadata across cursor pages. |
| `go run ./cmd/brosettlement transaction status\|wait --id ID [options]` | Read once or wait sequentially for a bounded transaction lifecycle. |
| `go run ./cmd/brosettlement withdraw [options]` | Submit one exact withdrawal and perform one immediate read-back in the same process. |
| `go run ./cmd/brosettlement mpc status` | Read the current MPC and chain readiness status. |
| `go run ./cmd/brosettlement mpc initialize --confirm [options]` | Send the verified staging-compatible idempotent initialization request. |
| `go run ./cmd/brosettlement websocket listen --stop-after 30s [options]` | Run a bounded synchronous listener, reconnect as needed, and record events as JSONL. |

`POST /api/v1/wallets` and `POST /api/v1/transactions` require an explicit stable
`--idempotency-key`; the CLI does not generate a throwaway key for either create. This preserves
the exact logical request for scope-fix retries and unknown-outcome reconciliation.

The guarded high-level form is:

```bash
go run ./cmd/brosettlement withdraw \
  --wallet-id '<wallet-id>' \
  --asset USDT \
  --to '<destination>' \
  --amount-atomic '6000000' \
  --idempotency-key '<stable-key>' \
  --confirm
```

Other guarded high-level forms are:

```bash
go run ./cmd/brosettlement account create \
  --name 'Treasury' --external-id 'treasury-001' --confirm

go run ./cmd/brosettlement wallet create \
  --account-id '<account-id>' --chain tron:nile \
  --idempotency-key '<stable-key>' --confirm

go run ./cmd/brosettlement wallet show --wallet-id '<wallet-id>' --asset USDT
go run ./cmd/brosettlement wallet resolve --address '<address>' --chain tron:nile
go run ./cmd/brosettlement asset show --chain tron:nile --asset USDT
go run ./cmd/brosettlement transaction status --id '<transaction-id>'
go run ./cmd/brosettlement transaction wait --id '<transaction-id>' --timeout 30s
```

`account create` has no server idempotency contract. Its high-level command therefore requires a
stable external ID, sends one POST only, and uses exact external-ID lookup plus read-back to
reconcile conflicts or uncertain outcomes. `wallet create` requires an explicit stable
idempotency key, never substitutes a generated throwaway key, and performs one immediate read-back
without polling an undocumented wallet state. Resolver commands exhaust cursor pages before
deciding uniqueness. Transaction waiting is sequential, uses a fresh signature per GET, and has a
hard two-minute limit. No high-level command performs unrelated access probes.
For structured lifecycle commands, process exit success only means that the command completed and
emitted JSON. Callers must inspect `state`, `verificationPending`, `outcomeUnknown`, and terminal
flags; pending, timed-out, or uncertain JSON must stop the workflow and must not trigger a blind
mutation retry.

Optional `--client-reference` and TRON `--fee-limit-sun` values are included only when explicitly
resolved. The command accepts only the official production or staging API origin, uses one HTTP
client, never retries the POST, performs at most one GET by the returned transaction ID, and
returns structured create and verification responses plus stage timings. An accepted POST remains
accepted when the GET is non-terminal or fails; the caller must report verification pending and
must not repeat the mutation. A transport failure, response-read failure, or HTTP 5xx after the
create attempt returns `state: "create_outcome_unknown"` and `outcomeUnknown: true`; treat this as
a non-retry result and reconcile before considering any further create request.

Build a reusable binary after creating `./bin`:

```bash
mkdir -p ./bin
go build -o ./bin/brosettlement ./cmd/brosettlement
```

The older `list-commands`, `api-request`, and `ws-listener` entry points remain for compatibility.

The signed tools read:

- `BROSETTLEMENT_API_KEY_ID`
- `BROSETTLEMENT_API_PRIVATE_KEY_FILE`
- `BROSETTLEMENT_ENVIRONMENT` (`production` by default; set `staging` explicitly when needed)

Keep the private key file outside the skill and source repository.

The CLI version gate and selected-environment OpenAPI fetch are session prerequisites, not
per-operation prerequisites. Run each at most once per top-level user turn. A read-only
authentication probe is conditional: run it only when no successful operation in the current
API-key/environment session already establishes access. During a continuous onboarding session,
reuse each successful result across account creation, wallet creation, their read-backs, and an
optional follow-up withdrawal. Repeat a prerequisite only when its inputs change or a relevant
server error invalidates it.

WebSocket checks stop after `30s` by default. For synchronous checks, keep that default or pass an
explicit `--stop-after` of no more than `2m`. Pass `--follow` only when the user explicitly requests
an unbounded listener and it runs as a separately managed background process rather than blocking
the current turn.

## Controlled test ladder

1. Invoke a released, successfully version-gated high-level account, wallet, asset, transaction, or
   withdrawal command directly when all of its required inputs are resolved. Do not add a
   `commands` lookup, Swagger download, or authentication probe around it. For an operation without
   such a command, fetch the current integration OpenAPI document once for the selected environment
   and reuse it for this turn or continuous onboarding session.
2. For a generic operation, select it from that one contract and record method, exact target,
   required scope, body schema, success response, body-hash rule, and idempotency rule. The released
   high-level commands already encapsulate this review for their fixed operations.
3. Confirm API Key ID and the local private-key file path. Request scope/settings confirmation only
   for a new or changed key, or after a current access error; reuse successful session evidence
   instead of asking again.
4. For a generic mutation only, if no successful operation in the current key/environment session
   already establishes access, run one applicable read-only probe to validate the signature,
   timestamp, nonce, key status, and allowlist. Reuse that result unless a relevant configuration
   change or server error invalidates it. Never add a probe merely to predict a mutation-only scope.
   After a user fixes the exact scope named by `INSUFFICIENT_SCOPE`, retry that failed operation
   directly rather than inserting an unrelated probe.
5. Show a redacted mutation plan and obtain confirmation unless the current user instruction itself
   unambiguously authorizes the exact operation or a calling skill has captured explicit standing
   authorization. The onboarding tutorial may
   use this for one ledger account and one linked **testnet blockchain wallet** against either the
   production or staging API environment. Pass the CLI's `--confirm` flag under that authorization
   without another question. Keep the plan concise and never extend this exception to MPC
   initialization, a future withdrawal, signing, a mainnet blockchain wallet or operation,
   destructive actions, or additional resources. An explicit request containing the exact source,
   destination, asset, amount, and network authorizes only that one withdrawal and requires no
   duplicate confirmation. Using the production API does not itself mean mainnet.
6. Send the mutation once. Use a caller-supplied stable idempotency key whenever the contract
   supports or requires it. Account creation has no server idempotency contract, so its high-level
   command instead requires a stable external ID and never repeats the POST. Use the corresponding
   high-level command so its mutation and fixed read-back stay within one process.
7. Let a high-level command own its documented read-back; do not repeat it outside the command. For
   a generic operation, verify the resulting resource or lifecycle through REST and, when relevant,
   bounded WebSocket observation. Return
   the sanitized API response and identifiers, and claim terminal success only after read-back
   succeeds. If the mutation was accepted but read-back lacks its read scope, report the accepted
   result and verification as pending, ask only for that read scope, and never repeat the mutation.
   For a withdrawal, the required verification is one immediate documented GET for the returned
   transaction ID. If it is non-terminal, report the accepted withdrawal as verification pending,
   return the retrieval command, and stop. Additional balance, ledger-list, wallet, Co-Signer, MPC,
   or WebSocket checks are optional and must not block the result unless that transaction exposes a
   concrete inconsistency.
8. If the outcome is uncertain, inspect status before retrying; do not switch payloads or
   idempotency keys blindly. If the create returned no lookup handle, replay the identical bytes at
   most once with the same idempotency key so the server can return the existing logical result.

## Diagnostic order

| Symptom | Check first |
|---|---|
| Signature rejected | Six canonical lines, exact raw query, API Key ID final line, timestamp skew, nonce grammar, padded Base64, and matching key pair. |
| Body hash mismatch | Exact serialized bytes and `Content-Type`; for MPC initialization require the exact `{}` bytes and their documented hash. |
| `INSUFFICIENT_SCOPE` | Request only the exact failed-operation scope from current Swagger: the transaction-create scope for a rejected withdrawal POST, or the transaction-read scope for a rejected verification GET. |
| `IP_NOT_ALLOWED` | Direct the user to the current API-key allowlist settings; do not request another scope. |
| Validation or idempotency error | Current operation schema, required headers, stable logical-request key, and whether the key was reused with different bytes. |
| Timeout or unknown mutation result | Read the returned resource/status first. If no lookup handle exists, replay identical bytes at most once with the same idempotency key; never create a new logical request. |
| WebSocket authentication failure | Separate `WS_CONNECT` canonical, fresh timestamp and nonce, `websockets:read`, URL encoding, and API-key page access settings. |

Do not rotate credentials, change the body, or generate a new idempotency key as the first
troubleshooting step.
