# DEV-PLAN-015Z6：OrgUnit SetID Store 向模块侧收缩为 Server 薄壳（承接 DEV-PLAN-015Z）

**状态**: 已完成（2026-04-09 14:08 CST）

## 背景

在 `015Z1/015Z2/015Z3` 之后，`setid` 的主实现已经进入 `modules/orgunit/infrastructure/persistence`，但 [`internal/server/setid.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid.go) 仍保留一层明显的兼容包装：

1. [ ] `setidPGStore` 仍自己维持一层委派 helper。
2. [ ] `setidMemoryStore` 仍自己同步内部状态字段以兼容历史测试。
3. [ ] 这些包装虽然已比早期薄很多，但模块组合根仍未完整承接这些 helper 能力。

因此，本计划继续承接 `015` 收尾目标：将 `setid` 的 PG/Memory helper 前移到 `modules/orgunit/module.go`，让 server 侧进一步收缩为兼容薄壳。

## 目标与非目标

### 目标

1. [X] 将 `setidPGStore` 的通用 helper 前移到 [`modules/orgunit/module.go`](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/module.go)。
2. [X] 将 `setidMemoryStore` 的状态兼容包装前移到模块侧。
3. [X] 让 [`internal/server/setid.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid.go) 收缩为“保留旧类型外观 + 委派到模块侧”的薄壳。
4. [X] 保持现有 server 侧测试入口兼容。

### 非目标

1. [ ] 本计划不迁移 `setid` API/handler 层逻辑。
2. [ ] 本计划不移除 server 侧 `setidPGStore` / `setidMemoryStore` 类型名。
3. [ ] 本计划不改变 `SetIDGovernanceStore` 对外契约。
4. [ ] 本计划不处理 `setid` 以外的其它收尾事项。

## 实施方案

### 模块侧前移

在 [`modules/orgunit/module.go`](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/module.go) 新增：

1. [X] `SetIDPGStore` 包装类型，承接 `EnsureGlobalShareSetID` 等 helper。
2. [X] `SetIDMemoryStore` 包装类型，承接：
   - [X] `SetIDs / Bindings / ScopePackages / ScopeSubscriptions / GlobalScopePackages`
   - [X] `GlobalSetIDName / Seq`
   - [X] `EnsureBootstrap / CreateGlobalSetID / CreateScopePackage / CreateGlobalScopePackage` 的状态同步

### Server 侧收缩

在 [`internal/server/setid.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid.go)：

1. [X] `setidPGStore` 改为嵌入模块侧 `*orgunit.SetIDPGStore`。
2. [X] 保留 `pool` 字段与懒初始化，兼容历史测试直接构造 `&setidPGStore{pool: ...}`。
3. [X] `setidMemoryStore` 改为嵌入模块侧 `*orgunit.SetIDMemoryStore`。
4. [X] 保留 `setids/bindings/...` 等字段镜像，兼容现有测试读取内部状态。

## 测试与验证

本次命中 Go 生产代码，按仓库 SSOT 执行最小闭环验证：

1. [X] `go test ./internal/server -run 'TestSetID'`
2. [X] `go test ./modules/orgunit/...`
3. [X] `make check doc`
4. [X] `make check lint`

## 验收标准

1. [X] `modules/orgunit/module.go` 已承接 `setid` 的 PG/Memory helper 包装职责。
2. [X] `internal/server/setid.go` 已进一步收缩为兼容薄壳。
3. [X] 现有 server 侧测试入口保持兼容。
4. [X] 本地最小验证通过。
