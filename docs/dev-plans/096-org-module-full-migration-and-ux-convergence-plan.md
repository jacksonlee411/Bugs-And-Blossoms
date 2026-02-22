# DEV-PLAN-096：Org 模块全量迁移至 MUI X 与统一体验收口方案

**状态**: 实施中（2026-02-12 20:35 UTC）

## 0.1 与 DEV-PLAN-108 的关系（2026-02-18 补充）

本计划当前文本中，写操作仍以“多个原子动作对话框（Rename/Move/Enable/Disable/Set BU/Correction）+ 对应动作型 API”描述为主。

自 `DEV-PLAN-108` 起，OrgUnit CRUD 写交互目标收敛为：

- CRUD 区仅保留 5 按钮：新建组织 / 新建版本 / 插入版本 / 更正 / 删除；
- 新建组织/新建版本/插入版本/更正统一使用字段编辑表单（不再暴露动作类型选择）；
- 后端统一写入口 `POST /org/api/org-units/write`（intent 自动判定），append 多字段以 `UPDATE` 单事件承载。

因此，本计划中“动作型 Dialog/endpoint 作为长期目标”的内容在 108 后转为历史形态或兼容层参考；后续 UX/契约验收应以 108 为准。

> 本计划承接 `DEV-PLAN-090/093/094/095`，面向 org 模块完成“读 + 写 + 版本 + 审计 + 体验”全链路迁移。当前已完成左侧组织树与右侧列表改造，本计划补齐剩余能力并做统一风格收口。

## 1. 子计划范围

- 目标模块：`/app/org/units`（MUI SPA）对应的 org 模块完整业务链路。
- 目标形态：统一双栏工作区（树 + 列表）+ 详情面板 + 记录版本与写操作闭环。
- 范围覆盖：页面交互、前端 API 集成、权限显隐、错误回显、E2E 与性能收口。

## 2. 核心目标（DoD）

- [x] Org 模块在 MUI 页面达到“树 + 列表 + 详情 + 记录版本 + 批量/写操作”完整闭环。
- [x] 页面布局、视觉、文案、空态/加载态/错误态统一到平台组件与主题 Token。
- [x] org 模块关键主路径 E2E 稳定通过并纳入 CI 绿线。
- [ ] 迁移后形成可追溯验收记录（功能覆盖、质量门禁、性能基线、遗留风险）。

## 3. 非目标

- 不在本计划重写 org 领域模型、事件模型或 RLS/Authz 基础机制。
- 不在本计划新增与 org 无关的新业务模块页面。
- 不引入 legacy 双链路回退；迁移过程遵循单链路原则（No Legacy）。

## 4. 现状基线与缺口

### 4.1 已完成（基线）

- [x] `OrgUnitsPage` 已具备左树右表基础形态与 URL 参数协议。
- [x] 树组件已统一到 `TreePanel`，列表容器已统一到 `DataGridPage`。
- [x] `/org/api/org-units` 已接入首批真实查询（根节点 + 子节点）。

### 4.2 待补齐（迁移缺口）

1. [x] 详情区已接入真实详情 + 版本列表 + 审计列表，支持版本切换（`details/versions/audit` API）。
2. [x] 新建/重命名/移动/启停用/更正/撤销/BU 设置已在 MUI 页可视化落地，并接入真实写接口。
3. [x] 树内搜索定位已接入（`/org/api/org-units/search`），支持定位目标节点并回放到 URL 状态。
4. [ ] DataGrid 高阶能力（真 server-mode、列能力、批量操作链路）未完成收口。
5. [ ] org 模块主路径 E2E、性能压测与迁移验收报告未冻结。

### 4.3 能力覆盖矩阵（从“可用”到“可交付”）

> 目的：把“全部迁移”落到可验收的能力清单，避免只完成 UI 骨架但缺少写入/版本/审计导致“僵尸功能”。

| 能力 | 目标体验（行业最佳实践口径） | 主要落点（MUI） | 后端依赖（示例） | 验收要点 |
| --- | --- | --- | --- | --- |
| 树浏览 | 懒加载、选中/展开态保持、URL 可复现 | `TreePanel` + URL | `GET /org/api/org-units`（roots/children） | 刷新/回退不丢选中；大树不全量渲染 |
| 树搜索定位 | 按编码/名称检索，多匹配可选，自动展开路径并聚焦 | Tree 搜索框 + 结果列表 | `GET /org/nodes/search` 或新增 `GET /org/api/org-units/search` | 可复现定位到目标节点，错误态明确 |
| 列表（真 server-mode） | 服务端分页/排序/筛选；列配置持久化 | `DataGridPremium`（或基线 `DataGridPage`） | 需要支持分页/排序/筛选参数（必要时扩展 API） | 10k+ 数据仍流畅；URL 复制可重放 |
| 详情读取 | 真实字段（含负责人/上级/BU/状态/审计摘要），错误码可理解 | `DetailPanel` | 建议新增 `GET /org/api/org-units/details` | 404/409/422 回显一致；加载/空态统一 |
| 记录版本 | 按 `effective_date` 切换历史/未来记录；不丢上下文 | `DetailPanel` 的 Records 区 | 建议新增 `GET /org/api/org-units/versions` | 版本切换可复现；dirty 状态有保护 |
| 创建 | 新建组织可发现、可操作、可回显 | Create Dialog/Drawer | `POST /org/api/org-units` | 成功后树/表/详情一致刷新 |
| 重命名 | 可指定生效日；冲突/范围错误可解释 | Rename Dialog | `POST /org/api/org-units/rename` | 409/422 正确提示；成功后版本更新 |
| 移动 | 选择新上级 + 生效日；防环/越界提示 | Move Dialog | `POST /org/api/org-units/move` | 防环规则可测；树路径更新一致 |
| BU 设置 | 勾选 BU 并可追溯 request_id | BU Toggle | `POST /org/api/org-units/set-business-unit` | 幂等（request_code）；结果可追溯 |
| 启用/停用 | 明确语义与二次确认；同日冲突可提示 | Enable/Disable Dialog | `POST /org/api/org-units/enable|disable` | 状态徽标一致；禁止“静默失败” |
| 更正（Correction） | 原地更正历史记录，展示影响范围 | Correction Drawer | `POST /org/api/org-units/corrections` | 失败路径覆盖；成功后审计可见 |
| 撤销记录/删除组织 | 高风险操作强确认；仅允许规则内场景 | Rescind Dialog | `POST /org/api/org-units/rescinds*` | 根组织/有子组织受限提示清晰 |
| 审计与变更记录 | 可读、可筛选、可关联 request_id | Audit Tab | 建议新增 `GET /org/api/org-units/audit` | 撤销/更正可辨识；与 UI 操作一致 |
| 批量操作 | 多选 + 进度 + 部分失败可解释 | Grid Bulk Actions | 视后端是否支持批量；先落 UI 模型 | 有防误操作确认；结果可追踪 |

## 5. 实施步骤

### 5.1 TP-096-01：信息架构与页面布局冻结

1. [x] 冻结 org 工作区布局：`PageHeader + FilterBar + TreePanel + DataGridPage + DetailPanel`。
2. [x] 冻结响应式规则（桌面双栏、窄屏纵向堆叠）与树面板宽度策略（最小宽度/可拉伸）。
3. [x] 冻结交互状态模板：loading/empty/error/no-access/dirty-confirm，避免页面各自实现。

### 5.2 TP-096-02：读路径全量迁移

4. [x] 树查询收口：懒加载、搜索定位、节点路径展开、选中态保持与 URL 可复现。
5. [ ] 列表查询收口：分页/排序/筛选与 URL 参数协议统一，切到真实 server-mode。
   - 约束：列表必须只依赖服务端返回的 `items + total`，禁止前端再做全量排序/筛选/切片（避免“名义 server-mode，实际 client slicing”）。
   - 默认排序：未指定 `sort/order` 时按树顺序（`node_path`）稳定返回，保持与树浏览一致的“结构顺序”体验。
   - 必须支持：`q/status/page/size/sort/order`（见 8.2）。
6. [x] 详情读取收口：接入真实详情字段与版本记录；支持按 `effective_date` 切换历史记录。
7. [x] 读链路错误码与回显策略收口（404/409/422/5xx）并统一中英文本键。

### 5.3 TP-096-03：写路径全量迁移

8. [x] 在 MUI 页面落地核心写操作入口：创建、重命名、移动、设置 BU、启用/停用。
9. [ ] 落地记录级操作入口：新增记录、插入记录、删除记录、删除组织（按既有约束）。
10. [x] 落地更正与撤销入口：`corrections/status-corrections/rescinds` 的 UI 操作与结果反馈。
11. [x] 落地权限策略：无权限隐藏/禁用 + 明确原因提示；保持 fail-closed。

### 5.4 TP-096-04：统一风格与行业最佳实践收口

12. [ ] 统一视觉语言：间距、密度、状态色、按钮层级、表单校验反馈全部走主题 Token。
13. [ ] 统一交互反馈：Toast/Inline Error/Confirm Dialog 口径一致，避免“静默失败”。
14. [ ] 可访问性（A11y）达标：键盘可达、焦点可见、ARIA 标签、对比度满足 WCAG AA。
15. [ ] 统一可观测性：关键交互埋点覆盖（filter_submit/detail_open/bulk_action/write_submit）。

### 5.5 TP-096-05：质量收口与切换验收

16. [ ] 补齐测试：单元/组件/集成/E2E（至少覆盖 org 核心主路径与失败分支）。
17. [ ] 性能压测：树表联动、详情切换、批量操作场景纳入 `DEV-PLAN-095` 基线。
18. [ ] 切换与收尾：确认 MUI 入口具备全量能力后，冻结旧页面迁移策略与验收报告。

## 6. 路由与 URL 协议（可分享/可回放）

> 原则：URL 是状态源，满足“复制链接即可复现同一视图”。

- 目标路由：`/app/org/units`
- Query 参数（建议冻结）：
  - `as_of=YYYY-MM-DD`：树/列表“查看日期”（Valid Time，日粒度）。
  - `node=<org_code>`：选中树节点（驱动列表过滤）。
  - `q/status/page/size/sort/order`：列表查询（沿用 `gridQueryState` 协议）。
  - `detail=<org_code>&effective_date=YYYY-MM-DD`：详情打开与版本定位（可选）。
  - `tab=profile|records|audit`：详情区 Tab（可选）。

## 7. 页面与设计规范（统一风格）

### 7.1 布局规范

- 左侧：组织树（检索 + 展开 + 选中），右侧：数据网格（筛选 + 列表 + 批量动作）。
- 详情：使用统一 `DetailPanel`，内容分“基础信息 / 记录版本 / 修改记录”分区。
- 顶部：`PageHeader` 固定标题与动作区，筛选统一放 `FilterBar`，避免散落按钮。

### 7.2 交互规范

- URL 作为状态源：树节点、筛选条件、排序分页、版本日期均可直接分享与回放。
- 所有高风险写操作必须二次确认，并显示影响范围与不可逆提示。
- 表单遵循“字段级提示优先，页面级错误兜底”的分层反馈策略。

### 7.3 行业最佳实践对齐点

- 企业级数据页：默认支持 server-side 分页/排序/筛选，避免大数据量前端全量计算。
- 树表协同：选树即筛表、切版本不丢上下文、返回列表保持用户位置。
- 可维护性：页面禁止散落硬编码样式与文案，统一收口到主题与 i18n key。

## 8. 接口契约与文档协同

- 本计划实现过程中，如发生 org API/交互契约变化，先更新以下 SSOT 后落代码：
  - `docs/dev-plans/073-orgunit-crud-implementation-status.md`
  - `docs/dev-plans/074-orgunit-details-update-ui-optimization.md`
  - `docs/archive/dev-plans/075a-orgunit-records-ui-and-editing-issues.md`
- 写入口保持唯一：沿用既有 org internal API 与 DB Kernel 事件提交约束，不新增并行写入口。

### 8.1 API 扩展点（如需）

> 当前 `GET /org/api/org-units` 仅覆盖 roots/children。为了完成“详情 + 版本 + 审计”迁移，建议补齐 JSON 读接口；若改动接口契约，先更新契约文档再落代码。

- [x] `GET /org/api/org-units/details?org_code=...&as_of=...`
- [x] `GET /org/api/org-units/versions?org_code=...`
- [x] `GET /org/api/org-units/audit?org_code=...&limit=...`（或复用现有审计读模型）

### 8.2 列表（DataGrid 真 server-mode）契约（冻结）

> 目标：让 DataGrid 的分页/排序/筛选真实由后端承担，并保证 URL 可复现（复制链接即可回放同一列表状态）。

**Endpoint**：`GET /org/api/org-units`

**语义**：
- `parent_org_code` 为空：返回“根节点列表”（用于树根加载）。
- `parent_org_code` 有值：返回“该节点的直接子节点列表”（用于树懒加载与右侧列表）。
- 当请求包含 `page/size` 时：启用分页模式，必须返回 `total`（DataGrid 需要总行数）。

**Query 参数**：
- `as_of=YYYY-MM-DD`（可选；默认当天 UTC）：Valid Time 查询日期（日粒度）。
- `include_disabled=1`（可选；默认 0）：是否包含 disabled 记录。
- `parent_org_code=<org_code>`（可选）：父节点 org_code；缺省表示 roots。
- `q=<keyword>`（可选）：关键字（对 `org_code/name` 做 contains 匹配，大小写不敏感）。
- `status=all|active|inactive`（可选；默认 all）：列表状态筛选；`inactive` 对齐后端 `disabled`。
- `page=<int>=0..`（可选；缺省则不分页）。
- `size=<int>=1..200`（可选；缺省则不分页）。
- `sort=code|name|status`（可选；缺省则按树顺序返回）。
- `order=asc|desc`（可选；仅当 `sort` 存在时生效）。

**Response（200 OK）**：
```json
{
  "as_of": "2026-02-12",
  "include_disabled": false,
  "page": 0,
  "size": 20,
  "total": 123,
  "org_units": [
    {
      "org_code": "A001",
      "name": "销售一部",
      "status": "active",
      "is_business_unit": false,
      "has_children": true
    }
  ]
}
```

**兼容性约束**：
- 不带 `page/size` 的请求，保持旧行为（返回全量 `org_units`，可不返回 `total/page/size`）。
- `status` 字段在 roots/children 返回体中均必须存在（避免前端再补默认值导致口径漂移）。

### 8.3 列能力收口（持久化与统一口径）

> 目标：把列相关能力从页面“各自实现”收口到 `DataGridPage`，并且刷新后保持用户偏好。

**收口范围（本期最小闭环）**：
- 列显示/隐藏：`columnVisibilityModel` 持久化。
- 列顺序：拖拽 reorder 后持久化 `orderedFields`。
- 列宽：resize 后持久化 `dimensions[field].width`。
- 密度：`density`（compact/standard/comfortable）持久化。

**持久化策略**：
- 存储介质：`localStorage`（前端偏好，不进入业务数据）。
- Key 约定：`web-mui-grid-prefs/<storage_key>`（页面传入 `storage_key`，建议带 tenant 前缀）。
- 兼容：列字段增删时，未知字段忽略，新字段追加到末尾。

## 9. 工具链与质量门禁（SSOT 引用）

- 触发器矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
- 本计划预期命中门禁：
  - [x] UI 变更：`make generate && make css`
  - [x] Go/API 变更：`go fmt ./... && go vet ./... && make check lint && make test`
  - [x] 路由治理：`make check routing`
  - [x] Authz 变更：`make authz-pack && make authz-test && make authz-lint`
  - [x] E2E：`make e2e`
  - [x] 文档：`make check doc`

## 10. 风险与缓解

- 风险：一次性补齐读写链路导致回归面过大。  
  缓解：按 TP 子计划分批合入，每批都有可回归清单与验证记录。
- 风险：旧页面与新页面能力重叠引发用户路径混乱。  
  缓解：冻结主入口到 `/app/org/units`，并通过导航与文案明确唯一入口。
- 风险：真实后端错误码映射不完整导致前端提示不一致。  
  缓解：建立错误码映射表并在测试中覆盖 4xx/5xx 主要分支。

## 11. 交付物

- [x] Org 模块 MUI 页面全量迁移实现（含读写与版本链路）。
- [ ] 统一布局与交互规范清单（与平台组件对齐）。
- [ ] org 模块 E2E 用例与性能基线记录。
- [ ] 迁移验收报告（功能覆盖、门禁结果、风险与后续待办）。

## 12. 实施进展记录（2026-02-12）

### 13.1 代码落地（已完成）

- 后端 org internal API 已补齐并接入路由/Authz/allowlist：
  - `GET /org/api/org-units/details`
  - `GET /org/api/org-units/versions`
  - `GET /org/api/org-units/audit`
  - `GET /org/api/org-units/search`
- `GET /org/api/org-units` 已支持 `include_disabled`，roots/children 返回体补充 `status`。
- MUI org 页面已落地详情三 Tab（Profile/Records/Audit）、树搜索定位、核心写操作弹窗与批量启停用。
- i18n 文案键已补齐 org 模块新增交互文案。

### 13.2 质量验证（已执行）

- 后端：`go fmt ./...`、`go vet ./...`、`make check lint`、`make test`（覆盖率门禁 100% 通过）。
- 前端：`pnpm -C apps/web check`（lint/typecheck/test/build 全通过）。
- UI 构建：`make generate`、`make css` 通过。
- 治理门禁：`make check routing`、`make check doc` 通过。
- 权限门禁：`make authz-pack && make authz-test && make authz-lint` 通过。
- E2E：`make e2e`（5/5 通过）。

### 13.3 当前剩余事项

- 补齐 org 模块 E2E 主路径并纳入 `make e2e` 稳定绿线。
- 与 `DEV-PLAN-095` 联动补充性能/稳定性基线与验收报告。
- DataGrid 真 server-mode 与列能力收口（避免本地分页/排序模拟）。

## 13. 关联计划

- 总方案：`docs/dev-plans/090-mui-x-frontend-upgrade-plan.md`
- P2：`docs/dev-plans/093-mui-x-phase2-high-value-modules-plan.md`
- P3：`docs/dev-plans/094-mui-x-phase3-long-tail-convergence-plan.md`
- P4：`docs/dev-plans/095-mui-x-phase4-stability-performance-plan.md`
- Org CRUD：`docs/dev-plans/073-orgunit-crud-implementation-status.md`
- Org Details：`docs/dev-plans/074-orgunit-details-update-ui-optimization.md`
- 质量门禁：`docs/dev-plans/012-ci-quality-gates.md`
