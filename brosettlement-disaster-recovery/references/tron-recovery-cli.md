# Recovery TRON Sign — source-only CLI

`recovery-tron-sign.go` is plain Go source. Run it with `go run`; there is no precompiled executable.

The program creates either a native TRX transfer or a standard TRC-20 `transfer(address,uint256)` call through the selected public TRON RPC, signs its `txID` with Share B + Share C, independently verifies the signature, stores the signed transaction as a mode-`600` JSON file, and broadcasts it. It contains no BroSettlement API or Console integration and no organization-specific defaults.

## Required parameters

Every invocation requires these options:

```text
--network <nile|mainnet>
--source <TRON_ADDRESS>
--to <TRON_ADDRESS>
--amount <TRX_AMOUNT>
--output <ABSOLUTE_JSON_PATH>
--share-b <ABSOLUTE_SHARE_B_PATH>
--share-c <ABSOLUTE_SHARE_C_PATH>
--encryption-key <ABSOLUTE_ENCRYPTION_KEY_PATH>
```

The MPC key ID and encryption key reference are authenticated and inferred from the two share artifacts. The public `--source` address is authoritative: the CLI scans `m/44'/195'/0'/0/index` up to `--address-search-limit` (default `10000`) and signs only after an exact address match.

`--token-contract <TRON_ADDRESS>` is optional. Omit it for native TRX. When it is present, the CLI treats `--amount` as a TRC-20 token amount and also requires `--fee-limit-trx <TRX>`. Token decimals, token balance, transfer simulation, and unsigned transaction construction are obtained only through the public TRON RPC.

## Run

```bash
go run ./recovery-tron-sign.go \
  --network <nile|mainnet> \
  --source <SOURCE_ADDRESS> \
  --to <DESTINATION_ADDRESS> \
  --amount <AMOUNT_IN_TRX> \
  --output </ABSOLUTE/PATH/TO/SIGNED_TRANSACTION.json> \
  --share-b </ABSOLUTE/PATH/TO/SHARE_B.json> \
  --share-c </ABSOLUTE/PATH/TO/SHARE_C.json> \
  --encryption-key </ABSOLUTE/PATH/TO/ENCRYPTION_KEY>
```

`--amount` accepts a positive TRX decimal with up to six fractional digits. `--network` accepts only `nile` or `mainnet`. Selecting `mainnet` means the transaction will be signed and broadcast on TRON production.

TRC-20 example additions:

```text
--token-contract <TRC20_CONTRACT_ADDRESS>
--fee-limit-trx <MAXIMUM_TRX_FEE>
```

For TRC-20, `--amount` uses the decimals returned by the contract. `--fee-limit-trx` is a maximum transaction fee, not the token amount, and the source must hold enough native TRX for that conservative reserve.

On success, the program prints status `BROADCAST_ACCEPTED`. The signed JSON is stored before the broadcast attempt, so it remains available if the RPC rejects or cannot accept the transaction.

## Safety properties

- The core transfer and protected-file options are mandatory; `--token-contract` is optional and selects TRC-20 instead of native TRX.
- Share B, Share C, and the encryption key are supplied explicitly by absolute path.
- Share B and Share C must be distinct mode-`600` regular files and form one authenticated 2-of-3 recovery quorum.
- The artifacts must agree on encryption key reference, descriptor, MPC key ID, session, public key, chain-code binding, and codec version.
- A child address discovered from the shares must exactly match `--source` before transaction creation.
- Existing output files are never overwritten.
- The RPC-generated `txID` must equal `SHA256(raw_data)`, and the transaction must exactly match the requested source, destination, amount, asset type, optional token contract, transfer calldata, and fee limit.
- The B+C ECDSA signature and recovery ID are independently verified before output and broadcast.

Keep Share B and Share C in separate trust domains outside a controlled recovery ceremony. Possession of both shares provides a signing quorum.
