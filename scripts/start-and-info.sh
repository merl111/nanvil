#!/usr/bin/env bash
# Example: start nanvil and query node info
set -euo pipefail
nanvil start --port 8545 &
PID=$!
trap 'kill $PID' EXIT
sleep 1
curl -s http://127.0.0.1:8545 -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"nanvil_nodeInfo","params":[]}'
