# DEV-PLAN-348C：对标 Workday 的“一源数据 + 一安全模型 + 组织上下文”治理主线

**状态**: 已选定为平台主线（2026-03-19 CST）

## 1. 背景与定位

本计划原本是 [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md) 的第三个候选方案文档。  
现已被正式选定为平台主线，目标不再是提供“参考方向”，而是基于 **Workday 官方公开资料**提炼其在“主数据事实源、上下文、安全模型、流程与审计”上的原则级做法，并把它翻译为本仓库后续所有子计划必须遵守的正式治理基线。

本主线默认承接：

- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 中 `capability_key + context + as_of` 的统一决议协议；
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 中能力键、映射单点化与 fail-closed；
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 中 Job Catalog 作为固定骨架 + 可配置层消费域的边界；
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 中“从零重做时应主动减少偶然复杂度、避免继承历史包袱”的方法论。

### 1.1 来源边界（强制说明）

- [x] 本主线只使用 Workday **官方公开网页、官方 datasheet、官方白皮书、官方新闻稿**中的原则级信息。
- [x] 本主线中的“映射到本仓库的设计含义”均为**从公开资料做出的推断**，不是对 Workday 未公开内部实现细节的断言。
- [x] 本主线不主张复制 Workday 的底层技术实现（例如其对象数据模型或运行时引擎），只借鉴其治理原则与产品边界。

## 2. Workday 公开原则到本仓库主题的映射（推断）

| Workday 官方公开原则 | 公开来源摘要 | 对本仓库 `348` 主题的推断映射 |
| --- | --- | --- |
| Power of One：one source for data / one security model | Workday 公开强调“一份数据源、一套安全模型、一套体验” | 平台应优先避免 `setid/package` 这类并列治理键长期并存，减少双解释路径 |
| Business Process Framework 原生内建，并与 security + organizational hierarchy 协同 | Workday BPF 强调流程不是外挂，事件按安全与组织层次实时路由 | 流程与配置命中应优先依赖业务对象 + 组织上下文 + 权限模型，而不是额外发明跨域治理键 |
| Configurable security groups 基于 users / roles / jobs / organizations / location hierarchy / business sites | Workday 安全资料强调安全组与策略可按组织和角色等上下文组合 | 平台级 Context Key 应优先落在 `OrgContext + Role/Group + Time`，而不是抽象成独立 `setid` 层 |
| Organization Management 用层级结构支撑 routing / security / analysis / reporting | Workday 组织管理资料强调组织层级是一等业务能力 | 组织上下文应成为主配置/主流程/主安全的统一输入，而不是仅作为解析别名的来源 |
| Single access model across UI / APIs / integrations / business processing | Workday 云架构资料强调同一访问模型贯穿数据、交易、集成、业务处理与应用 | UI / API / Integration / Assistant 不应各自定义第二套授权或主键命中逻辑 |
| Job Architecture 用统一工作台管理 job profiles，并减少冗余维护 | Workday 官方新闻稿强调 Job Architecture 的一致性与减冗余价值 | Job Catalog 应优先维护规范化主骨架，避免通过 package 或上下文复制制造冗余目录 |

## 3. 主线方案陈述（C 方案）

### 3.1 主张

- [x] 平台级治理遵循 “**One Source / One Security / Org Context**”：
  - `Primary Fact Key`：优先是稳定的领域对象主键；
  - `Context Key`：优先是业务组织上下文，而不是独立的跨域数据集键或容器键；
  - `Time Anchor`：显式 `as_of / effective_date / policy_version`；
  - `Capability Key`：只表达动作语义，不表达上下文。
- [x] 平台**不引入**跨业务域统一的 `setid` 或 `package_uuid` 作为总治理键。
- [x] 业务域**同样不再保留** `package_uuid` 作为域内主事实键、治理容器键、隐藏 alias、迁移缓冲键或降级落点。
- [x] 配置差异、流程差异与权限差异优先通过“组织上下文 + 统一安全/流程框架”表达，而不是通过复制多套包/数据集表达。
- [x] Job Catalog / Org / Staffing / Person 等域的差异必须回归“领域对象主键 + 组织上下文 + 时间锚点 + 统一安全模型”，不得再通过容器键复制目录或拆分事实源。

### 3.2 参考约束

- [x] 禁止平台同时长期维护“两套上下文模型”：一套面向组织上下文，一套面向 `setid/package`。
- [x] 禁止 UI/API/Integration/Assistant 各自拥有不同的访问模型或不同的命中解释链。
- [x] 禁止把组织上下文只当作“解析别名来源”，再在后台继续依赖另一套隐藏主键。
- [x] 禁止把 `package_uuid` 继续保留为域内私有主事实键、目录分桶键、ownership 容器或“内部保留、对外隐藏”的降级实现。
- [x] 禁止 runtime fallback：缺上下文、缺命中、命中歧义一律 fail-closed 或进入澄清。

## 4. 合同定义（主线）

### 4.1 读合同

- [x] 读请求应以“业务对象 + 组织上下文 + 时间锚点”为主输入。
- [x] 服务端负责在统一安全模型下完成：
  - 权限校验；
  - 组织上下文装配；
  - 配置/流程/候选值/只读状态命中。
- [x] 响应必须回显：
  - 业务对象主键；
  - 命中的组织上下文；
  - 时间锚点；
  - 命中的策略/流程/只读状态摘要。
- [x] 响应不得回显 `package_uuid`、`setid` 或任何等价容器键作为治理解释的一部分。

### 4.2 写合同

- [x] 写入最终锚定在领域对象主键与 `action request / business process` 上，而不是锚定在平台级 `setid/package` 或任何等价容器键上。
- [x] 组织上下文影响：
  - 谁可发起；
  - 路由给谁；
  - 需要哪些审批；
  - 哪些字段可见/必填/可维护。
- [x] 组织上下文**不应替代**领域对象主键，也不应被再包装成第二写主键。
- [x] 服务端不得先解析出 `package_uuid` 再把它作为真正写锚点进入数据库或审计链。

### 4.3 Explain 合同

- [x] Explain 至少回显：
  - `business_object_key`
  - `capability_key`
  - `org_context`
  - `as_of / effective_date`
  - `security_group / policy / process_definition`
  - `decision / reason_code`
- [x] Explain 需要能回答：
  - 为什么命中这个组织上下文；
  - 为什么这个人/角色在这个层级下可见或不可见；
  - 为什么这个流程/字段规则在当前上下文下生效。
- [x] Explain 不得再借由 `package_uuid`、`setid` 或其他容器键来补充主解释链。

## 5. 影响面清单（已裁决方向）

### 5.1 平台层（340/345/347）

- [x] `340` 需要把“统一访问模型”冻结为平台不变量，避免不同入口重算权限。
- [x] `345` 需要把 `DecisionContext` 收敛到显式 `OrgContext + Time + Capability`，而不是抽象出第二个数据集键。
- [x] `347` 需要继续保证 `capability_key` 不编码上下文；上下文差异只能由组织上下文与时间锚承担。

### 5.2 业务域（360/363/364）

- [x] Job Catalog 应优先保持统一目录骨架，避免通过 package/setid 复制目录来表达组织差异。
- [x] 组织、职位、任职、人员等域的差异更接近“组织上下文下的规则/可见性/流程差异”，而不是“另一套平台级主键体系”。
- [x] 各业务域应删除 `package_uuid` 作为治理容器、主事实键或解释链中间层的残留口径，并改写为显式业务对象、组织上下文、发布时间线与统一安全/流程语义。

### 5.3 Assistant 与工作台（380/390）

- [x] Assistant 需要消费与 UI/API 同源的组织上下文、安全模型和流程命中结果。
- [x] 不允许 Assistant 自行发明“更适合聊天”的第二套键词汇。
- [x] 对话澄清应优先围绕组织上下文、对象候选与审批/确认，而不是再引入 `setid/package` 技术词汇。

## 6. 与其他方案的关系

- [x] 相比 [DEV-PLAN-348A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348a-setid-package-single-source-candidate-plan.md)，本主线不保留 `setid` 作为统一上下文键，也不把 `package_uuid` 自动升级为平台通用治理键。
- [x] 相比 [DEV-PLAN-348B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348b-package-uuid-direct-governance-candidate-plan.md)，本主线更进一步：不仅取消 `setid`，也不保留 `package_uuid` 作为域内私有主事实键或任何降级落点，而是强调**组织上下文优先**。
- [x] 相比 [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md)，本主线不走 `capability_key + setid` 方向，而是走 `capability_key + org_context + time` 方向。
- [x] 本主线是**原则级正式口径**，不应被误读为“必须完全复刻 Workday”，但也不允许再退回旧容器键心智。

## 7. 实施 stopline 与证据要求

### 7.1 关键评估问题

- [ ] 是否能在不引入平台级或域内 `setid/package_uuid` 词汇的情况下，稳定表达组织差异、权限差异与流程差异？
- [ ] 是否会把“平台简化”变成“各域私下自定义上下文”，导致更难治理？
- [ ] 是否能让 UI / API / Integration / Assistant 真正共享一套访问模型与 Explain？
- [ ] 是否能减少 Job Catalog 等域的冗余维护，而不是把目录复制成多套隐式容器？
- [ ] 是否与 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的“减少偶然复杂度”方向一致？

### 7.2 必备证据（通过评估前）

- [ ] `OrgContext` 词汇表与标准字段清单（如 company / supervisory_org / business_site / location / role / manager-chain / worker attributes）。
- [ ] 关键旅程对照：同一业务旅程里，用户需要理解/输入的治理键数量是否少于 `348A/348B`。
- [ ] 关键路径测试证据（上下文命中、权限拒绝、流程路由、Explain 完整性、跨租户拒绝）。
- [ ] Job Catalog 减冗余证据：目录/配置是否能在不复制包、也不保留任何 `package_uuid` 变体的前提下表达组织差异。
- [ ] 门禁草案：阻断平台级与业务域内 `setid/package_uuid` 回流，阻断隐藏 alias、容器键降级与二次解释路径。

## 8. 实施承接（占位）

1. [ ] 新建实施文档（平台线承接编号待后续实施计划确定）。
2. [ ] 冻结平台级 `OrgContext` 词汇与字段口径。
3. [ ] 重写 `363` 及其直接下游引用口径，移除 `package_uuid/setid` 作为治理表达。
4. [ ] 统一 UI / API / Integration / Assistant 的 Explain 与访问模型。
5. [ ] 接入门禁，阻断 `setid/package_uuid` 作为治理键、容器键或隐藏 alias 回流。

## 9. 验收标准（主线落地通过标准）

- [ ] 满足 `348` 第 6 节全部停线项，无“一票否决”项。
- [ ] `348C` 已被后续子计划当作正式主线，而不是继续被当作“还可回退的参考方向”。
- [ ] 平台与业务域均不再保留 `setid/package_uuid` 作为治理键、容器键、隐藏 alias 或降级落点。
- [ ] 已确定实施承接文档与责任边界，并记录哪些 Workday 原则仅作为方法论、不可直接翻译为实现细节。

## 10. 关联文档与公开来源

### 10.1 仓库内文档

- [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md)
- [DEV-PLAN-348A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348a-setid-package-single-source-candidate-plan.md)
- [DEV-PLAN-348B](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348b-package-uuid-direct-governance-candidate-plan.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)

### 10.2 Workday 官方公开来源（原则级参考）

- [Workday Architectural Principles](https://www.workday.com/en-ca/pages/it/architectural-principles.html)
- [The Workday Business Process Framework](https://www.workday.com/content/dam/web/en-us/documents/datasheets/workday-business-process-framework.pdf)
- [Organization Management in Workday](https://www.workday.com/content/dam/web/en-us/documents/datasheets/organization-management-in-workday-datasheet-en-us.pdf)
- [Workday Security Datasheet](https://www.workday.com/content/dam/web/se/documents/datasheets/datasheet-workday-security-se.pdf)
- [Why You Need the Benefits of the Cloud Now / Cloud Architecture Matters](https://blog.workday.com/content/dam/web/en-us/documents/other/global-enus-it-gde-why-cloud-202006-digital.pdf)
- [Workday Government Cloud and Zero Trust](https://forms.workday.com/content/dam/web/en-us/documents/whitepapers/zero-trust-whitepaper-re-enus.pdf?refCamp=7014X000001yvgK)
- [Workday Transforms How Companies Hire and Manage Talent with New AI-Powered HR Solutions (Job Architecture)](https://newsroom.workday.com/2024-08-01-Workday-Transforms-How-Companies-Hire-and-Manage-Talent-with-New-AI-Powered-HR-Solutions)
