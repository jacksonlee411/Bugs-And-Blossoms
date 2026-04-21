# DEV-PLAN-015M：JobCatalog Normalize Wrapper 从生产代码移除（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 10:54 CST）

## 背景

在 `DEV-PLAN-015L` 之后，`internal/server/jobcatalog.go` 已移除多组 test-only wrapper，但仍保留一个非常薄的转发函数：

1. [ ] `normalizeSetID`

该函数当前只是在 `internal/server` 中把调用转发到 `modules/jobcatalog/services.NormalizeSetID`，并不承担独立的 server 规则职责。

## 目标与非目标

### 目标

1. [ ] 从生产代码移除 `internal/server/jobcatalog.go` 中的 `normalizeSetID` 包装。
2. [ ] 将生产调用点直接改为使用模块服务入口。
3. [ ] 保持 `internal/server` 既有测试最小改动通过。

### 非目标

1. [ ] 本计划不调整 `resolveJobCatalogView` 等仍承担生产适配职责的函数。
2. [ ] 本计划不修改 JobCatalog API 行为。
3. [ ] 本计划不修改模块侧 Normalize 规则实现。

## 实施步骤

1. [X] 新建 `015M` 文档，冻结范围。
2. [X] 删除生产包装 `normalizeSetID`。
3. [X] 更新生产调用点与测试兼容 helper。
4. [X] 执行最小验证：`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 10:54 CST，本地通过）

## 验收标准

1. [ ] `internal/server/jobcatalog.go` 不再保留 `normalizeSetID` 包装。
2. [ ] 生产调用点直接面向模块服务规则。
3. [ ] 相关测试与门禁通过。
