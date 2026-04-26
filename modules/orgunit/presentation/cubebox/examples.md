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

说明：

- “这个组织” 只能依赖 `query_dialogue_context` 中最近已确认的结构化实体事实；当前范围不允许从页面状态隐式补 `org_code`

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
    entity_key: "100000"
    as_of: "2026-04-25"
  recent_confirmed_entities:
    - domain: orgunit
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
  recent_candidate_groups: []
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

## 示例 10：消费 recent_candidate_groups 中的“第一个”

planner 上下文：

```yaml
query_dialogue_context:
  recent_confirmed_entity: null
  recent_confirmed_entities: []
  recent_candidate_groups:
    - group_id: candgrp_aaa
      candidate_source: execution_error
      candidate_count: 2
      cannot_silent_select: true
      candidates:
        - domain: orgunit
          entity_key: "200000"
          name: "飞虫公司"
          as_of: "2026-04-25"
        - domain: orgunit
          entity_key: "300000"
          name: "鲜花公司"
          as_of: "2026-04-25"
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

- 当上一轮已经给用户展示了候选列表时，“第一个”“第二个”“那个公司”应优先消费 `recent_candidate_groups`
- 只有在候选为空或用户指代仍然歧义时，才回到澄清

## 示例 11：跨组引用“最开始那个 / 不是这个，另一个”

planner 上下文：

```yaml
query_dialogue_context:
  recent_confirmed_entity: null
  recent_confirmed_entities: []
  recent_candidate_groups:
    - group_id: candgrp_old
      candidate_source: execution_error
      candidate_count: 2
      cannot_silent_select: true
      candidates:
        - domain: orgunit
          entity_key: "100100"
          name: "华东销售中心"
          as_of: "2026-04-25"
        - domain: orgunit
          entity_key: "100200"
          name: "华东运营中心"
          as_of: "2026-04-25"
    - group_id: candgrp_new
      candidate_source: execution_error
      candidate_count: 2
      cannot_silent_select: true
      candidates:
        - domain: orgunit
          entity_key: "200100"
          name: "飞虫公司"
          as_of: "2026-04-25"
        - domain: orgunit
          entity_key: "200200"
          name: "鲜花公司"
          as_of: "2026-04-25"
  recent_candidates:
    - domain: orgunit
      entity_key: "200100"
      name: "飞虫公司"
      as_of: "2026-04-25"
    - domain: orgunit
      entity_key: "200200"
      name: "鲜花公司"
      as_of: "2026-04-25"
```

用户问法 1：

`最开始那个`

期望行为：

- 应优先回看较早的 `recent_candidate_groups[0]`
- 若表达足以唯一定位，则可继续查询 `org_code=100100`

用户问法 2：

`不是这个，另一个`

期望行为：

- 应在当前相关候选组内切换到另一项
- 若“这个”指向仍不稳定，应回到澄清，而不是静默猜测跨组对象

## 示例 12：同一 turn 内根据 `working_results` 继续展开

用户问法：

`展开完整组织树`

第一轮 planner 期望输出：

```json
{
  "outcome": "READ_PLAN",
  "plan": {
    "intent": "orgunit.list",
    "confidence": 0.88,
    "missing_params": [],
    "steps": [
      {
        "id": "step-1",
        "api_key": "orgunit.list",
        "params": {
          "as_of": "2026-04-25",
          "include_disabled": false
        },
        "result_focus": [
          "org_units[].org_code",
          "org_units[].name",
          "org_units[].has_children"
        ],
        "depends_on": []
      }
    ],
    "explain_focus": [
      "先查询一级组织",
      "观察哪些组织仍有下级"
    ]
  }
}
```

执行后 planner 看到的 `working_results` 示例：

```json
{
  "working_results": {
    "round_index": 1,
    "latest_observation": {
      "api_key": "orgunit.list",
      "items": [
        {
          "org_code": "100000",
          "name": "总部",
          "has_children": true
        },
        {
          "org_code": "200000",
          "name": "共享服务中心",
          "has_children": false
        }
      ],
      "item_count": 2
    },
    "executed_fingerprints": [
      "orgunit.list|as_of=\"2026-04-25\"|include_disabled=false"
    ]
  }
}
```

下一轮 planner 期望输出：

```json
{
  "outcome": "READ_PLAN",
  "plan": {
    "intent": "orgunit.list",
    "confidence": 0.86,
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
          "org_units[].has_children"
        ],
        "depends_on": []
      }
    ],
    "explain_focus": [
      "继续查询 has_children=true 的直接下级"
    ]
  }
}
```

若后续 `working_results.latest_observation.items` 已没有 `has_children=true` 的项目，planner 期望输出：

```json
{
  "outcome": "DONE"
}
```

说明：

- `working_results` 不是新的 API 或 DSL，只是当前 turn 已执行只读 API 的 observation。
- planner 可以阅读 `latest_observation.items[].has_children` 和 `org_code` 决定下一轮 `READ_PLAN`。
- 不要输出裸文本 `DONE`；完成时必须输出 JSON envelope。
- 不要重复执行 `executed_fingerprints` 中已经出现的查询。
- 不要生成 `remaining_parent_org_codes`、业务 winner、聚合事实或其他 orgunit 专用状态字段。
