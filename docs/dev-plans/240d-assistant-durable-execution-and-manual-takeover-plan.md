# DEV-PLAN-240D：Assistant 耐久执行与人工接管优先计划（承接 240-M5）

**状态**: 规划中（2026-03-07 23:59 CST）

## 1. 背景
1. [ ] 当前提交链路仍偏同步直提，抖动与失败场景缺少统一耐久恢复路径。
2. [ ] `240` 已冻结“高风险动作 partial failure 默认人工接管优先”，需落到任务编排主链。

## 2. 目标与非目标

### 2.1 目标
1. [ ] 将 commit 主链切换为 `receipt + async task` 的耐久执行模式。
2. [ ] 固化重试/退避/超时策略与 `MANUAL_TAKEOVER_REQUIRED` 进入条件。
3. [ ] 确保任务断点恢复与幂等重放可审计。

### 2.2 非目标
1. [ ] 不在本计划内改写 `260` 的业务语义与 Case 定义。
2. [ ] 不在本计划内扩大自动补偿范围；默认人工接管优先。

## 3. 实施步骤
1. [ ] 将 commit 请求改为创建任务并返回 receipt（同步只做受理与校验）。
2. [ ] 在任务执行器中落地标准状态：`QUEUED/EXECUTING/SUCCEEDED/FAILED/MANUAL_TAKEOVER_REQUIRED`。
3. [ ] 配置重试与退避上限，超限后进入人工接管。
4. [ ] 记录任务审计：`actor_id/request_id/trace_id` 与状态迁移证据。
5. [ ] 补齐失败回放、重复提交、服务重启恢复测试。

## 4. 停止线（Fail-Closed）
1. [ ] 若“已受理但不可追踪”任务仍存在，则本计划失败。
2. [ ] 若高风险 partial failure 未进入人工接管默认路径，则本计划失败。
3. [ ] 若任务重试导致重复写入，则本计划失败。

## 5. 验收标准
1. [ ] commit 主链默认异步耐久执行，receipt 可查询全过程状态。
2. [ ] 人工接管触发条件稳定且可追踪。
3. [ ] 任务恢复与幂等重放在服务重启后可用。

## 6. 门禁与命令（SSOT 引用）
1. [ ] `go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] `make e2e`
3. [ ] `make check no-legacy`
4. [ ] `make check error-message`
5. [ ] `make check doc`

## 7. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `AGENTS.md`
