# DEV-PLAN-240F 停止线搜索报告

- 生成时间：2026-03-09 12:31:22 CST
- 搜索范围：`AGENTS.md`、`docs/dev-plans/`、`internal/`、`e2e/`、`third_party/librechat-web/patches`
- 排除口径：`docs/archive/`、`docs/dev-records/`、第三方 vendored 上游原始源码目录
- 结论：`passed`

## 1. `/assistant-ui` 命中分类
### 允许保留
- `internal/server/assistant_ui_proxy.go`：仅保留历史别名重定向/拒绝语义，不承担正式入口职责。
- `internal/server/assistant_ui_proxy_test.go`、`internal/server/assistant_ui_proxy_log_test.go`、`internal/server/handler.go`、`internal/server/handler_test.go`、`internal/server/tenancy_middleware_test.go`：用于验证历史别名边界，不构成第二正式入口。
- `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`：显式验证 `/assistant-ui` 仅 `302` 到 `/app/assistant/librechat`，写方法 `405`。
- `e2e/tests/tp220-assistant.spec.js`：负测 `formal entry cannot bypass business write routes`，属于禁止旁路写入的验证，不是主链路入口声明。
- `docs/archive/dev-plans/283-librechat-formal-entry-cutover-plan.md`、`docs/archive/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`、`docs/archive/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`：均以“历史别名/禁止回流/边界复核”语义出现。
### 历史/调查引用
- `docs/dev-plans/220/220a/224/230/231/232/233/234/235/261/263/265/266/280/292` 中的命中均用于历史背景、迁移前置、stopline 或兼容边界说明，不构成当前正式入口定义。
### 判定
- [X] 未发现把 `/assistant-ui/*` 重新写成正式交互入口或正式写入口的主线口径。

## 2. `bridge.js`
- 代码侧仅命中 `internal/server/assistant_ui_proxy_test.go` 的别名边界测试，请求 `/assistant-ui/bridge.js` 用于验证旧桥脚本已不再承担正式职责。
- 其余命中均位于 `docs/dev-plans/230/280/281/282/283/290/291/292/240f`，语义均为“禁止恢复 / 已删除 / 历史复杂度”。
- [X] 未发现运行时代码重新依赖 `bridge.js`。

## 3. `data-assistant-dialog-stream`
- 代码/测试侧仅命中 `e2e/tests/tp290-librechat-real-case-matrix.spec.js`，用于断言外挂消息容器数量为 `0`。
- 其余命中位于 `docs/dev-plans/280/282/290/290a/291/240f`，语义均为 stopline 或历史删除说明。
- [X] 未发现外挂消息流重新承担正式回执职责。

## 4. `assistantDialogFlow` / `assistantAutoRun`
- 未在 `internal/`、`e2e/`、`third_party/librechat-web/patches` 运行时路径发现正式职责实现命中。
- 命中均位于 `docs/dev-plans/280/282/284/291/240f` 与个别历史调查计划中，用于描述“已删除/禁止恢复”的旧 helper 职责。
- [X] 未发现页面级旧 helper 重新承担正式业务编排职责。

## 5. 结论
- [X] 搜索型 stopline 未发现新的主线口径回流。
- [X] 当前残留命中全部可归类为历史别名、边界负测、禁止恢复说明或迁移背景记录。
- [X] `240F` 可以将本搜索报告直接交给 `285` 使用，无需额外解释。
