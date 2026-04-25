# DEV-PLAN-468 Readiness

## 说明

- 本文件用于登记 `DEV-PLAN-468` 的代码落地、自动化验证、真实页面复验与后续待补证据。
- 当前阶段只记录已实际完成的验证；未做的真实页面复验保持待办，不以文档口径代替真实证据。

## 2026-04-25 实施记录

### 代码落地范围

- 已扩展 `modules/cubebox.QueryContext`：
  - `RecentConfirmedEntities`
  - `RecentDialogueTurns`
  - `LastClarification`
  - `RecentCandidates`
  - `ResolvedEntity`
  - 同时保留 `RecentConfirmedEntity` 兼容访问
- 已新增 query metadata event：
  - `turn.query_candidates.presented`
  - `turn.query_clarification.requested`
  - `turn.query_context.resolved`
- 已在 planner 输入中注入 `query_dialogue_context`。
- 已支持通用前序结果引用：
  - `@step-1.target_org_code`
  - `@step-1.payload.target_org_code`
- 已更新 `orgunit` 知识包，使模型可基于 `query_dialogue_context`、`recent_candidates` 与多步引用完成“先 search，再 details/list”的线性编排。

### 代码评审收口

- 已移除成功查询后基于旧 `RecentConfirmedEntity` 伪造 `turn.query_context.resolved` 的行为，避免把上一轮实体误记成“本轮已解析实体”。
- 已修复 `DecodeQueryClarification(...)` 对 `missing_params` 仅接受 `[]any` 的问题；当前同时兼容 `[]string` 与 `[]any`。
- 已重写 `QueryContextFromEvents(...)` 的 dialogue 构造逻辑：
  - 按事件正序扫描
  - `turn.agent_message.delta` 按 `message_id` 聚合
  - 在 `turn.agent_message.completed` 时落完整 assistant reply
  - `turn.query_clarification.requested` 不再重复塞入 `RecentDialogueTurns`

### 自动化验证

- 命令：`go test ./modules/cubebox ./internal/server`
- 结果：通过
- 覆盖重点：
  - `QueryContext` 扩展字段提取
  - 普通列表结果转 `recent_candidates`
  - `missing_params` `[]string` 解码
  - delta 聚合与澄清不重复
  - query flow 不再写 synthetic resolved-context event
  - 通用前序结果引用解析

- 命令：`make check doc`
- 结果：通过（`[doc] OK`）

## 当前结论

- `468 P0` 的代码闭环与自动化测试已完成。
- 当前剩余主要事项是 Slice D 的真实页面复验与会话样本证据补齐。

## 待补证据

- 真实 provider 环境下的同会话连续追问复验：
  - `系统里有哪些组织` -> `列出它的全部下级组织`
  - `请列出鲜花公司的全部下级组织，允许先按名称搜索定位该组织`
  - `查一下 100000 在 2026-04-25 的组织详情` -> `查该组织的下级组织` -> `那它的负责人呢`
- 页面截图、网络请求片段与 canonical event 样本。
