# Getting started

## Install

```bash
git clone <nanvil-repo>
cd nanvil
make build
```

## Start a local node

```bash
./bin/nanvil start
```

This starts:

- In-memory single-validator Neo3 chain (magic `NANV`)
- JSON-RPC on `127.0.0.1:8545`
- Block explorer on `127.0.0.1:8546` (disable with `--no-explorer`)
- 10 prefunded dev accounts
- Auto-mining on each submitted transaction

Persist chain state across restarts:

```bash
./bin/nanvil start --data-dir ./nanvil-data
```

## Send a transaction

Use `ncast` against the nanvil RPC:

```bash
export NCAST_RPC=http://127.0.0.1:8545
./bin/ncast balance <address>
```

## Dev RPC examples

```bash
# Mine a block manually
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_mine","params":[1]}'

# Impersonate an address
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_impersonateAccount","params":["N..."]}'

# Node info (accounts, fork metadata)
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_nodeInfo","params":[]}'
```

## Fork workflow

See [forking.md](forking.md).
