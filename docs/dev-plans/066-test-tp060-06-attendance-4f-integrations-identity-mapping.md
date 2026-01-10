# DEV-PLAN-066：全链路业务测试子计划 TP-060-06——考勤 4F（外部对接 + 身份映射）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（人员存在）与 `docs/dev-plans/064-test-tp060-04-attendance-4a-punch-ledger.md`（punches UI 可用）。

## 1. 背景

`DEV-PLAN-056` 将钉钉/企微事件摄入收敛为：外部身份映射（provider + external_user_id → person_uuid）+ worker 规范化 + 调用 kernel 写入 punch 事件，最终在 `/org/attendance-*` 页面同口径可见。本子计划验证：
- `/org/attendance-integrations` 映射管理页是否可见可操作；
- 外部事件进入后是否在 punches/daily results 可见；
- Authz/RLS 是否按契约可拒绝/fail-closed。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] `/org/attendance-integrations` 页面可发现、可操作（创建/禁用/状态可见，含 `last_seen_at/seen_count`）。
- [ ] 外部事件进入后，在 `/org/attendance-punches` 与 `/org/attendance-daily-results` 可见且与手工事件同口径。
- [ ] 未授权访问 integrations 必须 403；缺少 tenant context 必须 fail-closed（证据可复现）。

### 2.2 非目标

- 不覆盖“对外回写平台回执/对账闭环”（如需另立计划）。

## 3. 契约引用（SSOT）

- 4F：`docs/dev-plans/056-attendance-slice-4f-dingtalk-wecom-integration.md`
- Person identity：`docs/dev-plans/027-person-minimal-identity-for-staffing.md`
- RLS/Authz：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- E08/E09（Person 已存在，记录 `person_uuid`）：
  - E08：DINGTALK `userId`（示例值即可）
  - E09：WECOM `userid`（示例值即可）

## 5. 测试步骤（执行时勾选）

1. [ ] 打开 integrations 页面：`/org/attendance-integrations?as_of=2026-01-02`（确认页面可见）。
2. [ ] 建立映射：
   - E08：`provider=DINGTALK` + `external_user_id=<...>` → `person_uuid=<E08_UUID>`，状态为 `active`。
   - E09：`provider=WECOM` + `external_user_id=<...>`：覆盖 `pending → active → disabled` 的状态流转（按 UI 操作）。
3. [ ] 外部摄入（任选其一，按环境能力选定并记录）：
   - 运行 worker（`cmd/attendance-integrations` 或等效进程）；或
   - 通过测试桩/模拟接口注入一条外部 punch（产生 `punch_type=RAW`）。
4. [ ] 断言可见性：
   - `/org/attendance-punches?as_of=...` 可见外部来源的 punch；
   - `/org/attendance-daily-results?as_of=...&work_date=...` 可见对应日结果（口径与手工一致）。
5. [ ] Authz 拒绝（可选）：以只读用户访问 `/org/attendance-integrations` 的 POST 必须 403。

## 6. 验收证据（最小）

- integrations 页面证据（映射状态、`last_seen_at/seen_count`）。
- 外部事件进入 punches/daily results 的证据（至少 1 条）。
- 403 证据（若执行）。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

