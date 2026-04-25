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
- 已完成 `Slice E / P1` 的 prompt-view 收口：
  - narrator 输入已收敛为安全 presentation DTO，并只保留 `user_prompt`、`page_context`、`dialogue_context`、`results`
  - clarifier 输入已只保留 `user_prompt`、`page_context`、`dialogue_context`、`candidates`
  - prompt-facing 实体上下文已压缩为最小实体视图，仅保留 `domain`、`entity_key`、`as_of`
  - 前端已向 `/internal/cubebox/turns:stream` 传入受控 `page_context`
  - 已修复 `page_context` 对 `/org/units/field-configs` 的详情页误判；固定子路由不再伪装为 `orgunit` 对象事实
  - 已修复组织详情页 `effective_date` 页面历史锚点漏传；当前前端优先读取 `effective_date`，兼容写回 `view.as_of`
  - 知识包 API/参数集合已与执行注册表做双向一致性校验
  - provider/runtime/model 元数据已从 provider prompt view 剥离，仅保留在 metadata、管理 UI 或日志
  - `page_context.view.as_of -> effective_date` DTO 改名已独立拆到 `DEV-PLAN-470`，不混入本次回归修复

### 代码评审收口

- 已移除成功查询后基于旧 `RecentConfirmedEntity` 伪造 `turn.query_context.resolved` 的行为，避免把上一轮实体误记成“本轮已解析实体”。
- 已修复 `DecodeQueryClarification(...)` 对 `missing_params` 仅接受 `[]any` 的问题；当前同时兼容 `[]string` 与 `[]any`。
- 已重写 `QueryContextFromEvents(...)` 的 dialogue 构造逻辑：
  - 按事件正序扫描
  - `turn.agent_message.delta` 按 `message_id` 聚合
  - 在 `turn.agent_message.completed` 时落完整 assistant reply
  - `turn.query_clarification.requested` 不再重复塞入 `RecentDialogueTurns`
- 已把 narrator/clarifier 的 owner 边界继续回交模型：
  - 共享 query flow 不再拼模块专属候选澄清文案，改为把候选列表交给 clarifier 生成自然追问
  - 共享层不再为 narrator 生成结果总结；模块侧只产安全 DTO，最终自然语言组织完全回交 narrator
  - narrator 不再接收 `plan`
  - clarifier 不再接收 `query_intent`

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
  - narrator/clarifier 输入 DTO 收口与轻量上下文注入
  - prompt-facing 实体上下文不再泄露 `intent`、`source_api_key`、`target_org_code`、`parent_org_code`
  - narrator 不再接收 `plan`
  - clarifier 不再接收 `query_intent`

- 命令：`make check doc`
- 结果：通过（`[doc] OK`）

## 当前结论

- `468 P0` 的代码闭环与自动化测试已完成。
- `468 Slice D` 的真实页面复验与会话样本证据已补齐。
- `468 Slice E / P1` 的表达层、DTO 与 prompt-view 收口已完成；当前剩余主线为 `P2/后续 owner`。
- `468 Slice E / P1` 评审发现的前端页面事实误判、历史日期漏传与知识包-注册表单向校验残口已完成修复。

## 真实页面复验

- 状态：通过（2026-04-25 17:53 CST）。
- 环境：
  - 运行时通过 `tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh --verify-cubebox` 启动。
  - 先执行 `pnpm build` 与 `make css`，确保 `apps/web` 变更已同步到 `internal/server/assets/web` 的嵌入式产物后，再启动新 server 进程。
  - 浏览器脚本：`.local/dev-plan-468-revalidate.js`（临时脚本，不入仓）。
- 快捷键验证：
  - 仅纯 `Enter` 发送；带修饰键的 `Ctrl/Shift/Alt/Meta+Enter` 不发送请求。
  - 复验样本中 `Ctrl+Enter` 未发送，输入框内容保持为单行文本 `快捷键验证第一行`。
  - `Enter` 成功发送，`/internal/cubebox/turns:stream` 请求体为 `{"conversation_id":"conv_2303dc8cb9c94a528f69411192a45ad4","prompt":"快捷键验证第一行","next_sequence":2}`，响应 `200`。
- 链路 A：`系统里有哪些组织` -> `列出它的全部下级组织`
  - 会话：`conv_b3f8141f05094b33966f813ad2b63709`
  - 第一轮结果：页面返回 `截至 2026-04-25，系统里有 1 个组织：组织 100000“飞虫与鲜花”，当前为启用状态，并且还有下级组织。`
  - 第二轮结果：页面直接继承上一轮已返回的 `100000`，返回 `截至 2026-04-25，组织 100000 的下级组织有 2 个：飞虫公司（200000，启用，仍有下级）和鲜花公司（300000，启用，没有下级）。`
  - 结论：同会话内已出现并已返回给用户的组织，不再机械要求补 `org_code`。
- 链路 B：`请列出鲜花公司的全部下级组织，允许先按名称搜索定位该组织`
  - 会话：`conv_8e622a151785484585d23438a741bc91`
  - 页面结果：`截至 2026-04-25，按名称已定位到“鲜花公司”（组织编码 300000）。在系统中它当前没有记录任何启用状态的直接下级组织。`
  - 结论：只有名称时，模型已先 search 唯一命中 `300000` 再继续执行，不再回退为业务专用澄清。
- 链路 C：`查一下 100000 在 2026-04-25 的组织详情` -> `查该组织的下级组织` -> `那它的负责人呢`
  - 会话：`conv_e759a9389bcf4e529a909a1aa9d2ffa6`
  - 第一轮结果：页面返回 `截至 2026-04-25，组织 100000 名称为“飞虫与鲜花”，当前状态为启用，且属于业务单元。它的组织全路径为“飞虫与鲜花”。系统未记录其上级组织信息和负责人信息，也没有扩展字段。`
  - 第二轮结果：页面继承 `org_code=100000` 与 `as_of=2026-04-25`，返回 2 个下级组织：`200000「飞虫公司」`、`300000「鲜花公司」`。
  - 第三轮结果：页面继续指代同一组织，返回 `截至 2026-04-25，组织“飞虫与鲜花”（100000）的负责人信息系统里未记录：负责人姓名和工号都没有填写。`
  - 结论：第二轮未重复追问 `parent_org_code`，第三轮可继续复用同一查询锚点。
- unsupported query：
  - 会话：`conv_e7106a0f8af34a89af68471ce40cdd05`
  - 输入：`帮我查一下今天上海天气`
  - 页面结果：`当前请求未形成可安全执行的只读查询计划。请换成明确的数据查询问题，或补充查询对象、条件和日期后重试。`
  - 结论：unsupported domain 仍保持 fail-closed；未掉回“没有查询接口/权限”或普通对话链。

## 证据清单

- 结构化结果：`docs/dev-records/assets/dev-plan-468/slice-d-revalidation.json`
- 网络请求：`docs/dev-records/assets/dev-plan-468/network.txt`
- 截图：
  - `docs/dev-records/assets/dev-plan-468/slice-d-s1-roots-children.png`
  - `docs/dev-records/assets/dev-plan-468/slice-d-s2-name-search.png`
  - `docs/dev-records/assets/dev-plan-468/slice-d-s3-details-children-manager.png`
  - `docs/dev-records/assets/dev-plan-468/slice-d-s4-unsupported.png`
- canonical event 片段（摘自 `slice-d-revalidation.json` 的回放结果）：
  - 链路 A：
    - `sequence=10 type=turn.query_entity.confirmed`，payload 确认 `entity_key=100000`、`intent=orgunit.list`、`parent_org_code=100000`
    - `sequence=11 type=turn.query_candidates.presented`，payload 包含 `200000 飞虫公司`、`300000 鲜花公司`
  - 链路 B：
    - `sequence=4 type=turn.query_entity.confirmed`，payload 确认 `entity_key=300000`、`intent=orgunit.search_then_list`
    - `sequence=5 type=turn.query_candidates.presented`，payload 唯一候选为 `300000 鲜花公司`
  - 链路 C：
    - `sequence=4 type=turn.query_entity.confirmed`，payload 确认 `entity_key=100000`、`intent=orgunit.details`
    - `sequence=10 type=turn.query_entity.confirmed`，payload 确认 `entity_key=100000`、`intent=orgunit.list`
    - `sequence=17 type=turn.query_entity.confirmed`，payload 再次确认 `entity_key=100000`、`intent=orgunit.details`

## 待补证据

- 无。Slice D 真实页面复验、截图、网络请求片段与 canonical event 样本已完成登记。
