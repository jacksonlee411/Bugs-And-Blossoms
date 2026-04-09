# DEV-PLAN-015AA：IAM Dict Store 向模块侧收口（承接 DEV-PLAN-015Z）

**状态**: 已完成（2026-04-09 17:15 CST）

## 背景

`DEV-PLAN-015Z` 已把 `dict` 明确识别为 `015` 剩余 backlog 中第二块最明显的高耦合存量：[`internal/server/dicts_store.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/dicts_store.go) 同时承载了 `DictStore` 契约、PG/Memory 具体实现与默认装配入口，导致 `modules/iam/module.go` 仍是空壳，`handler.go` 也继续直接装配模块内部实现。

相较 `setid`，`dict` 的依赖更独立，适合作为 `015Z` 之后的第一刀真实实施切片。

## 目标与非目标

### 目标

1. [X] 将 `dict` 的 PG/Memory 具体实现迁入 `modules/iam`。
2. [X] 让 `modules/iam/module.go` 承接 `dict` 默认装配入口。
3. [X] 将 `internal/server/dicts_store.go` 收缩为兼容薄壳，而不再承载模块内部实现主体。
4. [X] 保持现有 API、测试语义与 baseline release 入口不变。

### 非目标

1. [ ] 本计划不迁移 `dicts_release.go` 的 release 协调逻辑。
2. [ ] 本计划不处理 `setid` 剩余 server store 收口。
3. [ ] 本计划不完成 `015` 的全部封板。

## 实施步骤

1. [X] 新建 `015AA` 文档，冻结本刀范围。
2. [X] 在 `modules/iam/infrastructure/persistence/` 新增 `dict` PG/Memory store 实现，并导出 `DictStore` 契约、错误值与辅助函数。
3. [X] 扩展 `modules/iam/module.go`，提供 `NewDictPGStore` / `NewDictMemoryStore` 模块入口。
4. [X] 将 `internal/server/dicts_store.go` 改为模块侧实现的兼容薄壳，保留 release 所需的最小类型外形。
5. [X] 将 `internal/server/handler.go` 的默认 `DictStore` 装配改为经由 `modules/iam` 入口承接。
6. [X] 执行验证：`go test ./internal/server/...`、`go test ./modules/iam/...`、`make check doc`（2026-04-09 17:15 CST，本地通过）

## 验收标准

1. [X] `internal/server` 不再持有 `dict` PG/Memory 具体实现主体。
2. [X] `modules/iam/module.go` 不再为空壳，并能承接 `dict` 默认装配入口。
3. [X] `handler.go` 不再直接 new `internal/server` 中的 `dict` 具体实现。
4. [X] 相关测试与文档门禁通过。
