# DEV-PLAN-015Z3：OrgUnit SetID PG 实现向模块侧收缩为 Server 薄壳（承接 DEV-PLAN-015Z2）

**状态**: 已完成（2026-04-09 18:11 CST）

## 背景

`015Z2` 已将 `SetID` 的 PG 默认装配入口迁到 `modules/orgunit`，但 [`internal/server/setid.go`](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid.go) 里仍保留整块 PG 主实现，导致 server 仍同时持有“模块侧入口”和“重复生产逻辑”两份实现。

## 目标与非目标

### 目标

1. [X] 将 `internal/server/setid.go` 中的 `setidPGStore` 收缩为模块侧薄委派。
2. [X] 保留现有 `internal/server` 测试兼容外形，不直接打碎旧测试入口。
3. [X] 继续减少 server 中的 SetID 生产实现重复体。

### 非目标

1. [ ] 本计划不删除 `setidPGStore` 兼容类型本身。
2. [ ] 本计划不处理 `setIDStrategyRegistryPGStore` 的位置。
3. [ ] 本计划不修改 SetID API 契约。

## 实施步骤

1. [X] 新建 `015Z3` 文档，冻结本刀范围。
2. [X] 将 `setidPGStore` 的 PG 方法改为转发到 `modules/orgunit/infrastructure/persistence.SetIDPGStore`。
3. [X] 保留 `ensureGlobalShareSetID` / `listGlobalSetIDs` 等兼容入口，但底层实现改为模块侧承接。
4. [X] 执行验证：`go test ./modules/orgunit/...`、`go test ./internal/server/...`、`make check doc`（2026-04-09 18:11 CST，本地通过）

## 验收标准

1. [X] `internal/server/setid.go` 不再保留整块 SetID PG 主实现重复体。
2. [X] 现有 server 测试可继续通过兼容壳运行。
3. [X] 模块测试、server 测试与文档门禁通过。
