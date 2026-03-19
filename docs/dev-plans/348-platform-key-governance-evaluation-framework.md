# DEV-PLAN-348：平台通用键治理规范与评估框架（Key Governance Evaluation Framework）

**状态**: 规划中（2026-03-19 07:24 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的平台治理子计划，承接：

- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 对 `capability_key + context + as_of` 决议协议的冻结；
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 对能力键/路由映射/颗粒度词汇的冻结；
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 对 Job Catalog “`package_uuid` 治理主键、`setid` 上下文入口”的业务收敛；
- [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md) 的历史方案分歧与后续评估需求。

目标不是立即裁决某一实现路线，而是先建立平台级统一评估框架，避免各模块各自定义“主键/上下文键/能力键”并再次产生双主源。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结平台级“键治理”词汇与角色边界（主事实键、上下文键、能力键、时间锚点）。
- [ ] 建立统一评估维度与停线（stopline），用于比较不同键治理候选方案。
- [ ] 明确“候选方案 -> 评估证据 -> 决策结论 -> 实施承接”的标准流程。
- [ ] 把评估结果作为 `350/360/370/380/390` 的统一输入，阻断再次分叉。
- [ ] 为后续新增门禁（防双主源、防上下文回流、防 runtime fallback）定义契约入口。

### 2.2 非目标

- [ ] 本计划不直接执行数据库迁移、接口替换或 UI 切换。
- [ ] 本计划不替代具体域的实施文档（如 Job Catalog、Staffing 等）。
- [ ] 本计划不否定历史方案本身；仅定义“如何统一评估并决策”。

## 3. 当前冲突与待评估问题

当前已出现多类口径并存，需要平台级评估裁决：

- [ ] “`package_uuid` 作为治理主键，`setid` 作为上下文入口/解析维度”的域内收敛口径（见 `363`）。
- [ ] “删除 package 概念，直接使用 `capability_key + setid`”的历史候选口径（见 `102C6`）。
- [ ] “取消 `setid` 平台级治理词汇，直接由业务上下文解析到 `package_uuid`”的候选口径（见 `348B`）。
- [ ] “对标 Workday 的一源数据 / 一安全模型 / 组织上下文参考口径”的候选方向（见 `348C`）。

`348` 要求先完成评估，再进入全平台实施，不允许在不同模块并行落地相互冲突的主键模型。

## 4. 平台级键治理词汇冻结

| 术语 | 定义 | 允许承担的职责 | 明确禁止 |
| --- | --- | --- | --- |
| `Primary Fact Key`（主事实键） | 标识某一业务事实归属与唯一性的主键 | 写入主键、唯一约束、FK 约束、主查询锚点 | 与另一键并列成为可替代写主键 |
| `Context Key`（上下文键） | 决议时的业务上下文输入（如 `setid`、`business_unit`） | 决议命中、入口筛选、Explain 回显 | 直接替代主事实键写入核心事实表 |
| `Capability Key`（能力键） | 动作语义标识（做什么） | 策略命中与权限映射 | 编码租户/SetID/BU/地域等上下文 |
| `Time Anchor`（时间锚点） | `as_of` / `effective_date` / `policy_version` | 时间切片、回放一致性、提交一致性锚点 | 被隐式默认值替代 |

## 5. 候选方案管理

### 5.1 编号规则

- [ ] 平台级候选方案使用 `348A / 348B / 348C ...` 管理；
- [ ] 每个候选方案必须声明：主键选择、上下文策略、读写合同、迁移方式、风险与回滚策略；
- [ ] 未登记到 `348` 候选清单的方案，不得进入实施评审。

### 5.2 当前候选清单

- [ ] `348A`：`setid/package` 单主源治理候选方案（待评估）。
- [ ] `348B`：取消 `setid`、收敛为 `package_uuid` 直达治理候选方案（待评估）。
- [ ] `348C`：对标 Workday 的“一源数据 + 一安全模型 + 组织上下文”参考治理候选方案（待评估）。

## 6. 统一评估维度与停线

| 维度 | 评估问题 | 最低停线（未达标即否决） |
| --- | --- | --- |
| 一致性 | 是否存在并列写主键或双解释路径 | 不允许双主写入口 |
| 可解释性 | 用户是否可解释“为何命中该配置/权限/候选值” | Explain 必须包含主键、上下文、时间锚 |
| 认知复杂度 | 用户、前端与服务端是否需要同时理解多套治理键与隐式解析层 | 常态业务流不得要求用户同时输入或选择两个以上治理键；不得以隐藏 alias 保留第二心智 |
| 时间确定性 | `current/as_of/history` 是否稳定复算 | 禁止隐式 today；回放结果必须确定 |
| 安全边界 | 租户隔离/权限边界是否可证明 fail-closed | 缺上下文/缺映射/缺发布必须拒绝 |
| 迁移成本 | 迁移是否可一次性收口并可验证 | 必须有数据对账与回滚预案 |
| 运维可控 | 故障定位、审计追踪是否可落地 | 必须有审计链与错误码收敛 |

## 7. 决策流程（评估到实施）

1. [ ] `M1`：词汇冻结  
   完成键角色与术语冻结，禁止新增模糊术语。
2. [ ] `M2`：候选方案登记  
   每个候选方案提交“合同 + 风险 + 证据计划”。
3. [ ] `M3`：证据评估  
   按第 6 节维度评分，形成评估结论与反例清单；并排评估矩阵统一记录在 `348D`。
4. [ ] `M4`：决策与承接  
   输出平台裁决（选型/否决），并创建实施承接计划（域内编号）。

## 8. 门禁与治理接线

- [ ] 复用既有门禁：`make check capability-key`、`make check capability-route-map`、`make check granularity`、`make check no-scope-package`。
- [ ] 评估结论通过后，新增“单主源”专项门禁（禁止并列主键写入口、禁止双解释路径）。
- [ ] 全部候选文档变更必须通过 `make check doc`。

## 9. 验收标准

- [ ] 平台级键治理词汇已冻结并被后续子计划统一引用。
- [ ] 至少两份方向明确不同的候选方案（`348A` / `348B`）完成标准化登记并进入评估流程。
- [ ] 已形成候选并排评估矩阵（`348D`），且能支撑 `348A / 348B / 348C` 在同一维度上比较。
- [ ] 评估维度、停线与决策流程可被复用，不依赖单一业务域经验。
- [ ] `340/345/347/360/363` 的引用口径一致，不引入新的第二事实源。

## 10. 关联文档

- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
- [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md)
- [DEV-PLAN-348C](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348c-workday-reference-key-governance-candidate-plan.md)
- [DEV-PLAN-348D](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348d-key-governance-candidate-comparison-matrix.md)
