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
| `--wif` | | Signing key (`NCAST_WIF`, `NANVIL_WIF`) — required for `send`, `send-call`, `deploy`, `burst` |

### Global examples

```bash
export NCAST_RPC=http://127.0.0.1:8545
./bin/ncast --json rpc getblockcount '[]'
```

### rpc

Raw JSON-RPC call.

```
ncast rpc <method> [params-json]
```

```bash
ncast rpc nanvil_nodeInfo '[]'
ncast rpc invokefunction '["0x...","balanceOf",[{"type":"Hash160","value":"..."}]]'
```

### block / block-number

```
ncast block <index|hash> [--full]
ncast block-number    # alias: height
```

```bash
ncast block 0
ncast block 0x<hash> --full
ncast block-number
```

### tx

```
ncast tx <hash> [--verbose]
```

```bash
ncast tx 0x<hash> --verbose
```

### balance

```
ncast balance <address> [--token gas]
```

```bash
ncast balance NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg
ncast balance <addr> --token <contract-hash-or-name>
```

### send

Send GAS or a NEP-17 token.

```
ncast send --wif <wif> <to> <amount> [--token gas]
```

Amount is in whole token units (GAS uses 8 decimal places internally).

```bash
ncast send --wif L2RG... <to-address> 1
ncast send --wif L2RG... <to> 0.5 --token gas
```

### call

Read-only contract invocation.

```
ncast call <contract> <method> [args...]
```

Arguments: addresses (`N...`), integers, booleans (`true`/`false`), hex (`0x...`), strings.

```bash
ncast call gas decimals
ncast call gas balanceOf NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg
```

### send-call

Signed state-changing contract call (alias: `invoke`).

```
ncast send-call --wif <wif> <contract> <method> [args...]
```

```bash
ncast send-call --wif <wif> gas transfer <from> <to> 100000000 null
```

### estimate

Gas estimation for a contract call.

```
ncast estimate <contract> <method> [args...]
```

### storage

Read contract storage item.

```
ncast storage <contract> <key>
```

```bash
ncast storage gas 0x0a
```

### contract

Contract state and manifest.

```
ncast contract <hash|name>
```

```bash
ncast contract gas
ncast contract 0x<hash>
```

### deploy

Deploy from compiled NEF + manifest (alias: `publish`).

```
ncast deploy --wif <wif> --nef <file.nef> --manifest <file.manifest.json> [--data json] [--wait] [--timeout]
```

```bash
ncast deploy --wif <wif> -i contract.nef -m contract.manifest.json
```

### burst

Bulk token transfers for load testing.

```
ncast burst --wif <wif> --to <address> --count N --amount <value> [options]
```

Run `ncast burst --help` for concurrency and token flags.

### mempool

List pending transactions.

```
ncast mempool
```

### chain-id

Print network magic.

```
ncast chain-id
```

### hash

Hash utilities.

```
ncast hash --type sha256|script <input>
```

```bash
ncast hash --type sha256 hello
```

### address

Convert Neo address ↔ script hash.

```
ncast address <address|0xhash>
```

### to-datoshi / from-datoshi

Convert GAS human units ↔ datoshi (10⁻⁸ GAS).

```
ncast to-datoshi 1.5
ncast from-datoshi 150000000
```

### watch

Stream live block and transaction events over WebSocket.

```
ncast watch [--lines N]
```

See [examples.md](examples.md) for full workflows.

