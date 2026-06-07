# Directory Layout
## Runtime Paths

`korobokcle` が実行時に参照する主なパスと、その基準・上書き可否は以下です。

`tool root` とは:

- `KOROBOKCLE_TOOL_ROOT` が設定されていればその値
- 未設定なら、実行ファイル配置ディレクトリまたはカレントディレクトリのうち、`config/app.yaml` もしくは `skills/default/design/skill.yaml` を持つ方

| 対象 | 既定パス | 用途 | 相対基準 | 上書き方法 |
| --- | --- | --- | --- | --- |
| `tool root` | `.` | 設定や成果物の基準となるルート | カレントディレクトリ | `KOROBOKCLE_TOOL_ROOT` でディレクトリ変更 |
| 設定ディレクトリ | `config/` | アプリに関する各種設定の保存先 | `tool root` |  |
| データディレクトリ | `data/` | DB や内部データの保存先 | `tool root` | `config/app.yaml` の `dataDir` |
| 成果物ディレクトリ | `artifacts/` | ジョブやワーカーの成果物保存先 | `tool root` | `config/app.yaml` の `artifactsDir` |
| 作業ディレクトリ | `**成果物ディレクトリ**/workers/**リポジトリ**/work` | repository worker の checkout 元。ソースコードのみを置く場所 | 成果物ディレクトリ | `config/app.yaml` の各 `monitoredRepositories[].workDir` |
| 結果格納ディレクトリ | `**成果物ディレクトリ**/workers/**リポジトリ**/jobs/issue_**番号**` | repository worker の設計・実装・PR などの出力先 | 成果物ディレクトリ |  |
| ワーカー作業ディレクトリ | `**成果物ディレクトリ**/workers/**リポジトリ**/workers/worker-**ワーカーID**` | repository worker が実際に作業する clone | 成果物ディレクトリ |  |
| ワーカーソースディレクトリ | `**成果物ディレクトリ**/workers/**リポジトリ**/workers/worker-**ワーカーID**/source` | repository worker が実際に作業する clone | 成果物ディレクトリ |  |
| スキル定義 | `skills/<set>/<name>/` | 実行するスキルの定義 | `tool root` |  |
| Web 静的ファイル | `frontend/dist/` | Web UI の配信元 | 実行ファイル配置ディレクトリ | 変更不可 |

## File Layout

ディレクトリではなく、個別ファイルとして参照される主要なものです。

| 対象 | 既定ディレクトリ | ファイル名 | 用途 | 相対基準 |
| --- | --- | --- | --- | --- |
| 各種設定ファイル | `設定ディレクトリ` | `*.yaml` | アプリ設定、watch rules、notifications、test profiles、tool commands の定義 | `tool root` |
| SQLite DB | `データディレクトリ` | `korobokcle.db` | 永続データ保存先 | `tool root` |
| AI成果物 | `結果格納ディレクトリ/{design,implementation,fix,review}/` | `*.md` | 各フェーズの要約、設計書、修正内容などの本文成果物 | 結果格納ディレクトリ |
| AI成果物(作業用) | `作業ディレクトリ/{design,implementation,fix,review}/` | `**issue番号**_**issueタイトル**.md` | 各フェーズの要約、設計書、修正内容などの本文成果物 | 作業ディレクトリ |
| 改善入力・下書き | `作業ディレクトリ/.improvement/` | `input.md`, `context.json`, `**issue番号**_**タイトル**.md` | 改善生成の入力、関連 job 情報、承認前 draft | 作業ディレクトリ |
| 承認済み改善方針 | `作業ディレクトリ/.improvements/` | `*.md` | front matter 付きの承認済み改善方針 | 作業ディレクトリ |
| AIログ | `結果格納ディレクトリ/{design,implementation,fix,review}/` | `*.log` | 各フェーズの標準出力・標準エラー | 結果格納ディレクトリ |
| テストレポート | `結果格納ディレクトリ/{implementation,fix}/` | `test-report.json` | 実装・修正フェーズのテスト結果 | 結果格納ディレクトリ |
| PR 生成結果 | `結果格納ディレクトリ/pr/` | `result.json` | PR URL、PR 番号、ブランチ名などの保存 | 結果格納ディレクトリ |
| PR 関連ログ・補助ファイル | `結果格納ディレクトリ/pr/` | `body.md`, `gh-pr-comments.json`, `gh-pr-comment-body.md`, `gh-pr-comment.log`, `gh-pr-create.log`, `git-*.log` | PR 本文、会話コメント、`git push` / `gh pr create` / `gh pr comment` のログ | 結果格納ディレクトリ |
| 改善成果物 | `結果格納ディレクトリ/improvement/` | `input.md`, `context.json`, `draft.md`, `result.md`, `approval.json`, `decision.json`, `git-*.log` | 改善生成の入力、承認前 draft、承認結果、改善ブランチ反映ログ | 結果格納ディレクトリ |
| worker ログ | `ワーカー作業ディレクトリ/logs/**日付**/` | `*.log` | ワーカーごとの実行ログ | ワーカー作業ディレクトリ |
| Web entrypoint | `frontend/dist/` | `index.html` | Web UI の SPA エントリ | 実行ファイル配置ディレクトリ |

補足:

- `dataDir` と `artifactsDir` は、相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `sqlitePath` も相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `作業ディレクトリ` は `artifactsDir` 配下の `artifacts/workers/<repo>/work` です。
- worker 設定画面の `workDir` を指定すると、各リポジトリごとに指定したディレクトリを基準に `worker-<id>` が自動付与されます。
- `workspaceDir` は作業ディレクトリ内で削除対象にするディレクトリ名です。既定値は `.workspace` ですが、ジョブごとの成果物は置きません。
- `frontend/dist/` は常に実行ファイル配置ディレクトリ直下を参照します。`KOROBOKCLE_TOOL_ROOT` では変わりません。
- `frontend/dist/index.html` が無い場合、Web UI は SPA を返せず `503` になります。
- 作業ディレクトリにはリポジトリのソースを clone します。clone 済みの場合は再 clone しません。
- ワーカー作業ディレクトリは `artifactsDir` 配下の `artifacts/workers/<repo>/worker-<id>` です。worker はこのディレクトリを使ってソースコードを修正します。
- 作業ディレクトリには成果物を置きません。成果物は `結果格納ディレクトリ` 配下に出力します。
