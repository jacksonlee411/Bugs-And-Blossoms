# DEV-PLAN-015U：Staffing Memory 兼容壳移出生产代码（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 13:48 CST）

## 背景

在 `DEV-PLAN-015T` 之后，Staffing 的 Position 与 Assignment memory 具体实现都已经回到模块侧，但 `internal/server/staffing.go` 仍保留 `staffingMemoryStore` 这一整套复合壳：

1. [ ] 生产代码里仍定义 `staffingMemoryStore`
2. [ ] 生产代码里仍保留 `newStaffingMemoryStore`
3. [ ] 这些实现当前只服务于测试兼容，不再被运行时默认装配使用

因此，这一层已经更适合被视为测试辅助代码，而不是生产实现的一部分。

## 目标与非目标

### 目标

1. [ ] 将 `staffingMemoryStore` 从生产代码移到 `internal/server/*_test.go`。
2. [ ] 保持所有现有 `internal/server` 测试调用方式不变。
3. [ ] 让 `internal/server/staffing.go` 只保留真实运行时职责。

### 非目标

1. [ ] 本计划不迁移 `staffingPGStore` 的 Position PG 实现。
2. [ ] 本计划不修改 Staffing API 契约。
3. [ ] 本计划不调整模块侧 memory store 行为。

## 实施步骤

1. [X] 新建 `015U` 文档，冻结范围。
2. [X] 在测试文件中承接 `staffingMemoryStore` 兼容壳。
3. [X] 从生产文件中删除对应定义与方法。
4. [X] 执行验证：`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 13:53 CST，本地通过）

## 验收标准

1. [ ] `internal/server/staffing.go` 不再包含 `staffingMemoryStore` 测试兼容壳。
2. [ ] 现有测试不需要改调用方式即可继续通过。
3. [ ] 相关门禁通过。
