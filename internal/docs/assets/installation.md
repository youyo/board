# Installation

[日本語版はこちら](installation_ja.md)

This guide describes how to install the `board` CLI on your system.

## Requirements

- macOS (Intel or Apple Silicon) or Linux (amd64 or arm64)
- An active BOARD account with API access (API key and access token)

---

## Option 1: Homebrew (macOS / Linux) — Recommended

Homebrew is the easiest way to install and keep `board` up to date.

### Install

```sh
brew install youyo/tap/board
```

### Upgrade

```sh
brew upgrade board
```

### Uninstall

```sh
brew uninstall board
```

### Verify

```sh
board --version
```

---

## Option 2: Download Binary from GitHub Releases

Download a pre-built binary directly from the [Releases page](https://github.com/youyo/board/releases).

### Steps

1. Visit the [Releases page](https://github.com/youyo/board/releases) and find the latest release.
2. Download the archive for your platform:

   | OS    | Architecture | File                               |
   |-------|--------------|------------------------------------|
   | macOS | Apple Silicon (M1/M2/M3) | `board_Darwin_arm64.tar.gz` |
   | macOS | Intel        | `board_Darwin_amd64.tar.gz`        |
   | Linux | amd64        | `board_Linux_amd64.tar.gz`         |
   | Linux | arm64        | `board_Linux_arm64.tar.gz`         |

3. Extract and install:

   ```sh
   # Example for macOS Apple Silicon
   tar -xzf board_Darwin_arm64.tar.gz
   sudo mv board /usr/local/bin/
   ```

4. Verify the installation:

   ```sh
   board --version
   ```

### Uninstall

```sh
sudo rm /usr/local/bin/board
```

---

## Option 3: go install

If you have Go 1.26 or later installed, you can install `board` directly from source.

### Prerequisites

- Go 1.26 or later ([https://go.dev/dl/](https://go.dev/dl/))

### Install

```sh
go install github.com/youyo/board/cmd/board@latest
```

This places the `board` binary in `$GOPATH/bin` (typically `~/go/bin`). Make sure this directory is in your `$PATH`:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Verify

```sh
board --version
```

### Uninstall

```sh
rm "$(go env GOPATH)/bin/board"
```

---

## Next Steps

Once installed, set up your credentials:

```sh
board configure
```

See the [Getting Started guide](guides/getting-started.md) for a step-by-step walkthrough.
