# DEV-PLAN-015H：JobCatalog PG Store 向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 10:08 CST）

## 背景

`DEV-PLAN-015G` 已完成 `jobcatalog` 共享契约与内存 store 的模块侧收口，但 `internal/server/jobcatalog.go` 仍保留大块 PG store 与 Kernel 访问实现。

这意味着：

1. [ ] `internal/server` 仍直接承载 JobCatalog 的模块内部持久化职责。
2. [ ] `handler.go` 的默认 PG 装配仍未完全转向模块入口。
3. [ ] `jobcatalog` 的 composition root 仍停留在“只有 memory constructor”的半收口状态。

因此需要继续下一刀：把 PG store 实现迁回 `modules/jobcatalog/infrastructure/persistence`，让 `internal/server` 只保留协议/视图适配。

## 目标与非目标

### 目标

1. [ ] 将 `jobcatalogPGStore` 与相关 PG helper 迁移到模块侧。
2. [ ] 为 `modules/jobcatalog/module.go` 增加 `NewPGStore(...)`。
3. [ ] 让 `internal/server/handler.go` 的 JobCatalog 默认 PG 装配改为调用模块入口。
4. [ ] 在不扩大生产依赖面的前提下，保持既有 `internal/server` 测试可迁移、可验证。

### 非目标

1. [ ] 本计划不重写 JobCatalog API handler。
2. [ ] 本计划不要求把 `jobcatalog.go` 剩余的 view / adapter 逻辑一并迁走。
3. [ ] 本计划不改变 JobCatalog 的数据库契约或事件函数。
4. [ ] 本计划不新增门禁豁免。

## 实施步骤

1. [X] 新建 `015H` 文档，冻结范围。
2. [X] 将 JobCatalog PG store 与 helper 迁入模块 `infrastructure/persistence`。
3. [X] 更新 `modules/jobcatalog/module.go` 与 `internal/server/handler.go` 的默认 PG 装配。
4. [X] 将 `internal/server` 中对旧具体类型/helper 的测试依赖收口为最小兼容层。
5. [X] 执行最小验证：`go test ./modules/jobcatalog/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 10:08 CST，本地通过）

## 验收标准

1. [ ] `internal/server/jobcatalog.go` 不再持有 JobCatalog PG store 具体实现。
2. [ ] `modules/jobcatalog/module.go` 同时承接 memory/PG 两类最小构造职责。
3. [ ] `handler.go` 的 JobCatalog 默认装配只依赖模块入口。
4. [ ] 相关测试与 lint 通过。
