# DEV-PLAN-288：266 剩余项 C——真实入口 E2E 与证据封板收口

**状态**: 实施中（2026-03-08 CST；真实入口 E2E 已接入默认 Playwright 基线，迁移 admin 环境阻塞已排除；`292` 已完成正式入口 vendored UI 与 `sid` 会话的认证/启动最小兼容层，`288` 当前从“实现阻塞”转入“基于正式入口的默认 E2E 复跑与证据固化”阶段）

## 1. 背景
1. [X] `DEV-PLAN-266` 当前已有 mock stopline 与部分 live runtime 证据，但“真实入口自动化断言 + 完整封板证据”仍未闭环。
2. [X] `266` 若缺少真实入口自动化与证据链，`285` 封板阶段将无法判定“是否真正达成而非临时可用”。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 在 `/app/assistant/librechat` 真实入口补齐 `266` 主通过 E2E。
2. [ ] 固化 `native_send_*`、官方消息树落点、同轮唯一气泡、三元组映射等关键证据。
3. [ ] 完成 `266` 文档、执行日志、资产目录三者一致的封板记录，作为 `285` 输入。

### 2.2 非目标
1. [ ] 不新增 UI 行为改造逻辑（实现缺口必须回退到 `286/287/284` 修复）。
2. [ ] 不替代 `285` 全量封板，只负责 `266` 子域证据闭环。

## 3. 266 剩余项映射（本计划承接）
1. [ ] `266 §6.5-1`：补齐/更新真实 E2E，断言单通道、统一气泡返回、无外挂容器。
2. [ ] `266 §7-6`：`6.6` 用户可见交互变化均需有录像/截图/DOM 断言/trace 佐证。
3. [ ] `266 §8-1`：`266` 专属真实 E2E 成为主通过条件。
4. [ ] `266 §8-4`：在新承载面重建并稳定执行 `266` 真实入口回归。
5. [ ] `266 §8-5`：文档门禁通过并完成证据落盘。

## 4. 顺序与 readiness
1. [X] `DEV-PLAN-286/287` 已完成，实现侧不再存在同轮多泡或失败回落外挂风险。
2. [X] `DEV-PLAN-284` 已进入正式 patch 并满足消息树接管前提。
3. [X] 若真实入口 E2E 未稳定通过，不得在 `266` 或 `285` 宣称封板完成。
4. [X] 先前“正式入口 vendored UI 与 sid 会话缺少认证/启动闭环”的实现阻塞已由 `292` 关闭；`tp288` 当前待基于新兼容层复跑默认 `make e2e` 并补齐证据固化。

## 5. 实施步骤
1. [X] 整理并更新 `266` 专属真实入口 E2E 用例骨架，覆盖成功、失败、重试、连续多轮四类路径。
2. [X] 将现有 live-runtime runner 接入默认 E2E 基线或形成等价常规触发入口，避免 `266` 主通过依赖人工绑定运行态。
3. [ ] 基于 `292` 已完成的正式入口 vendored UI `sid` 会话认证/启动最小兼容层（已覆盖 `refresh/user/roles/config/endpoints/models/logout`）复跑默认 E2E，并为每类路径采集/固化证据：页面录屏/截图、DOM 断言、请求与 trace 片段、`native_send_*` 指标。
4. [X] 补齐证据目录结构与命名规范，统一沉淀到 `docs/dev-records/assets/dev-plan-266/`。
5. [X] 更新 `docs/dev-records/dev-plan-266-execution-log.md`，补记默认基线接线结果与当前阻塞点。
6. [ ] 输出 `266` 收口清单，作为 `285` 封板输入项。

## 6. 验收标准
1. [ ] 真实入口 E2E 稳定通过，且断言覆盖 `266` 主 stopline。
2. [X] 主通过依据不再依赖人工指定 `TP288_USE_EXISTING_RUNTIME=1` 的临时运行方式。
3. [ ] 证据链可复核：任一验收结论均可追溯到对应截图/trace/日志。
4. [ ] `266` 文档勾选状态、执行日志与证据资产三者一致，无口径冲突。
5. [X] 若证据缺口仍存在，则 `266` 维持“实施中”并回退到对应实现子计划修复。

## 7. 测试与门禁（SSOT 引用）
1. [ ] 文档门禁：`make check doc`。
2. [ ] E2E 与相关触发器命令按 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 执行。
3. [X] 本计划仅接受真实入口 `/app/assistant/librechat` 的自动化与人工证据，不接受历史别名直链作为通过依据。

## 8. 交付物
1. [X] 本计划文档：`docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`。
2. [X] `266` 真实入口 E2E 用例（`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`，现已接入默认 Playwright 基线）。
3. [X] `docs/dev-records/assets/dev-plan-266/` 证据补录与索引（含 `tp288-real-entry-evidence-index.json`）。
4. [X] `docs/dev-records/dev-plan-266-execution-log.md` 的阶段性补充记录。

## 9. 关联文档
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/286-librechat-official-message-binding-and-single-bubble-closure-plan.md`
- `docs/dev-plans/287-librechat-dto-render-only-and-failure-in-bubble-closure-plan.md`
- `AGENTS.md`
