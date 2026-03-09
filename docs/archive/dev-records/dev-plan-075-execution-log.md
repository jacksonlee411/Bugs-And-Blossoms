# DEV-PLAN-075 执行日志

> 目的：记录 DEV-PLAN-075 的落地变更与可复现验证入口。

## 变更摘要

- 更正规则补齐上级组织有效性校验：更正生效日时要求父级在该日有效，且不早于父级最早生效日。
- `/org/nodes` 记录动作补齐体验：insert/add 默认日期与范围提示，最早记录允许回溯时使用更正生效日路径。
- 新增 OrgUnit 生效日更正写入口（store）与覆盖性单测，补齐回溯/异常分支。

## 本地验证

- 已通过：`go fmt ./...`
- 已通过：`go vet ./...`
- 已通过：`make check lint`
- 已通过：`make test`（coverage 100%）
- 已通过：`make check doc`
- 已通过：`make sqlc-generate`
- 已通过：`make orgunit plan`
- 已通过：`make orgunit lint`
- 已通过：`make orgunit migrate up`

## 集成验证（改生效日后重放一致）

- 数据库：`bb_075_integration_test`
- Schema：`00001_orgunit_schema.sql` / `00002_orgunit_org_schema.sql` / `00003_orgunit_engine.sql` / `00004_orgunit_read.sql`
- 执行命令：

```
psql 'postgres://app:app@127.0.0.1:5438/bb_075_integration_test?sslmode=disable' -v ON_ERROR_STOP=1 -P pager=off <<'SQL'
SELECT set_config('app.current_tenant', '00000000-0000-0000-0000-000000000075', false);

SELECT orgunit.submit_org_event(
  '00000000-0000-0000-0000-000000000701',
  '00000000-0000-0000-0000-000000000075',
  10000001,
  'CREATE',
  '2026-01-01',
  jsonb_build_object('name','Root','org_code','R075','is_business_unit',true),
  'r1',
  '00000000-0000-0000-0000-000000000701'
) AS root_create;

SELECT orgunit.submit_org_event(
  '00000000-0000-0000-0000-000000000702',
  '00000000-0000-0000-0000-000000000075',
  10000002,
  'CREATE',
  '2026-01-10',
  jsonb_build_object('parent_id',10000001,'name','Child','org_code','C075'),
  'r2',
  '00000000-0000-0000-0000-000000000701'
) AS child_create;

SELECT orgunit.submit_org_event(
  '00000000-0000-0000-0000-000000000703',
  '00000000-0000-0000-0000-000000000075',
  10000002,
  'RENAME',
  '2026-02-01',
  jsonb_build_object('new_name','ChildV2'),
  'r3',
  '00000000-0000-0000-0000-000000000701'
) AS child_rename;

SELECT orgunit.submit_org_event(
  '00000000-0000-0000-0000-000000000704',
  '00000000-0000-0000-0000-000000000075',
  10000002,
  'DISABLE',
  '2026-03-01',
  '{}'::jsonb,
  'r4',
  '00000000-0000-0000-0000-000000000701'
) AS child_disable;

SELECT orgunit.submit_org_event_correction(
  '00000000-0000-0000-0000-000000000075',
  10000002,
  '2026-02-01',
  jsonb_build_object('effective_date','2026-01-15'),
  'c1',
  '00000000-0000-0000-0000-000000000701'
) AS correction_uuid;

SELECT name AS name_asof_2026_01_12
FROM orgunit.get_org_snapshot('00000000-0000-0000-0000-000000000075', '2026-01-12')
WHERE org_id = 10000002;

SELECT name AS name_asof_2026_01_20
FROM orgunit.get_org_snapshot('00000000-0000-0000-0000-000000000075', '2026-01-20')
WHERE org_id = 10000002;

SELECT name AS name_asof_2026_02_15
FROM orgunit.get_org_snapshot('00000000-0000-0000-0000-000000000075', '2026-02-15')
WHERE org_id = 10000002;

SELECT count(*) AS count_asof_2026_03_02
FROM orgunit.get_org_snapshot('00000000-0000-0000-0000-000000000075', '2026-03-02')
WHERE org_id = 10000002;

CREATE TEMP TABLE pre_versions AS
SELECT tenant_uuid, org_id, parent_id, node_path, validity, name, full_name_path, status, is_business_unit, manager_uuid, last_event_id
FROM orgunit.org_unit_versions
WHERE tenant_uuid = '00000000-0000-0000-0000-000000000075';

SELECT orgunit.replay_org_unit_versions('00000000-0000-0000-0000-000000000075'::uuid);

SELECT count(*) AS diff_pre_post
FROM (
  SELECT * FROM pre_versions
  EXCEPT
  SELECT tenant_uuid, org_id, parent_id, node_path, validity, name, full_name_path, status, is_business_unit, manager_uuid, last_event_id
  FROM orgunit.org_unit_versions
  WHERE tenant_uuid = '00000000-0000-0000-0000-000000000075'
) diff;

SELECT count(*) AS diff_post_pre
FROM (
  SELECT tenant_uuid, org_id, parent_id, node_path, validity, name, full_name_path, status, is_business_unit, manager_uuid, last_event_id
  FROM orgunit.org_unit_versions
  WHERE tenant_uuid = '00000000-0000-0000-0000-000000000075'
  EXCEPT
  SELECT * FROM pre_versions
) diff;
SQL
```

- 关键输出：
  - `name_asof_2026_01_12 = Child`
  - `name_asof_2026_01_20 = ChildV2`
  - `name_asof_2026_02_15 = ChildV2`
  - `count_asof_2026_03_02 = 0`
  - `diff_pre_post = 0` / `diff_post_pre = 0`

## CI 证据

- PR #294
