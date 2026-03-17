# DEV-PLAN-340：Chat Assistant 能力子计划

**状态**: 规划中（2026-03-17 07:23 CST）

## 1. 背景与上下文

本计划承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的总体蓝图
- [DEV-PLAN-310](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/310-platform-and-iam-foundation-plan.md) 的平台基座
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-core-hr-domains-plan.md) 的核心 HR 业务模型
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-workflow-audit-assistant-and-integration-plan.md) 提供的审批、审计与集成边界

`340` 的职责不是“做一个聊天框”，而是为 HR 平台建立一套受控的对话式交互能力：

- 能理解用户目标
- 能检索业务对象
- 能提出动作建议
- 能生成确认摘要
- 能把所有可写动作严格约束在业务边界之内

Assistant 是一层横切能力，不能继续埋在工作流或集成计划里，否则后续会同时污染产品交互、业务写边界和平台治理。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立 Chat Assistant 的产品入口、会话模型与交互规范。
- [ ] 建立统一的模型网关与提供方适配层。
- [ ] 建立业务对象只读检索、候选匹配与上下文装配能力。
- [ ] 建立受控的 Action Gateway，使 Assistant 只能发起受约束的业务动作请求。
- [ ] 建立 confirm / approve / commit 边界，避免模型越权。
- [ ] 建立 Assistant 的审计、回执、评测与运行治理。

### 2.2 非目标

- [ ] 本计划不把 Assistant 设计成直接数据库写入入口。
- [ ] 本计划不重写 `320` 中各业务模块的主写模型。
- [ ] 本计划不替代前端业务页面；复杂操作仍应回落到明确的业务 UI 或确认页。
- [ ] 本计划不要求一开始就支持多智能体、多模态、长链自治代理。

## 3. 范围

### 3.1 用户能力

- 对话问答
- 业务对象检索
- 候选澄清
- 动作建议
- 确认摘要
- 操作回执

### 3.2 系统能力

- 会话与 turn 持久化
- 模型提供方适配
- 提示词与策略版本化
- 工具调用与 action gateway
- 审计与评测

## 4. 关键设计决策

### 4.1 Assistant 是受控编排层，不是业务主脑（选定）

- 模型负责理解、检索意图、摘要和建议。
- 业务系统负责：
  - 权限
  - 业务校验
  - 审批
  - 提交
  - 审计

### 4.2 Action Gateway 是唯一可写出口（选定）

- 所有 Assistant 发起的可写动作，都必须经过 Action Gateway。
- Gateway 只接受已注册、可审计、可校验的动作类型。
- 模型不得自由拼接内部 API 或写库命令。

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

### 4.5 本地助手与外部大模型采用“三层分工界面”（选定）

为最大化利用外部大模型能力，并避免在本地重建第二语义脑，`340` 冻结以下分工界面：

#### 4.5.1 语义层：由外部大模型主导

外部大模型负责：

- 用户目标理解
- 自然语言归一与槽位修复
- 缺失信息判断
- 下一问生成
- 用户可见回复与确认摘要初稿
- 是否需要检索、检索什么
- 候选对象选择建议

约束：

- 在单轮对话内，外部大模型是唯一主语义源。
- 本地不得再并行维护第二套 `intent / route / clarification / reply` 主决策链。

#### 4.5.2 事实层：由本地助手提供与回填

本地助手负责向模型提供和回填：

- 当前租户、用户、角色与权限上下文
- 最近会话摘要
- 允许动作白名单
- 真实业务对象检索结果
- 候选匹配结果
- dry-run 结果
- 审批状态、审计回执与任务状态

约束：

- 本地负责提供事实，不负责再次解释“用户真正想做什么”。
- 本地允许校验模型输出是否合法，但不得把模型输出重新翻译成另一套语义结论。

#### 4.5.3 执行层：由本地助手严格掌控

本地助手必须独占以下确定性职责：

- Action Gateway
- 参数校验
- 权限校验
- 业务规则校验
- dry-run
- confirm / approve / commit
- 审计、回执、幂等与任务编排

约束：

- 外部大模型不得直接写数据库。
- 外部大模型不得自由拼接内部 API 或业务写命令。
- 外部大模型不得直接决定 commit 放行。
- 所有可写动作都必须经过注册动作、确定性校验和显式确认/审批边界。

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

## 5. 功能拆分

### 5.1 M1：Chat 入口与会话模型

- [ ] Chat UI
- [ ] conversation / turn 模型
- [ ] 用户消息与系统回执
- [ ] 基础审计字段

### 5.2 M2：检索与候选匹配

- [ ] 组织、人员、职位、任职对象检索
- [ ] 候选匹配与 disambiguation
- [ ] 上下文装配
- [ ] 只读工具调用

### 5.3 M3：Action Gateway

- [ ] 注册可用动作
- [ ] 动作参数校验
- [ ] dry-run
- [ ] confirm summary
- [ ] 与审批流衔接

### 5.4 M4：运行治理

- [ ] 模型网关
- [ ] provider 配置
- [ ] 审计与回执
- [ ] 评测数据集
- [ ] 成本与延迟指标

## 6. 关键模型方向

- `assistant_conversations`
- `assistant_turns`
- `assistant_action_requests`
- `assistant_action_receipts`
- `assistant_prompt_versions`
- `assistant_eval_runs`
- `assistant_tool_logs`

## 7. 与其他子计划的关系

- `310` 提供登录、租户、权限、任务与通知基座。
- `320` 提供 Org / Person / JobCatalog / Staffing 的业务对象与查询入口。
- `330` 提供审批、审计增强与动作回执边界。
- `340` 不能反向拥有任何业务模块的主写模型。

## 8. 前端与 API 交付面

### 8.1 UI

- `/app/assistant`
- `/app/assistant/history`
- `/app/assistant/evals`

### 8.2 API

- `POST /api/assistant/conversations`
- `POST /api/assistant/conversations/{id}/turns`
- `POST /api/assistant/action-requests/{id}:confirm`
- `GET /api/assistant/action-requests/{id}`

## 9. 验收标准

- [ ] Assistant 能稳定完成对话、检索、候选澄清和建议生成。
- [ ] 外部大模型已成为单轮对话中的唯一主语义源，本地主链不存在第二套并行语义判断中心。
- [ ] 所有可写动作都通过 Action Gateway，不存在绕过确认的旁路。
- [ ] Assistant 相关操作均可审计、可追踪、可回放定位。
- [ ] 需要审批的动作能够与 `330` 的工作流边界正确衔接。

## 10. 后续拆分建议

1. [ ] `341`：会话与 Chat 交互模型详细设计
2. [ ] `342`：Assistant 编排层与 Action Gateway 详细设计
3. [ ] `343`：检索、候选匹配与上下文装配详细设计
4. [ ] `344`：Assistant 评测、审计与运行治理详细设计
