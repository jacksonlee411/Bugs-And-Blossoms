# DEV-PLAN-332：导出、审计与留存策略详细设计

**状态**: 规划中（2026-03-18 17:26 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 的 `332` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“导出是正式能力、审计与回执要可追溯、Assistant 受控交互”的冻结；
- [DEV-PLAN-331](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/331-sensitive-data-classification-and-access-governance-detailed-design.md) 对数据分级、暴露面与最小披露的冻结；
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 对租户隔离、tenant-scoped SQL、密钥与 Assistant 安全治理的冻结；
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md) 对票据、回执、快照分层的冻结；
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md) 对平台审计、通知、后台任务基座的冻结。

`330` 已明确需要导出治理、审计留存与删除策略，但若缺少 `332`：

- 导出能力容易与普通读取混同，出现超量披露；
- 审计与回执保留期限不一致，证据链不可解释；
- 删除任务可能误删高风险证据或违规保留；
- 控制面、数据工作台、Assistant 对留存规则口径不一致；
- “先导出再治理”变成长期技术债。

`332` 的职责是把上述问题收敛为 **可执行、可审计、可证明的留存与导出治理合同**。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结导出请求、导出数据集、导出产物、导出回执的共享治理语义。
- [ ] 冻结审计、回执、快照、导出文件的留存分层与最小留存要求。
- [ ] 冻结 `LegalHold`（法务/调查冻结）与例外处置合同。
- [ ] 冻结删除与清理策略（何时删、谁可删、如何审计、如何恢复判断）。
- [ ] 冻结高风险导出与高风险聚合数据集的审批/审计/追踪要求。
- [ ] 为 `342/344/370/380/390` 提供统一输入，避免各模块自造保留口径。

### 2.2 非目标

- [ ] 本计划不替代 [DEV-PLAN-331](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/331-sensitive-data-classification-and-access-governance-detailed-design.md) 的分级与最小披露真值。
- [ ] 本计划不替代 [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 的租户与安全 stopline。
- [ ] 本计划不替代 [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的权限矩阵；仅定义导出/留存治理边界。
- [ ] 本计划不要求第一阶段覆盖所有国家法域条款，但必须预留可扩展治理位。
- [ ] 本计划不允许用“内部调试”作为无限期保留高敏导出或日志原值的理由。

## 3. “业务规则优先”在导出与留存治理中的翻译

### 3.1 用户真正关心的是“谁拿走了什么、保存多久、何时删除”

用户关心的是：

- 同样的数据为什么在在线查看、导出文件、Assistant 回复里展示程度不同；
- 高风险导出是否可追踪、可审计、可追责；
- 系统是否会在应删时删除、在需保留时保留。

### 3.2 导出是独立治理对象，不是查询功能附带

`332` 冻结：

- 导出必须有独立请求与回执对象；
- 导出产物必须可追溯到目的、范围、操作者与审批/授权依据；
- 导出后的留存与删除策略不得依赖调用方自行处理。

### 3.3 留存策略先回答“证据价值与风险等级”，再回答存储成本

- 不同对象（审计、回执、快照、导出文件）留存语义不同；
- 保留过短会丢失证据，保留过长会增加泄露风险；
- 留存策略必须显式可审计，不能隐藏在脚本默认值。

### 3.4 删除是受控动作，不是清理脚本细节

- 删除与 purge 必须有触发依据、执行回执与可审计记录；
- 法务冻结期间不得执行冲突删除；
- 不允许“手工删文件”绕开治理。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md) 已明确导出治理、审计留存与删除策略属于系统治理。
- [DEV-PLAN-331](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/331-sensitive-data-classification-and-access-governance-detailed-design.md) 已明确分级、暴露面、最小披露与 `RetentionPolicy` 由 `332` 承接。
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 已明确导出与高风险查询必须 tenant-scoped 且可审计。
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md) 已明确回执与快照分层语义。
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md) 已明确平台审计与后台任务基座。

### 4.2 当前主要缺口

1. [ ] **缺少导出治理对象模型**  
   目前缺少统一的导出请求、导出产物、导出回执语义。

2. [ ] **缺少留存分层矩阵**  
   审计日志、回执、快照、导出文件尚未形成统一保留边界。

3. [ ] **缺少冻结与删除冲突处理规则**  
   `LegalHold` 与 purge 的优先级及处置流程未冻结。

4. [ ] **缺少跨模块一致的执行输入**  
   `380` 导出中心、`390` Assistant、`344` 平台任务尚无统一留存治理输入。

## 5. 导出与留存治理蓝图

### 5.1 领域使命

`332` 是平台内“**什么数据可以导出、导出后如何受控留存、何时必须删除、何时必须冻结保留，以及这些过程如何可审计**”的共享治理权威。

### 5.2 核心治理对象

| 治理对象 | 治理含义 | 是否由 `332` 拥有共享合同 |
| --- | --- | --- |
| `ExportRequest` | 导出意图、范围、用途、触发主体 | 是 |
| `ExportDatasetProfile` | 导出字段集与风险等级画像 | 是 |
| `ExportArtifact` | 实际生成的导出文件与元数据 | 是 |
| `ExportReceipt` | 导出执行阶段回执 | 是 |
| `RetentionPolicy` | 留存周期与删除策略定义 | 是 |
| `LegalHold` | 法务/调查冻结规则 | 是 |
| `DeletionSchedule` | 到期删除计划与执行窗口 | 是 |
| `PurgeJob` | 清理任务主档与运行记录 | 是 |
| `AuditEvidenceRef` | 审计/回执/快照的留存引用关系 | 是（共享关系） |

### 5.3 面向系统的主能力

- 把导出升级为可审计、可授权、可回执的正式能力；
- 根据数据分级与暴露面执行差异化留存；
- 支持冻结保留、例外保留与到期清理的共存；
- 让导出、审计、工作台、Assistant 使用同一治理语言。

## 6. `332` 冻结的目标规则矩阵

| 场景 | 用户真正要做什么 | 核心治理规则 | 治理结果 |
| --- | --- | --- | --- |
| 发起导出 | 获取可离线流转数据 | 必须有 `ExportRequest`，显式用途、租户、字段范围、风险级别 | 导出请求可追溯 |
| 下载导出文件 | 获取导出产物 | 文件必须绑定过期策略与访问审计 | 文件访问可控 |
| 查询导出进度 | 确认执行状态 | 导出任务必须有回执链，不允许“只看是否生成文件” | 执行状态可解释 |
| 审计留存 | 保留关键证据 | 审计、回执、快照按分层策略留存，不得混同 | 证据链完整可审计 |
| 法务冻结 | 暂停删除 | `LegalHold` 生效后相关对象暂停 purge | 调查期证据可保全 |
| 到期清理 | 降低风险与成本 | 清理必须经 `DeletionSchedule + PurgeJob`，保留执行回执 | 删除可追踪可证明 |

## 7. 共享合同、不变量与实现护栏

### 7.1 导出合同

- 导出必须显式声明：
  - `tenant_id`
  - 调用主体
  - 导出用途（`AccessPurpose`）
  - 时间视图
  - 字段范围与风险级别
- 导出产物必须绑定：
  - 创建时间
  - 到期时间
  - 访问审计引用
  - 对应回执/任务引用
- 不允许“临时 SQL + 本地下载”绕过导出中心。

### 7.2 高风险数据集合同

- `HighRiskDataset` 规则沿用 `331`，`332` 负责落地导出与留存执行面。
- 高风险导出默认要求更严格授权、审计与更短有效下载窗口。
- 未分级字段不得进入高风险导出链路。

### 7.3 审计与回执留存合同

- 平台审计留存遵循 `344` 基座，留存周期由 `332` 统一治理。
- 长事务回执与证据快照遵循 `323` 分层，留存策略由 `332` 统一映射。
- 不允许把审计、回执、快照合并成单一“万能留存桶”。

### 7.4 留存策略分层合同

- `RetentionPolicy` 至少应区分：
  - 平台审计事件
  - 业务回执/快照
  - 导出产物
  - 通知/任务运行记录
- 每类对象须明确：
  - 最小留存期
  - 最大留存期（如适用）
  - 到期动作（删除、归档、降敏保留）
  - 例外条件

### 7.5 `LegalHold` 与例外合同

- `LegalHold` 必须显式绑定对象范围、生效时间、触发原因、审批来源。
- 持有 `LegalHold` 的对象不得被 purge。
- 冻结解除必须有审计事件与执行回执。

### 7.6 删除与清理执行合同

- 删除必须由 `DeletionSchedule` 触发并通过 `PurgeJob` 执行。
- `PurgeJob` 必须记录：
  - 执行范围
  - 执行时间
  - 执行结果
  - 失败摘要
  - 关联审计 ID
- 删除失败不得静默吞掉，必须形成可查询回执。

### 7.7 Assistant 与控制面合同

- Assistant 生成导出建议与执行请求时必须遵守 `332 + 331 + 333` 联合约束。
- 控制面只可在授权与审计下操作留存策略，不得绕过 `LegalHold`。

### 7.8 stopline

- 不允许未分级数据直接导出到离线产物。
- 不允许导出文件无到期策略或无访问审计。
- 不允许在 `LegalHold` 生效期间执行冲突删除。
- 不允许通过手工脚本绕过 `PurgeJob` 与审计记录。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `342`（AuthZ 与权限矩阵）的输入

- [ ] 导出、留存配置、冻结解除、批量删除需明确独立权限，不得合并为 generic admin。
- [ ] 权限决策应与导出/删除审计事件关联可追溯。

### 8.2 对 `344`（平台审计/通知/任务基座）的输入

- [ ] `PurgeJob` 与导出执行任务应复用统一后台任务基座。
- [ ] 留存策略变更、冻结/解冻、删除执行都必须写入平台审计。

### 8.3 对 `370`（工作流、审计增强与集成）的输入

- [ ] 高风险导出与删除例外可接入审批流。
- [ ] 集成出站的数据留存应消费 `332`，不得自建保留窗口。

### 8.4 对 `380`（数据工作台与运营分析）的输入

- [ ] 导出中心必须消费 `ExportRequest/ExportArtifact/ExportReceipt` 共享对象。
- [ ] 工作台应可展示导出到期、冻结状态与可见回执。

### 8.5 对 `390`（Chat Assistant）的输入

- [ ] Assistant 在导出相关问答与动作建议中必须遵守同一留存与降敏规则。
- [ ] Assistant 不得绕过导出中心直接组装高风险离线结果。

## 9. 建议实施分期

1. [ ] `M1`：治理对象与术语冻结  
   冻结导出/留存/冻结/删除共享对象模型。
2. [ ] `M2`：导出合同与高风险数据集接线  
   冻结导出请求、产物、回执与高风险治理规则。
3. [ ] `M3`：留存分层矩阵冻结  
   冻结审计、回执、快照、导出文件的分层留存策略。
4. [ ] `M4`：`LegalHold` 与 purge 执行链冻结  
   冻结冻结优先级、解除流程、删除执行与回执规则。
5. [ ] `M5`：下游计划引用收口  
   让 `342/344/370/380/390` 正式消费 `332`，停止重写留存规则。

## 10. 验收标准

- [ ] `332` 已成为导出、审计与留存策略的单一事实源。
- [ ] 导出、审计、回执、快照、导出文件的留存分层清晰且可执行。
- [ ] 冻结、删除、例外处置具备可审计与可回放证据。
- [ ] 高风险导出与 Assistant 相关输出已纳入统一治理，不再各自例外。
- [ ] `342/344/370/380/390` 可直接消费 `332`，不再重复发明保留口径。

## 11. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md)
- [DEV-PLAN-330](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-security-compliance-and-data-governance-plan.md)
- [DEV-PLAN-331](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/331-sensitive-data-classification-and-access-governance-detailed-design.md)
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md)
- [DEV-PLAN-344](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/344-audit-notification-and-background-jobs-foundation-detailed-design.md)
- [DEV-PLAN-380](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/380-data-workbench-and-operational-analytics-plan.md)
- [DEV-PLAN-390](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/390-chat-assistant-capability-plan.md)
