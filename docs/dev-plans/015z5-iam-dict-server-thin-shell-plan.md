# DEV-PLAN-015Z5：IAM Dict Store 向模块侧收缩为 Server 薄壳（承接 DEV-PLAN-015Z）

**状态**: 已完成（2026-04-09 14:08 CST）

## 背景

在 `015AA` 之后，`dict` 的主实现已经迁入 `modules/iam/infrastructure/persistence`，但 [`internal/server/dicts_store.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/dicts_store.go) 仍保留一层较厚的兼容实现：

1. [ ] `dictPGStore` 虽主要委派到模块侧，但仍在 server 侧直接持有多组 helper 包装。
2. [ ] `dictMemoryStore` 的测试兼容 helper 也仍留在 server 侧。
3. [ ] 这些 helper 使 `internal/server` 继续承担一部分本应由模块组合根承接的“可复用存储包装”职责。

相较已经完成的 `setid` 薄壳化，`dict` 仍是 `internal/server` 中更显著的剩余厚块，因此需要继续把 helper 能力前移到 `modules/iam`，再将 server 收缩为兼容薄壳。

## 目标与非目标

### 目标

1. [X] 将 `dictPGStore` 的通用委派与事件 helper 能力前移到 `modules/iam/module.go`。
2. [X] 将 `dictMemoryStore` 的测试兼容 helper 能力前移到 `modules/iam/module.go`。
3. [X] 让 `internal/server/dicts_store.go` 收缩为“保留旧类型外观 + 委派到模块侧”的薄壳。
4. [X] 保持现有 `internal/server` 测试入口兼容，不在本刀强行改写测试体系。

### 非目标

1. [ ] 本计划不直接迁移 [`internal/server/dicts_release.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/dicts_release.go) 的发布流程逻辑。
2. [ ] 本计划不移除 server 侧 `dictPGStore` / `dictMemoryStore` 类型名。
3. [ ] 本计划不改变现有 `DictStore` 对外契约。
4. [ ] 本计划不处理 `setid/dict` 以外的其它剩余尾巴。

## 实施方案

### 模块侧前移

在 [`modules/iam/module.go`](/home/lee/Projects/Bugs-And-Blossoms/modules/iam/module.go) 新增：

1. [X] `DictPGStore` 包装类型：
   - [X] 承接 `List* / Create* / Disable* / Correct*` 等委派。
   - [X] 承接 `SubmitDictEvent` / `SubmitValueEvent` helper。
2. [X] `DictMemoryStore` 包装类型：
   - [X] 保留 `Dicts` / `Values` 状态访问。
   - [X] 承接 `ResolveSourceTenant` / `ValuesForTenant` helper。

### Server 侧收缩

在 [`internal/server/dicts_store.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/dicts_store.go)：

1. [X] `dictPGStore` 改为嵌入模块侧 `*iam.DictPGStore`。
2. [X] 保留 `pool` 字段与懒初始化，以兼容历史测试直接字面量构造 `&dictPGStore{pool: ...}` 的入口。
3. [X] `submitDictEvent` / `submitValueEvent` 改为委派到模块侧 helper。
4. [X] `dictMemoryStore` 改为嵌入模块侧 `*iam.DictMemoryStore`。
5. [X] `resolveSourceTenant` / `valuesForTenant` 改为委派到模块侧 helper。

## 测试与验证

本次命中 Go 生产代码，按仓库 SSOT 执行最小闭环验证：

1. [X] `go test ./internal/server -run 'TestDict'`
2. [X] `go test ./internal/server -run 'TestNewHandlerWithOptions_DictResolverTypedNilError'`
3. [X] `go test ./modules/iam/...`
4. [X] `make check doc`
5. [X] `make check lint`

## 验收标准

1. [X] `modules/iam/module.go` 已开始承接 `dict` 的 PG/Memory helper 包装职责。
2. [X] `internal/server/dicts_store.go` 已从实现层包装收缩为兼容薄壳。
3. [X] 现有 server 侧测试入口保持兼容。
4. [X] 本地最小验证通过。
