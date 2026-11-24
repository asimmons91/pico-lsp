# PICO-8 workflow

`pico-lsp` analyzes the same `.p8` and `.lua` files that PICO-8 loads. PICO-8
does not need to be running for the server to work, but keeping the two in sync
requires a small amount of care.

This guide targets PICO-8 0.2.7. Verify paths and behavior against your PICO-8
version before relying on them.

## Where PICO-8 stores carts

PICO-8 calls its cartridge folder the drive. The defaults are:

| OS | Default cart folder |
|---|---|
| Windows | `C:\Users\<you>\AppData\Roaming\pico-8\carts` |
| macOS | `/Users/<you>/Library/Application Support/pico-8/carts` |
| Linux | `~/.lexaloffle/pico-8/carts` |

Inside PICO-8, run `FOLDER` to open the current folder in your operating
system's file browser. `LS` and `CD` navigate the same virtual drive.

If the folder is not where you expect, check `config.txt` in the matching
`pico-8` application-data folder, or launch PICO-8 with `-root_path PATH`. The
`-home` switch changes where PICO-8 stores `config.txt` and other user data.

## Edit a cart outside PICO-8

PICO-8 can edit `.p8` files directly with another text editor. There is no
separate "attach" mode to enable.

1. Create or save the cart in PICO-8 so the `.p8` file exists.
2. Run `FOLDER` in PICO-8, or open the default cart folder from the table above.
3. Open the `.p8` file in VS Code, Neovim, or Helix.
4. Edit and save the file.
5. Switch to PICO-8 and press `CTRL-R` to reload and run the cart.

PICO-8 reloads the file on `CTRL-R` only when both conditions hold:

- there are no unsaved changes in the PICO-8 editors, and
- the file differs from the last loaded version.

If both the file on disk and PICO-8's in-memory cart changed, PICO-8 shows:

```
DIDN'T RELOAD; UNSAVED CHANGES
```

Decide which copy to keep before saving in either place. Running `INFO` in
PICO-8 reports `EXTERNAL CHANGES` when the file on disk is newer than the loaded
cart.

### Save order

The safe order is:

1. Save in the external editor.
2. Reload or run in PICO-8 with `CTRL-R`.
3. Only then use `CTRL-S` or `SAVE` in PICO-8.

Saving from PICO-8 while it still holds an older buffer overwrites the external
edit. If that happens, PICO-8 keeps a backup; see [Backups](#backups).

## Work with `#include` files

`#include` lets a cart compose code from `.lua` files or other carts:

```lua
-- main.p8
#include lib/player.lua
#include lib/enemies.lua
```

Rules that matter for editor support:

- Paths are relative to the cart.
- Includes are not recursive. A `.lua` file cannot include another file.
- Normal token and character limits apply to the flattened program.
- PICO-8 reads the current include contents each time the cart runs, so
  `CTRL-R` picks up external `.lua` edits without re-saving the cart.
- Saving a cart as `.p8.png` flattens includes into the image. The exported PNG
  no longer depends on the `.lua` files.
- A `.lua` file is owned by the nearest ancestor `.p8` cart that includes it.
  This is how `pico-lsp` attaches to includes and avoids treating unrelated Lua
  files as PICO-8 code.

For the best experience, open the owning `.p8` cart as well as the `.lua` file.
The VS Code extension attaches workspace `.lua` files automatically. The
`pico8.nvim` plugin provides the cart-owned attachment helper for Neovim. Helix
does not ship an include-attachment helper in v1; open the owning cart for full
analysis.

## Read-only `.p8.png` carts

PICO-8 saves `.p8.png` carts as flattened images. `pico-lsp` opens them read-only:

- Neovim: `:Pico8OpenPng path/to/cart.p8.png`, then `:Pico8ReloadPng` after
  replacing the image.
- VS Code and Helix: decode to a text cart with the CLI and open that file.

```sh
pico-lsp decode --output cart.p8 cart.p8.png
```

The server never writes PNG carts. To edit a PNG cart, open it in PICO-8, save a
text `.p8` copy, and edit that copy:

```text
> LOAD CART
> SAVE CART_EDIT.P8
```

## Backups

PICO-8 writes a backup when quitting without saving changes or overwriting an
existing file. The backups live under `{appdata}/pico-8/backup`; run
`FOLDER BACKUPS` to open the folder. `BACKUP` saves an extra copy of the current
cart.

Periodic backups may also be enabled in PICO-8 0.2.4c and later; check
`config.txt` if you want to change the interval.

## Common mistakes

- **Editing an exported PNG with a text editor.** Open the text `.p8` instead,
  or decode the PNG first.
- **Saving from PICO-8 before reloading.** The in-memory cart wins and the
  external edit is overwritten. Reload with `CTRL-R` first.
- **Opening only the `.lua` include.** The server can analyze it when the owning
  cart is attached, but the cart is the authoritative program.
- **Editing a cart in the wrong folder.** PICO-8's `FOLDER` command opens the
  actual drive; a copied file elsewhere is not the one PICO-8 loads.
- **Expecting `#include` to be recursive.** It is not. Flatten includes in the
  cart instead.
- **Forgetting that `.p8.png` is flattened.** Changes to external `.lua` files
  do not affect an already exported PNG.

## See also

- [Getting started](getting-started.md)
- [Troubleshooting](troubleshooting.md#included-lua-files-are-not-attached)
- [PICO-8 manual, sections 1.4–1.7](https://www.lexaloffle.com/dl/docs/pico-8_manual.html)
