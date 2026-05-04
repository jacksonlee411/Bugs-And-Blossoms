# DEV-PLAN-492 Readiness

## 说明

- 本文件记录 `DEV-PLAN-492` 的阶段性实施进展与验证结果，避免计划文档与实际代码状态漂移。
- 当前记录已完成且已验证的后端先行切片与 491 Phase A/B/C/D 前端消费进展；组织管理页浏览/编辑主树读取规则进一步下沉、SQL 级 scoped pagination 优化与更广联合 E2E 仍保持待办。

## 2026-05-04 实施记录

### 已完成

- 已新增 `modules/orgunit/services` 的 `OrgUnitReadService` 骨架、request/response 类型、scope filter 和 store port。
- 已补 `modules/orgunit/services` fake-store 单测，覆盖：
  - visible roots
  - 中段 scope 的 visible roots 去重
  - scope-aware children
  - search safe path
  - resolve 范围外 fail-closed
- 已在 `internal/server` 增加 ReadService adapter，并将默认 `GET /org/api/org-units` roots 语义切到 scope-aware visible roots。
- 已把 list DTO 的 `org_node_key` 从隐藏字段改为响应字段，并新增 `has_visible_children`。
- 已补 handler contract 测试，覆盖：
  - roots 响应暴露 `org_node_key`
  - roots 响应暴露 `has_visible_children`
  - 授权范围位于树中段时返回 visible root，而不是物理 root
- PR-3 已将默认 children HTTP 与 search HTTP 迁移到 `OrgUnitReadService`：
  - children 响应通过 ReadService 统一返回 `org_node_key` 与 `has_visible_children`
  - children 请求对范围外 parent fail-closed，不回退物理父节点展开
  - search 通过 ReadService 处理 scope-aware candidates 与 safe `path_org_codes`
  - search 命中深层节点时 `path_org_codes` 从当前 visible root 开始，不泄露范围外祖先
  - `all_org_units=true` + pagination 的 HTTP 响应 total 已通过全量候选过滤后再分页，保持对外 scope 裁剪语义一致
- 491 Phase A/B 已新增前端 selector facade 与 `OrgUnitTreeSelector` / `OrgUnitTreePickerDialog` / `OrgUnitTreeField`，并消费 492 roots/children/search contract。
- 491 Phase C 已将 `AuthzRolePages.tsx` 用户授权页组织范围行切到 `OrgUnitTreeField`：
  - 页面内 `listOrgUnits()` 一级候选与 `OrgUnitAPIItem` 依赖已移除
  - `addScopeRow` 新增空行，不再自动取第一个根组织
  - selector 选中后写入 `orgCode/orgNodeKey/orgName`
  - `buildScopePayload()` 仍沿用 489 payload
  - assignment 响应缺 `org_name` 时使用 `org_code` 作为回显 fallback，避免既有授权行显示为空
- 491 Phase D 已将其他主要组织选择入口切到同一 selector：
  - `OrgUnitsPage` 创建组织弹窗的上级组织字段切到 `OrgUnitTreeField`
  - `OrgUnitDetailsPage` 编辑上级组织字段切到 `OrgUnitTreeField`
  - `OrgUnitTreeField` 增加可选清空与 helper text 支持，写入 payload 仍沿用既有 `parent_org_code`
  - details API 响应补 `parent_org_node_key`，支撑 selector 稳定回显当前父组织
  - `orgUnits.ts` 补齐 `org_node_key`、`has_visible_children` 与详情父节点回显字段
- 已补 safe path 深层/跨分支验证：
  - ReadService search 覆盖 visible root 下多层深路径，`path_org_codes` 从 visible root 开始
  - ReadService search 覆盖当前 principal 只可见其他分支时不返回目标分支路径
  - HTTP `/org/api/org-units/search` 覆盖深层 safe path 与其他分支不可见时 404/fail-closed

### 已验证

- `go fmt ./modules/orgunit/services ./internal/server`
- `go test ./modules/orgunit/services ./internal/server`
- `go vet ./...`
- `make check lint`
- `make test`
- `pnpm --dir apps/web test AuthzRolePages`
- `pnpm --dir apps/web typecheck`
- `pnpm --dir apps/web lint`
- `pnpm --dir apps/web build`
- `make css`
- `npm --prefix apps/web test -- --run src/pages/org/OrgUnitDetailsPage.test.tsx src/components/OrgUnitTreeSelector.test.tsx src/pages/org/OrgUnitsPage.test.tsx`
- `npm --prefix apps/web run typecheck`
- `npm --prefix apps/web run lint`
- `go test ./modules/orgunit/services ./internal/server -run 'TestOrgUnitReadServiceSearch|TestHandleOrgUnitsSearchAPI|TestHandleOrgUnitsDetailsAPI'`
- `git diff --check`

### 当前结论

- `492 P1/P2/P3` 的首轮后端 contract 已落地：roots、children、search 均已通过 ReadService 对外提供 selector-ready DTO 与 scope-aware safe path。
- `491 Phase A/B/C/D` 已消费 492 contract 并完成用户授权页、创建组织上级组织、组织详情编辑上级组织这些主要选择入口接入；不再存在“selector/facade 已有但页面主要选择入口仍用一级下拉/手填”的窗口。
- 目前尚未完成的工作仍包括：
  - list/grid 的 SQL 级 scoped pagination 优化；当前 HTTP 层已保证对外 total/page 以 scope 裁剪后的结果为准
  - 组织管理页浏览/编辑主树局部读取规则继续向 492 ReadService 下沉
  - 更广 491/492 联合 E2E；当前已有用户授权页受限管理员 selector E2E 与本轮组件/API contract 测试
