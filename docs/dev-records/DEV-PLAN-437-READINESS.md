# DEV-PLAN-437 Readiness

## 目标

- 作为 `DEV-PLAN-437` 的执行证据入口，记录 CubeBox 快速开工路线图的阶段状态、命中的 owner 计划、执行命令与可审计证据。
- 将“快速开工”收敛为一组可验证阶段，而不是口头上的优先级建议。
- 首轮先固化 `Phase A / PR-437A` 的 readiness 清单，为后续阶段预留统一记录结构。

## 当前状态

- 日期：2026-04-21
- owner：`DEV-PLAN-437`
- 当前结论：`PR-437A` 已完成其文档层收敛范围，且当前实现已命中右侧抽屉、`/internal/cubebox`、`modules/cubebox` 三个冻结路径并通过对应门禁，因此 `Phase A` 已完成，`Phase B` 已完成当前范围内的收口验证。
- `2026-04-21` 更新：当前实现已完成本地对话运行时、SSE handler、右侧抽屉共享 reducer、右侧入口打通，并完成前端依赖补齐、Vitest 回归、类型检查、构建验证与 Go/routing/authz/chat-surface-clean 收口。
- `2026-04-21` 更新：为符合 `DEV-PLAN-003` 的“分阶段冻结边界而非临时绕行”要求，当前前端入口权限暂时统一复用现有 `orgunit.read`；正式权限矩阵仍由 `Phase E / 435` owner 收口。
- `2026-04-21` 更新：根据当前产品决策，CubeBox 已从“页面 + 抽屉双承载”收口为“仅右侧抽屉承载”；`/app/cubebox` 路由、左侧导航入口与完整页面跳转按钮均已移除。

## 阶段总览

| 阶段 | 对应 PR | 主要 owner 计划 | 目标 | 当前状态 |
| --- | --- | --- | --- | --- |
| `Phase A` | `PR-437A` | `436`、`430`、`431`、`433`、`434` | 开工门禁、最小上游冻结、共享 canonical contract、本地运行时口径 | `已完成` |
| `Phase B` | `PR-437B` | `431`、`433` | 首轮可用对话链路 | `已完成` |
| `Phase C` | `PR-437C` | `432`、`431` | 会话持久化与恢复 | `进行中` |
| `Phase D` | `PR-437D` | `434`、`431` | 压缩最小闭环 | `未开始` |
| `Phase E` | `PR-437E` | `435`、`433` | 管理面与权限闭环 | `未开始` |

## Phase A / PR-437A

### 目标

- 把 CubeBox 从“文档已拆分但仍不好开工”推进到“具备首轮实现前置条件”。
- 只冻结最小必要项，不要求 `431-435` 全量映射一次性补齐。
- 为首轮对话能力提供单一共享输入。

### 勾选项

1. [x] `chat-surface-clean` 已补充显式批准的新主线路径清单：`/internal/cubebox`、`modules/cubebox`；当前文档层 allowlist 与路线图口径一致。
2. [x] `430` 已回填“按阶段快速开工”的引用，且不与 `437` 路线图冲突。
3. [x] `431`、`433`、`434` 已补齐首轮会使用到的上游 `commit SHA` 与最小文件级映射对象。
4. [x] 已形成共享 canonical contract，明确：
   - conversation / turn / item 命名
   - SSE event envelope
   - `turn.agent_message.delta` / `turn.completed` / `turn.error` / `turn.interrupted`
   - compact / token usage 事件名
   - reducer 输入与 reconstruction 输出 shape
5. [x] 本地可控运行时 / mock SSE / fake provider 口径已冻结，不把真实外部模型调用作为 merge 前置条件。

### 当前证据

- 路线图文档：
  - `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`
- 共享 companion doc：
  - `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
- 文档地图更新：
  - `AGENTS.md`
- `430` 回链与首轮冻结：
  - `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `431` 最小映射冻结：
  - `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- `433` 最小映射冻结：
  - `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `434` 最小映射冻结：
  - `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `432` 消费 shared contract 的恢复输出对齐说明：
  - `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- 门禁 allowlist 更新：
  - `scripts/ci/check-chat-surface-clean.sh`

### 待补证据位

| 证据主题 | 目标文件 / 产物 | 期望证据形态 | 当前状态 |
| --- | --- | --- | --- |
| 反回流门禁更新 | `scripts/ci/check-chat-surface-clean.sh`、相关文档 | diff + 命令结果 | `已补并验证通过；待当前实现首次命中新路径时再次确认` |
| `430` 回链 | `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md` | 文档 diff | `已补` |
| `431` 最小映射冻结 | `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md` | `commit SHA` + 文件级映射 | `已补` |
| `433` 最小映射冻结 | `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md` | `commit SHA` + 文件级映射 | `已补` |
| `434` 最小映射冻结 | `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md` | `commit SHA` + 文件级映射 | `已补` |
| 共享 canonical contract | `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md` | 文档 diff + owner 对齐说明 | `已补` |
| 本地运行时口径 | `433` / companion doc / fixture 方案文档 | 文档 diff + fixture 路径 | `口径已冻结；代码/fixture 已有首轮实现` |

### 命令记录

本阶段已进入文档与门禁修改；执行命令结果如下：

| 日期 | 命令 | 结果 | 备注 |
| --- | --- | --- | --- |
| `2026-04-21` | `bash scripts/ci/check-chat-surface-clean.sh` | `通过` | 输出批准的新主线路径清单，并返回 `[chat-surface-clean] OK` |

### Phase A 完成判定

同时满足以下条件时，`Phase A / PR-437A` 才可标记完成：

1. 上述 5 个勾选项全部完成。
2. `chat-surface-clean` 可通过且显式批准的新主线路径与 `437` 路线图一致。
3. reviewer 能指出首轮实现具体依赖哪份 shared contract，而不是继续依赖口头约定。
4. `Phase B` 已具备可直接开工条件，不再被“先补完整映射表”阻塞。

## 后续阶段预留

### Phase B 已落地证据

- 首轮可用对话链路：
  - [x] Web Shell 右侧抽屉入口已命中活体路径
  - [x] 右侧抽屉已接入 `CubeBoxProvider`、统一 reducer/store 与 timeline/composer 语义
  - [x] `/internal/cubebox` 已提供 create/load/stream/interrupt 最小链路
  - [x] `turn.agent_message.delta` / `turn.completed` / `turn.error` / `turn.interrupted` 已打通
  - [x] `stop / interrupt` 已可用并具备最小回归覆盖
- 主要落地文件：
  - `apps/web/src/pages/cubebox/**`
  - `apps/web/src/layout/AppShell.tsx`
  - `apps/web/src/router/index.tsx`
  - `apps/web/src/navigation/config.tsx`
  - `internal/server/cubebox_api.go`
  - `modules/cubebox/runtime.go`
  - `config/routing/allowlist.yaml`
  - `config/access/policy.csv`
  - `pkg/authz/registry.go`
- 实际执行命令：
  - `pnpm install`（`apps/web`；用于补齐本地缺失的前端依赖，修复 `vitest: not found`）
  - `pnpm --dir apps/web test`
  - `pnpm --dir apps/web typecheck`
  - `pnpm --dir apps/web build`
  - `pnpm --dir apps/web check`
  - `bash scripts/ci/check-chat-surface-clean.sh`
  - `make check doc`
  - `make check routing`
  - `make authz-pack && make authz-test && make authz-lint`
  - `go fmt ./internal/server ./modules/cubebox ./pkg/authz`
  - `go vet ./internal/server ./modules/cubebox ./pkg/authz`
  - `go test ./internal/server ./modules/cubebox ./pkg/authz`
- 结果摘要：
  - 前端本地依赖已按 `apps/web` 工具链口径补齐，`vitest` 可直接运行
  - `apps/web` 的 `test` / `typecheck` / `build` / `check` 已通过
  - `chat-surface-clean`、`doc`、`routing`、`authz` 已通过
  - Go 侧命中目录的 `fmt` / `vet` / `test` 已通过
- 真实页面验证（`2026-04-21`，本地浏览器 + 运行中 `:8080` dev server）：
  - 登录链路已验证：访问 `/app/login`，使用 `admin@localhost / admin123` 登录后成功跳转到 `/app`
  - 入口可发现性已验证：工作台顶栏出现“打开 CubeBox 抽屉”
  - 抽屉链路已验证：打开右侧抽屉后，标题、会话区、timeline、输入框、发送/停止控件均可见
  - 正常消息链路已验证：输入 `hello cubebox` 后，UI 出现 `conv_000001`、用户消息、assistant 流式完成消息，状态收敛为“已完成”
  - 错误消息链路已验证：输入 `please error now` 后，UI 出现 error alert 与 timeline 错误项“当前回复暂时失败，请稍后重试。”，状态收敛为“失败”
  - 中断链路已验证：输入 `stop verification run` 后，真实页面已命中停止按钮，触发 `POST /internal/cubebox/turns/turn_000030:interrupt`，UI 状态收敛为“已中断”
  - 实际命中请求：
    - `POST /internal/cubebox/conversations` => `201`
    - `POST /internal/cubebox/turns:stream` => `200`
    - `POST /internal/cubebox/turns/turn_000030:interrupt` => `200`
  - 当前残余控制台噪音：
    - `GET /favicon.ico` => `404`
    - 早先未登录/未授权阶段的历史记录里仍能看到一次 `422 /iam/api/sessions` 与一次 `403 /internal/cubebox/conversations`；两者不再阻断当前已登录后的主链验证结果

### Phase C / PR-437C 当前证据（`2026-04-21`）

- 会话持久化与恢复：
  - [x] 最小正式数据面已落地：`iam.cubebox_conversations` + `iam.cubebox_conversation_events`
  - [x] `modules/cubebox` 已新增正式 store，并由 `internal/server` 消费；会话主事实源不再是纯前端内存
  - [x] append-only message/event log 已接入流式主链：`turn.started` / `turn.user_message.accepted` / `turn.agent_message.delta` / `turn.error` / `turn.interrupted` / `turn.completed` 会在 stream 期间顺序写入
  - [x] conversation lifecycle 最小 API 已落地：
    - `GET /internal/cubebox/conversations`
    - `POST /internal/cubebox/conversations`
    - `GET /internal/cubebox/conversations/{conversation_id}`
    - `PATCH /internal/cubebox/conversations/{conversation_id}`（title / archived）
  - [x] 抽屉 reopen 恢复已接入：provider 初始化时先读 list，再自动恢复最近 active conversation
  - [x] 抽屉内最小会话列表 UI 已落地：列表、选中切换、重命名入口、归档/取消归档入口
  - [ ] reconstruction fixture / golden 测试仍需继续加强

- 主要落地文件：
  - `modules/iam/infrastructure/persistence/schema/00009_iam_cubebox_conversations.sql`
  - `migrations/iam/20260421120000_iam_cubebox_conversations.sql`
  - `modules/cubebox/infrastructure/sqlc/queries/conversations.sql`
  - `modules/cubebox/store.go`
  - `internal/server/cubebox_api.go`
  - `internal/server/cubebox_api_test.go`
  - `internal/server/handler.go`
  - `internal/server/authz_middleware.go`
  - `config/routing/allowlist.yaml`
  - `apps/web/src/pages/cubebox/api.ts`
  - `apps/web/src/pages/cubebox/CubeBoxProvider.tsx`
  - `apps/web/src/pages/cubebox/CubeBoxPanel.tsx`
  - `apps/web/src/pages/cubebox/types.ts`
  - `sqlc.yaml`

- 当前验证结果：
  - [x] `make sqlc-generate`
  - [x] `go test ./internal/server ./modules/cubebox/...`
  - [x] `pnpm --dir apps/web test`
  - [x] `pnpm --dir apps/web typecheck`
  - [x] `pnpm --dir apps/web build`
  - [x] `make check routing`
  - [x] `make authz-pack && make authz-test && make authz-lint`
  - [x] `make check doc`
  - 说明：`vite build` 通过，当前仍存在既有的 chunk size warning，但不阻断本轮 `437C` 收口

### Phase C / PR-437C 当前盘点（`2026-04-21`）

- [x] 已完成现状盘点：当前活体实现仅包含前端共享 reducer / provider、`/internal/cubebox` 的 create/load/stream/interrupt handler，以及 `modules/cubebox/runtime.go` 的内存 runtime。
- [x] 已确认当前分支不存在可直接复用的 `cubebox` 正式持久化对象：
  - 无活体 `modules/cubebox/infrastructure/sqlc/**`
  - 无活体 `modules/cubebox/infrastructure/persistence/**`
  - 无活体 `cubebox` schema / migration 文件
- [x] 已确认 `PR-437C` 的真实第一阻塞点是数据库对象，而不是前端壳层：
  - 若要把 conversation / message / event 落到正式存储，必须新增 `cubebox` 相关表与 sqlc/store
  - 该动作按仓库规则需用户手工确认后才能继续执行
- [ ] 待确认后按最小批次推进：
  - conversation / message / event 存储模型
  - `GET /internal/cubebox/conversations/{id}` 与 list/read API
  - 抽屉 reopen 恢复
  - reconstruction fixture / golden 测试
  - 会话列表 UI

### Phase D / PR-437D 预留证据

- 压缩最小闭环：
  - manual compact
  - pre-turn auto compact
  - canonical context reinjection
  - prompt shape snapshot

### Phase E / PR-437E 预留证据

- 管理面与权限闭环：
  - provider / active model / credential / health UI
  - Authz 矩阵
  - validation / readiness / masking / rotation 证据

## 当前风险

- `437A` 已存在，但若后续实现没有真正消费该 companion doc，`431/432/434` 仍可能在代码层重新分叉。
- 若 `435` 抢在 `433` 运行时对象命名前先做管理面，会重新引入 Slice 2 / Slice 5 双主参考或双命名问题。
- 若继续把“完整映射表冻结”当作普遍前置，路线图虽然存在，开工节奏仍会停在文档层。

## 当前裁决

- `DEV-PLAN-437` 已具备从文档冻结到当前可用对话能力的完整 readiness 证据链。
- `PR-437A` 已完成文档层冻结与门禁语义收敛，`Phase B` 已完成首轮对话能力并通过相应验证。
- 下一步进入 `PR-437C`：以 `432` 为 owner 推进会话持久化、恢复与 conversation lifecycle，避免把当前前端内存态误延长为长期实现。

## 关联文档

- `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`
- `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`
- `docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`
