# DEV-PLAN-064：全链路业务测试子计划 TP-060-04——考勤 4A（Punch Ledger：手工补卡 + 最小导入）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（至少确保 10 人存在）。

## 1. 背景

`DEV-PLAN-051` 定义考勤输入底座：punch 事件 append-only，唯一写入口为 DB Kernel（One Door），并提供 UI `/org/attendance-punches` 完成“可见/可操作”的最小闭环。本子计划验证 punches 的手工录入与最小导入路径，并为 4B-4E 结果链路准备输入数据。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] `/org/attendance-punches` 可按人员+日期范围查询流水。
- [ ] 可手工补打卡（IN/OUT）并立即在列表可见。
- [ ] 可通过最小导入（CSV 粘贴）写入 punches 并在列表可见。
- [ ] Authz 可拒绝：只读角色对 POST 必须 403（若无法分配只读角色按问题记录处理）。

### 2.2 非目标

- 不在本子计划验证日结果/重算/时间银行/外部对接（由 TP-060-05/06 承接）。

## 3. 契约引用（SSOT）

- 考勤 4A：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`
- RLS/Authz：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- `as_of`：建议 `2026-01-02`
- 需要的人员：E01/E03/E10（`person_uuid` 已记录）

## 5. 测试步骤（执行时勾选）

1. [ ] 打开 punches 页面：`/org/attendance-punches?as_of=2026-01-02`（确认页面可见）。
2. [ ] 手工录入（E01）：`2026-01-02 09:00 IN` 与 `18:00 OUT`，提交后列表可见。
3. [ ] 缺卡样例（E03）：仅录入 `2026-01-02 09:00 IN`（用于后续缺卡纠错链路）。
4. [ ] 最小导入（E10）：用 CSV 粘贴导入 `2026-01-02` 的 2 条记录（IN/OUT），提交后列表可见。
5. [ ] Authz 拒绝（可选）：以只读用户访问同一页面，提交任一 POST 必须 403（若无法分配只读角色按问题记录处理）。

## 6. 验收证据（最小）

- E01/E03/E10 的 punches 列表证据（含时间与类型）。
- 导入成功证据（至少 1 条导入记录可见）。
- 403 证据（若执行）。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

