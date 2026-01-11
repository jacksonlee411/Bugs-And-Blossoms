# DEV-PLAN-067：全链路业务测试子计划 TP-060-07——薪酬 041-043（主流程 + 工资条 + 社保）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（薪酬输入：base_salary/allocated_fte）。

## 1. 背景与上下文（Context）

TP-060 的薪酬主链路需要把 `DEV-PLAN-041/042/043` 三个切片串成“可见可操作”的端到端闭环（对齐 `docs/dev-plans/060-business-e2e-test-suite.md` 的覆盖矩阵）。本子计划（TP-060-07）聚焦验证：
- **主流程**：pay period → payroll run → calculate → finalize（定稿后只读）；
- **工资条**：run 下 payslips 列表/详情可见，且汇总可由明细解释；
- **社保**：单城市政策（as-of 命中）+ 扣缴情形（clamp + 舍入合同）可判定、可留证；
- **安全**：Authz 可拒绝（403），Tenancy/RLS fail-closed（缺租户上下文不得读/写）。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 目标（Done 定义）

- [ ] **pay period/run**：可创建、可计算、可定稿；定稿后只读（再次 calculate/finalize 必须失败，且能定位到稳定错误码或稳定失败原因）。
- [ ] **payslips**：run 下列表/详情可见；至少可对账两条可判定断言：
  - E04：`EARNING_BASE_SALARY` 明细金额为 `10,000.00`（`base_salary=20,000.00 × allocated_fte=0.5`）。
  - E02/E03：社保险种行（PENSION/MEDICAL）金额满足 clamp + 舍入合同（见 §6.6）。
- [ ] **社保政策**：`/org/payroll-social-insurance-policies?as_of=2026-01-01` 可配置并 as-of 命中；6 个险种均存在可用版本（即使费率为 0）。
- [ ] **安全与隔离**：
  - Authz：只读角色对任一写入（policy/period/run/calc/finalize）必须 403。
  - fail-closed：缺 tenant context 访问 payroll 页面/API 不得泄漏数据（允许 404/403/500/重定向，但不得返回数据或允许写入生效）。

### 2.2 非目标

- 不在本子计划验证 IIT/回溯/净额保证（由 TP-060-08 承接）。

## 2.3 工具链与门禁（SSOT 引用）

> 目的：避免在子计划里复制工具链细节导致 drift；本文仅声明“本子计划执行/修复时可能命中的门禁入口”，具体命令以 `AGENTS.md`/`Makefile`/CI workflow 为准。

- 触发器清单（按需勾选本计划命中的项）：
  - [ ] E2E（`make e2e`；若补齐 TP-060-07 自动化用例）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`；若权限口径 drift）
  - [ ] 路由治理（`make check routing`；若新增/调整 payroll 路由或 allowlist）
  - [X] 文档（本文件变更：`make check doc`）
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`；仅当为修复 drift 而改 Go）
  - [ ] DB/迁移（按模块 `make staffing plan && make staffing lint && make staffing migrate up`；仅当为修复 drift 而改 DB）

- SSOT 链接：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`

## 3. 契约引用（SSOT）

- 路线图：`docs/dev-plans/039-payroll-social-insurance-implementation-roadmap.md`
- P0-1：`docs/dev-plans/041-payroll-p0-slice-pay-period-and-payroll-run.md`
- P0-2：`docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`
- P0-3：`docs/dev-plans/043-payroll-p0-slice-social-insurance-policy-and-calculation.md`
- 测试套件与数据集：`docs/dev-plans/060-business-e2e-test-suite.md`
- 薪酬输入（assignment）：`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`
- Authz/RLS：`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`

## 4. 前置条件与数据准备（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- 运行态硬要求：`AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`（对齐 `docs/dev-plans/060-business-e2e-test-suite.md` §4.3）。
- 固定测试基准日（用于证据一致性与社保 policy as-of）：`AS_OF_BASE=2026-01-01`

> 说明（`as_of` 漂移规避）：`DEV-PLAN-041/042/043` 的 payroll UI 路由不强制要求 `as_of`；但若你所在环境对 payroll 路由也引入了 `as_of` 门禁/重定向，本子计划所有 payroll URL 统一追加 `?as_of=AS_OF_BASE`（并在证据里记录“实际访问 URL”）。

### 4.1 账号与权限

- 必备：Tenant Admin（可执行 payroll 写入动作：policy/period/run/calc/finalize）。
- 可选（用于 403 负例）：Tenant Viewer（`role_slug=tenant-viewer`；若无法分配按 TP-060-01 口径记录为 `CONTRACT_MISSING/ENV_DRIFT`）。

### 4.2 人员/任职（TP-060-03 输出，必须）

10 人 assignments 已具备：
- `base_salary`（CNY，月薪语义为 FTE=1.0；对齐 `DEV-PLAN-042`）
- `allocated_fte`（包含 E04=0.5）

> 阻塞判定：若无法写入/读取 `base_salary/allocated_fte`（缺 UI 字段或 internal API），本子计划直接判定为阻塞并记录（见 `docs/dev-plans/063-test-tp060-03-person-and-assignments.md` §8.4）。

### 4.3 Pay Group / Pay Period（P0 冻结）

- `pay_group`：`monthly`（小写）
- 本子计划主验证周期（用于后续 TP-060-08 复用）：
  - 2026-01：`[2026-01-01, 2026-02-01)`（自然月，闭开区间）

### 4.4 社保政策（单城市，P0）

- `city_code`：`CN-310000`（示例：上海；必须全大写 + trim）
- `hukou_type`：`default`（P0 冻结）
- 6 个险种均需存在可用版本：`PENSION/MEDICAL/UNEMPLOYMENT/INJURY/MATERNITY/HOUSING_FUND`
  - 为简化对账，不关心的险种可设为 0 费率，但仍需满足枚举/Schema 约束。

建议用于“可判定断言”的测试政策值（对齐 `docs/dev-plans/043-payroll-p0-slice-social-insurance-policy-and-calculation.md`）：
- 通用：`base_floor=5000.00`、`base_ceiling=30001.00`
- PENSION：`employer_rate=0.160000`、`employee_rate=0.080000`、`rounding_rule=HALF_UP`、`precision=2`
- MEDICAL：`employer_rate=0.095530`、`employee_rate=0.020070`、`rounding_rule=CEIL`、`precision=2`

### 4.5 数据保留（强制）

本子计划创建的社保政策/period/run/payslips，以及 `finalized` 状态的定稿结果必须保留（后续 TP-060-08 依赖 2026-01 pay period 的已定稿结果；SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

### 4.6 可重复执行口径（Idempotency / Re-run）

- 若环境已存在 2026-01 的 pay period/run/policies：优先“校验 + 记录 ID + 复用”，仅在缺失时补齐创建。
- 若已存在 `finalized` run：本子计划的“创建/计算/定稿”步骤可改为“只读校验 + 只读负例（再次操作失败）”。
- 若 UI 反馈不包含稳定错误码：优先用 internal API（若存在）或对同一路由加 `Accept: application/json` 获取稳定错误 envelope；仍无法定位则按 `CONTRACT_DRIFT` 记录。

## 5. 页面/接口契约（用于断言；SSOT 摘要）

> 本节只摘录 TP-060-07 执行所需的最小 URL/Method/期望；细节以 `DEV-PLAN-041/042/043` 为准。

### 5.1 UI（HTML + HTMX）

- Pay periods：
  - `GET /org/payroll-periods`
  - `POST /org/payroll-periods`
- Payroll runs：
  - `GET /org/payroll-runs`
  - `POST /org/payroll-runs`
  - `GET /org/payroll-runs/{run_id}`
  - `POST /org/payroll-runs/{run_id}/calculate`
  - `POST /org/payroll-runs/{run_id}/finalize`
- Payslips（按 run 入口）：
  - `GET /org/payroll-runs/{run_id}/payslips`
  - `GET /org/payroll-runs/{run_id}/payslips/{payslip_id}`
- 社保政策（单城市）：
  - `GET /org/payroll-social-insurance-policies?as_of=YYYY-MM-DD`（`as_of` 可选，但本计划固定使用 `2026-01-01` 以避免“默认今日”漂移）
  - `POST /org/payroll-social-insurance-policies`

### 5.2 Internal API（用于排障/稳定断言，若环境已实现）

- `GET /org/api/payroll-periods`
- `GET /org/api/payroll-runs?pay_period_id=...`
- `GET /org/api/payslips?run_id=...`
- `GET /org/api/payslips/{payslip_id}`
- `GET /org/api/payroll-social-insurance-policies?as_of=YYYY-MM-DD`

### 5.3 关键稳定错误码（本子计划会用到）

> 以实现返回为准；若与下表冲突，先按 `CONTRACT_DRIFT` 记录并回到对应 dev-plan 处理。

- `STAFFING_IDEMPOTENCY_REUSED`：幂等冲突（同 event_id 但载荷不一致）
- `STAFFING_PAYROLL_MISSING_BASE_SALARY`：缺少算薪输入（`DEV-PLAN-042`）
- `STAFFING_PAYROLL_RUN_ALREADY_FINALIZED`：周期内已存在 finalized run / 已定稿只读（`DEV-PLAN-041`）
- `STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD`：period 内存在社保政策变更（P0 必须 fail-closed；`DEV-PLAN-043`）

### 5.4 ID 记录口径（避免“执行期猜测”）

本子计划需要记录的关键 ID：
- `PAY_PERIOD_ID_2026_01`
- `RUN_ID_2026_01`
- `E02_PAYSLIP_ID` / `E03_PAYSLIP_ID` / `E04_PAYSLIP_ID`

获取顺序（优先级从高到低）：
1) **internal API（若可用）**：通过 `GET /org/api/payroll-periods`、`GET /org/api/payroll-runs?pay_period_id=...`、`GET /org/api/payslips?run_id=...` 获取。
2) **UI 链接/URL（必须可用）**：
   - `RUN_ID_2026_01`：从 `/org/payroll-runs/{run_id}` 的 URL path 提取。
   - `*_PAYSLIP_ID`：从 `/org/payroll-runs/{run_id}/payslips/{payslip_id}` 的 URL path 提取。
   - `PAY_PERIOD_ID_2026_01`：若 UI 列表不直接展示 id，记录“创建 run 表单中选中的 pay period”的 value（通常为 uuid），或用 internal API 补齐。

## 6. 测试步骤（执行时勾选）

> 执行记录要求：每步至少记录 `Host/AUTHZ_MODE/RLS_ENFORCE`；涉及金额断言时必须保留“详情页截图或等效文本证据（含 item_code/amount/险种行）”。

### 6.1 运行态确认（硬要求）

1. [ ] 确认使用租户 Host：`http://t-060.localhost:8080`（禁止用 `127.0.0.1` 作为常规访问 Host；负向用例除外）。
2. [ ] 记录运行态：`AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`。
3. [ ] 记录 `AS_OF_BASE=2026-01-01`，并在证据中注明 payroll URL 是否追加了 `?as_of=AS_OF_BASE`（见 §4 顶部说明）。

### 6.2 配置社保政策（必须）

1. [ ] 打开：`/org/payroll-social-insurance-policies?as_of=AS_OF_BASE`
2. [ ] 确保 `city_code=CN-310000` 且 6 个险种均存在“as_of 命中”的版本（缺失则补齐创建；已有则记录并复用）。
3. [ ] 对 PENSION/MEDICAL 使用 §4.4 的建议测试值；其余险种可设 0 费率（仍需满足必填字段与枚举）。
4. [ ] 断言：保存后 `303` 跳回列表；刷新后可见各险种的 `effective_date` 与关键字段（费率/上下限/舍入）。

### 6.3 创建/复用 pay period（必须）

1. [ ] 打开：`/org/payroll-periods`
2. [ ] 确保存在 2026-01 的 period（缺失则创建；已有则记录并复用）：
   - `pay_group=monthly`
   - `start_date=2026-01-01`
   - `end_date_exclusive=2026-02-01`
3. [ ] 记录：`PAY_PERIOD_ID_2026_01`（从列表或 internal API 获取）。

### 6.4 创建/复用 payroll run（必须）

1. [ ] 打开：`/org/payroll-runs`
2. [ ] 选择 `PAY_PERIOD_ID_2026_01` 创建 run（若已存在可复用其中一个非 finalized run；若仅存在 finalized run，跳到 §6.8 做只读校验）。
3. [ ] 记录：`RUN_ID_2026_01`。

### 6.5 计算 run（必须）

0. [ ] **断言前置（必须，避免“数据差异被误判为算法错误”）**：在开始 calculate 前，先在 assignments 页面校验 3 人的薪酬输入与生效日：
   - 打开 `/org/assignments?as_of=AS_OF_BASE&pernr=102`（E02），断言：
     - primary assignment 在 `AS_OF_BASE` 下可见
     - `effective_date=2026-01-01`（覆盖整月）
     - `base_salary=80,000.00`、`allocated_fte=1.0`、`currency=CNY`
   - 打开 `/org/assignments?as_of=AS_OF_BASE&pernr=00000103`（E03），断言：
     - primary assignment 在 `AS_OF_BASE` 下可见
     - `effective_date=2026-01-01`
     - `base_salary=3,000.00`、`allocated_fte=1.0`、`currency=CNY`
   - 打开 `/org/assignments?as_of=AS_OF_BASE&pernr=104`（E04），断言：
     - primary assignment 在 `AS_OF_BASE` 下可见
     - `effective_date=2026-01-01`
     - `base_salary=20,000.00`、`allocated_fte=0.5`、`currency=CNY`
   - 若上述任一断言不成立：本子计划按 `ENV_DRIFT/CONTRACT_DRIFT` 记录并停止后续金额断言步骤（否则会得到不可判定结果）。
1. [ ] 在 run 详情页执行 calculate（或直接 `POST /org/payroll-runs/{run_id}/calculate`）。
2. [ ] 断言：
   - run 状态进入 `calculated`（或 UI 等效提示/字段）。
   - `calc_started_at/calc_finished_at` 可见（至少 `calc_finished_at` 非空）。
3. [ ] 打开 payslips 列表：`/org/payroll-runs/{RUN_ID_2026_01}/payslips`
4. [ ] 断言：列表存在至少 1 条 payslip；并能进入详情页。

### 6.6 工资条断言（必须，可判定）

> 建议用 `pernr` 过滤快速定位（若 UI 支持）：`/org/payroll-runs/{run_id}/payslips?pernr=...`。

1. [ ] E04（FTE 0.5）：
   - 打开 E04 的 payslip 详情（记录 `E04_PAYSLIP_ID`）
   - 断言：存在 `item_code=EARNING_BASE_SALARY` 的明细行，金额为 `10,000.00`
2. [ ] E03（社保基数下限 clamp）：
   - 打开 E03 的 payslip 详情（记录 `E03_PAYSLIP_ID`）
   - 断言（PENSION，HALF_UP，2 位）：employee=`400.00`、employer=`800.00`
   - 断言（MEDICAL，CEIL，2 位）：employee=`100.35`、employer=`477.65`
3. [ ] E02（社保基数上限 clamp）：
   - 打开 E02 的 payslip 详情（记录 `E02_PAYSLIP_ID`）
   - 断言（PENSION，HALF_UP，2 位）：employee=`2,400.08`、employer=`4,800.16`
   - 断言（MEDICAL，CEIL，2 位）：employee=`602.13`、employer=`2,866.00`
4. [ ] （建议）对任一 payslip 抽样核对：社保分项区块可见（至少展示 `insurance_type/base_amount/employee_amount/employer_amount`），并能定位到 `city_code=CN-310000`（或 UI 等效字段）。

### 6.7 定稿 run（必须）

1. [ ] 在 run 详情页执行 finalize（或 `POST /org/payroll-runs/{run_id}/finalize`）。
2. [ ] 断言：
   - run 状态进入 `finalized`（终态，只读）
   - pay period 状态进入 `closed`（对齐 `DEV-PLAN-041`）

### 6.8 finalized 只读负例（必须）

1. [ ] 对同一 `RUN_ID_2026_01` 再次执行 calculate
   - 断言：必须失败（期望 409 或等效），且能定位稳定错误码（满足其一即可）：
     - `code=STAFFING_PAYROLL_RUN_ALREADY_FINALIZED`；或
     - `code` 以 `STAFFING_PAYROLL_RUN_` 开头，语义为“状态机冲突/非法跃迁/终态只读”，且响应中能判定当前 `run_state=finalized`（或 UI 明确提示“已定稿，只读”）。
   - 若 UI 页面不返回错误码：对同一路由用 `Accept: application/json` 重试以获取 JSON error envelope（示例）：
     ```bash
     curl -i -sS \
       -H 'Accept: application/json' \
       -X POST "http://t-060.localhost:8080/org/payroll-runs/<RUN_ID_2026_01>/calculate"
     ```
2. [ ] 对同一 `RUN_ID_2026_01` 再次执行 finalize
   - 断言：必须失败（同上）。

### 6.9 安全与 fail-closed 负例（必须）

1. [ ] Authz（403）：以只读用户对以下任一写入动作执行一次即可：
   - `POST /org/payroll-social-insurance-policies`
   - `POST /org/payroll-periods`
   - `POST /org/payroll-runs/{run_id}/calculate`
   - `POST /org/payroll-runs/{run_id}/finalize`
2. [ ] fail-closed（缺 tenant context）：用非租户 host 访问（示例）：`http://127.0.0.1:8080/org/payroll-runs`
   - 断言：不得展示任何租户数据（允许 404/403/500/重定向，但不得返回数据或允许写入生效）。
3. [ ] fail-closed（缺 tenant context 的“写入不得生效”，必须，可判定）：
   - 先在租户 host 的 pay periods 页面确认：不存在 `2099-01` 的 period（pay_group=monthly）。
   - 在非租户 host 发送创建 period 请求（示例用 curl；若你的环境 payroll 路由要求 `as_of`，则把 URL 改为 `.../org/payroll-periods?as_of=AS_OF_BASE`）：
     ```bash
     curl -i -sS \
       -X POST "http://127.0.0.1:8080/org/payroll-periods" \
       -H "content-type: application/x-www-form-urlencoded" \
       --data "pay_group=monthly&start_date=2099-01-01&end_date_exclusive=2099-02-01"
     ```
   - 断言 A：该请求不得返回 303/200/201（允许 4xx/5xx/重定向到 login/tenant_not_found 等稳定失败）。
   - 断言 B：回到租户 host 的 `/org/payroll-periods` 刷新确认：仍不存在 `2099-01` period（写入未生效）。
   - 若写入竟然生效：立即记录为 P0（`BUG`），停止后续 TP-060-*（因为租户注入/隔离已失效）。

### 6.10 （可选扩展）policy 期内变更必须 fail-closed（`DEV-PLAN-043`）

> 说明：为避免影响 `RUN_ID_2026_01` 的可复用定稿结果，本扩展建议使用**另一个** pay period（例如 2026-03）单独验证。

1. [ ] 创建/复用一个额外 pay period：2026-03 `[2026-03-01, 2026-04-01)`，并创建对应 run（记录 `PAY_PERIOD_ID_2026_03`、`RUN_ID_2026_03`）。
2. [ ] 在社保政策页追加一个版本（任一险种即可，例如 MEDICAL）：`effective_date=2026-03-15`（落在 period 内）。
3. [ ] 对 `RUN_ID_2026_03` 执行 calculate：
   - 断言：必须失败，稳定错误码为 `STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD`。

## 7. 验收证据（最小）

- 环境记录：`Host/AUTHZ_MODE/RLS_ENFORCE`。
- 社保政策页证据：`as_of=AS_OF_BASE` 下 6 个险种的版本可见（至少包含 PENSION/MEDICAL 的费率、上下限、舍入规则）。
- pay period 列表证据：2026-01 的 `pay_group/period` 可见，且记录 `PAY_PERIOD_ID_2026_01`。
- payroll run 详情证据：`RUN_ID_2026_01` 的 `run_state` 与时间戳（calculated/finalized）可见。
- payslips 列表/详情证据：
  - E04：`EARNING_BASE_SALARY=10,000.00`
  - E03：PENSION/MEDICAL 金额断言（下限 clamp）
  - E02：PENSION/MEDICAL 金额断言（上限 clamp）
- finalized 只读证据：同一 `RUN_ID_2026_01` 的重复 calculate/finalize 失败（含稳定错误码或等效稳定失败原因）。
- （可选扩展）policy 期内变更 fail-closed 证据：`STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD`。

## 8. 执行记录（Readiness/可复现记录）

> 说明：此处只记录“本次执行实际跑了什么、结果如何”；命令入口以 `AGENTS.md` 为准。执行时把 `[ ]` 改为 `[X]` 并补齐时间戳与结果摘要。

- [ ] 文档门禁：`make check doc`
- [ ] （可选）E2E：`make e2e`（若补齐 TP-060-07 自动化用例）
- [ ] （可选）Authz：`make authz-pack && make authz-test && make authz-lint`
- [ ] （可选）路由治理：`make check routing`

## 9. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
