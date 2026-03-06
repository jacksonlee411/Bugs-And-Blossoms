# DEV-PLAN-260：AI对话真实业务闭环主计划（多轮补全 / 候选确认 / 提交回执）

**状态**: 规划中（2026-03-06 18:11 CST）

> 历史执行记录仍保留在 `docs/dev-records/dev-plan-260-execution-log.md`，但其“已完成”只代表旧口径阶段性实现；**不再等同于当前真实需求已达成**。

## 1. 背景与重开原因
- 用户已明确指出：239A 及其后续落地**偏离真实需求**，没有真正实现“通过 AI 对话形式完成多轮补充 / 确认信息、自动执行操作，并通过对话告诉用户结果”的闭环。
- 当前问题不是单一的文案或截图问题，而是**主计划与子问题边界混乱**：
  1. 旧 260 更像一次阶段性收口记录，但没有把“真实业务闭环”与“官方 UI / 通道落点”分层冻结；
  2. 266 当前主要解决官方 UI 单通道与气泡内回写问题，**不能单独代表业务对话闭环**；
  3. 因此需要把 260 重新打开，恢复为**主计划**，把真实 Case 1~4 作为唯一验收口径。
- 本次重开后，计划分工冻结为：
  - **260 = 主计划**：定义真实业务对话闭环、FSM、确认语义、补全语义、候选语义、自动执行时机。
  - **266 = 前置子计划**：保证这些对话都发生在官方 UI 的同一聊天流、同一气泡内，不再出现外置容器和官方 `Connection error`。

## 2. 唯一目标口径（以用户真实案例为准）

### 2.1 验证入口
- 验证入口按用户当前口径冻结为：`http://localhost:8080/app/assistant/AI对话`
- 若运行态存在路由别名或 iframe 落点差异，**以用户实际可见的“AI对话独立页”体验为准**，不得以技术内部路径差异规避验收。

### 2.2 真实 Case（必须 100% 达成）
1. [ ] **Case 1：通道连通（前置）**
   - 输入：`你好`
   - 预期：
     - 页面出现“自动执行通道已连接：可直接在 AI对话 对话中输入需求。”
     - 页面不白屏、输入框可用、可正常发消息。
2. [ ] **Case 2：一句话自动执行（完整信息）**
   - 输入：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`
   - 预期：
     - AI 先在对话中返回准备提交的信息摘要；
     - AI 在对话中询问用户是否确认提交；
     - 用户通过对话输入确认；
     - 系统自动执行 `create -> confirm -> commit`；
     - AI 通过对话告诉用户已提交成功。
3. [ ] **Case 3：信息不充分 -> 对话补全**
   - 第一句：`在 AI治理办公室 下新建 人力资源部239A补全`
   - 第二句：`生效日期 2026-03-25`
   - 预期：
     - 第一句先提示缺少字段；
     - AI 在对话中明确指出缺的是哪个字段，并引导用户补全；
     - 用户第二句通过对话补充确切信息；
     - AI 在对话中给出准备提交的信息并要求确认；
     - 用户确认后，系统自动执行并提交成功；
     - AI 通过对话告诉用户已提交成功。
4. [ ] **Case 4：多候选确认（对话内完成）**
   - 第一句：`在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26`
   - 第二句：`选第2个`（或候选编码）
   - 预期：
     - AI 发现系统中有多个匹配后，在对话中以用户友好形式列出候选并编号；
     - 用户通过对话反馈“选第N个/编码”完成候选选择；
     - AI 再次确认用户选择的是哪个具体候选项；
     - 用户通过对话确认“是的”；
	   - 系统自动执行并提交成功；
	   - AI 通过对话告诉用户已提交成功。

### 2.3 Case 1~4 共通 UI / 体验前提（继承 266）
1. [ ] Case 1~4 的真实页面验收，必须同时满足 `DEV-PLAN-266` 第 6.6 节“用户可见交互与体验变化”与第 7 节“验收标准（硬门槛）”。
2. [ ] 同一轮用户输入只允许一条有效发送通道；不得再出现“官方原始发送 + 本仓桥接请求”双链路并存。
3. [ ] 同一轮 assistant 最终回复只能出现一次，且必须位于官方聊天流内部。
4. [ ] 页面外 bridge 容器、overlay、notice 不得承担用户可见业务回执职责。
5. [ ] 任一 Case 若出现官方 `Connection error`、双写、串泡或外挂回执，则该 Case 直接判失败，即使业务语义本身正确。

## 3. 主从关系冻结（260 主计划 / 266 前置子计划）
1. [ ] **260 主计划职责**：
   - 定义 Case 1~4 的业务 FSM；
   - 定义哪些轮次等待补全、等待候选、等待二次确认、等待提交确认；
   - 定义用户输入如何驱动 `create / confirm / commit`；
   - 定义最终成功 / 失败回执的业务语义。
2. [ ] **266 前置子计划职责**：
   - 收掉官方原始发送链路；
   - 保证所有业务回执都写入官方 UI 同一聊天流、同一气泡体系；
   - 移除页面外外挂容器；
   - 消除官方 `Connection error` 干扰。
3. [ ] **边界冻结**：
	   - 未完成 266，不得宣称 260 用户体验达成；
	   - 即使 266 完成，若 260 的业务 FSM/确认语义未完成，也不得宣称 Case 2~4 达成。
	   - 260 任一 Case 的通过，必须同时满足 266 的单通道、气泡内回写、无外挂容器、无官方原始错误体验等前置门槛；若 266 回归退化，则 260 视为未通过。

## 4. 目标与非目标

### 4.1 核心目标
1. [ ] 所有业务闭环步骤都必须通过**对话**完成，不得依赖页面外提示、浮层、表单按钮或隐藏状态提示来完成业务确认。
2. [ ] 正常、缺字段、多候选、提交成功、提交失败五类结果，都必须通过 AI 对话返回给用户。
3. [ ] 写入动作仍保持 One Door：只允许走既有 `/internal/assistant/*` 与 DB Kernel 提交链路。
4. [ ] 用户可见业务文案必须来自真实大模型，不允许本地模板 / fallback 冒充。
5. [ ] `AssistantPage` 与 AI对话独立页必须复用同一套业务 FSM helper，禁止双份编排漂移。
6. [ ] Case 1~4 除业务语义达成外，还必须同时满足单通道、官方气泡内回写、同轮唯一回复、无外挂回复容器、无官方原始 `Connection error` 干扰。

### 4.2 非目标
1. [ ] 不新增数据库 schema / 迁移 / sqlc 改动。
2. [ ] 不引入 legacy 双链路、兼容快路径或第二业务写入口。
3. [ ] 不修改 LibreChat 上游源码；UI 适配通过 266 的代理注入与本仓前端编排收口。
4. [ ] 不以“局部单测通过”“页面外出现提示”或“接口返回成功”作为 Case 2~4 达成依据。

## 5. 业务状态机（FSM）冻结

### 5.1 运行态阶段
```ts
interface DialogFlowState {
  phase:
    | 'idle'
    | 'await_missing_fields'
    | 'await_candidate_pick'
    | 'await_candidate_confirm'
    | 'await_commit_confirm'
    | 'committing'
    | 'committed'
    | 'failed'
  conversation_id: string
  turn_id: string
  pending_draft_summary: string
  missing_fields: string[]
  candidates: AssistantCandidateOption[]
  selected_candidate_id: string
}
```

### 5.2 阶段语义
1. [ ] `idle`
   - 仅表示当前没有待补全 / 待选择 / 待确认上下文。
2. [ ] `await_missing_fields`
   - AI 必须明确告诉用户缺哪些字段；
   - 用户补充后，系统重新生成草案；
   - 不允许在该阶段直接 commit。
3. [ ] `await_candidate_pick`
   - AI 必须以编号列表形式给出候选；
   - 用户可通过“选第N个/候选编码”反馈选择；
   - 选择后转 `await_candidate_confirm`。
4. [ ] `await_candidate_confirm`
   - AI 必须复述用户选中的候选具体内容；
   - 用户确认后才能执行 `confirm(candidate_id)`。
5. [ ] `await_commit_confirm`
   - AI 必须展示准备提交的摘要；
   - 只有用户明确确认后才能执行 `commit`。
6. [ ] `committing`
   - 后台正在提交；
   - 完成后转 `committed` 或 `failed`。
7. [ ] `committed`
   - AI 必须通过对话明确告诉用户提交成功。
8. [ ] `failed`
   - AI 必须通过对话解释失败原因与下一步建议；
   - 不允许仅在页面外给 notice/alert。

### 5.3 不变量
1. [ ] `phase != await_candidate_confirm && phase != await_commit_confirm` 时，确认词不得触发写入。
2. [ ] `selected_candidate_id` 为空时，不得进入 `await_candidate_confirm`。
3. [ ] `pending_draft_summary` 为空时，不得进入 `await_commit_confirm`。
4. [ ] 任意 `confirm/commit` 失败后必须转入 `failed` 并在对话中回执。
5. [ ] 任意阶段若用户可见业务回执不在聊天流内，则整轮验收判失败。
6. [ ] 任意轮用户发送不得触发双链路；若官方原始发送实际发出，则该轮验收直接失败。
7. [ ] 任意轮 assistant 最终回复只能出现一次，且必须能与同轮 `conversation_id/turn_id/request_id` 一一对应。
8. [ ] 页面外 bridge 容器、overlay、notice 不得承担用户可见业务回执职责。

## 6. 内部调用序列（冻结）
1. [ ] **Case 2**：
   - `POST /conversations/:id/turns`
   - 等待用户确认
   - `:confirm`
   - `:commit`
   - `:reply`
2. [ ] **Case 3**：
   - `turns(首轮缺字段)`
   - 等待用户补全
   - `turns(补全后草案)`
   - 等待用户确认
   - `:confirm`
   - `:commit`
   - `:reply`
3. [ ] **Case 4**：
   - `turns(候选列表)`
   - 等待用户选择候选
   - AI 二次确认用户选中项
   - `:confirm(candidate_id)`
   - 等待提交确认
   - `:commit`
   - `:reply`

## 7. 实施分解

### 7.1 M1：业务语义重新冻结（主计划）
1. [ ] 将 Case 1~4 作为唯一业务验收契约写入测试与执行日志模板。
2. [ ] 统一确认词、候选选择词、补全语义解析规则。
3. [ ] 明确“哪些回复必须等待用户下一轮输入，哪些回复可以自动推进”。

### 7.2 M2：共享 FSM 与编排收口
1. [ ] 抽离并冻结共享 FSM helper，供 `AssistantPage` 与 AI对话独立页共用。
2. [ ] 删除页面级分叉编排，避免一个页面支持、另一个页面失效。
3. [ ] 保证 Case 2~4 在运行态中严格按 FSM 阶段推进。

### 7.3 M3：对话文案与模型链路收口
1. [ ] 所有业务回执统一走真实大模型回复链路。
2. [ ] 缺字段提示、多候选提示、确认提示、成功/失败回执，都必须由对话消息返回。
3. [ ] 禁止页面外 notice/alert 承担业务确认职责。

### 7.4 M4：依赖 266 完成 UI / 通道前置收口
1. [ ] 以 `266` 第 6.6 节与第 7 节为 readiness：只有当单通道、气泡内回写、无外挂容器、无官方原始错误体验全部达成后，260 才能进入最终 Case 通过判定。
2. [ ] 将官方原始发送链路收掉，并把“官方原始发送未实际发出”作为 260 Case 1~4 的共通前置断言。
3. [ ] 保证所有业务回执落到官方 UI 同一聊天流气泡中，并把“同轮唯一 assistant 回复”作为 260 Case 1~4 的共通前置断言。
4. [ ] 彻底去掉外挂容器与官方错误气泡干扰；若 266 回归退化，则 260 不得封板。

### 7.5 M5：真实验收与证据固化
1. [ ] 用真实页面按 Case 1~4 顺序逐条验收。
2. [ ] 每个 Case 必须保存页面全图、对话局部图、同轮 trace / 网络证据，并额外证明：无官方 `Connection error`、无页面外挂回复容器、同轮仅一份 assistant 回复。
3. [ ] 执行记录写回 `docs/dev-records/dev-plan-260-execution-log.md` 新章节，明确区分“旧 260 验收记录”与“本次重开后的真实需求验收记录”。

## 8. 验收标准（硬门槛）
1. [ ] Case 1~4 必须全部在**AI 对话中**闭环，不得借助页面外提示补齐业务流程。
2. [ ] Case 2 必须是“先草案、后确认、再提交”，不得首轮自动 commit。
3. [ ] Case 3 必须是“先缺字段提示、再补全、再确认、再提交”，不得跳过确认。
4. [ ] Case 4 必须是“先候选列表、再选择、再二次确认、再提交”，不得选中后直接提交。
5. [ ] 成功与失败回执都必须由真实大模型生成，并显示在聊天流气泡内。
6. [ ] Case 1~4 的每一轮都必须同时满足 `266` 第 6.6 节“用户可见交互与体验变化”与第 7 节“验收标准（硬门槛）”。
7. [ ] 任一 Case 如出现双链路、官方 `Connection error`、页面外挂容器承担回复或同轮多份 assistant 回复，则该 Case 直接判失败。
8. [ ] 266 未完成或回归退化前，不得宣布 260 用户体验达成。

## 9. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- 260 当前最低验证集：
  1. [ ] `go test ./internal/server -run 'TestAssistantUIProxy|TestAssistantReply|TestAssistantRenderReply' -count=1`
  2. [ ] `pnpm --dir apps/web test -- src/pages/assistant/assistantDialogFlow.test.ts src/pages/assistant/assistantAutoRun.test.ts src/pages/assistant/AssistantPage.test.tsx src/pages/assistant/LibreChatPage.test.tsx`
  3. [ ] `pnpm --dir e2e exec playwright test tests/tp260-librechat-dialog-closure.spec.js --reporter=line`
  4. [ ] 补充“AI对话独立页真实 Case 1~4”专属 E2E；每个 Case 必须同时断言 266 的共通 stopline：无官方原始发送、无官方错误气泡、无外挂回复容器、同轮唯一 assistant 气泡。
  5. [ ] `make check doc`

## 10. 交付物
1. [ ] 主计划文档：`docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
2. [ ] 前置子计划：`docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
3. [ ] 更新后的执行日志：`docs/dev-records/dev-plan-260-execution-log.md`
4. [ ] 真实用例证据目录：`docs/dev-records/assets/dev-plan-260/`
5. [ ] 相关后端 / Web / E2E 用例补强。

## 11. 关联文档
- `docs/dev-plans/239a-librechat-dialog-auto-execution-and-standalone-page-plan.md`
- `docs/dev-plans/239b-239a-direct-validation-report-and-implementation-gaps.md`
- `docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
- `docs/dev-plans/264-librechat-gpt52-reply-single-pipeline-and-real-evidence-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-records/dev-plan-260-execution-log.md`
- `AGENTS.md`
