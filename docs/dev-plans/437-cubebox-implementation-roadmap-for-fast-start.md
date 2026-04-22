# DEV-PLAN-437：CubeBox 实施路线图（快速开工版）

**状态**: 执行中（2026-04-22 CST，`PR-437A` / `Phase B` 已完成；`Phase C` 与 `Phase D` 已具备正式封板条件）

## 0. 适用范围与评审分级

- **评审分级**：`T3`
- **范围一句话**：作为 `DEV-PLAN-430` 及其子计划 `431-435` 的实施路线图，冻结“如何快速开工”的执行顺序、并行策略、最小前置冻结项、首轮可用能力和阶段验收口径；不替代各子计划自身的契约 owner。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`、`docs/dev-plans/431a-cubebox-page-design-contract.md`、`docs/dev-plans/432-codex-session-persistence-reuse-plan.md`、`docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`、`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`、`docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`、`docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`、`apps/web`、`internal/server`、`modules/cubebox`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-430`、`DEV-PLAN-431`、`DEV-PLAN-432`、`DEV-PLAN-433`、`DEV-PLAN-434`、`DEV-PLAN-435`、`DEV-PLAN-436`、`DEV-PLAN-437A`
- **用户入口/触点**：Web Shell 右侧 CubeBox 入口、`/internal/cubebox` API、模型配置页、会话列表/恢复、流式对话、压缩提示

### 0.1 Simple > Easy 三问

1. **边界**：`437` 只定义实施顺序、切片编排、并行原则与开工门槛；`430` 继续是总架构 PoR，`431-435` 继续分别持有 UI、持久化、网关、压缩、管理面的正式契约。
2. **不变量**：不得因为追求“快速开工”而绕过 `430` 已冻结的无 legacy、单前端主链、服务端网关、append-only 审计、当前用户权限执行与上游复用审计要求。
3. **可解释**：reviewer 必须能在 5 分钟内讲清为什么先做哪一轮能力、哪些项必须先冻结、哪些项可以并行，以及为什么这条路线比“所有子计划先写完再开工”更简单。

### 0.2 现状研究摘要

- **现状实现**：`430` 已定义 Slice 0-6 的目标切片，但当前仍是“总计划 + 子计划集合”，缺少一份面向实施的执行路线图，容易出现全部前置冻结同时要求、实际无法开工的问题。
- **现状约束**：`431-435` 都要求上游 `commit SHA`、文件级映射表、采用状态与 stopline；`436` 已冻结旧对话栈清场方向；仓库仍要求单主链、无 legacy、RLS/Authz/错误码/i18n/文档门禁一致。
- **最容易出错的位置**：
  - 把“上游映射表冻结”扩张为“所有子计划全部冻结完才允许任何代码落地”
  - `431/432/434` 各自先行实现，形成不同的 conversation/turn/event/compaction 语义
  - 再次长出抽屉之外的第二套路由页面或第二套 store/reducer/SSE 消费链
  - 先做全量管理面或全量持久化，迟迟没有首轮可用对话链路
- **本次不沿用的“容易做法”**：
  - 先把 `431-435` 全文档冻结到巨细无遗再开工
  - 先做“模型管理后台”或“复杂持久化”再回头拼对话主链
  - 为了快速演示而在前端直连外部模型 API
  - 让 UI、API、compaction 各自发明一套接近但不一致的 DTO/事件名

## 1. 背景与上下文

- `DEV-PLAN-430` 已把 CubeBox 重做拆成 UI 壳、AI 网关、会话持久化、上下文压缩、模型配置管理面与封板验证六个切片。
- `DEV-PLAN-431-435` 已补齐各切片的 owner 文档，但它们的写法仍偏“契约冻结”，尚未把“最快形成第一条用户可见闭环”的执行顺序收敛为单独路线图。
- 当前最需要的不是再补一份“更大的总方案”，而是明确：
  - 哪些前置项必须先冻结
  - 哪些切片可以并行
  - 首轮能力边界是什么
  - 何时从本地可控运行时进入持久化和 compaction 闭环

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结 `430/431-435` 的实施顺序，使 CubeBox 可以在最小前置条件下快速开工。
- [ ] 定义“最小前置冻结集”，避免把全部子计划同时阻塞在文档完善阶段。
- [ ] 定义首轮用户可见能力：右侧入口 -> 发送消息 -> 流式回复 -> 停止/完成。
- [ ] 定义第二阶段、第三阶段的扩展顺序，使持久化、恢复、压缩、管理面按依赖自然落位。
- [ ] 冻结并行规则和 stopline，阻断双主链、伪复用和管理面抢跑。

### 2.2 非目标

- 不替代 `430` 的架构总契约。
- 不替代 `431-435` 的详细对象模型、事件协议、上游映射表与验收清单。
- 不把 `437` 写成新的“全功能设计文档”；它只回答“先做什么、怎么分批、什么必须等什么”。
- 不把 fallback/failover、quota、route alias、default model、真实外部模型 E2E 加回首期 required gate。

### 2.3 用户可见性交付

- **用户可见入口**：Web Shell 右侧悬挂入口。
- **最小可操作闭环**：用户可以打开 CubeBox、输入问题、收到流式回复、停止生成、关闭后重新打开仍看到当前 UI 状态。
- **快速开工的定义**：不是“文档写完”，而是首轮能力在不违背 `430` stopline 的前提下进入可运行状态。

## 2.4 工具链与门禁（SSOT 引用）

> `437` 本身是路线图文档，主要命中文档与 readiness；实际代码实施仍按各子计划命中的触发器执行。

- **命中触发器（勾选）**：
  - [ ] Go 代码
  - [ ] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [ ] DB Schema / Migration / Backfill / Correction
  - [ ] sqlc
  - [ ] Routing / allowlist / responder / 相关路由注册/映射
  - [ ] AuthN / Tenancy / RLS
  - [ ] Authz（Casbin）
  - [ ] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`chat-surface-clean`

- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/000-docs-format.md`
  - `docs/dev-plans/001-technical-design-template.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
  - `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
  - `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
  - `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
  - `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
  - `docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`
  - `docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`

## 3. 路线图总原则

### 3.1 最小前置冻结集

在任何 CubeBox 活体代码开工前，只要求以下项目先冻结，不要求 `431-435` 全量细节一次性写完：

1. `436` 的旧对话面硬删除与 `chat-surface-clean` 精确 allow/block 规则已可作为前置门槛。
2. `430` 已冻结首期暂缓项：`fallback/failover`、`quota`、`route alias`、`default model` 不进入首期。
3. `431/433/434` 至少已补齐首轮要消费的上游 `commit SHA` 与“本阶段使用到的文件级映射对象”，不要求先覆盖全部未来切片。
4. 已形成一份共享 canonical contract 草案，至少冻结：
   - conversation/turn/item 的命名
   - SSE event envelope
   - `turn.agent_message.delta` / `turn.completed` / `turn.error` / `turn.interrupted`
   - compact 事件名，以及后续可选的 token usage 扩展位
   - 该 companion doc 现冻结为 `DEV-PLAN-437A`
5. 已明确首轮能力使用本地可控运行时 / mock SSE / fake provider，不把真实外部模型连通性作为 merge 前置条件。

### 3.2 串行与并行规则

必须串行的事项：

1. `436` 清场门禁与 `chat-surface-clean` allowlist 更新先于 `modules/cubebox` 活体代码。
2. 共享 canonical contract 先于 `431` reducer、`432` reconstruction、`434` compact event 对接。
3. `433` 的 provider/config 命名先于 `435` 管理面对象命名和 active model 配置 UI。

可以并行的事项：

1. `431` 的抽屉壳层与 `433` 的本地可控运行时 / SSE mock 可以并行。
2. `432` 的持久化对象职责冻结与 `434` 的 compaction 纯函数接口冻结可以并行，但二者都要消费同一份 canonical contract。
3. `431A` 的页面视觉合同可以与 `431` 组件 IA 同步推进，但不得反向阻塞 reducer/SSE 首轮实现。

### 3.3 第一性排序原则

首轮优先级按以下顺序冻结：

1. 能让用户“看到并操作”的主链优先于后台治理面。
2. 能冻结共享语义的 contract 优先于各层各写一套局部 DTO。
3. 能形成可回归测试的本地可控实现优先于真实 provider 连接。
4. 能减少未来返工的 owner 边界优先于“先写起来再说”的局部便利。

## 4. 分阶段实施路线

### Phase A：开工前冻结与最小骨架

目标：把“可以安全开工”的门槛降到最小，但不破坏 `430` 不变量。

1. [X] 完成 `chat-surface-clean` 精确 allowlist 更新，允许 `/internal/cubebox`、`modules/cubebox`，继续阻断旧 `assistant` / LibreChat / 旧表名 / 旧错误码。
2. [X] 在 `430` 与 `431/433/434` 中补齐首轮会用到的上游 `commit SHA` 与最小文件级映射，不等待未来全部映射对象冻结完毕。
3. [X] 产出一份共享 canonical contract 附录或 companion doc，作为 `431/432/434` 共同输入；当前 companion doc 冻结为 `DEV-PLAN-437A`：
   - conversation/turn/item 生命周期
   - SSE event naming
   - reducer 输入 shape
   - reconstruction 输出 shape
4. [X] 冻结本地可控运行时口径：mock SSE / fake provider / fixed transcript fixture。

阶段完成定义：

- 可以开始写 `modules/cubebox`、`apps/web`、`internal/server` 的首轮代码。
- reviewer 不再要求“先把 431-435 所有映射表全部补完”。

### Phase B：首轮可用对话能力

目标：尽快形成“可打开、可发问、可流式回复、可停止”的用户闭环。

owner：

- `431`：抽屉壳层、统一 store/reducer、timeline/composer/status bar、SSE 消费
- `433`：本地可控运行时、request mapping 最小链路、SSE passthrough、错误映射

范围：

1. [X] Web Shell 右侧图标 + 抽屉挂载完成。
2. [X] 右侧抽屉承载唯一正式 UI 主链，复用同一套 store/reducer/component 语义。
3. [X] `turn.agent_message.delta`、`turn.completed`、`turn.error`、`turn.interrupted` 已打通。
4. [X] 可用本地可控运行时返回稳定流式输出。
5. [X] stop/interrupt 可见且可回归测试。
6. [X] 不要求真实持久化，不要求真实 provider health，不要求 active model 管理面闭环。

Phase B 入口权限说明：

- 首轮对话能力的运行时 object 始终保持 `cubebox.conversations`，不再借用其他业务域对象命名。
- `2026-04-22` 起，前端抽屉入口已从早期临时 `orgunit.read` 收口为 `cubebox.conversations.read/use`。
- 完整四类角色矩阵仍由 `435` 继续收口；不得把当前已落地的最小动作键误记为最终角色设计已完成。

阶段完成定义：

- 用户已能看到“真实对话界面”，不是设计样板页。
- E2E/组件测试可以稳定回归基础对话链路。

### Phase C：会话持久化与恢复

目标：把“能聊”升级为“能恢复、能归档、能追溯”。

owner：

- `432`：conversation lifecycle、append-only message log、read/list/resume/archive/rename contract
- `431`：会话列表与恢复入口 UI

范围：

1. [ ] conversation/message/summary 职责冻结并落到 schema/sqlc 方案；`usage_event` 数据面后移。
2. [X] append-only 原始消息写入与 final 状态固化打通。
3. [X] conversation list/read/resume/archive/rename 的 API contract 可被 UI 消费。
4. [X] 抽屉关闭后重新打开可以恢复 active conversation。
5. [X] reconstruction 输出与 `431` reducer 对齐。

阶段完成定义：

- 用户关闭再打开后可恢复会话。
- 会话读取不再依赖纯前端内存态。
- 当前备注（`2026-04-22`）：
  - 正式数据面、最小 lifecycle API、抽屉恢复、会话列表 UI 与前端 reconstruction fixture / golden 已落地。
  - `432` owner 下的后端 `PATCH` rename/archive/unarchive handler 级验证、前端 reconstruction fixture / restore 对照、以及“压缩后恢复”回归已补齐。
  - store/API/UI 三层已围绕同一 lifecycle roundtrip fixture / golden 补齐对照，store 级跨租户 fail-closed 验证与 readiness 证据已回填，因此 `Phase C` 当前已具备正式封板条件。
  - 更大范围的持久化扩展项，例如完整 summary 数据面、后续 `usage_event` 数据面与更细颗粒审计对象，仍由 `432` 后续切片继续推进，但不再阻断本轮 `Phase C` 封板。

### Phase D：上下文压缩最小闭环

目标：只交付首期真正需要的 compaction 能力，不把 P1 复杂项挤进首轮。

owner：

- `434`：manual compact、pre-turn auto compact、canonical context reinjection、prompt view replacement
- `431`：`/compact` 入口与状态提示

首期必须项：

1. [X] manual compact
2. [X] pre-turn auto compact
3. [X] canonical context reinjection
4. [X] prompt shape snapshot / summary prefix fixture / compaction 纯函数测试

明确后移到 P1 的项：

- mid-turn compact
- model downshift compact
- remote compaction

阶段完成定义：

- 对话不会无限追加历史。
- 压缩后仍保持 append-only 原始消息审计链不变。
- 当前备注（`2026-04-22`）：
  - manual compact API、pre-turn auto compact、canonical context reinjection、`turn.context_compacted` timeline 消费、`/compact` UI 入口与 compaction 纯函数测试已落地。
  - 本轮实现级收口已补齐：no-op compaction 不再伪造 compact event / 空摘要项，compaction 序号推进也已收敛为单事务安全，不再因并发 compact 抢占 `sequence` 而阻断正常流式请求。
  - 首期 `prompt shape snapshot` 以纯函数 fixture / snapshot 承担 golden 等价物，已满足当前封板口径，不再要求本轮额外拆出独立 golden 文件。
  - 当前实现仍属最小闭环：未引入 mid-turn compact、remote compaction、provider downshift，也未承诺真实 tokenizer 精度或独立 summary model；这些后移项不阻断 `Phase D` 正式封板。

### Phase E：模型配置管理面与权限闭环

目标：在运行时对象命名已经稳定后，再补 provider/config 管理面。

当前状态（`2026-04-22`）：

- `435/5A` 上游映射已完成首轮冻结：`Bifrost` 为唯一主参考，`One API` 仅补 IA，`Codex` 仅补 capability / metadata。
- `433/5C` 已补 `provider` / `credential` / `active model` / `health` 共享对象口径，`PR-437E` 已完成最小运行态闭环。
- 代码实现已进入首轮落地：右侧抽屉内 `settings` 入口、新版 settings 弹窗、provider / credential / active model / verify 最小表单与后端链路已通过页面运行态验证。
- 当前仍未封板：完整管理面 IA、`platform admin / platform operator / tenant admin / user` 权限矩阵、Authz object/action 最终收口，以及管理面 E2E 仍属本阶段待办。

owner：

- `435`：provider/credential/active model/health 管理面与权限矩阵
- `433`：health/readiness/validation 运行时语义与 provider capability owner

范围：

1. [ ] provider 列表、active model 面板、密钥生命周期、健康验证 UI。
2. [ ] 平台 admin / platform operator / tenant admin / user 权限矩阵落地。
3. [ ] 前端不暴露密钥明文。
4. [ ] route/fallback/quota/default model 继续留在非首期。

阶段完成定义：

- 管理员可配置首期 one provider + active model + health validation。
- 普通用户可以看到可用模型展示名并使用，不具备密钥读写权限。

## 5. 共享契约与 owner 边界

| 主题 | 正式 owner | 其他计划的消费方式 | 本路线图要求 |
| --- | --- | --- | --- |
| 总体架构/stopline | `430` | `431-435` 承接切片 | 不得在 `437` 重写总体架构 |
| UI 协议/状态机/抽屉壳层 | `431` | `432/434` 对齐 reducer 输入输出 | 先冻结 `DEV-PLAN-437A` 再开工 |
| 页面视觉合同 | `431A` | `431` 页面实现消费 | 不得反向阻塞首轮能力 |
| 会话 lifecycle/persistence | `432` | `431` 消费其 list/read/resume/archive/rename 结果 | UI 不得偷持 lifecycle owner |
| AI gateway/provider/health runtime | `433` | `431/435` 消费其运行时对象 | 管理面不得抢先定义命名 |
| compaction 语义与执行链 | `434` | `431` 只承接 `/compact` 入口与状态提示 | UI 不得自造第二套 compact 语义 |
| 模型配置管理面/权限矩阵 | `435` | `433` 提供 provider/config/health 运行时事实 | 运行时与管理面命名必须统一 |

## 6. 快速开工的 PR 编排建议

### PR-437A：开工门禁与共享 contract

- 更新 `chat-surface-clean`
- 冻结最小上游 SHA 与首轮文件级映射
- 补共享 canonical contract
- 不进入真实业务实现

当前状态：已完成。

### PR-437B：首轮对话能力

- `431` 抽屉壳层 + 统一 reducer/store
- `433` 本地可控运行时 + SSE passthrough
- 打通发送、流式回复、停止生成

当前状态：已完成；当前仓库已命中右侧抽屉、`/internal/cubebox`、`modules/cubebox` 三个活体路径，并通过前端、Go、routing、authz 与 `chat-surface-clean` 收口验证。

### PR-437C：会话持久化与恢复

- `432` append-only message log / conversation lifecycle
- `431` 恢复入口与列表 UI

### PR-437D：压缩最小闭环

- `434` manual compact + pre-turn auto compact + reinjection
- `431` `/compact` UI 入口

当前状态：已具备正式封板条件。

### PR-437E：管理面与权限闭环

- `435` provider/config/health/credential 页面
- `433` health/validation 运行时语义补齐

当前状态：最小运行态闭环已通过，但仅完成“抽屉内 settings 弹窗 + 最小运行时配置表单”这一首轮落点；完整管理面 IA 与四类角色权限矩阵仍未完成，因此当前应记为“最小运行态闭环已通过，权限矩阵与完整管理面未封板”。

## 7. 测试与验收

### 7.1 分阶段验收口径

- **Phase B**：组件测试 + 前端交互测试 + 本地可控 SSE/E2E
- **Phase C**：API/service 测试 + reconstruction fixture + 恢复 E2E
- **Phase D**：compaction 纯函数测试 + prompt shape snapshot + 权限/租户 reinjection 测试
- **Phase E**：Authz test + 配置验证 fixture + 管理面 E2E

### 7.2 快速开工阶段的 required gate

首轮 required gate 只要求：

1. `make check chat-surface-clean`
2. 命中文档时 `make check doc`
3. 命中前端时 `pnpm --dir apps/web check`
4. 命中 Go 时 `go fmt ./... && go vet ./... && make check lint && make test`
5. 命中 routing/authz/db/sqlc 时按各子计划触发相应门禁

### 7.3 明确不作为首轮阻断项

- 真实外部模型调用
- fallback/failover
- quota
- route alias / default model
- remote compaction

## 8. Stopline

- 不得把“快速开工”解释为绕过 `430` 的 no-legacy、单前端主链、服务端网关、append-only 审计与当前用户权限边界。
- 不得在 `436` 清场门禁未更新前新增 `modules/cubebox` 活体代码。
- 不得要求 `431-435` 所有上游映射表全量冻结完才允许写首轮代码。
- 不得跳过共享 canonical contract，直接让 `431/432/434` 各自实现接近但不同的 conversation/turn/event 语义。
- 不得让 `435` 管理面先于 `433` 运行时对象命名抢跑。
- 不得为抽屉形态和页面形态分别实现第二套 store/reducer/SSE 消费链。
- 不得把真实外部 provider 连通性当作 CI 阻断项。

## 9. readiness 与证据要求

- 新增 `DEV-PLAN-437-READINESS.md`，按阶段记录：
  - 本阶段命中的子计划
  - 相关 PR / commit
  - 实际执行命令
  - 结果与截图 / fixture / E2E 证据
  - 未完成项与后续 owner
- 每个阶段完成后回填 `430` 与对应子计划的“回链/封板”章节，避免路线图与 owner 计划脱节。

## 10. 关联文档

- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- `docs/dev-plans/431a-cubebox-page-design-contract.md`
- `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`
- `docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`
- `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
