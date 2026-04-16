# DEV-PLAN-380F：LibreChat vendored/runtime/deploy 资产退役与收口

**状态**: 规划中（2026-04-17 07:23 CST；基于 `380A/380B/380C/380E` 已完成、`380D` 主链已切且仅剩少量 file-plane 收尾的最新状态，按 `DEV-PLAN-001` T2 模板重写细化）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `LibreChat` vendored Web UI、runtime、部署链、Makefile/debug 入口与相关历史资产退役的实施 SSOT。  
> `380A` 持有 PostgreSQL 数据面 contract，`380B` 持有后端正式实现面切换，`380C` 持有 API/DTO 收口与旧 `/internal/assistant/*` 退役，`380D` 持有文件面正式化，`380E` 持有 `apps/web` 正式前端收口，`380G` 持有最终回归与封板。  
> 本文只裁决“哪些 LibreChat 历史资产仍应保留为退役解释层/历史证据，哪些必须从仓库正式运行基线、脚本入口、构建链、文档说明与调试技能中删除或改口”。

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：在 `CubeBox` 前后端主链已稳定、旧 assistant formal entry 已冻结退役的前提下，把仓库内仍以 LibreChat 为名暴露的 vendored 源码、runtime baseline、部署脚本、Makefile 帮助入口、README 与技能说明，从“正式运行/调试基线”收口为“删除、归档或仅保留退役解释层”三类。
- **关联模块/目录**：
  - `Makefile`
  - `deploy/librechat/**`
  - `scripts/librechat/**`
  - `scripts/librechat-web/**`
  - `third_party/librechat-web/**`
  - `internal/server/assets/librechat-web/**`
  - `internal/server/cubebox_retired.go`
  - `internal/server/assistant_ui_proxy.go`
  - `internal/server/assistant_runtime_status.go`
  - `config/routing/allowlist.yaml`
  - `config/errors/catalog.yaml`
  - `apps/web/src/errors/presentApiError.ts`
  - `tools/codex/skills/bugs-and-blossoms-dev-login/SKILL.md`
- **关联计划/标准**：
  - `AGENTS.md`
  - `docs/dev-plans/000-docs-format.md`
  - `docs/dev-plans/001-technical-design-template.md`
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/017-routing-strategy.md`
  - `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
  - `docs/dev-plans/360-librechat-depower-and-langgraph-langchain-layered-takeover-plan.md`
  - `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
  - `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
  - `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/dev-plans/380d-cubebox-file-plane-formalization-plan.md`
  - `docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
  - `docs/dev-plans/380g-cubebox-regression-gates-and-final-closure-plan.md`
- **用户入口/触点**：
  - `make help`
  - `tools/codex/skills/bugs-and-blossoms-dev-login/SKILL.md`
  - `/app/assistant/librechat`
  - `/assets/librechat-web/**`
  - `/assistant-ui/*`
  - `/internal/assistant/runtime-status`

### 0.1 Simple > Easy 三问

1. **边界**：`380F` 只处理“旧资产是否仍作为正式基线暴露”的问题；`380C` 持有旧 API 退役 contract，`380E` 持有 `apps/web` 正式前端，`380D` 持有文件面主链，`380G` 持有最终封板。
2. **不变量**：
   - 仓库只能有一条正式产品主链：`apps/web + /internal/cubebox/* + modules/cubebox`
   - `/app/assistant/librechat`、`/assets/librechat-web/**`、`/assistant-ui/*` 只能表现为退役态，不得重新成为可操作正式入口
   - 不允许继续通过 `Makefile`、README、skill、runbook 把 LibreChat runtime 或 vendored build 暗示为当前推荐调试入口
   - 退役解释层必须稳定返回 `410 Gone`，不能因资产清理误删而退化成 404/500/无语义
3. **可解释**：作者必须能在 5 分钟内讲清当前残留资产分类、为什么删/保留/归档、删后谁继续承接 `410 Gone` 退役 contract，以及 `380F` 如何把结果交给 `380G` 封板。

### 0.2 现状研究摘要

- **现状实现**：
  - `380C` 已完成 `/internal/cubebox/*` 正式 API/DTO 收口，旧 `/internal/assistant/ui-bootstrap`、`/session*`、`model-providers*` 已统一冻结为稳定 `410 Gone`。
  - `380E` 已完成 `apps/web` 收口：正式页面入口为 `/app/cubebox*`；`/app/assistant/librechat` 只剩退役语义，不再是页面主链。
  - 服务端当前通过 `internal/server/cubebox_retired.go` 与 `internal/server/assistant_ui_proxy.go` 为 `/app/assistant/librechat`、`/assets/librechat-web/**`、`/assistant-ui/*` 返回稳定 `410 Gone`。
  - 但 `Makefile` 仍暴露 `assistant-runtime-*`、`librechat-web-*` 帮助入口；`deploy/librechat/**`、`scripts/librechat/**`、`scripts/librechat-web/**`、`third_party/librechat-web/**`、`internal/server/assets/librechat-web/**` 仍保留为活体资产；开发 skill 仍把 LibreChat runtime 作为可选联调基线。
  - `internal/server/assistant_runtime_status.go` 仍读取 `deploy/librechat/versions.lock.yaml` 与 `deploy/librechat/runtime-status.json` 生成 `/internal/assistant/runtime-status` 的诊断输出。
- **现状约束**：
  - `360/360A` 已冻结“旧入口退役优先于长期兼容”的原则，且明确 `/assistant-ui/*` Phase 4 后必须退役，不得回流。
  - `380G` 仍需要旧路径负向断言，因此 `380F` 不能把退役 handler 与错误码清理得比 contract 更快。
  - `380D` 仍有 file-plane 少量收尾，但已足以证明 LibreChat 资产不再承担正式文件职责。
  - 仓库仍有大量 archive/dev-record 历史证据包含 `librechat` 字样，它们应作为归档保留，而不是主干入口。
- **最容易出错的位置**：
  - 把“仍需保留的退役解释层”与“必须下架的正式运行入口”混为一谈，导致误删。
  - 只删入口文案，不删 Makefile/skill/debug 说明，继续形成事实上的第二运行基线。
  - 只删目录，不更新 `runtime-status`、allowlist、错误码与负向断言，导致退役 contract 断裂。
  - 用“先留着以后再说”的方式让 `deploy/librechat`、`third_party/librechat-web` 长期滞留主干。
- **本次不沿用的“容易做法”**：
  - 不通过“保留但不再提”来处理历史资产。
  - 不把 `410 Gone` 退役 contract 一起删除，寄希望于 404 兜底。
  - 不继续保留 `Makefile` 的 LibreChat runtime/build 入口作为“调试方便”。
  - 不把 archive/dev-record 中的历史证据误当成当前活体 runbook。

## 1. 背景与上下文（Context）

- **需求来源**：
  - 用户要求“根据 `380A` 至 `380E` 的完成重新评估是否需要调整 `380F`”，并进一步要求“根据 `001` 指引先细化该方案”。
  - `380C` 已明确：只有其完成后，`380F` 才能安全删除旧 runtime/deploy/vendored 资产。
- **当前痛点**：
  - 旧版 `380F` 仅是 2026-04-14 的骨架草案，没有反映 `380C/380E` 已完成这一新事实。
  - 仓库虽然已经把用户正式入口切到 `CubeBox`，但内部仍通过帮助入口、技能说明和部署资产保留了一条“看起来还能用”的 LibreChat 调试基线。
  - 若不收口这些入口，团队后续会继续误把 LibreChat 资产当作活体依赖，造成文档、脚本、技能和封板口径漂移。
- **业务价值**：
  - 让仓库主干只剩 `CubeBox` 一方产品面，减少 onboarding 与后续维护时的语义误导。
  - 为 `380G` 提供清晰封板输入：哪些资产已删除，哪些仅剩退役解释层，哪些进入归档。
  - 避免未来有人因为“调试方便”恢复第二前端/第二 runtime 基线。
- **仓库级约束**：
  - No Legacy：不保留长期 legacy 别名、双主链、双运行基线。
  - 单前端链路：正式 UI 只允许 `apps/web`。
  - 单 API 命名空间：正式 API 只允许 `/internal/cubebox/*`。
  - 文档收敛：活体文档必须可发现、可维护，不允许保留失真的运行说明。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 核心目标

- [ ] 把 LibreChat 从仓库的正式运行基线、正式构建链、正式调试入口中移除。
- [ ] 冻结残留资产的唯一分类：删除、退役解释层保留、归档保留。
- [ ] 收口 `Makefile`、README、skill、runbook 与相关说明，统一改口为 `CubeBox` 唯一正式主链。
- [ ] 保证旧入口仍稳定表现为 `410 Gone`，不出现 contract 断裂。
- [ ] 向 `380G` 输出资产去向清单、负向断言清单与 readiness 证据。

### 2.2 非目标（Out of Scope）

- 不在本文重新设计 `CubeBox` 数据面、文件面、后端主链或页面功能。
- 不在本文接管 `380D` 尚未关闭的 file-plane 收尾问题。
- 不在本文要求删除所有历史 archive/dev-record 中的 LibreChat 证据。
- 不在本文立刻物理删除所有退役 handler / retired error code；若其仍承担稳定退役 contract，则允许暂留到 `380G` 之前的零行为差异清理。
- 不在本文扩张 Assistant/Knowledge runtime 的新能力或新平台依赖。

### 2.3 用户可见性交付

- **用户可见入口**：
  - `make help` 中的常用入口说明
  - 本仓开发 skill / runbook
  - 浏览器访问 `/app/assistant/librechat`、`/assistant-ui/*`
- **最小可操作闭环**：
  - 开发者或用户查看当前仓库入口说明时，只会被引导到 `CubeBox` 正式链路
  - 若访问旧 LibreChat 入口，只会看到稳定退役语义
  - 不会再有任何说明引导其去启动 LibreChat runtime 作为当前正式联调前提
- **后端先行/非僵尸说明**：
  - 本计划不是“只有文档没有行为”的僵尸收口，因为它直接约束 `Makefile`、服务端退役路由、skills、README 与封板负向断言。

## 2.4 工具链与门禁（SSOT 引用）

- **命中触发器（勾选）**：
  - [ ] Go 代码
  - [ ] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [ ] DB Schema / Migration / Backfill / Correction
  - [ ] sqlc
  - [x] Routing / allowlist / responder / capability-route-map
  - [ ] AuthN / Tenancy / RLS
  - [ ] Authz（Casbin）
  - [x] E2E
  - [x] 文档 / readiness / 证据记录
  - [x] 其他专项门禁：`no-legacy`、`error-message`
- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/017-routing-strategy.md`
  - `docs/dev-plans/360a-librechat-feature-disablement-and-runtime-cutover-plan.md`
  - `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `internal/server` | 退役 handler、旧入口 `410 Gone` 行为、retired error envelope | `internal/server/handler_test.go`、`internal/server/librechat_web_ui_test.go`、`internal/server/assistant_ui_proxy_test.go` | 只验证退役 contract，不把它当活体 UI |
| `apps/web/src/errors/**` | retired error code 到用户提示的映射仍可解释 | `apps/web/src/errors/presentApiError.test.ts` | 保证前端消费退役语义不退化 |
| `E2E` | `/app/assistant/librechat`、`/assistant-ui/*`、`/assets/librechat-web/**` 负向断言；`/app/cubebox` 正向断言 | `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js` 与 `380G` 汇总用例 | 作为最终封板负向证据 |
| 文档/技能 | Makefile 帮助、README、skill、runbook 是否仍引导旧链路 | readiness + `make check doc` | 文档行为即交付对象 |

- **黑盒 / 白盒策略**：
  - 退役入口优先黑盒：只看 HTTP status / error code / path contract。
  - 不为历史 vendored 构建脚本补新白盒测试；这些资产目标是退出主干，而不是继续增强。
- **并行 / 全局状态策略**：
  - 触碰全局 env、路由、help 输出或旧入口断言的测试保持串行或现有隔离。
  - 文档检查由 `make check doc` 统一承担。
- **fuzz / benchmark 适用性**：
  - 本计划聚焦资产退役与入口收口，不额外补 fuzz / benchmark。
- **前端测试原则**：
  - 仅保留退役错误码和旧入口解释所必需的测试；不再为 LibreChat 旧页面/旧 runtime 增加页面级测试。

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 5 分钟主流程

```mermaid
flowchart LR
  DEV[Developer / Reviewer] --> DOC[Makefile / README / Skill / Dev Plan]
  DOC --> FORMAL[/app/cubebox + /internal/cubebox/*]
  DOC -.old refs removed.-> RETIRED[/app/assistant/librechat / assistant-ui / assets/librechat-web]
  RETIRED --> GONE[410 Gone retired handlers]
  FORMAL --> G[380G final regression]
```

- **主流程叙事**：
  - 开发者查看仓库帮助入口、技能或 runbook 时，只会看到 `CubeBox` 正式主链。
  - 若误访问旧 LibreChat 路由，服务端统一返回 `410 Gone`，前端错误映射给出“已退役，请改用 CubeBox 正式入口”。
  - `380F` 通过清理帮助入口与旧资产说明，让主干不再把 LibreChat 当作 current baseline。
- **失败路径叙事**：
  - 若清理过程中误删退役 handler 或 allowlist，旧入口会退化为不稳定 404/500，这属于 stopline。
  - 若只删了文档，没有删 Makefile/skill/debug 说明，团队仍会继续误用旧基线，这也属于 stopline。
- **恢复叙事**：
  - 若某步清理导致退役 contract 断裂，恢复策略应是前向修复退役 handler / allowlist / error mapping，而不是恢复 LibreChat runtime/build 入口。

### 3.2 模块归属与职责边界

- **owner module**：`380F` 没有单一业务模块 owner；它是仓库级资产退役计划，由文档、脚本、服务端退役解释层与开发入口共同承接。
- **交付面**：
  - `Makefile` 帮助入口
  - `deploy/librechat/**`、`scripts/librechat/**`、`scripts/librechat-web/**`
  - `third_party/librechat-web/**` 与 `internal/server/assets/librechat-web/**`
  - `internal/server` 退役 handler
  - skills / README / 活体 dev-plan
- **跨模块交互方式**：
  - 本计划不引入新的业务模块交互；只消费 `380C/380E` 的完成态作为输入。
- **组合根落点**：
  - `internal/server/handler.go` 继续是退役入口组合点。
  - `Makefile` 继续是仓库级帮助与执行入口的唯一汇总点；`380F` 要求把其中的 LibreChat 基线入口下架。

### 3.3 落地形态决策

- **形态选择**：
  - [x] `A. Go DDD`
  - [ ] `B. DB Kernel + Go Facade`
- **选择理由**：
  - `380F` 不是业务写路径收口，也不涉及 DB Kernel；它主要是仓库入口、文档和退役 handler 的整理。
  - 这里的“owner”不是业务领域聚合，而是仓库级 contract。

### 3.4 ADR 摘要

- **决策 1：把残留资产分为“删除 / 退役解释层保留 / 归档保留”三类，而不是一刀切全删**
  - **备选 A**：全部立即物理删除。缺点：容易连 `410 Gone` 退役 contract 一起删掉，破坏 `380G` 负向断言。
  - **备选 B**：全部保留但不再提。缺点：继续形成事实上的第二运行基线，违背 `No Legacy`。
  - **选定理由**：三类模型最简单，也最符合当前主链已经切完但封板未结束的状态。

- **决策 2：先下架正式入口，再考虑物理删目录**
  - **备选 A**：先删目录，再补文档和帮助入口。缺点：容易造成说明、测试和退役 contract 断裂。
  - **备选 B**：先改口为 historical only，再无限期保留目录。缺点：主干残留会长期漂移。
  - **选定理由**：先切断“主干可见入口”，再做物理删改，能让风险最小、审计最清楚。

- **决策 3：`/internal/assistant/runtime-status` 与 `deploy/librechat/runtime-status.json` 视为待收口对象，而不是长期保留能力面**
  - **备选 A**：继续把 runtime-status 作为现行开发入口。缺点：继续放大历史 runtime 基线的重要性。
  - **备选 B**：立即删除 runtime-status 相关一切路径。缺点：若相邻计划仍需负向或历史诊断信息，风险过大。
  - **选定理由**：先在 `380F` 中明确其不再是正式运行面，再由实施批次决定是下架、改名为 historical diagnostics，还是在 `380G` 前一并清除。

### 3.5 Simple > Easy 自评

- **这次保持简单的关键点**：
  - 不引入“保留但默认隐藏”的灰色状态，而是明确三类资产去向。
  - 不把退役 handler 与旧 runtime/build 入口混在一个完成定义里。
  - 不让 `380F` 越权去修改 `380C/380D/380E` 已冻结的主链 contract。
- **明确拒绝的“容易做法”**：
  - [x] legacy alias / 双链路 / fallback
  - [x] 第二写入口 / controller 直写表
  - [x] 页面内自造第二套 object/action/capability 拼装
  - [x] 为过测临时加死分支或兼容层
  - [x] 复制一份旧页面/旧 DTO/旧 store 继续改

## 4. 数据模型、状态模型与约束（Data / State Model & Constraints）

### 4.1 数据结构定义

- 本计划不新增业务表、DTO 主字段或 schema。
- 只关注以下“资产清单”型结构：
  - 运行入口清单：`Makefile` / skill / README / runbook 中对 LibreChat 的暴露点
  - 退役解释层清单：allowlist path、retired error code、retired handler
  - 历史归档清单：`docs/archive/**`、`docs/dev-records/**`、上游来源元数据与历史证据

### 4.2 时间语义与标识语义

- 本计划不涉及 `effective_date` / `as_of` / 业务日粒度时间。
- 所有 readiness 证据仍需记录实际时间戳。
- 退役错误码与路径 literal 必须保持稳定：
  - `assistant_ui_retired`
  - `assistant_vendored_api_retired`
  - `librechat_retired`

## 5. 路由、UI 与 API 契约（Route / UI / API Contracts）

### 5.1 交付面与路由对齐表

| 交付面 | Canonical Path / Route | `route_class` | owner module | Authz object/action | capability / route-map | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| UI 页面 | `/app/cubebox` | `ui` | `apps/web / cubebox` | 以正式 UI 为准 | N/A | 唯一正式聊天入口 |
| UI retired path | `/app/assistant/librechat` | `ui` | retired handler | N/A | N/A | 稳定 `410 Gone`，不是 alias |
| UI retired path | `/app/assistant/librechat/{path}` | `ui` | retired handler | N/A | N/A | 稳定 `410 Gone` |
| static retired path | `/assets/librechat-web/**` | `static` | retired handler | N/A | N/A | 稳定 `410 Gone` |
| UI retired path | `/assistant-ui` | `ui` | retired handler | N/A | N/A | 稳定 `410 Gone` |
| UI retired path | `/assistant-ui/{path}` | `ui` | retired handler | N/A | N/A | 稳定 `410 Gone` |
| internal API | `/internal/assistant/runtime-status` | `internal_api` | assistant retired diagnostics | N/A | N/A | 待 `380F` 判断是否下架、改口或延后清理 |

- **要求**：
  - `CubeBox` 相关正式路径不得回流为 `assistant/librechat`。
  - 退役路径必须继续被 allowlist 与 responder 稳定治理。
  - 若移除 `/internal/assistant/runtime-status` 之类历史诊断接口，必须同步更新调用方、文档和断言。

### 5.2 `apps/web` 交互契约

- **页面/组件入口**：
  - 正式入口只允许 `/app/cubebox*`
  - 旧路径不再绑定 `apps/web` 页面组件
- **数据来源**：
  - 旧路径仅由服务端 retired handler 返回错误 envelope
- **状态要求**：
  - 访问 `/app/assistant/librechat` 时看到明确退役语义，而不是重定向、白屏或旧页面
  - 前端对 `librechat_retired`、`assistant_ui_retired`、`assistant_vendored_api_retired` 保持明确映射
- **i18n**：
  - 仅允许 `en/zh`
  - 退役错误码文案必须继续可解释
- **禁止**：
  - 在 `apps/web` 中重新引入 `/app/assistant/librechat` 页面
  - 在前端把 retired error 解读为“请启动 LibreChat runtime”

### 5.3 JSON API 契约

本计划不新增正式 JSON API，只冻结历史诊断与退役行为边界：

#### 5.3.1 `GET /internal/assistant/runtime-status`

- **用途**：历史 runtime baseline 诊断输出；不属于 `CubeBox` 正式产品 API。
- **owner module**：历史 assistant/runtime diagnostics
- **route_class**：`internal_api`
- **完成态约束**：
  - 不得再被 Makefile、skill、README 作为正式联调前提推荐
  - 若保留，必须在文档中明确为 historical diagnostics，而非正式运行面
  - 若删除或改口，必须有等价的 `380F` readiness 记录与 `380G` 负向断言调整

### 5.4 失败语义 / stopline

| 失败场景 | 正式错误码 | 是否允许 fallback | explain 最低输出 | 是否 stopline |
| --- | --- | --- | --- | --- |
| 访问 `/app/assistant/librechat*` | `librechat_retired` | 否 | `code/message/meta.path/meta.method` | 是 |
| 访问 `/assistant-ui*` | `assistant_ui_retired` | 否 | `code/message/meta.path/meta.method` | 是 |
| 访问已退役 vendored compat API | `assistant_vendored_api_retired` | 否 | `code/message/meta.path/meta.method` | 是 |
| 文档/Makefile/skill 仍引导 `make assistant-runtime-up` 或 `make librechat-web-build` | N/A | 否 | readiness 记录中必须列出残留入口 | 是 |

- **错误码约束**：
  - canonical retired error code 只保留一套；
  - 不允许把 retired path 退化为模糊 404/500；
  - 不允许通过恢复旧 runtime/build 入口作为“修复旧路径访问问题”的 fallback。

## 6. 核心流程与算法（Business Flow & Algorithms）

### 6.1 读路径主算法

1. 开发者查看仓库帮助入口、README、skill 或 runbook。
2. 系统只暴露 `CubeBox` 正式主链，不再暴露 LibreChat runtime/build 作为推荐步骤。
3. 若开发者或用户访问旧路径：
   - `/app/assistant/librechat*` -> `newLibreChatRetiredHandler()`
   - `/assistant-ui*` -> `newAssistantUIRetiredHandler()`
   - `/assets/librechat-web/**` -> `newLibreChatRetiredHandler()`
4. 响应统一返回 `410 Gone + retired code`。

### 6.2 写路径主算法

本计划不新增业务写路径；其“写”主要是仓库资产收口：

1. 盘点当前活体 LibreChat 入口。
2. 将每一项标记为：删除 / 退役解释层保留 / 归档保留。
3. 对“删除/下架”的入口更新 `Makefile`、README、skill、runbook。
4. 对“退役解释层保留”的入口保持 handler / allowlist / error mapping 稳定。
5. 对“归档保留”的资产明确 historical only 语义。

### 6.3 幂等、回放与恢复

- **幂等键**：本计划不涉及业务幂等键。
- **恢复策略**：
  - 若资产下架导致旧路径语义断裂，前向修复 retired handler / allowlist / error code；
  - 不恢复 `assistant-runtime-*`、`librechat-web-*` 作为默认主干入口；
  - 不通过回退到 LibreChat runtime baseline 替代修复。

## 7. 安全、租户、授权与运行保护（Security / Tenancy / Authz / Recovery）

### 7.1 AuthN / Tenancy

- 旧 UI 路径仍经过现有 `withTenantAndSession(...)` 护栏。
- 退役 handler 会记录 `tenant_id`、`request_id`、`trace_id` 等最小审计信息。
- 本计划不新增新的 tenant 解析方式。

### 7.2 Authz

- 退役路径不再承担正式业务权限逻辑。
- 正式授权继续由 `CubeBox` 主链持有；`380F` 只要求旧路径不回流为可授权正式入口。

### 7.3 运行保护

- 不引入“临时恢复 LibreChat runtime/build 入口”的开关。
- 故障处置优先：保留退役 contract -> 修复文档/入口漂移 -> 前向清理资产 -> 最终封板。
- 若某目录短期保留，只能以 “historical only / not a runtime baseline” 口径存在。

## 8. 依赖、切片与里程碑（Dependencies & Milestones）

### 8.1 前置依赖

- `380B` 已完成，后端正式主链足以支撑 `380F`。
- `380C` 已完成，旧 API 退役 contract 已冻结。
- `380E` 已完成，正式前端入口已收口。
- `380D` 主链已切，但少量 file-plane 收尾仍待关闭；不阻塞 `380F` 启动。

### 8.2 建议实施切片

1. [ ] **Contract Slice**：冻结资产分类清单、退役解释层清单与 stopline。
2. [ ] **Build / Runtime Slice**：下架 `Makefile` 中的 LibreChat runtime/build 入口，收口 `deploy/librechat/**` 与 `scripts/librechat/**` 的主干地位。
3. [ ] **Docs / Skill Slice**：更新 README、skills、相关 dev-plan 引用，统一到 `CubeBox` 正式主链。
4. [ ] **Test & Gates Slice**：复核退役断言、`error-message`、`make check doc` 与必要 E2E 负向断言。
5. [ ] **Readiness Slice**：形成 `DEV-PLAN-380F-READINESS.md`，并向 `380G` 交接。

### 8.3 每个切片的完成定义

- **Contract Slice**
  - **输入**：`380C/380E` readiness 已可引用
  - **输出**：删除/保留/归档三类资产表已冻结
  - **阻断条件**：若资产去向仍含糊，禁止开始物理清理

- **Build / Runtime Slice**
  - **输入**：资产分类已明确
  - **输出**：`Makefile`/脚本/部署说明不再把 LibreChat 当作正式 baseline
  - **阻断条件**：若会误删退役 contract 或 break 现有负向断言，必须暂停

- **Docs / Skill Slice**
  - **输入**：Build/Runtime 入口已收口
  - **输出**：所有活体说明统一改口
  - **阻断条件**：若仍有一个活体文档或 skill 推荐旧链路，视为未完成

- **Test & Gates Slice**
  - **输入**：代码/文档收口完成
  - **输出**：退役断言稳定，专项门禁通过
  - **阻断条件**：若旧路径只剩模糊 404/500，则不得进入 readiness

- **Readiness Slice**
  - **输入**：上述切片完成
  - **输出**：`380F` readiness 与 `380G` 交接清单
  - **阻断条件**：若没有资产去向清单与负向断言清单，禁止宣告完成

## 9. 测试、验收与 Readiness（Acceptance & Evidence）

### 9.1 验收标准

- **边界验收**：
  - [ ] `380F` 只处理旧资产退役，不越权修改 `380C/380D/380E` 主链 contract
  - [ ] 退役解释层与正式入口删除面已清晰分离

- **用户可见性验收**：
  - [ ] 开发者查看仓库入口说明时，只会被引导到 `CubeBox`
  - [ ] 用户访问旧入口时，仍能得到明确退役提示

- **数据 / 时间 / 租户验收**：
  - [ ] 旧入口退役仍经过 tenant/session 边界，不出现旁路
  - [ ] retired handler 审计信息仍可记录最小租户/请求上下文

- **UI / API 验收**：
  - [ ] `/app/cubebox` 仍是唯一正式 UI 入口
  - [ ] `/app/assistant/librechat`、`/assistant-ui/*`、`/assets/librechat-web/**` 稳定 `410 Gone`
  - [ ] `/internal/assistant/runtime-status` 若保留，已被明确定义为历史诊断而非正式运行面

- **测试与门禁验收**：
  - [ ] `make check doc` 通过
  - [ ] `error-message` 口径未被破坏
  - [ ] 负向 E2E 断言仍可复用给 `380G`
  - [ ] 没有通过保留第二运行基线来“降低清理风险”

### 9.2 Readiness 记录

- [ ] 新建 `docs/dev-records/DEV-PLAN-380F-READINESS.md`
- [ ] 在 readiness 中记录：
  - 时间戳
  - 删除/保留/归档清单
  - 实际执行入口
  - 结果
  - 负向断言与关键日志/截图/命令摘要
- [ ] 本文档不复制执行输出；只链接 readiness 证据

### 9.3 例外登记

- **白盒保留理由**：不新增针对历史 vendored 源码的白盒测试，因为目标是退出主干而非增强该实现。
- **暂不并行理由**：文档、Makefile、全局入口与退役路由断言具有共享上下文，避免并行引入噪声。
- **不补 fuzz / benchmark 理由**：本计划不是解析/高频性能类问题。
- **暂留 `internal/server` 测试 理由**：退役 handler 与旧入口 contract 仍由服务端交付，必须继续有最小黑盒断言。
- **暂不能下沉到 `services` / `pkg/**` 的理由**：本计划主要是仓库入口与退役 contract 整理，不是业务域逻辑。
- **若删除死分支/旧链路**：必须说明该入口已失去正式职责，且对外行为由稳定 `410 Gone` 维持不变。

## 10. 附：作者自检清单（可复制到评审评论）

- [ ] 我已经写清边界、owner、不变量、失败路径与验收，不是只写“要做什么”
- [ ] 我已经写清为什么这是“更简单”，而不是“只是更容易先塞进去”
- [ ] 我没有把关键契约藏到实现里，也没有预设 legacy/fallback/第二运行基线
- [ ] 我已经说明本次命中的门禁入口与证据落点，但没有复制命令矩阵
- [ ] 我已经说明开发者/用户现在应如何发现并使用正式入口，以及旧入口为何只剩退役态
- [ ] 我已经为 reviewer 提供 5 分钟可复述的主流程与失败路径
