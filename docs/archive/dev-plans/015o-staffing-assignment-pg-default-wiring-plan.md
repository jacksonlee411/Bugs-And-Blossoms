# DEV-PLAN-015O：Staffing Assignment 默认 PG 装配向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 11:04 CST）

## 背景

在 `DEV-PLAN-015I` 之后，`modules/staffing/module.go` 已开始承接 Assignment 相关组合根，但 `internal/server/handler.go` 的默认 PG 装配仍沿用：

1. [ ] `newStaffingPGStore(pgStore.pool)` 同时承担 `PositionStore` 与 `AssignmentStore`

这意味着 Assignment 的默认 PG 装配仍停留在 `server` 侧组合实现上，没有真正走模块入口。

## 目标与非目标

### 目标

1. [ ] 将 `handler.go` 中 `AssignmentStore` 的默认 PG 装配改为直接调用 `modules/staffing` 入口。
2. [ ] 保持 `PositionStore` 的现状不变，避免扩大到整块 Position store 迁移。
3. [ ] 继续缩小 `internal/server` 对 Staffing Assignment 装配职责的承担。

### 非目标

1. [ ] 本计划不迁移 `staffingPGStore` 本体。
2. [ ] 本计划不修改 memory 分支的 Assignment 默认装配。
3. [ ] 本计划不调整 Staffing HTTP handler 行为。

## 实施步骤

1. [X] 新建 `015O` 文档，冻结范围。
2. [X] 更新 `internal/server/handler.go` 的 Assignment 默认 PG 装配。
3. [X] 执行最小验证：`go test ./internal/server/...`、`go test ./modules/staffing/...`、`make check lint`、`make check doc`（2026-04-09 11:04 CST，本地通过）

## 验收标准

1. [ ] `AssignmentStore` 的默认 PG 装配直接走 `modules/staffing` 入口。
2. [ ] `PositionStore` 现状保持不变。
3. [ ] 相关测试与门禁通过。
