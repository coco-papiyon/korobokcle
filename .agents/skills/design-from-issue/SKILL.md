---
name: design-from-issue
description: Issueをもとに設計をまとめる。Issue起点で要件整理、設計案、変更対象、テスト計画、リスクを整えるときに使う。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: issue_design -->

## 必須出力形式
次のレベル2見出しだけをこの順で出力する。

## 概要
## 要件
## 設計
## 変更対象ファイル
## テスト計画
## リスク

`テスト計画` には、テスト方法を明記する。
Go言語を修正した場合は、 `go test ./...` 、npm(Vue.js)を修正した場合は `cd frontend && npm ci && npm test` を記載する。
