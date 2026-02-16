# DEV-PLAN-104A：Job Catalog UI 优化补充修订（对齐 DEV-PLAN-002）

**状态**: 草拟中（2026-02-16 02:56 UTC）

> 本文件是 `DEV-PLAN-104` 的规范对齐补充；主方案与 IA 约束见 `docs/dev-plans/104-jobcatalog-ui-optimization.md`。

## 1. 背景

`DEV-PLAN-104` 已完成 Job Catalog 页的信息架构收敛（ContextBar + Tabs + DataGrid + Dialog），但与 `DEV-PLAN-002` 的组件与可访问性规范仍存在若干缺口。  
本修订用于补齐“可验收条款”，避免实现阶段出现“方向正确但不达标”。

> 约束：若本文件与 `DEV-PLAN-104` 存在冲突，以本文件为准（仅限 UI 规范对齐条款）。

## 2. 目标（DoD）

1. [ ] 筛选区满足 `Apply (Primary) + Reset/Clear (Secondary)` 双操作结构。
2. [ ] 有效日期输入统一优先使用 MUI X DatePicker（day 粒度，`YYYY-MM-DD`）。
3. [ ] 行级长尾操作统一收纳到“更多(⋯)”菜单，并满足 Tooltip/aria。
4. [ ] 页面样式不落散乱 hex“魔法值”，全部通过 Theme/Token 分发。
5. [ ] 交互状态矩阵完整：default/hover/focus/disabled/error/loading。
6. [ ] Light/Dark 两套主题下语义一致（主按钮、边界、空错禁用态均可读）。

## 3. 修订范围（仅补充，不改业务契约）

- 仅补充 UI 规范落地条款与验收口径；
- 不改 `DEV-PLAN-104` 既定 IA（单页 + Tabs + Dialog）；
- 不改后端 API 与业务语义。

## 4. 冻结条款（对齐 DEV-PLAN-002）

### 4.1 筛选区结构（强制）

- `JobCatalogContextBar` 必须提供：
  - `Apply Context`：`Button variant="contained" color="primary"`（Primary）
  - `Reset/Clear`：`Button variant="outlined"` 或 `text`（Secondary）
- `Reset/Clear` 行为：恢复 URL Query 到默认值（`as_of=today_utc`，清空 `package_code/group_code`，`tab` 保留当前）。

### 4.2 日期控件（强制）

- `as_of` 与所有写对话框的 `effective_date` 优先使用 `@mui/x-date-pickers` DatePicker。
- 日期粒度固定为 day（不出现时分秒输入）。
- 对外展示/提交格式统一 `YYYY-MM-DD`（与 `DEV-PLAN-032` 一致）。

### 4.3 行级操作（强制）

- DataGrid 的行级长尾动作（Move/View 等）默认进入 `More(⋯)` 菜单。
- 显式 icon/button 必须有 Tooltip 或 `aria-label`。
- 危险操作（若后续新增）必须二次确认，不与 Primary 同层抢占注意力。

### 4.4 Token 与样式来源（强制）

- 主色与对比色来自 Theme Token（`primary.main=#09a7a3`，`primary.contrastText=#ffffff`）。
- 页面层禁止新增随机灰阶/随机圆角/随机间距；优先使用 Theme spacing/shape/palette。
- 允许的例外：语义色（success/warning/error/info）必须通过语义通道表达，不直接硬编码。

### 4.5 A11y 与状态矩阵（强制）

- 每个关键控件具备并可验证：
  - default / hover / focus / disabled / error / loading
- 关键路径键盘可达：
  - ContextBar 筛选与 Apply/Reset；
  - Tab 切换；
  - Dialog 打开、表单填写、提交、关闭；
  - DataGrid 行菜单操作。

### 4.6 Dark Mode 一致性（强制）

- 深浅主题下必须保证：
  - Primary 按钮白字可读；
  - 分隔线/边框可辨；
  - 空态/错误/禁用语义一致；
  - Focus ring 不丢失。

## 5. 代码落点（增补）

- 页面：`apps/web/src/pages/jobcatalog/JobCatalogPage.tsx`
- 组件：`apps/web/src/components/FilterBar.tsx`、`apps/web/src/components/DataGridPage.tsx`
- 日期控件：`@mui/x-date-pickers` 相关 Provider/适配器（按现有工程基线接入）
- 主题：`apps/web/src/theme/theme.ts`（仅在必要时补 token 映射，不做页面内硬编码）

## 6. 验收清单（实施完成前必须全部勾选）

1. [ ] ContextBar 同时存在 Apply（contained primary）与 Reset（outlined/text）。
2. [ ] `as_of/effective_date` 均为 DatePicker（day 粒度），提交格式 `YYYY-MM-DD`。
3. [ ] DataGrid 行操作采用“更多(⋯)”为默认收纳；显式图标均有 Tooltip/aria-label。
4. [ ] 页面无新增散落 hex 魔法值（主色/语义色来自 Theme/Token）。
5. [ ] 关键交互状态与键盘路径通过手测（含 focus 可见）。
6. [ ] Light/Dark 双主题下视觉语义一致，无不可读文本。

## 7. 门禁与验证（SSOT 引用）

- 命令入口与触发器以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
- 本修订落地通常命中：前端 lint/typecheck/build、文档门禁（`make check doc`）。
