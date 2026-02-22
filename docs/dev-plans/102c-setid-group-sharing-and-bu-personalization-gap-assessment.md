# DEV-PLAN-102C：SetID 对标 Workday 的集团共享与业务单元个性化差距评估（承接 102/102B）

**状态**: 准备就绪（2026-02-22 09:44 UTC，已获用户批准进入实施）

## 0. 主计划定位（Plan of Record）
- 本计划是“**能力评估与验收口径**”PoR：用于回答“SetID 在集团共享与 BU 个性化上，与 Workday 原则级能力还有哪些差距”。
- 本计划**不是** 070B 的替代，也不重复其实施任务；`DEV-PLAN-070B` 仍是“共享改发布、运行时 tenant-only”的工程实施 PoR。
- 冲突处理顺序：
  1. 架构/迁移/接口改造细节以 `070B` 与 `102B` 为准；
  2. 业务能力目标、差距优先级、验收评分以 `102C` 为准。

## 1. 背景与上下文（Context）
- `DEV-PLAN-102/102B` 已完成时间语义收敛：`as_of/effective_date` 显式必填、禁止 default today、fail-closed。
- `DEV-PLAN-070A` 已完成方案对标并给出结论：推荐“共享发布到租户（tenant-native）”。
- `DEV-PLAN-070B` 正在同步实施 PR-1~4，聚焦“运行时去 global fallback + 发布基座 + tenant-only 读链路”。
- 现阶段缺口：虽然方向清晰，但“对标 Workday 后的业务能力差距清单、优先级和验收尺子”尚未形成独立 SSOT。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 建立 SetID 能力对标矩阵（集团共享、BU 个性化、安全上下文、流程个性化、审计可解释性、运维治理）。
- [ ] 明确当前能力等级（As-Is）与目标等级（To-Be），形成 P0/P1 缺口清单与排序依据。
- [ ] 输出与 070B 不重叠的“后续能力子计划清单”（仅定义目标与验收，不展开重复实施项）。
- [ ] 固化一套“对标验收评分卡”（Scorecard），作为 070B 收口后的业务能力评审基线。

### 2.2 非目标（避免与 070B 重复）
- 不重复定义 070B 已覆盖的迁移与改造任务：
  - 不重复设计“global_tenant 下线路径”；
  - 不重复设计“发布任务数据模型/迁移脚本”；
  - 不重复定义“dict tenant-only 代码改造步骤”。
- 不在本计划直接新增表/改接口/改路由；如需落地，另立子计划承接。
- 不引入 legacy 双链路或 feature flag 兼容窗口。

### 2.3 工具链与门禁（本计划阶段）
- [x] 文档门禁：`make check doc`
- [ ] 若后续进入代码实施，按触发器矩阵执行：`AGENTS.md` + `docs/dev-plans/012-ci-quality-gates.md`

## 3. 对标框架（Workday 原则级）
> 说明：仅基于公开资料做“原则级对标”，不推断 Workday 私有实现细节。

### 3.1 六维能力模型
1. **共享模型**：集团标准数据如何复用到各 BU（运行时共享 vs 发布式共享）。
2. **个性化粒度**：是否支持租户/BU/组织上下文差异化。
3. **上下文化安全**：授权是否同时考虑角色 + 组织上下文 + 条件。
4. **流程个性化**：除数据外，业务流程是否可按 BU 变体执行。
5. **可解释性**：是否可回答“为何命中该配置/包/规则”。
6. **运维治理**：发布、回放、审计、故障处置是否标准化。

### 3.2 能力等级定义（L0-L3）
- **L0**：仅租户级配置，缺少组织上下文个性化。
- **L1**：支持 SetID/Package 数据个性化，但流程与安全上下文弱。
- **L2**：数据 + 安全 + 审计解释链具备组织上下文。
- **L3**：数据 + 安全 + 流程均可按 BU 可配置，且可回放可审计。

## 4. 差距矩阵（As-Is vs To-Be）
| 维度 | As-Is（当前） | To-Be（目标） | 差距级别 | 承接计划 |
| --- | --- | --- | --- | --- |
| 共享模型 | 仍存在 global 运行时入口（迁移中） | 运行时 tenant-only，集团标准靠发布落地 | 高 | 070B（实施）+ 102C（验收） |
| 数据个性化 | SetID/Package 已具备基础能力 | 支持跨模块稳定扩展，不局限单一样板域 | 中 | 071/071A/071B |
| 安全上下文 | 以租户角色为主，组织上下文约束不足 | 角色 + 组织上下文 + 条件化策略 | 高 | 102C 子计划（新增） |
| 流程个性化 | 主要是数据集选择，流程变体未体系化 | 关键流程可按 BU 配置并可审计 | 高 | 102C 子计划（新增） |
| 可解释性 | 具备事件审计，但“命中原因”暴露不足 | API/UI 可解释“why this package/rule” | 高 | 102C 子计划（新增） |
| 运维治理 | 070B 正在补发布与对账 | 形成常态化评分与巡检机制 | 中 | 070B + 102C |

## 5. 与 070B 的边界冻结（No-Overlap）
| 主题 | 070B 责任 | 102C 责任 | 不重叠约束 |
| --- | --- | --- | --- |
| global_tenant 退出 | 设计与实施迁移、切流、收口 | 定义“业务能力验收项” | 102C 不写迁移 SQL/API 步骤 |
| 字典发布到租户 | 实现发布基座、幂等、对账 | 定义“集团共享能力评分” | 102C 不重复发布链路任务 |
| 时间参数显式化 | 由 102B 收敛并门禁 | 复用 102B 作为前提 | 102C 不新建时间口径 |
| Workday 对标 | 070A 给原则结论 | 102C 给差距闭环与验收基线 | 070A/102C 不重复工程任务 |

## 6. 102C 后续子计划建议（新增能力，不与 070B 重复）
1. [ ] **102C1：上下文化安全模型**
   - 目标：把 `owner_setid` 与 BU/组织上下文纳入授权判定，不只看租户角色。
2. [ ] **102C2：BU 个性化策略注册表**
   - 目标：建立“哪些能力可个性化、在哪个组织层级生效”的统一注册表。
3. [ ] **102C3：配置命中可解释性（Explainability）**
   - 目标：提供“命中链路”可观测输出（setid/scope/package/as_of/规则来源）。
4. [ ] **102C4：流程个性化样板（暂缓）**
   - 目标：选 1 条关键流程验证“BU 变体 + 审计 + 回放”。
   - 暂缓原因：项目当前尚未建设流程模块，待流程域具备基础能力后重启。
5. [ ] **102C5：102C1-102C3 UI 专项方案**
   - 目标：将上下文化安全、策略注册表、命中解释收敛为“可发现、可操作、可验收”的前端交付。
6. [ ] **102C-T：102C1-102C3 测试方案**
   - 目标：验证同租户跨 BU 的字段必填/可见/默认值差异，并输出支持性结论与阻塞项。
7. [ ] **102C6：能力评分卡常态化**
   - 目标：建立季度评审机制，跟踪 L1→L2→L3 进展。

## 7. 验收标准（Capability Acceptance）
- [ ] 形成并冻结六维能力评分卡（含评分口径、证据来源、责任人）。
- [ ] 完成 As-Is 基线评分与 To-Be 目标评分，且差距项有唯一承接计划。
- [ ] 明确至少 3 项“非 070B”高优先级差距，并落到 102C 子计划。
- [ ] 能在评审中回答：
  - “集团共享如何进入租户本地并可审计？”
  - “BU 个性化除了数据，还覆盖哪些流程与权限？”
  - “为何某 BU 在某日命中该配置？”

## 8. 里程碑（文档与评审）
1. [ ] **M1 基线冻结**：完成 As-Is 评分与证据挂载。
2. [ ] **M2 差距收敛**：完成 P0/P1 差距排序与承接映射。
3. [ ] **M3 子计划立项**：完成 102C1~102C3 至少 1 个草案并评审通过。
4. [ ] **M4 联合验收**：与 070B 收口评审联动，输出最终能力结论。

## 9. 风险与缓解
- **R1：与 070B 职责重叠**
  - 缓解：按 §5 边界冻结；评审时先做“是否重复 070B”检查。
- **R2：对标过度推断外部实现**
  - 缓解：仅采用公开资料的原则级口径，不落私有细节断言。
- **R3：只做技术改造，忽略业务可见性**
  - 缓解：将“流程个性化 + 命中解释”列为独立验收项。

## 10. 依赖与引用
- `docs/archive/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/070a-setid-global-share-vs-tenant-native-isolation-investigation.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/archive/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/archive/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`

## 11. 外部公开资料（原则级对标）
- https://www.workday.com/en-us/enterprise-resource-planning.html
- https://www.workday.com/en-ae/why-workday/trust/security.html
- https://blog.workday.com/en-us/2021/how-workday-supports-gdpr-and-data-subject-rights.html
