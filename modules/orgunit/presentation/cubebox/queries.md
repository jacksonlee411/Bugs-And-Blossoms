# OrgUnit Queries

## 查询意图总表

```yaml
intents:
  - key: orgunit.details
    description: 查询单个组织在指定时点的详情
    required_params: [org_code, as_of]
    optional_params: [include_disabled]
  - key: orgunit.list
    description: 查询组织列表、全租户关键词组织列表，或某个上级组织下的直接子组织列表
    required_params: [as_of]
    optional_params: [include_disabled, parent_org_code, keyword, status, is_business_unit, page, size]
  - key: orgunit.search
    description: 根据关键词搜索组织并返回命中的组织与路径
    required_params: [query, as_of]
    optional_params: [include_disabled]
  - key: orgunit.audit
    description: 查询某个组织的审计事件摘要
    required_params: [org_code]
    optional_params: [limit]
no_query_guidance:
  scope_summary: 当前主要支持组织相关只读查询。
  suggested_prompts:
    - 查“华东销售中心”的详情
    - 查“华东销售中心”当前的下级组织
    - 搜索名称包含“销售”的组织
```

## 意图细则

### `orgunit.details`

适用于：

- “查看 1001 组织详情”
- “查一下华东销售中心在 2026-04-01 的信息”
- “这个组织当前负责人是谁”

参数规则：

- 必填：`org_code`、`as_of`
- 可选：`include_disabled`
- 若用户只给名称、不给编码，不应直接猜编码；应先改走 `orgunit.search` 或发起澄清

缺参追问：

- 缺 `org_code`：请提供组织编码，或先告诉我要查的组织名称
- 缺 `as_of`：请告诉我要按哪一天查询，格式例如 `2026-04-23`

### `orgunit.list`

适用于：

- “列出当前所有一级组织”
- “看一下 1001 下面的子组织”
- “按名称搜今天有效的组织列表”
- “列出全部业务单元”

参数规则：

- 必填：`as_of`
- 可选：`include_disabled`、`parent_org_code`、`keyword`、`status`、`is_business_unit`、`page`、`size`
- `status` 如需填写，只使用 canonical 值 `active`、`disabled`、`all`
- 若用户只说“查询组织树”“列出组织”“看组织树”，未给 `as_of` 时默认按当前自然日
- 若用户未说明范围，默认先查当前租户一级组织，不要求首轮必须提供 `parent_org_code`
- 若用户说“列出全部/所有的 X 组织”“名称包含 X 的组织列表”，且没有给上级组织范围，使用 `keyword` 且不要填写 `parent_org_code`，表示在当前租户全部有效组织中检索
- 若用户说“业务单元”“全部业务单元”“所有业务单元”，使用 `is_business_unit=true`；若没有给上级组织范围，不要填写 `parent_org_code`，表示在当前租户全部有效组织中按业务单元标记过滤
- 若用户只说“某个组织下面有哪些组织”，优先使用 `parent_org_code`
- 若用户强调“分页”“第几页”“每页多少条”，可补 `page`、`size`

缺参追问：

- 仅当用户明确要求历史日期且日期无法确定时：请告诉我要按哪一天列出组织，格式例如 `2026-04-23`
- 仅给上级组织名称未给编码，且范围必须限定到某个上级组织时：请先提供上级组织编码，或允许我先搜索定位该组织

### `orgunit.search`

适用于：

- “搜索包含销售的组织”
- “帮我找一下华东”
- “查一下名字里有共享服务的组织”

参数规则：

- 必填：`query`、`as_of`
- 可选：`include_disabled`
- `query` 应保留用户原始搜索词，不要擅自扩写
- 搜索结果主要用于定位目标组织
- 如果用户后续还要看详情，优先让用户确认或直接提供 `org_code`，不要依赖本地隐藏字段把搜索结果自动续接到 `orgunit.details`

缺参追问：

- 缺 `query`：请告诉我要搜索的组织名称或关键词
- 缺 `as_of`：请告诉我要按哪一天搜索，格式例如 `2026-04-23`

### `orgunit.audit`

适用于：

- “看一下 1001 的最近变更”
- “这个组织最近被谁改过”
- “查这个组织的审计记录”

参数规则：

- 必填：`org_code`
- 可选：`limit`
- 若用户未给 `limit`，可使用系统默认值；不要要求用户必须提供

缺参追问：

- 缺 `org_code`：请提供要查看审计记录的组织编码

## 自然语言映射提示

- “详情”“信息”“负责人”“父组织”“全路径” 通常映射到 `orgunit.details`
- “列表”“下面有哪些”“子组织”“分页” 通常映射到 `orgunit.list`
- “搜索”“找一下”“名称里有” 通常映射到 `orgunit.search`
- “审计”“变更记录”“谁改过”“最近变更” 通常映射到 `orgunit.audit`
- “该组织”“这个组织”“那个组织”“最开始那个组织”“第一个”属于查询连续性指代；应读取 planner 输入里的 `query_evidence_window`
- `query_evidence_window.observations` 只提供历史工具 observation 和轻量结果摘要，不是本地目标绑定
- `observations.kind=entity_fact` 只表示先前工具结果曾产生某个实体事实；模型需结合当前输入和 `recent_turns` 判断是否引用它
- `observations.kind=presented_options` 只表示先前给用户展示过一组选项；当用户说“第一个”“第二个”“最开始那个”“不是这个，另一个”时，由模型结合 `recent_turns` 与当前输入解析
- 若 `query_evidence_window.open_clarification.reply_candidate=true`，先判断当前轮是否在回答上一轮澄清；不要因为输入短就退回 `NO_QUERY`
- `open_clarification.raw_user_reply` 是当前轮原文；`open_clarification.known_params` 只可消费结构化保留的已知事实，不能假设代码已经做了自然语言解析
- `open_clarification.options` 是上一轮澄清相关的选项；当前轮答“以上”“以上全部”“全部”“都查”“都要”时，由模型判断是否为集合答复，不要重新要求用户选择范围 A/B/C
- 若用户说“本月9日”“这个月9号”，使用 planner system prompt 中的当前自然日年月补成完整 `YYYY-MM-DD`；例如当前自然日为 `2026-04-25` 时应输出 `2026-04-09`
- 若上一轮已有 `2025年1月` 这类日期上下文，而当前轮只给 `1日`、`1号`、`1月1日`，应优先结合 `open_clarification`、最近问答和当前轮原文补全完整 `YYYY-MM-DD`；若上下文不足，继续澄清
- 若用户仍在询问组织架构，但缺少可继承实体或必要参数，应返回澄清型 `ReadPlan`，不得输出 `NO_QUERY` 让普通聊天链回答“没有查询接口/权限”
- 若当前轮显式给出新的组织编码、组织名称或日期，应优先采用当前轮显式事实
- 会话压缩摘要不能作为查询锚点；不要从自然语言 summary 中猜测组织编码、日期或父组织编码
- `open_clarification` 只提供最近一次仍待续接的结构化澄清事实，可用于理解用户是否在确认候选，但不能替代执行结果事实

## 多步只读编排提示

- 当用户先要“找到组织”，再要“看详情/下级组织/审计”时，优先先把目标组织定位清楚；如果可以先 search 唯一命中，再继续执行后续只读查询，则应优先生成线性多步 `ReadPlan`
- 多步只读编排必须是线性的前序依赖，不能并发、不能回环
- 多步参数引用只能使用前序 step 的受控字段，例如 `@step-1.target_org_code` 或 `@step-1.payload.target_org_code`
- 若第一步不能稳定定位唯一组织，应停止执行并回到澄清；必要时只给出少量候选组织供用户确认，不要猜测下一步参数，也不要静默选择第一条结果
- 列表状态过滤不要输出 `inactive` 这类别名；若用户说“无效/停用”，统一落到 `disabled`
