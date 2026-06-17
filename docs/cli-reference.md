# CLI reference

## nanvil start

Start the development node.

| Flag | Default | Description |
|------|---------|-------------|
| `--host` | `127.0.0.1` | RPC bind host |
| `--port` | `8545` | RPC port |
| `--accounts` | `10` | Number of dev accounts |
| `--balance` | `1000000000000` | GAS per account (8 decimals) |
| `--mnemonic` | Anvil test mnemonic | BIP39 phrase |
| `--block-time` | `0` | Block interval (`0` = mine on tx) |
| `--no-mine-empty` | false | With `--block-time`, only mine when the mempool has transactions |
| `--empty-block-interval` | `0` | Mine empty blocks on an interval when idle (use with `--block-time=0`) |
| `--no-mining` | false | Disable auto-mining |
| `--auto-impersonate` | off (on for forks) | Auto-impersonate transaction signers; enabled by default when `--fork-url` or a fork manifest is loaded |
| `--print-traces` | false | Reserved; invocation logs are saved by default (see [tracing.md](tracing.md)) |
| `--dump-state` | | Dump full chain state on exit |
| `--load-state` | | Load chain state on start |
| `--data-dir` | | Persistent directory (`chain.state.json` auto load/dump) |
| `--state-interval` | | Periodic full state dump while running |
| `--fork-url` | | Remote RPC for fork (alias: `--rpc-url`) |
| `--fork-block-number` | `0` | Branch height (0 = latest validated state) |
| `--fork-cache-path` | temp dir | Fork remote storage cache directory |
| `--no-storage-caching` | false | Always fetch contract storage from remote |
| `--with-explorer` | true | Enable block explorer UI |
| `--no-explorer` | false | Disable block explorer UI |
| `--explorer-host` | same as `--host` | Explorer bind host |
| `--explorer-port` | `8546` | Explorer port |

## nanvil fork create

Create a fork manifest JSON file.

```
nanvil fork create --rpc-url <url> [--block N] [--out fork.json]
```

## nanvil fork prefetch

Pre-download contract storage from remote fork.

```
nanvil fork prefetch --manifest fork.json --contract <hash> [--cache-path dir]
```

## nanvil fork info

Display fork manifest metadata.

```
nanvil fork info --manifest fork.json
```

## nanvil policy sync

Display guidance for syncing Policy contract settings from remote (read-only helper).

```
nanvil policy sync --rpc-url <url>
```

## ncast

Cast-style CLI for transactions and contract calls. Built with `make build` as `./bin/ncast`.

| Flag / env | Default | Description |
|------------|---------|-------------|
| `--rpc` / `-r` | `http://127.0.0.1:8545` | RPC endpoint (`NCAST_RPC` or `NANVIL_RPC`) |
| `--json` | false | Raw JSON output |

Common commands: `balance`, `send`, `call`, `send-call`, `deploy`, `block`, `tx`, `storage`, `mempool`.
