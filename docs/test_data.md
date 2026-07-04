# テストデータ一覧

`create_test_data.ps1` で作成されるテストデータを、実際の出力先ごとに整理する。

## 生成先

- 既定ルート: `tests/`
- 作成対象:
  - `config/settings.json`
  - `db/jobs.json`
  - `db/mock_jobs.json`
  - `.workspace/...` の Markdown 成果物
  - `prompt/`, `workspace/`, `state/`, `logs/` のディレクトリ
  - `workspace/<repo-id>/<job-id>/logs/...` のジョブログ

## 設定ファイル

### `config/settings.json`

| 項目 | 値 |
| --- | --- |
| repository | `mock-owner/mock-repo` |
| aiProvider | `codex` |
| pollIntervalSeconds | `3600` |
| baseBranch | `main` |
| branchNamePattern | `issue_#<issueNumber>` |
| aiAllowedCommands | `go test ./...`, `cd frontend && npm test` |

`models` は `codex` と `githubCopilot` の両方が `default` モードで入る。
`issue` / `pullRequest` の検索条件は、すべて空配列で初期化される。

## DB データ

### `db/jobs.json`

通常表示用のジョブ一覧として、次の 4 件を作成する。

| ID | Kind | State | Number | Title | 追加情報 |
| --- | --- | --- | --- | --- | --- |
| `issue-101` | `issue_design` | `completed` | `101` | `login-page-improvements` | `issueContext` あり |
| `issue-102` | `issue_implementation` | `completed` | `102` | `job-detail-panel-improvements` | `issueContext` あり |
| `pr-201` | `pr_review` | `completed` | `201` | `add-filter-conditions` | `issueContext` なし |
| `pr-202` | `pr_feedback` | `completed` | `202` | `review-feedback-fix` | `issueContext` なし |

各ジョブの時刻は次の通り。

| ID | fetchedAt | updatedAt |
| --- | --- | --- |
| `issue-101` | `2026-07-01T00:00:00Z` | `2026-07-01T03:04:05Z` |
| `issue-102` | `2026-07-01T00:10:00Z` | `2026-07-01T03:14:05Z` |
| `pr-201` | `2026-07-01T00:20:00Z` | `2026-07-01T03:24:05Z` |
| `pr-202` | `2026-07-01T00:30:00Z` | `2026-07-01T03:34:05Z` |

### `db/mock_jobs.json`

モックモードでの表示確認用ジョブとして、次の 3 件を作成する。

| ID | Kind | State | Number | Title | 追加情報 |
| --- | --- | --- | --- | --- | --- |
| `issue-301` | `issue_design` | `detected` | `301` | `mock-detected-design` | `issueContext` あり |
| `issue-302` | `issue_implementation` | `design_approved` | `302` | `mock-detected-implementation` | `issueContext` あり |
| `pr-401` | `pr_review` | `review_running` | `401` | `mock-pr-review` | `issueContext` なし |

各ジョブの時刻は次の通り。

| ID | fetchedAt | updatedAt |
| --- | --- | --- |
| `issue-301` | `2026-07-01T00:40:00Z` | `2026-07-01T03:44:05Z` |
| `issue-302` | `2026-07-01T00:50:00Z` | `2026-07-01T03:54:05Z` |
| `pr-401` | `2026-07-01T01:00:00Z` | `2026-07-01T04:04:05Z` |

## workspace 成果物

### `tests/.workspace/design/101_login-page-improvements.md`

- 種別: 設計
- 対応ジョブ: `issue-101`
- 内容: UI テスト用の設計成果物

### `tests/.workspace/implementation/102_job-detail-panel-improvements.md`

- 種別: 実装
- 対応ジョブ: `issue-102`
- 内容: UI テスト用の実装成果物

### `tests/.workspace/review/201_add-filter-conditions.md`

- 種別: レビュー
- 対応ジョブ: `pr-201`
- 内容: UI テスト用のレビュー成果物

### `tests/.workspace/review_fix_design/202_review-feedback-fix.md`

- 種別: レビュー指摘修正の設計
- 対応ジョブ: `pr-202`
- 内容: UI テスト用のレビュー指摘修正成果物

## 補足

- `create_test_data.ps1` は、表示確認しやすいように `completed` と進行中状態を混在させる。
- モックモードでは GitHub への投稿は行わない。
- `tests/` 配下は画面テスト用の固定データとして使う。
