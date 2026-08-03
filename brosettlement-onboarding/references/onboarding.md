# BroSettlement onboarding reference

Use the current public documentation when network access is available:

- Quickstart: https://www.brolabel.io/en/api-reference/quickstart
- Co-Signer: https://www.brolabel.io/en/api-reference/co-signer
- API reference: https://www.brolabel.io/en/api-reference/api-overview
- Companion skill: `../brosettlement-api/SKILL.md`
- Source repository: https://github.com/BroLabel/brosettlement-mpc-co-signer

## API execution rule

Use the companion `$brosettlement-api` skill for every signed API request and remote status
check. If named skill activation is unavailable, read `../brosettlement-api/SKILL.md` and use
its unified `brosettlement` CLI. Never reimplement the signature workflow in this onboarding
skill.

Required API checkpoints:

| Onboarding checkpoint | API operation through `brosettlement-api` | Verification |
|---|---|---|
| API key created | `brosettlement mpc status` | Signature, server-side access rules, active key, and `Read MPC` access |
| Co-Signer configured | `brosettlement api GET /api/v1/co-signer/intents/pending` | Raw Co-Signer API access; this does not prove local process health |
| Before initialization | `brosettlement mpc status` | Current key and chain state recorded |
| Initialize MPC | `brosettlement mpc initialize --idempotency-key <stable-key> --confirm` | Accepted idempotent initialization request after explicit confirmation |
| DKG monitoring | `brosettlement mpc status` | MPC key and required testnet chains reach ready states |
| Ledger account | Separate integration key: `POST /api/v1/ledger/accounts`, then `GET /api/v1/ledger/accounts/{accountId}` | Key has `wallets:create` and `accounts:read`; created resource is readable |
| Wallet | Separate integration key: `POST /api/v1/wallets`, then `GET /api/v1/wallets/{walletId}` | Key also has `wallets:read`; wallet reaches `ACTIVE` |

Fetch the current staging Swagger before each state-changing operation. Treat exact fields,
scopes, enums, and errors from Swagger as authoritative.

## API key creation is user-only

Never operate **API Keys** in BroSettlement Console. Do not create, edit, rotate, revoke, submit,
or copy an API key for the user, including through browser automation. Give the exact manual
steps and wait for the user to confirm completion. Do not ask permission to perform the Console
action.

## Wizard checkpoints

Follow these checkpoints in order:

1. BroSettlement account and organization visible in Console.
2. Admin-panel URL copied from the browser page where the user registered and can see the organization.
3. Explicit Co-Signer installation folder selected.
4. Host prerequisites confirmed.
5. Dedicated Ed25519 key pair generated without overwriting existing keys, with the complete
   public PEM shown to the user for copying into Console.
6. Dedicated API key created with the three required MPC permissions.
7. Official Co-Signer source installed, tested, and built.
8. Persistent shares storage and separate encryption key protected.
9. Co-Signer local health ready, raw Co-Signer API accessible, and Console status Online.
10. MPC initialization explicitly started through the API skill and DKG completed.
11. MPC key, Co-Signer, and testnet chain readiness verified through the API skill.
12. Separate integration API key prepared with least-privilege account and wallet scopes.
13. First ledger account and testnet wallet created and read back through the API skill.

Pause at any incomplete checkpoint. Ask only for the information required to resolve that checkpoint.

After account access is confirmed, ask the user to copy the full URL from the browser address
bar of the BroSettlement admin panel where they registered and can see their organization. Do
not phrase this as "What Console URL do you use for [organization]?" Use that URL to identify
the environment. For `app-staging.brolabel.io` or another unambiguous staging admin hostname,
set `CO_SIGNER_MONOLITH_URL` to `https://brosettlement-staging-api.brolabel.io/` automatically.
Never ask the user for the exact `CO_SIGNER_MONOLITH_URL`, and never use the admin-panel URL
itself as the API URL. For unknown or production environments, stop rather than inventing a URL.

## Prerequisites

- BroSettlement organization on the Free/Testnet plan.
- Linux or macOS host with outbound HTTPS access.
- Git, Go 1.24 or later, and OpenSSL.
- Protected storage for the API private key, share-encryption key, and encrypted MPC shares.

## 1. Generate the Ed25519 key pair

```bash
openssl genpkey -algorithm ED25519 -out private.pem
openssl pkey -in private.pem -pubout -out public.pem
chmod 600 private.pem
```

Immediately read `public.pem` and show its complete contents in a fenced `pem` block, including
both boundary lines. Tell the user to copy the entire displayed block into **Public key (PEM)**
in BroSettlement Console. Do not provide only a filename or file link. Only the public key may be
shown; keep `private.pem` on client-controlled infrastructure and never display its contents.

## 2. Instruct the user to create the Co-Signer API key

Give the user these manual BroSettlement Console instructions:

1. Open **API Keys** and create a dedicated key.
2. Copy the complete public PEM block displayed by the agent and paste it into **Public key
   (PEM)**, including the `BEGIN PUBLIC KEY` and `END PUBLIC KEY` lines.
3. Complete **IP whitelist (CIDR)**. During temporary staging/testnet onboarding, the user may
   enter `0.0.0.0/0` to accept requests from any IPv4 address and effectively bypass the IP
   restriction while testing. State clearly that this is intentionally permissive. Never use
   `0.0.0.0/0` for production; require the correct narrow public egress IP or CIDR of the server
   running the Co-Signer or API client before activating a production key.
4. Enable all three MPC permissions:
   - **Initialize MPC**
   - **Read MPC**
   - **Raw MPC co-signer**
5. Store the resulting API Key ID securely.

Stop and wait for the user to confirm completion. Do not open the authenticated Console, upload
the public key, select permissions, submit the form, or retrieve the API Key ID for the user.

## 3. Build and configure the Co-Signer

```bash
git clone https://github.com/BroLabel/brosettlement-mpc-co-signer.git
cd brosettlement-mpc-co-signer
go mod download
go test ./...
go build -o ./bin/co-signer ./cmd/co-signer

mkdir -p ./data/shares
chmod 700 ./data/shares
openssl rand -base64 32 > share-encryption.key
chmod 600 share-encryption.key
```

Required runtime variables:

| Variable | Purpose |
|---|---|
| `CO_SIGNER_MONOLITH_URL` | BroSettlement API URL. For staging, use `https://brosettlement-staging-api.brolabel.io/`. |
| `CO_SIGNER_API_KEY_ID` | Dedicated Co-Signer API key UUID. |
| `CO_SIGNER_API_PRIVATE_KEY` | Client-held Ed25519 private key. |
| `CO_SIGNER_SHARE_ENCRYPTION_KEY` | Separate secret used to encrypt MPC shares. |

Recommended explicit variables:

| Variable | Recommended value |
|---|---|
| `CO_SIGNER_SHARES_DIR` | A protected persistent directory such as `./data/shares`. |
| `CO_SIGNER_HTTP_ADDR` | `127.0.0.1:8081` unless private monitoring requires another binding. |

Inject secrets through a service manager or secret-management platform. Do not commit a populated `.env` file.

## 4. Verify health and connectivity

```bash
curl --fail http://127.0.0.1:8081/health
```

Require:

- HTTP `200`;
- local `"ready": true`;
- DKG and signing capabilities;
- successful signed access to `GET /api/v1/co-signer/intents/pending` through
  `$brosettlement-api`;
- Co-Signer status **Online** in BroSettlement Console.

## 5. Initialize MPC

Confirm the Co-Signer is **Online**, then use `$brosettlement-api` to run the guarded,
idempotent `brosettlement mpc initialize --idempotency-key <stable-key> --confirm`. Add
`--confirm` only after explicit user confirmation. Keep the Co-Signer online and poll with
`brosettlement mpc status` through the companion skill until DKG finishes.

Require all of the following before wallet creation:

- MPC key: **Active** or **Ready**;
- Co-Signer: **Online**;
- selected testnet chain: **Ready**.

## 6. Create and test the first wallet

1. Ask whether the user already has a separate integration API key and matching local private key
   with exactly `wallets:create`, `wallets:read`, and `accounts:read`. Keep the long-running
   Co-Signer key limited to its three MPC scopes.
2. If the answer is **No**, ask permission to generate a new, separate Ed25519 pair in a resolved
   protected directory. Generate it only after confirmation, use distinct integration filenames,
   and never reuse or overwrite the Co-Signer pair.
3. Show the complete integration public key in a fenced `pem` block. Tell the user to copy it into
   **Public key (PEM)** and manually create an integration API key with exactly the three required
   scopes. Never show the private key or operate the Console.
4. Wait until the user confirms the key is active and its matching credentials are available to
   `$brosettlement-api` through the approved runtime environment.
5. Treat the user's request to complete onboarding as authorization for exactly one staging/testnet
   ledger account and one linked staging/testnet wallet. Do not ask separate confirmations for
   these two tutorial resources and do not show a verbose mutation plan unless requested. The CLI
   may still receive its required `--confirm` flag under this standing authorization.
6. Create the ledger account through `$brosettlement-api`, require its returned ID, and read it
   back by ID. Only then state that creation succeeded, show the sanitized response and returned
   fields, and explain that all accounts can be listed with:

   ```text
   @brosettlement api GET '/api/v1/ledger/accounts'
   ```

   The same resources are visible in Console under **Accounts**.
7. Create a wallet linked to that account on a ready testnet chain such as TRON Nile, require its
   returned ID, read it back, and poll until it becomes **Active**. State success explicitly and
   show the sanitized response plus returned wallet ID, account ID, network, public address,
   status, and timestamps when those fields are present. List wallets later with:

   ```text
   @brosettlement api GET '/api/v1/wallets'
   @brosettlement api GET '/api/v1/ledger/accounts/<accountId>/wallets'
   ```

   Wallets are also visible in Console under **Wallets**. Never claim success when read-back or
   lifecycle verification is incomplete; return the sanitized response and explain what remains.
9. If the user wants to test a TRON Nile deposit, confirm the selected asset is returned by
   `GET /api/v1/assets`, then show the public deposit address and these resources:
   - [TRON Nile faucet](https://nileex.io/join/getJoinPage);
   - official [TRON testnet-token guide](https://developers.tron.network/docs/getting-testnet-tokens-on-tron),
     which documents community faucet alternatives;
   - [Nile explorer](https://nile.tronscan.org) for checking the public transaction.
10. Tell the user to enter only the public TRON Nile address in a faucet. Never request a seed
    phrase or private key, and never use mainnet funds for this test.
11. Record the balance and ledger state before the transfer. The user then requests the supported
    asset from a faucet or sends it from an external testnet wallet and confirms broadcast. A
    public transaction hash is helpful but optional.
12. Poll balances and ledger entries through `$brosettlement-api`. If the integration key has
    `transactions:read`, reconcile the deposit transaction; `websockets:read` may optionally
    supplement polling. Never call `POST /api/v1/transactions` to create a deposit.
13. Offer a small withdrawal as a separate, explicitly confirmed operation, only after the user
    has manually added the current required scopes to a suitable integration key.
14. Confirm the ledger and transaction lifecycle match the on-chain result.

## Storage, backup, and recovery

Back up together:

- the complete encrypted shares directory;
- the matching share-encryption key;
- the Ed25519 API private key or a tested rotation procedure.

Store encrypted shares and their encryption key in separate protected locations. Test restoration. Never edit share files or copy them between organizations.

## Troubleshooting order

1. Validate required environment variables and Ed25519 key format.
2. Validate shares-directory ownership and permissions.
3. Validate the environment-derived API URL and outbound HTTPS access.
4. Confirm private and registered public keys match.
5. Confirm all three MPC permissions.
6. Review the network and allowlist settings shown on the API-key page.
7. Confirm the API key is active.
8. Confirm MPC was explicitly initialized.
9. Review Co-Signer JSON logs without exposing secrets.
