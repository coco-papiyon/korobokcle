# Guide

## Overview

- Dashboard でジョブ一覧を確認します。
- Job Detail で設計結果、実装結果、テスト結果、ログを確認します。
- Settings 配下で worker、watch rule、test profile、tool command を設定します。

## Basic Flow

1. `Workers` で監視対象の repository を登録します。
2. `Watch Rules` で対象イベント、provider、model、skill set、test profile を設定します。
3. 必要なら `Tool Commands` で動作確認用コマンドを登録します。
4. Issue や Pull Request が検知されると自動で job が作成されます。
5. `Job Detail` で成果物を確認し、承認、再実行、PR 作成に進みます。

## Tool Commands

- resident なコマンドは Flow から起動後、停止ボタンで止められます。
- one-shot なコマンドは終了後にログだけが残ります。
- ログは `Test Report` の下に表示されます。

## Notes

- `copilot` を使う場合、成果物は artifact directory 配下に保存されます。
- `codex` は最終結果を返し、runner が `result.md` に保存します。
- 失敗時も `Implementation Artifact` や `stdout.log` から再実行できる場合があります。

## Installation

### GitHub CLI (`gh`)

1. GitHub CLI のインストールページを開きます。
2. 利用中の OS 向けパッケージをインストールします。
3. インストール後に `gh auth login` を実行して認証します。

確認:

```bash
gh --version
gh auth status
```

### Codex CLI

1. Codex CLI をインストールします。
2. 必要なら API キーや認証設定を行います。
3. `codex` コマンドが PATH に入っていることを確認します。

確認:

```bash
codex --help
```

### GitHub Copilot CLI

1. GitHub Copilot CLI をインストールします。
2. GitHub アカウントでサインインし、CLI 利用可能な状態にします。
3. `copilot` コマンドが PATH に入っていることを確認します。

確認:

```bash
copilot --help
```

### Claude Code

1. Claude Code CLI をインストールします。
2. 必要なら Anthropic アカウントで認証し、利用可能な状態にします。
3. `claude` または利用環境で指定されたコマンドが PATH に入っていることを確認します。

確認:

```bash
claude --help
```

### Application Setup

1. repository を clone します。
2. 必要なら frontend 依存をインストールします。
3. `exec/base/config/` または `config/` 配下の設定を調整します。
4. `Workers`、`Watch Rules`、`Test Profiles`、`Tool Commands` を設定します。

例:

```bash
git clone https://github.com/coco-papiyon/korobokcle.git
cd korobokcle
cd frontend && npm ci
cd ..
go run ./cmd/korobokcle -port 8080
```
