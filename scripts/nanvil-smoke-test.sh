#!/usr/bin/env bash
# nanvil-smoke-test.sh — integration smoke tests for nanvil + ncast
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NCAST="${NCAST:-$ROOT/bin/ncast}"
NANVIL="${NANVIL:-$ROOT/bin/nanvil}"
NEO_GO="${NEO_GO:-$ROOT/bin/neo-go}"
RPC="${NCAST_RPC:-http://127.0.0.1:8545}"
EXPLORER="${NANVIL_EXPLORER:-http://127.0.0.1:8546}"

# Dev account #0 from default nanvil mnemonic
WIF="${NCAST_WIF:-L2RGfeLD3ZU13yvgQ75VjSnw3bfAP4VUnGRa6NGYsEFXvMwi5GKa}"
SENDER="${NCAST_SENDER:-NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg}"
RECEIVER="${NCAST_RECEIVER:-NT67oPAtnqZMsWouA3GSumGtxGDYE4nh7F}"
WIF_ALT="${NCAST_WIF_ALT:-L1UzBy1UxBDj4erZiRt15fsiFnHUPDEPtH6tjDtXzA72mFU479dp}"

# Bulk transfer load test (set TRANSFER_COUNT=0 to skip)
TRANSFER_COUNT="${TRANSFER_COUNT:-3000}"
TRANSFER_PARALLEL="${TRANSFER_PARALLEL:-16}"
TRANSFER_AMOUNT="${TRANSFER_AMOUNT:-0.00000001}"
TRANSFER_PACED="${TRANSFER_PACED:-1}"

NANVIL_PID=""
STARTED_NANVIL=0
PASS=0
FAIL=0

log()  { printf '\033[36m→\033[0m %s\n' "$*"; }
ok()   { PASS=$((PASS + 1)); printf '\033[32m✓\033[0m %s\n' "$*"; }
die()  { FAIL=$((FAIL + 1)); printf '\033[31m✗\033[0m %s\n' "$*" >&2; exit 1; }
step() { printf '\n\033[1m━━ %s ━━\033[0m\n' "$*"; }

run() {
  log "$*"
  "$@" || die "command failed: $*"
}

build_bins() {
  step "Build tools"
  if [[ ! -x "$NANVIL" ]]; then
    log "building nanvil"
    go build -o "$NANVIL" ./cmd/nanvil
  fi
  if [[ ! -x "$NCAST" ]]; then
    log "building ncast"
    go build -o "$NCAST" ./cmd/ncast
  fi
  if [[ ! -x "$NEO_GO" ]]; then
    log "building neo-go (for contract compile)"
    go build -o "$NEO_GO" ./cli
  fi
  ok "binaries ready"
}

rpc_up() {
  curl -sf "$RPC" -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","id":1,"method":"getblockcount","params":[]}' >/dev/null 2>&1
}

explorer_up() {
  curl -sf "$EXPLORER/" | grep -q 'Nanvil Explorer' && \
    curl -sf "$EXPLORER/api/rpc" -H 'Content-Type: application/json' \
      -d '{"jsonrpc":"2.0","id":1,"method":"getblockcount","params":[]}' | grep -q '"result"'
}

kill_rpc_listener() {
  local port pid
  for port in 8545 8546; do
    pid="$(ss -tlnp 2>/dev/null | awk "/127\\.0\\.0\\.1:${port}/{print}" | sed -n 's/.*pid=\([0-9]*\).*/\1/p' | head -1)"
    if [[ -n "$pid" ]]; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  for _ in $(seq 1 30); do
    rpc_up || break
    sleep 0.1
  done
}

start_nanvil() {
  if rpc_up; then
    local pending
    pending="$(NCAST_WIF="$WIF" "$NCAST" --rpc "$RPC" mempool 2>/dev/null | wc -l | tr -d ' ')"
    if (( pending == 0 )); then
      log "nanvil already running at $RPC"
      return
    fi
    log "restarting nanvil (stale mempool: $pending txs)"
    kill_rpc_listener
  fi
  log "starting nanvil (with explorer)"
  "$NANVIL" start --with-explorer >/tmp/nanvil-smoke.log 2>&1 &
  NANVIL_PID=$!
  STARTED_NANVIL=1
  for _ in $(seq 1 40); do
    if rpc_up && explorer_up; then
      ok "nanvil started (pid $NANVIL_PID)"
      return
    fi
    sleep 0.25
  done
  cat /tmp/nanvil-smoke.log >&2 || true
  die "nanvil failed to start"
}

stop_nanvil() {
  if [[ "$STARTED_NANVIL" -eq 1 && -n "$NANVIL_PID" ]]; then
    log "stopping nanvil (pid $NANVIL_PID)"
    kill "$NANVIL_PID" 2>/dev/null || true
    wait "$NANVIL_PID" 2>/dev/null || true
  fi
}

ncast() {
  NCAST_WIF="$WIF" "$NCAST" --rpc "$RPC" "$@"
}

compile_contract() {
  local tmpdir
  tmpdir="$(mktemp -d)"
  CONTRACT_NEF="$tmpdir/contract.nef"
  CONTRACT_MANIFEST="$tmpdir/contract.manifest.json"
  log "compiling test deploy contract"
  run "$NEO_GO" contract compile \
    --in cli/smartcontract/testdata/deploy/main.go \
    --config cli/smartcontract/testdata/deploy/neo-go.yml \
    --out "$CONTRACT_NEF" \
    --manifest "$CONTRACT_MANIFEST"
  python3 -c "
import json, time
with open('$CONTRACT_MANIFEST') as f:
    m = json.load(f)
m['name'] = f\"TestDeploy{int(time.time())}\"
with open('$CONTRACT_MANIFEST', 'w') as f:
    json.dump(m, f)
"
  ok "contract compiled"
}

test_chain_basics() {
  step "Chain basics"
  local height network
  height="$(ncast block-number)"
  [[ "$height" =~ ^[0-9]+$ ]] || die "invalid block-number: $height"
  ok "block-number: $height"

  network="$(ncast chain-id | head -1)"
  [[ "$network" == network:* ]] || die "unexpected chain-id: $network"
  ok "$network"

  ncast rpc getblockcount '[]' >/dev/null
  ok "raw rpc getblockcount"

  ncast block "$height" >/dev/null
  ok "block $height"

  if explorer_up; then
    ok "explorer UI + RPC proxy at $EXPLORER"
  else
    log "explorer not reachable at $EXPLORER (skipped — start with --with-explorer)"
  fi
}

test_balances_and_transfer() {
  step "Balances & GAS transfer"
  local bal_before bal_after tx
  bal_before="$(ncast balance "$RECEIVER")"
  ok "receiver balance before: $bal_before"

  tx="$(ncast send --wif "$WIF" "$RECEIVER" 1.5)"
  [[ "$tx" =~ ^(0x)?[0-9a-f]{64}$ ]] || die "send returned: $tx"
  ok "sent 1.5 GAS → $tx"

  for _ in $(seq 1 30); do
    bal_after="$(ncast balance "$RECEIVER")"
    [[ "$bal_after" != "$bal_before" ]] && break
    ncast rpc nanvil_mine '[1]' >/dev/null 2>&1 || true
    sleep 0.05
  done

  ncast tx "$tx" >/dev/null
  ok "tx lookup"

  bal_after="$(ncast balance "$RECEIVER")"
  [[ "$bal_after" != "$bal_before" ]] || die "balance unchanged after transfer"
  ok "receiver balance after: $bal_after"
}

json_stack() {
  python3 -c '
import sys, json, base64
d = json.load(sys.stdin)
stack = d.get("stack") or []
if not stack:
    print("")
    sys.exit(0)
item = stack[0]
val = item.get("value", "")
typ = item.get("type", "")
if typ in ("ByteString", "Buffer", "String") and val:
    try:
        decoded = base64.b64decode(val).decode()
        if decoded.isprintable() or "|" in decoded:
            print(decoded)
            sys.exit(0)
    except Exception:
        pass
print(val)
'
}

test_read_calls() {
  step "Read-only contract calls"
  local symbol decimals
  symbol="$(ncast --json call gas symbol | json_stack)"
  [[ "$symbol" == *GAS* ]] || die "gas symbol: $symbol"
  ok "gas.symbol → $symbol"

  decimals="$(ncast --json call gas decimals | json_stack)"
  [[ "$decimals" == "8" ]] || die "gas decimals: $decimals"
  ok "gas.decimals → $decimals"

  ncast estimate --wif "$WIF" gas symbol >/dev/null
  ok "estimate gas.symbol"
}

test_deploy_and_invoke() {
  step "Contract deploy & invoke"
  local out tx contract val
  compile_contract

  out="$(ncast deploy --wif "$WIF" --nef "$CONTRACT_NEF" --manifest "$CONTRACT_MANIFEST")"
  tx="$(echo "$out" | awk '/^txhash:/{print $2}')"
  contract="$(echo "$out" | awk '/^contract:/{print $2}')"
  [[ -n "$tx" && -n "$contract" ]] || die "deploy output: $out"
  ok "deployed contract $contract (tx $tx)"

  ncast contract "$contract" >/dev/null
  ok "contract state"

  val="$(ncast --json call "$contract" getValue | json_stack)"
  [[ "$val" == *on\ create* ]] || die "getValue: $val"
  ok "getValue → $val"

  ncast storage "$contract" 6b6579 >/dev/null 2>&1 || true
  ok "storage read (key)"
}

test_mempool_and_utils() {
  step "Mempool & utilities"
  ncast mempool >/dev/null || true
  ok "mempool"

  ncast to-datoshi 1.5 | grep -q 150000000
  ok "to-datoshi"

  ncast from-datoshi 100000000 | grep -q 1
  ok "from-datoshi"

  local addr
  addr="$(ncast address "$SENDER")"
  [[ "$addr" =~ ^(0x)?[0-9a-f]{40}$ ]] || die "address conversion: $addr"
  ok "address → $addr"
}

settle_chain() {
  local tries=0 max_tries=500
  while (( tries < max_tries )); do
    local pending
    pending="$(ncast mempool 2>/dev/null | wc -l | tr -d ' ')"
    if (( pending == 0 )); then
      return 0
    fi
    ncast rpc nanvil_mine '[1]' >/dev/null 2>&1 || true
    tries=$((tries + 1))
    sleep 0.02
  done
  die "mempool not empty after settle ($(ncast mempool 2>/dev/null | wc -l) pending)"
}

test_bulk_transfers() {
  if [[ "$TRANSFER_COUNT" -le 0 ]]; then
    log "bulk transfers skipped (TRANSFER_COUNT=$TRANSFER_COUNT)"
    return
  fi

  step "Bulk transfers ($TRANSFER_COUNT txs via ncast burst)"
  local height_before height_after elapsed blocks_mined
  height_before="$(ncast block-number)"

  local burst_args=(burst --wif "$WIF" --wif-alt "$WIF_ALT" --to "$RECEIVER" --to-alt "$SENDER"
    --count "$TRANSFER_COUNT" --parallel "$TRANSFER_PARALLEL" --amount "$TRANSFER_AMOUNT")
  if [[ "$TRANSFER_PACED" == 1 ]]; then
    burst_args+=(--paced)
  fi

  local burst_out
  burst_out="$(ncast "${burst_args[@]}" 2>&1)" || die "burst failed: $burst_out"
  echo "$burst_out" | while IFS= read -r line; do log "$line"; done
  ok "burst completed"

  if [[ "$TRANSFER_PACED" != 1 ]]; then
    log "settling chain (mining pending mempool txs)…"
    settle_chain
    ok "mempool drained"
  else
    pending="$(ncast mempool 2>/dev/null | wc -l | tr -d ' ')"
    (( pending == 0 )) || die "mempool not empty after paced burst ($pending pending)"
    ok "mempool empty after paced burst"
  fi

  height_after="$(ncast block-number)"
  blocks_mined=$((height_after - height_before))
  (( blocks_mined >= 1 )) || die "chain did not grow (still at height $height_after)"
  ok "chain grew $blocks_mined blocks ($height_before → $height_after)"
}

main() {
  trap stop_nanvil EXIT

  build_bins
  start_nanvil

  test_chain_basics
  test_balances_and_transfer
  test_bulk_transfers
  test_read_calls
  test_deploy_and_invoke
  test_mempool_and_utils

  printf '\n\033[1m\033[32mAll smoke tests passed\033[0m (%d checks)\n' "$PASS"
}

main "$@"
