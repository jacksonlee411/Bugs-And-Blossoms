# DEV-PLAN-075B 执行日志

> 目的：记录 DEV-PLAN-075B 的落地变更与可复现验证入口。

## 变更摘要
- 新增迁移修复 `submit_org_event_correction` 的区间回溯规则（对齐受控回溯口径）。
- 修复 OrgUnit PG store 的 `set_config` 引号问题，避免 correction 分支潜在 SQL 错误。
- 新增针对 `set_config` 关键点的单测断言。

## 本地验证
- 2026-02-06 12:21 UTC：`make orgunit migrate up`
  - 结果：goose 成功应用 `20260206115000_orgunit_correction_backdating_range_fix.sql`，并完成 `orgunit-smoke`。
- 2026-02-06 12:21 UTC：`go test ./internal/server -run "TestOrgUnitPGStore_UsesQuotedCurrentTenantKey|TestHandleOrgNodes_RecordActions/insert_record_backdate_earliest_uses_correction" -count=1`
  - 结果：OK
- 2026-02-06 12:21 UTC：`make check doc`
  - 结果：`[doc] OK`
