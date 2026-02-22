# [Archived] DEV-PLAN-099：OrgUnit 信息页双栏化（左生效日期/修改时间，右侧详情）——对齐示例

**状态**: 已归档（2026-02-22，口径已并入 `DEV-PLAN-096`，本文保留实现与验收记录）

## 背景

- 参考示例截图：用户附件（本仓库未入仓；如需入仓，请将原图保存至 `docs/dev-records/` 并在此处补链接）
- 目标是在 **MUI 版 OrgUnit 详情页**中，对齐示例的“左侧时间轴列表 + 右侧详情”的双栏交互模型：
  - 基本信息：左侧按 **生效日期（effective_date）** 浏览版本，右侧展示该版本的字段详情。
  - 变更日志：左侧按 **修改时间（tx_time）** 浏览事件，右侧展示事件详情与字段差异。

> 说明：当前仓库内 Go 旧版 OrgUnit 页面已在 `DEV-PLAN-081` 落地双栏结构；本计划聚焦于 `apps/web` 的实现对齐与口径一致。

## 目标（DoD）

1. [x] OrgUnit 详情页 `基本信息/变更日志` 两个 Tab 均采用双栏布局：左侧列表、右侧详情。
2. [x] 基本信息 Tab：左侧显示版本列表（生效日期 + 事件类型），点击切换后右侧详情同步更新（以 `effective_date` 为唯一版本选择入口）。
3. [x] 变更日志 Tab：左侧显示事件列表（修改时间 + 操作人），点击切换后右侧展示该事件的摘要信息与字段差异（最小实现可先只展示 diff；原始 JSON 作为可折叠“原始数据”）。
4. [x] （增强）变更日志右侧补齐 `request_code / reason / 撤销标识（is_rescinded + rescinded_by_*）`，左侧事件列表增加“已撤销”标签。
5. [x] URL 可复现：
   - 版本选择继续使用 `effective_date=YYYY-MM-DD`（已存在）。
   - 变更日志选中项引入 `audit_event_uuid=<uuid>`（新增），用于刷新/分享时复现右侧详情。
6. [x] 文案与视觉对齐示例，且遵循全局主色 `#09a7a3`（MUI `palette.primary.main` 已冻结）。

## 非目标

- 不新增/修改后端 API（复用已存在的 details/versions/audit 接口）。
- 不引入“抽屉/页面”双实现并行，不新增 legacy 回退通道（UI 内部兼容旧 query 参数仅作为软迁移，不作为长期契约）。
- 不在本计划调整 Org 领域事件模型、RLS/Authz 边界与写入口（以既有 dev-plan 为准）。

## 方案概览

### 1) 信息架构（IA）

- Tabs 收敛为两项：
  - `基本信息`（原 `profile` + `records` 的信息合并承载）
  - `变更日志`（原 `audit`）
- 版本切换入口收敛：
  - 基本信息：只通过左侧“生效日期列表”切换。
  - 变更日志：只通过左侧“修改时间列表”切换。

### 2) 布局与组件（MUI）

- 双栏布局建议采用 `Paper(variant="outlined") + CSS Grid/Stack`：
  - `md+`：`240px / 1fr` 两列；左栏固定宽度并滚动；右栏自适应。
  - `xs/sm`：自动降级为上下布局（列表在上、详情在下），避免横向挤压。
- 列表项使用 `ListItemButton selected`，并通过主题色突出选中态（默认即可满足）。
- 右侧详情使用分段结构：标题行 + 分隔线 + 摘要字段 +（变更日志）字段差异表 + 原始数据折叠。

### 3) 数据映射

- 基本信息右侧“事件类型”：
  - 从 `versions` 列表按 `effective_date` 匹配得到 `event_type`（无需后端扩展）。
- 变更日志右侧字段差异：
  - 使用 `before_snapshot/after_snapshot` 做最小 diff（只对同层 key 做对比；展示 `字段/变更前/变更后`）。
  - 若快照缺失或不可解析为对象：显示空态，并允许展开“原始数据”查看 payload/snapshot。

### 4) i18n 与文案

- 更新 Org 页面的状态短标签为 **有效/无效**（新增 org 专用 key，避免污染 People 的“在职/离职”口径）。
- Tabs 文案对齐示例：`基本信息` / `变更日志`。

## 代码落点

- `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
  - Tabs 收敛与双栏布局实现。
  - 新增 `audit_event_uuid` query 参数解析与写回。
- `apps/web/src/i18n/messages.ts`
  - 新增/调整本计划涉及的文案 key（en/zh 同步）。
- （可选）`apps/web/src/pages/org/OrgUnitsPage.tsx`
  - Org 状态显示改用 org 专用 status key，保证列表/详情一致。

## 验收标准

- [x] `基本信息`：左侧生效日期列表可滚动；点击任意日期右侧详情更新；URL 中 `effective_date` 同步变化。
- [x] `变更日志`：左侧修改时间列表可滚动；点击任意事件右侧详情更新；URL 中 `audit_event_uuid` 同步变化；刷新页面后仍能复现选中项。
- [x] 选中态视觉突出且与主题色一致；窄屏下布局不溢出、不重叠。
- [x] 不新增后端变更；不破坏既有 details/versions/audit 查询与写操作对话框。

## 验证记录（本地）

- `pnpm --dir apps/web typecheck lint test build`
- `make css`
- `make check doc`
- `cd e2e && pnpm exec playwright test --list`
- E2E 新增用例：`e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`（建议在 `make e2e` 启动的标准环境下执行）

## 关联与 SSOT

- `AGENTS.md`
- `docs/dev-plans/081-orgunit-records-version-selector-two-pane-alignment.md`（旧版双栏基线）
- `docs/archive/dev-plans/097-orgunit-details-drawer-to-page-migration.md`（详情页 page pattern）
