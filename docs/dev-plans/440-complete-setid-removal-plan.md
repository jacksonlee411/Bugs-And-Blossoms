# DEV-PLAN-440：彻底删除 SetID 的全仓收口方案

**状态**: 规划中（2026-04-20 11:11 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：将 SetID 从当前实现仓库的运行时、数据库 schema、路由/API、组合根、测试与文档中彻底移除，并以不引入 legacy fallback 的方式完成主线切换。
- **关联模块/目录**：`modules/orgunit`、`modules/jobcatalog`、`modules/staffing`、`internal/server`、`internal/sqlc/schema.sql`、`migrations/orgunit`、`cmd/dbtool`、`docs/dev-plans`、`AGENTS.md`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/001-technical-design-template.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/005-project-standards-and-spec-adoption.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/015-ddd-layering-framework.md`、`docs/dev-plans/017-routing-strategy.md`、`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`、`docs/dev-plans/330-strategy-module-architecture-and-design-convergence-plan.md`
- **用户入口/触点**：`/app/org/setid/**` 页面、`/org/api/setid-*` 与 `/org/api/global-setids` API、依赖 SetID 的 explain / bootstrap / jobcatalog 视图解析 / staffing 上下文解析

### 0.1 Simple > Easy 三问

1. **边界**：本次变更的 owner 是 `orgunit`；SetID 不再作为跨模块治理主轴存在，`jobcatalog` / `staffing` 仅消费新的无 SetID 契约，不保留兼容入口。
2. **不变量**：不得保留第二条读/写语义链路；不得保留 `setid/global_setid/scope_package/scope_subscription` runtime fallback；删除后不得再有新代码读取或写入 SetID 相关 schema。
3. **可解释**：作者必须能在 5 分钟内说明“SetID 删除后，组织上下文、主数据基线、字段/策略差异化、jobcatalog 归属解析分别由什么替代”。

### 0.2 现状研究摘要

- **现状实现**：
  - `orgunit` 仍公开 `SetIDPGStore` / `SetIDMemoryStore`，并在模块边界公开 SetID 存储语义。
  - `internal/server` 仍注册 `/org/api/setid-bindings`、`/org/api/global-setids`、`/org/api/setid-strategy-registry`、`/org/api/setid-explain`。
  - `internal/sqlc/schema.sql` 仍包含 `setid_events`、`setids`、`setid_binding_versions`、`global_setids`、`setid_scope_packages`、`setid_scope_subscriptions`、`setid_strategy_registry` 等当前态 schema。
  - `jobcatalog` 仍通过 `SetIDStore` 和 `ResolveJobCatalogPackageBySetID(...)` 承接业务视图。
- **现状约束**：
  - 当前仓库禁止 legacy 双链路，删除 SetID 不能以“新旧并跑”长期拖尾。
  - 新增数据库表前需用户确认；本计划目标是删减 schema，不引入新表作为替代缓冲。
  - 现有 `internal/sqlc/schema.sql` 是当前态事实源之一，必须与模块 schema、migrations、sqlc 生成物同步收口。
- **最容易出错的位置**：
  - `jobcatalog` 与 `staffing` 对 SetID 的间接依赖。
  - `orgunit.ensure_setid_bootstrap(...)` 及其衍生 bootstrap 流程。
  - explain / capability context / assistant task snapshot 中的 `resolved_setid` 漂移。
  - 文档与门禁仍把 SetID 当成现行语义时造成“代码删了、契约没删”的 SSOT 分裂。
- **本次不沿用的“容易做法”**：
  - 保留 alias API 或旧 DTO 字段但内部置空。
  - 用“兼容空实现”保留 `SetIDStore` 接口名。
  - 先只删 UI，不切 runtime / schema。
  - 用 ignore/gate 豁免掩盖残留。

## 1. 背景与上下文

- 当前仓库里，SetID 已不只是历史迁移残留，而是仍在参与 bootstrap、路由、schema、组合根和跨模块解析。
- 用户目标不是“减少提及 SetID”，而是**完全删除 SetID**。
- 若不先冻结整仓删除方案，后续实现很容易退化为“删入口但保留运行时壳层”，最终留下无法维护的僵尸语义。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 删除所有当前态 SetID runtime：Store、resolver、API、bootstrap、explain、assistant snapshot、跨模块解析。
- [ ] 删除所有当前态 SetID schema：表、函数、RLS、kernel privileges、scope/package/subscription 结构、global share SetID 结构。
- [ ] 删除所有 SetID UI 与路由入口，并同步清理 capability-route-map、authz requirement、测试、文档地图。
- [ ] 以单主链方式完成切断，不保留 legacy fallback、compat wrapper、空壳接口。

### 2.2 非目标

- [ ] 本计划不负责设计新的“组织差异化平台”终态 UI；只冻结 SetID 删除的边界与迁移顺序。
- [ ] 本计划不新增新的跨租户共享模型，不以新治理抽象替代 SetID 做等量替换。
- [ ] 本计划不通过新增第二套中间映射表为删除提供缓冲。

### 2.3 用户可见性交付

- **用户可见入口**：移除 `/app/org/setid` 及其子页，相关导航、链接、页面测试一并下线。
- **最小可操作闭环**：删除后，用户仍能完成 orgunit、jobcatalog、staffing 的现行主流程，且不会再看到 SetID 输入、说明、explain 或报错。
- **当前验收方式**：
  - UI 无 `/app/org/setid/**`
  - server 无 `/org/api/setid-*` / `/org/api/global-setids`
  - 代码搜索对 runtime 路径不再出现 `SetID` / `setid` 语义，仅允许历史归档文档和归档迁移记录存在

## 2.4 工具链与门禁

- **命中触发器（勾选）**：
  - [X] Go 代码
  - [ ] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [X] DB Schema / Migration / Backfill / Correction
  - [X] sqlc
  - [X] Routing / allowlist / responder / capability-route-map
  - [X] AuthN / Tenancy / RLS
  - [X] Authz（Casbin）
  - [X] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`no-legacy`、`no-scope-package`、`granularity`、`capability-key`、`capability-route-map`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `pkg/**` | 删除 SetID helper / resolver / normalize 后的纯函数收口 | `pkg/setid/**`、相关使用方 | 能删则删，保留者需重新定义职责 |
| `modules/*/services` | `jobcatalog` / `orgunit` / `staffing` 去 SetID 后的业务规则 | 对应 services 测试 | 不让 server 侧承接领域残余 |
| `internal/server` | 路由下线、错误映射、capability-route-map、旧 API 不再注册 | `internal/server/*setid*`、router tests | 删除相关测试或改为“路由不存在”断言 |
| `E2E` | UI 入口消失、主流程回归 | 相关 Playwright spec | 验证无 SetID 页面或弹窗残留 |

## 3. 架构与关键决策

### 3.1 5 分钟主流程

删除完成后的主流程不再出现 `SetID -> package -> subscription -> explain` 链路。组织上下文、主数据归属与字段动态策略必须通过非 SetID 契约承接，且这些承接点不得再保留旧字段名、旧 API 或旧 bootstrap 依赖。

### 3.2 模块归属与职责边界

- `orgunit`：
  - owner SetID 删除主任务。
  - 负责删除 schema、store、bootstrap、resolver、server wiring、capability route。
- `jobcatalog`：
  - 删除 `SetIDStore` 依赖与 by-setid 视图解析。
  - 重写为不依赖 SetID 的 package / baseline 视图契约。
- `staffing`：
  - 删除对 `resolved_setid` / `setid-explain` 的依赖和展示。
- `internal/server`：
  - 只做路由/API 切断与组合层去壳，不承接替代性业务逻辑。

### 3.3 ADR 摘要

- **决策 1**：SetID 采用“整仓单主链切断”，不采用“入口先删、schema 后留”的长期尾态。
  - **备选 A**：仅删 UI。
  - **备选 B**：保留 schema 与 resolver 供少量链路使用。
  - **选定理由**：这两种做法都会把 SetID 从“显式主轴”变成“隐式依赖”，维护风险更高。

- **决策 2**：`scope_package / scope_subscription / global_setid_*` 与 SetID 同批定义为待删除对象，不再视为独立保留语义。
  - **备选 A**：保留 package/subscription，先只删 setid binding。
  - **选定理由**：当前这些结构就是 SetID 语义展开的一部分，拆开保留会制造第二套历史负担。

### 3.4 Simple > Easy 自评

- **这次保持简单的关键点**：按 runtime、schema、routing、docs 四层一次性冻结删除范围，避免边删边发明兼容层。
- **明确拒绝的“容易做法”**：
  - [X] legacy alias / 双链路 / fallback
  - [X] 第二写入口 / controller 直写表
  - [X] 为过测临时加空壳 store 或空 handler
  - [X] 复制旧页面或旧 DTO 改名后继续保留

## 4. 删除范围冻结

### 4.1 必删运行时对象

1. [ ] `modules/orgunit/module.go` 中 `SetIDPGStore` / `SetIDMemoryStore` 与相关包装。
2. [ ] `internal/server` 中所有 `setid-*` handler、resolver、explain、route capability entry。
3. [ ] `jobcatalog` 中 `SetIDStore` 及 `ResolveJobCatalogPackageBySetID(...)`。
4. [ ] `assistant` / `staffing` / `orgunit` 中 `ResolvedSetID` 快照、解释、兼容字段。
5. [ ] `cmd/dbtool` 中所有 rehearsal / snapshot / validate 的 setid 专项工具。

### 4.2 必删数据库对象

1. [ ] `orgunit.setid_events`
2. [ ] `orgunit.setids`
3. [ ] `orgunit.setid_binding_events`
4. [ ] `orgunit.setid_binding_versions`
5. [ ] `orgunit.global_setid_events`
6. [ ] `orgunit.global_setids`
7. [ ] `orgunit.setid_scope_packages`
8. [ ] `orgunit.global_setid_scope_packages`
9. [ ] `orgunit.setid_scope_package_events`
10. [ ] `orgunit.global_setid_scope_package_events`
11. [ ] `orgunit.setid_scope_package_versions`
12. [ ] `orgunit.global_setid_scope_package_versions`
13. [ ] `orgunit.setid_scope_subscription_events`
14. [ ] `orgunit.setid_scope_subscriptions`
15. [ ] `orgunit.setid_strategy_registry`
16. [ ] 所有 `submit_*setid*`、`resolve_setid(...)`、`ensure_setid_bootstrap(...)`、`global_tenant_id()` 及相关权限/RLS

### 4.3 必删用户入口

1. [ ] `/app/org/setid`
2. [ ] `/app/org/setid/base`
3. [ ] `/app/org/setid/registry`
4. [ ] `/app/org/setid/explain`
5. [ ] `/app/org/setid/ops`
6. [ ] `/org/api/setid-bindings`
7. [ ] `/org/api/global-setids`
8. [ ] `/org/api/setid-strategy-registry`
9. [ ] `/org/api/setid-strategy-registry:disable`
10. [ ] `/org/api/setid-explain`

## 5. 实施步骤

1. [ ] 冻结 blast radius：列出所有 Go/runtime/schema/doc 命中点，并形成 readiness 证据。
2. [ ] 切断 server/UI 入口：先让用户与外部调用方无法再进入 SetID 路径。
3. [ ] 切断组合根与跨模块依赖：删除 `SetIDStore`、resolver、snapshot 字段与 `jobcatalog` / `staffing` 依赖。
4. [ ] 切断 schema 与 sqlc：删除所有 SetID / global_setid / scope_package / subscription / strategy_registry 对象，并重建 `internal/sqlc/schema.sql`。
5. [ ] 删除 dbtool / tests / fixtures / gate references。
6. [ ] 更新文档与 `AGENTS.md`，将 SetID 从现行主线契约中摘除或移入归档。
7. [ ] 执行全量验证并补 `docs/dev-records/` readiness 证据。

## 6. 验收标准

1. [ ] `rg -n '\\bSetID\\b|\\bsetid\\b|global_setid|setid_scope_|setid_binding|resolved_setid'` 在生产代码与当前态 schema 中不再命中，仅允许归档文档/归档迁移存在。
2. [ ] `internal/server` 无 `setid-*` 路由注册。
3. [ ] `modules/*` 无 `SetIDStore`、`SetIDPGStore`、`SetIDMemoryStore` 暴露。
4. [ ] `internal/sqlc/schema.sql` 无 SetID 相关当前态对象。
5. [ ] `AGENTS.md` 不再把 SetID 作为现行运行时能力或治理入口。

## 7. 风险与停止线

- **高风险**：`jobcatalog` 与 `staffing` 仍依赖 SetID 解释视图；若替代契约未先冻结，直接删 schema 会导致主流程失效。
- **停止线**：
  - 若发现某条主流程仍只能通过 SetID 解释完成，先补替代契约或拆分计划，不得以保留空壳 SetID 为代价继续推进。
  - 若删除需要新增表来维持旧语义，必须回到设计层重新审批，而不是在实施中偷偷扩表。

## 8. 交付物

- `docs/dev-plans/440-complete-setid-removal-plan.md`
- 后续对应 readiness 文档：`docs/dev-records/DEV-PLAN-440-READINESS.md`
- 代码与 schema 删除 PR（预计拆分为多 PR，但必须共享本计划作为唯一契约）
