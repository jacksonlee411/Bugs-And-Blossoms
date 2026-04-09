# DEV-PLAN-015Y：JobCatalog 视图服务适配入口向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 16:18 CST）

## 背景

`DEV-PLAN-015G/015H/015J` 已将 JobCatalog 的 memory/PG store 与默认装配收回模块侧，但 [`internal/server/jobcatalog.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/jobcatalog.go) 仍保留一组面向 `modules/jobcatalog/services` 的 store 适配入口。

## 目标与非目标

### 目标

1. [ ] 将 JobCatalog store 到 services 的适配入口收进 `modules/jobcatalog/module.go`。
2. [ ] 让 `internal/server/jobcatalog.go` 仅保留 server 上下文与 SetID 记录类型的薄适配。
3. [ ] 不改变 JobCatalog API 与运行时行为。

### 非目标

1. [ ] 本计划不迁移 SetID store 的 server 侧适配类型。
2. [ ] 本计划不改动 JobCatalog PG/Memory store 实现。
3. [ ] 本计划不改动 JobCatalog API 契约。

## 实施步骤

1. [X] 新建 `015Y` 文档，冻结本刀范围。
2. [X] 在 `modules/jobcatalog/module.go` 增加视图解析与 store 适配入口。
3. [X] 将 `internal/server/jobcatalog.go` 改为调用模块侧入口。
4. [X] 执行验证：`go test ./modules/jobcatalog/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 16:22 CST，本地通过）

## 验收标准

1. [ ] `internal/server/jobcatalog.go` 不再直接持有 JobCatalog store 到 services 的适配实现。
2. [ ] 模块入口能够承接 JobCatalog 视图解析的最小适配职责。
3. [ ] 相关测试与门禁通过。
