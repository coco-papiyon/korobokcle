# 状態設計

`korobokcle` のジョブ状態を、ソースコード上の定義をもとに整理する。

## 正本

- ドメイン定義: [`internal/domain/job.go`](../internal/domain/job.go)
- UI 表示: [`frontend/src/utils/jobState.ts`](../frontend/src/utils/jobState.ts)
- テストデータ: [`create_test_data.ps1`](../create_test_data.ps1)
- 画面説明: [`README.md`](../README.md), [`docs/design.md`](design.md)

## 結論

- `completed` は全フェーズ共通の終端状態で、`設計完了` や `実装完了` の上位概念ではない。
- `design_ready`、`implementation_ready`、`review_ready` などは、それぞれ独立した「完了直前 / 承認待ち」状態として扱う。
- UI の表示名は日本語、内部管理は英語の enum とラベルで分ける。

## 状態一覧

### 共通

| 内部状態 | 表示名 | 用途 |
| --- | --- | --- |
| `detected` | 検知済み | ジョブの起点 |
| `completed` | 完了 | 終端 |
| `failed` | 失敗 | 終端 |

### 設計系

| 内部状態 | 表示名 | 用途 |
| --- | --- | --- |
| `design_running` | 設計中 | 設計実行中 |
| `design_ready` | 設計完了 | 設計結果の確認待ち |
| `design_approved` | 設計承認済み | 設計承認後、実装へ進む |

### 実装系

| 内部状態 | 表示名 | 用途 |
| --- | --- | --- |
| `implementation_running` | 実装中 | 実装実行中 |
| `implementation_ready` | 実装完了 | 実装結果の確認待ち |
| `implementation_approved` | 実装承認済み | 実装承認後、PR 作成へ進む |

### PR 系

| 内部状態 | 表示名 | 用途 |
| --- | --- | --- |
| `pr_created` | PR済み | PR 作成済み |
| `pr_review_comment` | レビュー指摘あり | PR レビュー指摘を受けた状態 |
| `review_running` | レビュー中 | PR レビュー実行中 |
| `review_ready` | レビュー完了 | レビュー結果の確認待ち |
| `review_approved` | レビュー承認済み | レビュー承認後の終端候補 |

### レビュー指摘対応

| 内部状態 | 表示名 | 用途 |
| --- | --- | --- |
| `review_fixed` | レビュー指摘修正済み | レビュー指摘修正後の次工程起点 |
| `review_fix_design_running` | レビュー指摘検討中 | 設計修正の実行中 |
| `review_fix_design_ready` | レビュー指摘検討済み | 設計修正結果の確認待ち |
| `review_fix_design_approved` | レビュー検討承認済み | 設計修正承認後、実装へ進む |
| `review_fix_implementation_running` | レビュー指摘修正中 | 実装修正の実行中 |
| `review_fix_implementation_ready` | レビュー指摘修正完了 | 実装修正結果の確認待ち |
| `review_fix_implementation_approved` | レビュー指摘修正承認済み | 実装修正承認後の次工程候補 |

### コンフリクト系

| 内部状態 | 表示名 | 用途 |
| --- | --- | --- |
| `pr_conflict` | コンフリクト検知済み | コンフリクト検知の起点 |
| `pr_conflict_running` | コンフリクト解消中 | 解消実行中 |
| `pr_conflict_ready` | コンフリクト解消完了 | 解消結果の確認待ち |
| `pr_conflict_resolved` | コンフリクト解消済み | 解消承認後の終端候補 |

## 状態グループ

一覧画面のフィルターでは、個別状態ではなく次の状態グループを使う。

| グループ | 含める状態 |
| --- | --- |
| `すべて` | 全状態 |
| `未完了` | `completed` 以外の全状態 |
| `実行中` | `design_running`、`implementation_running`、`review_running`、`review_fix_design_running`、`review_fix_implementation_running`、`pr_conflict_running` |
| `承認待ち` | `design_ready`、`implementation_ready`、`review_ready`、`review_fix_design_ready`、`review_fix_implementation_ready`、`pr_conflict_ready` |
| `承認済み` | `design_approved`、`implementation_approved`、`review_approved`、`review_fix_design_approved`、`review_fix_implementation_approved`、`pr_conflict_resolved` |
| `完了` | `completed` |
| `失敗` | `failed` |
| `その他` | 上記に含まれない状態 |

## 遷移の見方

- `issue_design` は `detected` から始まり、`design_running` -> `design_ready` -> `design_approved` と進む。
- `issue_implementation` は `design_approved` から始まり、`implementation_running` -> `implementation_ready` -> `implementation_approved` と進む。
- `pr_review` は `review_running` から始まり、`review_ready` -> `review_approved` と進む。
- `pr_feedback` は `pr_review_comment` または `review_fixed` を起点に、レビュー指摘対応の設計 / 実装へ進む。
- `pr_conflict` は `pr_conflict` から始まり、`pr_conflict_running` -> `pr_conflict_ready` -> `pr_conflict_resolved` と進む。
- `completed` は各フローの最終終端であり、どの `*_ready` でも到達するわけではない。

## テストデータでの使われ方

`create_test_data.ps1` では、状態が個別に分かれていることを確認できるようにしている。

- `issue-101` は `completed`
- `issue-102` は `completed`
- `pr-201` は `completed`
- `pr-202` は `completed`
- `issue-301` は `detected`
- `issue-302` は `design_approved`
- `pr-401` は `review_running`

このため、`完了` を終端状態として使い、`設計完了` / `実装完了` / `レビュー完了` は個別の承認待ち状態として区別して扱うのが正しい。

## UI への反映

- 一覧の状態フィルターは、状態グループをプルダウンで選ぶ形が向く。
- 一覧や詳細の表示名は `jobState.ts` の日本語ラベルを正とする。
- テストデータや画面説明で状態を記述するときは、個別状態と状態グループを区別して書く。
