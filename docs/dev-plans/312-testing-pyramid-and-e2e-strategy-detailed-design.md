# DEV-PLAN-312：测试金字塔与 E2E 策略详细设计

**状态**: 规划中（2026-03-18 17:26 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md) 的 `312` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“垂直切片验收优先、受控 Assistant、租户隔离 fail-closed、effective-dated 主模型”的冻结；
- [DEV-PLAN-311](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/311-engineering-structure-and-local-development-baseline-detailed-design.md) 对工程结构、本地运行入口、`SeedDataset` 与 `TestFixtureDataset` 分层的冻结；
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)、[DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md)、[DEV-PLAN-324](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/324-ef-core-query-filter-dapper-sql-and-database-native-capabilities-boundary-detailed-design.md) 对时间合同、回执合同、持久化边界的冻结；
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 对租户安全、tenant-scoped SQL 与 Assistant 安全护栏的冻结。

`310` 已明确测试分层是工程主线的一部分，但如果没有 `312`，后续实现容易继续出现：

- 单测、集成、E2E 互相挤占职责；
- 同一业务规则在多个层级重复验证，却都不完整；
- E2E 只测页面 happy path，不验证租户隔离、时间语义与回执链；
- 测试数据混用 demo/seed/fixture，导致不可重现、不可并行；
- Assistant / 审批 / 导入导出等长链路只在手工联调中“看起来能跑”。

`312` 的职责是把上述问题收敛为 **平台级测试语义合同**，让 `313/340/350/360/370/380/390` 共享同一测试语言。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结测试金字塔分工，明确 Unit / Integration / E2E 分别回答什么业务问题。
- [ ] 冻结 `SeedDataset` 与 `TestFixtureDataset` 的测试语义边界，确保可重现与可并行。
- [ ] 冻结时间语义（`current / as_of / history`）、租户隔离、权限拒绝、回执链路的测试合同。
- [ ] 冻结垂直切片验收的最小 E2E 套件语言，覆盖登录、主链写入、审批、导出、Assistant 关键路径。
- [ ] 冻结 flaky 用例治理与测试失败证据要求，避免“先忽略再说”的质量漂移。
- [ ] 为 `313` 输出可直接执行的流水线测试分层输入。

### 2.2 非目标

- [ ] 本计划不定义具体业务功能与页面 IA；它只定义测试语义与分层边界。
- [ ] 本计划不替代 [DEV-PLAN-313](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/313-ci-cd-linux-container-deployment-and-observability-baseline-detailed-design.md)；流水线编排、发布与环境 promotion 由 `313` 承接。
- [ ] 本计划不以“测试方便”为理由引入默认租户、默认超级权限或绕过确认的旁路。
- [ ] 本计划不允许把 E2E 变成唯一真相，也不允许把所有验证都下沉为单测。

## 3. “业务规则优先”在测试策略中的翻译

### 3.1 用户真正关心的是“改动是否可信”，不是某个测试框架名字

用户关心的是：

- 关键链路是否在真实边界下仍能完成；
- 高风险失败是否会被尽早阻断；
- 回归问题能否稳定重现并定位。

### 3.2 测试分层首先回答“哪类风险在何处最经济地被发现”

`312` 冻结：

- Unit 测试优先验证纯业务规则与分支语义；
- Integration 测试优先验证数据库、事务、租户、时间、回执等系统边界；
- E2E 测试优先验证用户可见垂直切片与跨模块状态衔接。

### 3.3 E2E 是交付证据，不是兜底垃圾桶

- E2E 不负责穷举所有业务分支；
- E2E 必须覆盖 stopline 级场景与关键链路；
- 能在 Unit / Integration 提前发现的问题，不应推迟到 E2E 才暴露。

### 3.4 测试数据是合同，不是脚本副产物

- 演示样例、平台 seed、自动化 fixture 必须分层；
- fixture 需可组合、可重置、可并行，不依赖人工状态修补。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已冻结“垂直切片优先验收”。
- [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md) 已冻结测试分层与 Linux 容器 smoke 要求。
- [DEV-PLAN-311](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/311-engineering-structure-and-local-development-baseline-detailed-design.md) 已冻结 runtime entry、bootstrap、seed/fixture 分层。
- [DEV-PLAN-322/323/324](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 已冻结时间合同、回执合同、持久化边界。
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 已冻结租户与安全 stopline。

### 4.2 当前主要缺口

1. [ ] **缺少统一测试对象模型**  
   目前缺少“测试在验证什么对象、由谁拥有语义”的共享词汇。

2. [ ] **缺少跨层一致的失败证据合同**  
   测试失败时需要输出哪些上下文（tenant/time/request/receipt），尚未冻结。

3. [ ] **缺少长链路测试模板**  
   审批、导入导出、Assistant action request 仍缺可复用的 E2E 样板。

4. [ ] **缺少对 `313` 的直接输入**  
   流水线需要测试分层、并行策略、失败阻断规则，当前仍依赖口头约定。

## 5. 测试金字塔与 E2E 策略蓝图

### 5.1 领域使命

`312` 是 Greenfield 平台内“**哪些业务风险应该在哪一层被验证、测试数据如何组织、关键切片如何形成持续交付证据**”的共享测试权威。

### 5.2 核心测试对象

| 测试对象 | 测试含义 | 是否由 `312` 拥有共享合同 |
| --- | --- | --- |
| `UnitSpec` | 纯规则与纯函数层测试规格 | 是 |
| `IntegrationSpec` | 基于真实存储/事务/边界的测试规格 | 是 |
| `E2ESliceSpec` | 面向用户切片的端到端规格 | 是 |
| `TestFixtureDataset` | 自动化可重现测试夹具 | 是（语义） |
| `TestEvidence` | 失败证据与执行摘要 | 是 |
| `CoverageProfile` | 覆盖率统计口径与阈值引用方式 | 是（口径） |
| `QualityGateBinding` | 测试层与 `313` 流水线绑定规则 | 是 |

### 5.3 面向系统的主能力

- 在统一语言下组织测试分层，避免职责重叠；
- 把租户、时间、权限、回执这些系统边界变成可执行测试；
- 让垂直切片持续可回归，而非阶段性演示；
- 为 CI 提供稳定、可并行、可诊断的测试输入。

## 6. `312` 冻结的目标规则矩阵

| 场景 | 系统真正要做什么 | 核心测试规则 | 测试结果 |
| --- | --- | --- | --- |
| 领域规则变更 | 验证规则分支与错误语义 | 优先 Unit，禁止仅靠 E2E 覆盖分支 | 规则回归早发现 |
| 持久化/事务变更 | 验证租户、时间、约束与一致性 | 必须有 Integration，覆盖 `tenant + time view + transaction` | 数据边界可验证 |
| 页面/流程变更 | 验证用户可见闭环 | 必须落到 E2E 切片并包含关键成功/拒绝路径 | 交付可感知 |
| effective-dated 逻辑 | 验证 `current/as_of/history` | Unit + Integration 联合验证，E2E 验证关键视图切换 | 时间语义不漂移 |
| 审批/异步长链路 | 验证票据与回执一致性 | 必须验证 `OperationTicket/Receipt`，禁止只看主表结果 | 长链路可解释 |
| Assistant 动作 | 验证澄清、确认、执行与拒绝 | E2E 必测受控动作链，不允许模型直写旁路 | AI 行为可控 |

## 7. 共享合同、不变量与实现护栏

### 7.1 测试分层合同

- Unit：默认覆盖 domain/application 纯规则、错误码分支、时间写意图映射。
- Integration：默认覆盖数据库约束、tenant-scoped SQL、事务边界、回执落库与查询。
- E2E：默认覆盖用户切片、权限门禁、审批与异步回执、导入导出与 Assistant 关键流程。
- 同一风险优先在最低成本层捕获，不得全部上推到 E2E。

### 7.2 测试数据合同

- `SeedDataset` 负责平台最小可运行样例，不承担测试断言语义。
- `TestFixtureDataset` 负责确定性输入与可重置状态。
- fixture 必须显式声明：`tenant_id`、时间锚点、对象初始状态、预期回执。
- 不允许把人工 demo 数据当自动化夹具。

### 7.3 时间与租户测试合同

- 所有涉及业务时间的测试必须显式选择 `current / as_of / history`。
- 所有涉及跨对象查询/导出的测试必须显式带租户上下文。
- 缺少 tenant/time 上下文的执行必须验证 fail-closed。

### 7.4 长链路与回执测试合同

- 审批、导入导出、后台任务、Assistant 动作必须验证：
  - 票据可查询；
  - 回执 append-only；
  - 最新状态可投影；
  - 失败原因可解释。
- 禁止只断言“最终主表有数据”。

### 7.5 失败证据合同

测试失败至少应记录：

- `tenant_id`、`principal`（如适用）；
- 时间视图与时间锚点；
- `request_id / trace_id / correlation_id`（如适用）；
- 关键回执或错误码；
- 最小可重放步骤。

### 7.6 覆盖率合同

- 覆盖率口径、阈值、统计范围以 `Makefile` 与 CI workflow 为单一事实源。
- `312` 负责冻结“覆盖率如何服务风险治理”的策略，不在文档里复制门禁实现细节。
- 对可证明不可达分支优先删分支，不以扩大排除项替代修复。

### 7.7 stopline

- 不允许以“E2E 已覆盖”为由跳过核心 Unit/Integration 测试。
- 不允许测试通过默认租户、默认超级权限、隐式 today 伪造通过。
- 不允许长期 quarantine flaky 用例而无修复计划与到期清理。
- 不允许把人工联调结果当作自动化测试通过证据。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `313`（CI/CD、部署与观测基线）的输入

- [ ] 流水线必须按 `Unit -> Integration -> E2E` 分层执行并保留失败证据。
- [ ] 测试并行与重试策略不得改变测试语义与失败归因。

### 8.2 对 `340/350` 的输入

- [ ] 平台入口与前端壳层变更必须接入最小切片 E2E：登录、鉴权、导航、首屏列表。
- [ ] UI 权限感知与后端拒绝语义需保持一致断言。

### 8.3 对 `360` 的输入

- [ ] `361/362/363/364` 必须声明各自核心对象的 Unit/Integration 必测清单。
- [ ] effective-dated 对象必须提供 `current/as_of/history` 测试样板。

### 8.4 对 `370/380` 的输入

- [ ] 审批、导入导出、批处理必须输出可复用的长链路 E2E 模板。
- [ ] 工作台查询需验证 tenant-scoped SQL 与高风险导出拒绝路径。

### 8.5 对 `390` 的输入

- [ ] Assistant 必须有只读检索、受控动作、拒绝与降级路径的 E2E 套件。
- [ ] Assistant 不得以“模型行为难测”为由跳过关键业务断言。

## 9. 建议实施分期

1. [ ] `M1`：测试对象模型冻结  
   冻结 Unit/Integration/E2E 的风险分工与对象词汇。
2. [ ] `M2`：测试数据合同冻结  
   冻结 seed/fixture 分层、时间与租户上下文表达、reset 规则。
3. [ ] `M3`：关键风险测试模板冻结  
   冻结时间语义、租户隔离、回执链路、Assistant 受控动作测试模板。
4. [ ] `M4`：垂直切片套件接线  
   以“登录-列表-写入-审批/任务-回执”为主样板接线跨模块 E2E。
5. [ ] `M5`：与 `313` 流水线收口  
   把分层执行、失败证据、覆盖率证据接入统一 CI 语义。

## 10. 验收标准

- [ ] `312` 已成为测试金字塔与 E2E 策略的单一事实源。
- [ ] Unit/Integration/E2E 的职责边界清晰，关键风险不再遗漏或重复堆叠。
- [ ] 租户隔离、时间语义、回执链路、Assistant 受控动作均有稳定自动化验证。
- [ ] 测试数据可重现、可并行、可诊断，不再依赖手工修补环境。
- [ ] `313/340/350/360/370/380/390` 可直接消费 `312` 测试合同，而不再各自发明测试口径。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md)
- [DEV-PLAN-311](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/311-engineering-structure-and-local-development-baseline-detailed-design.md)
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md)
- [DEV-PLAN-324](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/324-ef-core-query-filter-dapper-sql-and-database-native-capabilities-boundary-detailed-design.md)
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
- [DEV-PLAN-400](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/400-implementation-roadmap-and-vertical-slice-plan.md)
