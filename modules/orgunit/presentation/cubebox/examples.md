# OrgUnit Examples

## 示例 1：单步详情查询

用户问法：

`查一下 1001 在 2026-04-23 的组织详情`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.details",
  "confidence": 0.96,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.details",
      "params": {
        "org_code": "1001",
        "as_of": "2026-04-23",
        "include_disabled": false
      },
      "result_focus": [
        "org_unit.name",
        "org_unit.parent_org_code",
        "org_unit.manager_name",
        "org_unit.full_name_path"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "组织基本信息",
    "上级组织",
    "负责人",
    "全路径"
  ]
}
```

## 示例 2：列表查询

用户问法：

`列出 2026-04-23 当天 1001 下面的直接子组织`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.list",
  "confidence": 0.93,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.list",
      "params": {
        "as_of": "2026-04-23",
        "parent_org_code": "1001",
        "include_disabled": false
      },
      "result_focus": [
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status",
        "org_units[].has_children"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "直接子组织列表",
    "状态",
    "是否还有下级"
  ]
}
```

## 示例 3：组织树默认值查询

用户问法：

`查询组织树`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.list",
  "confidence": 0.88,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.list",
      "params": {
        "as_of": "2026-04-23",
        "include_disabled": false
      },
      "result_focus": [
        "as_of",
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status",
        "org_units[].has_children"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "当前租户一级组织",
    "状态",
    "是否还有下级"
  ]
}
```

说明：

- 本例默认按当前自然日查询
- 本例未给 `parent_org_code`，表示先返回当前租户一级组织

## 示例 4：缺参追问

用户问法：

`查一下这个组织详情`

期望返回：

```json
{
  "intent": "orgunit.details",
  "confidence": 0.42,
  "missing_params": [
    "org_code",
    "as_of"
  ],
  "clarifying_question": "请告诉我要查询的组织编码，以及查询日期（例如 2026-04-23）。"
}
```

## 示例 5：多步只读编排

用户问法：

`先帮我找到名字里带华东的组织，再看它在 2026-04-23 的详情`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.search_then_details",
  "confidence": 0.84,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.search",
      "params": {
        "query": "华东",
        "as_of": "2026-04-23",
        "include_disabled": false
      },
      "result_focus": [
        "target_org_code",
        "target_name"
      ],
      "depends_on": []
    },
    {
      "id": "step-2",
      "api_key": "orgunit.details",
      "params": {
        "org_code": "@step-1.target_org_code",
        "as_of": "2026-04-23",
        "include_disabled": false
      },
      "result_focus": [
        "org_unit.name",
        "org_unit.parent_org_code",
        "org_unit.manager_name"
      ],
      "depends_on": [
        "step-1"
      ]
    }
  ],
  "explain_focus": [
    "先定位唯一命中的组织",
    "再回答组织详情"
  ]
}
```

说明：

- 本示例体现 `468 P0` 口径：若第一步可以唯一命中，应优先生成线性多步 `ReadPlan`
- 搜索和详情之间允许通过 `@step-1.target_org_code` 或 `@step-1.payload.target_org_code` 引用前序结果
- 若按名称搜索会命中多个组织，也应先回到澄清，并给出少量候选供用户确认，不要静默选第一条继续

## 示例 6：列表状态过滤使用 canonical 参数

用户问法：

`列出 2026-04-23 当天 1001 下面已停用的直接子组织`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.list",
  "confidence": 0.91,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.list",
      "params": {
        "as_of": "2026-04-23",
        "parent_org_code": "1001",
        "status": "disabled",
        "include_disabled": true
      },
      "result_focus": [
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "已停用的直接子组织"
  ]
}
```

说明：

- `status` 只使用 canonical 值 `disabled`
- 不要输出 `inactive`

## 示例 7：继承最近已确认组织查询子组织

planner 上下文：

```yaml
query_dialogue_context:
  recent_confirmed_entity:
    domain: orgunit
    intent: orgunit.details
    entity_key: "100000"
    as_of: "2026-04-25"
  recent_confirmed_entities:
    - domain: orgunit
      intent: orgunit.details
      entity_key: "100000"
      as_of: "2026-04-25"
  recent_candidates: []
```

用户问法：

`查该组织的下级组织`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.list",
  "confidence": 0.9,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.list",
      "params": {
        "as_of": "2026-04-25",
        "parent_org_code": "100000",
        "include_disabled": false
      },
      "result_focus": [
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status",
        "org_units[].has_children"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "直接子组织列表",
    "状态",
    "是否还有下级"
  ]
}
```

说明：

- “该组织”继承最近已确认查询实体中的 `entity_key=100000`
- 用户未显式给新日期，因此继承 `as_of=2026-04-25`
- 若当前轮显式给出另一个组织编码，应使用当前轮编码覆盖历史实体

## 示例 8：仍在查询域但缺少可继承实体时 fail-closed

planner 上下文：

```yaml
query_dialogue_context:
  recent_confirmed_entity: null
  recent_confirmed_entities: []
  recent_candidates: []
```

用户问法：

`查该组织的下级组织`

期望返回：

```json
{
  "intent": "orgunit.list",
  "confidence": 0.44,
  "missing_params": [
    "parent_org_code"
  ],
  "clarifying_question": "请提供要查询下级组织的上级组织编码，或先告诉我要定位的组织名称。"
}
```

说明：

- 该问题仍属于组织架构查询域，不应输出 `NO_QUERY`
- 不得回答“没有查询接口”“没有工具权限”
- 不得从会话压缩摘要里猜测组织编码

## 示例 9：只有名称时先 search 再 list

用户问法：

`查询飞虫公司的下级组织，只有名称`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.search_then_list",
  "confidence": 0.88,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.search",
      "params": {
        "query": "飞虫公司",
        "as_of": "2026-04-25",
        "include_disabled": false
      },
      "result_focus": [
        "target_org_code",
        "target_name"
      ],
      "depends_on": []
    },
    {
      "id": "step-2",
      "api_key": "orgunit.list",
      "params": {
        "as_of": "2026-04-25",
        "parent_org_code": "@step-1.target_org_code",
        "include_disabled": false
      },
      "result_focus": [
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status",
        "org_units[].has_children"
      ],
      "depends_on": [
        "step-1"
      ]
    }
  ],
  "explain_focus": [
    "先定位组织编码",
    "再回答直接下级组织"
  ]
}
```

说明：

- 用户虽然只给了名称，但目标查询需要稳定编码时，不必先追问编码
- 若第一步搜索不是唯一命中，应回到澄清，并给出少量候选供用户确认

## 示例 10：消费 recent_candidates 中的“第一个”

planner 上下文：

```yaml
query_dialogue_context:
  recent_confirmed_entity: null
  recent_confirmed_entities: []
  recent_candidates:
    - domain: orgunit
      entity_key: "200000"
      name: "飞虫公司"
      as_of: "2026-04-25"
    - domain: orgunit
      entity_key: "300000"
      name: "鲜花公司"
      as_of: "2026-04-25"
```

用户问法：

`第一个`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.details",
  "confidence": 0.79,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.details",
      "params": {
        "org_code": "200000",
        "as_of": "2026-04-25",
        "include_disabled": false
      },
      "result_focus": [
        "org_unit.name",
        "org_unit.status"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "回答第一个候选组织的详情"
  ]
}
```

说明：

- 当上一轮已经给用户展示了候选列表时，“第一个”“第二个”“那个公司”应优先消费 `recent_candidates`
- 只有在候选为空或用户指代仍然歧义时，才回到澄清
