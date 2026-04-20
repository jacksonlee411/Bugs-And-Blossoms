# DEV-PLAN-450：直接切除 orgunit / jobcatalog / staffing 三模块方案

**状态**: 规划中（2026-04-20 16:18 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：以“直接切除、不可回退到 legacy、不可保留空壳兼容层”为原则，从当前 implementation repo 中整体删除 `orgunit`、`jobcatalog`、`staffing` 三个业务模块及其用户入口、运行时装配、数据库 schema/migration、权限对象、测试资产与现行契约文档引用。
- **关联模块/目录**：`modules/orgunit`、`modules/jobcatalog`、`modules/staffing`、`internal/server`、`apps/web`、`config/routing`、`config/access`、`config/capability`、`internal/sqlc`、`migrations/orgunit`、`migrations/jobcatalog`、`migrations/staffing`、`docs/dev-plans`、`docs/dev-records`、`AGENTS.md`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/015-ddd-layering-framework.md`、`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`、`docs/dev-plans/017-routing-strategy.md`、`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`、`docs/dev-plans/025-sqlc-guidelines.md`、`docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- **用户入口/触点**：`/app/org/**`、`/app/jobcatalog`、`/app/staffing/**`、对应 `/org/api/**` 与 `/org/**` 页面路由、导航入口、权限点、E2E 场景、README/dev-plan 中的现行引用

### 0.1 Simple > Easy 三问

1. **边界**：`450` 是三模块直接切除的唯一 owner；凡对象的现行存在价值仅服务于 `orgunit/jobcatalog/staffing` 三模块，均纳入本计划删除范围。
2. **不变量**：不得保留“隐藏入口仍可访问”“页面删了但 runtime 还在”“模块目录删了但 schema/权限/测试还保留”“先把 import 改薄壳以后再说”这类过渡尾态。
3. **可解释**：任一实施 PR 都必须能在 5 分钟内讲清楚“删除了哪一层、谁不再依赖它、若仍有依赖为何必须先停在 stopline 而不是偷留 compat”。

### 0.2 现状研究摘要

- **现状实现**：
  - 当前仓库的业务主干明确包含 `orgunit/jobcatalog/staffing/person` 四模块，且 `orgunit/jobcatalog/staffing` 三者在 UI、server、schema、Authz、E2E、文档中均为活体能力。
  - `person`、`iam`、若干平台门禁、以及一批现行 dev-plan 仍把三模块视为当前实现前提或上游依赖。
  - `AGENTS.md`、`009` 路线图、`060/062/063`、大量模块专项计划仍默认三模块继续存在。
- **现状约束**：
  - 本仓库禁止 legacy 双链路与兼容别名，删除不能以“先隐藏入口、保留旧 handler/store/schema 备用”的方式推进。
  - 新增数据库表需用户确认；本计划只允许删表/删函数/删迁移/删生成物，不允许为了删除再引入新的替代中间层。
  - `person` 与 `iam` 必须在删除后三方闭环仍可编译、可启动、可通过门禁，不能被遗留 import 或路由注册拖死。
- **最容易出错的位置**：
  - `internal/server` 组合根、导航、allowlist、capability-route-map、Casbin object/action。
  - `internal/sqlc/schema.sql` 与模块 schema/migrations 的联动删除。
  - `person`、`iam`、E2E、文档对三模块的隐式引用。
  - “只删目录不删契约”的文档回流。
- **本次不沿用的“容易做法”**：
  - 保留空目录、空 `module.go`、空路由、空页面占位。
  - 将三模块 API 改成 `410/404` 但保留 handler/store/schema。
  - 通过 `TODO`、skip、测试排除、门禁豁免掩盖未删干净的残留。

## 1. 背景与上下文

- 用户目标不是“弱化”或“冻结”三模块，而是**直接切除** `orgunit/jobcatalog/staffing`。
- 当前仓库的工程现实是，这三模块不是孤立目录，而是贯穿：
  - 用户入口与导航
  - server 组合根与路由
  - capability / Authz / routing 治理
  - schema / migration / sqlc
  - E2E / 单测 / 文档契约
- 因此本计划必须先冻结“什么叫切除完成”，否则实施极易退化为删目录但保留运行时依赖，最后形成更难收口的僵尸壳层。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 删除 `modules/orgunit`、`modules/jobcatalog`、`modules/staffing` 的生产代码、测试代码与模块装配入口。
- [ ] 删除三模块对应的 UI 页面、导航、前端请求层、i18n 文案、路由 allowlist 与 server handler 注册。
- [ ] 删除三模块对应的 schema、migrations、sqlc 输入与生成物、RLS/privilege/函数等数据库对象定义。
- [ ] 删除三模块对应的 Authz object、capability mapping、routing 分类、E2E/单测/文档主线引用。
- [ ] 保证删除后仓库剩余能力以 `iam/person` 为主仍可编译、门禁可运行、文档主线不再把三模块写成现行实现前提。

### 2.2 非目标

- [ ] 本计划不设计三模块的“替代产品方案”或“以后如何重建”。
- [ ] 本计划不保留任何过渡 API、compat DTO、legacy route、只读镜像页、空壳 module/store。
- [ ] 本计划不为了保住旧测试而降低 coverage 门槛、扩大排除项、放宽门禁。
- [ ] 本计划不新增任何新表、新缓存、新中间适配层来承接旧语义。

### 2.3 用户可见性交付

- **删除后用户入口**：不存在 `orgunit`、`jobcatalog`、`staffing` 的导航项、页面路由、按钮、表单、详情页、Explain/配置页。
- **删除后最小可见结果**：用户只能看到删除后三方不再存在的系统状态；不存在“点进去才发现 404/未实现”的假入口。
- **当前验收方式**：
  - 前端路由表与导航中不再出现三模块入口。
  - `internal/server` 不再注册三模块页面与 API。
  - 生产代码、schema、测试、文档搜索不再把三模块作为当前态能力。

## 2.4 工具链与门禁

- **命中触发器（勾选）**：
  - [X] Go 代码
  - [X] `apps/web/**` / presentation assets / 生成物
  - [X] i18n（仅 `en/zh`）
  - [X] DB Schema / Migration / Backfill / Correction
  - [X] sqlc
  - [X] Routing / allowlist / responder / capability-route-map
  - [X] AuthN / Tenancy / RLS
  - [X] Authz（Casbin）
  - [X] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`no-legacy`、`chat-surface-clean`、`ddd-layering-p0`、`ddd-layering-p2`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `pkg/**` | 删除仅服务三模块的 helper/契约/纯函数 | 按命中清单决定 | 不保留“也许以后会用”的共享壳 |
| `modules/*/services` | 删除三模块业务规则测试；修复剩余模块的断链 | `modules/orgunit/**`、`modules/jobcatalog/**`、`modules/staffing/**` | 目标是删，不是补空测试 |
| `internal/server` | 删除路由/协议解析/错误映射/Authz 编排测试 | `internal/server/*org*`、`*jobcatalog*`、`*staffing*` | 以“不再注册/不再存在”为验收 |
| `apps/web/src/**` | 删除页面/导航/API client/交互测试 | 三模块页面与组件目录 | 不保留页面壳 |
| `E2E` | 删除或重写依赖三模块的验收脚本 | `e2e/tests/**` 命中项 | E2E 不能继续把三模块当活体 |

## 3. 架构与关键决策

### 3.1 删除完成后的原则状态

删除完成后，当前态仓库不再存在：

1. `orgunit`、`jobcatalog`、`staffing` 的生产代码目录与默认装配。
2. 对应 UI 页面、导航入口、前端 API client、i18n 文案。
3. 对应 schema、migration、sqlc schema/export、RLS/函数/权限对象。
4. 对应 Authz object、capability-route-map、routing allowlist。
5. 把三模块当当前实现基础的现行 dev-plan / readiness / 测试合同表述。

### 3.2 模块归属与职责边界

- `450` 统一 owner 三模块切除顺序与 stopline，不把删除责任拆成多个无主专项。
- `internal/server` 只负责删除组合根、路由、导航与 API 注册，不承担“替代业务实现”。
- `apps/web` 只负责删除真实入口，不保留占位页。
- `docs/dev-plans` / `AGENTS.md` 负责把现行契约改成“仓库不再包含这三模块”，避免代码删完后文档继续回流。

### 3.3 ADR 摘要

- **决策 1**：采用“直接切除整条链路”的方案，不采用“先 UI 下线、后续再清 runtime/schema”的阶段拖尾。
  - **备选 A**：仅隐藏导航与页面。
  - **备选 B**：仅删除模块目录，保留 server/schema。
  - **选定理由**：两者都会制造僵尸能力与回流入口。

- **决策 2**：现行文档必须同步收口，不采用“代码先删、文档以后再补”。
  - **备选 A**：先改代码，后改 SSOT。
  - **选定理由**：当前 SSOT 明确把三模块写成活体主线，不同步改文档会持续制造冲突。

- **决策 3**：对三模块相关数据库对象采用“整模块删除 + 迁移重基线或整目录移除”口径，不保留空 migration 目录或历史生成物。
  - **备选 A**：保留旧 migration/schema/sqlc 作为历史参考。
  - **选定理由**：当前仓库是 implementation repo；保留活体目录只会继续被误用。

## 4. 删除范围冻结

### 4.1 必删运行时对象

1. [ ] `modules/orgunit/**`
2. [ ] `modules/jobcatalog/**`
3. [ ] `modules/staffing/**`
4. [ ] `internal/server` 中所有三模块 handler、resolver、module wiring、导航注册、页面渲染入口
5. [ ] `apps/web/src/pages`、`apps/web/src/components`、`apps/web/src/api` 中所有三模块页面与请求层
6. [ ] `config/routing/**`、`config/access/**`、`config/capability/**` 中三模块专属项

### 4.2 必删数据库对象

1. [ ] `migrations/orgunit/**`
2. [ ] `migrations/jobcatalog/**`
3. [ ] `migrations/staffing/**`
4. [ ] `modules/orgunit/infrastructure/persistence/schema/**`
5. [ ] `modules/jobcatalog/infrastructure/persistence/schema/**`
6. [ ] `modules/staffing/infrastructure/persistence/schema/**`
7. [ ] `internal/sqlc/schema.sql` 中三模块相关对象与导出内容
8. [ ] 三模块 sqlc 生成物与依赖测试资产

### 4.3 必删测试与文档对象

1. [ ] `e2e/tests/**` 中依赖三模块的 spec
2. [ ] `internal/server/**`、`apps/web/**`、`modules/**` 中三模块相关单测/集测
3. [ ] `AGENTS.md` 中把三模块写成现行主线或默认模块边界的表述
4. [ ] `009` 路线图、`016` 模块骨架、`060/062/063`、以及所有把三模块当现行能力的活体 dev-plan 条目
5. [ ] `docs/dev-records/**` 中仍把三模块视为当前验收对象的活体记录

## 5. 分阶段实施顺序

### 5.1 Phase 0：契约冻结与爆炸半径盘点

- [ ] 建立完整命中清单：代码、schema、路由、权限、测试、文档分别列出。
- [ ] 确认删除后三方以外的剩余系统边界，特别是 `iam/person` 是否仍为仓库现行能力。
- [ ] 先收口活体文档，把三模块从“当前默认主线”降级为“待删除对象”。

**Phase 0 完成条件**

- 所有后续实施 PR 都能引用同一个 owner 计划。
- 现行文档不再要求继续维护三模块。

### 5.2 Phase 1：用户入口与 server 外层协议切断

- [ ] 删除导航、页面路由、前端页面、前端 client、i18n 文案。
- [ ] 删除 allowlist、server handler 注册、页面渲染入口、API route。
- [ ] 删除 capability-route-map / Authz object / 路由分类中的三模块项。

**Phase 1 停止线**

- 若删除入口后仍有剩余主流程必须依赖三模块才能启动/登录/访问基础页面，必须先回到设计层说明仓库终态，而不是偷偷保留隐藏入口。

### 5.3 Phase 2：模块代码与组合根切断

- [ ] 删除三模块默认装配、server wiring、共享 helper 的上游依赖。
- [ ] 删除 `person`、`iam`、平台工具链中对三模块的 import、调用、DTO 依赖。
- [ ] 让剩余仓库在无三模块条件下可编译。

**Phase 2 停止线**

- 若 `person` 或平台基座仍把三模块视为不可替代上游，必须先冻结新的仓库边界，不得保留空壳 module 过编译。

### 5.4 Phase 3：schema / migration / sqlc 切断

- [ ] 删除三模块 schema、migration、sqlc export 与生成物。
- [ ] 同步收口 RLS/privilege/函数/测试数据脚本。
- [ ] 保证剩余数据库闭环命令不再触达三模块。

**Phase 3 停止线**

- 若删除 schema 需要新增中间表或假模块来维持现状，必须停下回到文档审批，不得实施期自造替代层。

### 5.5 Phase 4：测试、Readiness、文档封板

- [ ] 删除或重写命中三模块的单测、E2E、readiness、执行记录。
- [ ] 收口 `AGENTS.md` 文档地图、路线图、模块骨架与测试合同。
- [ ] 以门禁结果确认仓库已不存在三模块活体痕迹。

## 6. 验收口径

### 6.1 搜索验收

- `rg -n "orgunit|jobcatalog|staffing" modules internal apps/web config migrations docs` 不再命中任何“当前态实现/入口/契约”语义。
- 若因 archive/历史研究必须保留，必须明确迁入 `docs/archive/**` 并标明历史属性。

### 6.2 运行与门禁验收

- [ ] `make check doc`
- [ ] `go fmt ./... && go vet ./...`
- [ ] `make check lint`
- [ ] `make test`
- [ ] 命中的 `routing/authz/sqlc` 门禁通过
- [ ] 若仓库仍保留 E2E，则 `make e2e` 在删除后三模块条件下仍有有效测试集且非 0 tests

### 6.3 用户可见验收

- 不存在三模块导航与页面。
- 不存在三模块 API 可调用入口。
- 不存在三模块的 capability/authz object 可授权对象。

## 7. 风险与停止线

1. **仓库定位风险**：三模块被切除后，当前仓库是否仍保留足够明确的产品边界，必须在 Phase 0 冻结。
2. **文档回流风险**：若 `AGENTS.md`、`009`、`016`、`060/062/063` 不同步收口，代码删除后仍会被后续实现当作回流依据。
3. **门禁误伤风险**：删除三模块会波及 routing/authz/sqlc/E2E；必须按层分批删，不能一次性硬砍后再靠 skip 修门禁。
4. **数据库闭环风险**：模块级 Atlas/Goose/sqlc 当前以三模块存在为前提；若不先设计“删除后三方之外的闭环如何成立”，会出现命令入口整体失效。

## 8. 相关计划联动

- `DEV-PLAN-009`：当前路线图仍把三模块视为 Greenfield 主干，实施前必须同步改写或降级对应口径。
- `DEV-PLAN-016`：当前模块骨架默认包含四业务模块，实施前必须改成删除后三模块的新边界。
- `DEV-PLAN-060/062/063`：测试合同需改写为历史或重建，不能继续作为现行验收依据。
- `DEV-PLAN-440/441`：若命中 SetID/旧策略残余，按各自 owner 收口，但三模块切除顺序服从 `450` 的整体删除目标。

## 9. 交付物

1. [ ] `docs/dev-plans/450-direct-removal-of-orgunit-jobcatalog-staffing-modules-plan.md`
2. [ ] `AGENTS.md` 文档地图入口更新
3. [ ] 后续若进入实施，新增对应 `docs/dev-records/DEV-PLAN-450-READINESS.md`
