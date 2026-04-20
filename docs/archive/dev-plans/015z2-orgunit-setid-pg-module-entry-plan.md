# DEV-PLAN-015Z2：OrgUnit SetID PG 默认装配入口向模块侧收口（承接 DEV-PLAN-015Z1）

**状态**: 已完成（2026-04-09 18:02 CST）

## 背景

`015Z1` 已把 `SetID` 契约与 memory 实现迁到 `modules/orgunit`，但 `handler.go` 在 PG 路径下仍直接 new `internal/server/setidPGStore`。这意味着默认装配职责还没有完全收回模块侧。

## 目标与非目标

### 目标

1. [X] 在 `modules/orgunit/infrastructure/persistence` 新增 SetID PG store 模块侧实现。
2. [X] 在 `modules/orgunit/module.go` 提供 `NewSetIDPGStore()` 入口。
3. [X] 让 `internal/server/handler.go` 的默认 PG 装配改为调用模块入口。
4. [X] 保持 `internal/server/setid.go` 的兼容外形与现有测试稳定。

### 非目标

1. [ ] 本计划不完全删除 `internal/server/setid.go` 中的 PG 兼容代码。
2. [ ] 本计划不处理 `setid` 的全部 server 薄壳消除。
3. [ ] 本计划不修改 SetID API 契约。

## 实施步骤

1. [X] 新建 `015Z2` 文档，冻结本刀范围。
2. [X] 新增 `modules/orgunit/infrastructure/persistence/setid_pg_store.go`，承接模块侧 PG 实现。
3. [X] 扩展 `modules/orgunit/module.go`，暴露 `NewSetIDPGStore()`。
4. [X] 修改 `internal/server/handler.go`，让默认 PG 装配经由 `modules/orgunit` 入口。
5. [X] 执行验证：`go test ./modules/orgunit/...`、`go test ./internal/server/...`、`make check doc`（2026-04-09 18:02 CST，本地通过）

## 验收标准

1. [X] `handler.go` 不再直接构造 `internal/server` 中的 SetID PG store。
2. [X] `modules/orgunit` 已具备 SetID PG 默认装配入口。
3. [X] 模块测试、server 测试与文档门禁通过。
