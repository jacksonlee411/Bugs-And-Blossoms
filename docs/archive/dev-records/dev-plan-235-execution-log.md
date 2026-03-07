# DEV-PLAN-235 执行记录（LibreChat 身份/会话/租户边界硬化）

## 1. 记录信息
- 计划：`docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- 记录时间：2026-03-03 15:10 UTC
- 记录人：Codex

## 2. 本次交付范围
1. 将 `/assistant-ui` 与 `/assistant-ui/*` 纳入与 `/app/**` 一致的租户+会话+主体校验链路（不再允许匿名旁路）。
2. 强化 `assistant-ui` 代理边界：仅允许 `GET`，路径限制为 `/assistant-ui` 前缀，请求/响应头最小透传并剥离敏感头。
3. 补齐单测与回归用例，覆盖未登录、跨租户 SID 复用、方法越界与旁路写防护场景。

## 3. 代码与测试证据
- 代码提交：
  - `295c281`：`fix(assistant-ui): harden session and proxy boundaries`
  - `a9dc456`：`test(assistant-ui): cover cross-tenant session mismatch redirect`
- 关键变更文件：
  - `internal/server/handler.go`
  - `internal/server/assistant_ui_proxy.go`
  - `internal/server/tenancy_middleware_test.go`
  - `internal/server/assistant_ui_proxy_test.go`
  - `internal/server/handler_test.go`
- 验收命令（2026-03-03 UTC）：
  - `go test ./internal/server -count=1` ✅
  - `make check routing` ✅
  - `make check capability-route-map` ✅
  - `make check error-message` ✅
  - `make e2e` ✅（13/13 通过，含 `tp220-e2e-007: librechat shell cannot bypass business write routes`）

## 4. 运行态样例（8080 本地验证）
- 正向样例：
  - `POST /iam/api/sessions`（`Host: localhost:8080`）返回 `204`，并设置 `Set-Cookie: sid=...`。
  - 同租户携带 SID 访问业务 API（`Host: saas.localhost:8080`）返回 `200`。
- 负向样例：
  - 未登录访问 `GET /assistant-ui` 返回 `302`，`Location: /app/login`。
  - 租户 A 的 SID 访问租户 B 的业务 API 返回 `401 unauthorized`。
  - `POST /assistant-ui` 返回 `405 Method Not Allowed`（方法越界阻断）。
  - 租户不匹配 SID 访问 `GET /assistant-ui` 返回 `302`，重定向到 `/app/login`。

## 5. 备注
- 首轮 `/assistant-ui` 代理期的 `502` 行为仅代表 2026-03-03 阶段性现状；自 2026-03-07 第二轮 cutover 对齐后，现行语义已收敛为 `GET/HEAD /assistant-ui/* -> 302 /app/assistant/librechat`。
- 本计划未新增数据库表/迁移，符合 “No Legacy / One Door / fail-closed” 约束。

## 6. 第二轮对齐（2026-03-07 CST）
- 目标：按 `DEV-PLAN-280/281/283` 的正式入口口径，补齐 `/app/assistant/librechat` 与 `/assets/librechat-web/**` 边界，并将 `/assistant-ui/*` 收敛为历史别名。
- 本轮交付：
  1. 新增正式入口服务端子树：`/app/assistant/librechat`。
  2. 冻结并接入受保护静态路径：`/assets/librechat-web/**`。
  3. 历史别名代理进一步收敛为：`GET/HEAD /assistant-ui/* -> 302 /app/assistant/librechat`，非允许方法继续 `405`。
  4. 更新 `DEV-PLAN-235` 契约，把正式入口 stopline、正式静态资源边界与历史别名 cutover 语义写回 SSOT。
- 关键变更文件：
  - `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
  - `internal/server/librechat_web_ui.go`
  - `internal/server/handler.go`
  - `internal/server/assistant_ui_proxy.go`
  - `config/routing/allowlist.yaml`
  - `scripts/librechat-web/common.sh`
  - `internal/server/librechat_web_ui_test.go`
  - `internal/server/assistant_ui_proxy_extra_test.go`
- 本轮验证：
  - `make librechat-web-build` ✅
  - `go test ./internal/server -count=1` ✅
  - `make check routing` ✅
  - `make check doc` ✅
