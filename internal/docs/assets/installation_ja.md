# インストール

[English version](installation.md)

`board` CLI のインストール方法を説明します。

## 動作環境

- macOS（Intel または Apple Silicon）または Linux（amd64 / arm64）
- BOARD アカウント（API キーとアクセストークンが必要）

---

## 方法 1: Homebrew（macOS / Linux）— 推奨

Homebrew を使うと、インストールとアップデートを簡単に管理できます。

### インストール

```sh
brew install youyo/tap/board
```

### アップグレード

```sh
brew upgrade board
```

### アンインストール

```sh
brew uninstall board
```

### 動作確認

```sh
board --version
```

---

## 方法 2: GitHub Releases からバイナリをダウンロード

[Releases ページ](https://github.com/youyo/board/releases) からビルド済みバイナリを直接ダウンロードします。

### 手順

1. [Releases ページ](https://github.com/youyo/board/releases) から最新リリースを確認します。
2. お使いのプラットフォーム向けのアーカイブをダウンロードします:

   | OS    | アーキテクチャ | ファイル                               |
   |-------|---------------|----------------------------------------|
   | macOS | Apple Silicon (M1/M2/M3) | `board_Darwin_arm64.tar.gz` |
   | macOS | Intel         | `board_Darwin_amd64.tar.gz`            |
   | Linux | amd64         | `board_Linux_amd64.tar.gz`             |
   | Linux | arm64         | `board_Linux_arm64.tar.gz`             |

3. 展開してインストールします:

   ```sh
   # macOS Apple Silicon の例
   tar -xzf board_Darwin_arm64.tar.gz
   sudo mv board /usr/local/bin/
   ```

4. 動作確認:

   ```sh
   board --version
   ```

### アンインストール

```sh
sudo rm /usr/local/bin/board
```

---

## 方法 3: go install

Go 1.26 以上がインストール済みの場合、ソースからインストールできます。

### 前提条件

- Go 1.26 以上（[https://go.dev/dl/](https://go.dev/dl/)）

### インストール

```sh
go install github.com/youyo/board/cmd/board@latest
```

バイナリは `$GOPATH/bin`（通常 `~/go/bin`）に配置されます。このディレクトリが `$PATH` に含まれているか確認してください:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

### 動作確認

```sh
board --version
```

### アンインストール

```sh
rm "$(go env GOPATH)/bin/board"
```

---

## 次のステップ

インストール後、認証情報を設定します:

```sh
board configure
```

詳細な手順は[クイックスタートガイド](guides/getting-started.md)を参照してください。
