# Examples cookbook

Copy-paste workflows for common nanvil development tasks. Assumes a running node at `http://127.0.0.1:8545` unless noted.

## Environment setup

```bash
make build
./bin/nanvil start &
export NCAST_RPC=http://127.0.0.1:8545
```

Get dev account info:

```bash
./bin/ncast rpc nanvil_nodeInfo '[]'
# or
curl -s $NCAST_RPC -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_nodeInfo","params":[]}' | jq .
```

## Send GAS

Use account `(0)` from startup output:

```bash
# Check balance (replace with your dev address)
./bin/ncast balance NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg

# Send 1 GAS (amount is in whole GAS units)
./bin/ncast send --wif L2RGfeLD3ZU13yvgQ75VjSnw3bfAP4VUnGRa6NGYsEFXvMwi5GKa \
  NT67oPAtnqZMsWouA3GSumGtxGDYE4nh7F 1

# JSON output
./bin/ncast --json send --wif <wif> <to> 0.5
```

## Read-only contract calls

```bash
# GAS decimals
./bin/ncast call gas decimals

# NEP-17 balanceOf
./bin/ncast call gas balanceOf NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg

# JSON stack output
./bin/ncast --json call gas symbol
```

## Signed contract invocation

```bash
# NEP-17 transfer via send-call (from, to, amount, data)
./bin/ncast send-call --wif <wif> gas transfer \
  <from-address> <to-address> 100000000 null
```

Prefer `ncast send` for simple GAS transfers — it handles NEP-17 wiring for you.

## Mine blocks manually

When auto-mining is off (`--no-mining`) or you use interval mining:

```bash
# Mine one block
./bin/ncast rpc nanvil_mine '[1]'

# Mine five blocks
curl -s $NCAST_RPC -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_mine","params":[5]}'

# EVM alias
./bin/ncast rpc evm_mine '[]'
```

Toggle auto-mining:

```bash
./bin/ncast rpc nanvil_setAutomine '[false]'
./bin/ncast rpc nanvil_getAutomine '[]'
```

## Time manipulation

```bash
# Advance chain time by 3600 seconds and mine
./bin/ncast rpc nanvil_increaseTime '[3600]'

# EVM alias
./bin/ncast rpc evm_increaseTime '[86400]'

# Set absolute timestamp for next block
./bin/ncast rpc nanvil_setNextBlockTimestamp '[1893456000]'
```

## Impersonation

Register an address so transactions signed as that account pass `CheckWitness`:

```bash
ADDR=NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg
./bin/ncast rpc nanvil_impersonateAccount "[\"$ADDR\"]"

# Auto-impersonate all signers
./bin/ncast rpc nanvil_autoImpersonateAccount '[true]'

# Stop impersonating
./bin/ncast rpc nanvil_stopImpersonatingAccount "[\"$ADDR\"]"
```

On forked nodes, auto-impersonation is enabled by default.

## Snapshots and revert

```bash
# Create snapshot (returns snapshot id)
./bin/ncast rpc nanvil_snapshot '[]'

# Revert to snapshot (chain must be stopped in current implementation)
./bin/ncast rpc nanvil_revert '["0x1"]'
```

For hot testing, prefer restarting with `--data-dir` persistence instead of in-process revert.

## Mempool control

```bash
# List mempool
./bin/ncast mempool

# Drop a stuck transaction
./bin/ncast rpc nanvil_dropTransaction '["<txhash>"]'
```

## Deploy a contract

```bash
# Compile your contract first (neo-go compiler), then:
./bin/ncast deploy --wif <wif> \
  --nef contract.nef \
  --manifest contract.manifest.json

# Skip waiting for confirmation
./bin/ncast deploy --wif <wif> -i contract.nef -m contract.manifest.json --wait=false
```

## Fork mainnet state

```bash
# Create manifest at block 5,000,000 on testnet
./bin/nanvil fork create \
  --rpc-url http://seed1t5.neo.org:20332 \
  --block 5000000 \
  --out fork.json

# Start forked node
./bin/nanvil start \
  --fork-url http://seed1t5.neo.org:20332 \
  --fork-block-number 5000000

# Persist local deploys across restarts
./bin/nanvil start --data-dir ./nanvil-data \
  --fork-url http://seed1.neo.org:10332

# Prefetch a heavy contract before testing
./bin/nanvil fork prefetch --manifest fork.json --contract 0x<hash>
```

## Block and transaction inspection

```bash
./bin/ncast block-number
./bin/ncast block 0 --full
./bin/ncast tx 0x<hash> --verbose
./bin/ncast contract gas
./bin/ncast storage gas 0x0a
```

## Utilities

```bash
# Address ↔ script hash
./bin/ncast address NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg
./bin/ncast address 0x<scripthash>

# GAS unit conversion (8 decimals)
./bin/ncast to-datoshi 1.5
./bin/ncast from-datoshi 150000000

# Hash helpers
./bin/ncast hash --type sha256 hello
./bin/ncast hash --type script <33-byte-hex>
```

## Watch live events

```bash
./bin/ncast watch --lines 20
```

Streams block and transaction events over WebSocket (same feed the explorer uses).

## Load testing

```bash
./bin/ncast burst --wif <wif> --to <address> --count 100 --amount 0.001
```

See `ncast burst --help` for token and concurrency options.

## Explorer workflow

1. Start nanvil: `./bin/nanvil start`
2. Open `http://127.0.0.1:8546` — watch blocks and mempool live
3. Send a transaction with `ncast send`
4. Search the tx hash in the explorer search bar
5. Open `http://127.0.0.1:8546/docs/examples` for this guide in the browser
