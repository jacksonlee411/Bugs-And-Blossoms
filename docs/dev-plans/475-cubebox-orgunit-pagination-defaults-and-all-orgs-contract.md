# DEV-PLAN-475：CubeBox OrgUnit 分页默认值与全部组织查询契约收敛方案

**状态**: 已完成（2026-04-27 22:10 CST）

## 0. 范围

本计划收敛 CubeBox 组织查询里“全部组织 / 分页清单”交互退化为参数表单的问题，并补齐“从关键词/候选结果纠正为全部组织”时的上下文覆盖规则。分页是执行控制参数，不是业务缺参；用户要求“全部组织”时，系统应默认选择可控分页策略，而不是要求用户确认 `page=1,size=100`。

关联计划：`DEV-PLAN-473`（模型主导查询链）、`DEV-PLAN-471`（同一 turn 迭代式只读编排）、`DEV-PLAN-464`（查询链轻量化收敛）。

## 1. 目标

1. [x] 冻结用户可见分页默认值：未指定分页时默认第一页、每页 100 条。
2. [x] 冻结短答数字续接口径：若上一轮只是在询问分页，用户只回复一个正整数，优先解释为 `size`，页码默认第一页；不得继续追问 `page`。
3. [x] 冻结 page 语义：用户可见 `page` 为 1 基页码；执行层内部 offset 可保持 0 基，但必须转换。
4. [x] 更新 orgunit CubeBox 知识包：`page` / `size` 不得作为 `missing_params` 追问；只有用户显式要求特定页码/每页条数时才覆盖默认值。
5. [x] 增加回归测试，阻断 planner prompt 或执行层回流成“请提供 page 和 size”式表单追问。
6. [x] 冻结当前轮纠错优先规则：用户明确说“不只是/不是/不限于 X，而是全部组织”时，不得继承历史 `keyword`、`parent_org_code`、单个候选或上一轮 `result_list`。

## 2. 非目标

1. 不新增本地关键词 NLU 或 slot repair engine。
2. 不新增绕过 `ExecutionRegistry` 的 orgunit 专用快捷分支。
3. 不在本计划中建设全层级递归 API；若“全部信息”需要逐项详情，继续由 `DEV-PLAN-471` 的 loop 和预算承接。
4. 不放松租户、权限、只读、日期合法性、page/size 上限等本地硬约束。

## 3. 决策

1. **分页不是业务缺参**：`page` / `size` 必须从澄清缺参集合中排除。用户没给时使用默认值。
2. **默认值由模型输出，执行层强兜底**：知识包和 planner prompt 要指导模型输出 `page=1,size=100`；执行层无论是否收到 `page` / `size`，都必须按 `page=1,size=100` 兜底，并兼容只给 `page` 或只给 `size`。
3. **用户页码 1 基，内部 offset 0 基**：执行层将 `page=1` 转成 `offset=0`，`page=2` 转成 `offset=size`；`page=0` 作为兼容旧 planner 输出处理为第一页。
4. **短答不反复索要参数**：当前 evidence 显示上一轮要求分页，而用户答 `100` 时，模型应输出完整计划 `page=1,size=100`，而不是再次追问。
5. **坏澄清不透传给用户**：若 planner 仍输出“只缺 `page` / `size`”的澄清，query flow 应进行一次内部修正重规划，不把该澄清展示给用户；重复偏航才按计划契约错误失败闭环。
6. **纠错输入覆盖历史过滤**：`query_evidence_window` 只提供历史事实。当前用户输入显式否定或扩大上一轮范围时，planner 必须重新规划，不继承旧关键词、父组织、候选 winner 或小结果集 target set。
7. **全租户组织清单显式化**：`orgunit.list` 的 `all_org_units=true` 表示当前租户全部组织分页清单；无 `parent_org_code` 且无 `all_org_units=true` 仍表示一级组织。

## 4. 实施步骤

1. [x] 更新 `modules/orgunit/presentation/cubebox/queries.md`、`apis.md`、`examples.md`。
2. [x] 更新 planner system prompt，明确禁止把 `page` / `size` 当业务必填缺参。
3. [x] 更新 orgunit list executor page offset 语义和默认 page/size 兜底。
4. [x] 增加单元测试覆盖 prompt 约束、默认分页与 1 基页码转换。
5. [x] 执行最小验证：`go test ./internal/server/...`、`go test ./modules/cubebox/...`、`make check doc`。
6. [x] 增加“成本关键词历史结果 -> 当前轮全部组织”的回归约束，避免历史过滤条件粘连。
