# Contributing

Thank you for helping improve BroSettlement Agent Skills.

## Before opening a pull request

1. Keep each skill self-contained and preserve its exact folder name.
2. Treat the selected environment's live Swagger document as the source of truth for API
   operations, schemas, scopes, signing requirements, idempotency, and errors.
3. Use production by default. Switch to staging only when the user explicitly identifies staging,
   and never mix credentials, URLs, or resources between environments.
4. Never add credentials, private keys, API Key IDs, signed WebSocket URLs, organization data,
   MPC shares, or real customer payloads.
5. Keep API-key creation and management as manual user actions in BroSettlement Console.
6. Require explicit authorization before state-changing API calls. An unambiguous current user
   instruction for one exact operation is already confirmation; pass the CLI's `--confirm` flag
   without asking the same yes/no question again.

## Validate changes

Run:

```bash
make check
```

For changes to the API skill, also forward-test at least one read-only command and one guarded
mutation plan. Do not execute a live mutation as part of a pull request test.

For changes to the disaster-recovery skill, compile and test the source-only CLI with synthetic or
fixture material only. Never combine real Share B + Share C artifacts, sign a real transaction, or
broadcast to Nile or mainnet as part of repository validation.

## Pull requests

Describe:

- the user problem;
- the affected skill or CLI command;
- the production or staging contract and documentation used;
- the checks performed;
- any security or compatibility impact.

Keep pull requests focused. Avoid unrelated formatting or generated artifacts.
