# DEV-PLAN-320：共享数据架构与建模约定子计划

**状态**: 规划中（2026-03-17 22:41 CST）

## 1. 背景与上下文

`340/360/370/380/390` 都会落到数据库、查询、历史、租户、审计和读写边界上。  
如果没有一个共享的数据架构计划，后续子计划会各自发明：

- effective date 语义
- 主表与历史表模式
- 审计快照结构
- 代码与 ID 约定
- 租户可扩展能力（字段/字典/策略）的共享边界
- EF Core 与 Dapper 的边界

`320` 的任务就是把这些系统级数据约定冻结下来。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 定义全系统统一的数据建模基本规则。
- [ ] 定义 effective date / current view / history view 的统一口径。
- [ ] 定义主档、历史、审计、任务、会话等常见模型模式。
- [ ] 定义 EF Core 与 Dapper/SQL 的职责边界。
- [ ] 定义时间区间完整性与数据库兜底约束的共享口径。

### 2.2 非目标

- [ ] 本计划不替代具体业务模块的详细 schema 设计。
- [ ] 本计划不定义所有字段，只定义共享约定与模式。

## 3. 核心主题

- Tenant 字段规范
- ID / Code 约定
- 租户可扩展能力（字段/字典/策略）
- 生效日期与历史模型
- 时间区间完整性约束
- 主表 + version 表模式
- 审计快照模式
- 查询与分页约定
- EF Core / Dapper 边界

## 4. 关键设计决策

### 4.1 历史优先采用“主表 + 版本表”模式（选定）

- 主表承载稳定标识。
- 版本表承载有效期切片。
- 查询 current / as-of / history 时有统一路径。

### 4.2 时间语义统一到日粒度（选定）

- 业务生效日期采用 `date` 粒度。
- 审计时间采用 timestamp。
- 对 effective-dated 对象的读取必须显式表达 `current / as-of / history` 语义。

### 4.3 复杂读用 SQL，常规写用 ORM（选定）

- 常规业务写入：EF Core
- 复杂读模型、报表、搜索：Dapper/SQL
- 时间切片、层级检索等复杂路径允许在详细设计中直接使用数据库原生能力，不强求 ORM 单栈覆盖。

### 4.4 JSON 字段只作补充，不作默认主模型（选定）

- 关键业务字段必须显式建模。
- JSON 只用于少量扩展属性或快照。

### 4.5 共享不变量优先由数据库能力兜底（选定）

- 同主体、同自然键下的有效期重叠、唯一性与区间完整性，不应只依赖应用层校验。
- 应用层校验负责更好的错误反馈，数据库约束负责最终兜底。
- 具体采用哪些 PostgreSQL 区间类型、排他约束或索引策略，下沉到 `322 / 324` 详细设计冻结；租户可扩展能力的共享业务模型与边界由 `321` 冻结。

## 5. 交付范围

- [ ] 建模约定清单
- [ ] 表命名与字段命名约定
- [ ] 历史/审计模式模板
- [ ] 时间语义与区间约束模板
- [ ] 查询与分页规范
- [ ] ORM 与 SQL 的边界规范

## 6. 验收标准

- [ ] 后续业务子计划都能引用本计划，而不是各自重写数据约定。
- [ ] 历史、生效日期、审计和查询模式有统一语言。
- [ ] 不再出现“某模块用 ORM 生成历史，另一模块用 JSON 拼历史”的双轨。
- [ ] effective-dated 查询语义与区间完整性约束有统一契约，且不依赖隐式默认过滤器。

## 7. 后续拆分建议

1. [ ] [DEV-PLAN-321：租户可扩展能力（字段/字典/策略）业务规则优先蓝图与共享模型方案](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/321-tenant-extensibility-business-rules-and-shared-model-plan.md)
2. [ ] [DEV-PLAN-322：历史、生效日期、区间完整性与 `current / as_of / history` 详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)
3. [ ] [DEV-PLAN-323：审计、任务、会话与快照模式详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md)
4. [ ] `324`：EF Core Query Filter、Dapper/SQL 与数据库原生能力边界详细设计
