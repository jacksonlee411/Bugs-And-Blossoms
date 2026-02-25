# DEV-PLAN-062：全链路业务测试子计划 TP-060-02——主数据（组织架构 + SetID + JobCatalog + 职位）

**状态**: 草拟中（2026-01-24 05:00 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`（可登录 + 隔离基线）。

## 1. 背景与上下文（Context）

- **需求来源**：TP-060 总纲中的“主数据最小可复现闭环”（`docs/dev-plans/060-business-e2e-test-suite.md`），以及 Phase 4 纵切片的交付口径（`DEV-PLAN-009`）。
- **覆盖范围**：OrgUnit → SetID → JobCatalog → Position（契约分别对齐 `docs/dev-plans/026/070/029/030`）。
- **业务价值**：为后续 TP-060-03（Person/Assignments）及考勤/薪酬子计划提供稳定的主数据底座，避免“不可见/不可操作”的僵尸交付（见 `AGENTS.md` 的用户可见性原则）。
- **关键不变量（必须 fail-closed）**：
  - `as_of` 为日粒度（date），所有读取必须显式传入 `as_of=YYYY-MM-DD`（对齐 `docs/dev-plans/032-effective-date-day-granularity.md`）。
- SetID 绑定必须显式存在：根组织绑定 `DEFLT`，业务单元节点允许绑定其他 SetID；绑定管理与审计解析仍沿 `is_business_unit=true` 的祖先链路，缺绑定或非法状态必须 fail-closed（对齐 `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`）。
- 配置主数据入口必须显式携带 `setid`；Position 创建必须选择 Job Profile，且可选列表由 `org_code` 解析得到的 setid 提供（不要求手工选择 setid，缺绑定/非法必须 fail-closed）。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [X] **OrgUnit**：可在 `/org/units` 完成 Root + 5 个一级部门创建，刷新后列表可见；并记录每个 `org_code`。
- [X] **SetID**：可在 `/org/setid` 完成 SetID 创建与组织绑定；业务单元节点允许绑定 SetID；不存在/disabled 的 `org_code` 必须 fail-closed。
- [X] **JobCatalog**：在 `/org/job-catalog` 能看到 `SetID: S2601`，并覆盖 groups/families/levels/profiles 的“写入→as_of 读取→UI 可见”闭环，且包含至少 1 个跨日期场景（Job Family reparenting 的前后对比）。
- [X] **Position（基础）**：在 `/org/positions` 能创建 10 条职位，刷新后列表可见且包含 `position_id`；创建时 OrgUnit 下拉可用（不出现 `(no org units)`）。
- [X] **Position（M5：与 JobCatalog/SetID 组合）**：至少 1 条职位能绑定 `org_code=<R&D>` + `job_profile=JP-SWE`（由 org_code 解析 setid），且列表中可见：
  - `org_code=<R&D>`
  - `jobcatalog_setid=S2601`（解析结果）
  - `job_profile` 显示 `JP-SWE (...)`（或等效可解释文本）
- [X] **Position（M5：fail-closed 负例，Internal API）**：至少覆盖 3 个负例断言（1 个参数校验 + 2 个稳定错误码）：
  - `400`：缺失 `org_code`（`code=invalid_request`）
  - `SETID_BINDING_MISSING/SETID_DISABLED/ORG_NOT_FOUND_AS_OF`：org_unit 解析 setid 失败（422）
  - `JOBCATALOG_REFERENCE_NOT_FOUND`：`job_profile_id` 不属于解析得到的 setid（422）

### 2.2 非目标

- 本子计划不覆盖 **Position 与 Assignments 的交叉裁决**（例如 “disabled 与任职冲突” / “capacity 与 allocated_fte 裁决”），这些由后续 TP-060-03 承接。
- 本子计划不验证考勤/薪酬计算结果（由 TP-060-04~08 承接）。

### 2.3 工具链与门禁（SSOT 引用）

> 目的：避免在子计划里复制工具链/脚本细节导致 drift；本文仅声明“本子计划执行/修复时可能命中的门禁入口”，具体命令以 `AGENTS.md`/`Makefile`/CI workflow 为准。

- **触发器清单（勾选本计划命中的项；执行记录见 §9）**：
  - [X] E2E（`make e2e`，套件入口对齐 `docs/dev-plans/060-business-e2e-test-suite.md`）
  - [X] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [X] 路由治理（`make check routing`，必要时更新 `config/routing/allowlist.yaml`）
  - [X] 文档（`make check doc`）
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）——仅当为修复 drift 而改 Go 时
  - [ ] DB/迁移（按模块 `make <module> plan/lint/migrate ...`）——仅当为修复 drift 而改 DB 时
  - [ ] sqlc（`make sqlc-generate`）——仅当为修复 drift 而改 sqlc 输入时

- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`

## 3. 契约引用（SSOT）

- OrgUnit：`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- SetID：`docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- JobCatalog：`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- Position：`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
- Valid Time（日粒度）：`docs/dev-plans/032-effective-date-day-granularity.md`
- 路由/UI 可见性：`AGENTS.md`、`docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- Authz（对象/动作冻结）：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 路由策略（route_class/allowlist）：`docs/dev-plans/017-routing-strategy.md`

## 4. 前置条件与环境（Prerequisites）

### 4.1 租户与 Host

- Tenant：`T060`（示例 host：`t-060.localhost`）
- `as_of`：基准使用 `2026-01-01`（数据底座），并在 JobCatalog 的跨日期断言中额外使用 `2026-01-15/2026-02-15/2026-03-15`（本子计划所有 URL 必须显式带 `as_of`，避免被缺省重定向到“当天”导致不可复现）

### 4.2 账号与权限

- 必备：Tenant Admin（可执行 `POST` 写入动作）。
- 可选（用于 403 负例）：Tenant Viewer（若环境已支持角色区分；若不支持则记录为 `CONTRACT_MISSING` 并写明阻塞点，口径对齐 `docs/dev-plans/060-business-e2e-test-suite.md`）。

### 4.3 `as_of` 缺省行为（避免踩坑）

- `GET` 且未提供 `as_of`：服务端会 `302` 重定向补上 `as_of=<当前UTC日期>`。
- `POST` 且未提供 `as_of`：服务端会使用 `as_of=<当前UTC日期>` 作为默认值。
- 结论：本子计划所有步骤必须显式带 `as_of=2026-01-01`，且表单中的 `effective_date` 建议同样固定为 `2026-01-01`。

### 4.4 可重复执行口径（Idempotency / Re-run）

> 目的：同一租户/同一环境重复跑本子计划时，避免“重复创建导致失败或脏数据”。

- 若 060-DS1 已在本租户建立：本子计划优先做“**校验 + 记录 ID**”，仅在缺失时补齐创建。
- 所有“创建”动作若返回“已存在/重复”类错误：应先在列表中确认是否已存在对应记录；若已存在则改为记录其 ID 并继续；若未存在则记录为 `BUG/CONTRACT_DRIFT` 并停止该子步骤。
- 若环境允许但数据已明显污染（重复 Root、重复 SetID/绑定/JobFamilyGroup、职位数量异常且无法判定）：记录为 `ENV_DRIFT`，建议重置租户或更换干净租户再跑（不得在测试中隐式修补口径）。

### 4.5 数据保留（强制）

- 本子计划创建/补齐的数据（OrgUnit、SetID/绑定/业务单元标记、JobCatalog、Positions）构成 060-DS1 的主数据底座，必须保留以供 TP-060-03/04/05/07/08 复用（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。
- 禁止在本子计划执行完后清理数据；若必须重置环境，需按 §4.4 的口径登记并重建 060-DS1。

## 5. 数据准备要求（060-DS1 子集 + 本计划增量）

> 基线数据 SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` 的 §5.2–§5.5。本文只声明本子计划用到的“子集与增量”。

### 5.1 最小数据（必须）

- OrgUnit：
  - Root：`Bugs & Blossoms Co., Ltd.`（若环境已有 Root，可复用；必须记录其 `org_code`）
  - L1：`HQ`、`R&D`、`Sales`、`Ops`、`Plant`（Parent ID = Root）
- 业务单元标记（`is_business_unit=true`）：
  - Root：必须为 `true`
  - `R&D`：设置为 `true`（用于绑定 `S2601`）
  - `Sales`：设置为 `true`（用于绑定 `S2602` 与跨 SetID 负例）
- SetID 与组织绑定：
  - SetID：`S2601`（主用）
  - 根组织绑定：`DEFLT`（必须存在）
  - 绑定：`R&D -> S2601`
  - 断言：绑定只能落在 `is_business_unit=true` 的节点；解析命中最近祖先绑定
- JobCatalog（`setid=S2601`）：
  - Job Family Group：至少 2 条（建议使用 `JFG-ENG`、`JFG-SALES`）
- Position：10 条（建议命名见 §7.4；需覆盖多个 OrgUnit）

### 5.2 增量数据（用于显式 SetID + fail-closed）

- fail-closed 负例：使用不存在/非法 `setid` 访问 JobCatalog。
- 跨 SetID 负例准备：
  - 创建 `SetID=S2602`（若不存在）。
  - 将 `Sales` 绑定为 `S2602`（`is_business_unit=true`）。

## 6. 路由与交互契约（UI Contracts，便于排障）

> 说明：本节用于把“测试动作”具体化到 UI 路由与关键表单字段，便于定位 drift。若实现已变更，请优先记录为 `CONTRACT_DRIFT` 并指向对应 SSOT。

### 6.1 OrgUnit：`/org/units`（UI）

- `GET /org/units?as_of=YYYY-MM-DD`：展示组织树、列表与创建入口。
- `POST /org/api/org-units`：创建节点（成功后刷新 `/org/units?as_of=<as_of>` 可见）。
- 关键字段：
  - `effective_date`（默认：`as_of`；必须是 `YYYY-MM-DD`）
  - `parent_org_code`（可空；根节点为空）
  - `name`（必填；空值应提示 `name is required`）
- Authz 口径：`GET /org/units`、`GET /org/api/org-units*=read`，`POST /org/api/org-units*=admin`（对齐 `docs/dev-plans/022-authz-casbin-toolchain.md`）。

### 6.2 SetID：`/org/setid`（UI）

- `GET /org/setid?as_of=YYYY-MM-DD`：展示 SetIDs + 组织树 + 绑定编辑。
- `POST /org/setid?as_of=YYYY-MM-DD`：两类动作（成功后 `303` 跳回 `/org/setid?as_of=...`）：
  - `action=create_setid`：`setid` + `name` 必填
  - `action=bind_setid`：`org_code` + `setid` + `effective_date` 必填
- Authz 口径：`GET=read`，`POST=admin`。

### 6.3 JobCatalog：`/org/job-catalog`（UI）

- `GET /org/job-catalog?as_of=YYYY-MM-DD&setid=<setid>`
  - 页面应显示 `SetID: <setid>`；若 `setid` 缺失/非法，应显式显示错误信息。
- `POST /org/job-catalog?as_of=...&setid=...`：创建 Job Family Group（成功后 `303` 跳回 `/org/job-catalog?setid=...&as_of=<effective_date>`）。
- 关键字段：
  - `effective_date`（默认：`as_of`）
  - `setid`（显式；必须为租户 `active` 且非 `SHARE`）
  - `code`/`name`（必填；空值应提示 `code/name is required`）
  - `description`（可空）
- Authz 口径：`GET=read`，`POST=admin`。

### 6.4 Position：`/org/positions`（UI）

- `GET /org/positions?as_of=YYYY-MM-DD`：展示 JobCatalog Context（OrgUnit 选择 + 解析得到的 SetID）、创建表单、更新/停用表单与职位列表。
  - 可选：`org_code=<uuid>`（用于列表过滤与预选 OrgUnit）
- `POST /org/positions?as_of=YYYY-MM-DD`：
  - Create：`position_id` 为空时创建职位（成功后 `303` 跳回 `/org/positions?as_of=<effective_date>`，若携带 `org_code` 则 redirect 保留）。
  - Update/Disable：`position_id` 非空时更新/停用职位（同上）。
- 关键字段：
  - JobCatalog Context（GET 表单）：`org_code`（用于解析 setid 并加载 job profiles）
  - Create（POST）：`effective_date`（默认：`as_of`；非法日期应提示 `effective_date 无效: ...`）、`org_code`（必填）、`job_profile_id`（必填；必须归属解析得到的 setid）、`capacity_fte`（可空；默认 1.0）、`name`（可空）
  - Update/Disable（POST，patch 语义）：`position_id` 必填，其余字段为“可选 patch”
    - `org_code` / `reports_to_position_id` / `job_profile_id` / `capacity_fte` / `name` / `lifecycle_status`
- Authz 口径：`GET=read`，`POST=admin`。

### 6.5 Position：`/org/api/positions`（Internal API，用于稳定错误码断言）

- `GET /org/api/positions?as_of=YYYY-MM-DD`：
  - 200：`{"as_of","tenant","positions":[...]}`（positions 元素包含 `PositionID/OrgUnitID/JobCatalogSetID/JobProfileID/JobProfileCode/CapacityFTE/LifecycleStatus/...`）
  - 400：`code=invalid_as_of`
- `POST /org/api/positions?as_of=YYYY-MM-DD`：
  - Create：`{"effective_date","org_code","job_profile_id", ...}`（`position_id` 为空；`job_profile_id` 必填，由 org_unit 解析 setid 校验）
  - Update：`{"effective_date","position_id", ...}`（`position_id` 非空；至少 1 个 patch 字段）
  - 400：`code=bad_json` / `code=invalid_effective_date` / `code=effective_date is required` / `code=position_id is required` / `code=at least one patch field is required`（等）
  - 409：`code=STAFFING_IDEMPOTENCY_REUSED`
  - 422：稳定 DB 错误码（示例：`STAFFING_ORG_UNIT_NOT_FOUND_AS_OF` / `SETID_BINDING_MISSING` / `SETID_DISABLED` / `JOBCATALOG_REFERENCE_NOT_FOUND` / `STAFFING_INVALID_ARGUMENT`（例如 `org_code` 格式非法））

## 7. 测试步骤（执行时勾选）

> 约定：每个步骤都要把关键 ID 记录到“验收证据”里（§8），并在出现偏差时填“问题记录”（§10）。

### 7.1 OrgUnit：创建与可见

1. [ ] 打开：`/org/units?as_of=2026-01-01`
2. [ ] 确认 Root（若不存在则创建；若已存在则记录并复用）
   - 表单：`effective_date=2026-01-01`，`parent_org_code=`（空），`name=Bugs & Blossoms Co., Ltd.`
   - 断言：Root 在列表可见，并能看到其 `org_code`
   - 约束：若列表已存在大量节点且无法确认 Root（例如已有多个候选 Root），记录为 `ENV_DRIFT` 并停止 OrgUnit 步骤
3. [ ] 确认 5 个 L1 节点（缺失则创建；已有则记录并复用）
   - `HQ`、`R&D`、`Sales`、`Ops`、`Plant`
   - 断言：刷新后列表可见，且每条都有 `org_code`
4. [ ] 设置业务单元标记（`is_business_unit=true`）
   - Root（必须为 `true`）
   - `R&D`（用于绑定 `S2601`）
   - `Sales`（用于绑定 `S2602`）
   - 若 UI 不支持，记录为 `CONTRACT_DRIFT` 并改走 `/org/api/org-units/set-business-unit`
5. [ ] 负例：提交空 `name`
   - 断言：页面提示 `name is required`；不得创建新节点

### 7.2 SetID：创建与组织绑定

1. [ ] 打开：`/org/setid?as_of=2026-01-01`
2. [ ] 断言：已存在可用于共享的 `SHARE` SetID（若不存在，记录为 `CONTRACT_DRIFT/ENV_DRIFT` 并停止后续 SetID 步骤）
3. [ ] 确认 SetID：`S2601`（缺失则创建；已有则记录并复用）
   - 断言：SetIDs 表格存在 `S2601`，状态为 active（或记录实际状态）
4. [ ] 断言：根组织已绑定 `DEFLT`（若绑定缺失，记录为 `CONTRACT_DRIFT/ENV_DRIFT` 并停止后续步骤）
5. [ ] 绑定 `S2601` → `R&D`
   - 使用 `action=bind_setid`，填写 `org_code=<R&D>`、`setid=S2601`、`effective_date=2026-01-01`
   - 断言：绑定列表可见 `R&D -> S2601`
6. [ ] 负例：尝试绑定到非业务单元节点（例如 `HQ`）
   - 断言：应失败并提示 `ORG_NOT_BUSINESS_UNIT_AS_OF`（若无法稳定提取错误码，记录实际提示）

### 7.3 JobCatalog：显式 SetID、写入闭环与 fail-closed 负例

1. [ ] 打开：`/org/job-catalog?as_of=2026-01-01&setid=S2601`
2. [ ] 断言：页面显示 `SetID: S2601`（且无错误提示）
3. [ ] 确认 Job Family Group（至少 2 条；缺失则创建；已有则记录并复用）
   - 建议：
     - `code=JFG-ENG`，`name=Engineering`
     - `code=JFG-SALES`，`name=Sales`
   - 断言：创建后列表可见，且每条包含 `id`；记录两条 `job_family_group_id`
4. [ ] 确认 Job Families（至少 2 条；缺失则创建；已有则记录并复用）
   - 基准：`as_of=2026-01-01`，`effective_date=2026-01-01`
   - 建议：
     - `code=JF-BE`，`name=Backend`，`group=JFG-ENG`
     - `code=JF-FE`，`name=Frontend`，`group=JFG-ENG`
   - 断言：创建后列表可见；每条包含 `id`；列表可见其 `group`（至少能判定 `JF-BE` 初始归属为 `JFG-ENG`）
5. [ ] 跨日期断言：Job Family reparenting（同一 `code` 在不同 `as_of` 下归属不同 group）
   - 在 `effective_date=2026-02-01` 提交对 `JF-BE` 的 UPDATE（reparent 到 `JFG-SALES`）
   - 断言 A：访问 `/org/job-catalog?as_of=2026-01-15&setid=S2601`，`JF-BE` 的 `group=JFG-ENG`
   - 断言 B：访问 `/org/job-catalog?as_of=2026-02-15&setid=S2601`，`JF-BE` 的 `group=JFG-SALES`
6. [ ] 确认 Job Levels（至少 1 条；缺失则创建；已有则记录并复用）
   - 基准：`as_of=2026-01-01`，`effective_date=2026-01-01`
   - 建议：`code=JL-1`，`name=Level 1`
   - 断言：创建后列表可见，且包含 `id`
7. [ ] 确认 Job Profiles（至少 1 条；缺失则创建；已有则记录并复用）
   - 基准：`as_of=2026-01-01`，`effective_date=2026-01-01`
   - 建议：`code=JP-SWE`，`name=Software Engineer`，`families=[JF-BE,JF-FE]`，`primary=JF-BE`
   - 断言：创建后列表可见；能看到 families 与 primary 的归属信息（文本/列表均可）
8. [ ] 负例：Profile families/primary 不变量
   - 提交：`job_family_ids=[JF-BE]`，`primary=JF-FE`（primary 不在 families）
   - 断言：页面提示稳定错误（例如 `payload.primary_job_family_id must be included ...`），且不得创建新 profile
9. [ ] fail-closed 负例：使用不存在/非法的 `setid`，直接访问：
   - `/org/job-catalog?as_of=2026-01-01&setid=S9999`
   - 断言：页面显式报错，且不得显示 `SetID`

### 7.4 Position：创建与列表可见

1. [ ] 打开：`/org/positions?as_of=2026-01-01&org_code=<R&D>`
2. [ ] 断言：OrgUnit 下拉不为 `(no org units)`，且选项包含你在 §7.1 创建的部门（名称 + `org_code`）
3. [ ] 确保至少 10 个职位（不足则补齐创建；已有则记录其中 10 条）
   - 建议命名（可按 OrgUnit 分配；若重复可加后缀）：
     - `P-ENG-01`、`P-ENG-02`
     - `P-SALES-01`
     - `P-HR-01`、`P-FIN-01`、`P-MGR-01`
     - `P-OPS-01`、`P-SUPPORT-01`
     - `P-PLANT-01`、`P-PLANT-02`
  - 断言：每次创建后 `303` 跳转回 `/org/positions?as_of=2026-01-01&org_code=<R&D>`；列表出现新行并包含 `position_id`；记录 10 条 `position_id`
4. [ ] 负例：提交非法 `effective_date`（例如 `bad`）
   - 断言：页面提示 `effective_date 无效: ...`；不得创建新职位

### 7.5 Position（M5）：与 JobCatalog/SetID 组合（OrgUnit + SetID + Job Profile）

> 目标：覆盖 `DEV-PLAN-030` 的 M5 关键链路：org_unit 解析 setid → job_profile identity 校验 → UI 可见。

1. [ ] 打开：`/org/positions?as_of=2026-01-01&org_code=<R&D>`
2. [ ] 断言：页面显示解析得到的 `SetID: S2601`，且 Create 表单的 `Job Profile` 下拉包含 `JP-SWE (...)`（来自 §7.3 创建的 Job Profile）
3. [ ] 绑定 Job Profile（正例）
   - 选择任一既有职位（建议：`P-ENG-01`）
   - 在 Update/Disable 表单提交：
     - `effective_date=2026-01-15`
     - `position_id=<P-ENG-01 的 position_id>`
     - `org_code=<R&D>`
     - `job_profile_id=<JP-SWE 的 id>`
   - 断言：职位列表对应行可见：
     - `org_code=<R&D>`
     - `jobcatalog_setid=S2601`
     - `job_profile` 显示 `JP-SWE (...)`（或等效）

### 7.6 Position（M5）：fail-closed 负例（Internal API，断言 HTTP status + code）

> 说明：负例优先用 Internal API 断言 `code`，避免 UI 只显示红字但无法稳定提取错误码。

1. [ ] 负例 A（缺 org_code）：`POST /org/api/positions?as_of=2026-01-01`
   - body：`{"effective_date":"2026-01-01","job_profile_id":"<JP-SWE-id>","name":"TP062-BAD-NO-ORG"}`
   - 断言：400 且 `code=invalid_request`
2. [ ] 负例 B（缺 job_profile_id）：`POST /org/api/positions?as_of=2026-01-01`
   - body：`{"effective_date":"2026-01-01","org_code":"<R&D>","name":"TP062-BAD-NO-JP"}`
   - 断言：400 且 `code=job_profile_id is required`
3. [ ] 负例 C（不存在/不可解析 org_unit）：`POST /org/api/positions?as_of=2026-01-01`
   - body：`{"effective_date":"2026-01-01","org_code":"<不存在的 org_code>","job_profile_id":"<JP-SWE-id>","name":"TP062-BAD-ORG404"}`
   - 断言：422 且 `code=STAFFING_ORG_UNIT_NOT_FOUND_AS_OF`（或 `SETID_BINDING_MISSING`/`SETID_DISABLED`）
4. [ ] 负例 D（跨 SetID 引用）：准备并断言
   - 在 `/org/setid?as_of=2026-01-01`：
     - 创建 `SetID=S2602`（若不存在）
     - 将 `Sales` 绑定为 `S2602`（`is_business_unit=true`）
   - 在 `/org/job-catalog?as_of=2026-01-01&setid=S2602`：
     - 创建最小 Job Profile（例如 `JP-OPS`；需先创建其 families）
    - 调用：`POST /org/api/positions?as_of=2026-01-16`（Update）
      - body：`{"effective_date":"2026-01-16","position_id":"<P-ENG-01 position_id>","org_code":"<R&D>","job_profile_id":"<JP-OPS-id>"}`
   - 断言：422 且 `code=JOBCATALOG_REFERENCE_NOT_FOUND`

## 8. 验收证据（最小）

- OrgUnit：
  - `/org/units?as_of=2026-01-01` 页面证据（Root + 5 节点可见）
  - 记录表：`root_org_code` + 5 个 L1 `org_code`
- SetID：
  - `/org/setid?as_of=2026-01-01` 页面证据（包含 `S2601`，以及绑定：Root→`DEFLT`、`R&D`→`S2601`、`Sales`→`S2602`）
  - 记录表：`root_org_code -> DEFLT`、`R&D -> S2601`、`Sales -> S2602`
- JobCatalog：
  - `/org/job-catalog?as_of=2026-01-01&setid=S2601` 页面证据（显示 `SetID: S2601`）
  - 两条 Job Family Group 的列表证据（含 `id`）
  - Job Families 列表证据（含 `id`，且可判定 group 归属）
  - reparenting 证据（`as_of=2026-01-15` 与 `as_of=2026-02-15` 的前后对比）
  - Job Levels 列表证据（含 `id`）
  - Job Profiles 列表证据（含 families + primary）
  - Profile families/primary 不变量负例证据（稳定报错即可）
  - fail-closed 负例证据（不存在/非法 `setid` 时的错误提示，且无 `SetID`）
- Position：
  - `/org/positions?as_of=2026-01-01&org_code=<R&D>` 页面证据（10 条职位可见，含 `position_id`）
  - 记录表：10 个 `position_id` + 对应 `org_code`
  - M5 正例：`/org/positions?as_of=2026-01-15&org_code=<R&D>` 页面证据（显示解析得到的 `SetID: S2601`；且至少 1 条职位显示 `org_code=<R&D>`、`jobcatalog_setid=S2601`、`job_profile=JP-SWE (...)`）
  - M5 负例：`/org/api/positions` 的失败响应证据（含 HTTP 状态码与 `code`：`invalid_request` / `job_profile_id is required` / `org_code_not_found` / `STAFFING_ORG_UNIT_NOT_FOUND_AS_OF` / `SETID_BINDING_MISSING` / `JOBCATALOG_REFERENCE_NOT_FOUND`）

## 9. 执行记录（Readiness/可复现记录）

> 说明：此处只记录“本次执行实际跑了什么、结果如何”；命令入口以 `AGENTS.md` 为准。执行时把 `[ ]` 改为 `[X]` 并补齐时间戳与结果摘要。

- [ ] 文档门禁：`make check doc` —— （待更新）
- [ ] E2E：`make e2e` —— （待更新；包含 `e2e/tests/tp060-02-master-data.spec.js`）
- [ ] Authz：`make authz-pack && make authz-test && make authz-lint` —— （待更新）
- [ ] 路由治理：`make check routing` —— （待更新）

## 10. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
