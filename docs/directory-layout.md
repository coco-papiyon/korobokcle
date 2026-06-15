# Directory Layout
## Runtime Paths

`korobokcle` が実行時に参照する主なパスと、その基準・上書き可否は以下です。

`tool root` とは:

- `KOROBOKCLE_TOOL_ROOT` が設定されていればその値
- 未設定なら、実行ファイル配置ディレクトリまたはカレントディレクトリのうち、`config/app.yaml` もしくは `skills/default/design/skill.yaml` を持つ方

| 対象 | 既定パス | 用途 | 相対基準 | 上書き方法 |
| --- | --- | --- | --- | --- |
| `tool_root` | `.` | 設定や成果物の基準となるルート | カレントディレクトリ | `KOROBOKCLE_TOOL_ROOT` でディレクトリ変更 |
| 設定ディレクトリ | `tool_root/config/` | アプリに関する各種設定の保存先 | `tool root` |  |
| データディレクトリ | `tool_root/data/` | DB や内部データの保存先 | `tool root` | `config/app.yaml` の `dataDir` |
| 成果物ディレクトリ | `tool_root/artifacts/` | ジョブやワーカーの成果物保存先 | `tool root` | `config/app.yaml` の `artifactsDir` |
| 結果格納ディレクトリ | `tool_root/artifact/**リポジトリ**/jobs/issue_**番号**` | repository worker の設計・実装・PR などの出力先 | 成果物ディレクトリ |  |
| ソースディレクトリ | `tool_root/source/` | repository worker の base clone と worktree を置くルート | `tool root` |  |
| Base clone | `tool_root/source/**リポジトリ**` | repository worker の共有元 clone。ここから branch worktree を切る | ソースディレクトリ | `config/app.yaml` の各 `monitoredRepositories[].workDir` |
| 作業用 worktree | `tool_root/source/**リポジトリ**-**ブランチ名**` | repository worker が実際に作業するディレクトリ。設定されたブランチ名ごとに worktree を作る | Base clone |  |
| 改善ブランチ worktree | `tool_root/source/**リポジトリ**-**ブランチ名**` | 改善指示用ブランチを worktree と同様に扱う作業場所。ブランチ名の既定値は `improvement` | Base clone | repository ごとの改善設定でブランチ名を変更 |
| スキル定義 | `skills/<set>/<name>/` | 実行するスキルの定義 | `tool root` |  |
| Web 静的ファイル | `frontend/dist/` | Web UI の配信元 | 実行ファイル配置ディレクトリ | 変更不可 |

**リポジトリ** は **リポジトリオーナー**-**リポジトリ名** とする
例：coco-papiyon/korobokcleの場合、coco-papiyon-korobokcle

## File Layout

ディレクトリではなく、個別ファイルとして参照される主要なものです。

| 対象 | 既定ディレクトリ | ファイル名 | 用途 | 相対基準 |
| --- | --- | --- | --- | --- |
| 各種設定ファイル | `設定ディレクトリ` | `*.yaml` | アプリ設定、watch rules、notifications、test profiles、tool commands の定義 | `tool root` |
| SQLite DB | `データディレクトリ` | `korobokcle.db` | 永続データ保存先 | `tool root` |
| AI成果物 | `結果格納ディレクトリ/{design,implementation,fix,review}/` | `*.md` | 各フェーズの要約、設計書、修正内容などの本文成果物 | 結果格納ディレクトリ |
| AI成果物(作業用) | `作業ディレクトリ/{design,implementation,fix,review}/` | `**issue番号**_**issueタイトル**.md` | 各フェーズの要約、設計書、修正内容などの本文成果物 | 作業ディレクトリ |
| 改善案ドラフト | `作業ディレクトリ/.improvement/` | `draft/*.md` | 承認前の改善案編集用ファイル | 作業ディレクトリ |
| 承認済み改善指示 | `作業ディレクトリ/.improvement/` | `*.md` | front matter 付き Markdown の正本 | 作業ディレクトリ |
| 改善監査成果物 | `結果格納ディレクトリ/jobs/**issue番号**/improvement/` | `input.md`, `context.json`, `notes.md`, `result.md`, `approval.json`, `decision.json`, `implementation-prompt.md`, `*.log` | 改善案生成と承認の監査記録 | 結果格納ディレクトリ |
| AIログ | `結果格納ディレクトリ/{design,implementation,fix,review}/` | `*.log` | 各フェーズの標準出力・標準エラー | 結果格納ディレクトリ |
| テストレポート | `結果格納ディレクトリ/{implementation,fix}/` | `test-report.json` | 実装・修正フェーズのテスト結果 | 結果格納ディレクトリ |
| PR 生成結果 | `結果格納ディレクトリ/pr/` | `result.json` | PR URL、PR 番号、ブランチ名などの保存 | 結果格納ディレクトリ |
| PR 関連ログ・補助ファイル | `結果格納ディレクトリ/pr/` | `body.md`, `gh-pr-comments.json`, `gh-pr-comment-body.md`, `gh-pr-comment.log`, `gh-pr-create.log`, `git-*.log` | PR 本文、会話コメント、`git push` / `gh pr create` / `gh pr comment` のログ | 結果格納ディレクトリ |
| worker ログ | `成果物ディレクトリ/**リポジトリ**/logs/**日付**/` | `worker-**番号**-**日時**.log` | ワーカーごとの実行ログ | 成果物ディレクトリ |
| Web entrypoint | `frontend/dist/` | `index.html` | Web UI の SPA エントリ | 実行ファイル配置ディレクトリ |

補足:

- `dataDir` と `artifactsDir` は、相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `sqlitePath` も相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `Base clone` は `tool_root/source/<repo>` です。`workDir` を空にするとこのパスが使われます。
- `作業用 worktree` は `tool_root/source/<repo>-<branch>` です。設定されたブランチ名で `git worktree add` して、設計・実装・レビューの各フェーズで使います。
- 改善機能を有効にした repository では、`改善ブランチ worktree` を `tool_root/source/<repo>-improvement` のように作成し、その配下に承認前の `.improvement/` を置きます。改善の監査ファイルは `jobs/.../improvement/` に分けます。
- 改善指示を保持する Git ブランチは repository ごとに 1 本とし、既定名は `improvement` です。
- `.improvement/` は改善指示の正本で、provider へ渡す前の中立な保存形式です。
- `.improvement/` は承認前の編集領域で、`draft/*.md` のみを置きます。
- worker 設定画面の `workDir` を指定すると、各リポジトリごとに指定した base clone を使います。branch worktree はその配下に作成します。
- `workspaceDir` は作業ディレクトリ内で削除対象にするディレクトリ名です。既定値は `.workspace` ですが、ジョブごとの成果物は置きません。
- `frontend/dist/` は常に実行ファイル配置ディレクトリ直下を参照します。`KOROBOKCLE_TOOL_ROOT` では変わりません。
- `frontend/dist/index.html` が無い場合、Web UI は SPA を返せず `503` になります。
- 作業用 worktree にはリポジトリのソースを `git worktree add` で取得します。既存の worktree があれば再利用し、なければ作成します。
- worker はこの worktree ディレクトリを使ってソースコードを修正します。
- PR が承認されたら、該当する worktree ディレクトリは削除します。
- 作業ディレクトリには成果物を置きません。成果物は `結果格納ディレクトリ` 配下に出力します。
