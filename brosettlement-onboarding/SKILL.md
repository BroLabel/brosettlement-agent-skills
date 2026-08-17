---
name: brosettlement-onboarding
description: Run an interactive BroSettlement onboarding wizard from account access to a working testnet wallet, using the companion brosettlement-api skill for signed API operations and status verification. Use when a user wants step-by-step help creating or checking a BroSettlement account, choosing a Co-Signer installation folder, generating Ed25519 credentials, receiving manual Console instructions for user-created API keys, installing and starting the client-controlled Co-Signer, initializing MPC/DKG, verifying readiness, diagnosing Co-Signer version drift or MPC readiness anomalies, and creating the first ledger account and wallet.
---

# BroSettlement onboarding

Run the onboarding as a stateful conversation. Read
[references/onboarding.md](references/onboarding.md) before executing installation,
configuration, MPC initialization, or wallet creation.

## Required companion skill

This skill is installed together with `brosettlement-api`. Use `$brosettlement-api` for every
signed BroSettlement REST request, current Swagger lookup, response interpretation, and remote
status check.

Use the companion skill's unified `brosettlement` Go CLI for execution. Do not reconstruct API
signatures or send equivalent ad hoc HTTP requests when the CLI is available. Follow the API
skill's CLI version gate at the start of each request; it may update only the compiled executable,
never either installed skill.

If the agent cannot activate a sibling skill by name, read
`../brosettlement-api/SKILL.md` and follow its workflow directly. If the sibling folder is
missing, stop before the first API call and ask the user to reinstall the complete
BroSettlement Agent Skills bundle.

Do not duplicate or improvise authentication, body hashing, canonical signing, idempotency, API
paths, schemas, scopes, or status values inside the onboarding workflow. The onboarding skill
owns the conversation and local Co-Signer setup; the API skill owns API discovery and execution.

## API key creation boundary

Treat API key creation, editing, rotation, and revocation as user-only Console actions.

- Never create, edit, rotate, or revoke an API key for the user.
- Never operate the **API Keys** Console UI, even when browser control is available and the user
  appears to authorize it.
- Never click **Create API key**, upload or paste a public key, change an allowlist, select scopes,
  submit the form, or copy the resulting API Key ID from the Console.
- Never ask, "May I create the API key?" or offer to perform the Console steps.
- Provide the exact manual instructions, then stop and wait for the user to confirm completion.
- After confirmation, perform only permitted verification through `$brosettlement-api` using
  credentials already placed by the user in an approved runtime environment.

## Conversation protocol

- Ask one blocking question at a time.
- Do not present the entire questionnaire at once.
- Keep a private progress checklist in the conversation and resume from the first incomplete checkpoint.
- After each completed checkpoint, briefly state what was verified. Ask a question only when the
  user's input or authorization is actually required; otherwise continue automatically.
- Use the user's language for conversation, while preserving Console labels, commands, environment variables, and status values exactly.
- Perform safe read-only checks when tools are available. Ask before package installation, local
  credential generation, process startup, MPC initialization, production/mainnet activity, a
  withdrawal, or another higher-risk mutation. API key management remains user-only and is never
  an agent action.
- Treat an explicit request to complete onboarding or create the first testnet wallet as standing
  authorization for exactly one staging/testnet ledger account and one linked staging/testnet
  wallet. Do not ask separate yes/no confirmation before either tutorial create. The standing
  authorization ends after those two resources are created, or immediately if the environment,
  payload purpose, or resource count changes.
- Do not front-load a technical mutation plan for the two tutorial creates unless the user asks
  for it. Use one short progress sentence, perform the operation, verify it, and teach from the
  actual result.
- Do not offer, schedule, or create recurring Co-Signer monitoring, periodic health checks,
  background alerts, reminders, or automations as part of onboarding. After the completion report,
  stop without asking whether the user wants ongoing monitoring.
- Never claim that a step is complete without a direct result or the user's explicit confirmation.

## Required sequence

Follow this order. Do not skip ahead.

### 1. Confirm the BroSettlement account

The first response after this skill activates must ask only:

> Do you already have a BroSettlement account and access to your organization in BroSettlement Console?

If the answer is no:

1. Help the user open the official registration or invitation flow available to them.
2. Do not invent a Console or signup URL. Ask the user to use the URL supplied by BroSettlement if it is not already available in context.
3. Help with registration and sign-in when browser control is available.
4. Stop until the user confirms that the account exists and the organization is visible in Console.

If the answer is yes, confirm that the organization is visible and continue.

Then ask only:

> Please copy the link to the BroSettlement admin panel where you registered and can see your organization. Paste the full URL from your browser's address bar here.

Translate this question into the user's language. If helpful, show
`https://app-staging.brolabel.io/` as an example, not as an assumed URL. Do not ask an abstract
question such as "What BroSettlement Console URL do you use for [organization]?" If the copied
URL contains a query string, session token, or other sensitive parameters, retain only the
scheme and host. Use the admin-panel URL to determine the environment, but do not use the
admin-panel URL itself as the API URL.

### 2. Ask for the installation folder

Ask only:

> In which folder should I install the BroSettlement Co-Signer?

Require an explicit folder path. Resolve it to an absolute path before making changes. Use
`<selected-folder>/brosettlement-mpc-co-signer` as the repository directory unless the selected
folder is already the repository directory.

Before changing files:

- inspect whether the folder or repository already exists;
- preserve unrelated files;
- never overwrite an existing private key, share-encryption key, shares directory, or configuration;
- ask how to proceed if an existing installation or conflicting file is found.

### 3. Check prerequisites

Check the selected host for Git, Go 1.24 or later, OpenSSL, outbound HTTPS access, a protected
secrets location, and enough access to create the installation directory.

If a prerequisite is missing, explain it and help install or configure it before continuing.
Do not silently install system packages.

Do not ask the user for an outbound public IP address and do not call an IP-discovery service.
The API-key creation page provides the current network and allowlist instructions.

### 4. Generate the Ed25519 key pair

Create a new dedicated key pair using the Quickstart commands. Store it outside the Git
repository or in a protected ignored secrets directory. Set private-key permissions to `600`.

Before generation:

- check whether the target files exist;
- never overwrite keys;
- ask whether to use a confirmed existing Ed25519 pair or choose new filenames when a collision exists.

After generation, read `public.pem` and immediately show its complete contents to the user in a
fenced `pem` block. Include the `-----BEGIN PUBLIC KEY-----` and `-----END PUBLIC KEY-----`
lines. Tell the user to copy that entire block into the Console field labeled **Public key
(PEM)**. A public key is safe to display; the private key is not.

Do not merely link to or name the local public-key file, because the user's Console may be in a
different environment and its **Public key (PEM)** control expects pasted PEM text. Never print,
upload, or paste the private key into chat.

### 5. Instruct the user to create the Co-Signer API key

Tell the user to perform these steps manually in BroSettlement Console:

1. Open **API Keys**.
2. Select **Create API key**.
3. Use a recognizable name such as `Testnet Co-Signer`.
4. Copy the complete PEM block shown in the chat and paste it into **Public key (PEM)**, including
   its `BEGIN PUBLIC KEY` and `END PUBLIC KEY` lines.
5. Complete **IP whitelist (CIDR)**:
   - for this temporary staging/testnet onboarding, enter `0.0.0.0/0` if the user wants to accept
     requests from any IPv4 address and effectively bypass the IP restriction while testing;
   - explicitly warn that `0.0.0.0/0` is intentionally permissive and must never be used for a
     production API key;
   - for production, require the correct narrow public egress IP or CIDR of the server that runs
     the Co-Signer or API client before the key is activated.
6. Expand **MPC** and enable exactly:
   - **Initialize MPC**
   - **Read MPC**
   - **Raw MPC co-signer**
7. Create the key.
8. Return to the **API Keys** list, find the new `Testnet Co-Signer` key, and select **View**.
9. On the key details page, locate **Key ID** and click the copy button beside it. Store the
   copied **API Key ID** securely without exposing private key material.

Do not open or operate the Console for this checkpoint. Do not ask permission to do so. Ask the
user to reply when the key is created and the API Key ID, all three permissions, all required
fields shown on the page, and active status are confirmed. Pause until that explicit
confirmation. Do not ask for the user's IP address, request unrelated scopes, or ask the user to
paste private key material into chat.

Use `$brosettlement-api` to run `brosettlement mpc status`. This sends signed
`GET /api/v1/mpc/status` and verifies the API key,
signature, server-side access rules, and `Read MPC` access against the current API contract.
Keep the private key in its protected file and provide only its path through the API skill's
approved runtime environment.

### 6. Install and configure the Co-Signer

Clone the official repository into the selected folder. Download modules, run tests, and build
the executable using the Quickstart commands.

Then:

1. Create the protected persistent shares directory.
2. Generate a separate share-encryption key with `600` permissions.
3. Determine `CO_SIGNER_MONOLITH_URL` from the previously captured admin-panel environment:
   - if the admin-panel hostname is `app-staging.brolabel.io` or otherwise clearly identifies
     staging, set it to `https://brosettlement-staging-api.brolabel.io/`;
   - do not ask the user what `CO_SIGNER_MONOLITH_URL` is;
   - do not use the admin-panel URL itself as `CO_SIGNER_MONOLITH_URL`;
   - for an unknown or production environment, do not guess a mapping. Explain that the
     production API URL is not configured in this skill and stop before starting the Co-Signer.
4. Configure the required environment variables.
5. Keep secrets out of committed `.env` files, logs, command arguments, and chat.

If tests or the build fail, stop and diagnose them before startup.

### 7. Start and verify the Co-Signer

Start the Co-Signer as a long-running process using the selected installation and protected
secrets. Verify:

- local `/health` returns HTTP `200`;
- local `ready` is `true`;
- DKG and signing capabilities are enabled;
- Console reports the Co-Signer as **Online**.

Then use `$brosettlement-api` to:

1. run `brosettlement api GET /api/v1/co-signer/intents/pending` and verify that the dedicated
   key can access the raw Co-Signer API;
2. run `brosettlement mpc status` and record the current MPC key and per-chain readiness.

The pending-intents request verifies API access, not that the local process is online. Local
health alone is also insufficient. Do not proceed while Console shows **Offline**.

When local `/health` is ready and Console reports **Online**, but the remote MPC key disappears,
`keyId` is null, a previously ready chain reports `MPC_KEY_NOT_READY`, or DKG/signing behavior
becomes incompatible after a restart, perform the Co-Signer version check in the onboarding
reference before proposing MPC initialization, key replacement, or share changes. The check is
read-only and does not require confirmation. If a newer official release or upstream revision is
available, report the installed and available versions/commits and ask whether the user wants a
controlled update. Never update, rebuild, restart, or reinitialize automatically.

Treat update approval as permission to test the approved version, not permission to discard local
changes or replace the working binary. If the candidate cannot read the existing encrypted-share
format, keep the compatible Co-Signer running and follow the protected legacy-archive workflow in
the reference. After the old MPC share and its matching recovery material are archived, keep the
existing Ed25519 pair, Console API key and Key ID, shares-directory path, share-encryption key, and
runtime configuration unchanged. Offer to create only a new MPC key. Explain the wallet-continuity
impact and obtain the required confirmations before archiving active share files or initializing it.

### 8. Initialize MPC/DKG

Explain that MPC initialization is an external state-changing operation and obtain any
confirmation required by the active agent before submitting it.

1. Reconfirm Co-Signer **Online**.
2. Use `$brosettlement-api` to inspect the current `POST /api/v1/mpc/initialize` contract.
3. After explicit confirmation, run `brosettlement mpc initialize --idempotency-key <stable-key>
   --confirm`. Never add `--confirm` before the user has confirmed the redacted mutation plan.
4. Keep the Co-Signer running while DKG completes.
5. Poll with `brosettlement mpc status` through `$brosettlement-api` using reasonable backoff.
6. Monitor Co-Signer logs and Console status.

Do not restart the process, rotate the API key, replace the share-encryption key, or alter the
shares directory during DKG.

Initialization completes only when:

- the MPC key is **Active** or **Ready**;
- the Co-Signer remains **Online**;
- required testnet chains are **Ready**.

If DKG fails or expires, diagnose the terminal state before retrying. Do not create repeated
initialization attempts blindly.

### 9. Complete the Quickstart

After MPC readiness:

1. Ask only whether the user already has a separate integration API key, with its matching private
   key stored locally, that has exactly `accounts:read`, `accounts:create`, `wallets:read`, and
   `wallets:create`. Do
   not add these scopes to the long-running Co-Signer key by default.
2. If the answer is **No**, ask only:

   > May I generate a new, separate Ed25519 key pair for the integration API key in
   > `<proposed-protected-directory>`?

   Resolve the proposed path before asking. Wait for explicit confirmation. If the user declines,
   ask whether they want to choose another protected path or provide an existing integration key.
3. After confirmation, generate the separate pair with distinct filenames such as
   `integration-api-private.pem` and `integration-api-public.pem`. Apply the collision and private
   key protections from checkpoint 4. Never reuse or overwrite the Co-Signer key pair.
4. Immediately read `integration-api-public.pem` and show its complete contents in a fenced `pem`
   block, including both boundary lines. Tell the user to copy the displayed block into **Public
   key (PEM)** while manually creating a new integration API key in Console. Instruct the user to
   use a recognizable name such as `Testnet Integration`. For temporary staging/testnet use,
   explain that `0.0.0.0/0` in **IP whitelist (CIDR)** accepts any IPv4 source and bypasses the IP
   restriction; never allow it for production, where the user must enter the correct narrow public
   egress IP or CIDR. Select exactly these four Console permissions:
   - **Read ledger accounts** (`accounts:read`)
   - **Create ledger accounts** (`accounts:create`)
   - **Read wallets & assets** (`wallets:read`)
   - **Create wallets** (`wallets:create`)

   Create and activate the key. Return to the **API Keys** list, find `Testnet Integration`, select
   **View**, and click the copy button beside **Key ID** on the key details page. Place that copied
   API Key ID in the approved runtime environment. Never show the integration private key or
   operate the Console.
5. Pause until the user confirms the integration key is active, all four scopes are selected,
   and its matching credentials are available to `$brosettlement-api`. If the answer in step 1
   was **Yes**, obtain the same confirmation without generating another pair.
6. Briefly say that the first testnet ledger account is being created. Use `$brosettlement-api`
   to inspect and call `POST /api/v1/ledger/accounts`. When the user did not choose tutorial
   names, use a clear default such as `Testnet Treasury` and generate a unique `externalId`; do
   not interrupt the flow merely to approve those harmless defaults. Pass the CLI's required
   `--confirm` under the standing onboarding authorization above without asking the user again.
7. Require an account ID from the create response and verify it with
   `GET /api/v1/ledger/accounts/{accountId}`. Only after successful read-back, say explicitly
   **Ledger account created successfully.** Show the sanitized create response and a compact
   summary of the fields actually returned, such as `id`, `name`, `externalId`, organization ID,
   and timestamps. Never invent missing fields. Then teach both ways to find it later:

   ```text
   @brosettlement api GET '/api/v1/ledger/accounts'
   ```

   The same accounts are available in BroSettlement Console under **Accounts**. Continue to the
   wallet automatically; do not pause for another confirmation.
8. Briefly say that a wallet is being created for the verified account. Use
   `$brosettlement-api` to inspect and call `POST /api/v1/wallets` for a ready testnet chain such
   as **TRON Nile**, using a stable idempotency key when the current contract requires one. Pass
   `--confirm` under the same standing onboarding authorization without asking the user again.
9. Require a wallet ID from the response, verify it with `GET /api/v1/wallets/{walletId}`, and
   poll with reasonable backoff until it becomes **Active**. Then say explicitly **Wallet created
   successfully.** Show the sanitized create response and a compact summary of the fields
   actually returned, including the wallet ID, linked account ID, network/chain, public address,
   status, and timestamps when present. Never display secrets or fabricate fields. Teach how to
   retrieve it later:

   ```text
   @brosettlement api GET '/api/v1/wallets'
   @brosettlement api GET '/api/v1/ledger/accounts/<accountId>/wallets'
   ```

   The user can also open **Wallets** in Console for the wallet list and **Accounts** for the
   linked account details. If a create request is accepted but read-back or lifecycle verification
   fails, report **verification incomplete**, show the sanitized API response and returned ID,
   and do not claim successful creation.
10. Ask: **Would you like to test a small TRON Nile deposit next?** If the answer is **Yes**:
    - use `$brosettlement-api` to confirm the wallet is **Active**, its network is exactly TRON
      Nile, and the proposed asset is currently returned by `GET /api/v1/assets`;
    - show the wallet's public deposit address, the network, and the supported test asset;
    - give the user the [TRON Nile faucet](https://nileex.io/join/getJoinPage), the official
      [TRON testnet-token guide](https://developers.tron.network/docs/getting-testnet-tokens-on-tron)
      for documented community faucet alternatives, and the
      [Nile explorer](https://nile.tronscan.org) for public transaction verification;
    - explain that the user may request the supported test asset from a faucet or send it from an
      external testnet wallet. A faucet needs only the public Nile address. Never ask for or enter
      a seed phrase or private key, and never use mainnet funds;
    - snapshot the wallet balance and ledger entries, then ask the user to complete the faucet or
      external-wallet transfer and confirm when it has been broadcast. The user may provide the
      public transaction hash, but it is optional;
    - monitor the wallet balance and ledger entries with `wallets:read`. If the key also has
      `transactions:read`, reconcile the deposit transaction; if it has `websockets:read`, the
      WebSocket stream may supplement polling. Confirm the balance delta and final ledger entry.
    - do not call `POST /api/v1/transactions` for a deposit; that endpoint creates an outgoing
      transaction.
11. Offer a small withdrawal separately. Before any withdrawal, provide manual Console
    instructions for the current required scopes, wait for confirmation, and obtain explicit
    authorization for the state-changing request.
12. Reconcile completed tests with REST, optional WebSocket events, and ledger records.
13. Report the existing credential and MPC-share locations as described below. During normal
    onboarding, do not ask for a backup destination or copy, move, archive, upload, or display any
    secret. Use the protected archive exception only for an approved incompatible upgrade.

### Report secret locations and recovery requirements

At the end of onboarding, inspect the resolved configuration and print the exact absolute path of
every credential or recovery artifact that was created or selected. Group them by their actual
locations; do not imply that all files are in one directory when the shares directory is elsewhere.
Use a compact table with **Artifact**, **Absolute path**, **Purpose**, and **Recovery importance**.
For each item, explain its purpose:

- Co-Signer Ed25519 private PEM: signs Co-Signer API requests for its current API Key ID;
- Co-Signer public PEM: registered in Console and safe to display, but not a secret;
- share-encryption key: decrypts the locally stored encrypted MPC share;
- complete encrypted MPC shares directory: contains the client-controlled MPC material used to
  participate in signing for wallets created under this MPC key;
- integration Ed25519 private PEM: signs ledger, wallet, and other integration API requests for
  its current API Key ID;
- integration public PEM: registered in Console and safe to display, but not a secret;
- runtime configuration or launcher files: identify API Key IDs, endpoints, and the paths from
  which the protected values are loaded; state whether they contain secrets before listing them.

Omit artifacts that were not created or used. Never print file contents, secret values, key
fingerprints that were not already approved for display, or environment-variable values. Tell the
user to preserve these paths and make their own protected, encrypted backup in a trusted secret
manager or offline storage. Do not ask where to save it and do not perform the copy.

State clearly that moving the same BroSettlement organization and Co-Signer to production
infrastructure without losing signing access to previously created wallets requires restoring the
same complete encrypted MPC shares directory together with its matching share-encryption key.
They are a matched recovery set; losing either can make the client-held MPC share unusable. Also
preserve each API private key when the corresponding existing API Key ID will continue to be used;
an API credential can instead be rotated through the supported Console process, but that does not
replace or recover the MPC shares. Never initialize a replacement MPC key as a backup procedure.

## Operating rules

- Never request, display, transmit, log, or commit private keys, share-encryption keys, or MPC share files.
- Never operate BroSettlement Console to create, edit, rotate, or revoke an API key.
- Never ask for authorization to manage an API key; provide instructions and wait for the user.
- Never implement or send a BroSettlement API request independently when the companion API skill is available.
- Never upload the client private key or client MPC share to BroSettlement.
- Do not invent Docker images, packages, environment variables, or deployment commands. Use the current official repository and Console setup instructions.
- Do not treat local `ready: true` as end-to-end readiness. Also verify the Console heartbeat, MPC key status, and chain status.
- Do not create a wallet before MPC is ready.
- Do not change the API key, share-encryption key, or shares directory while DKG or signing is active.
- During MPC readiness anomalies, compare the running Co-Signer with the official GitHub
  repository before proposing a reset or reinitialization. Never call an untagged commit a release.
- Do not update the Co-Signer without explicit approval. Preserve the existing API credentials,
  runtime configuration, complete shares directory, and matching share-encryption key during an
  approved update; updating the executable must not initialize a replacement MPC key.
- Never run `git reset --hard`, `git clean`, discard a patch, overwrite modified files, or otherwise
  remove local Co-Signer changes as part of an update. Use a separate candidate checkout when the
  existing worktree is dirty.
- When a candidate is incompatible with legacy shares, archive the old MPC share and matching
  recovery material only after explicit approval. Reuse the existing Ed25519 pair, Console API
  key, shares-directory path, share-encryption key, and runtime configuration; offer only a new
  MPC key. Do not present it as a migration of old wallets.
- Use testnet by default. Require explicit user authorization and verified production readiness before any mainnet operation.
- Do not turn operational monitoring guidance into an onboarding checkpoint or follow-up question.
- Do not ask for a backup directory or copy secrets during normal onboarding; report the existing
  absolute paths and let the user perform their own secure backup. The only exception is the
  explicitly approved protected legacy archive required for an incompatible Co-Signer upgrade.
- Stop and explain the blocker when credentials, scopes, allowlists, plan limits, or readiness checks are incomplete.

## Completion report

At the end, report each checkpoint without exposing secrets or claiming optional tests ran:

- account and organization access confirmed;
- installation path;
- Co-Signer API key created with required MPC scopes;
- integration API key and scopes used for ledger account and wallet creation;
- Co-Signer installed and built;
- Co-Signer running version and Git commit, plus the latest-version comparison when troubleshooting required it;
- Co-Signer local health;
- Console heartbeat;
- MPC/DKG status;
- chain readiness;
- ledger account and wallet identifiers;
- test transaction status;
- exact paths and purposes of the credential files and encrypted shares, without showing values;
- the matched shares-directory and share-encryption-key recovery requirement for existing wallets.
