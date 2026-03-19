# DEV-PLAN-348D：键治理候选方案并排评估矩阵（348A / 348B / 348C）

**状态**: 草拟中（2026-03-19 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md) 的评估配套文档。  
职责不是提出新的候选方案，而是把当前已登记的候选方案放到**同一张矩阵**里比较，避免每个候选只讲自己的优点、无法横向裁决。

当前纳入比较的候选：

- [DEV-PLAN-348A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348a-setid-package-single-source-candidate-plan.md)：`setid/package` 单主源治理候选方案
- [DEV-PLAN-348B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348b-package-uuid-direct-governance-candidate-plan.md)：取消 `setid`、收敛为 `package_uuid` 直达治理候选方案
- [DEV-PLAN-348C](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348c-workday-reference-key-governance-candidate-plan.md)：对标 Workday 的“一源数据 + 一安全模型 + 组织上下文”参考治理候选方案

## 2. 使用说明

- [ ] 本矩阵是 **M3 证据评估** 的统一载体，不直接替代 `348A/B/C` 的详细论证。
- [ ] 本矩阵中的“初评”只表达当前文档证据下的暂时判断，不等同最终裁决。
- [ ] 若后续候选方案正文有实质更新，必须同步刷新本矩阵，避免评估漂移。

### 2.1 评分口径

采用 5 分制，`5` 为当前维度表现最好，`1` 为最弱。  
若某维度暂时无法判断，仍需给出“当前保守分 + 证据缺口说明”，避免用“待补证据”绕过横向比较。

## 3. 候选摘要（单行版）

| 候选 | 单行主张 | 平台级治理词汇重心 |
| --- | --- | --- |
| `348A` | 保留 `setid` 作为上下文键，`package_uuid` 作为主事实键 | `package_uuid + setid + time` |
| `348B` | 取消 `setid`，由业务上下文直接解析到 `package_uuid` | `package_uuid + org_context + time` |
| `348C` | 不把 `setid/package` 升级为平台总词汇，优先以领域主键 + 组织上下文 + 统一安全/流程模型治理 | `business_object_key + org_context + capability + time` |

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

## 5. 初评结论（非最终裁决）

### 5.1 面向当前仓库延续性

- [ ] `348A` 当前最稳。
- [ ] 原因：它与 [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 和现仓 `package_uuid` 事实主键最接近，迁移与验证成本最低。
- [ ] 风险：概念层仍保留 `setid + package` 双层心智，认知复杂度不占优。

### 5.2 面向 `300` 的 Greenfield 简化

- [ ] `348B` 与 `348C` 更值得认真压力测试。
- [ ] `348B` 代表“最直接的概念减法”，优点是词汇少、业务语义直达；缺点是很容易把复杂度下沉到各域私有解析器。
- [ ] `348C` 代表“平台原则级减法”，优点是最接近 Workday 的“一源数据 + 一安全模型 + 组织上下文”叙事；缺点是前提最重，需要先冻结 `OrgContext` 词汇和统一访问模型。

### 5.3 当前不建议直接跳过的结论

- [ ] 不建议在未完成 `OrgContext` 词汇冻结前，直接宣布 `348C` 为实施主线。
- [ ] 不建议在未证明“各域不会各自长解析器”前，直接宣布 `348B` 为实施主线。
- [ ] 也不建议因为 `348A` 最容易落地，就跳过对 `348B/348C` 的完整比较，否则 `348` 会退化成“为既有实现背书”。

## 6. 各案 stopline 风险摘要

| 候选 | 最需要先证明的 stopline 风险 | 若无法证明，最可能的否决原因 |
| --- | --- | --- |
| `348A` | `setid` 是否只是上下文键，而不是事实上的第二写主键 | 双主源未真正消除，只是改名保留双路径 |
| `348B` | 各域是否会各自实现“上下文 -> package_uuid”私有解析链 | 平台级词汇变少，但整体复杂度反而扩散 |
| `348C` | `OrgContext` 是否足够稳定、统一且可操作地覆盖核心差异 | 原则漂亮，但落地时变成抽象口号或二次发明 |

## 7. 下一轮补证重点

### 7.1 `348A`

- [ ] 补“用户旅程中的双键暴露面”证据：哪些页面/接口仍同时要求理解 `setid` 与 `package_uuid`。
- [ ] 补“Explain 是否仍需两段解释”证据。

### 7.2 `348B`

- [ ] 补“各域上下文直达 package 的统一合同”。
- [ ] 补“禁止域内解析器漂移”的门禁草案。

### 7.3 `348C`

- [ ] 产出 `OrgContext` 最小词汇表。
- [ ] 证明 Job Catalog / Org / Staffing 至少一条端到端旅程可只靠“对象主键 + 组织上下文 + 时间锚 + 统一安全模型”完成解释。
- [ ] 明确哪些 Workday 原则只适合作为方法论，不适合直接翻译成实现约束。

## 8. 建议的评审顺序

1. [ ] 先用 `348D` 冻结矩阵维度与评分口径，防止候选各自改题目。
2. [ ] 先补 `348B` 的“域内解析器蔓延”证据，再补 `348C` 的 `OrgContext` 词汇。
3. [ ] 待关键缺口补齐后，再进入 `348` 的正式裁决会。
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
