# テストレコード一覧

`tests/data` に含まれる fixture の一覧です。`go run ./scripts/create-testdata` で再生成される前提で、ジョブの種類と状態名、主なイベント、保存される成果物をまとめています。

## 一覧

| 概要 | 種別 | GitHub 番号 | 状態 | 主なイベント | 主な成果物 |
| --- | --- | ---: | --- | --- | --- |
| 登録直後 (fixture-issue-registered) | issue | 100 | detected | issue_matched | なし |
| 設計済み (fixture-design-ready) | issue | 101 | waiting_design_approval | issue_matched, design_started, design_ready, waiting_design_approval | design/result.md, design/stdout.log, design/stderr.log |
| 実装済み (fixture-implementation-ready) | issue | 102 | waiting_final_approval | issue_matched, design_ready, waiting_design_approval, design_approved, implementation_ready, waiting_final_approval | design/result.md, implementation/result.md, implementation/test-report.json, implementation/stdout.log, implementation/stderr.log |
| PR 作成済み (fixture-pr-created) | issue | 106 | completed | issue_matched, pr_created | pr/result.json, pr/gh-pr-comments.json |
| エラー状態 (fixture-failed) | issue | 103 | failed | issue_matched, design_approved, implementation_failed | implementation/result.md, implementation/test-report.json, implementation/stdout.log, implementation/stderr.log |
| 削除済み (fixture-deleted) | issue | 105 | waiting_design_approval | issue_matched, design_started, design_ready, waiting_design_approval | design/result.md, design/stdout.log, design/stderr.log |
| レビュー済み (fixture-review-completed) | pr_review | 104 | review_ready | pull_request_matched, review_started, review_ready | review/result.md, review/stdout.log, review/stderr.log |
| PR フィードバック完了 (fixture-pr-feedback-completed) | pr_feedback | 107 | completed | pull_request_review_matched, implementation_ready, waiting_final_approval, final_approved, pr_updated | implementation/result.md, implementation/stdout.log, implementation/stderr.log, pr/result.json, pr/gh-pr-comments.json |
| PRコメント分析中 (fixture-pr-comment-analysis-running) | issue | 108 | design_running | issue_matched, pr_created, pr_comment_analysis_requested | implementation/result.md, pr/result.json, pr/gh-pr-comments.json |
| PRコメント分析済み (fixture-pr-comment-analysis-ready) | issue | 109 | waiting_design_approval | issue_matched, pr_created, pr_comment_analysis_requested, pr_comment_analysis_ready | implementation/result.md, pr/result.json, pr/gh-pr-comments.json, pr/result.md |

## 補足

- `fixture-pr-created` は issue 起点で PR を作成した完了レコードです。
- `fixture-pr-created` には改善案 `draft_created` の `jobs/.../improvement/` 監査成果物と shared workdir の `.improvement/draft/*.md` が含まれます。
- `fixture-pr-comment-analysis-ready` には改善案 `approved` の `jobs/.../improvement/` 監査成果物と shared workdir の `.improvement/*.md` / `.improvements/*.md` が含まれます。
- `fixture-failed` には改善不要 `no_improvement_needed` の `jobs/.../improvement/decision.json` が含まれます。
- `fixture-pr-feedback-completed` は PR フィードバックを反映した完了レコードです。
- `fixture-deleted` は論理削除済みのレコードです。UI 上で削除済み一覧の確認に使えます。

## 再生成

```bash
go run ./scripts/create-testdata
```
