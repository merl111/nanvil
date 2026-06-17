#!/bin/sh

die() {
	echo "$*"
	exit 1
}

find -name go.mod -print0 |
	xargs -0 -n1 grep -o 'pkg/interop v\S*' |
	uniq | wc -l |
	xargs -I{} -n1 [ 1 -eq {} ] ||
	die "Different versions for dependencies in go.mod"

INTEROP_COMMIT="$(sed -E -n -e 's/.*pkg\/interop.+-.+-(\w+)/\1/ p' go.mod)"
git merge-base --is-ancestor "$INTEROP_COMMIT" HEAD ||
	die "pkg/interop commit $INTEROP_COMMIT was not found in git"
