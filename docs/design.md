# korobokcle 設計

## 1. 目的

GitHub の Issue / Pull Request を監視し、以下のフローを自動化する Go 製ツールを提供する。
フロントエンドはTypeScript/Vueを使用する。  

- Issue 検知時
  - 条件に一致する Issue を検知
  - AI に設計を実施させる
  - 設計成果物を特定フォルダに出力
  - ユーザへ確認通知
  - ユーザ承認後に AI で実装、テスト、修正
  - ユーザ最終確認後に PR 作成
- PR 検知時
  - 関連 Issue / 設計を参照してレビュー
  - PR 条件（UT / E2E など）をチェック
  - レビュー結果を出力し、ユーザへ通知

制約:

- Issue 待ち、承認待ちなどの待機は AI に行わせない
- 監視、待機、状態管理はプログラム側で実施する
- AI の詳細動作はスキル定義で差し替え可能にする
- 当面の AI 対象は GitHub Copilot とする

## 2. 全体アーキテクチャ

1 プロセス内で `main` と Web サーバを同時起動する。

- `main`
  - 設定読込
  - 永続化ストア初期化
  - GitHub 監視ワーカー起動
  - ジョブ実行ワーカー起動
  - 通知基盤起動
  - Web サーバ起動
- Web サーバ
  - 監視条件の設定 UI
  - ジョブ一覧 / 詳細
  - 設計レビュー / 承認 UI
  - 実装レビュー / 承認 UI
  - PR レビュー結果表示
  - 手動再実行 / 保留 / 却下

構成イメージ:

```text
+-------------------------------+
| main process                  |
|                               |
|  +-------------------------+  |
|  | Web Server              |  |
|  | - UI                    |  |
|  | - Approval API          |  |
|  +-------------------------+  |
|                               |
|  +-------------------------+  |
|  | Orchestrator            |  |
|  | - state machine         |  |
|  | - job scheduler         |  |
|  +-------------------------+  |
|                               |
|  +-------------------------+  |
|  | GitHub Watcher          |  |
|  | - issue poller/webhook  |  |
|  | - pr poller/webhook     |  |
|  +-------------------------+  |
|                               |
|  +-------------------------+  |
|  | Skill Runner            |  |
|  | - design skill          |  |
|  | - coding skill          |  |
|  | - review skill          |  |
|  +-------------------------+  |
|                               |
|  +-------------------------+  |
|  | Notification Adapter    |  |
|  | - Windows toast         |  |
|  +-------------------------+  |
|                               |
|  +-------------------------+  |
|  | Storage                 |  |
|  | - sqlite (runtime)      |  |
|  | - yaml (settings)       |  |
|  | - workspace files       |  |
|  +-------------------------+  |
+-------------------------------+
```

## 3. 基本方針

### 3.1 AI とプログラムの責務分離

プログラムが担当するもの:

- GitHub の監視
- 監視条件評価
- ワークフロー状態管理
- 承認待ち制御
- 通知
- 成果物保存
- テスト実行
- PR 作成
- タイムアウト / リトライ / 排他制御

AI が担当するもの:

- 設計文書作成
- 実装案作成
- コード修正
- レビューコメント生成
- 変更理由の整理

### 3.2 イベント駆動

コスト削減のため、承認待ちや監視待ちは以下で処理する。

- GitHub 監視: ポーリングまたは Webhook
- 対象判定: 取得した Issue / PR に対する監視条件評価
- 承認待ち: DB 上の状態遷移 + Web API / ファイル更新監視
- ジョブ再開: イベント投入で再開

AI には「待つ」「監視する」責務を持たせない。
また、監視後に「この Issue / PR を処理対象にするか」を判定する責務も AI には持たせない。

処理対象判定は必ずプログラム側で実施する。

- ラベル一致
- タイトル条件
- repository 条件
- assignee / author 条件
- draft 除外
- 既処理判定
- 重複ジョブ判定

AI を起動するのは、監視条件に一致し、ジョブ作成済みの対象に限定する。

### 3.3 スキル拡張

設計、実装、レビューは固定ロジックではなくスキルとして定義する。

- プロンプトテンプレート
- 入出力ファイル規約
- 前処理 / 後処理フック
- 実行コマンド
- 使用コンテキスト

これにより、業務ルールやレビュー観点をスキル変更だけで差し替え可能とする。

### 3.4 設定と状態の分離

設定値は `yaml` で保持し、ランタイム状態は SQLite で保持する。

`yaml` で保持するもの:

- アプリ全体設定
- GitHub 監視対象設定
- 監視条件
- 通知設定
- テストプロファイル
- 使用スキル設定

SQLite で保持するもの:

- ジョブ
- ジョブイベント
- スキル実行履歴
- 通知送信履歴
- 一時的な実行状態

この分離により、設定はユーザがファイルとして管理・編集しやすくし、状態は安全に更新可能なストアで管理する。

## 4. 想定ユースケース

### 4.1 Issue フロー

1. GitHub Watcher が Issue を検知
2. 監視条件に一致するか判定
3. `IssueJob` を作成
4. 設計スキルを起動
5. 設計成果物を `artifacts/designs/<job-id>/` に保存
6. ユーザへ通知
7. ユーザが Web またはファイルで確認
8. 承認時に実装スキルを起動
9. 実装後にテスト実行
10. 失敗時は修正スキルを起動
11. 成果物を `artifacts/changes/<job-id>/` に保存
12. ユーザへ最終確認通知
13. 承認時にブランチ push と PR 作成

### 4.2 PR フロー

1. GitHub Watcher が PR を検知
2. 監視条件に一致するか判定
3. `PRReviewJob` を作成
4. 関連 Issue と設計成果物を収集
5. 条件チェックを実行
   - UT
   - E2E
   - Lint
   - 必須ラベル
   - 必須テンプレート
6. レビュースキルを起動
7. レビュー結果を `artifacts/reviews/<job-id>/` に保存
8. 必要に応じて通知、コメント投稿、ステータス更新

## 5. 状態管理

ジョブごとにステートマシンで管理する。

### 5.1 IssueJob 状態

```text
detected
  -> design_running
  -> design_ready
  -> waiting_design_approval
  -> design_rejected | implementation_running
  -> test_running
  -> fix_running (loop)
  -> implementation_ready
  -> waiting_final_approval
  -> final_rejected | pr_creating
  -> completed | failed
```

### 5.2 PRReviewJob 状態

```text
detected
  -> collecting_context
  -> checks_running
  -> review_running
  -> review_ready
  -> completed | failed
```

### 5.3 承認イベント

承認操作はすべてイベント化する。

- `design_approved`
- `design_rejected`
- `final_approved`
- `final_rejected`
- `rerun_requested`
- `job_canceled`

### 5.4 イベント保存

ジョブ作成以降の主要イベントはすべて永続化する。

保存対象:

- ジョブ作成
- 状態遷移
- 監視条件評価結果
- スキル実行開始 / 完了 / 失敗
- テスト開始 / 完了 / 失敗
- 通知送信
- ユーザ承認 / 却下 / 再実行
- PR 作成

イベントは時系列で保存し、Web UI から参照可能にする。
これにより、各ジョブについて「何が起きたか」「どこで止まったか」「AI をいつ実行したか」を追跡できるようにする。

## 6. 監視方式

### 6.1 初期実装

初期実装はポーリングを採用する。

理由:

- 実装が単純
- ローカル常駐ツールとして扱いやすい
- Webhook 受信環境を必須にしない

ポーリングは AI ではなく通常コードで行う。
GitHub API へのアクセス認証は `gh` コマンドを利用して取得した認証情報を使う。

対象:

- Issue 一覧
- PR 一覧
- 対象更新時刻
- ラベル
- assignee
- author
- repository

### 6.1.1 監視対象の検討メモ

初期実装の主対象は Issue と PR とするが、将来的な監視対象候補として以下を整理しておく。

- `issue_comment`
  - GitHub 上のコメントで承認、再実行、補足指示を受けたい場合に有効
- `pull_request_review`
  - 承認、差し戻し、レビュー結果の収集に有効
- `pull_request_review_comment`
  - 行単位の指摘や修正依頼の収集に有効
- `label`
  - `ai:design` や `do-not-auto` のような制御に有効
- `check_run` / `workflow_run`
  - UT、E2E、Lint などの結果連携に有効
- `push`
  - ブランチ更新や PR 作成前後の補助トリガとして利用可能
- `milestone`
  - 特定マイルストーンのみ対象にしたい場合に有効
- `project_item`
  - GitHub Projects の状態変化を起点にしたい場合に有効
- `discussion`
  - Issue 化前の議論を拾いたい場合に有効
- `release`
  - リリース連動の運用に広げる場合に有効

優先度の考え方:

1. `issue`
2. `pull_request`
3. `issue_comment`
4. `pull_request_review`
5. `pull_request_review_comment`
6. `check_run` / `workflow_run`

初期実装では `issue` と `pull_request` を必須対象とし、次点で `issue_comment`、`pull_request_review`、`check_run` 系の追加を検討する。

### 6.2 将来拡張

Webhook アダプタを追加可能にする。

インターフェース:

```go
type EventSource interface {
    Start(ctx context.Context, out chan<- DomainEvent) error
}
```

実装候補:

- `PollingSource`
- `GitHubWebhookSource`

## 7. 監視条件

監視条件は Web 画面から設定する。

例:

- 対象リポジトリ
- Issue / PR のどちらを監視するか
- ラベル一致
- タイトル正規表現
- 作成者
- assignee
- draft PR を除外
- branch 条件
- 実行スキルセット
- テストプロファイル

条件は `yaml` に保存し、Watcher が毎回評価する。
評価結果は以下のいずれかに分類する。

- `matched`: ジョブ作成対象
- `ignored`: 条件不一致のため処理対象外
- `skipped`: 既処理や重複のため新規処理不要

Web UI で変更した設定は `yaml` へ反映する。
必要に応じてファイルを直接編集できるようにし、編集後は再読込またはホットリロードで反映する。

## 8. 承認方式

### 8.1 Web 承認

最優先の承認導線。

- 設計文書表示
- 差分表示
- テスト結果表示
- ジョブイベント履歴表示
- AI 実行ログ表示
- 承認 / 却下 / 再実行
- コメント入力

Web UI は参照と承認の導線を提供するが、成果物の実体はワークスペース上のファイルとして保持する。

### 8.2 ファイル承認

補助的な承認導線。

- `artifacts/approvals/<job-id>/design-approval.json`
- `artifacts/approvals/<job-id>/final-approval.json`

プログラムはこれらのファイル変更を監視し、承認イベントへ変換する。

ファイル例:

```json
{
  "status": "approved",
  "comment": "ok",
  "updatedBy": "user",
  "updatedAt": "2026-05-15T10:00:00+09:00"
}
```

ファイル監視も AI ではなく OS / Go の監視機構で実施する。

### 8.3 ファイル直接編集

設計結果、レビュー結果、承認ファイルはワークスペース内に配置し、任意のエディタや VSCode で直接編集可能とする。

想定用途:

- ユーザが設計書を手修正する
- ユーザがレビュー結果を補記する
- GitHub Copilot Chat を使って設計書やレビュー結果を編集する

このため、Web UI はファイル内容を表示するが、編集の主導線は Web に限定しない。

## 9. 通知方式

通知はアダプタ方式にする。

```go
type Notifier interface {
    Notify(ctx context.Context, n Notification) error
}
```

初期実装:

- `WindowsToastNotifier`

将来拡張:

- Slack
- Teams
- Email
- Desktop app notification

通知タイミング:

- 設計完了
- 設計承認待ち
- 実装完了
- 最終確認待ち
- PR レビュー完了
- 失敗

## 10. AI / スキル実行設計

### 10.1 方針

AI 実行基盤は `SkillRunner` と `AIProvider` に分離する。

- `SkillRunner`
  - スキル定義読込
  - 入力コンテキスト構築
  - ワークスペース準備
  - 実行結果の成果物化
- `AIProvider`
  - 実際の AI 実行

### 10.2 Copilot 対応

当面は GitHub Copilot を対象とするため、AI 呼び出しを抽象化する。

```go
type AIProvider interface {
    Run(ctx context.Context, req AIRequest) (AIResult, error)
}
```

初期実装では、GitHub Copilot CLI を外部コマンドとして実行する。

理由:

- CLI ベースでツールから制御しやすい
- 入出力をファイル化しやすく、成果物管理と相性が良い
- 将来別 Provider を追加する余地を残しつつ、初期実装を単純化できる
- ツール本体をプロバイダ非依存にできる

実装候補:

- `CopilotCLIProvider`
  - GitHub Copilot CLI を呼び出す
  - 入力はプロンプトファイルとコンテキストファイル
  - 出力は成果物フォルダへ保存
  - 標準出力 / 標準エラー出力をログとして保存

Copilot CLI の実行オプション差異は、ツール本体ではなくプロバイダ実装またはスキル側設定で吸収する。

### 10.3 Copilot CLI 実行方針

初期設計では、各スキルは以下の流れで実行する。

1. プログラムがプロンプトファイルを生成
2. プログラムが Issue / PR / 設計 / 差分などのコンテキストファイルを生成
3. `CopilotCLIProvider` が GitHub Copilot CLI を実行
4. 出力を成果物ディレクトリへ保存
5. 必要に応じて後処理フックを実行

実行責務:

- コマンド起動、タイムアウト、リトライ、ログ保存はプログラム側
- 設計、実装、レビュー本文の生成は Copilot CLI 側
- 待機、判定、状態遷移はプログラム側

AI 実行時の標準出力 / 標準エラー出力はジョブに紐づくログファイルへ保存し、Web UI から参照可能にする。

### 10.4 スキル定義

スキルはファイルベースで管理する。

例:

```text
skills/
  design/
    skill.yaml
    prompt.md.tmpl
  implement/
    skill.yaml
    prompt.md.tmpl
  review/
    skill.yaml
    prompt.md.tmpl
  fix/
    skill.yaml
    prompt.md.tmpl
```

`skill.yaml` 例:

```yaml
name: design
provider: copilot
mode: oneshot
inputs:
  - issue
  - repository_context
  - custom_rules
outputs:
  - design_doc
artifacts:
  output_dir: artifacts/designs/{{ .JobID }}
hooks:
  pre:
    - scripts/build-design-context.ps1
  post:
    - scripts/validate-design.ps1
```

### 10.5 スキルの責務

- `design`
  - Issue から設計書を生成
- `implement`
  - 承認済み設計に基づき実装
- `fix`
  - テスト失敗 / 指摘に基づき修正
- `review`
  - PR を Issue / 設計 / 変更差分からレビュー

## 11. 成果物配置

```text
artifacts/
  designs/
    <job-id>/
      design.md
      context.json
      ai-stdout.log
      ai-stderr.log
  changes/
    <job-id>/
      summary.md
      test-report.json
      ai-stdout.log
      ai-stderr.log
  reviews/
    <job-id>/
      review.md
      check-results.json
      ai-stdout.log
      ai-stderr.log
  approvals/
    <job-id>/
      design-approval.json
      final-approval.json
  events/
    <job-id>/
      events.jsonl
```

設計確認は `design.md`、実装確認は Git 差分と `summary.md` を中心に行う。
`design.md` や `review.md` はワークスペース内の通常ファイルとして保持し、Web UI だけでなく任意のエディタや VSCode からも編集できるようにする。

イベントログは `events.jsonl` などの追記形式で保持し、監査とデバッグをしやすくする。

## 12. テスト / 修正ループ

実装後はプログラムが定義済みコマンドを実行する。

```go
type TestProfile struct {
    Name     string
    Commands []string
}
```

例:

- `go test ./...`
- `npm test`
- `npm run e2e`
- `make lint`

失敗時:

1. テスト結果を整形
2. `fix` スキルへ入力
3. 修正後に再テスト
4. 最大試行回数到達で失敗終了

ここでも、再試行制御は AI ではなくプログラムが管理する。

## 13. PR 条件チェック

PR に対しては AI レビューとは別に機械的チェックを行う。

チェック例:

- UT 成功
- E2E 成功
- Lint 成功
- 関連 Issue が紐付いている
- 設計成果物が存在する
- 必須ラベルが付与されている
- Draft かどうか
- 変更量上限を超えていないか

この結果を AI レビューへ渡すことで、レビュー精度を安定させる。

## 14. データモデル

### 14.1 DB

永続化は SQLite を推奨する。

理由:

- ローカル常駐ツールに適する
- セットアップが容易
- ジョブ状態管理に十分

SQLite はランタイム状態の保存に使用し、設定値の保存先には使わない。

主テーブル:

- `jobs`
- `job_events`
- `artifacts`
- `notifications`
- `skill_runs`

`jobs` の主要カラム例:

- `id`
- `type` (`issue`, `pr_review`)
- `repository`
- `github_number`
- `state`
- `branch_name`
- `watch_rule_id`
- `created_at`
- `updated_at`

`job_events` の主要カラム例:

- `id`
- `job_id`
- `event_type`
- `state_from`
- `state_to`
- `payload_json`
- `created_at`

`skill_runs` の主要カラム例:

- `id`
- `job_id`
- `skill_name`
- `provider`
- `status`
- `stdout_log_path`
- `stderr_log_path`
- `artifact_dir`
- `started_at`
- `finished_at`

### 14.2 YAML 設定

設定値はワークスペース内の `yaml` ファイルで保持する。

想定配置:

```text
config/
  app.yaml
  watch-rules.yaml
  notifications.yaml
  test-profiles.yaml
```

役割:

- `app.yaml`
  - HTTP ポート
  - ポーリング間隔
  - ワークスペース配下の各種ディレクトリ
  - 利用 Provider
- `watch-rules.yaml`
  - Issue / PR の監視条件
  - 対象リポジトリ
  - 実行スキルセット
- `notifications.yaml`
  - 有効な通知方式
  - 通知タイミング
- `test-profiles.yaml`
  - テストコマンド群
  - PR 条件チェック用プロファイル

設定変更方法:

- Web UI から編集して `yaml` に保存
- 任意のエディタや VSCode から直接編集
- ファイル変更検知または手動再読込で反映

## 14.3 GitHub 認証

GitHub 連携は `gh` コマンドを認証の正とする。

方針:

- ツールは GitHub Token を独自に保持しない
- `gh auth token` で取得したトークンを API クライアントに渡す
- `gh auth status` で事前チェックを行う
- 未ログイン時は Web UI とログで明示的にエラーを出す

想定インターフェース:

```go
type TokenProvider interface {
    Token(ctx context.Context) (string, error)
}
```

初期実装:

- `GHTokenProvider`
  - `gh auth token` を実行してトークン取得
  - 一定時間メモリキャッシュ可能
  - 取得失敗時はジョブ開始前に失敗させる

## 15. Go パッケージ案

```text
cmd/korobokkle/
  main.go

internal/
  app/
    bootstrap.go
  config/
    config.go
    loader.go
  domain/
    job.go
    event.go
    watch_rule.go
    artifact.go
  orchestrator/
    issue_flow.go
    pr_flow.go
    scheduler.go
  github/
    client.go
    watcher.go
    poller.go
    auth.go
  skill/
    runner.go
    loader.go
    provider.go
  notification/
    notifier.go
    windows_toast.go
  approval/
    web_handler.go
    file_watcher.go
  storage/
    sqlite/
  web/
    server.go
    handlers.go
    templates/
  executor/
    command.go
    test_runner.go
  vcs/
    git.go
    pr.go
```

## 16. Web UI 画面案

最低限必要な画面:

- ダッシュボード
  - 実行中ジョブ
  - 承認待ちジョブ
  - 失敗ジョブ
- 監視条件管理
  - ルール一覧
  - ルール作成 / 編集
- Issue ジョブ詳細
  - Issue 情報
  - 設計成果物
  - イベント履歴
  - AI 実行ログ
  - 承認ボタン
  - 実装結果
  - テスト結果
- PR レビュー詳細
  - PR 情報
  - 条件チェック結果
  - AI レビュー結果
  - イベント履歴
  - AI 実行ログ

## 17. 排他制御

同一 Issue / PR の二重処理を防ぐ。

- `repository + issue_number + job_type` で一意制約
- 実行中ジョブは mutex / DB ロックで保護
- 再実行は同一ジョブ再開または明示的なリランとして扱う

## 18. エラーハンドリング

- GitHub API 一時失敗: 指数バックオフ
- AI 実行失敗: スキル実行ログ保存 + 再試行回数制御
- テスト失敗: `fix` フローへ移行
- 通知失敗: ジョブは継続、通知失敗のみ記録
- 承認ファイル不正: エラー表示、状態は維持

## 19. セキュリティ

- GitHub 認証は `gh` のログイン状態を利用する
- `gh auth token` で取得したトークンはメモリ上でのみ扱い、永続化しない
- Web UI にローカル認証を追加可能にする
- AI に渡す情報をスキルで制御できるようにする
- 任意コマンド実行は許可リスト化する

## 20. 初期実装の優先順位

### Phase 1

- Issue / PR ポーリング
- YAML ベース設定読込
- Watch Rule 管理 UI
- SQLite によるジョブ管理
- 設計スキル実行
- Windows Toast 通知
- Web 承認
- 実装スキル実行
- テスト実行
- PR 作成

### Phase 2

- ファイル承認
- PR 条件チェック強化
- レビュースキル改善
- 修正ループ高度化

### Phase 3

- GitHub Webhook
- Slack / Teams 通知
- 複数 AI Provider 対応
- 監査ログ / 権限制御

## 21. この設計で重要な判断

- 常駐監視と承認待ちは Go プログラムが担当する
- AI は都度起動されるワーカーとして扱う
- GitHub 認証は `gh` コマンドに委譲する
- AI 実行は GitHub Copilot CLI を起動する方式にする
- 業務ロジックの差分はスキル定義で変更可能にする
- ユーザ操作は Web を主導線、ファイル承認を補助導線とする

## 22. 次の実装候補

次に着手するなら以下の順が妥当。

1. `cmd/korobokkle/main.go` とアプリ起動骨格
2. YAML ベースの設定読込と保存
3. GitHub Poller
4. Web UI の最小画面
5. Windows Toast Notifier
6. SQLite ベースの `jobs` / `job_events` 永続化
7. GitHub 認証取得 (`gh auth token`) と GitHub Client
8. SkillRunner と CopilotCLIProvider
9. Issue 設計フロー
10. 承認後の実装 + テスト + PR 作成
