---
name: brosettlement-onboarding
description: Run an interactive BroSettlement onboarding wizard from account access to a working wallet, using the companion brosettlement-api skill for signed API operations and status verification. Use when a user wants step-by-step help creating or checking a BroSettlement account, choosing a Co-Signer installation folder, generating Ed25519 credentials, receiving manual Console instructions for user-created API keys, installing and starting the client-controlled Co-Signer, initializing MPC/DKG, verifying readiness, diagnosing Co-Signer version drift or MPC readiness anomalies, and creating the first ledger account and wallet.
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
signatures or send equivalent ad hoc HTTP requests when the CLI is available. Run the API skill's
CLI version gate exactly once before the first API operation in an onboarding session, then reuse
that verified executable for the rest of the session, even if the selected API environment changes;
the updater is environment-independent. The gate may update only the compiled executable, never
either installed skill.

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
  credential generation unless it is covered by the narrow onboarding authorization below,
  process startup, MPC initialization, a blockchain-mainnet mutation, or another higher-risk
  mutation when the current user message has not already authorized the exact action. An explicit
  withdrawal instruction that resolves the source wallet, network, asset, amount, and destination
  is authorization for that one withdrawal; do not ask the same yes/no question again. Merely
  using the production API is not a mainnet mutation. API key management remains user-only and is
  never an agent action.
- Treat an explicit request to complete onboarding or create the first testnet wallet as standing
  authorization for exactly one ledger account and one linked wallet on a testnet blockchain
  network, whether the selected BroSettlement API environment is production or staging. Do not ask
  separate yes/no confirmation before either tutorial create. Pass the CLI's `--confirm` flag under
  this standing authorization. The standing authorization ends after those two resources are
  created, or immediately if the blockchain network, payload purpose, or resource count changes.
- Mainnet wallets and operations are supported during onboarding when the user explicitly selects
  the mainnet network. They are outside the standing testnet tutorial authorization: show the
  exact network, asset, destination, amount, fees, and state-changing action, verify production
  readiness, and obtain explicit confirmation immediately before execution. Never state that
  onboarding prohibits mainnet operations.
- Do not front-load a technical mutation plan for the two tutorial creates unless the user asks
  for it. Use one short progress sentence, perform the operation, verify it, and teach from the
  actual result.
- Do not offer, schedule, or create recurring Co-Signer monitoring, periodic health checks,
  background alerts, reminders, or automations as part of onboarding. After the completion report,
  stop without asking whether the user wants ongoing monitoring.
- Never claim that a step is complete without a direct result or the user's explicit confirmation.

## Existing-installation fast path

Do not replay the full wizard when the user asks for a routine ledger-account or wallet operation
and an existing installation may already be configured. Before asking onboarding questions, use
read-only checks to inspect the resolved installation and approved runtime configuration for:

- the selected API environment;
- an existing integration API Key ID reference and matching private-key file path, without reading
  or displaying either secret value;
- a usable CLI, Co-Signer health when relevant, MPC status, and selected-chain readiness;
- an existing ledger account when the wallet request identifies one or the current configuration
  already resolves it unambiguously.

Reuse every verified checkpoint and continue from the first genuinely missing prerequisite. Do not
ask again about account access, installation folder, existing keys, Co-Signer setup, or MPC/DKG when
the available configuration and read-only checks already answer those questions. If the request is
only a routine post-onboarding ledger-account, wallet, or withdrawal operation, route execution
through `$brosettlement-api` and use this skill only for the onboarding state and safety rules that
still apply. For a resolved withdrawal, do not inspect or recheck Co-Signer/MPC state first.

For a new onboarding, the user's request to complete onboarding or create the first testnet wallet
authorizes the linked tutorial ledger-account and wallet pair described above. For a routine
post-onboarding request, an explicit instruction to create one specific testnet ledger account or
wallet authorizes only that exact resource; pass the CLI's `--confirm` flag without a duplicate
yes/no question. Routine authorization does not extend to another linked resource, a different
network, or any mainnet mutation.

For a routine wallet request, use the resolved existing ledger account and never create another
account. Resolve only the scopes required by the requested operation from the cached current
Swagger; do not require `accounts:create` merely to create a wallet for an existing account. After
the requested operation, return its compact result and retrieval commands. Do not continue into the
optional deposit, withdrawal, backup-path, or full onboarding completion flow unless the user asked
to resume onboarding.

For a routine withdrawal after this onboarding, reuse the same key/environment session evidence,
resolved wallet, and successful account/wallet checkpoints. Route the exact request through the
companion skill's withdrawal fast path; do not repeat permission probes or confirmation questions.

The mandatory first-account question and full ordered sequence below apply only to a new onboarding
whose state cannot be recovered safely from the current conversation or existing configuration.
Never infer a credential value, account selection, or network when read-only evidence is ambiguous;
ask only for that missing decision.

## Required sequence

For a new onboarding, follow this order. The existing-installation fast path may resume at the first
unverified checkpoint instead of replaying completed checkpoints.

### 1. Confirm the BroSettlement account

For a new onboarding whose account access is not already established, the first response must ask
only:

> Do you already have a BroSettlement account and access to your organization in BroSettlement Console?

If the answer is no:

1. Help the user open the official registration or invitation flow available to them.
2. Do not invent a Console or signup URL. Ask the user to use the URL supplied by BroSettlement if it is not already available in context.
3. Help with registration and sign-in when browser control is available.
4. Stop until the user confirms that the account exists and the organization is visible in Console.

If the answer is yes, confirm that the organization is visible and continue.

Do not ask the user for a Console URL or environment selection. Set the onboarding API
environment to production by default and continue directly to the installation-folder question.
Use `https://brosettlement-api.brolabel.io/` for the Co-Signer and leave
`BROSETTLEMENT_ENVIRONMENT` unset or set it to `production` for the CLI.

Switch to staging only when the user explicitly says that their organization or environment is
staging, or voluntarily provides a URL whose hostname is `app-staging.brolabel.io`. In that case,
use `https://brosettlement-staging-api.brolabel.io/` and set
`BROSETTLEMENT_ENVIRONMENT=staging`. A request for a testnet wallet, TRON Nile, or test assets does
not by itself mean staging. Do not advertise staging, ask whether the user wants staging, or ask
them to provide a URL. If the user voluntarily supplies a URL with query parameters or other
sensitive data, use only its scheme and hostname and do not repeat the full URL.

### 2. Ask for the installation folder

Ask only:

> In which folder should I install the BroSettlement Co-Signer?

Require an explicit folder path. Resolve it to an absolute path before making changes. Use
`<selected-folder>/brosettlement-mpc-co-signer` as the repository directory unless the selected
folder is already the repository directory.

Treat this as the local installation root, not an API path. Before accepting a path for a
production Linux deployment, require an absolute path with no whitespace, control characters, or
shell metacharacters. Prefer conventional service paths such as `/opt/brosettlement/co-signer`
for the application, `/var/lib/brosettlement/co-signer` for mutable state, and
`/etc/brosettlement/co-signer` for protected configuration. Do not create, move, or rename an
existing installation merely to adopt these examples; explain the conflict and ask the user to
choose the production paths.

Before changing files:

- inspect whether the folder or repository already exists;
- preserve unrelated files;
- never overwrite an existing private key, share-encryption key, shares directory, or configuration;
- ask how to proceed if an existing installation or conflicting file is found.

### 3. Check prerequisites

Check the selected host for Git, Go 1.24 or later, OpenSSL, outbound HTTPS access, a protected
secrets location, and enough access to create the installation directory.

Apply these platform rules before installation:

- **Linux:** use Linux for the officially supported production/mainnet Co-Signer runtime. Keep
  the active shares, lock files, and secrets on one local writable Linux filesystem; do not use
  NFS, another shared filesystem, or an active-active replica topology.
- **macOS:** the Co-Signer can build and run natively, including immutable share publication. Use
  it for local development, verification, onboarding, and testnet flows. It may connect to the
  production BroSettlement API for an explicitly selected testnet/onboarding flow, but the
  current official production/mainnet operating runbook remains Linux-only.
- **Windows:** native Windows is not supported. The current Co-Signer does not compile or provide
  the required atomic share-publication and process-locking guarantees on Windows. Do not attempt
  a native Windows installation or weaken those safeguards.
- **Windows with WSL2 or a Linux VM:** offer this only as a Linux-hosted alternative, not as native
  Windows support. Keep the repository, active shares, locks, and secrets inside the Linux
  filesystem (for example, the WSL2 ext4 filesystem), never under `/mnt/c`, a Windows network
  drive, or another shared mount. A dedicated Linux VM or server remains the recommended
  production host.

Do not confuse the BroSettlement API environment with the blockchain network or Co-Signer host
support. Selecting the production API does not itself choose testnet or mainnet. Onboarding may
perform explicitly selected mainnet operations after the required confirmation and production
readiness checks. Using macOS for onboarding against the production API does not make macOS an
officially supported production/mainnet runtime.

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

After the numbered creation steps, always show the following as its own visible paragraph with the
heading **Where to find the API Key ID**. Do not compress it into the sentence about creating the
key or leave it as a short numbered item:

> After the key is created, return to **API Keys**, find `Testnet Co-Signer`, and select **View**.
> On the key details page, find the **Key ID** field near the top and click the copy button on the
> right side of that field. Use the value copied from **Key ID**—do not copy the identifier from
> the browser URL. Store the copied API Key ID securely.

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

1. Create separate, protected, non-overlapping primary and recovery shares directories for Share B
   and Share C. Never place them under the same parent storage boundary in production.
2. Generate one separate share-encryption key with `600` permissions and assign its stable
   non-secret key ID. Preserve both for the lifetime of the artifacts.
3. Configure the environment selected by the conversation rule above:
   - use production by default and set `CO_SIGNER_MONOLITH_URL` to
     `https://brosettlement-api.brolabel.io/`;
   - switch to `https://brosettlement-staging-api.brolabel.io/` only when the user explicitly
     identifies their environment as staging or voluntarily provides an
     `app-staging.brolabel.io` URL;
   - do not ask the user what `CO_SIGNER_MONOLITH_URL` is;
   - do not use the admin-panel URL itself as `CO_SIGNER_MONOLITH_URL`;
   - configure the companion CLI with `BROSETTLEMENT_ENVIRONMENT=production` for production or
     `BROSETTLEMENT_ENVIRONMENT=staging` for staging. The CLI defaults to production when this
     variable is absent.
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

Launch safely:

- pass the executable and arguments as separate values and set the process working directory
  explicitly; never concatenate paths into a command string or pass the launch through `sh -c`;
- when the available execution tool accepts only a shell string, apply correct shell quoting to
  every path and value, and keep secret values out of the command line;
- use the production service manager described by the official Co-Signer runbook for a durable
  deployment, with fixed binary, working-directory, configuration, and state paths;
- before retrying a failed start, verify that no Co-Signer process was created and no external
  state changed;
- if parsing or quoting fails, report the exact non-secret path, identify whether it is the
  binary, working directory, configuration, private-key file, or shares directory, and explain
  the corrected invocation. Never report only "a path with spaces."

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
5. Poll with `brosettlement mpc status` through `$brosettlement-api` after `2s`, `5s`, `10s`,
   `15s`, and `30s`, then at most every `30s` within a strict three-minute interactive observation
   window. Bound each request timeout to the remaining window.
6. Monitor Co-Signer logs and Console status only within the same three-minute window. If DKG is
   still non-terminal when it expires, report **initialization accepted; verification pending**,
   show the current non-secret status and key ID when available, and return this continuation
   command instead of waiting longer:

   ```text
   @brosettlement mpc status
   ```

Do not restart the process, rotate the API key, replace the share-encryption key, or alter the
shares directory during DKG.

Initialization completes only when:

- the MPC key is **Active** or **Ready**;
- the Co-Signer remains **Online**;
- every chain selected for the onboarding flow is **Ready**.

If DKG fails or expires, diagnose the terminal state before retrying. Do not create repeated
initialization attempts blindly.

### 8a. Separate Share C after DKG

After DKG is terminal and the MPC key and chains are ready, follow the Share B/Share C custody
checkpoint in the onboarding reference before creating wallets. Explain that normal BroSettlement
signing uses platform Share A plus client Share B; Share C does not participate. Share B must remain
available to the running Co-Signer, while Share C is a client-controlled recovery share.

Show only the absolute artifact paths and purposes, never their contents. Require the user to make
and verify a complete protected backup of Share C with the original share-encryption key and key
ID. Ensure Share B has its own independent backup in a different trust domain. Use only a read-only
filesystem existence check for the active-host Share C path. If the file is still present, say
plainly that it was verified at that exact path, that keeping Share B and Share C on the active
Co-Signer host is a critical security gap because together they form a signing quorum, and that
the user must remove Share C from that host after verifying independent recovery custody. Do not
ask for permission to stop or restart the Co-Signer, and never copy, move, rename, archive, upload,
delete, or otherwise alter Share C. The agent must not perform the remediation itself.

Explain that B+C form a client-controlled 2-of-3 recovery quorum that can sign without platform
Share A or BroSettlement participation. For a native TRX or standard TRC-20 recovery transfer on
TRON Nile or mainnet, hand off only to the sibling `$brosettlement-disaster-recovery` skill and its
controlled ceremony. Do not claim support for arbitrary contracts, other chains, or sign-only
recovery. Never store B and C in one
host, filesystem, vault, cloud account, backup set, or shared administrative domain outside the
short recovery execution window, and never give either share to a third party.

### 9. Complete the Quickstart

After MPC readiness:

1. Before asking about credentials, inspect the approved runtime environment and resolved local
   configuration read-only for an integration API Key ID reference and matching private-key file
   path. Check only presence, file type, and permissions; never display the ID or private key. A
   full onboarding key must have exactly `accounts:read`, `accounts:create`, `wallets:read`, and
   `wallets:create`; a routine fast-path request needs only the current Swagger scopes for that
   requested operation. Do not add integration scopes to the long-running Co-Signer key by default.
   If the references are present, do not ask whether the user has a key; verify them with the
   session's single authentication probe in step 6.
2. If no usable integration credentials can be resolved, use the protected secrets path already
   approved during this onboarding and resolve distinct integration-key filenames there. An
   explicit request to complete onboarding authorizes generating this separate Ed25519 pair without
   another yes/no question only when the resolved directory is already approved, both target files
   are absent, and no existing credential would be overwritten or displaced. Ask only when the path
   is missing or ambiguous, either target collides, an existing key may need replacement, or the
   user must choose between multiple credentials.
3. When authorized by the rule above, generate the separate pair with distinct filenames such as
   `integration-api-private.pem` and `integration-api-public.pem`. Apply the collision and private
   key protections from checkpoint 4. Never reuse or overwrite the Co-Signer key pair.
4. Immediately read `integration-api-public.pem` and show its complete contents in a fenced `pem`
   block, including both boundary lines. Tell the user to copy the displayed block into **Public
   key (PEM)** while manually creating a new integration API key in Console. Instruct the user to
   use a recognizable name such as `Testnet Integration`. For temporary staging/testnet use,
   explain that `0.0.0.0/0` in **IP whitelist (CIDR)** accepts any IPv4 source and bypasses the IP
   restriction; never allow it for production, where the user must enter the correct narrow public
   egress IP or CIDR. For a new full onboarding, select exactly these four Console permissions:
   - **Read ledger accounts** (`accounts:read`)
   - **Create ledger accounts** (`accounts:create`)
   - **Read wallets & assets** (`wallets:read`)
   - **Create wallets** (`wallets:create`)

   For a routine fast-path request, select only the scopes required for the requested operation by
   the cached current Swagger. In particular, do not request `accounts:create` merely to create a
   wallet for an existing account.

   Create and activate the key. Return to the **API Keys** list, find `Testnet Integration`, select
   **View**, and click the copy button beside **Key ID** on the key details page. Place that copied
   API Key ID in the approved runtime environment. Never show the integration private key or
   operate the Console.
5. When a new Console key was required, pause until the user confirms it is active, the scopes
   required for the current full-onboarding or routine flow are selected, and its matching
   credentials are available to `$brosettlement-api`. This user-only Console completion gate cannot
   be inferred or skipped. For reused credentials, do not request a duplicate confirmation after
   their read-only probe succeeds.
6. For this onboarding session and integration key, run one read-only authentication probe before
   the first mutation and reuse that verified result for both the account and wallet creates. Fetch
   the current Swagger once for the selected API environment before the first create and reuse that
   same contract for both operations. Refetch only if the environment changes or the server reports
   a contract/schema mismatch. Do not repeat the CLI update gate, Swagger fetch, or authentication
   probe for the wallet after the account create succeeds.
7. For a new full onboarding, briefly say that the first ledger account is being created. Use
   `$brosettlement-api`
   to inspect and call `POST /api/v1/ledger/accounts`. When the user did not choose tutorial
   names, use a clear default such as `Testnet Treasury` and generate a unique `externalId`; do
   not interrupt the flow merely to approve those harmless defaults. Pass the CLI's required
   `--confirm` under the standing onboarding authorization above without asking the user again.
8. For a new full onboarding, require an account ID from the create response and verify it with
   `GET /api/v1/ledger/accounts/{accountId}`. Only after successful read-back, say explicitly
   **Ledger account created successfully.** Show the sanitized create response and a compact
   summary of the fields actually returned, such as `id`, `name`, `externalId`, organization ID,
   and timestamps. Never invent missing fields. Then teach both ways to find it later:

   ```text
   @brosettlement api GET '/api/v1/ledger/accounts'
   ```

   The same accounts are available in BroSettlement Console under **Accounts**. Continue to the
   wallet automatically; do not pause for another confirmation. During a routine wallet fast path,
   skip steps 7–8 and use the resolved existing ledger account.
9. Briefly say that a wallet is being created for the verified account. Use
   `$brosettlement-api` to inspect and call `POST /api/v1/wallets`. Use a ready testnet chain such
   as **TRON Nile** for the default tutorial. If the user explicitly requested a supported
   mainnet chain, verify production readiness and obtain separate explicit confirmation for that
   exact wallet creation before passing `--confirm`. Use a stable idempotency key when the current
   contract requires one.
10. Require a wallet ID from the response and verify it immediately with
    `GET /api/v1/wallets/{walletId}`. If it is not yet **Active**, poll after `2s`, `5s`, `10s`,
    `15s`, and `30s`, then use only the remaining time before a strict 90-second wall-clock
    activation deadline for at most one final check. Bound each request timeout to the remaining
    deadline and never continue polling past 90 seconds. If it becomes **Active**, say explicitly
    **Wallet created successfully.** Show the sanitized create response and a compact summary of
    the fields actually returned, including the wallet ID, linked account ID, network/chain, public
    address, status, and timestamps when present. Never display secrets or fabricate fields. Teach
    how to retrieve it later:

    ```text
    @brosettlement api GET '/api/v1/wallets'
    @brosettlement api GET '/api/v1/ledger/accounts/<accountId>/wallets'
    ```

    The user can also open **Wallets** in Console for the wallet list and **Accounts** for the
    linked account details. If the create is accepted but the wallet is not **Active** by the
    deadline, report **verification pending** rather than waiting longer. Return the sanitized
    create response, wallet ID, last observed status, and both retrieval commands above. If
    read-back fails or a terminal failure appears, report **verification incomplete** and do not
    claim successful creation. Never start an unbounded WebSocket listener synchronously while
    waiting for activation.
11. Only during a new full onboarding, ask: **Would you like to test a small TRON Nile deposit
    next?** If the answer is **Yes**:
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
    - monitor the wallet balance and ledger entries with `wallets:read` after `5s`, `10s`, `15s`,
      and `30s`, then at most every `30s` within a strict two-minute observation window after the
      user confirms broadcast. If the key also has
      `transactions:read`, reconcile the deposit transaction; if it has `websockets:read`, a
      WebSocket stream with an explicit finite `--stop-after` may supplement polling. Never run an
      unbounded listener synchronously or let it extend the current operation's time bound. Confirm
      the balance delta and final ledger entry when observed. If the deadline expires first, report
      **deposit verification pending**, include the public transaction hash when supplied, and
      return the wallet and ledger retrieval commands instead of waiting longer;
    - do not call `POST /api/v1/transactions` for a deposit; that endpoint creates an outgoing
      transaction.
12. Offer a small withdrawal separately. Once the source wallet, network, asset, amount, and
    destination are resolved, invoke `$brosettlement-api`'s withdrawal fast path. The user's exact
    instruction is confirmation for that one request; do not preflight permissions or ask again.
13. For a withdrawal, use one immediate `GET /api/v1/transactions/{id}` as the required
    verification. If it is non-terminal, report **withdrawal accepted; verification pending**,
    return the retrieval command, and stop. Do not add account, wallet, authentication, balance,
    Co-Signer, MPC, ledger-list, or WebSocket probes unless the returned transaction exposes a
    concrete inconsistency. Optional checks must not block or delay the result.
14. Report the existing credential and MPC-share locations as described below. During normal
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
- primary Share B directory and artifact: remains available to the Co-Signer for normal A+B signing;
- recovery Share C directory and artifact: must be backed up and removed from the active Co-Signer
  host after DKG;
- share-encryption key ID: non-secret binding required together with the original encryption key
  to restore either immutable artifact;
- integration Ed25519 private PEM: signs ledger, wallet, and other integration API requests for
  its current API Key ID;
- integration public PEM: registered in Console and safe to display, but not a secret;
- runtime configuration or launcher files: identify API Key IDs, endpoints, and the paths from
  which the protected values are loaded; state whether they contain secrets before listing them.

Omit artifacts that were not created or used. Never print file contents, secret values, key
fingerprints that were not already approved for display, or environment-variable values. Tell the
user to preserve these paths and make their own protected, encrypted backup in a trusted secret
manager or offline storage. Do not ask where to save it and do not perform the copy.

State clearly that retaining signing access requires the immutable Share B and Share C artifacts,
the original share-encryption key, and its exact key ID. Keep B and C in separate client-controlled
trust domains and never combine them in one backup. Losing B blocks normal signing; losing C removes
the client recovery quorum. Also preserve each API private key when the corresponding existing API
Key ID will continue to be used; rotating an API credential does not recover MPC shares. Never
initialize a replacement MPC key as a backup procedure.

## Operating rules

- Never request, display, transmit, log, or commit private keys, share-encryption keys, or MPC share files.
- Never operate BroSettlement Console to create, edit, rotate, or revoke an API key.
- Never ask for authorization to manage an API key; provide instructions and wait for the user.
- Never ask for a Console URL or environment selection during normal onboarding. Use production
  by default and switch to staging only from the user's explicit staging statement or a
  voluntarily supplied `app-staging.brolabel.io` URL. Do not infer staging from testnet usage.
- For an existing configured installation or a routine account/wallet request, use the read-only
  fast path and do not restart the questionnaire or replay verified checkpoints.
- Treat production/staging as API environments and testnet/mainnet as blockchain-network choices.
  The tutorial standing authorization covers one ledger account plus one linked testnet wallet in
  either API environment. Only an actual blockchain-mainnet mutation requires the separate
  per-action mainnet confirmation.
- Never implement or send a BroSettlement API request independently when the companion API skill is available.
- Never upload the client private key or client MPC share to BroSettlement.
- Do not invent Docker images, packages, environment variables, or deployment commands. Use the current official repository and Console setup instructions.
- Never claim native Windows support or bypass unsupported filesystem publication, locking, or
  durability checks. On Windows, use only a properly configured WSL2/Linux VM path as a Linux
  environment, with all operational files kept off Windows-mounted filesystems.
- Do not tell a macOS user that the production API is unavailable. Explain that API environment,
  blockchain network, and supported production host are separate choices; Linux remains the
  official production/mainnet Co-Signer runtime.
- Do not treat local `ready: true` as end-to-end readiness. Also verify the Console heartbeat, MPC key status, and chain status.
- Do not create a wallet before MPC is ready.
- Do not change the API key, share-encryption key, or shares directory while DKG or signing is active.
- After successful DKG, use only a read-only existence check for Share C. If it remains on the
  active Co-Signer host, report its path, identify the critical signing-quorum security gap, and
  tell the user to remove it themselves only after backup verification. Never stop or restart the
  Co-Signer and never copy, move, rename, archive, upload, or delete Share C.
- Never store, back up, transmit, or administer Share B and Share C together. Do not grant a third
  party access to either share or upload either artifact to chat, email, tickets, or shared drives.
- Use `$brosettlement-disaster-recovery` only for its supported native TRX or standard TRC-20
  Nile/mainnet recovery ceremony. Do not generalize it into support for other chains or arbitrary
  contract calls.
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
- Never say that onboarding prohibits mainnet operations. Testnet is the no-funds tutorial
  default; an explicitly selected mainnet flow is allowed after production-readiness checks and
  confirmation of each state-changing mainnet action.
- Do not turn operational monitoring guidance into an onboarding checkpoint or follow-up question.
- Within one onboarding session, run the environment-independent CLI update gate exactly once.
  Fetch Swagger once per selected environment. Run a read-only authentication probe only when no
  successful operation already establishes access for that key and environment. Refetch Swagger
  only when the environment changes or the contract is invalidated; repeat a probe only when its
  key/environment changes or access is invalidated. Reuse successful account and wallet operations
  as session evidence, and never add another probe solely to predict a mutation-only scope before
  an explicitly authorized withdrawal.
- Never wait synchronously on an unbounded WebSocket listener. Use REST for required lifecycle
  verification and give every optional WebSocket observation an explicit finite stop condition.
- Do not ask for a backup directory or copy secrets during normal onboarding; report the existing
  absolute paths and let the user perform their own secure backup. The only exception is the
  explicitly approved protected legacy archive required for an incompatible Co-Signer upgrade.
- For a production Linux deployment, reject installation, configuration, secret, and state paths
  containing whitespace, control characters, or shell metacharacters before startup. Regardless
  of the path, never construct the Co-Signer launch by interpolating it into an unquoted shell
  command.
- Stop and explain the blocker when credentials, scopes, allowlists, plan limits, or readiness checks are incomplete.

## Completion report

At the end, report each checkpoint without exposing secrets or claiming optional tests ran:

For a routine fast-path request, report only the requested operation, API environment, sanitized
response, resource ID and status, verification result, and retrieval commands. Do not replay this
full onboarding report or list credential/share paths unless the user asked to resume onboarding.

- account and organization access confirmed;
- installation path;
- Co-Signer API key created with required MPC scopes;
- integration API key and scopes used for ledger account and wallet creation;
- Co-Signer installed and built;
- Co-Signer running version and Git commit, plus the latest-version comparison when troubleshooting required it;
- Co-Signer local health;
- Console heartbeat;
- MPC/DKG status;
- Share B operational custody, separate Share C backup status, and the read-only Share C presence
  result. If Share C is still present, report the unresolved critical security gap without altering
  the file;
- chain readiness;
- ledger account and wallet identifiers;
- test transaction status;
- exact paths and purposes of the credential files and encrypted shares, without showing values;
- the matched shares-directory and share-encryption-key recovery requirement for existing wallets.
