# DEV-PLAN-164：组织类型策略控制范围与继承缺口分析（OrgType）

**状态**: 规划中（2026-02-25 00:38 UTC）

## 1. 背景与问题清单
围绕 `d_org_type`（组织类型）的策略控制，当前存在三类用户可见问题：

1. 仅“新建组织（create_org）”受控；`add_version` / `insert_version` / `correct` 仍能看到完整字典列表。
2. 仅对 `00000002`、`00000004` 生效，未继承到下级。以 `00000003` 为父节点新建下级时出现：`FIELD_POLICY_MISSING`（前端提示“当前上下文缺少字段策略，请刷新后重试”）。
3. 需要让 `00000002` 与 `00000004`（含下级）拥有不同候选值列表，但现状是“按单节点写死唯一值”，维护成本高。

## 2. 现状实现与根因
### 2.1 作用范围仅覆盖 create_org
- 新建策略读取仅走 `GET /org/api/org-units/create-field-decisions`，且固定只解析 `org_code`、`d_org_type`。
- 写入版本校验（`policy_version`）仅在 `intent=create_org` 时强制；其他 intent 未接入同一策略版本校验。
- 结论：`add_version` / `insert_version` / `correct` 不会消费 create-field-decisions 的 `allowed_value_codes`。

### 2.2 下级不继承的直接原因
- SetID Registry 命中条件是：`org_applicability=business_unit` 且 `business_unit_id` **精确匹配**，或 tenant 兜底；没有“按组织树祖先继承”语义。
- create 场景上下文里的 `business_unit_id` 由 `parent_org_code -> org_id` 直接转换，不会自动回溯到“上级业务单元”。
- 因此若仅配置 `00000002/00000004`，而实际创建时父节点是 `00000003`，就会因无精确行而 `FIELD_POLICY_MISSING`。

### 2.3 列表值差异化的能力边界
- Registry 本身支持 `allowed_value_codes`，可按不同 `business_unit_id` 配置不同候选值。
- 但由于无继承语义，想覆盖“含下级”只能给每个父节点逐条补配置，无法一次定义整棵子树。

## 3. 对三个问题的“仅调配置”可行性结论
| 问题 | 仅调配置是否可解 | 结论 |
| --- | --- | --- |
| 1) 仅新建受控，其他 intent 未受控 | 否 | 当前运行链路未接入；需代码改造（前后端都要接入策略消费与校验）。 |
| 2) 00000002/00000004 不继承到下级 | 部分可缓解 | 可通过“给每个实际父节点补一条 business_unit 记录”临时绕过，但不是继承，维护不可持续。 |
| 3) 00000002 与 00000004（含下级）列表差异 | 部分可缓解 | 若只覆盖少量明确节点可配置；若要求“自动覆盖下级”，现模型不支持，仅靠配置无法达成。 |

## 4. 配置层可执行的临时方案（止血）
1. [ ] 为当前实际会作为“父节点”的组织逐条补 `d_org_type` 记录（`org_applicability=business_unit`，`business_unit_id` 为 8 位 org_id，不是 org_code）。
2. [ ] 保留 tenant 级兜底记录，避免出现全量 `FIELD_POLICY_MISSING`。
3. [ ] 在治理页面统一维护 `allowed_value_codes`，并建立“父节点新增时必须同步补策略”的运维清单。

> 该方案只能止血，不能满足“自动继承到下级”的长期目标。

## 5. 建议的结构性改造方向（非本次立即实施）
1. [ ] 将 `add_version` / `insert_version` / `correct` 纳入同一字段策略消费链路（UI 选项 + 后端二次校验 + policy_version 一致性）。
2. [ ] 引入“组织树继承解析”语义：上下文从父节点回溯到最近可用业务单元策略（或显式新增 org_scope 维度）。
3. [ ] 在字段选项 API 增加策略上下文入参并服务端过滤 `allowed_value_codes`，避免仅靠前端裁剪。
4. [ ] 增加门禁与用例：覆盖 `00000002`、`00000004` 及其下级创建/新建版本/插入版本/更正四类操作。

## 6. 最终判断（答复本次问题）
- **不能**通过“只调整现有配置”一次性解决你提出的三点问题。
- 配置只能临时缓解第 2、3 点（通过逐节点补表），但：
  - 无法让策略自然继承到下级；
  - 无法覆盖 `add_version` / `insert_version` / `correct` 的当前运行链路。
- 若目标是“00000002 与 00000004（含下级）稳定差异化控制”，必须补充运行时能力（至少包含：继承解析 + 非 create intent 接入）。

## 7. 关联文档
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/161a-setid-capability-registry-editable-and-maintainable.md`
