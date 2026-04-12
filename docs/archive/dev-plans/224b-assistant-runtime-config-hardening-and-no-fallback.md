# DEV-PLAN-224B：Assistant 运行时配置强制生效与无回退硬化方案

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 草拟中（2026-03-03 07:20 UTC）

## 1. 背景与问题复盘
- 关联计划：
  - `docs/archive/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
  - `docs/archive/dev-plans/224a-assistant-codex-live-api-and-multi-turn-workspace-plan.md`
- 现状问题（本方案直接针对）：
  1. [ ] **配置未稳定进入运行时**：当前模型配置既有启动默认值兜底，也有进程内 apply（重启丢失）路径，容易出现“页面已配置但实际未生效”的错觉。
  2. [ ] **存在内置规则回退路径**：在 endpoint 为 `builtin://*`、`simulate://*` 或 gateway 不可用时，仍可落到 deterministic/rule-based 解析，导致未调用真实模型也能继续流程。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 解决“配置没有真正进入运行时”：模型配置必须可验证、可观测、可追溯，且实际参与 turn 解析。
2. [ ] 删除默认回退与内置规则解析：Assistant 意图解析必须走真实模型运行时；缺配置或不可用时明确报错并 fail-closed。

### 2.2 非目标
1. [ ] 不放宽 One Door、confirm/commit、RLS、Authz 既有边界。
2. [ ] 不引入 legacy 双链路（禁止保留“仅应急可切回 builtin/rule-based”的开关）。

## 3. 强制不变量（224B 生效后）
1. [ ] **No Config, No Start**：无有效模型运行时配置时，Assistant 服务启动即失败（不再静默回落默认配置）。
2. [ ] **No Model, No Turn**：创建 turn 时若无可用真实模型，返回明确错误码；不得以内置规则继续。
3. [ ] **No Builtin Endpoint in Runtime**：运行时配置禁止 `builtin://*` 与 `simulate://*`。
4. [ ] **No Nil Gateway Fallback**：`modelGateway == nil` 不再触发 rule-based 路径，直接报错。
5. [ ] **No Env-based Deterministic Adapter**：`test/dev/prod` 运行时统一走真实 provider adapter，不得按环境自动替换为 deterministic。
6. [ ] **Healthy Means Reachable**：模型治理页 `healthy` 必须来自真实连通性探测（例如 `/models` 探测），不得仅靠本地静态校验。
7. [ ] **Schema Incomplete Is Recoverable**：意图结构不完整时必须进入“可继续对话补充信息”状态，不得直接以 422 终止会话。

## 4. 方案设计
### 4.1 运行时配置“强制生效”改造
1. [ ] 将 `assistantModelGateway` 初始化改为显式返回错误（`newAssistantModelGateway() (*assistantModelGateway, error)`），禁止 silent ignore。
2. [ ] 启动阶段必须完成配置加载 + 校验（provider 白名单、HTTPS endpoint、key_ref、secret 存在、priority 唯一）；任一失败直接阻断 `NewHandler`。
3. [ ] 统一“生效配置快照”输出：
   - 启动日志打印 `config_source/config_revision/config_hash/enabled_providers`（不打印密钥）；
   - `GET /internal/assistant/model-providers` 返回当前运行时 revision/hash，供 UI 与排障对账。
4. [ ] `POST /internal/assistant/model-providers:apply` 语义改为“校验通过后即时替换运行时配置并返回 revision/hash”；失败不得部分生效。
5. [ ] 文档与运维口径统一：本地必须通过 `make dev-server`（确保环境注入路径一致），避免直接 `go run ./cmd/server` 导致变量缺失。
6. [ ] `GET /internal/assistant/model-providers` 的 `healthy/health_reason` 必须包含真实探测结果（超时/鉴权失败/连通失败），而非仅 endpoint/key_ref 语法检查。

### 4.2 删除默认回退与内置解析
1. [ ] 移除 `defaultAssistantModelConfig()` 的 dev 默认 `builtin://openai` 行为；运行时不再自动注入可执行默认 provider。
2. [ ] 移除 openai adapter 对 `builtin://*` / `simulate://*` 的 fallback 分支。
3. [ ] 移除 `resolveIntent` 中 `s == nil || s.modelGateway == nil` 的 rule-based 解码回退，改为 `ai_model_provider_unavailable`（或新错误码）直接失败。
4. [ ] 仅允许真实 provider adapter 执行；若 provider 配置了但 adapter 缺失，视为配置错误并 fail-closed。
5. [ ] 单测中如需桩实现，使用测试注入 adapter 或本地 mock OpenAI-compatible server（test-only），不通过生产回退逻辑实现。

### 4.3 错误码与前端提示收敛
1. [ ] 新增/复用错误码并文案收敛：
   - `ai_runtime_config_missing`（缺少运行时配置）
   - `ai_runtime_config_invalid`（配置非法）
   - `ai_model_provider_unavailable`（真实 provider 不可用）
2. [ ] `/app/assistant` 明确区分“配置缺失/配置非法/模型不可用”，禁止泛化“生成失败”。

### 4.4 严格结构化约束下的多轮补全
1. [ ] 对“缺少必填字段/日期格式不合法”场景，`create turn` 返回 200，并在 `dry_run.validation_errors` 返回缺失项（如 `missing_parent_ref_text` / `missing_entity_name` / `missing_effective_date`）。
2. [ ] `dry_run.explain` 提供可执行补充指引（明确提示下一轮需要补什么），而不是返回 `ai_plan_schema_constrained_decode_failed`。
3. [ ] `confirm/commit` 双保险：存在必填缺失时必须阻断提交（返回 `conversation_confirmation_required`），直到用户补齐信息并重新生成回合。
4. [ ] 前端将 `validation_errors` 映射成人话提示，并禁用 Confirm/Commit，避免“可点但必失败”体验。

## 5. 实施切片（建议）
### PR-224B-01：启动硬失败 + 配置可观测
1. [ ] 网关初始化显式报错，接入 `NewHandler` 启动门禁。
2. [ ] 增加 runtime revision/hash 暴露与日志。
3. [ ] 补齐配置校验单测（缺变量、非法 endpoint、重复 priority、secret 缺失）。

### PR-224B-02：删除回退路径
1. [ ] 删除 builtin/simulate fallback 与 nil gateway rule-based fallback。
2. [ ] 调整 createTurn 错误映射与 API 契约测试。
3. [ ] 清理/重写依赖 fallback 语义的测试用例。

### PR-224B-03：前端与文档收口
1. [ ] Assistant 页面补齐错误提示分层。
2. [ ] 更新 dev-plan 与执行记录文档，补全门禁证据。

## 6. 测试与验收标准
1. [ ] `go test ./internal/server -run Assistant -count=1` 全绿，且新增以下必测：
   - 启动时无有效模型配置 -> handler 初始化失败；
   - 配置为 `builtin://*` / `simulate://*` -> 校验失败；
   - `modelGateway=nil` 或 provider 不可用 -> 创建 turn 返回明确错误，不再进入 rule-based；
   - 配置 apply 成功后，后续 turn 的 `plan.model_provider/model_name/model_revision` 与 runtime snapshot 一致。
2. [ ] 前端/API 测试验证错误提示准确性（对齐 `make check error-message`）。
3. [ ] `make check routing && make check capability-route-map && make check error-message` 通过。

## 7. 风险与回滚
1. [ ] 风险：去回退后，模型平台不可用会直接影响 turn 创建。
2. [ ] 缓解：通过可观测的 health/revision/hash 快速定位，并采用“环境级保护 + 停写/重试”处理，不引入代码回退链路。
3. [ ] 回滚原则：仅允许回滚到“上一版本真实模型配置”，禁止回滚到 builtin/rule-based 实现。

## 8. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/archive/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/archive/dev-plans/224a-assistant-codex-live-api-and-multi-turn-workspace-plan.md`
