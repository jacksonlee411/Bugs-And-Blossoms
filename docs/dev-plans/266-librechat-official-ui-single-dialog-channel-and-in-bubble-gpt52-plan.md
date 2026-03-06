# DEV-PLAN-266：AI对话官方 UI 单通道与气泡内回写前置子计划

**状态**: 规划中（2026-03-06 16:31 CST）

## 1. 计划定位
- 266 不再被视为独立的业务闭环主计划。
- 266 的唯一定位是：**DEV-PLAN-260 的 UI / 通道前置子计划**。
- 260 负责真实业务对话闭环（补全 / 候选 / 确认 / 提交 / 回执）的 FSM 与语义；266 只负责把这些对话安全、单链路地落到官方 UI 的同一聊天流、同一气泡内。

## 2. 背景与问题定义
- `DEV-PLAN-265` 已确认：当前官方 UI 虽已恢复真实 upstream 与模型回复能力，但仍存在两类阻断 260 达成的前置问题：
  1. 官方原始发送链路仍会触发，导致出现官方 `Connection error`；
  2. 本仓回复仍可能落在对话框外的外挂容器，而不是官方对话气泡内。
- 因此，在 260 的真实业务 FSM 达成之前，266 必须先完成“**单通道 + 气泡内回写**”底座收口。

## 3. 266 的职责边界

### 3.1 必须负责
1. [ ] 收掉官方原始发送链路，避免一条用户输入同时触发“官方原生请求 + 本仓桥接请求”。
2. [ ] 消除官方 `Connection error` 对用户业务体验的干扰。
3. [ ] 将本仓模型回复写回官方 UI 同一消息流、同一 assistant 气泡体系。
4. [ ] 移除页面外外挂容器承担用户可见业务回复的职责。
5. [ ] 为 260 的 Case 1~4 提供统一 UI 承载面。

### 3.2 明确不负责
1. [ ] 不定义 Case 2~4 的业务 FSM。
2. [ ] 不定义缺字段补全语义、候选选择语义、二次确认语义。
3. [ ] 不单独宣布 Case 2~4 达成。
4. [ ] 不把“消息显示在官方气泡中”误当成“业务对话闭环已实现”。

## 4. 目标与非目标
### 4.1 目标（必须同时满足）
1. [ ] 官方 LibreChat 发送动作在进入上游原生请求前即被本仓接管，不再产生官方 `Connection error` 聊天气泡。
2. [ ] 用户每次在官方输入框发送后，只保留**单一回复通道**：本仓 Assistant 链路 → 真实大模型回复 → 官方消息流气泡。
3. [ ] 正常回复与错误回复都写入**官方对话流内部**，而不是页面外层 bridge panel / overlay / notice 容器。
4. [ ] 真实页面验收时，用户在官方对话框内只能看到本仓模型链路的最终文案，不再看到官方原始错误文案。
5. [ ] 审计与测试能够区分“官方链路已被阻断”与“模型回复已成功回写官方气泡”。

### 4.2 非目标
1. [ ] 不修改业务域 schema / 迁移 / sqlc。
2. [ ] 不长期保留“双链路并存 + 页面外外挂回执”的过渡方案作为最终形态。
3. [ ] 不通过新增第二套聊天 UI 替代官方 UI；最终承载界面仍是官方 LibreChat UI。
4. [ ] 不以“隐藏错误提示 DOM”冒充修复；必须从发送链路与消息落点两个根因层面完成收口。
5. [ ] 不承担 260 的业务状态机职责。

## 5. 当前现状与根因
1. [ ] **双链路并存**：桥接脚本当前只监听/追加，不阻断官方原始发送；用户一次发送会同时触发官方原生请求与本仓 Assistant 请求。
2. [ ] **官方失败气泡仍可见**：官方原始请求命中上游连接错误后，官方消息流内仍生成 `Connection error` 相关错误气泡。
3. [ ] **回复落点错误**：本仓回复当前被渲染到对话框外的 bridge 容器，而不是官方消息列表中的 assistant message item。
4. [ ] **260 验收被 UI 干扰**：即便 260 的业务语义部分达成，只要消息不在官方气泡内、或官方错误气泡仍可见，用户体验仍判未达成。

## 6. 设计与实施步骤

### 6.1 M1：冻结单发送通道契约
1. [ ] 明确官方 UI 中“发送”动作的唯一接管点（按钮提交、回车提交、重试提交等）。
2. [ ] 梳理当前 bridge 对官方 DOM / 事件 / 网络请求的挂载点，标注哪些只监听、哪些需要改为阻断 + 接管。
3. [ ] 在代理层或注入脚本层增加“原始发送已拦截”的可观测标识，供 260/266 测试复用。

### 6.2 M2：收掉官方原始发送链路
1. [ ] 拦截官方发送事件，阻断 LibreChat 对上游原始提交与默认错误渲染触发条件。
2. [ ] 将用户输入统一改写为本仓 Assistant 单请求路径，确保每轮只生成一个可追踪的 `conversation_id/turn_id`。
3. [ ] 对重试、重新生成、回车发送、按钮发送等交互保持同一拦截口径，避免漏网路径。
4. [ ] 增加测试探针或日志证据，验证“官方原始发送未再发生”。

### 6.3 M3：把模型回复写回官方消息流
1. [ ] 调查官方消息列表 DOM / 数据流最小可接管点，选择最稳妥的“消息项内注入/替换”方案。
2. [ ] 将本仓模型最终文案写回官方 assistant message item，使其出现在官方对话记录内部。
3. [ ] 错误场景与正常场景使用同一消息落点。
4. [ ] 清理或下线现有对话框外 bridge 容器渲染逻辑，避免同轮重复显示。

### 6.4 M4：审计、容错与回归防线
1. [ ] 审计字段补齐：区分“官方发送已拦截”“消息已内嵌官方气泡”“是否存在外挂渲染”。
2. [ ] fail-closed：若官方发送拦截失败或消息无法回写官方气泡，则整轮判失败。
3. [ ] 为首轮、错误回复、重试回复分别补充回归测试。

### 6.5 M5：作为 260 前置验收项固化证据
1. [ ] 新增/更新真实 E2E，用官方 UI 实际输入并断言：
   - [ ] 不出现官方 `Connection error` 气泡；
   - [ ] 模型回复出现在官方聊天流内部；
   - [ ] 页面外不存在旧 bridge 回复容器。
2. [ ] 固化证据到 `docs/dev-records/assets/dev-plan-266/`。
3. [ ] 将实施与验证过程记录到 `docs/dev-records/dev-plan-266-execution-log.md`。

## 7. 验收标准（硬门槛）
1. [ ] 官方 UI 中真实发送后，用户看不到官方原始 `Connection error` 聊天气泡。
2. [ ] 同一轮回复只出现一次，且位于官方对话流内部的 assistant 气泡中。
3. [ ] 页面外 bridge 容器不再承担用户可见业务回复职责。
4. [ ] 266 通过只能表示“260 的 UI / 通道前置条件满足”，**不能单独代表 260 的 Case 2~4 已达成**。

## 8. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- 266 最低验证集计划：
  1. [ ] `go test ./internal/server -run 'TestAssistantUIProxy|TestModifyAssistantUIProxyResponse|TestAssistantReply|TestAssistantRenderReply' -count=1`
  2. [ ] `pnpm --dir apps/web test -- src/pages/assistant/LibreChatPage.test.tsx src/pages/assistant/AssistantPage.test.tsx src/pages/assistant/assistantAutoRun.test.ts`
  3. [ ] `pnpm --dir e2e exec playwright test tests/tp264-librechat-gpt52-dialog-response-real.spec.js tests/tp265-librechat-gpt52-blocked-reply-real.spec.js --reporter=line`
  4. [ ] 补充 266 专属真实 E2E，硬断言“无官方错误气泡 + 官方气泡内回写成功”。
  5. [ ] `make check doc`

## 9. 交付物
1. [ ] 前置子计划文档：`docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
2. [ ] 执行日志：`docs/dev-records/dev-plan-266-execution-log.md`
3. [ ] 证据目录：`docs/dev-records/assets/dev-plan-266/`
4. [ ] 相关后端 / 前端 / E2E 用例补强。

## 10. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
- `docs/dev-plans/264-librechat-gpt52-reply-single-pipeline-and-real-evidence-plan.md`
- `docs/dev-plans/265-librechat-gpt52-reply-goal-attainment-audit-and-gap-closure-plan.md`
- `docs/dev-records/dev-plan-265-execution-log.md`
- `AGENTS.md`
