# Configuration

pico-lsp reads configuration from the `pico8` section using
`workspace/configuration` when the client supports pulls, falling back to
`initializationOptions.pico8` and `workspace/didChangeConfiguration`.

```json
{
  "pico8": {
    "apiVersion": "0.2.7",
    "typeChecking": "basic",
    "inlayHints": false,
    "maxProblems": 100,
    "limits": {
      "codeLens": true,
      "diagnostics": true
    },
    "diagnostics": {
      "undefinedGlobal": "warning",
      "unusedLocal": "off",
      "unreachableCode": "hint",
      "duplicateTableField": "hint",
      "callbackPrecedence": "hint",
      "callbackParams": "warning",
      "noStdlib": "hint",
      "shadowedLocal": "off",
      "emptyBlock": "off",
      "globalInFunction": "off",
      "includeOutsideWorkspace": "warning",
      "includeUnusualType": "hint",
      "includeDuplicate": "off"
    },
    "lint": {
      "apiRanges": false
    }
  }
}
```

## Settings

| Setting | Values | Default | Meaning |
|---|---|---|---|
| `apiVersion` | `0.2.7` | `0.2.7` | PICO-8 API version. |
| `typeChecking` | `off`, `basic`, `strict` | `basic` | Type-diagnostic mode. |
| `inlayHints` | boolean | `false` | Show inferred local type hints. |
| `maxProblems` | integer | `100` | Per-document diagnostic cap; `0` disables truncation. |
| `limits.codeLens` | boolean | `true` | Show the cart budget code lens. |
| `limits.diagnostics` | boolean | `true` | Publish token/char limit diagnostics. |
| `lint.apiRanges` | boolean | `false` | Optional literal API range checks. |

Diagnostic severity values are `off`, `hint`, `info`, `warning`, and `error`.
Set a rule to `off` to disable it. Syntax errors and missing includes remain
errors and cannot be disabled globally.

Settings changes are applied without restarting the server. Open carts are
re-analyzed and diagnostics are republished.

See [diagnostics.md](diagnostics.md) for the complete code list and
[api.md](api.md) for `pico8/cartInfo` and `pico8.showCartInfo`.
