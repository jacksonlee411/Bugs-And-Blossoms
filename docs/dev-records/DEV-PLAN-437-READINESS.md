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
- `2026-04-22` 更新：前端 CubeBox 入口已从临时 `orgunit.read` 收口为 `cubebox.conversations.read/use`；抽屉内 settings 权限也已拆分为 `read/update/rotate/select/verify/deactivate`，但正式四类角色矩阵仍由 `Phase E / 435` owner 收口。
- `2026-04-21` 更新：根据当前产品决策，CubeBox 已从“页面 + 抽屉双承载”收口为“仅右侧抽屉承载”；`/app/cubebox` 路由、左侧导航入口与完整页面跳转按钮均已移除。

## 阶段总览

| 阶段 | 对应 PR | 主要 owner 计划 | 目标 | 当前状态 |
| --- | --- | --- | --- | --- |
| `Phase A` | `PR-437A` | `436`、`430`、`431`、`433`、`434` | 开工门禁、最小上游冻结、共享 canonical contract、本地运行时口径 | `已完成` |
| `Phase B` | `PR-437B` | `431`、`433` | 首轮可用对话链路 | `已完成` |
| `Phase C` | `PR-437C` | `432`、`431` | 会话持久化与恢复 | `已具备正式封板条件` |
| `Phase D` | `PR-437D` | `434`、`431` | 压缩最小闭环 | `已具备正式封板条件` |
| `Phase E` | `PR-437E` | `435`、`433` | 管理面与权限闭环 | `最小运行态闭环已通过，权限矩阵与完整管理面未封板` |

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
   - compact 事件名，以及后续可选的 token usage 扩展位
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
| `435` Phase E 映射冻结 | `docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md` | `commit SHA` + 对象级映射 + 采用状态 | `2026-04-22 已补首轮冻结` |
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
  - [x] reconstruction fixture / golden 测试已补首轮封板：`apps/web/src/pages/cubebox/reconstruction.fixtures.ts` + `apps/web/src/pages/cubebox/reducer.test.ts`
  - [x] 恢复页面级验证已补：`apps/web/src/pages/cubebox/CubeBoxPanel.restore.test.tsx`
  - [x] 恢复链路已补行为纠偏：reopen 时跳过 archived conversation，恢复最近 active conversation；reconstruction 现已回放 `conversation.renamed / archived / unarchived`

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
  - `apps/web/src/pages/cubebox/CubeBoxProvider.test.tsx`
  - `apps/web/src/pages/cubebox/CubeBoxPanel.tsx`
  - `apps/web/src/pages/cubebox/CubeBoxPanel.restore.test.tsx`
  - `apps/web/src/pages/cubebox/reconstruction.fixtures.ts`
  - `apps/web/src/pages/cubebox/reducer.test.ts`
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

### Phase C / PR-437C 当前盘点（`2026-04-22`）

- [x] 正式会话数据面、最小 lifecycle API、append-only event log、抽屉 reopen 恢复、最小会话列表 UI 均已落地。
- [x] reconstruction 首轮封板证据已补：
  - fixture / golden：`apps/web/src/pages/cubebox/reconstruction.fixtures.ts`
  - reducer 回放验证：`apps/web/src/pages/cubebox/reducer.test.ts`
  - 页面级恢复验证：`apps/web/src/pages/cubebox/CubeBoxPanel.restore.test.tsx`
- [x] store / API / UI 三层共享 lifecycle roundtrip golden 已补：
  - 共用 fixture / golden：`apps/web/src/pages/cubebox/lifecycle.fixture.ts`
  - store 级 roundtrip 对照：`modules/cubebox/store_test.go`
  - API 级 roundtrip 对照：`internal/server/cubebox_api_test.go`
  - UI reducer 对照：`apps/web/src/pages/cubebox/reducer.test.ts`
- [x] 已消除一处恢复语义偏差：
  - provider 恢复链路不再盲选列表第一项，而是显式跳过 archived conversation，恢复最近 active conversation
  - reducer 已开始回放 `conversation.renamed / conversation.archived / conversation.unarchived`，避免读取事件日志恢复后标题/归档态停留旧值
- [x] `PATCH /internal/cubebox/conversations/{conversation_id}` 的 rename / archive / unarchive handler 级成功路径验证已补：`internal/server/cubebox_api_test.go`
- [x] 压缩后恢复验证已补：`turn.context_compacted` 现已纳入 restore fixture 与页面级恢复验证
- [x] store/持久层跨租户隔离的 fail-closed 验证已补：错租户/错 principal 命中 `ErrConversationNotFound`，且 `app.current_tenant` 注入已在 store 测试中显式断言
- [x] `summary 不替代原始消息` 的跨层证据已补：
  - compaction 纯函数验证 `prompt view` 只新增摘要，不覆盖原始 timeline：`modules/cubebox/compaction_test.go`
  - reducer / reconstruction 验证 compact event 回放后，原始 user/agent message 仍可恢复：`apps/web/src/pages/cubebox/reducer.test.ts`
- [x] `PR-437C` 当前已满足最小正式封板口径：
  - store / API / UI 已围绕同一 lifecycle roundtrip fixture / golden 完成对照，恢复语义不再仅停留在前端单层验证
  - `432` 中 `archive/unarchive/read/list/resume 测试` 与 `list/read/resume/archive/unarchive/rename 生命周期冻结` 已可按当前最小范围记为完成

### Phase D / PR-437D 当前证据（`2026-04-22`）

- 压缩最小闭环：
  - [x] manual compact：`POST /internal/cubebox/conversations/{conversation_id}:compact`
  - [x] pre-turn auto compact：stream 前自动 compact 并续接 `next_sequence`
  - [x] canonical context reinjection：`modules/cubebox/compaction.go`
  - [x] summary prefix / compaction 纯函数测试：`modules/cubebox/compaction_test.go`
  - [x] `/compact` UI 入口与 `compact_item` timeline 消费：`apps/web/src/pages/cubebox/CubeBoxPanel.tsx`、`apps/web/src/pages/cubebox/reducer.ts`
  - [x] 压缩后恢复验证：`apps/web/src/pages/cubebox/CubeBoxPanel.restore.test.tsx`
  - [x] no-op compaction 不落 `turn.context_compacted` 事件，前端不再伪造空摘要项：`modules/cubebox/store.go`、`internal/server/cubebox_api_test.go`、`apps/web/src/pages/cubebox/CubeBoxProvider.test.tsx`
  - [x] compaction 序号推进已收敛为单事务安全，避免 pre-turn auto compact / manual compact 并发时撞 `sequence` 唯一索引：`modules/cubebox/store.go`、`modules/cubebox/store_test.go`

- 当前验证结果：
  - [x] `go test ./modules/cubebox ./internal/server`
  - [x] `pnpm -C apps/web exec vitest run src/pages/cubebox/api.test.ts src/pages/cubebox/reducer.test.ts src/pages/cubebox/CubeBoxProvider.test.tsx src/pages/cubebox/CubeBoxPanel.test.tsx src/pages/cubebox/CubeBoxPanel.restore.test.tsx`
  - [x] `make check routing`
  - [x] `make authz-test`
  - [x] `make check doc`
  - [x] `make check chat-surface-clean`

- 封板裁决：
  - [x] `PR-437D` 当前已满足最小正式封板口径：
    - manual compact、pre-turn auto compact、canonical context reinjection、`/compact` UI 入口、压缩后恢复与 prompt shape fixture / snapshot 已闭环。
    - no-op compaction 不再伪造 compact event / 空摘要项，并发 compact 的 `sequence` 竞争也已收敛为单事务安全。
    - 首期 `prompt shape snapshot` 以纯函数 fixture / snapshot 承担 golden 等价物，不再把“尚未拆出独立 golden 文件”视为阻断项。
  - [x] 以下能力明确后移且不阻断本轮封板：mid-turn compact、remote compaction、model downshift、真实 tokenizer 校准、更完整的 store 级跨租户隔离扩展测试。

### Phase E / PR-437E 预留证据

- 管理面与权限闭环：
  - provider / active model / credential / health UI
  - Authz 矩阵
  - validation / readiness / masking / rotation 证据
- `2026-04-22` 文档前置冻结已补：
  - `435/5A` 已固定 `Bifrost 865c097cadcfe3677dbf2cc0fb1d63a699fab552`、`One API 8df4a2670b98266bd287c698243fff327d9748cf`、`Codex 69c8913e24f2f455c8f000fa7afe039a38bdd48d`
  - `435/5A` 已将 `provider config / provider keys / health / active model / capability metadata` 映射到具体上游对象，不再停留在“风格参考”
  - `433/5C` 已冻结 `provider` / `credential` / `active model` / `health` 共享对象名，避免 `433` 与 `435` 双命名
- `2026-04-22` 运行态 / 页面态验证已补：
  - `:8080` 上已重建并切到新版前端静态产物，右上角 CubeBox 抽屉与抽屉内齿轮 settings 入口可见
  - 抽屉内 `CubeBox 设置` 弹窗已不再是旧占位文案，而是最小真实表单：当前模型、health、provider、credential、active model、verify 均可见
  - 运行态已验证 `provider -> credential / selection / verify` 最小链路可通，settings 读面已能回显真实配置与健康数据
- 当前裁决：
  - `PR-437E` 已不再是“未开始”；当前已完成抽屉内 settings 弹窗的首轮实现，且最小运行态闭环已通过
  - 但 `PR-437E` 仍未封板：当前实现尚不足以覆盖 `435` 要求的完整管理面 IA，也尚未完成 `platform admin / platform operator / tenant admin / user` 权限矩阵落地
  - 实施入口应继续沿 `435 Slice 5.1-5.5` 与 `433 Slice 2.3/2.4` 收口，而不是把当前临时 settings 弹窗误记为 `Phase E` 已完成

## 当前风险

- `437A` 已存在，但若后续实现没有真正消费该 companion doc，`431/432/434` 仍可能在代码层重新分叉。
- 若 `435` 抢在 `433` 运行时对象命名前先做管理面，会重新引入 Slice 2 / Slice 5 双主参考或双命名问题。
- `435/5A` 虽已冻结，但若实现时绕开这些对象级映射，仍会退回“名义参考 Bifrost、实际自研管理面”的偏航。
- 当前仓内虽然已有最小 settings API 与抽屉内 settings 弹窗，但 `cubebox.model_provider` / `cubebox.model_credential` / `cubebox.model_selection` 的 Authz object/action 与四类角色边界仍未完整落地，`PR-437E` 的真正风险已从“文档未冻结”转为“代码只完成了临时形态，尚未收口为正式管理面”。

## 当前裁决

- `DEV-PLAN-437` 已具备从文档冻结到当前可用对话能力的完整 readiness 证据链。
- `PR-437A` 已完成文档层冻结与门禁语义收敛，`Phase B` 已完成首轮对话能力并通过相应验证。
- `PR-437C` 已具备正式封板条件：正式数据面、最小 lifecycle API、抽屉恢复、压缩后恢复、跨租户 fail-closed，以及 store/API/UI 共用 lifecycle roundtrip golden 均已落地并有回归证据。
- `PR-437D` 已具备正式封板条件：最小 compaction 闭环、恢复链路、prompt shape fixture / snapshot、no-op 收口与并发序号安全均已落地并回填证据。
- `PR-437E` 的文档前置条件现已补齐，且首轮代码已进入运行态：新版 settings 弹窗、settings 读面与最小 provider / credential / selection / verify 链路已可验证。
- `PR-437E` 当前应记为“最小运行态闭环已通过，权限矩阵与完整管理面未封板”：新版 settings 入口和最小表单已存在，但完整管理面 IA 与四类角色权限矩阵仍未落地。
- `PR-437E` 口径保持不变：最小运行态闭环已通过，但权限矩阵与完整管理面未封板；下一条产品主链已改由 `433 Slice 2.1-2.3` 承接，用于把 CubeBox 从 deterministic runtime 推进到真实 provider 对话主链。
- `2026-04-22` 已继续完成 `433 Slice 2.4` 首轮封板：`POST /internal/cubebox/settings/verify` 已切成真实 provider 验证，并把验证结果真实写回 `health`；`status / latency_ms / error_summary / validated_at` 不再是启发式占位。
- `2026-04-22` 已补充 `433 Slice 2.5` 的首轮冻结：`usage_event` 数据面暂缓，不作为当前 merge gate；首轮只要求最小 lifecycle telemetry 与 canonical event final 语义稳定。
- 项目当前尚未建设 `outbox` 能力，`outbox` 已从 `DEV-PLAN-433` 暂停实施；当前计划不承接事务内登记 + 异步重试的最终一致保障，仅要求 `turn.started / turn.error / turn.completed` 的 final 语义在单请求路径内稳定收口。
- `2026-04-22` 已完成 `DEV-PLAN-433A` 代码实施：terminal error 改为 `AppendEvents` append-first，失败 turn 写入 `turn.error` + `turn.completed(status=failed)`；`settings/verify` 复用真实 provider adapter / stream parser / 错误归一化并写回 health；新增 `/internal/cubebox/capabilities`，settings 入口按真实 session capability fail-closed；compact context 使用 `provider_id / provider_type / model_slug / runtime` 分离字段，不再写 `deterministic-runtime`。
- `DEV-PLAN-433A` 自动化证据：`go test ./modules/cubebox ./internal/server` 通过；`cd apps/web && pnpm typecheck && pnpm test -- CubeBoxPanel reducer api` 通过；`git diff --check` 通过。前端测试仍有既有 React `act(...)` warning，但断言全部通过。
- `DEV-PLAN-433A` 真实浏览器复验已补（2026-04-22 20:17 CST）：Playwright 从 `http://localhost:8080/app/login` 登录 `admin@localhost`，进入 `/app` 后打开主壳层右侧 CubeBox 抽屉；旧整页入口计数为 `0`，抽屉 `role=complementary` 可见。
- `DEV-PLAN-433A` 网络证据：`/internal/cubebox/capabilities`、`/internal/cubebox/settings`、`/internal/cubebox/settings/providers`、`/internal/cubebox/settings/credentials`、`/internal/cubebox/settings/selection`、`/internal/cubebox/settings/verify`、`/internal/cubebox/conversations`、`/internal/cubebox/turns:stream` 均通过真实浏览器 session 发起；未出现 `/internal/cubebox/**` 401。
- `DEV-PLAN-433A` settings/verify 证据：`settings/verify` 返回 `200` 并写回 health；health 回显从 `validated_at=2026-04-22T12:01:03Z / latency_ms=30012 / error_summary=provider_stream_timeout` 更新为 `validated_at=2026-04-22T12:17:30Z / latency_ms=30002 / error_summary=provider_stream_timeout`。
- `DEV-PLAN-433A` 真实 provider turn 证据：`turns:stream` 返回 `200 text/event-stream`，SSE 为 `turn.started -> turn.user_message.accepted -> turn.error -> turn.completed(status=failed)`；生命周期字段包含 `runtime=openai-chat-completions`、`trace_id`、`provider_id=openai-compatible`、`provider_type=openai-compatible`、`model_slug=gpt-4.1`，未回退 `deterministic-fixture`。
- `DEV-PLAN-433A` DB replay 证据：新会话 `conv_4a32db7ea99e4fb5b171bdd0e137ad4d` 已落库 `conversation.loaded`、`turn.started`、`turn.user_message.accepted`、`turn.error(code=ai_model_provider_unavailable, latency_ms=30013)`、`turn.completed(status=failed, latency_ms=30013)`，失败恢复不再留下 dangling streaming turn。
- `DEV-PLAN-433A` 收尾状态：按用户要求保留真实 credential 引用并把 active selection 恢复为 `model_slug=gpt-5.2`；provider 仍为 `openai-compatible / https://api.openai.com/v1 / enabled=true`，active credential version 为 `4`，证据未记录真实 key。
- `DEV-PLAN-433A` 剩余阻塞：真实 provider 外网/上游当前 30 秒超时，未拿到成功 `turn.agent_message.delta`，因此“成功 turn”和“streaming 中点击停止”的真实页面证据仍待 provider 连通后补验；测试专用 credential 破坏性 fail-closed 页面用例仍待独立测试 provider/credential。

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
