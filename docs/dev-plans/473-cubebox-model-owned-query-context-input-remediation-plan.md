# DEV-PLAN-473：CubeBox 模型主导查询链与消极防御收敛纠偏方案

**状态**: 实施完成（2026-04-27；已落地 `query_evidence_window`，planner / clarifier / narrator 均改为消费中性事实窗口，旧 `recent_*` 不再进入 prompt-facing 主契约）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：专门纠偏 CubeBox 查询链中“最近确认实体 / 最近候选列表 / 业务 target 绑定”以及同类消极防御式本地规则堆砌，把责任收敛为“默认交给模型判断、编排和恢复；本地提供事实、工具边界、硬约束校验和执行”。
- **关联模块/目录**：`internal/server/cubebox_query_flow.go`、`modules/cubebox/*`、`modules/orgunit/presentation/cubebox/*`、`docs/dev-plans/468c-cubebox-query-context-fact-window-plan.md`、`docs/dev-plans/472-cubebox-clarification-slot-repair-and-partial-date-continuity-plan.md`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`、`docs/dev-plans/464-cubebox-query-architecture-convergence-plan.md`、`docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md`、`docs/dev-plans/468c-cubebox-query-context-fact-window-plan.md`、`docs/dev-plans/471-cubebox-intra-turn-iterative-read-planning-plan.md`、`docs/dev-plans/472-cubebox-clarification-slot-repair-and-partial-date-continuity-plan.md`
- **用户入口/触点**：CubeBox 抽屉、`/internal/cubebox/turns:stream`、查询 planner、只读 executor、narrator

### 0.1 Simple > Easy 三问

1. **边界**：本计划只整改查询链的 owner 分工；不建设本地 NLU、slot repair engine、target cache、候选集合状态机、防御性关键词分支、跨会话记忆或第二套查询 endpoint。
2. **不变量**：模型拥有当前用户输入的语义理解、目标选择、澄清恢复、日期补全、候选集合解释和多步只读编排权；本地只提供事实、工具目录、硬约束校验和执行。所有可执行 target 必须来自模型输出的显式 `ReadPlan` 参数并通过校验。
3. **可解释**：reviewer 必须能在 5 分钟内说明：模型拿到了哪些事实和工具；模型输出了什么计划或回复；本地只在哪些硬边界拦截；没有哪个本地分支替模型决定“用户指的是谁、缺什么、该不该继续”。

### 0.2 本计划的扩大纠偏结论

本计划不仅纠偏 `recent_*` 状态槽，也纠偏更普遍的实现倾向：遇到模型不稳或样本失败时，在本地增加一层消极防御、保守拦截、关键词判断、slot 修补或兜底分支。

这类做法的问题是：

1. 它把模型应该承担的语言理解和任务编排拆回 Go 代码。
2. 它会把每个失败样本变成一个局部补丁，最后形成不可解释的状态机。
3. 它降低了模型获得信息后自行恢复的机会，导致系统越来越像表单机器人。
4. 它把“安全边界”和“不信任模型”混在一起；真正必须本地拦截的是权限、租户、只读、schema、预算、日期合法性等硬约束，而不是自然语言理解。
5. 它会让 prompt、executor、narrator、server 各自保留一份判断逻辑，形成多事实源。

默认原则调整为：

> 只要模型拥有足够上下文和受控工具，就优先让模型判断和继续编排；本地只在硬约束失败时拒绝或要求模型重试，不因为“可能理解错”而预先增加消极防御分支。

### 0.3 `recent_*` 只是本原则的一个症状

`recent_confirmed_entity`、`recent_candidates`、`recent_candidate_groups`、`target_binding` 这类设计容易把 CubeBox 拉回本地硬编码对话状态机：

1. “最近”不是用户意图，只是时间顺序启发式。
2. “确认实体”会变成 privileged winner，覆盖当前用户输入和更近的对话关系。
3. “候选列表”若作为本地状态，会诱导代码解析“以上全部 / 第一个 / 另一个 / 审计信息”等自然语言。
4. “target binding”一旦在本地隐式发生，模型、用户和测试都很难看见为什么绑定到了某对象。
5. 这些槽位会制造 transcript、工具结果、摘要之外的第二事实源。

因此，整改方向不是把这些槽位做得更完整，也不是给它们叠更多“不确定就澄清”的防御规则，而是删除它们的语义 owner 地位：本地只构造中性的上下文事实输入；模型根据这些事实输出显式计划、继续追问或最终回答；执行器只按硬约束执行或拒绝。

### 0.4 Codex 对标边界

本地第三方 Codex 源码的可借鉴点是通用 agent runtime 形态，而不是业务实体状态：

- `third_party/openai-codex/codex-rs/core/src/context_manager/history.rs` 维护的是 transcript 历史项，不维护业务实体 winner。
- `third_party/openai-codex/codex-rs/core/src/session/turn_context.rs` 维护的是运行环境、模型、权限、cwd、日期等 turn context，不维护业务查询 target。
- `third_party/openai-codex/codex-rs/protocol/src/models.rs` 中工具调用通过显式 `arguments` 传参；runtime 不从“最近对象”隐式补工具参数。

CubeBox 不能照搬 Codex 的 shell/tool 能力，但应借鉴这个边界：上下文是模型输入，工具参数是显式输出，执行器只负责受控执行。Codex 没有为每类自然语言失败增加本地防御状态机；它把历史和工具结果交回模型，让模型继续工作。

## 1. 背景与失败样本

真实会话中出现过以下漂移：

1. 用户先查“财务”相关候选，系统给出 `财务部(200001)`、`财务一组(200002)`、`财务四组(200004)`。
2. 用户说“以上全部”，随后说“审计信息”。
3. 系统却返回了 `成本B组(200006)` 的审计信息。

这不是缺少一个更强的 `last_candidate_list`，而是查询链没有把“上一轮候选、用户选择全部、当前审计意图”作为模型可消费的上下文事实稳定输入；同时本地隐式状态或历史裁剪把目标漂移到了无关的最近结果。

另一个风险样本是：用户前面问过“根组织的审计信息”，后面又问“审计信息”。系统不应靠“最早/最近确认实体”抢答，而应让模型看到当前上下文后决定：

- 当前输入是否仍延续根组织审计；
- 是否已有更新的用户选择覆盖旧范围；
- 是否需要澄清。

## 2. 目标

1. [X] 冻结“模型主导查询链”原则：默认让模型处理语义续接、目标选择、澄清恢复、日期补全、候选解释、多步只读编排和最终表达。
2. [X] 将 `468C` 中“recent confirmed / recent candidate groups 作为主语义输入”的方向降级或替换为中性 `query_evidence_window`。
3. [X] 从 planner prompt、知识包和 query flow 中移除 privileged winner、关键词防御、slot repair、静态澄清模板和本地语义兜底。
4. [X] 确保 executor 不从本地上下文隐式补 `org_code`、`as_of`、审计对象或集合 target，只校验模型输出的显式参数。
5. [X] 对真实失败样本补回归测试：模型可根据上下文输出显式多目标审计计划、继续澄清或通过 loop 追加查询；本地不得抢先把请求降级为保守失败。
6. [X] 建立轻量反回流检查：以测试和评审清单为主，不首期新增全仓永久门禁。

## 3. 非目标

1. 不新增本地 NLU、关键词解析、slot repair engine、target cache、候选集合状态机或防御性意图分类器。
2. 不新增 `resolved_entity`、`selected_target`、`target_binding`、`current_working_set` 这类业务 winner 字段。
3. 不把“模型可能不稳”作为新增 Go 分支、硬编码澄清、局部 fallback 或静态回答模板的理由。
4. 不恢复 `page_context` 作为补参来源。
5. 不建设通用 FactSet 平台、长期记忆、跨会话用户画像或摘要事实数据库。
6. 不新增 orgunit 专用递归 API、审计 fanout API 或绕过 `ExecutionRegistry` 的快捷分支。
7. 不把模型摘要、assistant prose 或失败消息当作执行事实源；它们只能作为模型输入参考。

## 4. 架构原则

### 4.1 上下文输入是 evidence，不是 state owner

新增或重构 planner 输入时，推荐使用中性事实窗口：

```json
{
  "query_evidence_window": {
    "current_user_input": "审计信息",
    "recent_turns": [
      {
        "user_prompt": "财务",
        "assistant_reply": "展示了 3 个可选组织：财务部(200001)、财务一组(200002)、财务四组(200004)。"
      },
      {
        "user_prompt": "以上全部"
      }
    ],
    "observations": [
      {
        "source": "query_event",
        "kind": "presented_options",
        "result_summary": {
          "group_id": "candgrp_finance",
          "option_source": "execution_error",
          "item_count": 3,
          "requires_explicit_user_choice": true,
          "items": [
            { "domain": "orgunit", "entity_key": "200001", "name": "财务部", "as_of": "2026-01-07" },
            { "domain": "orgunit", "entity_key": "200002", "name": "财务一组", "as_of": "2026-01-07" },
            { "domain": "orgunit", "entity_key": "200004", "name": "财务四组", "as_of": "2026-01-07" }
          ]
        }
      }
    ],
    "open_clarification": {
      "reply_candidate": true,
      "option_group_id": "candgrp_finance",
      "option_source": "execution_error",
      "option_count": 3,
      "requires_explicit_user_choice": true,
      "raw_user_reply": "以上全部"
    }
  }
}
```

关键约束：

1. `query_evidence_window` 只能表达“发生过什么、工具返回了什么、用户说过什么”。
2. 不出现 `confirmed`、`selected`、`target_binding`、`winner` 这类本地语义裁决字段。
3. `items` 是工具 observation 的摘要，不是本地候选状态机。
4. 当前用户输入必须与历史事实同屏提供给模型，由模型判断引用关系。
5. 上下文可裁剪，但不得重排、合并、改写为本地推断结论。

### 4.2 模型拥有语义判断和继续编排权

planner system prompt 必须明确：

1. 当前用户输入优先于历史事实。
2. 历史事实只是 evidence，不是 target 绑定。
3. 若用户说“以上全部 / 第一个 / 它 / 审计信息”等短语，模型应基于 `recent_turns + observations + open_clarification` 判断引用对象。
4. 模型可以主动选择继续查询、继续澄清、修正前一轮理解或给出最终回答；本地不在 planner 前用保守规则截断这些选择。
5. 若判断目标明确，输出显式 `ReadPlan` 参数，例如明确的 `org_code` 或 `org_codes`。
6. 若确实缺少执行所需事实，模型输出澄清；澄清问题由模型生成，本地不硬编码入口级选项列表。

### 4.3 本地硬约束边界

本地只在以下硬约束上拦截：

1. JSON / `ReadPlan` schema 非法。
2. capability 未登记或不是只读能力。
3. 参数类型非法、日期不是合法 `YYYY-MM-DD`、集合规模超预算。
4. tenant / session / principal / RLS / authz 不满足。
5. planner loop 超过轮次、步骤、token 或重复查询预算。

本地不因为以下理由拦截或改写：

1. 用户输入太短，例如“全部”“审计信息”“1日”。
2. 历史里存在多个可能对象。
3. 模型可能选择错对象。
4. 模型可能需要再查一步才能确认。
5. narrator 可能表达不够好。

### 4.4 Executor owns validation, not interpretation

执行器只做以下事情：

1. 校验 `ReadPlan` schema。
2. 校验 capability 是否在注册表内。
3. 校验参数类型、日期合法性、租户、权限、RLS、预算和只读边界。
4. 执行已登记只读 API。

执行器禁止：

1. 从 `query_evidence_window` 自动补 target。
2. 从 `recent_turns` 解析“以上全部”。
3. 从上一次工具结果中挑选默认 `org_code`。
4. 在审计、详情、下级组织之间做本地 intent/target 绑定。

### 4.5 Narrator 默认交给模型表达

narrator 可以使用 evidence 解释“为什么这是接着刚才的范围查”，并应由模型组织用户可见表达。本地不再堆叠静态回答模板、入口菜单或“当前主要支持...”类保守兜底，除非是明确的硬边界错误。事实性业务结论只能来自本轮执行结果；若执行结果里没有某个组织的审计记录，narrator 不得从历史候选、assistant prose 或模型推理补出结论。

## 5. 整改步骤

### 5.1 P0：文档纠偏与实施止血

1. [X] 在 `DEV-PLAN-468C` 顶部标记：`recent_confirmed_entity`、`recent_candidate_groups` 等语义槽位方向已由本计划纠偏，未完成代码不得按该方向继续实施。
2. [X] 更新 `DEV-PLAN-472` 引用关系：澄清续接继续保留“模型 owner 回正”，但不得依赖 `recent_*` 作为主语义输入。
3. [X] 更新知识包实施说明，删除“优先使用最近确认实体/最近候选”和“不确定就返回入口菜单”的措辞。
4. [X] 以 `rg` 盘点当前代码、prompt、测试中的以下词：`recent_confirmed_entity`、`recent_candidates`、`recent_candidate_groups`、`resolved_entity`、`target_binding`、`selected_target`。
5. [X] 形成保留/删除/兼容迁移清单；兼容字段只能作为旧测试或旧序列化读取过渡，不得进入新 planner prompt。

### 5.2 P1：构造中性 evidence window

1. [X] 在 `modules/cubebox` 下沉 `query_evidence_window` projection helper。
2. [X] 输入来源限定为 canonical conversation events、用户文本、assistant 文本摘要、工具结果摘要、open clarification 输入事实；本次不把工具调用参数作为独立执行事实暴露，以避免参数回灌为 target 绑定。
3. [X] 投影 helper 只裁剪数量和长度，不推断 target，不生成 winner，不做自然语言解析。
4. [X] 默认窗口控制在最近 5 个相关 turn、planner observation 最多 5 个实体事实和 5 组选项事实、每组选项最多 100 个轻量 item；clarifier / narrator 使用更小窗口。
5. [X] 明确 `query_evidence_window` 是 prompt input，不是 executor state。

### 5.3 P2：重写 planner prompt 与知识包规则

1. [X] planner prompt 改为消费 `query_evidence_window`。
2. [X] 删除 `recent_confirmed_entity` / `recent_candidates` 的主语义说明。
3. [X] 增加固定契约文本：

```text
查询上下文契约：
- query_evidence_window 只是历史事实与工具 observation，不是本地目标绑定。
- 当前用户输入优先；模型负责判断是否引用历史事实、是否需要继续查询、是否需要澄清。
- 如果目标明确，输出显式 ReadPlan 参数。
- 如果缺少执行所需事实，由模型生成澄清问题。
- 如果已有结果足够，由模型生成最终回答。
- 本地不会替你从历史上下文补 target，也不会因为输入短而抢先拒绝。
```

4. [X] orgunit 知识包只描述组织字段、日期、审计、详情、下级组织的查询语义和边界；不得写本地 recent-slot 继承规则、保守菜单回复或关键词防御。

### 5.4 P3：执行链反隐式绑定

1. [X] 检查 `ExecutionRegistry` 和各 executor：所有目标参数必须来自 planner 输出。
2. [X] 禁止 server 在 planner 前后填充 `org_code`、`org_codes`、`as_of`、audit target。
3. [X] 对缺 target 的审计/详情/下级查询，优先把 evidence 交给模型继续规划或澄清；只有模型输出非法或硬约束失败时，才返回受控参数错误。
4. [X] 对集合型 target，必须由模型显式输出多个 `ReadPlan` step 或通过 `471` loop 逐步取得并显式执行。
5. [X] 删除或降级 query flow 中的入口级静态兜底、关键词拒绝、固定“当前主要支持...”答复；领域能力说明应由模型基于工具目录生成。

### 5.5 P4：回归测试与验收夹具

1. [X] 增加 planner 输入投影测试：断言真实失败样本中，`query_evidence_window` 同时包含上一轮搜索 observation、assistant 展示的组织、用户“以上全部”和当前“审计信息”。
2. [X] 增加 stub planner/query flow 测试：当 planner 显式输出 3 个 `orgunit.audit` step 时，执行器只按这些显式目标执行。
3. [X] 增加反漂移测试：同一会话中即使更早问过“根组织审计信息”，后续“以上全部 -> 审计信息”不得被本地绑定回根组织或其他最近执行结果。
4. [X] 增加模型 owner 测试：用户只说“审计信息”且 evidence 存在多个可能范围时，query flow 必须调用 planner 并接受模型的澄清/计划输出，不得在 planner 前本地拒绝或取最近实体。
5. [X] 增加文本检查测试或 prompt golden：新 planner prompt 不出现 `recent_confirmed_entity`、`recent_candidate_groups` 作为主契约字段。
6. [X] 增加消极防御反例测试：短输入 `全部`、`1日`、`审计信息` 不应被本地入口菜单或静态兜底截断，必须进入模型判断链。

### 5.6 P5：兼容字段删除或降级

1. [X] 若现有代码已经有 `recent_*` 字段，先停止注入 planner prompt。
2. [X] 若旧测试或序列化仍依赖这些字段，短期保留读兼容，但不再作为 prompt-facing 主契约。
3. [X] 后续若继续删除内部兼容字段，必须保证 canonical event replay 与既有测试夹具仍可迁移；本计划不要求为删除字段新增第二套状态机。

## 6. 验收标准

1. 真实失败样本不再漂移到 `成本B组(200006)`。
2. planner 输入中可以清楚看到当前用户输入、最近相关用户/助手文本、工具 observation 和 open clarification。
3. planner 输出的每次可执行查询都包含显式目标参数。
4. executor 不读取任何“最近确认实体 / 最近候选列表 / target binding”字段来补参。
5. 短输入、残缺日期、候选集合答复和裸 intent 默认进入模型判断链；本地不得用入口菜单、静态兜底或关键词防御抢答。
6. `DEV-PLAN-468C` 中与本计划冲突的主语义槽位方案已被标记为被本计划纠偏。

## 7. 停止线

出现以下任一情况，必须停止实现并回到本计划评审：

1. 需要在 Go 代码里判断“以上全部”“第一个”“它”“审计信息”这类自然语言。
2. 需要新增 `selected_target`、`current_entity`、`target_binding`、`working_set` 作为执行事实源。
3. 需要从历史工具结果自动补 `org_code` 或 `as_of`。
4. 模型输出不理想时，试图新增本地 fallback、静态菜单、保守拒绝或关键词防御来兜底，而不是改善事实输入、工具目录、prompt 或 loop。
5. 为了修一个样本新增 orgunit 专用执行捷径，而不是改善 evidence 输入或走 `471` loop。
6. 把“信不过模型”作为新增本地语义判断层的理由。

## 8. 工具链与门禁

- **命中触发器**：
  - [X] Go 代码
  - [X] 文档 / readiness / 证据记录
  - [ ] Routing / Authz / DB / sqlc / i18n
  - [ ] E2E

- **本计划建议验证**：
  - 文档变更：`make check doc`
  - Go 实施阶段：按 `AGENTS.md` Go 代码触发器执行 `go fmt ./... && go vet ./... && make check lint && make test`
  - CubeBox 查询链实施阶段补跑相关 query flow / planner / orgunit presentation 测试

首期不新增永久 CI gate。若后续连续发生 `recent_*` 语义状态回流，再评估增加专项门禁。

## 9. 实施记录

- 新增 `modules/cubebox.BuildQueryEvidenceWindow(...)`，把内部 `QueryContext` 投影为 prompt-facing 中性事实窗口。
- `internal/server/cubebox_query_flow.go` 的 planner、clarifier、narrator 输入从 `query_dialogue_context` 改为 `query_evidence_window`。
- `modules/cubebox/knowledge_pack.go` 删除 `context_followup_prompts`，no-query 不再根据最近实体切换本地续问提示。
- `modules/orgunit/presentation/cubebox/*` 改为描述 `query_evidence_window`、`open_clarification`、显式 `ReadPlan` 参数和硬约束边界。
- 回归覆盖 `以上全部 -> 审计信息`：即使更早存在根组织审计事实，执行器也只执行 planner 显式输出的 3 个财务组织审计 step。
