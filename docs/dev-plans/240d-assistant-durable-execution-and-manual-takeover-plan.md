# DEV-PLAN-240D：Assistant 耐久执行与人工接管优先计划（承接 240-M5）

**状态**: 已完成（2026-03-09 11:35 CST；`PR-240D-01/02/03/04` 已全部完成：共享 commit core、任务执行器真实提交、正式 `:commit` cutover 到 `202 + receipt`、正式入口 `receipt -> poll -> refresh/cancel` 与 `manual_takeover_required` 可见性均已落地；无 PG 同步兼容 seam 已删除，且已按 `271-S5` 时序要求重跑 `288/290/291` 通过）

## 1. 评审结论（2026-03-09 CST）
1. [X] 结论：原版 `240D` 只表达了方向，没有把“已有基础设施”和“真正需要切换的主链”说清楚，尚未达到直接实施门槛。
2. [X] 关键缺口一：未明确当前仓内已经有 `DEV-PLAN-225` 落下的任务表、outbox、任务详情/取消 API 与基础重试逻辑，导致 `240D` 容易与 `225` 重复造轮子。
3. [X] 关键缺口二：未明确真正待改造的主问题是 `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:commit` 仍然走同步直提，而不是“完全没有任务系统”。
4. [X] 关键缺口三：未冻结 `commit -> receipt -> poll -> terminal state -> conversation refresh` 的完整契约，也未定义 `MANUAL_TAKEOVER_REQUIRED` 的进入条件与最小可操作面。
5. [X] 关键缺口四：未给出按代码目录可执行的分批实施顺序、文件落点、测试矩阵、证据刷新规则，因此 reviewer 无法判断是否能直接开工。
6. [X] 本次修订处理：将 `240D` 重写为“复用 `225` 任务底座、把 `commit` 默认切到耐久执行主链、并把人工接管做成 fail-closed 的最小可操作闭环”的实施计划。

## 2. 背景与当前基线
1. [X] `DEV-PLAN-225` 已在仓内落下任务耐久化骨架：`iam.assistant_tasks`、`iam.assistant_task_events`、`iam.assistant_task_dispatch_outbox` 以及 `internal/server/assistant_task_store.go`。
2. [X] 实施前 `commit` 主入口是同步直提：`internal/server/assistant_api.go` 的 `:commit` 分支直接调用 `commitTurn(...)`，再进入 `commitTurnPG(...)` 执行业务写入；现已切到任务受理主链。
3. [X] 实施前任务执行器 `executeAssistantTaskWorkflowTx(...)` 只做契约快照校验与状态流转演示，尚未真正调用组织写入主链，也未把 partial failure 分类为“可自动完成”与“必须人工接管”；现已接入真实提交与错误分类。
4. [X] 实施前前端/调用侧仍可能直接理解 `:commit` 会返回完整 conversation；现已完成正式入口 `receipt + async task` 主链切换。
5. [X] 因此，`240D` 的真实范围是：
   - [X] 复用现有任务持久化与 outbox；
   - [X] 将 `:commit` 从“同步返回 conversation”切换为“异步受理返回 receipt”；
   - [X] 让任务执行器调用真实提交主链；
   - [X] 将高风险异常统一收敛到 `manual_takeover_required`；
   - [X] 为 `271-S5` 提供可复核的新鲜证据。

## 3. 目标与非目标

### 3.1 目标
1. [ ] 将 `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:commit` 切换为默认异步耐久执行：同步阶段仅做受理、校验、落盘、派发，不直接完成业务写入。
2. [ ] 将任务执行器接到真实提交主链，确保实际写入仍然遵守 `One Door`、显式事务、租户注入、RLS fail-closed。
3. [ ] 冻结高风险组织操作的失败语义：只要出现 partial failure、结果不确定、重试耗尽、契约漂移、派发超时，默认进入 `manual_takeover_required`。
4. [ ] 提供最小可操作闭环：receipt 可轮询，任务详情可审计，`manual_takeover_required` 可见、可取消、可按 runbook 接管处理。
5. [ ] 在最终 cutover 后，`288/290` 按 `271-S5` 规则重跑并刷新证据，避免“旧证据覆盖新主链”。

### 3.2 非目标
1. [ ] 不在本计划内改写 `260` 的业务 Case 定义、提示词语义或场景矩阵。
2. [ ] 不在本计划内扩大自动补偿/自动 saga 范围；本期默认人工接管优先。
3. [ ] 不在本计划内新增第二套用户提交入口；浏览器/正式入口不直接调用 `POST /internal/assistant/tasks` 组装任务快照。
4. [ ] 不在本计划内新建表；若实现过程中确认必须补列/索引，需单独评审并先获得用户确认后再建迁移。
5. [ ] 不在本计划内提供 end-user 自助“重新执行任务”按钮；人工接管的 MVP 以“详情可见 + 取消 + runbook 接管 + 修复后重新触发业务流程”为准。

## 4. 冻结决策与不变量

### 4.1 冻结决策
1. [X] `:commit` 是 Assistant 组织操作的唯一对外提交入口；切换完成后该接口必须返回 `202 Accepted + AsyncTaskReceipt`，不再直接返回 conversation。
2. [X] receipt 仅表示“已受理并可追踪”，不表示“业务写入已完成”；最终业务结果必须通过 `GET /internal/assistant/tasks/{task_id}` 与 `GET /internal/assistant/conversations/{conversation_id}` 联合确认。
3. [X] 任务快照由服务端在 `:commit` 受理阶段根据当前 turn/conversation 状态生成并落盘；正式入口不得自行拼装 `contract_snapshot` 或重算业务阶段。
4. [X] 任务执行器必须复用与同步提交同一套 commit core，不允许复制第二份业务写入逻辑。
5. [X] 对当前 `assistant_async_plan` 高风险写任务，异常终态统一优先收敛到 `manual_takeover_required`；`failed` 状态在本链路中保留但不作为默认落点。

### 4.2 Greenfield/Assistant 不变量
1. [ ] `One Door`：任务执行器只能调用现有提交主链/commit adapter，不能形成第二写入口。
2. [ ] `No Tx, No RLS`：任务受理、状态迁移、真实写入都必须运行在显式事务与租户注入上下文中。
3. [ ] `No Legacy`：切换完成后不得保留“默认同步 commit + 可选异步 commit”双链路；只允许一个正式主链。
4. [ ] `240C` 前置约束：最终 cutover 合入前，`plan/confirm/commit` 的统一拦截链必须已可用，或与 `240D` 同批合入；不得让异步执行绕过 `capability/authz/risk/required_checks`。
5. [ ] `223` / `280` 约束：前端只消费 DTO 与任务状态，不得通过 helper/DOM/runtime adapter 私自推进业务阶段。
6. [ ] `271-S5` 新鲜度约束：任何影响 `240D` 主链、错误码、路由/认证、fail-closed 语义的合入，都会使 `288/290` 旧证据失效。

## 5. 目标契约（实施冻结）

### 5.1 `:commit` 对外契约
1. [ ] 请求：继续使用 `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:commit`，请求体保持空对象 `{}`。
2. [ ] 成功响应：返回 `202 Accepted` 与 `AssistantTaskAsyncReceipt`，字段至少包括：`task_id`、`task_type`、`status`、`workflow_id`、`submitted_at`、`poll_uri`。
3. [ ] 同步拒绝：若受理前校验失败，继续返回稳定 4xx/5xx 错误码，例如 `conversation_confirmation_required`、`ai_plan_contract_version_mismatch`、`ai_actor_auth_snapshot_expired`，且**不得创建任务**。
4. [ ] 异步完成：客户端收到 receipt 后必须轮询 `GET /internal/assistant/tasks/{task_id}`；当任务为 `succeeded` 时，再调用 `GET /internal/assistant/conversations/{conversation_id}` 刷新最终 conversation。
5. [ ] 前端禁止直接调用 `POST /internal/assistant/tasks` 提交 Assistant 组织操作；该接口仅保留为内部 seam/测试支点，是否收口由后续计划单独结论化。

### 5.2 任务状态机（本链路冻结）
| 维度 | 枚举 | 语义 |
| --- | --- | --- |
| `status` | `queued` | 已受理并落盘，等待派发/执行 |
| `status` | `running` | 已进入真实执行阶段 |
| `status` | `succeeded` | 提交链路完成，最终 conversation 可刷新读取 |
| `status` | `manual_takeover_required` | 触发人工接管，自动链路停止，等待人工处置 |
| `status` | `canceled` | 用户/操作者明确取消，任务关闭 |
| `dispatch_status` | `pending` | outbox 待派发或待下一次重试 |
| `dispatch_status` | `started` | 已开始派发/执行 |
| `dispatch_status` | `failed` | 派发阶段已停止并进入终态 |

### 5.3 允许的状态迁移
1. [ ] 允许：`queued -> running / canceled / manual_takeover_required`。
2. [ ] 允许：`running -> succeeded / manual_takeover_required / canceled`。
3. [ ] 允许：`manual_takeover_required -> canceled`（本期 MVP）；“人工修复后重跑同 `request_id`”沿用 `225` 的方向，但不作为本期 end-user 功能交付项。
4. [ ] 禁止：任何终态回到 `queued`；禁止跳过 `running` 直接把真实写入标记为成功。
5. [ ] 禁止：`status=succeeded` 但 conversation 未可读或审计状态迁移缺失。

### 5.4 `manual_takeover_required` 进入条件（冻结）
1. [ ] 派发重试耗尽或超过 `dispatch_deadline_at`。
2. [ ] 任务执行前发现 contract snapshot/plan hash/context hash 漂移。
3. [ ] 真实提交阶段出现 partial failure、下游结果不确定、或无法证明“未写入/已完整写入”。
4. [ ] 取消请求与执行结果发生竞态，且系统无法给出确定结果。
5. [ ] 任何需要人为核对后续修复动作的异常，不允许以泛化 `failed` 掩盖。

### 5.5 人工接管 MVP（本期必须可操作）
1. [ ] 任务详情必须展示：`task_id`、`workflow_id`、`request_id`、`trace_id`、`last_error_code`、`attempt/max_attempts`、`updated_at`。
2. [ ] `manual_takeover_required` 状态必须可通过现有 `cancel` 动作关闭，避免僵尸任务长期悬挂。
3. [ ] 必须形成面向执行人的 runbook：按 `request_id/trace_id/task_id` 定位审计、确认真实写入结果、执行环境级保护/只读/修复/重试。
4. [ ] 正式入口必须能把“任务待处理 / 已转人工接管”清楚反馈给用户，而不是继续显示“提交中”或直接假定成功。

## 6. 方案设计

### 6.1 受理阶段（同步）
1. [ ] 在 `:commit` handler 中保留当前同步前置校验，但成功路径改为“生成任务快照 -> 写入 `assistant_tasks`/`assistant_task_events`/`assistant_task_dispatch_outbox` -> 返回 receipt”。
2. [ ] 任务快照的来源必须是当前 turn/conversation 的服务端事实：`intent_schema_version`、`compiler_contract_version`、`capability_map_version`、`skill_manifest_digest`、`context_hash`、`intent_hash`、`plan_hash`。
3. [ ] 幂等键继续使用 `tenant + conversation_id + turn_id + request_id`，并以 `request_hash` 防止同键异载荷。
4. [ ] 如果同步受理失败，必须 fail-closed：不创建任务、不返回假 receipt、不偷偷回退到同步提交。

### 6.2 执行阶段（异步）
1. [ ] 从 `commitTurnPG(...)` / `applyCommitTurn(...)` 中抽出共享 commit core，供同步遗留测试与异步任务执行器共同复用。
2. [ ] `executeAssistantTaskWorkflowTx(...)` 不再“伪成功”；它必须在真实事务中调用共享 commit core，并完成 conversation/turn/state transition 的真实更新。
3. [ ] 执行前必须再次校验：租户、actor 归属、turn 仍存在、contract snapshot 未漂移、`240C` 拦截链要求的运行时条件仍满足。
4. [ ] 执行后必须写齐任务事件与审计字段，保证 `request_id/trace_id/task_id/workflow_id` 可串联查询。
5. [ ] 如果提交链路报错，必须先做错误分类：
   - [ ] 可证明未落库且可安全重试：走重试/退避；
   - [ ] 结果不确定或存在 partial failure：进入 `manual_takeover_required`；
   - [ ] 不允许以“吞错后 succeeded”或“直接 failed 无人工痕迹”掩盖高风险异常。

### 6.3 消费阶段（轮询与刷新）
1. [X] 正式入口拿到 receipt 后，进入基于 `task_id` 的轮询，而不是等待 `:commit` 返回完整 conversation。
2. [X] `status=succeeded` 时刷新 conversation；`status=manual_takeover_required` 时展示人工接管提示；`status=canceled` 时结束等待并保留明确状态说明。
3. [ ] 正式入口不得根据本地计时器或 DOM 状态推断成功；只能相信后端任务状态与 conversation 读取结果。
4. [ ] 若轮询超时，前端应提示“任务仍在执行，可稍后继续查看”，而不是误报失败；最终真相仍以后端任务状态为准。

### 6.4 当前 `:commit` 消费者盘点（2026-03-09）
1. [X] 实施前后端正式入口由 `internal/server/assistant_api.go` 的 `:commit` 分支直接调用 `commitTurn(...)`，成功语义是 `200 + conversation`；现已切到 `submitCommitTask(...)` 与 `202 + receipt`。
2. [X] 实施前前端 API helper 直接消费 `AssistantConversation`；现已改为消费 task receipt 并轮询任务。
3. [X] 当前与 `:commit` 成功语义直接耦合的回归面已盘点：`internal/server/assistant_api_test.go`、`apps/web/src/api/assistant.test.ts`、`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`、`e2e/tests/tp290-librechat-real-case-matrix.spec.js`。
4. [X] 现有任务 seam 已可直接复用：`internal/server/assistant_tasks_api.go` 与 `internal/server/assistant_task_store.go` 已提供 `submit/get/cancel` 基础能力；正式入口 cutover 后只能消费 `receipt -> poll -> refresh`，不得直接改走 `/internal/assistant/tasks` 组装提交。

## 7. 直接实施拆分（按 PR/批次）

### 7.0 当前启动口径（2026-03-09）
1. [X] 允许立即启动 `PR-240D-01` 与 `PR-240D-02`；这两个批次只处理共享 commit core、任务执行器与错误分类，不切 `:commit` 的对外成功语义。
2. [X] `PR-240D-03` 已执行完成：`:commit` 成功语义已从 `200 + conversation` 切到 `202 + receipt`，且成功路径的同步回退已删除。
3. [X] `PR-240D-04` 已执行完成：已基于最新影响性合入重跑 `288/290/291`，并补齐 `manual_takeover_required` 的用户可见状态。

### 7.1 PR-240D-01：共享 commit core 抽取（无行为切换）
1. [X] 目标：从当前同步提交链中抽出共享 commit core，为异步执行器复用做准备。
2. [X] 重点文件：
   - [X] `internal/server/assistant_persistence.go`
   - [ ] `internal/server/assistant_api.go`
   - [X] `internal/server/assistant_task_store.go`
   - [X] `internal/server/assistant_persistence_gap_test.go`
   - [X] `internal/server/assistant_task_store_gap_test.go`
3. [X] DoD：
   - [X] 共享 core 可在事务中独立调用；
   - [X] 无新增第二份业务写入逻辑；
   - [X] 现有同步测试保持通过。

### 7.2 PR-240D-02：任务执行器接入真实提交与错误分类
1. [X] 目标：让 `executeAssistantTaskWorkflowTx(...)` 调用真实 commit core，并把异常稳定分流到重试或 `manual_takeover_required`。
2. [X] 重点文件：
   - [X] `internal/server/assistant_task_store.go`
   - [X] `internal/server/assistant_task_store_gap_test.go`（沿用现有相邻测试文件命名）
   - [ ] `internal/server/assistant_tasks_api.go`
   - [ ] `internal/server/assistant_api_gap_test.go`
3. [X] DoD：
   - [X] 任务成功时真实写入完成，conversation 可刷新读取；
   - [X] 契约漂移/派发超时/结果不确定进入 `manual_takeover_required`；
   - [X] 不存在“任务 succeeded 但 conversation 未变化”假阳性。

### 7.3 PR-240D-03：`:commit` 主链 cutover 到 receipt
1. [X] 目标：把正式 `:commit` 成功响应从 `200 + conversation` 切换为 `202 + receipt`；正式入口 cutover 已完成，且无 PG 同步兼容 seam 已删除。
2. [X] 重点文件：
   - [X] `internal/server/assistant_api.go`
   - [X] `internal/server/assistant_api_test.go`
   - [X] `third_party/librechat-web/source/client/src/assistant-formal/api.ts`
   - [X] `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/__tests__/AssistantFormalMessage.test.tsx`
   - [X] `third_party/librechat-web/source/client/src/assistant-formal/runtime.ts` / `third_party/librechat-web/source/client/src/components/Chat/Messages/Content/AssistantFormalMessage.tsx`
3. [X] DoD：
   - [X] 正式 PG-backed `:commit` 只负责受理与回 receipt；
   - [X] 正式入口已改为轮询任务并在成功后刷新 conversation；
   - [X] 正式入口不再依赖 `:commit` 响应体中的 conversation。

### 7.4 PR-240D-04：人工接管可见性、证据与回归
1. [X] 目标：补齐 `manual_takeover_required` 的最小可操作面，并刷新 `271-S5` 证据。
2. [X] 重点文件：
   - [X] `apps/web/src/errors/presentApiError.ts`
   - [X] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`
   - [X] `e2e/tests/tp290-librechat-real-case-matrix.spec.js`
   - [X] `docs/dev-records/dev-plan-240d-execution-log.md`（新建）
3. [X] DoD：
   - [X] 正式入口可见“执行中 / 已成功 / 已转人工接管 / 已取消”；
   - [X] `288 + 290` 已在最新代码上重跑并刷新索引；
   - [X] 人工接管演练结果已入 `docs/dev-records/dev-plan-240d-execution-log.md`。

## 8. Readiness 清单（满足后可切到“准备就绪”）
1. [X] `240C` 的统一拦截链契约已冻结；异步执行阶段复用 `capability/authz/risk/required_checks` 的约束已在本计划与 `240C` 契约中明确。
2. [X] 已确认本期默认不新增表；当前启动范围限定为 `PR-240D-01/02`，不涉及 schema 变更；若后续需要补列/索引，仍需单独列出并取得用户确认。
3. [X] 已完成当前 `:commit` 消费者盘点，确认正式 PG-backed 正式入口切换后不会再依赖 `200 + conversation`。
   - [X] 盘点结果已记录在 `§6.4`。
   - [X] receipt/poll 主链已切到正式入口，`tp288/tp290` 已随 `PR-240D-03/04` 完成切换验证。
4. [X] 已确认 `288/290` 将在最终影响性合入后按 `271` 规则重跑，旧证据不复用。
5. [X] 已确认 `manual_takeover_required` 的提示文案与错误码映射满足 `DEV-PLAN-140`。
6. [X] `240C` 运行时 gate 实现已可用；`PR-240D-03` 之后的 cutover 不再受 `240C` 实现状态阻塞，但仍受消费者切换确认与错误文案映射约束。

## 9. 测试与覆盖率
1. [ ] 覆盖率口径：沿用仓库当前 Go 测试与 CI 覆盖率门禁；本计划新增/改动代码不得通过保留死分支或扩大排除项规避测试。
2. [ ] 统计范围：至少覆盖 `internal/server/**` 中的 `:commit` handler、任务存储/派发、共享 commit core、错误分类与状态迁移；若改动正式入口消费逻辑，则同步覆盖其 API/client 层与关键页面状态。
3. [ ] 目标阈值：遵循仓库既有阈值与 `make test`/CI 口径；若发现不可测分支，优先重构或删除，不得以降门槛替代。
4. [ ] 后端必测矩阵：
   - [X] `:commit` 受理成功返回 `202 + receipt`；
   - [X] 同键同载荷返回同 receipt；同键异载荷返回 `idempotency_key_conflict`；
   - [ ] worker 成功执行后 task=`succeeded` 且 conversation 可刷新；
   - [ ] contract drift / determinism violation / dispatch retry exhausted / deadline exceeded 均进入 `manual_takeover_required`；
   - [ ] 取消竞态、服务重启恢复、重复派发不产生重复写入。
5. [ ] 前端/E2E 必测矩阵：
   - [X] 正式入口收到 receipt 后进入轮询并在成功后刷新 conversation；
   - [X] `manual_takeover_required` 呈现明确提示，不出现“永远 pending”；
   - [X] `288` 与 `290` 已在最终影响性合入后重跑通过。
6. [X] 证据记录：执行命令、时间戳、结果已统一写入 `docs/dev-records/dev-plan-240d-execution-log.md`，并与 `271-S5` 证据索引保持一致。

## 10. 停止线（Fail-Closed）
1. [ ] 若 `:commit` 成功返回后仍存在“无 `task_id`、无 `poll_uri`、不可查询详情”的受理结果，则本计划失败。
2. [ ] 若异步执行绕过共享 commit core、绕过 `One Door`、或绕过 `240C` 统一拦截链，则本计划失败。
3. [ ] 若高风险异常被标成 `succeeded`、或以泛化失败掩盖 partial failure，则本计划失败。
4. [ ] 若重试/恢复导致重复写入，或同一 `(tenant, conversation_id, turn_id, request_id)` 出现多条有效提交结果，则本计划失败。
5. [ ] 若正式入口在 `240D` 合入后仍依赖旧的同步 `commit` 返回语义，则本计划失败。
6. [ ] 若 `240D` 影响性合入后未按 `271` 规则重跑 `288 + 290`，则不得宣称 `271-S5` 通过。

## 11. 交付物
1. [X] `:commit` 默认异步耐久执行的后端实现。
2. [X] 复用共享 commit core 的任务执行器与错误分类逻辑。
3. [X] `manual_takeover_required` 的任务详情、取消能力与提示文案。
4. [X] 正式入口对 receipt/poll/conversation refresh 的消费改造。
5. [X] `docs/dev-records/dev-plan-240d-execution-log.md`。
6. [X] 基于最新影响性合入刷新的 `288/290` 证据与索引。

## 12. 门禁与命令（SSOT 引用）
1. [ ] 基础 Go 门禁、文档门禁、No Legacy、错误消息收敛，统一以 `AGENTS.md` 的“变更触发器矩阵”为准执行，不在本计划重复维护整张命令矩阵。
2. [ ] 本计划实施时至少命中以下 SSOT 入口：
   - [ ] `go fmt ./... && go vet ./... && make check lint && make test`
   - [ ] `make check no-legacy`
   - [ ] `make check error-message`
   - [ ] `make check doc`
   - [ ] 若触达路由/能力映射/Authz：按 `AGENTS.md` 追加 `make check routing`、`make check capability-route-map`、`make authz-pack && make authz-test && make authz-lint`
   - [ ] 若触达正式入口与端到端链路：按 `271-S5` 重跑 `make e2e` 与对应 `288/290` 用例
3. [ ] 合并前推荐执行 `make preflight`，并在执行记录中标注时间戳与结果。

## 13. 关联文档
- `docs/dev-plans/140-error-message-clarity-and-gates.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/240c-assistant-action-interceptor-and-risk-gate-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `AGENTS.md`
