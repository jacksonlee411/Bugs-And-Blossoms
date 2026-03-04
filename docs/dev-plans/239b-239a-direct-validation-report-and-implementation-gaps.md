# DEV-PLAN-239B：239A 直接验证用例测试报告与实现缺口

**状态**: 已完成（2026-03-04 16:57 CST）

## 1. 背景与范围
- 本报告用于验证 `DEV-PLAN-239A` 第 7 节“直接验证用例（Case 1 ~ Case 6）”的**实际落地完成度**，并识别实现缺口。
- 评估对象：
  - 消息桥与代理注入（`/assistant-ui`）
  - `AssistantPage` 自动执行编排
  - `/app/assistant/librechat` 独立页
  - 239A 用例对应的测试覆盖与证据完备性

## 2. 验证方法与执行记录

### 2.1 验证方法
1. [X] 静态实现核对（代码路径与关键逻辑分支）。
2. [X] 单元/组件测试复跑（Go + Web）。
3. [ ] 真实运行态端到端人工复测（按 239A Case 1~6 全量走查）。

### 2.2 已执行命令（本次）
1. [X] Go（代理与桥脚本相关）
   - 命令：`go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase|TestInjectAssistantUIProxyServiceWorkerCleanupScript|TestStripAssistantUIProxyServiceWorkerRegistration|TestWithTenantAndSession" -count=1`
   - 结果：`ok   github.com/jacksonlee411/Bugs-And-Blossoms/internal/server 0.096s`
2. [X] Web（自动执行/消息桥/AssistantPage）
   - 命令：`pnpm --dir apps/web test -- src/pages/assistant/assistantAutoRun.test.ts src/pages/assistant/assistantMessageBridge.test.ts src/pages/assistant/AssistantPage.test.tsx`
   - 结果：`Test Files 22 passed (22)`，`Tests 98 passed (98)`；其中 `assistantAutoRun`、`assistantMessageBridge`、`AssistantPage` 均通过。
3. [X] 运行态可用性快照（辅助）
   - 命令：`make assistant-runtime-status`
   - 结果：`[librechat-runtime] OK: status=healthy .../deploy/librechat/runtime-status.json`

## 3. 239A Case-by-Case 测试报告

### Case 1：通道连通（前置）
- 239A 目标：页面出现“自动执行通道已连接...”；页面不白屏、可输入。
- 实现证据：
  - 代理注入桥脚本并发送 `assistant.bridge.ready`（`internal/server/assistant_ui_proxy.go:322`）。
  - 独立页收到 `assistant.bridge.ready` 后更新状态文案（`apps/web/src/pages/assistant/LibreChatPage.tsx:398`）。
  - 代理端具备 bridge.js 单测（`internal/server/assistant_ui_proxy_test.go:373`）。
- 判定：**部分通过**（桥消息链路有实现且有测试；“白屏/输入可用”缺少自动化直测证据）。

### Case 2：一句话自动执行（完整信息）
- 239A 目标：对话触发 create -> confirm -> commit，无需右侧按钮。
- 实现证据：
  - 自动编排主链路存在（`apps/web/src/pages/assistant/AssistantPage.tsx:750`、`apps/web/src/pages/assistant/AssistantPage.tsx:616`）。
  - `assistant.prompt.sync` 消息驱动自动提交测试通过（`apps/web/src/pages/assistant/AssistantPage.test.tsx:456`）。
- 判定：**部分通过**（工作台页面路径已验证；独立页与真实后端落库尚无自动化证据）。

### Case 3：信息不充分 -> 对话补全
- 239A 目标：第一句缺字段提示，第二句补全后自动执行提交。
- 实现证据：
  - 缺字段识别与补全合并逻辑存在（`apps/web/src/pages/assistant/AssistantPage.tsx:787`、`apps/web/src/pages/assistant/assistantAutoRun.ts:66`）。
  - 文本抽取/合并有 util 测试（`apps/web/src/pages/assistant/assistantAutoRun.test.ts:32`）。
- 判定：**实现存在、验证不足**（未见 Case 3 的页面级两轮对话回归测试；独立页无对应测试文件）。

### Case 4：多候选确认（对话内完成）
- 239A 目标：候选列表 -> “选第N个/编码” -> 自动提交。
- 实现证据：
  - 候选解析与自动确认逻辑存在（`apps/web/src/pages/assistant/assistantAutoRun.ts:120`、`apps/web/src/pages/assistant/AssistantPage.tsx:729`）。
  - 页面级“选第2个，确认执行”测试通过（`apps/web/src/pages/assistant/AssistantPage.test.tsx:474`）。
- 判定：**部分通过**（工作台页面 mock 回归通过；独立页真实链路未直测）。

### Case 5：执行前确认词
- 239A 目标：待确认回合仅通过确认词推进提交。
- 实现证据：
  - 确认词识别与待确认分支存在（`apps/web/src/pages/assistant/assistantAutoRun.ts:19`、`apps/web/src/pages/assistant/AssistantPage.tsx:720`）。
  - 关键词识别基础测试通过（`apps/web/src/pages/assistant/assistantAutoRun.test.ts:48`）。
- 判定：**部分通过**（有实现与基础测试；缺少“确认词误触发/误提交”负测与独立页场景测试）。

### Case 6：结果落库核验
- 239A 目标：`/app/org/units` 可检索创建记录且生效日期一致。
- 实现证据：
  - 当前测试以 mock 为主，未提供“通过 LibreChat 对话触发后再到 OrgUnits 实际检索”的自动化证据。
- 判定：**未完成验证**（缺少端到端落库核验与可重复测试脚本）。

## 4. 总体完成度评估（239A -> 239B）
- Case 全量状态：
  - 通过：0/6
  - 部分通过：4/6（Case 1/2/4/5）
  - 实现存在但验证不足：1/6（Case 3）
  - 未完成验证：1/6（Case 6）
- 结论：239A 代码主干已落地，但“直接验证用例”尚未形成可复现的全链路验收闭环，当前更接近 **P0/P1 功能实现完成 + 验证证据未收口**。

## 5. 实现缺口清单（Gap List）

### GAP-239B-01（高）独立页自动执行缺少专属测试
- 现状：`LibreChatPage` 已实现自动执行链路，但未见 `LibreChatPage.test.tsx`。
- 风险：239A 明确验证入口是独立页，当前主要证据来自 `AssistantPage`。
- 建议：新增 `apps/web/src/pages/assistant/LibreChatPage.test.tsx`，覆盖 bridge.ready、prompt.sync、create/confirm/commit 三段链路。

### GAP-239B-02（高）Case 3 两轮补全缺少页面级回归
- 现状：有 util 测试，无页面级“第一句缺字段 + 第二句补全后提交”验证。
- 风险：补全拼装只要任一字段映射变更，容易静默退化。
- 建议：新增页面级两轮消息桥测试（AssistantPage + LibreChatPage 各一条）。

### GAP-239B-03（高）Case 6 落库核验缺失
- 现状：无“聊天 -> commit -> OrgUnits 查询”端到端验证。
- 风险：One Door 最后 1 公里（真实入库）无自动化保障。
- 建议：补充 e2e：完成聊天提交后直接到 `/app/org/units` 校验组织名与 `effective_date`。

### GAP-239B-04（中）确认词匹配过宽，存在误触发风险
- 现状：确认词正则包含 `确认|提交|执行|ok|yes` 等宽匹配（`apps/web/src/pages/assistant/assistantAutoRun.ts:19`）。
- 风险：自然对话中可能出现非提交语义但被识别为提交。
- 建议：收窄策略（精确短语 + 回合状态前置 + 明确二次确认提示），并补充负测。

### GAP-239B-05（中）239A 文档状态与证据未闭环
- 现状：239A 状态仍为“草拟中”，验收项未打勾（`docs/dev-plans/239a-librechat-dialog-auto-execution-and-standalone-page-plan.md:3`）。
- 风险：实现与契约状态不一致，影响后续计划衔接与审计可追溯性。
- 建议：在补齐 239B 缺口后更新 239A 验收勾选，并新增对应执行记录（`docs/dev-records/`）。

## 6. 239B 收口任务（建议）
1. [ ] 新增 `LibreChatPage` 单测套件（覆盖 Case 1/2/4/5 主干）。
2. [ ] 新增 Case 3 双轮补全页面级回归测试（AssistantPage + LibreChatPage）。
3. [ ] 新增 Case 6 端到端落库校验（e2e，含 `/app/org/units` 断言）。
4. [ ] 收窄确认词识别规则并补负测。
5. [ ] 补齐 239A 执行记录与状态收口（计划与证据一致）。

## 7. 关联文档
- `docs/dev-plans/239a-librechat-dialog-auto-execution-and-standalone-page-plan.md`
- `docs/dev-plans/239-librechat-chat-write-path-recovery-and-runtime-stability-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`

## 8. 缺口关闭结果（收口）
- [X] GAP-239B-01（独立页专属测试）已关闭：新增 `apps/web/src/pages/assistant/LibreChatPage.test.tsx`。
- [X] GAP-239B-02（Case 3 两轮补全页面级回归）已关闭：`AssistantPage` 与 `LibreChatPage` 均补齐。
- [X] GAP-239B-03（Case 6 落库核验）已关闭：新增 `internal/server/assistant_api_test.go` 中落库/查询断言。
- [X] GAP-239B-04（确认词误触发）已关闭：收窄确认词识别 + 新增负测。
- [X] GAP-239B-05（文档状态与证据闭环）已关闭：239A 状态回填、执行日志落盘。
