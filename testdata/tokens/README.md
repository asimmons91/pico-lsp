# Token and character fixtures

`counts.json` stores the expected PICO-8 token and character counts for the
corpus carts. Paths are relative to the repository root.

The counts were generated with
[shrinko8](https://github.com/thisismypassport/shrinko8) v1.1.2:

```sh
python3 shrinko8.py path/to/cart.p8 --count
```

`internal/cart/corpus_test.go` loads each cart and compares `Cart.Tokens` and
`Cart.Chars`. shrinko8 expands includes recursively while the server follows
PICO-8's non-recursive rule, so include-bearing fixtures must not contain nested
includes. The PECS example satisfies that requirement and exercises include
flattening, tabs, and source maps in the golden set.
