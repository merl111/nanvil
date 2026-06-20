# Development

Nanvil lives in `cmd/nanvil`, `cmd/ncast`, and `pkg/nanvil/`. The rest of `pkg/` is the neo-go blockchain core this project is built on.

## Build

```bash
make build
```

`make build` runs `make sync-docs` first, copying `docs/` into `pkg/nanvil/explorer/embedded-docs/` for the explorer documentation browser.

## Test

```bash
make test
```

## Project layout

- `cmd/nanvil/` — dev node CLI
- `cmd/ncast/` — cast-style transaction CLI
- `pkg/nanvil/` — dev node, fork, RPC, producer
- `pkg/ncast/` — ncast library
- `config/protocol.nanvil.yml` — reference config
- `integration/` — end-to-end tests

## Contributing

Keep neo-go core patches minimal and behind dev-mode flags. Add tests and docs for new RPC methods.

## Publishing a release

Releases are created manually from GitHub Actions:

1. Merge changes to `main` and update `CHANGELOG.md`.
2. Open **Actions → Release → Run workflow**.
3. Enter a tag such as `v0.1.0` (must not already exist).
4. Optionally mark as draft or pre-release, then run.

The workflow runs tests, cross-compiles `nanvil`, `ncast`, and `nsmith` for Linux/macOS/Windows, pushes the tag, and publishes assets plus `SHA256SUMS` to GitHub Releases.

To build a single platform locally:

```bash
VERSION=v0.1.0 GOOS=linux GOARCH=amd64 ./scripts/package-release.sh
```
