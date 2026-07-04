# korobokcle API 仕様書

## 1. 目的

この文書は、`korobokcle` の現行実装に対応する HTTP API を整理した設計書である。

- フロントエンドの実装基準にする
- テストデータやモックの入出力を合わせる
- 画面から見える JSON 形と内部状態の関係を明示する

## 2. 共通仕様

- すべて JSON を返す
- リクエストボディがある場合は `Content-Type: application/json` を使用する
- 時刻は UTC の RFC3339Nano 形式で返す
- 存在しないリソースは `404`
- 入力不正は `400`
- 未設定の機能は `503`
- 画面側の更新は原則 `cache: no-store` を前提にする

## 3. エンドポイント一覧

| Method | Path | 概要 |
| --- | --- | --- |
| `GET` | `/healthz` | ヘルスチェック |
| `GET` | `/api/settings` | 監視・実行設定の取得 |
| `PUT` | `/api/settings` | 監視・実行設定の保存 |
| `GET` | `/api/jobs` | ジョブ一覧の取得 |
| `POST` | `/api/jobs` | ジョブの作成 |
| `GET` | `/api/jobs/:id` | ジョブ詳細の取得 |
| `PATCH` | `/api/jobs/:id/state` | ジョブ状態の更新 |
| `DELETE` | `/api/jobs/:id` | ジョブ削除 |
| `GET` | `/api/jobs/:id/artifact` | 成果物の取得 |
| `POST` | `/api/jobs/:id/artifact` | 成果物の承認 |
| `PATCH` | `/api/jobs/:id/artifact` | 成果物の再実行 |
| `POST` | `/api/jobs/:id/artifact/request-changes` | 修正依頼 |
| `GET` | `/api/skills` | スキル状態の一覧取得 |
| `POST` | `/api/skills` | スキル生成 |

## 4. データモデル

### 4.1 Job

`GET /api/jobs` と `GET /api/jobs/:id` で返す基本ジョブ情報。

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `id` | string | ジョブ ID |
| `kind` | string | `issue_design` などの種別 |
| `state` | string | 内部状態 |
| `subStatus` | string? | 実装ループ中の補助状態 |
| `repository` | string | 対象リポジトリ |
| `number` | number | Issue / PR 番号 |
| `title` | string | タイトル |
| `branch` | string? | 対象ブランチ |
| `aiProvider` | string? | 実行時 AI プロバイダー |
| `aiModel` | string? | 実行時 AI モデル |
| `issueContext` | string? | Issue 本文などの参照情報 |
| `errorMessage` | string? | エラー内容 |
| `failedFromState` | string? | 失敗元の状態 |
| `fetchedAt` | string? | 取得日時 |
| `updatedAt` | string? | 更新日時 |

### 4.2 WatchSettings

`GET /api/settings` / `PUT /api/settings` で使う設定。

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `repository` | string | 監視対象リポジトリ |
| `aiProvider` | string | 既定の AI プロバイダー |
| `pollIntervalSeconds` | number | 監視間隔 |
| `jobConcurrency` | number | 同時実行数 |
| `implementationLoopCount` | number | 実装ループ回数 |
| `verificationAiProvider` | string? | 検証者用 AI プロバイダー |
| `verificationAiModel` | object? | 検証者用モデル選択 |
| `baseBranch` | string | PR 作成時のベースブランチ |
| `branchNamePattern` | string | ブランチ名ルール |
| `aiAllowedCommands` | string[] | 許可コマンド |
| `codexAllowedCommands` | string[]? | 後方互換用入力 |
| `models` | object | プロバイダー別モデル定義 |
| `issue` | object | Issue 監視条件 |
| `pullRequest` | object | PR 監視条件 |

### 4.3 SkillStatus

`GET /api/skills` の `skills` に含まれる。

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `purpose` | string | スキル種別 |
| `name` | string | スキル名 |
| `displayName` | string | 表示名 |
| `exists` | boolean | ローカル存在 |
| `aiExists` | boolean | AI 判定済み存在 |
| `generated` | boolean | 生成済み |
| `path` | string? | 保存先 |

### 4.4 JobDetailResponse

`GET /api/jobs/:id` の返却形式。

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `updatedAt` | string | 詳細取得時点の更新日時 |
| `job` | Job | ジョブ本体 |
| `branch` | string | 解決済みブランチ名 |
| `issueContext` | string? | Issue 文脈 |
| `logs` | JobLogGroup[]? | 役割・試行ごとのログ |

### 4.5 JobLogGroup / JobLogFile

`GET /api/jobs/:id` で返すログ情報。

`JobLogGroup`

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `role` | string | `agent` / `verifier` など |
| `roleLabel` | string | 画面表示用ラベル |
| `attempt` | number | 試行回数 |
| `files` | JobLogFile[] | 該当ログファイル |

`JobLogFile`

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `kind` | string | `activity` / `stdout` / `stderr` |
| `label` | string | 表示名 |
| `path` | string | 表示用パス |
| `content` | string | ログ本文 |

### 4.6 DesignArtifact

`GET /api/jobs/:id/artifact` の返却形式。

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `content` | string | 成果物本文 |
| `path` | string | 保存先パス |

### 4.7 SkillGenerationRequest / Result

`POST /api/skills` で使う。

`SkillGenerationRequest`

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `projectContext` | string | プロジェクト固有情報 |
| `testCommand` | string | 設計に従うテストコマンド |
| `maxFixLoops` | number | 再修正上限 |
| `forcePurposes` | string[] / string / object | 生成対象の強制指定 |
| `overwriteExisting` | boolean | 既存上書き可否 |

`SkillGenerationResult`

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `provider` | string | 生成に使った AI プロバイダー |
| `skills` | SkillStatus[] | 生成結果 |
| `message` | string | 生成結果メッセージ |

## 5. エンドポイント詳細

### 5.1 `GET /healthz`

疎通確認用。

#### Response

```json
{ "status": "ok" }
```

### 5.2 `GET /api/jobs`

ジョブ一覧を返す。フロントエンド側で `kind`、状態グループ、並び順を制御する前提で、API 側は全件返却する。`subStatus` は実装ジョブの途中経過を表す補助情報として含めるが、一覧画面では表示しない。

#### Response

```json
{
  "updatedAt": "2026-07-04T12:09:33.0740984Z",
  "jobs": [
    {
      "id": "issue-102",
      "kind": "issue_implementation",
      "state": "implementation_ready",
      "subStatus": "検証(1回目)",
      "repository": "mock-owner/mock-repo",
      "number": 102,
      "title": "job-detail-panel-improvements",
      "fetchedAt": "2026-07-01T00:10:00Z",
      "updatedAt": "2026-07-01T03:14:05Z"
    }
  ]
}
```

### 5.3 `POST /api/jobs`

ジョブを新規作成する。

#### Request

```json
{
  "kind": "issue_design",
  "repository": "owner/repo",
  "number": 42,
  "title": "design the thing"
}
```

#### Response

`201 Created`

```json
{
  "id": "issue_design-owner-repo-42-design-the-thing",
  "kind": "issue_design",
  "state": "design_running",
  "repository": "owner/repo",
  "number": 42,
  "title": "design the thing"
}
```

### 5.4 `GET /api/jobs/:id`

ジョブ詳細を返す。対象が Issue 系の場合、`issueContext` が空なら詳細取得時に補完される。`branch` が空なら branch resolver で補完する。

#### Response

```json
{
  "updatedAt": "2026-07-04T12:09:33.0740984Z",
  "job": {
    "id": "issue-102",
    "kind": "issue_implementation",
    "state": "implementation_ready",
    "subStatus": "検証(1回目)",
    "repository": "mock-owner/mock-repo",
    "number": 102,
    "title": "job-detail-panel-improvements",
    "issueContext": "#102 job-detail-panel-improvements\n\nRefine the job detail panel.",
    "fetchedAt": "2026-07-01T00:10:00Z",
    "updatedAt": "2026-07-01T03:14:05Z"
  },
  "branch": "issue_#102",
  "issueContext": "#102 job-detail-panel-improvements\n\nRefine the job detail panel.",
  "logs": [
    {
      "role": "agent",
      "roleLabel": "実装者",
      "attempt": 1,
      "files": [
        {
          "kind": "activity",
          "label": "処理ログ",
          "path": "workspace/mock-owner-mock-repo/issue-102/logs/implementation_attempt-1_agent.log",
          "content": "..."
        }
      ]
    }
  ]
}
```

### 5.5 `PATCH /api/jobs/:id/state`

ジョブ状態を更新する。状態遷移は `JobState.CanTransitionTo` に従う。

#### Request

```json
{ "state": "design_running" }
```

#### Response

更新後の `Job`。

### 5.6 `DELETE /api/jobs/:id`

ジョブを削除する。

#### Response

`204 No Content`

### 5.7 `GET /api/settings`

監視設定を返す。内部では正規化後の値を返す。

#### Response

```json
{
  "repository": "owner/repo",
  "aiProvider": "codex",
  "pollIntervalSeconds": 120,
  "jobConcurrency": 4,
  "implementationLoopCount": 3,
  "baseBranch": "main",
  "branchNamePattern": "issue_#<issue番号>",
  "aiAllowedCommands": ["npm ci"],
  "models": {
    "codex": { "mode": "default" },
    "githubCopilot": { "mode": "custom", "value": "gpt-4.1" }
  },
  "issue": {
    "enabled": true,
    "aiProvider": "codex",
    "aiModel": { "mode": "default" },
    "labelIncludes": [],
    "labelExcludes": [],
    "titleContains": [],
    "authors": [],
    "assignees": []
  },
  "pullRequest": {
    "enabled": true,
    "aiProvider": "codex",
    "aiModel": { "mode": "default" },
    "labelIncludes": [],
    "labelExcludes": [],
    "titleContains": [],
    "authors": [],
    "assignees": []
  }
}
```

### 5.8 `PUT /api/settings`

監視設定を保存する。保存時も正規化される。

### 5.9 `GET /api/jobs/:id/artifact`

成果物を返す。画面では設計結果、実装結果、レビュー結果などの本文表示に使う。

#### Response

```json
{
  "content": "# 設計結果\n...",
  "path": ".workspace/design/102_job-detail-panel-improvements.md"
}
```

### 5.10 `POST /api/jobs/:id/artifact`

成果物を承認する。コメント付きで送る。

#### Request

```json
{ "comment": "OK" }
```

#### Response

更新後の `Job`。

### 5.11 `PATCH /api/jobs/:id/artifact`

成果物を再実行する。コメント付きで送る。

#### Request

```json
{ "comment": "修正してください" }
```

#### Response

更新後の `Job`。

### 5.12 `POST /api/jobs/:id/artifact/request-changes`

PR レビューで修正依頼を出す。

#### Request

```json
{ "comment": "追加でここも修正" }
```

#### Response

更新後の `Job`。

### 5.13 `GET /api/skills`

スキルの状態一覧を返す。

#### Response

```json
{
  "skills": [
    {
      "purpose": "issue_design",
      "name": "design-from-issue",
      "displayName": "Design from Issue",
      "exists": true,
      "aiExists": true,
      "generated": false
    }
  ]
}
```

### 5.14 `POST /api/skills`

スキルを生成する。`forcePurposes` は配列、単一値、オブジェクトいずれも受け付ける。

#### Request

```json
{
  "projectContext": "Go + Vue のローカルツール",
  "testCommand": "go test ./...",
  "maxFixLoops": 3,
  "forcePurposes": ["issue_design", "issue_verification"],
  "overwriteExisting": false
}
```

#### Response

```json
{
  "provider": "codex",
  "skills": [],
  "message": "generated"
}
```

## 6. ログ仕様

`GET /api/jobs/:id` の `logs` に、ジョブごとのログをまとめて返す。

- ログは `work_dir/workspace/<repo-id>/<job-id>/logs` から読む
- `*.log` ファイルのみ対象にする
- ファイル名から `attempt` と `role` を判定する
- 同じ `role` と `attempt` は 1 グループにまとめる
- ファイルの並びは `activity` -> `stdout` -> `stderr`

ファイル名例:

- `implementation_attempt-1_agent.log`
- `implementation_attempt-1_agent_stdout.log`
- `implementation_attempt-1_agent_stderr.log`
- `implementation_attempt-1_verifier.log`
- `implementation_attempt-1_verifier_stdout.log`
- `implementation_attempt-1_verifier_stderr.log`

## 7. 補足

- 一覧のフィルタと並び替えは API ではなくフロントエンドで行う
- ログは専用 API ではなくジョブ詳細に含める
- 検証者が未指定の場合は、設定上は実装者の AI 設定を流用する
