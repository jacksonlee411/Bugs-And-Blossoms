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

## 追加修复（2026-01-30 00:13 UTC / dev）

- 修复 CI 中 `jobcatalog-smoke` 触发的 `PACKAGE_INACTIVE_AS_OF`：确保 `ensure_setid_bootstrap` 在复用既有 DEFLT 包时补齐 `v_root_valid_from` 版本。
- 新增迁移：`migrations/orgunit/20260130100000_orgunit_setid_bootstrap_deflt_effective_date.sql`。
- 重新生成 sqlc：`make sqlc-generate`。
- DB 门禁复跑：`make orgunit plan` / `make orgunit lint` / `make orgunit migrate up`（已执行到 `20260130100000`）。

## 071A owner_setid 回填记录（2026-01-30 / dev）

> 目的：处理 `OWNER_SETID_BACKFILL_MULTI_SUBSCRIBERS` / `OWNER_SETID_BACKFILL_MISSING` 的本地阻断，明确 owner_setid 归属以完成迁移验证。

### 多订阅包（手工指定 owner_setid）

统一设置为 `DEFLT`（默认包由根 SetID 持有，订阅者只读）：

| tenant_id | package_id | scope_code | package_code | owner_setid | 备注 |
| --- | --- | --- | --- | --- | --- |
| `00000000-0000-0000-0000-000000000001` | `1246e5a1-5453-4fe7-948d-2226544c5bf5` | jobcatalog | DEFLT | DEFLT | multi-subs |
| `18377f6a-3ba4-4f40-b453-c4237e9adbf6` | `1030d210-b026-4302-ac82-4f6fe6c774fa` | jobcatalog | DEFLT | DEFLT | multi-subs |
| `1df3b4d3-e93a-49e9-bb08-fcc06a9cb5ee` | `62a45c9f-ef68-4a8c-b040-563cce6e6e9a` | jobcatalog | DEFLT | DEFLT | multi-subs |
| `676ce0ef-0284-4e9c-b171-b7a111653b50` | `987e5c22-9fa9-4cc3-aebf-1552dc8bd9d3` | jobcatalog | DEFLT | DEFLT | multi-subs |
| `6fca04bc-88a9-4a61-99d9-f11633ab15a3` | `d3ab3190-2dbf-4f94-acc1-f0fbfa81e40a` | jobcatalog | DEFLT | DEFLT | multi-subs |
| `c5aaea70-8ddd-4919-842c-649d77548279` | `8fadf6ae-350b-4133-a17b-88b9d477d878` | jobcatalog | DEFLT | DEFLT | multi-subs |
| `e2b8da72-76ab-46e0-a001-72f5082436d4` | `12bf3ecb-3f4e-4cf2-9ce8-4767b9e7967b` | jobcatalog | DEFLT | DEFLT | multi-subs |

### 无订阅包（补洞）

默认设置为 `DEFLT`，用于本地迁移闭环（后续如需变更 owner_setid，需按业务规则明确）：

| tenant_id | package_id | scope_code | package_code | owner_setid | 备注 |
| --- | --- | --- | --- | --- | --- |
| `00000000-0000-0000-0000-00000000000a` | `33a0e10b-1daf-4bb8-b3ca-e5b6f5db6560` | jobcatalog | DEFLT | DEFLT | no-subs |
| `26a9fdd8-a7f5-4e9f-9ece-4feade619eb1` | `7403307e-6780-4cd3-81e4-38702b9c8933` | jobcatalog | DEFLT | DEFLT | no-subs |
| `2e87c5de-1c1e-4cef-9717-448b2c3f2bbf` | `738820f9-adbf-42a4-b914-b96d92b418ae` | jobcatalog | DEFLT | DEFLT | no-subs |
| `34b97399-adef-4691-983d-924c6868d3da` | `e2dd6397-98e8-432d-8431-a14b17ef2f82` | jobcatalog | DEFLT | DEFLT | no-subs |
| `355f7382-e5bb-4f2a-a0f3-267a58b701c4` | `4799f519-ef19-4d3a-85b4-954db4fe0515` | jobcatalog | DEFLT | DEFLT | no-subs |
| `49b6bc2f-dfce-4299-9b04-6c44eaf24e44` | `c4d52a2b-2ab8-4113-92f8-3131ec7047df` | jobcatalog | DEFLT | DEFLT | no-subs |
| `4b741a66-487a-469e-8e3a-9d322113d6e4` | `5804e769-f77d-44d8-ace2-aa8b7f6c39f3` | jobcatalog | DEFLT | DEFLT | no-subs |
| `6a29c16e-c2ed-47cb-a155-4d58e4b7c17f` | `7584f215-1694-4e09-9736-f032fe0efa42` | jobcatalog | DEFLT | DEFLT | no-subs |
| `743fd986-6f07-46a8-8d5d-df397120f552` | `643c6b45-6620-4f30-8539-cd5d79da16dd` | jobcatalog | DEFLT | DEFLT | no-subs |
| `760d6d0a-2a83-472e-bb6b-9660c194a1b9` | `2e16ca70-a1db-4b72-8768-30a39978ee76` | jobcatalog | DEFLT | DEFLT | no-subs |
| `76384e47-d50d-489c-8dc8-bcbe980e4032` | `c3c1ba73-cae7-4eb6-b886-4fb15c2e869f` | jobcatalog | DEFLT | DEFLT | no-subs |
| `79c7e5c3-16a6-4366-8ffa-5caa72e4e072` | `6ca31915-722e-497a-9e93-1b399ef9608e` | jobcatalog | DEFLT | DEFLT | no-subs |
| `9c012b79-b753-4b6b-817e-fc9169748423` | `1d6c8582-a4ce-4327-82ae-82831ce391b6` | jobcatalog | DEFLT | DEFLT | no-subs |
| `9f520f7f-250d-4ec9-a979-c39d49800eea` | `88b74afb-f88c-47dd-adaa-969dad990774` | jobcatalog | DEFLT | DEFLT | no-subs |
| `bbd23bf2-a15f-406d-93e7-c95ea882ac3d` | `03e08128-cd0f-40d9-8166-af5d5b4a1579` | jobcatalog | DEFLT | DEFLT | no-subs |
| `ca733458-af35-48cf-a627-67e989629f24` | `f721c9a3-21d4-47b8-b4da-5c86da6d2a42` | jobcatalog | DEFLT | DEFLT | no-subs |
| `db57091f-10ea-42d6-b49f-eb52f67824f5` | `71d469be-4aee-480e-9d35-cd0f481531c0` | jobcatalog | DEFLT | DEFLT | no-subs |
| `e579e3e0-775b-4fdc-9429-4de188b9207f` | `64a11b86-613a-4f0e-9e7e-045e04ecbe52` | jobcatalog | DEFLT | DEFLT | no-subs |
| `f508dd10-6f9a-44b4-9d08-5ba1be337e9d` | `2e93ebb8-92ff-458e-a800-4b014cf06004` | jobcatalog | DEFLT | DEFLT | no-subs |

### SMOKE 包特殊处理（避免 owner_setid overlap）

- tenant `00000000-0000-0000-0000-00000000000a` 的 `SMOKE` 包需要独立 owner_setid 以避免与 `DEFLT` 冲突。
- 处理方式：
  - 创建 SetID：`SMK01`（name=Smoke Owner）。
  - `scope_package`：`package_id=00000000-0000-0000-0000-00000000d101` 设置 `owner_setid=SMK01`。
  - `scope_package_versions`：同步 `owner_setid=SMK01` 且 `status=active`，保障 `jobcatalog-smoke` 可用。

## 071A E2E 修复（2026-01-30 / dev）

- 修复 `tp060-02-master-data` E2E：创建 scope package 时补齐 `owner_setid`，避免 `POST /orgunit/api/scope-packages` 的 `invalid_request`。
