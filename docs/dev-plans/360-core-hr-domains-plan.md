# DEV-PLAN-360：核心 HR 业务域子计划（Org / JobCatalog / Staffing / Person）

**状态**: 规划中（2026-03-17 03:16 CST）

## 1. 背景与上下文

本计划承接 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 的业务蓝图，并默认依赖 [DEV-PLAN-320](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/320-shared-data-architecture-and-modeling-conventions-plan.md) 与 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 提供的共享建模与平台基座。

`360` 负责平台最核心的 HR 主数据与事务主链：

- 组织架构
- 岗位分类
- 职位
- 任职
- 人员

这是整个系统最关键的中轴。如果 `320` 的边界不清楚，后续审批、报表、AI 助手、导入导出都会陷入双写、双解读和历史口径漂移。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 定义核心 HR 模块边界：Org、JobCatalog、Staffing、Person。
- [ ] 落地“有效期 + 历史 + 当前视图”统一模型。
- [ ] 建立 Position 与 Assignment 的核心事务闭环。
- [ ] 建立 Person 与 Staffing 的清晰关联方式，避免隐式解析耦合。
- [ ] 建立字典驱动的可配置字段与查询能力。
- [ ] 提供一套用户可见的后台管理界面：列表、详情、历史、生效日期操作。

### 2.2 非目标

- [ ] 本计划不实现复杂审批流，由 `370` 承接。
- [ ] 本计划不实现完整报表中心，由 `380` 承接汇总与导出。
- [ ] 本计划不引入事件溯源，不把数据库函数作为默认业务权威。

## 3. 模块划分

### 3.1 Org Structure

- 组织树
- 子树检索与祖先/后代定位
- 组织详情
- 上下级调整
- 有效期版本
- 状态启停

### 3.2 Job Catalog

- Job Family Group
- Job Family
- Job Level
- Job Profile

### 3.3 Staffing

- Position
- Assignment / Job Data
- 编制状态
- 任职起止
- 职位与任职关系

### 3.4 Person

- 人员基础身份
- 联系信息
- 雇佣基础属性
- 与任职的关联展示

## 4. 关键设计决策

### 4.1 业务主模型（选定）

- 采用“主档 + 历史版本”模式。
- 生效日期是一级业务能力，而不是附属字段。
- 所有关键对象都支持：
  - 当前视图
  - 历史记录
  - 指定日期视图

### 4.2 模块边界（选定）

- `Org` 只负责组织。
- `JobCatalog` 只负责岗位分类主数据。
- `Staffing` 负责 Position 与 Assignment 的业务闭环。
- `Person` 负责人员主档。

### 4.3 强关联决策（选定）

- `Position` 与 `Assignment` 保持在同一业务域。
- `Person` 不承接任职写入逻辑，但承接人员身份锚点。
- 前端可以在 Person 页面组合展示任职历史，但写侧仍归 `Staffing`。

### 4.4 有效期策略（选定）

- 以日粒度为主。
- 所有生效日期变更必须显式告诉用户：
  - 是新增未来记录
  - 是修正当前记录
  - 是插入历史记录
- 对 effective-dated 对象的查询必须能表达当前、指定日期和全历史三种读取语义。

### 4.5 组织与职位分类采用可索引的层级/归属检索方案（选定）

- `Org` 的层级检索必须支持祖先、后代与子树范围查询。
- `JobCatalog` 必须支持 Group / Family 层级检索、Profile 对 Family 的归属检索，以及相应的范围查询。
- 不应把递归 CTE 作为唯一主查询路径。
- 具体采用哪种 PostgreSQL 路径类型、索引与 ORM 映射方式，下沉到 `361 / 363` 详细设计冻结。

## 5. 业务能力拆分

### 5.1 M1：Org Structure

- [ ] 组织树查询
- [ ] 子树范围查询与祖先/后代检索
- [ ] 组织详情页
- [ ] 组织新建
- [ ] 组织更名
- [ ] 调整上级
- [ ] 生效日期版本切换
- [ ] 停用/启用

### 5.2 M2：Person

- [ ] 人员列表
- [ ] 人员详情
- [ ] 人员新建
- [ ] 基础信息修改
- [ ] 人员与任职关系展示

### 5.3 M3：Job Catalog

- [ ] 组合分类体系管理（Group / Family / Level / Profile，而非单一树）
- [ ] 分类的组织上下文、`read_only` 与共享基线消费
- [ ] Family / Level / Profile CRUD 与 Family 归属调整
- [ ] `current / as_of / history` 统一读取语义
- [ ] 搜索、筛选与下游引用解释
- [ ] 可配置属性与动态规则消费（基于 `321 / 345`）

### 5.4 M4：Staffing

- [ ] Position CRUD
- [ ] Assignment CRUD
- [ ] Position 与 Assignment 关系校验
- [ ] Assignment 有效期重叠拦截
- [ ] 当前任职与历史任职展示
- [ ] 按组织子树/人员/职位维度查询

## 6. 数据建模原则

### 6.1 建议的主表形态

- `org_units`
- `org_unit_versions`
- `job_families`
- `job_family_versions`
- `job_levels`
- `job_level_versions`
- `job_profiles`
- `job_profile_versions`
- `positions`
- `position_versions`
- `assignments`
- `assignment_versions`
- `persons`

### 6.2 建模约束

- 所有核心表带 `tenant_id`
- 所有版本表带 `effective_date` 与 `end_date`
- 当前行与历史行应有清晰读取路径
- 同主体、同自然键下不得出现非法重叠的激活有效期区间
- 树形主数据必须支持可索引的层级路径检索
- 不允许把业务主字段大规模塞进 JSON

## 7. API 与 UI 交付面

### 7.1 API

- `/api/org/units`
- `/api/person/persons`
- `/api/jobcatalog/*`
- `/api/staffing/positions`
- `/api/staffing/assignments`

### 7.2 UI

- `/app/org/units`
- `/app/person/persons`
- `/app/jobcatalog`
- `/app/staffing/positions`
- `/app/staffing/assignments`

每个页面至少应提供：

- 列表
- 详情
- 历史/版本
- 新建/编辑
- 基础筛选

## 8. 与其他子计划的关系

- `320` 提供 effective date、历史、审计快照与 EF/Dapper 边界等共享建模约定。
- `321` 冻结租户可扩展能力的共享业务合同；`360` 各业务域只能消费，不得各自重做第二套动态字段/规则/Explain。
- `330` 提供敏感数据分级、导出治理与租户隔离等安全治理基线。
- `340` 提供 tenancy、auth、dictionary、audit、jobs。
- `345` 提供平台配置与策略蓝图，是 JobCatalog 等业务域的可配置化基座。
- `350` 提供列表、详情、历史与表单的统一前端交互模式。
- `360` 为 `370/380/390` 提供 workflow、workbench、assistant 所需的核心业务对象。
- `361` 负责冻结 Org Structure 的业务规则与业务蓝图，作为 `370/380/390` 的组织域输入。
- `363` 负责冻结 Job Catalog 的业务规则、组合分类体系与可配置化边界，作为 `345/364/370/380/390` 的职位分类域输入。
- `370/380/390` 不得重新定义 Org / Person / Staffing / JobCatalog 的主写模型。

## 9. 验收标准

- [ ] 组织、人员、岗位分类、职位、任职均具备最小可用 CRUD 闭环。
- [ ] 用户可以看到并操作有效期历史，而不是只看到当前状态。
- [ ] Position 与 Assignment 的关键约束在应用层有明确校验与错误反馈。
- [ ] 组织树与职位分类体系支持稳定的层级/归属范围检索能力。
- [ ] Assignment 等 effective-dated 关键对象不会出现同主体同时间段重叠激活记录。
- [ ] Person 页面与 Staffing 页面之间组合展示清晰，但不形成写侧越界。
- [ ] UI 已具备“列表 + 详情 + 历史”统一交互范式。

## 10. 后续拆分建议

1. [ ] [DEV-PLAN-361：组织架构（Org Structure）业务规则优先蓝图与详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/361-org-structure-business-rules-and-blueprint-plan.md)
2. [ ] [DEV-PLAN-362：人员主档（Person）业务规则优先蓝图与详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/362-person-business-rules-and-detailed-design.md)
3. [ ] [DEV-PLAN-363：职位分类（Job Catalog）业务规则优先蓝图与可配置化基座方案](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/363-job-catalog-business-rules-and-configurability-foundation-plan.md)
4. [ ] [DEV-PLAN-364：Staffing（Position / Assignment）业务规则优先蓝图与详细设计](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/364-staffing-position-assignment-business-rules-and-detailed-design.md)
