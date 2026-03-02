# DEV-PLAN-224：Assistant 多模型适配与 LLM 意图治理详细设计（修订版）

**状态**: 规划中（2026-03-02 18:25 UTC，补充“确定性与契约版本化”）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
  - `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
  - `docs/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
  - `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- **当前痛点（修订确认）**:
  1. 当前仓库对 LibreChat 的利用停留在 `/assistant-ui/*` 反向代理与 iframe 嵌入，尚未形成平台侧多模型治理闭环。
  2. 意图识别仍以规则匹配为主，缺少 LLM + strict decode + boundary lint 的确定性主链路。
  3. 会话为内存态，虽支持 turn append，但重启后不可恢复，难以支撑多轮对话稳定性。
  4. 用户无法在产品内配置 provider/model，无法实现“可视化多模型切换 + 治理审计”。
- **业务价值**:
  - 构建“可治理模型接入层 + 可审计意图到指令转换层 + 可恢复多轮会话层”，在不破坏 One Door 的前提下对齐 LibreChat 的多模型与会话体验能力。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
1. [ ] 建立平台侧多模型配置治理能力（OpenAI / DeepSeek / Anthropic Claude / Google Gemini），并提供可操作 UI 配置页。
2. [ ] 建立统一模型网关（超时、重试、健康检查、受控回退、错误归一化），统一屏蔽 provider 差异。
3. [ ] 建立完整“用户意图 -> 操作指令”链路：`Prompt -> RequirementIntentSpec -> strict decode -> boundary lint -> SkillExecutionPlan -> ConfigDeltaPlan -> DryRunResult`。
4. [ ] 建立多轮会话能力：支持会话恢复、上下文重建、幂等重放、版本漂移检测与回退。
5. [ ] 与 LibreChat 功能对齐：模型选择、多轮会话、消息桥接安全、前台体验一致；并保持 `confirm/commit/re-auth/One Door` 业务边界不变。
6. [ ] 通过门禁并产出可审计证据（含 provider/model/request_id/trace_id/conversation_id/turn_id）。

### 2.2 非目标 (Out of Scope)
1. [ ] 不把最终提交裁决权下放给模型。
2. [ ] 不在本计划实现 Temporal 异步编排（由 `DEV-PLAN-225` 承接）。
3. [ ] 不新增数据库表存储明文密钥。
4. [ ] 不允许 LibreChat 直接调用业务写路由。

## 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（本计划命中）**：
  - [X] Go 代码
  - [X] MUI Web UI（新增模型配置页）
  - [ ] 多语言 JSON（如新增文案则触发）
  - [X] Authz（assistant 相关动作回归）
  - [X] 路由治理（新增/调整内部路由）
  - [ ] DB 迁移 / Schema（224 本身不直接建表；依赖 223）
  - [ ] sqlc（若落地持久化查询由 223 触发）
  - [X] 文档门禁
- **SSOT 引用**：
  - `AGENTS.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`
  - `docs/dev-plans/009A-r200-tooling-playbook.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 架构图 (Mermaid)
```mermaid
graph TD
    A[Assistant Workspace /app/assistant] --> B[Right Panel: Tx Control]
    A --> C[Left Panel: LibreChat Embed]
    C --> D[Message Bridge postMessage]
    D --> E[Assistant BFF /internal/assistant/*]
    B --> E
    E --> F[Intent Orchestrator]
    F --> G[Model Gateway]
    G --> H[OpenAI Adapter]
    G --> I[DeepSeek Adapter]
    G --> J[Claude Adapter]
    G --> K[Gemini Adapter]
    G --> L[Health & Fallback]
    F --> M[Strict Decode]
    M --> N[Boundary Lint]
    N --> O[RequirementIntentSpec]
    O --> P[SkillExecutionPlan]
    P --> Q[ConfigDeltaPlan]
    Q --> R[DryRunResult]
    R --> S[Confirm/Commit Pipeline (One Door)]
    T[Model Config Page /app/assistant/models] --> U[/internal/assistant/model-providers]
    U --> G
```

### 3.2 关键设计决策 (ADR 摘要)
- **决策 1：多 provider 统一网关（选定）**
  - 选项 A：业务层直接调用各 provider SDK。缺点：耦合高、错误语义分裂。
  - 选项 B（选定）：统一 `ModelGateway` + `ProviderAdapter`，业务层不感知供应商差异。
- **决策 2：LLM 输出必须 strict decode（选定）**
  - 选项 A：自由文本 + 正则抽取。缺点：漂移大、边界难控。
  - 选项 B（选定）：结构化 schema 输出，失败即 `ai_plan_schema_constrained_decode_failed`。
- **决策 3：意图到指令必须显式分层（选定）**
  - 选项 A：意图解析后直接拼业务写请求。缺点：不可审计、不可门禁。
  - 选项 B（选定）：`IntentSpec -> SkillExecutionPlan -> ConfigDeltaPlan -> DryRunResult`，各层可校验、可审计、可回放。
- **决策 4：会话恢复能力依赖持久化层（选定）**
  - 选项 A：继续内存会话。缺点：重启丢失、幂等不可证。
  - 选项 B（选定）：224 定义接口与语义，持久化落地由 223 承接。
- **决策 5：受控回退，不做 silent fallback（选定）**
  - 选项 A：主模型失败静默降级规则解析并继续流程。缺点：行为不可预期。
  - 选项 B（选定）：可配置 provider fallback；全部失败 fail-closed 返回明确错误码。
- **决策 6：意图链路确定性优先（选定）**
  - 选项 A：以“自然波动可接受”为前提，仅做结构校验。缺点：同输入输出不稳定，难以审计与回放。
  - 选项 B（选定）：冻结推理参数 + 规范化产物 + 哈希对账 + 契约版本化，构建可重复执行链路。

## 4. 数据模型与约束 (Data Model & Constraints)
> 224 不直接新建表；配置与契约先行，持久化模型对齐 223。

### 4.1 Provider 配置模型（配置契约）
```yaml
assistant:
  provider_routing:
    strategy: priority_failover
    fallback_enabled: true
  providers:
    - name: openai
      enabled: true
      model: gpt-4o-mini
      endpoint: https://api.openai.com/v1
      timeout_ms: 8000
      retries: 1
      priority: 10
      key_ref: OPENAI_API_KEY
    - name: deepseek
      enabled: false
      model: deepseek-chat
      endpoint: https://api.deepseek.com
      timeout_ms: 8000
      retries: 1
      priority: 20
      key_ref: DEEPSEEK_API_KEY
```

### 4.2 意图与指令契约（新增）
1. [ ] `RequirementIntentSpec`：`conversation_id + turn_id + user_prompt + intent + constraints + expected_outcome + intent_schema_version + context_hash`。
2. [ ] `SkillExecutionPlan`：`selected_skills[] + execution_order + risk_tier + required_checks + compiler_contract_version + skill_manifest_digest`。
3. [ ] `ConfigDeltaPlan`：仅表达 capability 注册范围内的结构化变更，不允许 SQL/表名/未注册动作。
4. [ ] `DryRunResult`：`diff + explain + validation_errors + would_commit=false + plan_hash`。

### 4.3 会话上下文契约（对齐 223）
1. [ ] `conversation_id + turn_id + request_id + trace_id` 全链路可追踪。
2. [ ] 支持 `state/history/version snapshot` 回放。
3. [ ] 提交前比对 `policy/composition/mapping` 漂移，命中后回退 `validated`。

### 4.4 配置约束
1. [ ] `name` 必须在 `{openai, deepseek, claude, gemini}` 白名单内。
2. [ ] `enabled=true` 的 provider 必须提供 `model/endpoint/timeout_ms/key_ref`。
3. [ ] `priority` 必须唯一，避免路由歧义。
4. [ ] 密钥仅环境变量或密钥服务注入，不存入业务库。
5. [ ] 未通过配置校验时服务启动 fail-closed。

### 4.5 确定性与契约版本化约束（新增）
1. [ ] 模型调用采用确定性参数档位：`temperature=0`、`top_p=1`、`n=1`、固定 schema strict decode。
2. [ ] provider/model 必须固定到可审计 revision（禁止浮动 alias 直接用于生产提交链路）。
3. [ ] 编译器必须是纯函数：禁止读取时钟/随机数/外部可变状态；`selected_skills[]`、`diff[]` 需稳定排序。
4. [ ] 所有链路产物执行规范化（canonical JSON）并计算 `context_hash/intent_hash/plan_hash`。
5. [ ] 提交前强校验 `intent_schema_version + compiler_contract_version + capability_map_version + skill_manifest_digest` 一致性；不一致 fail-closed。
6. [ ] 规则匹配解析只允许用于诊断日志，不得作为提交决策主链路。

## 5. 接口契约 (API Contracts)
### 5.1 内部接口（Go）
- `ModelGateway.ResolveIntent(ctx, request) (IntentResult, error)`
- `ProviderAdapter.Invoke(ctx, prompt, schema) (StructuredOutput, error)`
- `IntentBoundaryLint.Validate(intent) error`
- `IntentToCommandCompiler.Compile(intent, context) (SkillExecutionPlan, ConfigDeltaPlan, error)`
- `DeterminismGuard.Verify(input, output, snapshots) (DeterminismReport, error)`

### 5.2 新增内部 API（模型配置页）
1. [ ] `GET /internal/assistant/model-providers`：读取 provider 配置与健康状态。
2. [ ] `POST /internal/assistant/model-providers:validate`：保存前校验（endpoint/model/priority/key_ref）。
3. [ ] `POST /internal/assistant/model-providers:apply`：应用配置（含审计字段）。
4. [ ] `GET /internal/assistant/models`：返回可用模型清单（供 UI/LibreChat 对齐）。

### 5.3 会话 API（沿用 + 语义增强）
- 继续沿用 `/internal/assistant/conversations/*`。
- 语义增强为：支持多轮上下文恢复、意图链路结构化回显、模型路由元数据审计。
- 返回体必须带出：`intent_schema_version`、`compiler_contract_version`、`capability_map_version`、`skill_manifest_digest`、`context_hash`、`intent_hash`、`plan_hash`。

### 5.4 错误码契约（修订）
1. [ ] strict decode 失败 -> `ai_plan_schema_constrained_decode_failed`
2. [ ] boundary lint 失败 -> `ai_plan_boundary_violation`
3. [ ] provider 全部不可用 -> `ai_model_provider_unavailable`
4. [ ] provider 超时 -> `ai_model_timeout`
5. [ ] provider 限流 -> `ai_model_rate_limited`
6. [ ] 模型配置非法 -> `ai_model_config_invalid`
7. [ ] 模型密钥缺失 -> `ai_model_secret_missing`
8. [ ] 契约版本不一致 -> `ai_plan_contract_version_mismatch`
9. [ ] 确定性校验失败 -> `ai_plan_determinism_violation`

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 provider 路由与回退算法（伪代码）
```text
providers = enabled providers ordered by priority
for p in providers:
  result, err = invoke(p)
  if success: return result
  if err in {timeout, rate_limit, transient}: continue
  if err in {config_invalid, secret_missing}: break
return ai_model_provider_unavailable
```

### 6.2 意图 -> 操作指令主链路（伪代码）
```text
raw = modelGateway.resolve(prompt, output_schema)
intent = strictDecode(raw, RequirementIntentSpec)
if decodeFail: return ai_plan_schema_constrained_decode_failed

if boundaryLintFail(intent): return ai_plan_boundary_violation

skillPlan, deltaPlan = compiler.compile(intent, context)
dryRun = runDryRun(deltaPlan)
hashes = canonicalHash(context, intent, deltaPlan)
if !determinismGuard.pass(hashes, versions): return ai_plan_determinism_violation

return {
  intent_spec: intent,
  skill_execution_plan: skillPlan,
  config_delta_plan: deltaPlan,
  dry_run_result: dryRun,
  context_hash: hashes.context,
  intent_hash: hashes.intent,
  plan_hash: hashes.plan
}
```

### 6.3 多轮会话上下文算法（伪代码）
```text
ctx = loadConversationContext(conversation_id)
ctx = compact(ctx, max_turns=N, summary_token_budget=M)
input = buildPrompt(ctx, new_user_input, policy_snapshots)
result = resolveIntent(input)
appendTurn(conversation_id, result)
```

### 6.4 LibreChat 对齐算法（消息桥接）
```text
onMessage(event):
  if origin not in allowlist: drop
  if !schemaValid(event.data): drop
  if nonce/channel mismatch: drop
  dispatch event to assistant workspace state
```

### 6.5 诊断降级策略
- 仅允许“诊断日志级”规则解析用于故障定位；不得作为静默业务提交路径。

### 6.6 确定性守护算法（新增）
```text
normalize(input, context, snapshots) -> canonical_input
result = llm(strict_schema, deterministic_params)
intent = strictDecode(result)
plan = compile(intent, canonical_input)
hashes = hash(canonical_input, intent, plan)
if contractVersionChanged(): return ai_plan_contract_version_mismatch
if replay(canonical_input).plan_hash != hashes.plan: return ai_plan_determinism_violation
return plan + hashes
```

## 7. 安全与鉴权 (Security & Authz)
1. [ ] 密钥仅环境注入，日志掩码处理，禁止明文输出。
2. [ ] provider endpoint 白名单校验，禁止任意 URL 注入。
3. [ ] assistant capability 与路由映射不漂移。
4. [ ] 模型输出必须 boundary lint，禁止越界能力进入提交链路。
5. [ ] LibreChat iframe 通信必须通过 origin/schema/nonce-channel 三重校验。
6. [ ] LibreChat 只允许访问 `internal/assistant/*` 编排接口，禁止业务写旁路。
7. [ ] 反向代理与路由层对 `assistant-ui` 到业务写路由实施硬阻断（非“仅测试约束”）。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**：
  - `DEV-PLAN-221` 错误码与状态机契约冻结。
  - `DEV-PLAN-222` iframe/postMessage 安全契约与 FE/E2E 收口。
  - `DEV-PLAN-223` 会话持久化与审计字段落地（支撑多轮恢复）。
  - M4 启动条件：223 完成并通过其门禁与证据验收。
- **里程碑**：
  1. [ ] M1：Provider 配置契约 + `ModelGateway` 接口冻结（含配置页 IA）。
  2. [ ] M2：四类 provider adapter + 健康检查 + 错误归一化实现。
  3. [ ] M3：意图到指令链路切换（strict decode + boundary lint + compiler）。
  4. [ ] M4：多轮会话恢复接入（对接 223 持久化能力）。
  5. [ ] M5：LibreChat 对齐收口（模型切换/会话恢复/消息安全）+ 证据归档。

## 9. 测试与验收标准 (Acceptance Criteria)
- **单元测试**：
  1. [ ] 配置加载校验（字段缺失、非法 provider、priority 冲突、密钥缺失）。
  2. [ ] provider adapter 错误归一化（timeout/rate_limit/unavailable/config_invalid）。
  3. [ ] strict decode、boundary lint、compiler 分支覆盖。
  4. [ ] 确定性守护：同输入同快照重复执行 20 次 `plan_hash` 一致。
- **集成测试**：
  1. [ ] 多 provider 路由与受控回退一致性。
  2. [ ] 同输入在同配置下输出 `IntentSpec/SkillPlan/DeltaPlan` 结构稳定。
  3. [ ] 会话恢复后多轮上下文一致（重启前后行为一致）。
  4. [ ] 契约版本漂移命中时稳定拒绝（`ai_plan_contract_version_mismatch`）。
- **前端/E2E**：
  1. [ ] 模型配置页可完成“查看/校验/应用”闭环。
  2. [ ] `/app/assistant` 多轮会话可恢复（会话列表 + 回放）。
  3. [ ] postMessage 三重校验均有自动化断言。
  4. [ ] 右侧事务面板展示 `*_version` 与 `*_hash`，并支持“版本不一致”阻断提示。
- **验收对齐**：
  1. [ ] 对齐 `TC-220-BE-003/004` 及 220A 的多模型/意图缺口项。
  2. [ ] 新增“多模型切换一致性”“intent->command 稳定性”“会话恢复一致性”证据。
  3. [ ] `make preflight` 全绿。
  4. [ ] 高信心阈值：同输入同快照一致率 ≥ 99.5%，schema/boundary 误放行 = 0，版本漂移拦截覆盖率 = 100%。

## 10. 运维与监控 (Ops & Monitoring)
- 不引入复杂开关平台，遵循仓库“早期最小运维”原则。
- 最小可观测要求：
  1. [ ] 请求级记录 `provider/model/revision/latency_ms/error_code/request_id/trace_id/conversation_id/turn_id`。
  2. [ ] 统计 provider 成功率/超时率/限流率（用于路由优先级调优）。
  3. [ ] 统计 intent->command 拒绝率（decode 失败、boundary 失败、编译失败）。
  4. [ ] 统计 `contract_version_mismatch` / `determinism_violation` 拒绝率并保留样本快照。
  5. [ ] 故障处置遵循 No-Legacy：环境级保护 → 只读/停写 → 修复 → 重试/重放 → 恢复。

## 11. 交付物
1. [ ] 多模型治理配置与 `ModelGateway` 代码。
2. [ ] 模型配置页面（MUI）与对应 API。
3. [ ] LLM 意图识别与“意图到操作指令”主链路实现。
4. [ ] 与 223 对齐的多轮会话恢复接入与测试证据。
5. [ ] LibreChat 对齐能力（模型选择、会话恢复、消息安全）与 E2E 证据。
6. [ ] `DEV-PLAN-224` 执行记录文档（新增到 `docs/dev-records/`）。
7. [ ] 意图链路确定性证据包（样本集、版本矩阵、哈希一致性报告）。

## 12. 实施任务拆解（可执行）
### 12.1 M1：模型配置契约与配置页（P0）
1. [ ] 新增页面路由：`/app/assistant/models`（仅 assistant 管理权限可见）。
2. [ ] 新增前端页面模块（MUI）：
   - [ ] Provider 列表（name/enabled/model/endpoint/priority/timeout_ms/retries）。
   - [ ] 配置校验与错误提示（字段级 + 全局级）。
   - [ ] 健康状态展示（healthy/degraded/unavailable）。
3. [ ] 新增后端 API：
   - [ ] `GET /internal/assistant/model-providers`
   - [ ] `POST /internal/assistant/model-providers:validate`
   - [ ] `POST /internal/assistant/model-providers:apply`
   - [ ] `GET /internal/assistant/models`
4. [ ] 新增配置装载器与校验器（启动校验 fail-closed）。
5. [ ] 定义配置生效语义（热加载 + 原子切换 + 回滚），保证“应用后可审计且重启一致”。
6. [ ] 路由治理与授权映射更新：
   - [ ] `config/routing/allowlist.yaml`
   - [ ] `config/capability/route-capability-map.v1.json`
   - [ ] capability catalog / authz requirement 对齐。

### 12.2 M2：ModelGateway 与多 provider adapter（P0）
1. [ ] 实现 `ModelGateway`（priority failover、timeout/retry、错误归一化）。
2. [ ] 实现 adapter（openai/deepseek/claude/gemini）统一调用契约。
3. [ ] 实现 provider 健康检查与最小指标采集（成功率/超时率/限流率）。
4. [ ] 错误码收敛到统一目录：
   - [ ] `ai_model_provider_unavailable`
   - [ ] `ai_model_timeout`
   - [ ] `ai_model_rate_limited`
   - [ ] `ai_model_config_invalid`
   - [ ] `ai_model_secret_missing`

### 12.3 M3：意图到操作指令链路（P1）
1. [ ] 定义并冻结 `RequirementIntentSpec` schema（additionalProperties=false）。
2. [ ] 实现 strict decode（失败返回 `ai_plan_schema_constrained_decode_failed`）。
3. [ ] 实现 boundary lint（失败返回 `ai_plan_boundary_violation`）。
4. [ ] 实现 `IntentToCommandCompiler`：
   - [ ] 输出 `SkillExecutionPlan`
   - [ ] 输出 `ConfigDeltaPlan`
   - [ ] 输出 `DryRunResult`
5. [ ] 冻结并注入版本字段：`intent_schema_version`、`compiler_contract_version`、`capability_map_version`、`skill_manifest_digest`。
6. [ ] 实现规范化与哈希：`context_hash`、`intent_hash`、`plan_hash`，并进入 API 返回与审计日志。
7. [ ] 实现确定性守护：
   - [ ] 推理参数冻结（temperature/top_p/n）。
   - [ ] 编译器纯函数化与稳定排序。
   - [ ] 回放对账（同输入同快照）失败返回 `ai_plan_determinism_violation`。
8. [ ] 右侧事务面板改为消费结构化结果（不允许前端本地拼写提交命令）。

### 12.4 M4：多轮会话恢复（依赖 223）
1. [ ] 对接 223 的持久化会话读取能力（conversation/turn/transition/idempotency）。
2. [ ] 实现上下文构建器：
   - [ ] 最近 N 轮窗口 + 摘要压缩。
   - [ ] 候选主键与版本快照注入。
3. [ ] 会话恢复与回放：
   - [ ] 重启后可继续同会话。
   - [ ] 同 `request_id` 幂等重试不重复提交。
4. [ ] 漂移检测：
   - [ ] `policy/composition/mapping` 漂移命中后强制回退 `validated`。

### 12.5 M5：LibreChat 对齐收口（P1/P2 前置）
1. [ ] 消息桥接安全落地（origin/schema/nonce-channel 三重校验）。
2. [ ] 模型选择与平台配置联动（左侧聊天壳与右侧状态一致）。
3. [ ] 多轮会话体验对齐（会话列表、恢复、回放定位）。
4. [ ] 边界验证：LibreChat 不得直接访问业务写路由，仅可访问 `internal/assistant/*`。

### 12.6 测试任务清单（与门禁一一对应）
1. [ ] 单元测试：配置校验、网关回退、decode/lint/compiler。
2. [ ] 集成测试：多 provider 切换一致性、会话恢复一致性。
3. [ ] 前端测试：模型配置页表单校验与错误映射。
4. [ ] E2E：模型切换、多轮恢复、消息桥接安全、越权阻断。
5. [ ] 确定性回归：固定样本集重复回放、跨 provider 结构等价性、版本漂移阻断。
6. [ ] 门禁命令：
   - [ ] `make check routing`
   - [ ] `make check capability-route-map`
   - [ ] `make authz-pack && make authz-test && make authz-lint`
   - [ ] `make check error-message`
   - [ ] `make check assistant-intent-determinism`
   - [ ] `make test`
   - [ ] `make e2e`
   - [ ] `make preflight`
   - [ ] `make check doc`

## 13. 关联文档
- `docs/dev-plans/001-technical-design-template.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
- `docs/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- `docs/dev-plans/209-blueprint-skill-manifest-tool-whitelist-and-risk-tier.md`
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `AGENTS.md`
