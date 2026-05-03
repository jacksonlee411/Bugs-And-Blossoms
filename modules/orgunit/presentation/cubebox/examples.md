# OrgUnit API-First Examples

## 示例 1：组织详情

用户问法：

`查 100000 今天的组织详情`

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units/details",
      "params": {
        "org_code": "100000",
        "as_of": "2026-04-25",
        "include_disabled": false
      },
      "result_focus": ["org_unit.org_code", "org_unit.name", "org_unit.status"],
      "depends_on": []
    }
  ]
}
```

## 示例 2：一级组织列表

用户问法：

`列出今天的一级组织`

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2026-04-25",
        "include_disabled": false,
        "page": 1,
        "page_size": 100
      },
      "result_focus": ["org_units[].org_code", "org_units[].name", "org_units[].has_children"],
      "depends_on": []
    }
  ]
}
```

## 示例 3：全部组织分页默认值

用户问法：

`今天的全部组织，实现方式你自己编排`

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2026-04-25",
        "include_disabled": false,
        "all_org_units": true,
        "page": 1,
        "page_size": 100
      },
      "result_focus": ["page", "page_size", "total", "org_units[].org_code", "org_units[].name"],
      "depends_on": []
    }
  ]
}
```

## 示例 4：关键词组织列表

用户问法：

`列出全部包含成本关键字的组织`

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2026-04-25",
        "keyword": "成本",
        "include_disabled": false,
        "page": 1,
        "page_size": 100
      },
      "result_focus": ["org_units[].org_code", "org_units[].name", "org_units[].status"],
      "depends_on": []
    }
  ]
}
```

说明：若用户没有给上级组织范围，不要填写 `parent_org_code`。

## 示例 5：搜索定位

用户问法：

`搜索名称包含华东的组织`

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units/search",
      "params": {
        "query": "华东",
        "as_of": "2026-04-25",
        "include_disabled": false
      },
      "result_focus": ["target_org_code", "target_name", "path_org_codes"],
      "depends_on": []
    }
  ]
}
```

## 示例 6：先搜索再查详情

用户问法：

`查华东销售中心今天的详情`

期望首轮可以先搜索：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units/search",
      "params": {
        "query": "华东销售中心",
        "as_of": "2026-04-25",
        "include_disabled": false
      },
      "result_focus": ["target_org_code", "target_name"],
      "depends_on": []
    }
  ]
}
```

如果 `working_results.latest_observation` 显示唯一命中 `target_org_code=100100`，下一轮输出：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-2",
      "method": "GET",
      "path": "/org/api/org-units/details",
      "params": {
        "org_code": "100100",
        "as_of": "2026-04-25",
        "include_disabled": false
      },
      "result_focus": ["org_unit.org_code", "org_unit.name", "org_unit.full_name_path"],
      "depends_on": ["step-1"]
    }
  ]
}
```

若命中不唯一，输出 `CLARIFY`，不要静默选择。

## 示例 7：继续展开组织树

用户问法：

`展开完整组织树`

首轮查询一级组织：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2026-04-25",
        "include_disabled": false,
        "page": 1,
        "page_size": 100
      },
      "result_focus": ["org_units[].org_code", "org_units[].name", "org_units[].has_children"],
      "depends_on": []
    }
  ]
}
```

如果 `working_results.latest_observation.items` 中 `100000` 的 `has_children=true`，下一轮继续：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-2",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2026-04-25",
        "parent_org_code": "100000",
        "include_disabled": false,
        "page": 1,
        "page_size": 100
      },
      "result_focus": ["org_units[].org_code", "org_units[].name", "org_units[].has_children"],
      "depends_on": ["step-1"]
    }
  ]
}
```

若已经没有需要继续展开的节点，输出：

```json
{
  "outcome": "DONE"
}
```

## 示例 8：日期澄清后的短答

相关上下文：

```yaml
query_evidence_window:
  current_user_input: 1日
  recent_turns:
    - user_prompt: 查出顶级点的全部各级下级组织，时间节点是2025年1月
      assistant_reply: 请提供完整查询日期，例如 2025-01-01。
  open_clarification:
    reply_candidate: true
    missing_params: [as_of]
    raw_user_reply: 1日
```

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2025-01-01",
        "include_disabled": false,
        "page": 1,
        "page_size": 100
      },
      "result_focus": ["org_units[].org_code", "org_units[].name", "org_units[].has_children"],
      "depends_on": []
    }
  ]
}
```

## 示例 9：纠正上一轮关键词过滤

相关历史事实：

```yaml
recent_turns:
  - user_prompt: 列出全部包含成本关键字的组织
    assistant_reply: 名称包含“成本”关键字的组织共有 3 个。
current_user_input: 不只是包含成本关键字的组织，而是全部的组织
```

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2026-04-25",
        "include_disabled": false,
        "all_org_units": true,
        "page": 1,
        "page_size": 100
      },
      "result_focus": ["page", "page_size", "total", "org_units[].org_code", "org_units[].name"],
      "depends_on": []
    }
  ]
}
```

说明：不得继承历史 `keyword=成本` 或历史结果集。

## 示例 10：审计记录

用户问法：

`看一下 100000 最近被谁改过`

期望 `API_CALLS`：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units/audit",
      "params": {
        "org_code": "100000",
        "limit": 20
      },
      "result_focus": ["events[].event_type", "events[].actor", "events[].created_at"],
      "depends_on": []
    }
  ]
}
```
