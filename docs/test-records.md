# テストレコード一覧

`tests/data` に含まれる fixture の一覧です。`go run ./scripts/create-testdata` で再生成される前提で、ジョブの種類と状態名、主なイベント、保存される成果物をまとめています。改善機能の fixture も同時に生成され、`work/.improvement/` と `work/.improvements/` で確認できます。

## 一覧

| 概要 | 種別 | GitHub 番号 | 状態 | 主なイベント | 主な成果物 |
| --- | --- | ---: | --- | --- | --- |
| 登録直後 (fixture-issue-registered) | issue | 100 | detected | issue_matched | なし |
| 設計済み (fixture-design-ready) | issue | 101 | waiting_design_approval | issue_matched, design_started, design_ready, waiting_design_approval | design/result.md, design/stdout.log, design/stderr.log |
| 実装済み (fixture-implementation-ready) | issue | 102 | waiting_final_approval | issue_matched, design_ready, waiting_design_approval, design_approved, implementation_ready, waiting_final_approval | design/result.md, implementation/result.md, implementation/test-report.json, implementation/stdout.log, implementation/stderr.log |
| PR 作成済み (fixture-pr-created) | issue | 106 | completed | issue_matched, pr_created | pr/result.json, pr/gh-pr-comments.json, improvement/draft.md, improvement/decision.json |
| エラー状態 (fixture-failed) | issue | 103 | failed | issue_matched, design_approved, implementation_failed | implementation/result.md, implementation/test-report.json, implementation/stdout.log, implementation/stderr.log, improvement/decision.json |
| 削除済み (fixture-deleted) | issue | 105 | waiting_design_approval | issue_matched, design_started, design_ready, waiting_design_approval | design/result.md, design/stdout.log, design/stderr.log |
| レビュー済み (fixture-review-completed) | pr_review | 104 | review_ready | pull_request_matched, review_started, review_ready | review/result.md, review/stdout.log, review/stderr.log |
| PR フィードバック完了 (fixture-pr-feedback-completed) | pr_feedback | 107 | completed | pull_request_review_matched, implementation_ready, waiting_final_approval, final_approved, pr_updated | implementation/result.md, implementation/stdout.log, implementation/stderr.log, pr/result.json, pr/gh-pr-comments.json |
| PRコメント分析中 (fixture-pr-comment-analysis-running) | issue | 108 | design_running | issue_matched, pr_created, pr_comment_analysis_requested | implementation/result.md, pr/result.json, pr/gh-pr-comments.json |
| PRコメント分析済み (fixture-pr-comment-analysis-ready) | issue | 109 | waiting_design_approval | issue_matched, pr_created, pr_comment_analysis_requested, pr_comment_analysis_ready | implementation/result.md, pr/result.json, pr/gh-pr-comments.json, pr/result.md, improvement/result.md, improvement/approval.json |

## 補足

- `fixture-pr-created` は issue 起点で PR を作成した完了レコードです。
- `fixture-pr-feedback-completed` は PR フィードバックを反映した完了レコードです。
- `fixture-deleted` は論理削除済みのレコードです。UI 上で削除済み一覧の確認に使えます。
- 改善点画面では `fixture-pr-created` が `draft_created`、`fixture-pr-comment-analysis-ready` が `approved`、`fixture-failed` が `no_improvement_needed` の例になります。
- 承認前 draft は `tests/data/artifacts/workers/coco-papiyon-korobokcle/work/.improvement/`、承認済み方針は `tests/data/artifacts/workers/coco-papiyon-korobokcle/work/.improvements/` に出力されます。

## 再生成

```bash
go run ./scripts/create-testdata
```
