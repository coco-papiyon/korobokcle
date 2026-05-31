# Directory Layout
## Runtime Paths

`korobokcle` が実行時に参照する主なパスと、その基準・上書き可否は以下です。

`tool root` とは:

- `KOROBOKCLE_TOOL_ROOT` が設定されていればその値
- 未設定なら、実行ファイル配置ディレクトリまたはカレントディレクトリのうち、`config/app.yaml` もしくは `skills/default/design/skill.yaml` を持つ方

| 対象 | 既定パス | 用途 | 相対基準 | 上書き方法 |
| --- | --- | --- | --- | --- |
| `tool root` | `.` | 設定や成果物の基準となるルート | カレントディレクトリ | `KOROBOKCLE_TOOL_ROOT` でディレクトリ変更 |
| 各種設定 | `config/*.yaml` | アプリに関する各種設定 | `tool root` |  |
| SQLite DB | `data/korobokcle.db` | 永続データ保存先 | `tool root` | `config/app.yaml` の `sqlitePath` |
| データディレクトリ | `data/` | DB や内部データの保存先 | `tool root` | `config/app.yaml` の `dataDir` |
| 成果物ディレクトリ | `artifacts/` | ジョブやワーカーの成果物保存先 | `tool root` | `config/app.yaml` の `artifactsDir` |
| 作業ディレクトリ | `artifacts/workers/**リポジトリ**/work` | repository worker の clone 元。人間が直接開いて `.workspace` 配下の結果ファイルを修正する場所 | 成果物ディレクトリ | `config/app.yaml` の各 `monitoredRepositories[].workDir` |
| 結果格納ディレクトリ | `artifacts/workers/**リポジトリ**/work/.workspace/{design,implementation...}` | repository worker の設計・実装・PR などの出力先 | 作業ディレクトリ |  |
| ワーカー作業ディレクトリ | `artifacts/workers/**リポジトリ**/worker-**ワーカーID**/source` | repository worker が実際に作業する clone | 成果物ディレクトリ |  |
| スキル定義 | `skills/<set>/<name>/` | 実行するスキルの定義 | `tool root` |  |
| Web 静的ファイル | `frontend/dist/` | Web UI の配信元 | 実行ファイル配置ディレクトリ | 変更不可 |

補足:

- `dataDir` と `artifactsDir` は、相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `sqlitePath` も相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `repository worker source root` は `artifactsDir` 配下の `artifacts/<repo>/worker-<id>/` です。
- worker 設定画面の `workerDirs` を指定すると、各ワーカーごとに指定したディレクトリを基準に `worker-<id>` が自動付与されます。
- `workspaceDir` は repository worker の source root 基準で解決します。既定値は `.workspace` です。ジョブごとの成果物は `.workspace/issue_<issue番号>/` の下に置きます。
- `frontend/dist/` は常に実行ファイル配置ディレクトリ直下を参照します。`KOROBOKCLE_TOOL_ROOT` では変わりません。
- `frontend/dist/index.html` が無い場合、Web UI は SPA を返せず `503` になります。
- 作業ディレクトリにはリポジトリのソースを clone し、結果ファイルは `.workspace` 配下に出力します。clone 済みの場合は再 clone しません。
- ワーカー作業ディレクトリは、作業ディレクトリから作成した別 clone です。worker はこのディレクトリを使ってソースコードを修正します。
- 作業ディレクトリは人間が vscode などで開いて、`.workspace` 配下の設計結果等を直接修正するためのディレクトリとして使用します。
