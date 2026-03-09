# DEV-PLAN-240B：Assistant 状态机统一与内存/PG 路径收敛计划（承接 240-M3）

**状态**: 已完成（2026-03-08 CST）

## 1. 背景
1. [X] 当前 `commitTurn` 与 `applyCommitTurn` 等路径存在双处维护，状态迁移规则易漂移。
2. [X] `240A` 完成后需要将 confirm/commit/task 迁移到单一状态迁移引擎。

## 2. 目标与非目标

### 2.1 目标
1. [X] 建立统一状态迁移引擎，覆盖 `READY_FOR_CONFIRM -> ... -> SUCCEEDED/FAILED`。
2. [X] 消除内存/PG 双实现的业务规则分叉。
3. [X] 固化 `EXPIRED` 惰性判定与非法迁移阻断。

### 2.2 非目标
1. [X] 不在本计划内处理内部知识包与只读 Resolver 收口（由 `240E` 承接）。
2. [X] 不在本计划内完成前端 send/render patch（由 `284` 承接）。

## 3. 实施步骤
1. [X] 抽取统一 transition 函数（输入：当前状态+动作+guard，输出：下一状态+审计事件）。
2. [X] 将内存路径与 PG 持久化路径统一接入该 transition 函数。
3. [X] 补齐非法迁移错误码（含超时确认、越级提交、状态错配）。
4. [X] 对齐 `223` 快照字段与状态迁移审计写入时序。
5. [X] 补齐 parity 测试：内存实现与 PG 实现在同输入下输出一致。

## 4. 停止线（Fail-Closed）
1. [X] 若同一动作在内存/PG 路径得到不同下一状态，则本计划失败。
2. [X] 若非法迁移未被阻断，或错误码不稳定，则本计划失败。
3. [X] 若 `EXPIRED` 仍可继续 confirm/commit，则本计划失败。

## 5. 验收标准
1. [X] confirm/commit/task 共享同一状态机与 guard 规则。
2. [X] `READY_FOR_CONFIRM` 超 TTL 后进入 `EXPIRED` 并禁止提交。
3. [X] 状态迁移审计可回放且与会话当前状态一致。

## 6. 门禁与命令（SSOT 引用）
1. [X] `go fmt ./... && go vet ./... && make check lint && make test`
2. [X] `make check error-message`
3. [X] `make check no-legacy`
4. [X] `make check doc`

## 7. 关联文档
- `docs/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `AGENTS.md`
