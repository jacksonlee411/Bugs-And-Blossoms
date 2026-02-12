# DEV-PLAN-094：MUI X 升级子计划 P3（长尾迁移与收口）

**状态**: 实施中（2026-02-12 08:53 UTC）

> 本计划承接 `DEV-PLAN-090` 的 §5.4（Phase 3），目标是把“高价值先行”阶段后的长尾页面与重复资产统一收口。

## 1. 子计划范围

- 对应 `DEV-PLAN-090` 步骤：11~13。
- 时间窗口：2~4 周。
- 重点：旧列表、旧树组件、样式/文案一致性、重复代码清理。

## 2. 核心目标（DoD）

- [ ] 旧列表页全面迁移到 DataGrid 基线组件。
- [ ] 旧树组件全面迁移到统一 Tree 基线组件。
- [ ] 全仓 UI 文案、状态反馈、空态/加载态统一。
- [ ] 移除重复组件和不可达死代码，降低维护负担。

## 3. 非目标

- 不在本阶段新增复杂业务功能。
- 不在本阶段扩展跨端能力（仅收口 Web 端）。

## 4. 实施步骤

1. [x] 长尾页面盘点
   - 输出《页面迁移清单》（当前 `apps/web-mui` 范围）：

| Route | 页面 | 当前实现 | 目标基线 | 风险 |
| --- | --- | --- | --- | --- |
| `/` | `FoundationDemoPage` | Tree + Grid + Detail | `TreePanel` + `DataGridPage` | 低 |
| `/org/units` | `OrgUnitsPage` | Tree + Grid（server-mode 模拟） | `TreePanel` + `DataGridPage` + URL 协议 | 中 |
| `/people` | `PeopleAssignmentsPage` | Grid + Bulk + Detail | `DataGridPage` + URL 协议 | 中 |
| `/approvals` | `ApprovalsInboxPage` | Grid + 状态流转 + Detail | `DataGridPage` + URL 协议 | 中 |

2. [x] 列表页收口
   - 统一到 `DataGridPage`（含分页/排序/筛选与空态口径）。
   - 统一 URL 参数协议：`q/status/page/size/sort/order`（并固化为 `gridQueryState` 工具）。
3. [x] 树组件收口（第一轮）
   - 抽取统一 `TreePanel` 组件并在新页面落地复用。
   - 统一选中态显示与 loading/empty 文案口径。
4. [ ] 视觉与文案收口
   - 统一按钮风格、提示文案、表单错误反馈。
   - 清理硬编码颜色与散落样式。
5. [x] 重复资产清理（第一轮）
   - 删除不可达占位页（已无引用）。
   - 清理无引用 i18n key，避免长期漂移。

## 5. 已落地变更（本轮）

- 新增统一树面板组件：`apps/web-mui/src/components/TreePanel.tsx`
- 基座示例页与组织架构页改为复用 `TreePanel`：
  - `apps/web-mui/src/pages/FoundationDemoPage.tsx`
  - `apps/web-mui/src/pages/org/OrgUnitsPage.tsx`
- 列表页参数协议工具化与落地：`apps/web-mui/src/utils/gridQueryState.ts`
- 列表页统一支持 URL 可复现的分页/排序（Org 页为 server-mode 模拟）：
  - `apps/web-mui/src/pages/FoundationDemoPage.tsx`
  - `apps/web-mui/src/pages/org/OrgUnitsPage.tsx`
  - `apps/web-mui/src/pages/people/PeopleAssignmentsPage.tsx`
  - `apps/web-mui/src/pages/approvals/ApprovalsInboxPage.tsx`
- 移除不可达占位页：`apps/web-mui/src/pages/ComingSoonPage.tsx`
- i18n 清理：移除无引用 key（coming-soon / select-department 等），保持 MessageKey 收敛：`apps/web-mui/src/i18n/messages.ts`
- i18n 支持简单变量插值，并收敛通用反馈文案 key：`apps/web-mui/src/i18n/messages.ts`
- DataGrid 容器样式收口到主题色板（修复暗色模式背景不一致）：`apps/web-mui/src/components/DataGridPage.tsx`

## 6. 验收标准

- [ ] 页面迁移清单中的目标项全部关闭。
- [ ] 旧组件引用数降为 0（或全部标记待退役且有截止时间）。
- [ ] UI 一致性检查通过（页面抽样评审 + 自动化检查）。

## 7. 风险与缓解

- 风险：批量替换引入隐藏回归。  
  缓解：按模块分批上线，每批次都有回归用例与可回退点。
- 风险：死代码清理误删。  
  缓解：先标记弃用窗口，再正式移除。

## 8. 执行记录（2026-02-12）

- 已执行并通过：
  - `pnpm -C apps/web-mui lint`
  - `pnpm -C apps/web-mui typecheck`
  - `pnpm -C apps/web-mui test`
  - `pnpm -C apps/web-mui build`
  - `pnpm -C apps/web-mui check`

## 9. 关联计划

- 总方案：`docs/dev-plans/090-mui-x-frontend-upgrade-plan.md`
- 文档治理：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
- 质量门禁：`docs/dev-plans/012-ci-quality-gates.md`
