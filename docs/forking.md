# Forking

Nanvil forks mainnet/testnet using the **WorkNet-style lazy StateService** model:

1. Capture branch metadata (`getstateheight`, `getstateroot`, `findstates`)
2. Lazy-fetch contract storage via `findstates` / `getproof` on access
3. Cache pages on disk under `--fork-cache-path`
4. Run local single-validator consensus for writable state after branch

## Requirements

Remote RPC node must expose:

- StateService plugin with **`FullState: true`**
- RpcServer plugin

Official seeds (documented as FullState-enabled):

- MainNet: `http://seed1.neo.org:10332`
- TestNet: `http://seed1t5.neo.org:20332`

## Create manifest

```bash
nanvil fork create --rpc-url http://seed1t5.neo.org:20332 --block 1000000 --out fork.json
```

## Start forked node

```bash
nanvil start --fork-url http://seed1t5.neo.org:20332 --fork-block-number 1000000
```

Auto-impersonation is on by default for forks. Disable with `--auto-impersonate=false` if needed.

Persist local changes across restarts:

```bash
nanvil start --data-dir ./nanvil-data --fork-url http://seed1.neo.org:10332
```

On restart, `--fork-url` can be omitted if the fork manifest is embedded in `chain.state.json`.

## Prefetch

Large contracts may stall on first access. Prefetch before testing:

```bash
nanvil fork prefetch --manifest fork.json --contract 0x...
```

## Limitations

- History before branch height is not fully validated locally
- First contract touch may download large storage sets
- Unsigned transition block semantics match Neo WorkNet caveats

See [fork-troubleshooting.md](fork-troubleshooting.md).
