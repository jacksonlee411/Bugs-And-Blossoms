# DEV-PLAN-330：安全、合规与数据治理子计划

**状态**: 规划中（2026-03-17 07:23 CST）

## 1. 背景与上下文

HR 平台天然处理大量敏感数据：

- 人员信息
- 任职信息
- 导出文件
- 审批记录
- 操作审计

这些能力若只零散地放在 `340` 的 IAM 或 `370` 的审计里，会导致“知道重要，但没人真正拥有”。

`330` 用来冻结系统级的安全、合规和数据治理边界。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 定义敏感数据分级与访问策略。
- [ ] 定义导出、审批、审计、集成、Assistant 的治理边界。
- [ ] 定义租户隔离、机密管理、数据保留与删除策略。
- [ ] 定义合规与审计最小要求。

### 2.2 非目标

- [ ] 本计划不替代具体 IAM 和业务实现。
- [ ] 本计划不要求第一阶段就满足所有企业级认证与法务要求，但必须提供扩展边界。

## 3. 范围

- Data classification
- Secrets & key management
- Export governance
- Audit governance
- Tenant isolation policy
- Retention & deletion policy
- Assistant safety governance

## 4. 关键设计决策

### 4.1 敏感数据必须显式分级（选定）

- 普通业务数据
- 敏感人员数据
- 高风险导出数据

### 4.2 导出是高风险动作（选定）

- 导出必须受权限、审计和任务记录约束。

### 4.3 Assistant 纳入安全治理（选定）

- 对话日志、工具调用、动作建议、回执都必须纳入治理范围。
- `action request`、审批衔接与操作状态回执也属于治理对象。

### 4.4 应用层租户隔离必须 fail-closed（选定）

- `tenant_id` 视为不可变边界字段。
- 跨租户 ID 访问必须被应用层入口显式拒绝，不能只依赖查询层“自然过滤”。
- Dapper / Raw SQL 必须经过 tenant-scoped 查询抽象、审查规则或等效护栏，避免手写 SQL 穿透租户边界。

## 5. 交付范围

- [ ] 数据分级矩阵
- [ ] 导出治理策略
- [ ] 审计保留与不可抵赖性要求
- [ ] 租户隔离策略
- [ ] tenant-scoped SQL 审查与防穿透要求
- [ ] 密钥与配置治理
- [ ] 数据保留/删除策略

## 6. 验收标准

- [ ] 敏感数据、导出、审计、集成、Assistant 都有明确治理边界。
- [ ] 系统安全责任不再散落于各业务子计划的备注中。
- [ ] 租户隔离与 Raw SQL 穿透风险有明确 fail-closed 约束，不靠默认约定“自觉遵守”。

## 7. 后续拆分建议

1. [ ] [DEV-PLAN-331：敏感数据分级与访问治理详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/331-sensitive-data-classification-and-access-governance-detailed-design.md)
2. [ ] [DEV-PLAN-332：导出、审计与留存策略详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/332-export-audit-and-retention-governance-detailed-design.md)
3. [ ] [DEV-PLAN-333：租户隔离、tenant-scoped SQL、密钥与 Assistant 安全治理详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
