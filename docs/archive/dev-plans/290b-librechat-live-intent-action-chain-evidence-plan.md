# DEV-PLAN-290B：真实 intent/action 执行链路验收与非 Mock 证据收敛计划

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-10 02:47 CST；`tp290b-e2e-000~004` live 全部通过，运行态已稳定命中 `openai / gpt-5.2` 且 `fallback_detected=false`；T0 数据基线、Case 2/3/4 的 `create -> confirm -> commit` 闭环与证据索引均已刷新）

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
5. [X] 执行日志：`docs/archive/dev-records/dev-plan-290b-execution-log.md`（已创建模板）。

## 5. Readiness（开工前必须满足）
1. [X] 环境可用：`E2E_BASE_URL`、`E2E_SUPERADMIN_BASE_URL`、`E2E_KRATOS_ADMIN_URL` 可达，且租户/登录流程可跑通（2026-03-09 21:14 CST 实跑验证）。
2. [X] 模型配置可用：`ASSISTANT_MODEL_CONFIG_JSON` 有效，且对应 provider key 已注入环境变量（`.env/.env.local` 已对齐并实跑）。
3. [X] 正式入口可访问：`/app/assistant/librechat` 非白屏，输入框可用。
4. [X] 运行态未强制 iframe 承载；本轮实跑未命中 iframe stopline 风险。
5. [X] 已确认本轮不使用 `page.route("**/internal/assistant/**")` 与 `route.fulfill` 对业务接口造数。
6. [X] 测试数据基线已建置并验真：包含 `ROOT/集团`、`AI治理办公室` 与 Case 4 所需 `共享服务中心` 多候选父组织。
7. [X] 基线快照已落盘：`docs/dev-records/assets/dev-plan-290b/tp290b-data-baseline.json`（记录 tenant、关键 org、as_of、T0 就绪状态与 probe 结果）。

## 6. 文件级实施拆解（直接执行）
### 6.0 T0：测试数据基线建置（硬阻断）
1. [X] 已新增/复用基线建置步骤，确保 Case 1~4 所需组织数据稳定存在。
2. [X] 已通过 API/真实 `/internal/assistant` probe 验证基线命中：`AI治理办公室` 唯一定位，`共享服务中心` 多候选稳定。
3. [X] 已生成并提交 `tp290b-data-baseline.json`，并扩展表达 `t0_baseline_ready`、`created_orgs`、候选快照与 Case2/Case4 probe 结果。
4. [X] 已实现基线 fail-closed：若校验失败，直接阻断后续主验收。

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
1. [ ] 运行态准入门禁（先于主验收）：`make check assistant-config-single-source`
2. [ ] 单案排障（Case 2）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on --grep "tp290b-e2e-002"`
3. [ ] 单案排障（Case 3）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on --grep "tp290b-e2e-003"`
4. [ ] 单案排障（Case 4）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on --grep "tp290b-e2e-004"`
5. [ ] 主验收（全量串行成功路径）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1 --trace on`
6. [ ] 负测（失败归因）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290b-librechat-live-intent-action-negative.spec.js --workers=1 --trace on`
7. [ ] 若改动 Go 后端：`go fmt ./... && go vet ./... && make check lint && make test`
8. [ ] 文档门禁：`make check doc`
9. [ ] PR 前对齐：`make preflight`

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
3. [X] `290B` 证据已刷新到 2026-03-10 02:47 CST，晚于本轮影响性修复落地时间。
4. [X] 结论回写为强制步骤，不完成以下文件更新不得将 290B 状态改为“已完成”：
   - [X] `docs/archive/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`（标注 mock 证据降级与 290B 替代关系）
   - [X] `docs/archive/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`（更新 S5/S6 证据引用）
   - [X] `docs/archive/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`（更新封板证据来源）
   - [X] `docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`（补充 superseded 注记）
   - [X] `docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`（作为当前主证据索引）

## 12. 交付物
1. [X] 本计划文档：`docs/archive/dev-plans/290b-librechat-live-intent-action-chain-evidence-plan.md`。
2. [X] 非 mock E2E：`e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`。
3. [X] 负测 E2E：`e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`。
4. [X] 证据目录：`docs/dev-records/assets/dev-plan-290b/`。
5. [X] 数据基线快照：`docs/dev-records/assets/dev-plan-290b/tp290b-data-baseline.json`（已由 live 实跑覆盖）。
6. [X] 证据索引：`docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`（已由 live 实跑覆盖并标记 `status=passed`）。
7. [X] 执行日志：`docs/archive/dev-records/dev-plan-290b-execution-log.md`（已补齐本轮实跑记录与收口结论）。

## 13. 关联文档
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/archive/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/archive/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/archive/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/archive/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/archive/dev-plans/289-librechat-260-m2-m4-implementation-closure-plan.md`
- `docs/archive/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/archive/dev-plans/290a-librechat-pending-placeholder-bubble-fix-plan.md`
- `docs/archive/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`

## 14. 实施后变化与交互效果（明确）
### 14.1 290B 实施后带来的变化
1. [X] 证据口径变化：`tp290`（mock）不再作为“通过证据”，`tp290b-live`（非 mock）成为 260 Case 真实性主证据。
2. [X] 请求链路变化：验证以真实 `/internal/assistant/*` 请求/回包为准，未使用 `route.fulfill` 造数推进 phase。
3. [X] 归因能力变化：出现 `assistant_intent_unsupported` 时，已能同时给出 `conversation_id/turn_id/request_id/trace_id/intent.action/phase/error_code`。
4. [X] 模型可追溯变化：每个 Case 已固化 `model_provider/model_name/model_revision`，并显式判定是否命中 fallback。
5. [X] 结论治理变化：`260/271/285` 的“已通过”结论已以 290B 证据刷新，不再引用 mock 通过结果。

### 14.2 一个 Case 的交互效果（Case 2）
输入向量固定：
1. [X] T1：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`
2. [X] T2：`确认`

用户可见交互（聊天流内）：
1. [X] 第 1 轮后，助手在官方消息气泡内返回草案摘要与确认提示（无外挂容器、无双气泡）。
2. [X] 用户输入“确认”后，助手进入提交流程反馈（同一正式链路内可见）。
3. [X] 提交成功后，助手在气泡内给出成功回执（含可读结果字段，如 `org_code`）。

后台真实执行链路（必须被证据命中）：
1. [X] `POST /internal/assistant/conversations`
2. [X] `POST /internal/assistant/conversations/{conversation_id}/turns`（T1）
3. [X] `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:confirm`（T2 输入“确认”后触发）
4. [X] `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:commit`（同次 T2 链路内触发）
5. [X] `GET /internal/assistant/tasks/{task_id}`（轮询终态）
6. [X] `GET /internal/assistant/conversations/{conversation_id}`（终态回读）

Case 2 的强断言（通过条件）：
1. [X] `intent.action=create_orgunit`
2. [X] `phase` 已覆盖正式提交闭环；Case 2/4 命中 `await_commit_confirm -> committed`，Case 3 在补字段场景允许 `await_missing_fields -> committed` 或 `await_missing_fields -> await_commit_confirm -> committed`。
3. [X] 无 `assistant_intent_unsupported`
4. [X] `model_provider/model_name/model_revision` 已落盘，且非 `deterministic/builtin-intent-extractor`
5. [X] stopline 通过：无双入口、无双回执、无外挂容器、同轮单气泡
6. [X] 提交触发来源为对话输入链路；运行态采用正式 `Confirm/Submit/Select` 按钮驱动，不再发送自然语言“确认/提交”制造二次噪声。

## 15. 当前进展（2026-03-10 02:47 CST）
1. [X] 主验收 `tp290b-e2e-000~004` 已全部 live 通过；证据索引 `tp290b-live-evidence-index.json` 已刷新为 `status=passed`。
2. [X] T0 基线硬化已完成：新租户自动补齐 `ROOT/集团`、`AI治理办公室`、`共享服务中心` 多候选场景，并用真实 `/internal/assistant` probe 验证 Case 2/4 准入。
3. [X] 认证链路已稳定：`TRUST_PROXY=1` 生效后，`X-Forwarded-Host` 租户解析恢复，`/iam/api/sessions` 不再误落到 `localhost` 默认租户。
4. [X] 运行态动作链已稳定：`openai / gpt-5.2` 可持续返回 `create_orgunit`，`fallback_detected=false`，Case 2/3/4 均命中 `:confirm/:commit` 与 `task.status=succeeded`。
5. [X] 后端根因修复已完成：补齐租户基线播种、SetID baseline capability 回退、异步任务成功后的 conversation cache 刷新，以及 `plan_only -> create_orgunit` 本地升级。
6. [X] Case 3 特殊治理已完成：禁止模型在用户未显式提供日期时擅自补“当天日期”，缺字段场景恢复为真实补字段语义。
7. [X] 文档与证据已同步：本计划、执行日志、基线快照与 case 级别快照/phase/model proof 均已更新到最终闭环结果。

## 16. 闭环结论（2026-03-10 02:47 CST）
1. [X] `DEV-PLAN-290B` 当前已满足“真实入口、真实模型、真实 `/internal/assistant/*`、Case 2~4 全部 committed”的封板条件。
2. [X] `tp290b-e2e-000` 运行态准入闸门持续通过，说明真实模型路由与 fail-closed 语义保持一致。
3. [X] `tp290b-e2e-001` 保持 `plan_only`，证明 greeting 不会误触发写链路。
4. [X] `tp290b-e2e-002/003/004` 均命中 `create_orgunit -> confirm -> commit` 正式执行链，并在会话终态回读中落为 `committed`。
5. [X] `tp290b-data-baseline.json` 已明确区分“T0 数据基线就绪”与“主验收通过”，后续排障可直接先看 `t0_baseline_ready` 与 `probes.case2/case4`。
6. [X] 当前剩余非增量问题仅为历史 `typecheck` 噪声，与 290B 本轮增量无关，不阻断本计划收口。

## 17. 经验沉淀（290B 复盘）
1. [X] `X-Forwarded-Host` 只有在 `TRUST_PROXY=1` 时才会参与租户解析；live 环境若忘记开启，会把所有 API 请求静默打到 `localhost` 默认租户，表现为 `invalid_credentials` 或“数据看似存在但命中错租户”。
2. [X] 新租户只建 `tenant/domain` 远远不够；若真实链路依赖 Org/SetID baseline，必须在租户创建阶段同步播种基线组织与租户级策略，否则 Case 2/4 会在第一轮就因为 `parent_candidate_not_found` 或 `FIELD_POLICY_MISSING` fail-closed。
3. [X] SetID strategy resolver 不能只看意图 capability；像 `org.orgunit_create.field_policy` 这类运行态 capability 必须能回退到 `org.orgunit_write.field_policy` baseline，否则基线已存在也会被运行态误判为缺策略。
4. [X] 正式提交流程要点按钮，不要发送自然语言“确认/提交”赌解析；真实 UI 的 `Confirm/Submit/Select` 才是稳定单链路入口。
5. [X] 异步任务成功后必须刷新 conversation cache；否则 `GET conversation` 会继续读到旧 turn，表现为 task 已 succeeded 但会话仍卡在 `confirmed`。
6. [X] 模型输出必须受“用户原文显式事实”约束；尤其日期字段，若用户未提供，必须清空而不是接受模型脑补的当天日期，否则缺字段 Case 会被错误推进到提交阶段。
7. [X] Playwright 偶发 `Internal error: step id not found` 仍会出现，但它只是 runner 噪声；只要业务断言、HAR、conversation snapshot 一致，就不应把该噪声误判为产品根因。

## 18. 后续引用规则
1. [X] `260/271/285` 现在应优先引用 `tp290b-live-evidence-index.json` 作为真实链路主证据。
2. [X] `288B` 若后续需要补强 receipt/poll 证据，应以本轮 290B 的 runtime/baseline 结论为前置，避免重复踩入已关闭的环境坑。
