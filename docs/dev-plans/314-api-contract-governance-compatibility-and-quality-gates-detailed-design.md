# DEV-PLAN-314：API 合同治理、兼容性分级与质量门禁详细设计

**状态**: 规划中（2026-03-19 14:06 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md) 的 `314` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“业务规则优先、API 作为正式交付面、OpenAPI/Swagger 可作为合同资产”的冻结；
- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 对“共享 API / DTO / Explain 合同冻结”的要求；
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 与 [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md) 已列出的平台与核心 HR API 交付面；
- [DEV-PLAN-346](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/346-platform-routing-governance-and-response-contract-plan.md) 对路由语义、`route_class` 与 responder 返回契约的冻结；
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md) 对 contract / integration / E2E 测试分层的冻结；
- [DEV-PLAN-313](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md) 对 required checks、阶段门禁与交付证据的冻结；
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 与 [DEV-PLAN-395](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/395-assistant-surface-registry-and-enforcement-gates-detailed-design.md) 对 Assistant 专项 API / surface / contract gate 的冻结。

`300` 体系已经明确：**不把 API-first 提升为全局首要设计原则**，因为业务对象、时间语义、Explain、审批与权限边界必须先于端点命名和 DTO 设计冻结。  
但这不等于 API 可以停留在“最后再写一层 Controller”的弱治理状态。

如果没有 `314`，后续实现仍然容易继续出现：

- 官方 API 已在文档中列出，但缺少机器可校验的合同资产；
- 路由和返回外壳由 `346` 统一了，payload/DTO 却在模块内部漂移；
- 同一个业务动作在 UI、API、导出、Assistant 中使用不同字段名或不同错误语义；
- 破坏兼容性的接口变更没有显式分类、没有消费者影响清单、没有阻断门禁；
- 前端类型、SDK 投影、contract tests 与服务端实现之间没有单一同步链路。

`314` 的职责就是把这些问题收敛为 **“API 不是首要设计原则，但所有正式 API 合同都必须显式冻结、可 diff、可测试、可门禁阻断”** 的平台级治理方案。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结 Greenfield 正式 API 的合同资产形态：至少包括 path、method、request/response schema、错误码、鉴权摘要与关键上下文字段。
- [ ] 冻结 API 变更分级：`additive / breaking / semantic-breaking / internal-only`（或等价冻结词汇）。
- [ ] 冻结 API 合同门禁分层：schema diff、生成类型一致性、contract tests、required checks 绑定。
- [ ] 冻结 API 合同与 `346` 路由/返回契约的职责边界，避免“路径语义”和“payload 语义”再次混写。
- [ ] 为 `340/360/370/380` 提供统一 API 合同治理输入，确保 API 最终只是业务规则的投影，而不是第二事实源。
- [ ] 明确 Assistant API 与普通业务 API 的治理边界：普通 HTTP API 合同由 `314` 统一治理，Assistant surface/support-level 专项门禁仍由 `390/395` 承接。

### 2.2 非目标

- [ ] 本计划不把 `API-first` 提升为 `300` 体系的首要设计原则。
- [ ] 本计划不替代 [DEV-PLAN-346](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/346-platform-routing-governance-and-response-contract-plan.md) 的 `route_class`、allowlist 与全局 responder 合同。
- [ ] 本计划不替代 [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md) 与 [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 的 Explain / `policy_version` / `DecisionSnapshot` 主规则。
- [ ] 本计划不定义具体业务规则、数据库 DDL 或页面 IA；它只定义 API 合同如何显式冻结与阻断漂移。
- [ ] 本计划不吸收 Assistant 覆盖目录、`support_level`、`assistant_action_id` 等专项语义；这些继续由 `390/395` 负责。

## 3. “业务规则优先”在 API 合同治理中的翻译

### 3.1 用户真正关心的是“API 是否忠实表达业务规则”，不是先画多少端点

用户与下游消费者真正关心的是：

- 这个 API 代表的对象、时间语义、权限边界是否稳定；
- 同一能力在 UI/API/Assistant/导出中的字段和错误解释是否一致；
- 变更是否会破坏已有调用方；
- 为什么这次调用被拒绝、失败或要求重新确认。

### 3.2 API 合同是派生合同，但必须在交付前显式冻结

`314` 冻结：

- 业务规则与业务对象仍由上游领域计划拥有；
- API 合同是这些业务规则的**正式投影**；
- 投影一旦成为“官方 API”，就必须具备可 diff、可测试、可门禁阻断的合同资产；
- “最后再补一下 DTO/Controller”不是允许路径。

### 3.3 路由合同和 payload 合同必须分轨治理

- `346` 负责路径分类、暴露面、全局 responder 与按类返回壳；
- `314` 负责 request/response schema、错误码、兼容性分级、生成类型与 contract tests；
- 任何一方缺位都会让 API 表面看似统一，实际继续分叉。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确 API 是正式交付面，并建议使用 OpenAPI / Swagger 与类型同步。
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)、[DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)、[DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md) 已列出正式 API 面。
- [DEV-PLAN-346](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/346-platform-routing-governance-and-response-contract-plan.md) 已冻结 `route_class` 与 responder 合同。
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md) 已冻结 contract / integration / E2E 测试分层语义。
- [DEV-PLAN-313](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md) 已冻结 required checks 绑定语言。

### 4.2 当前主要缺口

1. [ ] **缺少普通业务 API 合同的共享 owner**  
   目前只有 API 清单和路由治理，没有单独 owner 负责 schema/DTO/兼容性。

2. [ ] **缺少 API 变更分级与兼容性 stopline**  
   新增字段、删字段、改错误码、改时间语义时，缺少统一分类口径。

3. [ ] **缺少生成类型与前端/客户端投影的一致性合同**  
   文档已提到 DTO 对齐，但没有冻结“如何阻断 drift”。

4. [ ] **缺少面向普通业务 API 的 contract tests 主计划**  
   `312` 当前更强调系统测试语义与 Assistant 套件，普通业务 API 仍缺正式 contract fixture 语言。

5. [ ] **缺少与 `346` 的清晰职责切分**  
   路由返回壳与 payload 语义若不分开冻结，后续很容易再次混写。

## 5. API 合同治理蓝图

### 5.1 领域使命

`314` 是 Greenfield 平台内“**官方 API 如何被显式表达、如何做兼容性分级、如何通过 schema diff / 生成类型 / contract tests / required checks 阻断漂移**”的共享治理权威。

### 5.2 核心合同对象

| 合同对象 | 合同含义 | 是否由 `314` 拥有共享合同 |
| --- | --- | --- |
| `ApiContractSpec` | 官方 API 的机器可校验合同资产（OpenAPI 或等价 schema 产物） | 是 |
| `ApiChangeClass` | API 变更分级：additive / breaking / semantic-breaking / internal-only | 是 |
| `ApiCompatibilityReport` | 当前变更相对基线的 diff 与结论 | 是 |
| `ContractFixture` | contract tests 使用的稳定输入输出夹具与断言语义 | 是（语义） |
| `ClientTypeProjection` | 前端/客户端从 API 合同派生的类型投影 | 是 |
| `ApiErrorContract` | 错误码、错误 envelope 与字段级错误最小合同 | 与 `346/353` 共治，`314` 拥有 API payload 侧部分 |
| `ApiSurfaceCatalog` | 正式 API 面清单与 owner 元数据 | 是 |

### 5.3 面向系统的主能力

- 把正式 API 从“文档清单”提升为机器可校验合同资产；
- 在不改变“业务规则优先”的前提下，阻断 API schema/DTO 的无声漂移；
- 为前端、集成、导出、Assistant 等消费者提供同源的 payload 语义；
- 把破坏兼容性的接口改动前移到 PR 阶段识别和阻断。

## 6. `314` 冻结的目标规则矩阵

| 场景 | 系统真正要做什么 | 核心 API 合同规则 | 合同结果 |
| --- | --- | --- | --- |
| 平台共享 API | 暴露登录、当前用户、租户上下文、平台控制面能力 | 合同必须显式声明鉴权、错误码、`tenant/org_context` 要求与返回 envelope | 平台 API 不再靠口头理解 |
| 核心域写 API | 创建/更新 effective-dated 或治理型对象 | Request/response 必须显式声明时间锚点、确认/回执/审批语义与错误码 | 写 API 不再隐式补语义 |
| 查询 API | 列表、详情、历史、导出前置查询 | 合同必须显式区分 `current / as_of / history`、分页、筛选与 Explain 约束 | 查询语义稳定 |
| 外部集成 API | 对外开放或 webhook 交互 | 必须显式分类为 external/internal，并执行更严格兼容性与错误契约检查 | 外部消费者风险可控 |
| 兼容性变更 | 修改字段、错误码、状态码或 payload 结构 | 每次改动必须先完成 change class 判定与消费者影响说明 | 破坏性变更可被前移发现 |
| 类型投影 | 让前端/客户端复用合同生成类型 | 类型投影必须来自同一合同资产或同源编译产物，drift 直接阻断 | DTO 不再双写 |

## 7. 共享合同、不变量与实现护栏

### 7.1 官方 API 合同资产合同

- 每个正式 HTTP API 都必须存在对应的 `ApiContractSpec`；
- `ApiContractSpec` 至少应覆盖：
  - path / method
  - request schema
  - response schema
  - 错误码与错误 envelope
  - 鉴权摘要
  - 是否需要 `tenant_id / org_context / time anchor / policy_version / decision_snapshot_id`
- 合同资产可以采用 code-first 生成，但 PR/CI 必须能得到稳定 diff 结果；
- “只有代码，没有可校验合同资产”视为未完成 API 交付。

### 7.2 API 变更分级合同

- `additive`：新增可选字段、非破坏性新端点、向后兼容的新错误码映射；
- `breaking`：删除/重命名字段、可选改必填、状态码/response shape 破坏兼容；
- `semantic-breaking`：schema 未明显变化，但业务含义、时间语义、错误含义或确认锚点发生改变；
- `internal-only`：仅限单仓内部消费者，且显式声明无外部承诺的变更；
- `breaking / semantic-breaking` 必须：
  - 回到对应上游业务 plan 更新合同；
  - 列出受影响消费者；
  - 在门禁中要求显式确认，而不能静默合并。

### 7.3 生成类型与客户端投影合同

- 前端类型、SDK 类型或内部 typed client 必须来自 `ApiContractSpec` 或同源编译产物；
- 手写 DTO 若作为过渡存在，必须受 drift 检查保护；
- “服务端 schema 已变，但前端/客户端类型未更新”必须 fail-fast。

### 7.4 与 `346` 的职责切分合同

- `346` 负责：
  - `route_class`
  - allowlist / 暴露面
  - responder envelope
  - 按类 404/405/500 返回契约
- `314` 负责：
  - request/response payload schema
  - 错误码字段与业务错误 payload
  - 兼容性分级
  - 类型投影
  - contract tests
- 若 response envelope 与 payload schema 不一致，视为双方合同同时失败。

### 7.5 contract tests 合同

- `312` 负责测试语义分层，`314` 负责普通业务 API contract fixture 语义；
- contract tests 至少应验证：
  - 必填字段与默认值
  - 错误码与错误字段
  - 权限/租户拒绝路径
  - `current / as_of / history` 等关键查询语义
  - `policy_version / decision_snapshot_id` 等提交锚点（如适用）
- contract tests 不替代集成测试，但应优先捕获 API 合同 drift。

### 7.6 流水线绑定合同

- `validate`：
  - 生成或校验 `ApiContractSpec`
  - 执行 schema diff / compatibility classification
  - 校验生成类型或客户端投影一致性
  - 校验 `346` route/responder 与 `314` payload contract 的一致性
- `test`：
  - 执行 `312 + 314` 定义的 contract / integration 测试
- `smoke`：
  - 对平台共享 API、关键写 API 与高风险查询 API 执行最小 smoke
- `313` 负责把这些 gate 绑定进稳定 required checks。

### 7.7 stopline

- 新增/修改正式 API 但没有更新 `ApiContractSpec`。
- 破坏兼容性变更没有显式 change class 与消费者影响说明。
- API 需要 `org_context / time anchor / policy_version / decision_snapshot_id`，但合同未显式声明。
- response payload 与 `346` 冻结的 responder / route_class 契约不一致。
- 生成类型与服务端合同漂移。
- API schema 引入了上游领域计划未冻结的新业务语义，却未先回到对应业务 plan 更新合同。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `312`

- [ ] 把普通业务 API contract fixtures 纳入测试语义对象模型。
- [ ] contract tests 需区分“schema drift”“业务规则 drift”“权限/租户 drift”。

### 8.2 对 `313`

- [ ] `ApiContractSpec` diff、类型投影一致性、普通业务 API contract tests 必须进入稳定 required checks。
- [ ] 命中 API 合同触发器的变更不得以 docs-only、手工联调或 `skipped` 视为通过。

### 8.3 对 `340`

- [ ] 平台共享 API 需声明统一错误 envelope、租户/会话上下文字段与鉴权摘要。
- [ ] `/api/auth/*`、`/api/me`、`/api/tenants/current`、控制面 API 必须先具备合同资产再进入实现封板。

### 8.4 对 `360`

- [ ] 核心 HR API 必须把 `current / as_of / history`、effective-dated 写语义与关键错误码冻结到合同里。
- [ ] `361/362/363/364` 的业务规则变更若影响 API 语义，必须同步更新 API 合同资产。

### 8.5 对 `370/380`

- [ ] 外部集成 API、导出前置查询 API、工作台高价值查询 API 需显式标记兼容性等级与消费者类别。
- [ ] 报表/导出类 API 的时间语义与字段血缘必须体现在合同里，而不是只写在实现里。

### 8.6 对 `390/395`

- [ ] Assistant 专项 API 继续由 `390/395` 负责其 support-level / surface gate，但其普通 HTTP payload contract 仍应与 `314` 的 schema / DTO / error contract 规则保持一致。
- [ ] Assistant 专项 contract tests 不替代普通 API compatibility gates。

## 9. 建议实施分期

1. [ ] `M1`：合同资产与 owner 冻结  
   冻结 `ApiContractSpec / ApiChangeClass / ApiSurfaceCatalog`。
2. [ ] `M2`：兼容性分级与 diff 规则冻结  
   明确 additive / breaking / semantic-breaking 的判断标准与输出物。
3. [ ] `M3`：生成类型与 contract fixture 冻结  
   冻结前端/客户端类型投影与 contract fixtures 的一致性规则。
4. [ ] `M4`：门禁与 required checks 接线  
   将 schema diff、类型一致性、contract tests 接入 `313`。
5. [ ] `M5`：平台与核心业务 API 收口  
   优先收口 `340/360`，再扩展至 `370/380/390`。

## 10. 验收标准

- [ ] `314` 已成为普通业务 API 合同治理的单一事实源。
- [ ] Greenfield 保持“业务规则优先”，但所有正式 API 都已具备机器可校验合同资产。
- [ ] 路由/返回壳合同与 payload schema 合同的 owner 已清晰切分，不再混写。
- [ ] 破坏兼容性的 API 改动不能在没有显式分类与消费者影响说明的情况下通过验收。
- [ ] 前端/客户端类型投影与服务端 API 合同保持同源，不再双写漂移。
- [ ] `312/313/340/360/370/380/390` 能直接消费 `314`，不再各自发明 API contract gate。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md)
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md)
- [DEV-PLAN-313](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md)
- [DEV-PLAN-321](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md)
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-346](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/346-platform-routing-governance-and-response-contract-plan.md)
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
- [DEV-PLAN-395](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/395-assistant-surface-registry-and-enforcement-gates-detailed-design.md)
