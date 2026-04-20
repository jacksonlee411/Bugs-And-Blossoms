# DEV-PLAN-015R：Staffing Position 领域类型与 Port 契约前移到模块侧（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 13:02 CST）

## 背景

截至 `DEV-PLAN-015Q`，`staffing` 的 Assignment 已经逐步向模块侧收口，但 Position 仍保留一个明显的“边界前置不足”问题：

1. [ ] `internal/server/staffing.go` 仍定义 `Position` 结构体
2. [ ] `internal/server/staffing.go` 仍定义 `PositionStore` 接口
3. [ ] `modules/staffing` 还没有 Position 的最小 domain contract

这会让后续 Position 的 memory/PG 实现收口继续被 `internal/server` 类型边界卡住。

## 目标与非目标

### 目标

1. [ ] 将 `Position` 领域类型前移到 `modules/staffing/domain/types`。
2. [ ] 将 `PositionStore` port 前移到 `modules/staffing/domain/ports`。
3. [ ] 将 `internal/server` 收敛为对模块契约的 alias/适配层，不改变现有行为。

### 非目标

1. [ ] 本计划不迁移 `staffingPGStore` 的 Position PG 实现。
2. [ ] 本计划不迁移 `staffingMemoryStore` 的 Position memory 实现。
3. [ ] 本计划不修改 Staffing Position API 行为与错误语义。

## 实施步骤

1. [X] 新建 `015R` 文档，冻结范围。
2. [X] 新增模块侧 `Position` 类型与 `PositionStore` port。
3. [X] 更新 `internal/server/staffing.go`，将 Position 类型与接口改为模块侧 alias。
4. [X] 执行验证：`go test ./modules/staffing/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 13:08 CST，本地通过）

## 验收标准

1. [ ] `modules/staffing` 已具备 Position 的最小领域契约。
2. [ ] `internal/server` 不再自持有 Position 结构与接口定义。
3. [ ] 行为不变，相关测试与门禁通过。
