# DEV-PLAN-028：SetID 管理（Greenfield）

**状态**: 进行中（2026-01-24 04:25 UTC）— 已被 DEV-PLAN-070 取代，后续以 070 为准

> **重要**：本文件仅作为历史记录保留，不再作为实现/评审/验收契约。所有涉及 `business_unit_id` / `record_group` 的设计与口径均已废弃，必须以 `docs/dev-plans/070-setid-orgunit-binding-redesign.md` 为 SSOT。

> 本计划作为历史记录保留，新的 SetID 方案以 `docs/dev-plans/070-setid-orgunit-binding-redesign.md` 为准。

> 适用范围：**Greenfield 全新实现**（路线图见 `DEV-PLAN-009`）。  
> 本文研究 PeopleSoft 的 SetID 机制，并提出引入 SetID 的最小可执行方案：在同一租户内实现“主数据按业务单元共享/隔离”的配置能力，且可被门禁验证，避免实现期各模块各写一套数据共享规则导致漂移。

## 0. 评审补丁（2026-01-12）

> 目的：把“本轮评审新增功能（NEW）/原计划缺口（GAP）/口径澄清（CLARIFY）”显式标注出来，避免读者误判范围变更。

### 0.1 新增功能（NEW）

- [ ] **UI：显式 Business Unit**：任何 setid-controlled 的 UI 入口（例如 `/org/job-catalog`）若缺少 `business_unit_id`，必须 `302` 重定向补齐默认 `BU000`（最终请求必须显式携带 `business_unit_id`；禁止 silent default）。
- [X] **Staffing：Position 必填 BU**：`position` 的创建事件必须携带 `business_unit_id`，并由 DB Kernel/UI/API 共同强制（保证“人员→任职→岗位→BU”的可推导链路；也避免后续接入 setid 解析时出现不可判定上下文）。
- [ ] **（可选，非 P0）Tree Controls**：当出现“某 BU 需要访问不属于其 record group 解析 setid 的树/层级配置”的需求时，引入 Tree Controls 映射（契约见 5.3.1）。

### 0.2 实现缺口（GAP：原计划已包含但尚未补齐）

- [ ] **Record Group**：`orgunit` 尚未在 DB 约束/bootstrap/UI 中落地（现状仅 `jobcatalog`）；需补齐稳定枚举、bootstrap backfill、门禁与接入样板。
- [ ] **管理面能力**：SetID/BU 的 `RENAME/DISABLE` 与“多 record group 映射矩阵”尚未补齐（现状管理面仅支持 `jobcatalog` 的 create + mapping）。
- [ ] **门禁覆盖**：SetID 合同/无缺省洞/禁止绕过等需补齐 tests/gates 覆盖（目标见 §8）。

### 0.3 口径澄清（CLARIFY：新增说明，不引入新功能）

- PeopleSoft 的 TableSet Security（按 SetID 做访问控制）在本项目不实现；SetID 不作为权限边界（见 3.2）。
- PeopleSoft 的 TableSet Tree Control 在 v1 不作为必需能力；除非出现“跨 setid tree 访问”的明确需求，否则不落地（如需落地，按 0.1 的 NEW-TreeControls 承接）。
- `business_units` 与 `setids` 必须同时存在：BU 是业务上下文/控制值，SetID 是共享数据集；只用 SetID 会丢失“多 BU 共享同一 SetID/按 record group 选择 SetID”的表达能力（详见 11.2.1）。

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

### 1.4 TableSet Control（Record Group + Tree）

- PeopleSoft 的核心“发生作用”点是 PeopleTools 的 **TableSet Control**：
  - **Record Group**：在 TableSet Control - Record Group（组件：`SET_CNTRL_TABLE1`）里，为每个 Set Control Value（常见=Business Unit）配置各 record group 对应的 SetID。
  - **Tree**：在 TableSet Control - Tree（组件：`SET_CNTRL_TABLE2`）里登记“树名/树对象”的 SetID，以支持“tree 的 SetID 与 BU 默认 SetID 不一致”时仍可在 prompt/处理过程中可见。
- PeopleSoft 的“Default SetID / preliminary tableset sharing”也会在 BU 建立/配置过程中出现：用于引导该 BU 的初始 tableset sharing 配置。

### 1.5 TableSet Security（按 SetID 的访问控制，可选）

- PeopleSoft 在 FSCM 等域提供 **TableSet Security**（例如按 Permission List 或 User 授权可访问的 SetID），从而影响页面可选 SetID、prompt 结果与可见数据范围。
- 本项目将此能力显式排除在 v1（见 3.2），避免把治理机制误用为授权边界。

### 1.6 SetID 如何在运行时“发生作用”（机制细化）

> 这里描述的是 PeopleSoft 的典型实现思路：**Set Control Value（常见=Business Unit）驱动“每类主数据（Record Group）用哪个 SetID”**，从而让页面 prompt/校验/处理过程在不暴露复杂配置的情况下稳定使用同一套数据集。

1) **确定 Set Control Value**：通常来自交易/业务上下文中的业务单元字段（如 BU），系统在组件缓冲区里“先拿到 BU”。  
2) **确定 Record Group**：把一批 setid-controlled 表归入同一组（Record Group），从而避免“每张表都配置一次映射”。  
3) **查表得到 SetID**：按 `(Set Control Value, Record Group) -> SetID` 查到唯一 SetID（PeopleSoft 常见落点是 `PS_SETCONTROL` 等表）。  
4) **在 prompt/校验/处理时使用 SetID**：对 setid-controlled 表的查询都带上 `WHERE SETID = <resolved>`（或等价 join/view），保证同一 BU 在同一类主数据上使用同一套数据集。

### 1.7 PeopleSoft vs 本项目 SetID：关键差异与取舍

- **控制值简化**：PeopleSoft 可有多种 Set Control Value/Set Control Record；本项目冻结为 **BU（`business_unit_id`）唯一控制值**（见 5.4），降低早期漂移风险。
- **映射完备性**：PeopleSoft 允许配置/默认值的历史包袱较多；本项目选定“无缺省洞”（见 5.5/5.9），并要求 fail-closed。
- **显式上下文**：PeopleSoft 运行时通常隐式携带 BU；本项目要求 UI/API **显式携带 `business_unit_id`**，默认值必须通过 redirect 写回 URL（避免 silent default）。
- **安全边界**：PeopleSoft 可选 TableSet Security；本项目明确 **不把 SetID 用作权限边界**（RLS/Casbin 承担安全语义）。
- **Tree 控制**：PeopleSoft 的 Tree Controls（`SET_CNTRL_TABLE2`）可选；本项目 v1 不做，避免偶然复杂度（仅保留扩展点）。
- **时间语义**：PeopleSoft 的主数据多为 effective-dated；本项目 Valid Time 统一收敛为 day 粒度（`DEV-PLAN-032`），Set Control 映射不做有效期（见 5.6）。

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
- 现状限制：record group 的 DB 约束 + 管理面当前仅覆盖 `jobcatalog`（`orgunit` 等 group 仍待补齐）
- 证据：`docs/dev-records/DEV-PLAN-010-READINESS.md`（第 10 节）

### 3.2 非目标（明确不做）

- 不做跨租户共享 SetID（RLS/tenant 是硬边界，SetID 仅用于同租户内共享/隔离）。
- 不做“多 SetID 合并视图/union”（一次查询只使用一个解析出的 SetID；不引入层级继承或叠加规则）。
- 不做 PeopleSoft 全量 UI 复刻；只保留需要的最小配置与可验证性。
- **（新增澄清）不做 TableSet Security**：不提供按 SetID 的访问控制/授权模型（PeopleSoft 的 TableSet by Permission List/User ID）；若未来需要，必须另立 dev-plan，并与 Casbin/RLS 的边界协同设计。
- **（新增澄清）v1 不做 Tree Controls**：除非出现明确的“跨 setid tree 可见性/提示集”需求，否则不引入 `SET_CNTRL_TABLE2` 等价机制；如确需落地，按 0.1 的 NEW-TreeControls 承接（避免提前引入偶然复杂度）。

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
- **现状（009M1）**：DB 侧约束/解析函数/bootstrap/UI 管理面目前仅覆盖 `jobcatalog`；`orgunit` 仍为计划内待办（见 0.2/9.2）。

### 5.3.1 Tree / Hierarchy Controls（NEW：可选扩展）

> 背景：PeopleSoft 的 `SET_CNTRL_TABLE2` 用于解决“tree 的 SetID 与 BU 默认 SetID 不一致导致 tree 不可见”的问题。

- **v1 口径（冻结）**：不实现 Tree Controls；任何需要被 BU/SetID 驱动的 tree/层级配置，必须作为 setid-controlled 数据落库并归入某个 record group，读取时仅按 `ResolveSetID(..., record_group)` 的结果过滤（不做跨 setid 混用）。
- **如需扩展（NEW，非 P0）**：新增 Tree Controls 映射以支持跨 setid tree 可见性，建议契约为：
  - `ResolveTreeSetID(tx, tenant_id, business_unit_id, tree_key) -> setid`
  - 其中 `tree_key` 必须是稳定命名空间（禁止自由拼接/临时别名），并纳入门禁（防漂移）。

### 5.4 Set Control Value（选定：抽象为“控制值”，后续对齐 Business Unit）

- **选定（冻结）**：Set Control Value 即 **Business Unit**，以稳定标识 `business_unit_id` 表达，用于驱动映射：
  - `(tenant_id, business_unit_id, record_group) -> setid`
- **约束**：
  - `business_unit_id` 必须可枚举、可在 UI 中显式选择；禁止在业务代码里“从路径/会话/环境推导”隐式生成。
  - **（新增：UI 约束）**若 UI 需要提供默认 BU（`BU000`），必须通过 redirect 把 `business_unit_id=BU000` 写回 URL（使其显式可见），而不是在 handler 内部 silent fallback。
  - `business_unit_id` 的来源与生命周期由 OrgUnit/租户治理承接（本计划只冻结：写入/读取必须显式携带该字段）。
  - **字段合同（冻结）**：`business_unit_id` 值为 `[A-Z0-9]{1,5}`，存储为全大写；展示名使用 `name`，不允许“改 key”。

### 5.4.1 BU 上下文如何匹配到请求（CLARIFY）

- **列表读取（setid-controlled）**：由调用方显式传入 `business_unit_id`（URL query / API 参数）；服务端只负责校验/解析，不负责“猜测”。
- **写入（setid-controlled）**：调用方显式传入业务 payload；解析阶段使用同一 `business_unit_id` 得到 `setid` 并落库（禁止调用方直填 `setid`）。
- **跨模块引用（示例：Position）**：`staffing.position` 的创建强制携带 `business_unit_id`，从而能在投射中确定性解析 `jobcatalog_setid` 并校验 `job_profile_id`（否则会出现“岗位绑定的 job profile 属于哪套 jobcatalog”的不可判定）。

### 5.4.2 人员如何“锁定归属”到 BU（CLARIFY）

> 口径：BU 不是安全边界，但必须可推导、可解释、可复现。

- **当前模型（009M1+）**：人员的 BU 以“任职/Assignment”为主线推导：`Person -> Assignment(primary) -> Position -> business_unit_id`（按 as-of date）。  
- **强一致前提**：Position 必填 BU，因此只要人员存在有效任职，就能确定性得到其 BU；人员无任职则 BU 为空（显式状态，不做隐式默认）。  
- **多 BU 情况**：若未来允许多任职/多 assignment_type，需要另起 dev-plan 明确“人员层面的 BU”口径（例如主任职优先/多值集合/按业务场景选择）。

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

**现状（009M1）**
- `/org/setid` 当前仅支持 `jobcatalog` record group 的映射展示与保存，且仅实现 `create_setid/create_bu/save_mappings`；`rename/disable` 与多 group 矩阵仍待补齐（见 0.2）。

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
7. [ ] **（NEW）显式 BU 门禁**：setid-controlled 的 UI 入口缺少 `business_unit_id` 时必须 redirect 补齐默认 `BU000`，并在测试中断言“最终 URL 显式携带 business_unit_id”。

现状说明（009M1）：
- DB 级约束与 kernel 函数已覆盖 1/2/5/6 的大部分“数据层 fail-closed”；但 tests/gates 仍需补齐，且 record group 目前只覆盖 `jobcatalog`。

## 9. 实施步骤 (Checklist)

1. [X] 落地 tenant bootstrap（009M1：`SHARE` + `BU000` + `jobcatalog` mapping）—— 证据见 `docs/dev-records/DEV-PLAN-010-READINESS.md`（第 10 节，2026-01-06）。
   - [ ] 扩展：bootstrap 覆盖所有 stable record group（新增 `orgunit` 后必须补齐）。
2. [ ] 补齐 record group 的稳定枚举与落地清单（计划：`jobcatalog/orgunit`；现状 DB/UI 仅 `jobcatalog`），并在实现模块的 dev-plan 中声明“哪些表受控于哪个 group”。
3. [X] 冻结共享包 API：`ResolveSetID(tx, tenant_id, business_unit_id, record_group) -> setid`（含错误契约），并约束所有模块复用（009M1：`pkg/setid`）。
4. [ ] 补齐 SetID/BU/Mapping 的“管理面闭环”：
   - [X] DB Kernel 单点写入口（`submit_*_event(...)` + 同事务投射，含幂等 `request_id`）已落地（009M1）。
   - [ ] HTTP 管理面：补齐 `RENAME/DISABLE` 与多 record group 的映射矩阵（现状仅 `jobcatalog`）。
5. [ ] 实现 “无缺省洞” 的初始化与演化路径：
   - [X] 新增 BU 时：为 `jobcatalog` 自动补齐 mapping=`SHARE`（009M1）。
   - [ ] 扩展：新增 BU 时为所有 stable record group 自动补齐 mapping=`SHARE`。
   - [ ] 新增 record group 时：为所有 BU 自动 backfill mapping=`SHARE`（并补齐门禁）。
6. [X] 为主数据写入路径接入 setid 解析（先从 `jobcatalog` 起步，形成样板；009M1 已完成并留证）。
7. [ ] 落地门禁（tests/gates）并接入 CI required checks（见 §8）。
8. [ ] 补齐 UI 管理面（多 group + disable/rename），并在后续模块实现中复用同一套解析入口与 contracts。

## 10. 验收标准 (Acceptance Criteria)

- [ ] 同一 tenant 内可配置多个 SetID，并能在同一业务键下并行维护多套主数据（按 SetID 隔离）。
- [X] 给定 `(business_unit_id, record_group=jobcatalog)`，系统能稳定解析出唯一 SetID（009M1 已验证；证据见 `docs/dev-records/DEV-PLAN-010-READINESS.md` §10）。
- [ ] 扩展：`record_group=orgunit` 及后续 group 同样满足解析与无缺省洞。
- [ ] 任何绕过解析入口直接写 setid 的路径会被门禁阻断。
- [X] 新 tenant 初始化后即满足：`SHARE` + 至少 1 个 BU + `jobcatalog` mapping（默认 `SHARE`），无需手工补洞（009M1 已落地；证据见 `docs/dev-records/DEV-PLAN-010-READINESS.md` §10）。
- [ ] 扩展：初始化覆盖所有 stable record group（新增 `orgunit` 后必须补齐）。
- [X] `jobcatalog` 至少一个主数据实体完成端到端接入（解析→写入→读取→UI 展示；009M1 已落地并留证）。
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

### 11.2.1 是否可以不引入 BU，只用 SetID（CLARIFY：结论=不可以）

- **结论（冻结）**：不能只用 `setids`，必须同时存在 `business_units`（控制值）与 `setids`（共享数据集）。  
- **原因 1：表达能力**：SetID 解决的是“共享/隔离哪套主数据”；BU 解决的是“业务上下文是谁”。多个 BU 需要共享同一 SetID（多对一），且同一 BU 需要按 **record group** 选择不同 SetID（矩阵映射）；只用 SetID 无法表达这些关系。  
- **原因 2：业务可解释性**：用户/流程通常以 BU 作为入口（岗位、任职、算薪批次等），直接让业务侧选择 SetID 会把治理细节暴露成业务概念。  
- **可选退化（不选）**：若强行将 `business_unit_id == setid`（一对一），则共享能力退化为“复制/同步”，与引入 SetID 的初衷冲突。

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
