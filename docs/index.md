# Nanvil documentation

Nanvil is a Neo3 local development node inspired by [Foundry Anvil](https://getfoundry.sh/anvil/). Use it for instant chains, prefunded accounts, mainnet/testnet forking, dev RPC cheats, and a built-in block explorer.

## Quick links

| Topic | Description |
|-------|-------------|
| [Getting started](getting-started.md) | Install, start a node, first transaction |
| [Examples cookbook](examples.md) | End-to-end workflows with copy-paste commands |
| [CLI reference](cli-reference.md) | `nanvil` and `ncast` flags and commands |
| [RPC reference](rpc-reference.md) | `nanvil_*` JSON-RPC methods |
| [Block explorer](explorer.md) | Web UI, live updates, docs browser |
| [Forking](forking.md) | Mainnet / testnet fork workflow |
| [Impersonation](impersonation.md) | Dev witness bypass |
| [State management](state-management.md) | Snapshots, persistence, reset |
| [Tracing](tracing.md) | Invocation logs and debugging |
| [Architecture](architecture.md) | Components and data flow |
| [Anvil comparison](anvil-comparison.md) | vs Foundry Anvil |
| [Fork troubleshooting](fork-troubleshooting.md) | Common fork issues |
| [Development](development.md) | Build, test, contribute |
| [Upstream sync](upstream-sync.md) | neo-go fork maintenance |

## Default services

When you run `./bin/nanvil start`:

| Service | URL |
|---------|-----|
| JSON-RPC | `http://127.0.0.1:8545` |
| Block explorer | `http://127.0.0.1:8546` |
| Documentation | `http://127.0.0.1:8546/docs/` |

## Default dev accounts

Mnemonic (10 accounts, 10,000 GAS each):

```
test test test test test test test test test test test junk
```

Query accounts at any time:

```bash
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_nodeInfo","params":[]}'
```

Or open the explorer and check the terminal output from `nanvil start`.
