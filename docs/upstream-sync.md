# Upstream sync

Nanvil is a fork of [nspcc-dev/neo-go](https://github.com/nspcc-dev/neo-go).

## Sync procedure

1. Add upstream remote: `git remote add upstream https://github.com/nspcc-dev/neo-go.git`
2. Fetch: `git fetch upstream`
3. Merge or rebase: `git merge upstream/master`
4. Resolve conflicts in patched files:
   - `pkg/core/interop/runtime/witness.go`
   - `pkg/core/blockchain.go` (`verifyTxWitnesses`)
   - `pkg/services/rpcsrv/register.go`, `server.go`
   - `pkg/config/application_config.go`, `nanvil_config.go`
5. Run tests: `go test ./pkg/nanvil/... ./integration/...`
6. Run neo-go suite selectively if needed: `go test ./pkg/core/... -count=1 -short`

## Patch policy

Prefer extending `pkg/nanvil/` over modifying core. Core patches should remain dev-mode gated.
