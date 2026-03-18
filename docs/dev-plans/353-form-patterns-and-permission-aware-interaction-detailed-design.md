# DEV-PLAN-353：表单与权限感知交互详细设计

**状态**: 规划中（2026-03-18 14:41 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md) 的 `M3: 表单与交互规范` 子计划，同时承接：

- [DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md) 对列表/详情/历史页面骨架、状态承载面与时间上下文的冻结；
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 对“隐藏 / 只读 / 禁用 / 403”四类权限感知表达的冻结；
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 对“未识别租户 / 登录失效 / 租户停用 / 无访问权限”失败态区分的前端输入；
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 对决议预览、版本锚点、Explain 与提交前后口径一致性的要求；
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md) 对 capability/policy_version、effective-dated 写语义与 fail-closed 字段编辑的业务输入；
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 对只读共享包、`as_of + package + read_only` 上下文与“解释为什么不能改”的业务输入；
- [DEV-PLAN-002](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/002-ui-design-guidelines.md) 与 [DEV-PLAN-104](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104-jobcatalog-ui-optimization.md)、[DEV-PLAN-104A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md) 已验证的按钮、表单、日期、错误反馈与危险操作规范。

`352` 已经说明页面骨架该如何统一，但它刻意没有定义：

- 为什么某个入口应该隐藏，而不是禁用；
- 为什么某个页面是只读，而不是 403；
- 表单提交前到底需要预览、确认，还是可以一步保存；
- 后端拒绝后，用户输入该如何保留、错误应该落在哪一层；
- effective-dated 写操作、审批态提交、异步回执、只读共享包这些场景在交互上如何一致表达。

`353` 的职责就是把这些问题收敛为 **Greenfield HR 平台的表单与权限感知交互 SSOT**，让后续业务页不再通过“某个模块现在先这么做”的局部经验继续漂移。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述表单、提交、确认与权限感知，不让 `Dialog`、`Drawer`、按钮变体或局部实现成为主叙事。
- [ ] 冻结 `hidden / read_only / disabled / 403` 四类交互表达的产品语义、适用边界与一致性要求。
- [ ] 冻结表单生命周期：进入编辑、填写、校验、确认、提交、失败、回执跟踪的统一合同。
- [ ] 冻结 effective-dated 写操作、危险操作、审批前置操作、只读共享上下文的交互细则。
- [ ] 为 `351/360/370/380/390` 提供统一交互输入，避免后续页面各自发明“自己的提交语义”和“自己的权限表达”。

### 2.2 非目标

- [ ] 本计划不承接路由树、应用壳、页面骨架与对象页组合方式；这些以 [DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md) 与后续 `351` 为准。
- [ ] 本计划不替代 [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的授权矩阵，只定义产品层如何表达它。
- [ ] 本计划不定义数据库 DDL、后端权限判定算法或前端组件代码实现。
- [ ] 本计划不把 Assistant 编排、审批状态机或导出批处理规则并入表单层；这些能力只消费本计划冻结的交互合同。
- [ ] 本计划不允许通过隐藏错误、弱化失败文案或乐观伪成功来掩盖真实权限/治理边界。

## 3. “业务规则优先”在表单与权限感知交互中的翻译

### 3.1 用户关心的是“我能否做、为什么、做完会怎样”，不是按钮长什么样

用户真正关心的不是：

- 这是 `Dialog` 还是 `Drawer`；
- 是 `contained` 还是 `outlined`；
- 为什么后端回了 403。

用户真正关心的是：

- 我现在是否有资格看到这个入口；
- 如果看得到但不能改，是因为只读、缺权限，还是缺治理前提；
- 我这次提交会改什么、对哪一天生效、是否需要审批；
- 失败后是不是要重填；
- 成功后应该去哪儿验证结果。

因此，`353` 冻结的顺序必须是：

1. 先定义资格与交互语义；
2. 再定义表单生命周期；
3. 最后才决定具体由按钮、对话框、Alert、Snackbar 还是页内面板承载。

### 3.2 权限感知不是“有没有 403”，而是完整的产品表达

`353` 正式承接 [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的四类表达：

- `hidden`
- `read_only`
- `disabled`
- `403`

并选定：

- 它们是四种不同的产品语义，不允许互相代替；
- 同一能力在列表、详情、表单、确认页、Assistant 确认摘要中的含义必须一致；
- 产品层不能把“所有失败都等价为 403”。

### 3.3 提交是业务承诺，不是“点一下按钮”

对 Greenfield HR 来说，提交至少要回答：

- 提交的对象是谁；
- 提交的业务意图是什么；
- 对哪一天生效；
- 当前依据哪一版权限/策略/配置决议；
- 是直接落地、进入审批，还是变成异步/待处理回执。

因此，表单层不得把“提交”简化为：

- 只传一坨字段而不回显关键业务锚点；
- 点击后马上 toast 成功，却没有可靠结果入口；
- 因为实现方便而丢弃用户输入、迫使重填。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的规则与样板

#### 4.1.1 `342` 已冻结四类权限感知表达

- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 已明确：
  - `hidden` 代表当前角色根本不应感知该入口存在；
  - `read_only` 代表可查看但不可修改；
  - `disabled` 代表用户应知道该动作存在，但当前缺权限或缺治理前提；
  - `403` 是直接 URL/API 命中未授权资源时的最终拒绝。

#### 4.1.2 `341` 已冻结入口失败态不能混成一类

- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md) 已明确：
  - 前端必须区分未识别租户、登录失效、租户停用、无访问权限；
  - 前端不得私自缓存第二套当前用户/租户事实源。

#### 4.1.3 UI 基础规范已稳定

- [DEV-PLAN-002](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/002-ui-design-guidelines.md) 已明确：
  - `disabled` 与 `read-only` 必须视觉区分；
  - loading 态需要阻止重复提交；
  - 危险操作必须二次确认；
  - 错误态必须可读、可定位、可行动。

#### 4.1.4 Job Catalog 已验证若干高价值交互细则

- [DEV-PLAN-104](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104-jobcatalog-ui-optimization.md) 与 [DEV-PLAN-104A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md) 已验证：
  - 写对话框必须显式绑定 `effective_date`；
  - 提交按钮附近应再次回显“将对 YYYY-MM-DD 生效”；
  - `read_only=true` 时应禁用提交并解释原因；
  - 后端 403/网络错误应保留用户输入，并在表单内显示明确错误。

#### 4.1.5 业务域已冻结“前端不得猜字段可编辑性”

- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md) 已明确：
  - 字段是否可编辑必须由服务端 capability/policy 决议返回；
  - 写入应带 `policy_version`，防止页面拿旧策略提交。
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 已明确：
  - 共享包是可见但只读；
  - 页面需要解释“为什么当前能看但不能改”。

### 4.2 当前主要缺口

1. [ ] **四类权限表达还未落到统一交互合同**  
   `342` 给了语义，但还没有一份文档说明它们在按钮、表单、详情页、确认页各自如何落地。
2. [ ] **提交生命周期仍缺统一语言**  
   各模块容易各自发明“保存成功”“提交审批”“排队处理中”的表达与跳转。
3. [ ] **失败反馈容易混层**  
   字段错误、表单错误、页面错误、全局错误目前缺一份明确分层文档。
4. [ ] **effective-dated 与策略版本写入还缺统一交互细则**  
   什么时候必须显式回显生效日、什么时候必须携带 `policy_version`、版本过期后怎样提示，还没有统一产品语言。
5. [ ] **危险操作与治理前置操作仍可能被做得过轻**  
   删除、停用、历史更正、导出、发布、审批前置这类动作若不统一约束，很容易被普通“保存”按钮稀释。

## 5. 表单与权限感知交互目标蓝图

### 5.1 领域使命

`353` 是 Greenfield HR 平台内“**用户如何进入编辑、系统如何表达权限与治理边界、提交前如何确认、失败后如何恢复、成功后如何追踪结果**”的唯一交互权威。  
它不拥有授权真值、不拥有页面骨架、不拥有业务规则，但拥有所有写交互和权限感知表达共同依赖的产品语言。

### 5.2 核心交互对象

| 交互对象 | 业务含义 | 是否由 `353` 拥有 |
| --- | --- | --- |
| `InteractionMode` | 当前页面处于 hidden / read_only / editable / disabled / forbidden 中哪种交互模式 | 是 |
| `ActionPresentation` | 某个动作入口应隐藏、展示为可点、还是展示为禁用并带原因 | 是 |
| `FieldPresentationState` | 某字段当前是 hidden / read_only / editable / required / invalid 中哪种状态 | 是 |
| `FormDraft` | 用户当前正在填写但尚未提交的局部意图 | 是 |
| `ConfirmationSummary` | 提交前需回显的对象、动作、生效日、风险、影响范围摘要 | 是 |
| `SubmitEnvelope` | 提交时最小必要锚点：对象、意图、生效日、版本锚点、幂等语义 | 是（产品合同） |
| `FeedbackLayer` | 错误/警告/成功/进行中消息应落在哪一层 | 是 |
| `ReceiptHandoff` | 提交后跳往哪里查看结果、审批、异步回执 | 是 |
| `AuthorizationDecision` | allow/deny 的原始权限决议 | 否，`342` 拥有 |
| `PolicyDecisionSnapshot` | 字段/选项/策略的运行时决议结果 | 否，`345/361/363` 提供 |

### 5.3 面向用户的主能力

- 理解当前能不能看、能不能改、为什么；
- 在同一页面骨架内进入编辑而不丢上下文；
- 在提交前看到关键业务锚点与风险摘要；
- 在失败后保留输入、快速修正并重试；
- 在成功、待审批、异步处理中进入可靠的结果追踪入口；
- 在只读共享、治理前置、版本过期等复杂场景下得到明确可解释反馈。

### 5.4 选定的交付形态

#### 5.4.1 权限感知表达采用“四分语义”

`353` 选定：

- **hidden**
  - 适用于用户根本不应感知该入口或字段存在；
  - 默认不留占位，不显示“你没有权限看到它”。
- **read_only**
  - 适用于允许查看，但当前上下文不允许改；
  - 值应可读、可选中/复制（如适用），并有显式只读标识。
- **disabled**
  - 适用于用户应知道动作存在，但当前缺权限、缺前置条件或对象状态不满足；
  - 动作仍可见，但必须提供原因提示。
- **403**
  - 适用于直接命中未授权路由/API 的最终拒绝；
  - 页面需明确区分它与未登录、租户停用、上下文无效。

#### 5.4.2 表单采用“草稿 - 确认 - 提交 - 回执”生命周期

`353` 冻结的统一交互流程：

1. 进入编辑态  
   用户从列表、详情或工作台进入局部编辑，不应丢失上下文锚点。
2. 填写草稿  
   页面根据服务端返回的字段决议决定哪些字段可见、可改、必填。
3. 本地与服务端校验  
   本地只做确定性、可立即发现的校验；业务与权限真值仍以服务端为准。
4. 确认摘要  
   对高风险或 effective-dated 写操作，提交前必须回显对象、动作、生效日、关键差异与风险。
5. 提交  
   按 `SubmitEnvelope` 发送最小必要锚点，不得只发一坨无语义字段。
6. 结果承接  
   成功、待审批、异步处理中、失败，分别进入不同反馈与追踪路径。

#### 5.4.3 提交前确认按风险分层

`353` 选定三档：

- **C1：低风险直接保存**  
  普通字段维护、明确低风险且无需审批的动作，可一步提交，但仍要有清晰结果反馈。
- **C2：业务确认后提交**  
  effective-dated 写入、影响多个字段、可能改变当前显示结果的动作，提交前必须展示确认摘要。
- **C3：危险/治理型确认**  
  删除、停用、历史更正、导出、发布、激活、审批前置动作必须使用明确二次确认，且按钮文案不能写成“确定”。

#### 5.4.4 提交结果必须回到可靠结果面

`353` 选定：

- 同步成功：返回当前页面可验证的结果位置，例如列表高亮、详情刷新、历史新增节点；
- 待审批：跳转或链接到审批/回执入口；
- 异步处理中：展示 `receipt_pending`，并提供票据或历史入口；
- 失败：原表单仍在，输入不丢，错误可修正。

## 6. `353` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心交互规则 | 业务结果 |
| --- | --- | --- | --- |
| 没资格感知功能 | 不应看到某能力入口 | 使用 `hidden`，不占主界面注意力，不用 403 替代 | 界面不泄露不该感知的能力 |
| 能看不能改 | 查看对象但不可维护 | 使用 `read_only`；值仍可读，编辑控件与主提交入口转为只读表达 | 用户知道当前是浏览态 |
| 应知道动作存在但当前不能做 | 明白动作存在及受限原因 | 使用 `disabled`，必须有原因提示，不靠点击后 403 教育用户 | 治理或权限前置可解释 |
| 直接命中未授权资源 | 深链或 API 命中未授权页面 | 使用 `403` 最终拒绝页/面板，并区分未登录、租户停用、无权限等原因 | 拒绝原因明确且可恢复 |
| 编辑 effective-dated 对象 | 确认改动对哪一天生效 | 表单必须显式展示 `effective_date`，提交按钮附近再次回显“将对 YYYY-MM-DD 生效” | 生效日不再被隐式猜测 |
| 提交策略驱动字段 | 按当前规则安全提交 | 前端只能消费服务端字段决议；提交必须带版本锚点，如 `policy_version` | 不会拿旧规则盲提 |
| 危险操作 | 删除、停用、历史更正、发布等 | 不得作为普通主按钮直出；必须二次确认，文案明确，影响范围可见 | 高风险动作不被轻率提交 |
| 校验失败 | 修正输入后重试 | 字段错误就地显示；表单级错误留在表单内；用户输入保留 | 用户不需要重填整单 |
| 网络/403/上下文失败 | 理解这次失败属于哪一类 | 页面、表单、全局错误按层承载；不把所有失败混成一条 toast | 用户知道下一步该怎么恢复 |
| 提交后需要审批或异步执行 | 持续跟踪结果 | 成功页不是终点；必须提供 receipt/history/workflow 入口 | 用户有稳定的结果追踪路径 |

## 7. 共享合同、不变量与实现护栏

### 7.1 权限感知合同

前端必须遵守：

- 不得自己推导授权真值；
- 不得把所有受限态都做成 `disabled`；
- 不得把所有拒绝都延迟到 403；
- 不得在列表、详情、表单、确认页上对同一能力给出相互矛盾的权限表达。

### 7.2 字段状态合同

字段状态至少包括：

- `hidden`
- `read_only`
- `editable`
- `required`
- `invalid`

冻结原则：

- 字段可编辑性来自服务端决议，而不是客户端猜测；
- `required` 不等于 `editable`，只读字段仍可能是必备背景信息；
- 非法字段、未知字段、未启用字段一律不得被前端偷偷提交。

### 7.3 提交合同

每次正式提交至少应绑定：

- 对象锚点；
- 业务意图；
- 关键上下文锚点，如 `effective_date / as_of / package`；
- 决议版本锚点，如 `policy_version` 或等效版本；
- 幂等/回执关联标识（如存在）。

实现护栏：

- 不允许提交按钮只说“保存”却不回显关键业务锚点；
- 不允许一成功就关闭页面但不给用户验证落点；
- 不允许失败后清空用户输入。

### 7.4 反馈分层合同

反馈至少分四层：

- **字段级**：字段自身格式或值域错误；
- **表单级**：跨字段、权限、业务规则、版本过期等错误；
- **页面级**：页面加载失败、上下文缺失、结果集为空、无权访问；
- **全局级**：跨页面任务完成提醒、系统级告警、长事务状态更新。

冻结原则：

- 字段错误不要只用全局 toast；
- 页面错误不要塞进字段 `helperText`；
- 403、登录失效、租户停用不能与一般业务校验失败混层。

### 7.5 重复提交与并发护栏

前端必须：

- 在提交进行中禁用重复提交；
- 长任务提供明确进行中状态；
- 版本过期、对象状态变化、治理前置变化导致失败时，返回可解释刷新路径，而不是静默覆盖。

### 7.6 危险操作护栏

危险操作必须满足：

- 主文案明确，如“删除”“停用”“发布”“回滚”，禁止仅写“确定”；
- 二次确认中必须回显对象、关键影响与不可逆后果；
- 不与普通主按钮并列抢占主视觉；
- 若动作将进入审批或异步执行，确认页必须说明后续不是“立刻生效”。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `351`（Product Shell 与路由信息架构）的输入

- [ ] 全局壳层应为登录失效、租户停用、403、系统级告警预留统一反馈容器。
- [ ] 深链路由与页面返回路径需要支持“提交后回到可靠结果面”的交互闭环。

### 8.2 对 `360`（核心 HR 业务域）的输入

- [ ] `361/362/363/364` 的表单必须消费服务端字段决议，不得自行猜字段是否可改。
- [ ] effective-dated 主对象的写操作必须显式展示 `effective_date`，并在提交前回显业务影响。
- [ ] 业务页必须区分“对象只读”“动作禁用”“直接无权访问”。

### 8.3 对 `370`（工作流、审计增强与集成）的输入

- [ ] 审批前置动作在交互上不得伪装成已成功提交；必须明确“已提交审批”与“已生效”的区别。
- [ ] 审批、回执、异步执行结果应接入 `353` 冻结的反馈分层与结果承接方式。

### 8.4 对 `380`（数据工作台与运营分析）的输入

- [ ] 导出、批量导入、批处理诊断等高风险动作必须采用治理型确认，不得做成轻量普通按钮。
- [ ] 批次失败与行级错误应区分表单级和工作台级反馈，而不是都挤进全局提示。

### 8.5 对 `390`（Chat Assistant）的输入

- [ ] Assistant 确认摘要应与 Web 表单确认摘要使用同一组产品语义：对象、动作、生效日、影响、是否审批。
- [ ] Assistant 不得因为“对话更自然”而跳过 `read_only / disabled / 403` 的共享边界。
- [ ] Assistant 结果页与动作回执页应复用本计划的反馈分层与结果承接方式。

## 9. 建议实施步骤

1. [ ] `M1`：权限感知表达冻结  
   把 `hidden / read_only / disabled / 403` 从抽象语义落实到页面、字段、动作与直达访问的交互合同。
2. [ ] `M2`：表单生命周期冻结  
   明确草稿、校验、确认、提交、失败、回执跟踪的共享步骤。
3. [ ] `M3`：effective-dated 与策略驱动写入冻结  
   明确 `effective_date`、`policy_version`、只读共享上下文与版本过期交互。
4. [ ] `M4`：危险操作与治理前置冻结  
   明确删除、停用、导出、发布、审批前置操作的确认与结果反馈。
5. [ ] `M5`：下游引用收口  
   让 `351/360/370/380/390` 正式引用 `353`，不再各自重写权限感知与提交交互规则。

## 10. 验收标准

- [ ] `353` 已成为 Greenfield HR 平台表单与权限感知交互的单一事实源，而不是继续分散在 `002/104/342/361/363/352` 的局部经验中。
- [ ] 后续业务子计划可以直接引用 `353`，不再各自定义“隐藏、只读、禁用、403”。
- [ ] effective-dated 写入、策略驱动字段、只读共享上下文与危险操作已具备统一的提交与确认语言。
- [ ] 失败反馈已按字段、表单、页面、全局四层分开，不再互相混用。
- [ ] `353` 与 `352`、`342`、`351` 边界清晰：不重复页面骨架，不越权拥有授权真值。

## 11. 关联文档

- [DEV-PLAN-002](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/002-ui-design-guidelines.md)
- [DEV-PLAN-104](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104-jobcatalog-ui-optimization.md)
- [DEV-PLAN-104A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md)
- [DEV-PLAN-341](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/341-tenancy-authn-business-rules-and-entry-boundary-plan.md)
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md)
- [DEV-PLAN-361](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
