# README.md を日本語化

## Context

現在 `README.md`（英語）と `README_ja.md`（日本語）の2ファイル構成。
ユーザーはメインの `README.md` を日本語にしたい。英語版は `README_en.md` として残す。

## 変更内容

### 1. `README.md` → `README_en.md` にリネーム（git mv）
- 冒頭リンクを `[日本語版はこちら](README_ja.md)` → `[日本語版はこちら！！！！！](README.md)` に更新

### 2. `README_ja.md` → `README.md` にリネーム（git mv）
- 冒頭リンクを `[English version](README.md)` → `[English version](README_en.md)` に更新

### 対象ファイル
- `README.md`（英語 → `README_en.md` へ移動）
- `README_ja.md`（日本語 → `README.md` へ移動）

## 検証

- `README.md` が日本語になっていること
- `README_en.md` が英語版であること
- 相互リンクが正しいこと
- git diff で想定通りの変更であること
