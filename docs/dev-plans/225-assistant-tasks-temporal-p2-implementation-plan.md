# DEV-PLAN-225：Assistant Tasks API 与 Temporal（P2）详细设计（修订版）

**状态**: 规划中（2026-03-02 18:55 UTC，基于评审补充幂等/可靠派发/224 契约继承）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
  - `docs/dev-plans/211-blueprint-temporal-m10d0-minimal-orchestration-foundation.md`
  - `docs/dev-plans/212-blueprint-eval-gates-and-triggered-temporal-productionization.md`
  - `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- **前置事实（本修订冻结）**:
  1. `DEV-PLAN-224` 已按目标实施，已具备 `intent_schema_version/compiler_contract_version/capability_map_version/skill_manifest_digest/context_hash/intent_hash/plan_hash` 产出能力。
  2. `DEV-PLAN-223` 已提供会话与回合持久化、幂等与审计基础。
- **当前痛点**:
  1. P2 关键能力尚未落地：`/internal/assistant/tasks*`、任务状态机、Temporal workflow/activity。
  2. 长耗时助手流程缺少异步编排入口，失败恢复与可观察性不足。
  3. 缺少“任务提交幂等主键 + 可靠派发”定义，存在重复任务与 queued 僵尸任务风险。
  4. 缺少 224 契约快照继承与执行前漂移校验，异步重试稳定性无法闭环。
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
7. [ ] 与 224 对齐：异步任务必须继承并校验版本快照与哈希（fail-closed）。

### 2.2 非目标 (Out of Scope)
1. [ ] 不改变既有业务裁决边界（Temporal 只编排，不裁决）。
2. [ ] 不引入与业务写库耦合的 Temporal 存储账号。
3. [ ] 不把任务结果直接写入业务事实表（必须经既有提交链路）。
4. [ ] 不实现双人确认审批待办（后续接入审批流时再单列计划扩展）。

## 2.3 工具链与门禁（SSOT 引用）
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
    B --> C[(assistant_tasks)]
    B --> D[(assistant_task_events)]
    B --> O[(assistant_task_dispatch_outbox)]
    O --> P[Dispatcher Worker]
    P --> T[Temporal Client]
    T --> E[AssistantTaskWorkflow]
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
- **决策 4：任务提交幂等键冻结（选定）**
  - 选项 A：仅以 `task_id` 标识请求。缺点：重试会重复创建任务。
  - 选项 B（选定）：冻结 `(tenant_id, conversation_id, turn_id, request_id)`，并校验 `request_hash` 防同键异载荷。
- **决策 5：可靠派发采用 outbox（选定）**
  - 选项 A：事务提交后立即调用 Temporal。缺点：Temporal 短故障会产生 queued 僵尸任务。
  - 选项 B（选定）：`task + dispatch_outbox` 同事务落盘，独立派发器重试，超限转 `manual_takeover_required`。
- **决策 6：224 契约快照不可变继承（选定）**
  - 选项 A：任务执行时读取“当前配置”。缺点：异步重试期间漂移导致不可复现。
  - 选项 B（选定）：任务创建时固化版本与哈希，workflow 启动前强校验，不一致 fail-closed。

## 4. 数据模型与约束 (Data Model & Constraints)
> 若涉及新增表（`CREATE TABLE`），必须先获得用户确认后执行迁移。

### 4.1 任务元数据模型（修订草案）
```sql
-- assistant_tasks
task_id uuid pk
tenant_id uuid not null
conversation_id uuid not null
turn_id uuid not null
task_type text not null
request_id text not null
request_hash text not null
workflow_id text not null
status text not null
dispatch_status text not null default 'pending'
dispatch_attempt int not null default 0
dispatch_deadline_at timestamptz not null
attempt int not null default 0
max_attempts int not null
last_error_code text
trace_id text

-- inherited from DEV-PLAN-224 (immutable snapshot)
intent_schema_version text not null
compiler_contract_version text not null
capability_map_version text not null
skill_manifest_digest text not null
context_hash text not null
intent_hash text not null
plan_hash text not null

submitted_at timestamptz not null
cancel_requested_at timestamptz
completed_at timestamptz
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
payload jsonb
occurred_at timestamptz not null

-- assistant_task_dispatch_outbox
id bigserial pk
tenant_id uuid not null
task_id uuid not null
workflow_id text not null
status text not null default 'pending'
attempt int not null default 0
next_retry_at timestamptz not null
created_at timestamptz not null
updated_at timestamptz not null
```

### 4.2 约束与索引
1. [ ] `assistant_tasks(tenant_id, task_id)` 唯一。
2. [ ] `assistant_tasks(tenant_id, workflow_id)` 唯一。
3. [ ] `assistant_tasks(tenant_id, conversation_id, turn_id, request_id)` 唯一（冻结任务提交幂等键）。
4. [ ] `assistant_tasks(tenant_id, conversation_id, turn_id)` 复合外键引用 `assistant_turns`（对齐 `DEV-PLAN-223`）。
5. [ ] `assistant_tasks.status` 仅允许 `queued/running/succeeded/failed/manual_takeover_required/canceled`。
6. [ ] `assistant_tasks.dispatch_status` 仅允许 `pending/started/failed`。
7. [ ] `assistant_tasks(tenant_id, status, updated_at)` 索引。
8. [ ] `assistant_tasks(tenant_id, dispatch_status, dispatch_deadline_at)` 索引（僵尸任务清扫）。
9. [ ] `assistant_task_events(tenant_id, task_id, occurred_at)` 索引。
10. [ ] `assistant_task_dispatch_outbox(status, next_retry_at)` 索引。
11. [ ] 状态转移仅允许白名单路径（含取消竞态 CAS 守护）。

### 4.3 状态机与转移白名单（修订）
1. [ ] 允许：`queued -> running/succeeded/failed/manual_takeover_required/canceled`。
2. [ ] 允许：`running -> succeeded/failed/manual_takeover_required/canceled`。
3. [ ] 允许：`manual_takeover_required -> running/canceled`（人工接管后重跑同 `request_id`）。
4. [ ] 终态（`succeeded/failed/canceled`）禁止非人工显式操作下的隐式回退。
5. [ ] 取消写入采用 compare-and-swap：仅当当前状态仍可取消时落 `canceled`。

### 4.4 迁移策略
1. [ ] 用户确认新增表后，执行模块级 `plan/lint/migrate up`。
2. [ ] 若命中 sqlc，执行 `make sqlc-generate` 并检查生成物漂移。
3. [ ] 迁移失败按 No-Legacy 处置：停写 -> 修复 -> 重试。
4. [ ] 若先上线任务表后上线 outbox，需提供一次性 backfill（仅把 `dispatch_status='pending'` 且未开始的任务补入 outbox）。

### 4.5 数据保留与清理策略（新增）
1. [ ] `assistant_task_dispatch_outbox`：`status in (started, failed)` 且 `updated_at < now()-30d` 可归档/清理。
2. [ ] `assistant_task_events`：默认保留 180 天；超期仅在审计策略允许时归档，不做硬删除直出。
3. [ ] `assistant_tasks`：终态保留 180 天（至少覆盖问题追溯窗口），清理前需确保 `docs/dev-records/` 已固化关键证据。
4. [ ] 清理任务不得影响 223 会话审计链路；禁止清理后出现“会话可见但任务消失且无证据索引”。

## 5. 接口契约 (API Contracts)
### 5.1 `POST /internal/assistant/tasks`
- **Request**:
  ```json
  {
    "conversation_id": "uuid",
    "turn_id": "uuid",
    "task_type": "assistant_async_plan",
    "request_id": "string",
    "trace_id": "string",
    "contract_snapshot": {
      "intent_schema_version": "string",
      "compiler_contract_version": "string",
      "capability_map_version": "string",
      "skill_manifest_digest": "string",
      "context_hash": "string",
      "intent_hash": "string",
      "plan_hash": "string"
    }
  }
  ```
- **Response (202, AsyncTaskReceipt)**:
  ```json
  {
    "task_id": "uuid",
    "task_type": "assistant_async_plan",
    "status": "queued",
    "workflow_id": "string",
    "submitted_at": "2026-03-02T06:00:00Z",
    "poll_uri": "/internal/assistant/tasks/{task_id}"
  }
  ```
- **幂等语义**:
  1. [ ] 同 `(tenant, conversation, turn, request_id)` + 同 `request_hash`：返回同一 `AsyncTaskReceipt`。
  2. [ ] 同键异载荷：返回 `idempotency_key_conflict`（409）。

### 5.2 `GET /internal/assistant/tasks/{task_id}`
- **Response (200)**:
  ```json
  {
    "task_id": "uuid",
    "task_type": "assistant_async_plan",
    "status": "running",
    "dispatch_status": "started",
    "attempt": 1,
    "max_attempts": 3,
    "last_error_code": "",
    "workflow_id": "string",
    "updated_at": "2026-03-02T06:00:00Z",
    "contract_snapshot": {
      "intent_schema_version": "string",
      "compiler_contract_version": "string",
      "capability_map_version": "string",
      "skill_manifest_digest": "string",
      "context_hash": "string",
      "intent_hash": "string",
      "plan_hash": "string"
    }
  }
  ```

### 5.3 `POST /internal/assistant/tasks/{task_id}:cancel`
- **Response (202)**：返回最新任务状态与“是否已受理取消”标志；终态保持幂等。
- **行为约束**:
  1. [ ] 若 `dispatch_status=pending` 且 workflow 尚未启动，可直接转 `canceled`。
  2. [ ] 若 workflow 已启动，先发送 cancel signal，收到 workflow canceled 事件后再 CAS 写 `canceled`。
  3. [ ] 若并发下 workflow 已先进入终态，保持原终态，不做回写覆盖。

### 5.4 错误码契约（修订）
1. [ ] `assistant_task_not_found`
2. [ ] `assistant_task_state_invalid`
3. [ ] `assistant_task_cancel_not_allowed`
4. [ ] `assistant_task_workflow_unavailable`
5. [ ] `assistant_task_dispatch_failed`
6. [ ] `idempotency_key_conflict`
7. [ ] `request_in_progress`
8. [ ] `ai_plan_contract_version_mismatch`
9. [ ] `ai_plan_determinism_violation`

### 5.5 状态码与响应头语义（新增）
1. [ ] `POST /tasks`：
   - [ ] 新建成功返回 202；
   - [ ] 幂等重放返回 202（同 `task_id`）；
   - [ ] 同键异载荷返回 409 + `idempotency_key_conflict`。
2. [ ] `GET /tasks/{task_id}`：
   - [ ] 找不到返回 404 + `assistant_task_not_found`；
   - [ ] 租户越界返回 403（fail-closed）。
3. [ ] `POST /tasks/{task_id}:cancel`：
   - [ ] 可取消返回 202；
   - [ ] 不可取消返回 409 + `assistant_task_cancel_not_allowed`。
4. [ ] `request_in_progress` 场景必须返回 `Retry-After`（默认 1s，可调但不得缺失）。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 任务提交与可靠派发算法（修订伪代码）
```text
canonical_payload = canonical_json(request without trace-only fields)
request_hash = sha256(task_type + canonical_payload)
workflow_id = deterministic("assistant_async_orchestration_v1", tenant, conversation, turn, request_id)

begin tx
validate tenant + conversation/turn visibility
validate contract_snapshot fields from DEV-PLAN-224 are all present

row = select task by (tenant, conversation, turn, request_id) for update
if row exists:
  if row.request_hash != request_hash: return 409 idempotency_key_conflict
  return existing AsyncTaskReceipt (idempotent replay)

insert assistant_tasks(status=queued, dispatch_status=pending, workflow_id, request_hash, snapshots...)
insert assistant_task_events(event=queued)
insert assistant_task_dispatch_outbox(status=pending, next_retry_at=now)
commit

dispatcher loop:
  fetch pending outbox row for update skip locked
  start workflow(workflow_id, task_id)
  if success:
    mark outbox started
    update task.dispatch_status=started
  else:
    exponential backoff and retry
    if dispatch retry budget exhausted or now > dispatch_deadline_at:
      mark outbox failed
      update task.status=manual_takeover_required, dispatch_status=failed, last_error_code=assistant_task_dispatch_failed
      append event(dead-letter)
```

### 6.2 workflow 执行算法（修订伪代码）
```text
load task snapshot
verify snapshot versions/hashes against current execution context
if mismatch:
  set manual_takeover_required + ai_plan_contract_version_mismatch
  append dead-letter marker
  return

CAS set status queued->running
for attempt in [1..max_attempts]:
  run activities with timeout
  if success: set succeeded; set completed_at; break
  if retryable: checkpoint + retry
  if determinism violation: set manual_takeover_required + ai_plan_determinism_violation; break
  if non_retryable: set failed or manual_takeover_required; break
if exceeded retries: set manual_takeover_required and dead-letter marker
```

### 6.3 取消算法（修订伪代码）
```text
begin tx
task = select for update
if status in terminal: return idempotent
if dispatch_status == pending:
  set status=canceled, completed_at=now
  append event(canceled)
  mark outbox canceled/noop
  commit and return
set cancel_requested_at=now
append event(cancel_requested)
commit

signal workflow cancel
on workflow canceled callback:
  update task set status=canceled where status in (queued, running)
  append event(canceled)
```

### 6.4 标识生成规则（新增）
1. [ ] `workflow_id` 必须可重算且可审计，建议格式：`assistant_async_orchestration_v1:{tenant}:{conversation}:{turn}:{request}`。
2. [ ] `task_id` 使用 UUIDv7（若工具链未就绪可临时 UUIDv4，但需在执行记录说明）。
3. [ ] `request_hash` 口径冻结：`sha256(task_type + canonical_json(payload_without_trace_only_fields))`。
4. [ ] `event_type` 命名冻结：`queued/running/succeeded/failed/manual_takeover_required/cancel_requested/canceled/dead_lettered`。

## 7. 安全与鉴权 (Security & Authz)
1. [ ] tasks API 必须经过 capability 授权与租户隔离校验。
2. [ ] 查询与取消均要求 `tenant_id` 边界一致，fail-closed。
3. [ ] workflow 输入不包含敏感明文密钥。
4. [ ] Worker 不注入业务库写凭据，不允许直接调用业务写入口。
5. [ ] 异步链路仅可访问 `internal/assistant/*` 编排接口，不得绕过 re-auth/confirm/commit/One Door。
6. [ ] `contract_snapshot` 一经创建不可变更，防止重试期间静默漂移。
7. [ ] 双人确认审批流未启用时，保持一次确认策略不变。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：
  - `DEV-PLAN-223` 持久化基础（conversation/turn 可关联、幂等语义可复用）。
  - `DEV-PLAN-224` 已完成并稳定产出可编排输入 + 版本/哈希快照。
  - `DEV-PLAN-220` P2 Temporal 边界与 TC-220-TMP-001~006 冻结。
- **里程碑**：
  1. [ ] M1：任务幂等键、`AsyncTaskReceipt`、`contract_snapshot` 契约冻结。
  2. [ ] M2：tasks store + outbox 可靠派发 + API handler 落地。
  3. [ ] M3：Temporal workflow/activity + timeout/retry/dead-letter + 取消竞态收口。
  4. [ ] M4：路由/capability/authz 收口，联调 224 版本漂移拦截。
  5. [ ] M5：测试证据与运维观测指标归档（`docs/dev-records/`）。

## 9. 测试与验收标准 (Acceptance Criteria)
- **单元测试**：
  1. [ ] 状态机合法/非法转移。
  2. [ ] 提交幂等：同键同载荷返回同收据；同键异载荷 `idempotency_key_conflict`。
  3. [ ] 取消幂等、终态拒绝、取消竞态 CAS 正确性。
  4. [ ] retryable / non-retryable / determinism violation 分流。
- **集成测试**：
  1. [ ] workflow 超时与重试行为符合上限。
  2. [ ] Temporal 短时不可用时，outbox 重试可恢复；超限后转 `manual_takeover_required`（无 queued 僵尸任务）。
  3. [ ] dead-letter 与 manual takeover 标记可查询。
  4. [ ] 224 版本/哈希漂移命中时 fail-closed（`ai_plan_contract_version_mismatch`）。
  5. [ ] `TC-220-TMP-001~006` 全覆盖通过。
- **验收对齐**：
  1. [ ] 对齐 `TC-220-BE-012`（tasks 路由映射完整性）。
  2. [ ] 明确一次确认策略不变，未引入双人确认待办。
  3. [ ] `make check routing`
  4. [ ] `make check capability-route-map`
  5. [ ] `make authz-pack && make authz-test && make authz-lint`
  6. [ ] `make test && make preflight` 全绿。

### 9.1 TC-220-TMP-001~006 映射矩阵（新增）
1. [ ] TMP-001（收据）：断言 `task_id/task_type/submitted_at/status/poll_uri` 全部存在且稳定。
2. [ ] TMP-002（状态转移）：断言 `queued -> running -> succeeded` 有完整事件链与时间顺序。
3. [ ] TMP-003（超时重试）：断言 `attempt` 递增、退避生效、`max_attempts` 不越界。
4. [ ] TMP-004（重试耗尽）：断言终态为 `manual_takeover_required` 且含 `dead_lettered` 事件。
5. [ ] TMP-005（恢复幂等）：断言恢复后无重复任务、无重复提交、副作用计数不增加。
6. [ ] TMP-006（fail-closed）：断言 Temporal 故障下不可绕过 `confirm/commit/re-auth/One Door`。

## 10. 运维与监控 (Ops & Monitoring)
- 不引入复杂运维开关；遵循早期最小运维原则。
- 最小观测要求：
  1. [ ] 日志字段：`task_id/workflow_id/attempt/error_code/request_id/trace_id/conversation_id/turn_id`。
  2. [ ] 任务指标：`queued/running/failed/manual_takeover_required/canceled` 数量与时延。
  3. [ ] 派发指标：`dispatch_pending_count/dispatch_pending_max_age/dispatch_retry_exhausted_count`。
  4. [ ] 契约指标：`contract_version_mismatch_count/determinism_violation_count`。
  5. [ ] 异常处置：环境级保护 → 停写/限流 → 修复 → 重试/重放 → 恢复。

## 11. 交付物
1. [ ] tasks API + Temporal 编排实现与测试。
2. [ ] 任务元数据表 + outbox 迁移 + sqlc 契约与生成物。
3. [ ] route/capability/authz 映射更新与门禁证据。
4. [ ] `DEV-PLAN-225` 执行记录文档（新增到 `docs/dev-records/`）。

## 12. 实施任务拆解与 PR 切片（按目录）
### 12.1 PR-225-01：契约冻结与 API 壳（M1）
1. [ ] 冻结 `AsyncTaskReceipt`、`contract_snapshot`、幂等语义与错误码。
2. [ ] 新增/更新后端 DTO 与 handler 壳（暂不接入真实 workflow）：
   - [ ] `internal/server/assistant_api.go`（路由挂载）
   - [ ] `internal/server/assistant_tasks_api.go`（新文件，tasks submit/get/cancel handler）
   - [ ] `internal/server/assistant_tasks_api_test.go`（新文件，契约与错误码测试）
3. [ ] 本 PR DoD：
   - [ ] OpenAPI/接口文档样例与本计划一致；
   - [ ] `TC-220-TMP-001`（收据字段）可通过 mock 级测试。

### 12.2 PR-225-02：任务存储 + 幂等键 + Outbox（M2）
1. [ ] 新增任务表、事件表、派发表（需用户确认后执行迁移）。
2. [ ] 新增查询与写入仓储（含 `request_hash` 冲突校验、同键重放返回同收据）：
   - [ ] `migrations/iam/*.sql`（新增 225 迁移）
   - [ ] `internal/server/assistant_persistence.go`（扩展或拆分任务持久化）
   - [ ] `internal/server/assistant_task_store.go`（新文件，任务与 outbox 仓储）
   - [ ] `internal/server/assistant_task_store_test.go`（新文件，幂等与并发测试）
3. [ ] 本 PR DoD：
   - [ ] 同键同载荷重试返回同 `task_id`；
   - [ ] 同键异载荷返回 `idempotency_key_conflict`；
   - [ ] 无 Temporal 时不会出现不可追踪任务记录。

### 12.3 PR-225-03：Dispatcher + Temporal Workflow（M3）
1. [ ] 落地 outbox 派发器、workflow/activity、checkpoint/retry/dead-letter。
2. [ ] 新增取消竞态收口（cancel signal + CAS 终态写入）。
3. [ ] 224 契约快照校验接入（version/hash mismatch fail-closed）：
   - [ ] `internal/server/assistant_tasks_dispatcher.go`（新文件）
   - [ ] `internal/server/assistant_tasks_workflow.go`（新文件）
   - [ ] `internal/server/assistant_tasks_cancel.go`（新文件或并入 workflow）
   - [ ] `internal/server/assistant_tasks_workflow_test.go`（新文件）
4. [ ] 本 PR DoD：
   - [ ] Temporal 短时不可用可自动恢复；
   - [ ] 派发重试耗尽进入 `manual_takeover_required`；
   - [ ] `TC-220-TMP-003/004/005/006` 后端集成测试通过。

### 12.4 PR-225-04：路由/能力/Authz 门禁收口（M4）
1. [ ] 更新 tasks 路由 allowlist 与 capability 映射。
2. [ ] 明确 tasks API capability（submit/get/cancel）与租户边界策略。
3. [ ] 目录与门禁：
   - [ ] `config/routing/allowlist.yaml`
   - [ ] `config/capability/route-capability-map.v1.json`
   - [ ] `internal/routing/allowlist.go` 与相关测试
4. [ ] 本 PR DoD：
   - [ ] `make check routing`
   - [ ] `make check capability-route-map`
   - [ ] `make authz-pack && make authz-test && make authz-lint`

### 12.5 PR-225-05：前端轮询与 E2E 证据（M5）
1. [ ] 前端接入 `AsyncTaskReceipt` 与任务轮询页面态。
2. [ ] 显示任务关键字段：`task_id/workflow_id/attempt/error_code/request_id/trace_id`。
3. [ ] 增补自动化用例：
   - [ ] `apps/web/src/api/assistant.ts` 与 `apps/web/src/api/assistant.test.ts`
   - [ ] `apps/web/src/pages/assistant/AssistantPage.tsx`
   - [ ] `e2e/tests/tp220-assistant.spec.js`（扩展 TMP-001~006）
4. [ ] 本 PR DoD：
   - [ ] `TC-220-TMP-001~006` 全绿；
   - [ ] `make e2e && make preflight` 全绿；
   - [ ] `docs/dev-records/` 补齐 225 执行证据文档。

### 12.6 关键停止线（Stopline）
1. [ ] 发现重复任务（同 `tenant+conversation+turn+request_id` 出现多条有效任务）立即阻断发布。
2. [ ] 发现 `queued` 且 `dispatch_status=pending` 超过 `dispatch_deadline_at` 未自动收敛，阻断发布。
3. [ ] 发现任务执行绕过 `confirm/commit/re-auth/One Door` 任一边界，阻断发布。
4. [ ] 发现 224 版本/哈希漂移未被拦截（误放行），阻断发布。

### 12.7 每个 PR 的最小门禁命令（新增）
1. [ ] PR-225-01：`go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] PR-225-02：在 01 基础上追加 `make iam plan && make iam lint && make iam migrate up && make sqlc-generate`
3. [ ] PR-225-03：在 02 基础上追加 `make test`（含 workflow/dispatcher 集成测试）
4. [ ] PR-225-04：在 03 基础上追加 `make check routing && make check capability-route-map && make authz-pack && make authz-test && make authz-lint`
5. [ ] PR-225-05：在 04 基础上追加 `make e2e`
6. [ ] 合并前统一执行：`make preflight && make check doc`

## 13. 执行记录模板（新增）
1. [ ] 新建：`docs/dev-records/dev-plan-225-execution-log.md`。
2. [ ] 每个 PR 至少记录：
   - [ ] 变更范围（文件与模块）；
   - [ ] 命中门禁命令与结果；
   - [ ] 对应 TMP/BE 用例通过证据（测试名 + 时间戳）；
   - [ ] 风险项与处置结论（是否触发 Stopline）。
3. [ ] 发布前补齐“最终验收快照”：
   - [ ] 指标快照（dispatch pending max age、manual takeover rate）；
   - [ ] 关键日志样本（脱敏）；
   - [ ] 回滚/人工接管演练结果。

## 14. 开放问题与截止（新增）
1. [ ] `dispatch_deadline_at` 默认值（建议 10 分钟）需在 M1 评审冻结。
2. [ ] `max_attempts` 默认值（建议 3）与退避参数需在 M1 冻结并写入配置契约。
3. [ ] UUIDv7 可用性（Go 版本与库）需在 PR-225-01 结论化。
4. [ ] 是否需要独立任务列表 API（按会话查询）暂不纳入本计划；若新增需单独补充契约评审。

## 15. 关联文档
- `docs/dev-plans/001-technical-design-template.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/211-blueprint-temporal-m10d0-minimal-orchestration-foundation.md`
- `docs/dev-plans/212-blueprint-eval-gates-and-triggered-temporal-productionization.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `AGENTS.md`
