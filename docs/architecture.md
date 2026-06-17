# Architecture

Nanvil is built on the [neo-go](https://github.com/nspcc-dev/neo-go) blockchain core and adds:

- `pkg/nanvil/` — dev node, fork bootstrap, RPC cheats
- `cmd/nanvil/` — dev node CLI
- `cmd/ncast/` — cast-style transaction CLI
- Patches to `witness.go`, `blockchain.go`, `rpcsrv/register.go`

## Components

- **DevNode** — in-memory chain, RPC server, optional explorer, account funding, state persistence
- **BlockProducer** — auto/interval/on-demand mining
- **Impersonation registry** — dev witness bypass
- **RemoteStateStore** — lazy StateService fetch + disk cache
- **TrackingOverlay** — local write overlay for fork mode
- **Chain snapshot** (`pkg/nanvil/persist`) — full storage dump/load via `--data-dir`

## Data flow

Local mode: `sendrawtransaction` → mempool → auto-mine → `AddBlock`

Fork mode: reads from remote via `findstates`/`getproof`, caches locally, local blocks append after branch.
