# Directory Layout
## Runtime Paths

`korobokcle` が実行時に参照する主なパスと、その基準・上書き可否は以下です。

`tool root` とは:

- `KOROBOKCLE_TOOL_ROOT` が設定されていればその値
- 未設定なら、実行ファイル配置ディレクトリまたはカレントディレクトリのうち、`config/app.yaml` もしくは `skills/default/design/skill.yaml` を持つ方

| 対象 | 既定パス | 相対基準 | 上書き方法 |
| --- | --- | --- | --- |
| アプリ設定 | `config/app.yaml` | `tool root` | `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |
| Watch Rule 設定 | `config/watch-rules.yaml` | `tool root` | `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |
| 通知設定 | `config/notifications.yaml` | `tool root` | `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |
| テスト設定 | `config/test-profiles.yaml` | `tool root` | `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |
| SQLite DB | `data/korobokcle.db` | `tool root` | `config/app.yaml` の `sqlitePath`、または `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |
| データディレクトリ | `data/` | `tool root` | `config/app.yaml` の `dataDir`、または `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |
| 成果物ディレクトリ | `artifacts/` | `tool root` | `config/app.yaml` の `artifactsDir`、または `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |
| Web 静的ファイル | `frontend/dist/` | 実行ファイル配置ディレクトリ | 変更不可 |
| スキル定義 | `skills/<set>/<name>/` | `tool root` | `KOROBOKCLE_TOOL_ROOT` で基準ディレクトリ変更 |

補足:

- `dataDir` と `artifactsDir` は、相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `sqlitePath` も相対パスなら `tool root` 基準、絶対パスならそのまま使います。
- `frontend/dist/` は常に実行ファイル配置ディレクトリ直下を参照します。`KOROBOKCLE_TOOL_ROOT` では変わりません。
- `frontend/dist/index.html` が無い場合、Web UI は SPA を返せず `503` になります。
