# Token and character fixtures

`counts.json` stores the expected PICO-8 token and character counts for the
corpus carts. Paths are relative to the repository root.

The counts were generated with
[shrinko8](https://github.com/thisismypassport/shrinko8) v1.1.2:

```sh
python3 shrinko8.py path/to/cart.p8 --count
```

`internal/cart/corpus_test.go` loads each cart and compares `Cart.Tokens` and
`Cart.Chars`. Carts with `#include` are excluded from the shrinko8 comparison
because shrinko8 counts the container source while the server counts the
flattened cart; include-count behavior is covered by dedicated tests.
