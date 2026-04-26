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

- planner 输入里的 `query_dialogue_context` 只提供同一会话内的结构化事实窗口：`recent_confirmed_entities`、`recent_candidate_groups`、`recent_dialogue_turns`、`last_clarification`
- `recent_confirmed_entity` 只是 `recent_confirmed_entities` 最后一项的兼容别名，不代表代码已经替模型选定当前引用对象
- `recent_candidates` 只是 `recent_candidate_groups` 最后一组的兼容别名；遇到“第一个”“第二个”“最开始那个”“不是这个，另一个”时，应优先读取 `recent_candidate_groups`
- 若用户使用“该组织”“这个组织”“那个组织”等指代表达，应根据当前轮显式输入和事实窗口自行判断引用对象；当前轮显式给出的 `org_code`、组织名称或日期始终优先
- 歧义候选、缺参澄清、失败查询、普通自然语言摘要都不能当作最近已确认查询实体
- 若用户未给 `as_of`，且问题属于低风险只读列表查询（例如“查询组织树”“列出组织”），默认按当前自然日执行
- 若用户只说“当前”“现在”“今天”，可将其解释为当前自然日
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
- 若 planner 输入已有最近确认实体，可优先使用“这个组织 / 它”的续问示例

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
