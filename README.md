# korobokcle

`korobokcle` は、GitHub Issue / Pull Request を監視し、設計・実装・レビューの自動化フローを扱うための Go 製ローカルツールです。

現状の実装では、以下を提供しています。

- Go バックエンド
  - YAML 設定読込
  - SQLite ベースのジョブ / イベント保存
  - JSON API
  - Vue フロントエンドの静的配信
- TypeScript / Vue フロントエンド
  - ダッシュボード
  - ジョブ詳細
  - Watch Rule 一覧

## Requirements

- Go `1.22.5` 以上
- Node.js `20` 系推奨
- npm `10` 系推奨
- GitHub CLI `gh`

## Directory Layout

```text
cmd/                 Go エントリポイント
config/              YAML 設定
docs/                設計資料
frontend/            Vue + TypeScript フロントエンド
internal/            Go アプリケーション実装
```

## Build

フロントエンドは Go サーバが `frontend/dist` を配信するため、先にビルドが必要です。

### 1. フロントエンド依存をインストール

```powershell
cd frontend
npm install
```

### 2. フロントエンドをビルド

```powershell
cd frontend
npm run build
```

ビルド成果物は `frontend/dist` に出力されます。

### 3. Go バックエンドをビルド

リポジトリルートで実行します。

```powershell
go build ./cmd/korobokcle
```

Windows で実行ファイルを明示して出したい場合:

```powershell
go build -o korobokcle.exe ./cmd/korobokcle
```

## Run

### GitHub 認証の前提

GitHub 監視機能は `gh auth token` を使って GitHub API トークンを取得します。

重要な点:

- `gh auth token` を毎回手で実行する必要はありません
- `go run ./cmd/korobokcle` または `.\korobokcle.exe` を起動した後、watcher がポーリングを開始するタイミングでアプリ内部から `gh auth token` を実行します
- そのため、事前に必要なのは `gh` にログイン済みであることです
- 監視ルールはデフォルトで `enabled: false` です。監視対象画面で有効化するまでポーリングは実行されません

最初に一度だけ、必要に応じて以下を実行してください。

```powershell
gh auth login
```

ログイン状態の確認:

```powershell
gh auth status
```

`gh auth status` が正常なら、その後は通常どおり `go run` すれば構いません。

### 開発用にそのまま起動

リポジトリルートで実行します。

```powershell
go run ./cmd/korobokcle
```

デバッグログを有効にする場合:

```powershell
go run ./cmd/korobokcle --debug
```

推奨手順:

1. `gh auth login` を必要に応じて一度だけ実行
2. `gh auth status` でログイン状態を確認
3. `frontend` をビルド
4. リポジトリルートで `go run ./cmd/korobokcle` を実行

起動後、以下にアクセスします。

- Web UI: `http://localhost:8080`
- Health Check: `http://localhost:8080/healthz`

### ビルド済みバイナリで起動

```powershell
.\korobokcle.exe
```

## Configuration

初期設定ファイルは `config/` にあります。

- `config/app.yaml`
- `config/watch-rules.yaml`
- `config/notifications.yaml`
- `config/test-profiles.yaml`

デフォルトでは `config/app.yaml` の `httpPort` は `8080` です。

SQLite ファイルはデフォルトで `data/korobokcle.db` に作成されます。

`config/app.yaml` の `provider` は `mock` / `copilot` / `codex` を指定できます。
Web UI の `Settings` 画面からも切り替え可能です。

`copilot` と `codex` は外部 CLI を実行します。
デフォルトでは `copilot suggest -t general ...` と `codex exec ...` を呼びますが、
環境に応じて `KOROBOKCLE_COPILOT_BIN` / `KOROBOKCLE_COPILOT_ARGS_JSON`、
`KOROBOKCLE_CODEX_BIN` / `KOROBOKCLE_CODEX_ARGS_JSON` で上書きできます。
`*_ARGS_JSON` は JSON 配列で、`{{prompt}}`, `{{work_dir}}`, `{{artifact_dir}}`, `{{output_path}}`, `{{skill_name}}` を使えます。

`config/watch-rules.yaml` の `repositories` は `owner/repo` 形式を推奨します。
`https://github.com/owner/repo` の形式が入っていても、現在は自動で `owner/repo` に正規化されます。

## Development Notes

- Go サーバは `frontend/dist/index.html` が存在しない場合、SPA を返せず `503` を返します。
- フロントエンドを変更したら再度 `npm run build` が必要です。
- GitHub watcher は起動後の初回ポーリング時に `gh auth token` を内部実行します。
- `gh auth token` の結果は一定時間メモリキャッシュされ、毎回の API 呼び出しで都度手入力は不要です。
- `--debug` を付けて起動すると、ポーリング開始、取得件数、マッチ件数、イベント処理結果をデバッグログで出力します。
- CSS は `frontend/src/styles/` に集約しています。
  - `tokens.css`: デザイントークン
  - `base.css`: ベーススタイル
  - `utilities.css`: 共通レイアウト / ユーティリティ
  - `components.css`: 共通コンポーネントスタイル

## Verify

バックエンドのテスト:

```powershell
go test ./...
```

フロントエンドの型チェックとビルド:

```powershell
cd frontend
npm run build
```
