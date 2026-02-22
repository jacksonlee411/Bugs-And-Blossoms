# DEV-PLAN-102C-T：102C1-102C3 测试方案（同租户跨 BU 字段差异）

**状态**: 实施中（2026-02-22 19:05 UTC，已完成首轮验收并给出支持性结论）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 的测试子计划，服务于 `102C1/102C2/102C3` 的验收闭环。
- 本计划聚焦“同一租户下，不同 BU 的字段差异行为”验证，不覆盖 102C4（流程模块暂缓）。
- 本计划输出：测试分层、数据夹具、用例矩阵、支持性评估与阻塞项。

## 1. 测试目标（按用户要求冻结）
1. [x] 同一租户下，不同 BU：某字段对 BU-A 必填，对 BU-B 非必填。  
2. [x] 同一租户下，不同 BU：某字段对 BU-A 可见，对 BU-B 不可见。  
3. [x] 同一租户下，不同 BU：某字段默认值规则在 BU-A 为 `a1`，在 BU-B 为 `b2`。  
4. [x] 对以上 3 项给出“当前是否可支持”的明确结论；若不支持，给出阻塞原因与前置改造项。

## 2. 测试范围与边界
### 2.0 术语对齐（与 102C1/102C2/102C3 一致）
- 本计划以 `business_unit` 作为差异命中主上下文。
- `org_unit_id` 仅作为资源定位上下文（可选），不参与层级命中与冲突消解。

### 2.1 范围
- C1（安全上下文）：验证“角色 + 上下文”判定是否影响字段级行为入口（至少影响写入准入与错误反馈）。
- C2（策略注册表）：验证字段差异策略是否可登记、可检索、可审计。
- C3（命中解释）：验证字段差异命中链路是否可解释（brief/full）。

### 2.2 非范围
- 不验证流程编排差异（102C4 暂缓）。
- 不扩展到全部业务模块；先在 SetID/JobCatalog/Staffing 样板路径做闭环。

## 3. 测试分层
1. **L1 合同测试（API/Service）**
   - 校验字段差异策略输入输出合同、错误码、reason_code。
2. **L2 集成测试（SetID/Registry/Explain）**
   - 验证“策略登记 -> 命中 -> 拒绝/允许 -> explain 输出”链路一致性。
3. **L3 E2E（UI）**
   - 验证同租户 BU-A/BU-B 切换后，页面字段必填/可见/默认值行为差异。
4. **L4 审计与可解释性**
   - 验证 `trace_id/request_id/reason_code/capability_key` 留证完整。

## 4. 测试数据夹具（同租户双 BU）
- Tenant：`T1`
- BU：`BU-A`、`BU-B`
- 可选资源定位：`org_unit_id=10000001`（仅用于资源定位，不参与差异命中）
- 统一 capability 样板键：`staffing.assignment_create.field_policy`
- 样板字段：`field_x`（测试字段）
- 规则设定目标：
  - BU-A：`required=true` / `visible=true` / `default_rule=a1`
  - BU-B：`required=false` / `visible=false` / `default_rule=b2`

## 5. 用例矩阵（核心）
### 5.1 目标 1：必填差异（A 必填 / B 非必填）
- `TC-REQ-001`（BU-A）：
  - 前置：登录同一租户，切换上下文到 BU-A。
  - 操作：提交表单时缺失 `field_x`。
  - 期望：提交失败；返回稳定 reason_code；UI 给出字段级错误与下一步建议。
- `TC-REQ-002`（BU-B）：
  - 前置：同租户切换到 BU-B。
  - 操作：提交表单时缺失 `field_x`。
  - 期望：允许提交（若其它约束满足）。

### 5.2 目标 2：可见性差异（A 可见 / B 不可见）
- `TC-VIS-001`（BU-A）：
  - 期望：`field_x` 可见、可输入（或只读，取决于策略）。
- `TC-VIS-002`（BU-B）：
  - 期望：`field_x` 不在主表单展示；若通过调试参数强行提交应 fail-closed。

### 5.3 目标 3：默认值差异（A=a1 / B=b2）
- `TC-DEF-001`（BU-A）：
  - 期望：新建时 `field_x` 自动带出 `a1`，explain 可追溯规则来源。
- `TC-DEF-002`（BU-B）：
  - 期望：新建时 `field_x` 自动带出 `b2`，并与 BU-A 明显不同。

## 6. 支持性评估（当前结论）
| 测试目标 | 当前支持性 | 结论 | 证据 |
| --- | --- | --- | --- |
| 目标1（必填差异） | 部分支持 | **策略层已支持，业务写入层待接入** | `TestSetIDStrategyRegistryRuntime_BUFieldVarianceAcceptance` + `TestHandleSetIDExplainAPI_BUVarianceAcceptance` 已验证 BU-A required=true、BU-B required=false。 |
| 目标2（可见性差异） | 部分支持 | **策略层已支持，业务字段渲染层待接入** | 同上测试已验证 BU-A visible=true、BU-B visible=false，且 explain 返回 deny + `FIELD_HIDDEN_IN_CONTEXT`。 |
| 目标3（默认值差异） | 支持 | **已支持** | 同上测试已验证 BU-A 默认值 `a1`、BU-B 默认值 `b2`，并由 explain 输出 `resolved_default_value`。 |

> 结论：102C1-102C3 在 **策略登记 + 命中解释** 维度已完成同租户跨 BU 差异验收；“业务写入字段必填阻断/页面字段显隐”仍需业务表单执行器接入。

## 7. 阻塞项与前置改造（必须先做）
1. [x] **字段策略合同冻结（必填/可见/默认）**  
   - 102C2 运行时已支持 `field_key + required + visible + default_rule + business_unit_id` 最小闭环。
2. [ ] **运行时执行器接入（业务写入层）**  
   - 业务表单提交链路仍需接入字段级 required/visible 阻断，当前仅在策略命中与 explain 层可验。
3. [ ] **默认值规则引擎对接（业务赋值层）**  
   - 当前默认值差异已可解释；自动回填到业务表单仍需与 `DEV-PLAN-120` 规则引擎收口。
4. [x] **解释链路补全**  
   - 102C3 已输出字段级 explain（`field_key/default_rule_ref/resolved_default_value/decision/reason_code`）。

## 8. 验收标准（本测试计划）
- [ ] 3 类差异（必填/可见/默认）均有 API + E2E + 审计证据。
- [ ] 同租户 BU-A/BU-B 对照测试可稳定复现差异，不依赖手工改数据。
- [ ] deny 路径统一返回可解释 reason_code，且 UI 有可执行下一步动作。
- [ ] 若目标未实现，测试报告必须标记为 `Blocked` 并附阻塞原因与承接计划。

## 9. 里程碑
1. [ ] **M1 用例冻结**：冻结本计划用例与夹具数据。
2. [ ] **M2 合同就绪**：字段策略执行合同落地后，联调 L1/L2。
3. [ ] **M3 E2E 回归**：完成 L3 用例并产出截图/录屏证据。
4. [ ] **M4 评审结论**：形成“支持/不支持”最终判定与后续行动清单。

## 10. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/102c5-ui-design-for-setid-context-security-registry-explainability.md`
- `docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`
- `docs/dev-plans/060-business-e2e-test-suite.md`
- `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`

## 11. 本次验收证据（2026-02-22）
- `go test ./internal/server -run "BUVarianceAcceptance|SetIDExplain|SetIDStrategyRegistry" -count=1`
- `make test`
- `make preflight`（含前端构建、路由门禁、E2E）
