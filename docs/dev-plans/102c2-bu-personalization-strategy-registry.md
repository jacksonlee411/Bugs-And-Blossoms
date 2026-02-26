# DEV-PLAN-102C2：BU 个性化策略注册表（承接 102C，避免与 070B/102C1 重复）

**状态**: 草拟中（2026-02-22 17:15 UTC）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 的子计划，聚焦“**哪些能力允许按 BU 个性化、在哪个组织层级生效、如何审计解释**”的注册表机制。
- 本计划不是 070B 的迁移计划，也不是 102C1 的授权模型计划。
- 本计划输出：注册表契约、优先级分层、评审门禁、验收口径；不在本计划直接落地大规模代码改造。
- 若与 `DEV-PLAN-150` 存在口径冲突，以 `DEV-PLAN-150` 作为 capability_key 收口与分层治理的最终 PoR。

## 1. 背景与问题陈述（Context）
- 当前系统已具备 SetID 差异化能力，但缺少统一“可个性化能力目录”。
- 现状问题：
  1. 新能力是否允许 BU 级差异，常靠临时约定，缺少 SSOT；
  2. 同类能力在不同模块可能出现不同口径（tenant-only / setid）；
  3. 评审中难快速回答“该能力是否允许个性化、为何允许、生效边界是什么”。
- 业务影响：没有注册表会导致个性化策略漂移，长期增加治理与审计成本。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 定义 BU 个性化策略注册表（Strategy Registry）最小字段与语义。
- [ ] 建立能力分层（禁止个性化 / 可个性化但受限 / 可个性化且可组合）。
- [ ] 对齐 `functional_area` 与 `capability_type` 分层，避免静态能力与流程能力混用。
- [ ] 冻结“新能力准入流程”：新增能力必须先登记注册表再进入实施。
- [ ] 输出可执行验收标准：评审、门禁、审计三位一体。
- [ ] 增补字段级策略模型：可表达同租户跨 BU 的 `required/visible/default_rule` 差异。
- [ ] 补齐策略激活元数据：`policy_version` 与 `activation_state(draft/active)`。

### 2.2 非目标（与 070B/102C1 明确隔离）
- 不重复 070B 的发布/迁移/切流任务（global 下线、tenant-only 改造等）。
- 不重复 102C1 的“谁能改”的上下文化授权逻辑。
- 不在本计划直接扩展所有模块代码；先冻结治理契约。

### 2.3 工具链与门禁（本计划阶段）
- [x] 文档门禁：`make check doc`
- [ ] 进入实施后按触发器矩阵执行（Go/DB/Authz/Routing）

## 3. 注册表设计（Strategy Registry）
### 3.1 最小字段（草案）
| 字段 | 含义 | 示例 |
| --- | --- | --- |
| `capability_key` | 能力唯一键（跨模块稳定） | `jobcatalog.profile_defaults` |
| `functional_area_key` | 能力归属功能域 | `staffing` |
| `capability_type` | 能力类型分层 | `domain_capability` / `process_capability` |
| `owner_module` | 归属模块 | `jobcatalog` |
| `field_key` | 字段键（字段级策略时必填） | `field_x` |
| `personalization_mode` | 个性化模式 | `tenant_only` / `setid` |
| `org_applicability` | 生效组织层级 | `tenant` / `business_unit` |
| `bu_selector` | BU 选择器 | `business_unit=BU-A` |
| `required` | 该字段是否必填 | `true` |
| `visible` | 该字段是否可见 | `false` |
| `default_rule_ref` | 默认值规则引用 | `rule://a1` |
| `priority` | 命中优先级（冲突消解） | `100` |
| `effective_date_range` | 生效日期区间 | `[2026-01-01,9999-12-31]` |
| `policy_version` | 租户级策略版本号 | `2026.03.01.1` |
| `activation_state` | 激活状态 | `draft` / `active` |
| `explain_required` | 是否必须提供命中解释 | `true` |
| `is_stable` | 是否进入稳定能力清单 | `true` |
| `change_policy` | 变更策略 | `plan_required` |

### 3.2 个性化模式定义（冻结）
- `tenant_only`：仅租户统一策略，不允许 BU 差异。
- `setid`：允许按 SetID 差异（不再引入 scope/package 中间层，遵循 102C6）。

### 3.3 组织层级定义（冻结）
- `tenant`：全租户统一。
- `business_unit`：按 BU 差异。

### 3.4 优先级与冲突消解（新增冻结）
- 命中顺序：`business_unit > tenant > baseline`。
- 同级冲突按 `priority` 决策；若仍冲突则 fail-closed（`FIELD_POLICY_CONFLICT`）。
- 不允许存在 `visible=false` 且 `required=true` 的策略组合。

### 3.5 Functional Area 与激活协议（新增冻结）
- 每个 `capability_key` 必须且仅能归属一个 `functional_area_key`；缺失归属 fail-closed。
- `capability_type` 必填，且仅允许 `domain_capability` / `process_capability`。
- `activation_state=draft` 的策略仅可预览，不得参与运行时判定。
- 仅 `activation_state=active` 且版本为当前 `policy_version` 的策略可进入命中链路。
- `functional_area` 为 `reserved` 时，禁止该域下 capability 进入运行时路由映射。

## 4. 能力分层（L0-L1）
- **L0（不可个性化）**：基础安全/法务/核算不变量，只允许 tenant_only。
- **L1（受限个性化）**：允许 setid 差异，必须满足 explain_required + 审计链。

## 4.1 capability_key 命名与反退化（冻结）
- `capability_key` 仅表达“能力动作”，不得编码上下文（setid/bu/tenant/region）。
- 格式：`<module>.<capability>[.<action>]`，全小写，段间用 `.`。
- 禁止示例：`jobcatalog.setid_s2601`、`staffing.assignment_create.bu_a`。
- 禁止运行时拼接 key（例如 `capability_key + "_" + setid`）。

## 5. 与现有计划边界（No-Overlap）
| 主题 | 070B | 102C1 | 102C2 |
| --- | --- | --- | --- |
| 共享改发布 | 实施主责 | 不涉及 | 不涉及 |
| 上下文化授权 | 不主责 | 实施主责 | 仅引用其结果 |
| 能力可否个性化 | 不主责 | 不主责 | 实施主责（治理契约） |
| 新能力准入流程 | 不主责 | 部分关联 | 实施主责（注册先行） |

## 6. 新能力准入流程（冻结）
1. [ ] 新能力提出时先填写 `capability_key + functional_area_key + capability_type + personalization_mode + org_applicability`。
2. [ ] 若 `personalization_mode != tenant_only`，必须提供 explain 方案与审计字段方案。
3. [ ] 字段级差异场景必须登记 `field_key + required + visible + default_rule_ref + bu_selector`。
4. [ ] 路由/动作到 `capability_key` 的映射必须登记并评审，缺失时禁止上线。
5. [ ] 发布前必须完成 `draft -> active` 激活审批并生成新 `policy_version`。
6. [ ] 通过评审后方可进入实施子计划；未登记不得编码。

## 7. 门禁建议（后续实施）
- [ ] 文档门禁：检测新增能力文档是否包含 `capability_key` 与 `personalization_mode`。
- [ ] 代码门禁：新增个性化逻辑时必须引用注册表键，不允许临时硬编码模式。
- [ ] 评审门禁：PR 模板增加“是否新增个性化能力、是否登记注册表”检查项。
- [ ] 一致性门禁：阻断 `visible=false && required=true`、`default_rule_ref` 缺失、`priority` 冲突未解的策略落库。
- [ ] 命名门禁：阻断 `capability_key` 含上下文禁词与动态拼接生成。
- [ ] 归属门禁：阻断 capability 缺失 `functional_area_key` 或多归属冲突。
- [ ] 激活门禁：阻断未激活（`draft`）策略进入运行时命中，阻断 `policy_version` 非原子切换。

## 8. 里程碑（文档到实施）
1. [ ] **M1 契约冻结**：注册表字段与模式定义评审通过。
2. [ ] **M2 字段策略扩展**：完成字段级字段模型与冲突消解规则。
3. [ ] **M3 基线登记**：首批能力清单完成登记（至少覆盖 SetID/JobCatalog/Dict 关键能力）。
4. [ ] **M4 准入门禁**：文档与评审门禁启用。
5. [ ] **M5 试点实施**：选择 1 个新能力验证“先登记再实施”闭环。

## 9. 验收标准（Acceptance Criteria）
- [ ] 形成可检索的能力注册表清单（字段齐全、无歧义）。
- [ ] 至少 1 个新增能力按“登记 -> 评审 -> 实施”闭环执行并留证。
- [ ] 评审可回答：
  - “该能力是否允许 BU 个性化？”
  - “在哪个组织层级生效？”
  - “命中原因如何解释与审计？”
- [ ] 与 070B/102C1 无重复任务。
- [ ] 可表达并稳定命中同租户跨 BU 的字段必填差异（A 必填 / B 非必填）。
- [ ] 可表达并稳定命中同租户跨 BU 的字段可见性差异（A 可见 / B 不可见）。
- [ ] 可表达并稳定命中同租户跨 BU 的字段默认规则差异（A=`a1` / B=`b2`）。

## 10. 风险与缓解
- **R1：注册表沦为文档摆设**
  - 缓解：绑定准入门禁与 PR 评审项。
- **R2：键命名失控**
  - 缓解：`capability_key` 采用 `module.capability` 规则并集中审校。
- **R3：分层定义过粗或过细**
  - 缓解：先 L0-L1 两层，试点后再扩展。
- **R4：capability_key 退化为“新 scope”**
  - 缓解：执行命名禁词与映射门禁；上下文差异只允许走 `setid + business_unit + as_of`。

## 11. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/102c-t-test-plan-for-c1-c3-bu-field-variance.md`
- `docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`
