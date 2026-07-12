# テストデータ一覧

`go run ./tests/scripts/create-testdata` で作成されるテストデータを、実際の出力先ごとに整理する。

## 生成先

- 既定ルート: `tests/`
- 作成対象:
  - `config/settings.json`
  - `db/jobs.json`
  - `db/mock_jobs.json`
  - `.workspace/...` の Markdown 成果物
  - `workspace/<repo-id>/<job-id>/logs/...` のジョブログ
  - `prompt/`, `workspace/`, `state/`, `logs/` のディレクトリ

## 設定ファイル

### `config/settings.json`

| 項目 | 値 |
| --- | --- |
| `repository` | `mock-owner/mock-repo` |
| `aiProvider` | `codex` |
| `pollIntervalSeconds` | `3600` |
| `baseBranch` | `main` |
| `branchNamePattern` | `issue_#<issueNumber>` |
| `aiAllowedCommands` | `go test ./...`, `cd frontend && npm test` |

`models` は `codex` と `githubCopilot` の両方が `default` モードで入る。
`issue` / `pullRequest` の検索条件は、すべて空配列で初期化される。

## DB データ

### `db/jobs.json`

通常表示用のジョブ一覧として、各状態を 1 件以上含む固定データを作成する。

- `issue_design`
  - `detected`
  - `design_running`
  - `design_ready`
  - `design_approved`
  - `completed`
  - `failed`
- `issue_implementation`
  - `implementation_running`
  - `implementation_ready`
  - `implementation_approved`
  - `pr_created`
- `pr_review`
  - `review_running`
  - `review_ready`
  - `review_approved`
- `pr_feedback`
  - `pr_review_comment`
  - `review_fix_design_running`
  - `review_fix_design_ready`
  - `review_fix_design_approved`
  - `review_fix_implementation_running`
  - `review_fix_implementation_ready`
  - `review_fix_implementation_approved`
  - `review_fixed`
- `pr_conflict`
  - `pr_conflict`
  - `pr_conflict_running`
  - `pr_conflict_ready`
  - `pr_conflict_resolved`

補足:

- `issue_*` のジョブには `issueContext` を含める
- `issue-201` は `subStatus: 検証(2回目)` を持つ
- `issue-203` はチャット画面の見本として、Markdown の表・引用・コードブロック・HTML を含む成果物を持つ
- `failed` 状態のジョブには `failedFromState` と `errorMessage` を含める

### `db/mock_jobs.json`

モックモードの入力データとして、`db/jobs.json` と同じ状態構成を `[]domain.Job` 形式で作成する。

モックモードでの動作:

- Poller は `mock_jobs.json` の状態を `jobs.json` に反映する
- ジョブは自動実行しない
- そのため、各ジョブは作成済みの状態で停止したまま表示される

## 成果物

確認対象の状態で詳細画面を開いたときにエラーにならないよう、成果物ファイルも生成する。

対象状態:

- `design_ready`
- `design_approved`
- `implementation_ready`
- `implementation_approved`
- `pr_created`
- `review_ready`
- `review_approved`
- `review_fix_design_approved`
- `review_fix_implementation_ready`
- `review_fix_implementation_approved`
- `review_fixed`
- `pr_conflict_ready`
- `pr_conflict_resolved`
- `completed`

主な保存先:

- `tests/.workspace/design/...`
- `tests/.workspace/implementation/...`
- `tests/.workspace/review/...`
- `tests/.workspace/review_fix_implementation/...`
- `tests/.workspace/pr_conflict/...`

## ログ

`detected` を除く各ジョブに、ジョブ詳細画面で確認できるログを生成する。

- 保存先: `tests/workspace/mock-owner_mock-repo/<job-id>/logs/...`
- 実装ジョブには `agent` と `verifier` の両方のログを作る
- それ以外のジョブには `agent` ログを作る
- `issue-203` はチャット表示確認用に、会話を意識した `agent` ログを含める

## 補足

- `go run ./tests/scripts/create-testdata` は、画面確認用に各状態を固定表示できるデータセットを作る
- モックモードでは GitHub への投稿と AI 実行は行わない
- `tests/` 配下は画面テスト用の固定データとして使う
