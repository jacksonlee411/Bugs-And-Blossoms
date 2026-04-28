# DEV-PLAN-470：CubeBox `page_context` 当前范围剔除与清理方案

**状态**: 已完成（2026-04-26 10:10 CST；`page_context` 剔除、代码清理与文档回写已完成）

## 1. 背景

`DEV-PLAN-467` 曾把“当前页面上下文未进入请求体”列为次级问题：当时发现 `orgunit` 知识包中存在“若当前页面已经是组织架构相关页面，可优先从当前页面上下文补 `org_code`”的描述，但运行时没有真实页面对象事实可用。

后续 `DEV-PLAN-468 Slice E / P1` 为了兑现这条知识包描述，引入了轻量 `page_context`，并由前端向 `/internal/cubebox/turns:stream` 发送页面路径、业务对象、当前对象和日期视图。随后又出现 `page_context.view.as_of -> effective_date` 的 DTO 改名讨论。

重新评估后，本计划裁决：**`page_context` 不纳入当前实施范围**。原因是：

1. `467` 的真实主问题是查询会话事实没有进入 planner，即上一轮已确认的 `org_code=100000` 与 `as_of=2026-04-25` 未被继承；这应由 `query_dialogue_context` / query fact window 解决，不应归因到当前页面。
2. 知识包中“当前页面可补 `org_code`”只有一条宽泛规则，证据不足以支撑建设或扩展页面事实协议。
3. 当前 `page_context` 没有进入 query planner 的一等输入；它主要进入 canonical context、narrator 和 clarifier，无法兑现“计划阶段补参”的主要诉求。
4. 页面相对指代容易误选对象：列表页、详情页、字段配置页、树选中节点和筛选条件并不天然等价于“用户当前要问的组织”。
5. 继续扩展 `page_context` 会把一个窄 UX 辅助点膨胀成复杂状态协议，偏离当前最小闭环。

因此，470 不再是日期字段改名计划，而是 `page_context` 当前范围剔除与清理的唯一 owner。

## 2. 问题定义

当前仓面存在三类漂移：

1. **错误 owner 漂移**：`468C` 曾把 `P2-3c` 定义为“扩展 `page_context` 与 query context 候选组/事实窗口”，导致页面事实协议和查询会话事实窗口被绑在一起。
2. **运行时表面漂移**：前端、API、canonical context、narrator、clarifier 和普通 provider prompt view 已出现 `page_context` 字段，但它没有成为 planner 的有效事实来源。
3. **知识包语义漂移**：`orgunit` 知识包把“当前页面上下文补 `org_code`”写成可用规则，但当前阶段没有足够证据证明它应作为产品查询语义保留。

这些漂移会造成两个实际风险：

1. 误导后续实现继续扩展页面事实 DTO，而不是先修 query dialogue fact window。
2. 让模型把“页面上看得到的对象”误当成“用户当前明确指代的对象”。

## 3. 目标

1. [x] 从当前实施范围中剔除 `page_context`。
2. [x] 删除前端 `/internal/cubebox/turns:stream` 请求体中的 `page_context` 构造与发送。
3. [x] 删除服务端请求 DTO、规范化、canonical context、prompt-facing envelope 中的 `page_context` 字段。
4. [x] 删除 `modules/cubebox/page_context.go` 及对应测试，或在无引用后完全移除。
5. [x] 删除 `orgunit` 知识包中的“从当前页面补参”规则。
6. [x] 将 `468 P2-3c` 收敛为 query dialogue fact window：候选组、最近确认实体列表、最近问答、澄清状态与 `recent_confirmed_entity` 降级。
7. [x] 文档回写：`461`、`462`、`468`、`468C`、readiness、AGENTS 文档地图不再把 `page_context` / 页面上下文列为当前能力、当前扩展项或每轮注入要求。

## 4. 非目标

1. 不在本计划内用 `effective_date` 替换 `page_context.view.as_of`；因为 `page_context` 整体退出当前范围，日期字段改名不再成立。
2. 不新增替代性的 `screen_context`、`ui_context`、`visible_context`、`route_context` 或类似字段。
3. 不在当前阶段实现“页面选中对象可被模型引用”的能力。
4. 不改变组织详情页、组织列表页自身的 URL、日期、筛选或页面状态逻辑。
5. 不改变 `query_dialogue_context` 的当前已落地能力；后续候选组/事实窗口仍由 468C 承接。

## 5. 方案

### 5.1 文档 owner 收敛

1. `DEV-PLAN-468`：
   - 保留 `P2-3a` / `P2-3b` 已完成事实。
   - 将 `P2-3c` 改为“query dialogue fact window”，不再包含 `page_context`。
   - 明确 `page_context` 当前范围剔除由 `DEV-PLAN-470` 承接。

2. `DEV-PLAN-468C`：
   - 改写为“查询上下文事实窗口扩展方案”。
   - 删除 `page_context` DTO 扩展、planner 注入、前端页面事实收集、服务端页面事实规范化、页面事实验收场景。
   - 保留并细化 `recent_candidate_groups`、`recent_confirmed_entities`、`recent_dialogue_turns`、`last_clarification`、`ResolvedEntity` 语义收敛。

3. `DEV-PLAN-470`：
   - 作为 `page_context` 剔除和反回流 owner。
   - 不再承接 `view.as_of -> effective_date` 改名。

4. 其他现行文档：
   - `DEV-PLAN-461` 删除“当前页面/当前对象用于知识包发现或补参”的当前契约，改为按 query intent / 已登记知识包 / query dialogue context 收敛。
   - `DEV-PLAN-462` 删除“页面上下文每轮重新注入”的当前约束，改为权限、租户、语言和查询会话事实按轮重建。
   - readiness 只保留历史说明，不得把 `page_context` 写成当前验收或当前能力。

### 5.2 前端清理

目标文件：

1. `apps/web/src/pages/cubebox/types.ts`
2. `apps/web/src/pages/cubebox/api.ts`
3. `apps/web/src/pages/cubebox/api.test.ts`
4. `apps/web/src/pages/cubebox/CubeBoxProvider.tsx`
5. `apps/web/src/pages/cubebox/CubeBoxProvider.test.tsx`

要求：

1. 删除 `CubeBoxPageContext` 类型。
2. 删除 `buildCubeBoxPageContext(...)`、`compactPageContext(...)`、`resolvePageContextAsOf(...)` 及相关 normalize helper。
3. 删除 `pageContextRef`。
4. `streamTurn(...)` 请求体只发送 `conversation_id`、`prompt`、`next_sequence`。
5. 删除“posts controlled page context”与“derives controlled orgunit page context”等测试；如需保留测试，应改为断言不发送 `page_context`。

### 5.3 服务端清理

目标文件：

1. `internal/server/cubebox_api.go`
2. `internal/server/cubebox_api_test.go`
3. `internal/server/cubebox_query_flow.go`
4. `internal/server/cubebox_query_flow_test.go`
5. `modules/cubebox/gateway.go`
6. `modules/cubebox/compaction.go`
7. `modules/cubebox/compaction_test.go`
8. `modules/cubebox/page_context.go`

要求：

1. 删除 `GatewayStreamRequest.PageContext`。
2. 删除 `cubeboxTurnStreamRequest.PageContext`。
3. 删除 `CanonicalContext.PageContext`。
4. 删除 `PageContext`、`PageObjectContext`、`PageViewContext` 与 `NormalizePageContext(...)`。
5. 删除 canonical block 中的 `page_facts` 输出。
6. 删除 narrator / clarifier envelope 中的 `page_context`。
7. narrator / clarifier system prompt 改为只允许依据 `user_prompt`、`dialogue_context`、`results` 或 `candidates`；不得再提 `page_context`。
8. 删除所有请求、prompt、envelope、canonical context 中关于 `page_context` 的测试断言。

说明：`CanonicalContext.Page` 与 `CanonicalContext.BusinessObject` 作为普通 shell/canonical 元数据是否继续保留，由当前代码清理时按最小变更判断。本计划强制删除的是 `PageContext` DTO、`page_facts`、prompt-facing 页面事实和页面补参语义；不得再由这些字段承载模型补参。

### 5.4 知识包清理

目标文件：

1. `modules/orgunit/presentation/cubebox/CUBEBOX-SKILL.md`
2. `modules/orgunit/presentation/cubebox/queries.md`
3. `modules/orgunit/presentation/cubebox/examples.md`

要求：

1. 删除“若当前页面已经是组织架构相关页面，可优先从当前页面上下文补 `org_code`”。
2. 删除“不要覆盖为页面上下文”等残留引用；知识包不得再出现“页面上下文”“页面补参”“current page”“page context”作为当前查询规则。
3. 连续追问只指向 `query_dialogue_context`，不再提页面上下文。
4. 默认日期规则保留为 planner 当前自然日或用户显式日期，不从页面日期视图推导。
5. 示例只覆盖用户输入、query dialogue context、候选组和前序结果引用，不覆盖页面事实补参。

### 5.5 反回流门禁

本计划不要求立即新增全仓 gate，但实现时至少要补最小测试断言：

1. 前端 stream 请求不包含 `page_context`。
2. 服务端 decode 请求后不存在 `PageContext` 字段。
3. canonical context block 不输出 `page_facts`。
4. narrator / clarifier envelope 不输出 `page_context`。
5. 知识包搜索不再命中“当前页面上下文补 `org_code`”或其他页面补参同义表述。
6. Web 生成资产重新生成后不得残留 `page_context` / `PageContext` / `page_facts`。

如后续出现 `page_context`、`PageContext`、`page_facts` 回流，应先回到本计划评审，不得在 468C 或其他计划中顺手恢复。

## 6. 安全边界与 Stopline

1. [x] 不允许保留 legacy `page_context` 兼容读取、兼容写入、双字段发送或静默映射。
2. [x] 不允许换名引入同义字段，例如 `screen_context`、`route_context`、`ui_context`。
3. [x] 不允许知识包继续声明“从当前页面补参”，也不允许保留“页面上下文不覆盖用户输入”这类暗示性规则。
4. [x] 不允许 planner、narrator、clarifier prompt-facing 输入继续暴露页面事实。
5. [x] 不允许为了补参方便在 server 中新增 orgunit 页面路径特判。
6. [x] 若未来重新引入页面上下文，必须新建或重开独立计划，先证明真实用户闭环依赖它，且不得绕过 query dialogue fact window。

## 7. 实施步骤

1. [x] 重写 `DEV-PLAN-470` 为当前文档。
2. [x] 回写 `DEV-PLAN-468`：`P2-3c` 不再包含 `page_context`。
3. [x] 回写 `DEV-PLAN-468C`：改为 query dialogue fact window 专项。
4. [x] 更新 `AGENTS.md` 文档地图标题。
5. [x] 更新 readiness 记录，说明 `page_context` 已裁决为当前范围外。
6. [x] 回写 `DEV-PLAN-461` / `DEV-PLAN-462` 中与页面上下文当前能力冲突的条款。
7. [x] 前端删除 `page_context` 构造与发送。
8. [x] 服务端删除 `PageContext` DTO 与 prompt-facing 输出。
9. [x] 知识包删除页面补参规则。
10. [x] 执行测试与门禁。

## 8. 验收

1. [x] `rg -n "page_context|PageContext|page_facts|pageContext|CubeBoxPageContext|buildCubeBoxPageContext|NormalizePageContext|PageViewContext|PageObjectContext" apps/web/src internal modules/cubebox modules/orgunit/presentation/cubebox` 不再命中当前运行时代码和知识包，测试 fixture 中也不得作为请求/响应字段存在。
2. [x] `/internal/cubebox/turns:stream` 请求体只包含 `conversation_id`、`prompt`、`next_sequence`。
3. [x] provider canonical context 不再输出 `page_facts`。
4. [x] query planner、narrator、clarifier 输入均不包含 `page_context`。
5. [x] `orgunit` 知识包不再要求模型从当前页面补 `org_code`。
6. [x] 467 真实失败场景仍由 `query_dialogue_context` / query fact window 解决：`100000 -> 查该组织的下级组织` 不能依赖当前页面事实。
7. [x] `rg -n "当前页面上下文|页面上下文|页面补参|当前页面.*补|current page|page context" modules/orgunit/presentation/cubebox` 不再命中当前知识包。
8. [x] Web 生成资产重新生成后，`rg -n "page_context|PageContext|page_facts" internal/server/assets/web/assets apps/web/dist` 不再命中可提交产物。
9. [x] 文档中仅允许 `DEV-PLAN-470`、`DEV-PLAN-468/468C` 和 readiness 的“剔除/历史说明”出现 `page_context`；不得作为当前能力、当前实施要求或补参规则出现。

## 9. 验证命令

代码实施时按实际触发器运行：

1. [x] `go fmt ./... && go vet ./... && make check lint && make test`
2. [x] Web 源码变更后执行 `make generate && make css`，并确认 `git status --short` 只包含预期变更。
3. [x] 文档变更执行 `make check doc`。
4. [x] 执行 §8 中的运行时代码、知识包、生成资产和文档漂移搜索。

## 10. 当前状态

- [x] 470 已完全重写为 `page_context` 当前范围剔除与清理方案。
- [x] 468 / 468C / readiness / AGENTS 文档回写。
- [x] 461 / 462 冲突口径回写。
- [x] 代码与知识包清理。
- [x] 文档门禁验证：`make check doc`。
