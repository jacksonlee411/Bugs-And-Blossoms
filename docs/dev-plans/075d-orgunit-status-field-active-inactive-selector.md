# DEV-PLAN-075D：OrgUnit 页面状态字段与“有效/无效”显式切换方案（评审修订版）

**状态**: 已完成（2026-02-07）

## 背景
- DEV-PLAN-075C 已完成“删除记录=事件撤销（rescind）”语义收敛，但页面仍存在状态可见性与可操作性缺口。
- 当前 OrgUnit Details 页头部状态徽标固定写死为 `Active`，未展示真实记录状态。
- 当前 UI 仅保留“停用（disable）”历史动作路径，缺少“启用（enable）”对称入口，导致“状态变更”体验不完整。

## 现状评估（承接 DEV-PLAN-075C）
1. 页面未展示真实状态：
   - 详情模型无 `status` 字段，头部固定渲染 `Active`。
2. 读取路径默认过滤 `active`：
   - 详情/列表/搜索大量查询使用 `v.status = active`，禁用后组织不可达。
3. 写入能力不对称：
   - 内核支持 `DISABLE/ENABLE`，但 Go 层仅暴露 `Disable`，未暴露 `Enable`。
4. 结果：
   - 用户无法在页面显式完成“有效/无效”切换，也无法确认当前记录状态。

## 目标
1. 在 OrgUnit 页面显式展示记录状态字段（`有效` / `无效`）。
2. 在页面提供显式状态切换入口，支持 `active <-> disabled` 双向切换。
3. 补齐“无效记录可达性”，确保禁用后仍可在 UI 中找到并恢复。
4. 保持 One Door：状态变更仅通过内核事件（`ENABLE`/`DISABLE`）提交。
5. 不引入 legacy 双链路，不新增临时兼容分支。

## 非目标
- 不改动删除语义（`delete_record/delete_org` 仍按 075C 的 rescind 语义）。
- 不新增审批链/独立后台入口。
- 本轮不新增数据库表（沿用现有事件与投射结构）。

## 关键决策（本次评审冻结）
### 1) 可达性契约（解决 disabled 不可见）
- 默认保持“仅显示有效组织”（不改变现有主视图认知负担）。
- 新增“显示无效组织”开关（`include_disabled=1`），作用于：
  - 左侧树与列表读取；
  - 搜索候选与搜索定位；
  - 详情读取。
- 当 `include_disabled=0` 且用户通过直达链接打开无效组织时：
  - 允许详情页展示该组织；
  - 页面提示“当前组织为无效状态，可切换为有效”。

### 2) 操作矩阵（避免“字段变更 + 状态变更”语义冲突）
- `add_record` / `insert_record`：本轮不承载状态切换，状态固定沿既有规则处理。
- `correct_record`：仅处理字段修正，不承载状态切换。
- 新增独立动作 `change_status`（显式下拉）：
  - 输入：`org_code + effective_date + target_status`。
  - 不复用 `record_change_type`，避免组合歧义。
- `delete_record` / `delete_org`：状态字段不参与提交。

### 3) 服务端判定规则（fail-closed）
- 前端不提交 `current_status`，仅提交 `target_status`。
- 服务端在事务内读取当前状态并判定：
  - 当前状态 = 目标状态：返回“未检测到状态变更”（不写事件）。
  - `active -> disabled`：提交 `DISABLE`。
  - `disabled -> active`：提交 `ENABLE`。
- `target_status` 非法或缺失：400（稳定错误码，fail-closed）。

### 4) 兼容与不变量
- 状态切换写入必须走 `submit_org_event(..., DISABLE/ENABLE, ...)`。
- No Tx, No RLS / One Door / fail-closed 继续有效。
- 不增加第二写入口，不引入 legacy 回退通道。

## 用户与交互契约（UI）
### 1) 详情区状态展示
- 头部状态徽标改为动态值：
  - `active` -> `有效`
  - `disabled` -> `无效`
- 详情“基本信息”中新增只读状态字段。

### 2) 显式状态变更入口
- 在记录操作区域新增按钮：`状态变更`。
- 打开后展示独立表单：
  - `生效日期`（默认当前选中版本）；
  - `状态` 下拉（`有效` / `无效`）；
  - 提交按钮（根据目标状态显示“启用”或“停用”）。
- 默认值规则：
  1. 用户已手动选择：保持用户选择。
  2. 否则默认当前选中版本的真实状态。

### 3) 提示与错误
- 失败提示统一映射稳定错误码（400/404/409/403）。
- 403（缺权限）保持 fail-closed，文案与 JSON/HTML 语义一致。
- 无状态变更（同值提交）显示非错误提示，不落库。

## 应用层与路由契约
1. Service 层补齐对称能力：新增 `Enable(ctx, tenantID, req)`。
2. Internal API 新增：`POST /org/api/org-units/enable`。
3. Routing/Authz：
   - allowlist 增加 `/org/api/org-units/enable`。
   - authz 要求与 `/disable` 一致（orgunit admin）。
4. UI POST 处理：
   - 新增 `action=change_status` 分支。
   - UI 仅传 `target_status`，后端自行读取当前状态并决策。

## 数据与读取契约
- 详情读取需透传 `status`（不再硬编码 Active）。
- 树/搜索/详情读路径支持 `include_disabled`，默认 `0`。
- 不新增表；沿用现有事件与投射结构。

## 分阶段实施（建议每阶段独立 PR）
1. [x] **P0（契约冻结）**：冻结可达性方案（`include_disabled`）、操作矩阵、错误码矩阵。
2. [x] **P1（读路径与展示）**：详情 `status` 透传；树/搜索/详情接入 `include_disabled`。
3. [x] **P2（写路径对称）**：补 `Enable` service + API + routing/authz；UI 接入 `change_status` 独立表单。
4. [x] **P3（测试与门禁）**：补双向切换、可达性、冲突/403 回归，跑通质量门禁并收口文档。

## 执行记录
- `docs/dev-records/dev-plan-075d-execution-log.md`

## 验收标准
- 页面可见真实状态字段，且不再固定显示 `Active`。
- 页面存在显式状态下拉入口，用户可完成“有效/无效”切换。
- `active <-> disabled` 双向切换可用，行为与错误码可预测。
- 禁用后的组织在同会话与新会话均可达，并可恢复为有效。
- 不破坏 075C 已交付的 rescind 语义与路径。

## 最小测试清单
- T1: 当前为有效，选择无效并提交 -> 200/302，状态变更为无效。
- T2: 当前为无效，选择有效并提交 -> 200/302，状态变更为有效。
- T3: 目标状态与当前状态一致 -> 提示“未检测到状态变更”，不写事件。
- T4: 缺权限提交 -> 403（JSON/HTML 一致）。
- T5: 生效日冲突或业务冲突 -> 409 + 稳定错误码。
- T6: 详情页切换版本后状态字段同步刷新。
- T7: `include_disabled=0` 与 `include_disabled=1` 的树/搜索/详情行为符合契约。
- T8: 禁用后通过“显示无效组织”或直达详情可再次进入并成功启用。
- T9: 陈旧页面提交（并发改动后）返回可预测错误，不出现脏写。
- T10: `delete_record` / `delete_org` 不读取或提交状态字段。

## 触发器与门禁
- Go/Router/Authz/UI 变更：
  - `go fmt ./... && go vet ./...`
  - `make check lint && make test`
  - `make check routing`
  - `make authz-pack && make authz-test && make authz-lint`
- 文档收敛：
  - `make check doc`

## 关联文档
- `docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- `docs/dev-plans/074-orgunit-details-update-ui-optimization.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
