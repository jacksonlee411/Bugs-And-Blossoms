# DEV-PLAN-002：UI 设计规范（React + MUI Core + MUI X / Material Design Web）

**状态**: 草拟中（2026-02-15 14:03 UTC）

## 背景
本仓库前端已收敛为 React SPA，并以 MUI（Material UI）为组件与设计实现基础。需要一份可执行的 UI 设计规范，作为“设计 → 实现 → 验收”的共同契约，减少页面间视觉与交互漂移。

## 目标
- 明确前端 UI 主框架：**React + MUI Core + MUI X**。
- 设计语言遵循 **Google Material Design**，以 **Web** 场景为主。
- 固化全局视觉 Token（主色、字体、间距、圆角等）与关键组件使用规范（按钮、表单、表格、筛选区、对话框等）。
- 明确主操作（Primary action）口径（例如 Create / 应用筛选）与按钮层级规则，保证一致性与可访问性。

## 非目标
- 不引入第二套组件库或自研 Design System（除非后续另立 dev-plan 并评审）。
- 不覆盖业务信息架构（IA）与页面内容策略（由各业务 dev-plan 负责）。

## 设计原则（Material Design / Web）
1. 一致性优先：尽量使用 MUI 组件默认行为 + Theme 覆盖，不做零散的局部“手搓样式”。
2. 语义优先：颜色、按钮样式、图标用于表达语义（主/次/危险/禁用/加载），避免“仅为好看”而破坏可用性。
3. 可访问性默认开启：键盘可达、焦点可见、对比度合规、表单错误可读。

## 技术栈与实现边界（SSOT）
- 前端主框架：React。
- UI 组件：MUI Core（@mui/material）。
- 高级组件：MUI X（例如 Data Grid、Date Pickers 等）。
- Theme 入口（实现参考，非唯一）：`apps/web-mui/src/theme/theme.ts`。

## 设计 Token（全局）
> 目标：Token 先于页面样式。页面不直接写“魔法值”，优先从 Theme / CSS 变量取值。

### 主色（Primary）
- 项目 UI 主色（主题色）：**丘比蓝** `#09a7a3`。
- 承载方式：优先以全局 CSS 变量承载，并在 MUI Theme 中引用（例如 `palette.primary.main` 绑定到该变量）。

建议约定（如无既有约定可采用）：
- `--bb-color-primary: #09a7a3;`
- `--bb-color-on-primary: #ffffff;`（主操作按钮文字色）

### 字体（Typography）
- Web 首选字体栈：`Inter, system-ui, -apple-system, Segoe UI, Roboto, sans-serif`。
- 字重与字号：遵循 MUI Typography 默认层级（h1-h6/body1/body2/caption/button），必要时通过 Theme 统一调整，禁止页面内随意定义“新层级”。

### 间距（Spacing）
- 以 8px 作为基础网格（Material 推荐），优先使用 MUI spacing（例如 1=8px, 2=16px...）。
- 页面与区块的 padding/gap 应来自 spacing 体系，避免出现 7px/13px 等“不可复用”值。

### 圆角（Shape）
- 默认圆角：10px（如需调整，以 Theme 的 `shape.borderRadius` 为准）。
- 组件圆角不在页面单独覆盖（除非是组件级 styleOverride）。

### 阴影（Elevation）
- 使用 MUI elevation（shadow）体系表达层级，不使用随意 box-shadow。
- AppBar/固定导航等按需去阴影但要保持分隔线一致（例如 border-bottom）。

## 颜色语义与状态
### 中性色
- 背景（default/paper）、边框、分割线、文字主/次色由 Theme 统一定义；页面不直接写随机灰度。

### 语义色（Success/Warning/Error/Info）
- 用于状态提示、表单校验、toast/snackbar、徽标（chip/badge）等。
- 禁止用 Primary 去表达“错误/危险”。

### 禁用与只读
- 禁用（disabled）：不可点击 + 低对比呈现，仍需可读。
- 只读（read-only）：可聚焦/可复制（如适用），视觉上与 disabled 区分（避免误解为不可用）。

## 按钮规范（Actions）
### 按钮层级
- Primary button（主操作按钮 / 主操作）：每个“局部任务域”同一屏最多 1 个（例如表单提交区、列表工具栏）。
- Secondary button（次操作）：与主操作并列但弱化（例如“取消”“重置”“导出”）。
- Tertiary/Text button（三级/文本操作）：用于低风险辅助动作（例如“查看详情”“复制链接”）。
- Destructive（危险操作）：删除/停用等，使用 error 语义与明确文案（必要时二次确认）。

### 主操作按钮（Primary button / Primary action）
- 典型：**Create/新建**、**应用筛选/应用**、**保存/提交**、**确认**。
- 视觉：优先使用 `variant="contained"` + `color="primary"`。
- **字体颜色必须为白色**（`#ffffff`）。建议通过 `palette.primary.contrastText` / `--bb-color-on-primary` 统一保证。

### 文案与图标
- 文案动词优先：新建/保存/应用/提交/确认。
- 若搭配图标，优先使用 Material Icons（与 MUI 生态一致），并保持同一位置规则（一般左侧图标）。

## 表单与筛选（Forms & Filters）
- 字段对齐：标签、输入框、帮助文本、错误信息使用统一布局组件（例如 MUI Grid/Stack/FormControl）。
- 校验：错误信息要可读、可定位；避免仅用红色边框表达错误。
- 筛选区：
  - “应用筛选”作为 Primary action；
  - “重置/清除”作为 Secondary action（通常 outlined 或 text）。

## 页面控件与元素规范（面向专业 HR / 桌面 Web）
> 目标：专业 HR 的高频场景以“数据密集 + 低认知负担 + 键盘友好”为优先级；控件选择以 MUI Core/MUI X 为准，避免自定义控件导致交互不一致。

### 信息密度（Information Density）结论（本项目建议）
- 默认（表单基线）：`Standard`（标准密度）——作为全站表单的默认档位，兼顾现代观感与填写效率。
- 关键编辑/高风险提交表单：`Comfortable`（舒适密度）——增加组间留白与字段间距，降低误填/漏填风险。
- 高频筛选条/批量录入等效率场景：`Compact`（紧凑密度）——提升单位屏信息量，但不建议整页编辑表单全量紧凑化。
- 数据密集区（表格/列表）：可使用 `Compact`（与 MUI X Data Grid 的 density 口径一致）；页面其余区域保持 `Standard/Comfortable`，避免“整页过密”导致疲劳与误操作。

### 基础要求（适用于所有控件）
- 默认使用 MUI 组件的可访问性能力（label/aria/焦点态），禁止移除 focus ring。
- 点击/可交互目标尺寸：桌面 Web 仍需可用，建议不小于 32px 高（按钮/输入/图标按钮容器）。
- 状态必须齐全：default/hover/focus/disabled/error/loading（loading 需禁用重复提交）。

### 文本与输入（TextField / 输入框）
- 组件：优先 `TextField`（`variant="outlined"`），保持企业后台风格与可读性。
- Label 必填：使用 `label` 作为字段语义来源；placeholder 仅用于示例（不可替代 label）。
- 帮助与错误：使用 `helperText`；错误态必须给出可行动的提示（如“请输入 8 位编号”）。
- 只读与禁用：
  - read-only：允许选中/复制（如适用），并提供清晰视觉（不要与 disabled 混淆）。
  - disabled：不可交互，但信息仍需可读。

### 数字/代码类输入（Code-like）
- 适用：组织编码、人员号（Pernr）、岗位编号等。
- 约束要前置：长度/字符集/前导零等在 UI 上显式（mask/提示文案/校验）。
- 展示建议：只在“纯代码”展示区域使用等宽字体（monospace）；输入框本身保持与全局字体一致，避免跳变。

### 选择类控件（Select / Autocomplete）
- 小集合（可完整枚举）：`Select`/`MenuItem`。
- 大集合（可搜索）：`Autocomplete`（支持输入搜索、清空、无结果提示）。
- 选项文案：使用“人能读懂”的显示名；必要时在次要位置展示 code（例如 `Name (CODE)`）。

### 日期与有效期（MUI X Date Picker）
- 组件：优先使用 MUI X DatePicker/DateRangePicker（如需要范围）。
- 粒度：业务有效期遵循“日粒度”（参见 `docs/dev-plans/032-effective-date-day-granularity.md`）；UI 显示与输入格式建议统一为 `YYYY-MM-DD`。
- 禁止在“有效期”场景引入时分秒输入；时间戳仅用于审计时间展示（如 created_at/updated_at）。

### 多行文本（Textarea）
- 使用 `TextField multiline`，并给出合理的最小/最大行数（避免无边界高度）。
- 长文本仅在确有业务必要时使用；优先结构化字段（可检索、可比较）。

### 勾选/单选/开关（Checkbox / Radio / Switch）
- 二元布尔：优先 Checkbox（表单）或 Switch（设置面板），同一页面不要混用两种表达同一语义。
- 单选互斥：RadioGroup；选项不超过 5 个为佳，否则考虑 Select。
- 必须配套文字标签；仅图形（无文字）禁止用于关键业务决策。

### 按钮（Button / IconButton）
- 主操作：`Button variant="contained" color="primary"`，且文字为白色（本规范已约束）。
- 次操作：`outlined` 或 `text`；“取消/关闭”一般为 text，“重置/清除”一般为 outlined。
- 图标按钮：`IconButton` 必须有 Tooltip（或 `aria-label`），避免“只靠图标猜含义”。
- 危险操作：放在主操作之外（通常右侧菜单或二次确认对话框中），避免误触。

### 链接（Link）
- 页面内导航使用 `Link`（视觉上可识别为可点击文本）；避免用普通文本伪装链接。
- 链接不承载“破坏性操作”（删除/停用），这些必须用按钮+确认。

### 筛选区与列表工具栏（Filter Bar / Toolbar）
- 筛选区建议结构：筛选字段（左） + 操作按钮（右：应用 Primary、重置 Secondary）。
- 列表工具栏建议结构：标题/统计（左） + 主操作 Create（右） + 其他操作（overflow menu）。

### 数据表格（MUI X Data Grid）
- 默认面向桌面密集使用：优先 `density="standard"` 或 `compact`（由页面统一决定，不要同模块混用）。
- 列对齐：数字右对齐、日期按统一格式显示、状态用 Chip/Badge。
- 行级操作：优先“更多(⋯)”菜单收纳长尾动作；关键动作可保留 1 个显式 icon（带 Tooltip）。
- 空/加载/错误：必须有明确占位与重试入口。

### 分页与批量操作（Pagination / Bulk Actions）
- 分页：使用 Data Grid 自带分页或 MUI Pagination，位置保持一致（表格底部右侧常见）。
- 批量操作：仅在用户明确选择后展示批量工具栏，并提供“取消选择/清除选择”。

## 表格与数据展示（Data Display）
- 列表/表格优先使用 MUI X Data Grid（如满足需求），避免自绘表格导致交互不一致。
- 空状态：必须有明确说明与下一步引导（例如“调整筛选条件”“新建一条记录”）。
- 加载状态：使用 skeleton/progress，避免页面抖动。

## 对话框与抽屉（Dialogs / Drawers）
- 仅在需要强制用户决策或编辑时使用对话框；信息展示优先在页面内完成。
- 危险操作必须二次确认，且按钮文案明确（“删除”“停用”而不是“确定”）。

## 导航与页面结构（Web）
- 以 App Shell（顶部/侧边导航 + 内容区）为主；页面标题、面包屑、工具栏位置保持一致。
- 主要动作（Primary action）的位置保持一致（例如列表页工具栏右侧）。

## 响应式与断点（Responsive）
- 断点遵循 MUI 默认 breakpoints（xs/sm/md/lg/xl），页面布局以 Web 为主，避免为移动端强行改写交互。
- 数据密集页面（表格/筛选/双栏详情）以 `md/lg` 为主要目标断点；在 `sm/xs` 下可退化为纵向堆叠，但不牺牲关键动作可达性。

## 主题模式（Light / Dark）
- 支持浅色/深色模式时，必须保证：主操作按钮可读（白字）、边界/分隔线可见、空/错/禁用状态在两种模式下语义一致。
- 颜色不以“手写 hex”方式在页面适配深色模式，应通过 Theme Token 统一分发。

## 反馈与通知（Feedback）
- 成功/失败/警告：优先使用 MUI Alert/Snackbar，文案“结果 + 下一步建议”。
- 进行中：长任务显示进度或可取消；短任务显示加载态并防止重复提交。

## 可访问性（A11y）
- 键盘可达：tab 顺序合理，交互组件可用键盘操作。
- 焦点可见：不要移除 focus ring；若主题化，必须保持可见对比。
- 对比度：文本与背景对比需满足 WCAG AA（以实际主题色计算为准）。

## i18n 与文案
- 仅支持 `en/zh`（参见 `docs/dev-plans/020-i18n-en-zh-only.md`）。
- 按钮与提示文案避免硬编码到组件内部，遵循项目既有 i18n 写入口。

## 验收清单（设计/实现自检）
1. [ ] 页面仅使用 React + MUI Core + MUI X 组件体系（无第二套 UI 库）。
2. [ ] 主色统一为 `#09a7a3`，且由全局变量/Theme Token 承载（无散落魔法值）。
3. [ ] Primary button 的文字色为 `#ffffff`，且主操作层级数量与位置符合本规范。
4. [ ] 控件（输入/选择/日期/表格/筛选区）遵循本规范：label 完整、状态齐全、键盘可达。
5. [ ] 表单错误、禁用/只读、空状态、加载状态均有一致实现。
6. [ ] 通过键盘可完成主要路径操作，焦点可见且不丢失。
