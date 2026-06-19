# Getting started

## Install

```bash
git clone <nanvil-repo>
cd nanvil
make build
```

`make build` compiles `nanvil` and `ncast` into `./bin/` and syncs documentation into the explorer binary.

## Start a local node

```bash
./bin/nanvil start
```

This starts:

- In-memory single-validator Neo3 chain (magic `NANV`)
- JSON-RPC on `127.0.0.1:8545`
- Block explorer on `127.0.0.1:8546` (disable with `--no-explorer`)
- Embedded docs at `http://127.0.0.1:8546/docs/`
- 10 prefunded dev accounts (Anvil test mnemonic)
- Auto-mining on each submitted transaction

Startup prints validator and dev account addresses with WIF keys. Save them for signing transactions.

### Persist chain state

```bash
./bin/nanvil start --data-dir ./nanvil-data
```

On shutdown, nanvil writes `chain.state.json` into the data directory and restores it on the next start.

### Custom mining behaviour

```bash
# Mine every second (including empty blocks)
./bin/nanvil start --block-time 1s

# Mine every second only when the mempool has transactions
./bin/nanvil start --block-time 1s --no-mine-empty

# Disable auto-mining; mine manually via RPC
./bin/nanvil start --no-mining
```

## First transaction with ncast

```bash
export NCAST_RPC=http://127.0.0.1:8545

# List node info and dev accounts
./bin/ncast rpc nanvil_nodeInfo '[]'

# Check GAS balance
./bin/ncast balance <address-from-startup>

# Send 1 GAS (use WIF from account 0)
./bin/ncast send --wif <wif> <to-address> 1

# Confirm in explorer: http://127.0.0.1:8546
```

See [examples.md](examples.md) for more copy-paste workflows.

## Dev RPC examples

```bash
# Mine a block manually
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_mine","params":[1]}'

# Impersonate an address (dev witness bypass)
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_impersonateAccount","params":["N..."]}'

# Node info (accounts, fork metadata)
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_nodeInfo","params":[]}'

# Advance time by one hour and mine
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_increaseTime","params":[3600]}'
```

Full method list: [rpc-reference.md](rpc-reference.md).

## Fork workflow

Fork mainnet or testnet at a specific block height:

```bash
./bin/nanvil fork create --rpc-url http://seed1t5.neo.org:20332 --block 1000000 --out fork.json
./bin/nanvil start --fork-url http://seed1t5.neo.org:20332 --fork-block-number 1000000
```

Details: [forking.md](forking.md).

## Documentation in the browser

With the explorer running, open:

```
http://127.0.0.1:8546/docs/
```

Pages include CLI reference, RPC reference, examples cookbook, and architecture notes.

## Next steps

- [Examples cookbook](examples.md) — end-to-end recipes
- [CLI reference](cli-reference.md) — all flags and commands
- [Block explorer](explorer.md) — UI features and RPC proxy
- [Impersonation](impersonation.md) — testing contracts without real signatures
