# DEV-PLAN-240F：Assistant 与 280 主链路对齐封板回归计划（承接 240-M7）

**状态**: 已完成（2026-03-09 12:31 CST；`240F` 固定产物与交接包已落盘：`240f-readiness-checklist.md`、`240f-joint-regression-matrix.md`、`240f-runtime-delta-report.md`、`240f-stopline-search-report.md`、`240f-evidence-index.json`、`240f-handoff-to-285.md`；已完成对 `240C/240D + 288 + 290 + 291` 的联合回归对齐、差量复核与 stopline 搜索，可直接作为 `285` 启动前置输入；`240E` 仍为非阻塞增强项）

## 1. 背景与定位
1. [ ] `240F` 是 `240` 的联合回归与封板输入计划，不是新的实现子计划；其职责是证明 **Assistant 编排语义** 已在 `280/284` 的正式承载链路中稳定成立。
2. [ ] `240F` 的核心问题不是“功能有没有局部可用”，而是：
   - [ ] `240` 的 `plan / confirm / commit / task / reply` 是否已完全落在正式入口 `/app/assistant/librechat`；
   - [ ] `280/284` 的前端降权是否真实成立：无 helper 业务推进、无页面外挂消息流、无双发送/双回复、无前端重算业务阶段；
   - [ ] `260` Case 1~4、`266` 单通道、`237` 升级兼容前置是否能以同一套最新证据共同支撑 `285` 封板。
3. [ ] `240F` 在路线图中的唯一定位冻结为：`271-S6` 的入口收口项；只有 `240F` 形成可复核通过产物后，`285` 才允许启动。
4. [ ] `240F` 的工作方式冻结为：**消费前置计划产物 + 做跨计划对齐复核 + 做最小必要抽检 + 输出交接包**；禁止在本计划内偷偷吞并 `240C/240D/288/290/291/285` 的剩余实现。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 证明 `240` 的编排能力已在正式入口 `/app/assistant/librechat` 与 `280/284` 主链路上成立。
2. [ ] 证明 `260` Case 1~4 的用户可见行为与 `240` 的后端 FSM/DTO 语义一致，不存在前端补丁式重算。
3. [ ] 证明 `266` 的单通道、气泡内回写、无外挂容器、无官方原始报错在 `240` 编排场景下仍然成立。
4. [ ] 证明 `240D` 的 receipt -> poll -> refresh、`manual_takeover_required` 可见性与审计链在正式入口未回退。
5. [ ] 输出可被 `285` 直接消费的封板输入包，不要求 `285` 再重新解释标准或补定义。

### 2.2 非目标
1. [ ] 不新增新的 Assistant 业务能力、页面能力、任务类型、候选算法或知识包运行时能力。
2. [ ] 不以“重跑一次全量 E2E”替代跨计划口径对齐；`240F` 必须明确每条结论分别来自哪个前置计划与哪份证据。
3. [ ] 不以放宽门禁、扩大排除项、弱化 stopline 或保留 legacy 回退通道来换取“通过”。
4. [ ] 不让 `285` 提前吞并实现缺口；一旦发现缺口，必须回退到对应子计划修复。

## 3. 启动前提与边界冻结

### 3.1 启动前提（硬条件）
1. [ ] `240C` 已形成可复核产物，且 `confirm / commit` 风控、风险门禁、错误码语义已冻结。
2. [ ] `240D` 已形成可复核产物，且 `:commit -> receipt -> poll -> refresh` 正式链路已切换完成，`manual_takeover_required` 已具备用户可见性。
3. [ ] `288` 已完成，且 `tp288-real-entry-evidence-index.json` 与 `tp288-handoff-to-285.md` 存在并可复核。
4. [ ] `290` 已完成，且 `tp290-real-case-evidence-index.json` 与 `Case Matrix v1` 存在并可复核。
5. [ ] `291` 已完成，且 `291-evidence-index.json`、`291-handoff-to-285.md`、`291-ref-288-290-freshness.md` 存在并可复核。
6. [ ] 用于 `240F` 判定的 `288/290/291` 证据生成时间，必须晚于最近一次影响以下结论的运行时合入：
   - [ ] `240C/240D` 的运行时判定、错误码、receipt/task 状态机、fail-closed 语义；
   - [ ] `280/284` 的 send/store/render 接管、DTO-only、消息树唯一落点；
   - [ ] `266` 的消息绑定、官方气泡回写、单轮唯一发送；
   - [ ] `237/291` 的 vendored UI compat alias、source/runtime compatibility 结论。
7. [ ] `240E` 当前不构成 `240F` 启动阻塞；但若其在 `240F` 启动前进入运行时主链并影响上述结论，则必须先刷新受影响的 `288/290/291` 证据。

### 3.2 边界冻结
1. [ ] `240F` 只接受“已实现能力”的联合回归；不接受“边验收边补实现”。
2. [ ] `240F` 期间如发现缺口，必须按根因回退：
   - [ ] 风控/确认/提交语义缺口 -> 回退 `240C`；
   - [ ] receipt/poll/manual takeover/任务状态缺口 -> 回退 `240D`；
   - [ ] 正式入口单通道/气泡/消息树缺口 -> 回退 `266/288/284`；
   - [ ] Case 1~4 行为/FSM/确认词/候选语义缺口 -> 回退 `260/290`；
   - [ ] vendored UI compat/source/runtime 回归缺口 -> 回退 `237/291`；
   - [ ] 总体验收与归档封板缺口 -> 留给 `285`，但前提是 `240F` 已完成自身输入收口。
3. [ ] `240F` 中发现的问题，只允许记录、归类、回退，不允许在本计划文档中直接把缺口“解释成通过”。

## 4. 输入、输出与固定产物

### 4.1 输入（固定消费）
1. [ ] 文档输入：
   - [ ] `docs/dev-plans/240c-assistant-action-interceptor-and-risk-gate-plan.md`
   - [ ] `docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md`
   - [ ] `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
   - [ ] `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
   - [ ] `docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`
   - [ ] `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
   - [ ] `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
2. [ ] 证据输入：
   - [ ] `docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`
   - [ ] `docs/dev-records/assets/dev-plan-266/tp288-handoff-to-285.md`
   - [ ] `docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`
   - [ ] `docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`
   - [ ] `docs/dev-records/assets/dev-plan-291/291-evidence-index.json`
   - [ ] `docs/dev-records/assets/dev-plan-291/291-handoff-to-285.md`
   - [ ] `docs/dev-records/assets/dev-plan-291/291-ref-288-290-freshness.md`

### 4.2 输出（本计划固定命名）
1. [ ] 证据根目录固定为：`docs/dev-records/assets/dev-plan-240f/`。
2. [ ] 必须产出以下固定文件：
   - [ ] `docs/dev-records/assets/dev-plan-240f/240f-readiness-checklist.md`
   - [ ] `docs/dev-records/assets/dev-plan-240f/240f-joint-regression-matrix.md`
   - [ ] `docs/dev-records/assets/dev-plan-240f/240f-runtime-delta-report.md`
   - [ ] `docs/dev-records/assets/dev-plan-240f/240f-stopline-search-report.md`
   - [ ] `docs/dev-records/assets/dev-plan-240f/240f-evidence-index.json`
   - [ ] `docs/dev-records/assets/dev-plan-240f/240f-handoff-to-285.md`
3. [ ] `240f-evidence-index.json` 的条目字段固定为：`id`、`kind`、`source_plan`、`source_artifacts[]`、`executed_at`、`result`、`owner`、`notes`。
4. [ ] `240f-handoff-to-285.md` 必须直接回答以下问题：
   - [ ] `240` 编排是否已在 `280/284` 正式主链路成立；
   - [ ] `288/290/291` 是否仍满足新鲜度；
   - [ ] 尚未关闭的问题是否都已回退到对应子计划；
   - [ ] `285` 启动时是否还需要新增命令或重定义标准；答案必须为“否”。

## 5. 联合回归矩阵（240F 唯一判定口径）

| 主题 | `240F` 必须证明的结论 | 主证据来源 | `240F` 追加动作 | 失败回退 |
| --- | --- | --- | --- | --- |
| A1 正式入口承载 | `/app/assistant/librechat` 是唯一正式交互入口 | `288`、`290`、`291` | 复核 `handoff` 与新鲜度 | `288/291` |
| A2 单发送/单回复 | 单轮只有一条有效发送路径与一条 assistant 业务回复 | `288`、`290` | 抽检 DOM/trace/网络索引是否一致 | `266/288/284` |
| A3 DTO-only | 前端只消费后端 DTO/receipt/task，不重算业务阶段 | `284`、`240D`、`290` | 建立“前端职责清单”并逐项对照 | `240D/284` |
| A4 编排语义 | `plan / confirm / commit / task / reply` 语义由后端主链裁决 | `240C`、`240D`、`290` | 对齐 Case -> phase/FSM -> 用户可见输出 | `240C/240D/290` |
| A5 Case 1~4 闭环 | `260` 的真实 Case 1~4 与 `240` 编排一致 | `290` | 复核 Case Matrix v1 与执行日志 | `260/290` |
| A6 任务与审计 | `request_id / trace_id / conversation_id / task_id` 可串联追踪 | `240D`、`223`、`290` | 补录链路映射与查询入口 | `240D/223` |
| A7 人工接管可见性 | `manual_takeover_required` 未因 cutover 回退 | `240D` | 引用现有证据并在矩阵显式列为通过项 | `240D` |
| A8 升级兼容前置 | source/runtime compatibility 与 compat alias 仍成立 | `291` | 复核 `R9` 引用新鲜度 | `237/291` |
| A9 封板可交接性 | `285` 可直接消费，不需补标准 | `288`、`290`、`291`、本计划产物 | 输出 `240f-handoff-to-285.md` | `240F` |

## 6. 直接实施拆分（按批次）

### 6.0 当前执行口径（2026-03-09 冻结）
1. [ ] `240F` 默认不新建 E2E 场景，而是复用 `288/290/291` 已冻结的回归矩阵与资产索引。
2. [ ] 仅当证据新鲜度失效、交接包缺字段、或跨计划结论出现冲突时，才允许回退去重跑对应前置计划；禁止在 `240F` 中另起一套“平行验收口径”。
3. [ ] `240F` 的所有新增结论必须能回溯到“来源计划 + 来源证据 + 本计划复核动作”。

### 6.1 PR-240F-01：启动校验与输入冻结
1. [ ] 目标：确认 `240F` 真的可以启动，并把输入与当前结论冻结成一份 readiness 清单。
2. [ ] 直接动作：
   - [ ] 检查 `240C/240D/288/290/291` 文档状态与对应资产路径是否齐备；
   - [ ] 检查 `288/290/291` 的证据时间与最近一次影响性合入的先后关系；
   - [ ] 记录本次 `240F` 消费的输入版本、证据时间、引用路径；
   - [ ] 初始化 `docs/dev-records/assets/dev-plan-240f/` 与 `240f-evidence-index.json`。
3. [ ] DoD：
   - [ ] `240f-readiness-checklist.md` 填写完成；
   - [ ] 若任何前置仍为规划态/骨架态/证据过期，`240F` 立即停止并回退；
   - [ ] `240F` 的启动时间、输入路径、引用结论全部可追溯。

### 6.2 PR-240F-02：联合回归矩阵对齐
1. [ ] 目标：把 `240/260/266/280/284/237` 的跨计划结论收敛到一张 `240F` 联合矩阵中。
2. [ ] 直接动作：
   - [ ] 以第 5 节矩阵为模板，逐项填写“结论 / 证据 / 追加动作 / 失败回退”；
   - [ ] 对齐 `260` Case 1~4 与 `240` phase/FSM 映射：
     - [ ] Case 1 = `idle -> idle`；
     - [ ] Case 2 = `idle -> await_commit_confirm -> committing -> committed`；
     - [ ] Case 3 = `idle -> await_missing_fields -> await_commit_confirm -> committing -> committed`；
     - [ ] Case 4 = `idle -> await_candidate_pick -> await_candidate_confirm -> await_commit_confirm -> committing -> committed`；
   - [ ] 对齐 `280/284` 的前端职责：只允许 render、receipt 展示、poll、refresh，不允许候选裁决、缺字段判断、确认约束重算；
   - [ ] 对齐 `240D` 的任务链：receipt、poll_uri、task detail、最终 conversation refresh 必须在联合矩阵中有对应证据位。
3. [ ] DoD：
   - [ ] `240f-joint-regression-matrix.md` 完成；
   - [ ] 不存在“结论已写但无来源证据”或“有来源证据但无人负责解释”的空洞项；
   - [ ] 每一项失败时都能明确回退到唯一子计划。

### 6.3 PR-240F-03：运行时差量复核
1. [ ] 目标：在不复制前置计划全量工作的前提下，完成 `240F` 必需的最小抽检与差量确认。
2. [ ] 直接动作：
   - [ ] 复核 `288/290/291` 的 `handoff` 与 `evidence-index`，确认其结论相互不冲突；
   - [ ] 抽检正式入口一轮真实对话资产，确认没有“双发送、双回复、外挂容器、前端重算阶段”的迹象；
   - [ ] 抽检一条 `receipt -> poll -> refresh` 成功路径资产，确认 `240D` 的正式消费语义未回退；
   - [ ] 抽检 `manual_takeover_required` 的现有可见性证据，确认没有被 `280/284` UI 改造吞掉；
   - [ ] 抽检 `291` 的 `R9` 新鲜度记录，确认其引用的 `288/290` 仍是当前有效证据。
3. [ ] DoD：
   - [ ] `240f-runtime-delta-report.md` 完成；
   - [ ] 所有抽检项都记录“抽检对象 / 来源资产 / 结论 / 是否需要重跑前置计划”；
   - [ ] 若抽检发现运行时回退，不得在 `240F` 继续推进，必须先回退修复。

### 6.4 PR-240F-04：停止线与搜索型回归收口
1. [ ] 目标：补齐 `240F` 自身负责的 stopline，确保不会把旧口径带入 `285`。
2. [ ] 直接动作：
   - [ ] 搜索活跃目录中是否仍把 `/assistant-ui/*`、旧工作台、bridge/iframe 口径写成正式承载结构；
   - [ ] 搜索是否仍存在 `bridge.js`、`data-assistant-dialog-stream`、`assistantDialogFlow`、`assistantAutoRun` 的正式职责描述；
   - [ ] 搜索是否存在“前端自行判断缺字段/候选/确认条件”的主口径残留；
   - [ ] 搜索是否存在“`240E` 仍为 `240F/285` 启动硬阻塞”的过期口径。
3. [ ] 搜索范围默认包含：`AGENTS.md`、`docs/dev-plans/`、`internal/`、`e2e/`、正式承载相关前端目录；默认排除 `docs/archive/`、`docs/dev-records/`、第三方 vendored 上游原始代码目录。
4. [ ] DoD：
   - [ ] `240f-stopline-search-report.md` 完成；
   - [ ] 所有命中项都被归类为“允许保留 / 必须修复 / 归档历史引用”；
   - [ ] 无新的主线口径回流。

### 6.5 PR-240F-05：交接 `285`
1. [ ] 目标：把 `240F` 的联合结论收敛成 `285` 可直接消费的入口包。
2. [ ] 直接动作：
   - [ ] 汇总 readiness、联合矩阵、差量复核、stopline 搜索四份结果；
   - [ ] 显式写明“已通过 / 失败回退 / 仍待外部处理但不阻塞 `285`”三类结论；
   - [ ] 逐条列明 `285` 还需消费的前置产物及其路径；
   - [ ] 声明 `285` 不需要新增判定标准或回头补解释。
3. [ ] DoD：
   - [ ] `240f-handoff-to-285.md` 完成；
   - [ ] `285 §2.3/§3/§4` 可直接引用 `240F` 结论；
   - [ ] `240F` 标记完成后，不存在“封板前还要补定义”的模糊地带。

## 7. 证据刷新规则（Fail-Closed）
1. [ ] 若 `240C/240D/240E/280/284/292` 的影响性合入发生在 `288/290/291` 证据之后，则 `240F` 不得直接引用旧证据，必须先回退刷新对应前置计划。
2. [ ] 若 `288/290/291` 之间的结论互相矛盾，例如：
   - [ ] `288` 证明单消息落点，但 `290` 出现双 assistant 气泡；
   - [ ] `290` 证明 DTO-only，但 `240D` receipt/poll 语义仍被前端 helper 接管；
   - [ ] `291` 证明 compat alias 安全，但搜索型 stopline 发现其被写成第二正式入口；
   则 `240F` 立即失败，必须先回退相关子计划。
3. [ ] 若 `240F` 输出包缺少固定命名文件、索引条目为空、或无法追溯来源证据，则 `240F` 失败。
4. [ ] 若 `240F` 结论需要靠“人工口头解释”而不是固定文档/索引/日志才能成立，则 `240F` 失败。

## 8. 停止线（Fail-Closed）
1. [ ] 若仍出现双发送、双回复、外挂回执、页面外挂消息容器，则本计划失败。
2. [ ] 若前端仍承担阶段推进、候选裁决、缺字段判断、确认约束或提交完成判定，则本计划失败。
3. [ ] 若 `receipt -> poll -> refresh` 正式链路被同步直返 conversation 或页面 patch 替代，则本计划失败。
4. [ ] 若 `manual_takeover_required` 在真实入口不可见、不可解释、不可追踪，则本计划失败。
5. [ ] 若 `291` 的 source/runtime compatibility 与 compat alias 边界未完成或证据失鲜，则本计划失败。
6. [ ] 若 `240C/240D/288/290/291` 任一仍停留在规划态、骨架态或未形成可复核产物，则本计划不得更新为“已完成”。
7. [ ] 若试图以 `240E` 的未来增强替代当前缺口修复，则本计划失败。

## 9. 验收标准
1. [X] `240F` 固定输出的六个文件全部存在，且条目完整可追溯。
2. [X] 第 5 节联合回归矩阵的 `A1~A9` 全部闭环，无未归属空洞项。
3. [X] `260` Case 1~4、`266` stopline、`240D` 任务正式消费语义、`291` 升级兼容前置四类结论在 `240F` 中可被统一解释，不相互冲突。
4. [X] 已明确证明 `240` 编排能力是在 `280/284` 正式主链路上成立，而不是依赖旧桥接、旧页面 helper 或双链路兜底。
5. [X] `240f-handoff-to-285.md` 可被 `285` 直接消费，且 `285` 无需新增标准、无需重新解释 `288/290/291` 关系。

## 10. 门禁与命令（SSOT 引用）
1. [X] 文档门禁：`make check doc`。
2. [ ] 若 `240F` 过程中判定 `288/290/291` 证据失鲜，则按对应计划原命令补跑，不在 `240F` 自定义替代命令。
3. [ ] `240F` 建议固定校验顺序：
   - [ ] `test -f docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`
   - [ ] `test -f docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`
   - [ ] `test -f docs/dev-records/assets/dev-plan-291/291-evidence-index.json`
   - [ ] `make check doc`
   - [ ] 若命中新鲜度失效：回退执行 `288/290/291` 原计划命令与顺序
4. [ ] 发起 `285` 前，推荐再次执行 `make preflight`，但它不是替代 `240F` 联合结论的证据来源。

## 11. 关联文档
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/240c-assistant-action-interceptor-and-risk-gate-plan.md`
- `docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md`
- `docs/dev-plans/240e-assistant-internal-knowledge-pack-and-readonly-resolver-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`
- `AGENTS.md`
