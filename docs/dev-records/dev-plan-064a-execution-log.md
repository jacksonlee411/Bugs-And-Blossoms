# DEV-PLAN-064A 执行日志：TP-060-05（Assistant）实施记录

> 对应计划：`docs/dev-plans/064a-test-tp060-05-assistant-conversation-intent-and-tasks.md`  
> 关联总纲：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 指引约束：`docs/dev-plans/226-test-guide-tg004-gate-caliber-change-approval.md`

## 1. 记录范围与时间

- 记录时间（UTC）：2026-03-03 02:06
- 目标：按 TP-060-05 的“下一步建议”完成执行留证（E2E 结果、阻塞点、修复与复跑、API/多模型证据、contract mismatch 专项）
- 说明：本轮未调整任何门禁口径文件（尤其是 `config/coverage/policy.yaml`）

## 2. 执行命令与结果

### 2.1 全量 E2E

- 命令：`make e2e`
- 结果：失败（Exit 2）
- 汇总：13 个用例中 12 通过、1 失败
- 与 TP-060-05 直接相关结果：
  - `e2e/tests/tp220-assistant.spec.js` 共 5 条全部通过（101/102/103/104/007）
- 阻塞用例（非 Assistant）：
  - `e2e/tests/tp060-02-orgunit-ext-query.spec.js` 失败，错误 `row bounding box missing`

### 2.2 阻塞修复与复跑

- 修复：`e2e/tests/tp060-02-orgunit-ext-query.spec.js`
  - 将行顺序断言从 `boundingBox` 坐标比较，改为 `rowgroup` 内文本顺序比较，消除无 box 场景导致的偶发失败。
- 命令：`make e2e`
- 结果：成功（Exit 0）
- 汇总：13 个用例全部通过（13/13）
- 与 TP-060-05 直接相关结果：
  - `e2e/tests/tp220-assistant.spec.js` 共 5 条全部通过（101/102/103/104/007）

### 2.3 API 与多模型证据补齐（单测）

- 命令：`go test -v ./internal/server -run "TestAssistantConversationFlow_AmbiguousCandidateConfirmAndCommit|TestAssistantConversationAPI_InvalidTurnInput|TestAssistantTurnActionHandler_CoverageMatrix|TestHandleAssistantModelProvidersValidateAndApplyAPI|TestAssistantModelProvidersAPI_Branches" -count=1`
- 结果：成功（PASS）
- 覆盖结论：
  - 会话链路（create/turn/confirm/commit）包含成功与失败分支；
  - 失败码覆盖 `invalid_request`、`assistant_candidate_not_found`、`conversation_state_invalid`；
  - 多模型覆盖 validate 正负例、apply 成功与 422 失败，以及 models 与 apply 后配置一致性断言。

### 2.4 `ai_plan_contract_version_mismatch` 专项补齐（单测）

- 变更文件：
  - `internal/server/assistant_api_coverage_test.go`（新增 commit 分支：contract version mismatch 返回码与状态回退断言）
- 命令：`go test -v ./internal/server -run "TestAssistantTurnActionHandler_CoverageMatrix|TestAssistantConversationFlow_AmbiguousCandidateConfirmAndCommit|TestAssistantConversationAPI_InvalidTurnInput|TestHandleAssistantModelProvidersValidateAndApplyAPI|TestAssistantModelProvidersAPI_Branches" -count=1`
- 结果：成功（PASS）
- 覆盖结论：
  - commit 在契约版本不匹配时返回 `409 ai_plan_contract_version_mismatch`；
  - turn 状态从 `confirmed` 回退到 `validated`，符合 fail-closed 预期。

## 3. 证据索引

- 失败上下文：`e2e/test-results/tp060-02-orgunit-ext-query-fbed6-list-ext-filter-sort-admin-/error-context.md`
- 失败截图：
  - `e2e/test-results/tp060-02-orgunit-ext-query-fbed6-list-ext-filter-sort-admin-/test-failed-1.png`
  - `e2e/test-results/tp060-02-orgunit-ext-query-fbed6-list-ext-filter-sort-admin-/test-failed-2.png`
- Trace：`e2e/test-results/tp060-02-orgunit-ext-query-fbed6-list-ext-filter-sort-admin-/trace.zip`
- E2E 服务日志：
  - `e2e/_artifacts/server.log`
  - `e2e/_artifacts/superadmin.log`
  - `e2e/_artifacts/kratosstub.log`
- 复跑成功记录：`make e2e` 控制台输出（13 passed）
- 单测证据锚点：
  - `internal/server/assistant_api_test.go`
  - `internal/server/assistant_api_coverage_test.go`
  - `internal/server/assistant_model_providers_api_test.go`
  - `internal/server/assistant_model_providers_api_more_test.go`

## 4. 问题分流（本轮）

| 时间（UTC） | 类别 | 严重级别 | 现象 | 建议 |
| --- | --- | --- | --- | --- |
| 2026-03-03 01:20 | BUG | P1 | 全量 `make e2e` 被 `tp060-02-orgunit-ext-query` 阻塞，报错 `row bounding box missing` | 先修复/稳定 tp060-02 用例后复跑 `make e2e`，再将 TP-060-05 的全量门禁项置为完成 |
| 2026-03-03 01:39 | BUG | P2 | 修复后复跑全量 E2E，13/13 全通过 | 保持该断言策略，后续如再现需优先检查 DataGrid DOM 稳定性 |

## 5. 当前结论

1. TP-060-05 的 FE 合同证据（`tp220-assistant`）稳定通过。  
2. TP-060-05 的“全量 `make e2e` 通过”已达成（13/13）。  
3. API 与多模型证据已补齐，且 `ai_plan_contract_version_mismatch` 专项样例已补齐。  
4. 在 Tasks/Temporal 未启用前提下，TP-060-05 本轮验收闭环完成，可标记“已完成”。
