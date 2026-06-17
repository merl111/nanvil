# RPC reference

Nanvil adds dev JSON-RPC methods alongside standard neo-go RPC.

## nanvil_mine

Mine one or more blocks.

**Params:** `[blocks]` (optional, default 1)

**Example:**

```json
{"jsonrpc":"2.0","id":1,"method":"nanvil_mine","params":[2]}
```

**Alias:** `evm_mine`

## nanvil_setAutomine / nanvil_getAutomine

Enable or query auto-mining on transaction relay.

```json
{"jsonrpc":"2.0","id":1,"method":"nanvil_setAutomine","params":[true]}
```

## nanvil_impersonateAccount

Register an address for dev witness bypass (see [impersonation.md](impersonation.md)).

```json
{"jsonrpc":"2.0","id":1,"method":"nanvil_impersonateAccount","params":["NV..."]}
```

## nanvil_stopImpersonatingAccount

Remove an impersonated address.

## nanvil_autoImpersonateAccount

Enable/disable auto-impersonation for all signers.

```json
{"jsonrpc":"2.0","id":1,"method":"nanvil_autoImpersonateAccount","params":[true]}
```

## nanvil_increaseTime

Advance block timestamp and mine one block.

**Params:** `[seconds]`

**Alias:** `evm_increaseTime`

## nanvil_setNextBlockTimestamp

Set absolute timestamp for the next mined block.

## nanvil_setBalance

Not implemented. Returns an error directing callers to fund accounts via GAS transfer from prefunded dev/validator accounts.

## nanvil_snapshot / nanvil_revert

Create and revert to named snapshots (height-based; chain must be stopped for revert).

## nanvil_reset

Reset chain (requires restart; use snapshots for hot revert).

## nanvil_dropTransaction

Remove a transaction from the mempool by hash.

## nanvil_nodeInfo

Return dev accounts, fork metadata, and cache stats.

## Standard neo-go RPC

All standard methods work: `invokefunction`, `sendrawtransaction`, `getblockcount`, `invokefunctionhistoric`, etc.
