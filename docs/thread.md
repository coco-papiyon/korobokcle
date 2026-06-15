# Thread Design

`korobokcle` の並行実行構造を、実際の goroutine 構成に沿って整理した設計書です。

Go ランタイム上では OS thread そのものではなく goroutine を直接設計対象とするため、本書では「thread」という名前を使いつつ、実体は goroutine とその役割を説明します。

## 1. 目的

この文書の目的は次の 3 点です。

- どの goroutine が常駐し続けるかを明確にする
- どの goroutine が job 処理を進めるかを明確にする
- Web 経由の非同期処理がどこで追加起動されるかを明確にする

## 2. 全体像

`Run()` の開始後、主な並行実行は次のように構成されます。

### 2.1 一覧表

| Thread名 | 日本語名 | 概要 | 起動単位 | 数 |
| --- | --- | --- | --- | --- |
| main goroutine | メイン制御 | 初期化、常駐 goroutine 起動、終了待ちを行う中心制御 | プロセスごと | 1 |
| watcher producer goroutine | 監視取得 | GitHub Poller を回し、一致 event を channel へ流す | プロセスごと | 1 |
| watcher consumer goroutine | 監視反映 | event channel を読み、Orchestrator に job 作成・更新を依頼する | プロセスごと | 1 |
| job worker manager goroutine | job worker 管理 | job 一覧をポーリングし、必要な job 専用 worker を起動する | プロセスごと | 1 |
| implementation job worker goroutine | 実装 worker | issue / PR feedback ごとに起動し、design / implementation / PR 完了までを同一 worktree で処理する | job ごと | 起動数は job 数、同時動作は `implementationWorkers` 合計まで |
| review job worker goroutine | PR レビュー worker | `pr_review` job ごとに起動し、レビュー承認までを同一 worktree で処理する | job ごと | 起動数は job 数、同時動作は `reviewWorkers` 合計まで |
| AI session stdout reader goroutine | AI 標準出力読取 | resident AI CLI の stdout を読み、worker session に流す | resident AI session ごと | worker ごとに最大 1 |
| AI session stderr reader goroutine | AI 標準エラー読取 | resident AI CLI の stderr を読み、worker session に流す。PTY では生成されないことがある | resident AI session ごと | worker ごとに 0 または 1 |
| AI session wait goroutine | AI 終了待機 | resident AI CLI 子プロセスの `Wait()` を受け持ち、終了を検知する | resident AI session ごと | worker ごとに最大 1 |
| web server goroutine | Web サーバ | HTTP サーバの accept loop を持ち、API / SPA を受け付ける | プロセスごと | 1 |
| HTTP request goroutine | リクエスト処理 | 各 API リクエストを処理する。`net/http` が request 単位で起動する | リクエストごと | 同時リクエスト数に比例 |
| PR comment analysis goroutine | PR コメント分析 | PR コメント分析を非同期実行し、HTTP 応答と切り離す | 分析要求ごと | 同時分析数に比例 |
| improvement generation goroutine | 改善案生成 | rerun / approval 契機で改善案生成をバックグラウンド実行する | 生成要求ごと | 同時生成数に比例 |
| resident tool wait goroutine | 常駐ツール待機 | 常駐ツール子プロセスの `Wait()`、終了検知、メタ更新を行う | ツール実行ごと | 同時 resident tool 数に比例 |

```text
main goroutine
  ├─ watcher producer goroutine
  ├─ watcher consumer goroutine
  ├─ job worker manager goroutine
  ├─ web server goroutine
  └─ request-scoped / one-shot goroutines
       ├─ implementation job worker
       │    └─ resident AI session goroutines
       │         ├─ AI session stdout reader
       │         ├─ AI session stderr reader
       │         └─ AI session wait
       ├─ review job worker
       │    └─ resident AI session goroutines
       │         ├─ AI session stdout reader
       │         ├─ AI session stderr reader
       │         └─ AI session wait
       ├─ PR comment analysis
       ├─ improvement generation
       └─ resident tool wait loop
```

起動点:

- [internal/app/bootstrap.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/bootstrap.go#L18)

## 3. 起動シーケンスと goroutine 増加点

`Run()` 自体は main goroutine 上で順に初期化を進めます。

1. 設定読込
2. DB 初期化
3. Orchestrator 構築
4. Watcher 起動
5. repository workspace 準備
6. job worker manager 起動
7. Web server 起動
8. `ctx.Done()` または server error を待つ

この中で新しい goroutine が増えるのは次の 3 箇所です。

- `startWatcher()`
- `startRepositoryWorkers()`
- `server.Start()` を包む `go func()`

参照:

- [internal/app/bootstrap.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/bootstrap.go#L59)

## 4. main goroutine の責務

main goroutine は coordinator です。自分ではポーリングループを持たず、常駐 goroutine を立ち上げた後は待機します。

責務:

- 初期化の順序制御
- 依存オブジェクトの生成
- Web server の終了待ち
- shutdown 時の context 伝播

終了条件:

- 親 `context.Context` が cancel される
- Web server goroutine が異常終了して `errCh` にエラーを返す

## 5. Watcher 系 goroutine

Watcher 系は 2 本の常駐 goroutine で構成されます。

### 5.1 watcher producer goroutine

役割:

- GitHub Poller を回す
- 一致した event を `events chan domain.DomainEvent` に流す

特徴:

- `watcher.Start(ctx, events)` を実行
- 停止時に `events` channel を close する

参照:

- [internal/app/watcher.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/watcher.go#L25)

### 5.2 watcher consumer goroutine

役割:

- `events` channel を読む
- watch rule を引き直す
- `orch.ProcessMatch()` を呼ぶ

特徴:

- event channel を単一 consumer で処理する
- ここでは job 実行をしない
- ここで行うのは job 作成や state/event 反映まで

参照:

- [internal/app/watcher.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/watcher.go#L32)

### 5.3 Watcher 系の設計意図

- GitHub 監視と job 化を分離する
- `watcher.Start()` の I/O 待ちと `orch.ProcessMatch()` の処理待ちを分ける
- producer が止まっても channel close で consumer を自然終了させる

## 6. job worker pool goroutine

`startRepositoryWorkers()` が、リポジトリごとに 2 つの worker pool を起動します。

### 6.1 役割

役割:

- `orch.ListJobs()` を 5 秒周期で監視する
- `issue` / `pr_feedback` は実装 worker pool に流す
- `pr_review` は PR レビュー worker pool に流す
- 同じ job を複数 worker が同時に処理しないように、job の worker への割り当てを固定する

参照:

- [internal/app/repository_workers.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/repository_workers.go#L25)

### 6.2 worker 種別

worker pool は 2 系統です。

1. 実装 worker
2. PR レビュー worker

分類:

- `issue`
- `pr_feedback`
  - 実装 worker
- `pr_review`
  - PR レビュー worker

### 6.3 同時実行数の制御

各 worker pool の同時実行数は repository 設定で分けて管理します。

- 実装 worker の同時実行上限
  - `monitoredRepositories[].implementationWorkers`
- PR レビュー worker の同時実行上限
  - `monitoredRepositories[].reviewWorkers`

重要:

- 実装系とレビュー系は別々に上限を持つ
- worker の起動数と、AI 実行の同時数は同じではない
- 実際の AI 実行は job 単位の worktree と session を使い回す

## 7. 実装 worker goroutine

### 7.1 起動条件

実装 worker は次の job に対して起動されます。

- `issue`
- `pr_feedback`

ただし、`completed` の job には起動しません。

### 7.2 ライフサイクル

1. job を取得する
2. repository 設定を解決する
3. base clone を準備する
4. 必要なら improvement workspace を準備する
5. `artifact/source/<repo>-<branch>` の worktree を作成する
6. job の `session.json` があれば読み込み、AI 実行に渡す
7. resident AI CLI を起動し、startup prompt を 1 回だけ送る
8. job state を監視しながら次の phase を順に処理する
   - design
   - implementation
   - PR
9. AI 実行が返した session ID を `session.json` に保存する
10. job が `completed` になったら停止する
11. 停止時に resident AI CLI を終了する
12. 停止時に作業用 worktree を削除する

参照:

- [internal/app/repository_workers.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/repository_workers.go#L25)

### 7.3 worktree の使い方

実装 worker は job ごとに専用 worktree を持ちます。

- パス
  - `tool_root/source/<repo>-<branch>`
- branch
  - `job.BranchName`

初期 checkout:

- `issue`
  - base branch から `job.BranchName` を作って開始
- `pr_feedback`
  - PR branch をそのまま checkout

これにより、design から PR まで同一 worktree を継続利用します。

### 7.4 session の使い方

各 job は session ID を 1 つ持ちます。

- 保存先
  - `artifacts/jobs/<job-id>/session.json`
- 読み込みタイミング
  - design / implementation / PR の各 phase 開始時
- 更新タイミング
  - AI 実行の完了後
- 再開条件
  - セッション ID が残っていれば、その ID を provider に渡す

現状の session 対象 provider:

- `codex`
- `copilot`

### 7.5 実行 phase

| Job State | Job Type | 実行 phase |
| --- | --- | --- |
| `detected` | `issue` | design |
| `implementation_running` | `issue`, `pr_feedback` | implementation / fix / test |
| `pr_creating` | `issue`, `pr_feedback` | PR create / push / update |

## 8. PR レビュー worker goroutine

### 8.1 起動条件

PR レビュー worker は次の job に対して起動されます。

- `pr_review`

ただし、`completed` の job には起動しません。

### 8.2 ライフサイクル

1. job を取得する
2. repository 設定を解決する
3. base clone を準備する
4. 必要なら improvement workspace を準備する
5. `artifact/source/<repo>-<branch>` の worktree を作成する
6. job の `session.json` があれば読み込み、AI 実行に渡す
7. `collecting_context` 中に review phase を実行する
8. AI 実行が返した session ID を `session.json` に保存する
9. 承認後に `completed` へ遷移したら停止する
10. 停止時に resident AI CLI を終了する
11. 停止時に作業用 worktree を削除する

### 8.3 worktree の使い方

PR レビュー worker も job ごとに専用 worktree を持ちます。

- パス
  - `tool_root/source/<repo>-<branch>`
- branch
  - 原則 `job.BranchName`

`pr_review` job は、PR event の `branchName` を優先し、無ければ repository 設定の branch を使います。

### 8.4 実行 phase

| Job State | Job Type | 実行 phase |
| --- | --- | --- |
| `collecting_context` | `pr_review` | review |

### 8.5 session

PR レビュー worker も job ごとに session ID を持ちます。

- review 再実行時は同じ session ID を使う
- worker 停止後も `session.json` が残る
- provider には `thread.started` / `session-id` 由来の ID を渡す

参照:

- [internal/app/repository_workers.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/repository_workers.go#L25)
- [internal/skill/resident_provider.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/skill/resident_provider.go#L93)

## 9. job 専用 worker の直列性

1 本の job worker は、自分が担当する 1 job だけを処理します。

したがって直列性は:

- 実装 worker
  - 1 job 内で `design -> implementation -> pr`
- PR レビュー worker
  - 1 job 内で `review`

です。

従来の repository 常駐 worker のように、1 本の worker が複数 job を順に処理する構造ではありません。

## 10. Web server goroutine

`Run()` は `server.Start()` を直接呼ばず、別 goroutine で起動します。

役割:

- HTTP accept loop を保持する
- リクエストごとの handler 実行を `net/http` に委ねる

起動点:

- [internal/app/bootstrap.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/bootstrap.go#L151)

補足:

- 各 HTTP request handler 自体は `net/http` 側が別 goroutine で処理する
- つまり API handler は request 単位で自然に並列化される

## 11. request-scoped goroutine

常駐ではなく、イベントや API 呼び出しを契機に単発で起動される goroutine があります。

### 11.1 PR comment analysis goroutine

PR コメント分析要求時、handler は job state を更新した後、別 goroutine で分析本体を流します。

目的:

- API 応答を先に返す
- 長時間の AI 実行を HTTP リクエストにぶら下げない

起動点:

- [internal/app/bootstrap.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/app/bootstrap.go#L98)

### 11.2 improvement generation goroutine

一部の rerun / approval 操作では、handler 側が改善案生成をバックグラウンドで起動します。

起動点:

- [internal/web/handlers.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/web/handlers.go#L679)

特徴:

- `context.Background()` を使うため、元の HTTP request cancel には引きずられない
- その分、明示停止はアプリ全体停止に依存する

### 11.3 resident tool wait goroutine

常駐ツール起動では、`cmd.Start()` 後に `cmd.Wait()` 専用 goroutine を立てます。

役割:

- プロセス終了待ち
- stdout/stderr file close
- 実行状態メタデータ更新

起動点:

- [internal/web/tool_runtime.go](/home/coco/dev/go/src/github.com/coco-papiyon/korobokcle/internal/web/tool_runtime.go#L140)

## 12. goroutine 間の主な接続

### 12.1 Watcher -> Orchestrator

接続手段:

- `chan domain.DomainEvent`

意味:

- 検知結果を job 作成系処理へ渡す

### 12.2 job worker -> Orchestrator

接続手段:

- メソッド呼び出し

意味:

- `ListJobs`
- `JobDetail`
- `UpdateJobState`

job worker は state machine を直接持たず、Orchestrator を経由して状態を進めます。

### 12.3 Web -> Orchestrator

接続手段:

- HTTP handler からのメソッド呼び出し

意味:

- 承認
- rerun
- review submit
- delete / restore / purge

### 10.4 Web -> background goroutine

接続手段:

- `go func()`

意味:

- API を同期 blocking させず、長時間処理を切り離す

## 13. 停止と終了

停止の基準は `context.Context` です。

### 11.1 常駐 goroutine

対象:

- watcher producer
- watcher consumer
- job worker manager
- web server

停止方法:

- 親 context cancel
- ticker loop の `ctx.Done()`
- watcher 側の channel close

### 11.2 非同期 goroutine

対象:

- PR comment analysis
- improvement generation
- resident tool wait
- implementation job worker
- review job worker

補足:

- `process.Wait()` 系は対象プロセスが終わるまで残る
- `context.Background()` 起動のものは request cancel と独立して生きる

## 14. 現在の設計上の特徴

### 12.1 強い点

- job 実行主系が job worker manager に集約されている
- job state を介して phase を疎結合に切り替えられる
- repository 単位かつ worker 種別ごとに同時実行数を制御できる
- HTTP request と長時間 AI 処理を分離できている

### 12.2 注意点

- job worker manager は polling ベースで、event-driven queue ではない
- job worker は job ごとに起動するため、待機中 goroutine 数は job 数に比例して増える
- `context.Background()` 起動の単発 goroutine は request 単位では止まらない
- 旧 `design_worker.go` などの単体 worker 実装が残っており、現行主系と名称が衝突しやすい

## 15. 旧 worker 実装との関係

以下のファイルにも goroutine ベースの worker ループ実装があります。

- `design_worker.go`
- `implementation_worker.go`
- `review_worker.go`
- `pr_worker.go`

これらも 5 秒 ticker で job を走査する構成ですが、現在の `Run()` からは起動されていません。

したがって現行設計として読むべき主系は:

- watcher 2 本
- job worker manager 1 本
- implementation / review の job worker 群
- web server 1 本
- request-scoped background goroutine 群

です。
