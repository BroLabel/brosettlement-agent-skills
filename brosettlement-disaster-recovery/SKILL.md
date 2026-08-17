---
name: brosettlement-disaster-recovery
description: Run a controlled BroSettlement disaster-recovery ceremony that creates, Share B + Share C threshold-signs, saves, and broadcasts one native TRX transfer without BroSettlement participation. Use when the platform Share A or normal A+B signing path is unavailable and an authorized client must recover TRON Nile or TRON mainnet funds from immutable Co-Signer Share B and Share C artifacts. Also use to validate recovery prerequisites, explain B+C quorum custody, or diagnose the bundled recovery CLI. Do not use for routine withdrawals, TRC-20 tokens, other chains, sign-only/offline output, or normal BroSettlement API transactions.
---

# BroSettlement disaster recovery

Treat this as a break-glass custody operation. Use the bundled source-only CLI; do not reimplement
MPC signing, artifact decoding, address derivation, or TRON transaction construction.

Read [references/tron-recovery-cli.md](references/tron-recovery-cli.md) before preparing or running
a transfer. The executable source and pinned module files are in `scripts/`.

## Hard boundaries

- Support exactly one native TRX transfer per invocation on `nile` or `mainnet`.
- Use the fixed derivation path `m/44'/195'/0'/0/0`.
- Do not use this workflow for TRC-20, arbitrary contracts, other derivation paths, or other chains.
- Do not call BroSettlement APIs or require platform Share A. The only external service used by the
  CLI is the selected public TRON RPC.
- The bundled CLI always creates, signs, saves, and broadcasts. It has no dry-run or sign-only mode.
  If the user requests sign-only or offline transaction creation, stop and explain that it is not
  supported by this tool.
- Never ask the user to paste Share B, Share C, the encryption key, or decrypted material into chat.
  Accept only absolute local paths on the client-controlled recovery host.
- Never upload, copy into the skill, print, decrypt for inspection, or expose either share.
- Never weaken file-mode, artifact-consistency, derived-address, txID, or signature checks.

## 1. Establish the recovery ceremony

Confirm that the user is intentionally invoking disaster recovery because the normal A+B signing
path is unavailable or unsuitable. Explain that B+C form the client-controlled 2-of-3 quorum and
can sign without BroSettlement, so bringing both shares into one process grants full signing power.

Require a clean, client-controlled recovery host with:

- Go 1.24 or later;
- no screen sharing, session recording, shell tracing, cloud-sync folder, or third-party operator;
- separate custodians or trust domains for B and C until the final execution window;
- an encrypted temporary workspace and direct HTTPS access to the selected TRON RPC.

B and C may coexist only temporarily in this controlled ceremony. Outside it, keep them in
separate hosts, vaults, backup sets, recovery media, cloud accounts, and administrative domains.

## 2. Collect non-secret inputs

Ask for these values and nothing secret:

1. network: `nile` or `mainnet`;
2. source TRON address;
3. destination TRON address;
4. exact native TRX amount, with no more than six fractional digits;
5. a new absolute output path for the signed transaction JSON;
6. absolute Share B artifact path;
7. absolute Share C artifact path;
8. absolute path to the matching share-encryption key.

Do not ask for the MPC key ID or encryption key reference; the CLI authenticates and derives them
from the artifacts. Do not request API credentials.

## 3. Run read-only preflight

Resolve this skill directory and work from `scripts/`. Before touching both shares:

1. Confirm the bundled `recovery-tron-sign.go`, `go.mod`, and `go.sum` exist.
2. Run `go version` and require Go 1.24 or later.
3. Run `go mod verify`, `go test ./...`, and build the CLI into a newly created private temporary
   directory. Do not place binaries beside the shares.
4. Run the candidate with `--help` only.
5. Inspect only filesystem metadata for the three protected inputs. Require absolute paths,
   distinct Share B and Share C paths, regular non-symlink files, and exact mode `600`.
6. Require the output path to be absolute, absent, outside all protected-input paths, and inside a
   mode-`700` client-controlled directory.

Do not read share contents during preflight. The CLI performs authenticated artifact validation only
after the final transfer confirmation.

## 4. Obtain one transaction-specific confirmation

Show a concise final summary containing only:

- network and whether it is testnet or production;
- source address;
- destination address;
- exact TRX amount;
- output path;
- that execution will temporarily combine B+C, sign, save, and immediately broadcast through the
  public TRON RPC.

Ask for explicit confirmation of this exact transfer. For `mainnet`, state plainly that real funds
will move and the broadcast cannot be undone. Do not accept a prior generic onboarding approval as
authorization. If any transaction field changes, discard the confirmation and ask again.

## 5. Execute exactly once

After confirmation, run the private candidate binary once with all eight required flags. Pass only
paths, never key or share contents:

```bash
<PRIVATE_TEMP_DIR>/recovery-tron-sign \
  --network <nile|mainnet> \
  --source <SOURCE_ADDRESS> \
  --to <DESTINATION_ADDRESS> \
  --amount <TRX_AMOUNT> \
  --output </ABSOLUTE/PATH/TO/NEW-SIGNED-TRANSACTION.json> \
  --share-b </ABSOLUTE/PATH/TO/SHARE-B.primary.json> \
  --share-c </ABSOLUTE/PATH/TO/SHARE-C.recovery.json> \
  --encryption-key </ABSOLUTE/PATH/TO/share-encryption.key>
```

Do not enable shell tracing, capture environment dumps, or place secret material in command-line
values. The path names themselves may be sensitive; redact them from the final report.

## 6. Verify the result before claiming success

Claim success only when the CLI returns JSON with all of the following:

- `status: BROADCAST_ACCEPTED`;
- `derivedAddressMatch: true`;
- `artifactsMatch: true`;
- `signatureVerified: true`;
- `broadcastPerformed: true`;
- `broadcastAccepted: true`;
- a non-empty `txId`;
- the expected network, source, destination, amount, and output file.

Report the public txID, network, source, destination, amount, expiration, and signed-transaction
output path. The output is created with mode `600` before broadcast; keep it for audit and recovery
evidence. Never report decrypted artifacts, share fingerprints, or encryption-key material.

If execution fails, do not rerun blindly. Check whether the output file was created. A signed
transaction may already exist even when broadcast failed. Preserve it, inspect only its public
transaction fields, check the public txID on the matching explorer, and explain the exact terminal
error before proposing any retry. Every retry needs a new output path and a new explicit transfer
confirmation.

## 7. Close the ceremony

After success or failure:

1. Stop all recovery processes.
2. Remove the temporary binary and module build cache created for the ceremony when practical.
3. Have each custodian remove or unmount its local share from the recovery host and return B and C
   to their separate trust domains. Do not delete the authoritative backups.
4. Destroy the encrypted temporary workspace key or ephemeral recovery environment according to
   the client's approved procedure. Do not promise secure deletion on SSD media.
5. Confirm that B and C are no longer co-located and that no third party retains access.

## Troubleshooting

- `protected input must be a mode-600 regular file`: fix ownership/mode or replace a symlink with
  the actual protected file; never relax the check.
- `Share B and Share C do not form one validated recovery quorum`: stop. Do not mix artifacts from
  different MPC keys, sessions, organizations, codecs, or encryption-key references.
- `derived B+C public key does not match the requested source address`: stop. Reconfirm the wallet
  address and correct recovery set; never override the source check.
- insufficient balance: reduce the amount only after a new transaction confirmation or fund the
  public address. The CLI checks native TRX balance, not token balances.
- RPC error after output creation: preserve the signed JSON and public txID; determine whether it
  was accepted or expired before considering another transaction.

Never improvise around a failed cryptographic or address-binding check.
