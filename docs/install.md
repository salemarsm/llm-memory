# Installation

Ginko ships pre-built binaries for Linux, macOS, and Windows (amd64 + arm64).  
The archive contains: `ginko`, `ginko-admin`, `memctl`, `memmcp`, `memserver`.

## Linux

**One-line installer** (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/salemarsm/ginko/main/install.sh | bash
```

Override install directory:

```bash
GINKO_INSTALL_DIR=/usr/local/bin curl -fsSL \
  https://raw.githubusercontent.com/salemarsm/ginko/main/install.sh | bash
```

Install a specific version:

```bash
GINKO_VERSION=v0.6 curl -fsSL \
  https://raw.githubusercontent.com/salemarsm/ginko/main/install.sh | bash
```

**Manual download:**

```bash
VERSION=v0.6
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -fsSL "https://github.com/salemarsm/ginko/releases/download/$VERSION/ginko_${VERSION#v}_linux_${ARCH}.tar.gz" \
  | tar -xz -C ~/.local/bin
```

**Verify checksums:**

```bash
curl -fsSL "https://github.com/salemarsm/ginko/releases/download/$VERSION/checksums.txt" \
  | sha256sum -c --ignore-missing
```

---

## macOS

**One-line installer** (same as Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/salemarsm/ginko/main/install.sh | bash
```

**Manual download (Apple Silicon):**

```bash
VERSION=v0.6
curl -fsSL "https://github.com/salemarsm/ginko/releases/download/$VERSION/ginko_${VERSION#v}_darwin_arm64.tar.gz" \
  | tar -xz -C ~/.local/bin
```

**Manual download (Intel):**

```bash
VERSION=v0.6
curl -fsSL "https://github.com/salemarsm/ginko/releases/download/$VERSION/ginko_${VERSION#v}_darwin_amd64.tar.gz" \
  | tar -xz -C ~/.local/bin
```

macOS may quarantine unsigned binaries. Remove the quarantine attribute if needed:

```bash
xattr -dr com.apple.quarantine ~/.local/bin/ginko
```

---

## Windows

Download the `.zip` from the [releases page](https://github.com/salemarsm/ginko/releases), extract it, and add the directory to `%PATH%`.

PowerShell:

```powershell
$version = "v0.6"
$arch    = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$url     = "https://github.com/salemarsm/ginko/releases/download/$version/ginko_$($version.TrimStart('v'))_windows_$arch.zip"
Invoke-WebRequest $url -OutFile "$env:TEMP\ginko.zip"
Expand-Archive "$env:TEMP\ginko.zip" -DestinationPath "$env:LOCALAPPDATA\ginko"
[Environment]::SetEnvironmentVariable("PATH", $env:PATH + ";$env:LOCALAPPDATA\ginko", "User")
```

---

## Docker

Run ginko as a local network memory server:

```bash
docker run -d \
  --name ginko \
  -p 8787:8787 \
  -v "$HOME/.ginko:/data" \
  ghcr.io/salemarsm/ginko:latest
```

With auth token:

```bash
docker run -d \
  --name ginko \
  -p 8787:8787 \
  -v "$HOME/.ginko:/data" \
  -e GINKO_AUTH_TOKEN=your-secret-token \
  ghcr.io/salemarsm/ginko:latest
```

---

## Build from source

Requirements: Go 1.22+, no CGO, no external services.

```bash
git clone https://github.com/salemarsm/ginko.git
cd ginko
make install   # installs all binaries to $GOPATH/bin
```

Or build to `./bin/`:

```bash
make build
```

---

## Verify

```bash
ginko version
ginko doctor
```

`ginko doctor` checks the home directory, config, database, auth policy, sibling binaries, port availability, and Claude Code setup.

---

## Next step

```bash
ginko setup claude-code
```

See [quickstart.md](quickstart.md) for first steps after installation.
