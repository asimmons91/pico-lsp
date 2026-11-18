# Compressed-size fixtures

These fixtures validate the PICO-8-compatible pxa compressor in
`internal/p8compress` and the compressed-size budget.

- `*.lua` are UTF-8 source samples covering empty/raw, simple, repetitive,
  shorthand, high-entropy, UTF-8/P8SCII, and near-limit code.
- `sizes.json` records the expected P8SCII byte count and compressed size for
  each sample.
- `simple.lua.pxa.hex` and `utf8.lua.pxa.hex` are exact golden pxa streams for
  stream-format regressions.
- `corpus.json` records the compressed size of every fixture cart under
  `testdata/carts/`. It is a regression check for informational values, not a
  second source of truth for PICO-8 limits.

Expected sizes were generated with Lexaloffle's MIT-licensed reference
compressor (`pxa_compress_snippets.c` from
<https://github.com/dansanderson/lexaloffle>) after converting each UTF-8 sample
to P8SCII using shrinko8's character table. The Go port was differential-tested
byte-for-byte against that reference before the fixtures were recorded, and its
compressed sizes were cross-checked against the reference for all 51 carts in
`testdata/carts/`.

To regenerate the corpus sizes after an intentional compressor change, walk
`testdata/carts` with `cart.Loader.Load` and `Snapshot.CompressedSize`, then
update `corpus.json`. Do not regenerate them to make an unexplained regression
pass.
