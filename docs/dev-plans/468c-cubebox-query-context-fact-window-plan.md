# DEV-PLAN-468C：CubeBox 查询上下文事实窗口扩展方案

**状态**: 规划中（2026-04-27 07:26 CST；`recent_*` 语义槽位与消极防御方向已由 `DEV-PLAN-473` 纠偏）

> 纠偏说明：本计划早期将 `recent_confirmed_entity`、`recent_candidates`、`recent_candidate_groups` 等设计为 prompt-facing 主事实窗口。该方向容易退化为本地“最近实体 / 最近候选 / target 绑定”状态机，也容易继续堆叠“不确定就本地防御”的保守分支。后续实施必须以 `DEV-PLAN-473` 为准：本地只提供中性的 `query_evidence_window`、工具目录和硬约束校验；模型负责语义续接、澄清恢复、日期补全、候选解释、显式 `ReadPlan` 参数和最终表达；执行器不得从 `recent_*` 字段隐式补 target。

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：承接 `DEV-PLAN-468 Slice E3 / P2-3c`，只扩展同一会话内的 query dialogue fact window，让 planner / clarifier / narrator 获得候选组、最近确认实体、最近问答和澄清状态等结构化事实。
- **关联模块/目录**：`modules/cubebox/query_entity.go`、`internal/server/cubebox_query_flow.go`、`modules/orgunit/presentation/cubebox/*`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-003`、`DEV-PLAN-012`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`、`DEV-PLAN-464`、`DEV-PLAN-468`、`DEV-PLAN-470`
- **用户入口/触点**：主应用壳层右侧 `CubeBox` 抽屉、`/internal/cubebox/turns:stream`、查询 planner、查询 clarifier、查询 narrator

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理同一会话内的短程查询事实窗口；不建设长期记忆、FactSet 平台、页面事实协议、第二套权限系统或第二套查询 endpoint。
2. **不变量**：候选组、最近实体、最近问答和澄清状态只是模型输入事实，不是授权来源，不声明权限，不扩大可执行 API，不绕过 `ReadPlan` schema、执行注册表、tenant/RLS/session/principal 边界。
3. **可解释**：reviewer 必须能在 5 分钟内说明：代码只从 canonical events 提取、限量并装配查询事实；模型负责判断当前问题是否引用上一轮实体、候选组、最近问答或澄清状态。

## 1. 背景

`DEV-PLAN-468` 已完成 `P2-3a` 与 `P2-3b`：

1. `P2-3a` 已放开 narrator 固定短答、业务字段名禁忌和本地字段白名单式裁剪。
2. `P2-3b` 已把 ambiguity clarification 从本地 prose owner 改为结构化候选事实输入。

当前剩余的 `P2-3c` 不再处理 `page_context`。`page_context` 已经被重新评估为当前范围外，并由 `DEV-PLAN-470` 承接剔除与清理。

本计划只处理 query dialogue fact window 的真实缺口：

1. `recent_candidates` 只暴露最后一组候选，无法稳定处理“第一个”“最开始那个”“不是这个，另一个”等跨候选组指代。
2. `recent_confirmed_entity` 虽已有 `recent_confirmed_entities` 补充，但仍容易被知识包或调用方当作 privileged winner。
3. `last_clarification`、`recent_dialogue_turns` 与候选事实没有形成清晰的 prompt-facing 事实窗口规则；`resolved_entity` 当前没有稳定 writer，不进入本计划事实窗口。
4. 467 真实失败场景需要继承上一轮已确认实体与日期，不能依赖页面事实。

## 2. 问题定义

### 2.1 候选窗口丢失分组

`QueryContextFromEvents(...)` 内部可收集多组候选，但当前 prompt-facing context 只暴露最后一组 `recent_candidates`。这会丢失候选来源、轮次和分组顺序，模型无法可靠解析“最开始那个”“上一轮第二个”“不是这个，另一个”。

### 2.2 最近确认实体仍像语义 owner

`recent_confirmed_entity` 是兼容访问器，不应继续代表代码替模型选定当前轮引用对象。主输入应改为有序事实窗口：最近确认实体列表、候选组、最近问答、澄清状态与当前用户输入。

### 2.3 澄清状态没有足够上下文

`LastClarification` 已包含 `missing_params`、`error_code`、`candidate_source`、`candidate_count` 和 `cannot_silent_select`，但模型看到的事实仍偏扁平。后续应让 planner / clarifier 能够知道候选来自哪一次澄清、是否禁止静默选择、用户后续是否在选择候选。

### 2.4 `resolved_entity` 退出当前范围

`resolved_entity` 当前没有冻结“谁写、何时写、优先级低于什么”，且现有 API 测试明确禁止由成功查询后旧实体伪造 synthetic resolved event。为避免它重新长成第二个 privileged winner，本计划不引入、不消费、不验收 `resolved_entity`；若后续确需引入，必须单独开 owner 先冻结 writer、写入时机和优先级。

### 2.5 页面事实退出当前范围

`page_context` 不解决 467 的主问题，且容易把页面当前对象误当成用户当前意图。当前实施不再扩展、不再接入 planner，并按 `DEV-PLAN-470` 完成清理。

## 3. 目标

1. [ ] 引入 `recent_candidate_groups`，保留最近若干候选组及其来源、顺序和候选项。
2. [ ] 将 `recent_confirmed_entity` 明确降级为 compatibility accessor；主语义输入转向 `recent_confirmed_entities`。
3. [ ] 统一 planner、clarifier、narrator 的 `dialogue_context` 事实窗口，避免各自裁剪出互相矛盾的上下文。
4. [ ] 更新知识包中的连续追问与候选选择规则，移除 privileged winner 倾向。
5. [ ] 减少组织查询域内因会话事实缺失导致的 `NO_QUERY` 误用；缺参继续走现有 `missing_params + clarifying_question`。

## 4. 非目标

1. 不实现、不扩展、不保留 `page_context`；相关剔除 owner 为 `DEV-PLAN-470`。
2. 不新增 planner 状态机，不引入 `pass_through`、`unsupported_query`、`need_clarification` 等新 envelope。
3. 不新增写能力，不新增模型工具调用，不允许模型直查 DB、拼 SQL 或调用未登记接口。
4. 不把知识包扩展成回答模板库、prose 模板库或权限声明来源。
5. 不建设通用 FactSet 平台、展示 DTO 平台、字段投影平台或跨会话记忆。
6. 不引入、不消费、不验收 `resolved_entity`；`QueryContextResolvedEventType` 不作为本计划交付物。
7. 不把候选组、最近问答或澄清状态作为结果事实源；narrator 的事实性业务结论仍只能来自执行后的 `results`。

## 5. 设计方案

### 5.1 query context 候选组

新增 prompt-facing 事实窗口：

```json
{
  "recent_candidate_groups": [
    {
      "group_id": "candgrp_<opaque>",
      "candidate_source": "execution_error",
      "candidate_count": 2,
      "cannot_silent_select": true,
      "candidates": [
        {
          "domain": "orgunit",
          "entity_key": "200000",
          "name": "飞虫公司",
          "as_of": "2026-04-25",
          "status": "active"
        }
      ]
    }
  ],
  "recent_candidates": []
}
```

规则：

1. `turn.query_candidates.presented` 的权威关联键只有 `group_id`。
2. `group_id` 使用 opaque 值，例如 `candgrp_<opaque>`；planner / clarifier / narrator 不得解析其内部结构。
3. `candidate_source` 只是描述字段，用于解释候选来源，不参与关联。
4. `turn_id` 不再作为可选主关联键；如事件 envelope 中存在 turn 信息，也只能用于审计或排查，不能作为 prompt-facing 关联规则。
5. `recent_candidate_groups` 最多保留最近 5 组。
6. 每组候选最多保留 100 个，planner 默认直接消费 canonical 上限，narrator / clarifier 可再按输入预算裁剪。
7. `recent_candidates` 保留为兼容字段，必须始终表示当前 projection 中最后一组候选的扁平 alias。
8. 写入 `turn.query_candidates.presented` metadata event 时必须补 `group_id`、来源、数量、不可静默选择和候选项。
9. 模型可用候选组解析“第一个”“第二个”“最开始那个”“上一轮那个”“不是这个，另一个”。

冻结事件 payload：

```json
{
  "group_id": "candgrp_<opaque>",
  "candidate_source": "execution_error",
  "candidate_count": 2,
  "cannot_silent_select": true,
  "candidates": []
}
```

### 5.2 `recent_confirmed_entity` 降级

1. 保留 `RecentConfirmedEntity` 字段，避免一次性大范围破坏调用方。
2. prompt block 中标记它为 compatibility accessor。
3. 主字段改为 `recent_confirmed_entities`。
4. 知识包不得再写“优先继承最近已确认实体”这类 privileged winner 规则；应改成“当前输入优先，模型从有序事实窗口判断引用对象”。

### 5.3 最近问答与澄清状态

1. `recent_dialogue_turns` 继续保留最近 5 轮轻量问答。
2. `last_clarification` 保留最近一次澄清事实：
   - `intent`
   - `missing_params`
   - `clarifying_question`
   - `error_code`
   - `candidate_group_id`
   - `candidate_source`
   - `candidate_count`
   - `cannot_silent_select`
3. `turn.query_clarification.requested` 可选携带 `candidate_group_id`，用于关联同一轮或最近一次 `turn.query_candidates.presented`。
4. `QueryClarification` 新增 `CandidateGroupID string`。
5. 若澄清与候选组来自同一执行错误，只能通过 `candidate_group_id == recent_candidate_groups[*].group_id` 建立可解释关联。

冻结可选 payload 扩展：

```json
{
  "candidate_group_id": "candgrp_<opaque>"
}
```

### 5.4 canonical 存储视图与 projection 视图

`QueryContext` 是 canonical storage view，语义完整、边界固定；planner / clarifier / narrator 看到的是 projection view，字段形状必须一致，只允许按预算裁剪。

canonical 上限：

1. `recent_confirmed_entities` 最多 5 项。
2. `recent_candidate_groups` 最多 5 组。
3. 每组 `candidates` 最多 100 项。
4. `recent_dialogue_turns` 最多 5 轮。

consumer projection 上限：

1. planner：直接消费 canonical 上限。
2. clarifier：最多 2 组候选、每组最多 20 项、最近 2 轮 dialogue。
3. narrator：最多 1 组候选、最多 5 项、最近 2 轮 dialogue。

关键约束：

1. 字段语义不能因 consumer 改变。
2. `recent_candidates` 必须始终是当前 projection 中最后一组的扁平 alias。
3. `recent_confirmed_entity` 必须始终是当前 projection 中最后一项 confirmed entity 的 alias。
4. projection helper 只能裁剪数量，不得重排、不生成新事实、不合并候选组。

### 5.5 planner 输入规则

`buildPlannerMessages(...)` 继续注入 `query_dialogue_context`，但 block 内容应调整为：

1. 明确当前用户输入优先于历史事实。
2. `recent_confirmed_entities`、`recent_candidate_groups`、`recent_dialogue_turns`、`last_clarification` 共同构成事实窗口。
3. `recent_confirmed_entity` 只是 `recent_confirmed_entities` 最后一项的 compatibility alias，不是唯一 winner。
4. `recent_candidates` 只是 `recent_candidate_groups` 最后一组的 compatibility alias，不是候选主来源。
5. 序号型、否定型、跨组引用必须优先读取 `recent_candidate_groups`。
6. 模型应自行判断“该组织”“第一个”“最开始那个”“不是这个，另一个”指向哪一项；无法稳定判断时输出澄清型 `ReadPlan`。
7. 不允许从自然语言 summary、失败查询或普通 assistant reply 猜测实体。

planner JSON 前必须增加固定 contract 文本：

```text
查询会话上下文契约：
- 当前轮用户显式输入优先于历史事实。
- recent_confirmed_entity 只是 recent_confirmed_entities 最后一项的兼容别名。
- recent_candidates 只是 recent_candidate_groups 最后一组的兼容别名。
- 序号型、否定型、跨组引用必须优先读取 recent_candidate_groups。
- 无法稳定判断引用对象时，输出澄清型 ReadPlan。
```

### 5.6 clarifier / narrator 对齐

1. clarifier 输入保持 `user_prompt + dialogue_context + candidates`。
2. narrator 输入保持 `user_prompt + dialogue_context + results`。
3. narrator 可以用对话事实解释“刚才那个/继续查”的衔接关系。
4. narrator 的事实性业务结论只能来自 `results`，不得从候选组或最近问答推导结果中不存在的字段、状态、层级或条数。

### 5.7 知识包调整

`modules/orgunit/presentation/cubebox/*` 应同步调整：

1. 只保留查询面、参数语义、字段含义、安全默认值、候选处理与澄清边界。
2. 删除页面事实补参规则。
3. 主规则只写 `recent_confirmed_entities` / `recent_candidate_groups`。
4. `recent_confirmed_entity` / `recent_candidates` 只能出现在 compatibility 说明里。
5. 增加候选组解析规则：序号、名称、最早/最近、否定后选择另一个。
6. 示例 10 改为消费 `recent_candidate_groups`，不得再使用裸 `recent_candidates` 作为主输入。
7. 增加一个“最开始那个 / 不是这个，另一个”的跨组示例。
8. 删除回答模板、固定 prose 和“最近确认实体单点优先”的倾向。
9. 明确知识包不是授权来源，也不是执行注册表事实源。

## 6. 安全边界与 Stopline

1. [ ] 未注册 `executor_key` 不能执行。
2. [ ] `ReadPlan` schema、参数白名单、必填/可选参数、类型、日期、枚举与前序结果引用仍必须 fail-closed。
3. [ ] tenant 隔离、RLS、当前 session、当前 principal 与审计归属不放松。
4. [ ] 知识包、模型输出、候选组、最近问答和历史会话都不能声明或扩大权限。
5. [ ] 用户可见输出不能原样泄露 raw JSON、`executor_key`、`payload`、`results`、`step-*`、执行计划结构、密钥或 provider 配置。
6. [ ] 不得把候选组、最近问答或澄清状态当作执行结果事实源。
7. [ ] 不得用本地关键词补丁替代模型对事实窗口的语义选择。
8. [ ] 不得在本计划中恢复或换名引入 `page_context`。

## 7. 实施步骤

1. [ ] 冻结 query dialogue fact window 契约：`recent_candidate_groups`、`recent_confirmed_entities`、`recent_dialogue_turns`、`last_clarification` 与兼容字段策略。
2. [ ] 引入 `QueryCandidateGroup` 与 `RecentCandidateGroups`，字段包含 `GroupID`、`CandidateSource`、`CandidateCount`、`CannotSilentSelect`、`Candidates`。
3. [ ] `QueryClarification` 新增 `CandidateGroupID string`，并从 `candidate_group_id` payload 解码。
4. [ ] 调整 `QueryContextFromEvents(...)`，从 canonical events 提取最近若干候选组并限量，`RecentCandidates` 始终指向当前 projection 最后一组。
5. [ ] 调整 ambiguous search metadata writer：`turn.query_candidates.presented` 写出 `group_id`；对应 `turn.query_clarification.requested` 写出 `candidate_group_id`。
6. [ ] 新增 projection helper，统一 planner / clarifier / narrator 的预算裁剪；helper 只允许裁剪数量，不得改变字段语义。
7. [ ] 调整 planner prompt block，增加固定 compatibility contract 文本，并输出 `recent_candidate_groups`。
8. [ ] 调整 narration / clarification envelope，按 projection 预算传候选组和最近问答，同时保留 narrator 只从 `results` 下事实结论的约束。
9. [ ] 从 468C 当前实现范围移除 `resolved_entity`：planner block、projection 和验收不得依赖它；不得新增 `QueryContextResolvedEventType` writer。
10. [ ] 更新 `orgunit` 知识包，移除 privileged winner 与回答模板倾向，补充候选组解析规则，并删除页面补参规则。
11. [ ] 回写 `DEV-PLAN-468` 中 `P2-3c` owner 指向本计划，避免双 owner。
12. [ ] 执行并登记验证命令与真实页面复验结果。

## 8. 验收场景

1. [ ] `turn.query_candidates.presented` 写出的候选事件包含 `group_id`，且 `candidate_source` 不参与关联。
2. [ ] `turn.query_clarification.requested` 在候选澄清场景写出 `candidate_group_id`。
3. [ ] 连续候选组中，用户输入“第一个”“最开始那个”“不是这个，另一个”时，planner 能看到候选分组事实。
4. [ ] planner / clarifier / narrator 看到的 projection 字段语义一致，只发生预算裁剪。
5. [ ] `recent_candidates` 始终是当前 projection 中最后一组候选的扁平 alias。
6. [ ] `recent_confirmed_entity` 始终是当前 projection 中最后一项 confirmed entity 的 alias。
7. [ ] `resolved_entity` 不进入 468C prompt-facing 事实窗口，不生成 `QueryContextResolvedEventType`。
8. [ ] 知识包主规则段落不再把 `recent_candidates` 写成一等来源。
9. [ ] `recent_confirmed_entity` 出现时必须带 compatibility 语义。
10. [ ] `100000 -> 查该组织的下级组织` 继续由 `query_dialogue_context` 继承实体与日期，不依赖页面事实。
11. [ ] narrator 可自然承接上下文，但事实性结论仍只来自 `results`。
12. [ ] `rg -n "page_context|PageContext|页面事实|当前页面上下文补|页面上下文|页面补参|current page|page context" modules/orgunit/presentation/cubebox` 不命中当前知识包；本计划文件中仅允许保留“范围外/不得恢复”的说明。

## 9. 测试与覆盖率

### 9.1 覆盖率口径

本计划不调整仓库覆盖率门禁、不新增 coverage 排除项、不降低阈值。覆盖率与测试分层继续遵循 `AGENTS.md`、`DEV-PLAN-300`、`DEV-PLAN-301`、`DEV-PLAN-304`。

### 9.2 最小测试范围

Go：

1. [ ] `modules/cubebox/query_entity_test.go`
   - 多候选组提取与 5 组上限。
   - 每组 100 项上限。
   - `candidate_group_id` 关联澄清。
   - `RecentCandidates` alias 正确。
   - `RecentConfirmedEntity` alias 正确。
2. [ ] `internal/server/cubebox_query_flow_test.go`
   - planner block 包含 `recent_candidate_groups`。
   - contract 文本包含 compatibility 说明。
   - clarifier projection 最多 2 组候选、每组 20 项、最近 2 轮 dialogue。
   - narrator projection 最多 1 组候选、最多 5 项、最近 2 轮 dialogue。
3. [ ] `internal/server/cubebox_api_test.go`
   - `turn.query_candidates.presented` 写出 `group_id`。
   - `turn.query_clarification.requested` 写出 `candidate_group_id`。
   - 不生成 `QueryContextResolvedEventType`。

知识包：

1. [ ] `modules/orgunit/presentation/cubebox/*` 与 registry 校验仍通过。
2. [ ] 主规则只消费 `recent_confirmed_entities` / `recent_candidate_groups`。
3. [ ] `recent_confirmed_entity` / `recent_candidates` 只出现在 compatibility 说明里。
4. [ ] 示例覆盖候选组序号选择、跨组引用、否定后选择另一个、缺参澄清和 `NO_QUERY` 边界。
5. [ ] 示例不再覆盖页面事实补参。

### 9.3 建议验证命令

实际实施时按 `AGENTS.md` 触发器矩阵执行；本计划建议最小命令为：

1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] 文档变更执行 `make check doc`。

## 10. 交付物

1. [ ] query context 候选组事实窗口实现与测试。
2. [ ] `recent_confirmed_entity` 降级后的 prompt 与知识包说明。
3. [ ] `orgunit` 知识包规则更新。
4. [ ] `DEV-PLAN-468` / `AGENTS.md` 文档地图与 owner 回写。
