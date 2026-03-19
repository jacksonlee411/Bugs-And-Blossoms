# DEV-PLAN-348D：键治理候选方案并排评估矩阵（348A / 348B / 348C）

**状态**: 已裁决（2026-03-19 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md) 的评估配套文档。  
职责不是提出新的候选方案，而是把当前已登记的候选方案放到**同一张矩阵**里比较，并在比较完成后保留最终裁决记录，避免每个候选只讲自己的优点、无法横向裁决。

当前纳入比较的候选：

- [DEV-PLAN-348A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348a-setid-package-single-source-candidate-plan.md)：`setid/package` 单主源治理候选方案
- [DEV-PLAN-348B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348b-package-uuid-direct-governance-candidate-plan.md)：取消 `setid`、收敛为 `package_uuid` 直达治理候选方案
- [DEV-PLAN-348C](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348c-workday-reference-key-governance-candidate-plan.md)：对标 Workday 的“一源数据 + 一安全模型 + 组织上下文”治理主线

## 2. 使用说明

- [x] 本矩阵承担过 **M3 证据评估** 的统一载体角色，但不直接替代 `348A/B/C` 的详细论证。
- [x] 第 4 节保留的是“初评 v0”历史快照；最终裁决以第 5 节为准。
- [ ] 若后续主线文档正文有实质更新，仍必须同步刷新本矩阵，避免裁决漂移。

### 2.1 评分口径

采用 5 分制，`5` 为当前维度表现最好，`1` 为最弱。  
若某维度暂时无法判断，仍需给出“当前保守分 + 证据缺口说明”，避免用“待补证据”绕过横向比较。

## 3. 候选摘要（单行版）

| 候选 | 单行主张 | 平台级治理词汇重心 |
| --- | --- | --- |
| `348A` | 保留 `setid` 作为上下文键，`package_uuid` 作为主事实键 | `package_uuid + setid + time` |
| `348B` | 取消 `setid`，由业务上下文直接解析到 `package_uuid` | `package_uuid + org_context + time` |
| `348C` | 平台与业务域均不再保留 `setid/package_uuid`，统一以领域主键 + 组织上下文 + 统一安全/流程模型治理 | `business_object_key + org_context + capability + time` |

## 4. 并排评估矩阵（初评 v0）

| 维度 | `348A` | `348B` | `348C` | 初步观察 |
| --- | --- | ---: | ---: | --- |
| 一致性（单主源） | 4 | 4 | 3 | `348A` 与现仓 `363` 最贴近，最容易形成单主源；`348B` 能做到单主源，但要证明“上下文直达 package”不会退化为多套域内私有解析；`348C` 原则最强，但平台与领域边界最难收敛 |
| 可解释性 | 4 | 4 | 5 | `348C` 最接近“同一访问模型 + 组织上下文 + 安全/流程解释”；`348A/348B` 都能解释，但更偏配置命中解释 |
| 认知复杂度 | 3 | 5 | 4 | `348B` 的概念最少；`348A` 需要长期理解 `setid + package` 两层；`348C` 概念少于 `348A`，但前提是 `OrgContext` 词汇体系足够清晰 |
| 时间确定性 | 4 | 4 | 4 | 三案都可以满足显式 `as_of / effective_date`；差异主要不在时间，而在上下文组织方式 |
| 安全边界 | 4 | 4 | 5 | `348C` 最自然对齐统一安全模型；`348A/348B` 仍能 fail-closed，但更容易把安全解释拆成“权限 + 数据集/包命中”两段 |
| 迁移成本（低成本=高分） | 5 | 3 | 2 | `348A` 最贴近当前仓库事实；`348B` 需要删除 `setid` 平台口径；`348C` 需要平台级 `OrgContext` 重塑，证明成本最高 |
| 运维可控 | 4 | 3 | 4 | `348A` 较易追踪；`348B` 若各域自有解析器会变难排障；`348C` 若统一访问模型落成则运维表达很好，但前置工程量大 |
| 跨域一致性 | 4 | 3 | 5 | `348A` 通过统一 `setid` 层维持跨域一致性；`348B` 最易出现“每域自定义上下文直达 package”；`348C` 若成功，跨 UI/API/Assistant/Integration 的一致性最好 |
| 与 `300` 简化方向一致性 | 3 | 5 | 5 | `348B/348C` 更符合“从零重做时主动减少偶然复杂度”；`348A` 更像当前仓库延续性方案 |
| Workday 原则对齐度 | 2 | 3 | 5 | `348C` 明确按 Workday 官方公开原则映射；`348B` 仅在“减概念”上部分接近；`348A` 更偏现仓收敛，不是 Workday 风格主线 |
| 关键未知数风险（低风险=高分） | 4 | 3 | 2 | `348A` 未知数最少；`348B` 主要风险是域内解析器蔓延；`348C` 主要风险是需要先证明 `OrgContext` 足以承载所有差异 |

## 5. 最终裁决

### 5.1 选定主线

- [x] 选定 `348C` 作为平台主线。
- [x] 选定理由：它最符合 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的 Greenfield 简化方向、`345/347/342/333` 的统一访问模型与统一安全模型，以及 Workday 的原则级启发。
- [x] 本轮裁决附加要求：比原候选稿更彻底，平台与业务域均不再保留 `package_uuid` 作为主事实键、治理容器、隐藏 alias 或降级落点。

### 5.2 被否决方案

- [x] 否决 `348A`：虽然最贴近现仓，但仍保留 `setid + package` 双层心智，本质上没有消除第二解释路径。
- [x] 否决 `348B`：虽然比 `348A` 更简化，但仍保留 `package_uuid` 作为隐藏治理词汇，且容易把复杂度下沉为各域私有解析器。

### 5.3 直接影响

- [x] `348/348C/348D` 需要同步转为“已裁决”口径。
- [x] `363` 需要去除 `package_uuid/setid` 治理叙事，改写为“统一目录骨架 + OrgContext + as_of + read_only”。
- [x] 后续实施文档必须默认：`business_object_key + org_context + capability + time` 是唯一治理语法。

## 6. 各案 stopline 风险摘要

| 候选 | 最需要先证明的 stopline 风险 | 若无法证明，最可能的否决原因 |
| --- | --- | --- |
| `348A` | 历史双层心智是否还会以 alias 形式回流 | 双主源未真正消除，只是改名保留双路径 |
| `348B` | `package_uuid` 是否会以域内私有键继续潜伏 | 平台级词汇变少，但整体复杂度反而扩散 |
| `348C` | `OrgContext` 是否足够稳定、统一且可操作地覆盖核心差异 | 若无法落成，主线会退化为抽象口号或被旧容器键反向侵蚀 |

## 7. 实施前置收口重点

### 7.1 `348C`

- [ ] 产出 `OrgContext` 最小词汇表。
- [ ] 证明 Job Catalog / Org / Staffing 至少一条端到端旅程可只靠“对象主键 + 组织上下文 + 时间锚 + 统一安全模型”完成解释。
- [ ] 清理 `363` 中残留的 `package_uuid/setid` 治理口径。
- [ ] 明确哪些 Workday 原则只适合作为方法论，不适合直接翻译成实现约束。

## 8. 建议的实施顺序

1. [x] 先用 `348D` 冻结矩阵维度与评分口径，防止候选各自改题目。
2. [x] 完成 `348` 正式裁决，明确 `348C` 为主线、`348A/B` 为否决方案。
3. [ ] 冻结 `OrgContext` 最小词汇表，并重写直接冲突的业务域文档。
4. [ ] 裁决输出时应同时给出：
   - 选中的主线候选；
   - 被否决候选的主要原因；
   - 对 `340/345/347/360/363` 的条目级修订清单。

## 9. 关联文档

- [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md)
- [DEV-PLAN-348A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348a-setid-package-single-source-candidate-plan.md)
- [DEV-PLAN-348B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348b-package-uuid-direct-governance-candidate-plan.md)
- [DEV-PLAN-348C](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348c-workday-reference-key-governance-candidate-plan.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
