# DEV-PLAN-104：Job Catalog（职位分类）页面 UI 优化方案（信息架构收敛：上下文工具条 + Tabs + DataGrid + Dialog）

**状态**: 已完成（2026-02-16 05:32 UTC）

> 补充修订：与 `DEV-PLAN-002` 的规范对齐细则见 `docs/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md`。  
> 若本文件与 `DEV-PLAN-104A` 存在冲突，以 `DEV-PLAN-104A` 为准（仅限 UI 规范对齐条款）。
> 实施与验证证据见：`docs/dev-records/dev-plan-104-execution-log.md`。

## 1. 背景

当前“职位分类 / Job Catalog”页（`apps/web/src/pages/jobcatalog/JobCatalogPage.tsx`）更像把 API 调试面板直接铺在页面上：读写上下文分散、任务域堆叠、多处 Primary action、读写反馈距离过远，并且大量使用裸 `<table>` 破坏 MUI 一致性与可访问性。

这些问题的本质不在“美观”，而在 **信息架构（IA）/上下文稳定性/操作流** 没有收敛，导致：

- 用户滚动后无法确认“当前正在查看哪个 as_of、哪个 package”，从而产生“写入到错误生效日/错误包”的风险。
- 页面缺乏主线：连续多个表单都在争夺“主操作”，认知负担高。
- 提交后需要滚动很远才能验证结果，成功/失败/生效日的验证成本高。
- Group / Family / Profile 的层级关系没有被布局表达，用户只能靠脑补浏览路径。

本计划在不改变业务/接口契约的前提下，给出一个可直接实现的 MUI X 页面收敛方案，对齐仓库 UI 基线（`DEV-PLAN-002`、`DEV-PLAN-103`）与时间上下文约束（`DEV-PLAN-032`、`DEV-PLAN-102`）。  
`DEV-PLAN-104A` 进一步冻结了 002 对齐项（Apply+Reset、DatePicker、行级 More 菜单、Token/A11y/Dark Mode），本计划实施时需一并满足。

## 2. 目标（DoD）

1. [X] 页面“全局上下文”稳定且始终可见：`as_of + package_code + owner_setid + read_only` 在滚动时不会丢失。
2. [X] 读视图与写入生效日的关系可解释且不易误用：
   - `as_of` 仅代表“查看口径”；
   - 每次写操作必须显式选择 `effective_date`（默认=当前 `as_of`），并在提交按钮附近再次展示“将对 YYYY-MM-DD 生效”。
3. [X] 任务域收敛：任一局部任务域（每个 Tab）最多 1 个 Primary action（对齐 `DEV-PLAN-002` 的意图）。
4. [X] 读写反馈距离缩短：提交成功后，用户无需滚动即可在当前 Tab 的列表中验证结果（新增/更新行可被定位或高亮）。
5. [X] 统一组件选型：移除裸 `<table>`，列表统一使用 `DataGridPage`（MUI X DataGrid），表单统一用 `Dialog`/`Drawer`（本计划默认 `Dialog`）。
6. [X] 响应式稳健：小屏不挤压/溢出，表单控件 `fullWidth`，布局在 `xs` 下自动换行。

## 3. 非目标（明确不做）

- 不改变后端 API 形状与业务语义（`getJobCatalog/applyJobCatalogAction/listOwnedJobCatalogPackages` 保持不变；业务合同以 `DEV-PLAN-029` 为准）。
- 不新增/删减 Job Catalog 的业务实体与字段，不引入新的写动作（仅重排、改组件、改交互承载）。
- 不引入“legacy/兼容兜底/双链路”回退通道（对齐 No Legacy）。
- 不在本计划内实现复杂的树形编辑、批量导入/导出、权限模型调整等扩展能力。

## 4. 路由与 URL 协议（冻结）

- 页面路由：`/app/jobcatalog`（router path：`jobcatalog`）
- Query（冻结）：
  - `as_of=YYYY-MM-DD`：读视图时间上下文（day 粒度；缺省=当天 UTC）。
  - `package_code=...`：所选 package（缺省为空表示“未选择”）。
  - `tab=groups|families|levels|profiles`（可选）：当前 Tab；缺省 `groups`。
  - `group_code=...`（可选）：当 Tab=Families/Profiles 时用于右侧列表过滤的“当前选中 group”。

> 说明：本计划不引入新的“写入口默认日期”全局字段；`effective_date` 只存在于每次写操作的表单对话框内。

## 5. 页面信息架构与组件级布局（MUI X）

### 5.1 页面骨架（从“卡片堆叠”收敛为“上下文 + 工作区”）

页面结构（自上而下）：

1) `PageHeader`
- title：`职位分类`（i18n：`nav_jobcatalog` 或新增页面 title key）
- subtitle：弱化“API 调试”表述，改为“在所选 Package 下维护职位族/族群/等级/画像（按生效日）”

2) `JobCatalogContextBar`（建议 sticky，作为全页 SoT）
- 形态：`Paper variant="outlined"` + 内部 `Stack`（对齐 `FilterBar` 风格）
- sticky 建议：`sx={{ position: 'sticky', top: { xs: 56, sm: 64 }, zIndex: 1, bgcolor: 'background.paper' }}`
- 左侧（读上下文）：
  - `as_of`：`TextField type="date" fullWidth`
  - `package`：优先 `Autocomplete`（options=ownedPackages；显示 `package_code + owner_setid + status`），兼容手输 `package_code`
  - `Load/Apply`：唯一 Primary button（全页级别的“应用上下文”按钮）
- 右侧（只读摘要）：
  - `Owner SetID`：只读文本或 `Chip`
  - `Read-only`：`Chip`（read_only=true 时显示；同时禁用所有写入口）
  - （可选）`Selection`：把原“Selection 卡片”合并到这里，减少重复

3) `Tabs`（主工作区）
- Tabs：`Groups / Families / Levels / Profiles`
- 每个 Tab 内部：`TabToolbar + DataGrid`
  - `TabToolbar`：
    - 左侧：标题 + 统计（例如 `共 N 条`）+ 二级筛选（keyword/status）
    - 右侧：仅 1 个 Primary action：`Create ...`（打开对话框）
  - `DataGridPage`：列表区（尽量占据首屏），行内操作用 `variant="text"`（或集中到行内 Menu，但不产生多个 contained）

### 5.2 层级数据的布局表达（两栏）

针对强层级的 `Group -> Family -> Profile`，在不引入复杂树控件的前提下，用“两栏”承载浏览路径：

- Tab=Groups：单列 `GroupsGrid`（可选中某行，写回 `group_code` query，供其他 Tab 复用过滤）
- Tab=Families：两栏布局
  - 左：`GroupsGrid (compact)`（可搜索/选择 group）
  - 右：`FamiliesGrid`（自动按选中 group 过滤；无 group 时展示提示“请选择一个 Group”）
- Tab=Profiles：两栏布局
  - 左：`GroupsGrid (compact)`（同上）
  - 右：`ProfilesGrid`（默认按选中 group 过滤其 Families 的 profiles；MVP 可先做“按 family_code contains”客户端过滤）

> MVP 优先级：先实现 Groups/Families 的两栏；Profiles 两栏可作为后续增强（但至少提供 `family_codes_csv` 的可筛选入口）。

### 5.3 列表列定义（建议）

统一列：`code`、`name`、`effective_day`（只读）、`is_active`（Chip，若后端返回）、`actions`

- GroupsGrid：
  - code / name / effective_day / actions（View、Create Family（跳到 Families Tab 并选中该 group））
- FamiliesGrid：
  - code / name / group_code / effective_day / actions（Move（reparent）、View）
- LevelsGrid：
  - code / name / effective_day / actions（View）
- ProfilesGrid：
  - code / name / primary_family_code / family_codes_csv / effective_day / actions（View）

## 6. 写操作承载（Dialog；每次写都绑定 effective_date）

### 6.1 通用规则（冻结）

- 所有写操作入口只出现在各 Tab 的 `Create` 按钮或行内 actions 中，不再在页面中堆叠多个表单卡片。
- 每个写对话框必须包含：
  - `effective_date`（`TextField type="date"`；默认值=当前 `as_of`）
  - 提交按钮附近的二次确认文案：`将对 YYYY-MM-DD 生效`
  - `package_code` 只读展示（来自 ContextBar）
  - `read_only=true` 时禁用提交并提示“当前包只读”
- 提交成功后：
  - 关闭对话框
  - 刷新 catalog query
  - 尽量把用户视线带回列表变化处（例如 set selection / 高亮最近变更 code）

### 6.2 对话框清单（与现有 actions 对齐）

- Create Group：`create_job_family_group`
- Create Family：`create_job_family`（包含 `group_code`，默认取当前选中 group）
- Move Family（现 Update Job Family Group 表单）：`update_job_family_group`（对话框文案应改为“调整 Job Family 归属 Group”，避免 action 名误导）
- Create Level：`create_job_level`
- Create Profile：`create_job_profile`（family 选择优先做 `Autocomplete multiple`；MVP 可先保留 CSV 输入但需加校验与示例）

## 7. 权限与失败模式（fail-closed）

- 页面 route 维持：`RequirePermission permissionKey='jobcatalog.read'`
- 写入口的可用性：
  - `catalog.view.read_only=true`：禁用所有写操作（按钮 disabled + 提示）
  - 后端 403/网络错误：在对话框内显示 `Alert`，保留用户输入；页面顶部可保留全局错误条

## 8. i18n 与文案（仅 en/zh）

- 现有硬编码英文文案应迁移到 i18n keys（对齐全站做法；`DEV-PLAN-020`）。
- MVP 只要求覆盖：PageHeader、ContextBar（as_of/package/load/owner/read-only）、Tabs label、Create/Move/View、错误/空态文案。

## 9. 验收标准（最小可交付）

1. [X] 滚动到任意位置时，仍能看见并修改 `as_of/package` 上下文；应用后刷新列表且上下文一致可解释。
2. [X] 页面不再出现 5 个并列 contained 主按钮；每个 Tab 仅 1 个 Primary action。
3. [X] 创建/移动等写操作完成后，用户无需滚动即可在当前 Tab 的 DataGrid 中验证结果。
4. [X] 页面不再使用裸 `<table>`；列表均为 `DataGridPage`，并具有一致的密度/边框/可访问性。
5. [X] 小屏（xs）下：ContextBar 与 TabToolbar 不溢出；输入框 `fullWidth`，按钮自动换行。

## 10. 实施步骤（建议拆 PR）

1. [X] P0：引入 `JobCatalogContextBar`（合并 Load + Selection + Owned Packages 选择入口），并把 `Action Effective Date` 从页面移除（后续迁入对话框）。
2. [X] P1：引入 Tabs + 将 4 张列表表格替换为 `DataGridPage`（先不做两栏也可，但需为两栏预留结构）。
3. [X] P2：把所有 Create/Update 表单迁移为 `Dialog`（每次写带 `effective_date`）。
4. [X] P3：实现 Groups/Families 两栏（选中 group 影响 families 列表；写回 query params）。
5. [X] P4：补齐 i18n keys 与基础可访问性（aria labels、helper text）。

## 11. 代码落点（建议）

- 页面：`apps/web/src/pages/jobcatalog/JobCatalogPage.tsx`（重构为“ContextBar + Tabs + 子组件”）
- 子组件（建议新建）：
  - `apps/web/src/pages/jobcatalog/components/JobCatalogContextBar.tsx`
  - `apps/web/src/pages/jobcatalog/components/JobCatalogGroupsTab.tsx` 等（按需）
- 公共组件复用：`apps/web/src/components/FilterBar.tsx`、`apps/web/src/components/DataGridPage.tsx`、`apps/web/src/components/PageHeader.tsx`
- i18n：`apps/web/src/i18n/messages.ts`

## 12. 门禁与验证（SSOT 引用）

- 文档/门禁入口以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
- 本计划实现将命中：前端 lint/typecheck/build、路由门禁（若调整 query/路径需保持 allowlist）、文档门禁（本文件）。
- 与 `DEV-PLAN-002` 的对齐验收以 `DEV-PLAN-104A` 为准，评审时需联合检查 `104 + 104A`。
