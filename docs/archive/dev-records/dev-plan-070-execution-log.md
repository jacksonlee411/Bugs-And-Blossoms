# DEV-PLAN-070 执行日志

> 目的：记录 DEV-PLAN-070 的执行进展与静态证据（本次为代码审阅评估）。

## 完成情况评估

- 里程碑 9 已覆盖：UI/路由/鉴权完成 SetID 绑定、业务单元标记、配置主数据显式 setid、业务数据解析结果展示。
- 解析与约束已落地：ResolveSetID、绑定约束、SHARE/DEFLT 规则、共享层 RLS 合同已实现。

## 静态证据

- UI 入口：`internal/server/setid.go`、`internal/server/orgunit_nodes.go`、`internal/server/jobcatalog.go`、`internal/server/staffing_handlers.go`。
- 路由与鉴权：`config/routing/allowlist.yaml`、`internal/server/authz_middleware.go`、`config/access/policy.csv`。
- 解析与约束：`modules/orgunit/infrastructure/persistence/schema/00005_orgunit_setid_schema.sql`、`modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`、`modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`。
- 残留排查：`rg -n "business_unit_id|record_group"` 仅命中文档说明。

## 待验证/遗留

- [x] 哨兵租户记录是否存在且无冲突（数据层验证）。
- [x] 共享层白名单 UI 标注“共享/只读”与不可编辑展示（SetID 列表新增 scope 标注，shared 不进入绑定下拉）。
- [x] DB 权限是否已限制为仅 kernel 写入口（role/grant 级验证；app_nobypassrls 仅保留 SELECT）。

## 本地验证

- iam.tenants 哨兵：`SELECT id::text, name, is_active FROM iam.tenants WHERE id='00000000-0000-0000-0000-000000000000';` 结果：`GLOBAL` 存在且 active。
- GLOBAL 同名冲突：`SELECT id::text FROM iam.tenants WHERE name='GLOBAL' AND id <> '00000000-0000-0000-0000-000000000000';` 结果：0 行。
- global_setids tenant_id 校验：`SELECT count(*) FROM orgunit.global_setids WHERE tenant_id <> orgunit.global_tenant_id();` 结果：0。
- 权限验证：`SELECT proname, proowner::regrole, prosecdef FROM pg_proc WHERE proname IN ('submit_setid_event','submit_setid_binding_event','submit_global_setid_event');` 结果：owner=`orgunit_kernel` 且 `prosecdef=true`。
- 权限验证：`SELECT grantee, privilege_type, table_name FROM information_schema.role_table_grants WHERE table_schema='orgunit' AND table_name IN ('global_setids','global_setid_events','setids','setid_events','setid_binding_events','setid_binding_versions') AND grantee IN ('app_runtime','app_nobypassrls','superadmin_runtime');` 结果：`app_nobypassrls` 仅保留 `SELECT`。
- 共享只读 UI：`go test ./internal/server -run TestRenderSetIDPage` 通过；SetID 列表新增 `scope` 列且 shared 显示 `Shared/Read-only (共享/只读)`，绑定下拉不再包含 shared。
