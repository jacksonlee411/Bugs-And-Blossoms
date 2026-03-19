# DEV-PLAN-390：Chat Assistant 能力子计划

**状态**: 规划中（2026-03-17 07:23 CST）

## 1. 背景与上下文

本计划承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的总体蓝图
- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的平台基座
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 的前端产品壳与交互系统
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md) 的核心 HR 业务模型
- [DEV-PLAN-362](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/362-person-business-rules-and-detailed-design.md) 的 Person 主档、工号解析与生命周期合同
- [DEV-PLAN-370](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/370-workflow-audit-and-integration-plan.md) 提供的审批、审计与集成边界
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md) 的导入导出、查询工作台、报表与运营视图
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的权限矩阵与高风险边界
- [DEV-PLAN-343](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/343-superadmin-console-and-tenant-lifecycle-business-rules-plan.md) 的控制面与租户生命周期语义
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 的配置/策略目录、Explain 与版本语义
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 对 capability 命名、映射与颗粒度治理底座的冻结
- [DEV-PLAN-353](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md) 的确认摘要、反馈分层与权限感知交互合同
- [DEV-PLAN-395](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/395-assistant-surface-registry-and-enforcement-gates-detailed-design.md) 对 Assistant 覆盖目录、支持级别与强制门禁的详细冻结

`390` 的职责不是“做一个聊天框”，也不是“用对话替代全部业务页面”，而是为 HR 平台建立一套受控、可审计、覆盖全平台的对话式交互能力：

- 能理解用户目标
- 能检索业务对象
- 能提出动作建议
- 能生成确认摘要
- 能把所有可写动作严格约束在业务边界之内
- 能为平台中每一类正式能力给出一致的“可问答 / 可读取 / 可操作 / 可跟踪 / 可回落”入口定义

Assistant 是一层横切能力，不能继续埋在工作流或集成计划里，否则后续会同时污染产品交互、业务写边界和平台治理。
如果 `340/360/370/380` 中存在“用户能在 UI 做、但 Assistant 既不能解释、也不能读取、也不能发起受控动作、也不能给出明确回落面”的暗面能力，则 `390` 视为未达成。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立 Chat Assistant 的产品入口、会话模型与交互规范。
- [ ] 建立“全平台 Assistant Addressability” 合同：`340/360/370/380` 的正式能力与数据面都必须声明自己的 Assistant 支持级别与回落方式。
- [ ] 建立统一的模型网关与提供方适配层。
- [ ] 建立覆盖平台对象、业务对象、工作流对象、工作台对象的只读检索、候选匹配、Explain 与上下文装配能力。
- [ ] 建立受控的 Action Gateway，使 Assistant 只能发起受约束的业务动作请求。
- [ ] 建立 confirm / approve / commit 边界，避免模型越权。
- [ ] 建立复杂操作的明确回落面：当能力不适合直接在聊天内完成时，Assistant 仍必须能解释原因、给出同源状态与引导到对应 UI/确认面。
- [ ] 冻结“权限单轨”合同：Assistant 只能消费操作者同源授权，不新增并行 Assistant 权限体系。
- [ ] 建立 Assistant 的审计、回执、评测与运行治理。

### 2.2 非目标

- [ ] 本计划不把 Assistant 设计成直接数据库写入入口。
- [ ] 本计划不重写 `320` 中各业务模块的主写模型。
- [ ] 本计划不替代前端业务页面；复杂操作仍应回落到明确的业务 UI 或确认页。
- [ ] 本计划不承诺“所有能力都必须以自由文本一步直达提交”；高风险、长事务或高密度表单场景允许采用“Assistant 发起 + UI/审批/任务承接”的交接模式。
- [ ] 本计划不新增 `assistant_role / assistant_permission / assistant_policy` 等并行权限体系。
- [ ] 本计划不授予 Assistant 任意 SQL、任意报表拼接、任意跨租户原始数据读取或绕过导出治理的能力。
- [ ] 本计划不要求一开始就支持多智能体、多模态、长链自治代理。

## 3. 范围

### 3.1 用户能力

- 对话问答与 Explain
- 平台对象与业务对象检索
- 工作流、导入导出、报表、文件与运营状态问答
- 候选澄清
- 动作建议
- 确认摘要
- 操作回执
- 状态跟踪
- 回落到明确业务 UI / 确认页 / 工作台入口

### 3.2 系统能力

- 会话与 turn 持久化
- 模型提供方适配
- 提示词与策略版本化
- 工具调用与 action gateway
- Assistant 覆盖目录与支持级别注册
- 审计与评测

### 3.3 平台覆盖范围

| 能力域 | `390` 至少应覆盖的 Assistant 能力 |
| --- | --- |
| `340/343/345` 平台与控制面 | 租户、域名、管理员初始化、平台配置、动态策略、Explain、版本、状态与控制动作的问答/读取/受控操作/回执跟踪 |
| `360/362` 核心 HR | 组织、人员、职位分类、职位与任职的检索、候选澄清、确认摘要、建议式写入与受控动作请求 |
| `370` 治理协同 | 审批状态、待办、操作回执、集成运行状态、失败原因、可审批动作与长事务跟踪 |
| `380` 数据工作台 | 导入批次、导出任务、查询工作台、报表、运营视图、文件工作台的问答、只读检索、任务发起与状态回执 |

其中“覆盖”不等于“全部在聊天内直接提交完成”。若某能力因风险、复杂度或交互密度不适合纯聊天完成，至少也必须具备：

- 可解释
- 可读取当前状态
- 可给出受控动作入口或明确不支持原因
- 可回落到同源的 UI / 确认页 / 工作台

## 4. 关键设计决策

### 4.1 Assistant 是受控编排层，不是业务主脑（选定）

- 模型负责理解、检索意图、摘要和建议。
- 业务系统负责：
  - 权限
  - 业务校验
  - 审批
  - 提交
  - 审计
- 授权判定只看当前操作者在租户内的真实权限，不因为入口是 Assistant 而切换为第二套权限口径。

### 4.2 Action Gateway 是唯一可写出口（选定）

- 所有 Assistant 发起的可写动作，都必须经过 Action Gateway。
- Gateway 只接受已注册、可审计、可校验的动作类型。
- 模型不得自由拼接内部 API 或写库命令。
- Gateway 生成的 `action request` 是 Assistant 侧唯一权威写动作票据。

### 4.3 对话确认与审批必须显式存在（选定）

- Assistant 不能把“我理解了你的意思”直接视为“允许执行”。
- 需要写入或高风险变更时：
  - 先生成确认摘要
  - 再由用户确认
  - 必要时再进入审批流

### 4.4 评测与治理是正式能力，不是附属日志（选定）

- Assistant 必须有：
  - 运行审计
  - 成本与延迟监控
  - 正确率/可完成率评测
  - Prompt 与 policy 版本管理

### 4.5 运行时采用“两段式 + 一个闸门”（选定）

为降低实现复杂度并避免本地重建第二语义脑，`390` 冻结为“两段式 + 一个闸门”运行时：

#### 4.5.1 语义段（外部大模型）

外部大模型只负责语义输出：

- `intent`
- `slots`
- `action_suggestion`
- `clarification`
- 用户可见回复草稿与确认摘要草稿

约束：

- 在单轮对话内，外部大模型是唯一主语义源。
- 本地不得并行维护第二套 `intent / route / clarification / reply` 主决策链。

#### 4.5.2 执行段（本地 Action Gateway）

本地在一个执行段内完成事实装配、校验与执行：

- `fact_load`
- `dry_run`
- `confirm`
- `approve`（可选）
- `commit`
- 审计、回执、幂等与任务编排

约束：

- 外部大模型不得直接写数据库。
- 外部大模型不得自由拼接内部 API 或业务写命令。
- 外部大模型不得直接决定 commit 放行。

#### 4.5.3 统一闸门（唯一写入口）

- 所有可写动作都必须经过注册动作 + Action Gateway。
- 统一生成并跟踪 `action request`，不得出现绕过闸门的写入链路。
- `acting_channel=assistant` 仅作为审计字段，不作为授权放行条件。

#### 4.5.4 反建设原则（Stopline）

为避免“表面使用大模型，实际在本地重建缩小版助手”，以下能力默认禁止在本地主链长期存在：

- 本地意图分类器与主路由决策器
- 本地澄清主引擎
- 本地回复生成器或二次润色主链
- 用正则/关键词兜底形成第二语义脑
- 在模型完成语义判断后，再由本地重算“下一问是什么”

允许保留的本地逻辑仅限于：

- 输出 schema 校验
- 事实检索执行
- 动作边界校验
- 确定性失败与审计回执

### 4.6 Action Request 是显式状态票据（选定）

- 每个可写动作都必须生成可查询的 `action request`，并具备明确生命周期状态。
- Assistant 只观察 `action request`、审批状态与操作回执，不直接猜测底层业务表是否已经变化。
- 默认通过状态查询接口获取进度，后续可以在不改变语义边界的前提下扩展推送或订阅。
- 最小状态机建议冻结为：`draft -> waiting_confirm -> waiting_approve(可选) -> ready_commit -> committed | failed`。
- 具体状态枚举、审批映射、超时与重试策略，下沉到 `392` 详细设计冻结。

### 4.7 横切能力的判定标准是“全平台可寻址”，不是“聊天替代 UI”（选定）

- `390` 的目标是让平台中每一类正式能力都具备一致的可问答、可读取、可发起受控动作、可查询状态、可回落到 UI 的合同。
- “Assistant 可问答可操作”指在统一安全模型内可寻址、可审计、可交接，不等于所有复杂流程都要以自然语言一步直达提交。
- 对话入口是平台统一入口之一，而不是新的主业务真值来源或新的页面体系。

### 4.8 不允许暗面功能与未声明暴露级别（选定）

- `340/360/370/380` 中新增 capability、工作台动作或高价值查询面时，必须同时声明自己的 Assistant 支持级别。
- 支持级别至少包括：`ask_only / read / draft / action_request / status_track / ui_handoff`（或等价冻结词汇）。
- 未声明支持级别的能力，不得在 `390` 中由模型或本地逻辑隐式猜测支持方式，也不得对外宣称“助手已覆盖该能力”。
- `route -> capability_key`、`assistant_action_id -> capability_key` 与 Assistant 支持级别应来自同一目录或同源编译产物。

### 4.9 门禁 ownership 切分（选定）

- `390` 拥有 Assistant 横切能力的业务目标、支持级别语义目标与“无暗面能力”不变量。
- `395` 拥有 Assistant 覆盖目录、变更触发矩阵、门禁证据口径与 required checks 的 Assistant 侧冻结语义。
- `347` 拥有 capability / action / surface 的静态结构门禁。
- `312` 拥有 Assistant contract / integration / E2E 测试门禁语义。
- `313` 拥有 Assistant 相关结构门禁、测试门禁与 smoke 门禁在流水线中的绑定与执行。

## 5. 功能拆分

### 5.1 M1：Chat 入口与会话模型

- [ ] Chat UI
- [ ] conversation / turn 模型
- [ ] 用户消息与系统回执
- [ ] 基础审计字段

### 5.2 M2：检索与候选匹配

- [ ] 组织、人员、职位、任职对象检索
- [ ] 租户、控制面、配置/策略、审批、导入导出、报表、文件与运营对象检索
- [ ] 候选匹配与 disambiguation
- [ ] `OrgContext` 装配与澄清
- [ ] Explain、版本与当前/历史时间锚点装配
- [ ] 只读工具调用

### 5.3 M3：Action Gateway

- [ ] 注册可用动作
- [ ] `assistant_action_id -> capability_key` 映射注册（与 route-capability 同源治理）
- [ ] 动作参数校验
- [ ] dry-run
- [ ] confirm summary
- [ ] `confirm summary / action request` 显式回显 `business_object_key + org_context + capability_key + time anchor`
- [ ] 核心 HR 维护动作、工作流动作、控制面动作、导入导出/报表任务动作的统一受控发起
- [ ] 与审批流衔接
- [ ] `action request` 生命周期状态与回执查询

### 5.4 M4：全平台覆盖目录与回落面

- [ ] Assistant 支持级别目录
- [ ] `capability -> ask/read/draft/action/status/handoff` 覆盖矩阵
- [ ] 不支持原因与 stopline 表达
- [ ] 同源 UI route / 结果页 / 工作台回落面
- [ ] “无暗面能力”检查口径

### 5.5 M5：运行治理

- [ ] 模型网关
- [ ] provider 配置
- [ ] 审计与回执
- [ ] 评测数据集
- [ ] 成本与延迟指标
- [ ] OpenTelemetry 链路与任务执行观测

## 6. 关键模型方向

- `assistant_conversations`
- `assistant_turns`
- `assistant_action_requests`
- `assistant_action_receipts`
- `assistant_prompt_versions`
- `assistant_eval_runs`
- `assistant_tool_logs`
- `assistant_surface_registry`（可为持久化目录或同源编译产物）

其中：

- `assistant_action_requests` 应承载显式生命周期状态、关联审批/执行引用与最近状态更新时间。
- `assistant_action_receipts` 应承载审批结果、执行回执、异步任务反馈与可审计错误信息。
- `assistant_action_requests / assistant_action_receipts` 至少应回显 `business_object_key`、`org_context`、`capability_key`、`time anchor`、`handoff_route` 与统一安全/审批结果摘要，不得引入聊天专用容器键。
- `assistant_surface_registry` 应声明至少以下字段：`capability_key`、`business_object_key`、支持级别、`assistant_action_id`、`receipt_type`、`risk_level`、`handoff_route`、所需最小权限/审批语义。
- `assistant_action_requests` 必须能覆盖核心 HR 维护动作、审批/治理动作、控制面动作与导入导出/报表任务动作，而不是只服务于单一业务写模型。

## 7. 与其他子计划的关系

- `340` 提供登录、租户、权限、任务与通知基座。
- `350` 提供 Assistant 页面、确认页、结果承接页与回落面应遵循的产品壳、信息架构与交互模式。
- `360` 提供 Org / Person / JobCatalog / Staffing 的业务对象与查询入口。
- `362` 提供 Assistant 在“搜索人员、精确解析工号、解释人员状态、装配人员确认摘要”时必须直接消费的 Person 语义。
- `370` 提供审批、审计增强与动作回执边界。
- `380` 提供导入导出、查询工作台、报表、文件工作台与运营分析对象及其状态语义。
- `342` 提供 Assistant 必须消费的同源权限矩阵、高风险动作边界与导出治理约束。
- `343` 提供控制面动作、租户生命周期与 `platform.control` 语义。
- `345` 提供配置/策略目录、版本、候选池、预览、Explain 与激活发布语义。
- `347` 提供 capability 命名、路由/动作映射与颗粒度治理边界。
- `353` 提供 Assistant 确认摘要、结果页、失败反馈与 `read_only / disabled / 403` 的共享交互边界。
- `390` 必须复用 `340/345/347` 提供的 `OrgContext` 装配、统一授权决议与 Explain 输入，不得自行发明“更适合对话”的第二套上下文键。
- `390` 不能反向拥有任何业务模块的主写模型。

### 7.1 `390` 对 `362` 的显式消费

- 人员检索必须直接消费 `362` 的 Person lookup 合同：
  - `persons:options` 只用于联想与候选；
  - `persons:by-pernr` 才是精确解析；
  - Assistant 不得把联想结果当作最终身份真值。
- 当用户输入带前导 0 的工号时，Assistant 必须沿用 Person 的 canonical 规则，不得自行发明第二套 `pernr` 解析逻辑。
- Assistant 在确认摘要、候选澄清和动作参数装配时，涉及人员至少要稳定引用：`person_uuid / pernr / display_name / status`。
- Assistant 必须把 `active / inactive` 作为 Person 主档生命周期信号来解释，而不是把它误写成任职状态或临时筛选条件。
- Person 页面中的任职展示边界也必须被 Assistant 继承：可以从“人”出发只读检索任职，但写侧仍归 `364`/Staffing。

### 7.2 `390` 对 `380` 的显式消费

- Assistant 查询、报表、导出、导入与文件工作台相关问答，必须直接消费 `380` 冻结的 `Query Workspace / Import Batch / Export Job / Report / File Workbench` 语义，而不是在对话层重新发明第二套查询/报表 DSL。
- 面向查询工作台的问答必须沿用 `OrgContext + current/as_of/history` 的上下文表达，不得把 `as_of`、历史视图或导出筛选偷藏进提示词默认值。
- 导入导出动作必须保留 `batch/job` 与状态回执语义，不允许出现“用户一句话后直接静默写库/静默导出原始数据”的旁路。
- Assistant 可触发的导入/导出/报表类动作，必须继续遵守 `342/330` 的导出与高风险权限治理，而不是因入口是对话而降级约束。

### 7.3 `390` 对 `343/345/342/353` 的显式消费

- Assistant 若触发控制面动作，必须显式消费 `343` 的 `platform.control` 与租户生命周期语义，不能借 tenant app 上下文越权发起全局治理。
- Assistant 读取配置与策略时，必须通过 `345` 的目录、预览、Explain、版本和候选池合同，不得凭提示词猜测字段语义、允许值或激活版本。
- Assistant 的授权判断必须直接消费 `342` 的同源权限矩阵与高风险边界，不得绕过 `tenant.history-maintain / tenant.approve / tenant.export / tenant.govern`。
- Assistant 的确认摘要、结果页与失败反馈必须复用 `353` 的共享产品语义：对象、动作、生效日、影响、是否审批，以及 `read_only / disabled / 403` 的统一解释链。

## 8. 前端与 API 交付面

### 8.1 UI

- `/app/assistant`
- `/app/assistant/catalog`
- `/app/assistant/history`
- `/app/assistant/evals`

### 8.2 API

- `GET /api/assistant/capabilities`
- `POST /api/assistant/conversations`
- `POST /api/assistant/conversations/{id}/turns`
- `POST /api/assistant/action-requests/{id}:confirm`
- `GET /api/assistant/action-requests/{id}`

其中 `GET /api/assistant/capabilities` 应作为 Assistant 覆盖目录与支持级别的权威查询入口，至少返回 capability、支持级别、风险、回落 route 与最小授权/审批语义。
其中 `GET /api/assistant/action-requests/{id}` 应作为 `action request` 生命周期状态的权威查询入口。
返回内容至少应能解释当前 `org_context`、目标对象、时间锚点与最近状态，不得依赖隐藏容器键补充主解释链。

## 9. 验收标准

- [ ] Assistant 能稳定完成对话、检索、候选澄清和建议生成。
- [ ] 外部大模型已成为单轮对话中的唯一主语义源，本地主链不存在第二套并行语义判断中心。
- [ ] 运行时已按“两段式 + 一个闸门”落地，且无旁路写入口。
- [ ] Assistant 与 UI/API 消费同源授权决策，不存在独立 Assistant 权限体系。
- [ ] `340/360/370/380` 的正式 capability 与高价值查询面都已声明 Assistant 支持级别，不存在“业务 UI 已交付、Assistant 无法解释/读取/回落”的暗面能力。
- [ ] 平台与控制面、核心 HR、治理协同、数据工作台四大能力域都已具备至少“可问答 + 可读取 + 可状态跟踪 + 可回落”的 Assistant 覆盖。
- [ ] 所有可写动作都通过 Action Gateway，不存在绕过确认的旁路；其中高风险动作继续遵守审批、导出治理与平台控制边界。
- [ ] Assistant 相关操作均可审计、可追踪、可回放定位。
- [ ] 需要审批或异步执行的动作，能够通过 `action request` 状态与回执持续跟踪，而不是依赖猜测业务表变化。
- [ ] 需要审批的动作能够与 `370` 的工作流边界正确衔接。
- [ ] Assistant 对人员检索、工号解析、人员状态解释与确认摘要装配已经显式消费 `362`，不存在第二套本地人员语义脑。
- [ ] 候选澄清、确认摘要与 `action request` 状态查询已经显式携带 `org_context + time anchor`，不存在聊天专用容器键或第二解释链。
- [ ] Assistant 对控制面、配置/策略、查询工作台、导入导出、报表与文件工作台的问答与操作已经显式消费 `343/345/380` 的同源语义，不存在第二套控制/配置/查询语义脑。
- [ ] Assistant 的授权、确认摘要、失败反馈与结果承接已经显式消费 `342/353` 的同源合同，不存在“同 capability 不同入口不同语义”的情况。
- [ ] `347/312/313/395` 已把 Assistant 覆盖目录、静态门禁、测试门禁与 required checks 接线完成；命中相关触发器的模块变更不得以 `skipped`、手工联调或“后续补 Assistant”通过交付验收。

## 10. 后续拆分建议

1. [ ] `391`：会话与 Chat 交互模型详细设计
2. [ ] `392`：Assistant 编排层与 Action Gateway 详细设计（`action request` 状态机、审批衔接与回执同步）
3. [ ] `393`：检索、候选匹配与上下文装配详细设计
4. [ ] `394`：Assistant 评测、审计与运行治理详细设计
5. [ ] `395`：Assistant 全平台覆盖目录与 UI 回落面详细设计（支持级别矩阵、`handoff_route` 与无暗面门禁）
