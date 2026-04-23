# DEV-PLAN-463：CubeBox 组织树发现缺口调查与修复方案

**状态**: 规划中（2026-04-23 22:34 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：专门记录 CubeBox 在“查询今天的组织树”场景中暴露出的组织树发现缺口，冻结调查事实、根因分层、owner 边界、修复步骤与验收口径，确保 CubeBox 至少能正确报告一级组织是否存在下级，并明确“组织树”当前能力边界与后续扩展方案。
- **关联模块/目录**：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`modules/cubebox`、`modules/orgunit/presentation/cubebox`、`internal/server`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`、`DEV-PLAN-430`、`DEV-PLAN-437A`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`
- **用户入口/触点**：主应用壳层右侧 CubeBox 抽屉、`/internal/cubebox/turns:stream`、`orgunit.list` 查询执行器、`/org/api/org-units`
- **证据记录 SSOT**：本计划的实施与验证记录统一回填 `docs/dev-records/DEV-PLAN-463-READINESS.md`；本文件只冻结调查结论、修复边界、实施步骤与验收标准，不承载零散运行日志。

### 0.1 Simple > Easy 三问

1. **边界**：本计划同时处理两个相邻但不同的问题：`A)` 已有 `orgunit.list` 返回面中的 `has_children` 丢失 bug；`B)` “组织树”自然语言在 CubeBox 中当前只落成“一级组织入口”而非“整棵树递归展开”的能力边界不清。两者必须分 owner、分验收，不允许混成一次模糊补丁。
2. **不变量**：CubeBox 查询必须继续复用既有 orgunit 读链路与租户/权限边界；不能因为要“看组织树”而新增数据库直查、自由递归 planner、未登记 `api_key` 或第二套 orgunit 读事实源。
3. **可解释**：reviewer 必须能在 5 分钟内说明“为什么当前会显示‘有下级：未知’”“为什么用户说‘组织树’时只拿到一级组织”“bugfix 与能力扩展分别由哪一层承接”。

## 1. 背景

2026-04-23 的真实用户会话 `conv_15a789fbf2bd4ba59fb5ddd02b05f79b` 中，用户输入：

- `查询今天的组织树`

CubeBox 返回：

- 已完成只读查询
- 本次关注：按当天列出组织（可作为组织树入口）、状态、是否还有下级（用于逐层展开）
- `step-1（orgunit.list）`
- `100000 | 飞虫与鲜花 | 状态：active | 业务单元：是 | 有下级：未知`

这与真实数据不一致。对同一租户、同一日期 `2026-04-23` 直接核验 orgunit 数据可见：

- `100000 | 飞虫与鲜花` 为根组织
- 其下存在直接下级：
  - `200000 | 飞虫公司`
  - `300000 | 鲜花公司`
- 进一步还存在二级下级：
  - `200001 | 财务部`

因此当前问题不是“没有下级组织”，而是：

1. 根组织的 `has_children` 在 CubeBox 返回面中被错误降级成了“未知”；
2. 用户说“组织树”时，CubeBox 当前并不会自动展开树，只会给出一级组织入口；
3. 这两件事叠加后，用户会合理感知为“CubeBox 查不到下级组织”。

## 2. 已完成调查（事实证据）

### 2.1 会话级事实

通过直接查询 `iam.cubebox_conversation_events`，已确认会话 `conv_15a789fbf2bd4ba59fb5ddd02b05f79b` 的实际事件流为：

1. 用户消息：`查询今天的组织树`
2. 回答面执行：`step-1（orgunit.list）`
3. 回答文本中明确写出：`有下级：未知`

这说明问题不是前端渲染错乱，而是服务端在回答层生成的摘要文本本身就已经错误。

### 2.2 数据级事实

对本地 PostgreSQL 中 `orgunit.org_unit_versions` / `orgunit.org_unit_codes` 直接核验，租户 `00000000-0000-0000-0000-000000000001` 在 `2026-04-23` 的当前树形事实为：

- 根节点 `100000 / 飞虫与鲜花 / active / is_business_unit=true`
- 根节点 `has_children = true`
- 当前有效组织共有 4 条：
  - `100000 | 飞虫与鲜花`
  - `200000 | 飞虫公司`
  - `200001 | 财务部`
  - `300000 | 鲜花公司`

因此“有下级：未知”与真实 orgunit 当前视图冲突。

### 2.3 代码级事实 A：非分页兜底路径会正确返回 `HasChildren`

`internal/server/orgunit_api.go` 中的 `listOrgUnitListPage(...)` 兜底逻辑在：

- 根列表路径：会把 `OrgUnitNode.HasChildren` 映射到 `orgUnitListItem.HasChildren`
- 子列表路径：会把 `OrgUnitChild.HasChildren` 映射到 `orgUnitListItem.HasChildren`

这说明 orgunit 领域读取链路并不天然缺失“是否有下级”信息。

### 2.4 代码级事实 B：实际分页实现只在子列表查询 `has_children`

`internal/server/orgunit_field_metadata_store.go` 的 `orgUnitPGStore.ListOrgUnitsPage(...)` 中：

- 当 `parentOrgNodeKey != ""` 时，`SELECT` 会额外查询 `has_children`
- 当查询根列表时，`SELECT` 只返回 `org_code / name / status / is_business_unit`
- 根列表扫描结果时也不会给 `item.HasChildren` 赋值

因此：

- 走分页读取器时，子列表有 `HasChildren`
- 走分页读取器时，根列表 `HasChildren == nil`

### 2.5 代码级事实 C：CubeBox 回答层会把 `nil` 渲染成“未知”

`internal/server/cubebox_query_flow.go` 中的 `renderQueryOptionalBoolCN(...)` 对 `nil` 指针固定渲染为：

- `未知`

所以一旦根列表 `HasChildren` 在分页实现中未赋值，CubeBox 最终对用户展示的就是：

- `有下级：未知`

### 2.6 代码级事实 D：当前“组织树”语义只冻结到 `orgunit.list`

`modules/orgunit/presentation/cubebox/queries.md` 与 `examples.md` 已冻结：

- `orgunit.list` 代表“一级组织列表”或“某个上级组织下的直接子组织列表”
- 不是整棵树递归展开能力

也就是说，用户说“组织树”时，当前知识包与执行器最稳定的落点就是：

- 先列根组织，作为树入口
- 若要继续展开，则需要后续针对某个父组织再查直接子组织

### 2.7 代码级事实 E：当前 `orgunit.list` 不支持前序结果派生 `parent_org_code`

当前执行器只支持：

- `orgunit.details` / `orgunit.audit` 从前序步骤解析 `org_code_from`

但 `orgunit.list` 只接受：

- `parent_org_code`

并不支持：

- `parent_org_code_from`

因此即使 `ReadPlan` 支持线性多步调度，目前也无法自然表达：

1. 先列根节点；
2. 再用前一步返回的 org_code 继续查询其直接子组织。

## 3. 根因结论

### 3.1 一级根因：根列表分页读取遗漏 `has_children`

这是本次“查不到下级组织”感知中的直接 bug 根因。

问题链路为：

1. CubeBox 执行 `orgunit.list`
2. orgunit 实际走 `orgUnitPGStore.ListOrgUnitsPage(...)`
3. 根列表分支未查询 `has_children`
4. `orgUnitListItem.HasChildren` 为空
5. 回答层统一渲染为 `未知`

该问题属于：

- **已有能力的实现缺陷**
- **不需要新增产品能力即可修复**

### 3.2 二级根因：当前“组织树”与“一级组织入口”语义没有明确外显给用户

当前知识包与查询能力设计本意是：

- 先返回根组织入口
- 再由用户逐层查询或进入业务页面查看

但回答面文案仍然使用了“组织树”这一用户词汇，而没有明确指出：

- 当前只返回一级入口，不代表已经展开整棵树

这会让用户把“没展开”误读为“没查到”。

### 3.3 三级根因：线性多步框架存在，但 `orgunit.list` 无法稳定承接“继续展开”

`DEV-PLAN-461` 已冻结：

- 支持线性多步只读编排

但当前 orgunit 查询面只落了：

- `details` 可从 `search` 结果继续取 `org_code_from`

并未落：

- `list` 从前一步结果继续取 `parent_org_code`

因此当前系统虽然理论上支持多步，但对“树形逐层展开”这个具体场景并没有真正打通。

## 4. 与 460/461/462 契约的关系

### 4.1 不偏离 `460`

本计划继续遵守 `DEV-PLAN-460`：

- CubeBox 是当前用户的数字助手
- 查询必须受当前租户、当前权限、当前会话约束
- 不得把文档或模型推理当成新的授权来源

### 4.2 履行 `461`，但暴露了两个未封死的缺口

`DEV-PLAN-461` 已冻结：

- 查询执行必须复用现有读 API
- 长结果由统一摘要层承接
- 不在总线里堆 capability-specific 特判

本次问题说明 `461` 在 orgunit 样板上仍有两个缺口：

1. 样板执行器返回面的字段完整性没有在分页实现上完全对齐；
2. “树形逐层展开”虽属于允许的线性多步场景，但当前样板没有形成可执行的参数派生路径。

### 4.3 对齐 `462`

`462` 要求：

- 长结果与解释必须走统一收敛入口
- 不通过到处 patch 文案来掩盖能力边界

因此本计划不接受的做法包括：

- 只把“未知”硬改成“可能有”
- 在回答总线里新增针对“组织树”字符串的临时分支
- 不修 `HasChildren` 数据面，只改前端文案

## 5. 修复方案（冻结执行清单）

### 5.0 总体策略

将本次问题拆成两个连续但独立的交付：

1. **PR-A / P0 bugfix**：修复根列表 `has_children` 丢失，确保 CubeBox 至少能正确报告“是否还有下级”
2. **PR-B / P1 能力补全**：明确“组织树”当前交付边界，并补齐最小的逐层展开能力或稳定澄清语义

P0 完成前，不得宣称 CubeBox 已能正确回答组织树发现问题。

### 5.1 PR-A：根列表 `has_children` 修复

#### 目标

- 根列表查询与子列表查询都稳定返回 `HasChildren`
- CubeBox 回答层不再对真实有下级的根组织输出“未知”

#### 实施步骤

1. [ ] 修复 `internal/server/orgunit_field_metadata_store.go` 中 `ListOrgUnitsPage(...)`
   - 根列表分支也要查询 `has_children`
   - 根列表扫描结果必须给 `item.HasChildren` 赋值
2. [ ] 对齐 `orgUnitPGStore.ListNodesCurrent*` 与 `ListOrgUnitsPage(...)` 的字段语义
   - 防止再次出现“非分页有、分页没有”的漂移
3. [ ] 补 server 层回归测试
   - 覆盖“根列表分页返回 `has_children=true/false`”
   - 保留“子列表分页返回 `has_children`”原有断言
4. [ ] 补 CubeBox 查询回答层回归测试
   - 覆盖 `orgunit.list` 根列表场景输出“有下级：是/否”，不再为“未知”

#### 验收标准

- 真实会话同类查询中，根组织 `100000 / 飞虫与鲜花` 在 `2026-04-23` 返回 `有下级：是`
- 所有相关单元/适配层测试通过

### 5.2 PR-B：组织树语义与逐层展开最小补全

#### 目标

- 用户说“组织树”时，不再让“一级入口”与“整棵树已展开”混淆
- 在不引入通用递归工作流的前提下，补一个最小可解释闭环

#### 候选方案

**方案 1：文档与回答语义收口，不补新能力**

- 把“查询今天的组织树”明确解释成：
  - “已列出今天的一级组织入口，可继续展开某个组织的直接下级”
- 当存在唯一根组织且 `has_children=true` 时，主动提示下一步：
  - “如需继续展开，请告诉我要查看哪个组织下面的直接子组织”

优点：

- 改动最小
- 不引入新的 planner 参数派生

缺点：

- 仍需用户再问一轮

**方案 2：为 `orgunit.list` 补 `parent_org_code_from`，支持最小线性展开**

- 给 `orgunit.list` 执行器增加前序结果引用能力
- 允许两步计划：
  1. `step-1` 列根组织
  2. `step-2` 用前序唯一结果继续列直接子组织

优点：

- 符合 `461` 线性多步框架
- 不需要新 `api_key`

缺点：

- 必须非常严格地限定“只能引用唯一结果”
- 若根列表不唯一，仍需澄清

#### 当前建议

本计划建议采用分阶段收口：

1. **先落方案 1**：把现有产品语义说清楚，避免误导
2. **再评估方案 2**：若用户确实高频提出“组织树”并期望至少自动展开一层，再在 `orgunit.list` 上补 `parent_org_code_from`

#### 实施步骤

1. [ ] 更新 `modules/orgunit/presentation/cubebox/queries.md`
   - 明确“组织树”当前默认映射是“一级组织入口 / 直接子组织列表”，不是整棵树递归展开
2. [ ] 更新 `modules/orgunit/presentation/cubebox/examples.md`
   - 增加“先列根组织，再展开直接子组织”的示例
3. [ ] 若采纳方案 1：
   - 调整 `explain_focus` / 回答文案，使其明确“作为组织树入口”
4. [ ] 若采纳方案 2：
   - 为 `orgunit.list` 增加 `parent_org_code_from`
   - 补唯一结果校验与相应测试

#### 验收标准

- 用户说“组织树”时，回答必须明确自己返回的是哪一层
- 不允许再出现“看起来像整棵树结果，实际只是一级入口”的歧义
- 若实现自动展开一层，必须在多步测试中稳定通过；若不实现，则必须稳定输出清晰下一步提示

## 6. 明确非目标

- 不在本计划内新增通用递归树查询引擎
- 不在本计划内引入 DAG planner、并发 fan-out/fan-in 或动态工具发现
- 不在本计划内把 orgunit 查询改成数据库直查器
- 不在本计划内增加新的 orgunit 专用自由文本回答链路
- 不在本计划内承诺“组织树一次性展开全部层级”

## 7. 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `internal/server` | 分页读取字段完整性、执行器参数收口、回答摘要输出 | `internal/server/orgunit_field_metadata_store_pg_methods_test.go`、`internal/server/orgunit_read_api_test.go`、`internal/server/cubebox_orgunit_executors_test.go`、`internal/server/cubebox_query_flow_test.go` | 本次核心 owner 在适配层与查询组装层 |
| `modules/orgunit/presentation/cubebox` | 知识包语义与示例对齐 | `queries.md`、`examples.md` | 通过文档评审和真实会话验证承接 |
| `E2E / 真实会话复验` | 真实页面中触发“查询今天的组织树” | `docs/dev-records/DEV-PLAN-463-READINESS.md` | 本计划至少要留一次真实会话证据 |

测试原则冻结如下：

1. 先补“根列表 `has_children` 不丢失”的直接测试，再谈产品语义。
2. 若引入 `parent_org_code_from`，必须补“唯一结果才允许引用”的负向测试。
3. 不允许通过改摘要模板绕过底层字段缺失问题。

## 8. 验收清单

1. [ ] 根列表分页查询返回 `HasChildren`
2. [ ] CubeBox 查询回答不再对真实有下级的根组织输出“未知”
3. [ ] 文档明确“组织树”当前默认交付边界
4. [ ] 若实现自动展开一层，补齐执行器与线性多步测试
5. [ ] 真实页面或真实会话复验通过，并记录到 `docs/dev-records/DEV-PLAN-463-READINESS.md`

## 9. 需要执行的门禁与核验

- 命中 Go 代码：按 `AGENTS.md` 触发 `go fmt ./... && go vet ./... && make check lint && make test`
- 命中文档：`make check doc`
- 若修改 CubeBox 查询主链或 orgunit 读适配层，建议补跑相关定向测试后再决定是否跑全量 `make preflight`

## 10. 交付物

- `docs/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md`
- `docs/dev-records/DEV-PLAN-463-READINESS.md`
- `internal/server` 相关修复与测试
- `modules/orgunit/presentation/cubebox` 知识包更新（若进入 PR-B）
