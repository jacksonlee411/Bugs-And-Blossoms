# DEV-PLAN-065：全链路业务测试子计划 TP-060-05——考勤 4B-4E（日结果/配置/时间银行/更正与审计）

**状态**: 已完成（2026-01-11 05:26 UTC；证据：见 §8.1）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`（平台基线）、`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（人员存在且可取到 person_uuid）、`docs/dev-plans/064-test-tp060-04-attendance-4a-punch-ledger.md`（已准备 punches 输入）。

## 1. 背景与上下文（Context）

本子计划是 TP-060 的“考勤主链路”中段：以 Punches（4A）为可重放输入，验证日结果（4B）可见可解释；再验证配置（4C）与时间银行（4D）可见可追溯；最后验证更正与审计（4E）可操作且 bounded replay（对齐 `docs/dev-plans/052-055`）。

约束与关键结论：
- 4B 选定 Option A：不提供“列表页手工范围重算入口”（更正/审计/重算由 4E 统一承接）。
- 4E 的最小闭环发生在日结果详情页：作废（void）→ 结果更新 → 审计可追溯（并允许“重算本日/范围重算”作为排障与回填入口）。
- 全程必须满足：Authz 可拒绝（403）+ RLS/tenancy fail-closed（缺租户上下文不得读/写）。

### 1.1 手工执行 vs 自动化回归（两条链路的职责分工）

- **手工执行（060-DS1 固定数据集）**：在 `T060 / t-060.localhost` 下复用 TP-060-03/04 的人员与 punches，并补齐本计划所需的“加班样例 punches”（E02/E10），保留数据供 TP-060-06/07/08 复用。
- **自动化回归（隔离数据）**：后续为本计划补齐独立的 E2E 用例（建议命名：`e2e/tests/tp060-05-attendance-4b-4e.spec.js`），使用 `runID` 创建独立 tenant 与独立数据，保证可重复跑；**不替代** 060-DS1 的固定可复现证据。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [X] **日结果可见可解释**：`/org/attendance-daily-results` 列表与 `/org/attendance-daily-results/{person_uuid}/{work_date}` 详情可用，且至少覆盖：`PRESENT` 与 `EXCEPTION` + `MISSING_OUT`。
- [X] **配置可操作**：`/org/attendance-time-profile` 与 `/org/attendance-holiday-calendar` 可保存并回显；Holiday 覆盖项能驱动 `day_type=LEGAL_HOLIDAY` 与 OT300 分桶出现。
- [X] **时间银行可追溯**：`/org/attendance-time-bank` 能展示月度累计（OT150/200/300 + comp_earned/used）与 trace 链接到日结果详情。
- [X] **更正与审计闭环（4E）**：在日结果详情页可作废一条 punch，作废后：日结果更新可见、审计区块能追溯 VOIDED 与原因；并支持“更正（replace）”口径：void → 回 punches 页补打一条正确 punch。
- [X] **安全与隔离**：
  - Authz：只读角色对任一 admin/写入动作（TimeProfile 保存、HolidayCalendar day_set、Daily Result void/recalc）必须 403。
  - fail-closed：缺 tenant context 不得读/写（允许 404/403/500/稳定错误码，但不得泄漏数据）。
- [X] **自动化回归（最小）**：`make e2e` 覆盖本子计划最小链路（列表可见 + void + time bank trace）。

### 2.2 非目标

- 不验证外部对接摄入（由 TP-060-06 承接）。
- 不在本子计划要求“精确分钟数/精确金额对账”；除非上游契约明确给出可判定断言，否则以“状态/桶非零/可追溯链接”作为最小验收口径。

### 2.3 工具链与门禁（SSOT 引用）

> 目的：避免在子计划里复制工具链细节导致 drift；本文只声明“本子计划执行/修复时可能命中的门禁入口”，命令细节以 `AGENTS.md`/`Makefile` 为准。

- 触发器清单（按需勾选本计划命中的项；执行记录见 §8.1）：
  - [X] E2E（`make e2e`）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`；若权限口径 drift）
  - [X] 路由治理（`make check routing`；若新增/调整路由或 allowlist）
  - [X] 文档（本文件变更：`make check doc`）
  - [X] Go 代码（仅当为修复 drift 而改 Go：`go fmt ./... && go vet ./... && make check lint && make test`）
  - [X] DB/迁移（仅当为修复 drift 而改 DB：按模块 `make <module> plan/lint/migrate ...`）

- SSOT 链接：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口与脚本实现：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`
  - 路由 allowlist（事实源）：`config/routing/allowlist.yaml`
  - Authz policy 产物（事实源）：`config/access/policy.csv`、`config/access/policy.csv.rev`

## 3. 契约引用（SSOT）

- 4B 日结果：`docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`
- 4C 配置：`docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`
- 4D 时间银行：`docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`
- 4E 更正与审计：`docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`
- 测试套件/数据集：`docs/dev-plans/060-business-e2e-test-suite.md`
- 安全：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`

## 4. 前置条件与数据准备（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- 运行态硬要求：`AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`（对齐 `docs/dev-plans/060-business-e2e-test-suite.md` §4.3）
- 统一时区语义：UI 输入按北京时间（Asia/Shanghai）
- 关键日期（建议固定，避免执行期漂移）：
  - `AS_OF_CONFIG=2026-01-01`（配置页）
  - `D_HOLIDAY=2026-01-01`（Holiday + OT300 样例）
  - `D_WORKDAY=2026-01-02`（PRESENT/EXCEPTION + MISSING_OUT 样例）
  - `D_RESTDAY=2026-01-03`（RESTDAY + OT200 + comp earned 样例）
  - `MONTH=2026-01`

### 4.1 `as_of` 缺省行为（避免执行期漂移）

- UI 路由：若 `GET` 未提供 `as_of`，服务端可能 `302` 重定向补上 `as_of=<当前UTC日期>`。
- 断言口径：本子计划的核心断言以 `work_date`（日结果）与 `month`（时间银行）为准；`as_of` 仅作为 UI Shell 的统一输入（可能影响默认值/跳转）。
- 结论：本子计划所有 URL 均显式带 `as_of=...`；涉及月份的页面显式带 `month=YYYY-MM`；涉及日期的查询显式带 `work_date=YYYY-MM-DD`；并尽量让 `as_of == work_date`（降低执行歧义）。

### 4.2 记录表（执行前填写）

> 说明：E01/E02/E03/E10 的 `person_uuid` 建议直接复用 TP-060-03 的证据记录表，避免重复建人（SSOT：`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`）。

| 人员 | person_uuid | 备注 |
| --- | --- | --- |
| E01 | `<E01_PERSON_UUID>` | 用于 PRESENT 与 void 更正样例 |
| E02 | `<E02_PERSON_UUID>` | 用于 Holiday（LEGAL_HOLIDAY）+ OT300 样例 |
| E03 | `<E03_PERSON_UUID>` | 用于缺卡（缺 OUT）样例 |
| E10 | `<E10_PERSON_UUID>` | 用于 RESTDAY + OT200 + comp earned 样例 |

### 4.3 punches 输入准备（060-DS1 子集 + 本计划增量）

> 说明：TP-060-04 已为 E01/E03 写入 `2026-01-02` punches；本计划在不清理数据的前提下补齐“加班/时间银行样例 punches”。

- 必须具备（若缺失则补齐创建）：
  - E01（来自 TP-060-04）：`2026-01-02 09:00 IN / 18:00 OUT`
  - E03（来自 TP-060-04）：`2026-01-02 09:00 IN`（缺 OUT）
  - E02（本计划补齐）：`2026-01-01 08:00 IN / 20:00 OUT`（配合 Holiday 覆盖项，生成 OT300）
  - E10（本计划补齐）：`2026-01-03 08:00 IN / 20:00 OUT`（周末 RESTDAY，生成 OT200 + comp earned）

### 4.4 配置输入准备（TimeProfile / HolidayCalendar）

- TimeProfile（租户默认，effective-dated）：在 `/org/attendance-time-profile?as_of=2026-01-01` 保存一条版本：
  - `effective_date=2026-01-01`
  - `shift_start_local=09:00`
  - `shift_end_local=18:00`
- HolidayCalendar（按月覆盖项）：在 `/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01` 把 `2026-01-01` 设置为：`day_type=LEGAL_HOLIDAY`。

### 4.5 可重复执行口径（Idempotency / Re-run）

- 本计划优先“存在则复用、缺失则补齐”；不得为了重跑而删除历史数据。
- punches：若目标人员在目标日期已存在相同分钟的 punch，不得重复提交；若必须新增用于验证，请使用未使用的分钟（例如 `18:01`）并在证据中记录。
- void：若目标 punch 已作废，按 4E 幂等语义视为 no-op；如需重复验证“PRESENT→EXCEPTION”的过程，请先补打一条新的 OUT 再对新 OUT 执行 void。

### 4.6 数据保留（强制）

- 本子计划创建/修改的配置（TimeProfile/HolidayCalendar）、作废记录（VOIDED）与日结果变化必须保留，供后续排障与 TP-060-06/07/08 的复用与回归（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

## 5. 页面/接口契约（用于断言；细节以 SSOT 为准）

> 本节只摘录测试必需的最小合同，避免与上游 `DEV-PLAN-052~055` 复制漂移。

### 5.1 UI：`GET/POST /org/attendance-time-profile`

- GET 查询参数（最小集）：`as_of`
- POST（保存）表单字段（最小集）：`op=save`、`effective_date`、`shift_start_local`、`shift_end_local`
- 成功响应：`303` 重定向回 GET；失败响应：`200` 返回页面并展示错误信息

### 5.2 UI：`GET/POST /org/attendance-holiday-calendar`

- GET 查询参数（最小集）：`as_of`、`month`
- POST（设置覆盖项）表单字段（最小集）：`op=day_set`、`day_date`、`day_type`
- 成功响应：`303` 重定向回 GET（保持 `as_of/month`）；失败响应：`200` 返回页面并展示错误信息

### 5.3 UI：`GET /org/attendance-daily-results`

- GET 查询参数（最小集）：`as_of`、`work_date`
- 响应（200 HTML）：展示结果表格，包含 `Person/Day Type/Status/Flags/OT150/OT200/OT300/.../Computed At`，并提供详情链接到：`/org/attendance-daily-results/{person_uuid}/{work_date}?as_of=...`

### 5.4 UI：`GET/POST /org/attendance-daily-results/{person_uuid}/{work_date}`

- GET：详情页应至少展示：`day_type/status/flags/first_in_time/last_out_time/worked_minutes/*overtime_minutes_*/computed_at`，并提供“查看 punches”跳转链接（到 `/org/attendance-punches?...`）。
- POST（更正/审计）：按 4E 的 `op` 分支执行（例如 `op=void_punch`、`op=recalc_day`、`op=recalc_range`）。

### 5.5 UI：`GET /org/attendance-time-bank`

- GET 查询参数（最小集）：`as_of`、`person_uuid`、`month`
- 响应（200 HTML）：展示月度累计（至少包含 OT150/OT200/OT300 + comp_earned/used）与 trace（对“有贡献”的日结果提供链接到日结果详情）。

### 5.6 Internal API（可选，用于断言）：`GET /org/api/attendance-daily-results`

- 用途：E2E/自动化验收与调试；不作为公共 API 承诺（SSOT：`docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`）。

## 6. 关键不变量（必须成立）

- **时间语义**：`work_date` 为 `date`；UI 输入 punches 按 Asia/Shanghai；不以“执行当天”作为默认口径做验收。
- **Option A 边界**：列表页不提供“范围重算入口”；更正/重算入口统一在详情页（4E）。
- **bounded replay**：对某人某日作废/重算不应导致全量数据重建；且结果变化可在详情页审计区块追溯。
- **fail-closed**：缺 tenant context 的读写不得生效（不得显示结果，不得落库写入）。
- **No Legacy / One Door**：更正必须走 4E 的事件入口（void/recalc），不得通过 SQL 直改读模表“修结果”。

## 7. 测试步骤（执行时勾选）

> 执行记录要求：每步至少记录 `Host/as_of/AUTHZ_MODE/RLS_ENFORCE`；失败必须填写 §9 问题记录。

### 7.1 运行态确认（硬要求）

1. [ ] 记录：`Host=t-060.localhost`、`AUTHZ_MODE`、`RLS_ENFORCE`、`AS_OF_CONFIG/D_HOLIDAY/D_WORKDAY/D_RESTDAY/MONTH`。

### 7.2 配置：TimeProfile（4C）

1. [ ] 打开：`/org/attendance-time-profile?as_of=2026-01-01`
2. [ ] 保存（`op=save`）：`effective_date=2026-01-01`、`shift_start_local=09:00`、`shift_end_local=18:00`
3. [ ] 断言：保存后回显（303 redirect 后页面可见已保存的版本信息）

### 7.3 配置：HolidayCalendar（4C）

1. [ ] 打开：`/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01`
2. [ ] 将 `2026-01-01` 设置为 `LEGAL_HOLIDAY`（`op=day_set`）
3. [ ] 断言：页面回显该覆盖项（同一月份视图可见）

### 7.4 补齐 punches 输入（4A → 4B/4D）

> 说明：若 punches 已存在则复用；若缺失则用 `/org/attendance-punches` 手工补齐（或 CSV 导入），并记录是否为“补齐/复用”。punches 页的写入/导入细节与原子性口径以 TP-060-04 为准：`docs/dev-plans/064-test-tp060-04-attendance-4a-punch-ledger.md`。

1. [ ] 确认/补齐 E02（Holiday）：
   - 打开：`/org/attendance-punches?as_of=2026-01-01&person_uuid=<E02_PERSON_UUID>&from_date=2026-01-01&to_date=2026-01-01`
   - 补齐：`2026-01-01T08:00 IN`、`2026-01-01T20:00 OUT`（若已存在则复用）
2. [ ] 确认/补齐 E10（RESTDAY）：
   - 打开：`/org/attendance-punches?as_of=2026-01-03&person_uuid=<E10_PERSON_UUID>&from_date=2026-01-03&to_date=2026-01-03`
   - 补齐：`2026-01-03T08:00 IN`、`2026-01-03T20:00 OUT`（若已存在则复用）
3. [ ] 确认 E01/E03（来自 TP-060-04）：
   - E01：`/org/attendance-punches?as_of=2026-01-02&person_uuid=<E01_PERSON_UUID>&from_date=2026-01-02&to_date=2026-01-02`（应有 `09:00 IN/18:00 OUT`）
   - E03：`/org/attendance-punches?as_of=2026-01-02&person_uuid=<E03_PERSON_UUID>&from_date=2026-01-02&to_date=2026-01-02`（应仅有 `09:00 IN`）

### 7.5 日结果列表断言（4B/4C）

1. [ ] `work_date=2026-01-02`：打开 `/org/attendance-daily-results?as_of=2026-01-02&work_date=2026-01-02`
2. [ ] 断言（可判定）：
   - E01：`Status=PRESENT`
   - E03：`Status=EXCEPTION` 且 `Flags` 包含 `MISSING_OUT`
3. [ ] `work_date=2026-01-01`：打开 `/org/attendance-daily-results?as_of=2026-01-01&work_date=2026-01-01`
4. [ ] 断言（可判定）：E02 行 `Day Type=LEGAL_HOLIDAY`，且 `OT300 > 0`
5. [ ] `work_date=2026-01-03`：打开 `/org/attendance-daily-results?as_of=2026-01-03&work_date=2026-01-03`
6. [ ] 断言（可判定）：E10 行 `Day Type=RESTDAY`，且 `OT200 > 0`

### 7.6 日结果详情断言：链接与可追溯性（4B）

1. [ ] 打开 E01 详情：`/org/attendance-daily-results/<E01_PERSON_UUID>/2026-01-02?as_of=2026-01-02`
2. [ ] 断言：详情页存在“查看 punches”链接，且指向 `/org/attendance-punches?...from_date=2026-01-02&to_date=2026-01-02`

### 7.7 更正与审计闭环（4E）：void → 结果更新 →（可选）更正补打

1. [ ] 在 E01 的 `2026-01-02` 详情页：
   - 从 “Void Punch” 下拉中选中目标 `18:00 OUT`；**记录其 `target_punch_event_id`（下拉选项 value）**，用于复现与排障。
2. [ ] 对该 `18:00 OUT` 执行作废（void）
3. [ ] 断言（可判定）：
   - 详情页 summary 变为 `EXCEPTION` 且 flags 包含 `MISSING_OUT`
   - punches 审计列表中该 `OUT` 标记为 `VOIDED`（且可见 void 事件/原因信息，若实现已展示）
4. [ ] 回到日结果列表（`work_date=2026-01-02`）断言 E01 行状态同步变更为 `EXCEPTION`
5. [ ] （可选但建议）更正（replace）：跳转 punches 页为 E01 在 `2026-01-02` 补打一条新的 `OUT`（建议用未使用分钟，例如 `18:01 OUT`），再回到详情页：
   - 断言：状态恢复为 `PRESENT` 且 flags 不含 `MISSING_OUT`
   - 断言：审计区块同时展示“已 VOID 的旧 OUT”与“新的 OUT”
6. [ ] （可选）重算入口：点击“重算本日”（`op=recalc_day`），断言 `computed_at` 更新（并记录页面是否展示 recalc 事件）

### 7.8 时间银行断言（4D）：月度累计 + trace

1. [ ] 打开 E10 的 time bank：`/org/attendance-time-bank?as_of=2026-01-03&month=2026-01&person_uuid=<E10_PERSON_UUID>`
2. [ ] 断言（可判定）：`OT200 > 0` 且 `comp_earned_minutes > 0`；trace 中包含 `2026-01-03` 且链接可跳转到对应日结果详情
3. [ ] （可选）打开 E02 的 time bank：`/org/attendance-time-bank?as_of=2026-01-01&month=2026-01&person_uuid=<E02_PERSON_UUID>`
4. [ ] （可判定）：`OT300 > 0`；trace 中包含 `2026-01-01` 且链接可跳转到对应日结果详情
5. [ ] 缺失判定与最小补救（用于留证，不作为“正常流程”）：
   - 若出现 `(no cycle computed yet)` 或 `(no daily results)`：先回到对应的日结果详情页点击 `Recalc This Day`（`op=recalc_day`），再刷新 time bank。
   - 若 recalc 后仍缺失：记录为 `BUG/CONTRACT_DRIFT`（契约引用：`docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`，联动口径见其“bounded replay/联动重算”章节），并在问题记录中附上“punches 已存在 + daily results/ time bank 为空”的证据。
   - 若 recalc 后出现：仍记录为 `BUG/CONTRACT_DRIFT`（说明“自动联动未发生，需要手工重算才出现”），并附上 before/after 证据。

### 7.9 安全与 fail-closed 负例（必须）

1. [ ] Authz：以只读用户执行任一 admin 动作必须 403（示例二选一即可）：
   - `POST /org/attendance-time-profile?as_of=2026-01-01`（保存）
   - `POST /org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01`（day_set）
   - `POST /org/attendance-daily-results/<E01_PERSON_UUID>/2026-01-02?as_of=2026-01-02`（void/recalc）
   - 若无法创建/分配只读角色，记录为 `CONTRACT_MISSING/ENV_DRIFT`（对齐 TP-060-01）。
2. [ ] fail-closed（缺 tenant context）：用非租户 host 访问（示例）：`http://127.0.0.1:8080/org/attendance-daily-results?as_of=2026-01-02&work_date=2026-01-02`
   - 断言：不得展示任何结果数据（允许 404/403/500/稳定错误码）。
3. [ ] 参数校验负例：`/org/attendance-daily-results?as_of=2026-01-02&work_date=BAD`
   - 断言：页面提示 `work_date 无效: ...`（或等效稳定错误信息）。

## 8. 验收证据与执行记录（最小）

- TimeProfile 配置保存成功证据（回显/截图）。
- HolidayCalendar 覆盖项保存证据（`2026-01-01=LEGAL_HOLIDAY`）。
- 日结果列表证据：
  - `work_date=2026-01-02`：E01= `PRESENT`、E03=`EXCEPTION + MISSING_OUT`
  - `work_date=2026-01-01`：E02 `LEGAL_HOLIDAY` 且 `OT300 > 0`
  - `work_date=2026-01-03`：E10 `RESTDAY` 且 `OT200 > 0`
- 更正与审计证据：void 后 `VOIDED` 标记 + 状态变化（**必须附 `target_punch_event_id`**）。
- 时间银行证据：
  - E10：`OT200 > 0` 且 `comp_earned_minutes > 0` + trace 链接可跳转
  - （可选）E02：`OT300 > 0` + trace 链接可跳转
- 安全证据：只读角色一次 403 + 缺 tenant context 负例（不泄漏数据）。

### 8.1 执行记录（Readiness/可复现记录）

> 说明：此处只记录“本次执行实际跑了什么、结果如何”；命令入口以 `AGENTS.md` 为准。执行时把 `[ ]` 改为 `[X]` 并补齐时间戳与结果摘要。

- [X] DB/迁移闭环（staffing）：`make staffing plan && make staffing lint && make staffing migrate up` ——（2026-01-11 05:19 UTC，结果：PASS）
- [X] 一键对齐 CI：`make preflight` ——（2026-01-11 05:24 UTC，结果：PASS；E2E 覆盖用例：`e2e/tests/tp060-05-attendance-4b-4e.spec.js`）

## 9. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-01-11 04:40 | `t-tp060-05-<runID>.localhost` / `as_of=2026-01-03` / `AUTHZ_MODE=enforce` + `RLS_ENFORCE=enforce` | `make e2e` 跑到 TP-060-05，Time Bank 页面显示 `(no cycle computed yet)` | 日结果重算必须同事务联动更新 time bank cycles（`docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md` + `docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`） | daily results 存在但 cycle 未生成，导致 Time Bank 汇总缺失 | P1 | CONTRACT_DRIFT | 在 `recompute_daily_attendance_result(...)` 末尾联动 `recompute_time_bank_cycle(...)` 并补迁移 | wt-dev-a | `migrations/staffing/20260111130000_staffing_attendance_daily_results_time_bank_linkage.sql` |
