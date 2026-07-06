#!/usr/bin/env bash
#
# run.sh builds iso8583tool and the single-shot TCP mock server it ships
# (e2e/mock), exports the environment the specs expect, and runs the atago
# end-to-end suite (e2e/atago/*.atago.yaml) against the real binary.
#
# The test DEFINITIONS are atago YAML — this script is only the environment
# bootstrap (a plain shell program, not a test framework).
#
# Environment contract used by the specs:
#   PATH            iso8583tool and iso-mock (the e2e/mock server) resolve here
#   ISO_EXAMPLES    absolute path to the bundled examples/ fixtures
#   REPLY_HEX       hex of the 0810 network-echo response the mock replies with
#
# Usage: e2e/run.sh [atago args...]        (e.g. e2e/run.sh --filter send)
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

if ! command -v atago >/dev/null 2>&1; then
	echo "e2e: atago is not installed. Install it from https://github.com/nao1215/atago" >&2
	echo "e2e: e.g. 'go install github.com/nao1215/atago@latest' (CI uses nao1215/setup-atago)" >&2
	exit 127
fi

TMP="$(mktemp -d "${TMPDIR:-/tmp}/iso8583tool-e2e.XXXXXX")"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT
mkdir -p "$TMP/bin"

# COVER=1 (set by scripts/coverage.sh) builds a coverage-instrumented binary so
# a real E2E run contributes to coverage.out. The caller must also export
# GOCOVERDIR; atago passes it through to every iso8583tool child (no clear_env
# in the specs), so each writes its own covdata there. The DEFAULT path is left
# byte-for-byte identical, keeping `make e2e` fast.
if [ -n "${COVER:-}" ]; then
	echo "e2e: building coverage-instrumented iso8583tool and the mock server..."
	(cd "$REPO_ROOT" && env CGO_ENABLED=0 go build -cover -covermode=atomic -coverpkg=./... -o "$TMP/bin/iso8583tool" main.go)
else
	echo "e2e: building iso8583tool and the mock server..."
	(cd "$REPO_ROOT" && env CGO_ENABLED=0 go build -o "$TMP/bin/iso8583tool" main.go)
fi
(cd "$REPO_ROOT" && go build -o "$TMP/bin/iso-mock" ./e2e/mock)

# Put the e2e-built binaries first on PATH so the specs exercise them.
export PATH="$TMP/bin:$PATH"
export ISO_EXAMPLES="$REPO_ROOT/examples"
REPLY_HEX="$(tr -d ' \t\n\r' < "$ISO_EXAMPLES/basei/0810-network-echo-response.hex")"
export REPLY_HEX

echo "e2e: iso8583tool $("$TMP/bin/iso8583tool" version | head -1)"
# Extra args (e.g. --filter X) go before the path so the flag parser sees them.
atago run "$@" "$SCRIPT_DIR/atago"
