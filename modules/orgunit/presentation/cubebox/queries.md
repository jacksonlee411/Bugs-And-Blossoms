# OrgUnit Queries

## 查询意图总表

```yaml
intents:
  - key: orgunit.details
    description: 查询单个组织在指定时点的详情
    required_params: [org_code, as_of]
    optional_params: [include_disabled]
  - key: orgunit.list
    description: 查询组织列表或某个上级组织下的直接子组织列表
    required_params: [as_of]
    optional_params: [include_disabled, parent_org_code, keyword, status, page, size]
  - key: orgunit.search
    description: 根据关键词搜索组织并返回命中的组织与路径
    required_params: [query, as_of]
    optional_params: [include_disabled]
  - key: orgunit.audit
    description: 查询某个组织的审计事件摘要
    required_params: [org_code]
    optional_params: [limit]
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

参数规则：

- 必填：`as_of`
- 可选：`include_disabled`、`parent_org_code`、`keyword`、`status`、`page`、`size`
- 若用户只说“查询组织树”“列出组织”“看组织树”，未给 `as_of` 时默认按当前自然日
- 若用户未说明范围，默认先查当前租户一级组织，不要求首轮必须提供 `parent_org_code`
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
- 搜索结果主要用于定位目标组织，必要时可作为下一步 `orgunit.details` 的前置步骤

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

## 多步只读编排提示

- 当用户先要“找到组织”，再要“看详情”时，可先 `orgunit.search`，后 `orgunit.details`
- 多步只读编排必须是线性的前序依赖，不能并发、不能回环
- 若第一步不能稳定定位唯一组织，应停止执行并回到澄清，而不是猜测下一步参数
