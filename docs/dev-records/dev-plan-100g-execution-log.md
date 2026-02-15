# DEV-PLAN-100G 执行日志

**状态**: 实施中（2026-02-15）

**关联文档**:
- `docs/dev-plans/100g-org-metadata-wide-table-phase4c-orgunits-list-ext-query-i18n-closure.md`

## 已完成事项
- 2026-02-15：后端允许 `parent_org_code + ext query`；field-definitions 输出补齐 `allow_filter/allow_sort`（含测试）。
- 2026-02-15：OrgUnitsPage 增加 ext 筛选/排序控件（admin-only），URL 状态写回与 fail-closed 清理。
- 2026-02-15：i18n（en/zh）补齐列表 ext 控件与错误提示文案。
- 2026-02-15：新增 E2E 用例 `e2e/tests/tp060-02-orgunit-ext-query.spec.js`。
- 2026-02-15：补充 corrections kernel 权限迁移，修复后续迁移覆盖 `SECURITY DEFINER` 导致的 `ORGUNIT_CODES_WRITE_FORBIDDEN`。

## 门禁与测试记录

### Web UI 单测
- `pnpm -C apps/web-mui test` ✅

### UI 产物
- `make css` ✅（Vite chunk size warning only）

### E2E
- `make e2e` ❌
  - 失败原因：`/org/api/org-units/corrections` 返回 `ORGUNIT_CODES_WRITE_FORBIDDEN`（422），导致新用例无法写入 ext。
  - 相关日志/证据：
    - `e2e/_artifacts/server.log`
    - `e2e/_artifacts/superadmin.log`
    - `e2e/_artifacts/kratosstub.log`
    - `e2e/test-results/`（截图/trace）
  - 备注：最新修复通过迁移重设 corrections kernel 权限，待 CI 复验。
