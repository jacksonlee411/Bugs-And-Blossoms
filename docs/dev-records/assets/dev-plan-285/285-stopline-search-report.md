# DEV-PLAN-285 Stopline Search Report

- 生成时间：2026-03-09 12:37:57 CST
- 搜索范围：`AGENTS.md`、`docs/dev-plans/`、`internal/`、`e2e/`
- 结论：`passed`

## 1. 旧入口 `/assistant-ui/*`
- 保留命中集中于：
  - 历史别名重定向/拒绝实现：`internal/server/assistant_ui_proxy.go`
  - 边界测试：`internal/server/assistant_ui_proxy_test.go`、`e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`
  - 历史/迁移/兼容口径文档：`235/283/285/291` 及更早调查计划
- 结论：未发现把 `/assistant-ui/*` 重新写成正式交互入口或正式写入口的主线口径。

## 2. 旧桥职责关键字
- `bridge.js`：未在运行时代码中承担正式职责；命中均为历史删除、禁止恢复或边界测试。
- `data-assistant-dialog-stream`：运行时仅在 E2E 中作为“外挂容器数量应为 0”的断言目标出现。
- `assistantDialogFlow` / `assistantAutoRun`：未在当前运行时代码承担正式业务编排职责；命中仅用于历史删除或禁止恢复说明。
- 结论：未发现旧桥职责回流。

## 3. 归档引用稳定性
- `docs/archive/dev-plans/222/239/239a/239b/262` 相关文件均存在。
- 主线命中均指向 `docs/archive/dev-plans/` 或明确写为历史阶段说明，不存在把归档文档重新作为正式承载方案的主线口径。
- 结论：归档文档引用稳定，无主线回流。

## 4. 综合结论
- [X] 无双入口口径回流。
- [X] 无旧桥正式职责回流。
- [X] 无旧测试口径回流为主验收标准。
- [X] 本搜索报告可作为 `285` 的封板 stopline 直接证据。
