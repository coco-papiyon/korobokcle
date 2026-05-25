# Status Transitions

`korobokcle` のジョブステータスと主な遷移を、現在のコード実装に基づいて整理したものです。

## Status List

| Status | 概要 |
| --- | --- |
| `detected` | Issue マッチ直後の初期状態 |
| `design_running` | 設計生成の実行中 |
| `design_ready` | 設計成果物の生成完了 |
| `waiting_design_approval` | 設計承認待ち |
| `implementation_running` | 実装の実行中 |
| `test_running` | 実装後テストの実行中 |
| `implementation_ready` | 実装成果物の生成完了 |
| `waiting_final_approval` | 最終承認待ち |
| `pr_creating` | PR 作成または PR 更新処理中 |
| `collecting_context` | PR レビュー系ジョブの初期状態 |
| `checks_running` | 定義はあるが、現状の遷移コードでは未使用 |
| `review_running` | レビュー生成の実行中 |
| `review_ready` | レビュー結果の生成完了、承認待ち |
| `completed` | 完了 |
| `failed` | 失敗 |
| `interrupted` | 起動復旧時に中断扱いへ変更された状態 |
| `design_rejected` | 設計差し戻し |
| `final_rejected` | 最終差し戻し |

## `issue` Job

### Initial State

| Trigger | Initial Status |
| --- | --- |
| `issue_matched` | `detected` |

### Normal Flow

| From | Event / Action | To | 補足 |
| --- | --- | --- | --- |
| `detected` | `design_started` | `design_running` | 設計ワーカー開始 |
| `design_running` | `design_ready` | `design_ready` | 設計成果物出力後 |
| `design_ready` | `waiting_design_approval` | `waiting_design_approval` | UI 承認待ちへ移行 |
| `waiting_design_approval` | `design_approved` | `implementation_running` | 設計承認 |
| `waiting_design_approval` | `design_rejected` | `design_rejected` | 設計差し戻し |
| `implementation_running` | `test_started` | `test_running` | テストプロファイルがある場合 |
| `implementation_running` | `implementation_ready` | `implementation_ready` | テストをスキップした場合など |
| `test_running` | `implementation_ready` | `implementation_ready` | テスト成功後 |
| `implementation_ready` | `waiting_final_approval` | `waiting_final_approval` | UI 最終承認待ちへ移行 |
| `waiting_final_approval` | `final_approved` | `pr_creating` | 最終承認 |
| `waiting_final_approval` | `final_rejected` | `final_rejected` | 最終差し戻し |
| `pr_creating` | `pr_created` | `completed` | 新規 PR 作成完了 |

### Failure / Rerun / Recovery

| From | Event / Action | To | 補足 |
| --- | --- | --- | --- |
| `design_running` | `design_failed` | `failed` | 設計処理失敗 |
| `implementation_running` | `implementation_failed` | `failed` | 実装処理失敗 |
| `test_running` | `test_failed` | `failed` | テスト失敗 |
| `pr_creating` | `pr_create_failed` / `pr_push_failed` | `failed` | PR 作成失敗 |
| `design_rejected` | `design_rerun_requested` | `detected` | 設計再実行 |
| `design_running` | `design_rerun_requested` | `detected` | 設計再実行 |
| `detected` | `design_rerun_requested` | `detected` | 設計再実行 |
| `final_rejected` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `implementation_running` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `test_running` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `implementation_ready` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `pr_creating` | `pr_rerun_requested` | `pr_creating` | PR 作成再実行 |
| `design_running` | `design_interrupted` | `interrupted` | 起動時復旧 |
| `implementation_running` | `implementation_interrupted` | `interrupted` | 起動時復旧 |
| `test_running` | `test_interrupted` | `interrupted` | 起動時復旧 |
| `pr_creating` | `pr_interrupted` | `interrupted` | 起動時復旧 |
| `failed` | `design_rerun_requested` | `detected` | 直近イベントが設計系の場合 |
| `failed` | `implementation_rerun_requested` | `implementation_running` | 直近イベントが実装系の場合 |
| `failed` | `pr_rerun_requested` | `pr_creating` | 直近イベントが PR 系の場合 |
| `interrupted` | `design_rerun_requested` | `detected` | 直近イベントが設計系の場合 |
| `interrupted` | `implementation_rerun_requested` | `implementation_running` | 直近イベントが実装系の場合 |
| `interrupted` | `pr_rerun_requested` | `pr_creating` | 直近イベントが PR 系の場合 |

## `pr_review` Job

### Initial State

| Trigger | Initial Status |
| --- | --- |
| `pull_request_matched` | `collecting_context` |

### Normal Flow

| From | Event / Action | To | 補足 |
| --- | --- | --- | --- |
| `collecting_context` | `review_started` | `review_running` | レビューワーカー開始 |
| `review_running` | `review_ready` | `review_ready` | レビュー成果物出力後 |
| `review_ready` | `review_approved` | `completed` | レビュー承認 |

### Failure / Rerun / Recovery

| From | Event / Action | To | 補足 |
| --- | --- | --- | --- |
| `collecting_context` | `review_failed` | `failed` | 文脈収集中またはレビュー処理失敗 |
| `review_running` | `review_failed` | `failed` | レビュー処理失敗 |
| `collecting_context` | `review_rerun_requested` | `collecting_context` | レビュー再実行 |
| `review_running` | `review_rerun_requested` | `collecting_context` | レビュー再実行 |
| `review_ready` | `review_rerun_requested` | `collecting_context` | レビュー再実行 |
| `collecting_context` | `review_interrupted` | `interrupted` | 起動時復旧 |
| `review_running` | `review_interrupted` | `interrupted` | 起動時復旧 |
| `failed` | `review_rerun_requested` | `collecting_context` | 直近イベントがレビュー系の場合 |
| `interrupted` | `review_rerun_requested` | `collecting_context` | 直近イベントがレビュー系の場合 |

## `pr_feedback` Job

### Initial State

| Trigger | Initial Status |
| --- | --- |
| `pull_request_review_matched` | `implementation_running` |

### Normal Flow

| From | Event / Action | To | 補足 |
| --- | --- | --- | --- |
| `implementation_running` | `test_started` | `test_running` | テストプロファイルがある場合 |
| `implementation_running` | `implementation_ready` | `implementation_ready` | テストをスキップした場合など |
| `test_running` | `implementation_ready` | `implementation_ready` | テスト成功後 |
| `implementation_ready` | `waiting_final_approval` | `waiting_final_approval` | UI 最終承認待ちへ移行 |
| `waiting_final_approval` | `final_approved` | `pr_creating` | 最終承認 |
| `waiting_final_approval` | `final_rejected` | `final_rejected` | 最終差し戻し |
| `pr_creating` | `pr_updated` | `completed` | 既存 PR への修正反映完了 |

### Failure / Rerun / Recovery

| From | Event / Action | To | 補足 |
| --- | --- | --- | --- |
| `implementation_running` | `implementation_failed` | `failed` | 実装処理失敗 |
| `test_running` | `test_failed` | `failed` | テスト失敗 |
| `pr_creating` | `pr_comment_failed` / `pr_push_failed` / `pr_create_failed` | `failed` | PR コメント・更新失敗 |
| `final_rejected` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `implementation_running` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `test_running` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `implementation_ready` | `implementation_rerun_requested` | `implementation_running` | 実装再実行 |
| `pr_creating` | `pr_rerun_requested` | `pr_creating` | PR 更新再実行 |
| `implementation_running` | `implementation_interrupted` | `interrupted` | 起動時復旧 |
| `test_running` | `test_interrupted` | `interrupted` | 起動時復旧 |
| `pr_creating` | `pr_interrupted` | `interrupted` | 起動時復旧 |
| `failed` | `implementation_rerun_requested` | `implementation_running` | 直近イベントが実装系の場合 |
| `failed` | `pr_rerun_requested` | `pr_creating` | 直近イベントが PR 系の場合 |
| `interrupted` | `implementation_rerun_requested` | `implementation_running` | 直近イベントが実装系の場合 |
| `interrupted` | `pr_rerun_requested` | `pr_creating` | 直近イベントが PR 系の場合 |

## Non-State Changes

| Operation | Event | Note |
| --- | --- | --- |
| 論理削除 | `job_deleted` | `deleted_at` を更新。`state` 自体は変わらない |
| 復元 | `job_restored` | `deleted_at` を解除。`state` 自体は変わらない |
| 物理削除 | なし | `purge` は削除済みジョブを DB から完全削除 |

## Notes

- `review_ready` はレビュー実行済みで、承認待ちの状態です。
- `ApproveFinal` は `issue` / `pr_feedback` ジョブで、通常 `waiting_final_approval` からのみ許可されます。
  例外として、`failed` でも直近イベントが `test_failed` の場合のみ許可されます。
- `pr_feedback` ジョブは `pull_request_review_matched` で開始し、レビューコメント反映のため `implementation_running` から始まります。
- 本書は現在のコード上の実装整理であり、状態遷移の妥当性そのものを保証する仕様書ではありません。
