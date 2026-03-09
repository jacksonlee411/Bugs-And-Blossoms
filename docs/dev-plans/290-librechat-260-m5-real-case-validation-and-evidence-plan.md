# DEV-PLAN-290：260-M5 真实 Case 验收与证据固化专项

**状态**: 已完成（2026-03-09 01:55 CST；`tp290-e2e-001~004` 已在 `290A` 修复后按正式入口重跑通过，Case 1~4 全部满足 `266` 共通 stopline 与 `280` 硬门槛，可作为 `271-S5` 与 `285` 的可复核输入）

## 1. 背景
1. [X] `DEV-PLAN-260` 的最终通过依赖 Case 1~4 在真实入口完整闭环，且必须同时满足 `266` 共通 stopline。
2. [ ] `M5` 以验证和证据为主，若与实现改造混在同一计划中，容易出现“边改边验收”导致口径漂移。
3. [ ] `DEV-PLAN-271` 已将 `290` 明确为 `S5` 的 `P0` 执行单元；`DEV-PLAN-280` 已冻结正式入口、官方消息树唯一落点、前端 DTO-only、No Legacy 的硬门槛。`290` 必须用真实 Case 证据把这些门槛落盘。

## 2. 目标与非目标
### 2.1 目标
1. [X] 在 `/app/assistant/librechat` 完成 Case 1~4 真实验收。
2. [X] 固化每个 Case 的页面、网络、trace、phase 与 stopline 证据，形成可复核证据链。
3. [X] 输出 `260` 收口结论，作为 `271-S5` 与 `285` 的输入。
4. [X] 验收结论与 `280 §10.2`（单入口、无外挂、无双链路、官方消息树承载、DTO-only）逐条一致。

### 2.2 非目标
1. [ ] 不承担 `M2~M4` 实现缺口修复（由 `DEV-PLAN-289` 回退处理）。
2. [ ] 不承担升级兼容回归实施（由 `DEV-PLAN-291/237` 承接）。
3. [ ] 不修改 `260` FSM/DTO 契约定义。
4. [ ] 不恢复旧桥接口径（`iframe` 正式承载、`bridge.js`、`data-assistant-dialog-stream`、页面 helper 业务编排）。
5. [ ] 不在本计划内承载 `pending placeholder bubble` 的实现修复；该缺口由 `DEV-PLAN-290A` 承接，本计划仅负责验收重跑与证据收敛。

## 3. 顺序与依赖（对齐 271 + 280）
1. [X] 前置：`DEV-PLAN-289` 完成，`260-M2~M4` 无待修缺口。
2. [X] 前置：`266` 剩余项由 `286/287/288` 覆盖并达到可验收状态；`288` 已完成默认基线通过。
3. [X] 前置：`292` 已关闭正式入口 vendored UI 与 `sid` 会话认证/启动阻塞，默认顺序 `292 -> 288 -> 290` 已满足。
4. [X] 路线图定位：`290` 作为 `271-S5` 的 `P0` 项执行，优先级高于 `240F/285` 启动。
5. [X] 主计划对齐：`290` 的通过判定显式继承 `280` 的停止线与验收硬门槛，不接受旧桥/双入口口径回流。
6. [X] 若命中 `pending placeholder bubble`，必须先执行 `DEV-PLAN-290A` 修复，再回到 `290` 重跑 Case Matrix。
7. [X] 后置：本计划通过后方可进入 `285` 封板汇总。

## 4. 实施步骤
1. [X] 前置口径已冻结并满足：`292` 已完成最小兼容层，`288` 已形成可复核复跑通过结果；本计划从“准备阶段”切换到“Case Matrix 执行与证据固化”。
2. [X] 固定 `Case Matrix v1` 并镜像到资产目录：`docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`。
3. [X] 固定证据目录与索引模板：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`（含 4 个 Case 证据位点）。
4. [X] Case 1 验收：通道连通 + `266` 共通 stopline + `280` 单入口/单气泡断言同时成立。
5. [X] Case 2 验收：草案 -> 确认 -> 提交顺序严格成立。
6. [X] Case 3 验收：缺字段补全 -> 确认 -> 提交闭环成立。
7. [X] Case 4 验收：多候选 -> 选择 -> 二次确认 -> 提交闭环成立。
8. [X] 证据固化：每个 Case 按 4.2 节固定命名产出截图、DOM 断言、请求日志、trace 与阶段断言。
9. [X] 执行日志：本轮真实验收仅写入 `docs/dev-records/dev-plan-290-execution-log.md`；`dev-plan-260` 仅保留“引用与摘要”。
10. [X] 首轮实跑已完成并登记：命令 `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290-librechat-real-case-matrix.spec.js --workers=1 --trace on`；证据见 `docs/dev-records/assets/dev-plan-290/`，结论见执行日志。
11. [X] 已按同口径多轮重跑并覆盖证据资产；截至 `2026-03-09 01:54 CST`，Case 1~4 均通过，Case 2/3/4 已清除 pending placeholder bubble。
12. [X] 修复分流：`pending placeholder bubble` 实现修复转入 `DEV-PLAN-290A`；本计划在 `290A` 完成后执行同口径回归重跑并更新通过判定。

### 4.1 Case Matrix v1（冻结输入向量）
> 本矩阵承接 `DEV-PLAN-260` 第 2.2 节 Case 1~4，作为 `290` 的唯一复跑口径。

| Case | 固定输入序列（按轮次） | 关键阶段断言（后端 phase） | 通过判定 |
| --- | --- | --- | --- |
| Case 1 | T1：`你好` | `idle -> idle`（不得进入提交链） | `/app/assistant/librechat` 可发送、可回包，且同轮满足 `266 stopline + 280 单入口约束` |
| Case 2 | T1：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`；T2：`确认` | `idle -> await_commit_confirm -> committing -> committed` | 对话内完成草案、确认、提交、成功回执 |
| Case 3 | T1：`在 AI治理办公室 下新建 人力资源部239A补全`；T2：`生效日期 2026-03-25`；T3：`确认` | `idle -> await_missing_fields -> await_commit_confirm -> committing -> committed` | 对话内提示缺字段、补全后确认并提交成功 |
| Case 4 | T1：`在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26`；T2：`选第2个`（或候选编码）；T3：`是的` | `idle -> await_candidate_pick -> await_candidate_confirm -> await_commit_confirm -> committing -> committed` | 对话内完成候选选择、二次确认并提交成功 |

补充规则：
1. [X] `Case Matrix v1` 的默认确认词冻结为 `确认` / `是的`；若运行时接受同义词，验收仍以本矩阵默认输入为准。
2. [ ] 仅当候选顺序在运行时不稳定时，Case 4 才允许改用“候选编码”；必须在执行日志记录原因与实际输入。
3. [ ] 任一 Case 输入向量偏离本矩阵且未在执行日志说明，不计入通过统计。

### 4.2 证据目录与命名冻结
1. [X] 证据根目录冻结为：`docs/dev-records/assets/dev-plan-290/`。
2. [X] 每个 Case 必须产出以下固定文件（`{id}` 取 `1..4`）：
   - `case-{id}-page.png`
   - `case-{id}-dom.json`
   - `case-{id}-network.har`
   - `case-{id}-trace.zip`
   - `case-{id}-phase-assertions.json`
3. [X] 汇总索引文件冻结为：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`。
4. [X] `Case Matrix v1` 资产副本冻结为：`docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`。

### 4.3 280 硬门槛映射（每个 Case 必须同时满足）
1. [X] `280 §10.2-1/4`：正式承载面唯一为 `/app/assistant/librechat`，不得回流为双正式入口或双渲染口径。
2. [X] `280 §10.2-2/3`：不得依赖 `bridge.js` 或 `data-assistant-dialog-stream` 等外挂链路显示回执。
3. [X] `280 §10.2-5`：每轮只有唯一 assistant 回复实体，并落在官方消息列表组件树。
4. [X] `280 §10.2-6`：前端只消费后端 DTO，不在页面 helper/adapter 重算业务 FSM、候选裁决、提交约束。
5. [X] `280 §8.4 + 271 §5.5/§6`：若出现双发送、双回复、双事实源，立即判失败并回退到对应实现子计划修复。

## 5. 验收标准
1. [X] Case 1~4 全部通过，且每个 Case 均满足 `266` 共通 stopline 与 4.3 节 `280` 硬门槛映射。
2. [X] 任一 Case 出现双链路、外挂回复、同轮多泡、双正式入口或官方原始错误体验即判失败。
3. [X] 证据可追溯、可复核、可重复执行。
4. [ ] 若 `288` 尚未基于 `292` 产物完成默认基线复跑与证据固化，则本计划不得更新为“已完成”。
5. [X] 任一 Case 未按 4.1 节固定输入向量执行，或未产出 4.2 节固定命名证据文件，不得判定通过。
6. [ ] 若 `240C/240D/240E` 有影响性合入（运行时 gate、路由/认证链路、错误码语义、MCP 写能力准入、fail-closed 行为），`290` 历史证据立即失效，必须基于最新代码重跑并刷新索引（对齐 `271`）。
7. [X] 若出现 `pending placeholder bubble (binding_key=::::)` 或等价“未绑定正式 turn/request 的遗留 assistant 气泡”，判定为 `single_assistant_bubble` 与 `official_message_tree_only` 未通过，`290` 不得完成。

## 6. 测试与门禁（SSOT 引用）
1. [X] 触发器与命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [X] 文档改动至少通过 `make check doc`。

## 7. 交付物
1. [X] 本计划文档：`docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`。
2. [X] Case 1~4 验收记录与证据资产索引模板：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`。
3. [X] `Case Matrix v1` 资产副本：`docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`。
4. [X] 执行日志：`docs/dev-records/dev-plan-290-execution-log.md`（已进入实施中，待补 Case 通过结果）。
5. [X] 面向 `285` 的 `260` 收口结论。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/289-librechat-260-m2-m4-implementation-closure-plan.md`
- `docs/dev-plans/290a-librechat-pending-placeholder-bubble-fix-plan.md`
- `docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`
- `docs/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- `docs/dev-records/dev-plan-290-execution-log.md`
- `AGENTS.md`

## 9. 前置发现登记（承接 288，2026-03-08）
1. [X] `292` 已关闭认证/启动兼容阻塞，`290` 不再受 auth/startup 链路阻塞。
2. [X] `288` 已完成渲染主路径修复并通过默认基线，`tp288` 已满足 `data-assistant-binding-key` DOM 命中与 retry 新增气泡断言。
3. [X] `290` 启动门槛已满足：可进入 Case 1~4 正式通过判定与证据固化。
4. [X] 单链路原则补充：`290` 执行阶段若发现再次依赖 `message.content` 缺失回退链才能通过，视为 `288` 回归，必须回退到 `288` 修复，不得以 Case 结果“暂时可用”替代结构性收口。
5. [X] 路线图对齐补充：`290` 已按 `271-S5` 重新标注为 P0 执行项，并继承 `280` 的正式入口/官方消息树/DTO-only/No Legacy 验收门槛。
