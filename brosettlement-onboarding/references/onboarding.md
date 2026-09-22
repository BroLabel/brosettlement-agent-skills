# BroSettlement onboarding reference

Use the current public documentation when network access is available:

- Quickstart: https://www.brolabel.io/en/api-reference/quickstart
- Co-Signer: https://www.brolabel.io/en/api-reference/co-signer
- API reference: https://www.brolabel.io/en/api-reference/api-overview
- Companion skill: `../brosettlement-api/SKILL.md`
- Disaster-recovery skill: `../brosettlement-disaster-recovery/SKILL.md`
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
| DKG monitoring | `brosettlement mpc status` | MPC key and every chain selected for onboarding reach ready states |
| Ledger account | Separate integration key: one `brosettlement account create --name <name> --external-id <stable-id> --confirm` invocation | Command sends one create, reconciles only when required, and reads the exact resource back |
| Wallet | Separate integration key: one `brosettlement wallet create --account-id <id> --chain <chain> --idempotency-key <stable-key> --confirm` invocation | Command sends one create and one immediate read-back; `ACTIVE` is success |
| Explicit withdrawal | One `brosettlement withdraw` invocation | Command sends one create with stable idempotency, then one immediate read-back when an ID is returned |

Run the companion CLI update gate exactly once before the first API operation in an onboarding
session; never repeat that environment-independent gate during the session. Invoke released,
version-gated high-level account, wallet, asset, transaction, and withdrawal commands directly
without a separate authentication probe, `commands` lookup, or Swagger download. Fetch the current
Swagger once only when a required high-level command is unavailable or the server explicitly
reports contract drift, then reuse it for that environment. Treat its exact fields, scopes, enums,
and errors as authoritative. Successful operations are reusable access evidence; never repeat their
GETs or add a generic probe solely to predict access for a later mutation. The current Integration
API may not expose complete current-key scope introspection; execute an explicitly authorized exact
operation once and handle a concrete `403` instead of asking the user to reconfirm scopes
preemptively.

## API key creation is user-only

Never operate **API Keys** in BroSettlement Console. Do not create, edit, rotate, revoke, submit,
or copy an API key for the user, including through browser automation. Give the exact manual
steps and wait for the user to confirm completion. Do not ask permission to perform the Console
action.

## Wizard checkpoints

Follow these checkpoints in order:

1. BroSettlement account and organization visible in Console.
2. Production API environment selected by default, or staging selected only from explicit user-provided context.
3. Explicit Co-Signer installation folder selected.
4. Host prerequisites confirmed.
5. Dedicated Ed25519 key pair generated without overwriting existing keys, with the complete
   public PEM shown to the user for copying into Console.
6. Dedicated API key created with the three required MPC permissions.
7. Official Co-Signer source installed, tested, and built.
8. Separate primary Share B and recovery Share C storage plus the shared encryption key and stable
   key ID protected.
9. Co-Signer local health ready, raw Co-Signer API accessible, and Console status Online.
10. MPC initialization explicitly started through the API skill and DKG completed.
11. MPC key, Co-Signer, and selected-chain readiness verified through the API skill.
12. Share C backup status recorded and its active-host path checked read-only. If Share C remains,
    the exact path and critical signing-quorum security gap are reported without changing files.
13. Separate integration API key prepared with least-privilege account and wallet scopes.
14. First ledger account and selected-network wallet created and read back through the API skill.
15. Existing credential, configuration, encryption-key, and encrypted-shares paths reported with
    their purposes and recovery requirements; no backup destination requested and no files copied.

Pause at any incomplete checkpoint. Ask only for the information required to resolve that checkpoint.
After checkpoint 15 and the completion report, stop. Do not offer or schedule recurring
Co-Signer monitoring, periodic health checks, background alerts, reminders, or automations.

### Existing-installation and routine-operation fast path

Do not restart at checkpoint 1 merely because this skill is activated again. When the user requests
a routine ledger-account or wallet operation, first inspect only the resolved configuration needed
for that operation. Reuse an already established API environment, integration API Key ID reference,
matching private-key path, successful API access evidence, and ledger-account selection when those
facts are unambiguous. For a resolved post-onboarding withdrawal, route directly through
`$brosettlement-api`'s single-call `brosettlement withdraw` command; do not inspect or recheck
installation, Co-Signer, or MPC state first, and do not repeat its create or verification request.

Ask the account-access question, installation-folder question, and other setup questions only for a
new onboarding or when the corresponding fact cannot be recovered safely. Never infer a credential
value, account selection, or blockchain network from ambiguous state. Existing security gates for
user-only Console actions, MPC initialization, blockchain-mainnet mutations, and destructive
actions still apply. For a withdrawal, a current instruction that resolves the exact source wallet,
network, asset, amount, and destination is the confirmation for that one operation; do not ask a
duplicate yes/no question.

For a new onboarding, the user's request to complete onboarding or create the first testnet wallet
authorizes the linked tutorial ledger-account and wallet pair defined below. For a routine
post-onboarding request, an explicit instruction to create one specific testnet ledger account or
wallet authorizes only that exact resource; pass `--confirm` without asking the same yes/no question
again. Do not extend routine authorization to another linked resource, a different network, or a
mainnet mutation.

For a routine wallet request, resolve and use the existing ledger account; never create another
account. Use the released high-level CLI command and require only its documented scopes rather than
the full onboarding scope set; do not fetch Swagger for that fixed operation. After returning the
sanitized result, ID, status, verification outcome, and retrieval commands, stop. Do not offer
deposit/withdrawal tests or replay backup and completion checkpoints unless the user asked to resume
onboarding.

After account access is confirmed, do not ask for a Console URL and do not present an environment
choice. Select production automatically. Switch to staging only when the user explicitly says
their organization/environment is staging or voluntarily supplies an
`app-staging.brolabel.io` URL. A testnet wallet, TRON Nile, or test-asset request does not imply
staging. Do not mention staging unless the user has supplied that context.

| Console hostname | Environment | `CO_SIGNER_MONOLITH_URL` | CLI selection |
|---|---|---|---|
| `app.brolabel.io` | production (default) | `https://brosettlement-api.brolabel.io/` | `BROSETTLEMENT_ENVIRONMENT=production` or unset |
| `app-staging.brolabel.io` when user-supplied | staging | `https://brosettlement-staging-api.brolabel.io/` | `BROSETTLEMENT_ENVIRONMENT=staging` |

Never ask the user for the exact `CO_SIGNER_MONOLITH_URL`, and never use the admin-panel URL
itself as the API URL. If the user voluntarily provides another hostname, do not infer a custom
API endpoint from it; retain production unless the user explicitly identifies the environment as
staging. If a supplied URL contains a query string or sensitive parameters, retain only its
scheme and hostname and do not echo the full value.

## Prerequisites

- BroSettlement organization with access to the networks required for the selected onboarding flow.
- A supported Linux or macOS host with outbound HTTPS access, subject to the platform rules below.
- Git, Go 1.24 or later, and OpenSSL.
- Protected storage for the API private key, share-encryption key, and encrypted MPC shares.

### Co-Signer platform support

- **Linux:** the officially supported production/mainnet runtime. Run exactly one Co-Signer
  replica with active shares and lock files on one local writable Linux filesystem. Do not use
  NFS, shared storage, or active-active replicas.
- **macOS:** supported for native local builds, immutable share publication, development,
  verification, onboarding, and testnet operation. A macOS host may use the production
  BroSettlement API when the selected onboarding/testnet environment requires it, but the
  current official production/mainnet operating runbook remains Linux-only.
- **Native Windows:** unsupported. The current source does not compile into a safe native Windows
  Co-Signer and does not implement the required atomic share-publication and lifetime-locking
  behavior there. Do not attempt to bypass those checks or describe native Windows as supported.
- **WSL2 or a Linux VM on Windows:** may be used as a Linux environment for onboarding/testing
  when all preflight checks pass. Store the repository, shares, lock files, encryption key, API
  private key, and runtime configuration inside the Linux filesystem, such as WSL2's ext4 volume.
  Never place operational Co-Signer state under `/mnt/c`, on a Windows network drive, or on another
  shared mount. Prefer a dedicated Linux VM or server for production.

The API environment, blockchain network, and Co-Signer host are separate decisions. The
production API can serve either a testnet onboarding flow or an explicitly selected mainnet flow.
Do not state that onboarding forbids mainnet operations. Before a mainnet mutation, require a
production-ready Linux host, verify the selected chain is ready, show the exact operation details,
and obtain explicit user confirmation.

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
5. Create the key.

Then render this as a separate, clearly visible paragraph rather than another compact numbered
step:

**Where to find the API Key ID**

After the key is created, return to **API Keys**, find `Testnet Co-Signer`, and select **View**. On
the key details page, find the **Key ID** field near the top and click the copy button on the right
side of that field. Use the value copied from **Key ID**; do not copy the identifier from the
browser address bar. Store the copied API Key ID securely.

Stop and wait for the user to confirm completion. Do not open the authenticated Console, upload
the public key, select permissions, submit the form, or retrieve the API Key ID for the user.

## 3. Build and configure the Co-Signer

```bash
git clone https://github.com/BroLabel/brosettlement-mpc-co-signer.git
cd brosettlement-mpc-co-signer
go mod download
go test ./...
go build -o ./bin/co-signer ./cmd/co-signer

mkdir -p ./data/shares-primary ./data/shares-recovery
chmod 700 ./data/shares-primary ./data/shares-recovery
openssl rand -base64 32 > share-encryption.key
chmod 600 share-encryption.key
```

Required runtime variables:

| Variable | Purpose |
|---|---|
| `CO_SIGNER_MONOLITH_URL` | BroSettlement API URL. Production (default): `https://brosettlement-api.brolabel.io/`. Staging: `https://brosettlement-staging-api.brolabel.io/`. |
| `CO_SIGNER_API_KEY_ID` | Dedicated Co-Signer API key UUID. |
| `CO_SIGNER_API_PRIVATE_KEY` | Client-held Ed25519 private key. |
| `CO_SIGNER_SHARE_ENCRYPTION_KEY` | Separate secret used to encrypt MPC shares. |
| `CO_SIGNER_SHARE_ENCRYPTION_KEY_ID` | Stable, printable-ASCII, non-secret identifier (1–255 bytes) that binds the encryption key to both share artifacts. |

Recommended explicit variables:

| Variable | Recommended value |
|---|---|
| `CO_SIGNER_PRIMARY_SHARES_DIR` | Protected directory for operational Share B, such as `./data/shares-primary`. |
| `CO_SIGNER_RECOVERY_SHARES_DIR` | Protected temporary DKG destination for Share C, such as `./data/shares-recovery`; move the resulting artifact to offline custody after DKG. |
| `CO_SIGNER_HTTP_ADDR` | `127.0.0.1:8081` unless private monitoring requires another binding. |

The two shares directories must be absolute, distinct, and non-overlapping. Do not use the legacy
`CO_SIGNER_SHARES_DIR` variable. Both directories coexist on the Co-Signer host only for the
minimum time required to complete DKG and separate Share C. Inject secrets through a service
manager or secret-management platform. Do not commit a populated `.env` file.

### Production path and launch safety

The installation path is a local filesystem location for the Co-Signer repository and binary; it
is not a BroSettlement API URL. For a production Linux deployment, resolve and validate all paths
before writing files or starting the process:

- application and binary: prefer `/opt/brosettlement/co-signer`;
- mutable state and shares: prefer a protected subtree of `/var/lib/brosettlement/co-signer`;
- protected configuration and key-file references: prefer a protected subtree of
  `/etc/brosettlement/co-signer`.

Require production paths to be absolute and free of whitespace, control characters, and shell
metacharacters. These examples are recommendations, not permission to move or overwrite an
existing installation. If a selected production path fails validation, explain why and ask the
user to choose a safe path before continuing.

Start the executable with a process API that passes an argument vector and explicit working
directory. Do not create one interpolated command string and do not use `sh -c`. If the available
tool exposes only a shell command string, shell-quote every path and non-secret value correctly;
never put secret values on the command line. A durable production deployment must use the service
manager and topology from the official Co-Signer runbook.

If a start fails because of path parsing, first verify that no process started and no external
state changed. Report the exact non-secret path and its role—binary, working directory,
configuration, private-key file, or shares directory—then explain the corrected invocation. A
message that says only "a path with spaces" is insufficient.

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

### Co-Signer version check for readiness anomalies

Run this read-only check when local health is ready and Console is **Online**, but the remote MPC
key is missing, `keyId` is null, a previously ready chain returns `MPC_KEY_NOT_READY`, or DKG or
signing becomes incompatible after a restart:

1. Read the running version from local `/health` and record the repository commit with
   `git -C <co-signer-repository> rev-parse HEAD`. Do not rely on the health version alone.
2. Check only the official
   [BroLabel/brosettlement-mpc-co-signer](https://github.com/BroLabel/brosettlement-mpc-co-signer)
   repository. Prefer the latest GitHub Release, then the latest semantic-version tag.
3. If the official repository has no releases or tags, compare the local commit with the remote
   default branch. Describe a newer commit as a **newer upstream revision**, not as a release.
4. If the installed build is current, say so and continue diagnosis without proposing an MPC reset.
5. If a newer release, tag, or upstream revision exists, report the installed version/commit and
   the available version/commit, link the official source, summarize relevant changes when they
   are available, and ask whether the user wants to update. Do not update automatically.

Treat explicit update approval as permission to prepare and test a candidate, not permission to
discard local changes or replace the running binary. Inspect `git status` first. If the worktree is
dirty, report the modified paths and leave them untouched; never use `git reset --hard`, `git
clean`, checkout-overwrite, automatic stash, or patch deletion. Prepare the approved release, tag,
or commit in a separate checkout or Git worktree. Run `go mod download`, `go test ./...`, and build
a side-by-side candidate binary outside the active binary path.

Before replacement, inspect the official changes for share-format migrations and compatibility.
Never start an unverified candidate against the only live shares directory. If the candidate is
compatible, stop the old process gracefully, atomically select the tested binary, start it with the
same credentials and recovery material, and verify `/health`, commit, Console **Online**, pending
intents, MPC key, and chain readiness. Keep a rollback binary until verification finishes.

If the candidate reports that the existing encrypted-share format is incompatible and no official
migration is available, keep or restore the old compatible process. Tell the user that a new MPC
key is a replacement, not a migration, and may not preserve signing access to wallets created with
the old MPC key. For production, mainnet, or any wallet containing assets, stop until BroSettlement
provides an approved migration or recovery plan.

For an explicitly approved staging/testnet reset, first create a timestamped protected legacy
archive under the existing secrets root. Copy, without modifying the active files:

- the complete old encrypted-shares directory;
- its matching old share-encryption key;
- the old Co-Signer Ed25519 private and public API-key pair;
- the active runtime configuration or launcher needed to restore their paths;
- the old working binary and a non-secret manifest containing its health version and Git commit.

Set the archive directory to `700` and secret files to `600`. Verify locally that every expected
file was copied and that the archived recovery material matches the originals; do not display
secret contents or secret hashes. Report the archive's absolute path and contents by purpose. Do
not revoke or replace the existing Console API key, Ed25519 pair, share-encryption key, runtime
configuration, or shares-directory path.

After the archive is verified, ask for explicit approval to remove only the archived legacy MPC
share files from the active shares directory so the new Co-Signer can use that same directory.
Keep the share-encryption key and every other credential unchanged. Do not touch unrelated files.
Maintain a rollback plan that restores the archived share files and compatible binary if the new
initialization does not complete.

Then offer to initialize only a new MPC key. Before proposing it, use `$brosettlement-api` to
confirm that the current server contract and organization state support replacement or new
initialization. Obtain explicit confirmation and use the existing API key, Ed25519 pair,
shares-directory path, share-encryption key, and runtime configuration. State that the new MPC key
does not migrate old wallets. Recovery of an old wallet would require the archived legacy share,
the matching unchanged share-encryption key, a compatible Co-Signer version, and corresponding
server-side support; the local archive alone does not guarantee recovery.

## 5. Initialize MPC

Confirm the Co-Signer is **Online**, then use `$brosettlement-api` to run the guarded,
idempotent `brosettlement mpc initialize --idempotency-key <stable-key> --confirm`. Add
`--confirm` only after explicit user confirmation. Keep the Co-Signer online and poll with
`brosettlement mpc status` through the companion skill after `2s`, `5s`, `10s`, `15s`, and `30s`,
then at most every `30s` within a strict three-minute interactive observation window. Bound each
request timeout to the remaining window. If DKG is still non-terminal at the deadline, report
**initialization accepted; verification pending**, show the current non-secret status and key ID
when available, and return `@brosettlement mpc status` instead of waiting longer or retrying
initialization.

Require all of the following before wallet creation:

- MPC key: **Active** or **Ready**;
- Co-Signer: **Online**;
- selected chain: **Ready**.

### Post-DKG Share B / Share C custody checkpoint

Complete this checkpoint before creating an integration key, ledger account, or wallet.

1. Wait until DKG is terminal, the MPC key and selected chains are ready, and no DKG worker is
   still writing artifacts. Resolve the current MPC `keyId` and report these paths without reading
   or displaying their contents:
   - Share B: `<CO_SIGNER_PRIMARY_SHARES_DIR>/<keyId>.primary.json`;
   - Share C: `<CO_SIGNER_RECOVERY_SHARES_DIR>/<keyId>.recovery.json`.
2. Explain the signing model clearly:
   - normal BroSettlement signing is a 2-of-3 quorum using platform Share A and client Share B;
   - the running Co-Signer uses Share B only;
   - Share C does not participate in normal signing and is the client's disaster-recovery share.
3. Tell the user to make and verify a complete client-controlled backup of the exact immutable
   Share C artifact together with the original 32-byte share-encryption key and its exact key ID.
   Do not alter, decrypt, rename, re-encrypt, print, or upload the artifact. Share B also needs its
   own recoverable backup with the matching encryption key and key ID, but that backup must be in
   a different trust domain from Share C.
4. Never place Share B and Share C in the same host, filesystem, disk, vault, cloud account,
   backup set, recovery medium, or administrative/security domain. Never give either share to a
   third party or place it in chat, email, a support ticket, or a shared drive. A person or system
   with both B and C controls a signing quorum.
5. Check only whether the exact active-host `.recovery.json` path exists; do not read its contents.
   If it exists, state: **I verified that Share C is still present at `<absolute-path>`. This is a
   critical security gap because Share B and Share C together form a signing quorum. After you
   verify the independent Share C backup, you must remove this active-host copy yourself. I will
   not delete or alter any file.** Do not ask whether the agent may stop or restart the Co-Signer.
   Never copy, move, rename, archive, upload, delete, or otherwise alter the active-host Share C or
   its recovery backup.
6. Report the checkpoint as unresolved while Share C remains on the active host. If the user later
   says they removed it, perform only another read-only existence check. When it is absent, report
   that result and verify local `/health`, Console **Online**, pending intents,
   `brosettlement mpc status`, and normal Share B signing readiness. Share C being absent from the
   active host is expected and must not be treated as loss of normal signing readiness.
7. Explain disaster recovery accurately: Share B plus Share C form the client-controlled 2-of-3
   quorum and can sign without platform Share A or BroSettlement participation. For one native TRX
   or standard TRC-20 transfer on TRON Nile or mainnet, use only the sibling
   `$brosettlement-disaster-recovery` skill and its controlled ceremony. That source-only CLI
   creates, signs, saves, and broadcasts without BroSettlement API or Console. It does not support
   other chains, arbitrary contract calls, or sign-only recovery. Do not generalize this narrow
   workflow into a turnkey recovery product for every wallet or asset.

If a later DKG creates a new Share C, repeat this checkpoint for the new `keyId`. Do not claim that
onboarding is securely complete while B and C remain together on the active Co-Signer host, but do
not remediate the file on the user's behalf.

## 6. Create and test the first wallet

1. Before asking about credentials, inspect the approved runtime environment and resolved local
   configuration read-only for an integration API Key ID reference and matching private-key file
   path. Check only presence, file type, and permissions; never display the ID or private key. When
   those references exist, do not ask whether the user has a key; let the first high-level command
   verify them. A full onboarding key must have exactly `accounts:read`, `accounts:create`,
   `wallets:read`, and `wallets:create`; a routine fast-path request needs only the scopes documented
   for that released high-level command. Keep the long-running Co-Signer key limited to its three
   MPC scopes.
2. If no usable integration credentials are found, resolve distinct integration-key filenames in
   the protected secrets directory already approved for this onboarding. An explicit request to
   complete onboarding authorizes generating the separate Ed25519 pair without another yes/no only
   when that directory is already approved, both files are absent, and nothing will be overwritten
   or displaced. Ask only when the path is missing or ambiguous, a target file collides, an existing
   key may need replacement, or multiple credentials require a user choice. Never reuse or
   overwrite the Co-Signer pair.
3. Show the complete integration public key in a fenced `pem` block. Tell the user to copy it into
   **Public key (PEM)** and manually create an integration API key. For a new full onboarding,
   select exactly **Read ledger accounts** (`accounts:read`), **Create ledger accounts**
   (`accounts:create`), **Read wallets & assets** (`wallets:read`), and **Create wallets**
   (`wallets:create`). For a routine fast-path request, select only the scopes documented for the
   released high-level command; do not request `accounts:create` merely to create a wallet for an
   existing account. Never show the private key or operate the Console.
   After creation, tell the user to return to **API Keys**, select **View** for the new integration
   key, and click the copy button beside **Key ID** on its details page.
4. When a new Console key was required, wait until the user confirms it is active, the scopes
   required for the current full-onboarding or routine flow are selected, and its matching
   credentials are available to `$brosettlement-api` through the approved runtime environment.
   This user-only Console gate is mandatory. Do not ask for the same confirmation when reused
   credential references are already present in the approved runtime environment.
5. Treat the user's request to complete onboarding or create the first testnet wallet as standing
   authorization for exactly one ledger account and one linked wallet on a testnet blockchain
   network, whether the BroSettlement API environment is production or staging. Do not ask separate
   confirmations for these two tutorial resources and do not show a verbose mutation plan unless
   requested. The CLI still receives its required `--confirm` flag under this authorization. Merely
   using the production API is not a mainnet action. If the user explicitly selects a mainnet
   blockchain network, show the exact network and mutation details, verify production readiness,
   and obtain explicit confirmation immediately before each state-changing mainnet operation.
6. Run the CLI update gate once before the first API operation in this session. Invoke the released
   high-level account and wallet commands directly without a separate authentication probe,
   `commands` lookup, or Swagger download. Their responses surface authentication and scope errors;
   request only a missing scope explicitly reported by the API and do not replay a completed
   operation. Fetch live Swagger only when the installed CLI lacks the required high-level command
   or the server explicitly reports contract drift. Never repeat the environment-independent CLI
   gate for the wallet.
7. For a new full onboarding, run `brosettlement account create --name <name> --external-id
   <stable-id> --confirm` through `$brosettlement-api`. The command owns the one create, exact
   reconciliation when required, and one read-back. Do not repeat those requests. Require its
   verified account ID before stating that creation succeeded; show the sanitized response and
   returned fields, and explain that all accounts can be listed with:

   ```text
   @brosettlement api GET '/api/v1/ledger/accounts'
   ```

   The same resources are visible in Console under **Accounts**. During a routine wallet fast path,
   skip this account-create step and use the resolved existing ledger account.
8. Create a wallet linked to that account with one `brosettlement wallet create --account-id <id>
   --chain <chain> --idempotency-key <stable-key> --confirm` invocation. Use a ready testnet chain
   such as TRON Nile for the default tutorial, or the supported mainnet chain explicitly selected
   and confirmed by the user. The command owns one create and one immediate read-back; do not
   repeat either request. The current contract documents only `ACTIVE`, `DISABLED`, and `ARCHIVED`,
   so do not poll for an invented transitional status. When the read-back is **Active**, state
   success explicitly and show the sanitized response plus returned wallet ID, account ID, network,
   public address, status, and timestamps when those fields are present. List wallets later with:

   ```text
   @brosettlement api GET '/api/v1/wallets'
   @brosettlement api GET '/api/v1/ledger/accounts/<accountId>/wallets'
   ```

   Wallets are also visible in Console under **Wallets**. If read-back is unavailable, malformed,
   or returns an undocumented status, report **verification incomplete**, the sanitized create
   response, wallet ID, last status, and both retrieval commands, then return control to the user.
   If the status is **Disabled** or **Archived**, report that terminal result and do not claim
   success. Do not add another GET or WebSocket listener merely to delay the result.
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
12. Poll balances and ledger entries through `$brosettlement-api` after `5s`, `10s`, `15s`, and
    `30s`, then at most every `30s` within a strict two-minute observation window after broadcast
    is confirmed. If the integration key has `transactions:read`, reconcile the deposit
    transaction; `websockets:read` may optionally supplement polling only with an explicit finite
    `--stop-after`. Never run an unbounded listener synchronously or let it extend the current
    operation's deadline. If no final balance delta and ledger entry appear by the deadline, report
    **deposit verification pending**, include the public transaction hash when supplied, and return
    the wallet and ledger retrieval commands instead of waiting longer. Never call
    `POST /api/v1/transactions` to create a deposit.
13. Offer a small withdrawal separately. Once the user supplies or accepts the exact source wallet,
    network, asset, amount, and destination, treat that as authorization for one operation. Reuse
    the established key/environment and wallet state; do not repeat account, wallet, authentication,
    transaction-list, Co-Signer, MPC, or balance requests merely to probe permissions. Invoke
    `brosettlement withdraw` once with the resolved wallet ID, trusted atomic amount, stable
    idempotency key, and `--confirm`. The command sends the exact create once and owns the single
    immediate verification GET. Reuse the source-address-to-wallet-ID mapping and asset metadata
    learned earlier in onboarding; perform only the minimum resolution read when either is truly
    unavailable. Do not refetch Swagger for this released command unless it is unavailable or the
    API returns a contract/access error. On create `403 INSUFFICIENT_SCOPE`, request only the write
    scope required by current Swagger and stop until the user confirms the Console change. On
    create `403 IP_NOT_ALLOWED`, direct the user to the key's allowlist settings instead.
14. After an accepted create, report the command's sanitized create response, ID, and single
    verification result; never issue another GET or POST outside it. If verification is
    non-terminal or unavailable, report **withdrawal accepted; verification pending**, return the
    retrieval command, and stop. If the verification GET returned `403 INSUFFICIENT_SCOPE`, use
    the same pending wording, request only the required transaction-read scope, and never submit
    the mutation again. If the command reports `create_outcome_unknown`, say that the mutation may
    have been accepted and requires reconciliation before any retry; never submit another POST
    blindly. Do not add account, wallet, authentication, balance, Co-Signer, MPC,
    ledger-list, or WebSocket probes unless the returned transaction exposes a concrete
    inconsistency.

## Storage, backup, and recovery

At the end of normal onboarding, do not ask the user for another protected directory and do not
copy, move, archive, upload, or delete secrets. Share C separation is always user-performed; the
agent only checks and reports its presence. An explicitly approved legacy archive for an
incompatible upgrade is the only agent-managed archival exception.
Otherwise, read the resolved paths from the active configuration
and report the exact absolute location, purpose, and recovery importance of each artifact that
exists in a compact table:

- Co-Signer Ed25519 private/public PEM files;
- integration Ed25519 private/public PEM files, when a separate integration key was created;
- the matching share-encryption key and its stable key ID;
- the primary Share B directory and current immutable `.primary.json` artifact used by the active
  Co-Signer;
- the former active path of Share C and confirmation that the exact immutable `.recovery.json`
  artifact is now in separately verified client custody, without asking for or exposing its
  backup destination;
- runtime configuration or launcher files, with a warning when any contains secret material.

Explain which files are private and which public PEM files are non-secret. Recommend that the user
personally preserve the artifacts in trusted encrypted or offline storage, without performing the
copy. To move the same organization and Co-Signer to production infrastructure while retaining
normal signing access to previously created wallets, restore the same immutable Share B artifact
with its matching share-encryption key and exact key ID. Preserve Share C separately for disaster
recovery; never put it on the normal production Co-Signer. Preserve API private keys when reusing
their existing API Key IDs; API-key rotation is separate and does not recover MPC shares. Never
edit share files, mix material between organizations, co-locate B and C, or initialize a
replacement MPC key as a backup.

## Troubleshooting order

1. Validate every resolved path and the launcher's argument/quoting behavior.
2. Validate required environment variables and Ed25519 key format.
3. Validate shares-directory ownership and permissions.
4. Validate the environment-derived API URL and outbound HTTPS access.
5. Confirm private and registered public keys match.
6. Confirm all three MPC permissions.
7. Review the network and allowlist settings shown on the API-key page.
8. Confirm the API key is active.
9. For the readiness anomalies above, compare the running version and commit with the official
   GitHub repository and offer a controlled update only when a newer build exists.
10. Preserve local source changes and test any update in a separate candidate checkout.
11. If the candidate cannot read legacy shares, keep the compatible runtime, archive the old MPC
    share and matching recovery material after approval, preserve all existing credentials and
    paths, and offer only a new MPC key with the continuity warning.
12. Confirm MPC was explicitly initialized; do not reinitialize solely because remote readiness is missing.
13. Review Co-Signer JSON logs without exposing secrets.
