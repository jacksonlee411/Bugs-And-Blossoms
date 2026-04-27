# DEV-PLAN-476：CubeBox 全部组织纠错与重复计划收敛修复方案

**状态**: 已完成（2026-04-27 16:45 CST）

## 0. 背景

会话 `conv_bc7a2562b71344ebb552b989a96df232` 暴露了一个独立回归：用户明确说“不是只查包含成本关键字的组织，而是全部组织”后，系统不应继续继承历史 `keyword=成本`、`parent_org_code=200006` 或单实体 `200007`。`DEV-PLAN-475` 已收敛分页默认值和事实窗口剪枝，但真实复测又出现 `cubebox_query_loop_repeated_plan`，说明查询链还缺少对“纠错到全部组织”坏计划和重复计划的运行时闭环。

关联计划：`DEV-PLAN-473`（模型主导查询链）、`DEV-PLAN-475`（分页默认值与全部组织契约）、`DEV-PLAN-471`（同一 turn 迭代式只读规划）。

## 1. 问题

1. 当前用户输入已经覆盖历史范围，但 planner 仍可能从被否定的前半句抽取旧 `keyword`、旧 `parent_org_code` 或单个候选。
2. 即使 planner 首次输出正确列表计划，执行后若模型继续输出同一已执行计划而非 `DONE`，query loop 会命中 repeated-plan 终止，用户看到错误而不是查询结果。
3. 不能用本地关键词 NLU 或 orgunit 专用快捷链路替代模型规划；修复必须是对模型计划的边界纠偏和已执行重复计划的收敛。

## 2. 目标

1. [x] 对“纠错为全部组织”的当前输入，若 planner 输出 `keyword`、无依据的 `parent_org_code`、`org_code` / `entity_key` 或单实体详情计划，则内部追加 correction 并重规划一次，不执行坏计划。
2. [x] 对同一纠错场景，若已执行的 `orgunit.list` 计划被模型重复输出，且计划不含历史关键词/单实体窄化条件，则收敛为 `DONE` 并进入 narration，不把 repeated-plan 错误暴露给用户。
3. [x] 保留通用 repeated-plan fail-closed：非纠错场景、仍带窄化条件的计划、无法证明已执行的计划继续按原保护失败。
4. [x] 增加回归测试覆盖历史 `成本` 结果集纠错为全部组织。
5. [x] 为 CubeBox `orgunit.list` 增加显式 `all_org_units=true` 参数，区分“全租户组织分页清单”和“无 parent 的一级组织清单”。

## 3. 非目标

1. 不新增本地自然语言 slot engine。
2. 不新增绕过 `ExecutionRegistry` 的 orgunit 快捷执行分支。
3. 不改变通用查询 loop 的预算、租户、权限、日期和 API catalog 校验。
4. 不在本计划实现全层级无界递归；分页和 loop 仍受现有预算约束。
5. 不改变 Web UI 现有无 `parent_org_code` 时展示一级组织的行为；全租户分页只通过 CubeBox 显式参数进入。

## 4. 决策

1. **只在明确纠错到全部组织时启用硬保护**：触发条件必须同时包含范围纠错信号和“全部/全量/全租户/不限关键字”等全部组织意图，避免误伤“不是成本，而是财务”这类新关键词查询。
2. **坏计划先重规划，不执行**：首轮或后续规划若仍带被否定的历史范围，query flow 只给 planner 一次明确 correction。
3. **重复正确计划可收敛为 DONE**：已执行过的 `orgunit.list` 计划在纠错场景中重复出现，且不含 `keyword`、单实体参数或无依据父组织时，可视为模型遗漏 `DONE`，直接 narrate 最新结果。
4. **全租户分页必须显式表达**：`orgunit.list` 新增 `all_org_units=true`。未设置该参数且无 `parent_org_code` 时仍是一级组织；设置后按当前租户全部有效组织分页。
5. **反回流测试优先覆盖真实会话形态**：测试应模拟历史成本候选、当前纠错输入、坏计划重规划、正确计划执行后重复输出。

## 5. 验收

1. `go test ./internal/server/...`
2. `go test ./modules/cubebox/...`
3. 同一会话 `conv_bc7a2562b71344ebb552b989a96df232` 复测输入 `不只是包含成本关键字的组织，而是全部的组织`，不应返回 `cubebox_query_loop_repeated_plan`，也不应只呈现 `200007 成本C组` 或仅一级组织；结果应按 `all_org_units=true` 返回全租户分页清单。
