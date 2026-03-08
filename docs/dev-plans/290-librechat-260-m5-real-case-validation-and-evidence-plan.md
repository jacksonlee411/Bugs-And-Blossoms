# DEV-PLAN-290：260-M5 真实 Case 验收与证据固化专项

**状态**: 规划中（2026-03-08 CST；`289` 已完成，`292` 已完成正式入口 vendored UI 与 `sid` 会话的认证/启动最小兼容层；当前等待 `288` 基于新兼容层复跑并固化可复核证据后，再进入 Case 1~4 的正式通过判定）

## 1. 背景
1. [ ] `DEV-PLAN-260` 的最终通过依赖 Case 1~4 在真实入口完整闭环，且必须同时满足 `266` 共通 stopline。
2. [ ] `M5` 以验证和证据为主，若与实现改造混在同一计划中，容易出现“边改边验收”导致口径漂移。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 在 `/app/assistant/librechat` 完成 Case 1~4 真实验收。
2. [ ] 固化每个 Case 的页面、网络、trace 与状态证据，形成可复核证据链。
3. [ ] 输出 `260` 收口结论，作为 `271-S5` 与 `285` 的输入。

### 2.2 非目标
1. [ ] 不承担 `M2~M4` 实现缺口修复（由 `DEV-PLAN-289` 回退处理）。
2. [ ] 不承担升级兼容回归实施（由 `DEV-PLAN-291/237` 承接）。
3. [ ] 不修改 `260` FSM/DTO 契约定义。

## 3. 顺序与依赖
1. [ ] 前置：`DEV-PLAN-289` 完成，`260-M2~M4` 无待修缺口。
2. [ ] 前置：`266` 剩余项由 `286/287/288` 覆盖并达到可验收状态。
3. [ ] 后置：本计划通过后方可进入 `285` 封板汇总。
4. [X] 当前最近实现阻塞已由 `292` 关闭；`290` 的默认前置顺序仍冻结为 `292 -> 288 -> 290`，当前等待 `288` 基于新兼容层完成复跑取证前，`290` 暂不应输出最终 Case 通过结论。

## 4. 实施步骤
1. [X] 前置口径已冻结：`292` 已完成最小兼容层；在 `288` 未据此形成可复核复跑证据前，本计划只准备 Case matrix、断言清单与证据目录，不提前宣称 Case 通过。
2. [ ] 固定 `Case Matrix v1` 并执行：Case 1~4 必须按 4.1 节的输入向量复跑，不接受临场改词替代。
3. [ ] Case 1 验收：通道连通 + `266` 共通 stopline 同时成立。
4. [ ] Case 2 验收：草案 -> 确认 -> 提交顺序严格成立。
5. [ ] Case 3 验收：缺字段补全 -> 确认 -> 提交闭环成立。
6. [ ] Case 4 验收：多候选 -> 选择 -> 二次确认 -> 提交闭环成立。
7. [ ] 证据固化：每个 Case 按 4.2 节固定命名产出截图、DOM 断言、请求日志、trace 与阶段断言。
8. [ ] 执行日志：本轮真实验收仅写入 `docs/dev-records/dev-plan-290-execution-log.md`；`dev-plan-260` 仅保留“引用与摘要”。

### 4.1 Case Matrix v1（冻结输入向量）
> 本矩阵承接 `DEV-PLAN-260` 第 2.2 节 Case 1~4，作为 `290` 的唯一复跑口径。

| Case | 固定输入序列（按轮次） | 关键阶段断言（后端 phase） | 通过判定 |
| --- | --- | --- | --- |
| Case 1 | T1：`你好` | `idle -> idle`（不得进入提交链） | `/app/assistant/librechat` 可发送、可回包，且同轮满足 `266` stopline |
| Case 2 | T1：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`；T2：`确认` | `idle -> await_commit_confirm -> committing -> committed` | 对话内完成草案、确认、提交、成功回执 |
| Case 3 | T1：`在 AI治理办公室 下新建 人力资源部239A补全`；T2：`生效日期 2026-03-25`；T3：`确认` | `idle -> await_missing_fields -> await_commit_confirm -> committing -> committed` | 对话内提示缺字段、补全后确认并提交成功 |
| Case 4 | T1：`在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26`；T2：`选第2个`（或候选编码）；T3：`是的` | `idle -> await_candidate_pick -> await_candidate_confirm -> await_commit_confirm -> committing -> committed` | 对话内完成候选选择、二次确认并提交成功 |

补充规则：
1. [ ] `Case Matrix v1` 的默认确认词冻结为 `确认` / `是的`；若运行时接受同义词，验收仍以本矩阵默认输入为准。
2. [ ] 仅当候选顺序在运行时不稳定时，Case 4 才允许改用“候选编码”；必须在执行日志记录原因与实际输入。
3. [ ] 任一 Case 输入向量偏离本矩阵且未在执行日志说明，不计入通过统计。

### 4.2 证据目录与命名冻结
1. [ ] 证据根目录冻结为：`docs/dev-records/assets/dev-plan-290/`。
2. [ ] 每个 Case 必须产出以下固定文件（`{id}` 取 `1..4`）：
   - `case-{id}-page.png`
   - `case-{id}-dom.json`
   - `case-{id}-network.har`
   - `case-{id}-trace.zip`
   - `case-{id}-phase-assertions.json`
3. [ ] 汇总索引文件冻结为：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`。
4. [ ] `Case Matrix v1` 资产副本冻结为：`docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`。

## 5. 验收标准
1. [ ] Case 1~4 全部通过，且每个 Case 均满足 `266` 共通 stopline。
2. [ ] 任一 Case 出现双链路、外挂回复、同轮多泡或官方原始错误体验即判失败。
3. [ ] 证据可追溯、可复核、可重复执行。
4. [ ] 若 `288` 尚未基于 `292` 产物完成默认基线复跑与证据固化，则本计划不得更新为“已完成”。
5. [ ] 任一 Case 未按 4.1 节固定输入向量执行，或未产出 4.2 节固定命名证据文件，不得判定通过。
6. [ ] 若 `240C/240D/240E` 有影响性合入（运行时 gate、路由/认证链路、错误码语义、MCP 写能力准入、fail-closed 行为），`290` 历史证据立即失效，必须基于最新代码重跑并刷新索引（对齐 `271`）。

## 6. 测试与门禁（SSOT 引用）
1. [ ] 触发器与命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 文档改动至少通过 `make check doc`。

## 7. 交付物
1. [ ] 本计划文档：`docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`。
2. [ ] Case 1~4 验收记录与证据资产索引：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`。
3. [ ] `Case Matrix v1` 资产副本：`docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`。
4. [ ] 执行日志：`docs/dev-records/dev-plan-290-execution-log.md`。
5. [ ] 面向 `285` 的 `260` 收口结论。

## 8. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/289-librechat-260-m2-m4-implementation-closure-plan.md`
- `docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`
- `docs/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- `docs/dev-records/dev-plan-290-execution-log.md`
- `AGENTS.md`

## 9. 前置发现登记（承接 288，2026-03-08）
1. [X] `292` 已关闭认证/启动兼容阻塞后，`290` 的最近前置缺口不再是 auth/startup，而是 `288` 中 vendored UI 渲染主路径对 formal 消息的命中问题。
2. [X] 当前已确认存在双渲染链并存：主路径为 `components/Messages/*`，旧兼容回退为 `Chat/Messages/*`；`290` 验收仅接受主路径证据，不接受“仅修旧链”的通过声明。
3. [ ] `290` 启动门槛补充：`288` 必须先提供以下证据后，`290` 才能进入 Case 1~4 正式通过判定。
   - `tp288` 正式入口 DOM 出现 `data-assistant-binding-key`；
   - 同轮消息无普通 GPT 气泡回退；
   - 证据时间晚于最近一次影响渲染主链或 `240C/240D/240E` 影响性合入。
4. [X] 单链路原则补充：`290` 执行阶段若发现再次依赖 `message.content` 缺失回退链才能通过，视为 `288` 未闭环，必须回退到 `288` 修复，不得以 Case 结果“暂时可用”替代结构性收口。
