# DEV-PLAN-313：CI/CD、Linux 容器平台部署与观测基线详细设计

**状态**: 规划中（2026-03-18 17:26 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md) 的 `313` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“默认部署目标是 Linux 容器平台、标准发布物是 OCI image、Kubernetes 不是第一阶段前置”的冻结；
- [DEV-PLAN-311](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/311-engineering-structure-and-local-development-baseline-detailed-design.md) 对 `Web/Api/Worker` 运行入口与本地闭环画像的冻结；
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md) 对测试分层、失败证据与切片验收语言的冻结；
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 对租户隔离、密钥注入与安全 stopline 的冻结。

`310` 已定义“要有 CI/CD 与最小观测”，但若没有 `313` 作为执行权威，后续会继续出现：

- 本地可跑、CI 不可跑；
- 构建产物不可复现，环境间漂移严重；
- 测试门禁与发布门禁混写，失败证据不足；
- 只看日志文本，不具备结构化指标与 trace 关联；
- 安全边界在流水线与部署阶段被“临时放宽”。

`313` 的职责是把上述问题收敛为 **可持续交付与可观测运行的统一合同**。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结从提交到发布的分层流水线合同（质量门禁、构建、镜像、部署、smoke、证据归档）。
- [ ] 冻结 Linux 容器平台交付基线与不可变产物策略（build once, promote many）。
- [ ] 冻结 `Web/Api/Worker` 的交付入口与运行画像映射，避免环境重命名漂移。
- [ ] 冻结最小观测基线（结构化日志、核心指标、trace 关联、错误码可检索）。
- [ ] 冻结失败处置与 stopline 规则，确保 fail-closed。
- [ ] 为 `340/350/360/370/380/390` 提供统一发布与运行基座。
- [ ] 冻结 `390/395` 横切 Assistant 门禁在流水线中的绑定方式，确保结构门禁、contract tests 与 cross-slice smoke 都进入统一 required checks。

### 2.2 非目标

- [ ] 本计划不定义业务领域规则、页面交互细节与权限矩阵真值。
- [ ] 本计划不替代 [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md)；测试语义归 `312`，`313` 只绑定执行。
- [ ] 本计划不要求第一阶段引入 Kubernetes-first、多区域多活或复杂发布编排。
- [ ] 本计划不允许为发布便利而提交 secret、放宽租户边界或跳过高风险门禁。

## 3. “业务规则优先”在交付与观测中的翻译

### 3.1 用户真正关心的是“改动可控上线且可回溯”，不是某个 CI 工具名称

用户关心的是：

- 发布后系统是否稳定可用；
- 出现问题时是否能快速定位；
- 高风险变更是否被门禁拦截。

### 3.2 流水线首先回答“风险在何时被阻断”

`313` 冻结：

- 语法/静态检查、测试、构建、smoke、发布证据必须分层；
- 失败需在最早可判断阶段停止，不能把问题推迟到生产。

### 3.3 构建产物是事实源，不是临时副本

- 构建产物不可变；
- 环境差异由配置与密钥引用控制，不由二次构建产生。

### 3.4 观测是交付合同的一部分

- 没有结构化观测就不算“可交付”；
- 日志、指标、trace、错误码必须可关联到请求与回执上下文。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已冻结 Linux 容器平台与 OCI 发布物。
- [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md) 已冻结测试与观测属于同一工程主线。
- [DEV-PLAN-311](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/311-engineering-structure-and-local-development-baseline-detailed-design.md) 已冻结本地运行与入口 ownership。
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md) 已冻结测试分层与失败证据合同。
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 已冻结安全边界与密钥治理。

### 4.2 当前主要缺口

1. [ ] **缺少共享流水线对象模型**  
   当前还没有统一定义“流水线阶段、发布证据、promotion 记录”的业务对象。

2. [ ] **缺少构建与部署单主源口径**  
   本地与 CI、CI 与部署环境之间仍缺统一命名与入口映射。

3. [ ] **缺少可执行观测最小集**  
   结构化日志、核心指标、trace 传播与错误码检索尚未冻结。

4. [ ] **缺少 stopline 级故障处置路径**  
   缺 tenant context、缺 secret 引用、smoke 失败等场景尚无统一阻断语义。

5. [ ] **缺少 390 横切能力的 required check 绑定**  
   `390` 已定义“无暗面能力”与支持级别目录，但还没有一份 CI/CD 计划明确哪些 gate 必须在 `validate/test/smoke` 阶段执行，哪些情况下不得以 `skipped` 或手工联调替代。

## 5. CI/CD 与运行观测蓝图

### 5.1 领域使命

`313` 是平台内“**如何把代码可靠地转化为可部署产物、如何在 Linux 容器平台受控发布、如何提供最小可观测与可回滚证据**”的共享交付权威。

### 5.2 核心交付对象

| 交付对象 | 交付含义 | 是否由 `313` 拥有共享合同 |
| --- | --- | --- |
| `QualityGatePipeline` | 质量门禁执行链 | 是 |
| `BuildArtifact` | 编译与打包产物 | 是 |
| `OciImageRelease` | 不可变镜像发布物 | 是 |
| `RuntimeProfile` | `Web/Api/Worker` 运行画像 | 是 |
| `PromotionRecord` | 环境推进与审批记录 | 是 |
| `SmokeVerification` | 部署后最小可用性验证 | 是 |
| `ObservabilityBaseline` | 日志/指标/追踪最小要求 | 是 |
| `ReleaseEvidence` | 发布证据与回滚依据 | 是 |
| `AssistantCoverageGateBinding` | `390/395` 门禁与 required checks 的绑定规则 | 是 |

### 5.3 面向系统的主能力

- 按固定阶段执行质量门禁并快速失败；
- 产出可复现 OCI 镜像并按环境推进；
- 让部署入口与本地入口保持同一 ownership 语言；
- 在发布后提供可检索、可关联、可定位的问题证据。

## 6. `313` 冻结的目标规则矩阵

| 场景 | 系统真正要做什么 | 核心交付规则 | 交付结果 |
| --- | --- | --- | --- |
| 提交合并前验证 | 先阻断明显质量风险 | 门禁按阶段执行，失败即停 | 风险前置阻断 |
| 构建镜像 | 产出可复现发布物 | `build once`，镜像不可变 | 产物一致可追溯 |
| 环境部署 | 在 Linux 容器平台运行 | 使用同一镜像+环境配置引用 | 环境差异可控 |
| 部署后校验 | 验证最小功能可用 | 必须执行 smoke 并保留证据 | 发布结果可判定 |
| 故障定位 | 快速还原问题上下文 | 日志/指标/trace/错误码可关联 | 排障路径可复用 |
| 高风险配置缺失 | 防止带病发布 | 缺 tenant/secret/runtime 合同即 fail-closed | 不安全发布被阻断 |

## 7. 共享合同、不变量与实现护栏

### 7.1 流水线阶段合同

建议至少冻结以下阶段语义：

1. `validate`：格式、静态检查、配置校验；
2. `test`：执行 `312` 定义的分层测试；
3. `package`：构建发布物与 OCI 镜像；
4. `smoke`：部署后最小可用性验证；
5. `promote`：环境推进与发布证据登记。

其中：

- `validate` 至少应绑定 `347/395` 定义的 Assistant 结构门禁，如 capability/surface 目录一致性、`assistant_action_id` 映射与 handoff contract。
- `test` 至少应绑定 `312` 定义的 Assistant contract / integration / E2E 套件，确保只读检索、受控动作、拒绝与降级路径在自动化中真实执行。
- `smoke` 对命中 Assistant 触发矩阵的变更，必须执行最小 cross-slice Assistant smoke；不得以 docs-only、手工联调或 `skipped` 视为通过。

### 7.2 构建产物合同

- 镜像必须可溯源到代码提交与门禁结果；
- 同一发布版本不得在不同环境二次编译；
- 运行时仅允许注入环境差异配置与 `SecretReference`。

### 7.3 运行入口与环境画像合同

- CI 与部署必须沿用 `311` 的 `Web/Api/Worker` ownership 命名。
- 不允许本地叫一套、交付叫一套。
- 本地 smoke 与容器 smoke 应共享核心入口语义。

### 7.4 测试绑定合同

- `313` 必须消费 `312` 的测试分层与失败证据合同。
- 不允许流水线跳过高风险 E2E 而直接发布。
- 对耗时测试可分层并行，但不得改变验收语义。
- `313` 必须把 `390/395` 横切 Assistant 门禁绑定为稳定 required checks；Assistant 相关 gate 不得长期停留在“人工验证后补证据”。

### 7.5 观测基线合同

最小观测至少包括：

- 结构化日志：包含 tenant、request/correlation、message code；
- 核心指标：请求成功率、错误率、关键异步任务状态；
- Trace 关联：跨 `Web/Api/Worker` 能关联请求与任务；
- 错误可检索：可按错误码与关联 ID 快速定位。

### 7.6 安全与密钥合同

- 所有密钥仅通过 `333` 的受控引用注入；
- 任何发布流程不得输出原始 secret；
- 缺关键安全配置必须 fail-closed，而不是降级放行。

### 7.7 失败处置合同

- smoke 失败：阻断 promotion，保留证据，进入修复流程；
- 安全 stopline 触发：立即阻断发布并标记高优先级处理；
- 回滚策略应基于不可变镜像与发布记录，不走 legacy 双轨。

### 7.8 stopline

- 不允许“跳过门禁后手工部署”成为常态路径。
- 不允许发布阶段继续使用未版本化、不可追溯构建物。
- 不允许部署成功但无可观测证据即判定“已交付”。
- 不允许通过共享测试账号、默认租户、明文密钥绕开发布阻断。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `340/350` 的输入

- [ ] 平台入口与前端壳层必须复用统一构建与部署链路。
- [ ] 平台登录、导航、session 关键路径应成为 smoke 基础切片。

### 8.2 对 `360` 的输入

- [ ] 核心业务域需提供可自动化的模块 smoke 与关键回归入口。
- [ ] 业务模块不得自建私有发布脚本与私有运行画像。

### 8.3 对 `370/380` 的输入

- [ ] 工作流、导入导出、集成任务要接入统一后台任务观测指标。
- [ ] 高风险导出与批处理失败需具备结构化证据输出。

### 8.4 对 `390` 的输入

- [ ] Assistant 运行链路要接入统一日志、trace、任务与回执观测。
- [ ] Assistant 评测与运行治理事件需纳入发布后健康检查视角。

### 8.5 对 `395`（Assistant 全平台覆盖目录与强制门禁）的输入

- [ ] `395` 定义的 Assistant 结构门禁、contract tests 与 smoke 触发矩阵，必须被 `313` 绑定为统一流水线阶段，不得散落在模块私有脚本或人工清单中。
- [ ] 命中 Assistant 触发器的变更，不得以 `skipped`、手工联调或“后续再补”绕过 required checks。

## 9. 建议实施分期

1. [ ] `M1`：流水线对象与阶段语义冻结  
   冻结 `QualityGatePipeline` 与阶段边界。
2. [ ] `M2`：产物与环境画像冻结  
   冻结 OCI 不可变产物与 `Web/Api/Worker` 运行映射。
3. [ ] `M3`：测试绑定与 smoke 合同冻结  
   把 `312` 分层测试和部署后 smoke 接线。
4. [ ] `M4`：观测基线接线  
   冻结日志、指标、trace、错误码最小集并验证可检索。
5. [ ] `M5`：发布证据与故障处置收口  
   固化 promotion、回滚、stopline 处置与审计证据。

## 10. 验收标准

- [ ] `313` 已成为 CI/CD、容器部署与最小观测基线的单一事实源。
- [ ] 提交到发布的阶段边界清晰，失败阻断可预测、可追溯。
- [ ] 发布物可复现、可回滚，环境差异可控且不依赖二次构建。
- [ ] 系统具备最小可观测能力，能快速定位跨入口与异步问题。
- [ ] `340/350/360/370/380/390` 可直接消费 `313` 交付合同，不再各自发明发布链路。
- [ ] `390/395` 定义的 Assistant 横切门禁已被稳定绑定到 required checks，结构门禁、测试门禁与 smoke 门禁都不可被 `skipped` 或手工联调替代。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-engineering-quality-testing-and-delivery-plan.md)
- [DEV-PLAN-311](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/311-engineering-structure-and-local-development-baseline-detailed-design.md)
- [DEV-PLAN-312](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/312-testing-pyramid-and-e2e-strategy-detailed-design.md)
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
- [DEV-PLAN-400](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/400-implementation-roadmap-and-vertical-slice-plan.md)
