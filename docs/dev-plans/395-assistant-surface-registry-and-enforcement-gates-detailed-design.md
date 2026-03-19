# DEV-PLAN-395：Assistant 全平台覆盖目录与强制门禁详细设计

**状态**: 规划中（2026-03-19 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 的 `395` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“Assistant 是横切能力、不能形成第二解释链、不能替代业务主写边界”的冻结；
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md) 对 Assistant contract / integration / E2E 测试合同的冻结；
- [DEV-PLAN-313](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md) 对 `QualityGatePipeline` 与 required checks 执行链的冻结；
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 对 capability、`assistant_action_id`、颗粒度词汇与结构门禁的冻结；
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 对 Assistant 安全 stopline 的冻结。

`390` 已明确“正式 capability 与高价值查询面不能形成暗面能力”，但如果没有 `395`，后续实现仍然容易继续出现：

- 模块功能已交付，但没有声明 Assistant 支持级别；
- action request 已可用，但 `assistant_action_id`、`handoff_route`、`receipt_type` 没有同源目录；
- 测试层只验证“聊天能说话”，不验证“新 capability 必须进 Assistant 覆盖目录”；
- 流水线层把 Assistant 门禁视为“附加测试”，长期停留在手工联调或 `skipped`。

`395` 的职责是把这些问题收敛为 **可执行、可阻断、可纳入 CI required checks 的 Assistant 横切门禁合同**。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结 `assistant_surface_registry` 的最小字段合同与单一事实源落点。
- [ ] 冻结 Assistant 支持级别词汇：`ask_only / read / draft / action_request / status_track / ui_handoff`。
- [ ] 冻结“无暗面能力”规则：正式 capability 与高价值查询面必须显式声明 Assistant 支持级别。
- [ ] 冻结 `390/395/347/312/313` 的门禁 responsibility split，避免各子计划对“谁负责卡住 Assistant 漂移”再次漂移。
- [ ] 冻结 Assistant 结构门禁、contract tests、cross-slice smoke 的触发矩阵与 required checks 绑定要求。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 的 capability 命名与静态映射治理；`395` 只定义 Assistant 覆盖目录和 Assistant 侧门禁语义。
- [ ] 本计划不替代 [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md) 的测试分层与 fixture 合同。
- [ ] 本计划不替代 [DEV-PLAN-313](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md) 的流水线编排与 OCI 交付基线。
- [ ] 本计划不要求所有能力都必须在聊天内一步直达完成；高风险或高密度交互能力允许采用 `ui_handoff`。

## 3. 核心对象与 ownership

| 对象 | 含义 | 共享 owner |
| --- | --- | --- |
| `assistant_surface_registry` | Assistant 覆盖目录与支持级别清单 | `395` |
| `AssistantSupportLevel` | Assistant 对 capability/query surface 的支持级别词汇 | `395` |
| `AssistantGateTrigger` | 什么类型的变更必须触发哪些 Assistant 门禁 | `395` |
| `AssistantStaticGate` | 结构门禁：目录、映射、handoff contract | `347`（语义输入由 `395` 提供） |
| `AssistantContractTest` | Assistant contract/integration/E2E 测试套件 | `312`（语义输入由 `395` 提供） |
| `AssistantCoverageGateBinding` | Assistant 门禁与 required checks 的绑定规则 | `313`（语义输入由 `395` 提供） |

## 4. 支持级别与目录合同

### 4.1 `assistant_surface_registry` 最小字段

建议至少冻结以下字段：

- `capability_key`
- `business_object_key`
- `support_level`
- `assistant_action_id`
- `handoff_route`
- `receipt_type`
- `risk_level`
- `owner_module`
- `required_permission_bundle`

其中：

- `assistant_action_id` 仅在 `support_level=action_request` 时必填；
- `handoff_route` 仅在 `support_level=ui_handoff` 时必填；
- `receipt_type` 在 `support_level=status_track` 或 `action_request` 时必填。

### 4.2 支持级别词汇（冻结）

- `ask_only`：可解释、可问答，但不承诺稳定读对象。
- `read`：可稳定读取对象或结果面。
- `draft`：可生成建议、草稿、确认摘要，但不直接进入受控动作票据。
- `action_request`：可发起受控动作请求，必须命中 `assistant_action_id -> capability_key`。
- `status_track`：可查询动作、任务、批次或回执状态。
- `ui_handoff`：聊天内不直接完成，但必须能回落到同源 UI/确认面/工作台。

### 4.3 无暗面能力规则（冻结）

- `340/360/370/380` 的正式 capability 与高价值查询面都必须在 `assistant_surface_registry` 中声明支持级别。
- “未声明”不是一种合法状态；不得以“暂不支持”作为默认空值。
- 仅对纯内部技术能力允许不进入目录，但必须明确不属于用户可感知 capability 或高价值查询面。

## 5. 变更触发矩阵

| 变更类型 | 必须更新什么 | 必须触发的门禁 |
| --- | --- | --- |
| 新增/修改 active capability | `assistant_surface_registry` 支持级别与字段 | `assistant-surface-catalog` |
| 新增/修改 `assistant_action_id` | Action 映射与覆盖目录 | `assistant-action-capability-map` |
| 新增/修改 handoff route / 工作台入口 | `handoff_route` 与回落面 | `assistant-handoff-contract` |
| 新增/修改高价值 query surface / report / export / batch | `support_level`、`receipt_type`、必要 handoff | `assistant-surface-catalog` + Assistant contract tests |
| 命中 Assistant 主路径的用户切片变更 | cross-slice smoke 期望 | `assistant-cross-slice-smoke` |

## 6. 门禁 responsibility split

### 6.1 `390`

- 定义 Assistant 横切业务目标；
- 冻结“全平台可寻址”“无暗面能力”的上位不变量；
- 不直接拥有静态门禁实现、测试框架实现或流水线 YAML。

### 6.2 `395`

- 冻结 `assistant_surface_registry`、支持级别、触发矩阵与门禁证据口径；
- 负责回答“什么能力必须进入目录、目录里至少要写什么、命中何种变更必须触发何种门禁”。

### 6.3 `347`

- 把 `395` 的目录合同翻译成静态结构门禁；
- 至少应承接：
  - `assistant-surface-catalog`
  - `assistant-action-capability-map`
  - `assistant-handoff-contract`

### 6.4 `312`

- 把 `395` 的 Assistant 覆盖合同翻译成 contract / integration / E2E 测试义务；
- 至少应承接：
  - Assistant capabilities contract tests
  - Assistant cross-slice smoke specs

### 6.5 `313`

- 把 `347` 与 `312` 产出的 Assistant 门禁绑定进统一 `QualityGatePipeline`；
- 保证命中 Assistant 触发矩阵的变更不能以 `skipped`、人工验证或手工补证据视为通过。

## 7. 强制门禁建议

### 7.1 结构门禁

- `assistant-surface-catalog`
  - 校验 active capability / 高价值查询面是否都有 Assistant 支持级别条目；
  - 校验 `support_level` 与字段完备性。
- `assistant-action-capability-map`
  - 校验 `assistant_action_id -> capability_key` 同源映射；
  - 校验 `action_request` 不可绕过注册。
- `assistant-handoff-contract`
  - 校验 `handoff_route` 存在且与 capability / route catalog 同源。

### 7.2 测试门禁

- `assistant-capabilities-contract`
  - 校验 `GET /api/assistant/capabilities` 与目录一致；
  - 校验 `GET /api/assistant/action-requests/{id}` 的 `capability_key / receipt_type / handoff_route` 可解释。
- `assistant-cross-slice-smoke`
  - 至少覆盖：
    - 平台/控制面
    - 核心 HR
    - 治理协同
    - 数据工作台

### 7.3 required checks 绑定原则

- 结构门禁必须进入 `validate`；
- contract / integration / E2E 必须进入 `test`；
- 命中 Assistant 触发矩阵的发布验证必须在 `smoke` 阶段保留最小 cross-slice smoke；
- 上述门禁不得长期处于 `skipped`、`manual-only` 或“仅 readiness 记录证明”的状态。

## 8. 对其他子计划的输入

### 8.1 对 `347`

- [ ] capability 目录需增加 Assistant 覆盖字段或提供同源编译产物。
- [ ] `assistant-surface-catalog / assistant-handoff-contract` 必须进入门禁清单。

### 8.2 对 `312`

- [ ] 新增 capability/high-value query surface 时，必须有对应 Assistant contract / smoke 样板。
- [ ] Assistant 不得以“模型不可预测”为由绕过关键断言。

### 8.3 对 `313`

- [ ] Assistant 相关 gate 必须被绑定到稳定 required checks。
- [ ] 命中 Assistant 触发矩阵的改动不得以 `skipped` 或人工联调替代。

### 8.4 对 `340/350/360/370/380`

- [ ] 新增正式 capability、高价值查询面、工作台动作、handoff route 时，必须同步更新 `assistant_surface_registry`。
- [ ] 任何“用户可做但 Assistant 无法解释/读取/回落”的功能都不得视为交付完成。

## 9. 验收标准

- [ ] `395` 已成为 Assistant 覆盖目录、支持级别、触发矩阵与门禁口径的单一事实源。
- [ ] `390/395/347/312/313` 的 responsibility split 已冻结，不再依赖口头约定。
- [ ] `340/360/370/380` 的正式 capability 与高价值查询面不存在未声明支持级别的暗面能力。
- [ ] Assistant 结构门禁、contract tests 与 cross-slice smoke 已被绑定到统一 required checks。
- [ ] 命中 Assistant 触发矩阵的变更不能以 `skipped`、手工联调或“后续补 Assistant”通过验收。

## 10. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md)
- [DEV-PLAN-313](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md)
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
