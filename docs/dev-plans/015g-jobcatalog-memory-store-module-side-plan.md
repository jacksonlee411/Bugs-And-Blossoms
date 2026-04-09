# DEV-PLAN-015G：JobCatalog 内存 Store 向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 09:35 CST）

## 背景

`DEV-PLAN-015A/015B` 已确认：`internal/server` 仍承载较多模块内部实现与默认装配职责。

在 `jobcatalog` 上，`internal/server/jobcatalog.go` 当前同时持有：

1. [ ] `JobCatalogStore` 共享契约类型。
2. [ ] 内存 store 实现。
3. [ ] PG store 实现与 Kernel 访问。

与 `015E/015F` 已完成的 Person 收口类似，`jobcatalog` 也需要先切出最小安全切片，把“共享契约 + 内存 store + 默认内存装配”先迁回模块侧，再为后续 PG 路径收口创造稳定 seam。

## 目标与非目标

### 目标

1. [ ] 将 `JobCatalogStore` 共享类型与端口从 `internal/server` 下沉到 `modules/jobcatalog/domain`。
2. [ ] 将 `jobcatalog` 内存 store 迁移到 `modules/jobcatalog/infrastructure/persistence`。
3. [ ] 让 `modules/jobcatalog/module.go` 开始承接最小默认构造职责。
4. [ ] 让 `internal/server/handler.go` 的默认内存装配改为调用模块侧入口。

### 非目标

1. [ ] 本计划不迁移 `jobcatalogPGStore`。
2. [ ] 本计划不重写 JobCatalog HTTP API handler。
3. [ ] 本计划不要求一次性清空 `internal/server/jobcatalog.go` 中的历史实现。
4. [ ] 本计划不修改 `015C` 的门禁策略，只在既有范围内收口。

## 实施步骤

1. [X] 新建 `015G` 文档，冻结本次切片范围。
2. [X] 在 `modules/jobcatalog/domain/types` / `domain/ports` 建立共享契约。
3. [X] 在 `modules/jobcatalog/infrastructure/persistence` 落地内存 store。
4. [X] 在 `modules/jobcatalog/module.go` 增加最小构造器。
5. [X] 更新 `internal/server` 调用点，仅改默认内存装配与类型引用。
6. [X] 执行最小验证：`go test ./modules/jobcatalog/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 09:35 CST，本地通过）

## 验收标准

1. [ ] `internal/server` 不再自带 JobCatalog 内存 store 具体实现。
2. [ ] `modules/jobcatalog/module.go` 不再为空壳。
3. [ ] JobCatalog 的共享契约不再定义在 `internal/server`。
4. [ ] 相关测试与 lint 通过。
