# DEV-PLAN-492 Readiness

## 说明

- 本文件记录 `DEV-PLAN-492` 的阶段性实施进展与验证结果，避免计划文档与实际代码状态漂移。
- 当前记录已完成且已验证的后端先行切片与 491 Phase A/B/C/D 前端消费进展；普通 list/grid 与 ext 字段 list/grid 查询均已接入 ReadService `List`，并通过 adapter page 原语下推 scope 裁剪、filter/sort、total、limit/offset；ext list/grid 已补 parent scope fail-closed 与 adapter scope path 补齐回归测试；更广联合 E2E 已覆盖主要 selector 入口。

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
- 普通 list/grid 读取已继续下沉：
  - `OrgUnitReadService` 新增 `List`，普通 list/grid 优先走 adapter page 原语；scope filter、keyword/status/business-unit 过滤、排序、total 与 limit/offset 由 store pager 同一口径处理
  - `handleOrgUnitsAPI` 的非 ext list/grid 分支已改为消费 ReadService `List`
  - `internal/server` adapter 已提供 page 原语，普通 list/grid 不再通过递归 children 收集可见全集
- ext 字段 list/grid 查询已继续下沉：
  - `OrgUnitListRequest` 新增 `ExtFilterFieldKey` / `ExtFilterValue` / `ExtSortFieldKey`，ext filter/sort 与普通 list/grid 统一进入 ReadService `List`
  - `internal/server` adapter 新增 ReadService list page port，字段元数据查询与物理列校验仍由 store adapter 背后的基础设施查询承接
  - `handleOrgUnitsAPI` 不再为 ext 分支直接调用旧 store path，也不再在 handler 内二次执行 scope 裁剪与分页
  - ext list/grid 带 `parent_org_code` / `parent_org_node_key` 时，ReadService 会先校验 parent 在当前 principal scope 内；范围外 parent fail-closed，不进入 page store
  - adapter 在 page rows 转换为 ReadService node 前补齐 `PathOrgNodeKeys`，确保 include-descendants scope 不因底层 pager 缺 path 被误裁
  - scoped principal 下 list/grid 不再由 ReadService 拉取候选全集后分页；PG pager 已将 scope where、filter/sort、count、limit/offset 合并到 SQL 主链，`has_visible_children` 也按当前 scope 与 include-disabled 口径计算
- details 读取 scope check 已继续下沉：
  - `handleOrgUnitsDetailsAPI` 使用 `OrgUnitReadService.Resolve` 校验当前 principal 对目标组织的可见性
  - 范围外 details 请求 fail-closed 为 `403 authz_scope_forbidden`，空 resolve 结果不会进入 details 读取或触发 panic
- write scope check 已继续向 ReadService 统一：
  - `ensureCurrentPrincipalOrgCodeScopeAllows` 通过 `OrgUnitReadService.Resolve` 判断目标组织或父组织是否在当前 principal 可见范围内
  - write helper 显式使用当前写请求的 `effective_date` 做 scope check，避免用当前日期推断导致历史/测试日期误判

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
- `go test ./modules/orgunit/services ./internal/server -run 'TestOrgUnitReadServiceList|TestOrgUnitReadServiceSearch|TestOrgUnitReadServiceChildren|TestHandleOrgUnitsAPI_ListPaginationTotalUsesScopedResult|TestHandleOrgUnitsAPI_List|TestListOrgUnitListPage'`
- `go test ./modules/orgunit/services ./internal/server`
- `go vet ./...`
- `make check lint`
- `make check doc`
- `make test`
- `git diff --check`
- `go test ./modules/orgunit/services`
- `go test ./internal/server -run 'TestHandleOrgUnitsAPI_(Ext|BusinessUnit|AllOrgUnits)|TestOrgUnitReadService|TestListOrgUnitListPage|TestSortOrgUnitListItems|TestFilterOrgUnitListItems'`
- `go test ./modules/orgunit/services -run 'TestOrgUnitReadServiceListExtQuery'`
- `go test ./internal/server -run 'TestOrgUnitReadStoreAdapterListPageHydratesScopePath|TestHandleOrgUnitsAPI_Ext|TestHandleOrgUnitsAPI_ListPaginationTotalUsesScopedResult'`
- `go test ./modules/orgunit/services -run 'TestOrgUnitReadService(VisibleRootsEmptyScopeFailsClosed|VisibleRootsDisabledScopeHonorsIncludeDisabled|ListHasVisibleChildrenUsesScopedCandidates|List|VisibleRoots|Children|Search|Resolve)'`
- `go test ./modules/orgunit/services ./internal/server -run 'TestOrgUnitReadService|TestHandleOrgUnitsAPI|TestHandleOrgUnitsWrite|TestHandleOrgUnitsCorrections|TestHandleOrgUnitsDetailsAPI|TestHandlePrincipalAuthzAssignmentPutAPI'`
- `npm --prefix apps/web test -- --run src/components/OrgUnitTreeSelector.test.tsx src/pages/org/OrgUnitsPage.test.tsx src/pages/org/OrgUnitDetailsPage.test.tsx`
- `node --check e2e/tests/dev491-authz-org-selector-scope.spec.js`
- `pnpm --dir e2e exec playwright test tests/dev491-authz-org-selector-scope.spec.js`
- `go test ./modules/orgunit/services -run 'TestOrgUnitReadServiceList'`
- `go test ./internal/server -run 'TestOrgUnitPGStore_ListOrgUnitsPage|TestOrgUnitReadStoreAdapterListPageHydratesScopePath|TestOrgUnitReadStoreAdapterListPagePushesScopeAndPagination|TestHandleOrgUnitsAPI_Ext|TestHandleOrgUnitsAPI_ListPaginationTotalUsesScopedResult|TestListOrgUnitListPage'`
- `go test ./modules/orgunit/services ./internal/server -run 'TestOrgUnitReadServiceResolve|TestHandleOrgUnitsDetailsAPI'`
- `go test ./internal/server -run 'TestHandleOrgUnitsWriteAPI_CreateOrgSkipsNewOrgScopeCheckButChecksParent|TestHandleOrgUnitsAPI|TestHandleOrgUnitsBusinessUnitAPI|TestHandleOrgUnitsDetailsAPI'`

## 2026-05-05 收口记录

- `DEV-PLAN-492` 首期已关闭：ReadService read core、handler 瘦身、store adapter bridge 收口、491 selector 消费与统一 scope fail-closed 均已完成。
- `DEV-PLAN-491` 首期已关闭：多选 selector 语义暂缓，不计入首期剩余实施事项；当前用户授权组织范围通过多行单选表达多个组织范围。
- 本次仅做文档状态收口，不新增代码验证；现有验证记录继续作为关闭证据保留。

### 当前结论

- `492 P1/P2/P3` 的首轮后端 contract 已落地：roots、children、search 均已通过 ReadService 对外提供 selector-ready DTO 与 scope-aware safe path。
- 普通 list/grid 与 ext 字段 list/grid 的读取规则已向 ReadService 下沉；adapter 已避免递归 children N+1；PG pager 已下推 scoped pagination，确保 scope 裁剪、filter/sort、count 与 limit/offset 在 SQL 主链内完成；ext parent scope、adapter path 补齐、空 scope、disabled scope、list `has_visible_children` scoped candidates 语义均已补回归测试。
- `491 Phase A/B/C/D` 已消费 492 contract 并完成用户授权页、创建组织上级组织、组织详情编辑上级组织这些主要选择入口接入；不再存在“selector/facade 已有但页面主要选择入口仍用一级下拉/手填”的窗口。更广 `dev491` E2E 已覆盖受限管理员在三类入口只能选择当前可见组织，范围外搜索与直接提交均 fail-closed。
- 统一 write API 已修正 create scope check 顺序：新建组织时不要求新 org code 已存在于当前 scope，仍对非空 parent 做当前 principal scope 校验；详情/更正等非 create intent 继续校验目标 org 与 parent。当前目标/父组织校验已复用 `OrgUnitReadService.Resolve` 的 scope 判断。
- `492 P4/P5` handler 瘦身已继续收口：`orgunit_api.go` 不再承载 list/filter/sort/path hydration adapter bridge，相关 helper 已移动到 `orgunit_read_service_adapter.go` 并标注为 adapter-only；`GET /org/api/org-units` parent 解析、details/versions/audit 目标定位均统一通过 ReadService；`set-business-unit` 旧 store fallback 已删除，只保留 `OrgUnitWriteService` 主链路。
- 本轮验证通过：
  - `go test ./modules/orgunit/services ./internal/server`
  - `make check ddd-layering-p0 && make check ddd-layering-p2 && make check no-legacy`
