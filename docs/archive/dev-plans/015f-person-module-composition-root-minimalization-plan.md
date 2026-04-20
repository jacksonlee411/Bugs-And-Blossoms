# DEV-PLAN-015F：Person 模块 Composition Root 最小化落地（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 08:55 CST）

## 背景

`DEV-PLAN-015A/015B` 指出：各模块虽然已有 `module.go` / `links.go` 文件位点，但尚未真正承接默认装配职责。

`DEV-PLAN-015E` 已经完成 Person store 实现从 `internal/server` 向模块 `infrastructure/persistence` 的迁移。下一步最自然的收口是：让 `modules/person/module.go` 不再为空壳，而是开始承接最小的默认装配入口。

## 目标与非目标

### 目标

1. [ ] 为 `modules/person/module.go` 增加最小默认构造器。
2. [ ] 让 `internal/server/handler.go` 与 `internal/server/person.go` 改为调用模块侧构造器，而不是直接 import `infrastructure/persistence`。
3. [ ] 继续缩小 `internal/server` 对模块具体实现的感知面。

### 非目标

1. [ ] 本计划不迁移 Person HTTP handler。
2. [ ] 本计划不要求 `links.go` 同步承担路由注册。
3. [ ] 本计划不把 Person 完整模块化到“零 server 感知”。

## 实施步骤

1. [X] 新建 `015F` 文档，冻结收口范围。
2. [X] 在 `modules/person/module.go` 增加最小默认构造器。
3. [X] 更新 `internal/server/handler.go` 与 `internal/server/person.go` 调用点。
4. [X] 执行最小验证：`go test ./modules/person/...`、`go test ./internal/server/...`、`make check lint`（2026-04-09 08:55 CST，本地通过）

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

1. [ ] `modules/person/module.go` 不再为空壳。
2. [ ] `internal/server` 对 Person 默认装配的直接具体实现感知继续减少。
3. [ ] 相关测试与 lint 通过。
