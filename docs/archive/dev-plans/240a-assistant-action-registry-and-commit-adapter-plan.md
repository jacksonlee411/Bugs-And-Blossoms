# DEV-PLAN-240A：Assistant ActionRegistry 与 CommitAdapter 落地计划（承接 240-M2）

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-08 CST；`ActionRegistry/CommitAdapter/version_tuple OCC` 已落地，门禁已本地跑绿）

## 1. 背景
1. [X] `DEV-PLAN-240` 已完成 `M0/M1` 契约冻结，但 `create_orgunit` 仍存在核心分支写死与直连提交流程。
2. [X] 需要先完成 “No Spec, No Commit / No Adapter, No Write” 的代码化落地，才能进入后续状态机统一与耐久执行阶段。

## 2. 目标与非目标

### 2.1 目标
1. [X] 落地 `ActionRegistry`（代码内注册 + allowlist），替换核心 `if/switch` 写死路径。
2. [X] 落地 `CommitAdapter` 受控分发，禁止直连 `writeSvc.Write` 作为正式实现。
3. [X] 引入 `version_tuple` 写前 OCC 校验并 fail-closed。

### 2.2 非目标
1. [X] 不在本计划内完成状态机内存/PG 合并（由 `240B` 承接）。
2. [X] 不在本计划内切换默认异步任务执行（由 `240D` 承接）。

## 3. 实施步骤
1. [X] 新增 `ActionRegistry` 接口与默认实现，最小先注册 `create_orgunit`。
2. [X] 新增 `CommitAdapter` 注册与分发层，禁止未知 key 回退。
3. [X] 将 `create_orgunit` 路径切换到 `registry -> dry_run -> confirm -> adapter` 主链。
4. [X] 在 adapter 写门前执行 `version_tuple` OCC 校验并返回稳定错误码。
5. [X] 补齐单测：未注册 action、未注册 adapter、OCC 失败、成功路径。

## 4. 停止线（Fail-Closed）
1. [X] 任一 action 若无 spec 仍可 commit，则本计划判失败。
2. [X] 发现直写分支（绕过 adapter）仍可达，则本计划判失败。
3. [X] OCC 校验失败仍落写门，则本计划判失败。

## 5. 验收标准
1. [X] 新增动作可通过注册扩展，不再改核心提交流程分支。
2. [X] `create_orgunit` 正式路径不再依赖硬编码 capability/dry-run/commit 分支。
3. [X] `version_tuple` 校验失败返回稳定错误码且不产生写入。

## 6. 门禁与命令（SSOT 引用）
1. [X] `go fmt ./... && go vet ./... && make check lint && make test`
2. [X] `make check capability-key`
3. [X] `make check capability-route-map`
4. [X] `make check no-legacy`
5. [X] `make check doc`

## 7. 关联文档
- `docs/archive/dev-plans/240-assistant-org-transaction-orchestration-modernization-plan.md`
- `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/archive/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `AGENTS.md`
