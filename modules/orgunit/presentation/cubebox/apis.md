# OrgUnit CubeBox API Tools

本文件只声明模型侧可引用的 API tool overlay 语义。运行时执行事实源来自 484/485 派生出的 HTTP API catalog 与 490 overlay builder；planner 必须使用 `api_tools` 中的 `method` + `path`，不能声明新业务工具。

```yaml
api_tools:
  - operation_id: orgunit.details
    query_intent: orgunit.details
    method: GET
    path: /org/api/org-units/details
    required_params: [org_code, as_of]
    optional_params: [include_disabled]
    observation: single_org_unit
  - operation_id: orgunit.list
    query_intent: orgunit.list
    method: GET
    path: /org/api/org-units
    required_params: [as_of]
    optional_params: [include_disabled, parent_org_code, all_org_units, keyword, status, is_business_unit, page, page_size]
    observation: org_unit_list
  - operation_id: orgunit.search
    query_intent: orgunit.search
    method: GET
    path: /org/api/org-units/search
    required_params: [query, as_of]
    optional_params: [include_disabled]
    observation: org_unit_search_result
  - operation_id: orgunit.audit
    query_intent: orgunit.audit
    method: GET
    path: /org/api/org-units/audit
    required_params: [org_code]
    optional_params: [limit]
    observation: org_unit_audit_events
```

## 使用规则

- `API_CALLS.calls[].method/path` 必须来自当前 planner 输入中的 `api_tools`。
- `params` 只能包含对应 tool 的 `request_schema.required` 和 `request_schema.optional` 参数。
- 缺少 required 参数时输出 `CLARIFY`。
- `orgunit.list` 的 `page` 是用户可见 1 基页码；`page_size` 是每页条数。
- `page` / `page_size` 缺省时按 `page=1,page_size=100` 处理，不要追问。
- `orgunit.search` 的 `query` 保留用户原始搜索词，不要擅自扩写。
- 多步查询必须线性排列，后一步 `depends_on` 只引用前一步 ID。
- 不要生成隐藏字段引用、SQL、store/helper 调用或页面状态依赖。
