# DEV-PLAN-015I：Staffing Assignment 组合根最小化收口（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 10:28 CST）

## 背景

在 `DEV-PLAN-015D` 之后，`staffing` 的 Assignment 分层回流已经修复，但 `internal/server/staffing.go` 仍直接感知：

1. [ ] `modules/staffing/infrastructure/persistence`
2. [ ] `modules/staffing/services`
3. [ ] 由两者自行拼装出的 Assignment facade

这说明 `modules/staffing/module.go` 仍是空壳，尚未开始承接模块内部的最小组合根职责。

考虑到 `staffing` 的 Position/Memory store 仍与大量 `internal/server` 测试耦合，本轮先不做大迁移，而是先把 Assignment 这一条链的模块内装配入口建立起来。

## 目标与非目标

### 目标

1. [ ] 让 `modules/staffing/module.go` 开始承接 Assignment 相关的最小构造职责。
2. [ ] 让 `internal/server/staffing.go` 不再直接拼 `persistence + services` 的 Assignment facade。
3. [ ] 在不扩大 `staffing` 迁移范围的前提下，继续缩小 `internal/server` 对模块内部实现的感知面。

### 非目标

1. [ ] 本计划不迁移 `staffingPGStore` / `staffingMemoryStore` 的具体实现。
2. [ ] 本计划不调整 `handler.go` 的 Position/Assignment 默认装配分支。
3. [ ] 本计划不改动 Staffing HTTP handler。
4. [ ] 本计划不新增 `staffing` 的 lint 例外。

## 实施步骤

1. [X] 新建 `015I` 文档，冻结本轮范围。
2. [X] 在 `modules/staffing/module.go` 增加 Assignment 相关构造入口。
3. [X] 更新 `internal/server/staffing.go` 调用点，改为使用模块组合根。
4. [X] 执行最小验证：`go test ./modules/staffing/...`、`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 10:28 CST，本地通过）

## 验收标准

1. [ ] `modules/staffing/module.go` 不再为空壳。
2. [ ] `internal/server/staffing.go` 不再直接 import Assignment 持久化实现与 facade 组装细节。
3. [ ] 相关测试与门禁通过。
