# Fork troubleshooting

## "Old state not supported"

Remote node does not have StateService `FullState: true`. Use a FullState-enabled RPC endpoint.

## Slow first contract call

Remote storage is fetched lazily. Run `nanvil fork prefetch` for the contract hash first.

## Proof verification failed

Branch state root may be stale. Recreate manifest at a recent block height.

## Missing contracts

Only contracts deployed at branch height appear in the manifest. Deployed-after-branch contracts exist only on the public chain.

## Cache issues

Delete `--fork-cache-path` directory and restart, or use `--no-storage-caching` for debugging.
