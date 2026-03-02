# DEV-PLAN-225：Assistant Tasks API 与 Temporal（P2）实施计划

**状态**: 规划中（2026-03-02 05:37 UTC）

## 1. 背景
- `DEV-PLAN-220` 将 P2 定位为“异步编排加速器”，不承载授权裁决与业务提交裁决。
- `DEV-PLAN-220A` 识别 P2 目前几乎未落地：`/internal/assistant/tasks*`、任务状态机、Temporal workflow/activity、TMP 测试均缺失。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 落地 Tasks API：提交、查询、取消。
2. [ ] 落地最小任务状态机：`queued/running/succeeded/failed/manual_takeover_required/canceled`。
3. [ ] 落地 Temporal workflow/activity 与 checkpoint/retry/dead-letter。
4. [ ] 确保异步失败不绕过 `confirm/commit/re-auth/One Door`。

### 2.2 非目标
1. [ ] 不改变既有业务裁决边界。
2. [ ] 不引入与业务写库耦合的 Temporal 存储账号。

## 3. 实施范围
- API：`POST /internal/assistant/tasks`、`GET /internal/assistant/tasks/{task_id}`、`POST /internal/assistant/tasks/{task_id}:cancel`。
- 路由治理：allowlist + capability-route-map + authz 映射 + 对应测试。
- 编排层：workflow/activity、超时、重试、死信、人工接管标记。

## 4. 实施步骤
1. [ ] 设计并冻结 `AsyncTaskReceipt` 与任务状态 DTO。
2. [ ] 实现 tasks store 与任务生命周期管理。
3. [ ] 实现 Temporal workflow/activity（仅编排，不裁决）。
4. [ ] 实现 timeout/retry/dead-letter/manual takeover 机制。
5. [ ] 接入路由与 capability 门禁，补齐映射测试。
6. [ ] 接入观测字段：`task_id/workflow_id/attempt/error_code/request_id/trace_id`。

## 5. 测试与验收
1. [ ] 对齐 `TC-220-TMP-001~006` 自动化测试。
2. [ ] 对齐 `TC-220-BE-012`（tasks 路由映射完整性）测试。
3. [ ] 验收标准：任务可观察、可取消、可恢复；失败路径 fail-closed。

## 6. 风险与缓解
- **任务悬挂风险**：设置超时与最大重试，并落死信。
- **编排与业务边界混淆**：在代码分层与评审清单中强制“编排不裁决”。
- **观测盲区**：任务全链路字段统一入日志与查询接口。

## 7. 交付物
1. [ ] tasks API + Temporal 编排实现与测试。
2. [ ] route/capability/authz 映射更新与门禁证据。
3. [ ] `DEV-PLAN-225` 执行记录文档（实施时新增到 `docs/dev-records/`）。

## 8. 关联文档
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/211-blueprint-temporal-m10d0-minimal-orchestration-foundation.md`
- `docs/dev-plans/212-blueprint-eval-gates-and-triggered-temporal-productionization.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `AGENTS.md`
