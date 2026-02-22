# [Archived] DEV-PLAN-101：OrgUnit 字段配置管理页（MUI）IA 与组件级方案（承接 DEV-PLAN-100）

**状态**: 已完成（2026-02-15 08:41 UTC）

## 1. 背景

`DEV-PLAN-100` 已明确 OrgUnit 扩展字段（宽表预留字段 + 元数据驱动）的落地路线，并在 Phase 4 要求形成“配置字段 -> 写入值 -> 列表筛选/排序 -> 详情回显”的用户可见闭环。  
当前缺口是：**字段配置管理页**的 IA（入口/导航）与组件级页面结构尚未冻结，容易在实现阶段出现“临时入口/分叉页/不可发现”的漂移。

本计划（DEV-PLAN-101）在不引入额外架构概念的前提下，给出一个**可直接实现**的 MUI 页面方案，且对齐仓库不变量与 UI 既有基线：

- One Door / No Legacy / No Tx, No RLS（对齐 `DEV-PLAN-100`）
- 权限边界（read/admin，统一 403）（对齐 `DEV-PLAN-022`）
- 路由治理（命名空间与门禁）（对齐 `DEV-PLAN-017`）
- 时间上下文（`as_of` 语义与来源收敛）（对齐 `DEV-PLAN-102`）
- Org 详情页形态：独立页面 + 双栏布局基线（对齐 `DEV-PLAN-097/099`）
- i18n：仅 en/zh；不做业务数据多语言（对齐 `DEV-PLAN-020`）

## 2. 目标（DoD）

1. [x] 字段配置管理页在租户 App 内**可发现、可达**（导航入口 + Org 页面内入口）。
2. [x] 仅 `orgunit.admin` 可访问；无权限时统一 `NoAccessPage`（fail-closed）。
3. [x] 页面可完成最小管理闭环：
   - 查看当前租户已配置字段（含有效期与映射槽位只读）。
   - 启用（新增）一个字段配置（由后端分配 `physical_col`，前端不可选）：
     - 内置字段：从 `field-definitions` 选择 `field_key`；
    - 自定义字段：输入自定义 `field_key`（`x_` 命名空间；PLAIN 直接值；启用时可选 `value_type=text/int/uuid/bool/date/numeric`；对齐 `DEV-PLAN-110`）；
     - DICT（对齐 `DEV-PLAN-106A`）：在“字段”选择阶段直接选择字典字段（`field_key=d_<dict_code>`），候选来源为字典模块 dict registry（`GET /iam/api/dicts?as_of=enabled_on`）；启用时允许填写可选展示名（label）；
     - ENTITY：仍需从 `field-definitions.data_source_config_options` 选择 `data_source_config`（枚举化候选）。
   - 停用一个字段配置（按 day 粒度选择 `disabled_on`）；若停用尚未生效，支持“延期停用”（仅向后延迟）。
   - “禁用”状态可解释：在 `status=disabled` 下区分 `未生效/已停用`（展示与二级筛选）。
4. [x] 页面加载/空态/错误态可解释；不出现“静默失败/看起来成功但实际未生效”。

## 3. 非目标（明确不做）

- 不在本计划内设计/变更 DB schema、Kernel 函数或元数据表结构（其契约归 `DEV-PLAN-100` Phase 1/2/3）。
- 不在本计划内引入“任意租户可自定义字段 label 的多语言存储结构”（这会触及“业务数据多语言”边界，需另立 dev-plan）。
- 不为字段配置管理页引入抽屉（Drawer）承载形态；遵循 Org 详情页 page pattern（`DEV-PLAN-097`）。

## 4. IA：入口与导航（冻结）

### 4.1 导航栏（MUI AppShell 左侧主导航）

新增一个仅管理员可见的导航项：

- label：`字段配置`（i18n：`nav_org_field_configs`）
- path：`/org/units/field-configs`
- permission：`orgunit.admin`
- 位置：紧随 `组织架构`（`/org/units`）之后（order=21），避免用户在“Org 模块上下文”之外寻找。

> 说明：
>
> - 当前 `apps/web` 导航为扁平结构（无二级菜单）。本方案不引入新的导航层级，只新增一项，保持简单。
> - 旧链路的服务端渲染导航（例如 `renderNav`/旧页面适配器）不在本计划范围；本计划交付物以 `/app/**` 内可发现为准（对齐 `DEV-PLAN-103` 方向）。

### 4.2 模块内入口（增强可发现性）

在以下位置提供“字段配置”快捷入口（仅 `orgunit.admin` 可见）：

1. OrgUnits 列表页 `PageHeader` actions 增加按钮：`字段配置` -> 跳转到 `/org/units/field-configs` 并携带 `as_of`（若存在；仅用于预览上下文，不作为写操作默认生效日）。  
2. OrgUnit 详情页 `PageHeader` actions 增加按钮：`字段配置` -> 同上（便于从详情直接进入管理）。

## 5. 路由与 URL 协议（冻结）

- UI 页面路由：`/app/org/units/field-configs`
  - 说明：`apps/web` router `basename='/app'`；本文档中的 `path: /org/...` 指“App 内路由 path（不含 basename）”，实际浏览器 URL 为 `/app` + `path`。
- Query：
  - `as_of=YYYY-MM-DD`：用于“查看/预览”某日生效的字段配置集合；缺省为当天 UTC（对齐 Org 页 `as_of` 习惯）。
    - `as_of` 仅影响列表/详情的“预览口径”，不应被理解为写入意图。
    - 本页不依赖 Shell/Topbar 注入/拼接 `as_of`；`as_of` 的缺省、校验与写回由本页 query params 自管理（对齐 `DEV-PLAN-102`）。
    - 启用/停用对话框中日期默认值：`max(today_utc, as_of)`（避免 `as_of` 为过去日期时误回溯或触发后端拒绝；SSOT：`DEV-PLAN-100B`）。
  - `status=all|enabled|disabled`（可选）：列表筛选；缺省 `all`。
    - status 口径 SSOT：`DEV-PLAN-100D` §5.2.1（半开区间 `[enabled_on, disabled_on)`；`disabled` 包含“未来生效（as_of < enabled_on）”与“已停用（disabled_on <= as_of）”，UI 不得另起一套判定）。
    - **用户显示口径（本计划冻结）**：
      - `enabled`：`enabled_on <= as_of` 且（`disabled_on IS NULL OR as_of < disabled_on`）
      - `pending`（未生效）：`as_of < enabled_on`（属于 `status=disabled` 的子集；用于 UI Chip/二级筛选）
      - `disabled`（已停用）：`disabled_on IS NOT NULL AND disabled_on <= as_of`（属于 `status=disabled` 的子集；用于 UI Chip/二级筛选）
  - `disabled_state=all|pending|disabled`（可选，**纯 UI 过滤**）：仅当 `status=disabled` 时启用；用于把“未生效/已停用”分开展示与分享链接，**不影响后端 API 查询**（API 仍仅接受 `status`）。

权限：
- Route 级别：`RequirePermission permissionKey='orgunit.admin'`（直接拒绝，不做 UI 内部 soft fallback）。

> API 契约 SSOT：`DEV-PLAN-100D`（field-definitions + field-configs list/enable/disable）与 `DEV-PLAN-100D2`（契约收口修订）；本计划仅冻结 UI IA/组件结构。
>
> 约定：`today_utc` 为当天 UTC day（`YYYY-MM-DD`；实现可用 `new Date().toISOString().slice(0, 10)`）。

## 6. 页面信息架构与组件级布局（MUI）

页面组件：`OrgUnitFieldConfigsPage`（建议新建）。

### 6.1 顶部结构

- `Breadcrumbs`
  - `组织架构`（链接 `/org/units`，携带 `as_of`）
  - `字段配置`（当前页）
- `PageHeader`
  - title：`字段配置`
  - subtitle：`管理 OrgUnit 扩展字段：启用、停用与数据源配置（管理员）`
  - actions：
    - `启用字段`（primary，打开启用对话框）

### 6.2 过滤区（FilterBar）

使用 `FilterBar`（outlined Paper）承载：

- `as_of`：`TextField type="date"`（InputLabel shrink），默认当天 UTC。
- `status`：`Select`（all/enabled/disabled）。
- `disabled_state`：`Select`（all/pending/disabled；仅当 `status=disabled` 时展示/生效；纯客户端过滤）。
- `keyword`（可选增强）：`TextField`，用于在列表中按 `field_key/label` 客户端过滤（不要求后端支持）。
- `应用筛选`：`Button variant="contained"`，写回 query params，触发重新加载。

### 6.3 列表区（DataGridPage）

使用 `DataGridPage` 展示字段配置列表（server-side 不强制，MVP 可一次性加载后前端展示）。

建议列：

- `field_label`：字段名称：
  - 内置字段：来自 `label_i18n_key`，前端 `t(...)` 渲染（服务端必须返回；禁止前端维护第二套映射）；
  - 字典字段（`d_`）：优先展示服务端返回的 `label`（启用时自定义 label 优先，否则为 dict name / dict_code；SSOT：`DEV-PLAN-106A`）；
  - 自定义字段（`x_`）：展示服务端返回的 `label`（MVP 可等于 field_key；不得引入租户可编辑多语言持久化结构；对齐 `DEV-PLAN-106`）。
- `field_key`：稳定键（只读）
- `value_type`：`text/int/uuid/bool/date/numeric`（只读）
- `data_source_type`：`PLAIN/DICT/ENTITY`（只读）
- `data_source_config`：只读（展示 `dict_code` 或 `entity/id_kind`，便于排障）
- `status`：`enabled/disabled`（Chip）
  - **冻结为三态 Chip**：`enabled/未生效/已停用`
    - enabled：`enabled`
    - as_of < enabled_on：`未生效`
    - disabled_on != null && disabled_on <= as_of：`已停用`
- `enabled_on`：date
- `disabled_on`：date（为空表示未计划停用）
- `physical_col`：只读（映射槽位，便于排障；对齐 `DEV-PLAN-100` “映射不可变”）
- `updated_at`：审计时间（可选）
- `actions`：行内操作（按钮组）
  - `查看`（打开“详情对话框”，只读）
  - `停用`（打开停用对话框；仅当 `disabled_on` 为空时可用）
  - `延期停用`（打开“延期停用”对话框；仅当 `disabled_on` 非空且 **未生效**（`today_utc < disabled_on`）时可用；只允许把日期**往后推**；对齐 DB 规则：仅向后延迟 + 原 disabled_on 尚未生效（SSOT：`DEV-PLAN-100B`））

空态：
- 列表为空时显示 `NoRowsOverlay` + CTA：`启用字段`。

### 6.4 启用字段对话框（Dialog，组件级）

目的：创建（启用）一个新的字段配置；由后端分配 `physical_col`。

- `DialogTitle`：`启用字段`
- `DialogContent` 表单（`Stack spacing={2}`）：
1) `field_key` 选择：
     - 控件（冻结）：第三步“字段”分三类来源（同一选择器内分组展示，或显式三选一模式）：
       - **内置字段（非 DICT）**：`Select/Autocomplete`（数据源：`GET /org/api/org-units/field-definitions`；UI 必须排除/禁用其中 `data_source_type=DICT` 的 built-in 字段，避免走旧路径）
       - **字典字段（DICT；106A）**：`Select/Autocomplete`（数据源：`GET /org/api/org-units/field-configs:enable-candidates?enabled_on=<enabled_on>`，展示为 `name + dict_code`，提交 `field_key=d_<dict_code>`）
      - **自定义字段（直接值）**：`TextField` 输入 `field_key`（必须满足 `x_[a-z0-9_]{1,60}`）；并可选择 `value_type=text/int/uuid/bool/date/numeric`（对齐 `DEV-PLAN-110`）
     - 约束：候选/输入都应排除已存在于 `field-configs(status=all)` 的 `field_key`（无论当前 `as_of` 下状态如何），避免触发后端 “已存在/不可重复启用” 冲突。
  2) `enabled_on`：
     - `TextField type="date"`，默认 `max(today_utc, as_of)`
     - 提示：`enabled_on` 启用后不可修改（SSOT：`DEV-PLAN-100B`），请谨慎选择。
  3) `label`（仅字典字段 `d_...`）：
     - 选择字典字段后展示一个可选输入框：`TextField label="描述/展示名"`；
     - 若不填：默认展示名由服务端按 dict name / dict_code 推导（SSOT：`DEV-PLAN-106A`）。
  4) `data_source_config`（仅 ENTITY）：
     - PLAIN：不展示（或只读展示为 `{}`），因为该类型无 options。  
     - DICT（字典字段）：不展示（dict_code 由 `field_key=d_<dict_code>` 推导；禁止再提交第二份 dict_code）。  
     - ENTITY：显示 `entity/id_kind` 选择器：
       - 选项来源：`GET /org/api/org-units/field-definitions` 的 `data_source_config_options`（枚举化标识；禁止输入任意表/列名，对齐 `DEV-PLAN-100` D7）。  
       - 若候选仅 1 个：只读展示。
- `DialogActions`：
  - `取消`
  - `确认启用`（primary；提交中 disable）
- 请求幂等（必须）：
  - `request_code` 为 UI 内部幂等键（用户无感知；例如 UUID）。
  - **复用规则（本计划冻结）**：
    - 对话框打开时生成一次 `request_code`。
    - 用户修改任一关键输入（`field_key/enabled_on/data_source_config/label`）时，必须生成新的 `request_code`（避免把“不同意图”误当成同一次重试，触发 `ORG_REQUEST_ID_CONFLICT`）。
    - 仅当输入未变化时：失败重试/重复点击需复用同一个 `request_code`，以便后端按幂等语义返回同一配置行（避免重复占用槽位）。
- 成功行为：
  - toast：`启用成功`
  - 关闭对话框
  - 刷新列表；新行应展示由后端分配的 `physical_col`
- 失败行为（至少覆盖）：
  - 槽位耗尽：展示稳定错误文案（例如“槽位已耗尽，请联系管理员扩容”）
  - data_source_config 不合法：展示可解释错误（例如“数据来源配置无效/不被允许，请刷新后重试”）
  - 权限不足：展示统一无权限提示（不在对话框内做“假成功”）

### 6.5 停用/延期停用字段对话框（Dialog，组件级）

目的：为已有字段配置设置 `disabled_on`（day 粒度）；若已存在且未生效，则支持“延期停用”（仅向后延迟）。

- `DialogTitle`：
  - 新增停用：`停用字段`
  - 延期停用：`延期停用字段`
- `DialogContent`：
  - 只读摘要：`field_label(field_key)` + `physical_col`
  - `disabled_on`：`TextField type="date"`
    - 新增停用（原 `disabled_on` 为空）：
      - 默认：`max(today_utc, as_of)`
      - 校验（前端先行拦截，后端为准）：`disabled_on >= today_utc` 且 `disabled_on >= enabled_on`（SSOT：`DEV-PLAN-100B`）。
    - 延期停用（原 `disabled_on` 非空且 `today_utc < old_disabled_on`）：
      - 入口：列表行内按钮 `延期停用`
      - 默认：`max(old_disabled_on + 1 day, today_utc, as_of)`（提示用户“当前计划停用日”为 old_disabled_on）
      - 校验（前端先行拦截，后端为准）：
        - `disabled_on > old_disabled_on`（只允许向后延迟）
        - `today_utc < old_disabled_on`（仅当原计划尚未生效）
        - 且仍需满足 `disabled_on >= enabled_on` 与 `disabled_on >= today_utc`
      - 备注：该能力与 DB trigger 规则一致（仅向后延迟 + 原 disabled_on 尚未生效；SSOT：`DEV-PLAN-100B`）。
    - 约束提示（冻结）：
      - `disabled_on` 一旦设置不允许回滚为 `null`（SSOT：`DEV-PLAN-100B`）。
  - 风险提示（Alert warning）：
    - “从 disabled_on 起，该字段在对应 as_of 下将不可写/不可见（details 的 ext_fields 将不再返回/不再展示）；映射槽位不可复用。若需查看历史，请切换 as_of 或查看 Audit（变更日志）。”（对齐 `DEV-PLAN-100D/100E`）
    - 备注：若未来需要“字段仍显示但禁用 + 给出禁用原因”，必须扩展 details 契约（例如返回 disabled 字段列表 + 禁用原因/日期）；本计划不做。
- `DialogActions`：
  - 新增停用：取消/确认停用
  - 延期停用：取消/确认延期
- 请求幂等（必须）：
  - `request_code` 为 UI 内部幂等键（用户无感知；例如 UUID）。
  - 复用规则同启用对话框：用户修改 `disabled_on` 时必须生成新的 `request_code`；仅当输入未变化时重试复用同一个 `request_code`。
- 成功：toast + 刷新列表

## 7. 权限与失败模式（fail-closed）

- 页面与所有写操作必须要求 `orgunit.admin`（对齐 `DEV-PLAN-022` read/admin 口径）。
- 无权限：
  - 导航项隐藏（navItems permissionKey）
  - 直接访问 URL：`RequirePermission` 返回 `NoAccessPage`
- API/网络错误：
  - 页面顶部 `Alert severity="error"` 显示错误摘要（不吞错）
  - 对话框提交失败：在对话框内显示 `Alert` 并保留用户输入，避免“点了就消失”

## 8. i18n 与文案（仅 en/zh）

- 页面静态文案使用 `apps/web/src/i18n/messages.ts` 增加 key（en/zh 同步）。
- 导航项使用 i18n key：`nav_org_field_configs`（en/zh 同步）。
- 字段名称（field_label）口径（MVP 冻结）：
  - 字段定义由后端返回 `label_i18n_key`，前端通过 `t(label_i18n_key)` 渲染（禁止前端再建一套字段 label 映射）；
  - 自定义字段（`x_`）无 `label_i18n_key`，UI 展示优先级：`label(display_label)` -> `field_key`（不得引入租户可编辑的多语言持久化结构）；
  - 禁止在本计划引入“租户可编辑 label_zh/label_en 并持久化”的多语言业务数据形态（如需，另立 dev-plan）。

## 9. 验收标准（最小可交付）

1. [x] `tenant_admin`（含 `orgunit.admin`）可在导航栏看到 `字段配置`，可进入页面并加载数据。
2. [x] `tenant_viewer`（仅 `orgunit.read`）看不到导航项，直接访问 URL 返回 `NoAccessPage`。
3. [x] 启用字段成功后：
   - 列表出现新配置行（含 `physical_col`）。
   - 回到 OrgUnit 详情页，在该字段的 enabled 区间内（`enabled_on <= as_of` 且 `disabled_on IS NULL OR as_of < disabled_on`）可看到该扩展字段（细节由 `DEV-PLAN-100` Phase 4 承接）。
4. [x] 停用字段成功后：
   - 在字段配置页可看到 `disabled_on` 生效；
   - 在 OrgUnit 详情页，当 `as_of >= disabled_on` 时该字段不再展示（不在 details 的 `ext_fields[]` 中出现）；若需查看历史，请切换 `as_of` 或查看 Audit（对齐 `DEV-PLAN-100D/100E`）。
5. [x] “禁用”展示可解释：
   - `as_of < enabled_on` 的行在列表中标识为 `未生效`；
   - `disabled_on <= as_of` 的行在列表中标识为 `已停用`；
   - 当 `status=disabled` 时，用户可通过 `disabled_state` 二级筛选快速分流两类。
6. [x] 延期停用可用且符合约束：
   - 若 `disabled_on` 已设置且 `today_utc < disabled_on`，可把 `disabled_on` 往后改（只能延期，不能提前/不能回滚为 null）；
   - 若 `today_utc >= disabled_on`，禁止延期（按钮不可用且提示原因）。
7. [x] 失败路径可解释：槽位耗尽/权限不足/网络错误不会静默。

## 10. 代码落点（建议）

- 页面：`apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx`（新建）
- 路由：`apps/web/src/router/index.tsx` 注册 `path: 'org/units/field-configs'`
- 导航：`apps/web/src/navigation/config.tsx` 增加 nav item（permissionKey=`orgunit.admin`）
- i18n：`apps/web/src/i18n/messages.ts` 增加页面/按钮文案 key（en/zh）
- API client（如需）：`apps/web/src/api/orgUnits.ts` 或拆分新文件 `apps/web/src/api/orgUnitFieldConfigs.ts`

> 说明：若按 `DEV-PLAN-103` 的“工程命名去技术后缀”执行机械改名（例如 `apps/web` → `apps/web`），本节路径需同步更新；不影响本计划冻结的 IA/契约。

## 11. 门禁与验证（SSOT 引用）

- 触发器与命令入口以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
- 本计划实现通常会命中：前端 Typecheck/Lint/Test/Build、路由门禁（若新增后端路由）、Authz 门禁（若新增权限点）、文档门禁（本文件）。

## 12. 关联文档

- `docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- `docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`
- `docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
- `docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- `DEV-PLAN-103（MUI-only 前端收敛）`
- `docs/dev-plans/097-orgunit-details-drawer-to-page-migration.md`
- `docs/dev-plans/099-orgunit-details-two-pane-info-audit-mui.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/020-i18n-en-zh-only.md`
- `AGENTS.md`
