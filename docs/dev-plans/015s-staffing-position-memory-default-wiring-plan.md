# DEV-PLAN-015S：Staffing Position 默认 Memory 装配向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 13:16 CST）

## 背景

在 `DEV-PLAN-015R` 之后，`Position` 的领域类型与 port 已经前移到 `modules/staffing`，但默认 memory 装配仍停留在：

1. [ ] `internal/server/handler.go` 继续用 `newStaffingMemoryStore()` 作为默认 `PositionStore`
2. [ ] 运行时默认 memory Position 行为仍由 `internal/server` 持有

这意味着 Position 这条链路的默认运行时装配，仍未开始向模块侧收口。

## 目标与非目标

### 目标

1. [ ] 为 `modules/staffing` 增加最小的 Position memory store。
2. [ ] 将 `internal/server/handler.go` 的默认 memory `PositionStore` 装配改为直接走模块入口。
3. [ ] 保持现有测试兼容用的 `staffingMemoryStore` 不变。

### 非目标

1. [ ] 本计划不迁移 `staffingPGStore` 的 Position PG 实现。
2. [ ] 本计划不删除 `internal/server/staffing.go` 中的 Position memory 测试兼容实现。
3. [ ] 本计划不改变 Staffing Position API 行为与错误语义。

## 实施步骤

1. [X] 新建 `015S` 文档，冻结范围。
2. [X] 在 `modules/staffing` 增加 Position memory store 与最小验证测试。
3. [X] 更新 `internal/server/handler.go` 的默认 memory Position 装配。
4. [X] 执行验证：`go test ./modules/staffing/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 13:20 CST，本地通过）

## 验收标准

1. [ ] `handler.go` 默认 memory Position 装配直接走 `modules/staffing` 入口。
2. [ ] `staffingMemoryStore` 继续仅承担测试兼容与本地 helper 角色。
3. [ ] 相关测试与门禁通过。
