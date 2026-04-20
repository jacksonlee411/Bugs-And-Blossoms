# DEV-PLAN-450：直接切除 jobcatalog / staffing / person 三模块方案（保留 orgunit）

**状态**: 规划中（2026-04-20 16:18 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：以“直接切除、不可回退到 legacy、不可保留空壳兼容层”为原则，从当前 implementation repo 中整体删除 `jobcatalog`、`staffing`、`person` 三个业务模块及其用户入口、运行时装配、数据库 schema/migration、权限对象、测试资产与现行契约文档引用；同时删除 `DEV-PLAN-150/156` 引入并扩散出的 `capability_key / capability-route-map / capability catalog / policy activation / functional_area` 治理链及其门禁、配置、注册表与文档主线；`orgunit` 明确保留，作为删除后仍然存在的业务模块边界。
- **关联模块/目录**：`modules/jobcatalog`、`modules/staffing`、`modules/person`、`modules/orgunit`、`internal/server`、`apps/web`、`internal/server/assets`、`config/routing`、`config/access`、`config/capability`、`internal/sqlc`、`migrations/jobcatalog`、`migrations/staffing`、`migrations/person`、`scripts/ci`、`Makefile`、`.github/workflows/quality-gates.yml`、`e2e/test-results`、`playwright-report`、`docs/dev-plans`、`docs/dev-records`、`AGENTS.md`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/015-ddd-layering-framework.md`、`docs/archive/dev-plans/016-greenfield-hr-modules-skeleton.md`、`docs/dev-plans/017-routing-strategy.md`、`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`、`docs/dev-plans/025-sqlc-guidelines.md`、`docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- **用户入口/触点**：`/app/jobcatalog`、`/app/staffing/**`、`/app/person/**`、对应 `/org/api/**` 与 `/org/**` 页面路由、导航入口、权限点、E2E 场景、README/dev-plan 中的现行引用；`orgunit` 页面与 API 不在删除范围内

### 0.1 Simple > Easy 三问

1. **边界**：`450` 是三模块直接切除的唯一 owner；凡对象的现行存在价值仅服务于 `jobcatalog/staffing/person` 三模块，均纳入本计划删除范围；凡 `orgunit` 仍作为现行业务能力所必需的对象，不得误删。
2. **不变量**：不得保留“隐藏入口仍可访问”“页面删了但 runtime 还在”“模块目录删了但 schema/权限/测试还保留”“先把 import 改薄壳以后再说”这类过渡尾态。
3. **可解释**：任一实施 PR 都必须能在 5 分钟内讲清楚“删除了哪一层、谁不再依赖它、若仍有依赖为何必须先停在 stopline 而不是偷留 compat”。

### 0.2 现状研究摘要

- **现状实现**：
  - 当前仓库的业务主干明确包含 `orgunit/jobcatalog/staffing/person` 四模块；本次调整后，`orgunit` 保留，`jobcatalog/staffing/person` 作为待删除活体能力处理。
  - `staffing` 的 `Position / Assignments` 是本计划明确删除对象，不允许作为 `staffing` 子能力单独保留。
  - `iam`、`orgunit`、若干平台门禁、以及一批现行 dev-plan 仍把后三个模块视为当前实现前提或上游依赖。
  - `AGENTS.md`、`009` 路线图、`060/062/063`、`027`、大量模块专项计划仍默认这三个模块继续存在。
- **现状约束**：
  - 本仓库禁止 legacy 双链路与兼容别名，删除不能以“先隐藏入口、保留旧 handler/store/schema 备用”的方式推进。
  - 新增数据库表需用户确认；本计划只允许删表/删函数/删迁移/删生成物，不允许为了删除再引入新的替代中间层。
  - `iam` 与平台基座必须在删除后仍可编译、可启动、可通过门禁，不能被遗留 import 或路由注册拖死。
- **最容易出错的位置**：
  - `internal/server` 组合根、导航、allowlist、capability-route-map、Casbin object/action、capability registry。
  - `internal/sqlc/schema.sql` 与模块 schema/migrations 的联动删除。
  - `orgunit` 与被删除三模块之间的隐式耦合。
  - `DEV-PLAN-150/156` 引入并经 `151-160/161/163/165/181/183/184` 扩散的 capability 治理链与 `orgunit` 保留边界之间的缠绕。
  - `functional_area` 已成为 capability governance 的一部分；本计划纳入删除，不作为独立平台能力保留。
  - `iam`、E2E、文档对三模块的隐式引用。
  - “只删目录不删契约”的文档回流。
- **本次不沿用的“容易做法”**：
  - 保留空目录、空 `module.go`、空路由、空页面占位。
  - 将三模块 API 改成 `410/404` 但保留 handler/store/schema。
  - 通过 `TODO`、skip、测试排除、门禁豁免掩盖未删干净的残留。

## 1. 背景与上下文

- 用户目标不是“弱化”或“冻结”三模块，而是**直接切除** `jobcatalog/staffing/person`，同时保留 `orgunit`。
- 当前仓库的工程现实是，这三个待删除模块不是孤立目录，而是贯穿：
  - 用户入口与导航
  - server 组合根与路由
  - capability / functional_area / policy activation / Authz / routing 治理
  - schema / migration / sqlc
  - E2E / 单测 / 文档契约
- 因此本计划必须先冻结“什么叫切除完成”，否则实施极易退化为删目录但保留运行时依赖，最后形成更难收口的僵尸壳层。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 删除 `modules/jobcatalog`、`modules/staffing`、`modules/person` 的生产代码、测试代码与模块装配入口。
- [ ] 显式删除 `staffing` 下的 `Position / Assignments` 全链路，包括 `positions`、`positions:options`、`assignments`、assignment timeline/list/create、position/job profile 关联与相关读写模型。
- [ ] 删除三模块对应的 UI 页面、导航、前端请求层、i18n 文案、路由 allowlist 与 server handler 注册。
- [ ] 删除三模块对应的 schema、migrations、sqlc 输入与生成物、RLS/privilege/函数等数据库对象定义。
- [ ] 删除三模块对应的 Authz object、capability mapping、routing 分类、E2E/单测/文档主线引用。
- [ ] 删除三模块对应的全部测试资产与开发辅助资产，包括 `unit/integration/e2e`、`testdata`、`fixtures`、`mocks`、`snapshots`、`golden files`、seed/demo data、脚本化 smoke/rehearsal、截图/录屏证据与一次性排障脚本。
- [ ] 删除 `DEV-PLAN-150/156` 引入并扩散出的 `capability_key / capability-route-map / capability catalog / policy activation / functional_area` 内容，包括注册表、配置文件、Go 注册实现、CI 门禁、Makefile 入口、workflow 接线、相关测试与现行文档入口。
- [ ] 将 `orgunit` 从 capability 治理链解耦：`orgunit` 保留字段可编辑性/默认值/写入校验时，必须转为 `orgunit` 模块内静态策略或模块内服务，不再暴露或依赖 `capability_key`。
- [ ] 保证删除后仓库剩余能力以 `iam/orgunit` 为主仍可编译、门禁可运行、文档主线不再把三模块写成现行实现前提。

### 2.2 非目标

- [ ] 本计划不设计三模块的“替代产品方案”或“以后如何重建”。
- [ ] 本计划不保留任何过渡 API、compat DTO、legacy route、只读镜像页、空壳 module/store。
- [ ] 本计划不为了保住旧测试而降低 coverage 门槛、扩大排除项、放宽门禁。
- [ ] 本计划不新增任何新表、新缓存、新中间适配层来承接旧语义。
- [ ] 本计划不保留任何“为了将来可能恢复”而留存的测试、fixture、seed、mock、示例数据或脚本模板。
- [ ] 本计划不保留 `capability_key` / `capability-route-map` 的空壳 contract、门禁脚本或文档占位来“以后再决定是否恢复”。
- [ ] 本计划不把 `functional_area` 作为 capability 删除后的替代治理层保留；若未来需要模块开关，应另起计划并重新定义，不复用本次删除对象。

### 2.3 用户可见性交付

- **删除后用户入口**：不存在 `jobcatalog`、`staffing`、`person` 的导航项、页面路由、按钮、表单、详情页、Explain/配置页；`orgunit` 入口继续存在。
- **删除后最小可见结果**：用户只能看到 `jobcatalog/staffing/person` 已被移除后的系统状态；不存在“点进去才发现 404/未实现”的假入口。
- **当前验收方式**：
  - 前端路由表与导航中不再出现三模块入口，但 `orgunit` 保持可用。
  - `internal/server` 不再注册三模块页面与 API。
  - 生产代码、schema、测试、文档搜索不再把三模块作为当前态能力。

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
  - [X] 其他专项门禁：`no-legacy`、`chat-surface-clean`、`ddd-layering-p0`、`ddd-layering-p2`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `pkg/**` | 删除仅服务三模块的 helper/契约/纯函数 | 按命中清单决定 | 不保留“也许以后会用”的共享壳 |
| `modules/*/services` | 删除三模块业务规则测试；修复 `orgunit/iam` 的断链 | `modules/jobcatalog/**`、`modules/staffing/**`、`modules/person/**` | 默认全删相关测试，不为过测保壳 |
| `internal/server` | 删除路由/协议解析/错误映射/Authz 编排测试 | `*jobcatalog*`、`*staffing*`、`*person*` | `orgunit` 相关 server 测试不在默认删除范围 |
| `apps/web/src/**` | 删除页面/导航/API client/交互测试 | 三模块页面与组件目录 | 连同 `.test`、mock、story/demo 一并清掉 |
| `E2E` | 删除依赖三模块的验收脚本与证据资产 | `e2e/tests/**`、截图、trace、录像 | 默认删除，不做“失效断言”保留 |

## 3. 架构与关键决策

### 3.1 删除完成后的原则状态

删除完成后，当前态仓库不再存在：

1. `jobcatalog`、`staffing`、`person` 的生产代码目录与默认装配。
2. `staffing` 下的 `Position / Assignments` 任一子链路，包括 API、页面、schema、seed、fixture、E2E 与文档合同。
3. 对应 UI 页面、导航入口、前端 API client、i18n 文案。
4. 对应 schema、migration、sqlc schema/export、RLS/函数/权限对象。
5. 对应 Authz object、历史 capability 路由映射、routing allowlist。
6. 把三模块当当前实现基础的现行 dev-plan / readiness / 测试合同表述。
7. `orgunit` 继续作为现行模块存在，且不得因本计划被误删或降级为壳层。
8. `DEV-PLAN-150/156` 引入并扩散出的 capability 注册、映射、catalog、policy activation、functional_area、门禁与文档主线。
9. `orgunit` 对 `capability_key`、`resolveCapabilityContext`、`evaluateFunctionalAreaGate`、`resolveOrgUnitEffectivePolicyVersion` 的现行运行时依赖。

### 3.2 模块归属与职责边界

- `450` 统一 owner 三模块切除顺序与 stopline，不把删除责任拆成多个无主专项。
- `internal/server` 只负责删除组合根、路由、导航与 API 注册，不承担“替代业务实现”。
- `apps/web` 只负责删除真实入口，不保留占位页。
- `docs/dev-plans` / `AGENTS.md` 负责把现行契约改成“仓库保留 `orgunit`，不再包含 `jobcatalog/staffing/person` 三模块”，避免代码删完后文档继续回流。

### 3.3 ADR 摘要

- **决策 1**：采用“直接切除整条链路”的方案，不采用“先 UI 下线、后续再清 runtime/schema”的阶段拖尾。
  - **备选 A**：仅隐藏导航与页面。
  - **备选 B**：仅删除模块目录，保留 server/schema。
  - **选定理由**：两者都会制造僵尸能力与回流入口。

- **决策 2**：现行文档必须同步收口，不采用“代码先删、文档以后再补”。
  - **备选 A**：先改代码，后改 SSOT。
  - **选定理由**：当前 SSOT 明确把三模块写成活体主线，不同步改文档会持续制造冲突。

- **决策 3**：对三模块相关数据库对象采用“整模块删除 + 迁移重基线或整目录移除”口径，不保留空 migration 目录或历史生成物；`orgunit` schema/migration 明确保留。
  - **备选 A**：保留旧 migration/schema/sqlc 作为历史参考。
  - **选定理由**：当前仓库是 implementation repo；保留活体目录只会继续被误用。

- **决策 4**：对三模块相关测试与辅助资产采用“默认全删”口径，不采用“先留测试/fixture/seed 以后再说”；`orgunit` 相关测试与辅助资产仅在确认唯一服务于被删三模块时才删除。
  - **备选 A**：把旧测试改成 skip 或 pending。
  - **备选 B**：保留 fixture/mock/snapshot 作为历史样本。
  - **选定理由**：快速彻底切割的目标是缩短爆炸半径；保留这些资产只会继续拖住搜索结果、门禁与误引用，同时避免误删 `orgunit` 的有效资产。

- **决策 5**：对 `DEV-PLAN-150/156` 引入的 capability 治理链采用“整链删除”口径，不保留 `capability_key` contract、route map、registry、脚本或 Makefile/CI 接线。
  - **备选 A**：只删三模块相关的 capability 条目，保留整套治理框架。
  - **备选 B**：保留 `route-capability-map` 但置空。
  - **选定理由**：用户要求删除 `150/156` 引入的全部内容；若保留框架壳层，后续仍会形成回流与误用入口。

- **决策 6**：`functional_area` 纳入本计划删除范围，不作为 capability 删除后的独立治理机制保留。
  - **备选 A**：只删除 capability route map，保留 functional area state/switch。
  - **备选 B**：将 functional area 改名为模块开关后继续保留。
  - **选定理由**：当前 functional area 与 capability catalog / policy activation 共同构成 `150` 主链治理面；保留会形成空转治理层和回流入口。

- **决策 7**：保留 `orgunit` 但删除其 capability 依赖；字段策略回收进 `orgunit` 模块内语义，不新增跨模块替代层。
  - **备选 A**：保留 `orgunit` 的 capability_key 字段与 policy version 校验。
  - **备选 B**：删除 `orgunit` 字段策略 UI 与 API。
  - **选定理由**：`orgunit` 是保留模块，但 `capability_key` 是本计划删除对象；保留运行时依赖会使 `150/156` 无法彻底退役。若实施期无法完成解耦，必须先停在 stopline，不得保留 capability 空壳。

## 4. 删除范围冻结

### 4.1 必删运行时对象

1. [ ] `modules/jobcatalog/**`
2. [ ] `modules/staffing/**`
3. [ ] `modules/person/**`
4. [ ] `internal/server` 中 `position` / `positions` / `assignment` / `assignments` 相关 handler、options API、resolver、module wiring
5. [ ] `apps/web` 中 `/staffing/positions`、`/staffing/assignments` 页面、API client、测试与路由入口
6. [ ] `internal/server` 中所有三模块 handler、resolver、module wiring、导航注册、页面渲染入口
7. [ ] `apps/web/src/pages`、`apps/web/src/components`、`apps/web/src/api` 中所有三模块页面与请求层
8. [ ] `config/routing/**`、`config/access/**`、`config/capability/**` 中三模块专属项
9. [ ] `orgunit` 中仅为三模块服务的 adapter、DTO、options API、桥接 helper
10. [ ] `internal/server/capability_route_registry.go`、`capability_catalog_api.go`、`policy_activation_api.go`、`policy_activation_runtime.go`、`capability_context_authz.go`、`functional_area_*` 中仅为 `150/156` 体系存在的注册、catalog、激活与开关逻辑
11. [ ] `internal/server/authz_middleware.go` 中三模块 API 与 capability route requirement
12. [ ] `internal/server/handler.go` 中三模块默认装配、API 注册、`/internal/capabilities/**`、`/internal/policies/**`、`/internal/functional-areas/**` 路由
13. [ ] `apps/web/src/router/**`、`apps/web/src/navigation/**`、`apps/web/src/errors/**` 中三模块路由、导航、permissionKey、错误映射
14. [ ] `pkg/authz/registry.go` 中 `jobcatalog.catalog`、`person.persons`、`staffing.positions`、`staffing.assignments` 对象常量
15. [ ] `orgunit` 中 `capability_key`、`policy_version`、`effective_policy_version`、`functional_area` 相关返回字段与校验逻辑

### 4.2 必删数据库对象

1. [ ] `migrations/jobcatalog/**`
2. [ ] `migrations/staffing/**`
3. [ ] `migrations/person/**`
4. [ ] `modules/jobcatalog/infrastructure/persistence/schema/**`
5. [ ] `modules/staffing/infrastructure/persistence/schema/**`
6. [ ] `modules/person/infrastructure/persistence/schema/**`
7. [ ] `staffing` schema/migrations 中 `position_*`、`positions`、`assignment_*`、`assignments`、position/job profile 关联、position options、assignment timeline/list/create 相关表、函数、RLS、privilege、seed
8. [ ] `internal/sqlc/schema.sql` 中三模块相关对象与导出内容
9. [ ] 三模块 sqlc 生成物与依赖测试资产
10. [ ] `orgunit` 中仅为三模块存在的 SQL/export/bridge 片段
11. [ ] `config/capability/route-capability-map.v1.json`、`config/capability/contract-freeze.v1.json` 等 `150/156` 引入的 capability contract 文件
12. [ ] `functional_area` / `policy activation` 如有独立持久化或 schema 片段，也应一并删除，不保留空表、空 schema 或空 seed

### 4.3 必删测试与文档对象

1. [ ] `e2e/tests/**` 中依赖三模块的 spec
2. [ ] `internal/server/**`、`apps/web/**`、`modules/**` 中三模块相关单测/集测
3. [ ] `apps/web`、`internal/server`、`e2e`、`modules/**` 下所有三模块相关 `.test.*`、`.spec.*`、benchmark、fuzz、golden、snapshot、录像、trace、截图证据
4. [ ] `testdata/**`、`fixtures/**`、`mocks/**`、`__snapshots__/**`、`__fixtures__/**` 中三模块相关资产
5. [ ] `AGENTS.md` 中把三模块写成现行主线或默认模块边界的表述
6. [ ] `009` 路线图、`016` 模块骨架、`060/062/063`、`027`、以及所有把三模块当现行能力的活体 dev-plan 条目；按“改写现行文档或迁入 `docs/archive/**`”处理，不直接硬删除计划文档
7. [ ] `docs/dev-records/**` 中仍把三模块视为当前验收对象的活体记录
8. [ ] `orgunit` 下仅用于验证三模块联动的测试与证据资产
9. [X] `DEV-PLAN-150`、`DEV-PLAN-156` 及其直接引入的 capability-route-map / capability-key 活体文档、readiness、证据入口已转入 `docs/archive/**` 或改为历史来源
10. [ ] `DEV-PLAN-151`~`160`、`161/161A`、`163`、`165`、`181`、`183`、`184` 等 capability 主链扩散文档入口；统一迁至 `docs/archive/**` 或改写为历史说明，不可再作为现行主线保留

### 4.4 必删开发辅助与生成资产

1. [ ] `scripts/**`、`cmd/**`、`tools/**` 中只为三模块服务的 seed、bootstrap、smoke、rehearsal、backfill、排障脚本
2. [ ] 本地或仓内 demo data、sample payload、Postman/HTTP collection、临时导入导出样本
3. [ ] 前端构建产物、生成 client/types、静态 chunk、页面级资源中仅由三模块引入的残留文件
4. [ ] 与三模块绑定的 CI 命中项、fixture pipeline、测试报告、覆盖率补洞文件
5. [ ] 所有仅为三模块存在而保留的 README、runbook、截图、录屏与执行记录
6. [ ] `scripts/ci/check-capability-route-map.sh`、`scripts/ci/check-capability-key.sh`、`scripts/ci/check-capability-contract.sh`、`scripts/ci/check-capability-catalog.sh` 及其 Makefile / workflow 接线
7. [ ] `internal/server/assets/**`、`apps/web/dist/**`、嵌入式 bundle 中仍暴露三模块或 capability/functional_area 治理入口的构建产物
8. [ ] `e2e/test-results/**`、`playwright-report/**`、trace/video/screenshot 等测试结果目录
9. [ ] `Makefile` 的 `.PHONY`、`help`、`preflight`、动态 `MODULE` 过滤中的 capability 与三模块目标
10. [ ] `.github/workflows/**` 中 capability 门禁与三模块 matrix / step

### 4.5 必删授权与路由对象

1. [ ] `pkg/authz/registry.go` 中 `ObjectJobCatalogCatalog`、`ObjectPersonPersons`、`ObjectStaffingPositions`、`ObjectStaffingAssignments`
2. [ ] `config/access/policy.csv`、`config/access/policies/**` 中三模块 object/action
3. [ ] `config/routing/allowlist.yaml` 中 `/app/jobcatalog`、`/app/staffing/**`、`/app/person/**`、`/jobcatalog/api/**`、`/person/api/**`、`/org/api/positions**`、`/org/api/assignments**`
4. [ ] `apps/web/src/router/**`、`apps/web/src/navigation/**` 中 `jobcatalog.read`、`person.read`、`staffing.positions.read`、`staffing.assignments.read`
5. [ ] `internal/server/authz_middleware.go`、capability route binding 中对应路由到 object/action 的映射

### 4.6 orgunit 保留边界与 capability 删除替代策略

1. [ ] `orgunit` 页面与 API 保留，但不得再对外暴露 `capability_key`、`baseline_capability_key`、`policy_version`、`effective_policy_version` 等 capability 治理字段
2. [ ] `orgunit` 创建/写入字段策略如需保留，必须内收为 `orgunit` 模块内静态规则或模块内 service；不得继续调用 `resolveCapabilityContext`、`evaluateFunctionalAreaGate`、`resolveOrgUnitEffectivePolicyVersion`
3. [ ] `/internal/capabilities/**`、`/internal/policies/**`、`/internal/functional-areas/**` 删除后，`orgunit` 不得通过隐藏依赖或兼容 DTO 继续消费这些能力
4. [ ] 若某个 `orgunit` 交互当前必须依赖 capability 才能成立，实施必须先做解耦 PR；在解耦完成前，不得把 capability runtime 留作空壳

## 5. 分阶段实施顺序

### 5.0 极速切除建议顺序

> 目标不是“平滑迁移”，而是以最短路径把 `jobcatalog/staffing/person` 及其残留资产从仓库中剔除，同时避免误伤 `orgunit`。因此执行优先级应按“先删最容易形成拖尾的资产，再删入口，再删实现，再删底座，最后封板”推进。

1. [ ] **第一刀先删全部测试与辅助资产**
   - 先删 `e2e/tests/**`、页面测试、server 测试、模块测试、`testdata/fixtures/mocks/snapshots/golden`、`e2e/test-results/**`、`playwright-report/**`、trace、录像、截图、seed/demo data、一次性排障脚本。
   - 原则：只要资产唯一服务于三模块，先删，不等待代码目录删除；若同时服务 `orgunit`，转入人工判定。
2. [ ] **第二刀删全部用户入口与外层协议**
   - 删除导航、页面、前端 client、allowlist、handler 注册、历史 capability 路由映射残留、Authz object、页面资源与构建残留。
   - 原则：先让用户与调用方彻底碰不到三模块，同时保留 `orgunit` 页面与 API。
3. [ ] **第三刀删全部模块代码与组合根**
   - 删除 `modules/jobcatalog`、`modules/staffing`、`modules/person`，以及 `internal/server`、`pkg/**`、`cmd/**` 中的上游依赖。
   - 原则：不保留空壳目录、空 `module.go`、空 DTO、空 store；`orgunit` 仅删除桥接代码，不删模块主体。
4. [ ] **第四刀连带删掉 150/156 的 capability 治理链**
   - 删除 `config/capability/**`、`internal/server/capability_route_registry.go`、相关 capability catalog/activation/functional-area 路由、`scripts/ci/check-capability-*`、Makefile 入口与 workflow 接线。
   - 原则：不保留 `capability_key`/`capability-route-map`/`functional_area` 空壳机制；`orgunit` 必须先完成 capability 解耦再执行此刀。
5. [ ] **第五刀删全部数据库与生成链路**
   - 删除 schema、migration、sqlc export、生成物、seed SQL、dbtool/rehearsal/backfill 辅助命令，并重生成 `internal/server/assets/**` 等嵌入式前端产物。
   - 原则：不保留可被误执行的数据库入口；`orgunit` 只删除和三模块相关的桥接对象。
6. [ ] **第六刀封板文档与门禁**
   - 改写 `AGENTS.md` 现行入口，并将 `009`、`016`、`060/062/063`、`027`、`150-160/161/163/165/181/183/184` 等相关开发计划文档按需要迁入 `docs/archive/**` 或改写为历史说明；`readiness/dev-record` 中不再属于现行主线的记录同步归档或收口，然后跑剩余门禁。
   - 原则：代码删完后，文档不能继续把三模块或 `150` 主链当现行能力；相关开发计划文档优先归档而不是直接删除，同时要明确 `orgunit` 仍保留。

**执行偏好（冻结）**

- [ ] 优先“整桶删除”而不是“逐文件精修”。
- [ ] 优先删测试和辅助资产，再修编译错误；不要先花时间修已经决定删除的测试。
- [ ] 优先删目录和路由入口，再处理 import 断链；不要为了短期可编译保留空壳实现。
- [ ] 除非某资产同时服务 `iam/orgunit` 现行能力，否则默认删除，不做迁移。

### 5.1 Phase 0：契约冻结与爆炸半径盘点

- [ ] 建立完整命中清单：代码、schema、路由、权限、测试、文档分别列出。
- [ ] 在命中清单中单独列出“可直接全删的资产桶”：测试、fixture、seed、mock、snapshot、trace、demo data、脚本、生成物。
- [ ] 在命中清单中单独列出 `150/156` 引入并扩散的 capability 治理资产桶：config、registry、catalog、policy activation、functional_area、scripts、Makefile、workflow、tests、docs。
- [ ] 确认删除后剩余系统边界，特别是 `iam/orgunit` 如何作为仓库现行模块保留。
- [ ] 单独冻结 `orgunit` 的 capability 解耦方案：保留哪些字段策略能力、删除哪些 capability 返回字段、哪些 API 直接退役。
- [ ] 先收口活体文档，把三模块从“当前默认主线”降级为“待删除对象”；需要退役的开发计划文档统一纳入归档方案，不直接删除。

**Phase 0 完成条件**

- 所有后续实施 PR 都能引用同一个 owner 计划。
- 现行文档不再要求继续维护三模块，并明确 `orgunit` 保留。

### 5.2 Phase 1：用户入口与 server 外层协议切断

- [ ] 删除导航、页面路由、前端页面、前端 client、i18n 文案。
- [ ] 删除 allowlist、server handler 注册、页面渲染入口、API route。
- [ ] 删除 capability-route-map / Authz object / 路由分类中的三模块项，以及 `150/156` 引入的 route-capability 注册表、catalog 入口、policy activation 入口、functional_area 入口。
- [ ] 同批删除页面级测试、Vitest/Playwright 资产、前端 mock、页面截图与构建残留 chunk，避免 UI 已删但资产仍在。
- [ ] 删除三模块对应 permissionKey、Casbin object、allowlist 路由和 `authz_middleware` 路由映射。

**Phase 1 停止线**

- 若删除入口后仍有剩余主流程必须依赖三模块才能启动/登录/访问基础页面，必须先回到设计层说明仓库终态，而不是偷偷保留隐藏入口；`orgunit` 不构成此 stopline。

### 5.3 Phase 2：模块代码与组合根切断

- [ ] 删除三模块默认装配、server wiring、共享 helper 的上游依赖。
- [ ] 删除 `iam`、`orgunit`、平台工具链中对三模块的 import、调用、DTO 依赖。
- [ ] 删除 `150/156` 引入的 capability registry / activation / catalog / functional_area 运行时代码与测试。
- [ ] 显式删除 `Position / Assignments` 的 API、页面、schema、E2E、fixtures、options API 与任职时间线相关资产。
- [ ] 删除三模块对应的单测、集测、benchmark、fuzz、golden、test helper 与一次性排障脚本。
- [ ] 重写 `orgunit` 的字段策略返回与写入校验，不再依赖 capability context / policy version。
- [ ] 让剩余仓库在无三模块条件下可编译，且 `orgunit` 仍可编译。

**Phase 2 停止线**

- 若平台基座或 `orgunit` 仍把三模块或 capability runtime 视为不可替代上游，必须先冻结新的仓库边界，不得保留空壳 module / 空壳 capability 过编译。

### 5.4 Phase 3：schema / migration / sqlc 切断

- [ ] 删除三模块 schema、migration、sqlc export 与生成物。
- [ ] 同步收口 RLS/privilege/函数/测试数据脚本、seed SQL、sample payload、dbtool/rehearsal 辅助命令。
- [ ] 保证剩余数据库闭环命令不再触达三模块，且 `orgunit` 闭环仍成立。
- [ ] 删除 `150/156` 引入的 capability contract 文件、门禁脚本、Makefile 入口与 CI workflow 接线。
- [ ] 重生成前端 bundle 与 `internal/server/assets/**` 嵌入资产，确保旧路由不再通过静态资源暴露。

**Phase 3 停止线**

- 若删除 schema 需要新增中间表或假模块来维持现状，必须停下回到文档审批，不得实施期自造替代层。

### 5.5 Phase 4：测试、Readiness、文档封板

- [ ] 删除命中三模块的单测、E2E、readiness、执行记录、截图、trace、录像、报告；默认不重写，除非其同时覆盖 `iam/orgunit` 仍存能力。
- [ ] 收口 `AGENTS.md` 文档地图、路线图、模块骨架、`150` 主链文档入口与测试合同；相关开发计划文档完成归档迁移或历史化改写。
- [ ] 以门禁结果确认仓库已不存在三模块活体痕迹，且 `orgunit` 仍保持活体能力。

## 6. 验收口径

### 6.1 搜索验收

- 生产代码与配置使用精确 token 验收，不再使用宽泛 `person` 单词全仓阻断：
  - `rg -n "modules/jobcatalog|/jobcatalog/api|jobcatalog\\.catalog|jobcatalog\\.read|JobCatalogPage|nav_jobcatalog" modules internal apps/web pkg config migrations internal/server/assets`
  - `rg -n "modules/staffing|/org/api/positions|/org/api/positions:options|/org/api/assignments|/org/api/assignment-events|staffing\\.positions|staffing\\.assignments|PositionsPage|AssignmentsPage|nav_staffing_" modules internal apps/web pkg config migrations internal/server/assets`
  - `rg -n "modules/person|/person/api|person\\.persons|person\\.read|PersonsPage|nav_person|person_uuid|pernr" modules internal apps/web pkg config migrations internal/server/assets`
  - 以上命令均不再命中任何“当前态实现/入口/契约”语义。
- capability 主链与治理面使用精确 token 验收：
  - `rg -n "capability_key|capability-route-map|route-capability-map|capability_route_registry|check-capability-route-map|check-capability-key|check-capability-contract|check-capability-catalog|/internal/capabilities|/internal/policies|/internal/functional-areas|policy activation|functional_area" AGENTS.md docs config internal scripts Makefile .github apps/web`
  - 不再命中 `150` 主链现行治理语义；若命中 `docs/archive/**`，必须明确为历史来源。
- 授权与路由面使用精确 token 验收：
  - `rg -n "jobcatalog\\.catalog|person\\.persons|staffing\\.positions|staffing\\.assignments|jobcatalog\\.read|person\\.read|staffing\\.positions\\.read|staffing\\.assignments\\.read" pkg config internal apps/web`
  - `rg -n "/app/jobcatalog|/app/staffing/positions|/app/staffing/assignments|/app/person/persons|/jobcatalog/api|/person/api|/org/api/positions|/org/api/assignments" config internal apps/web internal/server/assets`
- 测试与辅助资产使用精确路径验收：
  - `find e2e playwright-report testdata fixtures mocks . -path '*test-results*' -o -iname '*jobcatalog*' -o -iname '*staffing*' -o -iname '*assignments*' -o -iname '*positions*' -o -iname '*persons*'`
  - 不再返回三模块测试、trace、视频、截图和报告资产。
- `orgunit` 相关搜索命中必须只反映保留能力，而不是 capability 桥接或三模块残余桥接；特别是 `capability_key`、`resolveCapabilityContext`、`evaluateFunctionalAreaGate`、`resolveOrgUnitEffectivePolicyVersion` 不得再出现在 `orgunit` 当前运行链路中。
- 若因 archive/历史研究必须保留，必须明确迁入 `docs/archive/**` 并标明历史属性。

### 6.2 运行与门禁验收

- [ ] `make check doc`
- [ ] `go fmt ./... && go vet ./...`
- [ ] `make check lint`
- [ ] `make test`
- [ ] 命中的 `routing/authz/sqlc` 门禁通过；历史 capability 主链门禁不再作为现行门禁存在
- [ ] 若仓库仍保留 E2E，则 `make e2e` 仅覆盖剩余现行能力；三模块相关 spec/trace/录像必须为 0
- [ ] `make preflight` 不再调用 capability 门禁，也不再暴露 `jobcatalog/staffing/person` 模块目标

### 6.3 用户可见验收

- 不存在三模块导航与页面。
- 不存在三模块 API 可调用入口。
- 不存在 `/app/staffing/positions`、`/app/staffing/assignments`、`/org/api/positions`、`/org/api/positions:options`、`/org/api/assignments` 等 Position / Assignments 入口。
- 不存在三模块的 capability/authz object 可授权对象。
- 不存在 `/internal/capabilities/**`、`/internal/policies/**`、`/internal/functional-areas/**` 治理入口。
- `orgunit` 导航、页面与 API 继续可用。

## 7. 风险与停止线

1. **仓库定位风险**：三模块被切除后，当前仓库是否仍保留足够明确的 `iam/orgunit` 边界，必须在 Phase 0 冻结。
2. **文档回流风险**：若 `AGENTS.md`、`009`、`016`、`060/062/063`、`027` 不同步收口，代码删除后仍会被后续实现当作回流依据。
3. **门禁误伤风险**：删除三模块会波及 routing/authz/sqlc/E2E；必须按层分批删，不能一次性硬砍后再靠 skip 修门禁。
4. **数据库闭环风险**：模块级 Atlas/Goose/sqlc 当前以这些模块存在为前提；若不先设计“删除后三模块且保留 `orgunit` 的闭环如何成立”，会出现命令入口整体失效。
5. **测试资产拖尾风险**：如果只删实现不删测试/fixture/seed/mock/snapshot，后续搜索、CI、构建和开发排障会持续命中三模块残留。
6. **误删 orgunit 风险**：由于 `orgunit` 与三模块存在历史耦合，必须显式区分“桥接残留”与“orgunit 主体能力”。
7. **治理链拖尾风险**：如果只删业务模块、不删 `150/156` 引入的 capability 治理链，仓库仍会保留一套脱离业务实体的空转治理系统。

## 8. 相关计划联动

- `DEV-PLAN-009`：当前路线图仍把三模块视为 Greenfield 主干的一部分，实施前必须同步改写或降级对应口径。
- `DEV-PLAN-016`：当前模块骨架默认包含四业务模块，实施前必须改成保留 `orgunit`、删除 `jobcatalog/staffing/person` 的新边界。
- `DEV-PLAN-060/062/063`：测试合同需改写为历史或重建，不能继续作为三模块的现行验收依据。
- `DEV-PLAN-150`、`DEV-PLAN-156`：本计划视为待退役 owner 文档；其引入的 capability-key / route-capability-map 主链一并删除，不作为现行能力保留。
- `DEV-PLAN-440/441`：若命中 SetID/旧策略残余，按各自 owner 收口，但三模块切除顺序服从 `450` 的整体删除目标。

## 9. 交付物

1. [ ] `docs/dev-plans/450-direct-removal-of-jobcatalog-staffing-person-modules-plan.md`
2. [ ] `AGENTS.md` 文档地图入口更新
3. [ ] 后续若进入实施，新增对应 `docs/dev-records/DEV-PLAN-450-READINESS.md`
