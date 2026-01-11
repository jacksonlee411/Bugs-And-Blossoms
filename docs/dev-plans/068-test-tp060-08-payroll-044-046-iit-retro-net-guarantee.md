# DEV-PLAN-068：全链路业务测试子计划 TP-060-08——薪酬 044-046（个税累计预扣 + 回溯 + 税后发放）

**状态**: 草拟中（2026-01-11 08:41 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/067-test-tp060-07-payroll-041-043-run-payslip-si.md`（已定稿 `PP-2026-01`，并创建 `PP-2026-02` 的 run 作为承载）。

## 1. 背景与上下文（Context）

本子计划（TP-060-08）覆盖薪酬路线图的后半段，用于把 `DEV-PLAN-044/045/046` 串成“用户可见、可操作、可判定”的端到端验收闭环（对齐 `docs/dev-plans/060-business-e2e-test-suite.md` 的覆盖矩阵与证据要求）。

覆盖范围（契约 SSOT）：
- `DEV-PLAN-044`：个税累计预扣（O(1) balances）与专项附加扣除（SAD）输入载体；
- `DEV-PLAN-045`：回溯计算（定稿后历史生效变更 → 生成 recalc request → 结转差额到后续周期）；
- `DEV-PLAN-046`：税后发放（仅 IIT）的净额保证（分位精确）。

执行的核心前提（不满足则本计划直接阻塞并记录）：
- 运行态硬要求：`AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`（对齐 `docs/dev-plans/060-business-e2e-test-suite.md` §4.3）。
- 已存在可复用的 060-DS1 人员与任职输入（E05/E06/E07 相关字段），并已完成 TP-060-07 的 `PP-2026-01` 定稿（否则本计划无法验证 “finalized 后回溯/余额推进/只读裁决”）。
- Host/tenant 解析必须严格：常规访问使用 `t-060.localhost`；负例才使用 `127.0.0.1`（不得用 `127.0.0.1` 冒充租户 host）。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 目标（Done 定义）

- [ ] IIT（044）：
  - [ ] 工资条展示 IIT 明细与税后实发（用户可见）。
  - [ ] balances 可通过 internal API `GET /org/api/payroll-balances` O(1) 读取，并满足“累计减除费用月数”口径（可判定断言，见 §9）。
  - [ ] （可选但建议）可通过 internal API 录入 SAD（月度合计）并满足幂等/只读裁决（见 §9.3）。
- [ ] 回溯（045）：
  - [ ] 已定稿后提交更早 effective_date 的任职/定薪变更 → **同事务**生成 recalc request（UI 可见）。
  - [ ] apply 后差额结转到后续周期（`PP-2026-02`），且可追溯 origin；不得覆盖已定稿工资条（可判定断言，见 §9）。
- [ ] 净额保证（046，仅 IIT）：
  - [ ] 在 payslip 详情页可录入净额保证项（用户可操作）。
  - [ ] 计算后 `net_after_iit == target_net`（精确到分），且工资条展示 `gross_amount/iit_delta/net_after_iit` 的解释字段（可判定断言，见 §9）。

### 2.2 非目标

- 不覆盖多 tax_entity/多币种/多城市扩展（P0 边界见 `DEV-PLAN-040/039`）。
- 不覆盖跨税年回溯结转（P0 明确拒绝；见 `docs/dev-plans/045-payroll-p0-slice-retroactive-accounting.md`）。
- 不在本计划引入“绕过 UI/Kernel 的 SQL 直改”来伪造通过（One Door / No Legacy）。

## 2.3 工具链与门禁（SSOT 引用）

> 目的：避免在子计划里复制工具链细节导致 drift；本文只声明“本子计划执行/修复时可能命中的门禁入口”，具体命令以 `AGENTS.md`/`Makefile`/CI workflow 为准。

- 触发器清单（按需勾选本计划命中的项）：
  - [ ] E2E（`make e2e`；若补齐 TP-060-08 自动化用例）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`；若权限口径 drift）
  - [ ] 路由治理（`make check routing`；若新增/调整 internal API 路由或 allowlist）
  - [X] 文档（本文件变更：`make check doc`）
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`；仅当为修复 drift 而改 Go）
  - [ ] DB/迁移（按模块 `make staffing plan && make staffing lint && make staffing migrate up`；仅当为修复 drift 而改 DB）

- SSOT 链接：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`
  - 路由 allowlist（事实源）：`config/routing/allowlist.yaml`

## 3. 契约引用（SSOT）

- IIT：`docs/dev-plans/044-payroll-p0-slice-iit-cumulative-withholding-and-balances.md`
- 回溯：`docs/dev-plans/045-payroll-p0-slice-retroactive-accounting.md`
- 净额保证：`docs/dev-plans/046-payroll-p0-slice-net-guaranteed-iit-tax-gross-up.md`
- 测试套件与数据集：`docs/dev-plans/060-business-e2e-test-suite.md`
- 薪酬主流程载体（已定稿前置）：`docs/dev-plans/041-payroll-p0-slice-pay-period-and-payroll-run.md`、`docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`
- 任职写入口（回溯触发来源）：`docs/dev-plans/031-greenfield-assignment-job-data.md`、`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`

## 4. 架构与关键决策（面向测试的结论）

### 4.1 手工执行 vs 自动化回归（两条链路的职责分工）

- **手工执行（060-DS1 固定数据集）**：在 `T060 / t-060.localhost` 下复用 `PP-2026-01/02` 与 E05/E06/E07 的差异样例，形成可复现证据；数据必须保留供后续回归/排障复用。
- **自动化回归（隔离数据）**：若补齐 E2E，用例建议命名为 `e2e/tests/tp060-08-payroll-044-046.spec.js`，使用 `runID` 创建独立 tenant 与数据，保证可重复跑；**不替代** 060-DS1 的固定证据。

### 4.2 关键设计决策（ADR 摘要）

- 决策 A：balances 以 internal API `GET /org/api/payroll-balances` 做“可判定、可自动化”的 O(1) 断言（对齐 `DEV-PLAN-044` 的接口契约）。
- 决策 B：回溯以 “请求 → apply → 结转差额到后续周期” 的可视化 UI 证据为主，且 P0 明确不结转 IIT 差额（对齐 `DEV-PLAN-045`）。
- 决策 C：净额保证项写入复用 “工资条详情页” 的写入口（HTMX + Kernel One Door），并以 “分位精确相等” 为可判定断言（对齐 `DEV-PLAN-046`）。

## 5. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- Pay periods：
  - `PP-2026-01`：已 finalize（来自 TP-060-07）
  - `PP-2026-02`：存在 run（draft/failed）用于承载回溯结转、净额保证与继续累计
- 人员样例：
  - E06：`effective_date=2026-01-15` 入职（用于 first_tax_month/standard deduction 月数口径）
  - E07：录入净额保证工资项（`target_net=20,000.00`）
  - E05：用于回溯触发（定稿后提交更早生效变更）

### 5.1 数据保留（强制）

- 本子计划产生的 IIT 输入（SAD）、net-guarantee 项、recalc requests 与结转差额结果必须保留，用于后续回归与可排障证据（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

### 5.2 记录表（执行前填写）

> 说明：避免“执行期猜测 ID”。本计划的断言需要显式记录各类 `id/uuid`；建议将该表补齐后复用到 `docs/dev-records/` 的执行日志中。

| 名称 | 值 | 说明 |
| --- | --- | --- |
| `HOST` | `t-060.localhost` | 固定租户 host |
| `AS_OF_BASE` | `2026-02-01` | 若你的 payroll UI/路由也要求 `as_of`，统一追加 `?as_of=AS_OF_BASE` 并在证据中记录“实际访问 URL” |
| `AUTHZ_MODE` |  | 必须 `enforce` |
| `RLS_ENFORCE` |  | 必须 `enforce` |
| `PAY_PERIOD_ID_2026_01` |  | 来自 TP-060-07 |
| `RUN_ID_2026_01` |  | 来自 TP-060-07（finalized） |
| `PAY_PERIOD_ID_2026_02` |  | 若缺失需创建（TP-060-07 或本计划） |
| `RUN_ID_2026_02` |  | 必须为 `draft/failed`（可计算） |
| `E05_PERSON_UUID` |  | 回溯触发样例 |
| `E06_PERSON_UUID` |  | balances 断言样例（入职日 2026-01-15） |
| `E07_PERSON_UUID` |  | 净额保证样例 |
| `E07_PAYSLIP_ID_2026_02` |  | 用于录入净额保证项（从 payslips 列表定位） |

### 5.3 可重复执行口径（Idempotency / Re-run）

> 目的：同一租户/同一环境重复跑本子计划时，避免“重复创建导致污染”或“断言依赖执行期猜测”。

- 若 `PP-2026-01/PP-2026-02`、runs、payslips 已存在：优先“校验 + 记录 ID + 复用”，仅在缺失时补齐创建。
- 回溯请求（recalc requests）若已存在且已 apply：
  - 建议再次对 E05 提交一次 payroll-impactful 的 assignment 变更（例如将 `base_salary` 再上调一次），生成新的 pending 请求再继续；不得通过 SQL 清理旧请求。
- 净额保证项写入以 `request_id` 做幂等：重复提交同一 `request_id` 必须幂等成功；若需再次验证，请使用新的 `request_id`（建议使用新的 UUIDv4）。

## 6. 页面/接口契约（用于断言；SSOT 摘要）

> 本节只列出 TP-060-08 执行会用到的最小路径；细节以 `DEV-PLAN-044/045/046` 为准。

### 6.1 IIT balances / SAD（044）

- Internal API（route_class=`internal_api`）：
  - `GET /org/api/payroll-balances?person_uuid=<uuid>&tax_year=<yyyy>`
  - `POST /org/api/payroll-iit-special-additional-deductions`（JSON）

### 6.2 回溯请求（045）

- UI：
  - `GET /org/payroll-recalc-requests`
  - `GET /org/payroll-recalc-requests/{recalc_request_id}`
  - `POST /org/payroll-recalc-requests/{recalc_request_id}/apply`（form：`target_run_id`）
- Internal API（route_class=`internal_api`，可选，用于排障/稳定断言）：
  - `GET /org/api/payroll-recalc-requests`
  - `GET /org/api/payroll-recalc-requests/{recalc_request_id}`
  - `POST /org/api/payroll-recalc-requests/{recalc_request_id}:apply`（JSON：`{"target_run_id":"uuid"}`）

### 6.3 净额保证项（046）

- UI：
  - `GET /org/payroll-runs/{run_id}/payslips/{payslip_id}`
  - `POST /org/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items`（form：`item_code/target_net/request_id`）
- Internal API（route_class=`internal_api`，可选）：
  - `POST /org/api/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items`

## 7. 核心验收逻辑与失败路径（面向测试的结论）

### 7.1 balances 推进（044）

- balances 只在 `FINALIZE` 同事务推进；finalized 后只读。
- `tax_month` 必须单调递增（P0：自然月 pay period）；不满足必须失败并返回稳定错误码（见 `DEV-PLAN-044` §6.6）。

### 7.2 回溯链路（045）

- 回溯请求必须在 “assignment 写入成功” 的同事务内创建（命中已定稿 period 时）。
- apply 必须生成可审计差额（adjustments），并结转到后续周期；不得静默吞掉请求/静默失败（UI 必须可见稳定错误）。
- P0 冻结：IIT 差额不结转；IIT 在 Target period 由累计预扣引擎统一计算（见 `DEV-PLAN-045` §6.3.4-§6.3.6）。

### 7.3 净额保证（046）

- 计算必须确定性，分位精确；不收敛/合同漂移必须失败并返回稳定错误码（见 `DEV-PLAN-046` §6.6）。
- 输入写入口必须幂等，且 finalized run 必须只读裁决（见 `DEV-PLAN-046` §6.6）。

## 8. 安全与鉴权（Security & Authz）

- Authz（403 可拒绝）：
  - balances/SAD：对齐 `DEV-PLAN-044` §7（复用对象 `staffing.payroll-runs` 的 `read/admin`）。
  - recalc requests：对齐 `DEV-PLAN-045` §7（对象 `staffing.payroll-recalc-requests` 的 `read/admin`）。
  - 净额保证项：对齐 `DEV-PLAN-046` §7（对象 `staffing.payslips` 的 `read/admin`）。
- fail-closed（缺 tenant context）：
  - 非租户 host 下访问 UI/internal API：允许 404/403/500/重定向，但不得泄漏数据；写入不得生效。

## 9. 测试步骤（执行时勾选）

> 约定：所有步骤都要记录 `Host/AUTHZ_MODE/RLS_ENFORCE`；涉及 internal API 的 URL 必须使用租户 host（例如 `http://t-060.localhost:8080/...`）。

### 9.1 运行态确认（硬要求）

1. [ ] 记录：`HOST/AUTHZ_MODE/RLS_ENFORCE/RUN_ID_2026_01/RUN_ID_2026_02`（见 §5.2 记录表）。

### 9.2 准备 `PP-2026-02` 与可计算 run（必须）

1. [ ] 若 `PAY_PERIOD_ID_2026_02/RUN_ID_2026_02` 尚未存在：按 TP-060-07 的口径创建并记录（本计划不重复定义 041-043 的创建表单）。
2. [ ] 断言：`RUN_ID_2026_02` 的 `run_state IN ('draft','failed')`（必须可再次 calculate）。

### 9.3 SAD 输入（044，可选但建议）

> SSOT：`docs/dev-plans/044-payroll-p0-slice-iit-cumulative-withholding-and-balances.md` §5.2、§6.5。  
> 目的：覆盖 internal API 写入口的幂等与“定稿后只读裁决”失败路径（不要求 UI 入口）。

1. [ ] 生成并记录 `SAD_EVENT_ID=<uuid>`（幂等键）。
2. [ ] 调用（租户 host 下）：`POST /org/api/payroll-iit-special-additional-deductions`（JSON）
   - `person_uuid=E06_PERSON_UUID`、`tax_year=2026`、`tax_month=2`、`amount="0.00"`（P0 可用最小值即可；避免 `PP-2026-01` 已定稿导致 month=1 只读裁决）
3. [ ] 断言：200 且返回同一 `event_id/request_id`（可重放）。
4. [ ] 幂等负例（必须）：用同一 `event_id` 但修改任一字段（例如 `amount` 改为 `1.00`）再次提交
   - 断言：409 且稳定错误码为 `STAFFING_IDEMPOTENCY_REUSED`。
5. [ ] 定稿后只读裁决负例（建议）：对已定稿月份写入必须失败（对齐 `DEV-PLAN-044` §6.6）
   - 前置：`PP-2026-01` 已 finalize（否则本步标记为阻塞并在 §11 记录）。
   - 生成并记录 `SAD_EVENT_ID_FINALIZED_PROBE=<uuid>`。
   - 调用：`POST /org/api/payroll-iit-special-additional-deductions`（JSON）
     - `person_uuid=E06_PERSON_UUID`、`tax_year=2026`、`tax_month=1`、`amount="0.00"`
   - 断言：409 且稳定错误码为 `STAFFING_IIT_SAD_CLAIM_MONTH_FINALIZED`。

### 9.4 balances 断言（044，必须，可判定）

> 断言目标：验证 “入职日在月中（E06）” 的累计减除费用月数口径与 O(1) balances 可读性。

1. [ ] 确认 `PP-2026-01` 已 finalize（来自 TP-060-07；否则记录为阻塞并停止本节）。
2. [ ] 调用：`GET /org/api/payroll-balances?person_uuid=<E06_PERSON_UUID>&tax_year=2026`
3. [ ] 断言（可判定，P0 固定值）：
   - `PP-2026-01` 定稿后：`first_tax_month=1`、`last_tax_month=1`、`ytd_standard_deduction="5000.00"`

### 9.5 回溯触发（045，必须）

> SSOT：`docs/dev-plans/045-payroll-p0-slice-retroactive-accounting.md` §6.2（触发点冻结）。  
> 触发样例：在 `PP-2026-01` 定稿后，为 E05 提交“更早 effective_date 的定薪变更”（例如加薪），命中 `PP-2026-01`。

1. [ ] 在 `PP-2026-01` 已 finalize 前提下，为 E05 提交 assignment 更新（按 TP-060-03 的唯一写入口）：
   - 打开（建议）：`GET /org/assignments?as_of=2026-01-15&pernr=105`
   - 提交（示例，满足 payroll-impactful）：把 `base_salary` 从 060-DS1 基线（`30000.00`）改为 `31000.00`，并保持：
     - `effective_date=2026-01-15`
     - `allocated_fte=1.0`、`currency=CNY`
   - 断言：提交成功后能在 Timeline 中看到 `effective_date=2026-01-15` 的更新版本（否则记录为 `ENV_DRIFT/CONTRACT_DRIFT` 并停止回溯步骤）。
2. [ ] 打开回溯请求列表：`GET /org/payroll-recalc-requests`
3. [ ] 断言：出现一条新请求（记录 `RECALC_REQUEST_ID`）。
4. [ ] 打开详情：`GET /org/payroll-recalc-requests/{RECALC_REQUEST_ID}`
5. [ ] 断言（可判定）：
   - `hit_pay_period_id == PAY_PERIOD_ID_2026_01`
   - `applied=false`（或页面等效信息：未结转）

### 9.6 回溯 apply（045，必须）

1. [ ] 在回溯请求详情页执行 apply：
   - `POST /org/payroll-recalc-requests/{RECALC_REQUEST_ID}/apply`，表单：`target_run_id=RUN_ID_2026_02`
2. [ ] 断言：303 回到详情页，并展示：
   - `target_run_id=RUN_ID_2026_02`
   - `target_pay_period_id=PAY_PERIOD_ID_2026_02`

### 9.7 计算 `RUN_ID_2026_02`（生成 payslips，必须）

1. [ ] 对 `RUN_ID_2026_02` 执行 calculate（按 TP-060-07 的口径）。
2. [ ] 打开 payslips 列表并定位 E07（记录 `E07_PAYSLIP_ID_2026_02`）：
   - `GET /org/payroll-runs/{RUN_ID_2026_02}/payslips`

### 9.8 净额保证输入（046，必须）

> SSOT：`docs/dev-plans/046-payroll-p0-slice-net-guaranteed-iit-tax-gross-up.md` §5.1。

1. [ ] 打开 E07 payslip 详情页：
   - `GET /org/payroll-runs/{RUN_ID_2026_02}/payslips/{E07_PAYSLIP_ID_2026_02}`
2. [ ] 提交净额保证项：
   - `POST /org/payroll-runs/{RUN_ID_2026_02}/payslips/{E07_PAYSLIP_ID_2026_02}/net-guaranteed-iit-items`
   - 表单建议固定：
     - `item_code=EARNING_LONG_SERVICE_AWARD`（P0 推荐固定，避免“执行期猜测”；SSOT：`DEV-PLAN-046` §5.1）
     - `target_net=20000.00`
     - `request_id=<uuidv4>`（幂等；建议使用 UUIDv4）
3. [ ] 断言：303 跳转回详情页；详情页能看到该输入项（至少 `target_net` 可见），且 UI 提示需要重新计算（若环境有提示）。
4. [ ] 幂等断言（建议）：
   - 重复提交同一表单（同一 `request_id` 且 payload 不变）
   - 断言：应幂等成功（303 或稳定成功响应均可），且输入值不发生变化。
5. [ ] 幂等冲突负例（建议）：
   - 仍使用同一 `request_id=<uuidv4>`，但把 `target_net` 改为 `20001.00` 再提交
   - 断言：必须失败并回显稳定错误码 `STAFFING_IDEMPOTENCY_REUSED`（或与环境一致的等效稳定错误码；若无法获得错误码则记录为 `CONTRACT_DRIFT`）。

### 9.9 重新计算 `RUN_ID_2026_02`（使净额保证生效，必须）

1. [ ] 再次对 `RUN_ID_2026_02` 执行 calculate（同一 run 二次计算允许覆盖未定稿结果）。

### 9.10 回溯与净额保证断言（必须，可判定）

1. [ ] 回溯差额可见：
   - 断言：`PP-2026-02` 的 payslip 中出现至少一条“回溯差额”明细项，并能追溯到 origin（例如展示 `origin_pay_period_id=PAY_PERIOD_ID_2026_01`，或 UI 等效溯源信息；SSOT：`DEV-PLAN-045` §5.1）。
   - 佐证：`PP-2026-01` 的已定稿 payslip 不被覆盖（origin 期只读；可用 “查看 2026-01 payslip 详情仍保持 finalized 证据” 作为佐证）。
2. [ ] 净额保证断言（分位精确）：
   - 打开 E07 payslip 详情：`GET /org/payroll-runs/{RUN_ID_2026_02}/payslips/{E07_PAYSLIP_ID_2026_02}`
   - 断言：`target_net == "20000.00"` 且 `net_after_iit == "20000.00"`（精确到分），并展示 `gross_amount/iit_delta`（解释字段可见）

### 9.11 定稿 `PP-2026-02` 并验证 balances 月推进（044，必须，可判定）

1. [ ] 对 `RUN_ID_2026_02` 执行 finalize（按 TP-060-07 的口径）。
2. [ ] 调用：`GET /org/api/payroll-balances?person_uuid=<E06_PERSON_UUID>&tax_year=2026`
3. [ ] 断言（可判定）：
   - `last_tax_month=2`
   - `ytd_standard_deduction="10000.00"`

### 9.12 安全与 fail-closed 负例（必须）

1. [ ] Authz（403）：以只读角色执行任一写入动作必须 403（任选其一）：
   - `POST /org/payroll-recalc-requests/{RECALC_REQUEST_ID}/apply`
   - `POST /org/payroll-runs/{RUN_ID_2026_02}/payslips/{E07_PAYSLIP_ID_2026_02}/net-guaranteed-iit-items`
2. [ ] fail-closed（缺 tenant context）：用非租户 host 访问 internal API（示例）：
   - `http://127.0.0.1:8080/org/api/payroll-balances?person_uuid=<E06_PERSON_UUID>&tax_year=2026`
   - 断言：不得返回任何租户数据（允许 404/403/500/重定向等稳定失败）。
3. [ ] fail-closed（缺 tenant context 的“写入不得生效”，建议，可判定）：
   - 在非租户 host 下尝试写入净额保证项（示例用 curl；表单字段按 046 契约）：
     ```bash
     curl -i -sS \
       -X POST "http://127.0.0.1:8080/org/payroll-runs/{RUN_ID_2026_02}/payslips/{E07_PAYSLIP_ID_2026_02}/net-guaranteed-iit-items" \
       -H "content-type: application/x-www-form-urlencoded" \
       --data "item_code=EARNING_LONG_SERVICE_AWARD&target_net=19999.00&request_id=<uuidv4>"
     ```
   - 断言 A：该请求不得返回 303（允许 4xx/5xx/重定向等稳定失败）。
   - 断言 B：回到租户 host 打开 E07 payslip 详情，确认净额保证项仍为 `target_net=20000.00`（写入未生效）。

## 10. 验收证据与执行记录（最小）

### 10.1 验收证据（最小）

- balances：`GET /org/api/payroll-balances` 的返回证据（包含 `first_tax_month/last_tax_month/ytd_standard_deduction`）。
- 回溯：recalc request 列表/详情证据 + apply 成功证据 + 后续周期差额明细与 origin 溯源证据。
- 净额保证：E07 payslip 中 `target_net/gross_amount/iit_delta/net_after_iit` 的可解释证据（含“分位精确相等”）。
- 安全：只读角色一次 403 + 缺 tenant context 的 fail-closed 负例。

### 10.2 执行记录（Readiness/可复现记录）

> 说明：此处只记录“本次执行实际跑了什么、结果如何”；命令入口以 `AGENTS.md` 为准。执行时把 `[ ]` 改为 `[X]` 并补齐时间戳与结果摘要。

- [ ] 文档门禁：`make check doc`
- [ ] （可选）E2E：`make e2e`（若补齐 TP-060-08 自动化用例）
- [ ] （可选）Authz：`make authz-pack && make authz-test && make authz-lint`
- [ ] （可选）路由治理：`make check routing`

## 11. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
