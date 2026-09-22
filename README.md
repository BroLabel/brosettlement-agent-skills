# BroSettlement Agent Skills

[![CI](https://github.com/BroLabel/brosettlement-agent-skills/actions/workflows/ci.yml/badge.svg)](https://github.com/BroLabel/brosettlement-agent-skills/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

AI agent skills and Go tooling for BroSettlement onboarding, API integration, WebSocket events,
and client-controlled disaster recovery.

> [!IMPORTANT]
> The bundled API configuration targets BroSettlement **production** by default. It switches to
> staging only when the user explicitly says they are using staging or provides a staging Console
> or API URL. Never mix credentials, URLs, or resources between environments.

## Included skills

| Skill | Purpose |
|---|---|
| [`brosettlement-onboarding`](brosettlement-onboarding/) | Guides a user from account access through manual API-key creation, Co-Signer installation, MPC initialization, readiness checks, and the first wallet. |
| [`brosettlement-api`](brosettlement-api/) | Discovers the current Swagger contract, sends Ed25519-signed REST requests, lists API operations, and listens to WebSocket events. |
| [`brosettlement-disaster-recovery`](brosettlement-disaster-recovery/) | Runs a controlled Share B + Share C ceremony to create, threshold-sign, save, and broadcast one native TRX or standard TRC-20 recovery transfer without BroSettlement participation. |

The onboarding skill uses the API skill for every signed request and remote status check. API-key
creation remains a manual user action in BroSettlement Console. The disaster-recovery skill is an
independent break-glass workflow and does not call BroSettlement APIs.

## Install with an AI agent

Copy this prompt into an AI coding agent that supports skills:

```text
Install all three BroSettlement Agent Skills from
https://github.com/BroLabel/brosettlement-agent-skills.

First inspect all three SKILL.md files and the bundled scripts. Ask me which agent skills directory
to use, then install brosettlement-onboarding, brosettlement-api, and
brosettlement-disaster-recovery as sibling folders without overwriting existing skills. Validate
all three skills after installation. Never ask me to paste a private key, MPC share, password, JWT,
TOTP code, or API secret into chat.
```

## Manual installation

Clone the repository and provide the absolute skills directory used by your agent:

```bash
git clone https://github.com/BroLabel/brosettlement-agent-skills.git
cd brosettlement-agent-skills
./install.sh --target /absolute/path/to/agent-skills
```

The installer refuses to overwrite an existing or unrecognized skill directory. Installations
created by this script can be upgraded only when their tracked files are unchanged:

```bash
./install.sh --target /absolute/path/to/agent-skills --update
```

Preview removal of all three skills, then repeat with explicit confirmation:

```bash
./uninstall.sh --target /absolute/path/to/agent-skills --all
./uninstall.sh --target /absolute/path/to/agent-skills --all --confirm
```

Use `--skill brosettlement-onboarding`, `--skill brosettlement-api`, or
`--skill brosettlement-disaster-recovery` to remove one skill. The uninstaller recognizes only
installations created by `install.sh` and refuses to remove modified files unless both
`--force-modified` and `--confirm` are supplied. AI agents must never uninstall or force-remove a
modified skill without an explicit user request.

## Unified Go CLI

The API skill includes one CLI for contract discovery, signed REST calls, guarded withdrawals,
MPC operations, and WebSocket events:

Use the portable command surface in an AI chat:

```text
# Full command list
@brosettlement commands

# Search by topic
@brosettlement commands wallets
@brosettlement commands "ledger balance" --json

# Signed REST request
@brosettlement api GET '/api/v1/wallets'

# Submit one withdrawal and verify it once
@brosettlement withdraw --wallet-id '<wallet-id>' --asset USDT \
  --to '<destination>' --amount-atomic '6000000' \
  --idempotency-key '<stable-key>' --confirm

# MPC status
@brosettlement mpc status

# WebSocket
@brosettlement websocket listen --stop-after 30s
```

The agent resolves `@brosettlement` to the verified bundled executable. For direct shell use,
build or invoke the native path:

```bash
cd brosettlement-api
./scripts/build-cli.sh

./scripts/go/bin/brosettlement update --auto
./scripts/go/bin/brosettlement version
./scripts/go/bin/brosettlement commands wallets --json
./scripts/go/bin/brosettlement api GET /api/v1/mpc/status
./scripts/go/bin/brosettlement withdraw --wallet-id '<wallet-id>' --asset USDT \
  --to '<destination>' --amount-atomic '6000000' \
  --idempotency-key '<stable-key>' --confirm
./scripts/go/bin/brosettlement mpc status
./scripts/go/bin/brosettlement websocket listen --stop-after 30s
```

Only the compiled CLI executable self-updates. Installed skills, references, and scripts remain
unchanged. CLI releases use `cli-vMAJOR.MINOR.PATCH` tags and publish per-platform binaries plus
`checksums.txt`; the updater verifies them before atomically replacing the current executable.

Maintainers publish a CLI release by pushing an annotated semantic-version tag after the release
commit is on `main`:

```bash
git tag -a cli-v1.0.3 -m "BroSettlement CLI 1.0.3"
git push origin cli-v1.0.3
```

The release workflow tests the CLI, cross-compiles the supported platform binaries, generates
their SHA-256 checksums, and creates the GitHub Release. Do not reuse or move a published CLI tag.

State-changing REST methods require explicit authorization. When the current user message already
specifies one exact operation, such as a withdrawal with its source, network, asset, amount, and
destination, that message is the confirmation; the agent passes `--confirm` without asking the same
yes/no question again:

```bash
./scripts/go/bin/brosettlement mpc initialize \
  --idempotency-key '<stable-key-for-this-initialization>' \
  --confirm
```

Transaction creation additionally requires an explicit stable `--idempotency-key`. This lets the
agent reconcile an access error or uncertain response without risking a second logical transfer.
The high-level `withdraw` command keeps the create and one immediate verification read in a single
CLI process, reports per-stage timings, accepts only the official production or staging API
origin, and never polls or retries the mutation. Transport failures and HTTP 5xx responses are
reported as an unknown create outcome that must be reconciled instead of retried blindly.

The signed commands load credentials at runtime:

```bash
export BROSETTLEMENT_API_KEY_ID='<api-key-uuid>'
export BROSETTLEMENT_API_PRIVATE_KEY_FILE='/absolute/secure/path/private.pem'
```

Never commit the private key or paste its contents into chat, source files, command arguments, or
logs.

## Disaster recovery CLI

The disaster-recovery skill includes a separate source-only Go CLI. It supports one native TRX or
standard TRC-20 transfer on TRON Nile or TRON mainnet using the client-controlled Share B + Share C
quorum. The optional `--token-contract` selects TRC-20; omitting it selects native TRX. It does not
use BroSettlement API/Console or provide a sign-only mode.

Run it only through the controlled workflow in
[`brosettlement-disaster-recovery/SKILL.md`](brosettlement-disaster-recovery/SKILL.md). The skill
requires an explicit confirmation for the exact network, source, destination, and amount before
the CLI temporarily combines B+C, signs, saves a mode-`600` transaction JSON, and broadcasts it.
Outside the recovery ceremony, B and C must remain in separate trust and administrative domains.

## Development

Requirement: Go 1.24 or later.

Run the complete local verification:

```bash
make check
```

The default production integration contract is available in the
[production Swagger UI](https://brosettlement-api.brolabel.io/swagger-integration). Set
`BROSETTLEMENT_ENVIRONMENT=staging` to use the separate
[staging contract](https://brosettlement-staging-api.brolabel.io/swagger-integration#/).
Fetch the selected live contract before changing endpoints, schemas, scopes, signing rules, or
error handling.

## Security

Read [SECURITY.md](SECURITY.md) before reporting a vulnerability. Do not publish credentials,
signed WebSocket URLs, private keys, or exploit details in a public issue.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for validation and pull request
requirements.

## License

Licensed under the [Apache License 2.0](LICENSE).
