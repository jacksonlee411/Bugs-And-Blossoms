# DEV-PLAN-350：前端产品壳与交互系统子计划

**状态**: 规划中（2026-03-17 07:23 CST）

## 1. 背景与上下文

`340` 提供平台壳与会话上下文，`345` 提供共享决议与 Explain 合同，`346` 提供路由治理与返回契约输入，`347` 提供 capability 与颗粒度治理底座，`363` 提供 Job Catalog 的首批工作台样板，`360/370/380/390` 都会产生大量页面，但目前还没有一个计划真正拥有前端产品系统本身。

`350` 负责冻结前端的统一交付语言：

- 信息架构
- 导航与路由
- 列表/详情/历史模式
- 表单系统
- MUI 组件规范
- 权限感知 UI
- `345/347/363` 到 `351/352/353` 的前端消费翻译

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立统一前端信息架构和导航模式。
- [ ] 与路由治理合同保持一致：前端路由分组、入口语义与全局失败态表达不偏离平台 `route_class` 约束。
- [ ] 建立列表、详情、历史三件套页面模式。
- [ ] 建立表单、校验、错误反馈、空状态的统一规范。
- [ ] 建立权限感知的 UI 展示约定。
- [ ] 明确 `350` 如何消费 `345` 的决议合同、`347` 的能力治理与 `363` 的首批样板，并把它们翻译成可复用的前端产品语言。

### 2.2 非目标

- [ ] 本计划不承接具体业务规则。
- [ ] 本计划不替代设计系统工具链，但会定义产品交互规范。
- [ ] 本计划不重写 `345` 的 `DecisionContext / DecisionSnapshot / Explain` 合同，不重写 `347` 的 `capability_key` 与颗粒度词汇，也不重写 `363` 的 Job Catalog 业务语义。

## 3. 范围

- Product Shell
- Routing & Navigation
- Page Patterns
- Form Patterns
- Grid / Tree / Timeline 模式
- Permission-aware UI
- Error feedback semantics

## 4. 关键设计决策

### 4.1 页面模式优先统一（选定）

- 列表页
- 详情页
- 历史页
- 对话/工作台页

### 4.2 生效日期是 UI 一级概念（选定）

- 任何支持历史的业务对象，都必须在 UI 上显式暴露 effective date 视角。

### 4.3 不把复杂交互藏进零散弹窗（选定）

- 复杂对象优先使用详情页或双栏页。
- Dialog 只用于短事务。

### 4.4 前端只表达共享决议，不重算共享决议（选定）

- `read_only / hidden / disabled / 403 / required / maintainable` 等语义必须来自 `345 + 347 + 342` 的共享决议与授权结果，前端只负责稳定呈现，不得在组件树内自行推导第二套规则。
- 页面必须显式承接 `org_context / as_of / effective_date / policy_version / capability_key` 等上下文锚点；是否展示哪些锚点由页面模式决定，但不得隐去主解释链。
- Explain 是共享入口，不是前端自行拼接出来的“猜测性说明”。

### 4.5 Job Catalog 是首批样板，不是局部特例（选定）

- `363` 作为首批正式消费域，用来验证 `ContextBar + 列表/详情/历史 + 只读解释 + Explain` 的组合模式是否可复用到其他业务域。
- `350` 不得把 Job Catalog 简化成单棵树、隐藏 `OrgContext`，或把 `read_only` 误做成页面私有状态。

## 5. 功能拆分

### 5.1 M1：Product Shell

- [ ] 导航
- [ ] 路由分组
- [ ] 顶栏/侧栏
- [ ] 租户与用户上下文
- [ ] `PageHeader / ContextBar / StatusSurface` 的全局槽位
- [ ] 共享 Explain 入口与只读原因承接位

### 5.2 M2：Page Patterns

- [ ] 列表页模板
- [ ] 详情页模板
- [ ] 历史页模板
- [ ] 工作台模板
- [ ] `org_context + as_of / effective_date + read_only` 的稳定展示模式

### 5.3 M3：表单与交互规范

- [ ] 表单布局
- [ ] 校验反馈
- [ ] 错误/空状态
- [ ] 提交确认
- [ ] 共享决议驱动的字段状态与动作状态表达

## 6. 对 `345 / 347 / 363` 的显式消费合同

### 6.1 对 `345` 的消费

- `350` 只消费 `345` 冻结的共享决议与 Explain 合同，不在前端层重新定义“字段为什么可见/必填/只读”。
- `351 / 352 / 353` 必须把 `DecisionContext / DecisionSnapshot` 翻译成稳定产品语言，至少能承接：`org_context`、`as_of / effective_date`、`read_only`、`policy_version`。
- 业务表单、详情页与列表页只能消费共享决议结果，不得通过前端默认值、路由参数或组件局部状态暗中补出第二套行为规则。

### 6.2 对 `347` 的消费

- `350` 不得发明第二套 `capability_key` 命名、路由到动作映射或权限语义。
- 路由入口、页面主动作、字段级行为与 Assistant 直达入口，都必须与 `347` 的 capability/颗粒度词汇保持一致。
- `hidden / read_only / disabled / 403` 的产品表达可以由 `350` 统一，但其边界定义必须消费 `347 + 342` 的治理结果。

### 6.3 对 `363` 的消费

- `363` 是 `350` 的首批正式页面样板：前端必须保留 Job Catalog 的组合分类体系、工作台上下文与 `as_of + org_context + read_only` 视图语义。
- `351 / 352 / 353` 需要让 Job Catalog 成为可复用样板，而不是把它当作孤立特例。
- 只读共享基线、Explain 入口、分类视图澄清与版本切换模式，应先在 Job Catalog 场景里冻结，再复制到其他业务域。

## 7. 验收标准

- [ ] 后续所有业务页面都能复用统一的前端模式，而不是各自设计页面骨架。
- [ ] 有效期历史、详情和编辑交互已经有清晰统一的呈现方式。
- [ ] 权限差异在 UI 上有一致的行为表达。
- [ ] `350` 已明确成为 `345/347/363` 的前端消费 SSOT，后续页面不再自行定义第二套上下文、能力或只读语义。
- [ ] `351/352/353` 能直接引用 `345/347/363` 的合同，不再靠模块私有经验拼装页面和交互。

## 8. 后续拆分建议

1. [ ] [DEV-PLAN-351：Product Shell 与路由信息架构详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/351-product-shell-and-route-information-architecture-detailed-design.md)
2. [ ] [DEV-PLAN-352：列表/详情/历史页面模式详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md)
3. [ ] [DEV-PLAN-353：表单与权限感知交互详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md)

补充说明：
`350` 的路由语义、失败态页面与跨入口一致性默认消费 [DEV-PLAN-346](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/346-platform-routing-governance-and-response-contract-plan.md)；决议与 Explain 默认消费 [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)；能力/颗粒度边界默认消费 [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)；Job Catalog 首批样板默认消费 [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)；错误呈现与字段反馈默认对齐 `310 + 353` 冻结的合同。
