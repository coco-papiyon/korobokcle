# korobokcle

`korobokcle` は、GitHub Issue / Pull Request を監視し、設計・実装・レビューを AI と人間の承認で進めるローカルツールです。

フロントエンドは Vue + TypeScript で構成しています。

## 前提

- Go 1.22 以上
- Node.js 20 系
- npm 10 系
- GitHub CLI `gh`

`gh` を使うので、事前に GitHub にログインしてください。

```bash
gh auth login
```

Issue / PR の取得で `gh` の API を使います。必要に応じて `read:project` などの権限を追加してください。

## ディレクトリ

- `base_dir`
  - ツール実行時のカレントディレクトリ
  - 修正対象のローカルリポジトリ
  - 設計結果や実装結果の保存先
- `tool_dir`
  - `korobokcle` 本体の配置先
  - 設定、DB、作業用ディレクトリ、ログ、静的ファイルを保持

## 起動

### 1. フロントエンド依存を入れる

```bash
cd frontend
npm install
```

### 2. フロントエンドをビルドする

```bash
cd frontend
npm run build
```

### 3. Go サーバを起動する

リポジトリルートで実行します。

```bash
go run ./cmd/korobokcle
```

引数で `base_dir` と `tool_dir` を変えられます。

```bash
go run ./cmd/korobokcle --base-dir C:\path\to\repo --tool-dir C:\path\to\korobokcle
```

## API

- `GET /healthz`
- `GET /api/settings`
- `PUT /api/settings`
- `GET /api/jobs`
- `GET /api/jobs/:id`
- `PATCH /api/jobs/:id/state`

## フロントエンド

開発時は Vite を使います。

```bash
cd frontend
npm run dev
```

Vite は `/api` と `/healthz` を `http://127.0.0.1:8080` に proxy します。

## 保存先

- ジョブ保存: `tool_dir/db/jobs.json`
- 設計成果物: `base_dir/.workspace/...`
- 実装時 worktree: `tool_dir/workspace/...`
- ログ: `tool_dir/logs/...`

## 状態

内部状態は英語で管理し、画面では日本語を表示します。

例:

- `design_running` -> 設計中
- `implementation_running` -> 実装中
- `review_ready` -> レビュー完了

## 開発メモ

- `gh` が取得できない環境では GitHub source は無効になります
- 監視対象リポジトリと issue / PR 条件は Web UI の設定画面から保存します
- `jobs.json` は監視や検知結果の保存先です
- 設計や実装の細かいルールは、リポジトリ内のスキル定義を前提にしています
