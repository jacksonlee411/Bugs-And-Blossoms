# OrgUnit Queries

## 查询意图总表

```yaml
intents:
  - key: orgunit.details
    description: 查询单个组织在指定日期的详情
    required_params: [org_code, as_of]
    optional_params: [include_disabled]
  - key: orgunit.list
    description: 查询组织列表、全租户组织列表、关键词组织列表或直接下级组织列表
    required_params: [as_of]
    optional_params: [include_disabled, parent_org_code, all_org_units, keyword, status, is_business_unit, page, page_size]
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
runtime_hints:
  unsupported_prompt_terms:
    - 成本组织
    - 成本中心
    - 组织类型
    - org type
    - org_type
    - cost center
    - cost org
  scope_params:
    expand_all: [all_org_units]
    narrowing: [keyword, parent_org_code, org_code, entity_key, target_org_code]
```

## 意图细则

### `orgunit.details`

适用于“查看 1001 组织详情”“查华东销售中心在 2026-04-01 的信息”“这个组织当前负责人是谁”。

- 必填：`org_code`、`as_of`
- 可选：`include_disabled`
- 只给名称、不给编码时，先使用 `orgunit.search` 定位或输出 `CLARIFY`。
- 缺 `org_code` 时追问组织编码或组织名称。
- 缺 `as_of` 且无法从当前输入或上下文确定时追问日期。

### `orgunit.list`

适用于“列出当前所有一级组织”“看 1001 下面的子组织”“列出全部业务单元”“列出全部包含成本关键字的组织”。

- 必填：`as_of`
- 可选：`include_disabled`、`parent_org_code`、`all_org_units`、`keyword`、`status`、`is_business_unit`、`page`、`page_size`
- `status` 只使用 `active`、`disabled`、`all`。
- 未说明范围时默认一级组织。
- 明确要求“全部组织”“所有组织”“全租户组织清单”时设置 `all_org_units=true`，不要填写 `parent_org_code`。
- “名称包含 X”“包含 X 关键字的组织”使用 `keyword=X`；若没有上级范围，不要填写 `parent_org_code`。
- “业务单元”“全部业务单元”使用 `is_business_unit=true`。
- `page` / `page_size` 是执行控制参数，不是业务必填参数。
- 未指定分页时默认 `page=1,page_size=100`。
- 用户只提供一个正整数作为分页短答时，优先理解为 `page_size`，默认 `page=1`。

### `orgunit.search`

适用于“搜索包含销售的组织”“帮我找一下华东”“查名字里有共享服务的组织”。

- 必填：`query`、`as_of`
- 可选：`include_disabled`
- `query` 保留用户原始搜索词。
- 搜索主要用于定位目标组织；结果不唯一时应澄清。
- 如果搜索后唯一命中且用户已要求详情、下级或审计，可以在同一线性 API plan 后续继续调用对应 API。

### `orgunit.audit`

适用于“看一下 1001 的最近变更”“这个组织最近被谁改过”“查这个组织的审计记录”。

- 必填：`org_code`
- 可选：`limit`
- 缺 `org_code` 时追问组织编码或先搜索定位。
- 未给 `limit` 时使用系统默认值。

## 自然语言映射提示

- “详情”“信息”“负责人”“父组织”“全路径”通常映射到 `orgunit.details`。
- “列表”“下面有哪些”“子组织”“分页”通常映射到 `orgunit.list`。
- “搜索”“找一下”“名称里有”通常映射到 `orgunit.search`。
- “审计”“变更记录”“谁改过”“最近变更”通常映射到 `orgunit.audit`。
- “该组织”“这个组织”“那个组织”“第一个”“全部”应读取 `query_evidence_window`，不能依赖隐藏页面状态。
- `observations.kind=result_list` 表示上一轮已经返回一组明确结果；若当前轮要求补字段，规模可控时可线性调用 `orgunit.details`，否则输出 `CLARIFY`。
- 当前轮明确纠正或扩大上一轮范围时，不得继承历史 `keyword`、`parent_org_code`、`entity_key` 或 `result_list`。

## 多步只读编排提示

- 每轮只规划当前最小必要 API 调用。
- 后续调用必须继续使用 `API_CALLS` 的 `method/path/params/depends_on`。
- `depends_on` 只表达同一 `API_CALLS` envelope 内的线性步骤；新一轮第一个 call 必须使用 `depends_on: []`，不要引用上一轮的 `step-1`。
- 已有 `working_results` 足够回答时输出 `DONE`。
- 不要重复执行 `working_results.executed_fingerprints` 中已有的查询。
