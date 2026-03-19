# DEV-PLAN-348B：取消 `setid`、收敛为 `package_uuid` 直达治理候选方案（待评估）

**状态**: 草拟中（2026-03-19 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md) 的第二个候选方案文档。  
目标是提交“取消 `setid` 作为平台级治理词汇，由业务上下文直接解析到 `package_uuid`”的可评估方案，不在本计划内直接执行切换。

本候选默认承接：

- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 中 `capability_key + context + as_of` 的统一决议协议；
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 中能力键防退化、映射单点化与 fail-closed；
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 中 `package_uuid` 已成为 Job Catalog 配置事实主键的现行收敛方向；
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 关于“从零重做时优先减少偶然复杂度、避免继承历史包袱”的上位方法论。

## 2. 候选方案陈述（B 方案）

### 2.1 主张

- [ ] `package_uuid` 作为主事实键（Primary Fact Key）。
- [ ] `setid` 退出平台级治理词汇，不再作为读写合同、Explain 合同或 UI 合同的必备字段。
- [ ] 业务上下文直接解析到 `package_uuid`，解析入口形态为：`tenant + org_context + domain + as_of -> package_uuid`。
- [ ] 规范化后全链路仅使用 `package_uuid` 执行写入、唯一约束、关联校验与审计锚定。

### 2.2 简化约束

- [ ] 禁止把 `setid` 作为 `package_uuid` 的别名继续暴露在 API/UI/Explain 中。
- [ ] 禁止“业务上下文直达 `package_uuid`”与“`setid -> package_uuid` 间接解析”长期并存。
- [ ] 禁止在缺少唯一 `package_uuid` 时通过 runtime fallback 猜测默认包。
- [ ] 禁止以“内部保留 `setid`、对外隐藏”方式形成第二心智与第二解释路径。

## 3. 合同定义（候选）

### 3.1 读合同

- [ ] 业务页面允许通过显式业务上下文进入（如 `org_unit_id`、`business_unit_id`、领域自有上下文字段），也允许在配置工作台直接选择 `package_uuid`。
- [ ] 服务端必须回显规范化后的 `package_uuid` 与 `read_only`。
- [ ] 无法解析唯一 `package_uuid` 时 fail-closed；若产品层需要继续交互，必须先进入候选澄清，而不是静默兜底。

### 3.2 写合同

- [ ] 写接口最终以 `package_uuid` 为唯一权威参数。
- [ ] 若写操作起点是业务上下文，服务端必须先完成“上下文 -> package_uuid”规范化，再进入写校验与提交。
- [ ] DB 约束（UNIQUE/FK/索引）统一锚定 `(tenant_uuid, package_uuid, ...)`。

### 3.3 Explain 合同

- [ ] Explain 至少回显：`capability_key`（如适用）、`org_context`、`as_of`、`package_uuid`、`policy_version`。
- [ ] 必须能解释“为何命中这个 package、为何只读/可写、为何在当前上下文下无唯一 package”。
- [ ] Explain 不得再要求或回显 `setid` 作为平台级治理字段。

## 4. 影响面清单（待评估）

### 4.1 平台层（340/345/347）

- [ ] 需要明确平台只冻结“主事实键 / 上下文键 / 时间锚点”的角色，不强制引入跨域统一 `setid` 层。
- [ ] 需要门禁阻断新增 `setid` 平台级合同回流，防止 package-only 候选被暗中改回双层解析。

### 4.2 业务域（360/363/364）

- [ ] Job Catalog 可继续以 `package_uuid` 为唯一治理主键，不再额外暴露 `setid`。
- [ ] Dict、Org 扩展配置等其他消费域若需要“上下文选包”，应各自声明显式上下文与直达 `package_uuid` 的解析规则，而不是默认依赖统一 `setid` 抽象。
- [ ] 跨域引用不得再假定“共享同一个 `setid` 即共享同一个 package 事实”；如存在共享关系，必须通过显式 package 或显式上下文重新解析。

### 4.3 Assistant 与工作台（380/390）

- [ ] 对话、导出与工作台必须携带并回显规范化后的 `package_uuid`，避免“用户看到的是业务上下文，系统落的是另一套隐式键”。
- [ ] 歧义场景（多包、多候选）必须先澄清再提交；Assistant 不得自行虚构 `setid` 或 package 别名。

## 5. 与其他方案的关系

- [ ] 本候选与 [DEV-PLAN-348A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348a-setid-package-single-source-candidate-plan.md) 的差异在于：`348A` 保留 `setid` 作为统一上下文键，本候选取消 `setid` 的平台级治理地位。
- [ ] 本候选与 [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md) 一样追求概念减法，但方向相反：`102C6` 试图保留 `setid`、删除 package；本候选试图保留 `package_uuid`、删除 `setid`。
- [ ] `348` 评估阶段要求 `348A` 与 `348B` 按同一维度比较，不允许先入为主。
- [ ] 在 `348` 形成裁决前，本候选仅作为待评估方案，不直接替代既有 SSOT。

## 6. 评估问题与证据要求

### 6.1 关键评估问题

- [ ] 是否真正降低用户与开发者的认知复杂度，而不是把复杂度下沉为多个域内私有解析器？
- [ ] 是否避免“取消 `setid` 名称，但保留等价二层解析”的伪简单化？
- [ ] 是否保持 `current/as_of/history` 一致性与回放确定性？
- [ ] 是否满足租户隔离、只读共享、发布治理与 fail-closed？
- [ ] 是否会削弱跨域治理一致性，导致各域重新发明自己的上下文选包合同？

### 6.2 必备证据（通过评估前）

- [ ] 合同一致性对照表（API/DB/UI/Explain），明确哪些字段被保留、删除或改名。
- [ ] 关键旅程认知复杂度对照：同一业务旅程中，用户需要理解/输入的治理键数量、页面字段数量、澄清次数。
- [ ] 关键路径测试证据（读写、歧义拒绝、版本一致性、跨租户拒绝、缺 package 拒绝）。
- [ ] 数据迁移与回滚演练记录（含“已有 `setid` 历史数据如何收口”为 `package_uuid` 直达口径）。
- [ ] 门禁草案与误报评估结果（含禁止 `setid` 平台词汇回流的检查方案）。

## 7. 若评估通过的实施承接（占位）

1. [ ] 新建实施文档（平台线或 `360` 线承接编号，由评审会确定）。
2. [ ] 冻结“业务上下文 -> package_uuid”解析合同与各域责任边界。
3. [ ] 分批替换 API/UI/Explain 合同，删除 `setid` 平台级字段与相关门禁例外。
4. [ ] 完成全链路验证后封板并归档本候选。

## 8. 验收标准（候选评估通过标准）

- [ ] 满足 `348` 第 6 节全部停线项，无“一票否决”项。
- [ ] 形成明确结论：通过/否决/需补证据，并记录原因。
- [ ] 若通过，已确定实施承接文档与责任边界；若否决，已记录替代方向。

## 9. 关联文档

- [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md)
- [DEV-PLAN-348A](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348a-setid-package-single-source-candidate-plan.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md)
