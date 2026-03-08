# DEV-PLAN-266：AI对话官方 UI 单通道与气泡内回写前置子计划

**状态**: 实施中（2026-03-08 CST；`286/287` 已完成，`288` 已补真实入口 runner skeleton 与证据索引，待 live runtime 默认基线接线后封板）

## 1. 计划定位
- `266` 不再被视为独立的业务闭环主计划。
- `266` 的唯一定位是：**DEV-PLAN-260 的 UI / 通道前置子计划**。
- `260` 负责真实业务对话闭环（补全 / 候选 / 确认 / 提交 / 回执）的 FSM、DTO 与语义；`223` 负责这些语义的持久化事实源；`266` 只负责把这些对话安全、单链路地落到官方 UI 的同一聊天流、同一气泡内。

## 2. 背景与问题定义
- `DEV-PLAN-265` 已确认：当前官方 UI 虽已恢复真实 upstream 与模型回复能力，但仍存在两类阻断 `260` 达成的前置问题：
  1. 官方原始发送链路仍会触发，导致出现官方 `Connection error`；
  2. 本仓回复仍可能落在对话框外的外挂容器，而不是官方对话气泡内。
- 因此，在 `260` 的真实业务 FSM 达成之前，`266` 必须先完成“**单通道 + 气泡内回写**”底座收口。
- 本计划还需补上两项冻结：
  1. 验收入口必须与 `260` 一致，避免用 `/assistant-ui` 直链替代用户真实入口；
  2. 必须冻结“在哪一层阻断官方原始发送、如何把同轮回复绑定回唯一 assistant 气泡”的实现契约，避免后续实现漂移。

### 2.1 验收入口冻结
- `266` 的唯一用户交互入口与 `260` 保持一致：`http://localhost:8080/app/assistant/librechat`。
- `/app/assistant` 从本计划起冻结为**只读日志页**：允许展示运行态、会话、任务与审计记录，但**不得再提供输入框、提交按钮、Confirm / Commit / Task 等交互能力**。
- 历史 `/assistant-ui`、iframe 内部路由、代理层静态资源路径只可作为调试与定位手段，**不得单独作为 266 通过依据**，也不构成旧入口需长期保留的理由。
- 若运行态入口别名或历史 iframe 落点发生变化，仍以用户在 `/app/assistant/librechat` 页面中的实际体验为准；266 不要求这些历史路径长期存在。

## 3. 266 的职责边界

### 3.1 必须负责
1. [ ] 收掉官方原始发送链路，避免一条用户输入同时触发“官方原生请求 + 本仓桥接请求”。
2. [ ] 将错误统一收敛到官方气泡返回，不再依赖页面外挂提示或第二显示链路，也不以保留旧桥接链路作为兜底。
3. [ ] 将本仓模型回复写回官方 UI 同一消息流、同一 assistant 气泡体系。
4. [ ] 移除页面外外挂容器承担用户可见业务回复的职责，并推动旧桥接相关正式职责整体退役。
5. [ ] 为 `260` 的 Case 1~4 提供唯一 UI 承载面（`/app/assistant/librechat`）。

### 3.2 明确不负责
1. [ ] 不定义 Case 2~4 的业务 FSM。
2. [ ] 不定义缺字段补全语义、候选选择语义、二次确认语义。
3. [ ] 不定义 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` 的业务含义；这些以 `223/260` 为 SSOT。
4. [ ] 不在前端 store、helper、adapter 或消息组件中重算业务阶段、候选裁决或提交约束。
5. [ ] 不单独宣布 Case 2~4 达成。
6. [ ] 不把“消息显示在官方气泡中”误当成“业务对话闭环已实现”。

## 4. 目标与非目标
### 4.1 目标（必须同时满足）
1. [ ] 官方 LibreChat 发送动作在进入上游原生请求前即被本仓接管；无论成功还是失败，都通过统一官方气泡返回。
2. [ ] 用户每次在官方输入框发送后，只保留**单一回复通道**：本仓 Assistant 链路 → 真实大模型回复 → 官方消息流气泡。
3. [ ] 正常回复与错误回复都写入**官方对话流内部**，而不是页面外层 bridge panel / overlay / notice 容器。
4. [ ] 真实页面验收时，用户在官方对话框内只看到统一官方气泡返回；成功/失败都不允许落到页面外挂容器。
5. [ ] 审计与测试能够区分“官方链路已被阻断”与“模型回复已成功回写官方气泡”。
6. [ ] `266` 的通过证据必须来自用户真实入口页，不接受仅通过 `/assistant-ui` 直链验证即判完成。
7. [ ] `266` 的实现口径必须与 `280/284` 一致：以 vendored UI 的 send/store/render 控制点接管为正式目标，不再把注入脚本、DOM hack、HTML rewrite 当作目标形态。

### 4.2 非目标
1. [ ] 不修改业务域 schema / 迁移 / sqlc。
2. [ ] 不保留“双链路并存 + 页面外外挂回执”的过渡方案作为最终形态；在早期项目口径下，应尽快切断旧链路而非延长兼容窗口。
3. [ ] 不通过新增第二套聊天 UI 替代官方 UI；最终承载界面仍是 `/app/assistant/librechat` 内的官方 LibreChat UI。
4. [ ] 不以“隐藏错误提示 DOM”冒充修复；必须从发送链路与消息落点两个根因层面完成收口。
5. [ ] 不承担 `260` 的业务状态机职责。
6. [ ] 不把“注入脚本 / 代理层探针 / DOM 替换”继续当作长期正式方案；若临时存在，只能视为过渡实现，不得写成目标架构。

## 5. 当前现状与根因
1. [ ] **双链路并存**：旧桥接方案当前只监听/追加，不阻断官方原始发送；用户一次发送会同时触发官方原生请求与本仓 Assistant 请求。
2. [ ] **错误回包必须统一**：失败场景允许显示官方错误气泡，但必须属于统一官方消息流返回，而不是外挂容器或第二链路。
3. [ ] **回复落点错误**：本仓回复当前仍可能落到页面外 Alert / bridge 容器，或未稳定写回官方消息列表中的 assistant message item。
4. [ ] **SSOT 被破坏**：`/app/assistant` 历史上残留了额外聊天壳层与事务交互，造成“日志页 + 交互页”双承载面并存。
5. [ ] **260 验收被 UI 干扰**：即便 `260` 的业务语义部分达成，只要消息不在官方气泡内、或错误回包未走统一官方消息流，用户体验仍判未达成。
6. [ ] **实现口径仍有历史残留**：部分表述仍停留在注入脚本 / DOM hack / bridge stream 时代，需要收敛到 `280/284` 的源码级 send/store/render 接管口径。

## 6. 设计与实施步骤

### 6.0 先冻结的实现契约（不冻结不得开工）
1. [X] **主接管层冻结**：以 vendored UI 的发送 action / store / renderer 控制点作为正式接管层，在 LibreChat 原生请求发出前统一拦截发送、回车发送、重试、重新生成等动作；代理层探针只用于观测与止损，**不能作为唯一修复手段**。
2. [X] **非接受方案冻结**：仅隐藏 `Connection error` DOM、仅在响应后删气泡、或只靠代理层吞掉错误响应而不阻断原始发送，都不计入完成。
3. [X] **同轮关联契约冻结**：每次用户发送必须只生成一组可审计关联键，至少能稳定关联同轮 `conversation_id`、`turn_id`、`request_id` 与唯一 assistant message item；并发发送、重试发送、重新生成不得复用或串用旧气泡。
4. [X] **占位与回写契约冻结**：用户发送后，官方消息流内必须先出现或可稳定定位到唯一 assistant 占位消息；模型返回后只允许回写该气泡，不得同时写入页面外 bridge panel / overlay / notice。
5. [X] **Stopline 冻结**：若任一轮出现“官方原始请求已发出”或“回复无法绑定回官方 assistant 气泡”，则该轮直接判失败，不得以隐藏错误、外挂提示、旁路 notice 视为通过。
6. [X] **前端降权冻结**：`266` 只负责通道与落点，不得在前端重新计算 `phase`、候选裁决、确认约束或提交条件。

### 6.1 M1：冻结单发送通道契约
1. [X] 明确官方 UI 中“发送”动作的唯一接管点（按钮提交、回车提交、重试提交等）。
2. [X] 梳理历史 bridge 对发送入口、消息落点与代理链路的挂载点，标注哪些是遗留监听、哪些需要在新架构下直接删除而非继续强化。
3. [X] 在受控观测层增加“原始发送已拦截”的可观测标识，供 `260/266` 测试复用。
4. [X] 明确记录以下最小探针口径：`native_send_attempted`、`native_send_blocked`、`native_send_emitted`、`official_reply_bound`。

### 6.2 M2：收掉官方原始发送链路
1. [X] 在 vendored UI 发送控制点拦截官方发送事件，阻断 LibreChat 对上游原始提交与默认错误渲染触发条件。
2. [X] 将用户输入统一转发到本仓 Assistant 单请求路径，确保每轮只生成一个可追踪的 `conversation_id/turn_id/request_id`。
3. [X] 对重试、重新生成、回车发送、按钮发送等交互保持同一拦截口径，避免漏网路径。
4. [X] 增加测试探针或日志证据，验证“官方原始发送未再发生”；通过标准不是“看不到错误气泡”，而是 `native_send_emitted=0`。

### 6.3 M3：把模型回复写回官方消息流
1. [X] 调查官方消息列表 store / renderer 的最小可接管点，选择最稳妥的“唯一 assistant 气泡绑定/回写”方案。
2. [X] 为每轮发送建立唯一 assistant 占位消息或等价稳定消息锚点，避免回复回写时出现串位、丢位或重复 assistant 气泡。
3. [X] 将本仓模型最终文案写回官方 assistant message item，使其出现在官方对话记录内部。
4. [X] 错误场景与正常场景使用同一消息落点，且都保留同轮 `conversation_id/turn_id/request_id` 的可追溯映射。
5. [X] 清理或下线现有对话框外 bridge 容器渲染逻辑，避免同轮重复显示。
6. [X] 消息落点接管只负责“绑定与显示”，不得在 UI 层根据文本或局部上下文补算业务含义。

### 6.4 M4：审计、容错与回归防线
1. [X] 审计字段补齐：区分“官方发送已拦截”“官方原始发送是否实际发出”“消息已绑定官方气泡”“是否存在外挂渲染”。
2. [X] fail-closed：若官方发送拦截失败或消息无法回写官方气泡，则整轮判失败。
3. [X] 失败时只能在官方消息流内显示技术态失败提示或保留可重试状态，不得把最终用户可见业务回执降级到页面外 notice / overlay。
4. [X] 为首轮、错误回复、重试回复分别补充回归测试。

### 6.5 M5：作为 260 前置验收项固化证据
1. [X] 新增/更新真实 E2E，用官方 UI 实际输入并断言：
   - [X] 错误场景也通过统一官方气泡返回；
   - [X] 官方原始发送未实际发出（或等价强证据证明已在发出前拦截）；
   - [ ] 模型回复出现在官方聊天流内部；
   - [X] 页面外不存在旧 bridge 回复容器；
   - [ ] 同轮只存在一个 assistant 回复气泡，且能关联回同轮 `conversation_id/turn_id/request_id`。
2. [X] 固化证据到 `docs/dev-records/assets/dev-plan-266/`。
3. [X] 将实施与验证过程记录到 `docs/dev-records/dev-plan-266-execution-log.md`。

### 6.6 用户可见交互与体验变化（作为正式验收依据）
1. [X] **单次发送 = 单次有效通道**：用户在 `/app/assistant/librechat` 页面点击发送、按回车发送、点击重试或重新生成时，只感知到一条连续聊天链路；不得再出现“一次输入触发两条链路”的重复体验。
2. [ ] **回复留在官方聊天气泡内**：用户发送后，最终看到的 assistant 回复必须位于官方 LibreChat 聊天流内部，而不是落到页面外 bridge panel、overlay、notice 或其他外挂容器。
3. [ ] **同轮只看到一份回复**：同一轮对话中，用户只能看到一份 assistant 最终回复；不得出现官方气泡一份、外挂区域再一份、或同轮多个 assistant 气泡串位/重复。
4. [X] **用户错误回包统一**：在 `266` 达成后，如该轮失败，也只能以受控方式在官方消息流内呈现技术态失败，不得落到外挂提示或第二显示链路。
5. [X] **对话承载面连续统一**：从用户视角看，整个体验应表现为“始终在同一个官方聊天窗口内连续对话”，而不是“官方界面外再套一层桥接回复系统”。
6. [X] **266 的用户价值边界明确**：`266` 达成只表示聊天承载面、通道与落点收口完成；用户虽然会感受到更干净、更一致的聊天体验，但这不等于 `260` 的缺字段补全、多候选确认、提交确认、成功回执等业务 FSM 已全部完成。

## 7. 验收标准（硬门槛）
1. [X] 官方 UI 中真实发送后，成功与失败都通过统一官方气泡返回。
2. [ ] 同一轮回复只出现一次，且位于官方对话流内部的 assistant 气泡中。
3. [X] 页面外 bridge 容器不再承担用户可见业务回复职责。
4. [X] 验收证据必须能证明该轮官方原始发送未实际发出；仅“错误气泡不可见”不构成通过。
5. [ ] 验收证据必须能证明同轮 `conversation_id/turn_id/request_id` 与唯一 assistant 气泡一一对应，不存在串泡、外挂回执或双写。
6. [ ] 上述“6.6 用户可见交互与体验变化”各项必须都能被真实页面录像、截图、DOM 断言或 trace 佐证，作为 `266` 正式验收依据的一部分。
7. [X] `266` 通过只能表示“260 的 UI / 通道前置条件满足”，**不能单独代表 260 的 Case 2~4 已达成**。
8. [X] 若实现仍依赖注入脚本 / DOM hack / HTML rewrite 作为正式主链路，而不是 `280/284` 的源码级 send/store/render 接管，则 `266` 不得宣称完成。
9. [X] 若前端仍需根据文本、局部上下文或组件临时状态补算业务 phase、候选裁决或确认约束，则 `266` 不得宣称与 `223/260` 对齐。

## 8. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- `266` 最低验证集计划：
  1. [ ] 补充 `266` 专属真实 E2E，硬断言“官方原始发送未发出 + 成功/失败均经统一官方气泡返回 + 无页面外挂回复容器”；该用例是 `266` 的主通过条件。
  2. [X] `go test ./internal/server -run 'TestAssistantUIProxy|TestModifyAssistantUIProxyResponse|TestAssistantReply|TestAssistantRenderReply' -count=1`
  3. [X] `pnpm --dir apps/web test -- src/pages/assistant/LibreChatPage.test.tsx src/pages/assistant/AssistantPage.test.tsx`（旧桥 helper 测试已由 `DEV-PLAN-282` 退役）
  4. [ ] 旧桥专属 real E2E 已由 `DEV-PLAN-282` 删除；`266` 的新入口真实回归已由 `DEV-PLAN-288` 建立 runner skeleton，待默认基线接线。
  5. [ ] `make check doc`

## 9. 交付物
1. [X] 前置子计划文档：`docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
2. [X] 执行日志：`docs/dev-records/dev-plan-266-execution-log.md`
3. [X] 证据目录：`docs/dev-records/assets/dev-plan-266/`
4. [X] 相关后端 / 前端 / E2E 用例补强。

## 10. 关联文档
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
- `docs/dev-plans/264-librechat-gpt52-reply-single-pipeline-and-real-evidence-plan.md`
- `docs/dev-plans/265-librechat-gpt52-reply-goal-attainment-audit-and-gap-closure-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/archive/dev-records/dev-plan-265-execution-log.md`
- `AGENTS.md`
