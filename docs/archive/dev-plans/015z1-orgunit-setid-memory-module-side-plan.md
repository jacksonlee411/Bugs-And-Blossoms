# DEV-PLAN-015Z1：OrgUnit SetID Memory Store 向模块侧收口（承接 DEV-PLAN-015Z）

**状态**: 已完成（2026-04-09 17:42 CST）

## 背景

`DEV-PLAN-015Z` 已把 `setid` 识别为 `015` 剩余 backlog 中最大的 server 存量块，但它比 `dict` 更难直接整体迁移：接口、数据类型、PG/kernel 写入口与 memory 实现都揉在 [`internal/server/setid.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid.go) 一处，且测试对 `setidMemoryStore` 的内部字段有直接耦合。

因此 `015Z1` 先切出最稳的一刀：把 `SetID` 契约与 memory 实现迁到模块侧，并让默认 memory 装配先经由 `modules/orgunit` 入口承接。

## 目标与非目标

### 目标

1. [X] 将 `SetID` 相关数据类型与 `SetIDGovernanceStore` 契约前移到 `modules/orgunit/domain/ports`。
2. [X] 将 `setidMemoryStore` 迁移到 `modules/orgunit/infrastructure/persistence`。
3. [X] 让 `modules/orgunit/module.go` 提供 `NewSetIDMemoryStore()` 模块入口。
4. [X] 让 `internal/server/handler.go` 的默认 memory 装配不再直接依赖 server 内部实现。

### 非目标

1. [ ] 本计划不迁移 `setidPGStore` 与其 kernel 写入口。
2. [ ] 本计划不修改 SetID API 契约。
3. [ ] 本计划不处理 `setid` 的全部 server 存量。

## 实施步骤

1. [X] 新建 `015Z1` 文档，冻结本刀范围。
2. [X] 新增 `modules/orgunit/domain/ports/setid_governance.go`，承接 `SetID` 契约与数据类型。
3. [X] 新增 `modules/orgunit/infrastructure/persistence/setid_memory_store.go`，承接 memory 实现。
4. [X] 扩展 `modules/orgunit/module.go`，暴露 `NewSetIDMemoryStore()`。
5. [X] 将 `internal/server/setid.go` 收缩为契约别名与测试兼容薄壳；`handler.go` 的 memory 装配切到模块入口。
6. [X] 执行验证：`go test ./modules/orgunit/...`、`go test ./internal/server/...`、`make check doc`（2026-04-09 17:42 CST，本地通过）

## 验收标准

1. [X] `SetIDGovernanceStore` 契约已不再只存在于 `internal/server`。
2. [X] `setidMemoryStore` 的实现主体已迁到模块侧。
3. [X] `handler.go` 的默认 memory 装配已通过模块入口承接。
4. [X] 测试与文档门禁通过。
