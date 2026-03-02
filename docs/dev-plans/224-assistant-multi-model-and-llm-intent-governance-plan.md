# DEV-PLAN-224：Assistant 多模型适配与 LLM 意图判断能力实施计划

**状态**: 规划中（2026-03-02 05:37 UTC）

## 1. 背景
- `DEV-PLAN-220A` 补充评估确认：当前仓库仅实现 LibreChat 反向代理壳，未形成平台侧多模型治理，也未实现真实 LLM 意图判断链路。
- 本计划目标是把“聊天壳”升级为“可治理的模型能力平台入口”。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 建立平台侧多模型配置治理能力（OpenAI / DeepSeek / Anthropic Claude / Google Gemini）。
2. [ ] 建立模型路由与健康检查最小机制（可用性、超时、回退策略）。
3. [ ] 建立 LLM 意图识别 + strict decode + boundary lint 的确定性输出链路。
4. [ ] 保持 `confirm/commit/re-auth/One Door` 边界不变。

### 2.2 非目标
1. [ ] 不把业务提交裁决下放给模型。
2. [ ] 不在本计划实现 Temporal 异步编排（由 `DEV-PLAN-225` 负责）。

## 3. 实施范围
- 配置与密钥治理：provider 配置结构、可用性开关、安全存储策略（仅本地与部署注入，不入库明文）。
- 模型网关：统一调用接口、超时/重试、provider fallback、错误归一化。
- 语义层：LLM 输出 schema、strict decode、能力边界校验、候选主键确认协议衔接。

## 4. 实施步骤
1. [ ] 冻结 provider 配置契约（含 provider 名称、模型标识、endpoint、超时、重试、权重/优先级）。
2. [ ] 实现模型网关抽象（Provider Adapter + 统一响应结构）。
3. [ ] 增加 provider 健康探针与路由策略（主路由失败 -> 受控回退）。
4. [ ] 实现意图识别链路：
   - Prompt 模板与输出 schema
   - strict decode 失败 -> `ai_plan_schema_constrained_decode_failed`
   - boundary 违约 -> `ai_plan_boundary_violation`
5. [ ] 替换现有规则解析为“LLM 主路径 + 规则兜底最小路径（仅诊断/降级）”。
6. [ ] 补齐安全与审计：记录 `provider/model/latency/error_code/request_id/trace_id`。

## 5. 测试与验收
1. [ ] provider 级单测：OpenAI/DeepSeek/Claude/Gemini 配置加载、连接失败、超时、fallback。
2. [ ] 意图链路测试：合法输出、schema 违约、边界违约、多候选歧义。
3. [ ] 对齐 `TC-220-BE-003/004` 并新增“多模型切换一致性”测试。
4. [ ] 验收标准：模型可插拔、意图输出可约束、错误可解释、边界 fail-closed。

## 6. 风险与缓解
- **外部模型不稳定**：多 provider fallback + 熔断 + 超时上限。
- **输出漂移**：strict decode + deterministic lint 双保险。
- **密钥安全风险**：仅环境注入，禁止入库与日志明文。

## 7. 交付物
1. [ ] 多模型治理配置与模型网关代码。
2. [ ] LLM 意图识别与 strict decode/边界校验链路。
3. [ ] `DEV-PLAN-224` 执行记录文档（实施时新增到 `docs/dev-records/`）。

## 8. 关联文档
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `AGENTS.md`
