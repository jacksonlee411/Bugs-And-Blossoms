# DEV-PLAN-100E 执行日志

> 目的：记录 DEV-PLAN-100E（Phase 4A：OrgUnit 详情页扩展字段展示 + capabilities-driven 更正编辑）的实施与验证结果。

## 变更摘要

- Web API client（`apps/web-mui/src/api/orgUnits.ts`）补齐：
  - `getOrgUnitMutationCapabilities(...)`
  - `getOrgUnitFieldOptions(...)`
  - `correctOrgUnit` 支持 `patch.ext`
- OrgUnit 详情页（`apps/web-mui/src/pages/org/OrgUnitDetailsPage.tsx`）：
  - 展示 ext_fields（含 `display_value_source` 的可解释 warning）
  - 更正弹窗消费 `mutation-capabilities`（`allowed_fields/enabled/deny_reasons`），capabilities 异常时 fail-closed
  - DICT/ENTITY 扩展字段编辑接入 options endpoint（含 debounce 与缓存 key）
  - 支持“更正后生效日”（effective_date correction mode），成功后自动切换 URL `effective_date`
- i18n（`apps/web-mui/src/i18n/messages.ts`）补齐扩展字段 label key 与更正弹窗提示文案（en/zh）。
- 新增前端单测：
  - patch 构造：最小变更 + allowed_fields 裁剪 + 生效日更正模式（`apps/web-mui/src/pages/org/*test.ts`）

## 本地验证

- 已通过（2026-02-15）：
  - `pnpm -C apps/web-mui typecheck`
  - `pnpm -C apps/web-mui lint`
  - `pnpm -C apps/web-mui test`
  - `pnpm -C apps/web-mui build`
  - `make css`（产物同步到 `internal/server/assets/web`）
  - `make check doc`
  - `make e2e`

## 约束符合性

- 无 DB schema / migration / sqlc 变更（符合 100E stopline）。
- capabilities 不可用时 UI fail-closed（禁用/不提交），不引入第二套前端白名单或“乐观放行”分支。

