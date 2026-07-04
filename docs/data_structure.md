# データ構造設計書

## 1. 目的

`korobokcle` が永続化するデータについて、保存先ごとのファイル名と JSON 構造を整理する。

対象は次の 3 区分とする。

- `config`
- `db`
- `state`

## 2. 保存先一覧

`work_dir` 配下の永続ファイルは次のとおり。

| 区分 | ファイル | 用途 | 形 |
| --- | --- | --- | --- |
| `config` | `config/settings.json` | 監視・実行設定 | `WatchSettings` |
| `db` | `db/jobs.json` | ジョブ一覧の永続化 | `jobStoreFile` |
| `db` | `db/mock_jobs.json` | モックモードの入力データ | `[]domain.Job` |
| `state` | `state/skill-matches.json` | スキル一致判定キャッシュ | `map[string]skillMatchRecord` |

補足:

- `config` はアプリ設定を保存する
- `db` は業務データを保存する
- `state` は再生成可能な補助情報を保存する

## 3. `config/settings.json`

### 3.1 役割

監視対象、AI 設定、実行設定、検索条件を保持する。

### 3.2 JSON 構造

`internal/domain.WatchSettings` をそのまま保存する。

```json
{
  "repository": "owner/repo",
  "aiProvider": "codex",
  "pollIntervalSeconds": 120,
  "jobConcurrency": 4,
  "implementationLoopCount": 3,
  "verificationAiProvider": "",
  "verificationAiModel": {
    "mode": "default",
    "value": ""
  },
  "baseBranch": "main",
  "branchNamePattern": "issue_#<issue番号>",
  "aiAllowedCommands": [
    "npm ci"
  ],
  "models": {
    "codex": {
      "mode": "default",
      "value": ""
    },
    "githubCopilot": {
      "mode": "default",
      "value": ""
    }
  },
  "issue": {
    "enabled": true,
    "aiProvider": "",
    "aiModel": {
      "mode": "default",
      "value": ""
    },
    "labelIncludes": [],
    "labelExcludes": [],
    "titleContains": [],
    "authors": [],
    "assignees": []
  },
  "pullRequest": {
    "enabled": true,
    "aiProvider": "",
    "aiModel": {
      "mode": "default",
      "value": ""
    },
    "labelIncludes": [],
    "labelExcludes": [],
    "titleContains": [],
    "authors": [],
    "assignees": []
  }
}
```

### 3.3 補足

- `verificationAiProvider` が空の場合は、実装者の AI 設定を流用する
- `verificationAiModel` も同様にデフォルト扱いになる
- `settings.json` は読み込み時に正規化される

## 4. `db/jobs.json`

### 4.1 役割

ジョブの一覧と最終更新時刻を保持する。

### 4.2 ファイル外形

```json
{
  "updatedAt": "2026-07-04T12:09:33.0740984Z",
  "jobs": []
}
```

### 4.3 内部型

`jobStoreFile`

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `updatedAt` | time.Time | ジョブストア全体の更新時刻 |
| `jobs` | `[]domain.Job` | ジョブ一覧 |

### 4.4 `domain.Job`

`jobs.json` の各要素は `domain.Job` の JSON と同じ。

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `id` | string | ジョブ ID |
| `kind` | string | `issue_design` など |
| `state` | string | 内部状態 |
| `subStatus` | string? | 実装ループ中の補助状態 |
| `repository` | string | 対象リポジトリ |
| `number` | number | Issue / PR 番号 |
| `title` | string | タイトル |
| `branch` | string? | 対象ブランチ |
| `aiProvider` | string? | 実行 AI プロバイダー |
| `aiModel` | string? | 実行 AI モデル |
| `issueContext` | string? | Issue 文脈 |
| `errorMessage` | string? | エラー内容 |
| `failedFromState` | string? | 失敗元状態 |
| `fetchedAt` | string? | 取得日時 |
| `updatedAt` | string? | 更新日時 |

### 4.5 `db/mock_jobs.json`

モックモードでの入力データとして使う。

- 形式は `domain.Job` の配列
- モック起動時の初期ジョブ一覧として読む
- Poller はこの内容を `jobs.json` へ反映するが、自動実行は行わない
- 画面確認用の固定データを置く

## 5. `state/skill-matches.json`

### 5.1 役割

スキル生成時の「既存スキルが同等かどうか」の判定結果を保存する。

### 5.2 ファイル外形

`map[string]skillMatchRecord` を JSON 化したもの。

```json
{
  "issue_design": {
    "aiExists": true,
    "path": ".agents/skills/design-from-issue/SKILL.md",
    "generated": false
  }
}
```

### 5.3 内部型

`skillMatchRecord`

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `aiExists` | boolean | AI 判定で同等スキルがあると判断したか |
| `path` | string? | 判定対象のスキルパス |
| `generated` | boolean? | 生成済みマーカー有無 |

### 5.4 キー

map のキーは `domain.SkillPurpose` の文字列表現を使う。

例:

- `issue_design`
- `issue_implementation`
- `issue_verification`
- `pr_review`
- `review_feedback_design`
- `review_feedback_implementation`
- `pr_conflict_resolution`

### 5.5 補足

- `state` は補助キャッシュであり、消えても再生成できる
- スキル一覧の表示と生成判定を高速化するために使う

## 6. ディレクトリ補足

実行時に作成される関連ディレクトリは次のとおり。

| 区分 | ディレクトリ | 用途 |
| --- | --- | --- |
| `config` | `work_dir/config/` | 設定保存先 |
| `db` | `work_dir/db/` | ジョブ保存先 |
| `state` | `work_dir/state/` | 補助キャッシュ保存先 |
| `workspace` | `work_dir/workspace/` | ジョブごとの作業領域 |
| `logs` | `work_dir/logs/` | ツール全体のログ |

## 7. 更新ルール

- `settings.json` は設定保存時に更新する
- `jobs.json` はジョブの追加・更新・削除時に更新する
- `skill-matches.json` はスキル生成やスキル状態確認時に更新する
- `state` 配下の内容は再作成可能な前提とする
