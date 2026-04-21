# DEV-PLAN-015J：JobCatalog Server 侧冗余构造包装消除（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 10:37 CST）

## 背景

在 `DEV-PLAN-015G/015H` 之后：

1. [ ] `internal/server/handler.go` 已直接调用 `modules/jobcatalog` 的模块入口。
2. [ ] `internal/server/jobcatalog.go` 中残留的 `newJobCatalogMemoryStore` / `newJobCatalogPGStore` 已不再承担生产装配职责。

这两个函数目前主要只为 `internal/server` 测试提供兼容入口，继续保留在生产代码中会让 `server` 看起来仍像 JobCatalog 的装配入口。

## 目标与非目标

### 目标

1. [ ] 从生产代码移除 `internal/server/jobcatalog.go` 中的冗余构造包装。
2. [ ] 将这两个兼容入口下沉到测试侧 helper，不影响现有测试组织。
3. [ ] 继续缩小 `internal/server` 在 JobCatalog 模块上的装配表面积。

### 非目标

1. [ ] 本计划不修改 JobCatalog HTTP API。
2. [ ] 本计划不改动 JobCatalog store 的模块侧实现。
3. [ ] 本计划不要求同步清理所有历史测试命名。

## 实施步骤

1. [X] 新建 `015J` 文档，冻结范围。
2. [X] 删除 `internal/server/jobcatalog.go` 中的生产构造包装。
3. [X] 在测试 helper 中保留兼容构造入口。
4. [X] 执行最小验证：`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 10:37 CST，本地通过）

## 验收标准

1. [ ] 生产代码中的 `internal/server/jobcatalog.go` 不再导出/承载 JobCatalog store 构造包装。
2. [ ] 现有 `internal/server` 测试仍能通过。
3. [ ] 相关 lint 与文档门禁通过。
