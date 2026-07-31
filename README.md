# BroSettlement Agent Skills

[![CI](https://github.com/BroLabel/brosettlement-agent-skills/actions/workflows/ci.yml/badge.svg)](https://github.com/BroLabel/brosettlement-agent-skills/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

AI agent skills and a signed Go CLI for BroSettlement onboarding, API integration, and WebSocket events.

> [!IMPORTANT]
> The bundled API configuration targets the BroSettlement **staging environment**. A production
> endpoint is intentionally not defined until Bro Label publishes and verifies it.

## Included skills

| Skill | Purpose |
|---|---|
| [`brosettlement-onboarding`](brosettlement-onboarding/) | Guides a user from account access through manual API-key creation, Co-Signer installation, MPC initialization, readiness checks, and the first testnet wallet. |
| [`brosettlement-api`](brosettlement-api/) | Discovers the current Swagger contract, sends Ed25519-signed REST requests, lists API operations, and listens to WebSocket events. |

The onboarding skill uses the API skill for every signed request and remote status check. API-key
creation remains a manual user action in BroSettlement Console.

## Install with an AI agent

Copy this prompt into an AI coding agent that supports skills:

```text
Install both BroSettlement Agent Skills from
https://github.com/BroLabel/brosettlement-agent-skills.

First inspect both SKILL.md files and the bundled scripts. Ask me which agent skills directory
to use, then install brosettlement-onboarding and brosettlement-api as sibling folders without
overwriting existing skills. Validate both skills after installation. Never ask me to paste a
private key, password, JWT, TOTP code, or API secret into chat.
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

Preview removal of both skills, then repeat with explicit confirmation:

```bash
./uninstall.sh --target /absolute/path/to/agent-skills --all
./uninstall.sh --target /absolute/path/to/agent-skills --all --confirm
```

Use `--skill brosettlement-onboarding` or `--skill brosettlement-api` to remove one skill. The
uninstaller recognizes only installations created by `install.sh` and refuses to remove modified
files unless both `--force-modified` and `--confirm` are supplied. AI agents must never uninstall
or force-remove a modified skill without an explicit user request.

## Unified Go CLI

The API skill includes one CLI for contract discovery, signed REST calls, MPC operations, and
WebSocket events:

```bash
cd brosettlement-api/scripts/go
go build -o brosettlement ./cmd/brosettlement

./brosettlement commands wallets --json
./brosettlement api GET /api/v1/mpc/status
./brosettlement mpc status
./brosettlement websocket listen --stop-after 30s
```

State-changing REST methods require explicit confirmation:

```bash
./brosettlement mpc initialize \
  --idempotency-key '<stable-key-for-this-initialization>' \
  --confirm
```

The signed commands load credentials at runtime:

```bash
export BROSETTLEMENT_API_KEY_ID='<api-key-uuid>'
export BROSETTLEMENT_API_PRIVATE_KEY_FILE='/absolute/secure/path/private.pem'
```

Never commit the private key or paste its contents into chat, source files, command arguments, or
logs.

## Development

Requirement: Go 1.24 or later.

Run the complete local verification:

```bash
make check
```

The current integration contract is available in the
[staging Swagger UI](https://brosettlement-staging-api.brolabel.io/swagger-integration#/).
Fetch the live contract before changing endpoints, schemas, scopes, signing rules, or error
handling.

## Security

Read [SECURITY.md](SECURITY.md) before reporting a vulnerability. Do not publish credentials,
signed WebSocket URLs, private keys, or exploit details in a public issue.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for validation and pull request
requirements.

## License

Licensed under the [Apache License 2.0](LICENSE).
