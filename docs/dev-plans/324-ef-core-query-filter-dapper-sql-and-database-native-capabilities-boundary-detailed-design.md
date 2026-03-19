# DEV-PLAN-324：EF Core Query Filter、Dapper/SQL 与数据库原生能力边界详细设计

**状态**: 规划中（2026-03-18 15:34 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 的 `324` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 对“写模型以 `EF Core` 为主、复杂读以 `Dapper/SQL` 为主、不得依赖隐式查询过滤器承担全部边界”的冻结；
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 对“显式时间上下文、禁止隐式 today、`current / as_of / history` 必须分层表达”的冻结；
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md) 对“票据、回执、快照与当前状态分层”的冻结；
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 对“tenant-scoped SQL、Raw SQL fail-closed、后台任务与导出查询不得穿透租户边界”的冻结。

`320` 已经把共享数据建模语言冻结到一个足够高的层次，但如果没有 `324`，后续实现很容易继续出现：

- `EF Core`、`Query Filter`、`Dapper`、手写 SQL 的边界各模块各写一版；
- 把 `current = today`、tenant filter、业务上下文偷藏进隐式过滤器；
- 导出、报表、层级树、时间切片查询在实现时临时绕过租户护栏；
- 写事务、复杂读与确认回执各自走不同连接/事务边界，导致结果不一致。

`324` 的职责就是把这些问题收敛为 **Greenfield 持久化执行边界 SSOT**。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述 EF Core、Query Filter、Dapper/SQL 与数据库原生能力的分工，不让 ORM 与框架偏好喧宾夺主。
- [ ] 冻结标准写路径、标准读路径、复杂读路径与批量读路径的共享执行边界。
- [ ] 冻结 `Query Filter` 可以承载什么、禁止承载什么，避免其重新变成隐式时间语义或隐式越权通道。
- [ ] 冻结 `tenant-scoped SQL`、时间视图归一、事务共享与回执确认等跨实现路径的共同合同。
- [ ] 冻结数据库原生能力的准入边界，明确哪些能力是正式工具，哪些能力需要显式 ownership 与审查。
- [ ] 为 `340/360/370/380/390` 提供统一输入，阻断各模块再次发明第二套 ORM / SQL 约定。

### 2.2 非目标

- [ ] 本计划不直接定义具体业务表 DDL、迁移脚本或索引实现细节。
- [ ] 本计划不替代 [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 的共享时间合同，也不替代 [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md) 的状态与证据合同。
- [ ] 本计划不替代 [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 的安全 stopline；它消费 `333` 的 tenant-scoped SQL 护栏。
- [ ] 本计划不把所有对象强行收敛成同一种 repository 形状，也不要求所有查询都走 ORM。
- [ ] 本计划不允许为了实现省事而把业务时间、租户边界或权限语义藏进隐式默认值与“自然过滤器”。

## 3. “业务规则优先”在持久化执行边界中的翻译

### 3.1 用户真正关心的是“数据正确、边界正确、结果可解释”，不是 ORM 选型争论

用户与业务方真正关心的是：

- 当前看到的记录是否属于正确租户；
- 看到的是当前态、某日快照还是完整历史；
- 导出与报表是否与业务 UI 使用同一条语义；
- 写入后返回的确认摘要、审批票据与回执是否基于同一事实源。

### 3.2 Query Filter 是防线，不是第二业务语言

`324` 冻结：

- `Query Filter` 可以承载平台级防线与少量共享守卫；
- `Query Filter` 不能回答“今天是什么”“当前记录是哪条”“当前页面属于哪个时间视图”；
- `Query Filter` 更不能偷偷承载 capability、上下文、审批态或页面只读语义。

### 3.3 SQL 是一等产品能力，不是 ORM 失败后的退路

对于层级树、时间切片、报表、导出、搜索与批量处理：

- `Dapper/SQL` 是正式能力；
- 数据库原生能力是产品工具箱的一部分；
- 但它们必须落在显式 tenant、显式时间视图、显式用途声明和显式 ownership 下。

### 3.4 数据库原生能力允许使用，但必须带 ownership

`324` 不把 PostgreSQL 原生能力视为“危险捷径”，也不把它们变成随处可用的魔法。  
允许使用不等于允许无约束散落。

## 4. 当前基线：已沉淀的共享结论

### 4.1 已稳定的 Greenfield 方向

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 已明确：
  - 标准写路径以 `EF Core` 为主；
  - 复杂读、报表、导出、搜索以 `Dapper/SQL` 为主；
  - 生效日期时间切片、层级树检索与复杂报表不应强行依赖单一 ORM 的隐式能力；
  - 跨租户访问不得仅依赖隐式查询过滤器。
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 已明确：
  - `EF Core / Dapper` 的共享边界必须正式冻结；
  - `324` 是该问题域的专属承接计划。
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 已明确：
  - `current / as_of / history` 必须显式表达；
  - 禁止隐式 today；
  - 时间合同的实现边界由 `324` 承接。
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md) 已明确：
  - 任何 Dapper / Raw SQL / 导出查询都必须通过 tenant-scoped 护栏；
  - 缺少租户上下文的 SQL 执行属于 stopline 级问题。

### 4.2 当前主要缺口

1. [ ] **缺少 Query Filter 的准入合同**  
   目前只有“不能过度依赖”的原则，但还没有一份详细计划说明它到底能做什么、不能做什么。

2. [ ] **缺少 EF 与 Dapper 共享事务边界的统一语言**  
   写事务后立即回读确认摘要、导出与回执联动等场景，很容易各自长出不同连接与事务策略。

3. [ ] **缺少数据库原生能力的正式准入表述**  
   如果不冻结，后续会在“全靠 ORM”与“随手写 SQL”之间反复摆动。

4. [ ] **缺少面向下游计划的共享输入**  
   `360/370/380/390` 都会碰到复杂读与批处理，但目前没有一份可以直接引用的执行边界文档。

## 5. EF Core / Dapper / DB Native 边界蓝图

### 5.1 领域使命

`324` 是 Greenfield 平台内“**不同持久化执行路径分别回答什么问题、共享时间与租户合同如何落地、何时允许数据库原生能力进入系统**”的共享工程权威。

### 5.2 核心执行对象

| 执行对象 | 执行含义 | 是否由 `324` 拥有共享合同 |
| --- | --- | --- |
| `TenantScopedDbContext` | 标准写路径与常规读路径使用的上下文边界 | 是 |
| `QueryFilterPolicy` | 平台级隐式过滤护栏的准入范围 | 是 |
| `TimeViewQuerySpec` | `current / as_of / history` 在持久化层的显式时间视图表达 | 是 |
| `TenantScopedSqlQuery` | 复杂读、导出、报表与批处理的 SQL 执行合同 | 是 |
| `ReadModelQuery` | 面向列表、报表、搜索、工作台的只读查询对象 | 是 |
| `PersistenceTransactionBoundary` | EF 与 SQL 混合路径的连接/事务一致性合同 | 是 |
| `DbNativeCapabilityProfile` | 区间、排他约束、递归、窗口函数等数据库原生能力的准入描述 | 是 |
| `DomainRepository` | 具体业务聚合仓储 | 否，领域模块拥有，但必须消费 `324` 的边界 |

### 5.3 面向系统的主能力

- 用统一规则区分写模型、常规读模型与复杂读模型；
- 让 `current / as_of / history` 在持久化层始终显式可见；
- 让 ORM 与 SQL 在同一租户/事务/时间合同下协同工作；
- 让导出、报表、层级树与批处理成为正式读路径，而不是旁路；
- 让数据库原生能力可用、可审查、可复用，而不是散落为局部技巧。

## 6. `324` 冻结的目标规则矩阵

| 场景 | 系统真正要做什么 | 核心执行规则 | 执行结果 |
| --- | --- | --- | --- |
| 聚合写入 | 保存标准业务写操作 | 优先走 `EF Core`；事务边界显式；不得把复杂报表读混入同一路径 | 写入边界稳定 |
| 当前/某日/历史读取 | 回答“当前、某日、历史到底看什么” | 必须显式绑定 `TimeViewQuerySpec`；禁止隐式 today | 时间语义不漂移 |
| 层级树/时间切片查询 | 回答复杂结构化读问题 | 允许直接使用 SQL / DB native capability；不得伪装成普通 ORM 读 | 复杂读可控 |
| 导出/报表 | 回答批量、跨模块、只读查询 | 必须走 `TenantScopedSqlQuery`；显式声明 tenant、用途与时间视图 | 导出不越界 |
| 写后确认摘要 | 回答“这次提交后系统认可了什么” | EF 与 SQL 混合路径必须共享连接/事务或明确一致性边界 | 回执可解释 |
| 后台批处理 | 回答“批量读写如何在同一边界下执行” | 后台任务也必须携带 tenant + time view + read purpose | 批处理不旁路 |

## 7. 共享合同、不变量与实现护栏

### 7.1 EF Core 合同

- `EF Core` 默认负责：
  - 标准写路径；
  - 聚合保存；
  - 常规事务；
  - 少量简单当前态读取。
- `EF Core` 默认不负责：
  - 大批量导出；
  - 跨模块报表；
  - 重层级树查询；
  - 需要显式时间切片优化的复杂读路径。

### 7.2 Query Filter 合同

- `Query Filter` 只允许承载平台级共享守卫，如租户边界等防御性过滤。
- `Query Filter` 不允许承载：
  - `current = today`；
  - `as_of` 推导；
  - capability / policy / package / read_only 等业务上下文；
  - 导出、审批、Assistant 等高层产品语义。
- `Query Filter` 是 defense-in-depth，不是唯一租户阻断面；入口与调用方仍需显式绑定 tenant。

### 7.3 Dapper / SQL 合同

- 以下路径默认优先允许 `Dapper/SQL`：
  - 复杂列表；
  - 报表；
  - 导出；
  - 搜索；
  - 层级树；
  - 时间切片；
  - 工作台聚合查询。
- 所有 `Dapper / Raw SQL` 都必须消费 `333` 的 `tenant-scoped SQL` 合同。
- 复杂读必须显式声明：
  - `tenant_id`
  - 时间视图
  - 查询用途
  - 是否只读

### 7.4 事务与连接合同

- 在同一请求中若混用 `EF Core` 与 `Dapper/SQL`，必须显式定义连接与事务共享策略。
- “写后立即生成确认摘要/回执”路径不得在未声明一致性边界的前提下跨连接读旧数据。
- 后台任务与批处理若使用多阶段读写，必须显式声明每阶段的一致性与回执语义。

### 7.5 时间视图执行合同

- `current` 只能作为产品层显式意图存在，进入持久化层前必须归一为显式时间锚点或显式历史视图。
- `as_of` 与 `history` 不得依赖 ORM 默认值或 repository 隐式参数补全。
- 导出、报表、Assistant 检索与审批摘要都必须消费同一条时间视图语言。

### 7.6 数据库原生能力合同

`324` 允许数据库原生能力作为正式工具进入系统，典型包括：

- 区间与排他约束；
- 递归查询；
- 窗口函数；
- 只读视图或等价读模型承载；
- 针对层级树、时间切片与批量工作台的 PostgreSQL 原生能力。

冻结原则：

- 数据库原生能力必须回答明确业务问题；
- 必须有 owning module 与共享边界说明；
- 不允许变成绕开共享时间、租户或回执合同的私有技巧。

### 7.7 stopline

- 不允许把 `current / as_of / history` 偷藏进隐式过滤器或“缺参数默认 today”。
- 不允许出现缺少 tenant 仍可执行的 Dapper / Raw SQL / 导出查询。
- 不允许把复杂报表与导出强行塞进 ORM，造成不可控查询与影子语义。
- 不允许业务模块私造第二套 repository / query helper 语言，绕开 `324` 主合同。

## 8. 作为后续子计划的业务需求输入

### 8.1 对 `340`（平台与 IAM 基座）的输入

- [ ] 平台层需要提供统一的 tenant context、时间视图归一与事务边界支撑，供所有持久化路径复用。
- [ ] 平台错误模型需要能表达：缺租户、缺时间视图、非法查询用途、SQL 边界违规等共享失败类型。

### 8.2 对 `360`（核心 HR 业务域）的输入

- [ ] `361/362/363/364` 必须显式声明各自对象的写路径、常规读路径与复杂读路径。
- [ ] effective-dated 对象的查询实现不得重新发明 `current / as_of / history` 的持久化表达。

### 8.3 对 `370/380` 的输入

- [ ] 工作流、集成、导入导出、运营报表与数据工作台都必须建立在 `tenant-scoped SQL + 显式时间视图` 合同之上。
- [ ] 批量读写与回执摘要需要共享同一条事务与连接语言，而不是各自临时决定。

### 8.4 对 `390`（Chat Assistant）的输入

- [ ] Assistant 检索、dry-run、确认摘要与动作回执不得越过 `324` 的时间与 SQL 边界。
- [ ] Assistant 若需要复杂检索，必须通过正式只读查询对象或 tenant-scoped SQL，而不是临时拼接内部读路径。

## 9. 建议目录与落点

若按 `300` 的模块化单体落地，建议采用以下 ownership：

- `src/Shared/Data/Persistence/`：共享持久化合同、时间视图类型、事务边界 abstraction
- `src/Shared/Data/EfCore/`：`DbContext` 基类、`QueryFilterPolicy`、mapping helper
- `src/Shared/Data/TenantScopedSql/`：复杂读与导出查询的 tenant-scoped SQL 合同
- `src/<Module>/Infrastructure/Persistence/`：模块具体 repository、SQL query、DB native capability 实现

其中：

- `Shared` 只拥有合同与 helper，不拥有业务查询本身；
- 具体 SQL 与 ORM mapping 的最终 ownership 仍属于各自模块。

## 10. 建议实施分期

1. [ ] `M1`：路径分工冻结  
   明确哪些场景默认走 `EF Core`，哪些场景默认走 `Dapper/SQL`。
2. [ ] `M2`：Query Filter 与时间视图护栏冻结  
   明确 Query Filter 的允许范围，并冻结禁止隐式 today 的执行规则。
3. [ ] `M3`：tenant-scoped SQL 与事务边界冻结  
   把 `333` 的 SQL 护栏与 `324` 的持久化执行边界接线。
4. [ ] `M4`：数据库原生能力准入矩阵冻结  
   为层级树、时间切片、报表与导出路径建立正式工具箱语言。
5. [ ] `M5`：首批消费域接线  
   让 `340/360/370/380/390` 正式引用 `324`，停止重写自己的 ORM / SQL 主规则。

## 11. 验收标准

- [ ] `324` 已成为 Greenfield 对 EF Core、Query Filter、Dapper/SQL 与数据库原生能力边界的单一事实源。
- [ ] `current / as_of / history` 在持久化层已形成显式执行合同，不再存在隐式 today 与影子语义。
- [ ] tenant-scoped SQL、复杂读、导出与报表都已被纳入正式执行边界，而不是旁路实现。
- [ ] 写事务、复杂读与确认摘要之间的一致性边界已清晰可解释。
- [ ] 后续子计划可以直接消费 `324`，而不再各自发明第二套 ORM / SQL 语言。

## 12. 关联文档

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md)
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)
- [DEV-PLAN-323](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/323-audit-task-session-and-snapshot-patterns-detailed-design.md)
- [DEV-PLAN-333](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/333-tenant-isolation-tenant-scoped-sql-secrets-and-assistant-safety-detailed-design.md)
