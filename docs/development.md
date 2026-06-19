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
