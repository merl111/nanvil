# Contributing

Nanvil is derived from [neo-go](https://github.com/nspcc-dev/neo-go). Keep changes to the shared `pkg/core` and related libraries minimal and behind dev-mode flags where possible.

1. Open an issue or discuss the change before large features.
2. Add tests under `pkg/nanvil/` or `integration/` for new behavior.
3. Run `make test` and `make vet` before opening a PR.
4. Update docs in `docs/` when adding CLI flags or RPC methods.
