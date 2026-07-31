# BroSettlement Integration API reference

## Current source of truth

- Swagger UI: https://brosettlement-staging-api.brolabel.io/swagger-integration#/
- OpenAPI: https://brosettlement-staging-api.brolabel.io/swagger-integration-json
- API title: `BroSettlement Integration API`
- API version: `1.0`
- Staging base URL: `https://brosettlement-staging-api.brolabel.io`
- REST prefix: `/api/v1`
- Snapshot verified: 2026-07-30

Both links are staging. The production URL is intentionally not defined yet and must be updated by
the skill owner when it becomes available. Fetch the OpenAPI document whenever current fields,
commands, enums, required scopes, body-hash rules, idempotency rules, or error schemas matter.
Treat this file as workflow guidance, not a replacement for the schema.

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

Staging compatibility exception for `POST /api/v1/mpc/initialize`: Swagger marks
`X-Api-Body-Hash` required, but the verified server-compatible request uses an explicit
zero-length `application/x-www-form-urlencoded` body, an empty canonical `BODY_HASH` line, and no
`X-Api-Body-Hash` header. Do not send `{}` or a body file.

## Idempotency

The current OpenAPI requires `X-Idempotency-Key` for:

- `POST /api/v1/transactions`
- `POST /api/v1/wallets`
- `POST /api/v1/mpc/initialize`
- `POST /api/v1/co-signer/intents/{intentId}/claim`
- `POST /api/v1/co-signer/sessions/{sessionId}/messages`

Use one unique key per logical operation. After a timeout, retry the identical request with the same key. Never reuse a key with different parameters or body bytes.

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

## Bundled Go CLI

Run from `scripts/go`:

| Command | Purpose |
|---|---|
| `go run ./cmd/brosettlement commands [QUERY] [--json]` | Fetch Swagger and list or search current API operations. |
| `go run ./cmd/brosettlement api METHOD TARGET [options]` | Sign and send an exact REST request; mutations require `--confirm`. |
| `go run ./cmd/brosettlement mpc status` | Read the current MPC and chain readiness status. |
| `go run ./cmd/brosettlement mpc initialize --confirm [options]` | Send the verified staging-compatible idempotent initialization request. |
| `go run ./cmd/brosettlement websocket listen [options]` | Sign the WebSocket handshake, reconnect, and record events as JSONL. |

Build a reusable binary after creating `./bin`:

```bash
mkdir -p ./bin
go build -o ./bin/brosettlement ./cmd/brosettlement
```

The older `list-commands`, `api-request`, and `ws-listener` entry points remain for compatibility.

The signed tools read:

- `BROSETTLEMENT_API_KEY_ID`
- `BROSETTLEMENT_API_PRIVATE_KEY_FILE`

Keep the private key file outside the skill and source repository.

## Controlled test ladder

1. Fetch the current integration OpenAPI document.
2. Select the operation and record method, exact target, required scope, body schema, success
   response, body-hash rule, and idempotency rule.
3. Confirm API Key ID, local private-key file path, scope, and completion of the required
   settings shown on the API-key page without displaying secrets or asking for the user's IP.
4. Run an applicable read-only probe to validate the signature, timestamp, nonce, key status,
   and allowlist before the first mutation.
5. Show a redacted mutation plan and obtain confirmation.
6. Send the mutation once with a stable idempotency key.
7. Verify the resulting resource or lifecycle through REST and relevant WebSocket events.
8. If the outcome is uncertain, inspect status before retrying; do not switch payloads or
   idempotency keys blindly.

## Diagnostic order

| Symptom | Check first |
|---|---|
| Signature rejected | Six canonical lines, exact raw query, API Key ID final line, timestamp skew, nonce grammar, padded Base64, and matching key pair. |
| Body hash mismatch | Exact serialized bytes and `Content-Type`; for MPC initialization use the documented staging compatibility exception. |
| Forbidden request | Required operation scope, active key status, and network/access settings shown on the API-key page. |
| Validation or idempotency error | Current operation schema, required headers, stable logical-request key, and whether the key was reused with different bytes. |
| Timeout or unknown mutation result | Read the resource or status before retrying with the same idempotency key. |
| WebSocket authentication failure | Separate `WS_CONNECT` canonical, fresh timestamp and nonce, `websockets:read`, URL encoding, and API-key page access settings. |

Do not rotate credentials, change the body, or generate a new idempotency key as the first
troubleshooting step.
