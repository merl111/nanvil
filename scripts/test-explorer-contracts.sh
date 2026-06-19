#!/usr/bin/env bash
# test-explorer-contracts.sh — deploy a contract and verify explorer-style detection.
#
# The explorer Contracts tab lists non-native contracts by scanning block
# application logs for ContractManagement "Deploy" notifications (and deploy
# invocations). This script uses the same RPC checks.
#
# Usage:
#   ./scripts/test-explorer-contracts.sh
#   NCAST_RPC=http://127.0.0.1:8545 ./scripts/test-explorer-contracts.sh
#
# Requires a running nanvil node (start with: ./bin/nanvil start).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NCAST="${NCAST:-$ROOT/bin/ncast}"
COMPILE="${COMPILE:-$ROOT/bin/compilecontract}"
RPC="${NCAST_RPC:-http://127.0.0.1:8545}"
WIF="${NCAST_WIF:-L2RGfeLD3ZU13yvgQ75VjSnw3bfAP4VUnGRa6NGYsEFXvMwi5GKa}"
CONTRACT_SRC="${CONTRACT_SRC:-$ROOT/integration/testcontracts/explorer_deploy/main.go}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

log() { printf '→ %s\n' "$*"; }
ok()  { printf '✓ %s\n' "$*"; }
die() { printf '✗ %s\n' "$*" >&2; exit 1; }

rpc() {
  curl -sf "$RPC" -H 'Content-Type: application/json' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$1\",\"params\":${2:-[]}}"
}

build_tools() {
  log "building compilecontract"
  go build -o "$COMPILE" ./cmd/compilecontract/
  if [[ ! -x "$NCAST" ]]; then
    log "building ncast"
    go build -o "$NCAST" ./cmd/ncast/
  fi
  ok "tools ready"
}

wait_rpc() {
  for _ in $(seq 1 40); do
    if rpc getblockcount >/dev/null 2>&1; then
      ok "RPC up at $RPC"
      return
    fi
    sleep 0.25
  done
  die "nanvil RPC not reachable at $RPC (run: ./bin/nanvil start)"
}

compile_contract() {
  local manifest nef name
  name="ExplorerDeploy$(date +%s)"
  log "compiling $CONTRACT_SRC as $name"
  (cd "$TMPDIR" && "$COMPILE" --in "$CONTRACT_SRC" --out contract --name "$name")
  nef="$TMPDIR/contract.nef"
  manifest="$TMPDIR/contract.manifest.json"
  [[ -f "$nef" && -f "$manifest" ]] || die "compile output missing"
  ok "compiled contract $name"
}

deploy_contract() {
  local out tx contract
  log "deploying contract"
  out="$("$NCAST" --rpc "$RPC" deploy --wif "$WIF" \
    --nef "$TMPDIR/contract.nef" --manifest "$TMPDIR/contract.manifest.json")"
  tx="$(echo "$out" | awk '/^txhash:/{print $2}')"
  contract="$(echo "$out" | awk '/^contract:/{print $2}')"
  [[ -n "$tx" && -n "$contract" ]] || die "deploy failed: $out"
  DEPLOY_TX="$tx"
  DEPLOY_HASH="$contract"
  ok "deployed $contract (tx $tx)"
}

# Mirror explorer findDeployedContracts(): scan Deploy notifications in app logs.
find_deployed_contracts() {
  python3 - <<'PY' "$RPC"
import json, sys, urllib.request

rpc_url = sys.argv[1]

def rpc(method, params=None):
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params or []}).encode()
    req = urllib.request.Request(rpc_url, data=body, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read())
    if data.get("error"):
        raise RuntimeError(data["error"])
    return data["result"]

def norm(h):
    h = (h or "").lower().replace("0x", "")
    return h if len(h) == 40 else ""

def deployed_hash_from_notification(n):
    import base64
    state = n.get("state") or {}
    if state.get("type") != "Array":
        return ""
    vals = state.get("value") or []
    if not vals or vals[0].get("type") != "ByteString":
        return ""
    try:
        b = base64.b64decode(vals[0]["value"])
    except Exception:
        return ""
    if len(b) != 20:
        return ""
    return "0x" + b[::-1].hex()

natives = {norm(c.get("hash")) for c in (rpc("getnativecontracts") or [])}
count = rpc("getblockcount")
found = {}

for i in range(count):
    bh = rpc("getblockhash", [i])
    block = rpc("getblock", [bh, 1])
    for tx in block.get("tx") or []:
        if isinstance(tx, dict):
            if not tx.get("script"):
                continue
            th = tx.get("hash")
        else:
            th = tx
        if not th:
            continue
        try:
            log = rpc("getapplicationlog", [th])
        except Exception:
            continue
        for ex in log.get("executions") or []:
            for n in ex.get("notifications") or []:
                if (n.get("eventname") or "").lower() != "deploy":
                    continue
                contract_hash = deployed_hash_from_notification(n)
                h = norm(contract_hash)
                if not h or h in natives or h in found:
                    continue
                name = "Contract"
                try:
                    cs = rpc("getcontractstate", [contract_hash])
                    name = (cs.get("manifest") or {}).get("name") or name
                    contract_hash = cs.get("hash") or contract_hash
                except Exception:
                    pass
                found[h] = {"hash": contract_hash, "name": name}

for c in found.values():
    print(f"{c['hash']}\t{c['name']}")
PY
}

verify_deploy_listed() {
  local listed
  log "scanning chain for Deploy notifications (explorer logic)"
  listed="$(find_deployed_contracts)"
  echo "$listed" | grep -qi "$(echo "$DEPLOY_HASH" | tr 'A-Z' 'a-z')" \
    || die "deployed contract $DEPLOY_HASH not found in scan:\n$listed"
  ok "contract appears in deployed-contract scan"

  log "checking Deploy notification on deploy tx"
  rpc getapplicationlog "[\"$DEPLOY_TX\"]" | grep -qi '"eventname":"Deploy"' \
    || die "Deploy notification missing in application log"
  ok "Deploy notification present"

  log "calling deployed contract"
  local val
  val="$("$NCAST" --rpc "$RPC" --json call "$DEPLOY_HASH" getValue | python3 -c "
import json, sys, base64
d = json.load(sys.stdin)
stack = d.get('stack') or []
if not stack:
    sys.exit(1)
v = stack[0].get('value','')
try:
    print(base64.b64decode(v).decode())
except Exception:
    print(v)
")"
  [[ "$val" == "explorer-test-ok" ]] || die "getValue returned: $val"
  ok "contract getValue → $val"
}

main() {
  build_tools
  wait_rpc
  compile_contract
  deploy_contract
  verify_deploy_listed
  printf '\nAll explorer contract checks passed.\n'
  printf 'Open the explorer Contracts tab — deployed contract %s should be listed.\n' "$DEPLOY_HASH"
}

main "$@"
