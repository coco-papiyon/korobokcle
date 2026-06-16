# Test Data

動作確認用の fixture です。`KOROBOKCLE_TOOL_ROOT=test/data` を指定して起動すると、
この配下の `config/`、`data/`、`artifacts/` を使って UI を確認できます。

含まれるジョブ:

- `fixture-design-ready`: 設計済み。状態は `waiting_design_approval`
- `fixture-implementation-ready`: 実装済み。状態は `waiting_final_approval`
- `fixture-failed`: エラー状態。状態は `failed`
- `fixture-review-completed`: レビュー実行済みで承認待ち。状態は `review_ready`
- `fixture-deleted`: 削除済み。状態は `waiting_design_approval`

改善機能の fixture:

- `fixture-pr-created`: `draft_created`
- `fixture-pr-comment-analysis-ready`: `approved`
- `fixture-failed`: `no_improvement_needed`

改善機能の確認ポイント:

- `source/coco-papiyon-korobokcle-improvement/.improvement/`: 下書き中の改善案
- `source/coco-papiyon-korobokcle-improvement/.improvement/`: 承認済みの改善指示
- `artifacts/coco-papiyon-korobokcle/jobs/issue_<番号>/improvement/`: ジョブごとの監査スナップショット

再生成:

```powershell
go run ./scripts/create-testdata
```
