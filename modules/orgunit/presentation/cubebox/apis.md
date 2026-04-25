# OrgUnit APIs

## 说明

本文件是面向模型的只读 API 说明，用于帮助生成合法 `ReadPlan`。

本文件不是运行时执行事实源。运行时唯一执行事实源以后续代码中的 `api_key -> executor` 注册表为准。

planner 输入可能包含 `query_dialogue_context`。该上下文只用于补齐用户代词型追问中的稳定参数，不是新的 API，也不是授权来源；在 orgunit 域中 `entity_key` 表示组织编码，当前轮用户显式参数始终优先。

禁止项：

- 禁止直查数据库
- 禁止调用 SQL、store、projection 或内部函数
- 禁止调用未列出的接口
- 禁止声明新的 `api_key`
- 禁止把会话压缩摘要当作查询 API 结果或查询锚点
- 禁止在组织架构查询域内输出“没有查询接口/工具权限”类能力描述；缺参时必须澄清

## API 目录

```yaml
apis:
  - api_key: orgunit.details
    required_params: [org_code, as_of]
    optional_params: [include_disabled]
  - api_key: orgunit.list
    required_params: [as_of]
    optional_params: [include_disabled, parent_org_code, keyword, status, page, size]
  - api_key: orgunit.search
    required_params: [query, as_of]
    optional_params: [include_disabled]
  - api_key: orgunit.audit
    required_params: [org_code]
    optional_params: [limit]
```

### `orgunit.details`

- 用途：查询单个组织在指定时点的详情
- 必填参数：
  - `org_code`
  - `as_of`
- 可选参数：
  - `include_disabled`
- 关注字段：
  - `org_unit.org_code`
  - `org_unit.name`
  - `org_unit.status`
  - `org_unit.parent_org_code`
  - `org_unit.parent_name`
  - `org_unit.is_business_unit`
  - `org_unit.manager_pernr`
  - `org_unit.manager_name`
  - `org_unit.full_name_path`
  - `ext_fields`
- 权限前提：必须沿用当前用户、当前租户、当前 session 的现有只读权限边界

### `orgunit.list`

- 用途：查询组织列表，或某个上级组织下的直接子组织列表
- 必填参数：
  - `as_of`
- 可选参数：
  - `include_disabled`
  - `parent_org_code`
  - `keyword`
  - `status`
  - `page`
  - `size`
- 参数约束：
  - `status` 只接受 canonical 值 `active`、`disabled`、`all`
- 关注字段：
  - `as_of`
  - `include_disabled`
  - `org_units[].org_code`
  - `org_units[].name`
  - `org_units[].status`
  - `org_units[].is_business_unit`
  - `org_units[].has_children`
- 权限前提：必须沿用当前用户、当前租户、当前 session 的现有只读权限边界

### `orgunit.search`

- 用途：根据关键词搜索组织并返回命中目标与路径
- 必填参数：
  - `query`
  - `as_of`
- 可选参数：
  - `include_disabled`
- 关注字段：
  - `target_org_code`
  - `target_name`
  - `path_org_codes`
  - `tree_as_of`
- 多步编排提示：若 `orgunit.search` 已唯一命中，后续 step 可合法引用 `@step-1.target_org_code` 或 `@step-1.payload.target_org_code`
- owner 说明：`path_org_codes` 当前属于现有 `orgunit.search` 读契约字段；若未来删除，必须同步调整 `orgunit` 读契约与知识包，不能只在 CubeBox executor 单点删除
- 权限前提：必须沿用当前用户、当前租户、当前 session 的现有只读权限边界

### `orgunit.audit`

- 用途：查询某个组织的审计事件摘要
- 必填参数：
  - `org_code`
- 可选参数：
  - `limit`
- 关注字段：
  - `org_code`
  - `limit`
  - `has_more`
  - `events[].event_uuid`
  - `events[].event_type`
  - `events[].effective_date`
  - `events[].tx_time`
  - `events[].initiator_name`
  - `events[].request_id`
  - `events[].reason`
  - `events[].is_rescinded`
- owner 说明：`has_more` 当前属于现有 `orgunit.audit` 读契约字段；若未来删除，必须同步调整 `orgunit` 读契约与知识包，不能只在 CubeBox executor 单点删除
- 权限前提：必须沿用当前用户、当前租户、当前 session 的现有只读权限边界
