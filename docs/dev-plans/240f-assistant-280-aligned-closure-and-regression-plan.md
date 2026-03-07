# DEV-PLAN-240F：Assistant 与 280 主链路对齐封板回归计划（承接 240-M7）

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] `240` 的最终达成必须在 `283/284` 正式承载链路下验证，而非旧桥接口径。
2. [ ] 需建立 `240` 与 `260/266/280/284/285/237` 的联合回归与封板证据。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 在正式入口 `/app/assistant/librechat` 验证 `240` 编排能力与 `260` Case 语义一致。
2. [ ] 验证前端降权生效：无 helper 业务推进、无外挂消息流、无双发送。
3. [ ] 形成封板证据输入给 `285`。

### 2.2 非目标
1. [ ] 不在本计划内做新的核心功能开发。
2. [ ] 不以放宽门禁或扩大排除项替代问题修复。

## 3. 实施步骤
1. [ ] 对齐联合回归清单：`240 + 260 + 266 + 284 + 237`。
2. [ ] 跑通关键链路：plan/confirm/commit/task 与 Case 2~4。
3. [ ] 逐项验证 stopline：单入口、单消息落点、前端不重算、One Door。
4. [ ] 归档证据：截图、trace、日志、门禁结果、失败复盘。
5. [ ] 将封板输入提交至 `285`。

## 4. 停止线（Fail-Closed）
1. [ ] 若仍出现双发送/双回复/外挂回执，则本计划失败。
2. [ ] 若前端仍承担阶段推进或提交约束，则本计划失败。
3. [ ] 若升级兼容回归未完成（`237`），则不得进入封板。

## 5. 验收标准
1. [ ] `240` 剩余目标在正式承载链路下通过回归。
2. [ ] `260` Case 1~4 与 `240` 编排语义无冲突。
3. [ ] 证据完整可追溯，并可直接作为 `285` 封板输入。

## 6. 门禁与命令（SSOT 引用）
1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] `make e2e`
3. [ ] `make check no-legacy`
4. [ ] `make check routing`
5. [ ] `make check doc`
6. [ ] `make preflight`

## 7. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `AGENTS.md`
