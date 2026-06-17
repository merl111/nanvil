# Anvil comparison

| Anvil feature | Nanvil | Status |
|---------------|--------|--------|
| Local dev node | `nanvil start` | Supported |
| Prefunded accounts | `--accounts`, `--mnemonic` | Supported |
| Auto / interval mining | auto-mine, `--block-time` | Supported |
| `--fork-url` | `--fork-url`, StateService lazy fork | Supported |
| Impersonation | `nanvil_impersonateAccount` | Supported |
| Time travel | `nanvil_increaseTime` | Supported |
| Snapshots | `nanvil_snapshot` / `nanvil_revert` | Partial (in-memory height revert) |
| State dump/load | `--data-dir`, `--dump-state` / `--load-state` | Supported (full chain persistence) |
| Tracing | `--print-traces`, `getapplicationlog` | Supported |
| EVM cheatcodes | N/A | Not applicable |
| `hardhat_setCode` | N/A | Not supported |
| CREATE2 deployer | N/A | Not supported |

## Aliases

- `evm_mine` → `nanvil_mine`
- `evm_increaseTime` → `nanvil_increaseTime`
