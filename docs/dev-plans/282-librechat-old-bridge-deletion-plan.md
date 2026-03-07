# DEV-PLAN-282：LibreChat 旧桥接链路删除实施计划

**状态**: 实施中（2026-03-07 23:55 CST；旧桥正式职责已下线）

## 0. 当前实施进度（2026-03-08）
- [x] `/app/assistant/librechat` 已退役为历史入口占位页，不再承载 `iframe + postMessage` 正式职责。
- [x] `assistantAutoRun`、`assistantDialogFlow`、`assistantMessageBridge` 与 `assistantReplyFailurePayload` 已从前端主代码删除。
- [x] `assistant-ui/bridge.js` 特判、HTML 注入与 fallback shell 外挂流已从服务端主链路删除。
- [x] 旧桥专用的页面单测与 E2E 用例已删除。
- [ ] 正式入口切换、导航与验收口径统一仍由 `DEV-PLAN-283` 承接。

## 1. 背景
- 承接 `DEV-PLAN-280` 的 `280B`。
- 本计划聚焦“旧桥接负担删除”，而不是继续修补：`iframe`、`bridge.js`、HTML 注入、`data-assistant-dialog-stream`、`assistantDialogFlow`、`assistantAutoRun` 及等价职责都在删除范围内。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 删除旧桥接链路的正式职责与主要代码承载点。
2. [ ] 删除只服务旧桥接链路的测试、文案、路由别名与调试说明。
3. [ ] 将无法立即物理删除的少量残留先降为不可达、不可见、不可承担正式职责，并在同计划内继续删净。

### 2.2 非目标
1. [ ] 不在本计划内切换正式入口（由 `DEV-PLAN-283` 承接）。
2. [ ] 不在本计划内完成业务 FSM 后端化设计（以 `260/223` 为 SSOT）。

## 3. 删除清单（必须命中）
1. [ ] `iframe` 正式承载链路。
2. [ ] `bridge.js` 注入链路。
3. [ ] HTML rewrite / DOM 注入式回执。
4. [ ] `data-assistant-dialog-stream` 或等价外挂流。
5. [ ] `assistantDialogFlow`、`assistantAutoRun` 或等价页面级业务编排职责。
6. [ ] 只服务于旧桥的 E2E 断言、截图、文案与调试口径。

## 3.1 顺序与依赖
1. [ ] `282` 必须在 `281` 完成后启动，因为删除动作需要以新主链路冻结结果为目标。
2. [ ] `282` 可与 `235` 并行推进，但必须在 `283` 正式入口切换前完成“旧桥正式职责去除”。
3. [ ] `282` 未完成时，`283` 不得宣称切换完成，`285` 不得封板。

## 3.2 禁止项
1. [ ] 禁止把旧桥接代码仅标记为 deprecated 但继续承担正式职责。
2. [ ] 禁止以“后面再删”为由保留旧桥接主路径测试、主路径文案或主路径入口说明。
3. [ ] 禁止新增任何只服务旧桥接链路的兜底分支或兼容层。

## 3.3 搜索型 stopline
1. [ ] 完成 `282` 后，以下关键字不应再对应正式职责实现：`bridge.js`、`data-assistant-dialog-stream`、`assistantDialogFlow`、`assistantAutoRun`。
2. [ ] 完成 `282` 后，仓库中不应再存在把 `iframe`、`postMessage`、HTML 注入写成正式业务主链路的文档口径。
3. [ ] 若仍存在残留代码，必须能从搜索结果中明确看到其“不可达/不可见/不承担正式职责”的说明与最终删除点。

## 4. 实施步骤
1. [ ] 盘点旧桥接代码、测试、资源与文档残留。
2. [ ] 先删除正式职责，再删除物理代码与测试残留。
3. [ ] 对少量暂存残留增加“不可达/不可见/不可承担正式职责”保护。
4. [ ] 运行搜索与门禁，证明旧桥接职责未回流。

## 4.1 归档准备清单（与 283/285 联动）
1. [X] 以下文档已归档，停止新增实现口径：
   - `docs/archive/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
   - `docs/archive/dev-plans/239-librechat-chat-write-path-recovery-and-runtime-stability-plan.md`
   - `docs/archive/dev-plans/239a-librechat-dialog-auto-execution-and-standalone-page-plan.md`
   - `docs/archive/dev-plans/239b-239a-direct-validation-report-and-implementation-gaps.md`
   - `docs/archive/dev-plans/262-librechat-dialog-render-outside-chat-investigation-and-fix-plan.md`
2. [ ] 与 `DEV-PLAN-283` 对齐“正式入口唯一化”结果，确认上述文档不再承担正式验收口径。
3. [ ] 将最终物理归档动作放在 `DEV-PLAN-285` 封板步骤中一次完成，避免中途口径漂移。

## 5. 验收标准
1. [ ] 不再存在旧桥接链路承担正式用户可见业务职责。
2. [ ] 仓库中不再存在继续维护旧桥的测试主路径与文档主口径。
3. [ ] 若仍有残留代码，其职责已降为不可达/不可见，且已列明最终删除点。
4. [ ] `make check doc` 通过。

## 6. 关联文档
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/236-librechat-legacy-endpoint-retirement-and-single-source-closure-plan.md`
- `AGENTS.md`
