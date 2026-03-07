# DEV-PLAN-224C：Assistant 可扩展意图注册表与多场景受控提交实施计划

**状态**: 草拟中（2026-03-03 09:36 UTC）

> 口径说明（2026-03-08）：本计划只约束“意图注册表与受控提交”能力，不再绑定旧 `/app/assistant` 工作台 IA。正式对话入口与承载面以 `DEV-PLAN-280/283/284/266` 为准。

## 1. 背景与问题
- 关联计划：
  - `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
  - `docs/dev-plans/224a-assistant-codex-live-api-and-multi-turn-workspace-plan.md`
  - `docs/dev-plans/224b-assistant-runtime-config-hardening-and-no-fallback.md`
  - `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- 现状（基于当前实现）：
  1. [ ] 意图动作在提交链路仍是单一口径：`create_orgunit`（仅固定用途高通过率）。
  2. [ ] 解析与编译仍偏硬编码：`action -> capability/skill/dry-run` 主要围绕组织创建分支。
  3. [ ] 用户非“创建组织”类提示词容易落入 `plan_only` 或在 commit 阶段触发 `assistant_intent_unsupported`。
  4. [ ] 结果是“聊天看似通用，提交实则单场景”，与 224 的“意图到指令可扩展治理”目标存在落差。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 将 Assistant 从“单意图硬编码”升级为“意图注册表驱动”，在不破坏边界的前提下支持多业务场景。
2. [ ] 保持 strict decode + boundary lint + contract snapshot + determinism guard 全链路 fail-closed。
3. [ ] 让新意图接入具备模板化流程：`schema -> validate -> compile -> dry-run -> confirm gate -> commit executor -> 审计`。
4. [ ] 首批接入至少 3 类可提交意图（在现有 capability 注册范围内），并补齐 FE/BE/E2E 证据。

### 2.2 非目标
1. [ ] 不新增写入旁路；所有写入仍必须走现有 One Door 写服务。
2. [ ] 不放宽租户/RLS/Authz 边界，不引入 legacy 双链路。
3. [ ] 不在本计划引入新外部基础设施（缓存/消息系统/模型代理层扩容）。

## 3. 强制不变量
1. [ ] **No Registry, No Commit**：未注册 action 不得进入 commit。
2. [ ] **No Capability, No Plan**：action 必须映射到已注册 capability_key，否则 `ai_plan_boundary_violation`。
3. [ ] **No Schema, No Turn**：模型输出不满足 action schema，直接 `ai_plan_schema_constrained_decode_failed`。
4. [ ] **No Confirm, No High-Risk Commit**：高风险或候选歧义 action 必须先确认。
5. [ ] **No Drift, No Commit**：版本漂移/契约不一致继续按既有规则回退到 `validated`。

## 4. 总体方案
### 4.1 意图契约升级：`assistant.intent.v2`
1. [ ] 采用 `action` 判别 + `payload` 结构化载荷（按 action 约束字段），禁止自由扩展字段。
2. [ ] strict decode 仍要求 `additionalProperties=false`，并保留机器可读错误码。
3. [ ] `context_hash/intent_hash/plan_hash` 计算口径升级到 v2，但保持确定性与可回放。

### 4.2 引入 `IntentRegistry`
1. [ ] 定义统一注册接口（示意）：
   - `Validate(intent) error`
   - `Compile(intent, context) (SkillExecutionPlan, ConfigDeltaPlan, error)`
   - `BuildDryRun(intent, context) (DryRunResult, error)`
   - `ConfirmPolicy(intent, context) (required bool, reason string)`
   - `Commit(ctx, intent, resolvedContext) (CommitResult, error)`
2. [ ] `createTurn` 与 `createTurnPG` 统一改为“按 action 查 registry”执行，不再写死 `if action == create_orgunit`。
3. [ ] `commitTurn` 与 `applyCommitTurn` 统一改为 executor 分发，不再写死单 action。

### 4.3 首批 action 批次（受 capability 约束）
1. [ ] `create_orgunit`（现有能力，作为基线回归）。
2. [ ] `add_orgunit_version`（映射 `org.orgunit_add_version.field_policy`）。
3. [ ] `insert_orgunit_version`（映射 `org.orgunit_insert_version.field_policy`）。
4. [ ] `correct_orgunit`（映射 `org.orgunit_correct.field_policy`）。
5. [ ] 上述 action 命名与 payload 字段在实施前冻结到 capability catalog 与前端文案。

### 4.4 模型输出与边界治理
1. [ ] OpenAI/多 provider 输出 schema 改为多 action 联合约束（单入口 strict schema，不做自由文本兜底）。
2. [ ] `assistantNormalizeOpenAIIntentAction` 扩展同义词归一化，但归一后必须命中 registry。
3. [ ] boundary lint 新增 action 级字段白名单与 capability 反查，杜绝“模型幻觉 action”。

### 4.5 前端消费面适配（不绑定旧工作台）
1. [ ] 正式对话页内的事务交互区改为 action 感知：标题、dry-run diff、确认阻断提示均动态渲染。
2. [ ] 生成示例提示词不再固定“创建组织”单模板，改为多 action 样例切换。
3. [ ] 错误提示按 action 分类准确映射，避免“全部提示生成失败”。

## 5. 实施切片（PR 建议）
### PR-224C-01：注册表骨架与无行为变更迁移
1. [ ] 落地 `IntentRegistry`/`IntentExecutor` 接口与 wiring。
2. [ ] 仅迁移 `create_orgunit` 到 registry，保证行为与响应字段不退化。
3. [ ] 完成回归测试：create/confirm/commit 全链路与错误码不变。

### PR-224C-02：`assistant.intent.v2` 与 strict schema 升级
1. [ ] 升级模型 schema 与 strict decode 到 v2。
2. [ ] 升级 hash 计算输入（含 action/payload）并补齐确定性测试。
3. [ ] 旧分支硬编码解析逻辑下线，避免双链路长期共存。

### PR-224C-03：首批多 action 接入（后端）
1. [ ] 接入 `add_orgunit_version/insert_orgunit_version/correct_orgunit` registry + compiler + executor。
2. [ ] 对齐 capability catalog、route-capability-map、authz 要求。
3. [ ] 补齐 commit 前置校验（候选确认、版本漂移、策略版本快照）。

### PR-224C-04：前端与 E2E 收口
1. [ ] 按当前正式入口（以 `283` 切换结果为准）改造 action 感知 UI，新增多 action 示例与断言。
2. [ ] 新增 E2E：不同 action 的 generate -> confirm -> commit 闭环。
3. [ ] 收口错误码文案与证据文档。

## 6. 测试与覆盖率
1. [ ] 覆盖率口径：沿用仓库当前 CI 口径与阈值（以 `Makefile` 与 CI workflow 为准）。
2. [ ] 后端测试（`internal/server`）至少覆盖：
   - registry 命中/未命中；
   - action schema decode 失败；
   - boundary lint 拦截；
   - 多 action compile/dry-run/commit；
   - 版本漂移与契约漂移回退。
3. [ ] 前端测试（assistant 页面）至少覆盖：
   - action 动态渲染；
   - confirm/commit 按 action 门控；
   - 错误码到提示映射。
4. [ ] E2E 至少覆盖三类 action 的端到端闭环与失败路径（权限不足、候选缺失、越界 action）。

## 7. 验收标准（DoD）
1. [ ] 非创建类业务提示词可被稳定解析为已注册 action，并生成结构化计划。
2. [ ] commit 链路不再仅限 `create_orgunit`，而是 registry 中的受控 action。
3. [ ] 未注册 action / 未映射 capability / 非法 schema 均稳定 fail-closed。
4. [ ] 224/224A/224B 既有不变量（One Door、RLS、Authz、determinism）无回退。
5. [ ] 相关门禁与测试通过，并在 `docs/dev-records/` 产出执行证据。

## 8. 风险与回滚策略
1. [ ] 风险：一次性扩 action 可能放大测试面与回归成本。
2. [ ] 缓解：按 PR 切片渐进接入，先“骨架迁移不变更行为”，再增 action。
3. [ ] 回滚：仅允许回滚到“上一稳定 registry 版本”，不恢复硬编码双链路。

## 9. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/224a-assistant-codex-live-api-and-multi-turn-workspace-plan.md`
- `docs/dev-plans/224b-assistant-runtime-config-hardening-and-no-fallback.md`
