# DEV-PLAN-163A：SetID Governance 其余三页签字段下拉化收敛方案

**状态**: 已完成（2026-02-24 13:57 UTC）

## 1. 背景
- `DEV-PLAN-163` 已完成 `Strategy Registry` 页签的字段下拉化收敛。
- `SetID Governance` 其余三页签（`Governance` / `Security Context` / `Explainability`）仍存在大量自由输入，交互一致性与录入稳定性不足。
- 按 `AGENTS.md` 的“契约优先 + 用户可见性原则”，先冻结字段评估矩阵，再实施统一改造。

## 2. 目标与非目标
### 2.1 目标
- [X] 冻结其余三页签的“可下拉化字段”评估矩阵。
- [X] 完成 `Governance` 页签（Create/Bind）字段下拉化。
- [X] 完成 `Security Context` 与 `Explainability`（共享 `SetIDExplainPanel`）字段下拉化。
- [X] 保持现有 API 契约与 payload 字段名不变。

### 2.2 非目标
- 不新增后端字典/枚举接口。
- 不调整后端校验语义与错误码契约。
- 不改动 `Strategy Registry`（该范围已由 `DEV-PLAN-163` 覆盖）。

## 3. 字段评估矩阵（冻结）
### 3.1 Governance 页签
| 区块 | 字段 | 评估结论 | 收敛方式 |
| --- | --- | --- | --- |
| 全局上下文 | `as_of` | 不纳入 | 保持日期控件 |
| Create SetID | `setid` | 可下拉化 | 可搜索下拉（现有 SetID + 自定义输入） |
| Create SetID | `name` | 可下拉化 | 可搜索下拉（历史名称 + 自定义输入） |
| Bind SetID | `org_code` | 可下拉化 | 可搜索下拉（`/org/api/org-units` 返回的 `org_code` + 自定义输入） |
| Bind SetID | `setid` | 可下拉化 | 可搜索下拉（现有 SetID + 自定义输入） |

### 3.2 Security Context / Explainability（共享 `SetIDExplainPanel`）
| 字段 | 评估结论 | 收敛方式 |
| --- | --- | --- |
| `capability_key` | 可下拉化 | 可搜索下拉（Strategy Registry 历史值 + 自定义输入） |
| `field_key` | 可下拉化 | 可搜索下拉（Strategy Registry 历史值 + 自定义输入） |
| `business_unit_id` | 可下拉化 | 可搜索下拉（绑定记录/历史值 + 自定义输入） |
| `setid`（可选） | 可下拉化 | 可搜索下拉（SetID 列表 + 自定义输入） |
| `org_unit_id`（可选） | 可下拉化 | 可搜索下拉（绑定记录/历史值 + 自定义输入） |
| `level` | 可下拉化 | 枚举下拉（brief/full，沿用权限约束） |
| `as_of` | 不纳入 | 保持日期控件 |
| `request_id` | 不纳入 | 保持文本输入（便于追踪注入） |

## 4. 实施步骤
1. [X] 新建并评审 `DEV-PLAN-163A`，冻结字段矩阵与边界。
2. [X] 抽取/复用下拉组件：统一 `freeSolo + options` 行为，减少页签间重复代码。
3. [X] 改造 `Governance` 页签的 Create/Bind 表单字段为下拉。
4. [X] 改造 `SetIDExplainPanel` 字段为下拉，并确保两个页签同时生效。
5. [X] 回归验证与门禁：`make generate && make css && make check doc`。

## 5. 验收标准
- [X] 其余三页签中，矩阵内“可下拉化字段”全部改为下拉交互。
- [X] Security Context 与 Explainability 在同一组件改造后行为一致。
- [X] 关键交互（输入/选择/提交）不退化，错误提示契约保持一致。
- [X] payload 字段名、字段值语义、后端响应契约与改造前一致。
- [X] 本地门禁通过并可复现。

## 8. 验证与验收记录（2026-02-24 UTC）
- [X] 候选源口径收敛：`Bind SetID.org_code` 下拉仅使用 `/org/api/org-units` 的 `org_code`，不再使用 `setid-bindings.org_unit_id` 推断。
- [X] 共享组件落地：新增 `FreeSoloDropdownField`，并在 `SetIDGovernancePage` 与 `SetIDExplainPanel` 复用。
- [X] `Governance` 页签：`setid/name/org_code/setid` 已切换可搜索下拉（保留 freeSolo 手输）。
- [X] `Security Context` + `Explainability`：`capability_key/field_key/business_unit_id/setid/org_unit_id` 已切换可搜索下拉；`level` 保持枚举下拉。
- [X] 门禁：`make generate && make css && make check doc` 通过。

## 6. 风险与缓解
- **候选数据不足导致下拉为空**：采用 `freeSolo`，允许用户继续手输。
- **共享组件改造引入双页签回归**：以 `SetIDExplainPanel` 为单点回归对象，覆盖两个入口。
- **字段约束误伤（如 full explain 权限）**：保留既有权限判定逻辑，不在本计划变更授权规则。

## 7. 关联文档
- `docs/dev-plans/163-capability-key-form-dropdown-convergence.md`
- `docs/dev-plans/102c5-ui-design-for-setid-context-security-registry-explainability.md`
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `AGENTS.md`
