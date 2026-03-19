# DEV-PLAN-348A：`setid/package` 单主源治理候选方案（待评估）

**状态**: 草拟中（2026-03-19 07:24 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md) 的首个候选方案文档。  
目标是提交“`setid` 与 `package` 如何避免双主源”的可评估方案，不在本计划内直接执行切换。

本候选默认承接：

- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md) 中“`package_uuid` 是治理维度、`setid` 是入口上下文”的方向；
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 的 `capability_key + context + as_of`；
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md) 的能力键防退化与 fail-closed。

## 2. 候选方案陈述（A 方案）

### 2.1 主张

- [ ] `package_uuid` 作为主事实键（Primary Fact Key）。
- [ ] `setid` 作为上下文键（Context Key），用于入口选择与解析，不作为核心写主键。
- [ ] 读写前统一执行规范化解析：`tenant + setid + as_of -> package_uuid`。
- [ ] 规范化后全链路仅使用 `package_uuid` 执行写入、唯一约束与关联校验。

### 2.2 单主源约束

- [ ] 禁止“`setid` 可独立决定写入目标”。
- [ ] 禁止“`setid` 与 `package_uuid` 同时作为可替代写主键”。
- [ ] 禁止“请求中 `setid/package_uuid` 不一致仍继续执行”。
- [ ] 禁止 runtime global fallback；共享仅允许“发布到租户本地后读取”。

## 3. 合同定义（候选）

### 3.1 读合同

- [ ] 允许前端以 `setid + as_of` 进入上下文；
- [ ] 服务端必须回显规范化后的 `package_uuid` 与 `read_only`；
- [ ] 无法解析唯一 `package_uuid` 时 fail-closed。

### 3.2 写合同

- [ ] 写接口最终以 `package_uuid` 为权威参数；
- [ ] 若保留 `setid` 入参，服务端必须做一致性校验（不一致即拒绝）；
- [ ] DB 约束（UNIQUE/FK/索引）统一锚定 `(tenant_uuid, package_uuid, ...)`。

### 3.3 Explain 合同

- [ ] Explain 至少回显：`capability_key`、`setid`、`as_of`、`package_uuid`、`policy_version`；
- [ ] 必须能解释“为何命中这个 package、为何只读/可写”。

## 4. 影响面清单（待评估）

### 4.1 平台层（340/345/347）

- [ ] 需要统一“主事实键 vs 上下文键”词汇，不得回流成并列主键。
- [ ] 需要门禁阻断新增双主源接口合同。

### 4.2 业务域（360/363/364）

- [ ] Job Catalog 继续收敛到 `package_uuid` 主键，`setid` 仅作入口上下文。
- [ ] Staffing/Person/Org 对 Job Catalog 的引用统一走规范化后的 `package_uuid` 事实。

### 4.3 Assistant 与工作台（380/390）

- [ ] 对话与导出必须携带并回显规范化结果，避免“用户语义键”和“写入主键”断裂。
- [ ] 歧义场景（多包、多候选）必须先澄清再提交。

## 5. 与其他方案的关系

- [ ] 本候选与 [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md) 的“去 package 化”方向存在差异；
- [ ] `348` 评估阶段要求两类方向按同一维度比较，不允许先入为主；
- [ ] 在 `348` 形成裁决前，本候选仅作为待评估方案，不直接替代既有 SSOT。

## 6. 评估问题与证据要求

### 6.1 关键评估问题

- [ ] 是否真正消除 `setid/package` 双主源，而非仅换名保留双路径？
- [ ] 是否保持 `current/as_of/history` 一致性与回放确定性？
- [ ] 是否降低跨模块认知复杂度，而不是增加解析层复杂度？
- [ ] 是否满足租户隔离、只读共享、发布治理与 fail-closed？

### 6.2 必备证据（通过评估前）

- [ ] 合同一致性对照表（API/DB/UI/Explain）。
- [ ] 关键路径测试证据（读写、歧义拒绝、版本一致性、跨租户拒绝）。
- [ ] 数据迁移与回滚演练记录（含异常分支处理）。
- [ ] 门禁草案与误报评估结果。

## 7. 若评估通过的实施承接（占位）

1. [ ] 新建实施文档（`360` 线或平台线承接编号，由评审会确定）。
2. [ ] 冻结迁移窗口、回填策略与回滚路径。
3. [ ] 分批替换接口合同并补齐门禁。
4. [ ] 完成全链路验证后封板并归档本候选。

## 8. 验收标准（候选评估通过标准）

- [ ] 满足 `348` 第 6 节全部停线项，无“一票否决”项。
- [ ] 形成明确结论：通过/否决/需补证据，并记录原因。
- [ ] 若通过，已确定实施承接文档与责任边界；若否决，已记录替代方向。

## 9. 关联文档

- [DEV-PLAN-348](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/348-platform-key-governance-evaluation-framework.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-347](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/347-capability-and-granularity-governance-plan.md)
- [DEV-PLAN-363](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
- [DEV-PLAN-102C6](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md)
