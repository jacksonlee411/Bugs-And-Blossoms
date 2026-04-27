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

## 示例 3A：全部业务单元列表

用户问法：

`列出全部业务单元`

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
        "as_of": "2026-04-27",
        "include_disabled": false,
        "is_business_unit": true
      },
      "result_focus": [
        "as_of",
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status",
        "org_units[].is_business_unit",
        "org_units[].has_children"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "当前租户全部启用业务单元",
    "组织编码",
    "是否还有下级"
  ]
}
```

说明：

- 本例默认按当前自然日查询
- 本例未给 `parent_org_code`，`is_business_unit=true` 表示在当前租户全部有效组织中按业务单元标记过滤，不是只查一级组织

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

- “这个组织” 只能由模型结合 `query_evidence_window` 与当前输入判断；当前范围不允许从页面状态隐式补 `org_code`

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

## 示例 6A：全租户关键词组织列表

用户问法：

`列出全部包含成本关键字的组织`

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
        "as_of": "2026-04-27",
        "keyword": "成本",
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
    "名称或编码包含成本关键字的组织"
  ]
}
```

说明：

- “包含 X 关键字的组织”是组织列表关键词检索，使用 `orgunit.list.keyword=X`
- 没有上级组织范围时不要填写 `parent_org_code`，表示在当前租户全部有效组织中检索

## 示例 6B：跨 Turn 结果集补路径列

上一轮已回答：

- `200001 财务部`
- `200002 财务一组`
- `200003 财务三组`
- `200004 财务四组`

当前轮用户问法：

`增加列出他们的组织路径`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.details",
  "confidence": 0.82,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.details",
      "params": {
        "org_code": "200001",
        "as_of": "2026-04-27",
        "include_disabled": false
      },
      "result_focus": [
        "org_unit.org_code",
        "org_unit.name",
        "org_unit.full_name_path"
      ],
      "depends_on": []
    },
    {
      "id": "step-2",
      "api_key": "orgunit.details",
      "params": {
        "org_code": "200002",
        "as_of": "2026-04-27",
        "include_disabled": false
      },
      "result_focus": [
        "org_unit.org_code",
        "org_unit.name",
        "org_unit.full_name_path"
      ],
      "depends_on": [
        "step-1"
      ]
    },
    {
      "id": "step-3",
      "api_key": "orgunit.details",
      "params": {
        "org_code": "200003",
        "as_of": "2026-04-27",
        "include_disabled": false
      },
      "result_focus": [
        "org_unit.org_code",
        "org_unit.name",
        "org_unit.full_name_path"
      ],
      "depends_on": [
        "step-2"
      ]
    },
    {
      "id": "step-4",
      "api_key": "orgunit.details",
      "params": {
        "org_code": "200004",
        "as_of": "2026-04-27",
        "include_disabled": false
      },
      "result_focus": [
        "org_unit.org_code",
        "org_unit.name",
        "org_unit.full_name_path"
      ],
      "depends_on": [
        "step-3"
      ]
    }
  ],
  "explain_focus": [
    "组织编码",
    "组织名称",
    "组织路径长名称"
  ]
}
```

说明：

- 本例不是新的搜索或新的歧义候选选择，而是对上一轮明确 `result_list` 的补字段续接
- `org_unit.full_name_path` 属于 `orgunit.details` 返回面，不属于 `orgunit.list` 默认返回面
- 若上一轮结果集过大，不应静默展开全部详情查询，而应先澄清范围

## 示例 7：根据 evidence window 判断“该组织”的下级组织

planner 上下文：

```yaml
query_evidence_window:
  current_user_input: 查该组织的下级组织
  recent_turns:
    - user_prompt: 查 100000 在 2026-04-25 的详情
      assistant_reply: 组织 100000 是“飞虫与鲜花”，当前为启用状态。
  observations:
    - source: query_event
      kind: entity_fact
      result_summary:
        item:
          domain: orgunit
          entity_key: "100000"
          as_of: "2026-04-25"
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

- “该组织”由模型结合当前输入、最近问答和 `entity_fact` 判断为 `entity_key=100000`
- 用户未显式给新日期，模型可结合 evidence 中的 `as_of=2026-04-25` 输出显式参数
- 若当前轮显式给出另一个组织编码，应使用当前轮编码覆盖历史 evidence

## 示例 8：仍在查询域但缺少可继承实体时 fail-closed

planner 上下文：

```yaml
query_evidence_window:
  current_user_input: 查该组织的下级组织
  recent_turns: []
  observations: []
  open_clarification: null
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

## 示例 10：根据 presented_options 判断“第一个”

planner 上下文：

```yaml
query_evidence_window:
  current_user_input: 第一个
  recent_turns:
    - user_prompt: 查飞虫
      assistant_reply: 找到了多个候选项，请确认要继续查询哪一个：200000「飞虫公司」、300000「鲜花公司」。
  observations:
    - source: query_event
      kind: presented_options
      result_summary:
        group_id: candgrp_aaa
        option_source: execution_error
        item_count: 2
        requires_explicit_user_choice: true
        items:
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

- 当上一轮已经给用户展示了候选列表时，“第一个”“第二个”“那个公司”应由模型结合 `presented_options` 和 `recent_turns` 判断
- 只有在候选为空或用户指代仍然歧义时，才回到澄清

## 示例 11：跨组引用“最开始那个 / 不是这个，另一个”

planner 上下文：

```yaml
query_evidence_window:
  current_user_input: 最开始那个
  recent_turns:
    - user_prompt: 搜索华东
      assistant_reply: 找到了多个候选项：100100「华东销售中心」、100200「华东运营中心」。
    - user_prompt: 搜索公司
      assistant_reply: 找到了多个候选项：200100「飞虫公司」、200200「鲜花公司」。
  observations:
    - source: query_event
      kind: presented_options
      result_summary:
        group_id: candgrp_old
        item_count: 2
        items:
          - domain: orgunit
            entity_key: "100100"
            name: "华东销售中心"
            as_of: "2026-04-25"
          - domain: orgunit
            entity_key: "100200"
            name: "华东运营中心"
            as_of: "2026-04-25"
    - source: query_event
      kind: presented_options
      result_summary:
        group_id: candgrp_new
        item_count: 2
        items:
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

- 应由模型结合 `recent_turns` 和较早的 `presented_options` 判断
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

## 示例 13：日期澄清后的短答续接

planner 上下文：

```yaml
query_evidence_window:
  current_user_input: 1日
  recent_turns:
    - user_prompt: 查出顶级点的全部各级下级组织，时间节点是2025年1月
      assistant_reply: 请提供完整查询日期，例如 2025-01-01。
  observations: []
  open_clarification:
    reply_candidate: true
    source_turn_id: turn_prev
    intent: orgunit.list
    missing_params:
      - as_of
    clarifying_question: 请提供完整查询日期，例如 2025-01-01。
    raw_user_reply: 1日
```

用户问法：

`1日`

期望 `ReadPlan`：

```json
{
  "intent": "orgunit.list",
  "confidence": 0.82,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.list",
      "params": {
        "as_of": "2025-01-01",
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
    "顶级组织下的各级下级组织"
  ]
}
```

说明：

- `open_clarification.reply_candidate=true` 表示当前轮很可能在回答上一轮澄清。
- 应优先把 `1日` 解释成对上一轮日期澄清的补充，而不是重新追问“你想查什么”。
- 最终执行参数必须是完整 `YYYY-MM-DD`；不能输出 `as_of_day` 这类临时槽位。

## 示例 14：候选澄清后的“全部”

planner 上下文：

```yaml
query_evidence_window:
  current_user_input: 全部
  recent_turns:
    - user_prompt: 列出全部财务组织的详情
      assistant_reply: 找到了多个候选项，请确认要继续查询哪一个。
  observations:
    - source: query_event
      kind: presented_options
      result_summary:
        group_id: candgrp_finance
        option_source: execution_error
        item_count: 3
        requires_explicit_user_choice: true
        items:
          - domain: orgunit
            entity_key: "200001"
            name: 财务部
            as_of: "2026-04-25"
          - domain: orgunit
            entity_key: "200002"
            name: 财务一组
            as_of: "2026-04-25"
          - domain: orgunit
            entity_key: "200004"
            name: 财务四组
            as_of: "2026-04-25"
  open_clarification:
    reply_candidate: true
    source_turn_id: turn_prev
    option_group_id: candgrp_finance
    option_source: execution_error
    option_count: 3
    requires_explicit_user_choice: true
    options:
      - domain: orgunit
        entity_key: "200001"
        name: 财务部
        as_of: "2026-04-25"
      - domain: orgunit
        entity_key: "200002"
        name: 财务一组
        as_of: "2026-04-25"
      - domain: orgunit
        entity_key: "200004"
        name: 财务四组
        as_of: "2026-04-25"
    raw_user_reply: 全部
```

用户问法：

`全部`

期望行为：

- 先把 `全部` 理解为对上一轮候选澄清的集合型答复。
- 若当前预算和执行能力允许，可继续围绕这 3 个候选组织生成合法小计划。
- 若当前预算不足或集合语义仍不稳定，应继续澄清；不要跳回入口级重判或输出 `NO_QUERY`。

## 示例 15：直接问“本月9日”时使用当前自然日年月

planner 当前自然日：

`2026-04-25`

用户问法：

`查询全部财务组织本月9日的详情`

期望行为：

- `本月9日` 应解析为 `2026-04-09`，不要再次要求用户提供完整日期。
- `全部财务组织` 优先用 `orgunit.list` 的 `keyword=财务` 查询候选列表；若后续要看详情，再基于 `working_results` 中的候选组织编码继续生成线性小计划。

期望首轮 `ReadPlan`：

```json
{
  "intent": "orgunit.list",
  "confidence": 0.86,
  "missing_params": [],
  "steps": [
    {
      "id": "step-1",
      "api_key": "orgunit.list",
      "params": {
        "as_of": "2026-04-09",
        "keyword": "财务",
        "include_disabled": false
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
    "名称包含财务的组织",
    "2026-04-09 的组织详情"
  ]
}
```

若首轮结果只有少量候选，planner 后续可在同一 turn 内按 `working_results.latest_observation.items` 的出现顺序继续查详情；若候选过多或预算不足，应继续澄清缩小范围。

## 示例 16：全部组织分页默认值

用户问法：

`今天的全部组织，实现方式你自己编排`

期望行为：

- 不要要求用户确认 `page` 或 `size`。
- 默认按当前自然日、第一页、每页 100 条执行受控分页查询。
- 若用户没有明确要求包含停用组织，默认 `include_disabled=false`；若用户强调“全部包含停用”，则使用 `include_disabled=true`。
- 当前 `orgunit.list` 无 `parent_org_code` 且无 `keyword`、无 `all_org_units=true` 时仍是一级组织清单；用户明确要求全部组织或全租户组织清单时，必须设置 `all_org_units=true`。

期望首轮 `ReadPlan`：

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
        "as_of": "2026-04-27",
        "include_disabled": false,
        "all_org_units": true,
        "page": 1,
        "size": 100
      },
      "result_focus": [
        "page",
        "size",
        "total",
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status",
        "org_units[].has_children"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "分页组织清单",
    "本页范围",
    "是否还有下级"
  ]
}
```

## 示例 17：分页短答只给一个数字

相关历史事实：

```yaml
recent_turns:
  - user_prompt: "B) 直接按分页给全租户组织清单"
    assistant_reply: "已选择分页清单。未指定分页时默认第一页、每页 100 条。"
current_user_input: "100"
```

用户问法：

`100`

期望行为：

- 将 `100` 理解为 `size=100`，默认 `page=1`。
- 不要再要求用户补 `page`。
- 输出完整可执行 `ReadPlan`。

## 示例 18：纠正上一轮关键词过滤为全部组织

相关历史事实：

```yaml
recent_turns:
  - user_prompt: "列出全部包含成本关键字的组织"
    assistant_reply: "名称包含“成本”关键字的组织共有 3 个：成本A组、成本B组、成本C组。"
observations:
  - kind: result_list
    result_summary:
      items:
        - {domain: orgunit, entity_key: "200005", name: "成本A组", as_of: "2026-04-27"}
        - {domain: orgunit, entity_key: "200006", name: "成本B组", as_of: "2026-04-27"}
        - {domain: orgunit, entity_key: "200007", name: "成本C组", as_of: "2026-04-27"}
current_user_input: "不只是包含成本关键字的组织，而是全部的组织"
```

期望行为：

- 当前轮是在纠正并扩大查询范围，不是让系统继续查“成本”结果集。
- 不得继承历史 `keyword=成本`。
- 不得继承历史单个候选 `200007` 或把上一轮 `result_list` 当作当前 target set。
- 默认按当前自然日、第一页、每页 100 条执行组织清单查询。

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
        "as_of": "2026-04-27",
        "include_disabled": false,
        "all_org_units": true,
        "page": 1,
        "size": 100
      },
      "result_focus": [
        "page",
        "size",
        "total",
        "org_units[].org_code",
        "org_units[].name",
        "org_units[].status",
        "org_units[].has_children"
      ],
      "depends_on": []
    }
  ],
  "explain_focus": [
    "全部组织清单",
    "已取消历史关键词过滤"
  ]
}
```
