# DEV-PLAN-290B：真实 intent/action 执行链路验收与非 Mock 证据收敛计划

**状态**: 阻断中（2026-03-09 19:27 CST；Case 1 已通过，Case 2/3 阻断导致 Case 4 未执行，负测已稳定通过）

## 1. 背景（调查发现）
1. [X] `DEV-PLAN-260` Case 2 的核心验收是：输入 `在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01` 后，必须走真实 `create -> confirm -> commit` 闭环。
2. [X] 现有 `e2e/tests/tp290-librechat-real-case-matrix.spec.js` 通过 `page.route("**/internal/assistant/**")` + `route.fulfill` 造数推进阶段，不满足“真实后端链路”要求。
3. [X] 当前后端默认 action 注册表仅包含 `plan_only` 与 `create_orgunit`；当 intent.action 漂移或动作不可执行时，会返回 422 `assistant_intent_unsupported`。
4. [X] 现状结论缺口是“测试通过”和“真实运行态通过”可能不一致，因此需要新增非 mock 证据链并回灌 260/271/285 结论。

## 2. 目标与非目标
### 2.1 目标
1. [ ] Case 1~4 在 `/app/assistant/librechat` 真实入口完成，业务请求真实到达 `/internal/assistant/*`。
2. [ ] 证据必须覆盖 `intent.action -> action spec 可执行 -> phase 推进 -> commit 结果` 的完整链路。
3. [ ] 对 `assistant_intent_unsupported` 提供结构化归因：请求轮次、action 值、失败阶段、返回码、上下文 conversation/turn/request/trace。
4. [ ] 形成可复跑资产与执行日志，作为 `260/271/285` 后续判定输入。
5. [ ] 主验收仅统计成功闭环：Case 2~4 必须达到 `committed`，不得用 `manual_takeover_required` 或“可解释失败”替代通过。
6. [ ] 失败路径单列负测套件，不并入主通过统计。

### 2.2 非目标
1. [ ] 不扩展新业务能力或新增 action 类型。
2. [ ] 不放宽 `260/266/280` stopline。
3. [ ] 不允许通过 mock/前端过滤/脚本注入伪造通过。
4. [ ] 不恢复 legacy、双入口或双回执。

## 3. 决策冻结（调查发现 -> 解决决策）
| 调查发现 | 冻结决策 |
| --- | --- |
| `tp290` 使用业务 API mock | 新增 `tp290b-live`（非 mock），`tp290` 降级为“排障参考”，不再作为通过证据 |
| unsupported 缺归因 | 每个 Case 输出 `intent-action-assertions.json`，失败时必须落盘 action/phase/error |
| action 白名单窄 | 强断言 Case 1=`plan_only`，Case 2/3/4=`create_orgunit`，不满足即失败 |
| 证据可与真实模型脱节 | 强制记录 `model_provider/model_name/model_revision`；出现 `deterministic/builtin-intent-extractor` 直接失败 |
| 历史结论可能依赖弱证据 | `290B` 完成前将相关结论视为“待复核”；完成后按 290B 结果刷新 |

## 4. 实施入口与范围
1. [X] 实施入口：`e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`（已新建）。
2. [ ] 实施范围：Case 1~4 的用户输入、确认、提交、任务轮询、会话回读与证据固化。
3. [ ] 依赖计划：`260/271/280/285/290/292`（以 SSOT 约束为准）。
4. [X] 证据目录：`docs/dev-records/assets/dev-plan-290b/`（已创建并提交模板文件）。
5. [X] 执行日志：`docs/dev-records/dev-plan-290b-execution-log.md`（已创建模板）。

## 5. Readiness（开工前必须满足）
1. [ ] 环境可用：`E2E_BASE_URL`、`E2E_SUPERADMIN_BASE_URL`、`E2E_KRATOS_ADMIN_URL` 可达，且租户/登录流程可跑通。
2. [ ] 模型配置可用：`ASSISTANT_MODEL_CONFIG_JSON` 有效，且对应 provider key 已注入环境变量。
3. [ ] 正式入口可访问：`/app/assistant/librechat` 非白屏，输入框可用。
4. [ ] 运行态不强制 iframe 承载；若出现 iframe，判定为 stopline 风险，记录并阻断通过。
5. [ ] 已确认本轮不使用 `page.route("**/internal/assistant/**")` 与 `route.fulfill` 对业务接口造数。
6. [ ] 测试数据基线已建置并验真：至少包含 `AI治理办公室` 与可稳定复现 Case 4 的多候选父组织数据（含 `共享服务中心` 候选）。
7. [ ] 基线快照已落盘：`docs/dev-records/assets/dev-plan-290b/tp290b-data-baseline.json`（记录 tenant、关键 org、as_of 与校验时间）。

## 6. 文件级实施拆解（直接执行）
### 6.0 T0：测试数据基线建置（硬阻断）
1. [ ] 新增或复用基线建置脚本/步骤，确保 Case 1~4 所需组织数据稳定存在。
2. [ ] 通过 API 或页面回读验证基线命中：`AI治理办公室` 可唯一定位，`共享服务中心` 保持多候选场景稳定。
3. [ ] 生成并提交 `tp290b-data-baseline.json`，字段至少包含：`tenant_id`、`as_of`、`required_orgs`、`candidate_snapshot`、`validated_at`。
4. [ ] 若基线校验失败，阻断后续 T1~T5，不得进入主验收。

### 6.1 T1：新建非 Mock 测试骨架
1. [X] 新建文件：`e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`。
2. [X] 复用函数：`setupTenantAdminSession`、`openFormalEntry`、`sendFromFormalEntry`、`collectDOMEvidence`（已平移并适配）。
3. [X] 删除/禁止：`installCaseMock`、`configureCaseProgression`、任何 `route.fulfill` 业务返回（主验收脚本未使用该类 mock）。
4. [X] 新增 `network recorder`：仅记录请求与响应，不修改请求结果。

### 6.2 T2：真实链路采样与对账
1. [ ] 记录全部 `POST /internal/assistant/*` 请求路径、状态码、响应体关键字段。
2. [ ] 每轮输入后，从页面气泡读取 `conversation_id/turn_id/request_id`。
3. [ ] 使用同一 `appContext.request` 调用 `GET /internal/assistant/conversations/{conversation_id}`，抓取 latest turn 快照。
4. [ ] 抽取并落盘：`intent.action`、`phase`、`state`、`error_code`、`model_provider/model_name/model_revision`。

### 6.3 T3：Case 级强断言（逐轮）
1. [ ] Case 1 输入 `你好`：断言 action=`plan_only`，且无 `:confirm/:commit`。
2. [ ] Case 2 输入 `在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01` + `确认`：断言 action=`create_orgunit`，phase 至少覆盖 `await_commit_confirm -> committing -> committed`。
3. [ ] Case 3 输入缺字段后补全：断言 `await_missing_fields -> await_commit_confirm -> committing -> committed`。
4. [ ] Case 4 输入候选选择：断言 `await_candidate_pick -> await_candidate_confirm -> await_commit_confirm -> committing -> committed`。
5. [ ] 任一 Case 命中 `assistant_intent_unsupported`：立即失败并输出失败证据文件。
6. [ ] 主验收中禁止以 UI 提交按钮替代“对话确认触发提交”；提交触发必须由对话输入链路完成。

### 6.4 T4：异步提交链路验证
1. [ ] `:commit` 返回 202 receipt（含 `task_id/poll_uri`）后，轮询 `GET /internal/assistant/tasks/{task_id}`。
2. [ ] 主验收通过条件：任务终态必须为 `succeeded`；出现 `manual_takeover_required`、`failed`、`canceled` 或超时均判主验收失败。
3. [ ] 终态后强制 `GET conversation` 刷新，主验收要求最终状态为 `committed`。

### 6.5 T5：证据资产落盘
1. [X] 每个 Case 固定输出：`case-{id}-page.png`、`case-{id}-dom.json`、`case-{id}-network.har`、`case-{id}-trace.zip`、`case-{id}-phase-assertions.json`（脚本已实现落盘逻辑）。
2. [X] 每个 Case 新增输出：`case-{id}-intent-action-assertions.json`、`case-{id}-conversation-snapshot.json`、`case-{id}-model-proof.json`（脚本已实现落盘逻辑）。
3. [X] 汇总索引：`tp290b-live-evidence-index.json`，记录状态、输入向量、期望/实际 phase、证据路径、stale_on 条件（模板与 afterAll 写入逻辑已落地）。

### 6.6 T6：负测套件（与主验收分离）
1. [X] 新增负测文件：`e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`。
2. [X] 负测覆盖：已落地 `assistant_intent_unsupported`（稳定复现），并补 `manual_takeover_required` 可见性探针与任务超时归因探针（结果以证据文件记录，命中受环境影响）。
3. [X] 负测结果仅用于风险与归因证明，不计入 Case 1~4 主通过统计。

## 7. 通过/失败判定矩阵（执行时逐项勾选）
| 维度 | 通过条件 | 失败条件 |
| --- | --- | --- |
| 链路真实性 | 未出现业务接口 mock；请求真实进入 `/internal/assistant/*` | 出现 `page.route("**/internal/assistant/**")` 或 `route.fulfill` 造数 |
| Action 有效性 | Case 1=`plan_only`；Case 2/3/4=`create_orgunit` | action 为空、漂移、或无可解释映射 |
| 阶段推进 | 实际 phase 与 Case 预期路径一致，且 Case 2~4 最终 `committed` | 跳阶段、回退、停滞，或终态非 `committed` |
| 模型证据 | provider/model/revision 可追溯，且非 deterministic fallback | 回退到 deterministic 仍计入通过 |
| 错误归因 | 失败时可回溯 request/turn/action/error_code | 仅有黑盒失败，无结构化归因 |

## 8. 停止线（Fail-Closed）
1. [ ] 发现业务 API mock 拦截或造数，整轮作废。
2. [ ] 发现前端本地重算替代后端 phase，整轮作废。
3. [ ] 发现 `assistant_intent_unsupported` 但未落盘 action/phase/上下文证据，整轮作废。
4. [ ] 发现 `deterministic/builtin-intent-extractor` 被计入通过，整轮作废。
5. [ ] 出现双入口、双回执、外挂容器承担业务回执，按 `266/280` 直接失败。
6. [ ] 主验收中出现 `manual_takeover_required` 或任何非 `succeeded` 终态，整轮作废（转入负测结论）。

## 9. 执行命令序列（可直接复制执行）
1. [ ] 主验收（成功路径）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on`
2. [ ] 负测（失败归因）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-negative.spec.js --workers=1 --trace on`
3. [ ] 若改动 Go 后端：`go fmt ./... && go vet ./... && make check lint && make test`
4. [ ] 文档门禁：`make check doc`
5. [ ] PR 前对齐：`make preflight`

## 10. 产物模板（本计划固定格式）
### 10.1 `case-{id}-intent-action-assertions.json` 最小字段
1. [ ] `case_id`
2. [ ] `conversation_id`
3. [ ] `turn_id`
4. [ ] `request_id`
5. [ ] `trace_id`
6. [ ] `intent_action_expected`
7. [ ] `intent_action_actual`
8. [ ] `phase_expected_path`
9. [ ] `phase_observed_path`
10. [ ] `error_code`
11. [ ] `passed`

### 10.2 `case-{id}-model-proof.json` 最小字段
1. [ ] `model_provider`
2. [ ] `model_name`
3. [ ] `model_revision`
4. [ ] `fallback_detected`
5. [ ] `proof_source`（conversation snapshot path）

## 11. 结论刷新规则
1. [X] 若 290B 结果与 `290/285/271` 现状冲突，必须在对应计划追加“290B 复核结论”条目。
2. [X] 未完成 290B 前，不得将“tp290 mock 通过”作为真实链路通过依据。
3. [ ] `290B` 证据更新时间必须晚于最近影响性合入时间。
4. [X] 结论回写为强制步骤，不完成以下文件更新不得将 290B 状态改为“已完成”：
   - [X] `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`（标注 mock 证据降级与 290B 替代关系）
   - [X] `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`（更新 S5/S6 证据引用）
   - [X] `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`（更新封板证据来源）
   - [X] `docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`（补充 superseded 注记）
   - [X] `docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`（作为当前主证据索引）

## 12. 交付物
1. [X] 本计划文档：`docs/dev-plans/290b-librechat-live-intent-action-chain-evidence-plan.md`。
2. [X] 非 mock E2E：`e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`。
3. [X] 负测 E2E：`e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`。
4. [X] 证据目录：`docs/dev-records/assets/dev-plan-290b/`。
5. [X] 数据基线快照：`docs/dev-records/assets/dev-plan-290b/tp290b-data-baseline.json`（模板已创建，待实跑覆盖）。
6. [X] 证据索引：`docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`（模板已创建，待实跑覆盖）。
7. [X] 执行日志：`docs/dev-records/dev-plan-290b-execution-log.md`（模板已创建，待补实跑记录）。

## 13. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/289-librechat-260-m2-m4-implementation-closure-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/dev-plans/290a-librechat-pending-placeholder-bubble-fix-plan.md`
- `docs/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`

## 14. 实施后变化与交互效果（明确）
### 14.1 290B 实施后带来的变化
1. [ ] 证据口径变化：`tp290`（mock）不再作为“通过证据”，`tp290b-live`（非 mock）成为 260 Case 真实性主证据。
2. [ ] 请求链路变化：验证必须以真实 `/internal/assistant/*` 请求/回包为准，禁止通过 `route.fulfill` 造数推进 phase。
3. [ ] 归因能力变化：出现 `assistant_intent_unsupported` 时，必须同时给出 `conversation_id/turn_id/request_id/trace_id/intent.action/phase/error_code`。
4. [ ] 模型可追溯变化：每个 Case 固化 `model_provider/model_name/model_revision`，并显式判定是否命中 fallback。
5. [ ] 结论治理变化：`260/271/285` 的“已通过”结论需以 290B 证据刷新，不再引用 mock 通过结果。

### 14.2 一个 Case 的交互效果（Case 2）
输入向量固定：
1. [ ] T1：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`
2. [ ] T2：`确认`

用户可见交互（聊天流内）：
1. [ ] 第 1 轮后，助手在官方消息气泡内返回草案摘要与确认提示（无外挂容器、无双气泡）。
2. [ ] 用户输入“确认”后，助手进入提交流程反馈（同一正式链路内可见）。
3. [ ] 提交成功后，助手在气泡内给出成功回执（含可读结果字段，如 `org_code`）。

后台真实执行链路（必须被证据命中）：
1. [ ] `POST /internal/assistant/conversations`
2. [ ] `POST /internal/assistant/conversations/{conversation_id}/turns`（T1）
3. [ ] `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:confirm`（T2 输入“确认”后触发）
4. [ ] `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:commit`（同次 T2 链路内触发）
5. [ ] `GET /internal/assistant/tasks/{task_id}`（轮询终态）
6. [ ] `GET /internal/assistant/conversations/{conversation_id}`（终态回读）

Case 2 的强断言（通过条件）：
1. [ ] `intent.action=create_orgunit`
2. [ ] `phase` 至少覆盖：`await_commit_confirm -> committing -> committed`
3. [ ] 无 `assistant_intent_unsupported`
4. [ ] `model_provider/model_name/model_revision` 已落盘，且非 `deterministic/builtin-intent-extractor`
5. [ ] stopline 通过：无双入口、无双回执、无外挂容器、同轮单气泡
6. [ ] 提交触发来源为对话输入链路，不得依赖页面按钮作为主触发路径

## 15. 当前进展（2026-03-09 19:27 CST）
1. [X] 前端运行时单测通过：`runtime.test.ts` 9/9 通过，覆盖确认词解析、候选选择解析、phase 意图解析与 `failed` 可见文案映射。
2. [X] `tp290b` 负测套件通过：`tp290b-neg-001~004` 全部通过（其中 `neg-002/004` 在 `assistant_intent_unsupported` 前置场景按 `probe_skipped` 落盘，不再因环境能力缺失误判脚本失败）。
3. [X] 证据与执行日志已回填：`docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`、`tp290b-data-baseline.json`、`docs/dev-records/dev-plan-290b-execution-log.md` 已更新到本轮结果。
4. [X] 文档门禁已通过：`make check doc` 通过。
5. [ ] 主验收 `tp290b-e2e-001~004` 未全绿：当前仅 Case 1 稳定通过，Case 2/3 阻断，Case 4 未进入执行。

## 16. 仍待解决问题（阻断清单）
1. [ ] Case 2 阻断：真实后端持续返回 `intent.action=plan_only`（`model_provider=deterministic`），链路仅出现 `POST .../turns`，未触发 `:confirm/:commit`，无法进入 `committed`。
2. [ ] Case 3 阻断：执行中偶发未进入 formal 气泡链路（`data-assistant-binding-key` 缺失），失败截图停留在 `SuperAdmin / Tenants`，说明运行态稳定性仍不足。
3. [ ] Case 4 未执行：串行执行在 Case 2/3 失败后提前终止，当前无有效主验收证据。
4. [ ] `typecheck` 仍失败：存在大量历史类型错误（含 `librechat-data-provider/react-query` 缺失等），虽非本轮增量引入，但会影响全仓质量门禁闭环。

## 17. 下一步（执行顺序冻结）
1. [ ] 先修运行态前置：确保真实后端从 `plan_only/deterministic` 恢复到可执行 `create_orgunit` 链路（否则主验收无法成立）。
2. [ ] 修复/稳定 Case 3 入口与消息挂载时序，确保 `latestFormalBubble` 前置可稳定命中。
3. [ ] 重新串行重跑 `tp290b-e2e-001~004`，目标是 Case 2~4 全部达到 `committed + task.succeeded`。
4. [ ] 重跑后覆盖写入 `tp290b-live-evidence-index.json` 与 `tp290b-data-baseline.json`，并同步刷新 `260/271/285` 关联结论引用。
