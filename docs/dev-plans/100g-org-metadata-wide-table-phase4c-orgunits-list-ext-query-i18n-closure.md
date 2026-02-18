# DEV-PLAN-100G：Org 模块宽表元数据落地 Phase 4C：OrgUnit 列表扩展字段筛选/排序 + i18n 收口（闭环收口，MUI）

**状态**: 已完成（2026-02-16 06:17 UTC — E2E 闭环通过并完成 083B 同步收口）

**执行记录**:
- `docs/dev-records/dev-plan-100g-execution-log.md`

> 本文从 `DEV-PLAN-100` Phase 4 的 **4C** 拆分而来，作为 4C 的 SSOT；`DEV-PLAN-100` 继续保持为整体路线图。  
> 本文聚焦：**OrgUnit 列表页**接入扩展字段筛选/排序入口，并补齐 **en/zh i18n**（字段标签 + 列表相关错误提示），形成“配置字段 -> 写入 -> 列表筛选/排序 -> 详情回显”的用户可见闭环。

## 1. 背景与上下文（结合当前实现状态）

截至 2026-02-15，`DEV-PLAN-100` 的关键前置已落地：

- Phase 3（后端可读写 + allowlist + fail-closed）：`DEV-PLAN-100D`、`DEV-PLAN-100D2` 已完成。
- Phase 4A（详情页 ext_fields 展示 + capabilities-driven 更正编辑）：`DEV-PLAN-100E`、`DEV-PLAN-100E1` 已完成。
- Phase 4B（字段配置管理页可发现/可操作）：`DEV-PLAN-101` 已完成。
- 时间上下文（`as_of/tree_as_of/effective_date`）收敛：`DEV-PLAN-102` 已完成（避免列表把视图日期伪装为版本生效日）。
- 前端收敛为 MUI-only：`DEV-PLAN-103` 已完成（唯一 UI 入口 `/app/**`）。

**历史缺口（对应 `DEV-PLAN-100` 4C，现已收口）**：

1) 列表页（`apps/web` 的 OrgUnitsPage）尚未提供扩展字段的筛选/排序 UI 入口；  
2) 列表 ext query 的 URL 参数与前端 grid query state 解析存在“仅支持 core sort 字段”的约束，导致无法把 `sort=ext:<field_key>` 作为可分享状态稳定复现；  
3) i18n 已覆盖详情页所需的内置字段 label key（如 `org.fields.short_name`），但列表侧仍缺少“扩展筛选/排序”相关控件文案与错误提示的 i18n 收口。

此外，后端列表接口已具备 ext filter/sort 能力（SSOT：`DEV-PLAN-100D` §5.6）；本计划已完成与“树-列表联动”冲突点的收敛（`parent_org_code` 场景允许 ext query）。

## 2. 目标与非目标

### 2.1 核心目标（DoD）

1. [x] OrgUnit 列表页开放 **1~2 个扩展字段**的筛选入口（MVP 至少覆盖 `d_org_type`）。  
2. [x] OrgUnit 列表页开放 **1~2 个扩展字段**的排序入口（MVP 至少覆盖 `d_org_type`）。  
3. [x] 扩展筛选/排序入口必须由服务端元数据驱动（field_key/label_i18n_key/value_type/data_source_type + allow_filter/allow_sort），UI 不维护第二套白名单（对齐 `DEV-PLAN-100` D7/D8）。  
4. [x] i18n（en/zh）补齐：
   - [x] 列表扩展筛选/排序相关控件文案；
   - [x] 列表 ext query 失败路径的可解释错误提示（避免仅展示 raw message）。
5. [x] fail-closed：当字段在 `as_of` 下未启用/已停用/不允许筛选排序时，列表页必须给出明确禁用原因或错误提示，不允许静默吞掉或“看起来成功但无效果”。  
6. [x] 端到端证明：至少 1 条路径可在页面完成（管理员视角）：
   - 字段配置管理页启用字段（`DEV-PLAN-101`） -> 详情页写入扩展字段值（`DEV-PLAN-100E`） -> 列表页按该扩展字段筛选/排序 -> 打开详情页回显值。
7. [x] 权限边界冻结（admin-only MVP）：
   - [x] ext filter/sort 控件仅 `orgunit.admin` 可见；
   - [x] 非 admin 若访问带 ext query 的分享 URL：页面必须**清理 ext 参数**并给出可解释提示（i18n），避免出现“看不见筛选条件但结果被过滤”的隐形行为。

### 2.2 非目标（Stopline）

- 不新增/变更 DB schema、迁移、sqlc（如需必须另立 dev-plan 且先获用户确认）。
- 不引入“通用动态查询 DSL”（多条件组合、括号、跨字段 OR 等），MVP 仅做单字段 filter + 单字段 sort。
- 不在本计划引入“业务枚举值多语言”（DICT options 的 `label` 仍为 canonical label；SSOT：`DEV-PLAN-100D` ADR-100D-06、`DEV-PLAN-020`）。
- 不把扩展字段值强行塞进列表返回（是否需要列表展示 ext 值/列属于后续体验增强，可另立计划）。

## 2.3 工具链与门禁（SSOT 引用）

> 本文不复制命令矩阵；触发器与门禁入口以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。

- 触发器（本计划通常会命中）：
  - [x] 文档：`make check doc`（本文件 + 引用更新）
  - [x] Web UI（`apps/web/**`）：lint/typecheck/test/build + `make css`（产物入仓 + go:embed）
  - [x] 路由治理：若新增/调整后端路由，需 `make check routing`（SSOT：`DEV-PLAN-017`）
  - [x] Authz：若新增/调整权限映射，需 `make authz-pack && make authz-test && make authz-lint`（SSOT：`DEV-PLAN-022`）

## 3. 关键设计决策（冻结）

### KDD-100G-01：列表 ext query 与“树-列表联动”的兼容策略

冻结选择（MVP）：

- 列表页的 ext filter/sort 需要在“选中树节点（parent_org_code）”的常态路径下可用，因此**后端列表接口必须允许** `mode=grid` 下同时携带：
  - `parent_org_code=<...>`（子树范围）
  - `ext_filter_field_key/ext_filter_value`（可选）
  - `sort=ext:<field_key>&order=asc|desc`（可选）

约束：

- ext query 仍仅允许在 `mode=grid` 或显式分页参数存在时使用（保持 `DEV-PLAN-100D` 的“仅 grid/pagination”约束，避免落入 roots/children 兼容路径）。
- UI 不得把 ext query 参数带到 TreePanel 的 roots/children 懒加载请求里（避免无意义的动态 SQL）。

### KDD-100G-02：扩展筛选/排序字段清单的事实源（冻结为 admin-only MVP）

冻结选择（MVP）：

- 列表页的 ext filter/sort 控件 **仅对 `orgunit.admin` 可见**（与“字段配置管理/写入扩展字段值”的角色一致；同时避免为 read 用户新增额外元数据读取接口）。
- UI 通过服务端返回的字段定义 + `as_of` 下 enabled 配置行得到“可筛选/可排序”的 ext 字段集合，禁止在前端维护第二套静态 key 列表。

为支持 UI 作出正确选择，本计划冻结：扩展 `GET /org/api/org-units/field-definitions` 响应，补齐以下字段（admin-only）：

- `allow_filter`（bool）
- `allow_sort`（bool）

UI 侧组合口径（冻结）：

- enabled 集合来自：`GET /org/api/org-units/field-configs?as_of=...&status=enabled`
- queryable 集合来源（对齐 `DEV-PLAN-106A`）：
  - built-in 字段：来自 `field-definitions`（`allow_filter/allow_sort`）
  - 字典字段（`d_...`）与自定义字段（`x_...`）：来自 `field-configs` 行级元数据（`allow_filter/allow_sort`）
- 列表页实际可选字段 = enabled ∩ queryable（并按 `field_key` 升序稳定排序）

> 后续若需要把列表 ext filter/sort 开放给 read 用户（不要求 admin），另起 dev-plan 新增 read-only endpoint（例如 `fields:queryable`），避免在本计划扩大范围。

### KDD-100G-03：列表 UI 的交互形态（最小可用）

冻结选择：

- 扩展筛选放在 OrgUnitsPage 的 FilterBar（与 keyword/status/as_of 同级），以 URL query params 作为可分享状态：
  - `ext_filter_field_key`
  - `ext_filter_value`
- 扩展排序入口采用 “Sort By” 下拉框（或同级控件），写回既有 grid sort query params：
  - `sort=ext:<field_key>`
  - `order=asc|desc`
- 不要求列表直接展示 ext 值列；用户可通过“打开详情页”验证筛选/排序结果（闭环出口以 E2E 证明为准）。

### KDD-100G-04：非 admin 访问带 ext 参数的 URL 的行为（冻结）

冻结选择（fail-closed，避免“隐形筛选/排序”）：

- 若当前用户不具备 `orgunit.admin`：
  - 列表页不渲染 ext filter/sort 控件；
  - 若 URL 存在任意 ext 参数（`ext_filter_field_key`、`ext_filter_value`、`sort=ext:*`）：
    - 页面加载时必须以 replace 导航清理这些参数；
    - 并展示一次性提示（toast 或顶部 Alert）：例如 “扩展字段筛选/排序需要管理员权限，已清理该筛选条件”（i18n）。
  - 列表请求不得携带 ext query 参数（即使 URL 未及时清理也不得“偷偷带上”）。

### KDD-100G-05：ext sort 的 URL 状态与 DataGrid 的展示策略（冻结）

冻结选择（实现最小改动，且 URL 可分享/可复现）：

- 列表页以 URL query params 作为“排序/筛选”的唯一状态承载：
  - core sort：`sort=code|name|status&order=asc|desc`
  - ext sort：`sort=ext:<field_key>&order=asc|desc`
  - ext filter：`ext_filter_field_key=<field_key>&ext_filter_value=<value>`
- DataGrid 的 `sortModel` 仅用于 core sort（因为 ext 字段不是 DataGrid column，不应强行映射到列排序 UI）：
  - 当 URL 为 ext sort：`sortModel=[]`（不在列头展示箭头）
  - 当用户点击 core 列头排序：写回 `sort=<core>&order=<...>` 并清理 ext sort（以 core sort 覆盖 ext sort）
- 解析/写回原则：
  - 解析时不得依赖 `parseGridQueryState` 对 `sort=ext:*` 的支持（它会丢弃未知 sort）；应以 `searchParams.get('sort')` 原值判断是否为 ext sort，并独立解析 `order`。
  - 写回时允许直接把 `sort` 设置为 `ext:<field_key>`（URL 作为事实源），并确保分页/筛选 patch 不会意外清空 sort（除非用户显式改排序）。

## 4. 接口契约（仅列本计划新增/变更点）

> 既有 SSOT：`DEV-PLAN-100D`（list ext query 参数、错误码）；`DEV-PLAN-100D2`（field-definitions 契约收口）。

### 4.1 List：允许在 parent_org_code 下使用 ext query（如选择 KDD-100G-01）

- `GET /org/api/org-units?as_of=...&mode=grid&page=...&size=...&parent_org_code=...`
  - 允许追加：
    - `ext_filter_field_key=<field_key>&ext_filter_value=<value>`
    - `sort=ext:<field_key>&order=asc|desc`

### 4.2 Field Definitions：补齐 allow_filter/allow_sort（admin-only）

变更：

- `GET /org/api/org-units/field-definitions`（Authz：`orgunit.admin`）
- 在既有 `orgUnitFieldDefinitionAPIItem` 上补齐：
  - `allow_filter: boolean`
  - `allow_sort: boolean`

约束：

- 输出稳定排序（按 `field_key` 升序）。
- `allow_filter/allow_sort` 的事实源是服务端 fieldmeta（SSOT），前端只消费结果，不自造规则。

## 5. 实施步骤（100G 执行清单）

> 顺序：先补齐“可用字段事实源” -> 再做列表 UI -> 再补齐 i18n 与 E2E -> 最后收口门禁证据。

1. [x] **后端契约对齐（必需）**
   - [x] 允许 `parent_org_code` + ext query（按 KDD-100G-01）
     - [x] 移除/调整 `internal/server/orgunit_api.go` 中的拒绝分支（当前会返回 `invalid_request: ext query not allowed for parent_org_code`）。
     - [x] 更新并加固测试（避免回归/漂移）：
       - [x] `internal/server/orgunit_list_ext_query_test.go`：将 “ExtQueryNotAllowedForParentOrgCode” 调整为“允许”并断言 handler 能把 `ParentID + ExtSortFieldKey/ExtFilter*` 透传到 `ListOrgUnitsPage`（使用 `orgUnitListPageReaderStore` 捕获 req）。
   - [x] 为 `field-definitions` 响应补齐 `allow_filter/allow_sort`
     - [x] `internal/server/orgunit_field_metadata_api.go`：扩展 `orgUnitFieldDefinitionAPIItem` JSON 输出（`allow_filter/allow_sort`）。
     - [x] `internal/server/orgunit_field_metadata_api_test.go`：补齐契约测试断言（至少验证字段存在且排序稳定）。

2. [x] **前端：列表页 ext filter/sort UI（OrgUnitsPage）**
   - [x] 权限 gating（KDD-100G-04）：
     - [x] 仅 `orgunit.admin` 渲染 ext filter/sort 控件；
     - [x] 非 admin 若 URL 含 ext 参数，replace 清理并提示。
   - [x] 在 FilterBar 增加扩展筛选控件（admin-only）：
     - [x] field 下拉：仅展示 queryable 且 enabled 的字段（KDD-100G-02：field-definitions + field-configs(status=enabled)）
     - [x] value 输入：
       - [x] DICT：复用 `fields:options` endpoint（`q/limit/as_of`）做 Autocomplete，提交 value=code
       - [x] PLAIN/bool/date/int/uuid：按 `value_type` 提供最小输入控件（MVP 先支持 text/DICT）
   - [x] 扩展排序控件（admin-only；KDD-100G-05）：
     - [x] sort by：支持 core 字段（code/name/status）与 ext 字段（`ext:<field_key>`）
     - [x] order：asc/desc
   - [x] URL 可分享：筛选/排序写入 query params；页面 reload 后可复现。
   - [x] fail-closed：当 `as_of` 改变导致 ext 字段集合变化时，自动清理无效 ext filter/sort，并展示可解释提示（toast/Alert）。
   - [x] ext query 参数规范化（避免“半参数”）：
     - [x] `ext_filter_field_key` 与 `ext_filter_value` 必须成对；否则清理并提示。
     - [x] ext filter 仅在 field/value 都合法且字段在 enabled∩queryable 集合中时才进入列表请求；否则清理并提示。

3. [x] **i18n 收口（en/zh）**
   - [x] 增加列表页扩展筛选/排序控件文案 key（en/zh 同步）。
   - [x] 增加 ext query 失败路径提示文案 key（en/zh 同步），并在 UI 中基于“服务端业务错误码”映射到可解释提示（避免仅展示 raw message）。
     - [x] 冻结实现口径：读取 `ApiClientError.details?.code`（服务端 `ErrorEnvelope.code`）作为业务码来源；例如 `ORG_EXT_QUERY_FIELD_NOT_ALLOWED`、`invalid_request` 等。

4. [x] **测试与证据**
   - [x] 前端单测：URL query state 解析/写回（含 ext filter/sort 的“可分享 + 回放”）。
   - [x] E2E：补齐 1 条端到端用例（管理员）：
     - enable 字段（字段配置页） -> details 写值 -> list 按 ext filter/sort -> open details 验证回显。
   - [x] 本地门禁按触发器执行并在 `docs/dev-records/` 记录证据（如需新增执行日志，可按 `DEV-PLAN-010` 口径创建 `dev-plan-100g-execution-log.md`）。

## 6. 验收标准（最小可交付）

1. [x] 列表页至少支持 1 个扩展字段（`d_org_type`）的筛选与排序，并能通过 URL 分享复现（admin-only MVP）。
2. [x] 选择 ext filter/sort 时，请求严格走 `mode=grid` 分页路径；TreePanel roots/children 请求不携带 ext query 参数。
3. [x] 当字段在 `as_of` 下未启用/已停用/不允许筛选排序时：
   - [x] UI 禁用该字段或自动清理无效参数；
   - [x] 并展示可解释原因（i18n 文案，不是 raw error）。
4. [x] i18n en/zh 完整：新增的列表控件文案与错误提示 key 在两种语言下均可渲染。
5. [x] E2E 证明闭环：字段配置 -> 写入 -> 列表筛选/排序 -> 详情回显，至少 1 条路径通过。
6. [x] 非 admin 行为冻结（KDD-100G-04）：URL 若带 ext 参数必须被清理并提示；不得出现“隐藏筛选/排序”。

## 7. 代码落点（建议）

- 后端：
  - `internal/server/orgunit_api.go`（放开 parent_org_code + ext query；错误码/行为保持 fail-closed）
  - `internal/server/orgunit_field_metadata_api.go`（field-definitions 增加 `allow_filter/allow_sort`）
  - `internal/server/orgunit_list_ext_query_test.go`（更新 parent+ext query 的契约测试）
  - `internal/server/orgunit_field_metadata_api_test.go`（field-definitions 契约测试补齐）
- 前端（MUI）：
  - `apps/web/src/pages/org/OrgUnitsPage.tsx`（FilterBar 增加 ext filter/sort；URL 状态；admin gating；fail-closed 清理）
  - `apps/web/src/api/orgUnits.ts`（如需：list 接口 sort 参数类型放宽以支持 `ext:*`；复用 options/field-configs/field-definitions）
  - `apps/web/src/i18n/messages.ts`（新增列表 ext 控件与错误提示 key，en/zh 同步）
  - （如需）`apps/web/src/utils/gridQueryState.ts`（仅当决定让 parse 支持 ext sort；否则在 OrgUnitsPage 内独立解析）
- E2E：
  - `e2e/tests/`（新增或扩展 1 条覆盖“启用字段 -> 写值 -> 列表筛选/排序 -> 回显”的用例）

## 7. 关联文档（SSOT）

- 总路线图：`docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- Phase 3（API/列表 ext query 契约）：`docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- Phase 3 修订：`docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
- Phase 4A（详情页）：`docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- Phase 4B（字段配置 UI IA）：`docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- 时间上下文收敛：`docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- MUI-only：`docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`
- i18n 边界：`docs/dev-plans/020-i18n-en-zh-only.md`
- 门禁与触发器：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`
