# DEV-PLAN-451：OrgUnit 激进重构与重基线方案（一次性移除 SetID 与旧策略残余）

**状态**: 规划中（2026-04-21 11:56 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：在“项目仍处早期、无真实数据、无必须兼容的上下游”前提下，对 `orgunit` 采取一次性激进重构与 schema 重基线，直接移除 SetID、scope package/subscription、旧策略模块残余、相关 runtime/API/schema/tests/docs，并将 `orgunit` 收敛为仅保留组织树、有效期、审计、字段配置与纯 `orgunit` 写规则的最小活体模块。
- **关联模块/目录**：`modules/orgunit`、`internal/server`、`pkg/setid`、`internal/sqlc/schema.sql`、`migrations/orgunit`、`config/access`、`config/routing`、`config/errors`、`apps/web`、`docs/dev-plans`、`docs/dev-records`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/dev-plans/015-ddd-layering-framework.md`、`docs/dev-plans/017-routing-strategy.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/032-effective-date-day-granularity.md`、`docs/dev-plans/440-complete-setid-removal-plan.md`、`docs/dev-plans/441-legacy-strategy-module-residue-cleanup-plan.md`
- **用户入口/触点**：`/org/api/org-units`、`/org/api/org-units/details`、`/org/api/org-units/field-configs`、`/org/api/org-units/fields:options`、`/org/api/org-units/field-configs:enable-candidates`，以及所有现存 `/org/api/setid*` / `global-setids` 相关入口

### 0.1 Simple > Easy 三问

1. **边界**：`451` 是“早期项目条件下的 `orgunit` 一次性 reset 执行方案”，负责定义单列车实施顺序、停止线与重基线策略；`440` 仍是 SetID 根删除事实源，`441` 仍是旧策略残余清理事实源，`451` 不重新定义它们的概念 owner，只把它们在 `orgunit` 上的执行路径压成一次性落地。
2. **不变量**：不得保留 compat API、compat store、隐藏 resolver、历史 bootstrap 壳层、旧 schema 并存、旧字段继续对外暴露但置空值；删除后 `orgunit` 只保留纯组织语义与纯模块内规则。
3. **可解释**：维护者应能在 5 分钟内回答“`orgunit` 现在的唯一主流程是什么、字段决策依赖哪些事实、为何不再需要 `setid/resolved_setid/owner_setid/effective_policy_version` 这些旧语义”。

### 0.2 现状研究摘要

- **现状实现**：
  - `internal/server` 仍默认装配 `SetIDStore`，并注册 `/org/api/setids`、`/org/api/setid-bindings`、`/org/api/global-setids`。
  - `modules/orgunit` 仍公开 `SetIDPGStore` / `SetIDMemoryStore`，`pkg/setid` 仍直连 `ensure_setid_bootstrap(...)` 与 `resolve_setid(...)`。
  - `orgunit` 三套 precheck projection 仍显式产出 `resolved_setid`、`setid_source`、`effective_policy_version`、`mutation_policy_version`。
  - `field-config enable candidates` 与 `field options` 仍通过 SetID 上下文为字典选项回填 `setid/setid_source`。
  - `migrations/orgunit`、模块 schema 与 `internal/sqlc/schema.sql` 仍保留 SetID 表、函数、RLS、privileges、scope package/subscription 等整套底座。
- **现状约束**：
  - 当前仓库禁止 legacy 双链路，因此不能采用“旧接口保留但返回空数组/空字段”的做法。
  - 用户已明确本项目无真实数据、无必须兼容上下游，因此允许一次性重基线，不需要为历史数据编排回填或过渡兼容。
  - 新增数据库表仍需用户确认；本计划只允许删、改、重基线，不允许为过渡期新建映射表或兼容表。
- **最容易出错的位置**：
  - `create/append/maintain` 三套 precheck 合同。
  - `field-config enable candidates` 与 `fields:options` 仍依赖 SetID 上下文。
  - `internal/sqlc/schema.sql` 与 `migrations/orgunit` 若不同步重写，极易产生 schema 漂移。
  - `config/errors` / `authz` / `allowlist` / server tests 若漏删，会形成“协议已死、仓内仍活”的尾态。
- **本次不沿用的“容易做法”**：
  - 保留 `SetIDStore` 接口但不再装配。
  - 保留 `resolved_setid/setid_source` 字段但写死为 `DEFLT/none`。
  - 仅新增 drop migration 而继续保留旧基线文件。
  - 仅改名不删 `setid_context_resolver`、`setid_strategy_*` 等历史结构。

## 1. 背景与上下文

- 用户目标不是“继续分批收尾”，而是在早期项目窗口内对 `orgunit` 做一次性激进重构。
- 当前最大的工程问题不是目录结构，而是 `orgunit` 仍把 SetID 与旧策略残余当成运行时事实，从而让写前校验、字段候选、字典 options、server 装配和 DB kernel 都继续背负历史主链。
- 由于项目仍无真实数据、无外部依赖面，此时采用重基线比维护旧迁移与兼容壳层更简单，也更符合 `Simple > Easy`。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 一次性删除 `orgunit` 当前态 SetID runtime：`pkg/setid`、`SetIDStore`、server SetID API、resolver、memory/pg store、bootstrap helper。
- [ ] 一次性删除 `orgunit` 当前态 SetID schema：表、函数、RLS、privileges、scope package/subscription、global share 相关对象，并以无 SetID 的新 `orgunit` 基线替换旧 schema/migration/sqlc 输入。
- [ ] 一次性删除 `orgunit` 当前态旧策略残余：`setid_context_resolver`、旧命名、旧 explain/runtime 术语，以及任何仍显式依赖 `resolved_setid/setid_source/owner_setid` 的字段决策链路。
- [ ] 将 `orgunit` 收敛为仅依赖 tenant、org tree、effective date、field config、admin 权限、target existence/target event 这些纯模块内事实的主流程。
- [ ] 在单列车内同步删除相关测试、错误码、authz 对象、allowlist、前端契约与文档主线，避免删半套。

### 2.2 非目标

- [ ] 本计划不保留任何 SetID 或旧策略兼容层，不保留 alias route、compat DTO、compat schema、compat test。
- [ ] 本计划不为将来可能恢复 SetID 预留空壳接口、空目录、空 migration 或占位错误码。
- [ ] 本计划不为迁移历史数据编写 backfill/correction，因为当前前提明确为“无真实数据、允许重基线”。
- [ ] 本计划不在执行中引入新的平台治理层来替代 SetID；规则仅允许回收到 `orgunit` 模块内纯语义。

### 2.3 用户可见性交付

- **用户可见入口**：用户只看到 `orgunit` 自身页面/API，不再看到任何 SetID、global share、scope package/subscription、owner_setid、resolved_setid、策略模块相关入口或字段。
- **最小可操作闭环**：
  - 用户可完成 `orgunit` create
  - 用户可完成 `orgunit` append version
  - 用户可完成 `orgunit` maintain
  - 用户可查看并使用 `field-configs` 与 `fields:options`
- **显式破坏兼容说明**：现有任何包含 `resolved_setid` / `setid_source` / `setid` 字段的 `orgunit` 协议都允许在本计划中一次性删除，不视为回归。

## 2.4 工具链与门禁

- **命中触发器（勾选）**：
  - [X] Go 代码
  - [X] `apps/web/**` / presentation assets / 生成物
  - [X] i18n（仅 `en/zh`）
  - [X] DB Schema / Migration / Backfill / Correction
  - [X] sqlc
  - [X] Routing / allowlist / responder / 相关路由注册/映射
  - [X] AuthN / Tenancy / RLS
  - [X] Authz（Casbin）
  - [X] E2E
  - [X] 文档 / readiness / 证据记录
  - [X] 其他专项门禁：`no-legacy`、`no-scope-package`、`granularity`、`error-message`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `pkg/**` | 删除 `pkg/setid`；保留/补充纯 `orgunit` 解析与归一化测试 | `pkg/setid/**`、`pkg/orgunit/**` | 旧 helper 直接删，不保 compat 黑盒 |
| `modules/*/services` | 重写三套 precheck 与 mutation policy 的无 SetID 合同测试 | `modules/orgunit/services/*precheck*`、`orgunit_mutation_policy*` | 这里是主断言层 |
| `internal/server` | 删除 SetID API 测试；重写 `field-configs` / `org-units` 协议测试 | `internal/server/*setid*`、`orgunit_field_metadata_api*` | 只验证新协议，不写兼容断言 |
| `apps/web/src/**` | 删除页面/API client 中的 SetID 语义；重写受影响交互 | `apps/web/src/pages/**org**`、`api/**` | 若旧 UI 已无入口，直接删测试资产 |
| `E2E` | 仅保留无 SetID 的 `orgunit` 当前主流程验收 | `e2e/tests/**orgunit**` | 不保留“SetID 不存在”占位型 E2E |

## 3. 架构与关键决策

### 3.1 5 分钟主流程

删除完成后，`orgunit` 的唯一主流程为：

1. 用户从 `orgunit` 页面/API 发起 create / append / maintain。
2. server 仅负责 tenant、协议解析、错误映射、调用 `orgunit` service。
3. `orgunit` service 只基于 tenant、org tree、effective date、enabled field configs、target existence、admin 权限等事实做 precheck 与 mutation decision。
4. 写路径仍必须进入 DB Kernel `submit_*_event(...)`；读路径只读取 `orgunit` 自身表、读模型与函数，不再经过任何 SetID bootstrap、binding resolve、scope subscription、旧策略 registry。
5. 字段配置与 options 仅依赖 tenant-local dict/config，不再透出 `setid/setid_source`。

### 3.2 模块归属与职责边界

- **owner module**：`modules/orgunit`
- **交付面**：`internal/server` 作为协议层，`apps/web` 作为单前端入口
- **组合根落点**：`internal/server/handler.go` 只装配 `orgunit` 正式 store/service，不再装配 `SetIDStore`
- **禁止**：
  - server 自行 resolve SetID 上下文
  - field metadata API 回填历史 SetID 语义
  - `module.go` 继续暴露 `SetID*Store`

### 3.3 落地形态决策

- **形态选择**：
  - [ ] `A. Go DDD`
  - [X] `B. DB Kernel + Go Facade`
- **冻结边界**：
  - `451` 不得突破仓库级 `One Door`：`orgunit` 写入仍必须进入 DB Kernel `submit_*_event(...)`
  - `451` 不得引入第二写入口：不得把 create / append / maintain 改为 controller/service 直写表
  - `451` 的激进点仅限于删除 SetID / 旧策略污染层，不包括推翻 `orgunit` 现行写入骨架
- **选择理由**：本次目标是删除 SetID 与旧策略残余，而不是重做 `orgunit` 写架构。保留 DB Kernel + Go Facade，仍可一次性删除 SetID runtime/API/schema/tests/docs，同时不违反仓库硬不变量，也不破坏现行审计与事件链。

### 3.4 ADR 摘要

- **决策 1**：采用“一次性 reset + schema 重基线”，不采用“继续沿 440/441 分 PR 修尾巴”。
  - **备选 A**：延续 440/441 的增量收口。
  - **备选 B**：只删 API，不删 schema/runtime。
  - **选定理由**：当前前提允许破坏兼容，继续分批只会人为保留中间态。

- **决策 2**：`orgunit` 的字段决策与 precheck 不再依赖 SetID 解析上下文。
  - **备选 A**：保留 `resolved_setid` 但默认 `DEFLT`。
  - **备选 B**：把 SetID 改名为新的“context code”继续存在。
  - **选定理由**：这两种做法都只是换壳，不是删除历史主链。

- **决策 3**：对 `migrations/orgunit`、模块 schema 与 `internal/sqlc/schema.sql` 采取重基线，而不是在旧基线上继续叠加 drop migration。
  - **备选 A**：新增 drop migration，保留旧基线全部文件。
  - **选定理由**：当前仓库无真实数据；保留旧基线只会继续制造误引用和搜索噪音。

- **决策 4**：`effective_policy_version` 一次性删除，`policy_version` 收敛为唯一版本字段；若保留，则只表示纯 `orgunit` 配置快照版本，不再承载旧策略模块语义。
  - **备选 A**：继续保留 `effective_policy_version`
  - **备选 B**：两个字段都删除
  - **选定理由**：当前 server / service 已出现双字段分裂；继续并存只会扩大漂移。为降低一次性重构风险，先收敛为单字段 `policy_version`，再在后续计划中决定是否最终删除该字段。

### 3.5 Simple > Easy 自评

- **这次保持简单的关键点**：
  - 删除整条历史主链，而不是试图留壳
  - 只保留纯 `orgunit` 事实
  - schema、runtime、文档一次性同步切断
- **明确拒绝的“容易做法”**：
  - [X] legacy alias / 双链路 / fallback
  - [X] 第二写入口 / controller 直写表
  - [X] 页面或 API 继续返回历史字段空值
  - [X] 为过测临时加 compat store / compat error code
  - [X] 复制旧 SetID 逻辑改名后继续保留

## 4. 数据模型、状态模型与约束

### 4.1 新 `orgunit` 数据边界

- 保留：
  - `orgunit` 组织树与版本主表
  - 审计/事件/快照 presence 所需现行对象
  - `tenant_field_configs` 与现行字段配置对象
  - 与 `dict` 直接关联的 tenant-local 读取关系
- 删除：
  - `orgunit.setid_*`
  - `orgunit.global_setid_*`
  - `orgunit.*scope_package*`
  - `orgunit.*scope_subscription*`
  - 任何 `owner_setid` / `resolved_setid` / `setid_source` 持久化或协议映射对象

### 4.1A 不可突破的现行内核

- 必须保留 `submit_*_event(...)` 作为 `orgunit` 唯一写入口
- 必须保留 `orgunit.org_events` 作为唯一 append-only 审计事实源
- 必须保留 `orgunit.org_unit_versions` 作为业务 SoT
- 必须保留现行审计快照 / presence 机制所依赖的核心对象
- 允许删除的是 SetID / 旧策略相关字段、对象、payload 语义；不允许新增第二审计链，不允许把事件写入搬出 DB Kernel

### 4.2 时间语义

- `effective_date` / `as_of` 继续使用 `date`
- `created_at` / `updated_at` / `transaction_time` 继续使用 `timestamptz`
- 删除 SetID 后，任何“按 BU 命中 SetID 再派生策略”的时间语义一并删除，不再存在第二套 policy-as-of 解释链

### 4.3 迁移 / 重基线策略

- 不做历史数据 backfill
- 不做中间兼容表
- 允许重写 `migrations/orgunit/**` 中与 SetID/旧策略残余强绑定的基线文件
- 允许同步重写 `modules/orgunit/infrastructure/persistence/schema/**`
- 允许同步重写 `internal/sqlc/schema.sql`
- 重基线必须继续遵循 Atlas + Goose 闭环，不得脱离现行 `plan -> diff -> hash -> lint -> migrate up` 流水线
- `goose_db_version_orgunit` 继续作为唯一版本表，不新增第二套版本记账
- Phase C 固定执行顺序：
  1. 先重写 `modules/orgunit/infrastructure/persistence/schema/**`
  2. 再基于新 schema 生成新的 `migrations/orgunit` baseline
  3. 更新 `migrations/orgunit/atlas.sum`
  4. 执行 `make orgunit plan && make orgunit lint && make orgunit migrate up`
  5. 最后同步重写 `internal/sqlc/schema.sql` 并执行 `make sqlc-generate`
- 若需要保留 archive 证据，只允许进入 `docs/archive/` 或注释性记录，不得继续存在于当前态 schema 入口

### 4.4 协议字段收敛

- 写 API / precheck / tests / docs 统一删除 `effective_policy_version`
- `policy_version` 作为唯一版本字段保留在当前阶段，语义冻结为“纯 `orgunit` 配置快照版本”
- `policy_version` 不得再映射为任何 SetID / scope package / 旧策略 registry 版本
- `mutation_policy_version` 不再作为对外协议字段；若仍需保留，仅允许作为模块内实现常量或测试断言，不得继续作为 API / projection 合同的一部分
- `resolved_setid` / `setid_source` / `owner_setid` 在 API、projection、错误映射、测试断言中一次性删除，不保空值兼容

## 5. 激进打法与激进防范

### 5.1 激进打法（单列车）

1. [ ] 文档先收口：新增 `451`，并在 `440/441` 关联章节中说明 `orgunit` 一次性 reset 的执行路径。
2. [ ] 删除所有 SetID / 旧策略用户入口与协议层：server route、allowlist、authz object、错误码、前端请求层与页面残余。
3. [ ] 重写 `orgunit` 三套 precheck 合同，删除 `resolved_setid`、`setid_source`、旧 `effective_policy_version` 语义。
4. [ ] 重写 `field-config enable candidates` 与 `fields:options`，改为 tenant-local dict/config 取值，不再吃 SetID 上下文。
5. [ ] 删除 `pkg/setid`、`SetIDStore`、`setid_context_resolver`、`modules/orgunit` 的 SetID persistence 与 ports。
6. [ ] 重基线 `migrations/orgunit`、模块 schema 与 `internal/sqlc/schema.sql`，删掉 SetID / scope package / subscription / global share 整套对象。
7. [ ] 重写或删除所有相关测试、readiness、dev-record、文档主线引用。

### 5.2 激进防范（按本计划冻结）

1. [ ] 接受开发库/测试库直接重置，不为历史数据编排迁移兼容。
2. [ ] 接受 `orgunit` API 合同一次性破坏，不提供旧字段置空兼容。
3. [ ] 接受 `migrations/orgunit`、schema、sqlc 输入整体重写，不追求旧 hash 与旧文件连续性。
4. [ ] 接受测试大面积删改，但必须用新的主流程测试替代，不允许 skip 或 pending 壳层。
5. [ ] 若某条当前 `orgunit` 主流程在删 SetID 后无法解释，必须显式删掉该能力或补新契约；不得偷留历史 resolver。
6. [ ] 不接受突破 `One Door`、DB 闭环、`orgunit` 审计内核；若执行中碰到这三类边界，必须回到文档层先收敛，不得借重构名义顺手改架构。

## 6. 分阶段实施顺序与停止线

### 6.1 Phase A：协议与契约切断

- [ ] 删 SetID API / route / allowlist / authz
- [ ] 删 precheck projection 中历史字段
- [ ] 删 field metadata API 中 `setid/setid_source`
- [ ] 将写 API / precheck / docs 收敛为仅保留 `policy_version`，删除 `effective_policy_version`
- [ ] 将 `mutation_policy_version` 从 API / projection / docs 中移除；若保留，仅作为模块内实现常量
- [ ] 冻结 field metadata 新协议：
  - `field-configs:enable-candidates` 只接受 `enabled_on`，不再读取 `org_code`
  - `fields:options` 只接受 `field_key/as_of/q/limit`，不再从 `org_code` 推导上下文
  - 两个接口都不再返回 `setid/setid_source`

**停止线**
- `orgunit` create / append / maintain 的新协议必须已经确定；若前端/调用方仍依赖旧字段，不做兼容，直接同步改掉。

### 6.2 Phase B：模块与代码边界切断

- [ ] 删 `pkg/setid`
- [ ] 删 `modules/orgunit` 的 SetID ports/persistence/module exports
- [ ] 删 `setid_context_resolver` 与相关错误映射

**停止线**
- `orgunit` service 不得再 import / call 任何 SetID helper；若删后仍需某事实，必须证明它是纯 `orgunit` 事实。

### 6.3 Phase C：schema / sqlc / migration 重基线

- [ ] 先重写模块 schema 文件
- [ ] 再生成并重写 `migrations/orgunit`
- [ ] 更新 `migrations/orgunit/atlas.sum`
- [ ] 执行 `make orgunit plan && make orgunit lint && make orgunit migrate up`
- [ ] 最后重写 `internal/sqlc/schema.sql` 并执行 `make sqlc-generate`

**停止线**
- 生产代码中搜索不得再命中 SetID 当前态对象；若 schema 仍需某对象才能编译，说明前序代码边界未切干净，回退到 Phase B 继续删，不允许临时保留空表。
- 若 Phase C 需要绕开 `atlas.sum`、独立版本表或 DB gates 才能通过，视为方案失败，必须先修闭环，不得手工改库对齐。

### 6.4 Phase D：测试与文档封板

- [ ] 删所有 SetID / 旧策略残余测试
- [ ] 补齐无 SetID 的 `orgunit` 主流程测试
- [ ] 更新 AGENTS / dev-plans / readiness / E2E 总纲

**停止线**
- 若此阶段仍需依靠历史测试样本才能解释当前主流程，说明新合同未收敛，不得直接宣告完成。

## 7. 验收标准

1. [ ] 生产代码、当前态 schema、server route、authz、errors、前端入口搜索不再命中 `setid`、`resolved_setid`、`owner_setid`、`scope_package`、`scope_subscription`、`setid_strategy` 当前态语义。
2. [ ] `internal/server` 不再装配 `SetIDStore`，不再注册 `/org/api/setids`、`/org/api/setid-bindings`、`/org/api/global-setids`。
3. [ ] `modules/orgunit` 不再导出 `SetIDPGStore` / `SetIDMemoryStore` / `setid_governance`。
4. [ ] `orgunit` create / append / maintain / field-configs / field options 在无 SetID 条件下仍可运行。
5. [ ] `migrations/orgunit`、模块 schema 与 `internal/sqlc/schema.sql` 已完成无 SetID 的新基线重写。
6. [ ] `AGENTS.md` 文档地图已能发现 `451`，且不再把 SetID/旧策略残余当作 `orgunit` 现行前提。
7. [ ] `orgunit` 写入仍通过 DB Kernel `submit_*_event(...)`，未引入第二写入口。
8. [ ] `make orgunit plan && make orgunit lint && make orgunit migrate up && make sqlc-generate` 可跑通，`atlas.sum` 与生成物已提交。
9. [ ] `orgunit.org_events` / `orgunit.org_unit_versions` / 审计快照 presence 主干未被突破，仅删除 SetID / 旧策略污染层。
10. [ ] `field-configs:enable-candidates` 与 `fields:options` 已按 tenant-local 契约收敛：不再依赖 `org_code`，也不再返回 `setid/setid_source`。
11. [ ] `effective_policy_version` 与 `mutation_policy_version` 已不再作为对外协议字段存在；若仍保留内部常量，不得继续出现在 API / projection / 文档契约中。

## 8. 与 440 / 441 的关系

- `440` 仍是 SetID 根删除唯一 PoR。
- `441` 仍是旧策略残余清理 PoR。
- `451` 不推翻 `440/441` 的事实源定位；`451` 的作用是：在“无真实数据、允许重基线”的窗口条件下，将二者在 `orgunit` 上的执行面压缩为一次性 reset 列车。
- 若 `451` 与 `440/441` 出现冲突，不得由 `451` 直接覆盖 owner 文档；必须先修订 `440/441`，再回写 `451`，保持 SetID 根删除与旧策略残余清理的唯一事实源关系不漂移。

## 9. 交付物

- [ ] `docs/dev-plans/451-orgunit-aggressive-reset-no-setid-no-legacy-strategy-plan.md`
- [ ] 后续对应 readiness：`docs/dev-records/DEV-PLAN-451-READINESS.md`
- [ ] 关联文档更新：`AGENTS.md`、`DEV-PLAN-440`、`DEV-PLAN-441`、必要时 `DEV-PLAN-060`
