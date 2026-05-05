# DEV-PLAN-493：OrgUnit 组织架构页搜索与筛选入口唯一性收敛方案

**状态**: 规划中（2026-05-05 CST，已按 003/No-Legacy/API-CubeBox 评审意见修订为 T2；本次仅修订方案文档，暂不实施代码）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：收敛 `组织架构` 页面同时暴露三类搜索/筛选输入导致的用户心智分叉、URL 状态分叉与 `DEV-PLAN-003` 唯一性风险；以既有受保护 API `GET /org/api/org-units` 的显式 list scope/range 参数补齐页面范围列表契约；不重做 OrgUnit read core，不新增后端 route，不接管 491 selector 选择场景，不在首期扩张 CubeBox `orgunit.list` 工具参数。
- **关联模块/目录**：`apps/web/src/pages/org/OrgUnitsPage.tsx`、`apps/web/src/api/orgUnits.ts`、`apps/web/src/components/**`、`internal/server/orgunit_api.go`、`internal/server/orgunit_read_service_adapter.go`、`internal/server/cubebox_api_tools.go`（仅做不扩张校验）、`modules/orgunit/services/orgunit_read_service.go`、`modules/orgunit/presentation/cubebox/**`（仅做不扩张校验）、`docs/dev-plans/491-*`、`docs/dev-plans/492-*`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-002`、`DEV-PLAN-003`、`DEV-PLAN-004M1`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-489`、`DEV-PLAN-490`、`DEV-PLAN-491`、`DEV-PLAN-492`
- **用户入口/触点**：`组织架构` 页面顶部筛选栏、树内搜索栏、URL query 参数、树与列表联动、既有 MUI X DataGrid 工具栏。

### 0.1 Simple > Easy 三问

1. **边界**：493 拥有组织架构页的搜索/筛选信息架构、用户入口布局、URL 状态归属、页面级交互契约，以及既有 `/org/api/org-units` list 查询的“当前组织范围 + 是否包含各级下级”显式参数契约；492 继续拥有 OrgUnit ReadService 读取事实源；491 拥有“选择组织”的 selector/facade，不接管组织架构页浏览主 UI；490/CubeBox API tool overlay 首期不新增 `include_descendants`。
2. **不变量**：同一页面内，“搜索组织”只能有一个用户可理解的主入口；树选择表示当前浏览范围；`包含下级` 是范围 list 的后端契约，不由前端拼树；扩展字段筛选是二级筛选，不作为第二个主搜索入口；未显式传 `include_descendants` 时不得改变既有 `parent_org_code` 直接子节点语义。
3. **可解释**：用户先选择当前浏览范围，再使用一个主搜索入口完成“过滤/定位组织”；列表始终以当前选中组织为锚点；扩展字段筛选进入二级区域；树定位不再作为第三个独立搜索框占据页面主层级；CubeBox 仍按 490 现有工具参数执行，不把页面新增开关偷偷变成 AI 工具能力。

### 0.2 现状研究摘要

- 当前 `OrgUnitsPage` 同时维护 `keywordInput`、`extFilterValueInput`、`treeSearchInput` 三组输入状态，分别对应列表 keyword、扩展字段筛选值、树搜索定位。
- 顶部“搜索组织”最终写入 `q`，参与 `listOrgUnitsPage()` 的 grid/list 查询。
- 中部“扩展筛选值”写入 `ext_filter_field_key/ext_filter_value`，同样参与 grid/list 查询。
- 底部“树内搜索（编码/名称）”调用 `searchOrgUnit()`，通过 `path_org_codes` 展开树并写入 `node`。
- 这三组输入在视觉上都是“搜索/筛选”，但行为分别是“过滤列表”“过滤扩展字段”“定位树节点”。当前页面没有一个权威入口说明三者关系，违反 `DEV-PLAN-003` 中“同一概念只有一种权威表达”的结构原则。
- `DEV-PLAN-491` 已解决“选择组织”场景的 selector 唯一主链，但明确不接管组织管理页浏览/编辑主 UI。
- `DEV-PLAN-492` 已纳入组织管理页读取规则向 ReadService 收敛，但它不是页面级搜索/筛选交互 owner。
- 本次不沿用的容易做法：
  - 只改文案，把三个输入继续平铺。
  - 新增第四个“统一搜索”但保留旧三个入口。
  - 把组织架构页整体改造成 `OrgUnitTreeSelector`。
  - 在前端重新实现组织读取、权限裁剪或本地整树缓存。

### 0.3 评审发现问题的处理口径

1. **包含下级必须有后端 list scope/range 契约，且不得破坏既有 direct-children 语义**：493 采用既有 `/org/api/org-units` list 查询扩展，不新增 route、不新增表、不在前端拼全树。只有显式传入 `include_descendants` 的 `mode=grid`/分页列表读取才进入“范围列表”语义：先通过 492 ReadService resolve 当前范围锚点，再按当前 principal scope fail-closed 读取可见结果。`include_descendants=false` 表示只返回当前锚点组织本身；`include_descendants=true` 表示返回当前锚点组织本身及全部各级可见下级，锚点固定在第一页第一行，筛选/排序/分页只作用于锚点之后的下级结果。未传 `include_descendants` 时，既有 `parent_org_code` 继续表示直接子节点读取，不得被静默解释成 `false`。
2. **扩展字段筛选不做首屏平铺，也不能变成僵尸能力**：扩展字段筛选继续保留为管理员可见的二级筛选区；默认收起，存在 `ext_filter_*` 或 `ext:` sort deep link 时自动展开并显示“已应用扩展筛选/排序”的业务提示与清除动作。不得新增表格右上角自定义“高级筛选”按钮；表格右上角仍只使用当前 MUI X DataGrid 已实现工具栏。
3. **一个主搜索入口承接列表 keyword 与树定位，但交互分阶段**：同一个“搜索组织”输入停止输入后防抖写入 `q`，只刷新列表 keyword；Enter 或搜索图标触发定位。定位命中唯一组织时写入 `node` 并展开树；命中多候选时展示可选择候选，不随机选择第一个；用户选择候选后写入 `node`，并保留 `q` 作为列表 keyword。无结果或无权按现有错误映射显示明确提示。
4. **生效日期保留 current-mode deep link 语义**：首屏展示“生效日期”日期框，默认显示当天日期；但未显式修改日期时 URL 不写 `as_of`，仍表示动态 current view。用户修改日期后写入 `as_of=YYYY-MM-DD`，链接按固定 valid-time day 复现。切回当前视图时删除 `as_of`。
5. **页面旧 `status` deep link 是一次性迁移 shim，不是 legacy 第二入口**：首屏删除“组织状态”下拉，只保留“包含无效”开关。旧页面链接中的 `status=active/inactive/disabled` 可以在加载时继续被解析并向后端传递一次，页面用业务语言提示“已应用旧状态筛选”并提供清除动作；用户修改任一主筛选、范围开关、日期或主搜索时必须清理页面 URL 中的 `status`。新增页面链接、导航、测试 fixture 和 E2E 不得再写入 `status`。API 层 `status` 作为既有 list filter 仍可服务非页面调用与 CubeBox 现有工具面，不把它称为旧 API。

## 1. 背景与上下文

截图暴露的问题不是单个输入框文案不清，而是组织架构页在同一首屏把三个相似控件并列暴露：

1. 主筛选栏的“搜索组织”。
2. 高级/扩展字段筛选栏的“扩展筛选值”。
3. 树面板上方的“树内搜索（编码/名称）”。

这些入口都可以让用户输入文本，都可以改变页面结果，但它们改变的对象不同：列表结果、扩展字段条件、树选中节点。用户无法从页面结构直接判断哪个入口是权威搜索，哪一个只是高级筛选，哪一个只是定位工具。

按 `DEV-PLAN-003`，这属于“容易把功能塞进去”的历史叠加：每次新增能力都加一个局部输入框，短期可用，长期使页面状态机和用户心智变复杂。493 的目标是在不扩大后端重构范围的前提下，给该页面冻结一个可执行的收敛方案。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 冻结组织架构页搜索/筛选入口分层：一个主搜索入口、一个范围/层级/有效性开关区、一个二级扩展字段筛选区；树定位不得继续作为第三个平级搜索框。
2. [ ] 明确 URL query 参数归属：`q` 表示主搜索 keyword；`node` 表示当前树/列表范围锚点；`include_descendants` 表示范围列表是否包含全部各级下级；`ext_filter_*` 只表示扩展字段筛选；不同入口不得互相偷用语义。
3. [ ] 让树与列表联动可解释：选择树节点改变浏览范围；右侧列表必须包含当前选中组织且默认排第一；`包含下级` 开启时追加全部各级可见下级组织；列表 keyword 不再与树搜索形成双主入口，并通过防抖自动生效。
4. [ ] 保留 492 ReadService 作为数据读取事实源，不在页面侧补权限、路径或物理树规则。
5. [ ] 防止把实现契约搬进 UI：URL 参数名、`ext_filter_*`、dev-plan 解释和“第二个主搜索”一类治理语言只留在文档/测试中，不作为用户可见文案。
6. [ ] 提供页面级测试与最小 E2E 验收路径，证明入口收敛后用户仍能完成搜索、定位、筛选和查看详情。

### 2.2 非目标

1. 不重做 OrgUnit 后端 read core；后端 roots/children/search/list 继续由 492 承接。
2. 不把组织架构页主 UI 改造成 491 selector；491 只用于“选择组织”的表单/弹窗场景。
3. 不新增后端 route、搜索引擎、全量树缓存或前端权限裁剪。
4. 不删除扩展字段筛选能力；仅调整它的展示层级和入口语义。
5. 不改变 OrgUnit 写路径、字段配置规则、审计或历史查看语义。
6. 首期不把 `include_descendants` 加入 CubeBox `orgunit.list` API tool overlay、knowledge pack 或 planner prompt；若后续需要模型可控范围深度，必须另起计划或修订 490/493，并同步 484/485 覆盖事实、authz 目录与专项门禁。
7. 不改变未显式传 `include_descendants` 的既有 API 调用语义；尤其是 `parent_org_code` 默认仍是直接子节点读取，避免把页面新交互变成全仓隐式行为变更。

### 2.3 用户可见性交付

- **用户可见入口**：`组织架构` 页面。
- **最小可操作闭环**：
  1. 用户打开组织架构页，首屏只看到一个主搜索输入和必要的包含下级/包含无效/生效日期筛选；生效日期默认显示当天日期但未手工修改时不写 `as_of`；首屏不需要“应用”按钮。
  2. 用户输入编码或名称后，列表按 keyword 防抖刷新；用户按 Enter 或点击搜索图标时，页面按同一输入定位组织。唯一命中时树选中节点与列表范围同步更新；多候选时用户从候选列表选择一个再定位。
  3. 用户修改包含下级、包含无效或生效日期后，页面自动刷新列表和 URL；`包含下级` 的结果由后端 list scope/range 契约返回，不由前端拼树。
  4. 用户需要扩展字段筛选时，可展开管理员可见的二级扩展筛选区；旧 `ext_filter_*` deep link 自动展开该区域。用户需要列表工具时，继续使用项目当前已实现的 MUI X DataGrid 右上角工具栏；本计划不新增自定义“高级筛选”或额外“列设置”入口。
  5. 用户可通过页面链接复现同一搜索/筛选/选中节点状态；界面不直接暴露 `q/node/ext_filter_*` 等内部参数名。
  6. 用户点击列表行进入详情，保留当前阅读口径。

## 2.4 工具链与门禁

- **命中触发器**：
  - [ ] Go 代码（实现 list scope/range 契约时）
  - [ ] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`，如新增/调整文案）
  - [ ] 受保护 API 契约（`GET /org/api/org-units` query 语义扩展）
  - [ ] Authz/API 授权目录覆盖事实校验（确认未新增 route、未新增 capability，且现有 route 覆盖仍一致）
  - [ ] CubeBox API-first 反回流校验（确认 `include_descendants` 未进入 `orgunit.list` tool schema/knowledge pack，或后续修订时按 490 同步）
  - [ ] E2E（若调整关键用户流程）
  - [X] 文档

实际命令入口以 `AGENTS.md`、`Makefile` 与 CI 为准；本文只记录触发器，不复制命令矩阵。实施阶段至少需要在验证记录中列出：Go 命中包测试、前端检查/测试、`make authz-pack && make authz-test && make authz-lint`、`make check cubebox-api-first`、`make check no-legacy`、`make check routing`、`make check doc`；如调整 i18n 或 E2E，再补对应门禁。

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `apps/web/src/pages` | URL 状态解析、搜索/筛选入口联动、主搜索定位行为 | `OrgUnitsPage.test.tsx` | 页面级测试覆盖入口收敛后的关键状态变化 |
| `apps/web/src/components` | 如调整既有 DataGrid 工具栏适配，覆盖组件状态 | 新增或既有组件测试 | 只测组件自身交互，不重复测页面查询 |
| `apps/web/src/api` | 不新增 API；必要时只补类型或 helper 测试 | `orgUnits.ts` 相关测试 | 不在 API client 中增加页面语义 |
| `modules/orgunit/services` | range list 的锚点、下级范围、scope fail-closed、锚点置顶不变量 | `orgunit_read_service*_test.go` | 业务规则唯一承载点；避免 handler/store 各自实现 |
| `internal/server` | query parser、错误映射、API contract、CubeBox tool schema 不扩张 | `orgunit_*_test.go`、`cubebox_*_test.go` | 不重复测树业务规则，只测协议边界 |
| `E2E` | 用户打开页面、搜索定位、修改轻量筛选、打开详情 | `e2e/tests/**` | 只做用户路径验收 |

## 3. 架构与关键决策

### 3.1 目标交互模型

```mermaid
flowchart LR
  U[用户] --> S[主搜索入口]
  U --> T[树选择范围]
  S --> URL[q + node]
  T --> URL[node]
  URL --> API[/org/api/org-units + /search]
  API --> RS[492 ReadService]
  RS --> UI[树选中 + 列表结果]
```

主流程：

1. 页面加载 URL 状态，恢复 `node`、`q`、`include_descendants`、包含无效、生效日期与既有列表状态；未提供 `as_of` 时默认使用当天日期展示，但保持 URL 为空以表示动态 current view。
2. 主搜索入口用于组织编码/名称搜索；输入停止后防抖写入 `q` 并刷新列表；按 Enter 或点击搜索图标触发定位。命中唯一组织时调用现有 search contract，展开 safe path，写入 `node`，并保留 `q` 作为列表 keyword；命中多候选时展示候选选择，不随机选第一个。
3. 树点击改变当前范围 `node`；组织架构页列表调用必须显式传 `include_descendants`，使列表范围至少包含当前选中组织本身，并将其固定显示在第一页第一行。
4. `包含下级` 开启时，列表在当前选中组织之后追加全部各级可见下级组织；关闭时只显示当前选中组织本身；切换后立即刷新列表与 URL。该能力必须由后端 list scope/range 契约返回，前端不得通过已加载树、整树缓存或多次 children 请求拼接列表。非页面调用若未传 `include_descendants`，继续沿用既有 direct-children/root/list 语义。
5. `包含无效` 和 `生效日期` 属于轻量筛选，用户修改后自动应用；日期仅在合法 `YYYY-MM-DD` 时写入 `as_of`，避免半截输入触发错误请求；删除/清空日期回到 current view 并删除 `as_of`。
6. 扩展字段筛选保留为管理员可见二级筛选区，默认收起；存在扩展筛选或扩展排序 deep link 时自动展开。列表区右上角沿用项目当前已实现的 MUI X DataGrid toolbar，不新增设计稿专属的高级筛选按钮或额外列设置田字图标。
7. 列表查询继续调用 `listOrgUnitsPage()` 并通过既有 `/org/api/org-units` 扩展参数表达范围；树定位继续调用 `searchOrgUnit()`；二者都消费 492 后端读取事实。

失败路径：

1. 主搜索为空：提示用户输入组织编码或名称。
2. 搜索多候选：显示可点击/可选择候选澄清，不随机选第一个；候选选择后写入 `node` 并保留 `q`。
3. 搜索无权或无结果：按现有错误映射显示明确提示，不回退全租户。
4. 扩展字段筛选参数不可用：清理 `ext_filter_*` 并提示筛选条件已失效。

### 3.2 页面入口分层决策

选定方案：

1. **主搜索入口**：保留在首个筛选区，文案收敛为“搜索组织”“编码 / 名称”或等价业务表达；它是组织编码/名称搜索的唯一文本主入口。
2. **树范围选择**：树面板只保留树、选中态和展开能力；树内搜索框退出首屏平级入口。树节点首屏展示仅显示组织名称，不额外显示组织类型图标或括号编码；编码仍由主搜索和列表“编码”列承载。快捷定位并入主搜索，不再暴露第二个完整文本框。
3. **列表工具栏与扩展筛选**：列表区右上角沿用项目当前已实现的 MUI X DataGrid toolbar；本计划不新增自定义“高级筛选”按钮，也不新增额外“列设置”田字图标。扩展字段筛选保留为管理员可见的二级筛选区，默认收起；`ext_filter_*` / `ext:` sort deep link 自动展开该区域并提供清除动作，避免能力不可发现或变成僵尸功能。
4. **层级/有效性/生效日期筛选**：层级范围由“包含下级”开关承接，且位于“包含无效”左侧；有效性筛选只保留“包含无效”开关；不再并列暴露“组织状态”下拉，避免两个控件同时表达有效/无效范围。页面旧 `status` 参数仅作一次性迁移 shim，不在首屏继续提供入口。历史口径改为用户可理解的“生效日期”日期框，默认显示当天日期；用户未修改时不写 `as_of`，用户修改日期后，页面按该日期观察组织架构的 valid-time 视图。
5. **自动应用**：首屏轻量筛选不保留“应用”按钮。开关和日期合法变更即时更新 URL 与列表；文本搜索防抖自动更新，Enter 立即更新。若后续恢复多字段复杂高级筛选，确认按钮只能出现在高级筛选面板内部，不回到首屏主筛选区。
6. **用户可见文案**：页面文案只使用业务语言解释当前操作，例如“选择树节点查看范围，输入编码或名称定位组织”。禁止在用户界面展示 URL 参数名、内部字段 key 或 dev-plan 解释性语句。

拒绝方案：

1. 三个搜索框都保留，只改 label。
2. 把 `q`、`node`、`ext_filter_value` 合并成一个含糊参数。
3. 为了 UI 收敛新增 selector 专用后端 route。
4. 将高级字段筛选前移为默认主流程，导致普通用户首屏复杂度继续增加。
5. 为 `包含下级` 在前端循环调用 children 或维护整树缓存。

### 3.3 URL 状态契约

| 参数 | Owner | 语义 | 备注 |
| --- | --- | --- | --- |
| `node` | 树/当前浏览范围 | 当前选中组织节点，列表默认展示该节点范围内结果 | 搜索定位成功后同步更新 |
| `q` | 主搜索 / 列表 keyword | 组织编码/名称 keyword | 不承载扩展字段值 |
| `as_of` | 生效日期 | 指定 valid-time day | 日期框默认当天日期；未手工修改时不写 URL；用户修改后按该日期观察组织视图，继续沿用现有 read view 规则 |
| `include_descendants` | 包含下级 | 是否在列表中包含当前选中组织的全部各级下级 | 关闭时只显示当前选中组织本身；开启时当前选中组织仍固定第一行 |
| `include_disabled` | 包含无效 | 是否包含 disabled 节点/记录 | 当前 UI 的唯一有效性筛选入口 |
| `status` | 页面迁移 shim / API 既有过滤 | 页面旧 deep link 的 active/inactive/disabled 过滤；API 层仍是既有 list filter | 不再作为首屏 UI 入口；页面存在时显示已应用旧状态筛选并可清除；新增页面链接不得写入 |
| `ext_filter_field_key` | 高级筛选 | 扩展字段 key | 必须与 `ext_filter_value` 成对出现 |
| `ext_filter_value` | 高级筛选 | 扩展字段筛选值 | 不作为主搜索 |
| `sort/order` | 列表排序 | 列表排序字段与方向 | ext sort 仍使用 `ext:<field>` |

以上 URL 状态契约用于实现、测试和 deep link 兼容，不是 UI 文案来源。用户界面只需要表达“页面链接可恢复当前筛选/范围”这类业务含义，不展示 `q`、`node`、`ext_filter_*` 等参数名。

### 3.4 后端 list scope/range 契约

493 不新增后端 route；在既有 `GET /org/api/org-units` 的 grid/list 语义上扩展一个**显式**范围参数。该参数只改变显式传参请求，不改变未传参的既有 API 语义：

| 参数组合 | 语义 |
| --- | --- |
| 无 `node`/`parent_org_code` 且无 grid/list 条件 | 返回当前 principal scope-aware visible roots，沿用 492 |
| `mode=grid&page=&size=` + `parent_org_code=<code>` + 未传 `include_descendants` | 沿用当前 direct-children list/grid 语义：返回 `<code>` 的直接可见子节点；不返回锚点本身 |
| `mode=grid&page=&size=` + `parent_org_code=<code>` + `include_descendants=false` | 返回 `<code>` 对应组织本身，受当前 principal scope 裁剪 |
| `mode=grid&page=&size=` + `parent_org_code=<code>` + `include_descendants=true` | 返回 `<code>` 对应组织本身及全部各级可见下级，受当前 principal scope 裁剪 |
| `q` / `status` / `ext_filter_*` / `sort/order` 与显式 `include_descendants` 同时存在 | 锚点组织本身始终保留并置顶；其余下级结果在上述范围内过滤、排序、分页；不得扩大当前 principal scope |

实现约束：

1. `include_descendants` 只在 list/grid 查询中表达范围深度；selector roots/children/search 不消费该参数。
2. `parent_org_code` / 后续可选的 `parent_org_node_key` 只有在显式传 `include_descendants` 时才是范围锚点；未传时继续等价于“列直接子节点”。组织架构页新 UI 必须总是显式传 `include_descendants`，避免依赖隐式默认。
3. ReadService 是范围列表业务规则唯一承载点：必须先 resolve 锚点并验证当前 principal 可见；锚点范围外返回现有授权/范围错误，不返回空结果伪装成功。handler 只解析 query，store 只提供 scoped query/page 原语。
4. 显式 `include_descendants=false` 返回锚点本身；显式 `include_descendants=true` 返回锚点本身 + 全部各级可见下级；未传 `include_descendants` 不进入该规则。
5. 锚点固定在第一页第一行，且不被 `q`、旧 `status`、`ext_filter_*` 或排序移除；这些条件只过滤/排序锚点之后的下级结果。
6. 分页与 `total` 以 `锚点 + 过滤排序后的下级结果` 为准；第一页第一行必须是锚点，后续页不重复返回锚点。
7. 不得在前端通过多次 children 请求、已加载树节点或本地整树缓存拼接列表。
8. 首期不得把 `include_descendants` 加入 `internal/server/cubebox_api_tools.go`、`modules/orgunit/presentation/cubebox/apis.md`、`queries.md`、`CUBEBOX-SKILL.md` 或 planner prompt；专项测试应证明 CubeBox `orgunit.list` tool schema 未被页面参数污染。

## 4. 分阶段实施

### 4.1 Phase A：契约冻结与页面状态盘点

1. [ ] 确认 `OrgUnitsPage` 中三类输入状态、URL 参数、API 调用和错误提示的完整映射。
2. [ ] 明确主搜索是否在成功定位后保留 `q`；若保留，必须说明它同时服务列表 keyword 的可见效果。
3. [ ] 明确 `include_descendants` 后端 list scope/range 契约：仅显式传参时启用；锚点本身、全部各级下级、锚点置顶、scope fail-closed、过滤/排序/分页顺序；未传参继续 direct-children/root/list 既有语义。
4. [ ] 明确扩展筛选二级入口、页面旧 `status` 迁移 shim、`as_of` current-mode URL 语义和多候选选择交互。
5. [ ] 明确 CubeBox/API tool 影响面：首期不新增 `include_descendants` 到 `orgunit.list` tool schema；若实现发现必须暴露给 CubeBox，停止本计划实施，先更新 490/493 与 API 授权目录 owner 文档。
6. [ ] 补充页面级测试用例清单，先冻结预期行为再改 UI。

### 4.2 Phase B0：后端范围列表契约

1. [ ] 在 `/org/api/org-units` grid/list query parser 中接入三态 `include_descendants`（未传 / false / true），仅用于 list scope/range，不影响 selector roots/children/search。
2. [ ] 在 `modules/orgunit/services.ReadService.List` 中承接“范围锚点 + 是否包含各级下级”的 list 语义；锚点范围外 fail-closed；未传 `include_descendants` 时继续走既有 direct-children/root/list 语义。
3. [ ] 在 adapter/store 分页链路中提供 ReadService 所需的 scoped range/page 原语；不得把锚点置顶、过滤顺序或 scope fail-closed 业务规则散落到 handler。
4. [ ] 显式 `include_descendants=true/false` 时保证锚点本身进入结果集并固定在第一页第一行；`true` 返回全部各级可见下级，`false` 只返回锚点本身。
5. [ ] 明确过滤/排序/分页顺序：resolve/校验锚点 → scope/range 裁剪下级 → 对下级应用 keyword/status/ext 过滤与排序 → 将锚点置顶 → 分页；验收必须证明锚点不会被任何筛选或分页挤出。
6. [ ] 补 Go 测试覆盖：未传 `include_descendants` 的 direct-children 兼容、关闭包含下级、开启包含下级、多级下级、锚点范围外、keyword 与 ext 筛选、分页 total 与锚点置顶。
7. [ ] 补 CubeBox/API-first 反扩张测试或现有测试断言：`include_descendants` 不出现在 `orgunit.list` tool schema、knowledge pack、planner prompt 允许参数中。

### 4.3 Phase B：UI 收敛

1. [ ] 将树内搜索从第三个平级 `FilterBar` 移除或降级为主搜索的定位行为。
2. [ ] 删除设计稿新增的自定义“高级筛选”按钮；扩展字段筛选改为管理员可见二级筛选区，默认收起，命中 `ext_filter_*` 或 `ext:` sort deep link 时自动展开。
3. [ ] 删除设计稿新增的额外“列设置”田字图标，列表右上角沿用项目当前已实现的 MUI X DataGrid toolbar。
4. [ ] 简化左侧组织树节点：首屏节点只显示组织名称，删除组织类型图标和括号编码，避免和列表“编码”列重复。
5. [ ] 在“包含无效”左侧新增“包含下级”开关；默认开启，用于请求后端返回当前选中组织及全部各级可见下级；关闭时请求后端只返回当前选中组织本身。
6. [ ] 调整列表样例：当前选中组织本身必须出现在右侧列表第一行；开启“包含下级”时，下级组织按既有排序追加在后。
7. [ ] 将“历史视图”抽象入口改为“生效日期”日期框，默认显示当天日期；用户未修改时不写 `as_of`，用户修改日期后按该日期刷新组织视图并写入 `as_of`。
8. [ ] 删除首屏“应用”按钮：包含下级、包含无效、生效日期调整后自动应用；搜索组织输入防抖自动应用并支持 Enter 立即应用。
9. [ ] 删除首屏“组织状态”下拉；页面旧 `status` 参数继续生效时显示“已应用旧状态筛选”业务提示与清除动作；用户修改任一新主筛选、范围开关、日期或主搜索时必须从 URL 清理 `status`。
10. [ ] 主搜索多候选时展示可选候选；选择候选后写入 `node` 并保留 `q`。
11. [ ] 调整文案，区分“搜索组织”“当前范围”和既有列表工具栏，避免三个入口都叫搜索/筛选，同时删除 URL 参数名、内部字段 key 和治理说明式文案。
12. [ ] 清理 DataGrid 内与页面主搜索重复或无实现支撑的新增入口；表格工具栏仅保留当前代码已有能力。
13. [ ] 保持 URL 可复现，不破坏现有 `node/q/ext_filter_*` deep link；新增页面链接和测试 fixture 不再生成 `status`。

### 4.4 Phase C：验证与证据

1. [ ] Go 测试覆盖后端 range list 契约、未传 `include_descendants` direct-children 兼容、CubeBox tool schema 不扩张；页面测试覆盖：主搜索防抖/Enter 生效、多候选选择、树选择范围、包含下级开关、轻量筛选自动应用、页面旧 `status` shim 清理、URL 复现。
2. [ ] 执行并记录 Go、前端、Authz、Routing、No-Legacy、CubeBox API-first 与文档门禁；如新增/调整 i18n key，执行多语言门禁。
3. [ ] 如页面布局或关键流程变化明显，补 E2E smoke。
4. [ ] 更新 readiness/dev-record，记录验证命令与结果。

## 5. 与 491/492 的关系

- **491 已纳入的部分**：组织选择场景的 selector/facade 唯一主链；创建组织和详情编辑上级组织已复用 `OrgUnitTreeField`。
- **491 未纳入的部分**：组织架构页浏览主 UI 的搜索/筛选入口收敛；491 明确不强制接管该页面。
- **492 已纳入的部分**：后端读取事实源、visible roots、children、search、safe path、list/grid 语义向 ReadService 收敛。
- **492 未纳入的部分**：页面首屏三个相似搜索/筛选控件的 UX 唯一性与信息架构。
- **493 owner**：只补 491/492 之间遗漏的页面级入口收敛，不发明新的组织读取事实源。

## 6. 风险与停止线

| 风险 | 表现 | 止损 |
| --- | --- | --- |
| 主搜索语义过载 | 一个输入同时做定位、过滤、扩展字段搜索，用户更困惑 | 保持 `q` 和 `ext_filter_*` 分层；高级字段不并入主搜索 |
| 破坏 deep link | 旧 URL 打开后状态丢失 | 保留现有参数解析，迁移只改变展示层级 |
| 隐式改变 `parent_org_code` 语义 | 未传 `include_descendants` 的现有调用从直接子节点变成锚点/子树范围 | `include_descendants` 使用三态解析；只有显式传参才启用 range list；兼容测试覆盖未传参 direct-children |
| CubeBox 工具面被页面参数污染 | `include_descendants` 被加入 `orgunit.list` overlay/knowledge pack，模型开始依赖页面专用交互参数 | 首期不扩张 CubeBox tool schema；`make check cubebox-api-first` 与定向测试证明未新增参数；如确需扩张先修订 490/493 |
| 绕开 492 read core | 页面为搜索体验新增本地树补丁 | 禁止页面权限裁剪、物理路径修补、整树缓存 |
| ReadService 业务规则重复实现 | handler、adapter、store 各自实现锚点置顶、过滤顺序或 scope 判断 | ReadService 是 range list 业务规则唯一承载点；handler 只解析 query，store 只提供 scoped query/page 原语 |
| 491 selector 误接管主页面 | 为复用把组织架构页改成 picker | 只复用组件素材，不改变组织管理页浏览职责 |
| 只改样式不改心智 | 三个输入仍平铺，只是换位置或换 label | 验收必须检查首屏只有一个主文本搜索入口 |
| 契约语言外泄到 UI | 页面出现 `q/node/ext_filter_*`、dev-plan 说明或“第二个主搜索”等实现/治理文案 | URL 参数只留在代码、测试和文档；UI 用业务语言表达可恢复、当前范围和已应用筛选 |
| 无实现支撑的工具入口 | 设计稿新增“高级筛选”或额外“列设置”图标，但当前代码和本计划不承接实现 | 删除无实现支撑入口；列表右上角沿用当前已实现的 MUI X DataGrid toolbar |
| 组织树信息重复 | 树节点同时显示图标、名称、编码，和列表编码列/主搜索形成重复信息 | 树节点仅显示组织名称；编码用于搜索与列表，不作为树节点首屏附加文本 |
| 当前范围不可见 | 选中树节点后，右侧列表只显示下级，用户看不到当前选中组织本身 | 列表必须包含当前选中组织本身，并固定为第一行 |
| 层级范围不明确 | 用户无法判断右侧列表是只看当前组织、直接下级还是全部下级 | 使用“包含下级”开关显式控制；开启表示全部各级下级，关闭表示仅当前组织 |
| 包含下级前端拼接 | 为了快速交付，在页面用已加载树、多次 children 请求或本地整树缓存拼出列表 | 必须实现后端 list scope/range 契约；前端只传 `include_descendants` |
| 锚点被筛选/排序/分页挤出 | 当前组织本身不在第一页第一行，用户看不到范围锚点 | ReadService range list 契约保证锚点不参与下级过滤、始终置顶且不跨页重复，测试覆盖 keyword/status/ext、排序与分页 |
| 历史口径抽象 | “历史视图”像模式开关，用户不知道如何观察某一天的组织状态 | 改为“生效日期”日期框，默认当天日期；修改日期即改变 valid-time 视图 |
| current-mode deep link 被破坏 | 默认今天被写入 URL，导致动态当前视图变成固定日期链接 | 未手工修改日期时不写 `as_of`；只有用户修改日期才写入固定 day |
| 操作不流畅 | 用户切换开关或日期后还必须点击“应用”，形成控件值与列表结果两套状态 | 首屏轻量筛选自动应用；只有未来复杂高级筛选面板才允许面板内确认按钮 |
| 自动应用过度请求 | 文本输入每个字符都触发列表查询，或日期半截输入触发非法请求 | 文本搜索使用防抖并支持 Enter 立即提交；日期仅在合法 `YYYY-MM-DD` 后提交 |
| 扩展筛选变成僵尸能力 | 首屏移除扩展筛选后，用户无法发现或清除已生效 deep link | 管理员可见二级筛选区保留；命中 deep link 自动展开并提供清除 |
| 页面旧状态筛选隐形生效 | URL 中 `status` 继续过滤列表，但 UI 没有对应入口 | `status` 只作页面迁移 shim；显示业务提示与清除动作，任一新筛选修改时清理；新增页面链接/测试不得写入 |
| No-Legacy 漂移 | 把页面 `status` shim 或 direct-children 兼容写成长期第二入口/alias | shim 仅服务旧页面 URL 进入后的可见清理；API 既有 `status` filter 不称为旧 API；`make check no-legacy` 必须通过 |

## 7. 验收标准

1. [ ] 组织架构页首屏不再同时平铺三个相似文本搜索/筛选输入。
2. [ ] 用户能通过一个主搜索入口搜索/定位组织，树选中节点与列表范围同步；右侧列表第一行显示当前选中组织本身。
3. [ ] 后端 `/org/api/org-units` list/grid 支持显式 `include_descendants` 范围契约：未传时保留既有 direct-children/root/list 语义；关闭时只返回当前组织本身；开启时返回当前组织本身及全部各级可见下级；锚点固定第一页第一行且不被 keyword/status/ext/sort 移除；分页 total 与过滤结果正确。
4. [ ] 首屏不新增自定义“高级筛选”按钮；扩展字段筛选作为管理员可见二级筛选区保留，默认收起，命中扩展 deep link 时自动展开并可清除。
5. [ ] URL 参数 `node/q/include_descendants/as_of/ext_filter_*` 语义清晰且可复现页面状态，但用户可见 UI 不出现这些参数名或内部字段 key。
6. [ ] 左侧组织树节点不显示组织类型图标或括号编码；组织编码只在主搜索语义和列表“编码”列中出现。
7. [ ] “包含下级”位于“包含无效”左侧；开启时列表展示当前选中组织及全部各级可见下级，关闭时仅展示当前选中组织。
8. [ ] “历史视图”不再作为抽象按钮出现；首屏使用“生效日期”日期框，默认当天日期；未手工修改时不写 `as_of`，修改后写入固定日期并可复现。
9. [ ] 首屏不显示“应用”按钮；包含下级、包含无效、生效日期修改后自动应用，搜索组织防抖刷新列表并支持 Enter/图标立即定位。
10. [ ] 搜索多候选时展示候选选择，不随机选第一个；选择候选后写入当前范围。
11. [ ] 页面旧 `status` deep link 继续生效但不作为首屏入口；页面显示业务提示与清除动作；任一新筛选修改后 URL 删除 `status`；新增页面链接、页面测试 fixture 与 E2E 不再生成 `status`。
12. [ ] 表格内搜索入口不与页面级主搜索重复；列表工具栏不新增无实现支撑的田字列设置或高级筛选入口。
13. [ ] 页面实现不新增后端 route、不新增前端权限裁剪、不新增全量树缓存。
14. [ ] 491 selector 选择场景和 492 ReadService 读取事实源边界保持不变。
15. [ ] CubeBox `orgunit.list` 首期工具参数保持现状，不新增 `include_descendants`；`internal/server/cubebox_api_tools.go`、`modules/orgunit/presentation/cubebox/apis.md`、`queries.md`、`CUBEBOX-SKILL.md` 和 planner prompt/test 不出现该参数。
16. [ ] 实施验证记录包含 Authz、Routing、No-Legacy、CubeBox API-first、Go、前端和文档门禁；若 i18n/E2E 被修改，则包含对应门禁结果。
