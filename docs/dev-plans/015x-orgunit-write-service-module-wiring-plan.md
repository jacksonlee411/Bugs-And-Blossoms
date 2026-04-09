# DEV-PLAN-015X：OrgUnit Write Service 默认装配向模块侧收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 16:02 CST）

## 背景

虽然 `modules/orgunit/module.go` 与 `links.go` 已落点，但当前 [`internal/server/handler.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/handler.go) 仍直接负责 `OrgUnitWriteService` 的默认装配：

1. [ ] 直接调用 `orgunitservices.NewOrgUnitWriteService(...)`。
2. [ ] 在 PG 路径下直接拼 `orgunitpersistence.NewOrgUnitPGStore(...)`。

这说明 `orgunit/module.go` 仍未真正承接最小组合根职责。

## 目标与非目标

### 目标

1. [ ] 让 `modules/orgunit/module.go` 暴露 `OrgUnitWriteStore` / `OrgUnitWriteService` 的最小构造入口。
2. [ ] 将 `internal/server/handler.go` 的 `OrgUnitWriteService` 默认装配切换到模块入口。
3. [ ] 在不改变运行时行为的前提下，继续缩小 `internal/server` 的模块装配表面积。

### 非目标

1. [ ] 本计划不迁移 `internal/server` 中现有 `orgUnitPGStore` 的运行时实现。
2. [ ] 本计划不改动 OrgUnit HTTP API。
3. [ ] 本计划不触碰 SetID/Field Policy/Strategy Registry 的实现位置。

## 实施步骤

1. [X] 新建 `015X` 文档，冻结本刀范围。
2. [X] 在 `modules/orgunit/module.go` 增加最小模块构造入口。
3. [X] 将 `internal/server/handler.go` 的 `OrgUnitWriteService` 默认装配切到模块入口。
4. [X] 执行验证：`go test ./internal/server/...`、`go test ./modules/orgunit/...`、`make check lint`、`make check doc`（2026-04-09 16:05 CST，本地通过）

## 验收标准

1. [ ] `modules/orgunit/module.go` 不再是空壳。
2. [ ] `internal/server/handler.go` 不再直接承担 `OrgUnitWriteService` 的默认拼装。
3. [ ] 相关测试与门禁通过。
