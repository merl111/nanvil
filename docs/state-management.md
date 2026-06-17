# State management

## Snapshots

```json
{"method":"nanvil_snapshot","params":[]}
{"method":"nanvil_revert","params":["0x1"]}
```

Snapshots record block height. Revert calls `Blockchain.Reset` (requires chain not running — best used between test cases or after shutdown).

## Dump / load

Full chain state (storage, fork overlay, fork manifest, snapshot list) is written on shutdown when `--dump-state` or `--data-dir` is set.

```bash
# Persist across stop/start with a data directory (recommended)
nanvil start --data-dir ./nanvil-data --fork-url http://seed1.neo.org:10332

# Or explicit dump/load paths
nanvil start --fork-url http://seed1.neo.org:10332 --dump-state ./chain.state.json
nanvil start --load-state ./chain.state.json

# Periodic dumps while running
nanvil start --data-dir ./nanvil-data --state-interval 60s
```

On restart with `--load-state` or `--data-dir`, nanvil restores the backing store and fork overlay, reconnects to the remote fork RPC, and skips bootstrap when the chain height is already past the fork branch.

Legacy metadata-only state files (`height` + `snapshots` without `format: nanvil-chain-v1`) are still supported for snapshot revert metadata.

## Reset

`nanvil_reset` drops local blocks to genesis or fork branch point (requires node restart in current implementation).
