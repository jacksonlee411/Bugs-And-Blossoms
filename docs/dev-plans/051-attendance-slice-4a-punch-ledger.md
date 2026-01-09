# DEV-PLAN-051：考勤 Slice 4A——打卡流水闭环（手工/导入，先可见）

**状态**: 草拟中（2026-01-09 02:17 UTC）

## 1. 背景与上下文

- 本计划是 `DEV-PLAN-050` 的 Slice 4A 落地：先让“打卡流水”可写、可看、可在 UI 中演示，作为后续日结果/合规/额度/纠错/外部集成的输入底座。
- 路线图颗粒度对齐 `DEV-PLAN-009` Phase 4（业务垂直切片：业务 + UI 同步交付）。
- 模块落点：优先在 `modules/staffing` 内作为子域实现（避免过早新增模块；对齐 `DEV-PLAN-015/016`）。

## 2. 目标与非目标

### 2.1 目标（Done 的定义）

- [ ] **One Door**：提供唯一写入口（DB Kernel，`submit_*_event(...)` 形态），写入打卡事件并在同一事务内同步投射出可查询读模。
- [ ] **No Tx, No RLS**：相关表启用 `ENABLE + FORCE`；Go 侧所有读写路径必须显式事务 + 事务内注入 `app.current_tenant`，缺失注入必须 fail-closed（对齐 `DEV-PLAN-021`）。
- [ ] **UI 可见**：在 AHA Shell 中提供“考勤/打卡流水”页面，可按人员 + 日期范围查询，并支持手工补打卡（IN/OUT）作为端到端验收入口。
- [ ] **可测**：至少一条 DB/集成测试覆盖 fail-closed、跨租户隔离、幂等/冲突行为（入口以 SSOT 门禁执行）。

### 2.2 非目标（本计划不做）

- 不做日结果（出勤/缺勤/异常）计算（见 `DEV-PLAN-052`）。
- 不引入 TimeProfile/Shift/HolidayCalendar（见 `DEV-PLAN-053`）。
- 不接入钉钉/企微（见 `DEV-PLAN-056`）。
- 不引入异步队列写权威读模；不引入 Sidecar/物化视图刷新链路（对齐 `AGENTS.md` §3.6）。

## 3. 关键设计决策（草案）

- **事件 SoT 的写形态**：打卡事件作为 append-only ledger（仅 INSERT），不在事件表上维护“是否已处理”等处理标记（对齐 `DEV-PLAN-050` §0.3）。
- **幂等策略**：
  - 应用侧请求幂等：使用 `request_id`（同租户唯一）+ `event_id`。
  - 外部来源幂等键（`source_provider + source_event_id`）作为 `DEV-PLAN-056` 的一部分落地；可能需要独立的去重表以规避“分区表 unique 约束必须包含分区键”的限制。

## 4. 数据模型（草案；落地迁移前需手工确认）

> 红线：新增数据库表/迁移落地前必须获得你手工确认。

- 候选表（命名最终以迁移为 SSOT，schema 建议在 `staffing.*`）：
  - `time_punch_events`：打卡事件 SoT（按 `punch_time` 分区）；包含 `tenant_id/person_uuid/punch_time/punch_type/source_provider/source_payload/device_info/request_id/initiator_id/transaction_time` 等字段。
  - `time_punch_versions`（可选）：为 UI 查询提供稳定索引与派生字段（若直接查 events 性能足够可延后）。
- 候选 Kernel 写入口：
  - `staffing.submit_time_punch_event(...)`（命名待定）：同事务写入事件并同步投射读模。

## 5. 工具链与门禁（SSOT 引用）

### 5.1 触发器（勾选本计划命中的项）

- [ ] Go 代码
- [ ] DB 迁移 / Schema（Atlas+Goose）
- [ ] sqlc
- [ ] 路由治理（Routing）
- [ ] Authz（Casbin）
- [ ] `.templ` / Tailwind（如新增/调整 UI 资源）

### 5.2 SSOT 链接

- 触发器矩阵与本地必跑：`AGENTS.md`
- CI 门禁定义：`docs/dev-plans/012-ci-quality-gates.md`
- RLS 契约：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- 分层与模块边界：`docs/dev-plans/015-ddd-layering-framework.md`、`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- 路由策略：`docs/dev-plans/017-routing-strategy.md`
- Authz 工具链：`docs/dev-plans/022-authz-casbin-toolchain.md`
- Atlas+Goose 闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`

## 6. 实施步骤

1. [ ] 明确路由命名空间与 `route_class`（对齐 `DEV-PLAN-017`），必要时补齐 allowlist。
2. [ ] 设计并评审数据模型（含分区/索引/幂等约束）；在落迁移前获得手工确认。
3. [ ] 落地 migrations（`migrations/staffing/*`），并按 `DEV-PLAN-024` 走 plan/lint/migrate 闭环；启用 RLS（ENABLE+FORCE）与 policy（fail-closed）。
4. [ ] 实现 Kernel 写入口 `submit_*_event(...)` 与同步投射（不引入第二写入口）。
5. [ ] 用 sqlc 定义读写 queries，生成代码并提交生成物（对齐 `DEV-PLAN-025`）。
6. [ ] 实现 Go 服务层与 UI（列表 + 手工补打卡），确保用户可操作闭环（对齐“用户可见性原则”）。
7. [ ] 补齐测试：RLS fail-closed、跨租户隔离、request_id 幂等/冲突行为。
8. [ ] 运行命中触发器对应的门禁并记录结果（必要时补 `docs/dev-records/` 证据）。

## 7. 验收标准

- [ ] 页面可见：能在 UI 中完成“手工补打卡 → 列表可见”闭环。
- [ ] 写入口唯一：不存在绕过 `submit_*_event(...)` 的写路径。
- [ ] RLS 正确：缺失 tenant 注入 fail-closed；tenant A 不可见 tenant B。
- [ ] 生成物与门禁：命中触发器对应的 checks 全部通过，生成物无漂移。

## 8. 开放问题

- [ ] 打卡事件的外部幂等键：在分区表上实现全局唯一的策略（是否需要独立去重表）。
- [ ] 打卡类型枚举：本切片是否只收敛为 `IN/OUT`（建议先最小集合）。

