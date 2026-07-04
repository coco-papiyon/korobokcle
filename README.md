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
  - 実行バイナリ、静的ファイル、プロンプトを保持
- `work_dir`
  - `korobokcle` が使用するデータの配置先
  - 設定、DB、作業用ディレクトリ、ログを保持
  - 省略時は `tool_dir` と同じ

## 起動

開発時はリポジトリルートで次を実行します。

```bat
start.bat
```

バックエンド（既定 `http://localhost:8080`）と Vite 開発サーバー（`http://localhost:5173`）が別ウィンドウで起動します。画面は `http://localhost:5173` を開いてください。フロントエンドのソース変更は Vite HMR により自動反映されます。

以下は個別に起動する場合の手順です。

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
go run ./cmd/korobokcle --tool-dir . --work-dir .
```

引数で `base_dir`、`tool_dir`、`work_dir`、`addr` を変えられます。`addr` の既定は `:8080` です。

```bash
go run ./cmd/korobokcle --base-dir C:\path\to\repo --tool-dir C:\path\to\korobokcle --work-dir C:\path\to\korobokcle-data --addr :8082
```

## API

- `GET /healthz`
- `GET /api/settings`
- `PUT /api/settings`
- `GET /api/jobs`
- `GET /api/jobs/:id`
- `PATCH /api/jobs/:id/state`

`GET /api/jobs` は各ジョブの `kind` と `state` を返します。ジョブ一覧ではフロントエンド側で `Kind` と状態グループのフィルターをかけます。

## フロントエンド

開発時は Vite を使います。

```bash
cd frontend
npm run dev
```

Vite は `/api` と `/healthz` を `http://127.0.0.1:8080` に proxy します。`KOROBOKCLE_BACKEND_PORT` を設定すると、起動ポートに追従します。

ジョブ一覧では `Kind`、状態グループ、並び順を選んで絞り込み・並べ替えできます。既定では `完了` は非表示で、並び順は取得日時の新しい順です。条件不一致時は「条件に一致するジョブがありません。」と表示します。

## 保存先

- 静的ファイル: `tool_dir/static/...`
- プロンプト: `tool_dir/prompt/...`
- ジョブ保存: `work_dir/db/jobs.json`
- 設計成果物: `base_dir/.workspace/...`
- 実装時 worktree: `work_dir/workspace/...`
- ジョブログ: `work_dir/workspace/<repo-id>/<job-id>/logs/...`
- メインログ: `tool_dir/logs/...`

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

## 画面テスト用モック

実 GitHub と実 AI を使わずに画面確認する場合は、テストデータを作成してモックモードで起動します。

```powershell
.\create_test_data.ps1
go run ./cmd/korobokcle --tool-dir . --base-dir tests --work-dir tests --mock-mode
```

または、フロントエンドのビルド、静的ファイル更新、テストデータ作成、モック起動をまとめて実行します。

```cmd
start_test.bat
```

モックモードでは次を行いません。

- 実 AI の呼び出し
- Issue / PR へのコメント投稿
- ラベル更新
- PR 作成
- 実装用 git worktree 作成
