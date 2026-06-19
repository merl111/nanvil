# Block explorer

Nanvil ships a lightweight block explorer on port **8546** by default. It is enabled with `--with-explorer` (default) and disabled with `--no-explorer`.

## Open the explorer

```bash
./bin/nanvil start
# Explorer: http://127.0.0.1:8546
```

Custom bind:

```bash
./bin/nanvil start --explorer-host 0.0.0.0 --explorer-port 9000
```

## Features

### Live chain view

The header shows:

- **Height** — current block index
- **Mempool** — pending transaction count
- **Network** — chain magic (local dev chain uses `NANV`)
- **Live** — WebSocket connection indicator (green when connected)

### Tabs

| Tab | What you see |
|-----|----------------|
| **Blocks** | Recent blocks with hash, time, transaction count |
| **Transactions** | Recent transactions with type and status |
| **Contracts** | Native and deployed contracts with manifest summary |
| **Mempool** | Pending transactions waiting to be mined |

### Search

Use the search bar for:

- Block number (e.g. `42`)
- Transaction hash (with or without `0x` prefix)
- Contract script hash or native contract name (e.g. `gas`, `neo`)

### Detail pages

Click any row to open detail views:

- **Block** — header fields, paginated transaction list
- **Transaction** — signers, witnesses, invocations, application log, NEP-17 transfers
- **Contract** — manifest, methods, deployment info

### Live updates

The explorer connects to `/api/ws` and subscribes to:

- `block_added`
- `transaction_added`
- `mempool_event`
- `transaction_executed`

New blocks and transactions appear without refreshing the page.

## RPC proxy

The explorer does not talk to the node directly from the browser. It proxies through:

| Path | Purpose |
|------|---------|
| `POST /api/rpc` | JSON-RPC to the nanvil node |
| `GET /api/ws` | WebSocket relay to the node's `/ws` endpoint |

This avoids CORS issues and keeps the UI on a single origin.

Example via proxy:

```bash
curl -s http://127.0.0.1:8546/api/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"getblockcount","params":[]}'
```

## Documentation browser

Built-in docs are served at **`/docs/`** on the same port as the explorer.

```bash
# Overview
open http://127.0.0.1:8546/docs/

# Specific page
open http://127.0.0.1:8546/docs/getting-started
open http://127.0.0.1:8546/docs/examples
```

Docs are rendered from the repository `docs/` directory and embedded into the `nanvil` binary at build time. After editing markdown locally, run `make sync-docs && make build` to refresh the embedded copy.

## Disable the explorer

For headless CI or port-conflict scenarios:

```bash
./bin/nanvil start --no-explorer
```

Integration tests often use `--no-explorer` when binding RPC to port `0`.
