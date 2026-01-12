# DEV-PLAN-063：全链路业务测试子计划 TP-060-03——人员与任职（Person + Assignments）

**状态**: 已完成（2026-01-11；证据：`docs/dev-records/dev-plan-063-execution-log.md`）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：必须先完成 `docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`（positions 作为 assignments 输入来源）。  
> 执行日志：`docs/dev-records/dev-plan-063-execution-log.md`

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

- [X] 可创建/确认 10 个 Person（E01~E10），并记录每人的 `person_uuid`。
- [X] `persons:by-pernr` 精确解析满足契约：  
  - 400：`code=PERSON_PERNR_INVALID`（非法 pernr）  
  - 404：`code=PERSON_NOT_FOUND`（不存在）  
  - 200：返回 `person_uuid/pernr/display_name/status`（存在）
- [X] 可为 10 人创建/更新 Primary Assignment（绑定到 10 个 `position_id`），并可在 UI 时间线可见：
  - 必须展示 `effective_date`；
  - 页面不得展示 `end_date`（对齐当前 UI 合同）。

**Done-B（必须，为薪酬子计划准备输入；若写入口缺失则形成阻塞并记录）**

- [X] 可为 10 人的 assignments 设置薪酬输入字段：`base_salary`（CNY）与 `allocated_fte`，且至少包含一条 `allocated_fte=0.5` 的样例（E04）。

**Done-C（必须，自动化回归）**

- [X] `make e2e` 覆盖本子计划的最小链路（见 `e2e/tests/tp060-03-person-and-assignments.spec.js`）。
- [ ] 自动化补齐（对齐 `DEV-PLAN-031`）：覆盖“多切片 Valid Time（同一人两次不同 effective_date）”与“同日 upsert 可重复执行（相同输入幂等；不同输入 fail-closed=409 `STAFFING_IDEMPOTENCY_REUSED`）”。

**Done-D（必须，Position/Assignment 交叉不变量：fail-closed）**

> 说明：本条对齐 `DEV-PLAN-030`（Position）与 `DEV-PLAN-031`（Assignments）的交叉口径：不允许 UI/服务层临时兜底分支；必须由 Kernel 统一裁决并返回稳定错误码。

- [X] 禁止把任职写入 disabled position：422 `STAFFING_POSITION_DISABLED_AS_OF`
- [X] Position disable 与 active assignment 冲突必须 fail-closed：422 `STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF`
- [X] 容量裁决（M6a）：`allocated_fte > capacity_fte` 必须 fail-closed：422 `STAFFING_POSITION_CAPACITY_EXCEEDED`
- [X] 降容裁决（M6a）：Position 降容导致 `allocated_fte > capacity_fte` 必须 fail-closed：422 `STAFFING_POSITION_CAPACITY_EXCEEDED`
- [ ] Position 排他（M2）：同一时点一个 position 不得被多个 active assignment 占用；违反必须 fail-closed（稳定错误码优先，建议：422 `STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF`）。

### 2.2 非目标

- 不验证考勤/薪酬的计算结果（由 TP-060-04~08 承接）。
- 不在本子计划引入“临时绕过写入口”的双链路（例如手工 SQL 直改业务表）；若写入口缺失，按 §8.4 记录为阻塞并回到契约/实现处理。

### 2.3 工具链与门禁（SSOT 引用）

> 目的：声明本子计划执行/修复时可能命中的门禁入口；命令细节以 `AGENTS.md`/`Makefile` 为准。

- 触发器清单（勾选本计划命中的项；执行记录见对应 dev-record/PR）：
  - [X] E2E（`make e2e`；用例：`e2e/tests/tp060-03-person-and-assignments.spec.js`）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`；若权限口径 drift）
  - [ ] 路由治理（`make check routing`；若新增 internal API/页面路由）
  - [X] 文档（`make check doc`）
  - [X] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] DB/迁移（按模块 `make <module> plan/lint/migrate ...`；仅当为修复 drift 而改 DB 时）

- SSOT 链接：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`

### 2.4 契约引用（SSOT）

- Person identity：`docs/dev-plans/027-person-minimal-identity-for-staffing.md`
- Assignments（事件 SoT + 同步投射）：`docs/dev-plans/031-greenfield-assignment-job-data.md`
- Valid Time（日粒度）：`docs/dev-plans/032-effective-date-day-granularity.md`
- 薪酬输入语义（base_salary/FTE）：`docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 手工执行 vs 自动化（两条链路的职责分工）

- **手工执行（060-DS1 固定数据集）**：在 `T060` 下复用 TP-060-02 的 10 个 positions，按本文 §4.3 固定映射建立 10 个 Person + Primary Assignment，并把 `pernr/person_uuid/assignment_id/position_id` 记录为后续子计划可复用的证据。
- **自动化回归（隔离数据）**：`e2e/tests/tp060-03-person-and-assignments.spec.js` 使用 `runID` 创建独立 tenant 与独立数据，保证可重复跑；覆盖 `pernr` 解析/valid time/交叉不变量/同日幂等等断言；**不替代** 060-DS1 的“固定可复现数据集”证据。

### 3.2 `pernr` 规范化与写侧权威输入

- `pernr` 规范化（canonical）：去掉前导 `0`；若全为 `0`，canonical 为 `0`（对齐实现：`normalizePernr`）。
- Assignment 写入的权威输入为 `person_uuid`：UI 可通过 `pernr` 解析出 `person_uuid`，但最终必须以 `person_uuid` 调用写入口（对齐 `docs/dev-plans/031-greenfield-assignment-job-data.md` 的“写侧不以 pernr 作为主键”）。
- 若同时提交 `pernr` 与 `person_uuid`：必须强校验二者一致，否则 fail-closed（UI 应提示 `pernr/person_uuid 不一致`）。

### 3.3 One Door / No Legacy（写入口唯一）

- `base_salary/allocated_fte` 的写入必须通过：`/org/assignments`（UI）或 `/org/api/assignments`（Internal API）。
- 禁止通过 SQL 直改业务表绕过写入口（对齐 `AGENTS.md` 的 One Door / No Legacy）。

## 4. 前置条件与数据准备（Prerequisites）

- Tenant：`T060`（示例 host：`t-060.localhost`；手工执行建议使用固定 host 以便复用与留证）
- `AS_OF_BASE`：`2026-01-01`
- 输入依赖：
  - 10 个 `position_id`（来自 TP-060-02；建议记录为 `P-ENG-01..` 等 10 个职位）
  - 10 个员工（E01~E10；基线表见 `docs/dev-plans/060-business-e2e-test-suite.md` §5.8）

### 4.1 `as_of` 缺省行为（避免执行期漂移）

- UI 路由：若 `GET` 未提供 `as_of`，服务端会 `302` 重定向补上 `as_of=<当前UTC日期>`。
- UI 路由：若 `POST` 未提供 `as_of`，服务端会使用 `as_of=<当前UTC日期>` 作为默认值。
- Internal API：若未提供 `as_of`，服务端会使用 `as_of=<当前UTC日期>` 作为默认值。
- 结论：本子计划所有步骤必须显式使用固定 `as_of=2026-01-01`（或本文明确要求的其它日期，如 E06 的 `2026-01-15`）。

### 4.2 固定映射（建议作为默认；便于后续子计划复用）

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

### 4.3 可重复执行口径（Idempotency / Re-run）

> 目的：同一租户/同一环境重复跑本子计划时，避免“重复创建导致失败或脏数据”。

- Person：
  - 若 `pernr` 已存在：必须复用并记录其 `person_uuid`；不得重复创建同 pernr。
  - 若同 pernr 存在但 `display_name` 不符合预期：记录为 `ENV_DRIFT`（或 `CONTRACT_DRIFT`），并在 §11 说明是否允许覆盖/是否需要先清理环境。
- Assignment：
  - `/org/assignments` 的 upsert 为“写入/更新时间线”的动作；重复执行应表现为“同一 `effective_date` 的 slice 变更”或“幂等不变”，不得产生多条同日重复 slice（若出现，记录为 `BUG/CONTRACT_DRIFT`）。
  - **同日幂等（稳定可断言）**：
    - 同一 `person_uuid` + 同一 `effective_date` + 相同输入：必须成功（UI 为 303；Internal API 为 200）。
    - 同一 `person_uuid` + 同一 `effective_date` + 不同输入：必须 fail-closed，且返回稳定错误码（建议：409 `STAFFING_IDEMPOTENCY_REUSED`）。
  - **多切片 Valid Time（稳定可断言）**：同一人用两个不同的 `effective_date` 连续 upsert 后，`as_of` 前后读到的 snapshot 必须不同（至少体现在 `effective_date` 版本切换上）。

### 4.4 数据保留（强制）

- 本子计划创建/复用的 10 个 Person 与 Assignments 属于 060-DS1 的“人员任职底座”，必须保留并在后续子计划复用（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

## 5. 数据模型与约束（Data Model & Constraints）

> 目的：把“测试用到的最小字段/约束/错误码”冻结成可直接编码/断言的合同，避免执行期猜测（对齐 `docs/dev-plans/001-technical-design-template.md`）。

### 5.1 Person（Person Identity）

- 关键字段：
  - `person_uuid`：uuid 字符串（只读，由系统生成）。
  - `pernr`：1-8 位数字字符串；canonical 规则见 §3.2。
  - `display_name`：展示名（可为空，但建议填充以便后续子计划 UI 可读）。
  - `status`：例如 `active`（以实际实现为准；本子计划只要求能稳定返回）。

### 5.2 Assignment（Primary Assignment，时间线）

- 关键字段（UI/证据最小集）：
  - `assignment_id`：uuid 字符串（只读，由系统生成或派生）。
  - `person_uuid`：uuid 字符串（写侧权威输入）。
  - `position_id`：uuid 字符串（来自 TP-060-02）。
  - `effective_date`：`YYYY-MM-DD`（date；Valid Time 日粒度）。
  - `status`：例如 `active`。
- 展示合同：Timeline 只展示 `effective_date`，不得展示 `end_date`（对齐 `docs/dev-plans/031-greenfield-assignment-job-data.md`、`docs/dev-plans/032-effective-date-day-granularity.md`）。

### 5.3 薪酬输入字段（为 Payroll 准备）

- `base_salary`：CNY；以“十进制字符串”写入（例如 `"20000.00"`）。
- `allocated_fte`：`(0, 1]`；以“十进制字符串”写入（例如 `"0.50"`）。
  - UI 建议约束：`min=0.01`、`max=1.00`、`step=0.01`（以实际页面为准）。

## 6. 接口与页面契约（UI/API Contracts）

### 6.1 Person（UI）

- `GET /person/persons?as_of=YYYY-MM-DD`：展示 persons 列表（含 `person_uuid`）。
- `POST /person/persons?as_of=YYYY-MM-DD`：创建 person（成功后 `303` 回到同页）。

### 6.2 Person identity（Internal API）

- `GET /person/api/persons:by-pernr?pernr=<digits_max8>`：
  - 200：`{"person_uuid","pernr","display_name","status"}`
  - 400：`code=PERSON_PERNR_INVALID`
  - 404：`code=PERSON_NOT_FOUND`
  - 500：`code=PERSON_INTERNAL`（或 `tenant_missing`；不得返回任何 person 数据）

### 6.3 Assignments（UI）

- `GET /org/assignments?as_of=YYYY-MM-DD&pernr=<...>`：按 pernr 解析 person，展示 Timeline（含 `effective_date/assignment_id/position_id/status`）。
- `GET /org/assignments?as_of=YYYY-MM-DD&person_uuid=<...>`：按 `person_uuid` 加载人员（用于无需 pernr 的自动化/排障）。
- `POST /org/assignments?as_of=YYYY-MM-DD`：upsert primary assignment：
  - 成功后 `303` 跳回：`/org/assignments?as_of=<effective_date>&pernr=<canonical_pernr>`（若使用 pernr 提交）
  - 或 `303` 跳回：`/org/assignments?as_of=<effective_date>&person_uuid=<person_uuid>`（若仅使用 person_uuid 提交）
- 表单字段（写入）：
  - 必填：`effective_date`、`pernr`/`person_uuid`（其一必须能解析到 person）、`position_id`
  - 薪酬输入（为 Payroll 准备）：`base_salary`（CNY）、`allocated_fte`（(0,1]；默认 1.0）
- 负例提示（当前 UI 行为，便于排障）：
  - 缺少 pernr 且无 `person_uuid`：页面提示 `pernr is required`
  - `effective_date` 非法：页面提示 `effective_date 无效: ...`
  - `pernr` 与 `person_uuid` 不一致：页面提示 `pernr/person_uuid 不一致`

### 6.4 Assignments（Internal API）

- `GET /org/api/assignments?as_of=YYYY-MM-DD&person_uuid=<uuid>`：
  - 200：`{"as_of","tenant","person_uuid","assignments":[...]}`（当前实现的 `assignments[]` 元素字段为：`AssignmentID/PersonUUID/PositionID/Status/EffectiveAt`）
  - 400：`code=missing_person_uuid`（缺少 `person_uuid`）、或 `code=invalid_as_of`
  - 500：`code=list_failed`（list 出错）、或 `code=tenant_missing`
- `POST /org/api/assignments?as_of=YYYY-MM-DD`：
  - body：`{"effective_date","person_uuid","position_id","base_salary","allocated_fte"}`（`effective_date` 缺省时默认为 `as_of`；薪酬字段可选）
  - 400：`code=bad_json` / `code=invalid_effective_date` / `code=upsert_failed`
  - 500：`code=tenant_missing`

### 6.5 Positions（Internal API，用于交叉不变量的稳定错误码断言）

> 说明：Position UI 的写入入口是 `/org/positions`；但负例断言优先使用 Internal API 提取稳定 `code`，避免 UI 红字提示无法标准化采集。

- `POST /org/api/positions?as_of=YYYY-MM-DD`：
  - Update：`{"effective_date","position_id", ...}`（至少 1 个 patch 字段）
  - 常用 patch 字段：
    - disable：`{"position_id","lifecycle_status":"disabled"}`
    - reports_to：`{"position_id","reports_to_position_id":"<uuid>"}`（forward-only）
    - capacity：`{"position_id","capacity_fte":"0.50"}`
  - 422：稳定错误码（示例：`STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF` / `STAFFING_POSITION_CAPACITY_EXCEEDED` / `STAFFING_POSITION_DISABLED_AS_OF` / `STAFFING_POSITION_REPORTS_TO_SELF` / `STAFFING_INVALID_ARGUMENT`）

## 7. 安全与鉴权（Security & Authz）

- 写入动作（Person 创建、Assignment upsert）要求 Tenant Admin；只读角色（若可分配）应对写入返回 403（SSOT：`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/060-business-e2e-test-suite.md` §4.4）。
- `persons:by-pernr` 为 internal API 读路径：未授权应 fail-closed（403 或等效稳定错误码）；缺少 tenant context 必须 fail-closed（500/稳定错误码均可，但不得返回数据）。

## 8. 测试步骤（执行时勾选）

> 约定：所有步骤都要显式使用 `as_of=2026-01-01`；出现偏差必须填写 §11 问题记录。

### 8.1 Person：创建/复用 10 人

1. [ ] 打开：`/person/persons?as_of=2026-01-01`
2. [ ] 逐个确认 E01~E10：
   - 若列表已存在相同 `pernr`：记录其 `person_uuid` 并复用（不得重复创建导致数据污染）。
   - 若不存在：创建并刷新，记录 `person_uuid`。

### 8.2 Person identity：前导 0 同值断言（精确解析）

> 断言目标：`pernr=00000103` 与 `pernr=103` 解析到同一 `person_uuid`，且 canonical `pernr` 一致。

1. [ ] 调用：`GET /person/api/persons:by-pernr?pernr=00000103`（记录 `person_uuid` 与返回的 `pernr`）
2. [ ] 调用：`GET /person/api/persons:by-pernr?pernr=103`（断言 `person_uuid` 与上一步一致；返回 `pernr` 应为 `103`）
3. [ ] 负例：`GET /person/api/persons:by-pernr?pernr=BAD` 必须 400 `PERSON_PERNR_INVALID`
4. [ ] 负例：`GET /person/api/persons:by-pernr?pernr=99999999` 必须 404 `PERSON_NOT_FOUND`

### 8.3 Assignments：绑定职位 + 时间线可见

1. [ ] 打开：`/org/assignments?as_of=2026-01-01`
2. [ ] 对 E01~E10 逐个执行：
   - 以 `GET /org/assignments?as_of=2026-01-01&pernr=<pernr>` 加载人员（页面应展示 `Person: <pernr/...> (<person_uuid>)`）。
   - `POST` upsert primary：按 §4.2 的映射选择一个 `position_id`，`effective_date=<assignment_effective_date>` 提交，断言 `303` 跳回 URL 中的 `pernr` 为 canonical 形式。
   - 断言 Timeline 表格可见，且包含：
     - `effective_date=<assignment_effective_date>`
     - `assignment_id`（记录）
     - `position_id`（记录）
     - `status`（记录）
3. [ ] UI 合同断言：Timeline 区域不得出现 `end_date` 字段/列名。
4. [ ] Valid Time 抽样（必须，覆盖“晚入职生效日”语义）：对 E06（`effective_date=2026-01-15`）验证 as-of 口径：
   - 在 `as_of=2026-01-01` 下访问 `GET /org/assignments?as_of=2026-01-01&pernr=106`：Timeline 应为空（或明确提示 empty）。
   - 在 `as_of=2026-01-15` 下访问 `GET /org/assignments?as_of=2026-01-15&pernr=106`：Timeline 应包含 `effective_date=2026-01-15` 的记录。
   - 若上述行为不成立或无法解释，记录为 `CONTRACT_DRIFT/BUG` 并在 §11 给出证据。

### 8.4 薪酬输入字段（base_salary/allocated_fte）：就绪性与阻塞判定

> 目标：为 TP-060-07/08 提供可计算输入（否则 payroll kernel 将 fail-closed 报错）。

1. [ ] 确认 **唯一写入口**（应当存在）可为 assignment 写入：
   - UI：`/org/assignments?as_of=...` 表单支持 `base_salary/allocated_fte`
   - Internal API：`/org/api/assignments?as_of=...` 支持 `base_salary/allocated_fte`
2. [ ] 按 060-DS1 为 E01~E10 写入 `base_salary/allocated_fte`，并记录 E04 的 `allocated_fte=0.5` 证据。
3. [ ] 若环境中仍缺失（不应发生）：在 §11 记录为 `CONTRACT_MISSING/CONTRACT_DRIFT`，并明确这是 TP-060-07/08 的阻塞点（不得用 SQL 直改绕过 One Door）；同时记录“当前可见入口”现状（页面/路由/表单字段缺失的证据）。

### 8.5 Position/Assignment 交叉不变量（fail-closed，稳定错误码）

> 目标：把 `DEV-PLAN-030` 的 M3/M6a 不变量纳入 TP-060 套件，避免“岗位停用/容量”变成 UI 层分叉条件。

1. [ ] 禁止任职写入 disabled position（负例，Internal API）
   - 先在 `/org/positions?as_of=...` 创建或选择一个职位，并在更晚的 `effective_date`（例如 `2026-01-15`）将其更新为 `lifecycle_status=disabled`
   - 调用：`POST /org/api/assignments?as_of=2026-01-15`
     - body：`{"effective_date":"2026-01-15","person_uuid":"<E01 person_uuid>","position_id":"<disabled_position_id>","base_salary":"0","allocated_fte":"1.0"}`
   - 断言：422 且 `code=STAFFING_POSITION_DISABLED_AS_OF`
2. [ ] 容量裁决：`allocated_fte > capacity_fte` 必须 fail-closed（负例，Internal API）
   - 选择一个职位（建议：E04 的职位）并设置 `capacity_fte=0.50`
   - 调用：`POST /org/api/assignments?as_of=2026-01-15`
     - body：`{"effective_date":"2026-01-15","person_uuid":"<E04 person_uuid>","position_id":"<capacity_position_id>","base_salary":"0","allocated_fte":"1.0"}`
   - 断言：422 且 `code=STAFFING_POSITION_CAPACITY_EXCEEDED`
3. [ ] 降容裁决：Position 降容导致超编必须 fail-closed（负例，Internal API）
   - 前置：确保该 position 已存在一条 active assignment，且 `allocated_fte=0.50`
   - 调用：`POST /org/api/positions?as_of=2026-01-15`
     - body：`{"effective_date":"2026-01-15","position_id":"<capacity_position_id>","capacity_fte":"0.25"}`
   - 断言：422 且 `code=STAFFING_POSITION_CAPACITY_EXCEEDED`
4. [ ] Position disable 与 active assignment 冲突必须 fail-closed（负例，Internal API）
   - 前置：该 position 已存在 active assignment
   - 调用：`POST /org/api/positions?as_of=2026-01-15`
     - body：`{"effective_date":"2026-01-15","position_id":"<capacity_position_id>","lifecycle_status":"disabled"}`
   - 断言：422 且 `code=STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF`

### 8.6 汇报线（reports_to）不变量（M4a：无环 + forward-only）

> 目标：覆盖 `DEV-PLAN-030` 的 M4a：可编辑 + 禁止自指/禁止成环 + forward-only。

1. [ ] 正例：在 `/org/positions?as_of=2026-01-15` 将一个岗位的 `reports_to_position_id` 设置为另一个 active 岗位
   - 断言：列表中 `reports_to_position_id` 列可见（或等效展示）
2. [ ] 负例 A（自指）：`POST /org/api/positions?as_of=2026-01-15`
   - body：`{"effective_date":"2026-01-15","position_id":"<manager_position_id>","reports_to_position_id":"<manager_position_id>"}`
   - 断言：422 且 `code=STAFFING_POSITION_REPORTS_TO_SELF`
3. [ ] 负例 B（成环）：`POST /org/api/positions?as_of=2026-01-15`
   - body：`{"effective_date":"2026-01-15","position_id":"<manager_position_id>","reports_to_position_id":"<reportee_position_id>"}`
   - 断言：422 且 `code=STAFFING_POSITION_REPORTS_TO_CYCLE`
4. [ ] 负例 C（forward-only）：在已存在 `effective_date=2026-01-15` 的 reports_to 更新后，尝试 retro 写入
   - 调用：`POST /org/api/positions?as_of=2026-01-10`
   - body：`{"effective_date":"2026-01-10","position_id":"<reportee_position_id>","reports_to_position_id":"<manager_position_id>"}`
   - 断言：422 且 `code=STAFFING_INVALID_ARGUMENT`

## 9. 依赖与里程碑（Dependencies & Milestones）

- 依赖（必须先满足）：
  - [ ] TP-060-02 已完成并能提供 10 个 positions（`docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`）。
- 里程碑（本子计划内完成）：
  1. [X] 建立/复用 10 个 Person，并记录 `pernr -> person_uuid`。
  2. [X] 完成 `persons:by-pernr` 的正例/负例断言（含前导 0 同值）。
  3. [X] 完成 10 人 primary assignment 绑定并留证（timeline 可见）。
  4. [X] 写入并留证 `base_salary/allocated_fte`（至少包含 E04 的 `allocated_fte=0.5`）。
  5. [X] 自动化用例覆盖并纳入 `make e2e`（见 `e2e/tests/tp060-03-person-and-assignments.spec.js`）。

## 10. 验收证据（最小）

- `/person/persons?as_of=2026-01-01`：10 人列表证据（含 `pernr/person_uuid`）。
- `persons:by-pernr`：`00000103` 与 `103` 命中同一 `person_uuid` 的证据；以及 1 条 `PERSON_PERNR_INVALID` 的 400 负例证据。
- `/org/assignments?...`：10 人 Timeline 证据（含 `assignment_id/position_id/effective_date/status`），并证明 UI 未展示 `end_date`。
- `base_salary/allocated_fte` 写入证据（至少包含 E04 的 `allocated_fte=0.5`）。
- 交叉不变量证据（Internal API 响应即可）：
  - `STAFFING_POSITION_DISABLED_AS_OF`
  - `STAFFING_POSITION_CAPACITY_EXCEEDED`（含“写入超编”与“降容超编”两条）
  - `STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF`
- 汇报线证据（Internal API 响应即可）：
  - `STAFFING_POSITION_REPORTS_TO_SELF`
  - `STAFFING_POSITION_REPORTS_TO_CYCLE`
  - `STAFFING_INVALID_ARGUMENT`（forward-only 负例）
- 自动化证据：`make e2e` 通过（用例：`e2e/tests/tp060-03-person-and-assignments.spec.js`；截图输出位置见 `docs/dev-records/dev-plan-063-execution-log.md`）。

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

### 10.1 执行记录（Readiness/可复现记录）

> 说明：以 `docs/dev-records/dev-plan-063-execution-log.md` 为准；本节仅列出关键门禁入口，便于快速复跑。

- [X] E2E：`make e2e`（包含 `e2e/tests/tp060-03-person-and-assignments.spec.js`）
- [X] Go 测试：`make test`
- [X] Go lint：`make check lint`
- [X] 文档门禁（本文变更后）：`make check doc`（2026-01-11，PASS）

## 11. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
