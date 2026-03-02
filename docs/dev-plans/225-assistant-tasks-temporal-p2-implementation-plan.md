# DEV-PLAN-225：Assistant Tasks API 与 Temporal（P2）详细设计

**状态**: 规划中（2026-03-02 07:02 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
  - `docs/dev-plans/211-blueprint-temporal-m10d0-minimal-orchestration-foundation.md`
  - `docs/dev-plans/212-blueprint-eval-gates-and-triggered-temporal-productionization.md`
- **当前痛点**:
  1. P2 关键能力尚未落地：`/internal/assistant/tasks*`、任务状态机、Temporal workflow/activity。
  2. 长耗时助手流程缺少异步编排入口，失败恢复与可观察性不足。
  3. 缺少 `manual_takeover_required/dead-letter` 等受控失败路径。
- **业务价值**:
  - 提供“可追踪、可取消、可恢复”的异步执行能力，同时保持业务裁决边界不变。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
1. [ ] 落地 Tasks API：提交、查询、取消。
2. [ ] 落地任务状态机：`queued/running/succeeded/failed/manual_takeover_required/canceled`。
3. [ ] 落地 Temporal workflow/activity（含 timeout/retry/checkpoint/dead-letter）。
4. [ ] 落地观测字段：`task_id/workflow_id/attempt/error_code/request_id/trace_id`。
5. [ ] 确保异步失败不绕过 `confirm/commit/re-auth/One Door`。
6. [ ] 明确并冻结：本阶段仍为**一次确认**，不引入双人确认审批流。

### 2.2 非目标 (Out of Scope)
1. [ ] 不改变既有业务裁决边界（Temporal 只编排，不裁决）。
2. [ ] 不引入与业务写库耦合的 Temporal 存储账号。
3. [ ] 不把任务结果直接写入业务事实表（必须经既有提交链路）。
4. [ ] 不实现双人确认审批待办（后续接入审批流时再单列计划扩展）。

## 2.1 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [X] Go 代码
  - [ ] `.templ` / Tailwind
  - [ ] 多语言 JSON
  - [X] Authz
  - [X] 路由治理
  - [X] DB 迁移 / Schema（若新增任务元数据表）
  - [X] sqlc（若新增查询契约）
  - [X] 文档门禁
- **SSOT 引用**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - `docs/dev-plans/025-sqlc-guidelines.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 架构图 (Mermaid)
```mermaid
graph TD
    A[POST /internal/assistant/tasks] --> B[Task Service]
    B --> C[(task_store)]
    B --> D[Temporal Client]
    D --> E[AssistantTaskWorkflow]
    E --> F[Activity: Plan/Validate/Sync]
    E --> G[Activity: Checkpoint Persist]
    H[GET /tasks/{id}] --> B
    I[POST /tasks/{id}:cancel] --> B
    E --> J[(dead-letter / takeover marker)]
```

### 3.2 关键设计决策 (ADR 摘要)
- **决策 1：任务状态机显式化（选定）**
  - 选项 A：直接暴露 workflow 原始状态。缺点：业务语义弱。
  - 选项 B（选定）：统一任务状态 DTO，屏蔽 Temporal 内部细节。
- **决策 2：失败进入可操作终态（选定）**
  - 选项 A：统一 failed。缺点：人工接管信号弱。
  - 选项 B（选定）：可判定场景进入 `manual_takeover_required`。
- **决策 3：编排与业务提交隔离（选定）**
  - 选项 A：workflow 直接业务提交。缺点：破坏 One Door。
  - 选项 B（选定）：workflow 仅编排，不裁决提交。

## 4. 数据模型与约束 (Data Model & Constraints)
> 若涉及新增表（`CREATE TABLE`），必须先获得用户确认后执行迁移。

### 4.1 任务元数据模型（草案）
```sql
-- assistant_tasks
task_id uuid pk
tenant_id uuid not null
conversation_id uuid
turn_id uuid
workflow_id text not null
status text not null
attempt int not null default 0
max_attempts int not null
last_error_code text
request_id text not null
trace_id text
created_at timestamptz not null
updated_at timestamptz not null

-- assistant_task_events
id bigserial pk
tenant_id uuid not null
task_id uuid not null
from_status text
to_status text not null
event_type text not null
error_code text
occurred_at timestamptz not null
```

### 4.2 约束与索引
1. [ ] `assistant_tasks(tenant_id, task_id)` 唯一。
2. [ ] `assistant_tasks(tenant_id, workflow_id)` 唯一。
3. [ ] `assistant_tasks(tenant_id, status, updated_at)` 索引。
4. [ ] `assistant_task_events(tenant_id, task_id, occurred_at)` 索引。
5. [ ] 状态转移仅允许白名单路径。

### 4.3 迁移策略
1. [ ] 用户确认新增表后，执行模块级 `plan/lint/migrate up`。
2. [ ] 若命中 sqlc，执行 `make sqlc-generate` 并检查生成物漂移。
3. [ ] 迁移失败按 No-Legacy 处置：停写 -> 修复 -> 重试。

## 5. 接口契约 (API Contracts)
### 5.1 `POST /internal/assistant/tasks`
- **Request**:
  ```json
  {
    "conversation_id": "uuid",
    "turn_id": "uuid",
    "task_type": "assistant_async_plan",
    "request_id": "string",
    "trace_id": "string"
  }
  ```
- **Response (202)**:
  ```json
  {
    "task_id": "uuid",
    "status": "queued",
    "workflow_id": "string"
  }
  ```

### 5.2 `GET /internal/assistant/tasks/{task_id}`
- **Response (200)**:
  ```json
  {
    "task_id": "uuid",
    "status": "running",
    "attempt": 1,
    "last_error_code": "",
    "updated_at": "2026-03-02T06:00:00Z"
  }
  ```

### 5.3 `POST /internal/assistant/tasks/{task_id}:cancel`
- **Response (202)**：返回 `canceled` 或“已是终态”的幂等响应。

### 5.4 错误码契约
1. [ ] `assistant_task_not_found`
2. [ ] `assistant_task_state_invalid`
3. [ ] `assistant_task_cancel_not_allowed`
4. [ ] `assistant_task_workflow_unavailable`

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 任务提交算法（伪代码）
```text
begin tx
validate tenant + conversation/turn visibility
insert task(status=queued)
append task_event(queued)
commit
start workflow(task_id)
```

### 6.2 workflow 执行算法（伪代码）
```text
set status=running
for attempt in [1..max_attempts]:
  run activities with timeout
  if success: set succeeded; break
  if retryable: checkpoint + retry
  if non_retryable: set failed or manual_takeover_required; break
if exceeded retries: set manual_takeover_required and dead-letter marker
```

### 6.3 取消算法（伪代码）
```text
if status in terminal: return idempotent
request workflow cancel
set status=canceled
append task_event(canceled)
```

## 7. 安全与鉴权 (Security & Authz)
1. [ ] tasks API 必须经过 capability 授权与租户隔离校验。
2. [ ] 查询与取消均要求 `tenant_id` 边界一致，fail-closed。
3. [ ] workflow 输入不包含敏感明文密钥。
4. [ ] 不允许通过异步链路绕过 re-auth/confirm/commit。
5. [ ] 双人确认审批流未启用时，保持一次确认策略不变。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：
  - `DEV-PLAN-223` 持久化基础（conversation/turn 可关联）。
  - `DEV-PLAN-224` 意图链路可稳定产出可编排输入（若联动）。
- **里程碑**：
  1. [ ] M1：任务状态 DTO 与 API 契约冻结。
  2. [ ] M2：tasks store + API handler 落地。
  3. [ ] M3：Temporal workflow/activity + timeout/retry/dead-letter。
  4. [ ] M4：路由/capability/authz 收口与测试证据完成。

## 9. 测试与验收标准 (Acceptance Criteria)
- **单元测试**：
  1. [ ] 状态机合法/非法转移。
  2. [ ] 取消幂等、终态拒绝。
  3. [ ] retryable / non-retryable 错误分流。
- **集成测试**：
  1. [ ] workflow 超时与重试行为符合上限。
  2. [ ] dead-letter 与 manual takeover 标记可查询。
- **验收对齐**：
  1. [ ] 对齐 `TC-220-TMP-001~006`。
  2. [ ] 对齐 `TC-220-BE-012`（tasks 路由映射完整性）。
  3. [ ] 明确一次确认策略不变，未引入双人确认待办。
  4. [ ] `make preflight` 全绿。

## 10. 运维与监控 (Ops & Monitoring)
- 不引入复杂运维开关；遵循早期最小运维原则。
- 最小观测要求：
  1. [ ] 日志字段：`task_id/workflow_id/attempt/error_code/request_id/trace_id`。
  2. [ ] 任务指标：queued/running/failed/manual_takeover_required 的数量与时延。
  3. [ ] 故障处置：环境级保护 → 停写/限流 → 修复 → 重试/重放 → 恢复。

## 11. 交付物
1. [ ] tasks API + Temporal 编排实现与测试。
2. [ ] route/capability/authz 映射更新与门禁证据。
3. [ ] `DEV-PLAN-225` 执行记录文档（新增到 `docs/dev-records/`）。

## 12. 关联文档
- `docs/dev-plans/001-technical-design-template.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/211-blueprint-temporal-m10d0-minimal-orchestration-foundation.md`
- `docs/dev-plans/212-blueprint-eval-gates-and-triggered-temporal-productionization.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `AGENTS.md`
