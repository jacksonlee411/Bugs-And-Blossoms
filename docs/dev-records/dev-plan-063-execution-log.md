# DEV-PLAN-063 记录：TP-060-03——人员与任职（Person + Assignments）执行日志

**状态**：已完成（2026-01-11）

> 对应计划：`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`  
> 上游套件：`docs/dev-plans/060-business-e2e-test-suite.md`

## 交付物

- 自动化用例：`e2e/tests/tp060-03-person-and-assignments.spec.js`
- 任职写入口补齐（薪酬输入）：`/org/assignments` UI + `/org/api/assignments` Internal API 支持 `base_salary/allocated_fte`

## 执行结论

- `make e2e`：通过（包含 `tp060-02` 与 `tp060-03`）
- `make test`：通过（coverage 门禁 100%）
- `make check lint`：通过

## 关键修复/对齐点

- `/org/assignments`：当 `pernr` 与 `person_uuid` 同时提交时，强校验一致性并将 redirect `pernr` 规范化为 canonical（避免前导 0 漂移）。
- 任职事件 payload：`UpsertPrimaryAssignmentForPerson(...)` 支持写入 `base_salary/allocated_fte`（One Door：仍通过 `staffing.submit_assignment_event(...)`）。
- E2E “可见超时”根因：Playwright 误选了页面顶栏（Topbar）中用于保留 query 的隐藏 `pernr` input，导致 `fill()` 等待可见性直至超时；修复为精确定位 “Select Person” 表单与 “Upsert Primary” 表单。

## 证据与产物（本地）

- Playwright 报告：`e2e/playwright-report/`
- 失败/追踪（如需）：`e2e/test-results/`
- 截图（执行时生成，不入库）：`e2e/_artifacts/tp060-03-*.png`

## 数据保留（强制）

- `make e2e` 默认不会执行 `make dev-reset`/`docker compose down -v`，因此测试数据会保留在本地 dev DB 卷中（用于复盘与后续子计划复用）。

