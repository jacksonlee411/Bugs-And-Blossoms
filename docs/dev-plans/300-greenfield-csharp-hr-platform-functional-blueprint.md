# DEV-PLAN-300：功能导向的 Greenfield HR 平台重做蓝图（C#/.NET + React）

**状态**: 已批准（2026-03-17 03:16 CST）

## 1. 背景与定位

本文回答一个独立问题：

> 如果完全从零开始，不继承当前仓库的 Go、DB Kernel、RLS、sqlc、Atlas+Goose、Casbin、既有门禁与历史包袱，仅根据本项目想实现的功能本身来重新设计，一套更合适的产品级技术方案应当是什么？

本文件的立场是：

- 这是一个 **企业级 HR 平台**，核心复杂度在于组织、人、岗位、任职、生效日期、审批、权限、审计、导入导出、后台管理 UI 与 AI 助手协同。
- 它不是以极致吞吐、超低延迟、超小运行时体积为第一目标的基础设施产品。
- 因此，语言与框架选择应优先服务于 **复杂业务建模、后台管理体验、企业集成、长期可维护性**，而不是服务于“最小可执行二进制”。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 形成一套从零重做时的推荐主栈与整体架构。
- [ ] 覆盖本项目核心功能：租户/认证、组织架构、职位分类、职位与任职、人员、字典/配置、审批与审计、导入导出、AI 助手、后台控制台。
- [ ] 给出比“纯 CRUD 系统”更贴近 HR 业务本质的数据与模块划分方案。
- [ ] 明确哪些能力应该优先简单实现，哪些能力应延后，避免 Greenfield 过度设计。
- [ ] 给出可落地的分阶段交付顺序，便于后续拆成实施计划。

### 2.2 非目标

- [ ] 本文不是当前仓库的迁移计划，不讨论 Go → C# 的分阶段迁移路径。
- [ ] 本文不继承当前仓库关于 DB Kernel、One Door、RLS fail-closed、sqlc、Atlas+Goose、Casbin、LibreChat 入口等既有决定。
- [ ] 本文不包含薪酬、社保、考勤、排班等超出当前项目核心范围的扩展模块。
- [ ] 本文不定义精确到字段级别的最终数据库 schema；如进入实施，需再拆分为子计划。

## 3. 结论先行

### 3.1 推荐主栈

**建议从零重做时采用：**

- 后端：`C# / ASP.NET Core`
- 前端：`React + TypeScript + MUI`
- 数据库：`PostgreSQL`
- 数据访问：`EF Core + Dapper/SQL` 混合
- 身份认证：优先对接标准 `OIDC/OAuth2` 身份提供方
- 后台任务：`Hangfire`
- 观测：`OpenTelemetry`
- AI 助手：作为独立业务能力接入，使用单独的编排层与严格动作边界

### 3.2 一句话判断

如果只看功能本身，这个项目更像“复杂业务企业平台”，而不是“偏基础设施的后端服务”。  
对这种问题，**C#/.NET 比 Go 更适合作为主后端栈**。

## 4. 推荐方案总览

### 4.1 架构总图

```mermaid
flowchart LR
  U[HR 管理员 / 租户管理员 / 业务用户] --> W[React SPA + MUI]
  W --> API[ASP.NET Core API / BFF]
  API --> IAM[IAM / Tenant]
  API --> ORG[Org Structure]
  API --> JOB[Job Catalog]
  API --> STF[Staffing]
  API --> PER[Person]
  API --> CFG[Dictionary / Config]
  API --> WF[Workflow / Approval / Tasks]
  API --> AST[Assistant Orchestrator]
  IAM --> DB[(PostgreSQL)]
  ORG --> DB
  JOB --> DB
  STF --> DB
  PER --> DB
  CFG --> DB
  WF --> DB
  API --> JOBS[Hangfire Jobs]
  API --> FILES[Object Storage]
  AST --> LLM[LLM Gateway]
  API --> OBS[OpenTelemetry / Logs / Metrics]
```

### 4.2 推荐形态

**首选：模块化单体（Modular Monolith）**

理由：

- 这些模块之间存在天然强耦合：组织、职位、任职、人员、审批、权限、审计彼此共享上下文。
- 业务复杂度本来就高，若一开始拆微服务，会把“业务复杂度”再乘上“分布式复杂度”。
- 企业后台系统的主要价值通常来自一致性、可维护性和可变更性，而不是服务数量。

**明确不建议一开始就做：**

- 微服务
- 事件溯源作为默认主模型
- 全功能工作流引擎大一统接管全部业务
- 过早引入 ES/Kafka/Redis Cluster 等重型基础设施

## 5. 关键设计决策（ADR 摘要）

### 5.1 选 C#/.NET，而不是 Go（选定）

#### 原因

- 业务是“企业后台 + 复杂规则 + 大量管理页面 + 长生命周期维护”。
- `ASP.NET Core` 在 DI、中间件、配置、验证、后台任务、日志、集成测试、企业认证集成上的整体体验更适合这类产品。
- C# 对复杂领域对象、DTO、验证规则、权限策略、异步编排、后台作业的表达更自然。
- 如果后续 AI 助手继续增强，C# 在企业 AI SDK、编排生态上的选择面更大。

#### 结论

- **后端主栈选 C#/.NET。**
- Go 可作为性能敏感的边缘服务或基础设施工具语言，但不适合作为这个产品的默认主栈。

### 5.2 选 React + MUI，而不是 Blazor（选定）

#### 原因

- 本项目是典型管理台，React 生态在组件、表格、表单、权限壳、数据获取、测试和人才市场上更成熟。
- MUI 对 Data Grid、Tree、Date Picker、复杂表单有天然优势。
- 前后端职责更清晰，未来若需要开放 BFF/API 给其他入口，也更灵活。

#### 结论

- **前端保持 React SPA。**
- 不建议采用 Blazor 作为主前端方案。

### 5.3 选“关系型主模型 + 生效日期 + 审计日志”，而不是默认事件溯源（选定）

#### 原因

- HR 系统的本质是：**有效期、版本、审批、审计**，不一定非要上事件溯源。
- 多数核心需求都可以通过：
  - 当前实体表
  - 生效日期版本表
  - 统一审计日志
  - 业务动作回执
  
  来稳定实现。
- 事件溯源会显著增加查询、调试、补偿、回放、开发认知负担。

#### 结论

- **默认使用常规关系模型。**
- 对需要保留历史的对象使用 effective-dated version tables。
- 对关键变更保留 append-only audit log。
- 不把事件溯源作为系统总原则。

### 5.4 选 EF Core + Dapper/SQL 混合，而不是“纯 ORM”或“纯手写 SQL”（选定）

#### 原因

- 纯 EF Core 很适合标准写入流程、聚合保存、常规事务、对象生命周期管理。
- HR 系统又天然包含大量复杂查询：树、批量搜索、时间切片、报表、导出、候选匹配。
- 这些场景用 Dapper 或手写 SQL 更清晰、更可控。

#### 结论

- **写模型以 EF Core 为主。**
- **复杂读模型、报表、导出、检索以 Dapper/SQL 为主。**
- 不建议“纯 EF Core 统治一切”。

### 5.5 选“应用层租户隔离 + 权限策略”，而不是一开始就把全部隔离压到数据库（选定）

#### 原因

- 从零交付产品时，最先需要的是功能正确、开发效率高、边界清楚。
- 应用层 tenant scope、统一 repository/query filter、严格 API 策略，已经足够支撑大多数 SaaS HR 平台第一阶段。
- 数据库级 RLS 是高级强化手段，不应成为第一天的交付阻塞点。

#### 结论

- Phase 1 采用 **应用层强租户隔离**。
- 若后续合规、安全、运维成熟度提出更高要求，再评估引入数据库级 RLS。

### 5.6 选“AI 助手作为受控业务能力”，而不是“模型直连写业务”（选定）

#### 原因

- AI 助手适合做理解、检索、建议、草稿、解释，不适合作为事务写入主权威。
- 人事系统涉及组织与任职，错误写入代价高，必须保留确定性边界。

#### 结论

- Assistant 只能：
  - 做语义理解
  - 发起只读检索
  - 生成草稿/建议
  - 触发受控的业务 action request
- Assistant 不能：
  - 直接写数据库
  - 直接跳过确认
  - 直接绕过审批与权限

## 6. 功能模块划分（从产品视角）

### 6.1 平台模块

#### 1. IAM / Tenant

- 多租户
- 登录与会话
- 用户、角色、权限
- SSO / OIDC 接入
- Superadmin 控制台

#### 2. Dictionary / Configuration

- 通用字典
- 下拉选项
- 业务配置
- 可租户级覆盖的系统设置

#### 3. Audit / Task / Notification

- 操作审计
- 系统任务
- 通知（邮件、站内、Webhook）
- 后台作业与失败重试

### 6.2 核心 HR 模块

#### 4. Org Structure

- 组织单元树
- 生效日期版本
- 上下级调整
- 组织详情、状态、历史

#### 5. Job Catalog

- Job Family / Job Level / Job Profile
- 岗位分类主数据
- 分类结构与标签

#### 6. Staffing

- Position
- Assignment / Job Data
- 职位编制、任职关系、状态变更

#### 7. Person

- 人员主档
- 基础身份信息
- 与任职记录联动

### 6.3 支撑模块

#### 8. Workflow / Approval

- 提交、审批、驳回、撤回
- 关键主数据和任职变更审批
- 待办与审批历史

#### 9. Import / Export / Documents

- Excel/CSV 导入
- 模板下载
- 批量导出
- 业务附件

#### 10. Reporting / Query Workspace

- 组织视图
- 人员与任职报表
- 常用筛选、导出、汇总

#### 11. Assistant

- 对话理解
- 只读检索
- 生成建议
- 事务前草稿与确认

#### 12. Integration Hub

- HR 外部系统对接
- 批量同步任务
- Webhook / SFTP / API connectors

## 7. 推荐领域边界

### 7.1 建议的后端模块结构

```text
src/
  Platform/
    IAM/
    Tenancy/
    Configuration/
    Audit/
    Notifications/
    Assistant/
  HR/
    OrgStructure/
    JobCatalog/
    Staffing/
    Person/
    Workflow/
    Reporting/
    Integrations/
```

### 7.2 每个模块内部建议分层

```text
<Module>/
  Domain/
  Application/
  Infrastructure/
  Web/
```

说明：

- `Domain`：业务规则、实体、值对象、领域服务
- `Application`：用例编排、事务边界、命令/查询处理
- `Infrastructure`：EF Core、Dapper、外部系统、对象存储、消息与任务
- `Web`：API、DTO、验证、控制器/Endpoint

## 8. 数据建模策略

### 8.1 生效日期作为第一类能力

该产品天然适合使用 **day-granularity effective dating**：

- `effective_date`
- `end_date`
- `status`
- `is_current`

适用对象：

- 组织单元
- 职位分类
- 职位
- 任职
- 字典值
- 关键配置

### 8.2 推荐的持久化模式

#### 模式 A：当前实体 + 历史版本

适用于：

- 组织
- 职位分类
- 职位
- 任职

做法：

- 一张主表存稳定标识与少量当前状态
- 一张 versions/history 表存按 effective date 切片的数据

#### 模式 B：主档 + 附属快照

适用于：

- Person
- 配置项
- 审批记录

做法：

- 主档存当前事实
- 审批或操作时保留 before/after snapshot

#### 模式 C：统一审计表

建议提供统一 `audit_log`：

- actor
- tenant
- module
- action
- target_type
- target_id
- before_snapshot
- after_snapshot
- request_id
- timestamp

### 8.3 不建议的默认做法

- 不建议所有模块都采用事件溯源
- 不建议所有表都依赖软删除
- 不建议把审计日志和业务主表混写成一套抽象
- 不建议为了“灵活”把大量核心字段塞进 JSONB

## 9. 多租户、权限与认证

### 9.1 多租户

建议默认采用：

- 单库多租户
- 全业务表带 `tenant_id`
- 应用层统一 tenant context 注入
- 查询层统一 tenant filter

对于高隔离客户，可保留后续扩展路线：

- 单租户部署
- 单独数据库

### 9.2 认证

建议优先支持标准 OIDC/OAuth2：

- 企业客户：Entra ID / Okta / Auth0 / Keycloak
- 产品内部也可保留本地用户名密码作为 fallback

### 9.3 授权

建议采用：

- `role + permission` 模型
- ASP.NET Core policy-based authorization
- 数据库存储权限矩阵

而不是一开始就引入非常重的外部授权 DSL。

### 9.4 审批与权限关系

必须明确：

- “有权限发起” 不等于 “有权限审批”
- “有权限编辑” 不等于 “能修改历史生效记录”
- “Assistant 能建议” 不等于 “Assistant 能提交”

## 10. 后端技术选型建议

### 10.1 Web 与应用宿主

- `ASP.NET Core`
- `Minimal APIs` 或 `Controllers` 均可
- 对复杂后台系统，建议：
  - 面向内部业务 API 使用 Controllers 或清晰的 Endpoint 分组
  - 避免纯“everything minimal API”导致边界松散

### 10.2 数据访问

- `EF Core`：事务写入、聚合保存、常规查询
- `Dapper`：复杂列表、报表、搜索、导出
- `Npgsql`：PostgreSQL 驱动

### 10.3 验证与命令处理

- `FluentValidation`
- 可选 `MediatR`，但不要为了 CQRS 形式主义而到处套 Handler

建议：

- 用“命令/查询对象”组织复杂应用逻辑
- 但不要把每个 GET/POST 都机械切成十几层

### 10.4 后台任务

- `Hangfire`

适用场景：

- 导入导出
- 审批后续动作
- 通知发送
- 外部同步
- Assistant 长任务回执

### 10.5 文档与契约

- OpenAPI / Swagger
- API DTO 与前端类型可自动生成或半自动对齐

### 10.6 文件与导出

- 对象存储：S3 兼容
- Excel：`ClosedXML`
- PDF：按需要引入成熟库，不要过早内建复杂报表引擎

## 11. 前端技术选型建议

### 11.1 主栈

- `React + TypeScript`
- `MUI`
- `React Query`
- `React Hook Form`
- `Zod`

### 11.2 页面模式

建议统一以下管理台模式：

- 列表页
- 详情页
- 历史版本页
- 审批页
- 导入导出页
- 审计页

### 11.3 UI 交互原则

- 组织、职位、任职、人员页面统一采用“列表 + 详情 + 时间线/版本”
- 生效日期必须是界面上的一级概念，不要藏在高级选项里
- 审批与确认应显式分离
- Assistant 的结果必须回落到明确的业务 UI 或确认面板

## 12. Assistant 设计边界

### 12.1 Assistant 应该做什么

- 理解用户意图
- 检索候选组织、人员、职位、任职
- 生成“建议动作”
- 生成“确认摘要”
- 解释为什么不能执行

### 12.2 Assistant 不应该做什么

- 不直接改业务表
- 不跳过审批
- 不绕过权限
- 不伪造标识符和业务主键
- 不自己维护第二套业务状态机

### 12.3 推荐落地方式

- Assistant 单独有一层 orchestration service
- 通过只读接口访问 HR 领域
- 对可写操作，只能生成 `action request`
- 最终由业务应用层执行 dry-run / validate / confirm / commit

### 12.4 为什么这很重要

这个系统的数据不是聊天玩具数据，而是组织与人事数据。  
AI 提升的是“交互效率”和“理解能力”，不是“替代业务边界”。

## 13. 推荐实施顺序

### Phase 0：平台与基础

- [ ] IAM / Tenant
- [ ] 角色与权限
- [ ] 字典与基础配置
- [ ] 审计与通知基座
- [ ] React 壳、导航、表单规范、列表规范

### Phase 1：组织与人员基础闭环

- [ ] Org Structure
- [ ] Person
- [ ] 有效期记录展示与编辑
- [ ] 审计日志可追溯

### Phase 2：岗位与任职闭环

- [ ] Job Catalog
- [ ] Position
- [ ] Assignment / Job Data
- [ ] 批量导入导出

### Phase 3：审批、报表、集成

- [ ] Workflow / Approval
- [ ] 常用报表
- [ ] 通知
- [ ] 外部系统同步接口

### Phase 4：Assistant

- [ ] 对话式只读检索
- [ ] 建议式写入草稿
- [ ] 受控确认与执行

### Phase 5：强化与产品化

- [ ] SSO 扩展
- [ ] 高级权限
- [ ] 多租户运营能力
- [ ] 性能与观测强化

## 14. 为什么这个方案比“Go + 强 DB 内核”更适合从零开始

### 14.1 更贴近产品团队的真实重心

从零重做时，团队首先要解决的是：

- 业务规则是否清晰
- 页面能否快速交付
- 后台任务是否稳定
- 审批、权限、审计是否好维护
- 企业客户接入是否顺手

这些点上，C#/.NET 的整体工程体验更贴近问题本身。

### 14.2 更容易把复杂度留在正确的位置

推荐方案下：

- 业务复杂度留在领域与应用层
- SQL 复杂度留在真正复杂的查询场景
- 历史与审计通过标准关系模型表达
- AI 能力留在编排层

而不是把大量核心复杂度一开始就推入数据库内核。

### 14.3 更利于企业级长期演进

这类系统通常会长期演化出：

- 更复杂的审批
- 更多报表
- 更多权限矩阵
- 更多外部集成
- 更多后台工具
- 更多运营能力

主栈若采用 C#/.NET，长期扩展通常比 Go 更从容。

## 15. 风险与取舍

### 15.1 采用 C# 的主要风险

- 运行时与部署体积通常大于 Go
- 团队若不熟悉 .NET，会有学习成本
- EF Core 若被滥用，会造成查询不可控与性能问题

### 15.2 对应缓解

- 架构上坚持模块化单体，避免分布式爆炸
- 数据访问坚持 EF Core + Dapper 混合
- 对复杂查询与导出场景明确保留 SQL 权限
- 不把 AI 助手直接接到写模型

## 16. 验收标准

- [ ] 架构层面已明确：后端采用 C#/.NET，前端采用 React + MUI。
- [ ] 已明确：系统以模块化单体交付，而不是微服务优先。
- [ ] 已明确：采用关系型主模型 + effective-dated history + audit log，而非默认事件溯源。
- [ ] 已明确：数据访问采用 EF Core + Dapper/SQL 混合，而非纯 ORM。
- [ ] 已明确：Assistant 只能作为受控编排层，不得成为业务写入主边界。
- [ ] 已形成后续可拆分的实施顺序：平台 → 组织/人员 → 岗位/任职 → 审批/报表/集成 → Assistant。

## 17. 后续拆分建议

若进入实施，可继续拆分为以下子计划：

1. [ ] `300A`：平台与 IAM 详细设计
2. [ ] `300B`：组织架构与有效期模型设计
3. [ ] `300C`：Job Catalog / Position / Assignment 详细设计
4. [ ] `300D`：审批、审计与通知设计
5. [ ] `300E`：导入导出与报表设计
6. [ ] `300F`：Assistant 编排边界与动作网关设计
