# DEV-PLAN-224A：Assistant 真实 Codex API 接入与多轮对话工作台实施计划

**状态**: 规划中（2026-03-03 03:35 UTC）

## 1. 背景与上下文 (Context)
- 关联计划：
  - `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
  - `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
  - `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- 现状（2026-03-03）：
  1. [ ] 224 已完成多模型治理与意图链路基线，但 provider adapter 仍为 deterministic 基线，未真实调用 Codex/API。
  2. [ ] 页面层虽可发起 turn，但仍偏“单轮操作面板”，缺少可用的多轮会话工作台（会话列表、切换、回放、继续对话、状态总览）。
  3. [ ] 会话持久化已具备（223），但前端尚未完整消费“多会话+多轮”能力。
- 本计划目标：在不破坏 One Door / confirm / commit / re-auth / fail-closed 边界的前提下，完成“真实 Codex API + 页面多轮对话闭环”。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
1. [ ] 落地真实 Codex API 调用链路（生产链路不再依赖 deterministic adapter）。
2. [ ] 保持 strict decode + boundary lint + contract snapshot + determinism guard 全链路不退化。
3. [ ] 在 `/app/assistant` 落地“多轮对话工作台”：会话列表、会话切换、历史回放、继续追问、turn 状态可视化、任务状态联动。
4. [ ] 新增会话列表读取能力（按 actor + tenant），支持分页与稳定排序。
5. [ ] 补齐 FE/BE/E2E 验收证据，确保 224A 可独立验收。

### 2.2 非目标
1. [ ] 不新增业务写旁路，不允许模型直接提交业务写入。
2. [ ] 不在数据库存储明文密钥，不引入第二套密钥管理。
3. [ ] 不改动 225 的任务状态机语义（仅做页面消费增强与编排联动）。

## 3. 设计原则与不变量 (Invariants)
1. [ ] One Door 不变：最终业务提交仍仅允许 `confirm/commit` 后由既有写链路执行。
2. [ ] No Tx, No RLS：Assistant 读写继续走事务与租户注入，fail-closed。
3. [ ] 错误码契约不降级：模型/解析/边界/幂等错误继续走目录化错误码。
4. [ ] 同键幂等语义不变：`request_id + turn_action` 同键同响应、同键异载荷冲突。
5. [ ] 页面可见性原则：224A 必须可在 UI 上直接验证“多轮会话能力”，不接受仅 API 完成。

## 4. 架构方案 (Architecture)
### 4.1 真实 Codex API 接入
1. [ ] 在 `openai` provider 下接入真实调用通道（Codex 模型作为 `model` 配置值），保持 provider 白名单治理不变。
2. [ ] 默认通过环境变量注入密钥（`OPENAI_API_KEY`）；可选 base URL 仅用于兼容代理/私网网关。
3. [ ] 生产环境 endpoint 口径冻结：仅允许 `https://`（含私网网关 HTTPS），禁止 `builtin://*` 与 `simulate://*`。
4. [ ] 开发/测试环境允许 `builtin://*` 与 `simulate://*` 作为测试桩/故障注入，不进入生产默认链路。
5. [ ] OpenAI/Codex 真实调用必须校验 `OPENAI_API_KEY`（不因 endpoint 形式绕过密钥校验）。
6. [ ] 调用参数冻结为确定性档位（如 temperature/top_p/n/schema strict），保障可审计与重放稳定性。
7. [ ] 保留 deterministic adapter 仅用于测试桩与离线回归，不进入生产默认链路。

### 4.2 多轮工作台信息架构（页面必交付）
1. [ ] 左栏：会话列表（最近更新时间倒序，显示 state、最后一轮摘要、更新时间）。
2. [ ] 中栏：会话时间线（逐轮展示 user_input、intent、plan、dry-run、confirm/commit 结果、task 状态）。
3. [ ] 右栏：当前轮操作区（继续追问、候选确认、commit、submit task/cancel task）。
4. [ ] 顶部：模型来源与版本信息（provider/model/revision）可见。
5. [ ] 空态/错误态/加载态统一：首次进入、无会话、会话切换失败、轮询失败均有明确提示。

### 4.3 API 扩展（为页面多轮提供支撑）
1. [ ] 新增 `GET /internal/assistant/conversations`（分页）用于会话列表。
2. [ ] 继续沿用 `GET /internal/assistant/conversations/{conversation_id}` 作为会话详情与回放源。
3. [ ] 保持现有 `turns`、`confirm/commit`、`tasks` 接口不破坏；仅补强页面消费与数据聚合。

## 5. 接口契约 (API Contracts)
### 5.1 会话列表 API（新增）
- `GET /internal/assistant/conversations?page_size=20&cursor=<opaque>`
- 响应（200）示意：
```json
{
  "items": [
    {
      "conversation_id": "conv_xxx",
      "state": "validated",
      "updated_at": "2026-03-03T03:00:00Z",
      "last_turn": {
        "turn_id": "turn_xxx",
        "user_input": "在鲜花组织下新建运营部",
        "state": "confirmed",
        "risk_tier": "low"
      }
    }
  ],
  "next_cursor": "opaque"
}
```
- 约束：
  1. [ ] 仅返回当前 tenant + actor 可见会话。
  2. [ ] 排序固定 `updated_at DESC, conversation_id DESC`，避免分页漂移。
  3. [ ] `page_size` 默认 20，最大 100（超限按 100 处理）。
  4. [ ] `cursor` 为不透明游标，仅允许服务端签发；非法/过期 cursor 返回 400 + `assistant_conversation_cursor_invalid`。
  5. [ ] 首次请求不带 cursor；`next_cursor` 为空表示无下一页。

### 5.2 真实模型调用契约（后端内部）
1. [ ] ProviderAdapter 返回结构化 JSON（严格 schema），禁止自由文本直传业务层。
2. [ ] 归一化错误分类：`timeout/rate_limited/unavailable/config_invalid/secret_missing/schema_decode_failed`。
3. [ ] 在 turn 返回体带出 `model_provider/model_name/model_revision`，前端可回显。
4. [ ] 配置校验与运行时校验口径一致：`validate/apply/resolve` 三处对 endpoint/secret 规则保持同一判定。

## 6. 页面交互契约 (UI/UX Contracts)
1. [ ] 用户可创建新会话，并在同一页面继续多轮追问（至少 5 轮连续交互无状态丢失）。
2. [ ] 用户可从会话列表切换到历史会话并完整回放该会话 turns。
3. [ ] 每一轮必须可见关键字段：`turn_id/request_id/trace_id/state/risk/provider/model`。
4. [ ] 确认候选与提交动作必须与当前选中 turn 强绑定，禁止跨会话误操作。
5. [ ] 任务状态（225）在时间线内可见并可轮询更新：`queued/running/succeeded/failed/manual_takeover_required/canceled`。
6. [ ] 页面刷新后可恢复“最近活跃会话”（本地仅存 conversation_id，不存密钥与敏感 payload）。

## 7. 实施拆解与 PR 切片 (Implementation & PR Slices)
### 7.1 PR-224A-01：真实 Codex Adapter（后端）
1. [ ] 新增真实 OpenAI/Codex adapter 与 HTTP client 封装。
2. [ ] 保留 deterministic adapter 作为测试桩，并通过配置切换（默认生产走真实 adapter）。
3. [ ] 增加超时、重试、错误归一化与可观测字段。
4. [ ] 变更文件（建议）：
   - `internal/server/assistant_model_gateway.go`
   - `internal/server/assistant_intent_pipeline.go`
   - `internal/server/assistant_model_gateway*_test.go`

### 7.2 PR-224A-02：会话列表 API（后端）
1. [ ] 新增 `GET /internal/assistant/conversations` handler 与 service/store 查询。
2. [ ] 补齐 routing allowlist / capability-route-map / route registry / authz 映射。
3. [ ] 补齐 API 单测、分页稳定性测试、租户越界测试。
4. [ ] 变更文件（建议）：
   - `internal/server/assistant_api.go`
   - `internal/server/assistant_persistence.go`
   - `internal/server/assistant_api_test.go`
   - `config/routing/allowlist.yaml`
   - `config/capability/route-capability-map.v1.json`

### 7.3 PR-224A-03：多轮工作台页面（前端）
1. [ ] 重构 `/app/assistant` 为“三栏工作台”（会话列表/时间线/操作区）。
2. [ ] 新增会话列表拉取、会话切换、历史回放、继续追问。
3. [ ] 任务状态联动展示（与 225 SDK 对齐）。
4. [ ] 补齐组件测试与交互测试。
5. [ ] 变更文件（建议）：
   - `apps/web/src/pages/assistant/AssistantPage.tsx`
   - `apps/web/src/api/assistant.ts`
   - `apps/web/src/pages/assistant/AssistantPage.test.tsx`
   - `apps/web/src/api/assistant.test.ts`

### 7.4 PR-224A-04：E2E 与收口
1. [ ] 新增多轮工作台 E2E：会话创建、连续追问、切换回放、确认提交、任务轮询。
2. [ ] 门禁回归：routing/authz/error-message/preflight。
3. [ ] 产出执行记录：`docs/dev-records/dev-plan-224a-execution-log.md`。

## 8. 风险与回滚 (Risks & Rollback)
1. [ ] 外部模型波动导致响应不稳定：通过严格 schema + 重试 + 明确错误码 fail-closed。
2. [ ] 页面复杂度提升导致状态错绑：以 `conversation_id + turn_id` 作为唯一操作上下文键。
3. [ ] 性能风险（会话长历史）：列表分页 + 详情按需加载，必要时补充 turn 分页（不在本期默认范围）。
4. [ ] 回滚策略：仅允许“环境级保护 + 只读/停写 + 修复后重试”；禁止长期双链路回退。
5. [ ] deterministic adapter 应急启用仅限故障处置：必须绑定 incident 编号、审批人、启用时间与自动失效时间（建议 ≤ 24h），并在恢复后补齐执行记录。

## 9. 测试与验收标准 (Testing & Acceptance)
### 9.1 后端
1. [ ] 单测：Codex adapter 成功/超时/限流/鉴权失败/非结构化返回/非法 schema。
2. [ ] 单测：会话列表分页稳定性、租户隔离、actor 隔离。
3. [ ] 集成：`create conversation -> multi turns -> get detail` 跨重启可恢复。
4. [ ] 单测：环境规则校验（prod 禁止 `builtin://*`/`simulate://*`；dev/test 允许）。
5. [ ] 单测：OpenAI/Codex 真实调用路径密钥必填（缺失 `OPENAI_API_KEY` 返回 `secret_missing`）。
6. [ ] 确定性回归：同输入+同快照重复执行（建议 20 次）`plan_hash` 一致，否则判定失败。

### 9.2 前端
1. [ ] 组件测试：会话列表加载、切换、高亮、空态。
2. [ ] 组件测试：时间线渲染 turns 与关键字段。
3. [ ] 组件测试：confirm/commit/task 操作在会话切换后不串线。

### 9.3 E2E（必须）
1. [ ] 场景 A：新建会话，连续 5 轮追问；第 3 轮后刷新页面，恢复后完成第 4~5 轮（全程状态不丢失）。
2. [ ] 场景 B：存在歧义候选时完成 confirm，再 commit 成功。
3. [ ] 场景 C：提交 async task 后轮询到终态，并在时间线可见终态字段。
4. [ ] 场景 D：模型不可用时返回明确错误码且页面提示可理解（不出现泛化失败文案）。

### 9.4 门禁与证据
1. [ ] `make check routing`
2. [ ] `make check capability-route-map`
3. [ ] `make authz-pack && make authz-test && make authz-lint`
4. [ ] `make check error-message`
5. [ ] `pnpm -C apps/web test -- src/api/assistant.test.ts src/pages/assistant/AssistantPage.test.tsx`
6. [ ] `go test ./internal/server -run Assistant -count=1`
7. [ ] `make preflight`
8. [ ] 执行证据写入 `docs/dev-records/dev-plan-224a-execution-log.md`

## 10. 完成定义 (Definition of Done)
1. [ ] 生产默认链路为真实 Codex API 调用，deterministic adapter 仅用于测试/降级演练。
2. [ ] `/app/assistant` 页面可完成“多会话 + 多轮 + 回放 + 继续追问 + 提交任务”的全流程。
3. [ ] 224/225 既有契约（One Door、幂等、漂移检测、错误码）无回退。
4. [ ] CI 门禁全绿并完成 PR 合并后同步固定分支。

## 11. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
