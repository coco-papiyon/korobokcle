# korobokcle AI 設計書

## 1. 目的

`korobokcle` で利用する AI の役割分担と、処理ごとに AI へ渡す指示を整理する。
この文書では、設計検討、実装、検証、レビュー、レビュー指摘対応、PR コンフリクト解消で、どの役割を使い、どのようなプロンプトを与えるかを定義する。

## 2. 基本方針

- AI は 1 つの処理につき 1 つの役割に寄せる
- 指示は必要最小限にする
- 役割ごとに責務を分ける
- 実装時の worktree とログ出力先は `work_dir` 配下に統一する
- 出力は原則 Markdown とする
- 検証は設計書に書かれた確認項目とテスト計画に従う

## 3. 役割定義

### 3.1 実装者

担当範囲:

- 設計検討
- 実装
- レビュー指摘対応の実装
- PR コンフリクト解消

役割:

- 要件を整理する
- 変更を実装する
- 差分や指摘を反映する
- 必要なファイル修正を行う

### 3.2 検証者

担当範囲:

- 実装後の検証
- 設計書に対する受入基準の確認

役割:

- 実装結果が設計を満たすか確認する
- テストコマンドを実行する
- OK / NG を判定する
- NG の場合は、再実装向けの具体的な修正指示を返す

### 3.3 レビューア

担当範囲:

- PR レビュー

役割:

- PR 差分を読む
- 設計と実装の整合を確認する
- 問題点を重要度順に整理する
- OK / NG を返す

## 4. 設定との対応

### 4.1 既定プロバイダー

- 実装者: `aiProvider` / `models`
- 検証者: `verificationAiProvider` / `verificationAiModel`
- レビューア: `reviewerAiProvider` / `reviewerAiModel`

### 4.2 既定値の扱い

- 検証者が未指定の場合は、実装者の AI 設定を使う
- レビューアが未指定の場合は、実装者の AI 設定を使う
- モデルが `default` の場合は、選択したプロバイダーの既定モデルを使う

### 4.3 プロバイダー選択の考え方

- 設計検討と実装は、同じ実装者設定を基本にする
- 実装検証は、実装者とは別の設定に切り替えられる
- PR レビューは、レビュー専用の設定を切り替えられる

## 5. 処理別設計

### 5.1 一覧

| 処理 | 使用役割 | 使用スキル | 概要 |
| --- | --- | --- | --- |
| 設計検討 | 実装者 | `design-from-issue` | Issue と ADR をもとに、実装前に設計・画面・受入基準を確定する |
| 実装 | 実装者 | `implement-from-design` | 設計書をもとに、worktree 上でソース修正と実装確認を行う |
| 実装検証 | 検証者 | `verifier-from-design` | 設計の受入基準とテスト計画に沿って、実装結果の OK / NG を判定する |
| PR レビュー | レビューア | `review-pull-request` | PR 差分と設計・実装結果を照合し、指摘事項と確認事項を整理する |
| レビュー指摘対応の実装 | 実装者 | `review-comment-fix` | レビューコメントを反映して再実装する |
| PR コンフリクト解消 | 実装者 | `resolve-pr-conflicts` | head/base の競合を解消し、両方の意図を保つ |

### 5.2 設計検討

#### 入力

- Issue 本文
- 関連 ADR
- 既存の設計文書
- 画面や API の現状
- 受入基準
- 変更対象の制約

#### プロンプトに含める内容

- 対象 Issue の要約
- 実現したい機能
- 画面設計で確定すべき表示状態
- 設計で決めるべき変更対象
- テスト観点
- リスク

#### systemメッセージ

概要: 既存のリポジトリ指示を前提に、余計な手順を増やさず、簡潔な日本語 Markdown で出力する。

```text
You are an autonomous software engineer. Follow the repository instructions with minimal extra process and produce concise Japanese Markdown.
```

#### userメッセージ

この処理では、`design-from-issue` スキル全文を `Mandatory Agent Skill instructions (design-from-issue):` の直後にそのまま投入する。
さらに、コードで追加する固定文は次の通り。

```text
phase: {artifactSubdir(job)}
job_id: {job.ID}
job_kind: {job.Kind}
repository: {job.Repository}
number: {job.Number}
title: {job.Title}
provider: {provider}
model: {model}
running_state: {runningState}
ready_state: {readyState}
working_dir: {workDir}

GitHub context:
{contextText}

Return only Markdown in Japanese.
```

#### 出力

- 設計書
- 画面イメージ
- 変更対象
- 受入基準
- テスト計画

### 5.2 実装

#### 使用役割

- 実装者

#### 使用スキル

- `implement-from-design`

#### 入力

- 設計書
- Issue / PR 文脈
- 既存コード
- worktree パス
- 出力先パス

#### プロンプトに含める内容

- 設計書の要点
- 実装対象
- 変更してよい範囲
- テストで確認すべき項目
- 出力形式

#### systemメッセージ

概要: 既存のリポジトリ指示を前提に、余計な手順を増やさず、直接編集して簡潔な日本語 Markdown で報告する。

```text
You are an autonomous software engineer. Follow the repository instructions with minimal extra process. Edit the repository directly and report the result in concise Japanese Markdown.
```

#### userメッセージ

この処理では、`implement-from-design` スキル全文を `Mandatory Agent Skill instructions (implement-from-design):` の直後にそのまま投入する。
さらに、コードで追加する固定文は次の通り。

```text
phase: {artifactSubdir(job)}
job_id: {job.ID}
job_kind: {job.Kind}
repository: {job.Repository}
number: {job.Number}
title: {job.Title}
provider: {provider}
model: {model}
running_state: {runningState}
ready_state: {readyState}
working_dir: {workDir}
branch: {branch}

GitHub context:
{contextText}

All repository file reads, edits, and commands must use working_dir as the repository root.
Do not access the original repository root or any path outside working_dir.
Use paths relative to working_dir whenever possible.

Repository files:
{repoFileList}

Implement the requested changes directly in working_dir.
Run appropriate tests or checks after editing.
Return only a Markdown summary in Japanese. Do not return JSON or a git diff.
```

#### 出力

- 実装結果
- 変更内容の要約
- テスト結果
- 残課題

### 5.3 実装検証

#### 使用役割

- 検証者

#### 使用スキル

- `verifier-from-design`

#### 入力

- 設計書
- 実装結果
- 実装コード
- テスト計画
- テストコマンド

#### プロンプトに含める内容

- 受入基準
- 実際に確認したコマンド
- OK / NG の判定条件
- NG 時の修正ポイント

#### systemメッセージ

概要: 実装内容を検査・テストするが、リポジトリは変更しない独立した検証者として振る舞う。

```text
You are an independent software verification agent. Inspect and test the implementation without modifying the repository.
```

#### userメッセージ

この処理では、`verifier-from-design` スキル全文を `Mandatory Agent Skill instructions (verifier-from-design):` の直後にそのまま投入する。
さらに、コードで追加する固定文は次の通り。

```text
job_id: {job.ID}
attempt: {attempt}/{maxAttempts}
working_dir: {workDir}
branch: {branch}

GitHub context:
{contextText}

Implementation agent summary:
{implementationArtifact}

Inspect the current changes in working_dir and run the tests required by the design and repository instructions.
Do not edit, create, delete, or format repository files. Your role is verification only.
Return only one JSON object: {"status":"passed|changes_requested","feedback":"specific instructions for the implementer","summary":"Japanese verification summary"}.
Use passed only when the implementation and required tests are acceptable.
```

#### 出力

- `OK` / `NG`
- 判定理由
- 検証結果の要約
- 追加確認事項

### 5.4 PR レビュー

#### 使用役割

- レビューア

#### 使用スキル

- `review-pull-request`

#### 入力

- PR 差分
- Issue / ADR
- 実装結果
- テスト結果
- 関連ラベルや状態

#### プロンプトに含める内容

- 要件と実装の照合
- 重要度順の指摘
- 確認事項
- OK / NG の判定

#### systemメッセージ

概要: 既存のリポジトリ指示を前提に、余計な手順を増やさず、簡潔な日本語 Markdown で出力する。

```text
You are an autonomous software engineer. Follow the repository instructions with minimal extra process and produce concise Japanese Markdown.
```

#### userメッセージ

この処理では、`review-pull-request` スキル全文を `Mandatory Agent Skill instructions (review-pull-request):` の直後にそのまま投入する。
さらに、コードで追加する固定文は次の通り。

```text
phase: {artifactSubdir(job)}
job_id: {job.ID}
job_kind: {job.Kind}
repository: {job.Repository}
number: {job.Number}
title: {job.Title}
provider: {provider}
model: {model}
running_state: {runningState}
ready_state: {readyState}
working_dir: {workDir}
branch: {branch}

GitHub context:
{contextText}

Return only Markdown in Japanese.
```

#### 出力

- `OK` / `NG` / `コメントあり`
- 要件に対する実装状況
- 指摘事項
- 確認事項

### 5.5 レビュー指摘対応の実装

#### 使用役割

- 実装者

#### 使用スキル

- `review-comment-fix`

#### 入力

- PR コメント
- レビュー結果
- 既存実装
- worktree パス

#### プロンプトに含める内容

- 指摘内容の要約
- 修正対象のファイル
- 期待する修正範囲
- 再実装後に確認する項目

#### systemメッセージ

概要: 既存のリポジトリ指示を前提に、余計な手順を増やさず、直接編集して簡潔な日本語 Markdown で報告する。

```text
You are an autonomous software engineer. Follow the repository instructions with minimal extra process. Edit the repository directly and report the result in concise Japanese Markdown.
```

#### userメッセージ

この処理では、`review-comment-fix` スキル全文を `Mandatory Agent Skill instructions (review-comment-fix):` の直後にそのまま投入する。
さらに、コードで追加する固定文は次の通り。

```text
phase: {artifactSubdir(job)}
job_id: {job.ID}
job_kind: {job.Kind}
repository: {job.Repository}
number: {job.Number}
title: {job.Title}
provider: {provider}
model: {model}
running_state: {runningState}
ready_state: {readyState}
working_dir: {workDir}
branch: {branch}

GitHub context:
{contextText}

User comment:
{feedback}
```

#### 出力

- 修正版の要約
- 反映した指摘
- 残課題

### 5.6 PR コンフリクト解消

#### 使用役割

- 実装者

#### 使用スキル

- `resolve-pr-conflicts`

#### 入力

- PR の競合情報
- head/base ブランチ
- 関連 Issue
- 競合ファイル
- worktree パス

#### プロンプトに含める内容

- 競合の種類
- 両方の変更意図
- 解消方針
- テスト確認項目

#### systemメッセージ

概要: マージコンフリクトを慎重に解消し、可能なら両方の Issue の意図を保ったうえで、簡潔な日本語 Markdown で報告する。

```text
You are an autonomous software engineer. Resolve merge conflicts carefully, preserve both issue intents when possible, and report the result in concise Japanese Markdown.
```

#### userメッセージ

この処理では、`resolve-pr-conflicts` スキル全文を `Mandatory Agent Skill instructions (resolve-pr-conflicts):` の直後にそのまま投入する。
さらに、コードで追加する固定文は次の通り。

```text
phase: {artifactSubdir(job)}
job_id: {job.ID}
job_kind: {job.Kind}
repository: {job.Repository}
number: {job.Number}
title: {job.Title}
provider: {provider}
model: {model}
running_state: {runningState}
ready_state: {readyState}
working_dir: {workDir}
branch: {branch}

GitHub context:
{contextText}

All repository file reads, edits, and commands must use working_dir as the repository root.
Do not access the original repository root or any path outside working_dir.
Use paths relative to working_dir whenever possible.

Resolve the merge conflicts directly in working_dir.
Keep the intent of both issues and branches in mind while editing.
```

#### 出力

- 解消結果
- 競合解消時の判断
- 残課題

## 6. 共通プロンプト項目

すべての処理で共通して渡す情報は次の通り。

- フェーズ名
- 対象ジョブの ID
- Issue / PR 番号
- タイトル
- リポジトリ名
- 状態
- worktree パス
- 保存先パス
- 追加指示
- 既存成果物

## 7. プロンプト設計ルール

- システムプロンプトは役割の責務だけを短く伝える
- ユーザプロンプトは処理ごとの入力だけに絞る
- 実装詳細や内部クラス構成は、必要な場合だけ指示する
- 返答形式は Markdown で固定する
- 検証系は `OK` / `NG` を明示する
- レビュー系は結論と指摘を分けて書く

## 8. 出力の扱い

- 設計結果は人間が確認し、承認可否を決める
- 実装結果は人間が差分と合わせて確認する
- 検証結果は次の再実装に流用する
- レビュー結果は Issue へのコメントや修正依頼に流用する

## 9. まとめ

`korobokcle` の AI 処理は、次の 3 役割で整理する。

1. 実装者
2. 検証者
3. レビューア

各処理は、役割ごとに必要最小限のプロンプトを受け取り、Markdown で出力する。
これにより、設計検討から実装、検証、レビュー、修正対応までを一貫した流れで扱える。
