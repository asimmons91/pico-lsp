#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
ts="${TREE_SITTER:-tree-sitter}"

if ! command -v "$ts" >/dev/null 2>&1; then
  echo "tree-sitter CLI not found; install 0.24.x or set TREE_SITTER" >&2
  exit 1
fi

version="$("$ts" --version)"
case "$version" in
  "tree-sitter 0.24."*) ;;
  *)
    echo "tree-sitter 0.24.x is required for the committed ABI 14 parser; got: $version" >&2
    exit 1
    ;;
esac

cd "$here"

echo "==> generate"
"$ts" generate

if ! git diff --quiet -- src; then
  echo "generated parser sources are out of date:" >&2
  git diff --stat -- src >&2
  exit 1
fi

echo "==> grammar tests"
"$ts" test

echo "==> query validation"
sample="$here/.query-sample.p8"
trap 'rm -f "$sample"' EXIT
cp "$repo/testdata/smoke.p8" "$sample"
for query in queries/highlights.scm queries/indents.scm queries/textobjects.scm; do
  "$ts" query "$query" "$sample" >/dev/null
done

echo "==> highlight golden"
captures="$(mktemp)"
"$ts" query --captures queries/highlights.scm "$here/test/golden/sample.p8" 2>/dev/null \
  | sed -n 's/.*capture: [0-9]* - \([^,]*\), start: (\([0-9]*\), \([0-9]*\)), end: (\([0-9]*\), \([0-9]*\)), text: `\(.*\)`/\1 \2:\3-\4:\5 `\6`/p' \
  >"$captures"
if ! diff -u test/golden/captures.txt "$captures"; then
  echo "highlight captures changed; update test/golden/captures.txt" >&2
  rm -f "$captures"
  exit 1
fi
rm -f "$captures"

echo "==> corpus parse"
failed=0
total=0
while IFS= read -r -d '' file; do
  total=$((total + 1))
  if ! "$ts" parse --quiet "$file" >/dev/null 2>&1; then
    echo "parse failed: $file" >&2
    failed=$((failed + 1))
  fi
done < <(find "$repo/testdata" -name '*.p8' -print0)
if [ "$failed" -ne 0 ]; then
  echo "$failed of $total carts failed to parse" >&2
  exit 1
fi
echo "parsed $total carts"

echo "==> truncated cart safety"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"; rm -f "$sample"' EXIT
cart="$repo/testdata/carts/brubsby__tweetcarts/constellations.p8"
for size in 64 512 4096; do
  head -c "$size" "$cart" >"$tmp/truncated.p8"
  status=0
  timeout 5 "$ts" parse --quiet --timeout 2000000 "$tmp/truncated.p8" >/dev/null 2>&1 || status=$?
  case "$status" in
    0|1) ;;
    124)
      echo "parser timed out on a $size-byte cart" >&2
      exit 1
      ;;
    *)
      echo "parser crashed on a $size-byte cart (exit $status)" >&2
      exit 1
      ;;
  esac
done

echo "grammar checks passed"
