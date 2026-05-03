# OrgUnit CubeBox Skill

## 模块定位

`orgunit` 模块支持当前租户下组织架构的只读查询。CubeBox 只能生成 API-first 查询计划，不能触发写入、修正、停用、恢复、移动或重命名。

本技能包帮助 planner：

- 判断用户是否在询问组织架构
- 从自然语言补齐只读 API 参数
- 缺少必要事实时输出 `CLARIFY`
- 将可执行查询映射为 `API_CALLS`
- 结合 `working_results` 决定继续调用 API 或输出 `DONE`

## 主要业务对象

- `orgunit`：组织单元
- `org_code`：组织编码，是精确查询的稳定标识
- `as_of`：查询日期，日粒度，格式必须为 `YYYY-MM-DD`
- `include_disabled`：是否包含停用组织
- `status`：列表过滤只使用 `active`、`disabled`、`all`

## 上下文原则

- `query_evidence_window` 只是同一会话内的事实窗口，不是本地目标绑定。
- 当前轮显式给出的组织编码、组织名称、日期和范围优先。
- 代词如“该组织”“这个组织”“第一个”“全部”必须由 planner 结合当前输入、`recent_turns`、`observations` 和 `open_clarification` 判断；无法稳定判断时输出 `CLARIFY`。
- 用户明确纠正或扩大范围时，例如“不只是包含成本关键字的组织，而是全部组织”，不得继承历史 `keyword`、`parent_org_code`、单个 `entity_key` 或 `result_list`。
- `working_results` 只表示当前 turn 内已经执行过的 API observation；不要把它写成长时记忆或业务专用队列。
- 若 `working_results.latest_observation.items` 已足够回答，输出 `DONE`。

## 查询默认值

- 未给 `as_of` 且属于低风险只读列表查询时，默认当前自然日。
- “今天”“当前”“现在”按当前自然日解释。
- “本月9日”“这个月9号”按 planner system prompt 给出的当前年月补成完整日期。
- 未说明范围的“组织树”“一级组织”“列出组织”默认查询当前租户一级组织。
- “全部组织”“所有组织”“全租户组织清单”使用 `all_org_units=true`。
- 未说明包含停用组织时，默认 `include_disabled=false`。
- 列表分页使用 `page` / `page_size`；两者是执行控制参数，不是业务缺参。
- 缺省分页默认 `page=1,page_size=100`，不要向用户追问分页默认值。
- 用户只回复一个正整数作为分页短答时，优先当作 `page_size`，`page` 默认 1。

## 查询域 fail-closed

- 用户仍明显在问组织架构时，不要输出 `NO_QUERY` 让普通聊天链自由回答能力边界。
- 缺少 API required 参数或引用对象不稳定时，输出 `CLARIFY`。
- 会话压缩摘要不能作为查询锚点；不要从自然语言摘要中猜测 `org_code` 或 `as_of`。
- 页面事实不在当前范围内；不要假设用户正在看的页面对象就是当前查询对象。

## 文件关系

- 查询意图与补参规则见 `queries.md`
- 可调用 API tool overlay 引用见 `apis.md`
- API-first 输出示例见 `examples.md`
