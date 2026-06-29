---
name: implement-review-fix
description: レビュー指摘を反映して実装を更新する。レビュー後に実装の修正版をまとめるときに使う。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: review_feedback_implementation -->

## 必須出力形式
次のレベル2見出しだけをこの順で出力する。

## 概要
## 変更内容
## テスト結果
## 残課題

`テスト結果` には、指定された `go test ./...` と `cd frontend && npm test` をすべて記載し、各コマンドの実行結果と実際の修正回数を `最大3回中X回` の形で記載する。
frontend のテストでは必要に応じて `npm ci` を実行する。
