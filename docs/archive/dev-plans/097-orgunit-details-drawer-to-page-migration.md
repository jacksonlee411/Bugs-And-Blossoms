# [Archived] DEV-PLAN-097：OrgUnit 详情从抽屉（Drawer）迁移为独立页面（对齐 MUI CRUD Dashboard）

**状态**: 已归档（2026-02-22，详情页形态已收口，不再继续单列实施）

> 参考示例：MUI CRUD Dashboard 模板 `/employees/:id` 详情页交互（顶部面包屑 + 独立详情页 + 底部返回按钮）。
>
> 本计划是 `DEV-PLAN-096` 的 UI/IA 补丁：不改变 org 领域/事件/权限边界与后端写入口，仅将“详情承载形态”从右侧抽屉改为独立页面，并补齐面包屑与返回路径，提升可读性与可分享性。

## 背景

- 当前 org 详情通过 `DetailPanel`（MUI `Drawer`）在右侧弹出。
- 抽屉形态在“信息密度、可读性、可扩展性（记录/审计/写操作）”上受限；同时不利于对齐企业级 CRUD 模板的“列表页/详情页”心智模型。

## 目标（DoD）

1. [ ] OrgUnit 详情不再以抽屉弹出，改为独立详情页（与 CRUD Dashboard 示例一致的 page pattern）。
2. [ ] 详情页顶部展示面包屑（Breadcrumbs），并提供返回到 org 列表页的可达路径。
3. [ ] 详情页底部提供“返回”按钮（优先返回历史列表上下文；无历史时回退到 `/app/org/units`）。
	4. [ ] 迁移不丢能力：Basic Info/Change Log 两个 Tab（Records 信息合并进 Basic Info，见 `DEV-PLAN-099`）、版本切换、审计列表、以及现有写操作入口继续可用（权限与错误回显口径不变）。
5. [ ] URL 可复现：详情页核心状态（`as_of/effective_date/tab/include_disabled`）可通过 URL 直接回放。

## 非目标

- 不在本计划调整 DB Kernel 写入口、事件模型、RLS/Authz 基础机制（边界仍以 `DEV-PLAN-019/021/022/026` 为准）。
- 不在本计划扩展 org 字段集或引入新的列表/详情 API（仅复用 `DEV-PLAN-096` 已落地的 details/versions/audit 读接口与既有写接口）。
- 不保留“抽屉/页面”双实现并行（避免双链路与体验分叉）。

## 方案

### 1) 路由与 URL 协议（建议冻结）

> 说明：本节只定义“对外可观察的 URL 语义”，具体解析与默认值逻辑以实现为准。

- 列表页：`/app/org/units`（保持不变）
	- 详情页：`/app/org/units/:org_code`
	  - Query：
	    - `as_of=YYYY-MM-DD`（Valid Time，日粒度；缺省为当天 UTC）
	    - `effective_date=YYYY-MM-DD`（可选；定位某条记录版本；缺省回落到 `as_of`）
	    - `tab=profile|audit`（可选；缺省 `profile`）
	    - `audit_event_uuid=<uuid>`（可选；仅 `tab=audit` 时用于复现右侧事件详情）
	    - `include_disabled=1`（可选；缺省 0）

> 与 `DEV-PLAN-096` 的关系：原先的 `detail=<org_code>` 将由路径参数承载；其余 query 参数保持语义一致。

### 2) 列表页到详情页的导航规则

- DataGrid 行点击：跳转到详情页 `/app/org/units/<org_code>`，并携带 `as_of/include_disabled`（以及需要保留的 `tab/effective_date` 缺省值）。
- 返回策略：
  - 底部“返回”按钮优先 `navigate(-1)` 返回历史列表上下文（保留用户的筛选/分页/树选中等状态）。
  - 若无可返回历史（例如用户直接打开详情链接），则回退到 `/app/org/units`（可选择携带 `as_of/include_disabled`）。

### 3) 详情页信息架构与布局（对齐 CRUD Dashboard 详情页模式）

建议页面结构：

- 顶部：`Breadcrumbs`
  - `组织架构`（链接到 `/app/org/units`）
  - 当前组织：`{name} ({org_code})`（不可点击）
- 标题区：复用/扩展 `PageHeader`
  - Title：`{name} · 详情`（或 `{name} ({org_code})`）
  - Actions：承载当前详情相关的写操作入口（rename/move/set BU/enable/disable/correct/rescind...），按权限显隐/禁用并保持 fail-closed。
- 内容区：沿用现有 `Tabs(profile/audit)` + 内容渲染与错误回显口径
- 底部：`返回`按钮（与示例一致，增强“可发现的退出路径”）

### 4) 代码落点（建议）

- 新增页面组件：`apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
- 路由注册：`apps/web/src/router/index.tsx` 新增 `path: 'org/units/:orgCode'`
- 列表页改造：`apps/web/src/pages/org/OrgUnitsPage.tsx`
  - 移除 `DetailPanel` 抽屉使用
  - 行点击由“写 query 打开抽屉”改为“navigate 到详情页”
- i18n：补齐面包屑/返回按钮相关 key（中英一致），并复用既有 org 详情文案 key

## 实施步骤

1. [ ] 冻结路由与 URL 协议（本文件 + `DEV-PLAN-096` 相关段落同步更新，避免口径漂移）。
2. [ ] 新增 `OrgUnitDetailsPage` 页面骨架：Breadcrumbs + PageHeader + Tabs + Back button。
3. [ ] 将现有抽屉详情逻辑（details/versions/audit 查询、tab 切换、错误态）迁移到详情页，并确保加载/空态/错误态口径一致。
4. [ ] 将详情相关写操作入口从“抽屉/列表页 header”收口到详情页 actions（保持权限策略与回显）。
5. [ ] 列表页行点击改为路由跳转；移除 `DetailPanel` 抽屉依赖。
6. [ ] i18n 补齐：面包屑与返回按钮（en/zh），并跑语言门禁（见 `AGENTS.md` 触发器矩阵）。
7. [ ] E2E/集成测试更新：覆盖“列表 -> 详情 -> 返回”主路径与无历史回退路径；并在 `DEV-PLAN-096/095` 的质量收口章节登记影响。
8. [ ] 文档收口：在 `AGENTS.md` Doc Map 增加本计划链接；跑 `make check doc` 通过门禁。

## 验收要点（最小可交付）

- 详情页能直接通过 URL 打开（含无权限/404/422/5xx 的用户可理解回显）。
- 面包屑与返回按钮在桌面/窄屏均可用；返回按钮行为符合“优先回到历史上下文”的预期。
- 不存在抽屉详情入口残留（避免同一能力两种承载形态并行）。
