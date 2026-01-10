# DEV-PLAN-063：全链路业务测试子计划 TP-060-03——人员与任职（Person + Assignments）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：必须先完成 `docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`（positions 作为 assignments 输入来源）。

## 1. 背景与上下文（Context）

- **需求来源**：TP-060 总纲中的“人员与任职最小闭环”（`docs/dev-plans/060-business-e2e-test-suite.md`），以及 Phase 4 Person/Staffing 纵切片交付口径（`DEV-PLAN-009M2`）。
- **覆盖范围**：Person identity（`pernr → person_uuid`）+ Staffing assignments 时间线（position 绑定）。
- **业务价值**：为后续考勤（punches/daily results）与薪酬（payslip/items）提供稳定的“人员锚点 + 任职输入”。
- **关键不变量（必须成立）**：
  - `pernr` 为 1-8 位数字字符串；**前导 0 同值**（`DEV-PLAN-027`）。
  - Valid Time 为日粒度（date），测试固定 `as_of=2026-01-01`，避免执行期漂移（`DEV-PLAN-032`）。
  - 本套件测试数据必须保留（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 目标（Done 定义）

**Done-A（必须，当前子计划核心闭环）**

- [ ] 可创建/确认 10 个 Person（E01~E10），并记录每人的 `person_uuid`。
- [ ] `persons:by-pernr` 精确解析满足契约：  
  - 400：`code=PERSON_PERNR_INVALID`（非法 pernr）  
  - 404：`code=PERSON_NOT_FOUND`（不存在）  
  - 200：返回 `person_uuid/pernr/display_name/status`（存在）
- [ ] 可为 10 人创建/更新 Primary Assignment（绑定到 10 个 `position_id`），并可在 UI 时间线可见：
  - 必须展示 `effective_date`；
  - 页面不得展示 `end_date`（对齐当前 UI 合同）。

**Done-B（必须，为薪酬子计划准备输入；若写入口缺失则形成阻塞并记录）**

- [ ] 可为 10 人的 assignments 设置薪酬输入字段：`base_salary`（CNY）与 `allocated_fte`，且至少包含一条 `allocated_fte=0.5` 的样例（E04）。

### 2.2 非目标

- 不验证考勤/薪酬的计算结果（由 TP-060-04~08 承接）。
- 不在本子计划引入“临时绕过写入口”的双链路（例如手工 SQL 直改业务表）；若写入口缺失，按 §6.4 记录为阻塞并回到契约/实现处理。

### 2.3 工具链与门禁（SSOT 引用）

> 目的：声明本子计划执行/修复时可能命中的门禁入口；命令细节以 `AGENTS.md`/`Makefile` 为准。

- 触发器清单（勾选本计划命中的项；执行记录见对应 dev-record/PR）：
  - [ ] E2E（`make e2e`；若新增/维护 `tp060-03` 自动化用例）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`；若权限口径 drift）
  - [ ] 路由治理（`make check routing`；若新增 internal API/页面路由）
  - [X] 文档（`make check doc`；本文件变更）
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`；仅当为修复 drift 而改 Go 时）
  - [ ] DB/迁移（按模块 `make <module> plan/lint/migrate ...`；仅当为修复 drift 而改 DB 时）

- SSOT 链接：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`

## 3. 契约引用（SSOT）

- Person identity：`docs/dev-plans/027-person-minimal-identity-for-staffing.md`
- Assignments（事件 SoT + 同步投射）：`docs/dev-plans/031-greenfield-assignment-job-data.md`
- Valid Time（日粒度）：`docs/dev-plans/032-effective-date-day-granularity.md`
- 薪酬输入语义（base_salary/FTE）：`docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（示例 host：`t-060.localhost`；手工执行建议使用固定 host 以便复用与留证）
- `AS_OF_BASE`：`2026-01-01`
- 输入依赖：
  - 10 个 `position_id`（来自 TP-060-02；建议记录为 `P-ENG-01..` 等 10 个职位）
  - 10 个员工（E01~E10；基线表见 `docs/dev-plans/060-business-e2e-test-suite.md` §5.8）

### 4.1 固定映射（建议作为默认；便于后续子计划复用）

> 目的：消除“每次随便选一个 position”导致的漂移；后续考勤/薪酬子计划会引用这些人/岗位/差异字段。

| 编号 | pernr | display_name（建议） | position_name（来自 TP-060-02） | assignment_effective_date（建议） | base_salary（CNY） | allocated_fte |
| --- | --- | --- | --- | --- | --- | --- |
| E01 | `101` | Alice Zhang | `P-ENG-01` | `2026-01-01` | 20,000.00 | 1.0 |
| E02 | `102` | Bob Li | `P-SALES-01` | `2026-01-01` | 80,000.00 | 1.0 |
| E03 | `00000103` | Carol Wu | `P-ENG-02` | `2026-01-01` | 3,000.00 | 1.0 |
| E04 | `104` | David Chen | `P-ENG-02` | `2026-01-01` | 20,000.00 | 0.5 |
| E05 | `105` | Erin Sun | `P-MGR-01` | `2026-01-01` | 30,000.00 | 1.0 |
| E06 | `106` | Frank Zhou | `P-FIN-01` | `2026-01-15` | 25,000.00 | 1.0 |
| E07 | `107` | Grace Xu | `P-MGR-01` | `2026-01-01` | 35,000.00 | 1.0 |
| E08 | `108` | Henry Gao | `P-PLANT-01` | `2026-01-01` | 12,000.00 | 1.0 |
| E09 | `109` | Ivy He | `P-PLANT-02` | `2026-01-01` | 12,000.00 | 1.0 |
| E10 | `110` | Jack Lin | `P-SUPPORT-01` | `2026-01-01` | 15,000.00 | 1.0 |

### 4.2 可重复执行口径（Idempotency / Re-run）

> 目的：同一租户/同一环境重复跑本子计划时，避免“重复创建导致失败或脏数据”。

- Person：
  - 若 `pernr` 已存在：必须复用并记录其 `person_uuid`；不得重复创建同 pernr。
  - 若同 pernr 存在但 `display_name` 不符合预期：记录为 `ENV_DRIFT`（或 `CONTRACT_DRIFT`），并在 §9 说明是否允许覆盖/是否需要先清理环境。
- Assignment：
  - `/org/assignments` 的 upsert 为“写入/更新时间线”的动作；重复执行应表现为“同一 `effective_date` 的 slice 变更”或“幂等不变”，不得产生多条同日重复 slice（若出现，记录为 `BUG/CONTRACT_DRIFT`）。

### 4.3 数据保留（强制）

- 本子计划创建/复用的 10 个 Person 与 Assignments 属于 060-DS1 的“人员任职底座”，必须保留并在后续子计划复用（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

## 5. 接口与页面契约（UI/API Contracts）

### 5.1 Person（UI）

- `GET /person/persons?as_of=YYYY-MM-DD`：展示 persons 列表（含 `person_uuid`）。
- `POST /person/persons?as_of=YYYY-MM-DD`：创建 person（成功后 `303` 回到同页）。

### 5.2 Person identity（Internal API）

- `GET /person/api/persons:by-pernr?pernr=<digits_max8>`：
  - 200：`{"person_uuid","pernr","display_name","status"}`
  - 400：`code=PERSON_PERNR_INVALID`
  - 404：`code=PERSON_NOT_FOUND`

### 5.3 Assignments（UI）

- `GET /org/assignments?as_of=YYYY-MM-DD&pernr=<...>`：按 pernr 解析 person，展示 Timeline（含 `effective_date/assignment_id/position_id/status`）。
- `POST /org/assignments?as_of=YYYY-MM-DD`：upsert primary assignment（成功后 `303` 跳回 `/org/assignments?as_of=<effective_date>&pernr=<canonical_pernr>`）。
- 表单字段（写入）：
  - 必填：`effective_date`、`pernr`/`person_uuid`（其一必须能解析到 person）、`position_id`
  - 薪酬输入（为 Payroll 准备）：`base_salary`（CNY）、`allocated_fte`（(0,1]；默认 1.0）
- 负例提示（当前 UI 行为，便于排障）：
  - 缺少 pernr 且无 `person_uuid`：页面提示 `pernr is required`
  - `effective_date` 非法：页面提示 `effective_date 无效: ...`
  - `pernr` 与 `person_uuid` 不一致：页面提示 `pernr/person_uuid 不一致`

### 5.4 Assignments（Internal API）

- `POST /org/api/assignments?as_of=YYYY-MM-DD`：
  - body：`{"effective_date","person_uuid","position_id","base_salary","allocated_fte"}`
  - 说明：`base_salary/allocated_fte` 为可选字段（若缺省则沿用 DB kernel 默认/既有值；Payroll 子计划要求最终具备可计算输入）。

## 5.5 安全与鉴权（Security & Authz）

- 写入动作（Person 创建、Assignment upsert）要求 Tenant Admin；只读角色（若可分配）应对写入返回 403（SSOT：`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/060-business-e2e-test-suite.md` §4.4）。
- `persons:by-pernr` 为 internal API 读路径：未授权应 fail-closed（403 或等效稳定错误码）；缺少 tenant context 必须 fail-closed（500/稳定错误码均可，但不得返回数据）。

## 6. 测试步骤（执行时勾选）

> 约定：所有步骤都要显式使用 `as_of=2026-01-01`；出现偏差必须填写 §9 问题记录。

### 6.1 Person：创建/复用 10 人

1. [ ] 打开：`/person/persons?as_of=2026-01-01`
2. [ ] 逐个确认 E01~E10：
   - 若列表已存在相同 `pernr`：记录其 `person_uuid` 并复用（不得重复创建导致数据污染）。
   - 若不存在：创建并刷新，记录 `person_uuid`。

### 6.2 Person identity：前导 0 同值断言（精确解析）

> 断言目标：`pernr=00000103` 与 `pernr=103` 解析到同一 `person_uuid`，且 canonical `pernr` 一致。

1. [ ] 调用：`GET /person/api/persons:by-pernr?pernr=00000103`（记录 `person_uuid` 与返回的 `pernr`）
2. [ ] 调用：`GET /person/api/persons:by-pernr?pernr=103`（断言 `person_uuid` 与上一步一致；返回 `pernr` 应为 `103`）
3. [ ] 负例：`GET /person/api/persons:by-pernr?pernr=BAD` 必须 400 `PERSON_PERNR_INVALID`
4. [ ] 负例：`GET /person/api/persons:by-pernr?pernr=99999999` 必须 404 `PERSON_NOT_FOUND`

### 6.3 Assignments：绑定职位 + 时间线可见

1. [ ] 打开：`/org/assignments?as_of=2026-01-01`
2. [ ] 对 E01~E10 逐个执行：
   - 以 `GET /org/assignments?as_of=2026-01-01&pernr=<pernr>` 加载人员（页面应展示 `Person: <pernr/...> (<person_uuid>)`）。
   - `POST` upsert primary：按 §4.1 的映射选择一个 `position_id`，`effective_date=<assignment_effective_date>` 提交，断言 `303` 跳回 URL 中的 `pernr` 为 canonical 形式。
   - 断言 Timeline 表格可见，且包含：
     - `effective_date=<assignment_effective_date>`
     - `assignment_id`（记录）
     - `position_id`（记录）
     - `status`（记录）
3. [ ] UI 合同断言：Timeline 区域不得出现 `end_date` 字段/列名。
4. [ ] Valid Time 抽样（必须，覆盖“晚入职生效日”语义）：对 E06（`effective_date=2026-01-15`）验证 as-of 口径：
   - 在 `as_of=2026-01-01` 下访问 `GET /org/assignments?as_of=2026-01-01&pernr=106`：Timeline 应为空（或明确提示 empty）。
   - 在 `as_of=2026-01-15` 下访问 `GET /org/assignments?as_of=2026-01-15&pernr=106`：Timeline 应包含 `effective_date=2026-01-15` 的记录。
   - 若上述行为不成立或无法解释，记录为 `CONTRACT_DRIFT/BUG` 并在 §9 给出证据。

### 6.4 薪酬输入字段（base_salary/allocated_fte）：就绪性与阻塞判定

> 目标：为 TP-060-07/08 提供可计算输入（否则 payroll kernel 将 fail-closed 报错）。

1. [ ] 确认 **唯一写入口**（应当存在）可为 assignment 写入：
   - UI：`/org/assignments?as_of=...` 表单支持 `base_salary/allocated_fte`
   - Internal API：`/org/api/assignments?as_of=...` 支持 `base_salary/allocated_fte`
2. [ ] 按 060-DS1 为 E01~E10 写入 `base_salary/allocated_fte`，并记录 E04 的 `allocated_fte=0.5` 证据。
3. [ ] 若环境中仍缺失（不应发生）：在 §9 记录为 `CONTRACT_MISSING/CONTRACT_DRIFT`，并明确这是 TP-060-07/08 的阻塞点（不得用 SQL 直改绕过 One Door）；同时记录“当前可见入口”现状（页面/路由/表单字段缺失的证据）。

## 7. 依赖与里程碑（Dependencies & Milestones）

- 依赖（必须先满足）：
  - [ ] TP-060-02 已完成并能提供 10 个 positions（`docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`）。
- 里程碑（本子计划内完成）：
  1. [ ] 建立/复用 10 个 Person，并记录 `pernr -> person_uuid`。
  2. [ ] 完成 `persons:by-pernr` 的正例/负例断言（含前导 0 同值）。
  3. [ ] 完成 10 人 primary assignment 绑定并留证（timeline 可见）。
  4. [ ] 写入并留证 `base_salary/allocated_fte`（至少包含 E04 的 `allocated_fte=0.5`；若缺失则记录阻塞）。

## 8. 验收证据（最小）

- `/person/persons?as_of=2026-01-01`：10 人列表证据（含 `pernr/person_uuid`）。
- `persons:by-pernr`：`00000103` 与 `103` 命中同一 `person_uuid` 的证据；以及 1 条 `PERSON_PERNR_INVALID` 的 400 负例证据。
- `/org/assignments?...`：10 人 Timeline 证据（含 `assignment_id/position_id/effective_date/status`），并证明 UI 未展示 `end_date`。
- `base_salary/allocated_fte` 写入证据（至少包含 E04 的 `allocated_fte=0.5`）。

建议在证据中维护一份可复用映射表（后续子计划直接引用）：

| 编号 | pernr | person_uuid | primary assignment_id | position_id | 备注（FTE/薪资/特殊断言） |
| --- | --- | --- | --- | --- | --- |
| E01 |  |  |  |  |  |
| E02 |  |  |  |  |  |
| E03 |  |  |  |  |  |
| E04 |  |  |  |  | `allocated_fte=0.5` |
| E05 |  |  |  |  |  |
| E06 |  |  |  |  | `effective_date=2026-01-15` |
| E07 |  |  |  |  | `target_net` 输入由 TP-060-08 承接 |
| E08 |  |  |  |  |  |
| E09 |  |  |  |  |  |
| E10 |  |  |  |  |  |

## 9. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
