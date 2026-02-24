# DEV-PLAN-161A：SetID Capability Registry 可编辑与可维护化（承接 160/161）

**状态**: 已完成（2026-02-24 06:52 UTC）

## 1. 背景与问题陈述（Context）
- 现网页面 `/app/org/setid` 的 Registry 区域可展示 `capability_key` 列表，但用户反馈“无法删除、编辑”。
- `DEV-PLAN-160` 已完成 Registry 最小可用闭环（筛选 + 列表 + upsert），但未交付“行级编辑/删除”的维护体验。
- 161 主线聚焦 Org 新建字段策略消费；本 161A 作为治理台可维护性补完，避免策略长期只能“追加/手工覆盖”，形成维护成本与误操作风险。

## 2. 调查结论（Root Cause）
### 2.1 前端交互缺口（直接原因）
- Registry 列表只渲染 DataGrid，无 `Edit/Delete` 操作列，且未实现“点击行回填表单”。
- 当前“编辑”只能靠人工复制主键字段后再执行 upsert，属于低可用维护路径。

### 2.2 API 契约缺口（删除不可达）
- 客户端仅暴露 `listSetIDStrategyRegistry` + `upsertSetIDStrategyRegistry`，无删除 API。
- 服务端 `handleSetIDStrategyRegistryAPI` 仅支持 `GET/POST`；无 `DELETE` 或 `:disable`。
- 路由 capability 映射与 authz 测试明确“DELETE 不存在映射”，导致删除链路在契约层被禁止。

### 2.3 权限可见性分层（次要原因）
- 页面允许 `orgunit.read` 访问，但关键动作受 `setid.governance.manage` 控制；未具备该权限时呈现只读骨架。
- 即使权限满足，仍因 2.1/2.2 缺口无法完成行级编辑/删除。

## 3. 目标与非目标（Goals / Non-Goals）
### 3.1 核心目标
- [x] 在 Registry 列表提供可发现的行级“编辑/删除（停用）”操作。
- [x] 建立删除的正式 API 契约（优先逻辑删除，保留历史可追溯）。
- [x] 前后端统一 fail-closed 与错误码，删除/编辑均可追踪 `request_id/trace_id`。
- [x] 不引入 legacy 双链路；保持 capability_key 与路由映射门禁一致。

### 3.2 非目标
- 不改动 Capability Key 命名规范与注册表主键模型。
- 不重写 Activation/Functional Area/Explain 子页。
- 不引入新模块或绕过现有路由治理与 Authz 门禁。

## 4. 方案总览（Proposed Design）
### 4.1 交互层（MUI）
1. Registry DataGrid 新增 `actions` 列：`编辑`、`删除`。
2. `编辑`：将行数据回填到 Upsert 表单（含 key 字段锁定提示），保存后刷新当前 queryKey。
3. `删除`：弹出确认框（显示 capability_key/field_key/org_level/business_unit_id/effective_date），提交后刷新列表。
4. 新增“只读原因提示”统一文案：无 `setid.governance.manage` 时明确说明仅可查看不可维护。

### 4.2 API 契约（新增）
- 新增：`POST /org/api/setid-strategy-registry:disable`
  - 请求体（最小集）：
    - `capability_key`
    - `field_key`
    - `org_level`
    - `business_unit_id`
    - `effective_date`
    - `disable_as_of`（首个失效日；后端归一为 `end_date`）
    - `request_id`
  - 语义：逻辑删除（通过设置 `end_date` 使策略在目标日期后失效），不做物理删除。
  - 时间规则冻结：
    - `disable_as_of` 定义为“首个失效日”。
    - 后端换算：`end_date = disable_as_of`。
    - 强约束：`disable_as_of > effective_date`（禁止同日失效/零长度有效期）。
- 保持：`POST /org/api/setid-strategy-registry` 用于新增/更新。

### 4.3 存储与约束
- 复用 `orgunit.setid_strategy_registry`，不新增第二事实源。
- 通过 `end_date` 实现可逆维护与历史追溯；必要时补充同日失效语义约束（避免“今日误配无法撤回”）。
- 停用预检：
  - 对目标上下文执行“停用后可解析性预检”；若停用会导致无可命中策略，拒绝停用（fail-closed，稳定错误码）。
- 恢复语义：
  - `disable_as_of` 尚未生效（未来）时，允许撤销停用（清空 `end_date`）。
  - 已生效停用禁止回写历史；恢复必须新增一条新的 `effective_date` 版本记录。

### 4.4 编辑模型冻结（UI）
- 行级维护拆分为两个动作，避免时间主键误改：
  1. `编辑当前版本`：仅允许修改非主键字段（主键字段只读）。
  2. `另存为新版本`：复制当前记录并指定新的 `effective_date`，生成新版本。

### 4.5 鉴权与路由治理
- 新增 disable 路由到 capability-route-map：
  - `POST /org/api/setid-strategy-registry:disable` -> `org.setid_capability_config` + `admin`
- 同步更新：
  - `internal/server/capability_route_registry.go`
  - authz requirement 测试
  - capability-route-map 门禁测试

## 5. 实施拆分（Milestones）
1. [x] **M1 契约冻结**：冻结 disable API 请求/响应、错误码、权限语义、`disable_as_of` 时间语义、恢复语义、编辑模型（编辑当前/另存新版本）。
2. [x] **M2 后端落地**：store 增加 disable 能力；handler/route/authz/capability-map 全链路补齐。
3. [x] **M3 前端落地**：DataGrid actions + 编辑回填 + 删除确认 + 成功/失败提示。
4. [x] **M4 测试补齐**：Go 单测（handler/store/authz/route-map）+ 前端交互测试 + E2E 用例。
5. [x] **M5 门禁与证据**：触发器命中项跑绿并沉淀 `docs/dev-records/`。

## 6. 错误码与失败路径（Failure Paths）
- `setid_strategy_registry_disable_failed`：停用失败。
- `invalid_disable_date`：失效日期非法。
- `FIELD_POLICY_CONFLICT`：失效日期与生效日期冲突。
- `capability_context_mismatch`：上下文与 capability 不匹配。
- `FIELD_POLICY_DISABLE_NOT_ALLOWED`：停用后将导致策略不可解析（禁止停用）。
- 统一要求：错误提示映射 `error-message` 门禁，前端展示明确下一步。

## 7. 验收标准（Acceptance Criteria）
- [x] Registry 每行可见并可触发“编辑/删除”操作（有权限）。
- [x] 编辑支持“一键回填 + 保存 + 列表即时刷新”。
- [x] 编辑支持“编辑当前版本（主键只读）/另存为新版本（新 effective_date）”双动作。
- [x] 删除后目标策略在指定 `as_of` 下不再出现，且历史可追溯。
- [x] 同日失效请求被拒绝（`disable_as_of <= effective_date` 失败）。
- [x] 停用导致“无可命中策略”时被拒绝并返回稳定错误码。
- [x] 未来停用可撤销；已生效停用只能通过新增版本恢复。
- [x] 无权限用户仅可查看，不可执行编辑/删除，提示明确。
- [x] `make check capability-route-map`、`make check routing`、`make check error-message`、`make check no-legacy`、`make test` 通过。

## 8. 风险与缓解
- **R1：删除语义与 Valid Time 冲突**  
  缓解：采用逻辑删除（`end_date`），并在同日撤回场景补齐明确规则与测试。
- **R2：前后端权限口径不一致**  
  缓解：以前端按钮权限 + 后端 authz 双重校验，拒绝时返回稳定错误码。
- **R3：路由映射漂移导致门禁失败**  
  缓解：变更同 PR 内完成 route-map 与对应测试更新。

## 9. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/140-error-message-clarity-and-gates.md`
- `AGENTS.md`
