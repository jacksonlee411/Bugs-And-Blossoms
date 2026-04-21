# DEV-PLAN-440 Readiness

## 状态

- 日期：2026-04-21
- owner：`DEV-PLAN-440`
- 当前结论：`DEV-PLAN-450` 已完成其 owner 范围内的直接切除与旧策略主链收口。当前仓库中，`jobcatalog` / `staffing` / `person` 三模块、`pkg/fieldpolicy`、`orgunit.setid_strategy_registry` 及其相关 dbtool/E2E/当前态 schema 输入已不再作为活体执行面存在。`440` 剩余范围收敛为 `orgunit` 自身的 SetID / scope package / owner_setid 治理面，不再被三模块与旧 capability 主链阻塞。

## 已完成收口

### 1. 三模块与旧策略主链已退出当前态

- `modules/jobcatalog/**`
- `modules/staffing/**`
- `modules/person/**`
- `e2e/tests/m3-smoke.spec.js`
- `e2e/tests/tp060-02-master-data.spec.js`
- `cmd/dbtool/orgunit_stopline_capture*.go`
- `cmd/dbtool/orgunit_setid_strategy_registry_*.go`
- `scripts/db/orgunit-setid-strategy-registry-*.sh`
- `pkg/fieldpolicy/**`
- `modules/orgunit/domain/types/setid_strategy_field_decision.go`
- `internal/server/orgunit_field_policy_capabilities.go`

说明：

- 三模块目录、旧策略 PDP、registry snapshot/validate 辅助命令、相关 E2E 资产均已删除。
- `orgunit` 写前检查已从 `capability_key / setid_strategy_registry / OPA` 决议切换为模块内静态字段决议。

### 2. `orgunit` 已与 `person`、旧 capability 主链解耦

- `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`
  - 已删除 `ResolveSetIDStrategyFieldDecision(...)`
  - 已删除对 `orgunit.setid_strategy_registry` 的查询
- `internal/server/orgunit_nodes.go`
  - 已删除 `LEFT JOIN person.persons`
- `modules/orgunit/services/create_orgunit_precheck.go`
- `modules/orgunit/services/orgunit_append_version_precheck.go`
- `modules/orgunit/services/orgunit_maintain_precheck.go`
- `modules/orgunit/services/orgunit_write_service.go`

说明：

- `orgunit` 当前写链路不再依赖 `CapabilityKey`、`ResolvedSetIDStrategyFieldDecision` 或 `person.persons`。
- 旧的 capability bridge 与 registry runtime 已不再参与当前执行。

### 3. schema / migration / sqlc 输入闭环已同步收口

- `modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql`
- `modules/orgunit/infrastructure/persistence/schema/00021_orgunit_setid_strategy_registry_fields.sql`
- `modules/orgunit/infrastructure/persistence/schema/00022_orgunit_setid_strategy_registry_modes.sql`
- `migrations/orgunit/20260222193000_orgunit_setid_strategy_registry_schema.sql`
- `migrations/orgunit/20260223143000_orgunit_setid_strategy_registry_fields.sql`
- `migrations/orgunit/20260225120000_orgunit_setid_strategy_org_applicability.sql`
- `migrations/orgunit/20260227193000_orgunit_setid_strategy_registry_modes.sql`
- `migrations/orgunit/20260410113000_orgunit_setid_strategy_registry_business_unit_node_key.sql`
- `migrations/orgunit/20260411121500_orgunit_setid_strategy_registry_resolved_setid.sql`
- `internal/sqlc/schema.sql`

说明：

- `orgunit.setid_strategy_registry` 的 schema 输入与 migration 已删除。
- `make sqlc-generate` 后，`internal/sqlc/schema.sql` 不再包含 `setid_strategy_registry`。

### 4. 旧 superadmin bootstrap 已同步退场

- `internal/superadmin/handler.go`

说明：

- 租户创建时不再 seed `orgunit.setid_strategy_registry`。
- 避免新租户流程继续依赖已删除表。

## 当前剩余范围

`440` 当前只覆盖 `orgunit` 自身仍然存在的 SetID 治理面，而非三模块或旧 capability 主链：

- `internal/server/setid*.go`
- `modules/orgunit/infrastructure/persistence/setid_pg_store.go`
- `modules/orgunit/domain/ports/setid_governance.go`
- `modules/orgunit/infrastructure/persistence/schema/00005-00013`
- `migrations/orgunit` 中仍然存在的 setid / scope package / owner_setid 相关迁移
- `pkg/dict/dict.go`、`modules/orgunit/domain/fieldmeta/fieldmeta.go` 中仍保留的 `SetIDSource`
- `internal/server/orgunit_field_metadata*.go` 中仍面向 dict option / field enable candidates 的 SetID 上下文解析

## 不再成立的旧阻塞结论

以下旧结论已失效，不得继续作为当前态判断依据：

1. `jobcatalog` 仍是 `440` 的当前阻塞项。
2. `staffing` 仍是 `440` 的当前阻塞项。
3. `pkg/fieldpolicy` / `setid_strategy_registry` 仍是当前活体运行时。
4. `DEV-PLAN-440` 仍被三模块 runtime/schema 依赖阻塞。

这些命中已经由 `DEV-PLAN-450` 本轮收口清除。

## 当前停止线

### 停止线 A：`orgunit` SetID 治理面仍是活体

证据：

- `internal/server/setid.go`
- `internal/server/setid_api.go`
- `internal/server/setid_scope_api.go`
- `modules/orgunit/infrastructure/persistence/setid_pg_store.go`
- `modules/orgunit/infrastructure/persistence/schema/00005_orgunit_setid_schema.sql`
- `modules/orgunit/infrastructure/persistence/schema/00008_orgunit_setid_scope_schema.sql`

结论：

- `440` 尚未完成 SetID 根删除。
- 但该停止线已不再包含三模块与旧 capability 主链。

### 停止线 B：`owner_setid` / scope package 仍在当前态

证据：

- `modules/orgunit/domain/ports/setid_governance.go`
- `modules/orgunit/infrastructure/persistence/schema/00012_orgunit_scope_package_owner_setid.sql`
- `modules/orgunit/infrastructure/persistence/schema/00013_orgunit_scope_package_owner_setid_not_null.sql`
- `migrations/orgunit/atlas.sum`

结论：

- `owner_setid` 仍是 `440` 剩余治理面的现行契约。
- 若要宣称 `440` 完成，需继续删除 scope package / owner_setid 主链。

## 验证记录

本轮已完成并通过：

- `make sqlc-generate`
- `go test ./modules/orgunit/services/...`
- `go test ./modules/orgunit/infrastructure/persistence/...`
- `go test ./internal/server/...`

## 禁止事项

- 不得继续引用旧版本 readiness 中“jobcatalog/staffing/fieldpolicy 是当前阻塞”的结论。
- 不得把 archive / 历史调查文档中的 `setid_strategy_registry`、`jobcatalog`、`staffing` 命中误判为当前态活体。
- 不得因为 `440` 尚未完成，就回退或重建 `450` 已删除的三模块与旧策略主链。
