# DEV-PLAN-101 执行日志

> 目的：记录 DEV-PLAN-101（OrgUnit 字段配置管理页）实施与验证结果，作为 readiness 证据。

## 变更摘要

- 新增字段配置管理页（MUI）：
  - `apps/web-mui/src/pages/org/OrgUnitFieldConfigsPage.tsx`
  - 可查看/筛选（含 `status=disabled` 下的二级区分：未生效 vs 已停用）
  - 可启用字段（支持 DICT/ENTITY 的 data_source_config 选项选择）
  - 可停用字段（设置 disabled_on）
  - 可延期停用（disabled_on 未生效前，仅允许向后延迟）
- 路由与权限：
  - UI route：`/app/org/units/field-configs`（basename=/app）
  - 仅 `orgunit.admin` 可访问（`RequirePermission`）
- 可发现性入口：
  - 左侧导航新增“字段配置”（admin-only）
  - OrgUnits 列表页、OrgUnit 详情页 PageHeader actions 增加入口按钮（admin-only）
- API client/i18n：
  - `apps/web-mui/src/api/orgUnits.ts` 补齐 field-definitions / field-configs list+enable+disable
  - `apps/web-mui/src/i18n/messages.ts` 补齐页面文案（en/zh）
- UI 构建产物同步：
  - `make css` 更新 `internal/server/assets/web/**`（go:embed）

## 关键 UX 冻结点落地

- “禁用”展示拆分（用户可理解）：
  - `未生效 (pending)`：`as_of < enabled_on`
  - `已停用 (disabled)`：`disabled_on <= as_of`
  - `status=disabled` 支持二级筛选（纯 UI；不改后端 API）
- “延期停用”支持：
  - 仅当 `disabled_on` 已设置且 `today_utc < disabled_on` 时可操作
  - 仅允许把 `disabled_on` 往后改（与 DB 规则对齐：未生效 + 向后延迟）
- request_code 用户无感知：
  - 对话框打开生成 request_code
  - 任一关键输入变更即生成新的 request_code
  - 输入未变化时重试复用 request_code（避免重复占槽位/重复事件）

## 本地验证

- 已通过（2026-02-15）：
  - `pnpm -C apps/web-mui lint`
  - `pnpm -C apps/web-mui typecheck`
  - `pnpm -C apps/web-mui test`
  - `make css`（产物同步到 `internal/server/assets/web`）
  - `make check doc`
  - `make check lint && make test`

## 约束符合性

- 未引入 legacy/双链路；字段配置写入口仍走既有后端 One Door（本计划仅实现 UI + client）。

