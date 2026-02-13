# DEV-PLAN-101：OrgUnit 字段配置管理页（MUI）IA 与组件级方案（承接 DEV-PLAN-100）

**状态**: 草拟中（2026-02-13 05:00 UTC）

## 1. 背景

`DEV-PLAN-100` 已明确 OrgUnit 扩展字段（宽表预留字段 + 元数据驱动）的落地路线，并在 Phase 4 要求形成“配置字段 -> 写入值 -> 列表筛选/排序 -> 详情回显”的用户可见闭环。  
当前缺口是：**字段配置管理页**的 IA（入口/导航）与组件级页面结构尚未冻结，容易在实现阶段出现“临时入口/分叉页/不可发现”的漂移。

本计划（DEV-PLAN-101）在不引入额外架构概念的前提下，给出一个**可直接实现**的 MUI 页面方案，且对齐仓库不变量与 UI 既有基线：

- One Door / No Legacy / No Tx, No RLS（对齐 `DEV-PLAN-100`）
- 权限边界（read/admin，统一 403）（对齐 `DEV-PLAN-022`）
- 路由治理（命名空间与门禁）（对齐 `DEV-PLAN-017`）
- Org 详情页形态：独立页面 + 双栏布局基线（对齐 `DEV-PLAN-097/099`）
- i18n：仅 en/zh；不做业务数据多语言（对齐 `DEV-PLAN-020`）

## 2. 目标（DoD）

1. [ ] 字段配置管理页在租户 App 内**可发现、可达**（导航入口 + Org 页面内入口）。
2. [ ] 仅 `orgunit.admin` 可访问；无权限时统一 `NoAccessPage`（fail-closed）。
3. [ ] 页面可完成最小管理闭环：
   - 查看当前租户已配置字段（含有效期与映射槽位只读）。
   - 启用（新增）一个字段配置（由后端分配 `physical_col`，前端不可选）。
   - 停用一个字段配置（按 day 粒度选择 `disabled_on`）。
4. [ ] 页面加载/空态/错误态可解释；不出现“静默失败/看起来成功但实际未生效”。

## 3. 非目标（明确不做）

- 不在本计划内设计/变更 DB schema、Kernel 函数或元数据表结构（其契约归 `DEV-PLAN-100` Phase 1/2/3）。
- 不在本计划内引入“任意租户可自定义字段 label 的多语言存储结构”（这会触及“业务数据多语言”边界，需另立 dev-plan）。
- 不为字段配置管理页引入抽屉（Drawer）承载形态；遵循 Org 详情页 page pattern（`DEV-PLAN-097`）。

## 4. IA：入口与导航（冻结）

### 4.1 导航栏（左侧主导航）

新增一个仅管理员可见的导航项：

- label：`字段配置`
- path：`/org/units/field-configs`
- permission：`orgunit.admin`
- 位置：紧随 `组织架构`（`/org/units`）之后（order=21），避免用户在“Org 模块上下文”之外寻找。

> 说明：当前 `apps/web-mui` 导航为扁平结构（无二级菜单）。本方案不引入新的导航层级，只新增一项，保持简单。

### 4.2 模块内入口（增强可发现性）

在以下位置提供“字段配置”快捷入口（仅 `orgunit.admin` 可见）：

1. OrgUnits 列表页 `PageHeader` actions 增加按钮：`字段配置` -> 跳转到 `/org/units/field-configs` 并携带 `as_of/include_disabled`（若存在）。  
2. OrgUnit 详情页 `PageHeader` actions 增加按钮：`字段配置` -> 同上（便于从详情直接进入管理）。

## 5. 路由与 URL 协议（冻结）

- UI 页面路由：`/app/org/units/field-configs`
- Query：
  - `as_of=YYYY-MM-DD`：用于“查看/预览”某日生效的字段配置集合；缺省为当天 UTC（对齐 Org 页 `as_of` 习惯）。
  - `status=all|enabled|disabled`（可选）：列表筛选；缺省 `all`。

权限：
- Route 级别：`RequirePermission permissionKey='orgunit.admin'`（直接拒绝，不做 UI 内部 soft fallback）。

## 6. 页面信息架构与组件级布局（MUI）

页面组件：`OrgUnitFieldConfigsPage`（建议新建）。

### 6.1 顶部结构

- `Breadcrumbs`
  - `组织架构`（链接 `/org/units`，携带 `as_of/include_disabled`）
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
- `keyword`（可选增强）：`TextField`，用于在列表中按 `field_key/label` 客户端过滤（不要求后端支持）。
- `应用筛选`：`Button variant="contained"`，写回 query params，触发重新加载。

### 6.3 列表区（DataGridPage）

使用 `DataGridPage` 展示字段配置列表（server-side 不强制，MVP 可一次性加载后前端展示）。

建议列：

- `field_label`：字段名称（来自“字段定义”，只读）
- `field_key`：稳定键（只读）
- `value_type`：`text/int/uuid/bool/date`（只读）
- `data_source_type`：`DICT/ENTITY`（只读）
- `status`：`enabled/disabled`（Chip）
- `enabled_on`：date
- `disabled_on`：date（为空表示未计划停用）
- `physical_col`：只读（映射槽位，便于排障；对齐 `DEV-PLAN-100` “映射不可变”）
- `updated_at`：审计时间（可选）
- `actions`：行内操作（按钮组）
  - `查看`（打开“详情对话框”，只读）
  - `停用`（打开停用对话框；disabled 状态时置灰）

空态：
- 列表为空时显示 `NoRowsOverlay` + CTA：`启用字段`。

### 6.4 启用字段对话框（Dialog，组件级）

目的：创建（启用）一个新的字段配置；由后端分配 `physical_col`。

- `DialogTitle`：`启用字段`
- `DialogContent` 表单（`Stack spacing={2}`）：
  1) `field_key` 选择：
     - 控件：`Select` 或 `Autocomplete`
     - 数据：后端返回“可启用字段定义列表”（MVP：2~5 个；对齐 `DEV-PLAN-100` Phase 0 字段清单）
  2) `enabled_on`：
     - `TextField type="date"`，默认等于当前 `as_of`
  3) `data_source_config`：
     - DICT：显示 `dict_code`（若该字段允许配置则为 Select；若字段定义固定 dict 则只读展示）
     - ENTITY：显示 `entity`（枚举化标识，只读或 Select；不得允许输入任意表/列名，对齐 `DEV-PLAN-100` D7）
- `DialogActions`：
  - `取消`
  - `确认启用`（primary；提交中 disable）
- 成功行为：
  - toast：`启用成功`
  - 关闭对话框
  - 刷新列表；新行应展示由后端分配的 `physical_col`
- 失败行为（至少覆盖）：
  - 槽位耗尽：展示稳定错误文案（例如“槽位已耗尽，请联系管理员扩容”）
  - 权限不足：展示统一无权限提示（不在对话框内做“假成功”）

### 6.5 停用字段对话框（Dialog，组件级）

目的：为已有字段配置设置 `disabled_on`（day 粒度）。

- `DialogTitle`：`停用字段`
- `DialogContent`：
  - 只读摘要：`field_label(field_key)` + `physical_col`
  - `disabled_on`：`TextField type="date"`，默认等于当前 `as_of`
  - 风险提示（Alert warning）：
    - “停用后该字段在对应 as_of 下将不可写/不可见；映射槽位不可复用”（对齐 `DEV-PLAN-100`）
- `DialogActions`：取消/确认停用
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

- 页面静态文案使用 `apps/web-mui/src/i18n/messages.ts` 增加 key（en/zh 同步）。
- 字段名称（field_label）口径（MVP 冻结）：
  - 字段定义由后端返回 `label_key`（或前端内置映射），前端通过 `t(label_key)` 渲染；
  - 禁止在本计划引入“租户可编辑 label_zh/label_en 并持久化”的多语言业务数据形态（如需，另立 dev-plan）。

## 9. 验收标准（最小可交付）

1. [ ] `tenant_admin`（含 `orgunit.admin`）可在导航栏看到 `字段配置`，可进入页面并加载数据。
2. [ ] `tenant_viewer`（仅 `orgunit.read`）看不到导航项，直接访问 URL 返回 `NoAccessPage`。
3. [ ] 启用字段成功后：
   - 列表出现新配置行（含 `physical_col`）。
   - 回到 OrgUnit 详情页，在 `as_of >= enabled_on` 时可看到该扩展字段（细节由 `DEV-PLAN-100` Phase 4 承接）。
4. [ ] 停用字段成功后：
   - 在字段配置页可看到 `disabled_on` 生效；
   - 在 OrgUnit 详情页，当 `as_of >= disabled_on` 时该字段不可编辑且有明确禁用原因（对齐 `DEV-PLAN-100`）。
5. [ ] 失败路径可解释：槽位耗尽/权限不足/网络错误不会静默。

## 10. 代码落点（建议）

- 页面：`apps/web-mui/src/pages/org/OrgUnitFieldConfigsPage.tsx`（新建）
- 路由：`apps/web-mui/src/router/index.tsx` 注册 `path: 'org/units/field-configs'`
- 导航：`apps/web-mui/src/navigation/config.tsx` 增加 nav item（permissionKey=`orgunit.admin`）
- i18n：`apps/web-mui/src/i18n/messages.ts` 增加页面/按钮文案 key（en/zh）
- API client（如需）：`apps/web-mui/src/api/orgUnits.ts` 或拆分新文件 `apps/web-mui/src/api/orgUnitFieldConfigs.ts`

## 11. 门禁与验证（SSOT 引用）

- 触发器与命令入口以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
- 本计划实现通常会命中：前端 Typecheck/Lint/Test/Build、路由门禁（若新增后端路由）、Authz 门禁（若新增权限点）、文档门禁（本文件）。

## 12. 关联文档

- `docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- `docs/dev-plans/097-orgunit-details-drawer-to-page-migration.md`
- `docs/dev-plans/099-orgunit-details-two-pane-info-audit-mui.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/020-i18n-en-zh-only.md`
- `AGENTS.md`

