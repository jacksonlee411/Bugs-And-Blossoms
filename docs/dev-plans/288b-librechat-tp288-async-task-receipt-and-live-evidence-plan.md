# DEV-PLAN-288B：tp288 异步任务回执契约收敛与非 Mock 证据计划

**状态**: 规划中（2026-03-09 19:14 CST；已确认根因：tp288 commit mock 仍返回旧会话 DTO，导致前端按新链路轮询 `/internal/assistant/tasks/undefined` 并统一报 `assistant_task_dispatch_failed`）

## 0. 直接实施 TL;DR
1. [ ] 先改 `tp288` mock：`POST ...:commit` 只能返回 `202 receipt(task_id/poll_uri)`，禁止回旧 DTO。
2. [ ] 再补反回归断言：禁止出现 `/internal/assistant/tasks/undefined` 与 `assistant_task_dispatch_failed`。
3. [ ] 新增 `tp288b-live` 非 mock 用例，跑真实 `receipt -> poll -> refresh`。
4. [ ] 固化证据到 `docs/dev-records/assets/dev-plan-288b/`，回写 `288/271/285` 引用口径。

## 1. 背景（调查发现）
1. [X] `240D` 之后，`commit` 新契约为 `202 receipt + task_id/poll_uri`，前端主链按 `poll_uri` 轮询任务详情。
2. [X] 当前 `tp288` mock 仍存在“旧 DTO 回包”路径，与新契约不一致。
3. [X] 契约失配导致前端出现 `/internal/assistant/tasks/undefined` 请求，并落到统一错误 `assistant_task_dispatch_failed`。
4. [X] `290B` 已推进“非 mock + 真实链路证据”主口径，`tp288` 需要同级补强，避免 `266/271/285` 继续消费弱证据。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 冻结 `tp288` 的 commit/poll mock 契约：`commit` 只返回 receipt，不再返回旧会话 DTO。
2. [ ] 消除 `/internal/assistant/tasks/undefined` 访问路径，并将其纳入反回归断言。
3. [ ] 为 `tp288` 增补非 mock 验收链路与证据索引，作为 `266` 子域真实证据补强输入。
4. [ ] 明确 `tp288` 与 `290B` 的分工边界：`290B` 管 Case 1~4 主业务闭环，`288B` 管正式入口单通道与异步任务回执契约一致性。

### 2.2 非目标
1. [ ] 不新增业务 action 类型，不改变 `260` Case 语义。
2. [ ] 不放宽 `266/280/285` stopline。
3. [ ] 不恢复 legacy 或双入口兜底，不引入“旧 DTO 兼容分支”。

## 3. 决策冻结（问题 -> 决策）
| 问题 | 决策 |
| --- | --- |
| commit mock 仍返回旧 DTO | `tp288` mock 仅允许返回 `202 receipt`；禁止返回会话 DTO 作为 commit 主回包 |
| 前端请求 `/tasks/undefined` | 新增硬断言：任务轮询 URL 必须包含非空 `task_id`；命中 `undefined/empty` 直接失败 |
| `tp288` 证据与真实链路可能脱节 | 新增 `tp288b-live`（非 mock）并落盘独立证据索引 |
| 历史 `tp288` 结论可被弱证据污染 | `288B` 完成前，`tp288` 仅视为阶段性证据；完成后以 `288B` 索引刷新 `271/285` 引用口径 |

## 4. 实施输入/输出（冻结）
### 4.1 输入
1. [ ] 计划文档：`docs/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`。
2. [ ] 修订入口：`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`。
3. [ ] 新增入口：`e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`（`tp288b-` 前缀冻结）。

### 4.2 输出
1. [ ] 证据目录：`docs/dev-records/assets/dev-plan-288b/`。
2. [ ] 证据索引：`docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json`。
3. [ ] 执行日志：`docs/dev-records/dev-plan-288b-execution-log.md`。

## 5. Readiness（开工前必须满足）
1. [ ] `/app/assistant/librechat` 可访问，登录/租户链路可用。
2. [ ] 已确认主验收不允许业务接口 `page.route("**/internal/assistant/**") + route.fulfill`。
3. [ ] `tp288` 当前问题可复现并已留痕（至少一条 `/tasks/undefined` 请求证据）。
4. [ ] 变更窗口冻结：若 `240C/240D/240E` 有影响性合入，按 `271-S5` 规则重跑 `288/288B/290B`。

## 6. 实施拆解（PR 级，可直接派工）
### 6.1 PR-288B-01：tp288 mock 契约收敛
1. [ ] 文件：`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`。
2. [ ] 收敛 `:commit` 分支：统一 `202` 回包，主体字段必须含 `task_id/task_type/status/workflow_id/submitted_at/poll_uri`。
3. [ ] 删除或禁用旧 DTO commit 回包分支，避免同测内双契约并存。
4. [ ] `GET /internal/assistant/tasks/{task_id}` 的任务源必须来自 commit receipt；`task_id` 为空直接返回测试失败。
5. [ ] PR 验收：本文件内不再存在 commit 返回会话 DTO 的路径。

### 6.2 PR-288B-02：反回归断言补齐
1. [ ] 文件：`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`。
2. [ ] 新增网络断言：测试期间不得出现 `/internal/assistant/tasks/undefined`、`/internal/assistant/tasks/`（空尾）。
3. [ ] 新增错误断言：成功链路不得出现 `assistant_task_dispatch_failed`（网络体与 UI 双断言）。
4. [ ] 新增契约断言：每个 `:commit` 的 receipt 必须在后续请求中出现对应 `GET /tasks/{task_id}`。
5. [ ] PR 验收：`tp288-e2e-001/002` 均通过，且断言覆盖新回归点。

### 6.3 PR-288B-03：新增 tp288b-live 非 mock 验收
1. [ ] 新增文件：`e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`。
2. [ ] 测试入口：必须从 `/app/assistant/librechat` 进入，不允许旧入口别名。
3. [ ] 覆盖路径：`create -> confirm -> commit(receipt) -> poll(task) -> refresh(conversation)`。
4. [ ] stopline 断言：单通道发送、无外挂容器、同轮唯一 assistant 气泡、任务终态 `succeeded`。
5. [ ] 明确禁止：业务接口 mock（`page.route("**/internal/assistant/**")`）仅允许用于“监听/记录”而非造业务回包。
6. [ ] PR 验收：用例稳定通过并生成 trace。

### 6.4 PR-288B-04：证据落盘与索引
1. [ ] 新建目录：`docs/dev-records/assets/dev-plan-288b/`。
2. [ ] 固定命名产物：
   - [ ] `tp288b-case-1-page.png`
   - [ ] `tp288b-case-1-dom.json`
   - [ ] `tp288b-case-1-network.har`
   - [ ] `tp288b-case-1-trace.zip`
   - [ ] `tp288b-case-1-receipt-task-assertions.json`
3. [ ] 新增索引：`tp288b-live-evidence-index.json`，字段最小集合见第 10 节。
4. [ ] 新增执行日志：`docs/dev-records/dev-plan-288b-execution-log.md`（记录命令、时间、结果、失败回溯）。
5. [ ] PR 验收：索引中的每一条证据路径都可本地访问且可复核。

### 6.5 PR-288B-05：结论回写与引用收敛
1. [ ] 回写 `DEV-PLAN-288`：标注 `288B` 为 tp288 异步回执契约专项补强。
2. [ ] 回写 `DEV-PLAN-271`：登记 `288B` 子计划与证据新鲜度口径。
3. [ ] 如 `285` 消费了 `tp288` 相关证据，补记“已由 `288B` 新索引覆盖/刷新”。
4. [ ] PR 验收：`288/271/285` 文档引用一致，无冲突叙述。

## 7. 通过/失败判定矩阵
| 维度 | 通过条件 | 失败条件 |
| --- | --- | --- |
| commit 契约 | 仅返回 receipt（202） | 返回旧会话 DTO 或双契约并存 |
| task 轮询 | `task_id` 非空且可轮询到终态 | 请求 `/tasks/undefined`、空 task 或无法关联 receipt |
| 错误语义 | 成功链路无 `assistant_task_dispatch_failed` | 成功链路仍触发该错误 |
| 证据真实性 | `tp288b-live` 无业务 API mock | 通过 `route.fulfill` 伪造业务成功 |
| 结论可追溯 | `receipt -> task -> conversation` 可串联 | 索引字段缺失或证据不可定位 |

## 8. 停止线（Fail-Closed）
1. [ ] 发现 `tp288` 仍保留旧 DTO commit 路径，整轮作废。
2. [ ] 发现 `/tasks/undefined` 未被测试阻断，整轮作废。
3. [ ] 发现 `tp288b-live` 对业务接口做 mock，整轮作废。
4. [ ] 发现证据索引无法追溯 `receipt -> task -> conversation`，整轮作废。
5. [ ] 发现 `manual_takeover_required/failed/canceled` 被记为主通过，整轮作废。

## 9. 执行命令序列（可直接复制）
1. [ ] `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1 --trace on`
2. [ ] `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288b-librechat-live-task-receipt-contract.spec.js --workers=1 --trace on`
3. [ ] `rg -n 'page\\.route\\(\"\\*\\*/internal/assistant/\\*\\*\"|route\\.fulfill\\(' e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
4. [ ] `make check doc`
5. [ ] 发 PR 前：`make preflight`

## 10. 证据模板（固定最小字段）
### 10.1 `tp288b-case-1-receipt-task-assertions.json`
1. [ ] `case_id`
2. [ ] `conversation_id`
3. [ ] `turn_id`
4. [ ] `commit_status`
5. [ ] `receipt.task_id`
6. [ ] `receipt.poll_uri`
7. [ ] `poll_status_sequence[]`
8. [ ] `final_task_status`
9. [ ] `final_turn_state`
10. [ ] `passed`

### 10.2 `tp288b-live-evidence-index.json`
1. [ ] `id`
2. [ ] `scenario`
3. [ ] `command`
4. [ ] `executed_at`
5. [ ] `result`
6. [ ] `artifacts[]`
7. [ ] `assertions`
8. [ ] `stale_on`

## 11. 交付物（DoD）
1. [ ] `docs/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`
2. [ ] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`（契约修订 + 反回归断言）
3. [ ] `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
4. [ ] `docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json`
5. [ ] `docs/dev-records/dev-plan-288b-execution-log.md`
6. [ ] `DEV-PLAN-288/271/285` 引用回写完成且一致

## 12. 关联文档
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/290b-librechat-live-intent-action-chain-evidence-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
