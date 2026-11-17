#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
binary="${PICO_LSP_BIN:-$root/pico-lsp}"

if [[ ! -x "$binary" ]]; then
  echo "pico-lsp binary not found: $binary" >&2
  echo "build one with: CGO_ENABLED=0 go build -o pico-lsp ./cmd/pico-lsp" >&2
  exit 1
fi

PICO_LSP_ROOT="$root" PICO_LSP_BIN="$binary" \
  nvim --headless --clean -u NONE \
    --cmd "set runtimepath+=$root/editors/nvim" \
    -l "$root/editors/nvim/tests/helper.lua"
