# DEV-PLAN-062：全链路业务测试子计划 TP-060-02——主数据（组织架构 + SetID + JobCatalog + 职位）

**状态**: 草拟中（2026-01-10 14:29 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`（可登录 + 隔离基线）。

## 1. 背景与上下文（Context）

- **需求来源**：TP-060 总纲中的“主数据最小可复现闭环”（`docs/dev-plans/060-business-e2e-test-suite.md`），以及 Phase 4 纵切片的交付口径（`DEV-PLAN-009`）。
- **覆盖范围**：OrgUnit → SetID → JobCatalog → Position（契约分别对齐 `docs/dev-plans/026/028/029/030`）。
- **业务价值**：为后续 TP-060-03（Person/Assignments）及考勤/薪酬子计划提供稳定的主数据底座，避免“不可见/不可操作”的僵尸交付（见 `AGENTS.md` 的用户可见性原则）。
- **关键不变量（必须 fail-closed）**：
  - `as_of` 为日粒度（date），所有读取必须显式传入 `as_of=YYYY-MM-DD`（对齐 `docs/dev-plans/032-effective-date-day-granularity.md`）。
  - SetID 映射矩阵**无缺省洞**：新 BU 创建时会为各 Record Group 自动补齐映射到 `SHARE`；解析不做运行时“隐式回退”（对齐 `docs/dev-plans/028-setid-management.md`）。
  - 对不存在/disabled 的 BU 必须 fail-closed（不得解析出任何 SetID）。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] **OrgUnit**：可在 `/org/nodes` 完成 Root + 5 个一级部门创建，刷新后列表可见；并记录每个 `org_unit_id`。
- [ ] **SetID**：可在 `/org/setid` 完成 SetID/BU/mapping 的创建与保存；映射矩阵无缺省洞（新 BU 自动补齐到 `SHARE`）；不存在/disabled BU 必须 fail-closed。
- [ ] **JobCatalog**：在 `/org/job-catalog` 能看到 `Resolved SetID: S2601`，并能创建至少 2 条 Job Family Group，刷新后列表可见且包含 `id`。
- [ ] **Position**：在 `/org/positions` 能创建 10 条职位，刷新后列表可见且包含 `position_id`；创建时 OrgUnit 下拉可用（不出现 `(no org units)`）。

### 2.2 非目标

- 不在本子计划强制覆盖 JobCatalog 的 families/levels/profiles（若环境已实现可作为扩展验证；若未实现记录为 `SCOPE_GAP`）。

### 2.3 工具链与门禁（SSOT 引用）

> 目的：避免在子计划里复制工具链/脚本细节导致 drift；本文仅声明“本子计划执行/修复时可能命中的门禁入口”，具体命令以 `AGENTS.md`/`Makefile`/CI workflow 为准。

- **触发器清单（勾选本计划命中的项；执行记录见 §9）**：
  - [ ] E2E（`make e2e`，套件入口对齐 `docs/dev-plans/060-business-e2e-test-suite.md`）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [ ] 路由治理（`make check routing`，必要时更新 `config/routing/allowlist.yaml`）
  - [ ] 文档（`make check doc`）
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）——仅当为修复 drift 而改 Go 时
  - [ ] DB/迁移（按模块 `make <module> plan/lint/migrate ...`）——仅当为修复 drift 而改 DB 时
  - [ ] sqlc（`make sqlc-generate`）——仅当为修复 drift 而改 sqlc 输入时

- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`

## 3. 契约引用（SSOT）

- OrgUnit：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- SetID：`docs/dev-plans/028-setid-management.md`
- JobCatalog：`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- Position：`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
- Valid Time（日粒度）：`docs/dev-plans/032-effective-date-day-granularity.md`
- 路由/UI 可见性：`AGENTS.md`、`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- Authz（对象/动作冻结）：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 路由策略（route_class/allowlist）：`docs/dev-plans/017-routing-strategy.md`

## 4. 前置条件与环境（Prerequisites）

### 4.1 租户与 Host

- Tenant：`T060`（示例 host：`t-060.localhost`）
- `as_of`：固定使用 `2026-01-01`（本子计划所有 URL 必须显式带 `as_of`，避免被缺省重定向到“当天”导致不可复现）

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
- 若环境允许但数据已明显污染（重复 Root、重复 BU/SetID/JobFamilyGroup、职位数量异常且无法判定）：记录为 `ENV_DRIFT`，建议重置租户或更换干净租户再跑（不得在测试中隐式修补口径）。

## 5. 数据准备要求（060-DS1 子集 + 本计划增量）

> 基线数据 SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` 的 §5.2–§5.5。本文只声明本子计划用到的“子集与增量”。

### 5.1 最小数据（必须）

- OrgUnit：
  - Root：`Bugs & Blossoms Co., Ltd.`（若环境已有 Root，可复用；必须记录其 `org_unit_id`）
  - L1：`HQ`、`R&D`、`Sales`、`Ops`、`Plant`（Parent ID = Root）
- SetID/BU（Record Group：`jobcatalog`）：
  - SetID：`S2601`
  - BU：`BU000`、`BU901`
  - Mapping：`BU000 -> SHARE`、`BU901 -> S2601`
  - 断言：无缺省洞（对任一已存在 BU，必须存在映射；新建 BU 默认映射到 `SHARE`）
- JobCatalog（`business_unit_id=BU901` → `Resolved SetID=S2601`）：
  - Job Family Group：至少 2 条（建议使用 `JFG-ENG`、`JFG-SALES`）
- Position：10 条（建议命名见 §7.4；需覆盖多个 OrgUnit）

### 5.2 增量数据（用于无缺省洞 + fail-closed）

- 新建一个 BU（例如 `BU902`），不做任何 mapping 修改：用于验证“无缺省洞”（应自动映射到 `SHARE`）。
- 使用一个不存在的 BU（例如 `BU999`，**不**创建）：用于验证 JobCatalog 解析链路对不存在 BU 的 fail-closed。

## 6. 路由与交互契约（UI Contracts，便于排障）

> 说明：本节用于把“测试动作”具体化到 UI 路由与关键表单字段，便于定位 drift。若实现已变更，请优先记录为 `CONTRACT_DRIFT` 并指向对应 SSOT。

### 6.1 OrgUnit：`/org/nodes`（UI）

- `GET /org/nodes?as_of=YYYY-MM-DD`：展示节点列表与创建表单。
- `POST /org/nodes?as_of=YYYY-MM-DD`：创建节点（成功后 `303` 跳回 `/org/nodes?as_of=<effective_date>`）。
- 关键字段：
  - `effective_date`（默认：`as_of`；必须是 `YYYY-MM-DD`）
  - `parent_id`（可空）
  - `name`（必填；空值应提示 `name is required`）
- Authz 口径：`GET=read`，`POST=admin`（对齐 `docs/dev-plans/022-authz-casbin-toolchain.md`）。

### 6.2 SetID：`/org/setid`（UI）

- `GET /org/setid?as_of=YYYY-MM-DD`：展示 SetIDs、Business Units、Mappings（jobcatalog）。
- `POST /org/setid?as_of=YYYY-MM-DD`：三类动作（成功后 `303` 跳回 `/org/setid?as_of=...`）：
  - `action=create_setid`：`setid` + `name` 必填
  - `action=create_bu`：`business_unit_id` + `name` 必填
  - `action=save_mappings`：表单字段形如 `map_<BU>=<SETID>`
- Authz 口径：`GET=read`，`POST=admin`。

### 6.3 JobCatalog：`/org/job-catalog`（UI）

- `GET /org/job-catalog?as_of=YYYY-MM-DD&business_unit_id=BUxxx`
  - 页面应显示 `Resolved SetID: <setid>`；若解析失败，应显式显示错误信息，且不得展示错误的 `Resolved SetID`。
- `POST /org/job-catalog?as_of=...&business_unit_id=...`：创建 Job Family Group（成功后 `303` 跳回 `/org/job-catalog?business_unit_id=...&as_of=<effective_date>`）。
- 关键字段：
  - `effective_date`（默认：`as_of`）
  - `business_unit_id`（用于 SetID 解析）
  - `code`/`name`（必填；空值应提示 `code/name is required`）
  - `description`（可空）
- Authz 口径：`GET=read`，`POST=admin`。

### 6.4 Position：`/org/positions`（UI）

- `GET /org/positions?as_of=YYYY-MM-DD`：展示 OrgUnit 下拉与职位列表（需能看到 `position_id`、`org_unit_id`、`effective_date`）。
- `POST /org/positions?as_of=YYYY-MM-DD`：创建职位（成功后 `303` 跳回 `/org/positions?as_of=<effective_date>`）。
- 关键字段：
  - `effective_date`（默认：`as_of`；非法日期应提示 `effective_date 无效: ...`）
  - `org_unit_id`
  - `name`
- Authz 口径：`GET=read`，`POST=admin`。

## 7. 测试步骤（执行时勾选）

> 约定：每个步骤都要把关键 ID 记录到“验收证据”里（§8），并在出现偏差时填“问题记录”（§10）。

### 7.1 OrgUnit：创建与可见

1. [ ] 打开：`/org/nodes?as_of=2026-01-01`
2. [ ] 确认 Root（若不存在则创建；若已存在则记录并复用）
   - 表单：`effective_date=2026-01-01`，`parent_id=`（空），`name=Bugs & Blossoms Co., Ltd.`
   - 断言：Root 在列表可见，并能看到其 `org_unit_id`
   - 约束：若列表已存在大量节点且无法确认 Root（例如已有多个候选 Root），记录为 `ENV_DRIFT` 并停止 OrgUnit 步骤
3. [ ] 确认 5 个 L1 节点（缺失则创建；已有则记录并复用）
   - `HQ`、`R&D`、`Sales`、`Ops`、`Plant`
   - 断言：刷新后列表可见，且每条都有 `org_unit_id`
4. [ ] 负例：提交空 `name`
   - 断言：页面提示 `name is required`；不得创建新节点

### 7.2 SetID：创建与 mapping 保存

1. [ ] 打开：`/org/setid?as_of=2026-01-01`
2. [ ] 断言：已存在可用于共享的 `SHARE` SetID（若不存在，记录为 `CONTRACT_DRIFT/ENV_DRIFT` 并停止后续 SetID 步骤）
3. [ ] 确认 SetID：`S2601`（缺失则创建；已有则记录并复用）
   - 断言：SetIDs 表格存在 `S2601`，状态为 active（或记录实际状态）
4. [ ] 确认 BU：`BU901`（缺失则创建；已有则记录并复用；若 `BU000` 不存在则补建 `BU000`，名称可复用 060-DS1）
5. [ ] 保存 mappings（Record Group：jobcatalog）
   - 目标：`BU000 -> SHARE`、`BU901 -> S2601`
   - 断言：保存后刷新仍保持上述映射（防“表单保存但未落库”）

### 7.3 JobCatalog：解析链路、写入闭环与 fail-closed 负例

1. [ ] 打开：`/org/job-catalog?as_of=2026-01-01&business_unit_id=BU901`
2. [ ] 断言：页面显示 `Resolved SetID: S2601`（且无错误提示）
3. [ ] 确认 Job Family Group（至少 2 条；缺失则创建；已有则记录并复用）
   - 建议：
     - `code=JFG-ENG`，`name=Engineering`
     - `code=JFG-SALES`，`name=Sales`
   - 断言：创建后列表可见，且每条包含 `id`；记录两条 `job_family_group_id`
4. [ ] 无缺省洞验证：新建 BU `BU902`，不修改 mapping，然后访问：
   - 先回到：`/org/setid?as_of=2026-01-01`
   - 用 `action=create_bu` 创建 `BU902`（仅创建 BU；不需要保存 mappings）
   - `/org/job-catalog?as_of=2026-01-01&business_unit_id=BU902`
   - 断言：页面显示 `Resolved SetID: SHARE`（因为 BU 创建时自动补齐映射到 `SHARE`）
5. [ ] fail-closed 负例：使用不存在 BU（例如 `BU999`），直接访问：
   - `/org/job-catalog?as_of=2026-01-01&business_unit_id=BU999`
   - 断言：页面显式报错，且不得显示 `Resolved SetID`

### 7.4 Position：创建与列表可见

1. [ ] 打开：`/org/positions?as_of=2026-01-01`
2. [ ] 断言：OrgUnit 下拉不为 `(no org units)`，且选项包含你在 §7.1 创建的部门（名称 + `org_unit_id`）
3. [ ] 确保至少 10 个职位（不足则补齐创建；已有则记录其中 10 条）
   - 建议命名（可按 OrgUnit 分配；若重复可加后缀）：
     - `P-ENG-01`、`P-ENG-02`
     - `P-SALES-01`
     - `P-HR-01`、`P-FIN-01`、`P-MGR-01`
     - `P-OPS-01`、`P-SUPPORT-01`
     - `P-PLANT-01`、`P-PLANT-02`
   - 断言：每次创建后 `303` 跳转回 `/org/positions?as_of=2026-01-01`；列表出现新行并包含 `position_id`；记录 10 条 `position_id`
4. [ ] 负例：提交非法 `effective_date`（例如 `bad`）
   - 断言：页面提示 `effective_date 无效: ...`；不得创建新职位

## 8. 验收证据（最小）

- OrgUnit：
  - `/org/nodes?as_of=2026-01-01` 页面证据（Root + 5 节点可见）
  - 记录表：`root_org_unit_id` + 5 个 L1 `org_unit_id`
- SetID：
  - `/org/setid?as_of=2026-01-01` 页面证据（包含 `S2601`、`BU901`、Mappings）
  - 记录表：`BU000->SHARE`、`BU901->S2601`
- JobCatalog：
  - `/org/job-catalog?as_of=2026-01-01&business_unit_id=BU901` 页面证据（显示 `Resolved SetID: S2601`）
  - 两条 Job Family Group 的列表证据（含 `id`）
  - 无缺省洞证据（`BU902` 显示 `Resolved SetID: SHARE`）
  - fail-closed 负例证据（`BU999` 不存在时的错误提示，且无 `Resolved SetID`）
- Position：
  - `/org/positions?as_of=2026-01-01` 页面证据（10 条职位可见，含 `position_id`）
  - 记录表：10 个 `position_id` + 对应 `org_unit_id`

## 9. 执行记录（Readiness/可复现记录）

> 说明：此处只记录“本次执行实际跑了什么、结果如何”；命令入口以 `AGENTS.md` 为准。执行时把 `[ ]` 改为 `[X]` 并补齐时间戳与结果摘要。

- [X] 文档门禁：`make check doc` —— （2026-01-10 14:32 UTC，结果：PASS）
- [X] E2E：`make e2e` —— （2026-01-10 14:29 UTC，结果：PASS；包含 `e2e/tests/tp060-02-master-data.spec.js`）
- [X] Authz：`make authz-pack && make authz-test && make authz-lint` —— （2026-01-10 14:32 UTC，结果：PASS）
- [X] 路由治理：`make check routing` —— （2026-01-10 14:32 UTC，结果：PASS）

## 10. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
