# OrgUnit CubeBox Skill

## 模块定位

`orgunit` 模块负责组织架构的只读查询与写入维护。

在 `CubeBox` 查询场景中，首期只允许围绕当前租户下的组织架构做只读查询，不允许触发任何写入、修正、停用、恢复、移动或重命名操作。

本技能包的职责是帮助模型完成四件事：

- 理解用户是否在询问组织架构
- 从自然语言中选择默认值并补全查询参数
- 在缺少必要参数时先发起澄清
- 把问题映射为受控的 `ReadPlan`
- 在拿到原始结果后直接完成最终结果叙述

## 主要业务对象

- `orgunit`：组织单元
- `org_code`：组织编码，是大多数精确查询的稳定标识
- `as_of`：查询时点，日粒度，格式必须为 `YYYY-MM-DD`
- `include_disabled`：是否包含无效/停用组织
- `status`：列表过滤时只使用 canonical 值 `active`、`disabled`、`all`

## 上下文补参与继承原则

- planner 输入里的 `query_evidence_window` 只提供同一会话内的事实窗口：`current_user_input`、`recent_turns`、`observations`、`open_clarification`
- `query_evidence_window` 不是本地目标绑定，也不是代码已经替模型选定的组织、日期、候选集合或审计对象
- `observations.kind=entity_fact` 只表示先前工具结果曾产生某个实体事实；`observations.kind=presented_options` 只表示先前给用户展示过一组选项
- 若 `open_clarification.reply_candidate=true`，优先判断当前短输入是否在回答上一轮澄清；不要因为输入很短就误判为 `NO_QUERY`
- `open_clarification.raw_user_reply` 是当前轮原文；`known_params` 只可消费结构化保留的已知事实，不能假设代码已经做了自然语言解析
- `open_clarification.options` 是上一轮澄清相关的选项；用户答“以上”“以上全部”“全部”“都要”时，由模型结合 `recent_turns`、`observations` 和当前输入判断是否是集合答复
- 若用户使用“该组织”“这个组织”“那个组织”等指代表达，应根据当前轮显式输入和事实窗口自行判断引用对象；当前轮显式给出的 `org_code`、组织名称或日期始终优先
- 歧义候选、缺参澄清、失败查询、普通自然语言摘要都不能当作最近已确认查询实体
- 若用户未给 `as_of`，且问题属于低风险只读列表查询（例如“查询组织树”“列出组织”），默认按当前自然日执行
- 若用户只说“当前”“现在”“今天”，可将其解释为当前自然日
- 若用户说“本月9日”“这个月9号”这类相对月内日期，应结合 planner system prompt 给出的当前自然日年月补成完整 `YYYY-MM-DD`，不能要求用户再给完整日期
- 若上一轮已经在追问日期，而当前轮只给 `1日`、`1号`、`1月1日`、`那就1日` 这类残缺日期短答，应优先结合 `open_clarification`、最近问答和领域语义补全完整 `YYYY-MM-DD`；若上下文不足，继续澄清
- 若用户未说明范围，且问题是在问“组织树”“一级组织”“全部组织”，默认先查询当前租户一级组织
- 若用户没有明确要求包含停用组织，默认 `include_disabled=false`
- 若无法稳定判断当前指代对象，应输出带 `clarifying_question` 的澄清型 `ReadPlan`，不要静默选择候选

## 查询域 fail-closed 原则

- 用户输入仍明显在问组织架构时，不要输出 `NO_QUERY` 后让普通聊天链自由回答系统能力边界
- 若缺少可执行参数，应输出带 `missing_params` 与 `clarifying_question` 的 `ReadPlan`
- 若用户使用代词但 planner 输入没有最近已确认查询实体，应澄清要查询的组织编码或名称，不要说“没有查询接口/权限”
- 会话压缩摘要只用于普通对话连续性；不得把自然语言摘要当作查询锚点，也不得从摘要中猜测 `org_code` 或 `as_of`
- 页面事实不在当前范围内；不要假设用户正在看的页面对象就是当前查询对象

## no-query-guidance

- `scope_summary` 与 `suggested_prompts` 由 `queries.md` 的 `no_query_guidance` 结构化片段提供
- 默认示例必须使用组织名称、关键词或关系型问法，不把编码作为默认示例
- 不根据最近实体或候选切换本地续问示例；短输入应先交给 planner 根据 `query_evidence_window` 判断

## 首期允许的查询意图

- `orgunit.details`
- `orgunit.list`
- `orgunit.search`
- `orgunit.audit`

## 约束

- 只能生成只读查询计划
- 不能要求访问数据库、SQL、内部 store 或未登记接口
- 不能声明新的 `api_key`
- 不能把知识包当作执行事实源；执行事实源以后续代码注册表为准
- 不要依赖本地隐藏协议或跨步字段引用来续执行详情查询；如果搜索后仍需要用户确认目标，应回到澄清
- 不要为执行层补自然语言别名；例如列表状态过滤不要产出 `inactive`

## 文件关系

- 查询意图与补参规则见 `queries.md`
- 允许的 `api_key` 与参数说明见 `apis.md`
- 问法与 `ReadPlan` 示例见 `examples.md`
