# DEV-PLAN-028：SetID 管理（Greenfield）

**状态**: 部分完成（009M1：最小闭环已落地；2026-01-06 23:40 UTC）

> 适用范围：**Greenfield 全新实现**（路线图见 `DEV-PLAN-009`）。  
> 本文研究 PeopleSoft 的 SetID 机制，并提出引入 SetID 的最小可执行方案：在同一租户内实现“主数据按业务单元共享/隔离”的配置能力，且可被门禁验证，避免实现期各模块各写一套数据共享规则导致漂移。

## 1. PeopleSoft SetID 机制：作用与目的（摘要）

### 1.1 SetID 是什么
- **SetID** 是一个短标识（PeopleSoft 习惯为 5 位字符），作为大量“基础主数据表”的关键字段之一，用于把主数据划分为不同的数据集（Set）。
- **同一个 SetID 下的主数据**可以被多个业务单元（Business Unit）共享；不同 SetID 之间则天然隔离（可存在“同编码不同含义”的并行配置）。

### 1.2 解决的问题（为什么需要）
- **共享 vs 隔离**：同一集团内，多 BU 需要共享一套通用字典（如 Job Code、Location），同时又要允许某些 BU 拥有本地化差异（如部门、工资等级规则）。
- **避免复制**：不用为每个 BU 复制整套字典表；通过 SetID 选择“用哪一套”即可。
- **可控的一致性**：通过中心化配置（Set Control）约束每个 BU 在每类主数据上使用哪个 SetID，减少“自由组合”导致的数据漂移。

### 1.3 关键配套概念（PeopleSoft 的核心结构）
- **Set Control Value**：用于“选择 SetID”的控制维度，PeopleSoft 常用 Business Unit 作为 set control value。
- **Record Group**：把一组主数据表归为同一类（同一组共享同一个 SetID 选择），避免每张表单独配置。
- **Set Control（映射）**：`(business_unit_id, record_group) -> setid` 的确定性映射，保证每次查询/写入都能稳定得到唯一 SetID。

> 引入 SetID 的目标是复用上述“确定性映射 + 共享/隔离”的思想，而不是复刻 PeopleSoft 的全部页面/术语与历史包袱。

## 2. 背景与上下文 (Context)

- Greenfield 全新实施（路线图见 `DEV-PLAN-009`），需要在早期冻结“主数据共享/隔离”的权威机制，否则后续各模块会用不同的方式表达同一需求（例如：用 orgunit path、用 tenant 级别全局表、用 hardcode 前缀等）。
- SetID 属于“数据建模/组织治理”的横切能力，主要影响 `jobcatalog/orgunit/staffing` 的主数据读取与配置 UI。

## 3. 目标与非目标 (Goals & Non-Goals)

### 3.1 核心目标

- [X] 引入 SetID 作为“同租户内的主数据数据集”能力：同一编码可在不同 SetID 下并行存在。
- [X] 引入 **Set Control**：对每个控制值（后续对齐 Business Unit）和每个 Record Group，稳定映射到唯一 SetID（无歧义、可测试）。
- [X] 为主数据表提供一致的建模约束：`tenant_id + setid + business_key + valid_time(date)`。
- [X] 提供最小管理入口（API + UI）：创建/禁用 SetID、配置 set control value、维护映射矩阵。
- [X] 将关键约束固化为可执行门禁（tests/gates），避免实现期 drift。

已落地范围（009M1，最小闭环）：
- schema/迁移：`modules/orgunit/infrastructure/persistence/schema/00005_orgunit_setid_schema.sql`、`modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`、`migrations/orgunit/20260106100000_orgunit_setid_schema.sql`、`migrations/orgunit/20260106100500_orgunit_setid_engine.sql`
- 共享解析入口：`pkg/setid/setid.go`
- UI 入口：`/org/setid`（实现：`internal/server/setid.go`；allowlist：`config/routing/allowlist.yaml`）
- 证据：`docs/dev-records/DEV-PLAN-010-READINESS.md`（第 10 节）

### 3.2 非目标（明确不做）

- 不做跨租户共享 SetID（RLS/tenant 是硬边界，SetID 仅用于同租户内共享/隔离）。
- 不做“多 SetID 合并视图/union”（一次查询只使用一个解析出的 SetID；不引入层级继承或叠加规则）。
- 不做 PeopleSoft 全量 UI 复刻；只保留需要的最小配置与可验证性。

## 4. 工具链与门禁（SSOT 引用）

> 本计划不复制命令矩阵；触发器与门禁以 SSOT 为准。

- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 模块边界（将影响哪些模块）：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- Tenancy/AuthN 与主体模型：`docs/dev-plans/019-tenant-and-authn.md`
- SuperAdmin 控制面认证与会话：`docs/dev-plans/023-superadmin-authn.md`
- RLS 强租户隔离：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- Valid Time=DATE 口径：`docs/dev-plans/032-effective-date-day-granularity.md`

## 5. 关键决策（ADR 摘要）

### 5.0 5 分钟主流程（叙事）

1) 业务写入/按 BU 上下文读取列表时，调用方必须显式提供 `business_unit_id`（Set Control Value）与业务 payload（或查询条件）。  
2) 系统按 `(tenant_id, business_unit_id, record_group) -> setid` 解析得到唯一 `setid`（映射无缺省洞）。  
3) 写入的主数据记录必须落库 `setid`；列表读取按解析出的 `setid` 过滤；单条读取必须使用能唯一定位记录的 key（通常包含 `setid`），以记录自身 `setid` 为准（不重新解析）。  
4) Set Control 映射不做有效期：变更只影响“未来的解析结果”，历史记录因已落库 `setid` 而可复现。  
5) 解析入口必须单点权威（禁止模块自建回退/默认值逻辑），否则门禁阻断。

### 5.1 SetID 的边界（选定）

- **选定**：SetID 是 **tenant 内** 的“数据集选择”机制，不承担租户隔离或权限边界。
- **选定**：所有 setid-controlled 的数据表必须包含 `tenant_id`（RLS）与 `setid`（共享/隔离），且 SetID 不参与 RLS 策略（避免把治理机制误用为安全机制）。

### 5.2 SetID 字段合同（选定：5 字符、全大写）

- **选定**：`setid` 为 `CHAR(5)`（或等价约束），值为 `[A-Z0-9]{1,5}`，存储为全大写。
- **保留字**：`SHARE` 作为每个 tenant 的默认 SetID（不可删除/不可禁用；可用于“无 BU 差异”的主数据）。

### 5.3 Record Group（选定：稳定枚举，禁止运行时自由造组）

> 目的：把“哪些表受 SetID 控制”收敛为可审计的列表，避免模块各自发明分类导致不可验证。

- **选定**：Record Group 为稳定枚举（代码侧 + DB 侧约束），新增 group 必须走 dev-plan 并补齐门禁。
- **选定（冻结）**：Record Group 不做运行时 enable/disable；稳定枚举列表即“启用清单”。如未来需要停用，必须另起 dev-plan 并补齐迁移/门禁策略。
- **MVP group**（可扩展，但必须从最小集合开始）：
  - `jobcatalog`：职位分类主数据（Job Family/Job Profile/Level 等）
  - `orgunit`：组织基础主数据（部门/地点等，按实际建模落地）

### 5.4 Set Control Value（选定：抽象为“控制值”，后续对齐 Business Unit）

- **选定（冻结）**：Set Control Value 即 **Business Unit**，以稳定标识 `business_unit_id` 表达，用于驱动映射：
  - `(tenant_id, business_unit_id, record_group) -> setid`
- **约束**：
  - `business_unit_id` 必须可枚举、可在 UI 中显式选择；禁止在业务代码里“从路径/会话/环境推导”隐式生成。
  - `business_unit_id` 的来源与生命周期由 OrgUnit/租户治理承接（本计划只冻结：写入/读取必须显式携带该字段）。
  - **字段合同（冻结）**：`business_unit_id` 值为 `[A-Z0-9]{1,5}`，存储为全大写；展示名使用 `name`，不允许“改 key”。

### 5.5 SetID 解析算法（选定：确定性、无缺省洞）

- **选定**：Set Control 映射必须“无缺省洞”：每个 `(business_unit_id, record_group)` 都有映射（初始化时自动填充为 `SHARE`），从而避免运行时出现“缺映射时怎么办”的分支漂移。

### 5.6 Set Control 映射的时间语义（选定：不做有效期）

- **选定（冻结）**：Set Control 映射不做有效期（不引入 `effective_on/end_on`）。
- **后果（必须接受）**：
  - 映射变更会影响“未来的解析结果”（例如后续创建/读取列表的默认集合）。
  - 历史可复现性必须依赖“业务记录落库 `setid`”这一不变量：任何对单条记录的读取都以记录自身 `setid` 为准。

### 5.7 权威入口与依赖方向（冻结：避免跨模块漂移）

> 目的：把“SetID 解析/写入口/失败模式”冻结为可执行契约，避免实现阶段各模块各写一套。

- **选定（冻结）**：SetID 解析是一个“横切能力”，Go 侧实现放在 `pkg/setid`（或同等共享包）中，供各模块复用；禁止模块间互相 import。
- **选定（冻结）**：解析必须在“显式事务 + 租户注入（RLS context）”下执行；解析函数签名显式接收 `tx`（或等价）以满足 No Tx, No RLS。
- **选定（冻结）**：解析的唯一数据源是 Set Control 映射表；禁止任何“缺映射时默认 SHARE/回退到 tenant 全局/从 path 推导”等逻辑。
- **选定（冻结）**：所有 setid-controlled 的**写入口**必须走 DB Kernel 的单点写入口（One Door）：
  - 对业务主数据：依旧遵循各模块 `submit_*_event(...)` 事件入口。
  - 对 SetID/BU/Mapping 自身：提供专用的 kernel 写入口（形式见 11.4），同样要求单点入口 + 可门禁验证。

### 5.8 失败模式与错误契约（冻结：先写清再实现）

> 目的：避免实现阶段“为了跑通”临时补分支，导致 Easy 式隐性复杂度。

SetID 解析 `ResolveSetID(tenant_id, business_unit_id, record_group)` 的失败模式（均为 **fail-closed**）：
- `SETID_MAPPING_MISSING`：映射缺失（理论上被“无缺省洞”门禁阻断，但仍需定义错误用于数据损坏/手工改库等场景）。
- `BUSINESS_UNIT_DISABLED`：控制值（BU）已禁用。
- `RECORD_GROUP_UNKNOWN`：record group 不在稳定枚举内。
- `SETID_DISABLED`：映射指向的 SetID 已禁用。
- `RLS_TENANT_MISMATCH` / `TX_REQUIRED`：租户上下文不一致或缺少显式事务（用于强制不变量）。

写入路径禁止客户端直接提交 `setid`；若检测到绕过解析入口（例如 API payload 携带 `setid` 字段或直接写表），必须返回 `SETID_WRITE_BYPASS_FORBIDDEN`（或等价错误）并由门禁覆盖。

### 5.9 Bootstrap / 初始化（冻结：保证首条写入可用）

> 目的：避免“需要 BU 才能写入，但 BU/mapping 又不存在”的循环依赖；让“无缺省洞”从新租户开始即成立。

- **选定（冻结）**：tenant provision 时必须初始化：
  - `setid=SHARE`（enabled，保留字，不可删除/不可禁用）。
  - 至少 1 个 `business_unit_id`（建议系统默认创建 `BU000`，展示名可后改）。
  - 对所有 Record Group（稳定枚举）补齐 `mapping=(business_unit_id, record_group)->SHARE`。
- **选定（冻结）**：正常 tenant API 不提供“修复写 SHARE/补洞”能力；仅允许在租户创建/修复流程（对齐 `DEV-PLAN-023/019` 的系统入口）中执行，以避免绕过保留字与门禁。

## 6. 数据契约（Schema/约束级别）

> 本节定义“横切不变量”；具体表名与落点由各模块实现 dev-plan 承接。

### 6.1 基础表（最小集合）

- `setids`：`(tenant_id, setid, name, status, created_at, updated_at)`
- `business_units`：`(tenant_id, business_unit_id, name, status, created_at, updated_at)`
- `set_control_mappings`：`(tenant_id, business_unit_id, record_group, setid, created_at, updated_at)`

> 说明：**实施阶段新增表/迁移前必须获得手工确认**（遵循仓库合约），本计划仅冻结契约与字段语义。

### 6.2 主数据表通用约束（所有 setid-controlled 表必须满足）

- `setid` 的值必须存在于 `setids`，且写入时需通过 set control 解析得到（禁止调用方任意填 `setid`）。
- **唯一性/主键合同（冻结）**：
  - 若实体是 **effective-dated**（采用 versions + Valid Time 语义，见 `DEV-PLAN-032`）：唯一性必须包含 `tenant_id + setid + business_key + valid_time`（或等价的不重叠约束）。
  - 若实体不是 effective-dated：唯一性必须包含 `tenant_id + setid + business_key`。

### 6.3 DB 级约束（建议：可门禁验证）

> 本节不是实现细节，而是为了让“无缺省洞/禁止绕过/格式合同”等不变量可被 DB 与门禁共同强制。

- `setid` 格式合同：`[A-Z0-9]{1,5}` 且存储为全大写；`SHARE` 必须存在且不可删除/不可禁用。
- `business_unit_id` 格式合同：`[A-Z0-9]{1,5}` 且存储为全大写。
- `set_control_mappings` 必须满足：
  - `UNIQUE (tenant_id, business_unit_id, record_group)`（确保解析唯一）。
  - `FOREIGN KEY (tenant_id, setid)` 指向 `setids`（禁止指向不存在的 SetID）。
  - `FOREIGN KEY (tenant_id, business_unit_id)` 指向 `business_units`（禁止指向不存在的 BU）。
  - `record_group` 受稳定枚举约束（Postgres enum 或 check constraint）。
- RLS：上述三类表必须启用 `tenant_id = current_setting('app.current_tenant')` 的强隔离；SetID 不参与 RLS（避免误用为安全边界）。

## 7. API 与 UI（最小管理面）

### 7.1 路由归属与命名空间（对齐 DEV-PLAN-017）

- SetID 属于“组织治理/主数据治理”横切配置，但其控制维度是 Business Unit；为避免出现多处 owner，**选定管理面归属 `orgunit`**，内部 API 使用 `/{module}/api/*`：
  - `GET/POST /orgunit/api/setids`
  - `GET/POST /orgunit/api/business-units`
  - `GET/PUT /orgunit/api/setid-mappings`（批量矩阵更适合 PUT）
- 写入与列表读取必须显式携带 `business_unit_id`（禁止从 path/session 推导）。

### 7.2 API 契约（冻结：字段、错误、幂等）

> 说明：具体 route_class/responder 口径以 `DEV-PLAN-017` 为准；此处只冻结业务契约与错误语义。

**SetID**
- `POST /orgunit/api/setids`
  - Request：`{ "setid": "A0001", "name": "Default A", "request_id": "..." }`
  - Rules：`setid` 必须满足格式合同；`SHARE` 保留字不可用作创建参数（仅系统初始化/修复入口可写）。
  - Errors：`SETID_INVALID_FORMAT`、`SETID_RESERVED`、`SETID_ALREADY_EXISTS`、`RLS_TENANT_MISMATCH`
- `POST /orgunit/api/setids/{setid}/disable`
  - Request：`{ "request_id": "..." }`
  - Errors：`SETID_IN_USE`（仍被 mapping 引用）、`SETID_RESERVED`（`SHARE`）、`SETID_NOT_FOUND`

**Business Unit（Set Control Value）**
- `POST /orgunit/api/business-units`
  - Request：`{ "business_unit_id": "BU001", "name": "BU 001", "request_id": "..." }`
  - Rules：创建时必须为所有 record group（稳定枚举）自动补齐映射为 `SHARE`（保证无缺省洞）。
  - Errors：`BUSINESS_UNIT_ALREADY_EXISTS`、`BUSINESS_UNIT_INVALID_ID`
- `POST /orgunit/api/business-units/{business_unit_id}/disable`
  - Errors：`BUSINESS_UNIT_IN_USE`（仍被业务引用，策略见 11.3）、`BUSINESS_UNIT_NOT_FOUND`

**Mapping Matrix**
- `GET /orgunit/api/setid-mappings?record_group=jobcatalog`
  - Response：`{ "record_group": "jobcatalog", "rows": [ { "business_unit_id": "BU001", "setid": "SHARE" } ] }`
- `PUT /orgunit/api/setid-mappings`
  - Request：`{ "record_group": "jobcatalog", "mappings": [ { "business_unit_id": "BU001", "setid": "A0001" } ], "request_id": "..." }`
  - Rules：必须拒绝把启用 BU 指向 disabled setid；更新需事务化并可串行化（建议 tenant 级 advisory lock）。
  - Errors：`SETID_DISABLED`、`BUSINESS_UNIT_DISABLED`、`RECORD_GROUP_UNKNOWN`、`SETID_MAPPING_MISSING`（若要求全量矩阵提交）

**读取契约（冻结：消除歧义）**
- 列表读取：必须显式提供 `business_unit_id`，先 `ResolveSetID(...)` 后按解析得到的 `setid` 过滤。
- 单条读取：必须用包含 `setid` 的 key（例如 `(setid, business_key)` 或 record id），或显式携带 `setid`；禁止仅凭 `business_key` 且不提供 `setid`/`business_unit_id`。
- 若用 `business_unit_id` 解析后按 `business_key` 读取，其语义是“按该 BU 当前配置读取”，不承诺历史复现（历史复现依赖记录落库的 `setid`）。

### 7.3 UI（最小交互）

- SetID 列表：创建/禁用/重命名（禁用需校验是否被映射引用）。
- Business Unit 列表：创建/禁用。
- 映射矩阵：按 record group 展示一张“控制值 × group -> setid”的矩阵，默认初始化为 `SHARE`。

**最小交互闭环（对齐“可发现、可操作”）**
- 导航入口：OrgUnit → Governance → SetID（或同等可发现入口）。
- SetID 列表：新增/禁用（禁用前校验引用）；重命名（仅影响展示名，不影响 key）。
- Business Unit 列表：新增/禁用（禁用策略见 11.3）。
- 映射矩阵：选择 record group → 展示 BU 行 → 下拉选择 setid → 保存（`PUT` 批量提交，避免逐格写入导致并发漂移）。

## 8. 门禁（Routing/Data/Contract Gates）(Checklist)

1. [ ] SetID 合同门禁：`setid` 只能是 1-5 位大写字母/数字；`SHARE` 必存在且不可删除/不可禁用。
2. [ ] 映射完整性门禁：任意启用的 `business_unit_id` 对每个 record group（稳定枚举）必须存在映射（无缺省洞）；且每个 tenant 至少存在 1 个启用 BU（默认 `BU000`）。
3. [ ] 解析入口门禁：代码库内只能存在一个权威解析入口（共享包），业务写入路径必须通过该入口获取 `setid`。
4. [ ] 写入口门禁：任何 setid-controlled 写入必须走“解析 setid + 写入”的单一入口（禁止 payload 直接带 `setid`；禁止直接写表）。
5. [ ] 引用完整性门禁：`setid-mappings` 不得指向 `disabled setid`；禁用/删除 SetID 时若仍被引用必须阻断。
6. [ ] 失败模式门禁：缺映射/禁用/非法 group 等错误必须覆盖到测试，且为 fail-closed（不得隐式回退 SHARE）。

## 9. 实施步骤 (Checklist)

1. [ ] 落地 tenant bootstrap（见 5.9）：初始化 `SHARE` + 默认 BU + 完整 mapping 矩阵。
2. [ ] 明确 record group 的初始清单（`jobcatalog/orgunit`），并在实现模块的 dev-plan 中声明“哪些表受控于哪个 group”。
3. [ ] 冻结共享包 API：`ResolveSetID(tx, tenant_id, business_unit_id, record_group) -> setid`（含错误契约），并约束所有模块复用。
4. [ ] 实现 SetID/BU/Mapping 的 DB Kernel 单点写入口 + 读 API（含幂等 `request_id` 语义）。
5. [ ] 实现 “无缺省洞” 的初始化与演化路径：
   - [ ] 新增 BU 时：为所有 record group 自动补齐 mapping=`SHARE`。
   - [ ] 新增 record group 时：为所有 BU 自动 backfill mapping=`SHARE`。
6. [ ] 为主数据写入路径接入 setid 解析（先从 `jobcatalog` 起步，形成样板）。
7. [ ] 落地门禁（tests/gates）并接入 CI required checks。
8. [ ] 补齐 UI 管理面（最小可用），并在后续模块实现中复用同一套解析入口与 contracts。

## 10. 验收标准 (Acceptance Criteria)

- [ ] 同一 tenant 内可配置多个 SetID，并能在同一业务键下并行维护多套主数据（按 SetID 隔离）。
- [ ] 给定 `(business_unit_id, record_group)`，系统能稳定解析出唯一 SetID（无缺省洞）。
- [ ] 任何绕过解析入口直接写 setid 的路径会被门禁阻断。
- [ ] 新 tenant 初始化后即满足：`SHARE` + 至少 1 个 BU + 完整 mapping（默认 `SHARE`），无需手工补洞。
- [ ] `jobcatalog` 至少一个主数据实体完成端到端接入（解析→写入→读取→UI 展示）。
- [ ] 示例验收：同一 `code` 在 `setid=A0001` 与 `setid=B0001` 并存；BU1 映射到 A0001、BU2 映射到 B0001；两 BU 的列表读取互不串数据；单条读取能通过记录自身 `setid` 精确定位。

## 11. 已决策（本轮评审后冻结）

### 11.1 SetID/BU/Mapping 的“模块归属”与 DB schema

- **选定：方案 A**：归属 `orgunit`（DB schema/HTTP 管理面都在 orgunit），并提供 `pkg/setid` 作为跨模块共享解析入口。
  - 理由：Set Control Value 冻结为 Business Unit，本质属于 orgunit 治理；解析能力用 `pkg/**` 复用，避免 Go 跨模块 import；最少新增“平台层”概念。
- **不选（暂不做）**：归属 `iam`（作为平台治理能力），orgunit 只是 UI/数据来源之一。
  - 理由：SetID 是横切配置，放平台更“概念纯”；代价是需要额外定义跨模块 UI/路由 owner 与依赖边界，早期容易引入第二套抽象。

### 11.2 Set Control Values（Business Unit）是否复用 orgunit 现有实体

- **选定：方案 A**：Set Control Value 直接等价为 “Business Unit（orgunit 的一种实体）”，以 `business_units` 作为 BU 的权威表；SetID 管理面只提供对 BU 的“启用/禁用与显示名”（不改 key）。
  - 理由：避免早期出现两套 BU（orgunit 一套、setid 一套）导致漂移；对齐“同一概念只有一种权威表达”。
- **不选（暂不做）**：Set Control Value 独立成表（`set_control_values`），与 orgunit BU 通过外键或同步保持一致。
  - 理由：实现更独立、推进更快；代价是需要额外的同步/一致性策略与退场计划，属于偶然复杂度。

### 11.3 禁用策略：禁用 BU/SetID 时如何处理存量引用

- **选定：方案 A（保守）**：禁止禁用仍被引用的 BU/SetID（fail-closed），必须先迁移 mapping/业务引用再禁用。
  - 理由：契约最清晰，可预测；避免出现“禁用后读不到/写不了但数据还在”的僵尸状态。
- **不选（暂不做）**：允许禁用但保留历史可读，且禁止新写入（需要明确读写差异与 UI 提示）。
  - 理由：操作更灵活；代价是引入状态机与更多失败路径，需更强的门禁与可解释性。

### 11.4 SetID/BU/Mapping 的 DB Kernel 写入口形态（One Door 对齐）

- **选定：方案 A**：为 SetID/BU/Mapping 也提供 `submit_*_event(...)` 风格入口，并在同事务内同步投射到配置表（与 `orgunit.submit_org_event(...)` 的“事件 SoT + 同步投射”模式一致）。
  - 理由：最大化对齐仓库 One Door 不变量；天然具备幂等/审计；避免未来引入第二套“非事件写入口”标准。
  - 代价：需要为配置域定义最小事件类型与投射函数（但规模可控）。
- **不选（暂不做）**：仅提供“直接写配置表”的 kernel 函数（例如 `upsert_setid(...)` / `put_setid_mappings(...)`），不做事件 SoT。
  - 理由：实现更直接；代价是与主干写模型出现分叉，需要额外说明为何不违反 One Door（并补齐审计/幂等策略）。

## 12. Simple > Easy（DEV-PLAN-003）停止线

- [ ] 任何模块各自实现“SetID 解析/默认值/回退规则”，而不是复用单一权威入口。
- [ ] 允许调用方自由传入 setid 并绕过 set control（会导致不可审计的漂移）。
- [ ] 把 SetID 误用为安全隔离（跨租户/权限）机制，破坏 RLS/Authz 的边界。
