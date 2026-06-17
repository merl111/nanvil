# Impersonation

Neo3 uses **CheckWitness** and transaction **signers/scopes**, not Ethereum `msg.sender`.

Nanvil dev mode bypasses witness verification for impersonated accounts:

1. `nanvil_impersonateAccount(address)` registers a script hash
2. Transactions including that hash in **signers** pass `CheckWitness` without a valid signature
3. Transaction witness verification is skipped for impersonated signers

## Example workflow

```bash
# Enable impersonation
curl ... -d '{"method":"nanvil_impersonateAccount","params":["NVTi..."]}'

# Build tx with impersonated address as signer (empty/minimal witness)
# Submit via sendrawtransaction
```

## Auto mode

`nanvil start --auto-impersonate` or `nanvil_autoImpersonateAccount(true)` impersonates any signer.

On forked nodes, `--auto-impersonate` is **enabled by default** unless you pass `--auto-impersonate=false`.

## Scopes

Use `CalledByEntry` or `Global` scopes when contracts check witness context. Multisig accounts may need explicit scope configuration.

## Security

Impersonation is **dev-only**. Never enable on public networks.
