# DEV-PLAN-015Q：Staffing Assignment 内存实现向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 12:05 CST）

## 背景

在 `DEV-PLAN-015P` 之后，`AssignmentStore` 的默认 memory 装配已经改为直接走 `modules/staffing`，但 `internal/server/staffing.go` 中仍保留了完整的 Assignment 内存实现：

1. [ ] `staffingMemoryStore` 仍直接实现 `ListAssignmentsForPerson` / `UpsertPrimaryAssignmentForPerson`
2. [ ] `staffingMemoryStore` 仍直接实现 `CorrectAssignmentEvent` / `RescindAssignmentEvent`
3. [ ] 现有 `internal/server` 测试又直接依赖 `assigns` backing map，因此不能用大拆迁方式一次性把整个 memory store 挪走

这说明默认装配虽已收口，但 `internal/server` 仍持有 Assignment 的具体 memory 行为，和 `015B` 的 P1 目标还有一段距离。

## 目标与非目标

### 目标

1. [ ] 将 `staffingMemoryStore` 的 Assignment 具体实现改为委派给 `modules/staffing`。
2. [ ] 保留 `internal/server` 现有测试可见的 `assigns` backing map，避免把 Position memory store 一并重构。
3. [ ] 保持 API 行为、错误语义与现有测试结果不变。

### 非目标

1. [ ] 本计划不迁移 `staffingMemoryStore` 的 Position 相关实现。
2. [ ] 本计划不重写 `internal/server` 现有 Staffing 测试结构。
3. [ ] 本计划不引入新的 lint/gate 规则。

## 实施步骤

1. [X] 新建 `015Q` 文档，冻结本次切片范围。
2. [X] 为 `modules/staffing` 的 Assignment memory store 增加“共享 backing state”的最小构造入口。
3. [X] 将 `internal/server/staffing.go` 的 Assignment memory 方法改为委派模块侧实现。
4. [X] 补充最小验证，确认 shared state 兼容现有 `internal/server` 测试。
5. [X] 执行验证：`go test ./modules/staffing/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 12:16 CST，本地通过）

## 验收标准

1. [ ] `internal/server/staffing.go` 不再保留 Assignment memory 的具体实现逻辑。
2. [ ] `staffingMemoryStore.assigns` 仍可作为现有测试共享状态入口使用。
3. [ ] Position memory 行为保持原状，相关测试与门禁通过。
