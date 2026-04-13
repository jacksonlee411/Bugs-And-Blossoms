# DEV-PLAN-288B：tp288 异步任务回执契约收敛与非 Mock 证据计划

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-10 06:04 CST；`PR-288B-01 ~ PR-288B-05` 已全部完成：`tp288` mock `:commit` 已冻结为 `202 receipt`，`/tasks/undefined` 与 `assistant_task_dispatch_failed` 已被反回归断言封堵，`tp288b-live` 非 mock 用例与证据索引/执行日志已落地，并已回写 `288/271/285` 引用口径）

## 0. 直接实施 TL;DR
1. [X] 已先改 `tp288` mock：`POST ...:commit` 只返回 `202 receipt(task_id/poll_uri)`，不再回旧 DTO。
2. [X] 已补反回归断言：禁止出现 `/internal/assistant/tasks/undefined` 与 `assistant_task_dispatch_failed`。
3. [X] 已新增 `tp288b-live` 非 mock 用例，真实跑通 `receipt -> poll -> refresh`。
4. [X] 已固化证据到 `docs/dev-records/assets/dev-plan-288b/`，并回写 `288/271/285`。

## 1. 背景（调查发现）
1. [X] `240D` 之后，`commit` 新契约为 `202 receipt + task_id/poll_uri`，前端主链按 `poll_uri` 轮询任务详情。
2. [X] `288B` 启动前的 `tp288` mock 仍保留旧 DTO 回包路径，与新契约不一致。
3. [X] 契约失配会诱发 `/internal/assistant/tasks/undefined` 请求，并落到统一错误 `assistant_task_dispatch_failed`。
4. [X] `290B` 已产出主业务闭环的真实证据，`288B` 负责把正式入口的 async receipt/task 契约补强为同级强证据。

## 2. 目标与非目标
### 2.1 目标
1. [X] 冻结 `tp288` 的 commit/poll mock 契约：`commit` 只返回 receipt，不再返回旧会话 DTO。
2. [X] 消除 `/internal/assistant/tasks/undefined` 访问路径，并纳入反回归断言。
3. [X] 为 `tp288` 增补非 mock 验收链路与证据索引，作为 `266` 子域真实证据补强输入。
4. [X] 明确 `tp288` 与 `290B` 的分工边界：`290B` 管 Case 1~4 主业务闭环，`288B` 管正式入口 async receipt/task 契约一致性。

### 2.2 非目标
1. [X] 不新增业务 action 类型，不改变 `260` Case 语义。
2. [X] 不放宽 `266/280/285` stopline。
3. [X] 不恢复 legacy 或双入口兜底，不引入旧 DTO 兼容分支。

## 3. 决策冻结（问题 -> 决策）
| 问题 | 决策 |
| --- | --- |
| commit mock 仍返回旧 DTO | `tp288` mock 仅允许返回 `202 receipt`；禁止返回会话 DTO 作为 commit 主回包 |
| 前端请求 `/tasks/undefined` | 新增硬断言：任务轮询 URL 必须包含非空 `task_id`；命中 `undefined/empty` 直接失败 |
| `tp288` 证据与真实链路可能脱节 | 新增 `tp288b-live`（非 mock）并落盘独立证据索引 |
| 历史 `tp288` 结论可被弱证据污染 | `288B` 完成后以 `tp288b-live-evidence-index.json` 补强 `288/271/285` 引用口径 |

## 4. 实施输入/输出（冻结）
### 4.1 输入
1. [X] 计划文档：`docs/archive/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`。
2. [X] 修订入口：`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`。
3. [X] 新增入口：`e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`（`tp288b-` 前缀冻结）。

### 4.2 输出
1. [X] 证据目录：`docs/dev-records/assets/dev-plan-288b/`。
2. [X] 证据索引：`docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json`。
3. [X] 执行日志：`docs/archive/dev-records/dev-plan-288b-execution-log.md`。

## 5. Readiness（开工前必须满足）
1. [X] `/app/assistant/librechat` 可访问，登录/租户链路可用。
2. [X] 已确认主验收不允许业务接口 `page.route("**/internal/assistant/**") + route.fulfill`。
3. [X] `tp288` 旧问题已复现并留痕，随后由本计划反回归断言封堵。
4. [X] `290B` 已在 2026-03-10 02:47 CST 关闭 runtime/baseline 阻断，可直接复用其环境与模型结论。

## 6. 实施拆解（PR 级，可直接派工）
### 6.1 PR-288B-01：tp288 mock 契约收敛
1. [X] 文件：`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`。
2. [X] 已收敛 `:commit` 分支：统一 `202` 回包，主体字段包含 `task_id/task_type/status/workflow_id/submitted_at/poll_uri`。
3. [X] 已删除旧 DTO commit 回包路径，避免同测内双契约并存。
4. [X] `GET /internal/assistant/tasks/{task_id}` 的任务源已强制来自 commit receipt；`task_id` 为空直接判定失败。
5. [X] PR 验收：本文件内不再存在 commit 返回会话 DTO 的路径。

### 6.2 PR-288B-02：反回归断言补齐
1. [X] 文件：`e2e/tests/tp288-librechat-real-entry-evidence.spec.js`。
2. [X] 已新增网络断言：测试期间不得出现 `/internal/assistant/tasks/undefined`、`/internal/assistant/tasks/`（空尾）。
3. [X] 已新增错误断言：成功链路不得出现 `assistant_task_dispatch_failed`（网络体与 UI 双断言）。
4. [X] 已新增契约断言：每个 `:commit` 的 receipt 必须在后续请求中出现对应 `GET /tasks/{task_id}`。
5. [X] PR 验收：`tp288-e2e-001/002` 通过，且断言覆盖新回归点。

### 6.3 PR-288B-03：新增 tp288b-live 非 mock 验收
1. [X] 新增文件：`e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`。
2. [X] 测试入口：从 `/app/assistant/librechat` 进入，不使用旧入口别名。
3. [X] 已覆盖路径：`create -> confirm -> commit(receipt) -> poll(task) -> refresh(conversation)`。
4. [X] stopline 断言：单通道发送、无外挂容器、同轮唯一 assistant 气泡、任务终态 `succeeded`。
5. [X] 已明确禁止业务接口 mock：脚本内不存在 `page.route("**/internal/assistant/**")` 与 `route.fulfill` 造业务回包。
6. [X] PR 验收：用例稳定通过并生成固定命名 trace/HAR/截图/断言文件。

### 6.4 PR-288B-04：证据落盘与索引
1. [X] 已创建目录：`docs/dev-records/assets/dev-plan-288b/`。
2. [X] 已固化固定命名产物：
   - [X] `tp288b-case-1-page.png`
   - [X] `tp288b-case-1-dom.json`
   - [X] `tp288b-case-1-network.har`
   - [X] `tp288b-case-1-trace.zip`
   - [X] `tp288b-case-1-receipt-task-assertions.json`
3. [X] 已新增索引：`tp288b-live-evidence-index.json`。
4. [X] 已新增执行日志：`docs/archive/dev-records/dev-plan-288b-execution-log.md`。
5. [X] PR 验收：索引中的证据路径均可本地访问并复核。

### 6.5 PR-288B-05：结论回写与引用收敛
1. [X] 已回写 `DEV-PLAN-288`：标注 `288B` 为 `tp288` async receipt 契约专项补强。
2. [X] 已回写 `DEV-PLAN-271`：登记 `288B` 子计划与证据新鲜度口径。
3. [X] 已回写 `DEV-PLAN-285` 及其 readiness/execution 引用：补记 `288B` 新索引。
4. [X] PR 验收：`288/271/285` 引用一致，无冲突叙述。

## 7. 通过/失败判定矩阵
| 维度 | 通过条件 | 结果 |
| --- | --- | --- |
| commit 契约 | 仅返回 receipt（202） | [X] 已满足 |
| task 轮询 | `task_id` 非空且可轮询到终态 | [X] 已满足 |
| 错误语义 | 成功链路无 `assistant_task_dispatch_failed` | [X] 已满足 |
| 证据真实性 | `tp288b-live` 无业务 API mock | [X] 已满足 |
| 结论可追溯 | `receipt -> task -> conversation` 可串联 | [X] 已满足 |

## 8. 停止线（Fail-Closed）
1. [X] 已确认 `tp288` 不再保留旧 DTO commit 路径。
2. [X] 已确认 `/tasks/undefined` 被测试阻断。
3. [X] 已确认 `tp288b-live` 未对业务接口做 mock。
4. [X] 已确认证据索引可追溯 `receipt -> task -> conversation`。
5. [X] 已确认主通过只接受任务终态 `succeeded`，未把 `manual_takeover_required/failed/canceled` 计为通过。

## 9. 执行命令序列（已执行）
1. [X] `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1 --trace on`
2. [X] `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288b-librechat-live-task-receipt-contract.spec.js --workers=1 --trace on`
3. [X] `rg -n 'page\.route\("\*\*/internal/assistant/\*\*"|route\.fulfill\(' e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
4. [ ] `make check doc`
5. [ ] 发 PR 前：`make preflight`

## 10. 证据模板（固定最小字段）
### 10.1 `tp288b-case-1-receipt-task-assertions.json`
1. [X] `case_id`
2. [X] `conversation_id`
3. [X] `turn_id`
4. [X] `commit_status`
5. [X] `receipt.task_id`
6. [X] `receipt.poll_uri`
7. [X] `poll_status_sequence[]`
8. [X] `final_task_status`
9. [X] `final_turn_state`
10. [X] `passed`

### 10.2 `tp288b-live-evidence-index.json`
1. [X] `id`
2. [X] `scenario`
3. [X] `command`
4. [X] `executed_at`
5. [X] `result`
6. [X] `artifacts[]`
7. [X] `assertions`
8. [X] `stale_on`

## 11. 交付物（DoD）
1. [X] `docs/archive/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`
2. [X] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`（契约修订 + 反回归断言）
3. [X] `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
4. [X] `docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json`
5. [X] `docs/archive/dev-records/dev-plan-288b-execution-log.md`
6. [X] `DEV-PLAN-288/271/285` 引用回写完成且一致

## 12. 关联文档
- `docs/archive/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/archive/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/archive/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/archive/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/archive/dev-plans/290b-librechat-live-intent-action-chain-evidence-plan.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`

## 13. 当前进展（2026-03-10 06:04 CST）
1. [X] `tp288` mock 已按新契约收敛，commit 回包只保留 `202 receipt`。
2. [X] `tp288-e2e-001/002` 已重跑通过，新增的 receipt/task 断言已生效。
3. [X] `tp288b-live-001` 已真实跑通，命中 `openai / gpt-5.2` 且 `fallback_detected=false`。
4. [X] 证据目录、索引、执行日志与 `288/271/285` 引用回写已完成。

## 14. 联动阻断关闭结论
1. [X] `290B` 原“运行态阻断”前置已关闭：`docs/archive/dev-plans/290b-librechat-live-intent-action-chain-evidence-plan.md` 已是完成态。
2. [X] `288B` 已直接复用 `290B` 的 runtime/baseline 结论，不再重复登记旧阻断。
3. [X] `tp288` 历史“mock 与新契约并行”风险已被专项断言与 live 证据共同封堵。

## 15. 后续维护动作
1. [X] 本计划实施已结束，后续仅按 `271-S5` 新鲜度规则维护。
2. [ ] 若 `240C/240D/240E` 或 formal submit 主链再次发生影响性合入，重跑 `tp288/tp288b-live` 并刷新索引。
3. [ ] 发 PR 前执行 `make check doc` 与 `make preflight` 对齐 CI。
