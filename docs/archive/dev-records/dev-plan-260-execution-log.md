# DEV-PLAN-260 执行日志（对话闭环自动执行）

**状态**: 已验收通过（2026-03-06 04:39 CST）

## 1. 本次落地范围
1. [X] Web：新增共享对话 FSM helper（`assistantDialogFlow.ts`）并接入 `AssistantPage` / `LibreChatPage`。
2. [X] Web：改造自动执行语义为“先草案、后确认、再提交”；多候选改为“选择后再二次确认”。
3. [X] Web：确认词扩展支持 `是的，确认`。
4. [X] Go 代理：bridge 注入脚本支持 `assistant.flow.dialog`，业务回执渲染到聊天流容器，不再依赖浮层层级容器。
5. [X] 测试：更新/新增前端与 Go 用例覆盖 260 新语义。
6. [X] E2E：新增 `e2e/tests/tp260-librechat-dialog-closure.spec.js`（Case 1~4 串行闭环）。

## 2. 已执行命令与结果
1. [X] `pnpm --dir apps/web test -- src/pages/assistant/assistantDialogFlow.test.ts src/pages/assistant/assistantAutoRun.test.ts src/pages/assistant/assistantMessageBridge.test.ts src/pages/assistant/LibreChatPage.test.tsx src/pages/assistant/AssistantPage.test.tsx`
   - 结果：`24 passed`，`111 passed`。
2. [X] `go test ./internal/server -run "TestAssistantUIProxyHandler|TestServeAssistantUIBridgeScript|TestRewriteAssistantUIProxyHTMLBase" -count=1`
   - 结果：`ok github.com/jacksonlee411/Bugs-And-Blossoms/internal/server`。
3. [X] `make check doc`
   - 结果：`[doc] OK`。
4. [X] `pnpm --dir e2e exec playwright test tests/tp260-librechat-dialog-closure.spec.js`
   - 结果：`1 passed`（Case 1~4 串行闭环通过）。

## 3. 用例达成状态（按 260 Case 1~4）
1. [X] Case 1（连通提示）：单测覆盖（`LibreChatPage.test.tsx`）。
2. [X] Case 2（完整信息先草案后确认）：双页单测覆盖。
3. [X] Case 3（缺字段补全后确认提交）：双页单测覆盖。
4. [X] Case 4（多候选选择 + 二次确认）：双页单测覆盖。
5. [X] 本地 E2E 实跑证据：`tp260-librechat-dialog-closure.spec.js` 已通过。

## 4. 最终验收补充（按 dev-login skill 重启与验证）
1. [X] 按 skill 重启关键服务并重建登录身份：
   - `DATABASE_URL=... make iam migrate up`
   - `make dev-kratos-stub`（重启后重新 seed 3 个租户测试账号）
   - `DEV_SERVER_ENV_FILE=.env make dev-server`
2. [X] 门禁与全量测试补跑：
   - `GOPROXY=https://goproxy.cn,direct make check lint` -> 通过
   - `make test` -> 通过（coverage gate 100%）
   - `make check doc` -> 通过
3. [X] 登录链路与会话边界快验（skill 清单）：
   - `POST /iam/api/sessions`（Host=localhost）返回 `204`，含 `Set-Cookie: sid=...`
   - 未登录访问 `/assistant-ui` 返回 `302`，`Location: /app/login`
   - 已登录访问 `/assistant-ui` 返回 `200`
   - LibreChat runtime 健康检查：`http://127.0.0.1:3080/health` 返回 `200 OK`

## 5. 环境备注
1. `make dev-up` 在当前机器受 Docker socket 权限限制失败（`/var/run/docker.sock permission denied`）。
2. 本次验收使用已有运行中的 PostgreSQL/Redis/LibreChat 进程（端口 `5438/6379/3080`）完成，不影响 260 功能验收结论。

## 6. LibreChat 实时对话验收（最终完成条件）
1. [X] 在真实运行态执行 `/app/assistant/librechat` 对话（非 mock）并得到业务回执：
   - 流程：IAM 登录（`admin@localhost/admin123`）-> iframe 内 LibreChat 登录（`admin@localhost.local/admin123`）-> 发送提示词。
   - 提示词：`请输出严格JSON，不要解释：{"action":"plan_only"}`
   - 结果：聊天流出现 `已生成提交草案...`，且未出现 `decode failed`。
2. [X] 验收命令（Playwright 实跑）输出：
   - `PASS: librechat live conversation reached draft response in /app/assistant/librechat`

## 7. 会话列表/运行态不可用问题修复（用户现场问题）
1. [X] 处理 `assistant_runtime_dependency_unavailable`：
   - 修复 `deploy/librechat/healthcheck.sh`，当 `upstream(3080)` 可达但本地 compose 容器不可见时，按 `external_upstream_managed` 标记健康并生成 `status=healthy` 快照。
   - 复验：`make assistant-runtime-status` 输出 `OK: status=healthy`；`GET /internal/assistant/runtime-status` 返回 `status=healthy`。
2. [X] 处理“助手无回复/会话列表验证不通过”：
   - 在 `AssistantPage`/`LibreChatPage` 增加结构化自动重试：首次 `create turn` 命中 `ai_plan_schema_constrained_decode_failed|ai_model_timeout|ai_model_rate_limited|ai_model_provider_unavailable` 时，自动重试结构化提示词并继续闭环。
   - 复验（真实链路）：在 `/app/assistant/librechat` 输入 `在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`，收到助手业务回执（含“模型返回非结构化内容，已自动重试并生成可确认草案”与草案文案），不再停留无回复状态。
   - 会话列表复验：`GET /internal/assistant/conversations?page_size=5` 返回最新会话项，`last_turn.user_input` 为自动重试后的结构化提示词。
