# DEV-PLAN-471 Readiness Evidence

- 时间：2026-04-26T05:19:57Z
- 范围：CubeBox 同一 turn 内模型驱动迭代式只读规划 P0 实施
- 关联计划：`docs/dev-plans/471-cubebox-intra-turn-iterative-read-planning-plan.md`

## 自动化验证

已执行：

```bash
go fmt ./...
go vet ./...
make check lint
make test
make check error-message
make check doc
make check root-surface
make generate && make css
go test ./modules/cubebox ./internal/server ./internal/routing
make preflight
```

结果：

```text
go fmt ./... OK
go vet ./... OK
make check lint OK
make test OK
make check error-message OK
make check doc OK
make check root-surface OK
make generate && make css OK
ok  	github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox
ok  	github.com/jacksonlee411/Bugs-And-Blossoms/internal/server
ok  	github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing
make preflight OK (2026-04-26T06:30:55Z)
```

备注：

- `make test` 仍输出既有覆盖率 gate paused：`total 87.80% (configured threshold 98.00%)`，未阻断。
- `make generate && make css` 输出 Vite chunk size warning，构建通过。
- `make preflight` 覆盖 root-surface、no-legacy、chat-surface-clean、no-scope-package、granularity、DDD layering、request-code、as-of-explicit、dict-tenant-only、go-version、error-message、doc、fmt、lint、UI build、Go test、routing、E2E 与最终 root-surface；E2E 5 passed。

覆盖要点：

- planner outcome 解析覆盖 JSON envelope `READ_PLAN` / `CLARIFY` / `DONE` / `NO_QUERY`，兼容裸 `ReadPlan` / `NO_QUERY`，拒绝裸 `DONE`。
- `working_results` 覆盖 observation ledger、预算、fingerprint、重复检测与业务无关字段约束。
- query flow 覆盖 `READ_PLAN -> DONE`、`READ_PLAN -> READ_PLAN -> DONE`、已执行后 `NO_QUERY` fail-closed、`DONE` 无执行结果 fail-closed、预算耗尽 fail-closed、重复 fingerprint fail-closed。
- narrator 只在合法 `DONE` 后调用一次；预算耗尽和重复计划不输出 partial answer。
- `working_results` 只作为当前 turn 内 planner 输入，未写入 canonical events。
- pre-execution `NO_QUERY` 覆盖受控 facts、默认名称/关键词/关系型示例、有最近确认实体时的“这个组织 / 它”续问示例，以及禁止泄露 `NO_QUERY` / `ReadPlan` / `planner` / `API` 等内部术语。

## 真实页面验证

已执行：

```bash
tools/codex/skills/bugs-and-blossoms-dev-login/scripts/stop_dev_runtime.sh
tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh --verify-cubebox
```

结果：

```text
[dev-runtime] login OK: admin@localhost
[dev-runtime] configure CubeBox provider: openai-compatible / gpt-5.2
[dev-runtime] verify CubeBox active model
[dev-runtime] ready: http://localhost:8080/app
```

页面复验：

- 时间：2026-04-26T06:00:03Z
- 入口：`http://localhost:8080/app`
- 会话：新建 CubeBox 对话 `conv_409b6f28f5d6440383cc9b294c474777`
- 输入：`你好`
- 返回文本：

```text
当前主要支持组织相关只读查询。

你可以直接这样问：
1. 查“华东销售中心”的详情
2. 查“华东销售中心”当前的下级组织
3. 搜索名称包含“销售”的组织
```

验证结论：

- 页面未回退到通用 gateway fallback。
- `NO_QUERY` 用户可见输出为普通文本流，无卡片组件。
- DOM `innerText` 保留空行与 `1. 2. 3.` 序号列表换行。
- 输出未展示编码型默认示例，未泄露 `NO_QUERY`、`ReadPlan`、`planner`、知识包或 `API`。

本地环境备注：

- runtime 启动时提示 `.env` 与 `.env.local` 均存在 `OPENAI_API_KEY`；未触碰密钥文件。
- 首次复验发现 8080 被旧 `go run` 进程占用；确认该进程 cwd 为本仓库后停止，再重新启动 runtime 并完成上述验证。
