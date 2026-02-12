# DEV-PLAN-083：Org 变更能力模型重构（抽象统一 + 策略单点 + 能力外显）

**状态**: 规划中（2026-02-11 10:42 UTC）

## 1. 背景

基于 `DEV-PLAN-082`，当前 OrgUnit 的“字段可改范围”虽符合业务语义，但实现上存在结构性问题：

1. 规则分散在 Service/UI/Kernel，难以维护且容易漂移。
2. UI 缺少可编辑能力先验，出现“可输入但提交后失败（`PATCH_FIELD_NOT_ALLOWED`）”。
3. 规则表达停留在“字段白名单”，尚未上升到“变更语义模型”，扩展时容易出现无效组合。

本计划以“抽象统一”作为第一目标：先统一变更语义与策略结构，再落地代码与 API。

## 2. 抽象模型（本计划 SSOT）

## 2.1 核心定义：历史变换语言（History Transformation DSL）

OrgUnit 写入不是“改字段”，而是“对历史事实做受限变换”。

为此，定义四层模型：

- `action_kind`（意图层，Intent）：用户在做什么动作。
- `emitted_event_type`（执行层，Opcode）：系统实际写入哪种事件。
- `field_key/payload_key`（参数层，Operands）：可变更字段与落库 payload 键位映射。
- `preconditions`（约束层，Guards）：字段白名单之外的前置业务约束。

## 2.2 三类原子变换（最小完备基）

- `Append`（追加事实）
  - 对应动作：`create`、`event_update`
  - 对应事件：`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`
- `Rewrite`（改写解释）
  - 对应动作：`correct_event`、`correct_status`
  - 对应事件：`CORRECT_EVENT/CORRECT_STATUS`
- `Invalidate`（失效既有事实）
  - 对应动作：`rescind_event`、`rescind_org`
  - 对应事件：`RESCIND_EVENT/RESCIND_ORG`

> 判定规则：仅当新需求不能归入 `Append/Rewrite/Invalidate` 时，才允许新增事件语义。

## 2.3 完备性边界

- 本模型覆盖 **OrgUnit 写入域**（create/update/correct/rescind）。
- 本模型不覆盖读取域（Read）与跨模块语义（person/staffing/jobcatalog）。

## 3. 统一词表与命名冻结

- `action_kind`：`create` / `event_update` / `correct_event` / `correct_status` / `rescind_event` / `rescind_org`
- `emitted_event_type`：`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT/CORRECT_EVENT/CORRECT_STATUS/RESCIND_EVENT/RESCIND_ORG`
- `target_effective_event_type`：`CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT`（仅 `correct_*` / `rescind_*` 使用）

命名冻结规则：

- 服务层与内核事件枚举统一采用现有大写常量（例如 `CORRECT_STATUS`、`RESCIND_EVENT`）。
- 禁止引入并行小写事件常量，避免双词表。

## 4. 策略模型设计（Policy Matrix）

在 `modules/orgunit/services/` 新增策略模块（建议：`orgunit_mutation_policy.go`），并作为写入策略单点。

## 4.1 策略键（Policy Key）

统一键：

- `(action_kind, emitted_event_type, target_effective_event_type?)`

语义要求：

- `target_effective_event_type` 仅在需要目标事件的动作中出现（`correct_*` / `rescind_*`）。
- 对不合法组合（如 `create + target_effective_event_type!=null`）直接拒绝。

## 4.2 合法组合约束（无效组合 fail-closed）

- `create -> emitted_event_type=CREATE`
- `event_update -> emitted_event_type in {MOVE,RENAME,DISABLE,ENABLE,SET_BUSINESS_UNIT}`
- `correct_event -> emitted_event_type=CORRECT_EVENT + target_effective_event_type required`
- `correct_status -> emitted_event_type=CORRECT_STATUS + target_effective_event_type required`
- `rescind_event -> emitted_event_type=RESCIND_EVENT + target_effective_event_type required`
- `rescind_org -> emitted_event_type=RESCIND_ORG + target_effective_event_type must be null`

## 4.3 字段与 payload 映射规则

策略输出不仅有允许字段，还包含 `field_key -> payload_key` 映射：

- `name`：
  - target=`CREATE` -> `name`
  - target=`RENAME` -> `new_name`
- `parent_org_code`：
  - target=`CREATE` -> `parent_id`
  - target=`MOVE` -> `new_parent_id`
- `is_business_unit`：
  - target=`CREATE|SET_BUSINESS_UNIT` -> `is_business_unit`
- `manager_pernr`：
  - target=`CREATE` -> `manager_uuid + manager_pernr`（含人员解析）

默认规则：

- 未显式声明的字段一律拒绝（fail-closed）。

## 4.4 前置约束（preconditions）

`preconditions` 用于表达非字段白名单约束，至少包含：

- `target_exists`
- `target_not_rescinded`
- `root_guard`
- `children_guard`
- `dependency_guard`
- `date_window_guard`
- `same_day_uniqueness_guard`

要求：

- capabilities API 需把不满足的约束转为 `deny_reasons`，供 UI 解释。
- 服务层与 Kernel 错误码保持稳定对齐。

## 4.5 统一策略接口

建议接口：

- `ResolvePolicy(actionKind, emittedEventType, targetEffectiveEventType) (PolicyDecision, error)`
- `AllowedFields(decision) []FieldRule`
- `ValidatePatch(decision, patch) error`

其中 `PolicyDecision` 至少包含：

- `enabled`
- `allowed_fields`
- `field_payload_keys`
- `preconditions`
- `deny_reasons`

## 5. 能力外显 API（Capabilities）

新增只读 API：

- `GET /org/api/org-units/mutation-capabilities?org_code=<...>&effective_date=<...>`

返回契约（抽象）：

- `effective_target_event_type`：effective 视图下的目标事件类型
- `raw_target_event_type`：原始事件类型（审计对照）
- `capabilities`：
  - `correct_event`：`enabled` + `allowed_fields` + `field_payload_keys` + `deny_reasons`
  - `correct_status`：`enabled` + `allowed_target_statuses` + `deny_reasons`
  - `rescind_event`：`enabled` + `deny_reasons`
  - `rescind_org`：`enabled` + `deny_reasons`

契约约束：

- 不需要目标事件的动作不得返回伪 `target_effective_event_type`。
- API 不可用时，UI 进入只读或保守禁用，不允许乐观放行。

## 6. 实施步骤

## 6.1 文档与契约先行
1. [ ] 固化本文件为策略模型 SSOT（四层模型 + 三类原子变换 + 合法组合约束）。
2. [ ] 在 `internal/server/orgunit_api.go` 明确 capabilities 响应字段与错误码约束。

## 6.2 服务层改造
3. [ ] 新增 `orgunit_mutation_policy.go`，实现 `ResolvePolicy/AllowedFields/ValidatePatch`。
4. [ ] 重构 `buildCorrectionPatch(...)`，仅通过策略模块判定字段与 payload 映射。
5. [ ] 将 `CorrectStatus(...)`、`RescindRecord(...)`、`RescindOrg(...)` 的可用性判断收敛到同一策略模块。
6. [ ] 保持行为等价，不在该步骤引入新业务语义。

## 6.3 API 与 UI 联动
7. [ ] 新增 mutation capabilities API（含 `deny_reasons`）。
8. [ ] 详情页按 capabilities 执行字段禁用/隐藏与动作可用性控制。
9. [ ] 错误提示升级为“不可用原因可解释”，减少提交后失败。

## 6.4 Kernel 对齐与防回归
10. [ ] 复核 `submit_org_event_correction/submit_org_status_correction/submit_org_event_rescind/submit_org_rescind` 与服务层规则对齐。
11. [ ] 保留 Kernel 防守性校验，确保绕过服务层时仍 fail-closed。

## 7. 验收标准

- [ ] 规则判定从散落分支收敛为策略单点，可审计可测试。
- [ ] capabilities API 能返回“目标事件类型 + 动作能力 + 字段映射 + 拒绝原因”。
- [ ] UI 常见路径不再出现“可编辑但必然失败”的字段。
- [ ] UI 对禁用动作给出稳定可解释原因。
- [ ] `PATCH_FIELD_NOT_ALLOWED`、`ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET` 等稳定错误码不漂移。
- [ ] 无 legacy 分支，无第二写入口（One Door 保持）。

## 8. 测试与门禁

### 8.1 本地必跑（触发器命中）
- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 文档：`make check doc`

### 8.2 推荐增量测试
- `modules/orgunit/services/orgunit_mutation_policy_test.go`
  - 合法组合覆盖
  - 无效组合拒绝（table-driven）
  - `field_key -> payload_key` 映射覆盖
- `modules/orgunit/services/orgunit_write_service_test.go`
  - `buildCorrectionPatch` 与策略集成回归
- `internal/server/orgunit_api_test.go`
  - capabilities API 合约测试（含 deny reason）
- `internal/server/orgunit_nodes_test.go`
  - UI 禁用/提示回归

## 9. 风险与权衡

- 风险：抽象层级提升后，短期理解成本上升。  
  缓解：先在 OrgUnit 写入域收口，不跨模块泛化。

- 风险：动作语义与事件语义再次混用，回到“可表达无效组合”。  
  缓解：使用统一策略键并加启动期自检（非法组合即失败）。

- 风险：UI 过度依赖 capabilities API。  
  缓解：API 故障时 fail-closed（只读/禁用），不做隐式放行。

- 风险：服务层与 Kernel 边界口径偏移。  
  缓解：契约测试固定错误码和边界输入，并在评审中同时检查 Service/Kernel。

## 10. 与既有计划关系

- 语义事实源：`docs/dev-plans/082-org-module-field-mutation-rules-investigation.md`
- 状态纠错边界：`docs/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
- correction 失败排障与错误码稳定化：`docs/dev-plans/080b-orgunit-correction-failure-investigation-and-remediation.md`
- 审计快照约束：`docs/dev-plans/080c-orgunit-audit-snapshot-presence-table-constraint-plan.md`

## 11. 关联文档

- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/017-routing-strategy.md`
- `AGENTS.md`
