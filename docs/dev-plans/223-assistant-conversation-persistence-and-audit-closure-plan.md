# DEV-PLAN-223：Assistant 会话持久化与审计闭环详细设计

**状态**: 进行中（2026-03-07 15:45 CST；原持久化/审计闭环已完成，但已按 `DEV-PLAN-280/260` 方向重新评估，现作为“业务事实源 + phase/DTO 持久化”现行契约）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
  - `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
  - `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
  - `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
  - `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- **当前痛点**:
  1. 原 `223` 已解决“内存 map 无法恢复”的第一阶段问题，但其文档口径仍停留在 `validated/confirmed/committed` 这类较粗粒度状态，不足以承接 `260` 已冻结的 `phase` 级业务 FSM。
  2. 旧版持久化模型没有明确冻结 `await_missing_fields / await_candidate_pick / await_candidate_confirm / await_commit_confirm` 所需的最小交互快照，服务重启后无法稳定重建同一轮对话上下文。
  3. `280/284` 已明确前端降权：vendored UI 只能消费后端 DTO，不能在页面 helper / adapter 中重算业务 FSM；因此后端必须把 `phase/candidates/draft/commit-reply` 的事实源口径补齐并冻结。
  4. 旧 `223` 对前端依赖仍引用历史 `222` 口径，已不再适合作为新正式承载面的约束来源。
- **业务价值**:
  - 将 Assistant 从“可持久化”进一步升级为“`260` 业务 FSM 可恢复、`280/284` UI 可直接消费、全链路可追踪与可审计”的稳定事务能力。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
1. [ ] 落地会话/回合/状态转移/幂等最小持久化模型。
2. [ ] 支持 `tenant_id/actor_id/conversation_id/turn_id/request_id/trace_id` 全链路追踪。
3. [ ] 支持服务重启后的会话恢复、`phase` 恢复、待补全/待候选/待确认上下文恢复与提交幂等验证。
4. [ ] 幂等重试返回**同一响应语义**（同 `http_status + error_code + response_body`）。
5. [ ] 强化租户绑定防漂移（对齐 `TC-220-BE-011`）并形成可审计证据。
6. [ ] 明确业务事实源：本仓持久化的 `conversation_id/turn_id/request_id/trace_id + phase + 状态转移 + 交互快照` 是 Assistant 唯一业务真相，前端消息树只作为唯一用户可见渲染面。
7. [ ] 冻结 vendored UI 可消费的最小 DTO 契约：`phase / missing_fields / candidates / pending_draft_summary / selected_candidate_id / commit_reply / error_code`。
8. [ ] 形成 `223 -> 260 -> 280/284` 的职责链路：后端负责业务事实源与 DTO，前端只负责渲染与事件分发。

### 2.2 非目标 (Out of Scope)
1. [ ] 不引入 Temporal 任务模型与 `/tasks` API（由 `DEV-PLAN-225` 承接）。
2. [ ] 不在本计划内定义 vendored UI 的源码纳管、发送接管、渲染接管细节（由 `DEV-PLAN-280/284` 承接）。
3. [ ] 不在本计划内引入第二业务写入口、LibreChat 侧业务真相存储或前端本地 FSM 兜底。
4. [ ] 不在本计划内实现跨租户分析报表。

## 2.3 事实源冻结
1. [ ] `assistant_conversations.current_phase`、`assistant_turns` 的交互快照、状态转移审计与幂等记录共同构成 Assistant 的唯一业务事实源。
2. [ ] `260` 冻结的 `phase` 是业务阶段真相；官方 UI 的消息实体、气泡树、卡片渲染结果都不得替代上述持久化事实源。
3. [ ] 任一用户可见回执若无法回溯到 `conversation_id/turn_id/request_id/trace_id + phase + 状态转移`，则视为不满足事务化、可回放与 `280` 前端降权要求。
4. [ ] 前端若存在 adapter，只能做展示层归一、事件分发与协议适配；不得重算候选裁决、确认约束或阶段推进。

### 2.4 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [X] Go 代码
  - [ ] `.templ` / Tailwind
  - [ ] 多语言 JSON
  - [X] Authz
  - [X] 路由治理
  - [X] DB 迁移 / Schema
  - [X] sqlc（若新增查询契约）
  - [X] 文档门禁
- **SSOT 引用**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - `docs/dev-plans/025-sqlc-guidelines.md`

### 2.5 标准对齐（DEV-PLAN-005）
1. [ ] `STD-001`：业务幂等统一 `request_id`，追踪统一 `trace_id`，语义严格分离。
2. [ ] `STD-003`：ID/Code 命名单一权威表达，不引入同义字段。
3. [ ] `STD-004`：不新增对外版本噪音字段/别名窗口。
4. [ ] `STD-006`：鉴权失败/未登录口径保持既有 401/403 规范。
5. [ ] `DEV-PLAN-004M1`：禁止 legacy 回退（内存兜底/双链路并行写/旧实现兜底）。

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 架构图 (Mermaid)
```mermaid
graph TD
    A[Assistant API] --> B[Assistant Service]
    B --> C[(assistant_conversations current_phase)]
    B --> D[(assistant_turns phase_snapshot + dto)]
    B --> E[(assistant_state_transitions from_phase/to_phase)]
    B --> F[(assistant_idempotency)]
    G[GET conversation / DTO rebuild] --> B
    H[Vendored UI adapter] --> G
    I[POST turns/:confirm|:commit] --> B
```

### 3.2 关键设计决策 (ADR 摘要)
- **决策 1：最小四表模型升级为“会话当前 phase + turn 交互快照 + 状态转移 + 幂等”组合（选定）**
  - 选项 A：继续仅保存粗粒度 `state`。缺点：无法承接 `260` 的 phase/FSM，也无法支撑 `280/284` 的后端 DTO 驱动。
  - 选项 B（选定）：保留 `conversation/turn/state_transition/idempotency` 四表，但在 `conversation/turn` 中补齐 `phase` 与最小交互快照。
- **决策 2：幂等键采用上下文组合（选定）**
  - 选项 A：全局 request_id。缺点：跨会话冲突。
  - 选项 B（选定）：`(tenant_id, conversation_id, turn_id, request_id)`。
- **决策 3：幂等并发控制采用“预占键 + 冲突回读”（选定）**
  - 选项 A：`exists` 后写入。缺点：并发窗口导致重复提交或唯一键冲突。
  - 选项 B（选定）：`INSERT ... ON CONFLICT DO NOTHING` 预占幂等键，冲突后回读已落盘响应。
- **决策 4：phase 快照、状态转移与审计同事务落盘（选定）**
  - 选项 A：异步审计或仅在前端保留阶段状态。缺点：失败时出现证据缺口，且与 `280` 前端降权冲突。
  - 选项 B（选定）：`current_phase`、turn 交互快照、状态转移与幂等结果同事务落盘，保证重启后可恢复与可回放一致性。
- **决策 5：UI 消费 DTO 采用“白名单字段 + 可回放快照”策略（选定）**
  - 选项 A：让前端依据自然语言文本或局部 store 自行推断。缺点：升级脆弱，违反 `280/284`。
  - 选项 B（选定）：后端只暴露白名单 DTO，且这些 DTO 必须能从持久化事实源稳定重建。

### 3.3 冻结不变量（必须始终成立）
1. [ ] 同 `(tenant_id, conversation_id, turn_id, request_id)` 重试只产生一次业务提交副作用。
2. [ ] 幂等命中返回同一响应语义（`http_status + error_code + response_body`），不得“同键不同响应”。
3. [ ] `current_phase` 必须与最新一次状态转移同事务一致，不允许“conversation 显示一个 phase、审计链落另一个 phase”。
4. [ ] `await_candidate_confirm` 必须已有 `selected_candidate_id`；`await_commit_confirm` 必须已有 `pending_draft_summary`。
5. [ ] `:commit` 必须基于 turn 内快照字段执行版本漂移校验（`policy/composition/mapping`）。
6. [ ] 所有读写都必须携带租户上下文并 fail-closed；跨租户访问固定 403/`tenant_mismatch`。
7. [ ] 禁止内存兜底、双链路并行写、前端本地 FSM 兜底、旧实现回退。

## 4. 数据模型与约束 (Data Model & Constraints)
> 真正执行 `CREATE TABLE` 前，必须先获得用户确认。

### 4.1 Schema 定义（草案）
```sql
-- assistant_conversations
tenant_id uuid not null
conversation_id uuid not null
actor_id text not null
current_phase text not null
created_at timestamptz not null
updated_at timestamptz not null
primary key (tenant_id, conversation_id)
check (current_phase in ('idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed', 'canceled', 'expired'))

-- assistant_turns
tenant_id uuid not null
conversation_id uuid not null
turn_id uuid not null
request_id text not null
trace_id text not null
input_text text not null
phase text not null
pending_draft_summary text
missing_fields jsonb not null default '[]'::jsonb
candidate_options jsonb not null default '[]'::jsonb
selected_candidate_id text
commit_reply jsonb
error_code text
risk_tier text not null
policy_version text not null
composition_version text not null
mapping_version text not null
created_at timestamptz not null
primary key (tenant_id, conversation_id, turn_id)
foreign key (tenant_id, conversation_id)
  references assistant_conversations(tenant_id, conversation_id)
check (phase in ('idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed'))

-- assistant_state_transitions
id bigserial primary key
tenant_id uuid not null
conversation_id uuid not null
turn_id uuid
request_id text not null
trace_id text not null
from_phase text not null
to_phase text not null
reason_code text
actor_id text not null
changed_at timestamptz not null
foreign key (tenant_id, conversation_id)
  references assistant_conversations(tenant_id, conversation_id)
check (from_phase in ('init', 'idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed', 'canceled', 'expired'))
check (to_phase in ('idle', 'await_missing_fields', 'await_candidate_pick', 'await_candidate_confirm', 'await_commit_confirm', 'committing', 'committed', 'failed', 'canceled', 'expired'))

-- assistant_idempotency
tenant_id uuid not null
conversation_id uuid not null
turn_id uuid not null
turn_action text not null
request_id text not null
request_hash text not null
status text not null default 'pending'
http_status integer
error_code text
response_body jsonb
response_hash text
created_at timestamptz not null
finalized_at timestamptz
expires_at timestamptz not null
primary key (tenant_id, conversation_id, turn_id, turn_action, request_id)
foreign key (tenant_id, conversation_id, turn_id)
  references assistant_turns(tenant_id, conversation_id, turn_id)
check (status in ('pending', 'done'))
check (turn_action in ('confirm', 'commit'))
check (response_body is null or octet_length(response_body::text) <= 65536)
```

### 4.2 约束与索引要求
1. [ ] 会话、回合、幂等统一采用租户前缀主键，避免“键存在但租户上下文缺失”的 fail-open 查询。
2. [ ] `assistant_turns(tenant_id, conversation_id, created_at, turn_id)` 索引，保障会话恢复顺序稳定。
3. [ ] `assistant_state_transitions(tenant_id, conversation_id, changed_at, id)` 索引，保障审计回放顺序稳定。
4. [ ] `assistant_idempotency(tenant_id, conversation_id, turn_id, turn_action, request_id)` 主键 + `request_hash` 校验，阻断同键异载荷。
5. [ ] `assistant_turns` 必须固化 `phase/pending_draft_summary/missing_fields/candidate_options/selected_candidate_id/commit_reply/error_code` 最小交互快照，保障重启后可恢复。
6. [ ] `assistant_turns` 必须固化 `policy_version/composition_version/mapping_version`，用于 `:commit` 漂移校验。
7. [ ] `request_hash` 计算口径冻结为：`turn_action + canonical_json_payload`，防止跨动作键碰撞。
8. [ ] `candidate_options/missing_fields/commit_reply/response_body` 仅允许白名单 DTO 字段，禁止直接落盘上游原始消息对象、DOM 结构或完整业务对象。

### 4.3 RLS 与鉴权约束（落地项）
1. [ ] 新表全部 `ENABLE ROW LEVEL SECURITY`，默认拒绝（fail-closed）。
2. [ ] 所有策略以 `tenant_id = current_setting('app.tenant_id', true)::uuid` 为准，不提供跨租户兜底策略。
3. [ ] 事务开始后先注入租户上下文，再执行读写；缺上下文直接失败。
4. [ ] Casbin 仅负责路由授权，RLS 仅负责数据圈地，不交叉兜底。

### 4.4 迁移策略
1. [ ] 先完成契约评审与用户确认（涉及新表/新列时强制）。
2. [ ] 若仓内已落地旧版 `223` schema，本轮迁移默认采用**增量收敛**：补齐 `current_phase`、turn 交互快照与 `from_phase/to_phase`，不得回退到前端本地状态兜底。
3. [ ] 执行模块级 `plan/lint/migrate up`，确保 Atlas+Goose 闭环。
4. [ ] 在迁移中同步落地 RLS policy、必要索引与历史数据回填策略，不拆分到后续“补丁迁移”。
5. [ ] 若命中 sqlc，执行 `make sqlc-generate`，并在必要时执行 `make sqlc-verify-schema`。
6. [ ] 生成后要求 `git status --short` 无生成物漂移。
7. [ ] 为 `assistant_idempotency.expires_at` 提供默认值（建议 `created_at + interval '30 days'`）。

## 5. 接口契约 (API Contracts)
> 不新增对外路由，仅增强既有 assistant API 的持久化与审计语义。

### 5.1 既有 API 语义增强
1. [ ] `POST /internal/assistant/conversations`：创建后可持久化查询，并写入初始 `init -> idle` 状态转移审计。
2. [ ] `GET /internal/assistant/conversations/{conversation_id}`：返回持久化 turn、`current_phase`、最小交互快照与状态历史，顺序可回放。
3. [ ] `POST /internal/assistant/conversations/{conversation_id}/turns`：持久化 `request_id/trace_id`、版本快照字段与本轮 `phase` / DTO 快照。
4. [ ] `POST .../turns/:confirm`、`POST .../turns/:commit`：状态转移、交互快照更新、审计、幂等记录同事务落盘。
5. [ ] 以上接口返回的 DTO 必须稳定包含 `phase / missing_fields / candidates / pending_draft_summary / selected_candidate_id / commit_reply / error_code` 中的适用子集，供 `280/284` 直接消费。

### 5.2 错误码与租户绑定契约
1. [ ] 同 `(tenant, conversation, turn, turn_action, request_id)` 重试返回完全一致响应语义。
2. [ ] 同键不同载荷（`request_hash` 不一致）返回 `idempotency_key_conflict`（409）。
3. [ ] 违反 `phase` 状态机约束返回 `conversation_state_invalid`。
4. [ ] 租户不匹配返回 `tenant_mismatch`/403（对齐 `TC-220-BE-011`）。
5. [ ] 保持缺字段、候选固化、确认冲突、提交失败等错误码与 `DEV-PLAN-221/260` 一致。
6. [ ] 处理中请求返回 `request_in_progress`（409），并带 `Retry-After`、`retry_after_ms`、`advice=retry_same_request_id`。

### 5.3 `request_in_progress` 重试契约（客户端）
1. [ ] 客户端必须使用同一 `request_id` 重试，不得隐式改写幂等键。
2. [ ] 退避策略冻结：指数退避（300ms → 600ms → 1200ms → 2000ms，上限 2000ms），最多 5 次。
3. [ ] 达到重试上限后展示“请求处理中，可稍后刷新会话”，不再继续前台高频重试。
4. [ ] 服务端 `Retry-After` 建议默认 1s，可按负载调节但不得省略。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 会话创建流程（伪代码）
```text
begin tx
set local app.tenant_id
insert assistant_conversations(current_phase='idle')
insert assistant_state_transitions(init -> idle)
commit
```

### 6.2 turn 动作幂等流程（并发安全，伪代码）
```text
begin tx
set local app.tenant_id
lock conversation row for update

insert assistant_idempotency(key including turn_action..., request_hash, status='pending')
on conflict do nothing returning key

if not inserted:
  select existing idempotency row for update
  if existing.request_hash != request_hash:
    return 409 idempotency_key_conflict
  if existing.status = 'done':
    return existing.http_status + existing.error_code + existing.response_body
  return 409 request_in_progress + Retry-After

validate tenant + state transition + version drift
apply confirm/commit mutation
update assistant_conversations set current_phase=?
upsert assistant_turns phase snapshot + dto snapshot
insert assistant_state_transitions(from_phase -> to_phase)
build response envelope(whitelist fields only)

update assistant_idempotency
  set status='done', http_status=?, error_code=?, response_body=?, response_hash=?, finalized_at=now(), expires_at=now()+30d
where key...

commit
```

### 6.3 版本漂移校验流程（伪代码）
```text
on :commit
  read turn snapshot(policy/composition/mapping)
  compare with current versions
  if drift:
    transition await_commit_confirm -> await_commit_confirm (same tx, dto refresh)
    insert transition audit(reason_code=version_drift)
    return conversation_confirmation_required
```

### 6.4 恢复流程（伪代码）
```text
on GET conversation:
  read conversation by (tenant_id, conversation_id)
  read turns order by created_at, turn_id
  read transitions order by changed_at, id
  rebuild `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` DTO deterministically
  return
```

## 7. 安全与鉴权 (Security & Authz)
1. [ ] 所有持久化读写必须显式事务 + 租户注入，fail-closed。
2. [ ] RLS 做圈地、Casbin 做授权，职责不漂移。
3. [ ] 审计字段必须记录 `actor_id/request_id/trace_id`。
4. [ ] 禁止 legacy 回退（内存兜底/双链路并行写）。
5. [ ] 跨租户访问固定返回 403，不允许“自动降级为 not found”掩盖越权。
6. [ ] 禁止把业务 phase/FSM 放回前端本地 store 作为事实源兜底。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：
  - `DEV-PLAN-221` 错误码与基础状态约束口径冻结。
  - `DEV-PLAN-260` phase/FSM/业务对话语义冻结。
  - `DEV-PLAN-280` 前端降权与“官方消息树仅为渲染面”口径冻结。
  - `DEV-PLAN-284` 将直接消费本计划冻结的 DTO 字段。
  - 新增表前用户明确确认。
- **里程碑**：
  1. [ ] M1：`223/260` 对齐后的数据契约冻结（phase/DTO/索引/RLS/错误码）+ 用户确认。
  2. [ ] M2：迁移 + sqlc + repository/store 落地，补齐 `current_phase` 与 turn 交互快照。
  3. [ ] M3：service 幂等、恢复与 DTO rebuild 切换 + 租户绑定校验。
  4. [ ] M4：为 `284` 提供稳定消费契约，完成并发回归、门禁全跑、证据收口。

## 9. 测试、门禁与验收标准 (Acceptance Criteria)
- **单元测试**：
  1. [ ] `idle/await_missing_fields/await_candidate_pick/await_candidate_confirm/await_commit_confirm/committing/committed/failed` 的 phase 转移合法/非法分支。
  2. [ ] 幂等命中、并发冲突、同键异载荷冲突分支。
  3. [ ] 缺字段、候选固化、待确认摘要与错误码映射回归。
  4. [ ] 漂移触发 DTO 刷新与重新确认分支。
  5. [ ] 同 `request_id` 在 `confirm` 与 `commit` 下互不碰撞（动作维度幂等）。
- **集成测试**：
  1. [ ] 服务重启后会话、turn、`current_phase` 与待补全/待候选/待确认上下文可恢复。
  2. [ ] 并发重试不产生重复提交，且返回一致响应语义。
  3. [ ] 租户切换访问同 conversation 被阻断（`TC-220-BE-011`）。
  4. [ ] RLS 缺少租户上下文时 fail-closed。
  5. [ ] `request_in_progress` 返回 `Retry-After`，客户端按退避策略收敛重试流量。
  6. [ ] `response_body` 超限时被拒绝或裁剪，且不破坏幂等语义回放。
  7. [ ] `GET conversation` 返回的 DTO 可被 `280/284` 直接消费，而无需前端本地补算 phase 或候选语义。
- **门禁命令（按触发器矩阵）**：
  1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
  2. [ ] `make check routing && make check capability-route-map`
  3. [ ] `make authz-pack && make authz-test && make authz-lint`
  4. [ ] `make check request-code && make check no-legacy && make check error-message`
  5. [ ] `make <module> plan && make <module> lint && make <module> migrate up`
  6. [ ] `make sqlc-generate`（必要时 `make sqlc-verify-schema`）
  7. [ ] `make check doc && make preflight`
- **验收对齐**：
  1. [ ] 对齐 `TC-220-BE-009/011`。
  2. [ ] 执行记录中附“命中触发器 + 实际命令 + 结果 + 生成物漂移检查”。

## 10. 故障处置与最小可观测 (Ops & Recovery)
- 不引入额外运维开关；遵循最小运维原则。
- **日志最小字段**：`tenant_id/conversation_id/turn_id/request_id/trace_id/error_code`。
- **幂等数据治理**：
  1. [ ] `assistant_idempotency` 保留期默认 30 天（以 `expires_at` 为准）。
  2. [ ] 到期数据由定时清理任务删除；清理失败需告警并记录执行证据。
  3. [ ] 长期审计依赖 `assistant_state_transitions`，不依赖 idempotency 表长期保存。
- **处置预案（必须可执行）**：
  1. [ ] 触发条件：幂等冲突激增、跨租户误判、状态机误转移。
  2. [ ] 责任人：当班后端值守（执行）+ 模块 owner（审批恢复）。
  3. [ ] 处置顺序：环境级保护 → 只读/停写 → 修复 → 重试/重放 → 恢复。
  4. [ ] 恢复判定：目标测试（含 `TC-220-BE-009/011`）+ `make preflight` 全绿。

## 11. 交付物
1. [ ] assistant 持久化 schema（经确认后）与代码改造。
2. [ ] 持久化/幂等/租户绑定/RLS/phase DTO 恢复测试与门禁证据。
3. [ ] `docs/dev-records/dev-plan-223-execution-log.md`。
4. [ ] `260/280/284` 对齐证据补齐：
  - `docs/dev-records/dev-plan-220-execution-log.md`
  - `docs/dev-records/dev-plan-220-m0-chat-readonly-evidence.md`
  - `docs/dev-records/dev-plan-220-m1-conversation-commit-evidence.md`

## 12. 关联文档
- `docs/dev-plans/001-technical-design-template.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- `AGENTS.md`
