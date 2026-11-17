# Cart corpus

These carts are unmodified test fixtures for the parser, token counter, and LSP features.
Copyright belongs to the original authors. Each source repository license is preserved in
`licenses/`. Carts are included only where the source repository declares a permissive
license (MIT in this corpus).

## Sources

| Repository | License | Commit | Carts |
|---|---|---|---|
| [0xcafed00d/tac08](https://github.com/0xcafed00d/tac08) | MIT | `38c1f3bb2ae81c06983dc5132d45f0c8747dff95` | 20 |
| [apa64/try-pico8](https://github.com/apa64/try-pico8) | MIT | `21bdc11a123db0626e304ef8e5ba9e483c080440` | 18 |
| [brubsby/tweetcarts](https://github.com/brubsby/tweetcarts) | MIT | `7295bd5a291475c9990a431df3b6db88f9b0934b` | 12 |

## Manifest

| Path | Source |
|---|---|
| `0xcafed00d__tac08/cart.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/apitest1.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/btntest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/coroutine.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/exfont.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/expal.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/font.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/fstest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/geomtest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/mousetest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/oldfont.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/paltest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/sfxmusictest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/sfxtest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/soundlooptest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/sspr.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/stattest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/time.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/touchtest.p8` | 0xcafed00d/tac08 |
| `0xcafed00d__tac08/tests/zoomtest.p8` | 0xcafed00d/tac08 |
| `apa64__try-pico8/anim1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/closures1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/colourful_circles1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/draw1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/factorial1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/fn_as_arg.p8` | apa64/try-pico8 |
| `apa64__try-pico8/func1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/greenfield1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/helloworld1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/oop-topdown1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/oop1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/operators1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/tables1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/tables2.p8` | apa64/try-pico8 |
| `apa64__try-pico8/tictactoe1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/topdown1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/trajectory1.p8` | apa64/try-pico8 |
| `apa64__try-pico8/trigonometry1.p8` | apa64/try-pico8 |
| `brubsby__tweetcarts/10tropy.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/2chet.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/beach_day.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/bresenham_bounce.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/constellations.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/fade_meteor.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/furnace.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/helio.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/howdy.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/ripples.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/spiral_sphere.p8` | brubsby/tweetcarts |
| `brubsby__tweetcarts/truchet.p8` | brubsby/tweetcarts |

## Token-count oracle

The golden token and character counts in `../tokens/` were generated with
[shrinko8](https://github.com/thisismypassport/shrinko8) v1.1.2 using:

```sh
python3 shrinko8.py path/to/cart.p8 --count
```

Only carts without `#include` are used for shrinko8 count fixtures, because the
assembled-cart counter intentionally flattens includes before counting.
