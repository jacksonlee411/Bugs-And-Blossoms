# [Archived] DEV-PLAN-084：Org 模块组织树“下级可展开指示符”缺失问题分析与收敛方案

**状态**: 已归档（2026-02-22，树可展开契约已并入 `DEV-PLAN-073`；本文仅保留专项问题分析与修复记录）

## 背景
- 当前 `Org Units` 页面左侧组织树采用“按点击逐级懒加载”策略。
- 反馈问题：节点在未点击前默认不显示“可展开下级”的指示符，用户需要先点一次才知道是否存在下级组织。

## 问题现象
1. 初始进入页面时，树节点默认看起来都像叶子节点。
2. 用户点击某节点后，页面才发起子节点请求并更新树；此时才可判断该节点是否有下级。
3. 该行为导致“可发现性”不足，增加无效点击。

## 根因分析（代码层）
### 1) Tree 渲染层以“是否已挂载 children”决定是否显示展开指示
- `apps/web/src/components/TreePanel.tsx` 的 `renderNodes` 仅在 `node.children` 存在时渲染子 `TreeItem`。
- 当前 `TreePanelNode` 结构没有显式 `hasChildren` 元数据，导致“有下级但尚未加载”的节点与“真正叶子节点”在 UI 上不可区分。

### 2) Org 页面构树时丢弃了后端 `has_children` 信息
- `apps/web/src/pages/org/OrgUnitsPage.tsx` 的 `buildTreeNodes` 只根据 `childrenByParent`（已加载缓存）决定 `children` 是否存在。
- `OrgUnitAPIItem` 虽有 `has_children` 字段（`apps/web/src/api/orgUnits.ts`），但构树逻辑未使用该字段，导致“可展开能力”未传递到视图层。

### 3) 根节点查询链路未统一输出 `has_children`
- 后端 `internal/server/orgunit_api.go` 在 `parent_org_code` 有值时会返回 `HasChildren`；但根节点列表（无 `parent_org_code`）走兼容分支时未填充 `HasChildren`。
- 这使首屏根节点在数据契约层也缺少“是否有下级”的可靠信号。

### 4) 交互触发晚于可视提示
- `handleTreeSelect` 内通过 `ensureChildrenLoaded` 在点击后才加载子节点（`apps/web/src/pages/org/OrgUnitsPage.tsx`）。
- 现状等同于“先交互，后得知结构”，与树控件的预期认知模型相反。

## 影响范围
- Org 模块组织树导航体验（首屏与逐级展开）。
- 树搜索后定位节点时，用户无法快速预判后续分支结构。
- 可用性层面增加额外点击与认知成本，不影响后端数据正确性。

## 目标（收敛口径）
1. 首次渲染即可区分“可展开节点”与“叶子节点”。
2. 保持懒加载，不做全量预加载，不引入 legacy/双链路。
3. 前后端在 `has_children` 语义上保持单一契约。

## 非目标（Non-Goals）
- 不改动 Org 列表（DataGrid）筛选/排序/分页语义。
- 不改动路由结构、鉴权模型、RLS 与任何写入链路。
- 不引入“预加载整棵树”或“fallback 到旧树实现”的双链路。

## 边界与不变量
- 前端树节点的“可展开性”由**单一事实源** `has_children` 决定，而不是由 `children` 是否已加载推断。
- `children` 仅表示“当前已加载的子节点集合”；`has_children=true` 且 `children` 为空表示“可展开但未加载”。
- 叶子节点必须满足 `has_children=false`，且不显示展开指示符。

## 契约冻结（API/DTO）
### API 读契约
- `GET /org/api/org-units?as_of=...`（根节点）返回 `org_units[].has_children`。
- `GET /org/api/org-units?as_of=...&parent_org_code=...`（子节点）返回 `org_units[].has_children`。
- `has_children` 采用显式布尔值：`true`/`false`，不使用缺省空值表达“未知”。

### 前端 DTO 契约
- `OrgUnitAPIItem.has_children` 视为树渲染必需字段；缺失时按错误契约记录并在 UI 侧 fail-closed 为不可展开。
- `TreePanelNode` 增加 `hasChildren: boolean`，并由 `buildTreeNodes(...)` 显式赋值。

### 可见性一致性契约
- `include_disabled=false` 时，`has_children` 基于“可见子节点（active）”计算。
- `include_disabled=true` 时，`has_children` 基于“可见子节点（active + disabled）”计算。
- 上述计算均受同一 `as_of` 约束，不允许跨日期混算。

## 实施步骤（拟定）
1. [X] **契约补齐（API）**：统一根节点与子节点列表的 `has_children` 输出口径，确保前端首屏可获得该信号。
2. [X] **模型补齐（前端）**：为树节点引入“是否有子节点”的显式字段（如 `hasChildren`），不再仅依赖 `children` 是否已加载。
3. [X] **视图改造（前端）**：树组件按 `hasChildren` 展示展开指示符；展开时再触发 `ensureChildrenLoaded` 拉取真实子节点。
4. [X] **回归验证**：覆盖“根节点/二级节点/叶子节点/搜索定位路径”四类场景，确认指示符与实际子树一致。

## 实施结果
- 后端：
  - 根节点与子节点列表均输出 `has_children`。
  - `ListNodesCurrent` / `ListNodesCurrentWithVisibility` 在查询层直接计算 `has_children`，并由 API 透传。
- 前端：
  - `TreePanelNode` 新增 `hasChildren`，并以该字段驱动展开指示符展示。
  - 树展开事件触发懒加载；节点选择不再隐式触发子节点加载，避免重复请求。
  - `Org Units` 构树逻辑改为“未加载时信任 `has_children`，已加载后以真实 children 覆盖”。

## 回滚与迁移策略
- 回滚原则：仅回滚本次树可展开指示符改动，不新增 legacy 分支或并行读链路。
- 若后端 `has_children` 输出异常：前端保留当前懒加载能力，临时按“仅已加载 children 可展开”降级，同时记录缺失契约并阻断合并。
- 若前端渲染异常：可先回退 TreePanel 的 `hasChildren` 消费层变更，再独立修复 API 契约补齐，避免前后端耦合回滚。
- 不涉及数据库 schema 迁移；无 Atlas/Goose 变更路径。

## 验收标准
- 未点击前，存在下级的节点必须显示可展开指示符。
- 叶子节点不显示可展开指示符。
- 展开可展开节点后，子节点加载成功且结构一致；无多余重复请求。
- `include_disabled` 与 `as_of` 条件下，`has_children` 判断与可见性规则一致。

## 风险与注意事项
- 若仅改前端不补齐根节点 `has_children`，首层节点仍可能无法正确显示指示符。
- 若仅依赖“是否已加载 children”作为判断，会再次回到当前问题。
- 需与 `DEV-PLAN-017` 路由契约、`DEV-PLAN-012` 质量门禁保持一致，不新增旁路接口。

## 门禁与证据（对齐 DEV-PLAN-003）
- 触发器命中预期：
  - Go（若改 `internal/server/orgunit_api.go` / `internal/server/orgunit_nodes.go`）
  - Web UI（若改 `apps/web/src/components/TreePanel.tsx` / `apps/web/src/pages/org/OrgUnitsPage.tsx`）
  - 文档（本计划与执行记录）
- 实施阶段最小必跑：
  - Go：`go fmt ./... && go vet ./... && make check lint && make test`
  - UI：`make generate && make css`，且 `git status --short` 无未提交生成物漂移
  - 文档：`make check doc`
  - PR 前：`make preflight`
- 证据形态：
  - 在对应执行日志记录“时间戳 + 命令 + 结果”；
  - PR 描述显式列出“命中触发器 + 实跑命令 + 通过截图/日志摘要”。

## 关联文档
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/096-org-module-full-migration-and-ux-convergence-plan.md`
