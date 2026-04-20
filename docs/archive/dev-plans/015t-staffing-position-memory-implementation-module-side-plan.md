# DEV-PLAN-015T：Staffing Position 内存实现向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 13:31 CST）

## 背景

在 `DEV-PLAN-015S` 之后，Staffing Position 的默认 memory 装配已经走模块侧，但 `internal/server/staffing.go` 里仍保留完整的 Position memory 实现：

1. [ ] `staffingMemoryStore` 仍直接实现 `ListPositionsCurrent`
2. [ ] `staffingMemoryStore` 仍直接实现 `CreatePositionCurrent`
3. [ ] `staffingMemoryStore` 仍直接实现 `UpdatePositionCurrent`

这说明运行时默认装配虽然已经收口，但 `internal/server` 仍持有 Position memory 的具体行为。

## 目标与非目标

### 目标

1. [ ] 将 `staffingMemoryStore` 的 Position 具体实现改为委派 `modules/staffing`。
2. [ ] 保留 `internal/server` 测试可见的 `positions` backing map。
3. [ ] 保持 Position memory 行为、错误语义和现有测试结果不变。

### 非目标

1. [ ] 本计划不迁移 `staffingPGStore` 的 Position PG 实现。
2. [ ] 本计划不删除 `staffingMemoryStore` 这个测试兼容类型。
3. [ ] 本计划不调整 Staffing Position API 契约。

## 实施步骤

1. [X] 新建 `015T` 文档，冻结范围。
2. [X] 为模块侧 Position memory store 增加共享 backing state 的构造入口。
3. [X] 将 `internal/server/staffing.go` 的 Position memory 方法改为模块侧委派。
4. [X] 执行验证：`go test ./modules/staffing/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 13:37 CST，本地通过）

## 验收标准

1. [ ] `internal/server/staffing.go` 不再保留 Position memory 的具体实现逻辑。
2. [ ] `staffingMemoryStore.positions` 仍可作为现有测试共享状态入口使用。
3. [ ] 相关测试与门禁通过。
