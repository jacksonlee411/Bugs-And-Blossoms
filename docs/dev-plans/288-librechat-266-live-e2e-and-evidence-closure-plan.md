# DEV-PLAN-288：266 剩余项 C——真实入口 E2E 与证据封板收口

**状态**: 已完成（2026-03-09 10:55 CST；`240D-03/04` cutover 后已按 `271-S5` 时序要求立刻重跑 `tp288-e2e-001/002` 并通过；固定命名产物与索引已刷新，`266` 子域证据仍可继续供 `285` 直接引用）

## 1. 背景
1. [X] `DEV-PLAN-266` 当前已有 mock stopline 与部分 live runtime 证据，但“真实入口自动化断言 + 完整封板证据”仍未闭环。
2. [X] `266` 若缺少真实入口自动化与证据链，`285` 封板阶段将无法判定“是否真正达成而非临时可用”。

## 2. 目标与非目标
### 2.1 目标
1. [X] 在 `/app/assistant/librechat` 真实入口补齐 `266` 主通过 E2E。
2. [X] 固化 `native_send_*`、官方消息树唯一落点、同轮唯一 assistant 气泡、`conversation_id/turn_id/request_id` 三元组到唯一气泡映射等关键证据。
3. [X] 完成 `266` 文档、执行日志、证据索引与交接清单四者一致的封板记录，作为 `285` 输入。

### 2.2 非目标
1. [X] 不新增 UI 行为改造逻辑（实现缺口必须回退到 `286/287/284` 修复）。
2. [X] 不替代 `285` 全量封板，只负责 `266` 子域证据闭环。
3. [X] 不承接 `pending placeholder bubble` 的实现修复；该缺口由 `290A` 承接，`288` 仅在其合入后按需刷新受影响证据。

## 3. 266 剩余项映射（本计划承接）
1. [X] `266 §6.5-1`：真实入口 E2E 已补齐并覆盖单通道、统一气泡返回、无外挂容器的主 stopline。
2. [X] `266 §7-6`：`6.6` 用户可见交互变化已通过固定命名的截图/DOM 断言/trace/网络索引化固化。
3. [X] `266 §8-1`：`266` 专属真实 E2E 已进入默认 Playwright 基线，成为主通过依据的一部分。
4. [X] `266 §8-4`：已基于 `292` 新兼容层完成默认基线复跑，真实入口回归不再依赖人工绑定运行态。
5. [X] `266 §8-5`：`make check doc`、证据索引刷新、文档勾选一致性与交接清单已完成。
6. [X] `266` 未闭环硬门槛映射：已补齐 `single_assistant_bubble` 与 `conversation_id/turn_id/request_id -> 唯一 assistant bubble` 的可复核证据映射。

## 4. 顺序与 readiness
1. [X] `DEV-PLAN-286/287` 已完成，实现侧不再存在同轮多泡或失败回落外挂风险。
2. [X] `DEV-PLAN-284` 已进入正式 patch 并满足消息树接管前提。
3. [X] 若真实入口 E2E 未稳定通过，不得在 `266` 或 `285` 宣称封板完成。
4. [X] 先前“正式入口 vendored UI 与 sid 会话缺少认证/启动闭环”的实现阻塞已由 `292` 关闭；`tp288` 已基于新兼容层完成默认基线复跑并通过（`001/002`）。
5. [X] `pending placeholder bubble` 的实现修复已由 `290A` 承接；`288` 不吞并该实现缺口，只维护 `266` 子域证据闭环与交接输入。
6. [X] 若 `290A` 或 `240C/240D/240E` 后续合入触达消息绑定、渲染路径、路由/认证链路、错误码语义或 fail-closed 行为，则 `tp288` 历史证据立即失效，必须按 `271` 规则重跑并刷新索引后再判定完成。

## 5. 实施步骤
1. [X] 整理并更新 `266` 专属真实入口 E2E 用例骨架，覆盖成功、失败、重试、连续多轮四类路径。
2. [X] 将现有 live-runtime runner 接入默认 E2E 基线或形成等价常规触发入口，避免 `266` 主通过依赖人工绑定运行态。
3. [X] 基于 `292` 已完成的正式入口 vendored UI `sid` 会话认证/启动最小兼容层（已覆盖 `refresh/user/roles/config/endpoints/models/logout`）完成默认 E2E 复跑，`tp288-e2e-001/002` 已通过；固定命名的截图、DOM、网络、trace 与 `native_send_*` 断言资产均已固化。
4. [X] 补齐证据目录结构与命名规范，统一沉淀到 `docs/dev-records/assets/dev-plan-266/`。
5. [X] 更新 `docs/dev-records/dev-plan-266-execution-log.md`，补记默认基线接线结果、`tp288` 通过结果与环境前置（含 `TRUST_PROXY=1`）。
6. [X] 刷新 `docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json` 结构，至少为 `tp288-e2e-001/002` 固定记录 `command/executed_at/artifacts/assertions/result`，使每条 stopline 结论可回溯。
7. [X] 输出 `266` 收口清单，固定为 `docs/dev-records/assets/dev-plan-266/tp288-handoff-to-285.md`，供 `285` 直接引用。
8. [X] 若 `290A` 合入影响消息绑定或渲染生命周期，执行同口径 `tp288` 重跑并刷新索引时间戳，再更新本计划完成判定。

## 6. 验收标准
1. [X] 真实入口 E2E 稳定通过，且断言覆盖 `266` 主 stopline。
2. [X] 主通过依据不再依赖人工指定 `TP288_USE_EXISTING_RUNTIME=1` 的临时运行方式。
3. [X] `tp288-real-entry-evidence-index.json` 已按用例维度列出 `tp288-e2e-001/002` 的执行命令、执行时间、证据文件、关键断言与结论；任一结论均可追溯到对应截图/trace/日志。
4. [X] 证据链已明确证明：`single_assistant_bubble=true`，且同轮 `conversation_id/turn_id/request_id` 与唯一 assistant 气泡一一对应，不存在串泡、外挂回执或双写。
5. [X] `266` 文档勾选状态、`288` 计划状态、`docs/dev-records/dev-plan-266-execution-log.md` 与证据资产/交接清单四者一致，无口径冲突。
6. [X] 文档门禁 `make check doc` 已通过，且产物时间戳晚于最近一次影响 `tp288` 结论的本轮文档/证据更新。
7. [X] 若证据缺口仍存在，则 `266` 维持“实施中”并回退到对应实现子计划修复。

## 7. 测试与门禁（SSOT 引用）
1. [X] 文档门禁：`make check doc`。
2. [X] E2E 与相关触发器命令按 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 执行。
3. [X] 本计划仅接受真实入口 `/app/assistant/librechat` 的自动化与人工证据，不接受历史别名直链作为通过依据。
4. [X] 若 `290A` 或 `240C/240D/240E` 有影响性合入，必须补跑 `tp288` 并刷新证据索引，不得沿用旧结论直接封板。

## 8. 交付物
1. [X] 本计划文档：`docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`。
2. [X] `266` 真实入口 E2E 用例（`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`，现已接入默认 Playwright 基线）。
3. [X] `docs/dev-records/assets/dev-plan-266/` 证据补录与索引（含 `tp288-real-entry-evidence-index.json`）。
4. [X] `docs/dev-records/dev-plan-266-execution-log.md` 的阶段性补充记录。
5. [X] 面向 `285` 的 `266` 收口清单：`docs/dev-records/assets/dev-plan-266/tp288-handoff-to-285.md`。

## 9. 关联文档
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/286-librechat-official-message-binding-and-single-bubble-closure-plan.md`
- `docs/dev-plans/287-librechat-dto-render-only-and-failure-in-bubble-closure-plan.md`
- `docs/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`
- `AGENTS.md`

## 10. 最新进展与收口策略（2026-03-08）
1. [X] 启动链阻塞已关闭：`localhost` Service Worker 注册问题与 `/app/assistant/librechat/api/**` 的 `sid/auth` 启动兼容问题均已修复并落入 patch stack。
2. [X] 渲染主路径阻塞已关闭：`components/Messages/*` 主路径已命中 formal 渲染，`data-assistant-binding-key` 已进入 DOM。
3. [X] retry 二轮覆盖问题已关闭：同会话 retry 会新增 assistant 气泡而非覆盖首轮，`tp288-e2e-002` 通过。
4. [X] 默认基线验证结果：`tp288-e2e-001/002` 已通过，且已按固定命名产物完成截图、DOM、网络、trace 与断言固化。
5. [X] 与 `290A` 的边界已冻结：`binding_key=::::` 对应的 pending placeholder bubble 修复由 `290A` 承接；`288` 不再吸收该实现缺口。
6. [X] 本轮已补齐 `docs/dev-records/assets/dev-plan-266/` 证据索引与 `266` 收口清单，`266/288` 文档勾选、执行日志、资产索引与交接清单现已一致。
7. [X] 新鲜度规则：若 `290A` 或 `240C/240D/240E` 合入触发 `271-S5` 证据失效条件，必须先刷新 `tp288` 证据，再允许 `285` 引用本计划产物。

## 11. 卡点、解法与经验沉淀（288 复盘）
1. [X] 卡点：正式入口启动链早期被 `localhost` SW 注册与 `sid/auth` 兼容缺口阻塞，表现为白屏/401/登录回跳。
   - 解法：在 vendored patch stack 中补齐 `localhost` SW 与 `/app/assistant/librechat/api/**` 启动兼容，形成 `292 -> 288` 的固定前置顺序。
   - 经验：`288` 复跑前必须先验证 `292` 前置是否生效，避免把启动链问题误判为业务渲染问题。
2. [X] 卡点：`make librechat-web-build` 后直接复跑仍失败，出现“已改代码但页面不变”的假阴性。
   - 解法：明确 Go `go:embed` 产物只有在 server 进程重启后才会生效；将“重建 + 重启服务”作为固定步骤。
   - 经验：凡命中 `internal/server/assets/librechat-web/**` 变更，必须执行“重建前端产物 -> 重启服务 -> 再跑 E2E”三步，不可省略。
3. [X] 卡点：消息状态层已有 `assistantFormalPayload/bindingKey`，但 DOM 不出现 `data-assistant-binding-key`。
   - 解法：定位到渲染链分叉，主路径实际命中 `components/Messages/*` 与 `Chat/Messages/MessageParts`，将 formal 渲染短路与 remount key 补到真实命中路径。
   - 经验：排查渲染问题时先确认“真实命中组件树”，不要仅依据历史补丁位置判断。
4. [X] 卡点：retry 第二轮覆盖第一轮气泡，`tp288-e2e-002` 期望两轮气泡但实际只有一轮。
   - 解法：修正 `upsertAssistantFormalMessage` 对“同 messageId + 新 pending”场景的匹配策略，避免覆盖已绑定旧 turn 的消息，并补单测固化。
   - 经验：消息 upsert 逻辑必须以 `bindingKey` 作为主识别键，`messageId` 只能用于同轮占位更新，不能跨轮复用覆盖。
5. [X] 卡点：E2E 断言存在“同一文本既要求在气泡内存在，又要求全页为 0”的冲突，导致误报。
   - 解法：将该类断言修正为“全页计数 = 1（且不重复）”，保持“无外挂重复气泡”的真实语义。
   - 经验：禁止使用自相矛盾断言表达“唯一性”；应改为“目标容器命中 + 全页唯一计数”组合断言。
6. [X] 卡点：手写 patch 容易出现 malformed hunk，导致构建阶段 patch 应用失败。
   - 解法：统一改为“从原文件生成 diff”产出 patch，并以 `make librechat-web-build` 作为 patch stack 验证。
   - 经验：`third_party/librechat-web/patches/*` 变更后，构建成功是唯一有效校验；只看源码 diff 不足以证明可用。
7. [X] 卡点：Playwright 偶发 `Internal error: step id not found` 噪声干扰判断。
   - 解法：以业务断言结果为准，不把该噪声当根因；结合截图/trace 判断真实失败点。
   - 经验：测试框架噪声与业务失败要分层定位，避免把 runner 噪声误当产品缺陷。
