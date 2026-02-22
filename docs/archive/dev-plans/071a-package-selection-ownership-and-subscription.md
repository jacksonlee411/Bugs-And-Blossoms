# [Archived] DEV-PLAN-071A：基于 Package 的配置编辑与订阅显式化（TDD）

**状态**: 草拟中（2026-01-30 12:07 UTC；2026-02-22 起时间参数口径对齐 `DEV-PLAN-102B`/`STD-002`）

## 1. 背景与上下文 (Context)
- **需求来源**: `docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md` + SetID/Job Catalog 实际使用反馈。
- **当前痛点**:
  - 业务页面只暴露 SetID，无法直接选择/识别 `package_code`，用户创建的 package 无法被直观使用。
  - “订阅切换”与“配置编辑”混在同一路径，导致误解（以为业务页面会自动改订阅）。
  - SetID 作为配置入口的心智与“配置数据属于 package”不一致，导致操作反直觉。
- **业务价值**: 让配置编辑与 package 直接绑定，保持 SetID 作为跨 scope 订阅集合的治理入口，降低认知成本并保持可审计。

## 2. 目标与非目标 (Goals & Non-Goals)
### 核心目标
- [ ] 配置数据创建/维护时**只选择 `package_code`**，不需要选择业务单元/SetID。
- [ ] **SetID 是跨 scope 的包集合**（跨 scope 多包、单 scope 单包），同一 `scope_code` + `as_of_date` 只能解析出一个包。
- [ ] 订阅切换仅在 SetID Governance 入口发生；业务页面不触发订阅变更。
- [ ] **包可跨 SetID 复用**，但**只有包的 owner_setid 可编辑**，订阅者只读。
- [ ] 读路径保持 “业务单元 -> SetID -> 订阅包” 的解析链路不变（继续依赖 DEV-PLAN-071 的 `ResolveScopePackage`）。
- [ ] 原则必须可执行：单 scope 单包需要数据库约束 + 写入口校验双重保证。

### 非目标 (Out of Scope)
- 不改变 shared-only 包的编辑权（仍由 SaaS/共享端控制）。
- 不引入 legacy 或隐式回退入口。
- 不在本计划内做跨模块 UI 大规模重构，仅覆盖必要的交互调整。

## 2.1 工具链与门禁（SSOT 引用）
- **触发器清单（勾选本计划命中的项）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] `.templ` / Tailwind（本计划 UI 为 Go HTML 拼接；若改动 Astro/templ/Tailwind 生成物需改为勾选）
  - [ ] 多语言 JSON（`make check tr`）
  - [x] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [x] 路由治理（`make check routing`）
  - [x] DB 迁移 / Schema（按模块/域对应入口执行）
  - [x] sqlc（如引入 sqlc 代码生成，则 `make sqlc-generate`）
  - [ ] Outbox（按 `DEV-PLAN-017` 与 runbook 执行）
  - [x] 文档（`make check doc`）
- **SSOT 链接**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 架构图 (Mermaid)
```mermaid
graph TD
  UIJob[Job Catalog UI] --> JobAPI[Job Catalog Handler]
  UISet[SetID Governance UI] --> SetIDUI[SetID Governance Handler]
  SetIDUI --> ScopePkgAPI[/orgunit/api/scope-packages]
  SetIDUI --> ScopeSubAPI[/orgunit/api/scope-subscriptions]
  ScopePkgAPI --> PkgWriteEngine[orgunit.submit_scope_package_event]
  ScopeSubAPI --> SubWriteEngine[orgunit.submit_scope_subscription_event]
  JobAPI --> OwnedPkgAPI[/orgunit/api/owned-scope-packages]
  JobAPI --> PkgLookup[orgunit.setid_scope_packages]
  JobAPI --> JobDB[(jobcatalog tables)]
  JobRead[Job Catalog Read] --> Resolve[orgunit.resolve_scope_package]
```

### 3.2 关键设计决策 (ADR 摘要)
- **决策 1：配置编辑入口改为 package**
  - 选项 A：继续以 SetID 为输入。缺点：用户无法感知 package，认知断裂。
  - 选项 B（选定）：业务页面选择 `package_code`，仅编辑该包的数据。
- **决策 2：引入 owner_setid 表示包的编辑归属**
  - 选项 A：仅依赖订阅关系判断权限。缺点：订阅者与编辑者边界不清。
  - 选项 B（选定）：`owner_setid` 明确“谁能编辑包”，订阅者只读。
- **决策 3：订阅治理入口唯一**
  - 选项 A：业务页面自动订阅。缺点：隐式行为不可审计。
  - 选项 B（选定）：订阅切换仅在 SetID Governance 显式操作；创建包时在治理入口内同步写入 owner 订阅。
- **决策 4：package_code 保持租户内唯一**
  - 选项 A：允许相同 package_code 由不同 owner_setid 复用。缺点：选择歧义。
  - 选项 B（选定）：保持 `tenant_id + scope_code + package_code` 唯一，避免选择歧义。
- **决策 5：新增 owned-scope-packages API（不复用 scope-packages）**
  - 选项 A：复用 `GET /orgunit/api/scope-packages` 加 `owned=1` 参数。缺点：权限语义混淆、易误用全量列表。
  - 选项 B（选定）：新增 `GET /orgunit/api/owned-scope-packages`，仅返回可编辑包，治理入口继续使用全量列表 API。

## 4. 数据模型与约束 (Data Model & Constraints)
> 基于 `migrations/orgunit/20260129180000_orgunit_setid_scope_schema.sql` 的现有表结构。

### 4.1 Schema 调整（SQL 片段）
**包归属：orgunit.setid_scope_packages**
```sql
-- 先添加可空字段，回填后再置 NOT NULL
ALTER TABLE orgunit.setid_scope_packages
  ADD COLUMN owner_setid text;

ALTER TABLE orgunit.setid_scope_packages
  ADD CONSTRAINT setid_scope_packages_owner_format_check
    CHECK (owner_setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE orgunit.setid_scope_packages
  ADD CONSTRAINT setid_scope_packages_owner_fk
    FOREIGN KEY (tenant_id, owner_setid)
    REFERENCES orgunit.setids (tenant_id, setid);

CREATE INDEX IF NOT EXISTS setid_scope_packages_owner_lookup_idx
  ON orgunit.setid_scope_packages (tenant_id, scope_code, owner_setid, status);
```

**包版本：orgunit.setid_scope_package_versions**
```sql
ALTER TABLE orgunit.setid_scope_package_versions
  ADD COLUMN owner_setid text;

ALTER TABLE orgunit.setid_scope_package_versions
  ADD CONSTRAINT setid_scope_package_versions_owner_format_check
    CHECK (owner_setid ~ '^[A-Z0-9]{5}$');

ALTER TABLE orgunit.setid_scope_package_versions
  ADD CONSTRAINT setid_scope_package_versions_owner_fk
    FOREIGN KEY (tenant_id, owner_setid)
    REFERENCES orgunit.setids (tenant_id, setid);

CREATE INDEX IF NOT EXISTS setid_scope_package_versions_owner_lookup_idx
  ON orgunit.setid_scope_package_versions (tenant_id, scope_code, owner_setid, lower(validity));

-- 单 scope 单包（owner_setid 维度）的排他约束
ALTER TABLE orgunit.setid_scope_package_versions
  ADD CONSTRAINT setid_scope_package_versions_owner_scope_no_overlap
  EXCLUDE USING gist (
    tenant_id WITH =,
    scope_code WITH =,
    owner_setid WITH =,
    validity WITH &&
  )
  WHERE (status = 'active');
```
> 注：若 DB 尚未启用 `btree_gist`，需在本计划中显式启用（用于 text 的排他约束）。

**共享/全局包边界**
- `orgunit.global_setid_scope_packages` **不引入** `owner_setid`。
- `owned-scope-packages` **仅查询 `orgunit.setid_scope_packages`（租户包表）**，不依赖订阅表字段。
- shared-only scope 仍由 SaaS 端维护（租户侧仅只读展示，不进入编辑入口）。

**配置数据带 package_code（首批：jobcatalog）**
> 目标：所有“写入/事件/版本”链路均记录 `package_code`，确保审计可回放且 UI 可直接展示。

字段清单（新增 `package_code text`，回填后设为 NOT NULL）：
| 表 | 新增字段 | 备注 |
| --- | --- | --- |
| `jobcatalog.job_family_groups` | `package_code` | 基表，唯一性仍以 `tenant_id + package_id + code` 为准 |
| `jobcatalog.job_family_group_events` | `package_code` | 事件记录，保证审计可读 |
| `jobcatalog.job_family_group_versions` | `package_code` | 版本表，避免回放时再 join |
| `jobcatalog.job_families` | `package_code` | 同上 |
| `jobcatalog.job_family_events` | `package_code` | 同上 |
| `jobcatalog.job_family_versions` | `package_code` | 同上 |
| `jobcatalog.job_levels` | `package_code` | 同上 |
| `jobcatalog.job_level_events` | `package_code` | 同上 |
| `jobcatalog.job_level_versions` | `package_code` | 同上 |
| `jobcatalog.job_profiles` | `package_code` | 同上 |
| `jobcatalog.job_profile_events` | `package_code` | 同上 |
| `jobcatalog.job_profile_versions` | `package_code` | 同上 |
| `jobcatalog.job_profile_version_job_families` | （可选）`package_code` | 仅当 UI 需要免 join 展示时添加 |

写入口必须校验 `package_id` 与 `package_code` 一致（Fail-Closed），并在 replay 时把 `package_code` 投射到 versions 表。
`package_code` 为派生字段，**不可被业务侧直接修改**（仅由 package_id 解析写入）。

### 4.2 迁移策略
1. **新增可空字段**：`owner_setid`、`package_code` 先允许空值。
2. **回填 owner_setid（packages 表）**:
   - 若某 package 仅被一个 SetID 订阅 → 直接回填该 SetID。
   - 若多订阅者 → 必须由管理员明确 owner（记录到 `docs/dev-records/`）。
   - 若**无订阅** → 迁移 **阻断并失败**；需在 `docs/dev-records/` 明确 owner_setid 后重新执行。
3. **回填 owner_setid（versions 表）**：
   - 固定采用 **UPDATE join**：`UPDATE versions v SET owner_setid = p.owner_setid FROM packages p WHERE v.tenant_id = p.tenant_id AND v.package_id = p.package_id`。
4. **回填 package_code**：通过 `package_id` 关联 `orgunit.setid_scope_packages` 回填到各业务表/事件/版本表。
5. **加严约束**：`owner_setid` 设为 NOT NULL，添加 FK/Check/排他约束。
6. **写引擎同步**：`submit_scope_package_event` 投射 `owner_setid` 到 versions 表。

### 4.3 单 scope 单包落地路径（必须引用）
- **订阅层（SetID -> scope）**：依赖 `orgunit.setid_scope_subscriptions_no_overlap` 排他约束（`tenant_id + setid + scope_code + validity`）与 `orgunit.submit_scope_subscription_event` 写入口校验，确保同一 SetID 在同一 scope + 日期只能订阅一个包。
- **包层（owner_setid -> scope）**：新增 `setid_scope_package_versions_owner_scope_no_overlap` 约束，保证同一 owner_setid 在同一 scope + 日期只有一个生效包。

## 5. 接口契约 (API Contracts)
### 5.1 JSON API: `POST /orgunit/api/scope-packages`
**Request**:
> `package_code` 可选；留空则由后端生成。
```json
{
  "scope_code": "jobcatalog",
  "package_code": "PKG_CORE",
  "name": "Core Job Catalog",
  "effective_date": "2026-02-01",
  "owner_setid": "A0001",
  "request_id": "req-uuid"
}
```
**Response (201)**:
```json
{
  "package_id": "uuid",
  "scope_code": "jobcatalog",
  "package_code": "PKG_CORE",
  "owner_setid": "A0001",
  "status": "active"
}
```
**Error Codes**:
- `SETID_NOT_FOUND` / `SETID_RESERVED_WORD`
- `PACKAGE_CODE_DUPLICATE`
- `PACKAGE_CODE_INVALID` / `PACKAGE_CODE_RESERVED`
- `SCOPE_CODE_INVALID`
- `SUBSCRIPTION_OVERLAP` / `PACKAGE_INACTIVE_AS_OF`
**补充规则**:
- `package_code` 允许省略；若为空，后端生成 `PKG_` + base36 时间戳（大写、截断至 16 字符）。
- 若生成值与现有重复：重试最多 3 次；仍冲突则返回 `PACKAGE_CODE_DUPLICATE`。

### 5.2 JSON API: `GET /orgunit/api/owned-scope-packages`
**Query**: `scope_code=jobcatalog&as_of=YYYY-MM-DD`（`as_of` 必填）  
**Response (200)**:
```json
[
  {
    "package_id": "uuid",
    "scope_code": "jobcatalog",
    "package_code": "PKG_CORE",
    "name": "Core Job Catalog",
    "owner_setid": "A0001",
    "status": "active",
    "effective_date": "2026-02-01"
  }
]
```
**约束**:
- 返回当前用户**可编辑**的包（owner_setid 权限 + active + `as_of` 命中）。
- 用于 Job Catalog 页面下拉选择。
- **不复用** `GET /orgunit/api/scope-packages`，避免将“治理全量列表”误用于业务编辑列表。
- 共享/全局包不进入该列表（shared-only 保持只读）。

### 5.3 JSON API: `POST /orgunit/api/scope-subscriptions`
保持 DEV-PLAN-071 契约并按 `DEV-PLAN-102B` 收口：`effective_date` 必填，不允许默认 today。

### 5.4 UI/API：`/org/setid` 与 `/org/job-catalog`
- **SetID Governance**:
  - 在 `/org/setid` 页面新增 **“Scope Packages”** 区块（治理入口唯一）。
  - 包列表字段：`scope_code`、`package_code`、`name`、`owner_setid`、`status`、`effective_date`、`last_updated`、`actions`。
  - 仅展示 **tenant 包**（不展示 global/shared-only 包）。
  - Create 表单字段：`scope_code`（下拉）、`owner_setid`（下拉）、`package_code`（可选，留空自动生成）、`name`、`effective_date`。
  - 提交：`POST /orgunit/api/scope-packages`（`effective_date` 必填）；Disable：`POST /orgunit/api/scope-packages/{package_id}/disable`（`effective_date` 必填）。
  - 权限：仅 `org.scope_package` + `orgunit.setid` admin 可见/可操作。
  - Scope Subscriptions 区域保留，用于订阅切换（下拉 + 保存，调用 `scope-subscriptions` API）。
- **Job Catalog**:
  - URL 改为 `GET /org/job-catalog?package_code=PKG_CORE&as_of=YYYY-MM-DD`。
  - 表单移除 SetID/业务单元输入，新增 `package_code` 下拉（从 owned-scope-packages 获取）。
  - 若 `package_code` 为空，仅展示选择提示，不加载列表/表单。
  - **只读查看**：非 owner 订阅者可通过 SetID Governance 入口跳转只读视图（`/org/job-catalog?setid=...&as_of=...` 或根据订阅解析的只读链接），页面仅展示列表不提供编辑表单。
  - **参数互斥**：`package_code` 与 `setid` 不可同时出现；若同时出现，返回 400 并提示“请选择其一”。

### 5.5 Job Catalog UI 错误码（本计划新增）
- `PACKAGE_CODE_MISMATCH`：package_code 与 package_id 解析不一致（Fail-Closed）。

### 5.6 Job Catalog 写入接口错误码（前端交互）
- `POST /org/job-catalog`（各类 create/update）若 `PACKAGE_CODE_MISMATCH`，返回 422 并在表单区域显示错误（沿用现有错误渲染方式）。
- `POST /org/job-catalog` 若 `DEFLT_EDIT_FORBIDDEN`，返回 403 或 422（以现有错误渲染约定为准），提示“DEFLT 包仅限根组织权限管理员修改”。

## 6. 核心逻辑与算法 (Business Logic)
### 6.1 创建包（治理入口）
1. 校验 `scope_code` 合法，`owner_setid` 存在且 active。
2. 校验 `owner_setid + scope_code + effective_date` 不与现有 active 包重叠（DB 排他约束 + 预检）。
3. 写入 `submit_scope_package_event`（tenant 包）。
4. **同一事务内**写入 owner_setid 的订阅事件（治理入口内显式操作）。
5. 若订阅写入触发 `SUBSCRIPTION_OVERLAP`：**整笔失败并回滚**，不自动切换；用户需显式调整 effective_date 或在治理入口手动切换订阅。

### 6.2 查询可编辑包
1. 解析当前用户可编辑的 `owner_setid` 列表（对齐 `orgunit.setid` 权限）。
2. 查询 `setid_scope_packages` + `setid_scope_package_versions`，仅返回 active 且 `as_of` 命中的包。

### 6.3 创建配置数据（Job Catalog）
1. 从 `package_code` 查找 `package_id` + `owner_setid`（`orgunit.setid_scope_packages`）。
2. 校验包 active 且 `as_of` 命中（`orgunit.assert_scope_package_active_as_of`）。
3. 校验当前用户具备 `owner_setid` 编辑权限。
4. 若 `package_code == 'DEFLT'` 且不具备“根组织权限管理员” → `DEFLT_EDIT_FORBIDDEN`（Fail-Closed）。
5. 写入 `jobcatalog.submit_*_event`（传入 `package_id`，并写入 `package_code` 到 events/versions）。
6. 若 `package_code` 与 `package_id` 不一致 → `PACKAGE_CODE_MISMATCH`（Fail-Closed）。

### 6.4 读取配置数据（业务单元视角）
保持现有 `ResolveScopePackage`：`org_unit -> setid -> scope -> package_id`。

## 7. 安全与鉴权 (Security & Authz)
- **Casbin 对象**（沿用现有口径）：
  - 订阅治理：`org.scope_subscription` (admin)
  - 包治理：`org.scope_package` (admin)
  - 业务配置编辑：`jobcatalog.catalog` (admin)
  - `orgunit.setid` 为**既有对象**（见 `pkg/authz/registry.go` 与 `config/access/policy.csv`），非新对象/别名。
- **owner_setid 校验**：除 Casbin 外，必须追加“当前用户对 owner_setid 具备编辑权限”的二次校验（Fail-Closed）。
- **RLS/事务**：所有写入路径必须在事务内显式注入 `app.current_tenant`。
- **DEFLT 包修改权限**：仅具备“租户根组织（root node）管理权限”的管理员可修改 `package_code=DEFLT` 的包。
  - 当前阶段未引入用户-OrgUnit 权限模型，暂以 `org.scope_package` admin + `orgunit.setid` admin 的租户管理员作为等效准入；后续引入 root-node 授权后替换为真实根组织权限校验。

### 7.1 owner_setid 可编辑判定（当前阶段）
> 现阶段权限模型为租户级角色（无用户-OrgUnit 映射），因此“可编辑 owner_setid”以**角色 + SetID 状态**为准。

**规则**：
1. 若用户不具备 `jobcatalog.catalog` admin → 不可编辑任何 package（owned-scope-packages 返回空）。
2. 若用户不具备 `orgunit.setid` admin → 不可编辑任何 package（防止绕开治理入口）。
3. 否则，`editable_setids = SELECT setid FROM orgunit.setids WHERE tenant_id = $1 AND status = 'active'`。
4. owned-scope-packages 仅返回 `owner_setid ∈ editable_setids` 且 `status=active` 且 `validity @> as_of` 的包。
5. 写入路径必须校验 `package.owner_setid ∈ editable_setids`，否则返回 `OWNER_SETID_FORBIDDEN`。

**后续扩展（非本期）**：
- 若引入用户-OrgUnit 归属或权限域，替换步骤 3 为“用户可管理的 OrgUnit 绑定的 SetID 集合”。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**:
  - `DEV-PLAN-070`（SetID 与 OrgUnit 绑定）
  - `DEV-PLAN-071`（Scope Package/Subscription 基础）
  - **DEFLT 约定（引用 DEV-PLAN-070，不在本计划重定义）**：
    - 每租户必须存在 `DEFLT`，且绑定租户根组织。
    - `DEFLT` 状态固定为 `active`，禁止禁用/删除/重命名（DB 约束 + 写入口双重阻断）。
    - `DEFLT` 属于租户层（非共享表），`SHARE` 仅存在于全局租户。
    - 业务主数据域仅使用租户 SetID（含 `DEFLT`），不读取 `SHARE`。
- **里程碑**:
  1. [x] Schema 迁移：`owner_setid` 与排他约束落地。
  2. [x] Scope Package 写入链路支持 `owner_setid`（含回填）。
  3. [x] 新增 owned-scope-packages API。
  4. [x] SetID Governance UI 可交互订阅切换。
  5. [x] Job Catalog UI 改为 package_code 输入。
  6. [ ] 迁移记录与文档更新。
  - **完成记录**:
    - 2026-01-30：完成里程碑 1。命令：`./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/orgunit?format=goose"`、`./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/jobcatalog?format=goose"`、`make orgunit plan`、`make jobcatalog plan`、`make orgunit lint`、`make jobcatalog lint`、`make orgunit migrate up`、`make jobcatalog migrate up`、`make sqlc-generate`、`make test`（coverage=100%）、`make check doc`。
    - 2026-01-30：完成里程碑 2（owner_setid 写入 + 回填 + 包订阅同事务）。命令：`./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/orgunit?format=goose"`、`./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/jobcatalog?format=goose"`、`make orgunit plan`、`make jobcatalog plan`、`make orgunit lint`、`make jobcatalog lint`、`make orgunit migrate up`、`make jobcatalog migrate up`、`make sqlc-generate`、`go fmt ./...`、`go vet ./...`、`make check lint`、`make test`（coverage=100%）、`make check doc`；回填决策记录见 `docs/dev-records/dev-plan-071-execution-log.md`。
    - 2026-01-30：完成里程碑 3（owned-scope-packages API）。命令：`go fmt ./...`、`go vet ./...`、`make check lint`、`make test`（coverage=100%）、`make check routing`。
    - 2026-01-30：完成里程碑 4（SetID Governance UI 订阅切换交互）。命令：`go fmt ./...`、`go vet ./...`、`make check lint`、`make test`（coverage=100%）。
    - 2026-01-30：完成里程碑 5（Job Catalog UI 改为 package_code 输入）。命令：`go fmt ./...`、`go vet ./...`、`make check lint`、`make test`（coverage=100%）、`make check doc`、`make e2e`。

## 9. 测试与验收标准 (Acceptance Criteria)
- **单元/集成测试**:
  - [ ] owner_setid 重叠包写入被拒绝（排他约束）。
  - [ ] 非 owner 订阅者无法写入包数据。
  - [ ] owned-scope-packages 返回集正确且不包含不可编辑包。
- **E2E/手测**:
  - [x] Job Catalog 页面可选择 package_code 完成创建。
  - [ ] 订阅切换必须在 SetID Governance 完成，业务页面不改变订阅。
- **门禁**: 按 `AGENTS.md` 触发器执行并记录。

## 10. 运维与监控 (Ops & Monitoring)
本阶段不引入新的开关与监控（遵循 `AGENTS.md` 3.6）。

## 附录 A：Package 权限与可见性矩阵（SSOT 摘要）
> 目的：集中展示“创建/停用/编辑/查看”的权限口径，详细实现仍以 DEV-PLAN-071 与 access policy 为准。

| 场景 | 入口 | 权限对象 | 说明 |
| --- | --- | --- | --- |
| 租户包创建 | `/org/setid` → Scope Packages / `POST /orgunit/api/scope-packages` | `org.scope_package` admin + `orgunit.setid` admin | `DEFLT` 为保留值，禁止手工创建 |
| 租户包停用 | `/org/setid` → Disable / `POST /orgunit/api/scope-packages/{id}/disable` | `org.scope_package` admin + `orgunit.setid` admin | `DEFLT` 包禁止停用 |
| 共享包创建/停用 | SaaS 专用 API | SaaS (`actor_scope=saas`) | 租户不可写 |
| 业务配置编辑（包内数据） | `/org/job-catalog`（package_code） | `jobcatalog.catalog` admin + owner_setid 可编辑 | 订阅者只读 |
| DEFLT 包内数据编辑 | `/org/job-catalog`（package_code=DEFLT） | 根组织权限管理员 | 非 root 权限返回 `DEFLT_EDIT_FORBIDDEN` |
| 可编辑包列表 | `GET /orgunit/api/owned-scope-packages` | 同上 | 仅返回可编辑包 |
| 包列表查看 | `GET /orgunit/api/scope-packages` | `org.scope_package` read | 不含共享包 |
| 订阅查看 | `GET /orgunit/api/scope-subscriptions` | `org.scope_subscription` read | shared-only 只读 |
| DEFLT 包修改 | 治理入口 | 根组织权限管理员 | 当前阶段等价为 `org.scope_package` admin + `orgunit.setid` admin |
