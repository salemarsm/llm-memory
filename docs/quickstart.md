# Quickstart

## Install

**Linux / macOS** (one command):

```bash
curl -fsSL https://raw.githubusercontent.com/salemarsm/ginko/main/install.sh | bash
```

**Docker:**

```bash
docker run -d -p 8787:8787 -v "$HOME/.ginko:/data" ghcr.io/salemarsm/ginko:latest
```

For Windows, manual downloads, Homebrew, and checksum verification see [install.md](install.md).

Verify:

```bash
ginko version
ginko doctor
```

## Build from source

```bash
git clone https://github.com/salemarsm/ginko.git
cd ginko

go build -o bin/ginko ./cmd/ginko
go build -o bin/ginko-admin ./cmd/ginko-admin
go build -o bin/memserver ./cmd/memserver
go build -o bin/memmcp ./cmd/memmcp
go build -o bin/memctl ./cmd/memctl

bin/ginko init
bin/ginko doctor
bin/ginko ui
```

Open:

```txt
http://127.0.0.1:8787
```

Store a memory:

```bash
echo "The user prefers direct technical answers." \
  | bin/memctl -subject botmaster -scope global -type preference remember
```

Retrieve compact context:

```bash
bin/memctl -subject botmaster -scope global -max-tokens 400 context "How should I answer?"
```

Suggest learnings:

```bash
bin/memctl -subject botmaster suggest "I prefer Go examples and concise answers."
```
