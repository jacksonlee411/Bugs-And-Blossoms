# DEV-PLAN-015E：Person 默认装配向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 08:50 CST）

## 背景

`DEV-PLAN-015B` 的 `P1` 要求逐步把默认装配从 `internal/server` 收回模块侧。

当前 `person` 是最适合作为首个切片的路径之一，因为：

1. [ ] `person` 默认装配链相对简单，主要是 PG store / memory store。
2. [ ] 现有逻辑不涉及 OrgUnit / SetID / Assistant 这类高耦合路径。
3. [ ] 通过迁移 `person` 的 store 实现，可以验证“`handler.go` 直接调用模块构造器”的最小模式。

## 目标与非目标

### 目标

1. [ ] 将 `person` 的 PG / memory store 实现迁入 `modules/person/infrastructure/persistence`。
2. [ ] 让 `internal/server/handler.go` 的默认装配改为调用模块侧构造器。
3. [ ] 保持现有 handler 行为与测试基本不变。

### 非目标

1. [ ] 本计划不处理 `person` 的 HTTP handler 迁移。
2. [ ] 本计划不处理 `internal/server/person.go` 的全部瘦身，只先收口默认装配实现。
3. [ ] 本计划不扩展到 `staffing/jobcatalog/orgunit`。

## 实施步骤

1. [X] 新建 `015E` 文档，冻结最小切片范围。
2. [X] 将 `person` store 实现迁入模块 `infrastructure/persistence`。
3. [X] 更新 `internal/server/handler.go` 默认装配，改为使用模块侧构造器。
4. [X] 在 `internal/server/person.go` 保留薄包装，稳住既有测试与调用点。
5. [X] 执行最小验证：`go test ./modules/person/...`、`go test ./internal/server/...`、`make check lint`（2026-04-09 08:50 CST，本地通过）

## 测试与覆盖率

覆盖率与门禁口径遵循仓库 SSOT：

- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`

本计划命中的最小验证为：

1. [ ] `go test ./modules/person/...`
2. [ ] `go test ./internal/server/...`
3. [ ] `make check lint`

## 验收标准

1. [ ] `internal/server/handler.go` 不再直接持有 `person` store 具体实现。
2. [ ] `person` 默认装配已开始从 server 向模块侧移动。
3. [ ] 相关测试与 lint 通过。
