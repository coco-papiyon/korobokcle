# ガイド

## 概要

- ダッシュボードでジョブ一覧を確認します。
- ジョブ詳細で設計結果、実装結果、テスト結果、ログを確認します。
- 設定画面でワーカー、監視ルール、テストプロファイル、ツールコマンドを設定します。

## 基本フロー

1. `ワーカー` で監視対象のリポジトリを登録します。
2. `監視ルール` で対象イベント、provider、model、skill set、test profile を設定します。
3. 必要なら `ツールコマンド` で動作確認用コマンドを登録します。
4. Issue や PR が検知されると自動で job が作成されます。
5. `ジョブ詳細` で成果物を確認し、承認、再実行、PR 作成に進みます。

## ツールコマンド

- 常駐コマンドは Flow から起動後、停止ボタンで止められます。
- 単発コマンドは終了後にログだけが残ります。
- ログは `テスト結果` の下に表示されます。

## 注意事項

- `copilot` を使う場合、成果物は artifact directory 配下に保存されます。
- `claude` は標準入力で prompt を受け取り、必要に応じて `result.md` を返します。
- `codex` は最終結果を返し、runner が `result.md` に保存します。
- 失敗時も `実装結果` や `stdout.log` から再実行できる場合があります。

## インストール

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

### アプリケーション設定

1. リポジトリを clone します。
2. 必要なら frontend 依存をインストールします。
3. `exec/base/config/` または `config/` 配下の設定を調整します。
4. `ワーカー`、`監視ルール`、`テストプロファイル`、`ツールコマンド` を設定します。

例:

```bash
git clone https://github.com/coco-papiyon/korobokcle.git
cd korobokcle
cd frontend && npm ci
cd ..
go run ./cmd/korobokcle -port 8080
```
