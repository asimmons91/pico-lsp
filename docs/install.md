# Install

v1 release binaries are static and require no Go toolchain or runtime. The
supported v1 install channels are GitHub release archives and `go install` for
developers. Homebrew and Mason packaging are deferred to post-v1.

## Supported platforms

| OS | Architectures | Archive |
|---|---|---|
| Linux | amd64, arm64 | `pico-lsp_<version>_linux_<arch>.tar.gz` |
| macOS | amd64, arm64 | `pico-lsp_<version>_darwin_<arch>.tar.gz` |
| Windows | amd64, arm64 | `pico-lsp_<version>_windows_<arch>.zip` |

Every release also publishes `checksums.txt` with SHA-256 hashes.

## VS Code

The VS Code extension is distributed as `pico-lsp-vscode-<version>.vsix` with a
`.vsix.sha256` checksum. Marketplace and Open VSX publishing are follow-ups.

1. Download both files from the matching
   [GitHub release](https://github.com/asimmons91/pico-lsp/releases).
2. Verify the VSIX:

   ```sh
   sha256sum -c pico-lsp-vscode-<version>.vsix.sha256
   ```

3. In VS Code, open **Extensions**, select **...**, and choose
   **Install from VSIX...**.
4. Open a folder containing a `.p8` cart. The extension downloads the matching
   server version on first use, or use `pico8.serverPath` for a manual install.

See [../editors/vscode/README.md](../editors/vscode/README.md) for extension
settings, Lua attachment behavior, and limitations.

## Download a release

Set `VERSION` to the release without the leading `v` (for example `1.8.0`) and
choose the archive for your platform from the
[releases page](https://github.com/asimmons91/pico-lsp/releases).

### Linux and macOS

```sh
VERSION=1.8.0
OS=linux          # or darwin
ARCH=amd64        # or arm64
ASSET="pico-lsp_${VERSION}_${OS}_${ARCH}.tar.gz"

curl -LO "https://github.com/asimmons91/pico-lsp/releases/download/v${VERSION}/${ASSET}"
curl -LO "https://github.com/asimmons91/pico-lsp/releases/download/v${VERSION}/checksums.txt"
sha256sum -c checksums.txt --ignore-missing
tar -xzf "$ASSET"
install -m 0755 pico-lsp "$HOME/.local/bin/pico-lsp"
pico-lsp --version
```

On macOS, `sha256sum` may not be installed. Verify the downloaded archive with:

```sh
grep "$ASSET" checksums.txt | shasum -a 256 -c -
```

Make sure `$HOME/.local/bin` is on `PATH`. If it is not, either add it to your
shell profile or install to another directory such as `/usr/local/bin`.

### Windows

In PowerShell, set the version and architecture, download the zip and
checksums, and compare the SHA-256 hash:

```powershell
$Version = "1.8.0"
$Arch = "amd64" # or arm64
$Asset = "pico-lsp_${Version}_windows_${Arch}.zip"
$Base = "https://github.com/asimmons91/pico-lsp/releases/download/v$Version"

Invoke-WebRequest "$Base/$Asset" -OutFile $Asset
Invoke-WebRequest "$Base/checksums.txt" -OutFile checksums.txt

$Expected = (Select-String -Path checksums.txt -Pattern $Asset).Line.Split()[0]
$Actual = (Get-FileHash $Asset -Algorithm SHA256).Hash.ToLower()
if ($Actual -ne $Expected) { throw "checksum mismatch" }

Expand-Archive $Asset -DestinationPath $env:LOCALAPPDATA\pico-lsp -Force
$env:PATH = "$env:LOCALAPPDATA\pico-lsp;$env:PATH"
pico-lsp --version
```

Add `%LOCALAPPDATA%\pico-lsp` to your user `PATH` permanently to use
`pico-lsp` from any shell.

## Install with Go

For developers with Go 1.27.4 or newer:

```sh
go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.8.0
```

`go install` reports the module version from Go build information when no
release ldflags are present. `pico-lsp --version` should match the installed
module version.

## Verify checksums

`checksums.txt` uses the standard `sha256sum` format:

```
<sha256>  <filename>
```

Only check archives you downloaded from the project's GitHub release. Release
workflows generate the checksums from the same build artifacts that are
uploaded.

## Decode a `.p8.png` cart

The server can open PNG carts read-only. For editors without virtual-document
support, decode one to a text cart:

```sh
pico-lsp decode --output /tmp/cart.p8 path/to/cart.p8.png
```

The decoder never writes PNG files.

## Upgrade

Download the new archive for the same platform, replace the existing binary,
and run `pico-lsp --version` to confirm the new version. There is no auto-update
mechanism in v1.

## Uninstall

Remove the `pico-lsp` binary from the directory where you installed it. Server
logs created with `--log-file` and editor configuration are separate; remove
them if you no longer need them.

## macOS Gatekeeper and Windows SmartScreen

v1 release binaries are not code-signed or notarized.

- On macOS, Gatekeeper may block the first launch of an unsigned binary. If you
  trust the download and verified its checksum, allow it in System Settings >
  Privacy & Security, or remove the quarantine attribute with
  `xattr -d com.apple.quarantine /path/to/pico-lsp`.
- On Windows, SmartScreen may warn about an unrecognized app. Verify the
  checksum, then choose the option to run it if you trust the release.

## Homebrew and Mason

Homebrew and Mason packages are not available for v1. They are tracked as
post-v1 distribution work in the [roadmap](ROADMAP.md). Use a release archive or
`go install` until then.
