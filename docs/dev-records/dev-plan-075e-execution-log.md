# DEV-PLAN-075E 执行日志

## 基本信息
- 计划文档：`docs/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
- 完成日期：2026-02-08
- 当前状态：已完成

## 阶段交付

### P0 契约冻结
- PR #307（已合并）
- 冻结同日状态纠错契约：函数命名、API 路径、错误码与 fail-closed 边界。

### P1 Kernel + Service + API
- PR #307（已合并）
- PR #308（已合并，补齐迁移闭环）
- 交付内容：
  - DB Kernel：新增 `orgunit.submit_org_status_correction(...)`，并在 `org_events_effective` 投影 `CORRECT_STATUS -> ENABLE/DISABLE`。
  - Service：新增 `CorrectStatus(...)` 并接入参数校验、request_id 幂等透传。
  - API：新增 `POST /org/api/org-units/status-corrections`，错误码映射稳定。
  - Persistence：写路径统一通过 One Door，不引入 legacy 回退链路。

### P2 UI + 测试 + 回归
- PR #307（已合并）
- 交付内容：
  - UI 新增“同日状态纠错”勾选入口，并与“状态变更”语义区分。
  - 同日冲突文案引导用户切换纠错路径，不再提示改日期兜底。
  - 回归覆盖：Handler/API/Service/节点交互，403/409 行为保持一致。

## 本次故障修复（42883）
- 现象：`function orgunit.submit_org_status_correction(...) does not exist (SQLSTATE 42883)`。
- 根因：075E 代码已调用新函数，但运行库迁移目录缺失对应 migration。
- 修复：新增 `migrations/orgunit/20260208151000_orgunit_status_correction.sql` 并更新 `migrations/orgunit/atlas.sum`。

## 门禁与验证记录
- 本地执行：
  - `make orgunit migrate up`
  - `make orgunit lint`
  - `go test ./modules/orgunit/... ./internal/server/...`
- SQL smoke（事务内回滚）：
  - 手动调用 `submit_org_status_correction` 返回 `correction_uuid`，确认不再出现 42883。
- CI 结果（PR #308）：
  - Code Quality & Formatting：pass
  - Unit & Integration Tests：pass
  - Routing Gates：pass
  - E2E Tests：pass

## 验收结论
- 075E 验收标准满足：
  - 支持同日状态纠错（生效日不变）；
  - 保持同日唯一约束，不引入同日多事件；
  - replay 后状态与纠错目标一致；
  - 错误语义稳定，不向用户暴露 SQLSTATE。
