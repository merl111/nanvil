#!/usr/bin/env bash
# package-release.sh — build nanvil, ncast, and nsmith for one platform and archive.
#
# Usage:
#   VERSION=v0.1.0 GOOS=linux GOARCH=amd64 OUT_DIR=dist ./scripts/package-release.sh
set -euo pipefail

VERSION="${VERSION:?VERSION is required (e.g. v0.1.0)}"
GOOS="${GOOS:?GOOS is required}"
GOARCH="${GOARCH:?GOARCH is required}"
OUT_DIR="${OUT_DIR:-dist}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TAG="${VERSION#v}"
NAME="nanvil-${TAG}-${GOOS}-${GOARCH}"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

cd "$ROOT"
make sync-docs >/dev/null

GO_LDFLAGS="-s -w"
EXT=""
if [[ "$GOOS" == "windows" ]]; then
	EXT=".exe"
fi

for cmd in nanvil ncast nsmith; do
	GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build -trimpath -ldflags="$GO_LDFLAGS" \
		-o "$STAGE/${cmd}${EXT}" "./cmd/${cmd}/"
done

if [[ -f LICENSE.md ]]; then
	cp LICENSE.md "$STAGE/LICENSE.md"
fi

mkdir -p "$OUT_DIR"
if [[ "$GOOS" == "windows" ]]; then
	(
		cd "$STAGE"
		zip -rq "$OUT_DIR/${NAME}.zip" .
	)
	echo "$OUT_DIR/${NAME}.zip"
else
	tar -czf "$OUT_DIR/${NAME}.tar.gz" -C "$STAGE" .
	echo "$OUT_DIR/${NAME}.tar.gz"
fi
