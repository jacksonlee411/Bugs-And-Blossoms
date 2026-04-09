# DEV-PLAN-015V：Staffing PG Assignment 薄委派移出生产代码（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 14:01 CST）

## 背景

在 `DEV-PLAN-015U` 之后，`internal/server/staffing.go` 已经只剩 PG 运行时职责与少量模块契约 alias；但 `staffingPGStore` 上仍保留 4 个 Assignment 薄委派方法：

1. [ ] `ListAssignmentsForPerson`
2. [ ] `UpsertPrimaryAssignmentForPerson`
3. [ ] `CorrectAssignmentEvent`
4. [ ] `RescindAssignmentEvent`

这些方法当前不再承担实际业务逻辑，生产默认装配也早已直接走 `modules/staffing.NewAssignmentPGStore(...)`。

## 目标与非目标

### 目标

1. [ ] 将 `staffingPGStore` 的 Assignment 薄委派移出生产代码。
2. [ ] 在测试文件中提供兼容 helper，保持现有 Assignment 相关测试可继续表达“Position PG + Assignment PG 组合视图”。
3. [ ] 不触碰 Position PG 生产实现与行为。

### 非目标

1. [ ] 本计划不迁移 `staffingPGStore` 的 Position PG 具体实现。
2. [ ] 本计划不改动模块侧 Assignment PG store 行为。
3. [ ] 本计划不调整 Staffing API 契约。

## 实施步骤

1. [X] 新建 `015V` 文档，冻结范围。
2. [X] 在测试文件中增加 Assignment PG 兼容 helper。
3. [X] 将 Assignment 相关测试切换到新 helper。
4. [X] 从生产文件中删除 4 个 PG Assignment 薄委派。
5. [X] 执行验证：`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 14:09 CST，本地通过）

## 验收标准

1. [ ] `internal/server/staffing.go` 不再包含 PG Assignment 薄委派。
2. [ ] Assignment 相关测试继续通过且调用方式清晰。
3. [ ] 相关门禁通过。
