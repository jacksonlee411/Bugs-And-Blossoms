# DEV-PLAN-240F 联合回归矩阵

- 生成时间：2026-03-09 12:31:22 CST
- 结论：`passed`
- 口径：本矩阵只整合 `240/260/266/280/284/291` 已实现能力与已固化证据，不新增平行验收标准。

## Case / Phase 对齐

| Case | 输入 | 冻结 phase/FSM | 主要来源 | 结论 |
| --- | --- | --- | --- | --- |
| Case 1 | `你好` | `idle -> idle` | `docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md` | 已对齐 |
| Case 2 | `在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01` -> `确认` | `idle -> await_commit_confirm -> committing -> committed` | `docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md` | 已对齐 |
| Case 3 | `在 AI治理办公室 下新建 人力资源部239A补全` -> `生效日期 2026-03-25` -> `确认` | `idle -> await_missing_fields -> await_commit_confirm -> committing -> committed` | `docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md` | 已对齐 |
| Case 4 | `在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26` -> `选第2个` -> `是的` | `idle -> await_candidate_pick -> await_candidate_confirm -> await_commit_confirm -> committing -> committed` | `docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md` | 已对齐 |

## A1~A9 联合结论

| ID | 主题 | `240F` 结论 | 主证据来源 | `240F` 复核动作 | 结果 |
| --- | --- | --- | --- | --- | --- |
| A1 | 正式入口承载 | `/app/assistant/librechat` 是唯一正式交互入口；`/assistant-ui/*` 仅保留历史别名/拒绝语义 | `tp288-handoff-to-285.md`、`291-handoff-to-285.md`、`internal/server/assistant_ui_proxy.go`、`e2e/tests/tp283-librechat-formal-entry-cutover.spec.js` | 复核别名行为仅为 `302`/`405`，无第二正式入口 | passed |
| A2 | 单发送/单回复 | 单轮只有一条有效发送路径与一条 assistant 业务回复 | `tp288-handoff-to-285.md`、`tp290-real-case-evidence-index.json` | 复核 `native_send_emitted=0`、`official_message_tree_only=true`、`single_assistant_bubble=true` 仍成立 | passed |
| A3 | DTO-only | 前端只消费后端 DTO、receipt、task，不承担业务语义重算 | `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`、`tp290-real-case-evidence-index.json` | 对齐 `284` 的前端降权口径与 `290` Case 证据 | passed |
| A4 | 编排语义 | `plan / confirm / commit / task / reply` 由后端主链裁决，而不是页面 helper | `docs/dev-plans/240c-assistant-action-interceptor-and-risk-gate-plan.md`、`docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md`、`tp290-case-matrix-v1.md` | 按 Case -> phase/FSM -> 用户可见输出做逐项对照 | passed |
| A5 | Case 1~4 闭环 | `260` Case 1~4 与 `240` 编排一致，无局部口径冲突 | `tp290-real-case-evidence-index.json`、`tp290-case-matrix-v1.md` | 复核 4 个 Case 全部 `passed` 且 phase 正确 | passed |
| A6 | 任务与审计 | `conversation_id / turn_id / request_id / trace_id / task_id` 可在正式链路串联追踪 | `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`、`docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md` | 复核 `receipt -> poll -> refresh` 与审计字段要求仍为正式口径 | passed |
| A7 | 人工接管可见性 | `manual_takeover_required` 在正式入口仍可见、可解释、可追踪、可取消 | `docs/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md` | 复核 `240D-04` 已把人工接管最小可操作面纳入正式链路 | passed |
| A8 | 升级兼容前置 | source/runtime compatibility 与 compat alias 边界已完成且仍新鲜 | `291-evidence-index.json`、`291-ref-288-290-freshness.md` | 复核 `R1~R10` 全部通过且 `R9` 仍有效 | passed |
| A9 | 可交接性 | `285` 无需新增标准、无需重新解释 `288/290/291` 关系 | 本计划全部产物 + `288/290/291` 交接件 | 汇总成 `240f-handoff-to-285.md` | passed |

## 前端职责收口（240F 口径）
- [X] 允许：render 官方消息树、展示 receipt、轮询 task、刷新 conversation。
- [X] 禁止：候选裁决、缺字段判断、确认约束重算、提交成功判定重算、外挂消息容器回执。
- [X] 结论：`240` 编排能力已落在 `280/284` 正式主链，不依赖页面 helper 或旧桥职责。
