# DEV-PLAN-072 记录：命名收敛差异清单与映射表（冻结）

**状态**: 冻结（2026-02-03 09:00 UTC）

**冻结元数据**
- 冻结时间：2026-02-03 09:00 UTC
- 冻结提交号：TBD（合并前补齐）
- 覆盖模块范围：jobcatalog / staffing / iam / person
- 变更审批人：我
- 迁移样本统计：豁免（无样本数据）

> 本记录用于支撑 `docs/dev-plans/072-repo-wide-id-code-naming-convergence.md` 的步骤 1（差异清单与映射表）。
> 范围覆盖 Job Catalog、Staffing、IAM、Person；**OrgUnit 由 026B 承接**。

## 1. 扫描摘要（可复现）

```bash
# Job Catalog
rg -n "tenant_id|event_id|request_id|initiator_id|_id\b|\bcode\b" modules/jobcatalog

# Staffing（Position/Assignment）
rg -n "tenant_id|event_id|request_id|initiator_id|position_id|assignment_id|reports_to_position_id|job_profile_id|org_unit_id|jobcatalog_setid" modules/staffing internal/server

# IAM
rg -n "tenant_id|target_tenant_id|event_id|request_id|initiator_id|org_unit_id" modules/iam internal/server internal/superadmin

# Person
rg -n "tenant_id" modules/person internal/server/person.go
```

## 2. 差异清单（当前命名偏差）

### 2.1 Job Catalog
- UUID 字段未统一 `_uuid` 后缀（如 `package_id/job_profile_id/job_family_id`）。
- code 字段未统一 `_code` 后缀（如 `code`）。
- 事件字段仍使用 `event_id/request_id/initiator_id`，未对齐 `event_uuid/request_code/initiator_uuid`。
- `tenant_id` 未对齐 `tenant_uuid`。

### 2.2 Staffing（职位/任职）
- UUID 字段未统一 `_uuid` 后缀（如 `position_id/assignment_id/reports_to_position_id/job_profile_id`）。
- 事件字段仍使用 `event_id/request_id/initiator_id`，未对齐 `event_uuid/request_code/initiator_uuid`。
- `tenant_id` 未对齐 `tenant_uuid`。
- 边界层仍暴露 `org_unit_id`（将由 026B 解析器转换为 `org_id`；对外字段在 072 执行收敛）。

### 2.3 IAM
- `tenant_id/target_tenant_id` 未对齐 `tenant_uuid/target_tenant_uuid`。
- 事件字段仍使用 `event_id/request_id/initiator_id`，未对齐 `event_uuid/request_code/initiator_uuid`。
- `org_unit_id` 出现在生成模型与部分查询（需评估是否应改名为 `org_unit_id` 保持 8 位结构标识或转 `org_unit_uuid`）。

### 2.4 Person
- `tenant_id` 未对齐 `tenant_uuid`（表/索引/RLS/函数参数均需同步改名）。

## 3. 收敛映射表

### 3.1 Job Catalog（职位分类）

| 类别 | 旧字段 | 新字段 | 备注 |
| --- | --- | --- | --- |
| common | tenant_id | tenant_uuid | 全表、函数参数、RLS |
| common | event_id | event_uuid | events 表/函数幂等键 |
| common | request_id | request_code | events 表/函数参数 |
| common | initiator_id | initiator_uuid | events 表/函数参数 |
| identity | id | <entity>_uuid | `job_family_groups/job_families/job_levels/job_profiles` 主键 |
| identity | code | <entity>_code | 对外语义不变，仅命名收敛 |
| relation | package_id | package_uuid | `package_id` 为 UUID 字段 |
| fk | job_family_group_id | job_family_group_uuid | events/versions/relations |
| fk | job_family_id | job_family_uuid | events/versions/relations |
| fk | job_level_id | job_level_uuid | events/versions/relations |
| fk | job_profile_id | job_profile_uuid | events/versions/relations |
| payload | job_family_ids | job_family_uuids | Job Profile payload 列表字段 |
| payload | primary_job_family_id | primary_job_family_uuid | Job Profile payload 主引用 |
| keep | last_event_id | last_event_id | `bigserial` 技术字段，保持 `id` 语义 |
| keep | setid | setid | 专有名词豁免 `_code` 后缀 |

> 注：所有 `bigserial` 技术主键可保留 `id`；仅业务 UUID 统一 `_uuid` 后缀。

### 3.2 Staffing（职位/任职）

| 类别 | 旧字段 | 新字段 | 备注 |
| --- | --- | --- | --- |
| common | tenant_id | tenant_uuid | 全表、函数参数、RLS |
| common | event_id | event_uuid | events/修正/撤销表 |
| common | request_id | request_code | events/修正/撤销表 |
| common | initiator_id | initiator_uuid | events/修正/撤销表 |
| identity | positions.id | position_uuid | 业务 UUID 主键 |
| identity | assignments.id | assignment_uuid | 业务 UUID 主键 |
| fk | position_id | position_uuid | events/versions/查询参数 |
| fk | assignment_id | assignment_uuid | events/versions/查询参数 |
| fk | reports_to_position_id | reports_to_position_uuid | Position 关系字段 |
| fk | job_profile_id | job_profile_uuid | Job Catalog 引用字段 |
| keep | person_uuid | person_uuid | 已符合 `_uuid` 后缀 |
| keep | org_unit_id | org_unit_id | 8 位 int 结构标识（按 026A） |
| keep | jobcatalog_setid | jobcatalog_setid | `setid` 专有名词豁免 `_code` 后缀 |
| keep | last_event_id | last_event_id | `bigserial` 技术字段 |

> 注：payload/HTTP 字段名需与上述映射同步，避免“外部字段名与内部字段名分叉”。

### 3.3 IAM

| 类别 | 旧字段 | 新字段 | 备注 |
| --- | --- | --- | --- |
| common | tenant_id | tenant_uuid | 表字段/函数参数/索引 |
| common | target_tenant_id | target_tenant_uuid | superadmin audit 等 |
| common | event_id | event_uuid | audit/events |
| common | request_id | request_code | audit/events |
| common | initiator_id | initiator_uuid | audit/events |
| keep | last_event_id | last_event_id | `bigserial` 技术字段 |
| keep | org_unit_id | org_unit_id | 若为 8 位结构标识，保持不改名 |

### 3.4 Person

| 类别 | 旧字段 | 新字段 | 备注 |
| --- | --- | --- | --- |
| common | tenant_id | tenant_uuid | 表字段/索引/RLS/函数参数 |

## 4. 后续补充
- 具体到文件级的替换清单与 PR 原子切换范围，将在实施前拆分为模块级子清单。

## 5. 文件级替换清单（初稿）

### 5.1 Job Catalog（SQL/迁移/Go）
- `modules/jobcatalog/infrastructure/persistence/schema/00002_jobcatalog_job_family_groups.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00003_jobcatalog_engine.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00004_jobcatalog_job_families.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00005_jobcatalog_job_family_engine.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00006_jobcatalog_job_levels.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00007_jobcatalog_job_level_engine.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00008_jobcatalog_job_profiles.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00009_jobcatalog_job_profile_engine.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00010_jobcatalog_read.sql`
- `modules/jobcatalog/infrastructure/persistence/schema/00011_jobcatalog_package_id_schema.sql`
- `migrations/jobcatalog/20260106102000_jobcatalog_schema.sql`
- `migrations/jobcatalog/20260106102500_jobcatalog_engine.sql`
- `migrations/jobcatalog/20260111114500_jobcatalog_m2_group_contract_align.sql`
- `migrations/jobcatalog/20260111123000_jobcatalog_job_families.sql`
- `migrations/jobcatalog/20260111123500_jobcatalog_job_family_engine.sql`
- `migrations/jobcatalog/20260111124000_jobcatalog_disable_payload_empty_check.sql`
- `migrations/jobcatalog/20260111125000_jobcatalog_job_levels.sql`
- `migrations/jobcatalog/20260111125500_jobcatalog_job_level_engine.sql`
- `migrations/jobcatalog/20260111131000_jobcatalog_job_profiles.sql`
- `migrations/jobcatalog/20260111131500_jobcatalog_job_profile_engine.sql`
- `migrations/jobcatalog/20260111133000_jobcatalog_read_snapshot.sql`
- `migrations/jobcatalog/20260125024600_jobcatalog_setid_format_5.sql`
- `migrations/jobcatalog/20260129200000_jobcatalog_package_id_schema.sql`
- `migrations/jobcatalog/20260129201000_jobcatalog_package_id_engine.sql`
- `migrations/jobcatalog/20260129234500_jobcatalog_replay_versions_uuid_overload.sql`
- `migrations/jobcatalog/20260130135000_jobcatalog_package_code_backfill.sql`
- `migrations/jobcatalog/atlas.sum`
- `internal/server/jobcatalog.go`
- `internal/server/jobcatalog_test.go`

### 5.2 Staffing（职位/任职：SQL/迁移/Go/测试）
- `modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`
- `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`
- `modules/staffing/infrastructure/persistence/schema/00015_staffing_read.sql`
- `modules/staffing/infrastructure/persistence/assignment_pg_store.go`
- `modules/staffing/presentation/controllers/assignments_api.go`
- `migrations/staffing/20260123022202_staffing-rebaseline-069.sql`
- `migrations/staffing/20260130001000_staffing_jobcatalog_package_id_validation.sql`
- `migrations/staffing/20260130002000_staffing_position_snapshot_job_profile_code.sql`
- `internal/server/staffing.go`
- `internal/server/staffing_handlers.go`
- `internal/server/staffing_test.go`
- `internal/server/staffing_handlers_m4_extra_test.go`
- `internal/server/staffing_correct_rescind_store_test.go`
- `internal/server/handler_test.go`
- `internal/server/handler_m4_extra_test.go`

### 5.3 IAM（SQL/迁移/Go/测试）
- `modules/iam/infrastructure/persistence/schema/00001_iam_baseline.sql`
- `modules/iam/infrastructure/persistence/schema/00002_iam_tenancy.sql`
- `modules/iam/infrastructure/persistence/schema/00003_iam_superadmin_audit.sql`
- `modules/iam/infrastructure/persistence/schema/00004_iam_principals_and_sessions.sql`
- `modules/iam/infrastructure/sqlc/gen/models.go`（生成物）
- `internal/server/tenancy.go`
- `internal/server/session_store.go`
- `internal/server/identity_provider.go`
- `internal/superadmin/handler.go`
- `internal/server/identity_provider_test.go`
- `internal/server/handler_test.go`
- `internal/routing/classifier_test.go`
- `internal/routing/router_test.go`

### 5.4 Person（SQL/迁移/Go/测试）
- `modules/person/infrastructure/persistence/schema/00002_person_persons.sql`
- `modules/person/infrastructure/persistence/schema/00003_person_engine.sql`
- `internal/server/person.go`
- `internal/server/person_test.go`

### 5.5 工具与 E2E 对齐（覆盖收敛后的字段名）
- `cmd/dbtool/main.go`
- `e2e/tests/m3-smoke.spec.js`
- `e2e/tests/tp060-02-master-data.spec.js`
- `e2e/tests/tp060-03-person-and-assignments.spec.js`
