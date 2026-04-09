# DEV-PLAN-015L：JobCatalog Test-Only Wrapper 从生产代码移除（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 10:48 CST）

## 背景

在 `DEV-PLAN-015J` 之后，`internal/server/jobcatalog.go` 已移除冗余构造包装，但仍保留若干仅供 `internal/server/jobcatalog_test.go` 使用的轻量 wrapper：

1. [ ] `normalizePackageCode`
2. [ ] `canEditDefltPackage`
3. [ ] `ownerSetIDEditable`
4. [ ] `loadOwnedJobCatalogPackages`

这些函数已不承担生产职责，继续保留在生产文件中会放大 `server` 层对 JobCatalog 规则的表面积。

## 目标与非目标

### 目标

1. [ ] 将仅测试使用的 JobCatalog wrapper 从生产代码移除。
2. [ ] 在测试 helper 中保留兼容入口，保证现有测试最小改动通过。
3. [ ] 继续缩小 `internal/server/jobcatalog.go` 的生产职责。

### 非目标

1. [ ] 本计划不修改 JobCatalog API handler。
2. [ ] 本计划不调整 `resolveJobCatalogView` 这类仍承担生产职责的适配层。
3. [ ] 本计划不修改 JobCatalog 模块侧 services 规则。

## 实施步骤

1. [X] 新建 `015L` 文档，冻结范围。
2. [X] 删除 `internal/server/jobcatalog.go` 中的 test-only wrapper。
3. [X] 在测试 helper 中补回兼容函数。
4. [X] 执行最小验证：`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 10:48 CST，本地通过）

## 验收标准

1. [ ] `internal/server/jobcatalog.go` 不再承载纯测试用途的规则包装。
2. [ ] 现有 `internal/server` JobCatalog 测试仍能通过。
3. [ ] 相关 lint 与文档门禁通过。
