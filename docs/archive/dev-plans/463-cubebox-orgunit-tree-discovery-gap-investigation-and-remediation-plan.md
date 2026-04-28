# DEV-PLAN-463：CubeBox 组织树查询暴露的知识包驱动偏航调查与收敛方案

**状态**: 已归档（2026-04-24 17:10 CST；直接缺陷已修复并完成页面复验，后续轻量化整改与 owner 收口转入 `DEV-PLAN-464/465/466`）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：以“查询今天的组织树”真实失败场景为入口，冻结本次暴露出的两层问题：`A)` `orgunit.list` 根列表分页遗漏 `has_children` 的直接实现缺陷；`B)` CubeBox 查询主链从 `DEV-PLAN-461` 期望的“知识包 + 大模型驱动”偏航为“server 硬编码补丁驱动”的架构问题，并给出分层收敛方案与 stopline。
- **关联模块/目录**：`docs/dev-plans/460-cubebox-digital-assistant-positioning-and-execution-contract.md`、`docs/dev-plans/461-cubebox-query-scenarios-minimal-contract.md`、`docs/dev-plans/462-cubebox-codex-compaction-adoption-value-and-unified-convergence-plan.md`、`modules/cubebox`、`modules/orgunit/presentation/cubebox`、`internal/server`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`、`DEV-PLAN-430`、`DEV-PLAN-437A`、`DEV-PLAN-460`、`DEV-PLAN-461`、`DEV-PLAN-462`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_orgunit_executors.go`、`/org/api/org-units`
- **证据记录 SSOT**：真实页面与真链路复验证据统一记录在 `docs/dev-records/DEV-PLAN-463-READINESS.md`；本文件冻结调查结论、偏航诊断、stopline、收敛步骤与验收标准，不承载零散运行日志。

### 0.1 Simple > Easy 三问

1. **边界**：本计划不是单纯修一个 `has_children` 字段 bug；它同时处理“用户可见错误答案”与“为什么 CubeBox 查询链会持续靠 server 补丁撑住”的上游问题，但不扩张为通用 agent/workflow 计划。
2. **不变量**：CubeBox 查询必须继续复用既有 orgunit 读链路、租户/权限边界和现有只读 API；不能因为组织树场景失败，就引入数据库直查、第二读事实源、自由 planner 或模块级 AI 专用业务运行时。
3. **可解释**：reviewer 必须能在 5 分钟内说明三件事：为什么当前会显示“有下级：未知”；为什么这不是单点 UI 文案问题；为什么当前实现已经偏离 `461` 的“模型理解业务、代码只做通用映射”方向。

### 0.2 本轮冻结结论

- 本轮调查确认：`P0` 直接缺陷仍然成立，即根列表分页遗漏 `has_children`，导致回答层将真实值渲染为 `未知`。
- 本轮评审同时确认：这次故障不是孤立缺陷，而是 CubeBox 查询主链过度依赖 `internal/server` 硬编码默认值、关键词识别、模块专属 payload 和字符串摘要的一个用户可见症状。
- 本轮评审冻结一个更高层判断：当前 CubeBox 查询链是“看起来像知识包驱动，实际上靠 server 硬编码撑住”的实现；它还没有完全长成第二套 `orgunit` 系统，但已经明显偏离 `DEV-PLAN-461` 想要的“模型理解业务、代码只做通用执行映射”的方向。

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

因此当前问题表面上是：

1. 根组织的 `has_children` 在 CubeBox 返回面中被错误降级成了“未知”
2. 用户说“组织树”时，CubeBox 当前不会自动展开树，只会给出一级组织入口
3. 两者叠加后，用户会合理感知为“CubeBox 查不到下级组织”

但本计划认为，仅把问题理解成“某个字段漏查”是不够的。真正需要冻结的是：为什么 `461` 已经声明“知识包驱动 + 模型生成 `ReadPlan` + 代码只做执行映射”，当前实现却仍然在 `internal/server` 中靠关键词补丁、模块专属执行器协议和手写摘要模板维持行为闭环。

### 1.1 2026-04-24 真实页面复验补记

2026-04-24 已按真实页面链路再次复验：

- 入口：`http://localhost:8080/app/login`
- 登录：`admin@localhost / admin123`
- 页面路径：主壳层右侧 `CubeBox` 抽屉 -> 新建对话 -> 输入 `查询今天的组织树`
- 本次页面请求实际命中：
  - `POST /internal/cubebox/conversations`
  - `POST /internal/cubebox/turns:stream`

页面实际回答仍为：

- `已完成只读查询。`
- `本次关注：当前租户一级组织、状态、是否还有下级。`
- `step-1（orgunit.list）`
- `组织列表：1 条 / total null`
- `100000 | 飞虫与鲜花 | 状态：active | 业务单元：是 | 有下级：未知`

结论：

- `463` 当前不是“历史调查已过期”，而是“问题仍在、证据已补”
- 真实页面复验已完成，但修复与收敛尚未开始

## 2. 已完成调查（事实证据）

### 2.1 会话级事实

通过直接查询 `iam.cubebox_conversation_events`，已确认会话 `conv_15a789fbf2bd4ba59fb5ddd02b05f79b` 的实际事件流为：

1. 用户消息：`查询今天的组织树`
2. 回答面执行：`step-1（orgunit.list）`
3. 回答文本中明确写出：`有下级：未知`

这说明问题不是前端渲染错乱，而是服务端在回答层生成的摘要文本本身就已经错误。

### 2.2 数据级事实：根节点真实存在下级

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

### 2.5 代码级事实 C：回答层会把 `nil` 渲染成“未知”

`internal/server/cubebox_query_flow.go` 中的 `renderQueryOptionalBoolCN(...)` 对 `nil` 指针固定渲染为：

- `未知`

所以一旦根列表 `HasChildren` 在分页实现中未赋值，CubeBox 最终对用户展示的就是：

- `有下级：未知`

### 2.6 架构级事实 D：知识包已经定义了 `orgunit` 查询语义，但运行时没有把它当成一等事实源

`modules/orgunit/presentation/cubebox/queries.md`、`apis.md`、`examples.md` 已经冻结了以下知识：

- `orgunit.list` 代表“一级组织列表”或“某个上级组织下的直接子组织列表”
- “组织树”默认先查一级组织
- `orgunit.list/details/search/audit` 的参数与关注字段
- 多步只读编排示例与缺参追问口径

但当前运行时并没有把这些知识提炼成真正的结构化运行时契约；它们大多只是作为整段 prompt 文本喂给模型，后续实际行为又被 `internal/server` 硬编码逻辑二次改写。

### 2.7 架构级事实 E：查询主链仍在 `internal/server` 中做业务语义级硬编码补丁

`internal/server/cubebox_query_flow.go` 当前仍存在以下模式：

- 对 `orgunit.list` 做默认值补丁
- 用 `isOrgUnitRootListPrompt(...)` / `isOrgUnitChildrenPrompt(...)` 识别“组织树”“下级”“子组织”等关键词
- 在模型给出缺参计划后，由 server 自己把澄清计划改写成根列表默认执行，或重写澄清文案

这说明当前实现不是“模型借助知识包稳定输出合法 `ReadPlan`，代码只做校验和执行映射”，而是“模型先给一个草案，server 再做第二轮业务理解与补丁”。

### 2.8 架构级事实 F：`cubebox_orgunit_executors.go` 已经开始长出模块专属编排协议

`internal/server/cubebox_orgunit_executors.go` 当前不只是薄适配层，还持有：

- `org_code_from`
- `target_unique`
- `step.field` 形式的前序结果引用协议
- `orgunit.search` 的额外 enrichment（如 `path_org_codes`）
- `orgunit.list` / `orgunit.audit` 的模块专属 payload 结构

这些内容虽然尚未完全形成第二套 `orgunit` 系统，但已经让执行注册层偏离了 `461` 所要求的“只做白名单注册、参数收口、顺序调度和结果转交”的边界。

### 2.9 架构级事实 G：结果解释几乎完全退化成代码模板，而没有真正使用大模型

`internal/server/cubebox_query_flow.go` 当前使用：

- `buildCubeboxQueryAnswer(...)`
- `summarizeQueryResult(...)`
- `SummaryRenderer`
- `summarizeOrgUnitListQueryResult(...)`

来把执行结果直接拼成最终用户可见回答。

这意味着当前大模型在查询链里主要只承担：

- 生成结构化 `ReadPlan`

而并没有真正承担：

- 查询结果解释
- 多步结果整合
- 面向用户的自然语言总结

所以现状不是“充分利用大模型”，而是“把大模型压缩成严格 JSON 生成器，再由 server 手工完成后半程”。

### 2.10 架构级事实 H：知识包校验只验证外形，不提供可直接驱动运行时的结构化契约

`modules/cubebox/knowledge_pack.go` 当前主要校验：

- 文件存在
- fenced block 存在
- `queries.md` 有 intents
- `apis.md` 有 `executor_key`
- `examples.md` 有 `ReadPlan` 示例

但它并不会把知识包提炼成真正的运行时结构对象，用于：

- 默认值来源
- 参数约束来源
- 澄清口径来源
- 可执行面来源

因此知识包在现状里更像 prompt 素材，而不是第一事实源。

### 2.11 架构级事实 I：测试正在加固硬编码行为，而不是约束它们收缩

`internal/server/cubebox_api_test.go` 当前已有测试明确断言：

- “查询组织树”会被 server 自动降级成根列表默认查询
- children query 会被 server humanize

这类测试保护的是当前硬编码补丁行为，而不是 `461` 的“知识包驱动 + 模型生成计划 + 代码只做映射”目标。

### 2.12 代码级事实 J：`parent_org_code_from` 不是轻量补参

当前若想把“先列根节点，再自动展开直接子组织”做成真正可执行的两步计划，还存在三个未冻结的契约缺口：

1. `orgunit.list` 的执行器白名单当前只接受：
   - `include_disabled`
   - `parent_org_code`
   - `keyword`
   - `status`
   - `page`
   - `size`
2. `orgunit.list` 当前返回 payload 只提供：
   - `org_units`
   - 分页信息（若启用分页）
   - 不提供可直接被下一步引用的单值 `target_org_code`
   - 也不提供等价的唯一结果标记
3. 当前前序结果解析器只支持：
   - `step.field`
   - 并依赖前一步 payload 中的唯一性标记做 fail-closed 校验

因此，若未来采纳“自动展开一层”方案，至少还要同时冻结：

- `orgunit.list` 的前序引用入参语义
- 根列表结果里可被下一步引用的字段契约
- “仅在唯一结果时允许继续展开”的唯一性约束

这属于后续 `P1` 能力补全设计，不应被误写成当前 `P0` bugfix 的附带小改动。

## 3. 根因与偏航诊断

### 3.1 直接 `P0` 根因：根列表分页读取遗漏 `has_children`

这是本次用户可见错误答案的直接根因。

问题链路为：

1. CubeBox 执行 `orgunit.list`
2. orgunit 实际走 `orgUnitPGStore.ListOrgUnitsPage(...)`
3. 根列表分支未查询 `has_children`
4. `orgUnitListItem.HasChildren` 为空
5. 回答层统一渲染为 `未知`

该问题属于：

- **已有能力的实现缺陷**
- **不需要新增产品能力即可修复**

### 3.2 第一层偏航：大模型被降格为“严格 JSON 生成器”

`461` 原本期望：

- 模型借助知识包做业务理解
- 模型输出结构化计划
- 后续查询结果再被稳定解释

但当前现实是：

- 模型主要只负责生成 `ReadPlan`
- 一旦 `ReadPlan` 不符合预期，server 会用业务关键词补丁继续改写
- 结果解释则几乎全部退回到手写代码模板

因此当前大模型并未被充分用于高价值的“业务理解 + 结果解释”环节。

### 3.3 第二层偏航：知识包不是第一事实源，运行时代码中存在第二份业务语义

当前 `queries.md/apis.md/examples.md` 与 `internal/server` 中同时存在：

- 自然语言映射
- 默认值规则
- 缺参追问
- 可执行面边界
- 结果解释重点

这会导致：

- 知识包修改后，运行时代码未必同步
- reviewer 无法快速判断哪一处才是真正 owner
- CubeBox 渐渐回退成“prompt + 硬编码混合驱动”

### 3.4 第三层偏航：执行注册层正在变厚，开始承载模块专属 AI 编排语义

`cubebox_orgunit_executors.go` 当前已经不只是“参数收口 + 调现有 API”，而是逐步承载：

- 模块专属前序引用协议
- 模块专属唯一性标记
- 模块专属 payload 约定
- 模块专属 enrichment

如果继续沿这个方向扩张，`internal/server` 会长成 AI 专用业务编排层，最终逼近第二套 `orgunit` 运行时。

### 3.5 第四层偏航：结果解释路径仍然是能力专属模板化，而不是统一收敛

`462` 已明确警告：

- 不要把 `orgunit.list` 一类能力继续做成局部摘要器和模板分支

但当前实际仍然依赖：

- `SummaryRenderer`
- `summarizeOrgUnitListQueryResult(...)`
- `formatOrgUnitListSummaryItem(...)`

这使 CubeBox 的回答更像模板化报表，而不是数字助手式解释。

### 3.6 第五层偏航：测试资产正在保护现状补丁，而不是推动收敛

如果测试继续断言：

- server 会自动把“查询组织树”改写成根列表默认执行
- server 会继续 humanize 某类澄清

那么后续想真正把逻辑回迁到知识包/模型层时，首先要修改的将不是实现，而是大批测试预期。这说明当前测试资产也已参与固化偏航。

### 3.7 总判断

本计划冻结以下总判断作为 reviewer 口径：

- 当前 CubeBox 查询链是“看起来像知识包驱动，实际上靠 server 硬编码撑住”的实现。
- 它还没有完全长成第二套 `orgunit` 系统，但已经明显偏离 `DEV-PLAN-461` 想要的“模型理解业务、代码只做通用执行映射”的方向。
- `has_children` 只是这次最先被用户看到的一个 `P0` 症状，不是全部问题。

## 4. 与 460/461/462 契约的关系

### 4.1 对 `460` 的判断

当前实现仍基本遵守 `DEV-PLAN-460` 的这些不变量：

- CubeBox 仍沿用当前租户、当前用户、当前 session
- 查询仍复用现有只读链路
- 尚未引入绕过权限的第二读事实源

因此，本计划不是在质疑 `460` 的权限边界，而是在质疑 `461` 的查询运行时收敛方向。

### 4.2 对 `461` 的判断

`DEV-PLAN-461` 已冻结：

- 模块知识应放在 Markdown 知识包中
- 代码层只持有通用加载、校验和执行能力
- 执行注册层不是第二套业务实现

当前实现对 `461` 的偏离主要体现在四点：

1. 查询默认值与自然语言补丁没有留在知识包/模型层，而是回流到 `internal/server`
2. 执行注册层开始承载模块专属编排协议
3. 结果解释几乎完全走代码模板，而不是统一解释收敛
4. 测试资产在保护硬编码行为，而不是保护知识包驱动边界

### 4.3 对 `462` 的判断

`DEV-PLAN-462` 要求：

- 长结果与解释必须走统一收敛入口
- 不通过到处 patch 文案来掩盖能力边界

因此本计划明确不接受的做法包括：

- 只把“未知”硬改成“可能有”
- 在回答总线里新增针对“组织树”字符串的临时分支
- 继续新增 `orgunit.*` 专用 `SummaryRenderer` 或手写模板
- 不修数据面与边界面，只靠前端或回答文案掩盖问题

## 5. 收敛方案（冻结执行清单）

### 5.0 总体策略

将本次问题拆成三个连续但独立的交付：

1. **Phase A / P0 直接缺陷修复**：修复根列表 `has_children` 丢失，先消除用户可见错误答案
2. **Phase B / P1 查询主链去硬编码**：停止在 `internal/server` 用 prompt 关键词补丁重写模型计划，把默认值/澄清语义收回知识包与 planner owner
3. **Phase C / P1 执行层与解释层瘦身**：阻断 `cubebox_orgunit_executors.go` 继续长出模块专属运行时，并限制能力专属摘要器继续扩张

### 5.1 Phase A / P0：根列表 `has_children` 修复

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

### 5.2 Phase B / P1：查询主链去硬编码

#### 目标

- 不再让 `internal/server` 通过 prompt 关键词补丁决定 orgunit 业务语义
- 让“默认值 / 澄清 / 一级组织入口”尽量回到知识包与模型计划 owner

#### 实施步骤

1. [ ] 盘点并标注当前 `internal/server/cubebox_query_flow.go` 中与 `orgunit` 业务语义强耦合的补丁点
   - `normalizeCubeboxExecutableListPlan(...)`
   - `normalizeCubeboxClarifyingListPlan(...)`
   - `isOrgUnitRootListPrompt(...)`
   - `isOrgUnitChildrenPrompt(...)`
2. [ ] 冻结 stopline
   - 不再新增任何基于“组织树 / 下级 / 子组织”等关键词的 server 侧业务语义补丁
3. [ ] 重新定义默认值 owner
   - `orgunit.list` 的安全默认值与澄清语义，以知识包与 planner 输出为主
   - server 只做 fail-closed 校验，不再做第二轮业务理解
4. [ ] 重写相关测试
   - 不再把“server 自动降级/自动改写计划”当成应被长期保护的行为
   - 转而保护“知识包 + planner 输出 + 注册表校验”链路

#### 验收标准

- `internal/server` 不再新增 orgunit 自然语言关键词补丁
- orgunit 默认值与澄清逻辑可以明确追溯到知识包与 planner 输出，而不是 server 二次判断
- 原有依赖 server 改写行为的测试完成重写或降级为过渡性说明，不再作为长期合同

### 5.3 Phase C / P1：执行层与解释层瘦身

#### 目标

- 将 `cubebox_orgunit_executors.go` 收回到薄适配层
- 阻断结果解释继续扩张为能力专属模板体系

#### 实施步骤

1. [ ] 为 `cubebox_orgunit_executors.go` 增加明确 stopline
   - 不再新增未在 dev-plan 中冻结的 `*_from` 参数
   - 不再新增模块专属唯一性标记或 payload 字段
   - 不在执行器中新增新的业务语义折叠和展示语义
2. [ ] 评估现有模块专属协议
   - `org_code_from`
   - `target_unique`
   - `path_org_codes`
   - `SummaryRenderer`
   将其分类为：
   - 临时过渡能力
   - 可上提为通用 contract 的能力
   - 应删除的局部硬编码
3. [ ] 收紧解释层
   - 不再继续为单个 `executor_key` 扩张专用字符串模板
   - 若短期仍保留 `SummaryRenderer`，必须明确其为过渡能力，并禁止复制扩散
4. [ ] 若未来要做“自动展开一层”
   - 先冻结共享 contract：前序引用参数、可引用结果字段、唯一性约束
   - 再评估是否允许在执行层承接，而不是直接在 `orgunit` 执行器里做局部拼装

#### 验收标准

- `cubebox_orgunit_executors.go` 不再继续增长模块专属 orchestrator 语义
- 若新增跨步引用能力，必须先有上位 contract 文档，而不是直接改执行器
- 结果解释链不再扩张新的 `orgunit.*` 专用模板分支

## 6. 明确非目标

- 不在本计划内引入通用 agent 平台、workflow engine、DAG planner、并发 fan-out/fan-in 或动态工具发现
- 不在本计划内把 orgunit 查询改成数据库直查器
- 不在本计划内建设向量检索、租户知识库中台或第二套 AI 平台 runtime
- 不在本计划内承诺“组织树一次性展开全部层级”
- 不在本计划内直接把所有查询结果解释重新交给自由生成式模型输出；首轮目标是先阻断 server 硬编码继续扩张

## 7. 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `internal/server` | 根列表字段完整性、查询主链去硬编码、执行层 stopline | `internal/server/orgunit_field_metadata_store_pg_methods_test.go`、`internal/server/cubebox_query_flow_test.go`、`internal/server/cubebox_orgunit_executors_test.go`、`internal/server/cubebox_api_test.go` | 本计划核心 owner 在这里 |
| `modules/cubebox` | `ReadPlan`、执行注册表、知识包校验边界 | `modules/cubebox/read_plan.go`、`modules/cubebox/read_executor.go`、`modules/cubebox/knowledge_pack.go` | 防止知识包继续停留在“只有 prompt 素材”的状态 |
| `modules/orgunit/presentation/cubebox` | 查询语义、默认值、澄清口径、示例 | `queries.md`、`apis.md`、`examples.md` | 知识包应回到第一事实源位置 |
| `E2E / 真实会话复验` | 真实页面触发“查询今天的组织树” | `docs/dev-records/DEV-PLAN-463-READINESS.md` | 至少保留一次真实会话证据 |

测试原则冻结如下：

1. 先补“根列表 `has_children` 不丢失”的直接测试，再进行架构收敛。
2. 不允许再新增把 server prompt 关键词补丁当成长期合同的测试。
3. 若引入新的跨步引用能力，必须补共享 contract 级测试，而不是只补模块私有 happy-path。
4. 不允许通过继续扩张字符串模板和 `SummaryRenderer` 来替代真正的边界收敛。

## 8. 验收清单

1. [ ] 根列表分页查询返回 `HasChildren`
2. [ ] CubeBox 查询回答不再对真实有下级的根组织输出“未知”
3. [ ] `463` 明确记录并冻结“知识包驱动偏航”的诊断与 stopline
4. [ ] `internal/server` 不再新增 orgunit prompt 关键词补丁
5. [ ] `cubebox_orgunit_executors.go` 停止继续扩张模块专属运行时语义
6. [ ] 若引入 `parent_org_code_from` 或等价能力，先有上位 contract，再有执行器实现
7. [X] 真实页面或真实会话复验通过，并记录到 `docs/dev-records/DEV-PLAN-463-READINESS.md`

## 9. 需要执行的门禁与核验

- 命中 Go 代码：按 `AGENTS.md` 触发 `go fmt ./... && go vet ./... && make check lint && make test`
- 命中文档：`make check doc`
- 若修改 CubeBox 查询主链、知识包加载、执行注册表或 orgunit 读适配层，建议补跑相关定向测试后再决定是否跑全量 `make preflight`

## 10. 交付物

- `docs/archive/dev-plans/463-cubebox-orgunit-tree-discovery-gap-investigation-and-remediation-plan.md`
- `docs/dev-records/DEV-PLAN-463-READINESS.md`
- `internal/server` 中与 `orgunit.list` / `cubebox_query_flow` / `cubebox_orgunit_executors` 相关的修复与收敛实现
- `modules/cubebox` 与 `modules/orgunit/presentation/cubebox` 中与知识包和执行边界相关的收敛更新
