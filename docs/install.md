# Install

`pico-lsp` release binaries are static and require no Go toolchain or runtime.
Homebrew and Mason packaging are deferred to post-v1; see the
[roadmap](ROADMAP.md).

## Install channels

| Component | Channel | Use when |
|---|---|---|
| Server | GitHub release archive | Manual, pinned, or air-gapped installs |
| Server | `go install` | Development and Go users |
| Server | VS Code extension download | Normal VS Code use; the extension fetches the matching release |
| Extension | VS Code Marketplace | Planned for a later patch after publisher and token setup |
| Extension | Open VSX | Planned for a later patch after publisher and token setup |
| Extension | GitHub release VSIX | Supported for v1.8.1; pinned, offline, or registry-lag installs |

The extension and server versions match. Extension `1.8.1` downloads
`pico-lsp` v1.8.1.

## Supported platforms

| OS | Architectures | Archive |
|---|---|---|
| Linux | amd64, arm64 | `pico-lsp_<version>_linux_<arch>.tar.gz` |
| macOS | amd64, arm64 | `pico-lsp_<version>_darwin_<arch>.tar.gz` |
| Windows | amd64, arm64 | `pico-lsp_<version>_windows_<arch>.zip` |

Every release also publishes `checksums.txt` with SHA-256 hashes.

## VS Code

v1.8.1 is a GitHub-only release: the extension ships as a VSIX asset. Install it
with the steps below. The extension supports local desktop VS Code and Open
VSX-compatible editors only; remote extension hosts, web extension hosts, and
Codespaces are not supported. Offline use also requires a local server binary
and `pico8.serverPath`.

Marketplace and Open VSX listing is planned for a later patch. When those
listings are live, install the same extension ID (`asimmons91.pico-lsp`) from
the editor's Extensions view. The GitHub release VSIX remains available for
pinned, registry-lag, and offline use.

### Manual VSIX install

1. Download `pico-lsp-vscode-<version>.vsix` and
   `pico-lsp-vscode-<version>.vsix.sha256` from the matching
   [GitHub release](https://github.com/asimmons91/pico-lsp/releases).
2. Verify the checksum:

   ```sh
   sha256sum -c pico-lsp-vscode-<version>.vsix.sha256
   ```

   On macOS without `sha256sum`:

   ```sh
   shasum -a 256 -c pico-lsp-vscode-<version>.vsix.sha256
   ```

   On Windows PowerShell:

   ```powershell
   $Expected = (Get-Content pico-lsp-vscode-<version>.vsix.sha256).Split()[0]
   $Actual = (Get-FileHash pico-lsp-vscode-<version>.vsix -Algorithm SHA256).Hash.ToLower()
   if ($Actual -ne $Expected) { throw "checksum mismatch" }
   ```

3. In VS Code, open **Extensions**, select **...**, and choose
   **Install from VSIX...**.

### First-run server download

In a trusted workspace, the extension downloads the matching release archive for
the current platform on first use, verifies its SHA-256 from `checksums.txt`,
caches it in the extension's global storage, and starts it. No download occurs
in an untrusted workspace; the extension disables itself there.

If the download fails, check the **PICO-8 Language Server** output channel,
configure VS Code's `http.proxy` and `http.proxyStrictSSL` settings when behind a
proxy, or set `pico8.serverPath`.

### Cache location and cleanup

The extension stores downloaded servers under its global storage directory:

| OS | Directory |
|---|---|
| Linux | `~/.config/Code/User/globalStorage/asimmons91.pico-lsp` |
| macOS | `~/Library/Application Support/Code/User/globalStorage/asimmons91.pico-lsp` |
| Windows | `%APPDATA%\Code\User\globalStorage\asimmons91.pico-lsp` |

VSCodium and other VS Code-compatible editors use their own product directory
instead of `Code`. Run **PICO-8: Show Server Info** or **PICO-8: Copy Server
Diagnostics** to see the exact cache root for the running extension.

Cached servers live under `servers/v<version>/<platform>-<arch>/`. **PICO-8:
Clear Downloaded Server Cache** removes the cached server for the installed
extension version. Older version directories remain; delete the entire
`asimmons91.pico-lsp` directory to remove every cached version and download
temporary files.

### Offline or air-gapped use

Set an absolute path to an existing server and disable downloads:

```json
{
  "pico8.serverPath": "/usr/local/bin/pico-lsp",
  "pico8.autoDownload": false
}
```

The server binary must match the extension version, or the extension will show
its advisory version-mismatch warning. Use **PICO-8: Copy Server Diagnostics** to
confirm the versions.

### Upgrade

- **Registry listings (planned for a later patch):** when the listings are live,
  VS Code updates the extension automatically. The updated extension downloads
  its matching server on the next start. Previously cached server versions are
  not removed automatically.
- **VSIX:** install the newer VSIX over the existing extension. The same
  matching-server download applies.
- **Server only:** replace the binary and run `pico-lsp --version`. If you use
  `pico8.serverPath`, the extension keeps using that binary until you change it.

Do not mix an extension with a server release that does not match it unless that
is intentional; the extension warns once but keeps working.

### Uninstall

1. Optionally run **PICO-8: Clear Downloaded Server Cache** before uninstalling.
2. Uninstall the extension from the Extensions view.
3. Delete the global storage directory from the table above if it remains.
4. Remove any binary you configured with `pico8.serverPath` separately.

### Registry listings and VSIX fallback

v1.8.1 is GitHub-release-only. When the later Marketplace and Open VSX listings
are live, registry processing can lag behind the GitHub release. Until the
matching version is listed, use the VSIX from the GitHub release. A registry
listing may also lag during an outage; the VSIX does not depend on either
registry.

## Download a release

Set `VERSION` to the release without the leading `v` (for example `1.8.1`) and
choose the archive for your platform from the
[releases page](https://github.com/asimmons91/pico-lsp/releases).

### Linux and macOS

```sh
VERSION=1.8.1
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
$Version = "1.8.1"
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
go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.8.1
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

## Upgrade the server binary

Download the new archive for the same platform, replace the existing binary,
and run `pico-lsp --version` to confirm the new version. There is no auto-update
mechanism in v1. VS Code users normally receive the server through the extension
instead; see [Upgrade](#upgrade).

## Uninstall the server binary

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
post-v1 distribution work in the [roadmap](ROADMAP.md). Use a release archive,
`go install`, or the VS Code extension until then.
