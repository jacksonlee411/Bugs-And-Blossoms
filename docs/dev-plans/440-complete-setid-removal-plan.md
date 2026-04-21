# DEV-PLAN-440：彻底删除 SetID 的全仓收口方案

**状态**: 规划中（2026-04-20 15:10 CST，已完成方案补强与联动文档收口）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：将 SetID 从当前 implementation repo 的现行运行时、数据库 schema、路由/API、前端入口、鉴权对象、测试资产与现行契约文档中彻底删除，并以不引入 legacy fallback 的单主链方式完成切断。
- **关联模块/目录**：`modules/orgunit`、`modules/jobcatalog`、`modules/staffing`、`internal/server`、`pkg/setid`、`pkg/fieldpolicy`、`internal/sqlc/schema.sql`、`migrations/orgunit`、`migrations/jobcatalog`、`migrations/staffing`、`apps/web`、`cmd/dbtool`、`config/access`、`config/capability`、`config/routing`、`docs/dev-plans`、`AGENTS.md`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/017-routing-strategy.md`、`docs/archive/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/archive/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`、`docs/dev-plans/441-legacy-strategy-module-residue-cleanup-plan.md`
- **用户入口/触点**：`/app/org/setid/**`、`/org/api/setid-*`、`/org/api/global-setids`、JobCatalog/Staffing 中所有显式或隐式 SetID 上下文、解释面板、registry、dbtool rehearsal/validate 辅助

### 0.1 Simple > Easy 三问

1. **边界**：`440` 是 SetID 根删除的唯一 owner；凡对象同时命中“旧策略残余”和“SetID 根语义”，一律由 `440` 统一排序与切断。
2. **不变量**：删除过程不得保留第二条读/写语义链路；不得保留 alias API、compat DTO、空壳 Store、隐藏 bootstrap、只删 UI 不删 runtime 的尾态。
3. **可解释**：任一实施 PR 都必须能在 5 分钟内说明“删掉这一层后，上下游由什么承接；若尚无承接，为什么必须先停在文档和停止线，而不是偷留 compat”。

### 0.2 现状研究摘要

- **现状实现**：
  - `internal/server` 仍在默认装配 `SetIDStore`，并注册 `/org/api/setids`、`/org/api/setid-bindings`、`/org/api/global-setids`、`/org/api/setid-strategy-registry`、`/org/api/setid-explain`。
  - `modules/orgunit` 仍公开 `SetIDPGStore` / `SetIDMemoryStore`，`pkg/setid` 仍直接调用 `ensure_setid_bootstrap(...)` 与 `resolve_setid(...)`。
  - `apps/web` 仍保留 `/app/org/setid` 全套页面、导航、i18n、Explain/Registry 组件。
  - `internal/sqlc/schema.sql`、`modules/orgunit` schema 与 `migrations/orgunit` 仍保留 SetID 表、函数、权限、bootstrap、scope/package/subscription 等治理底座。
  - `jobcatalog` / `staffing` / `person` / `pkg/fieldpolicy` / `setid_strategy_registry` 的当前态执行面已由 `DEV-PLAN-450` 收口删除，不再是本计划剩余阻塞项。
- **现状约束**：
  - 当前仓库禁止 legacy 双链路，删除 SetID 不能以“旧接口先留着但置空”方式拖尾。
  - 新增数据库表需用户确认；本计划允许删表/删函数/删权限，不允许在实施中偷偷引入新的“过渡治理表”。
  - 现有 `060/062`、`AGENTS.md` 等现行文档仍把 SetID 写成主流程；若不先收口文档，后续代码删除会持续与 SSOT 冲突。
- **最容易出错的位置**：
  - `jobcatalog` 与 `staffing` 的间接依赖，因为它们表面上不是 SetID 模块，但仍把 SetID 当上下文解析主轴。
  - `pkg/fieldpolicy` / `setid_strategy_registry`，因为其命名既是 SetID 残留，也是旧策略残留。
  - `internal/sqlc/schema.sql` 与 migrations 的同步切除，若只删一边会造成 sqlc/schema 漂移。
  - 文档地图继续把 SetID 当现行主线，导致删代码后仍有回流压力。
- **本次不沿用的“容易做法”**：
  - 保留旧 API / 旧 DTO 字段但返回空值。
  - 先只删页面，再保留 runtime / schema “以后再说”。
  - 用 `ignore`、门禁豁免、文档备注来掩盖仍在运行的 SetID 依赖。
  - 把 SetID 删除拆成无 owner 的“大家各清一点”。

## 1. 背景与上下文

- 用户目标不是“弱化 SetID 的可见度”，而是**彻底删除 SetID**。
- 当前仓库里，SetID 仍同时存在于：
  - 用户入口
  - 运行时组合根
  - 跨模块业务解析
  - schema 与 kernel
  - authz / 路由注册
  - E2E / 文档 / i18n
- 因此本计划不是单一模块 refactor，而是一次**跨层收口**。如果不先冻结范围与分阶段停止线，实施很容易退化为“外层删了、里层壳还在”。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 删除所有当前态 SetID runtime：Store、resolver、API、bootstrap、explain、assistant/staffing 快照字段、跨模块解析与 registry。
- [ ] 删除所有当前态 SetID schema：表、函数、RLS/privileges、`scope_package/scope_subscription`、`global_setid_*` 等剩余治理对象；`setid_strategy_registry` 已由 `DEV-PLAN-450` 删除。
- [ ] 删除所有 SetID UI 与路由入口，并同步清理导航、i18n、错误码、authz object、历史 capability 路由映射残留。
- [ ] 删除现行测试合同中把 SetID 当主线能力的口径，并将相关测试改为“历史冻结/待重写/待删除”状态，不制造伪通过。
- [ ] 形成单一删除 PoR：后续任何“SetID 根删除”工作都以 `440` 排序，不再散落到其他计划文档并列 owner。

### 2.2 非目标

- [ ] 本计划不设计新的“组织差异化平台”终态产品方案。
- [ ] 本计划不以新抽象等量替换 SetID；如果某链路删除后确实缺少新的正式契约，必须在停止线处停住，而不是偷偷保留 SetID 壳层。
- [ ] 本计划不为删除引入新表、新总线、新 bootstrap 中间层。
- [ ] 本计划不把“历史归档文档继续保留”误写成“当前态仍支持”。

### 2.3 用户可见性交付

- **删除后用户入口**：不存在 `/app/org/setid` 及其子页面，不存在 SetID Explain/Registry/Ops 页，不存在 SetID 导航项。
- **删除后主流程**：用户仍可完成 `orgunit` 的现行主流程，但不再看到 `setid` 输入、`resolved_setid` 解释、`owner_setid`、`global share setid` 等 UI 语义。`jobcatalog/staffing/person` 已由 `DEV-PLAN-450` 从当前仓库删除，不属于本计划现行验收范围。
- **当前验收方式**：
  - 路由 allowlist 中不存在 `/app/org/setid/**` 与 `/org/api/setid-*` / `/org/api/global-setids`
  - `internal/server` 不再注册相关 handler
  - 生产代码 / 当前态 schema 搜索不再命中 SetID runtime 语义
  - 现行 dev-plan / AGENTS 不再把 SetID 当主线能力

## 2.4 工具链与门禁

- **命中触发器（勾选）**：
  - [X] Go 代码
  - [X] `apps/web/**` / presentation assets / 生成物
  - [X] i18n（仅 `en/zh`）
  - [X] DB Schema / Migration / Backfill / Correction
  - [X] sqlc
  - [X] Routing / allowlist / responder
  - [X] AuthN / Tenancy / RLS
  - [X] Authz（Casbin）
  - [X] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`no-legacy`、`no-scope-package`、`granularity`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `pkg/**` | 删除 `pkg/setid` 与 SetID 相关纯函数/rego/PDP | `pkg/setid`、残余 helper | `pkg/fieldpolicy` 已由 `DEV-PLAN-450` 删除 |
| `modules/*/services` | `orgunit` 去 SetID 后的业务规则收口 | 对应 services tests | 不允许 server 层替模块兜底 |
| `internal/server` | 路由下线、错误映射、authz/capability 收口 | `internal/server/*setid*` | 删除或改为“路由不存在/能力不存在”断言 |
| `apps/web` | 页面/导航/文案/请求层删除 | `pages/org/SetID*`、`api/setids.ts` | 页面级可见性必须真实消失 |
| `E2E` | 删除旧 SetID 测试，或改为不存在断言 | `e2e/tests/*setid*`、`tp060-02` | `m3-smoke` 已由 `DEV-PLAN-450` 删除 |

## 3. 架构与关键决策

### 3.1 删除完成后的原则状态

删除完成后，当前态仓库不再存在：

1. `SetID -> binding -> package -> subscription -> explain` 这条主链。
2. 任何以 `resolved_setid` 或 `owner_setid` 为名的外显业务上下文。
3. 任何显式面向用户的 SetID 页面、权限对象、导航入口或错误语义。
4. 任何“虽然页面没了，但 runtime 还在供内部链路调用”的隐式尾态。

### 3.2 模块归属与职责边界

- `orgunit`
  - owner SetID 根删除。
  - 负责 schema、kernel、store、bootstrap、server wiring、authz 对象与 UI 删除。
- `jobcatalog` / `staffing` / `person`
  - 已由 `DEV-PLAN-450` 直接切除，不再作为 `440` 的现行 owner 模块。
- `pkg/fieldpolicy`
  - 已由 `DEV-PLAN-450` 删除；本计划不再把它当作当前剩余阻塞项。
- `internal/server`
  - 只承接路由/API 切断和组合根去壳，不承担替代业务逻辑。

### 3.3 ADR 摘要

- **决策 1**：采用“文档契约先收口 + 分阶段单主链切断”，不采用“先删一层、其余以后说”。
  - **备选 A**：只删 UI。
  - **备选 B**：只删 schema。
  - **选定理由**：两者都会制造新的隐式依赖层。

- **决策 2**：`scope_package / scope_subscription / global_setid_* / strategy_registry` 全部视为 SetID 链路组成部分，由 `440` 同批 owner。
  - **备选 A**：将 package/subscription/registry 独立保留。
  - **选定理由**：当前仓库里它们仍是 SetID 语义展开，而不是独立稳定能力。

- **决策 3**：现行契约文档必须先降级 SetID 主线地位，才能进入大规模代码实施。
  - **备选 A**：先动代码，文档之后补。
  - **选定理由**：当前 SSOT 明显把 SetID 当活体主线，不先收口文档只会持续制造回流。

## 4. 删除范围冻结

### 4.1 必删运行时对象

1. [ ] `modules/orgunit/module.go` 中 `SetIDPGStore` / `SetIDMemoryStore` 与相关包装。
2. [ ] `internal/server` 中所有 `setid-*` handler、resolver、context resolver、explain、registry API、route capability entry。
3. [ ] `pkg/setid/**` 全量。
4. [X] `jobcatalog/staffing/fieldpolicy/setid_strategy_registry` 相关 runtime 已由 `DEV-PLAN-450` 删除，不再属于当前待办。
5. [ ] `assistant` 中仍残留的 `setid` 输入/输出与解释面。
6. [ ] `cmd/dbtool` 中所有仍存活的 setid / bootstrap 专项工具。

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
16. [ ] 所有 `submit_*setid*`、`resolve_setid(...)`、`ensure_setid_bootstrap(...)`、`global_tenant_id()` 及相关 privileges/RLS

### 4.3 必删用户入口

1. [ ] `/app/org/setid`
2. [ ] `/app/org/setid/base`
3. [ ] `/app/org/setid/registry`
4. [ ] `/app/org/setid/explain`
5. [ ] `/app/org/setid/ops`
6. [ ] `/org/api/setids`
7. [ ] `/org/api/setid-bindings`
8. [ ] `/org/api/global-setids`
9. [ ] `/org/api/setid-strategy-registry`
10. [ ] `/org/api/setid-strategy-registry:disable`
11. [ ] `/org/api/setid-explain`

### 4.4 必删现行契约口径

1. [ ] `AGENTS.md` 中把 SetID 写成现行 Greenfield 不变量的描述。
2. [ ] `AGENTS.md` 文档地图中把 SetID 主链文档当活体主线的表述。
3. [ ] `DEV-PLAN-060/062` 中将 SetID 作为现行主数据主流程的描述。
4. [ ] 任何现行 dev-plan 中把 `/app/org/setid`、`resolved_setid` 当当前用户能力或当前实施主线的表述；`setid_strategy_registry` 已由 `DEV-PLAN-450` 删除。

## 5. 分阶段实施顺序

### 5.1 Phase 0：契约与爆炸半径冻结

- [ ] 完成全仓命中清单，按“运行时 / schema / authz / front-end / tests / docs”分类。
- [ ] 建立 `DEV-PLAN-440-READINESS.md`，记录当前命中、分批次 owner、阻塞链路、禁止事项。
- [ ] 先完成文档收口：`440` 成为唯一 PoR，`441` 降为配套残余清理计划，`060/062` 改为历史测试合同或待重写状态，`AGENTS.md` 不再把 SetID 写成现行主线。

**Phase 0 完成条件**
- 现行文档不再把 SetID 当“应该继续实现/继续维持”的主线。
- 所有后续代码删除 PR 都能引用同一个 owner 计划。

### 5.2 Phase 1：用户入口与外层协议切断

- [ ] 删除前端页面、导航、文案、前端请求层。
- [ ] 删除 allowlist 路由、历史 capability 路由映射残留、authz 对象、Casbin policy 中的 SetID 条目。
- [ ] 删除 server 路由注册与对外 handler。

**Phase 1 停止线**
- 若此阶段发现某用户主流程仍必须直接访问 SetID UI/API 才能完成，必须先补“无 SetID 主流程契约”或记录为阻塞，不得继续靠隐藏入口维持。

### 5.3 Phase 2：组合根与跨模块依赖切断

- [ ] 删除 `SetIDStore` 默认装配。
- [ ] 删除 `pkg/setid` 与 `modules/orgunit` 的 SetID Store 暴露。
- [X] `jobcatalog` / `staffing` / `fieldpolicy` 中所有 SetID 上下文输入已由 `DEV-PLAN-450` 退出当前执行面。

**Phase 2 停止线**
- 当前停止线聚焦 `orgunit` 与 `pkg/setid` 自身；不得以保留 compat Store/DTO 的方式拖延 SetID 根删除。

### 5.4 Phase 3：schema / sqlc / migration 切断

- [ ] 删除模块 schema 与 `internal/sqlc/schema.sql` 中所有 SetID 相关对象。
- [ ] 同步删除 migrations、Atlas sum、sqlc 生成物与 stopline 资产。
- [ ] 删除所有 bootstrap / validate / rehearsal 脚本。

**Phase 3 停止线**
- 若 schema 删除需要新增“中间映射表”来保住旧语义，必须回到文档审批，不得在实施中自行扩表。

### 5.5 Phase 4：测试、错误码、文档封板

- [ ] 删除或重写 E2E、server tests、module tests 中的 SetID 主链。
- [ ] 删除错误码、i18n 文案、README/dev-record 中的活体 SetID 语义。
- [ ] 将确需保留的历史材料转入 archive 或显式标注“历史/非现行”。

## 6. 相关计划联动与 owner 边界

### 6.1 `DEV-PLAN-441`

- `441` 只负责“旧策略模块残余”中的命名、结构、工具与文档清理。
- 凡对象同时属于 SetID 根删除范围，排序和 owner 服从 `440`。
- `441` 不得单独定义 SetID 相关 schema/runtime 的删除顺序。

### 6.2 `DEV-PLAN-060/062`

- 当前 `060/062` 中把 SetID 写成主数据主流程，属于**与 440 冲突的现行测试合同**。
- 在 SetID 删除实施前，必须先将其改成：
  - 历史测试合同
  - 或待重写测试蓝图
  - 或显式标注“本合同依赖已退役 SetID 主线，不再作为当前实现验收依据”

### 6.3 `AGENTS.md`

- `AGENTS.md` 只能保留：
  - `440` 是当前 SetID 删除 PoR
  - 历史 SetID 文档为 archive 或历史来源
- `AGENTS.md` 不得继续把 SetID 写为 Greenfield 现行不变量、现行时间口径或当前用户主线能力入口。

### 6.4 历史 SetID 文档族

- `070/071/102C/015Z/161A/163A/185/191/203` 等文档当前多为历史调查、历史设计或中间阶段方案。
- 若仍需保留，必须在文档标题、状态或引用位置上明确“历史/待归档/不再作为现行实现依据”。
- `440` 不要求本轮一次性重写所有历史文档内容，但要求先切掉它们作为**现行入口**的地位。

## 7. Readiness 与停止线

### 7.1 Readiness 必备内容

后续 `docs/dev-records/DEV-PLAN-440-READINESS.md` 至少必须包含：

1. 当前态命中面清单
2. 允许保留的历史范围
3. 分阶段 PR 拆分建议
4. 每阶段前置条件与停止线
5. 验证命令矩阵
6. 风险登记与 owner

### 7.2 高风险与停止线

- **高风险 A**：`orgunit` 与 `pkg/setid` 仍保留完整 SetID 底座，删路由但不删底座会造成假收口。
  - **停止线**：不得以保留 compat Store/字段的方式继续推进。
- **高风险 B**：文档地图仍把 SetID 当活体主线。
  - **停止线**：在 `AGENTS.md` 和 `060/062` 未收口前，不进入大规模 runtime/schema 删除。
- **高风险 D**：schema 删除与 sqlc/Atlas 不同步。
  - **停止线**：任何一批 schema 删除必须包含 `internal/sqlc/schema.sql`、模块 schema、migrations、生成物的闭环收口。

## 8. 验收标准

1. [ ] `rg -n '\\bSetID\\b|\\bsetid\\b|global_setid|setid_scope_|setid_binding|resolved_setid'` 在生产代码与当前态 schema 中不再命中，仅允许 archive 文档或 archive 迁移记录存在。
2. [ ] `internal/server` 无 `setid-*` 路由注册，`apps/web` 无 `/app/org/setid/**` 路由与导航。
3. [ ] `modules/*` 无 `SetIDStore`、`SetIDPGStore`、`SetIDMemoryStore`、`pkg/setid` 暴露。
4. [ ] `internal/sqlc/schema.sql`、模块 schema、migrations 无 SetID 当前态对象。
5. [ ] `config/access`、`pkg/authz`、`config/capability`、`config/routing` 无 SetID 活体对象与映射。
6. [ ] `AGENTS.md`、`DEV-PLAN-060/062` 及其他现行入口文档不再把 SetID 当现行主线能力。
7. [ ] `docs/dev-records/DEV-PLAN-440-READINESS.md` 已补齐，并包含通过/未通过项与剩余阻塞说明。

## 9. 交付物

- 本计划：`docs/dev-plans/440-complete-setid-removal-plan.md`
- 对应 readiness：`docs/dev-records/DEV-PLAN-440-READINESS.md`
- 相关联动计划：
  - `docs/dev-plans/441-legacy-strategy-module-residue-cleanup-plan.md`
  - `docs/dev-plans/060-business-e2e-test-suite.md`
  - `docs/archive/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`
  - `AGENTS.md`
