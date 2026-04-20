# DEV-PLAN-015P：Staffing Assignment 默认 Memory 装配向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 11:10 CST）

## 背景

在 `DEV-PLAN-015O` 之后，`AssignmentStore` 的默认 PG 装配已经直接走模块入口，但 memory 分支仍然沿用：

1. [ ] `newStaffingMemoryStore()` 同时承担 `PositionStore` 与 `AssignmentStore`

这意味着 Staffing Assignment 的默认 memory 装配仍停留在 `server` 侧组合实现上。

## 目标与非目标

### 目标

1. [ ] 为 `modules/staffing` 补一个最小的 Assignment memory store。
2. [ ] 将 `handler.go` 中 `AssignmentStore` 的默认 memory 装配改为直接调用模块入口。
3. [ ] 保持 `PositionStore` 的 memory 分支现状不变。

### 非目标

1. [ ] 本计划不迁移 `staffingMemoryStore` 的 Position 相关实现。
2. [ ] 本计划不修改 `staffingMemoryStore` 现有测试组织。
3. [ ] 本计划不改变 Staffing API 行为。

## 实施步骤

1. [X] 新建 `015P` 文档，冻结范围。
2. [X] 新增模块侧 Assignment memory store 与构造器。
3. [X] 更新 `internal/server/handler.go` 的 Assignment memory 默认装配。
4. [X] 执行最小验证：`go test ./modules/staffing/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 11:10 CST，本地通过）

## 验收标准

1. [ ] `AssignmentStore` 的默认 memory 装配直接走 `modules/staffing` 入口。
2. [ ] `PositionStore` memory 分支现状保持不变。
3. [ ] 相关测试与门禁通过。
