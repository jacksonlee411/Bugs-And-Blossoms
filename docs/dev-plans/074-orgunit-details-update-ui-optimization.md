# DEV-PLAN-074：OrgUnit Details 集成更新能力与 UI 优化方案

**状态**: 规划中（2026-02-05 08:36 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**:
  - 现有 `/org/nodes` 详情区可读不可改，Nodes 列表仅根节点可见。
  - 参考：`docs/dev-plans/073-orgunit-crud-implementation-status.md`（路由与详情片段契约）、`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`（UI 单链路）、`docs/dev-plans/032-effective-date-day-granularity.md`（日粒度有效期）。
  - 设计对齐基准：`designs/orgunit/orgunit-details-ui.pen`。
- **当前痛点**:
  - 无法在 Nodes 列表中定位目标组织、详情区仅只读，无法完成更新。
  - 多条生效记录缺少版本切换与插入/删除操作入口，且“删除=停用”语义易误导。
  - 查找组织在“多匹配”场景缺少可选结果与回填机制。
- **业务价值**:
  - 在不新增模块与全局导航的前提下，让用户可发现、可操作地完成组织更新与生效记录管理。

## 2. 目标与非目标 (Goals & Non-Goals)
- **核心目标**:
  - [ ] Details 内完成“查看 + 编辑 + 保存/取消”的闭环，符合 UI 可发现性原则。
  - [ ] 支持多条生效记录的版本选择器（上一条/下一条/下拉）与新增/插入/删除记录（错误数据）。
  - [ ] 在同一详情入口补齐“删除组织（错误建档）”操作（V1：无子组织可删）。
  - [ ] “查找组织”支持多匹配选择并回填详情内容。
  - [ ] 有效期语义遵循日粒度，避免结束日期与 9999-12-31 魔法值展示。
  - [ ] 设计与实现对齐 `designs/orgunit/orgunit-details-ui.pen`。
  - [ ] 不引入 legacy/回退链路，保持 UI 单链路（对齐 DEV-PLAN-004M1/026）。
- **非目标 (Out of Scope)**:
  - 不新增数据库结构或迁移。
  - 不新增新的全局导航或独立业务模块页面。
  - 不引入复杂运维监控或开关切换。

## 2.1 工具链与门禁（SSOT 引用）
> 只声明触发器，脚本细节与门禁口径以 SSOT 为准。

- **触发器清单**:
  - [x] Go 代码（UI handler/片段渲染/校验逻辑）
  - [x] `.templ` / Tailwind / Astro UI（视实现触发）
  - [ ] Authz（复用现有策略；若新增需按 DEV-PLAN-022 补齐）
  - [ ] 路由治理（复用现有 `/org/nodes/*`；若新增需 `make check routing`）
  - [ ] DB 迁移 / Schema（不涉及）
  - [ ] sqlc（不涉及）
  - [ ] Outbox（不涉及）
  - [x] 文档（`make check doc`）

- **SSOT 链接**:
  - 触发器矩阵：`AGENTS.md`
  - CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
  - 路由策略：`docs/dev-plans/017-routing-strategy.md`
  - Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
  - 有效期语义：`docs/dev-plans/032-effective-date-day-granularity.md`
  - 现有 CRUD/路由契约：`docs/dev-plans/073-orgunit-crud-implementation-status.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 架构图 (Mermaid)
```mermaid
graph TD
  U[User Browser] --> P[GET /org/nodes?tree_as_of=...]
  P --> N[Nodes Panel (Tree)]
  P --> D[Details Panel]
  N --> C[GET /org/nodes/children]
  D --> S[GET /org/nodes/search]
  D --> T[GET /org/nodes/details]
  D --> W[POST /org/nodes (write actions)]
```

### 3.2 关键设计决策
- **版本选择器替代分页**：上一条/下一条作为版本切换控件，支持版本下拉；一页一条记录，无“查看全部记录”兜底入口。
- **记录操作统一入口**：新增/插入/删除记录（错误数据）与删除组织放在版本选择器区域，避免分散。
- **查找多匹配交互**：多匹配时展示下拉列表，点击回填详情并刷新版本选择器与表单。
- **去噪**：不显示“只读/编辑模式”等字样，操作按钮与标题同一行。
- **有效期显示**：仅展示/编辑生效日期，不展示结束日期，避免 9999-12-31 魔法值。
- **Nodes Panel 固定与可拖拽**：宽 260 / 高 1000，列表自适应高度，右侧拖拽手柄示意可拉宽。

### 3.3 分屏/状态矩阵（与画布一致）
- **OrgUnit Details - Readonly**：
  - 触发：默认进入或保存成功后。
  - 头部操作：显示“编辑”按钮。
  - 版本选择器与记录操作可见；点击“新增/插入/删除记录/删除组织”进入编辑态。
  - Tabs 互斥：默认显示“基本信息”，切换后显示“修改记录”。
- **OrgUnit Details - Edit**：
  - 触发：点击“编辑”或记录操作进入编辑态。
  - 头部操作：隐藏“编辑”，仅保留保存/取消。
  - 未保存变更提示仅在 dirty 状态出现。
- **OrgUnit Details - No Permission**：
  - 触发：无更新权限。
  - 头部“编辑”按钮禁用，记录操作全部置灰；展示原因提示。
- **OrgUnit Details - Save Failed**：
  - 触发：保存失败。
  - 保留已编辑内容，展示醒目错误提示；允许重试与取消。
  - 视为编辑态，隐藏“编辑”按钮。
- **OrgUnit Details - Create**：
  - 触发：点击“新建部门”。
  - 标题显示“新建部门”；表单字段使用占位提示，状态徽标不显示。
- **OrgUnit Details - Search Multi Match**：
  - 触发：查找组织返回多匹配。
  - 展示下拉列表；选中后回填详情并切换到对应状态。
  - 多匹配列表出现时隐藏“未找到匹配组织”提示。
- **OrgUnit Details - Records Version Selector**：
  - 触发：存在多条生效记录。
  - 标题不重复显示版本序号，版本信息集中在选择器内。

## 4. 数据模型与约束 (Data Model & Constraints)
- 本计划不新增/修改 DB Schema。
- 业务有效期统一按 **day (date)** 语义处理与展示（SSOT：DEV-PLAN-032）。
- 详情区展示字段包含：组织ID、组织ID链、UUID、修改人/创建日期/修改日期。
- 组织长名称与组织ID链展示格式参考“组织架构快照”。

### 4.1 关键不变量（落地必须满足）
- **单链路读取**：`/org/nodes` 仅走 current 读路径；不得引入 legacy/回退通道（对齐 DEV-PLAN-004M1/026）。
- **写入口唯一**：所有写入必须走 `/org/nodes` 的既定 action 并由 DB Kernel `submit_*_event(...)` 承担（对齐 DEV-PLAN-026）。
- **租户隔离 fail-closed**：缺失租户上下文或鉴权失败必须拒绝，不得放行（对齐 DEV-PLAN-021/022）。
- **有效期日粒度**：仅使用 date 语义，UI 不展示结束日期与 9999-12-31（对齐 DEV-PLAN-032）。

## 5. 接口契约 (API Contracts)
> 现有路由与字段口径以 DEV-PLAN-073 为准；本计划在 UI 层新增/扩展交互时必须同步修订对应契约文档。

### 5.1 页面与树
- **GET `/org/nodes?tree_as_of=YYYY-MM-DD`**
  - 渲染页面壳 + 初始根节点（或顶层节点）。
- **GET `/org/nodes/children?parent_id=...&tree_as_of=YYYY-MM-DD`**
  - 返回 HTML fragment（`sl-tree-item` 列表）。
  - `sl-tree-item` 必须包含 `data-org-id`/`data-org-code`/`data-has-children`。

### 5.2 详情面板
- **GET `/org/nodes/details?org_id=...&effective_date=YYYY-MM-DD`**
  - 返回详情面板 HTML fragment（容器建议为 `#org-node-details`）。
  - 面板内容需包含：查找组织、版本选择器、Tab（基本信息/修改记录）、编辑表单与状态提示。
  - **版本选择器最小数据**（建议内嵌在 fragment 中，避免额外 API）：
    - 当前版本序号与总数（例如 `3/12`）。
    - 版本列表（至少包含 `record_id`/`effective_date`/`label`）。
    - 当前版本标记（用于下拉与上一条/下一条）。

### 5.3 查找组织（多匹配）
- **现有路径定位（保持兼容）**：
  - `GET /org/nodes/search?query=...&tree_as_of=YYYY-MM-DD` 返回 JSON（含 `target_org_id` 与 `path_org_ids`）。
- **多匹配下拉列表（确定方案）**：
  - 使用同一路由：`GET /org/nodes/search?query=...&tree_as_of=...&format=panel`。
  - `format=panel` 返回 HTML 列表项（至少包含 `data-org-id`、`data-org-code`、`name`）。
  - 点击列表项后触发 `/org/nodes/details` 回填详情与版本选择器。
  - 该参数与返回格式需同步写入 DEV-PLAN-073 契约。

### 5.4 写入操作（复用 `/org/nodes`）
- **POST `/org/nodes?tree_as_of=YYYY-MM-DD`**
  - 沿用现有 action/字段命名（SSOT：DEV-PLAN-073、DEV-PLAN-026a/026b）。
  - 本计划新增 UI 行为：新增记录 / 插入记录 / 删除记录（错误数据） / 删除组织（错误建档）。
  - **action（确定命名）**：
    - `action=add_record`：新增记录（追加为最新版本，`effective_date` 必填）。
    - `action=insert_record`：插入记录（可在序列中间插入，`effective_date` 必填）。
    - `action=delete_record`：删除记录（需指定 `effective_date`，语义为物理删除错误事件，非停用）。
    - `action=delete_org`：删除组织（删除该组织全部历史事件；V1 仅无子组织且非根组织可删）。
  - **冲突规则（UI 需按错误码回显）**：
    - 同一 `effective_date` 已存在：`409 Conflict`。
    - 删除最后一条记录：`409 Conflict`（禁止通过 `delete_record` 清空，需改用 `delete_org`）。
    - 删除组织受限（根组织/存在子组织）：`409 Conflict`。
    - 删除后重放失败：`409/422` 并整体回滚。
    - 无权限：`403 Forbidden`（按钮禁用且回显原因）。
  - 上述 action/参数需同步写入 DEV-PLAN-073 契约后再实现 UI。

### 5.5 错误与回显规则（最小集合）
- `400 Bad Request`：`as_of`/`org_id`/`query` 非法或缺失 → 顶部错误提示并阻止提交。
- `403 Forbidden`：无权限 → 操作按钮禁用 + 原因提示。
- `404 Not Found`：目标组织或记录不存在 → “未找到匹配组织/记录”提示。
- `409 Conflict`：有效期冲突/删除受限（最后一条记录、根组织、有子组织） → 表单内联错误提示。
- `409/422`：删除后重放失败（已回滚） → 顶部错误提示并保留当前视图。
- `422 Unprocessable Entity`：字段校验失败 → 对应字段就地提示。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 版本选择器与编辑区联动
1) 默认选择覆盖 `as_of` 的有效记录；若无匹配，则选最新记录并提示。
1.1) 在只读态点击“新增/插入/删除记录/删除组织”时，先切换到编辑态并回填当前版本数据。
2) 切换版本：若存在未保存变更，先提示确认；确认后加载对应版本详情。
3) 新增/插入/删除记录成功后：刷新版本列表与详情区，保持一致。
4) 删除组织成功后：清空当前详情并提示“组织已删除”，左侧树同步刷新并取消选中。

### 6.2 查找多匹配处理
1) 查询返回多条时，展示下拉列表。
2) 选择列表项后，触发详情刷新并同步版本选择器。
3) 未找到时显示“未找到匹配组织”。

### 6.3 状态机（事件 → 状态）
| 事件 | 结果状态 | 说明 |
| --- | --- | --- |
| 进入页面/保存成功 | Readonly | 默认状态 |
| 点击“编辑” | Edit | 进入可编辑态 |
| 点击“新增/插入/删除记录/删除组织” | Edit | 先切换到 Edit 并回填当前版本 |
| 保存失败 | Save Failed | 保留内容并提示重试 |
| 取消 | Readonly | 放弃未保存变更 |
| 无权限 | No Permission | 操作禁用并提示原因 |
| 查找多匹配 | Search Multi Match | 展示候选列表 |
| 选中匹配项 | Readonly | 回填详情并退出多匹配态 |
| 多条生效记录 | Records Version Selector | 显示版本选择器 |

## 7. 安全与鉴权 (Security & Authz)
- 读权限：`GET /org/nodes`、`/org/nodes/children`、`/org/nodes/details`、`/org/nodes/search` 按 read 权限控制。
- 写权限：`POST /org/nodes` 为 admin 权限；无权限时编辑与记录操作按钮禁用并提示原因。
- 不新增新的 subject/domain/object/action 命名；沿用现有 Authz 约定。

## 8. 依赖与里程碑 (Dependencies & Milestones)
- **依赖**:
  - DEV-PLAN-073（OrgUnit CRUD/路由/详情契约）
  - DEV-PLAN-026（UI 单链路与事件写入口）
  - DEV-PLAN-032（有效期日粒度）
  - DEV-PLAN-022（Authz 口径）
  - DEV-PLAN-017（Routing 策略）
- **里程碑**:
  1. [X] 完成 UI 结构与交互（对齐 `.pen` 画布）。
  2. [X] 查找多匹配交互与详情回填。
  3. [ ] 版本选择器与新增/插入/删除记录（错误数据）/删除组织动作。
  4. [ ] 验收与文档记录（含截图与关键路径，见 §9.1）。

## 9. 测试与验收标准 (Acceptance Criteria)
- **功能验收**:
  - [X] 新建部门：表单可提交，成功后回到只读态并展示新值。
  - [X] 多匹配查找：列表可选，回填详情与版本选择器正确。
  - [X] 版本切换：上一条/下一条/下拉切换可用，编辑区随之更新。
  - [ ] 新增/插入/删除记录（错误数据）/删除组织：操作成功后版本列表与详情一致。
  - [X] 无权限：编辑与记录操作入口禁用并提示。
- **门禁**:
  - [X] `make check doc` 通过（文档变更）。
  - [X] 其他门禁按 AGENTS 触发器矩阵执行。

### 9.1 验收记录（2026-02-06）
- 新建部门：E2E 覆盖创建 OrgUnit 并回显（`m3-smoke` / `tp060-02` / `tp060-03`），本地 `make e2e` 通过；详见 `docs/dev-records/dev-plan-074-execution-log.md`。
- 多匹配查找：单测覆盖 `format=panel` 返回与候选渲染（`orgunit_nodes_read_test.go`），逻辑通过；UI 交互需后续手工走查补截图。
- 版本切换：已于 2026-02-06 通过 Playwright 复验（`pnpm -C e2e exec playwright test tests/tmp-orgunit-version-switch.spec.js`）：点击“上一条/下一条”与下拉切换后 URL `as_of` 变更，详情头部组织名称与“生效日期”文本随版本更新；后续如需截图留档可补拍。
- 新增/插入/删除记录：当前单测覆盖 `add_record/insert_record/delete_record` 旧语义路径（`delete_record=DISABLE`）；按 DEV-PLAN-075C 新语义（物理删除 + replay）需补充新用例与手工走查截图。
- 无权限：单测覆盖 `canEditOrgNodes` 权限判定（`orgunit_nodes_test.go`），UI 禁用逻辑已落地；需后续手工走查补截图。
- 门禁：`make check doc`（2026-02-06）通过；`go fmt ./... && go vet ./... && make check lint && make test`、`make e2e` 通过记录见执行日志。

## 10. 运维与监控 (Ops & Monitoring)
- 本阶段不引入额外运维/监控与开关切换（对齐项目早期运维原则）。

## 交付物
- UI 设计画布：`designs/orgunit/orgunit-details-ui.pen`。
- 详情区交互实现与接口契约更新（如有变更，需同步 DEV-PLAN-073）。
- 验收记录（截图/关键路径说明）。
