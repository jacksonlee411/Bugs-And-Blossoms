# DEV-PLAN-076：OrgUnit 版本切换选中保持（去全局 as_of）

**状态**: 规划中（2026-02-06 12:00 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**: 组织架构详情在版本切换时丢焦问题（本文件“背景与上下文”与验收口径）。
- **当前痛点**:
  - 在 `/org/nodes?tree_as_of=2026-01-03` 选中“财务部2”后，点击“上一条/下一条”触发全页刷新并回到根节点详情。
  - 详情区依赖全局 `as_of` 刷新；刷新后仅靠 `localStorage` 恢复选中，稳定性差。
- **业务价值**:
  - 用户切换版本时保持当前组织不丢焦，降低误操作风险。
  - 明确树与详情版本的职责边界，降低全局刷新带来的状态耦合。

## 2. 目标与非目标 (Goals & Non-Goals)
- **核心目标**:
  - [ ] 版本切换（上一条/下一条/下拉）仅刷新详情区，不触发全页刷新，树选中保持。
  - [ ] 取消全局 `as_of` 机制，URL 仅承载树生效日期（`tree_as_of`）。
  - [ ] 树生效日期只影响组织树与搜索；详情版本由版本选择器驱动。
  - [ ] 详情区切换失败不丢焦，不清空现有详情内容。
  - [ ] 对齐有效期日粒度（`YYYY-MM-DD`）。
- **非目标 (Out of Scope)**:
  - 不新增 DB 迁移或 schema。
  - 不新增新的业务模块或导航入口。
  - 不引入新的鉴权策略/运维开关。

## 2.1 工具链与门禁（SSOT 引用）
> 仅声明触发器；脚本细节与门禁口径以 SSOT 为准。

- **触发器清单**:
  - [x] Go 代码（页面渲染/handler/参数校验）
  - [x] `.templ` / Tailwind / Astro UI（页面/交互 DOM 结构改动）
  - [ ] Authz（沿用现有策略）
  - [ ] 路由治理（不新增路由；仅调整查询参数）
  - [ ] DB 迁移 / Schema（不涉及）
  - [ ] sqlc（不涉及）
  - [ ] Outbox（不涉及）
  - [x] 文档（`make check doc`）

- **SSOT 链接**:
  - 触发器矩阵：`AGENTS.md`
  - CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
  - 路由策略：`docs/dev-plans/017-routing-strategy.md`
  - 有效期语义：`docs/dev-plans/032-effective-date-day-granularity.md`
  - OrgUnit 现有契约：`docs/dev-plans/073-orgunit-crud-implementation-status.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 架构图 (Mermaid)
```mermaid
graph TD
  U[User Browser] --> P[GET /org/nodes?tree_as_of=...]
  P --> T[Nodes Panel (Tree)]
  P --> D[Details Panel]
  T --> C[GET /org/nodes/children?tree_as_of=...]
  T --> S[GET /org/nodes/search?tree_as_of=...]
  D --> V[GET /org/nodes/details?org_id=...&effective_date=...]
  D --> W[POST /org/nodes (write actions)]
```

### 3.2 关键设计决策
- **决策 A（确认）**：取消全局 `as_of`，URL 仅承载树生效日期 `tree_as_of`。
- **决策 B（确认）**：版本切换只刷新详情区（HTMX 局部更新），不再 `window.location.href`。
- **决策 C（确认）**：详情版本参数与树日期解耦；详情默认版本由服务端选择（系统日命中优先；若无命中则取不晚于系统日的最近版本；仍无则回退到最早版本）。
- **决策 D（确认）**：仍保留 `localStorage` 选中恢复逻辑，但不再依赖全页刷新。
- **决策 E（确认）**：失败路径 fail-open：版本切换失败时保持现有详情与树选中，提示错误。

### 3.3 状态与参数矩阵（树 vs 详情）
| 维度 | 参数 | 来源 | 影响范围 | 备注 |
| --- | --- | --- | --- | --- |
| 树生效日期 | `tree_as_of` | URL 查询参数 / 默认计算 | 组织树 + 子节点懒加载 + 搜索 | 必填；缺失/非法则重定向到带默认值的 URL |
| 详情版本 | `effective_date` | 版本选择器 / 默认选中 | 详情区版本加载 | 仅影响详情，不写回 URL |
| 历史参数 | `as_of` | **废弃** | 不参与任何逻辑 | 视为非法参数（不做兼容/回退） |

### 3.4 URL 与选中恢复规则
- URL 仅承载 `tree_as_of`，不承载详情版本与选中组织。
- 选中组织仍使用 `localStorage.org_nodes_last_org_code` 恢复；该机制不依赖全页刷新。
- 切换 `tree_as_of` 后：若选中组织在新树中不存在，仅提示，不强制重置详情区。

## 4. 数据模型与约束 (Data Model & Constraints)
- 本计划不新增/修改 DB Schema。
- 业务有效期遵循 **day/date** 语义；详情与树均使用 `YYYY-MM-DD`。
- 不引入 legacy 兼容参数（`as_of` 仅作为历史概念，不再接受）。

### 4.1 关键不变量（必须满足）
- **树/详情解耦**：`tree_as_of` 只影响树与搜索；`effective_date` 只影响详情。
- **URL 单一职责**：URL 只承载 `tree_as_of`；版本切换不修改 URL。
- **选中保持**：详情切换失败不得改变树选中与当前详情内容。
- **日粒度语义**：所有日期均按 `YYYY-MM-DD` 校验与传输。

## 5. 接口契约 (API Contracts)
> 路由与字段口径以 DEV-PLAN-073 为准；本计划仅调整查询参数与详情版本加载方式。

### 5.1 页面入口（树日期）
- **GET `/org/nodes?tree_as_of=YYYY-MM-DD`**
  - 仅承载树生效日期。
  - 若 `tree_as_of` 缺失或非法：服务端计算默认值并 **302 重定向** 到带 `tree_as_of` 的 URL（确保可分享）。
  - 页面渲染时将实际 `tree_as_of` 写入左侧“生效日期”输入框。
  - 若租户无任何组织数据：渲染空态提示（URL 仍保持 `tree_as_of=system_day`）。

### 5.2 树懒加载
- **GET `/org/nodes/children?parent_id=...&tree_as_of=YYYY-MM-DD`**
  - 返回 `sl-tree-item` 列表片段。
  - `tree_as_of` 必填；缺失/非法直接 400。

### 5.3 查找组织（多匹配）
- **GET `/org/nodes/search?query=...&tree_as_of=YYYY-MM-DD`**
  - JSON：返回 `target_org_id`、`path_org_ids`、`target_org_code`。
- **GET `/org/nodes/search?query=...&tree_as_of=YYYY-MM-DD&format=panel`**
  - HTML：返回多匹配列表片段。

### 5.4 详情加载（版本切换）
- **GET `/org/nodes/details?org_id=...`**
  - 默认版本：优先“系统日命中版本”；若无命中，取不晚于系统日的最近版本；仍无则回退到最早版本。
- **GET `/org/nodes/details?org_id=...&effective_date=YYYY-MM-DD`**
  - 强制加载指定版本。
  - 返回详情片段时必须包含：
    - 当前版本 `effective_date`（用于下拉与按钮状态）。
    - 版本列表（`effective_date` + label）。
    - `data-org-id`、`data-current-effective-date` 等数据属性。

### 5.5 写入操作（保持 `/org/nodes`）
- **POST `/org/nodes`**
  - 沿用现有 action/字段（参见 DEV-PLAN-073）。
  - 新增/插入/删除记录表单：仍以 `effective_date` 为主键，**不依赖 `tree_as_of`**。
  - 表单需携带隐藏字段 `tree_as_of`，用于成功后的回跳上下文。
  - 成功后重定向到 `/org/nodes?tree_as_of=<当前树日期>`，并通过 `localStorage` 或详情刷新恢复选中。

### 5.6 错误码与响应（最小集合）
- `400 Bad Request`
  - `tree_as_of` 缺失或非法：`invalid_tree_as_of`。
  - 请求携带废弃参数 `as_of`：`deprecated_as_of`（提示改用 `tree_as_of`）。
  - `effective_date` 非法：`invalid_effective_date`。
- `404 Not Found`
  - 组织不存在：`org_unit_not_found`。
  - 指定版本不存在：`org_version_not_found`。
- `403 Forbidden`
  - 无权限：`forbidden`（UI 仅提示，不改变详情）。

### 5.7 契约更新清单（同步 DEV-PLAN-073）
- `/org/nodes`：查询参数由 `as_of` 改为 `tree_as_of`。
- `/org/nodes/children`：必填参数由 `as_of` 改为 `tree_as_of`。
- `/org/nodes/search`：查询参数由 `as_of` 改为 `tree_as_of`；`format=panel` 逻辑保持。
- `/org/nodes/details`：新增 `effective_date` 参数作为详情版本切换入口。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 页面初始化
1. 若请求携带 `as_of` → 直接返回 400（提示改用 `tree_as_of`）。
2. 解析 `tree_as_of`；缺失/非法 → 计算默认值并重定向。
   - 默认值算法：
     1) `system_day = 今日(UTC)`。
     2) `candidate = max(effective_date <= system_day)`。
     3) 若 `candidate` 不存在，则回退到 `min(effective_date)`。
     4) 若仍无数据，则返回空态并提示“该租户暂无组织数据”，同时保持 `tree_as_of=system_day`。
   - 以上日期由存储层按租户查询。
3. 初始化树：`/org/nodes/children?tree_as_of=...`。
4. 读取 `localStorage` 最近组织，若存在则触发 `loadDetails(org_id)`。

### 6.2 版本切换（不刷新全页）
1. 用户点击“上一条/下一条/下拉”。
2. 若存在未保存变更 → 确认提示。
3. 调用 `loadDetails(org_id, effective_date)` → HTMX 局部刷新详情区。
4. 成功：更新 `data-current-effective-date` 与版本选择器状态；**不改变 URL**。
5. 失败：保留现有详情内容，提示错误。

### 6.3 树日期切换（仅影响树）
1. 用户修改左侧“生效日期”。
2. 更新 URL（`history.replaceState` 或 302 跳转），仅变更 `tree_as_of`。
3. 重新加载树；若当前选中组织在新树中不存在：保留详情区并提示“该生效日期下树中不存在该组织”。

### 6.4 版本切换交互细化（前端）
1. “上一条/下一条/下拉”统一调用 `loadDetails(org_id, effective_date)`。
2. `loadDetails` 成功后更新：
   - 详情容器 `data-current-effective-date`。
   - 版本选择器选中项与计数（由服务端渲染）。
3. `loadDetails` 失败时：
   - 不更新 DOM 与 URL；仅显示错误提示。

## 7. 安全与鉴权 (Security & Authz)
- 读权限：`/org/nodes`、`/org/nodes/children`、`/org/nodes/details`、`/org/nodes/search` 保持现有 read 权限。
- 写权限：`POST /org/nodes` 保持现有 admin 权限；无权限时编辑入口禁用并提示。
- 不新增新的 subject/domain/object/action 命名。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**:
  - DEV-PLAN-073（OrgUnit 路由/契约）
  - DEV-PLAN-074（详情区 UI/交互结构）
  - DEV-PLAN-032（有效期日粒度）
  - DEV-PLAN-017（Routing 策略）
  - DEV-PLAN-004M1（No Legacy）
- **里程碑**:
  1. [ ] 参数与 URL 口径调整（`tree_as_of` + 详情版本参数）。
  2. [ ] 版本切换改为局部刷新（JS/HTMX）。
  3. [ ] 树日期控件与树刷新逻辑落地。
  4. [ ] 失败路径与提示完善。
  5. [ ] 验收与文档记录。

### 8.1 实施切分与影响文件（草案）
- **后端参数解析**
  - 新增 `requireTreeAsOf`（仅限 OrgUnit 相关 handler 使用），避免影响其他模块仍使用的 `requireAsOf`。
  - 若请求携带 `as_of` 则返回 400（`deprecated_as_of`）。
- **默认日期计算**
  - 存储层新增“可用日期查询”能力：
    - `max(effective_date <= system_day)` 与 `min(effective_date)`。
- **页面渲染与表单**
  - `/org/nodes` 页面：顶部/详情去除“全局 as_of”展示；树区域新增生效日期控件并绑定 `tree_as_of`。
  - 所有 OrgUnit 表单/链接携带 `tree_as_of` 作为回跳上下文。
- **前端交互**
  - `getAsOf()` → `getTreeAsOf()`；所有树/搜索请求改用 `tree_as_of`。
  - `loadDetails(org_id, effective_date)` 仅改变详情区，不更新 URL。
  - 版本切换按钮/下拉不再 `window.location.href`。
- **详情 API**
  - `/org/nodes/details` 支持 `effective_date` 参数；默认版本选择逻辑后端兜底。

## 9. 测试与验收标准 (Acceptance Criteria)
- **功能验收**:
  - [ ] 版本切换后，树选中不丢失，详情显示同一组织的不同版本。
  - [ ] URL 不再包含 `as_of`，仅包含 `tree_as_of`。
  - [ ] 树日期切换只刷新树，不强制重置详情版本。
  - [ ] 详情切换失败时，保留现有详情并提示错误。
  - [ ] 租户无组织数据时显示空态提示，URL 仍包含 `tree_as_of=system_day`。
- **建议测试**:
  - 单测：`tree_as_of` 缺失/非法时的默认值与重定向（含“系统日无数据 → 回退到最近可用/最早可用”）。
  - E2E：选中组织 → 版本切换 → 验证树选中保持、URL 不变、详情更新。
  - E2E：切换树日期 → 树刷新 → 详情未被强制重置。

### 9.1 验收记录与证据
- 验收结果需记录在 `docs/dev-records/` 对应执行日志中（含关键截图或操作步骤）。
- 至少包含：
  - 版本切换不丢焦的操作链路截图。
  - `tree_as_of` 变化仅影响树的截图。
  - 失败路径（详情切换失败）的提示截图或日志。

## 10. 运维与监控 (Ops & Monitoring)
- 本阶段不引入额外运维/监控与 Feature Flag（对齐项目早期运维原则）。
