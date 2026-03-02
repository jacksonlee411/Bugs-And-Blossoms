# DEV-PLAN-220A：聊天框 AI 助手（DEV-PLAN-220）测试验证与功能缺口收敛方案

**状态**: 进行中（2026-03-02 05:07 UTC）

## 1. 背景
- DEV-PLAN-220 已定义 P0/P1/P2 目标、测试矩阵（TC-220-BE/FE/E2E/TMP）与门禁。
- 本次 220A 目标不是扩功能，而是先做“测试案例验证 + 代码现状审计”，形成当前完成度报告与明确缺口清单，避免盲目推进。

## 2. 220A 目标与非目标
### 2.1 目标
1. [ ] 基于 DEV-PLAN-220 的测试矩阵，对当前仓库进行逐项可追踪验证。
2. [ ] 输出“当前完成情况详细报告”（分 P0/P1/P2 与 BE/FE/E2E/TMP 维度）。
3. [ ] 输出“功能缺口清单（按阻断级别排序）”并给出收口步骤。

### 2.2 非目标
1. [ ] 本计划不直接改造业务逻辑与数据模型。
2. [ ] 本计划不引入新迁移/新表（仅识别是否需要，真正执行仍需用户确认）。

## 3. 验证方法（执行口径）
1. [X] **代码审计**：对照 DEV-PLAN-220 的 API/状态机/时序要求，审阅 `internal/server`、`apps/web`、`config/capability`、`config/routing`。
2. [X] **现有自动化验证**：执行已存在测试与门禁，确认“已落地能力”与“门禁就绪状态”。
3. [X] **测试矩阵映射**：将 TC-220-* 映射为三类结论：
   - `已实现且已验证`
   - `已实现但未形成对应自动化`
   - `未实现/未接入`

## 4. 执行记录（2026-03-02 UTC）
1. [X] `go test ./internal/server -run Assistant -count=1`（通过）
2. [X] `pnpm --dir apps/web exec vitest run src/api/assistant.test.ts`（通过）
3. [X] `make check routing`（通过）
4. [X] `make check capability-route-map`（通过）
5. [X] `make authz-pack && make authz-test && make authz-lint`（通过）
6. [X] `make check error-message`（通过）

## 5. 当前完成情况详细报告（基于代码与测试证据）

### 5.1 阶段完成度（结论）
- **P0（聊天 + 只读计划）**：约 **70% 完成**（页面、路由、会话/回合 API、候选解析与风险分层已落地；但 strict decode/边界违约码、dry-run diff 可视化断言、前端交互门控仍有缺口）。
- **P1（受控提交）**：约 **45% 完成**（confirm/commit、候选确认、re-auth 基础阻断、One Door 写入口已接；但状态机终态、版本漂移回退、会话持久化、候选固化不可变等关键契约未闭环）。
- **P2（编排增强）**：约 **5% 完成**（仅有文档口径；tasks API / Temporal workflow / TMP 测试均未落地）。

### 5.2 已落地能力（有代码证据）
1. [X] `/app/assistant` 页面与导航入口已存在，含左侧 iframe（`/assistant-ui/`）+右侧事务面板。
2. [X] 助手 API 已存在：
   - `POST /internal/assistant/conversations`
   - `GET /internal/assistant/conversations/{conversation_id}`
   - `POST /internal/assistant/conversations/{conversation_id}/turns`
   - `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_action}`（`:confirm` / `:commit`）
3. [X] confirm/commit 最小状态机已实现：`validated -> confirmed -> committed`。
4. [X] 候选歧义阻断与候选确认已实现；唯一候选自动解析（`resolution_source=auto`）已实现。
5. [X] Pre-commit re-auth 的最小阻断已实现：`ai_actor_auth_snapshot_expired` / `ai_actor_role_drift_detected`。
6. [X] 路由 allowlist 与 capability-route-map 对 assistant 路由已接入并通过门禁。

### 5.3 TC-220 测试矩阵完成概览

#### A. 后端（TC-220-BE-001~015）
- **已实现且已验证（7/15）**：001、002、005、007、011、013、014（覆盖在 `assistant_api_test.go` + `assistant_api_coverage_test.go`）。
- **已实现但与计划口径不完全一致（1/15）**：
  - 009 幂等：当前为“状态已提交即幂等返回”，但未形成“conversation+turn+request_id 级别持久化幂等契约”。
- **未实现/未闭环（7/15）**：003、004、006、008、010、012、015（详见第 6 节）。

#### B. 前端（TC-220-FE-001~007）
- **已实现（4/7）**：001、002、004、007（页面结构、追踪字段、错误码映射基础链路、候选展示具备）。
- **未闭环/未达成（3/7）**：003、005、006（高风险门控、状态机按钮可用性、postMessage 安全）。

#### C. E2E（TC-220-E2E-001~008 + 101~104）
- **当前仓库状态**：`e2e/tests/` 下尚无 `tp220-*.spec.js`；E2E 计划用例均未落地。

#### D. Temporal（TC-220-TMP-001~006）
- **当前仓库状态**：未发现 `/internal/assistant/tasks*` API、任务状态模型、Temporal workflow/activity 实现与对应测试。

### 5.4 补充评估（用户追加）

1. **多轮会话是否已实现**
   - 结论：**部分实现（基础形态）**。
   - 证据：后端会话对象支持 `Turns []*assistantTurn`，`createTurn` 会持续 append；`GET /internal/assistant/conversations/{conversation_id}` 可回读整段回合。
   - 缺口：当前每轮意图解析仍是“单轮输入独立解析”，未见跨轮语义记忆/澄清链路；且会话为内存态，重启不可恢复。

2. **LibreChat 是否已实现强多模型适配能力（OpenAI/DeepSeek/Claude/Gemini 配置）**
   - 结论：**仓库内未实现该能力**。
   - 证据：当前仅实现 `/assistant-ui/*` 到 `LIBRECHAT_UPSTREAM` 的反向代理 + iframe 嵌入；未见本仓库内“模型提供商配置管理/密钥配置/路由策略”代码与测试。
   - 说明：若 LibreChat 上游实例自行配置了多模型，这是“上游部署能力”，不等同于本仓库已交付“平台侧强适配能力”。

3. **大模型意图判断能力是否实现**
   - 结论：**未实现（当前为规则匹配）**。
   - 证据：`assistantExtractIntent` 采用正则/关键词解析（如“在X之下”“名为X的部门”“日期”），默认 action 为 `plan_only`；未见真实 LLM 调用或结构化解码管线。
   - 影响：复杂语义、多意图、多轮上下文、歧义消解能力不足，与 DEV-PLAN-220 目标中的“AI 语义解析协议”仍有明显差距。

## 6. 220 功能缺口清单（按阻断级别）

### 6.1 Blocker（不解决无法宣称 P1 完成）
1. [ ] **缺失 strict decode / boundary violation 错误码链路**：`ai_plan_schema_constrained_decode_failed`、`ai_plan_boundary_violation` 未落地。
2. [ ] **状态机不完整**：缺失 `canceled/expired` 终态与 `conversation_state_invalid` 提交阻断。
3. [ ] **版本漂移回退未实现**：未实现 `policy/composition/mapping` 漂移后自动回退 `validated` 并要求重确认。
4. [ ] **候选确认固化缺失**：`confirmed` 状态下仍可再次 confirm 并改写 `resolved_candidate_id`，违反“同 turn 不得静默变更”。
5. [ ] **P1 会话持久化缺失**：当前会话数据在内存 map，重启即丢失，不满足可回放与事务追踪要求。

### 6.2 High（影响可用性/验收可信度）
1. [ ] **前端按钮门控不足**：`risk_tier=high` 不会禁用直接提交；状态机按钮可用性未按计划收敛。
2. [ ] **前端未展示 dry-run diff 关键信息**：当前主要展示 plan summary，缺少可操作 diff 视图断言基础。
3. [ ] **E2E 全量缺失**：220 的核心验收路径（101~104）未形成自动化证据。
4. [ ] **LibreChat 安全边界不足**：缺失 postMessage origin/schema 白名单校验实现与测试。
5. [ ] **多模型适配治理缺失**：未见 OpenAI/DeepSeek/Claude/Gemini 的平台侧配置治理、健康检查与路由策略能力。
6. [ ] **LLM 意图判断引擎缺失**：当前为规则匹配，未形成 LLM + strict decode 的意图识别与约束输出闭环。

### 6.3 Medium（P2 前置/治理缺口）
1. [ ] **Tasks API 未落地**：`/internal/assistant/tasks*` 缺失，TC-220-BE-012/TMP-* 无法执行。
2. [ ] **Temporal 编排未落地**：无 receipt/status/cancel/dead-letter/manual takeover 模型与实现。
3. [ ] **审计证据文档缺失**：220 文档声明的 `docs/dev-records/dev-plan-220-*.md` 证据文件尚未创建。

## 7. 220A 收口实施步骤（建议）

### M1：契约与状态机补齐（优先级最高）
1. [ ] 扩展会话状态机：补齐 `canceled/expired` 与 `conversation_state_invalid`。
2. [ ] 新增版本漂移检测与回退：命中漂移即回退 `validated`。
3. [ ] 锁定候选固化语义：`confirmed` 后禁止静默改写 `resolved_candidate_id`（需显式 regenerate/new turn）。
4. [ ] 明确并实现 strict decode / boundary violation 错误码路径。

### M2：前端交互与 E2E 证据闭环
1. [ ] 实现 FE 按钮门控（risk/state 双门控）与 dry-run diff 展示。
2. [ ] 落地 `tp220-assistant-*.spec.js`（至少 101~104 + 003/004/005/006/008）。
3. [ ] 完成 postMessage 安全校验与对应 FE/E2E 测试。

### M3：持久化与审计闭环（需用户确认后再迁移）
1. [ ] 设计并评审会话持久化最小表集合（conversation/turn/state transition/idempotency）。
2. [ ] 获得用户书面确认后执行迁移与回归。
3. [ ] 补齐 `docs/dev-records/dev-plan-220-execution-log.md`、`...-m0-...md`、`...-m1-...md`。

### M4：P2 Tasks + Temporal（触发式推进）
1. [ ] 落地 `/internal/assistant/tasks*` API 与 capability-route-map/allowlist/authz 映射。
2. [ ] 落地最小 workflow/activity、timeout/retry/dead-letter/manual takeover。
3. [ ] 补齐 TC-220-TMP-001~006 自动化测试与指标证据。

## 8. 220A 验收标准
1. [ ] 220 测试矩阵中 P1 必需项（BE + FE + E2E-101/102/103/104）均有自动化证据。
2. [ ] Blocker 缺口全部关闭且门禁全绿：`make check routing`、`make check capability-route-map`、`make authz-*`、`make check error-message`、`make test`、`make e2e`、`make preflight`。
3. [ ] P2 相关内容如未触发，报告中必须明确“未触发原因 + 当前风险 + 触发条件”。

## 9. 关联文档
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/210-blueprint-conversation-transaction-and-actor-delegated-authz.md`
- `docs/dev-plans/211-blueprint-temporal-m10d0-minimal-orchestration-foundation.md`
- `docs/dev-plans/212-blueprint-eval-gates-and-triggered-temporal-productionization.md`
- `AGENTS.md`
