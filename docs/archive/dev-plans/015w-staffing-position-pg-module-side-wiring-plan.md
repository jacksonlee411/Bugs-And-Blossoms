# DEV-PLAN-015W：Staffing Position PG 实现与默认装配向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 15:05 CST）

## 背景

在 `DEV-PLAN-015V` 之后，`internal/server/staffing.go` 已只剩 Position PG 生产实现与少量模块契约 alias。与此同时，`internal/server/handler.go` 的默认 PG 装配仍直接调用 `newStaffingPGStore(...)`。

这意味着：

1. [ ] Staffing Position 的 PG 运行时实现仍停留在 `internal/server`。
2. [ ] `handler.go` 仍未完全通过 `modules/staffing/module.go` 承接 Position PG 默认装配。

## 目标与非目标

### 目标

1. [ ] 将 Position PG 生产实现迁入 `modules/staffing/infrastructure/persistence`。
2. [ ] 通过 `modules/staffing/module.go` 暴露 Position PG 装配入口。
3. [ ] 将 `internal/server/handler.go` 的 Position PG 默认装配切换到模块侧入口。
4. [ ] 保留 `newStaffingPGStore(...)` 作为测试兼容 helper 名字，避免本刀同时扩大测试改造面。

### 非目标

1. [ ] 本计划不移除 `newStaffingPGStore(...)` 这一测试兼容 helper 名字。
2. [ ] 本计划不重写 Staffing Position API 契约。
3. [ ] 本计划不处理下一步“将兼容 helper 彻底移到 `*_test.go`”的收尾动作。

## 实施步骤

1. [X] 新建 `015W` 文档，冻结本刀范围。
2. [X] 在 `modules/staffing/infrastructure/persistence` 新增 Position PG store。
3. [X] 在 `modules/staffing/module.go` 暴露 `NewPositionPGStore(...)`。
4. [X] 将 `internal/server/handler.go` 默认 PG Position 装配改为模块侧入口。
5. [X] 将 `internal/server/newStaffingPGStore(...)` 收敛为模块侧构造器的兼容转发。
6. [X] 执行验证：`go test ./internal/server/...`、`go test ./modules/staffing/...`、`make check lint`、`make check doc`（2026-04-09 15:31 CST，本地通过）

## 验收标准

1. [ ] `internal/server` 不再持有 Position PG 具体生产实现。
2. [ ] Staffing Position 的默认 PG 装配改由模块入口承接。
3. [ ] 现有 Position PG 相关测试继续通过。
