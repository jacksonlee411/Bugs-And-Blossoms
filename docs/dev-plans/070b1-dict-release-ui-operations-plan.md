# DEV-PLAN-070B1：字典基线发布 UI 可视化操作方案（承接 DEV-PLAN-070B）

**状态**: 规划中（2026-02-22 21:05 UTC）

## 1. 背景与问题 (Context)
- `DEV-PLAN-070B` 已完成后端“发布到租户本地”能力（preview/release API、权限、tenant-only 运行时、门禁与脚本）。
- 当前缺口是**用户可见性**：虽然后端可用，但在 `/dicts` 页面尚无“预检发布/执行发布”可操作入口。
- 按仓库“新增功能必须可发现、可操作”的原则，070B 需要补齐 UI 闭环，避免“后端已交付、用户不可用”的僵尸功能。
- 本计划聚焦“页面可操作 + 契约冻结 + 验收证据”，不引入新的后端写链路。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 在现有字典配置页面提供“发布到当前租户”可视化入口（预检 + 执行）。
- [ ] 用户可在页面完成完整链路：输入参数 -> 预检 -> 查看冲突 -> 执行发布 -> 查看结果。
- [ ] 与 070B 后端契约严格对齐，不引入第二写入口，发布仍走 One Door。
- [ ] 输出可审计信息：`release_id/request_id/as_of/计数结果/started_at/finished_at`。

### 2.2 非目标
- 不在本计划新增数据库表或迁移。
- 不在本计划改造为异步任务中心（先支持当前同步发布接口的 UI 闭环）。
- 不在本计划扩展到 scope package 全域发布（保持“字典样板”范围）。
- 不新增 `/dicts` 之外的新页面路由（仅在现有页面内扩展发布操作区）。

### 2.3 工具链触发器（SSOT 引用）
- [ ] Go 代码（若涉及后端 handler/契约调整）
- [ ] Web UI（`make generate && make css`，如命中生成物）
- [ ] Routing（仅当新增路由；本计划默认不命中）
- [ ] Authz（如新增 `permissionKey` 或策略映射）
- [ ] E2E（补齐 UI 发布流用例）
- [ ] 文档（`make check doc`）

> 命令与门禁以 `AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml` 为 SSOT，本计划不复制脚本实现细节。

## 3. 页面关系与 IA 冻结 (IA & Page Relationship)
### 3.1 与现有页面关系（冻结）
- **不新增路由、不新增导航项**。
- 在现有 `/dicts` 页面（`DictConfigsPage`）内新增“字典发布（Tenant Baseline Release）”操作区。
- 页面访问权限继续沿用现有路由门槛：`permissionKey=dict.admin`（已有字典管理入口不变）。

### 3.2 操作区可见性
- 在 `/dicts` 页面中，发布区采用“独立卡片（推荐）”或“二级 Tab”承载。
- 发布区内的“执行发布”按钮按独立权限控制（见 §5），不因页面可见即默认可执行。

### 3.3 页面最小字段
- `source_tenant_id`（默认 `global_tenant`，允许显式输入）。
- `as_of`（必填，day 粒度，`YYYY-MM-DD`）。
- `release_id`（必填）。
- `request_id`（执行发布时必填；可提供一键生成）。
- `max_conflicts`（可选，默认 200）。

### 3.4 结果区
- 预检摘要：`missing/mismatch` 计数。
- 冲突样例表格：`kind/dict_code/code/source_value/target_value`。
- 发布摘要：`dict/value total/applied/retried` + `status` + `started_at/finished_at`。

## 4. DEV-PLAN-005 标准对齐（冻结）
### 4.1 STD-001（`request_id` / `trace_id`）
- [ ] 发布写入幂等字段统一为 `request_id`，UI 与 API 禁止出现 `request_code` 新增输入。
- [ ] 追踪字段统一为 `trace_id`（若 UI 展示链路追踪信息，禁止复用 `request_id` 表达 tracing）。
- [ ] 错误提示与日志文案明确区分“幂等冲突（request_id）”与“链路追踪（trace_id）”。

### 4.2 STD-002（`as_of` / `effective_date`）
- [ ] 发布预检与执行中，`as_of` 仅表示查询时点，必须显式输入，禁止默认 today。
- [ ] 本页面不引入 `effective_date` 输入；若后续扩展写入生效日能力，须单独立项并遵守 `invalid_effective_date` 契约。
- [ ] 对非法 `as_of`，UI 需透传并展示后端稳定错误码 `invalid_as_of`。

## 5. 权限模型与映射冻结 (Authz Contract)

> 目标：消除“前端 permissionKey 与后端 object/action”断层，防止 UI 可点但后端必拒绝。

### 5.1 权限键与 Casbin 映射
| 场景 | 前端 permissionKey | 后端 object | action | 说明 |
| --- | --- | --- | --- | --- |
| 字典页面访问（现有） | `dict.admin` | `iam.dicts` | `admin` | 维持现状，不变更路由 |
| 发布预检/执行（新增） | `dict.release.admin` | `iam.dict_release` | `admin` | 新增独立发布权限 |

### 5.2 角色建议（冻结）
- `role:tenant-admin`：`iam.dicts admin` + `iam.dict_release admin`。
- `role:tenant-viewer`：无发布权限。
- 无权限行为：前端按钮禁用并提示“无发布权限”，后端保持 403 fail-closed。

### 5.3 约束
- 禁止在前端/后端手写 object/action 字符串；统一走 `pkg/authz` registry 常量。
- 禁止将发布能力并回 `dict.admin`（避免权限边界被放大）。

## 6. 交互状态机与失败路径 (State Machine)
### 6.1 状态定义（冻结）
- `idle`：初始态，可编辑参数。
- `previewing`：预检请求中；预检按钮 loading，执行按钮禁用。
- `conflict`：预检返回冲突（HTTP 409）；展示冲突清单，执行按钮禁用。
- `ready`：预检通过（HTTP 200 且冲突计数为 0）；允许执行发布。
- `releasing`：发布请求中；所有输入与按钮禁用，防重入。
- `success`：发布成功（HTTP 201）；展示结果摘要并提供复制字段操作。
- `fail`：发布失败（4xx/5xx）；保留参数与错误码，允许重试。

### 6.2 转移规则（冻结）
1. `idle -> previewing`：点击“预检发布”。
2. `previewing -> conflict`：收到冲突响应或冲突计数 > 0。
3. `previewing -> ready`：预检通过且冲突计数 = 0。
4. `ready -> releasing`：点击“执行发布”。
5. `releasing -> success`：发布成功。
6. `releasing -> fail`：发布失败。
7. `conflict/fail/success -> previewing`：用户修改参数后重新预检。

### 6.3 失败路径与防呆
- 防重入：`previewing/releasing` 期间禁止重复提交。
- 冲突阻断：`conflict` 态严格禁用“执行发布”。
- 参数校验：`as_of/release_id/request_id` 缺失时前端先拦截，并保留后端错误码透出。
- No Legacy：失败后只允许“修复参数/权限后重试”，不允许回退到旧链路或旁路写入。

## 7. 按钮层级与可访问性/i18n（对齐 DEV-PLAN-002）
### 7.1 按钮层级（冻结）
- 同一“字典发布”任务域仅保留 **1 个 Primary**：`执行发布`（`contained`）。
- `预检发布` 为 **Secondary**（`outlined` 或 `text`）。
- `重置参数` / `复制 ID` 为 Secondary/Text，不得与 Primary 同强调。

### 7.2 A11y 验收条目
- [ ] 键盘可达：可用 Tab 完成输入、预检、发布、复制结果。
- [ ] 焦点可见：不移除 focus ring；错误输入跳转后焦点可见。
- [ ] `IconButton` 均具备 `aria-label`（或 Tooltip + 可读名称）。
- [ ] 错误信息可读（非仅颜色表达），冲突表格具备列头语义。

### 7.3 i18n 验收条目（仅 `en/zh`）
- [ ] 新增文案全部走 i18n key，不得硬编码在组件中。
- [ ] 必备 key：页面标题、字段 label、按钮文案、状态提示、错误码映射提示。
- [ ] 同一错误码在 `en/zh` 文案语义一致（例如 `invalid_as_of`、`forbidden`、`dict_baseline_not_ready`）。

## 8. API 对接与错误码映射
### 8.1 API
- 预检：`POST /iam/api/dicts:release:preview`
- 执行：`POST /iam/api/dicts:release`

### 8.2 关键交互规则
- [ ] 先预检后发布：仅 `ready` 态允许点击“执行发布”。
- [ ] 预检冲突（HTTP 409）必须展示冲突明细并阻断发布。
- [ ] 发布成功（HTTP 201）展示结果摘要并提示刷新列表。
- [ ] 错误码统一映射可读提示，且保留原始 code 便于排障。

## 9. 测试与验收 (Testing & Acceptance)
### 9.1 测试范围
- [ ] 单元测试：参数校验、状态机转移、按钮禁用态、错误映射。
- [ ] 集成测试：preview 成功/冲突、release 成功/失败、403 权限拒绝。
- [ ] E2E：`/dicts` 页面完整链路（预检 -> 发布 -> 结果展示）。

### 9.2 验收标准
- [ ] `/dicts` 页面可直接发现并操作发布功能（满足用户可见性原则）。
- [ ] 发布按钮权限与后端鉴权一致，不出现“UI 放行/后端拒绝”漂移。
- [ ] 冲突场景不会误触发发布写入。
- [ ] 发布成功后可见审计关键字段与结果计数。
- [ ] 通过 `DEV-PLAN-002`（按钮层级/A11y/i18n）与 `DEV-PLAN-003`（边界/不变量/失败路径可解释）评审。

## 10. 实施拆解（建议 PR 轴）
1. [ ] **PR-070B1-1（权限合同 + API Client）**：
   - 新增 `dict.release.admin` 前端权限键与映射文档；
   - 扩展 `apps/web/src/api/dicts.ts` 的 preview/release client 与类型。
2. [ ] **PR-070B1-2（页面可操作入口）**：
   - 在 `DictConfigsPage` 增加“字典发布”操作区与结果区；
   - 落地按钮层级（预检 Secondary、执行发布 Primary）。
3. [ ] **PR-070B1-3（状态机/错误/可访问性收口）**：
   - 落地状态机与防重入；
   - 错误码映射、无权限提示、A11y/i18n key 完整收口。
4. [ ] **PR-070B1-4（测试与证据）**：
   - 补齐前端单测/集成/E2E；
   - 在 `docs/dev-records/` 记录门禁与执行证据。

## 11. 风险与缓解
- 风险：同步发布耗时导致用户误判“卡住”。  
  缓解：loading 态 + 按钮防重入 + 显式展示 `started_at/finished_at`。
- 风险：冲突信息过多导致可读性差。  
  缓解：默认“统计 + 样例”视图，后续迭代再补分页/展开。
- 风险：权限配置未同步导致“页面可见但不能发布”。  
  缓解：按 §5 冻结权限映射，并在联调阶段增加 403 用例。

## 12. 关联文档
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-records/dev-plan-070b-execution-log.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/002-ui-design-guidelines.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`
- `AGENTS.md`
