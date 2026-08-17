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
| DKG monitoring | `brosettlement mpc status` | MPC key and required testnet chains reach ready states |
| Ledger account | Separate integration key: `POST /api/v1/ledger/accounts`, then `GET /api/v1/ledger/accounts/{accountId}` | Key has `accounts:create` and `accounts:read`; created resource is readable |
| Wallet | Separate integration key: `POST /api/v1/wallets`, then `GET /api/v1/wallets/{walletId}` | Key also has `wallets:create` and `wallets:read`; wallet reaches `ACTIVE` |

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
8. Separate primary Share B and recovery Share C storage plus the shared encryption key and stable
   key ID protected.
9. Co-Signer local health ready, raw Co-Signer API accessible, and Console status Online.
10. MPC initialization explicitly started through the API skill and DKG completed.
11. MPC key, Co-Signer, and testnet chain readiness verified through the API skill.
12. Share C backup verified in independent client custody, Share C removed from the active
    Co-Signer, and normal Share B readiness reverified.
13. Separate integration API key prepared with least-privilege account and wallet scopes.
14. First ledger account and testnet wallet created and read back through the API skill.
15. Existing credential, configuration, encryption-key, and encrypted-shares paths reported with
    their purposes and recovery requirements; no backup destination requested and no files copied.

Pause at any incomplete checkpoint. Ask only for the information required to resolve that checkpoint.
After checkpoint 15 and the completion report, stop. Do not offer or schedule recurring
Co-Signer monitoring, periodic health checks, background alerts, reminders, or automations.

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
5. Create the key, return to the **API Keys** list, and select **View** for the new key.
6. On the key details page, locate **Key ID**, click the copy button beside it, and store the
   copied API Key ID securely.

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
| `CO_SIGNER_MONOLITH_URL` | BroSettlement API URL. For staging, use `https://brosettlement-staging-api.brolabel.io/`. |
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
`brosettlement mpc status` through the companion skill until DKG finishes.

Require all of the following before wallet creation:

- MPC key: **Active** or **Ready**;
- Co-Signer: **Online**;
- selected testnet chain: **Ready**.

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
5. Ask the user to confirm only that the independent Share C backup has been completed and
   verified; do not ask for its destination. Then instruct the user to stop the Co-Signer
   gracefully, remove the local `.recovery.json` copy from the active host, leave the configured
   recovery directory private and empty, and restart. Do not copy, move, display, upload, or
   delete the user's recovery backup. Never remove the active-host copy before backup verification.
6. After the user confirms the restart, verify local `/health`, Console **Online**, pending intents,
   `brosettlement mpc status`, and normal Share B signing readiness. Share C being absent from the
   active host after this checkpoint is expected and must not be treated as loss of normal signing
   readiness.
7. Explain disaster recovery accurately: Share B plus Share C form the client-controlled 2-of-3
   quorum and can sign without platform Share A or BroSettlement participation. For one native TRX
   transfer on TRON Nile or mainnet, use only the sibling `$brosettlement-disaster-recovery` skill
   and its controlled ceremony. That source-only CLI creates, signs, saves, and broadcasts; it does
   not support TRC-20, other chains, or sign-only recovery. Do not generalize this narrow workflow
   into a turnkey recovery product for every wallet or asset.

If a later DKG creates a new Share C, repeat this checkpoint for the new `keyId`. Do not proceed to
wallet creation while B and C remain together on the active Co-Signer host.

## 6. Create and test the first wallet

1. Ask whether the user already has a separate integration API key and matching local private key
   with exactly `accounts:read`, `accounts:create`, `wallets:read`, and `wallets:create`. Keep the long-running
   Co-Signer key limited to its three MPC scopes.
2. If the answer is **No**, ask permission to generate a new, separate Ed25519 pair in a resolved
   protected directory. Generate it only after confirmation, use distinct integration filenames,
   and never reuse or overwrite the Co-Signer pair.
3. Show the complete integration public key in a fenced `pem` block. Tell the user to copy it into
   **Public key (PEM)** and manually create an integration API key with exactly **Read ledger
   accounts** (`accounts:read`), **Create ledger accounts** (`accounts:create`), **Read wallets &
   assets** (`wallets:read`), and **Create wallets** (`wallets:create`). Never show the private key
   or operate the Console. After creation, tell the user to return to **API Keys**, select **View**
   for the new integration key, and click the copy button beside **Key ID** on its details page.
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

At the end of normal onboarding, do not ask the user for another protected directory and do not
copy, move, archive, or upload secrets. The user-performed post-DKG Share C separation above and
an explicitly approved legacy archive for an incompatible upgrade are the only exceptions.
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

1. Validate required environment variables and Ed25519 key format.
2. Validate shares-directory ownership and permissions.
3. Validate the environment-derived API URL and outbound HTTPS access.
4. Confirm private and registered public keys match.
5. Confirm all three MPC permissions.
6. Review the network and allowlist settings shown on the API-key page.
7. Confirm the API key is active.
8. For the readiness anomalies above, compare the running version and commit with the official
   GitHub repository and offer a controlled update only when a newer build exists.
9. Preserve local source changes and test any update in a separate candidate checkout.
10. If the candidate cannot read legacy shares, keep the compatible runtime, archive the old MPC
    share and matching recovery material after approval, preserve all existing credentials and
    paths, and offer only a new MPC key with the continuity warning.
11. Confirm MPC was explicitly initialized; do not reinitialize solely because remote readiness is missing.
12. Review Co-Signer JSON logs without exposing secrets.
