# DEV-PLAN-064：全链路业务测试子计划 TP-060-04——考勤 4A（Punch Ledger：手工补卡 + 最小导入）

**状态**: 已完成（2026-01-11；证据：见 §8.1）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（至少确保 10 人存在）。

## 1. 背景与上下文（Context）

`DEV-PLAN-051` 定义考勤输入底座：punch 事件 append-only，唯一写入口为 DB Kernel（One Door），并提供 UI `/org/attendance-punches` 完成“可见/可操作”的最小闭环。本子计划验证 punches 的手工录入与最小导入路径，并为 4B-4E 结果链路准备输入数据。

### 1.1 手工执行 vs 自动化回归（两条链路的职责分工）

- **手工执行（060-DS1 固定数据集）**：在 `T060 / t-060.localhost` 下，为 E01/E03/E10 写入 `2026-01-02` 的 punches，并**保留数据**供 TP-060-05 复用。
- **自动化回归（隔离数据）**：`e2e/tests/tp060-04-attendance-punch-ledger.spec.js` 使用 `runID` 创建独立 tenant 与独立数据，保证可重复跑；**不替代** 060-DS1 的“固定可复现数据集”证据。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 目标（Done 定义）

- [X] `/org/attendance-punches` 可按人员+日期范围查询流水。
- [X] 可手工补打卡（IN/OUT）并立即在列表可见。
- [X] 可通过最小导入（CSV 粘贴）写入 punches 并在列表可见。
- [X] Authz 可拒绝：只读角色对 POST 必须 403（若无法分配只读角色按问题记录处理）。
- [X] fail-closed：未注入 tenant context 时不得读/写 punches（允许 404/403/500/稳定错误码，但不得展示数据）。

### 2.2 非目标

- 不在本子计划验证日结果/重算/时间银行/外部对接（由 TP-060-05/06 承接）。

### 2.3 工具链与门禁（SSOT 引用）

> 目的：避免在子计划里复制工具链细节导致 drift；本文只声明“本子计划命中哪些触发器/门禁”，命令细节以 `AGENTS.md`/`Makefile`/CI workflow 为准。

- **触发器清单（勾选本计划命中的项；执行记录见 §8）**：
  - [X] E2E（`make e2e`；用例：`e2e/tests/tp060-04-attendance-punch-ledger.spec.js`）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`；仅当权限口径 drift）
  - [ ] 路由治理（`make check routing`；仅当新增/调整路由）
  - [X] 文档（`make check doc`）
  - [ ] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`；仅当为修复而改 Go）
  - [ ] DB/迁移（按模块 `make <module> plan/lint/migrate ...`；仅当为修复而改 DB）

- **SSOT 链接**：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`

## 3. 契约引用（SSOT）

- 考勤 4A：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`
- 测试套件口径：`docs/dev-plans/060-business-e2e-test-suite.md`
- RLS/Authz：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`

## 4. 关键不变量（必须成立）

- **One Door / append-only**：punch 写入只能通过 DB Kernel `staffing.submit_time_punch_event(...)`；`staffing.time_punch_events` 为事件 SoT，仅允许 INSERT（SSOT：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`）。
- **时间语义**：
  - UI 的 `punch_at` 为 `datetime-local`，按北京时间解释（SSOT：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md` §5.1）。
  - UI 查询 `from_date/to_date` 为北京时间语义，闭区间（SSOT：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md` §6.2）。
- **导入事务语义**：最小导入为“全量原子”，任一行失败则整体回滚并返回“第 N 行错误原因”（SSOT：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md` §5.1）。
- **强隔离**：缺失租户注入必须 fail-closed，不得返回 punches 数据或放行写入（SSOT：`AGENTS.md` 的 No Tx, No RLS + `docs/dev-plans/051-attendance-slice-4a-punch-ledger.md` §7.1）。

## 5. 前置条件与数据准备（手工：060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- `as_of`：固定 `2026-01-02`
- 需要的人员：E01/E03/E10（`person_uuid` 需提前记录；建议来源：`docs/dev-plans/063-test-tp060-03-person-and-assignments.md` 的执行证据与记录表）
- 查询范围：固定 `from_date=2026-01-02`、`to_date=2026-01-02`（单日）

### 5.1 数据保留（强制）

- 本子计划写入的 punches（含缺卡样例与导入样例）必须保留，用于 TP-060-05 的日结果/更正与审计链路复用（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

### 5.2 记录表（执行前填写）

| 人员 | person_uuid | 备注 |
| --- | --- | --- |
| E01 | `<E01_PERSON_UUID>` | 用于完整 IN/OUT 样例 |
| E03 | `<E03_PERSON_UUID>` | 用于“缺 OUT”样例 |
| E10 | `<E10_PERSON_UUID>` | 用于 CSV 导入样例 |

### 5.3 可重复执行口径（避免污染）

- 若目标人员在 `2026-01-02` 已存在相同时间点的 punch（例如已存在 `09:00 IN`），不得重复提交导致重复事件；应记录为“已存在并复用”，并在 §7 的证据里标注复用的行。
- 若出现同一人员同一分钟存在多条 punch 且无法判断来源（手工/导入重复）：记录为 `ENV_DRIFT` 并停止后续步骤（避免 4B-4E 输入不稳定）。

## 6. 页面/接口契约（用于断言；细节以 SSOT 为准）

> 本节只摘录测试必需的最小合同，避免与 `DEV-PLAN-051` 复制漂移。

### 6.1 UI：`GET/POST /org/attendance-punches`

- GET 查询参数（最小集）：`as_of`、`person_uuid`、`from_date`、`to_date`
- POST（手工补卡）表单字段（最小集）：`op=manual`、`person_uuid`、`punch_at`、`punch_type`
- POST（最小导入）表单字段（最小集）：`op=import`、`csv`
- 成功响应：`303` 重定向回 GET（携带 `person_uuid/from_date/to_date`）
- 失败响应：`200` 返回页面并展示可读错误信息

### 6.2 Internal API：`GET /org/api/attendance-punches`（可选，用于断言）

- 查询参数：`person_uuid`（Required） + `from/to`（RFC3339；UTC 半开区间 `[from,to)`）
- 响应（200）：返回 punches 列表（字段以 `DEV-PLAN-051` §5.2 为准）

## 7. 测试步骤（执行时勾选）

> 约定：所有 UI 输入的 `punch_at` 均按北京时间填写（`YYYY-MM-DDTHH:MM`）。

### 7.1 查询与页面可见

1. [ ] 打开 punches 页面（E01）：`/org/attendance-punches?as_of=2026-01-02&person_uuid=<E01_PERSON_UUID>&from_date=2026-01-02&to_date=2026-01-02`
2. [ ] 断言：页面可见，列表可按人员+日期范围展示 punches（即使为空也应稳定展示“无数据”状态）。

### 7.2 手工补卡（E01）

1. [ ] 提交 `2026-01-02T09:00`，`IN`（`op=manual`）
2. [ ] 提交 `2026-01-02T18:00`，`OUT`（`op=manual`）
3. [ ] 断言：提交后回到 GET 且列表立即可见两条记录（时间与类型正确）。

### 7.3 缺卡样例（E03，仅 IN）

1. [ ] 打开页面（E03）：`/org/attendance-punches?as_of=2026-01-02&person_uuid=<E03_PERSON_UUID>&from_date=2026-01-02&to_date=2026-01-02`
2. [ ] 提交 `2026-01-02T09:00`，`IN`（`op=manual`）
3. [ ] 断言：列表仅出现 `IN`；作为 TP-060-05 的“缺 OUT”输入样例必须保留。

### 7.4 最小导入（E10，CSV 粘贴）

1. [ ] 打开页面（E10）：`/org/attendance-punches?as_of=2026-01-02&person_uuid=<E10_PERSON_UUID>&from_date=2026-01-02&to_date=2026-01-02`
2. [ ] 在导入区粘贴（示例；以实际人员 uuid 替换）：
   ```csv
   <E10_PERSON_UUID>,2026-01-02T09:00,IN
   <E10_PERSON_UUID>,2026-01-02T18:00,OUT
   ```
3. [ ] 提交导入（`op=import`）
4. [ ] 断言：导入后列表可见两条记录；若任一行格式不合法，应整体失败且提示“第 N 行错误原因”。
5. [ ] 导入原子性负例（必须，可判定）：将以下 CSV 作为一次导入提交（注意选择一个未使用的分钟；若 `12:01/12:02` 已存在则换成别的分钟，并在证据里注明）：
   ```csv
   <E10_PERSON_UUID>,2026-01-02T12:01,IN
   <E10_PERSON_UUID>,2026-01-02T12:02,OUT
   <E10_PERSON_UUID>,2026-01-02T12:03,BAD
   ```
   - 断言 A：本次导入必须失败，且提示“第 N 行错误原因”（或等效稳定错误信息）。
   - 断言 B：导入失败后，列表中不得出现 `12:01 IN` 与 `12:02 OUT`（全量原子回滚）。

### 7.5 Authz 拒绝（可选）

1. [ ] 以只读用户访问同一页面（E01 或任一人员）
2. [ ] 断言：任一 POST 必须 `403`（若无法分配只读角色，按 §9 记录为 `CONTRACT_MISSING/ENV_DRIFT` 并写明阻塞点）

### 7.6 fail-closed：未注入 tenant context（必须）

1. [ ] 用非租户 host 访问（示例）：`http://127.0.0.1:8080/org/attendance-punches?as_of=2026-01-02`
2. [ ] 断言：不得展示任何 punches 数据；不得可写（允许 404/403/500/稳定错误码）

### 7.7 Internal API 断言（可选，建议）

1. [ ] 调用：`GET /org/api/attendance-punches?person_uuid=<E01_PERSON_UUID>&from=2026-01-02T00:00:00Z&to=2026-01-03T00:00:00Z`
2. [ ] 断言：返回 punches 至少包含 `IN/OUT` 两条；`punch_time` 应可与北京时间输入对应（例如 `09:00+08:00` 对应 `01:00Z`）。

### 7.8 fail-closed：未注入 tenant context 的“写入不得生效”（必须，可判定）

> 目的：避免只验证“页面空”，但写入口仍可能被误放行。

1. [ ] 在非租户 host 下对 internal API 发起写入尝试（示例用 curl；请选一个未使用的分钟，若 `12:34` 已存在则换成别的分钟，并在证据里注明）：
   ```bash
   curl -i -sS \
     -X POST "http://127.0.0.1:8080/org/api/attendance-punches" \
     -H "content-type: application/json" \
     -d '{"person_uuid":"<E01_PERSON_UUID>","punch_time":"2026-01-02T12:34:00+08:00","punch_type":"IN","source_provider":"MANUAL","payload":{"note":"fail-closed probe"},"source_raw_payload":{},"device_info":{}}'
   ```
2. [ ] 断言 A：该请求不得返回 201/200（允许 4xx/5xx/稳定错误码，但不得返回 punches 数据）。
3. [ ] 断言 B：回到租户 `t-060.localhost` 的 punches 列表（E01，单日范围）确认不存在 `12:34 IN`（写入未生效）。

### 7.9 跨租户隔离（可选，建议）

> 前置：按 `docs/dev-plans/060-business-e2e-test-suite.md` 的 060-DS2 在 `T060B / t-060b.localhost` 创建 Person `pernr=201` 并记录其 `T060B_PERSON_UUID`。

1. [ ] 在 `T060 / t-060.localhost` 下访问：`/org/attendance-punches?as_of=2026-01-02&person_uuid=<T060B_PERSON_UUID>&from_date=2026-01-02&to_date=2026-01-02`
2. [ ] 断言：不得泄漏 T060B 的任何 punches（允许空/404/稳定错误码）。

## 8. 验收证据与执行记录（最小）

- E01/E03/E10 的 punches 列表证据（含时间与类型）。
- 导入成功证据（至少 1 条导入记录可见）。
- 导入原子性负例证据（失败提示 + `12:01/12:02` 未落库）。
- 403 证据（若执行）。
- fail-closed 证据（非租户 host 不得展示 punches）。
- fail-closed 写入未生效证据（写入请求失败 + `12:34` 未落库）。
- 跨租户隔离证据（若执行）。
- 自动化回归用例：`e2e/tests/tp060-04-attendance-punch-ledger.spec.js`

### 8.1 执行记录（Readiness/可复现记录）

> 说明：此处只记录“本次执行实际跑了什么、结果如何”；命令入口以 `AGENTS.md` 为准。执行时把 `[ ]` 改为 `[X]` 并补齐时间戳与结果摘要。

- [X] 文档门禁：`make check doc` ——（2026-01-11 03:10 UTC，结果：PASS）
- [X] E2E：`make e2e` ——（2026-01-11 03:32 UTC，结果：PASS；用例：`e2e/tests/tp060-04-attendance-punch-ledger.spec.js`）

## 9. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
