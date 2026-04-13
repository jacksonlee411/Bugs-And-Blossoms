# DEV-PLAN-288B 执行日志

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

**状态**: 已完成（2026-03-10 06:04 CST；`tp288` mock 契约收敛、反回归断言、`tp288b-live` 非 mock 验收、证据索引与引用回写均已完成）

## 1. 实施摘要
1. [X] 已在 `e2e/tests/tp288-librechat-real-entry-evidence.spec.js` 收敛 `:commit -> 202 receipt` mock 契约，并补充 receipt/task 反回归断言。
2. [X] 已新增 `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`，真实跑通 `create -> confirm -> commit(receipt) -> poll(task) -> refresh(conversation)`。
3. [X] 已生成固定命名资产：`tp288b-case-1-page.png`、`tp288b-case-1-dom.json`、`tp288b-case-1-network.har`、`tp288b-case-1-trace.zip`、`tp288b-case-1-receipt-task-assertions.json`。
4. [X] 已生成索引：`docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json`。
5. [X] 已回写 `DEV-PLAN-288/271/285` 与 `AGENTS.md` 文档地图引用。

## 2. 执行记录
| 时间（CST） | 执行人 | 命令 | 结果 | 备注 |
| --- | --- | --- | --- | --- |
| 2026-03-10 05:58-05:59 | Codex | `node --check e2e/tests/tp288-librechat-real-entry-evidence.spec.js && node --check e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js` | 通过 | 两个脚本语法检查通过 |
| 2026-03-10 05:59-06:00 | Codex | `rg -n 'page\.route\("\*\*/internal/assistant/\*\*"|route\.fulfill\(' e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js` | 通过 | 新 live 脚本未使用业务接口 mock |
| 2026-03-10 06:00-06:01 | Codex | `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --workers=1 --trace on` | 通过 | `tp288-e2e-001/002` 共 2 条用例通过，新增 receipt/task 断言已生效 |
| 2026-03-10 06:02-06:03 | Codex | `pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288b-librechat-live-task-receipt-contract.spec.js --workers=1 --trace on` | 通过 | `tp288b-live-001` 真实跑通，任务终态 `succeeded`，conversation 最终 `committed` |

## 3. 关键结论
1. [X] `tp288` mock 成功路径已无旧 conversation DTO 回包，commit 仅返回 `202 receipt`。
2. [X] 未再出现 `/internal/assistant/tasks/undefined`、`/internal/assistant/tasks/` 空尾，且未出现 `assistant_task_dispatch_failed`。
3. [X] `tp288b-live` 已证明正式入口真实消费 `receipt -> poll -> refresh`，并保持 `single_channel=true`、`single_formal_entry=true`、`single_assistant_bubble=true`。
4. [X] 本轮 live 证据命中 `model_provider=openai`、`model_name=gpt-5.2`、`fallback_detected=false`。

## 4. 产物清单
1. [X] `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`
2. [X] `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
3. [X] `docs/dev-records/assets/dev-plan-288b/tp288b-case-1-page.png`
4. [X] `docs/dev-records/assets/dev-plan-288b/tp288b-case-1-dom.json`
5. [X] `docs/dev-records/assets/dev-plan-288b/tp288b-case-1-network.har`
6. [X] `docs/dev-records/assets/dev-plan-288b/tp288b-case-1-trace.zip`
7. [X] `docs/dev-records/assets/dev-plan-288b/tp288b-case-1-receipt-task-assertions.json`
8. [X] `docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json`
9. [X] `docs/archive/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`

## 5. 后续约束
1. [X] `288B` 已完成，不再保留“待实施”或“依赖 `290B` 阻断关闭”口径。
2. [ ] 若 `240C/240D/240E`、formal submit 主链或模型配置主源发生影响性合入，必须重跑 `tp288` 与 `tp288b-live` 并刷新本日志与索引。
3. [ ] 发 PR 前执行 `make check doc` 与 `make preflight`。
