# DEV-PLAN-474：CubeBox 跨 Turn 结果集续接与补字段查询收敛方案

**状态**: 进行中（2026-04-27 16:55 CST）

## 0. 范围一句话

冻结并实现 `CubeBox` 在“上一轮已经成功列出一批组织，下一轮再追问‘他们/这些/上面这些’并要求补充路径或其他详情字段”场景下的最小连续查询闭环，避免再次落入 `ai_plan_boundary_violation`。

## 1. 背景

真实会话已暴露以下链路：

1. 用户先问：`组织名称里包含“财务”（关键词）`
2. 系统成功返回一组组织列表
3. 用户继续问：`增加列出他们的组织路径`
4. 系统返回：`查询计划超出允许范围，请调整问题后重试。`

当前实现的问题不在于“路径字段不存在”，而在于：

- 上一轮成功列表结果虽然会以 `turn.query_candidates.presented` 写入 canonical events，但 prompt-facing `query_evidence_window` 仍将其统一投影为 `presented_options`
- `presented_options` 同时承担“歧义候选待用户选择”与“上一轮成功结果集”两种不同语义，导致 planner 无法稳定判断当前轮是在“选择候选”还是在“续接同一批对象补字段”
- 当前 turn 内 `working_results` 不跨 turn 继承，因此上一轮那批 `org_code` 不能以 executor state 的方式直接续接

## 2. 目标

1. [ ] 将“上一轮成功结果集”与“待用户显式选择的候选项”在 `query_evidence_window` 中区分建模。
2. [ ] 允许模型在结果集规模可控时，把上一轮明确结果集当作当前 turn 的 target set，生成线性 `ReadPlan` 去补查 `orgunit.details`。
3. [ ] 保持现有只读执行边界、租户/权限校验、`ReadPlan` schema 与 fail-closed 行为不变。
4. [ ] 为真实样例补回归测试：`财务组织列表 -> 增加列出他们的组织路径` 不再触发 `ai_plan_boundary_violation`。

## 3. 非目标

- 不把本次修复扩张成通用 DAG/fan-out 查询平台。
- 不引入第二套 memory store、缓存、页面上下文补参或前端拼 prompt。
- 不顺手解决“单轮直接问全部财务组织并带路径”的广义多对象展开问题；本次仅处理“上一轮已有明确结果集”的跨 turn 续接。

## 4. 契约收敛

### 4.1 新的 evidence 语义

- `query_evidence_window.observations.kind=presented_options`
  - 仅表示：系统给用户展示过待确认候选项
  - 对应特征：通常 `requires_explicit_user_choice=true`
- `query_evidence_window.observations.kind=result_list`
  - 表示：上一轮已经成功返回过一组明确结果集
  - 对应特征：`candidate_source=results` 且不要求用户显式选择

### 4.2 planner 使用规则

- 若当前轮是“他们/这些/上面这些/增加列出路径/补充路径长名称/增加组织路径”一类追问，且 `query_evidence_window` 中最近存在 `result_list`：
  - 可把该组 `entity_key` 视为当前 target set
  - 若请求字段不在 `orgunit.list` 返回面，而在 `orgunit.details` 返回面，应生成小批量线性 `READ_PLAN`
- 若 `result_list` 项数超出安全阈值，不得静默展开大批量 `details` 查询；应返回 `CLARIFY` 要求缩小范围

### 4.3 当前阈值

- 首期安全阈值冻结为：`<= 10` 个组织允许自动补查
- `> 10` 时进入澄清，不自动 fan-out

## 5. 实施步骤

1. [ ] 在 `modules/cubebox/query_entity.go` 为 `query_evidence_window` 增加 `result_list` 投影规则。
2. [ ] 在 planner prompt 与 `modules/orgunit/presentation/cubebox/*` 中补充 `result_list` 续接说明与示例。
3. [ ] 补单元/服务端测试，覆盖 evidence 投影与“财务列表 -> 组织路径”回归链路。
4. [ ] 执行文档与 Go 相关校验。

## 6. 验收标准

- 用户上一轮已拿到组织列表，下一轮要求“增加列出他们的组织路径”时，不再落入 `ai_plan_boundary_violation`
- planner 输入的 `query_evidence_window` 中可区分 `result_list` 与 `presented_options`
- `orgunit` 知识包存在明确示例，说明如何基于上一轮结果集补查 `full_name_path`
- 不新增新的执行器、数据库读面或 capability-specific Go 关键词补丁
