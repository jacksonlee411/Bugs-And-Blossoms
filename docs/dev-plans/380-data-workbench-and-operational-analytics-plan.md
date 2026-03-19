# DEV-PLAN-380：数据工作台与运营分析子计划

**状态**: 规划中（2026-03-17 07:23 CST）

## 1. 背景与上下文

本计划承接 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的“报表、导入导出、运营分析”能力，但不再把它们混在 `370` 的工作流与集成计划中。

其中，人员维度查询、报表与数据质量判断需要直接消费 [DEV-PLAN-362](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/362-person-business-rules-and-detailed-design.md) 冻结的 Person 语义，而不是把 Person 简化成任职查询附带字段。

`380` 关注的是“系统如何把业务数据变成可运营、可校验、可导出、可观察的工作台能力”：

- 导入
- 导出
- 查询工作台
- 运营分析
- 数据质量反馈

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立导入中心与批次回执能力。
- [ ] 建立导出任务与常用报表能力。
- [ ] 建立面向业务用户和运营人员的查询工作台。
- [ ] 建立基础数据质量检查与异常反馈面板。

### 2.2 非目标

- [ ] 本计划不承接核心主数据写模型。
- [ ] 本计划不承接审批流和外部系统集成边界。
- [ ] 本计划不把报表系统发展成独立 BI 平台替代品。

## 3. 范围

- Import Center
- Export Jobs
- Query Workspace
- Operational Dashboards
- Data Quality Panels

## 4. 关键设计决策

### 4.1 导入导出按“任务/批次”治理（选定）

- 所有导入都有批次。
- 所有导出都有任务。
- 不允许“用户上传后直接静默写库”。

### 4.2 查询工作台与业务 UI 分层（选定）

- 业务 UI 负责日常操作。
- Query Workspace 负责跨模块查询、汇总、筛选、导出。
- 二者不应互相取代。

### 4.3 数据质量可见化（选定）

- 导入失败、缺字典、缺映射、无效历史切片等问题必须可见。
- 不允许只在后台日志里留错误。

## 5. 功能拆分

### 5.1 M1：Import Center

- [ ] 导入模板
- [ ] 文件上传
- [ ] 校验预览
- [ ] 批次执行
- [ ] 行级错误回执

### 5.2 M2：Export & Reports

- [ ] 导出任务中心
- [ ] 常用报表
- [ ] 过滤条件模板
- [ ] 文件下载与历史记录

### 5.3 M3：Query Workspace

- [ ] 组织维度查询
- [ ] 人员维度查询
- [ ] 任职维度查询
- [ ] 跨模块筛选
- [ ] `OrgContext + current/as_of/history` 查询上下文锚点

### 5.4 M4：Operational Analytics

- [ ] 基础运营面板
- [ ] 数据质量异常面板
- [ ] 导入导出健康度

## 6. 关键模型方向

- `import_batches`
- `import_batch_rows`
- `export_jobs`
- `saved_queries`
- `report_definitions`
- `data_quality_issues`

## 7. 与其他子计划的关系

- `360` 提供核心业务对象与查询源。
- `362` 提供人员维度查询必须消费的 Person 真值：`person_uuid / pernr / display_name / status`、主档当前快照、lookup 语义与操作历史边界。
- `370` 提供审计与回执基础。
- `380` 只拥有数据工作台与运营视图，不拥有主业务写模型。
- 查询、报表、导出与 `saved_queries` 必须沿用 `340/345/347/360` 收口的 `OrgContext + Time + one security model`，不得自行引入数据集容器键或隐藏上下文别名。

## 8. `380` 对 `362` 的显式消费

- Query Workspace 的“人员维度查询”必须直接消费 `362` 的 Person 主档语义，而不是用 Assignment、负责人关联或临时 join 结果冒充 Person 主档。
- 人员维度的稳定筛选键至少包括：`person_uuid`、canonical `pernr`、`display_name`、`status`。
- 报表若同时展示 Person 与 Assignment，必须显式区分：
  - Person 侧是当前主档快照；
  - Assignment 侧是 `current / as_of / history` 的 effective-dated 事实。
- 数据质量检查至少要能发现：非法 `pernr`、canonical 工号重复、inactive Person 被错误引用、跨租户人员误读、人员主档缺失却仍被任职/负责人引用。
- 导出与查询结果中的人员字段解释权归 `362`；`380` 只能聚合、筛选、导出，不能反向定义人员状态或工号解析逻辑。

## 9. 验收标准

- [ ] 导入和导出都具备任务/批次、状态和回执。
- [ ] 管理员能通过查询工作台跨组织、人员、任职做常用查询。
- [ ] 数据质量问题有明确可见的工作台，而不是散落在日志中。
- [ ] 人员维度查询、报表与数据质量规则已经显式消费 `362` 的 Person 合同，不再把人员语义混写到任职或其他读模型里。
- [ ] 查询、报表与导出上下文已经统一表达为 `org_context + time anchor`，不存在隐藏容器键或与 UI/API 不一致的第二命中语法。

## 10. 后续拆分建议

1. [ ] `381`：Import Center 详细设计
2. [ ] `382`：Export & Reporting 详细设计
3. [ ] `383`：Query Workspace 与运营分析详细设计
