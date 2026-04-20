# DEV-PLAN-380F：LibreChat vendored/runtime/deploy 资产退役与收口

**状态**: 已实施（2026-04-17 CST；实施结果与验证记录见 `docs/archive/dev-records/DEV-PLAN-380F-READINESS.md`）

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

### 0.2 实施后现状摘要

- **现状实现**：
  - `380C` 已完成 `/internal/cubebox/*` 正式 API/DTO 收口，旧 `/internal/assistant/ui-bootstrap`、`/session*`、`model-providers*` 已统一冻结为稳定 `410 Gone`。
  - `380E` 已完成 `apps/web` 收口：正式页面入口为 `/app/cubebox*`；`/app/assistant/librechat` 只剩退役语义，不再是页面主链。
  - 服务端当前通过 `internal/server/cubebox_retired.go` 与 `internal/server/assistant_ui_proxy.go` 为 `/app/assistant/librechat`、`/assets/librechat-web/**`、`/assistant-ui/*` 返回稳定 `410 Gone`。
  - `Makefile` 中 `assistant-runtime-*`、`librechat-web-*` 入口已下架；开发 skill 已停止把 LibreChat runtime 作为联调基线。
  - `internal/server/assistant_runtime_status.go` 已切断默认 `deploy/librechat/**` 读取；旧 `/internal/assistant/runtime-status` 只保留退役解释语义。
  - capability / route-map / allowlist 已同步移除 `/internal/assistant/runtime-status` 的正式活体地位。
  - `/app/assistant/librechat/api/*` 与 `/assets/librechat-web/api/*` 已统一收敛到 `librechat_retired`，不再保留 `assistant_vendored_api_retired` 作为活体 contract。
  - `tp220/tp283` 已作为负向退役断言保留；`tp288/tp290` 已明确归入历史 skip，不纳入 `380G` 活体封板主链。
- **现状约束**：
  - `360/360A` 已冻结“旧入口退役优先于长期兼容”的原则，且明确 `/assistant-ui/*` Phase 4 后必须退役，不得回流。
  - `380G` 仍需要旧路径负向断言，因此 `380F` 不能把退役 handler 与错误码清理得比 contract 更快。
  - `380D` 仍有 file-plane 少量收尾，但已足以证明 LibreChat 资产不再承担正式文件职责。
  - 仓库仍有大量 archive/dev-record 历史证据包含 `librechat` 字样，它们应作为归档保留，而不是主干入口。
- **最容易出错的位置**：
  - 把“仍需保留的退役解释层”与“必须下架的正式运行入口”混为一谈，导致误删。
  - 只删入口文案，不删 Makefile/skill/debug 说明，继续形成事实上的第二运行基线。
  - 只删目录，不更新 `runtime-status`、allowlist、错误码与负向断言，导致退役 contract 断裂。
  - 只在文档里写“历史诊断”而不裁决 `/internal/assistant/runtime-status` 的 capability/route-map/allowlist 去向，导致旧 assistant internal API 继续被视为正式活体入口。
  - 把 vendored compat API 的 retired code 写成一套、实现继续返回另一套，导致 `380G` 按错 contract 封板。
  - 忽略仍访问 `/app/assistant/librechat` 正向 UI 的活体 E2E，导致 `380F` 实施后回归集自爆。
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

- [x] 把 LibreChat 从仓库的正式运行基线、正式构建链、正式调试入口中移除。
- [x] 冻结残留资产的唯一分类：删除、退役解释层保留、归档保留。
- [x] 冻结 `/internal/assistant/runtime-status` 的唯一完成态，并同步裁决 capability / allowlist / docs / tests 的跟随动作。
- [x] 收口 `Makefile`、README、skill、runbook 与相关说明，统一改口为 `CubeBox` 唯一正式主链。
- [x] 保证旧入口仍稳定表现为 `410 Gone`，不出现 contract 断裂。
- [x] 向 `380G` 输出资产去向清单、负向断言清单与 readiness 证据。

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
  - [x] Go 代码
  - [x] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [ ] DB Schema / Migration / Backfill / Correction
  - [ ] sqlc
  - [x] Routing / allowlist / responder / capability-route-map
  - [ ] AuthN / Tenancy / RLS
  - [x] Authz（Casbin）
  - [x] E2E
  - [x] 文档 / readiness / 证据记录
  - [x] 其他专项门禁：`no-legacy`、`error-message`
- **门禁执行原则**：
  - 只要 `380F` 触碰 `internal/server/**`、`config/routing/**`、`config/capability/**`、`Makefile`、`apps/web/src/errors/**` 或 E2E 断言，就必须把对应门禁写入 readiness，不能因为本文是“资产退役”就省略 Go / 前端 / Authz / capability 验证。
  - 若本批次未修改某类文件，可在 readiness 中标注“未命中”；但计划阶段不得把已知高概率命中的门禁静态写成未触发。
- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/017-routing-strategy.md`
  - `docs/dev-plans/022-authz-casbin-toolchain.md`
  - `docs/dev-plans/140-error-message-clarity-and-gates.md`
  - `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
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
| `E2E` | `/app/assistant/librechat`、`/assistant-ui/*`、`/assets/librechat-web/**` 负向断言；`/app/cubebox` 正向断言；旧 LibreChat 正向证据用例去向 | `e2e/tests/tp220-assistant.spec.js`、`e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`、`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`、`e2e/tests/tp290-librechat-real-case-matrix.spec.js` | `380F` 必须明确保留/迁移/归档，而不是默认全跑 |
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

- **决策 3：`/internal/assistant/runtime-status` 的完成态冻结为退役，不再保留 active capability**
  - **备选 A**：继续把 runtime-status 作为现行开发入口。缺点：继续放大历史 runtime 基线的重要性。
  - **备选 B**：仅在文档里改名为 historical diagnostics，但继续保留 active route capability。缺点：门禁与权限语义仍承认旧 assistant internal API 是活体入口。
  - **备选 C（选定）**：`380F` 中下架旧 `/internal/assistant/runtime-status` 的正式能力面：不再由 Makefile/skill/README 引导；route-capability-map 与 registry 不再把它标成 active；服务端完成态为稳定退役语义或物理删除，二者只能选一并写入 readiness；`/internal/cubebox/runtime-status` 保持唯一正式运行态 API。
  - **选定理由**：这是唯一同时满足 `380C` 单 API 命名空间、`380E` 单前端链路和 `No Legacy` 的方案。若 `380G` 仍需要历史负向断言，应断言旧 assistant runtime-status 不再是 active capability，而不是继续读取 `deploy/librechat/runtime-status.json`。

- **决策 4：vendored compat API retired code 与实际 handler 收敛为 `librechat_retired`**
  - **备选 A**：为 `/app/assistant/librechat/api/*`、`/assets/librechat-web/api/*` 新增专门 handler 返回 `assistant_vendored_api_retired`。缺点：会扩张一条只服务历史 compat API 的新退役分支，增加路由与测试面。
  - **备选 B（选定）**：把 vendored UI 页面、静态资源与其 API 子路径统一视为 LibreChat 入口退役面，全部返回 `410 Gone + librechat_retired`；`assistant_vendored_api_retired` 仅允许作为历史错误码留存到零引用清理批次，不再作为 `380F` canonical contract。
  - **选定理由**：当前服务端已通过 `newLibreChatRetiredHandler()` 统一承接这些路径；继续在文档中写另一套 code 会让 `380G` 按错误 contract 封板。

- **决策 5：活体 E2E 必须显式分流，不允许旧 LibreChat 正向用例进入 `380G` 全量封板**
  - **备选 A**：保留 `tp288/tp290` 等旧正向 UI 用例，依赖测试过滤规避。缺点：全量 E2E 口径不清，后续极易误跑。
  - **备选 B（选定）**：`tp220/tp283` 保留或改写为负向退役断言；仍打开 `/app/assistant/librechat` 并期待 textbox/iframe 的旧正向 UI 用例必须改写到 `/app/cubebox`、迁入 archive/dev-record 证据，或标记为历史 skip/fixme 且不得纳入 `380G` 封板必跑集。
  - **选定理由**：`380F` 的目标是消灭第二 UI/runtime 基线；让旧正向 E2E 继续活在默认回归里，会比保留旧脚本更容易造成语义回流。

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
  - `librechat_retired`
- `assistant_vendored_api_retired` 不再作为 `380F` 的 canonical runtime contract；若代码或 catalog 中仍暂留该错误码，必须在 readiness 中标注为“历史错误码 / 零引用清理候选”，不得用于新增退役路径。

### 4.3 资产分类矩阵（380F Contract）

| 资产 / 路径 | 当前问题 | 380F 完成分类 | 必须执行动作 | 保留原因 / 退出条件 | 验证项 |
| --- | --- | --- | --- | --- | --- |
| `Makefile` 中 `assistant-runtime-*`、`librechat-web-*` help 与 target | 仍把 LibreChat runtime/build 暴露为当前入口 | 删除 / 下架 | 从 `make help` 常用入口删除；target 若暂留必须改名或标注 historical-only，且不得被 skill/README 引用 | 若 target 仅用于历史复盘，可迁 archive/runbook；封板前不得作为默认调试入口 | `make help` 不出现推荐旧 runtime/build；`rg "make assistant-runtime-up|make librechat-web-build"` 只命中 archive/readiness 或 historical-only 说明 |
| `deploy/librechat/**` | 旧 compose/runtime baseline 与 status snapshot | 删除或归档保留 | 默认从主干运行基线移除；若短期保留，必须移入 historical-only 语义并从 runtime-status 读取链路断开 | 退出条件：`/internal/cubebox/runtime-status` 已承接正式运行态，旧目录不再被任何 Makefile/skill/server runtime 读取 | `rg "deploy/librechat"` 不再命中活体运行入口；必要时 readiness 记录删除/归档清单 |
| `deploy/librechat/runtime-status.json`、`deploy/librechat/versions.lock.yaml` | 被 `/internal/assistant/runtime-status` 读取，维持旧 runtime 诊断面 | 删除或归档保留 | 切断服务端默认读取；若保留，仅作为历史证据，不作为 runtime 输入 | 退出条件：旧 assistant runtime-status 退役或物理删除，正式 runtime-status 只走 `/internal/cubebox/runtime-status` | Go 测试覆盖旧接口退役/删除结果；`rg "defaultAssistantRuntime.*deploy/librechat"` 不再作为正式读取路径 |
| `scripts/librechat/**` | 启停/状态/清理旧 runtime | 删除或归档保留 | 从主干可执行工作流下架；不得被 Makefile、skill、README 引用为当前步骤 | 若保留，仅用于 archive 复盘且不纳入 `make help` | `rg "scripts/librechat"` 不再命中活体入口 |
| `scripts/librechat-web/**` | vendored Web UI 构建链 | 删除或归档保留 | 下架 build/verify target；不再生成 `internal/server/assets/librechat-web/**` | 若保留，仅作为历史来源说明；不得作为当前构建链 | `make librechat-web-build` 不再是推荐 target；生成物状态在 readiness 记录 |
| `third_party/librechat-web/**` | vendored upstream 源码与 patch | 删除或归档保留 | 默认从主干正式资产移除；若保留，必须 historical-only 且不被构建链消费 | 退出条件：`apps/web` 已是唯一正式前端，旧源码不再参与构建/测试 | `rg "third_party/librechat-web"` 不再命中活体构建入口 |
| `internal/server/assets/librechat-web/**` | vendored 构建产物仍在仓库内 | 删除 | 删除或停止服务端静态文件对其依赖；旧 `/assets/librechat-web/**` 由 retired handler 返回 `410 Gone` | 仅退役 handler 保留，不保留静态产物 | 访问 `/assets/librechat-web/**` 仍为 `410 + librechat_retired`，不是文件内容 |
| `internal/server/cubebox_retired.go` | 承接 `/app/assistant/librechat*` 与 `/assets/librechat-web/**` 退役语义 | 退役解释层保留 | 保持最小 handler；补足 code 断言；不得恢复旧 UI | `380G` 之前保留以稳定负向断言；最终零行为差异清理另行裁决 | server/E2E 断言 status=410 且 code=`librechat_retired` |
| `internal/server/assistant_ui_proxy.go` | `/assistant-ui*` 退役语义 | 退役解释层保留 | 保持最小 handler 与审计日志；不得恢复 proxy upstream | `380G` 之前保留以稳定负向断言 | server/E2E 断言 status=410 且 code=`assistant_ui_retired` |
| `internal/server/assistant_runtime_status.go` | 旧 assistant runtime-status 仍可读取 LibreChat runtime 文件 | 删除或退役解释层保留 | 选定完成态必须是：旧 `/internal/assistant/runtime-status` 不再 active；实现可删除或返回稳定 retired/gone，但不得继续读取 `deploy/librechat/**` 作为正式诊断 | 若短期保留 handler，只能解释“旧 runtime-status 已退役，请使用 `/internal/cubebox/runtime-status`” | route-capability-map/registry/allowlist 与 Go 测试同步；旧接口不再 active capability |
| `config/routing/allowlist.yaml` | 仍列出旧路径，部分路径需要负向治理 | 退役解释层保留 / 删除跟随 | `/app/assistant/librechat*`、`/assistant-ui*`、`/assets/librechat-web/**` 可保留以保障 `410`；`/internal/assistant/runtime-status` 跟随决策删除或改退役 | 退出条件由 `380G` 是否还需要旧路径负向断言决定 | `make check routing` |
| `config/capability/route-capability-map.v1.json` 与 `internal/server/capability_route_registry.go` | `/internal/assistant/runtime-status` 仍 active | 删除或 retired 化 | 不得继续将旧 assistant runtime-status 标为 active capability；若 handler 保留为 retired，不应映射正式业务 capability | 旧 UI/static retired path 不需要 capability；正式能力只在 `/internal/cubebox/*` | `make check capability-route-map`，必要时 `make authz-*` |
| `config/errors/catalog.yaml`、`apps/web/src/errors/presentApiError.ts` | retired code 仍需解释；`assistant_vendored_api_retired` 可能变成历史 code | 退役解释层保留 / 零引用候选 | `librechat_retired`、`assistant_ui_retired` 保持明确文案；`assistant_vendored_api_retired` 若无产出路径，标注为后续零引用清理候选或同步删除 | 不允许前端提示“启动 LibreChat runtime” | `make check error-message`，`pnpm --dir apps/web check` |
| `tools/codex/skills/bugs-and-blossoms-dev-login/SKILL.md` | 仍推荐可选 LibreChat runtime 与旧正式入口 | 删除 / 改口 | 改为只启动 `CubeBox` 正式链路；旧路径只用于验证退役负向行为 | 不得再描述 `/app/assistant/librechat` 为正式聊天入口 | `rg "assistant-runtime-up|/app/assistant/librechat"` 只允许退役说明 |
| `README` / 活体 runbook / 活体 dev-plan 引用 | 可能继续暗示 LibreChat 是 current baseline | 删除 / 改口 | 活体文档统一改到 `CubeBox`；历史说明迁 archive 或标 historical-only | archive/dev-record 历史证据不清理 | `make check doc` |

### 4.4 E2E 与测试资产分类矩阵

| 测试资产 | 当前语义 | 380F 完成分类 | 必须执行动作 | `380G` 口径 |
| --- | --- | --- | --- | --- |
| `e2e/tests/tp220-assistant.spec.js` | 已包含 `/app/cubebox` 正向与 `/app/assistant/librechat` 退役负向断言 | 保留 / 负向断言 | 保持或微调为 `CubeBox` 正向 + 旧入口 `410` | 可纳入封板必跑集 |
| `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js` | 旧 formal entry cutover 负向断言 | 保留 / 负向断言 | 补 code 断言为 `librechat_retired` / `assistant_ui_retired`；不再期待 vendored UI | 可纳入封板必跑集 |
| `e2e/tests/tp284-librechat-send-render-takeover.prep.spec.js` | 历史 fixme skeleton | 归档或 historical skip | 若仍保留，必须改口为历史计划，不得作为待启用正式用例 | 不纳入封板必跑集 |
| `e2e/tests/tp288-librechat-real-entry-evidence.spec.js` | 打开 `/app/assistant/librechat` 并期待 textbox/iframe 的旧正向证据 | 改写或归档 | 优先迁移核心业务断言到 `/app/cubebox`；无法迁移的历史证据转 archive/dev-record 或标 skip/fixme 并说明 retired | 不得以当前形态纳入封板必跑集 |
| `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js` | 文件名仍含 librechat，但已使用 `/app/cubebox` 与 `/internal/cubebox` | 改名或改口 | 可保留业务断言，但应在 `380F/380G` 中登记重命名或说明历史文件名不代表旧入口 | 可纳入封板必跑集，但 readiness 必须解释命名残留 |
| `e2e/tests/tp290-librechat-real-case-matrix.spec.js` | 打开 `/app/assistant/librechat` 并期待 textbox/iframe 的旧正向矩阵 | 改写或归档 | 迁移仍有价值的真实案例到 `/app/cubebox`；旧入口访问部分改为负向 `410` 或归档 | 不得以当前形态纳入封板必跑集 |

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
| internal API | `/internal/assistant/runtime-status` | `internal_api` | retired / deleted | 不再使用正式业务 capability | 不得为 active | `380F` 完成态必须是不再 active；删除或 `410 Gone` 二选一 |

- **要求**：
  - `CubeBox` 相关正式路径不得回流为 `assistant/librechat`。
  - 退役路径必须继续被 allowlist 与 responder 稳定治理。
  - `/internal/assistant/runtime-status` 不得继续读取 LibreChat runtime 文件作为正式诊断；若保留路由，必须返回稳定退役语义并从 active capability 中移除；若物理删除，必须同步更新 allowlist、route-capability-map、registry、调用方、文档和断言。
  - `/app/assistant/librechat/api/*` 与 `/assets/librechat-web/api/*` 统一归属 `librechat_retired`，不再单独冻结 `assistant_vendored_api_retired` 作为 canonical 响应。

### 5.2 `apps/web` 交互契约

- **页面/组件入口**：
  - 正式入口只允许 `/app/cubebox*`
  - 旧路径不再绑定 `apps/web` 页面组件
- **数据来源**：
  - 旧路径仅由服务端 retired handler 返回错误 envelope
- **状态要求**：
  - 访问 `/app/assistant/librechat` 时看到明确退役语义，而不是重定向、白屏或旧页面
  - 前端对 `librechat_retired`、`assistant_ui_retired` 保持明确映射
  - 若 `assistant_vendored_api_retired` 暂留于 catalog / 前端映射，只能作为历史零引用候选，不得新增调用方
- **i18n**：
  - 仅允许 `en/zh`
  - 退役错误码文案必须继续可解释
- **禁止**：
  - 在 `apps/web` 中重新引入 `/app/assistant/librechat` 页面
  - 在前端把 retired error 解读为“请启动 LibreChat runtime”

### 5.3 JSON API 契约

本计划不新增正式 JSON API，只冻结历史诊断与退役行为边界：

#### 5.3.1 `GET /internal/assistant/runtime-status`

- **用途**：旧 runtime baseline 诊断输出的退役对象；不属于 `CubeBox` 正式产品 API。
- **owner module**：retired assistant/runtime diagnostics
- **route_class**：`internal_api`
- **完成态约束**：
  - 不得再被 Makefile、skill、README 作为正式联调前提推荐。
  - 不得继续作为 active route capability / authz object/action 的正式能力入口。
  - 不得继续读取 `deploy/librechat/runtime-status.json`、`deploy/librechat/versions.lock.yaml` 作为正式运行态输入。
  - 若保留路由，必须返回稳定退役语义，并在 response 中指向 `/internal/cubebox/runtime-status` 作为 successor。
  - 若删除路由，必须同步更新 allowlist、route-capability-map、registry、tests 与 `380G` 负向断言，且不得退化为不受治理的 500。

### 5.4 失败语义 / stopline

| 失败场景 | 正式错误码 | 是否允许 fallback | explain 最低输出 | 是否 stopline |
| --- | --- | --- | --- | --- |
| 访问 `/app/assistant/librechat*` | `librechat_retired` | 否 | `code/message/meta.path/meta.method` | 是 |
| 访问 `/assistant-ui*` | `assistant_ui_retired` | 否 | `code/message/meta.path/meta.method` | 是 |
| 访问 `/app/assistant/librechat/api/*` 或 `/assets/librechat-web/api/*` | `librechat_retired` | 否 | `code/message/meta.path/meta.method` | 是 |
| `/internal/assistant/runtime-status` 仍被 active capability 注册 | N/A | 否 | readiness 记录中必须列出残留配置 | 是 |
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

1. [x] **Contract Slice**：冻结资产分类清单、退役解释层清单与 stopline。
2. [x] **Build / Runtime Slice**：下架 `Makefile` 中的 LibreChat runtime/build 入口，收口 `deploy/librechat/**` 与 `scripts/librechat/**` 的主干地位。
3. [x] **Docs / Skill Slice**：更新 README、skills、相关 dev-plan 引用，统一到 `CubeBox` 正式主链。
4. [x] **Test & Gates Slice**：复核退役断言、`error-message`、`make check doc` 与必要 E2E 负向断言。
5. [x] **Readiness Slice**：形成 `DEV-PLAN-380F-READINESS.md`，并向 `380G` 交接。

### 8.3 每个切片的完成定义

- **Contract Slice**
  - **输入**：`380C/380E` readiness 已可引用
  - **输出**：删除/保留/归档三类资产表已冻结；`/internal/assistant/runtime-status`、vendored compat API error code、活体 E2E 去向已裁决
  - **阻断条件**：若资产去向仍含糊，或 `runtime-status` / 旧 E2E 去向仍未写清，禁止开始物理清理

- **Build / Runtime Slice**
  - **输入**：资产分类已明确
  - **输出**：`Makefile`/脚本/部署说明不再把 LibreChat 当作正式 baseline；`/internal/assistant/runtime-status` 不再是 active capability / 正式诊断入口
  - **阻断条件**：若会误删退役 contract、break 现有负向断言，或让旧 runtime-status 变成不受治理的残留接口，必须暂停

- **Docs / Skill Slice**
  - **输入**：Build/Runtime 入口已收口
  - **输出**：所有活体说明统一改口；skill/README/runbook 不再把 `/app/assistant/librechat` 或 `assistant-runtime-*` 描述为正向联调入口
  - **阻断条件**：若仍有一个活体文档或 skill 推荐旧链路，视为未完成

- **Test & Gates Slice**
  - **输入**：代码/文档收口完成
  - **输出**：退役断言稳定，专项门禁通过，旧正向 LibreChat E2E 已迁移/归档/排除出封板必跑集
  - **阻断条件**：若旧路径只剩模糊 404/500，或旧正向 LibreChat E2E 仍在默认封板集内，则不得进入 readiness

- **Readiness Slice**
  - **输入**：上述切片完成
  - **输出**：`380F` readiness 与 `380G` 交接清单
  - **阻断条件**：若没有资产去向清单、负向断言清单、旧 E2E 去向清单与 `runtime-status` 裁决记录，禁止宣告完成

## 9. 测试、验收与 Readiness（Acceptance & Evidence）

### 9.1 验收标准

- **边界验收**：
  - [x] `380F` 只处理旧资产退役，不越权修改 `380C/380D/380E` 主链 contract
  - [x] 退役解释层与正式入口删除面已清晰分离

- **用户可见性验收**：
  - [x] 开发者查看仓库入口说明时，只会被引导到 `CubeBox`
  - [x] 用户访问旧入口时，仍能得到明确退役提示

- **数据 / 时间 / 租户验收**：
  - [x] 旧入口退役仍经过 tenant/session 边界，不出现旁路
  - [x] retired handler 审计信息仍可记录最小租户/请求上下文

- **UI / API 验收**：
  - [x] `/app/cubebox` 仍是唯一正式 UI 入口
  - [x] `/app/assistant/librechat`、`/assistant-ui/*`、`/assets/librechat-web/**` 稳定 `410 Gone`
  - [x] `/app/assistant/librechat/api/*`、`/assets/librechat-web/api/*` 稳定 `410 Gone + librechat_retired`
  - [x] `/internal/assistant/runtime-status` 已被删除或冻结为稳定退役语义，且不再是 active capability / 正式运行面
  - [x] `/internal/cubebox/runtime-status` 仍是唯一正式 runtime status API

- **测试与门禁验收**：
  - [x] `make check doc` 通过
  - [x] Go / routing / capability-route-map / authz / 前端相关门禁按实际改动命中并在 readiness 中记录
  - [x] `error-message` 口径未被破坏
  - [x] 负向 E2E 断言仍可复用给 `380G`
  - [x] 旧 LibreChat 正向 E2E 已迁移、归档或排除出 `380G` 必跑集
  - [x] 没有通过保留第二运行基线来“降低清理风险”

### 9.2 Readiness 记录

- [x] 新建 `docs/archive/dev-records/DEV-PLAN-380F-READINESS.md`
- [x] 在 readiness 中记录：
  - 时间戳
  - 删除/保留/归档清单
  - `/internal/assistant/runtime-status` 的最终裁决（删除 / retired handler）
  - route-capability-map / allowlist / registry / error-message 跟随动作
  - 活体 E2E 去向清单（保留负向 / 迁移到 `/app/cubebox` / 归档 / skip/fixme）
  - 实际执行入口
  - 结果
  - 负向断言与关键日志/截图/命令摘要
- [x] 本文档不复制执行输出；只链接 readiness 证据

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
