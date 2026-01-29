# DEV-PLAN-071 执行日志

> 目的：记录 DEV-PLAN-071 M5（回填与验证闭环）的执行进展与证据。

## 完成范围

- M5：stable scope 回填、shared-only 默认包与订阅补齐、证据记录与核对。

## 变更摘要

- 迁移：`migrations/orgunit/20260130080000_orgunit_setid_scope_shared_defaults.sql`（shared-only 默认包/订阅回填）。
- 迁移：`migrations/orgunit/20260130081000_orgunit_setid_scope_tenant_backfill.sql`（租户 stable scope 订阅与 tenant-only DEFLT 回填）。
- Schema：`modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`（shared-only 默认包/订阅逻辑补齐）。

## 本地验证（2026-01-29 23:52 UTC / dev）

- `make orgunit plan`：通过。
- `make orgunit lint`：通过。
- `make orgunit migrate up`：执行到 `20260130081000`。
- `./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/orgunit?format=goose"`：已更新 `migrations/orgunit/atlas.sum`。

## 证据快照（/tmp/dev-plan-071-evidence.txt）

- global tenant：`00000000-0000-0000-0000-000000000000`
- stable scopes：
  - `jobcatalog|jobcatalog|tenant-only|true`
  - `orgunit_geo_admin|orgunit|shared-only|true`
  - `orgunit_location|orgunit|shared-only|true`
  - `person_credential_type|person|shared-only|true`
  - `person_education_type|person|shared-only|true`
  - `person_school|person|shared-only|true`
- local tenant SetID 数量：`2`
- local tenant stable scope 订阅（as_of=current_date）：
  - `jobcatalog|2`
  - `orgunit_geo_admin|2`
  - `orgunit_location|2`
  - `person_credential_type|2`
  - `person_education_type|2`
  - `person_school|2`
- local tenant 默认包：`jobcatalog|1`
- global 默认包（shared-only）：
  - `orgunit_geo_admin|1`
  - `orgunit_location|1`
  - `person_credential_type|1`
  - `person_education_type|1`
  - `person_school|1`
