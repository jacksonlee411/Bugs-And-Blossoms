# DEV-PLAN-102C2：BU 个性化策略注册表（承接 102C，避免与 070B/102C1 重复）

**状态**: 草拟中（2026-02-22 23:34 UTC）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 的子计划，聚焦“**哪些能力允许按 BU 个性化、在哪个组织层级生效、如何审计解释**”的注册表机制。
- 本计划不是 070B 的迁移计划，也不是 102C1 的授权模型计划。
- 本计划输出：注册表契约、优先级分层、评审门禁、验收口径；不在本计划直接落地大规模代码改造。

## 1. 背景与问题陈述（Context）
- 当前系统已具备 SetID/Scope Package 的数据个性化能力，但缺少统一“可个性化能力目录”。
- 现状问题：
  1. 新能力是否允许 BU 级差异，常靠临时约定，缺少 SSOT；
  2. 同类能力在不同模块可能出现不同口径（tenant-only / setid / scope-package）；
  3. 评审中难快速回答“该能力是否允许个性化、为何允许、生效边界是什么”。
- 业务影响：没有注册表会导致个性化策略漂移，长期增加治理与审计成本。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 定义 BU 个性化策略注册表（Strategy Registry）最小字段与语义。
- [ ] 建立能力分层（禁止个性化 / 可个性化但受限 / 可个性化且可组合）。
- [ ] 冻结“新能力准入流程”：新增能力必须先登记注册表再进入实施。
- [ ] 输出可执行验收标准：评审、门禁、审计三位一体。

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
| `owner_module` | 归属模块 | `jobcatalog` |
| `personalization_mode` | 个性化模式 | `tenant_only` / `setid` / `scope_package` |
| `scope_code` | 若为 scope_package，绑定 scope | `jobcatalog` |
| `org_level` | 生效组织层级 | `tenant` / `business_unit` / `org_unit` |
| `explain_required` | 是否必须提供命中解释 | `true` |
| `is_stable` | 是否进入稳定能力清单 | `true` |
| `change_policy` | 变更策略 | `plan_required` |

### 3.2 个性化模式定义（冻结）
- `tenant_only`：仅租户统一策略，不允许 BU 差异。
- `setid`：允许按 SetID 差异，但不做跨 scope 组合。
- `scope_package`：允许按 scope/package 组合差异（受 071/071B 约束）。

### 3.3 组织层级定义（冻结）
- `tenant`：全租户统一。
- `business_unit`：按 BU 差异。
- `org_unit`：按组织节点差异（需更强审计与解释约束）。

## 4. 能力分层（L0-L2）
- **L0（不可个性化）**：基础安全/法务/核算不变量，只允许 tenant_only。
- **L1（受限个性化）**：允许 setid 差异，禁止跨 scope 组合。
- **L2（可组合个性化）**：允许 scope_package，需满足 explain_required + 审计链。

## 5. 与现有计划边界（No-Overlap）
| 主题 | 070B | 102C1 | 102C2 |
| --- | --- | --- | --- |
| 共享改发布 | 实施主责 | 不涉及 | 不涉及 |
| 上下文化授权 | 不主责 | 实施主责 | 仅引用其结果 |
| 能力可否个性化 | 不主责 | 不主责 | 实施主责（治理契约） |
| 新能力准入流程 | 不主责 | 部分关联 | 实施主责（注册先行） |

## 6. 新能力准入流程（冻结）
1. [ ] 新能力提出时先填写 `capability_key + personalization_mode + org_level`。
2. [ ] 若 `personalization_mode != tenant_only`，必须提供 explain 方案与审计字段方案。
3. [ ] 若选择 `scope_package`，必须关联已登记 `scope_code`，且给出 071/071B 承接路径。
4. [ ] 通过评审后方可进入实施子计划；未登记不得编码。

## 7. 门禁建议（后续实施）
- [ ] 文档门禁：检测新增能力文档是否包含 `capability_key` 与 `personalization_mode`。
- [ ] 代码门禁：新增个性化逻辑时必须引用注册表键，不允许临时硬编码模式。
- [ ] 评审门禁：PR 模板增加“是否新增个性化能力、是否登记注册表”检查项。

## 8. 里程碑（文档到实施）
1. [ ] **M1 契约冻结**：注册表字段与模式定义评审通过。
2. [ ] **M2 基线登记**：首批能力清单完成登记（至少覆盖 SetID/JobCatalog/Dict 关键能力）。
3. [ ] **M3 准入门禁**：文档与评审门禁启用。
4. [ ] **M4 试点实施**：选择 1 个新能力验证“先登记再实施”闭环。

## 9. 验收标准（Acceptance Criteria）
- [ ] 形成可检索的能力注册表清单（字段齐全、无歧义）。
- [ ] 至少 1 个新增能力按“登记 -> 评审 -> 实施”闭环执行并留证。
- [ ] 评审可回答：
  - “该能力是否允许 BU 个性化？”
  - “在哪个组织层级生效？”
  - “命中原因如何解释与审计？”
- [ ] 与 070B/102C1 无重复任务。

## 10. 风险与缓解
- **R1：注册表沦为文档摆设**
  - 缓解：绑定准入门禁与 PR 评审项。
- **R2：键命名失控**
  - 缓解：`capability_key` 采用 `module.capability` 规则并集中审校。
- **R3：分层定义过粗或过细**
  - 缓解：先 L0-L2 三层，试点后再扩展。

## 11. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/012-ci-quality-gates.md`
