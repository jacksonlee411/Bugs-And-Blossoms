# DEV-PLAN-362：人员主档（Person）业务规则优先蓝图与详细设计

**状态**: 规划中（2026-03-18 15:00 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md) 的 `M2: Person` 子计划，同时承接：

- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 关于“业务规则优先”的方法论；
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md) 关于 Pattern B、`current / as_of / history` 与共享时间语言的合同；
- [DEV-PLAN-016](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/016-greenfield-hr-modules-skeleton.md) 关于 Person 模块边界与 UI 组合方式的约束；
- [DEV-PLAN-027](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/027-person-minimal-identity-for-staffing.md) 关于 `pernr -> person_uuid` 最小身份锚点合同；
- [DEV-PLAN-063](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/063-test-tp060-03-person-and-assignments.md) 与 [DEV-PLAN-061](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md) 关于 Person/Assignments 和跨租户隔离的执行证据；
- [DEV-PLAN-351](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/351-product-shell-and-route-information-architecture-detailed-design.md)、[DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md)、[DEV-PLAN-353](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md) 关于路由、页面模式和表单交互的统一产品语言。

`300` 对 Person 域给出的提醒，不是“先设计 persons 表或 lookup API”，而是：

- 先定义企业真正维护的“人”是什么业务对象；
- 先定义人员主档到底拥有哪些真值、哪些只是下游组合展示；
- 再决定内部如何映射到 schema、API、页面和上游下游引用。

当前仓库中，Person 规则已经分散沉淀在：

- `027`：最小身份锚点、`pernr` 规范化与精确解析；
- `016`：Person 与 Staffing 的跨模块组合边界；
- `061/063`：RLS、跨租户隔离、Person + Assignments 的 E2E 证据；
- `069`：删除 payroll/attendance 后 Person 合同的减法收敛；
- 当前代码中的 `person.persons` schema、RLS、list/create/options/by-pernr API 与最小 MUI 页面。

但仓库里仍缺一份以**业务语言**表达的 Person SSOT。  
`362` 的任务就是把这些既有事实收敛成 `360` 的正式子计划，同时把“当前已落地的最小 identity anchor”和“目标 Person 主档蓝图”明确区分开。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 用“业务规则优先”的语言重述 Person 域，不再让 `person.persons`、`normalize_pernr` 和 lookup API 成为主叙事。
- [ ] 总结当前仓库中已经稳定沉淀的 Person 业务规则，并明确区分“现行规则”与“已冻结目标规则”。
- [ ] 冻结 Person 域的目标业务蓝图：对象、场景、边界、不变量、页面模式、操作历史和跨模块引用规则。
- [ ] 明确 Person 如何作为 Org / Staffing 的上游真值提供身份、状态与选择能力，而不越界承接任职写入。
- [ ] 作为 `360` 的详细拆分，为 `351/352/353/370/380/390` 提供可直接消费的 Person 业务输入。

### 2.2 非目标

- [ ] 本计划不把 Person 改写成 Pattern A 的 effective-dated 主对象；Person 核心主档默认遵循 Pattern B。
- [ ] 本计划不把 Assignment / Position 写入逻辑迁回 Person 域；任职主写模型继续由 `364`/Staffing 拥有。
- [ ] 本计划不直接定义最终 DDL、迁移脚本、handler 代码或 UI 细节；这些仍由实现与后续实施计划承接。
- [ ] 本计划不重新引入 payroll、attendance、external identity link 等已被移除的历史负担。
- [ ] 本计划不把 Org / JobCatalog 的主数据真值拉进 Person 域内部。

## 3. “业务规则优先”在 Person 域中的翻译

### 3.1 用户维护的是“人员主档”，不是“工号解析器”

Person 域里，用户真正维护的不是：

- `person.persons`
- `persons:by-pernr`
- `persons:options`
- `normalize_pernr`

他们真正维护的是：

- 某个人是否已经在当前租户被建档；
- 这个人的稳定身份是什么、当前名称是什么、是否处于 active；
- 这个人当前的联系信息和雇佣基础属性是什么；
- 这个人当前有哪些任职，但这些任职到底由谁维护。

因此，`362` 冻结以下表达顺序：

1. 先定义业务对象；
2. 再定义业务规则、状态语义和跨模块边界；
3. 最后才定义内部如何映射到 schema、API 和页面。

### 3.2 Person 采用 Pattern B：当前快照 + 操作历史，而不是逐日时间线

依 [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)，Person 核心主档默认属于 Pattern B：

- Person 的核心问题是“当前这个人是谁、现在是什么状态、最近被谁改过”，而不是“按业务日插入/更正一条主档版本线”；
- `current` 对应当前人员快照；
- `history` 对应主档操作历史与审计证据；
- 若页面携带 `as_of`，它主要用于组合下游 effective-dated 事实，例如某人的任职时间线，而不是让 Person 主档自己伪装成日粒度版本对象。

因此，`362` 选定：

- Person 核心主档不声明 `add / insert / correct / close / rescind` 这套 Pattern A 写意图；
- 若未来 Person 域中某个子对象真的需要 effective-dated 语义，必须另行显式声明为独立子对象，而不是把整个 Person 域强推成 Pattern A。

### 3.3 Person 是身份与主档上游真值，不是任职域

- Person 拥有身份锚点、当前主档快照、active/inactive 生命周期与 lookup 真值；
- Staffing 拥有 Assignment / Position 主写模型；
- Org 可以引用 Person 作为负责人身份来源，但不拥有 Person 真值；
- Person 页面可以组合展示任职历史，但写侧仍归 Staffing。

## 4. Person 业务蓝图（目标形态）

### 4.1 领域使命

Person 域是平台内“**人员稳定身份、人员当前主档快照、人员 active/inactive 生命周期、人员选择/解析能力，以及与任职/组织的只读组合展示**”的唯一业务权威。  
它既服务于 HR 管理者维护人员主档，也服务于 Org、Staffing、Reporting 与 Assistant 在同一租户内稳定引用“这个人到底是谁”。

### 4.2 核心业务对象

| 业务对象 | 业务含义 | `362` 是否视为 Person 拥有 |
| --- | --- | --- |
| `Person` | 稳定人员身份锚点 | 是 |
| `PersonIdentity` | `person_uuid / pernr / display_name` 等身份核心 | 是 |
| `PersonProfileSnapshot` | 当前主档快照 | 是 |
| `PersonLifecycle` | `active / inactive` 等主档可用性状态 | 是 |
| `PersonContactInfo` | 联系方式与通知触达信息 | 是，作为目标 Person 主档组成部分 |
| `PersonEmploymentBasics` | 雇佣基础属性，例如雇佣类型/用工状态等 | 是，作为目标 Person 主档组成部分 |
| `PersonSelectionOption` | 供表单选择器与 lookup 使用的最小投影 | 是 |
| `PersonOperationHistory` | 谁在什么时间改过此人主档 | 是 |
| `AssignmentSummary` | 某人的任职摘要/时间线 | 否，由 `364`/Staffing 拥有，Person 只组合展示 |
| `ManagerRef` | 被 Org 引用的负责人身份 | 否，Org 只是引用 Person 真值 |

### 4.3 面向用户的主能力

- 创建人员主档；
- 通过 `pernr` 或姓名搜索、联想和精确定位人员；
- 查看某个人当前主档快照；
- 修改人员基础信息、联系信息和雇佣基础属性；
- 激活/停用某个人；
- 在 Person 详情页只读查看其任职摘要/任职历史；
- 解释为什么某个人不能作为负责人、为什么 lookup 命中失败、为什么当前页面只读。

### 4.4 页面与交付语言

按照 `351 + 352 + 353` 的页面语言，Person 最终应统一呈现为：

- 列表页：回答“当前租户有哪些人”
- 详情页：回答“这个人的当前主档是什么样”
- 历史页：回答“这个人最近被谁改过什么”
- 关联区：回答“这个人当前/某日有哪些任职”，但只读组合，不越权写入

对应页面模式选择：

- 列表：`L1` 平面列表；
- 详情：`D1` 单快照详情；
- 历史：`H2` 审计/操作历史。

## 5. 当前基线：已沉淀的业务规则

### 5.1 现行且已落地的规则

#### 5.1.1 Person 最小身份锚点已经成立

- `person_uuid` 是跨模块稳定身份锚点；
- `pernr` 是用户输入与检索的主要自然键；
- `pernr` 必须是 1-8 位数字字符串；
- 前导 0 同值，`00000103` 与 `103` 视为同一工号；
- canonical `pernr` 是跨模块传递与落库存储的唯一权威表达。

#### 5.1.2 Person 当前主档快照已经落地为单表模式

- 当前已有 `person.persons` 作为 Person 主档事实源；
- 现行稳定字段至少包括：`person_uuid / pernr / display_name / status / created_at / updated_at`；
- `status` 当前受限于 `active / inactive`；
- 创建 Person 默认进入 `active`；
- 当前没有 Person 自己的 effective-dated 版本表，也没有把主档伪装成日粒度历史模型。

#### 5.1.3 Person 已纳入强租户隔离与 fail-closed 访问合同

- `person.persons` 已启用 `ENABLE/FORCE ROW LEVEL SECURITY`；
- 读写路径都必须先注入 `app.current_tenant`，遵守 “No Tx, No RLS”；
- 跨租户读取 `persons:by-pernr` 时，当前稳定结果是 fail-closed 到 `404 PERSON_NOT_FOUND`，不泄漏他租户数据；
- Person 已经不是“暂时不做 RLS 的例外模块”。

#### 5.1.4 最小可复用 API 合同已经成立

- `GET /person/api/persons`：可列出当前租户 Person 列表；
- `POST /person/api/persons`：可创建 Person；
- `GET /person/api/persons:options`：提供选择器/联想最小投影；
- `GET /person/api/persons:by-pernr`：提供精确解析 `pernr -> person_uuid`；
- 稳定错误码已至少包括：`PERSON_PERNR_INVALID`、`PERSON_NOT_FOUND`、`PERSON_INTERNAL`、`PERSON_CREATE_FAILED`。

#### 5.1.5 当前 UI 与权限闭环已经存在，但仍是最小形态

- 当前租户 App 中已经存在 `/app/person/persons` 入口；
- 当前页面已具备最小的“列表 + 创建”闭环；
- 权限对象已经收敛为 `person.persons`，至少存在 `read / admin` 两档；
- 目前尚未形成真正独立的 Person 详情页、历史页与分区化主档页，这仍属于 `362` 需要正式冻结的目标形态。

#### 5.1.6 Person 已是其他业务域的上游事实源

- Staffing 已通过 `persons:by-pernr/options` 消费 Person 身份能力，并以 `person_uuid` 作为写侧权威输入；
- Org 当前已使用 Person 作为 `manager_pernr -> manager_uuid` 的解析真值，并明确要求负责人必须是 active Person；
- Person 已不是孤立模块，而是其他业务域可以直接消费的上游身份真值。

#### 5.1.7 Person 当前合同已经做过减法收敛

- payroll / attendance 相关 Person 负担已被移除；
- `external_identity_links` 这类考勤/外部映射负担不再属于当前 Person 现行合同；
- Person 当前基线明确保持精简，不再承接与“人员主档当前快照”无关的历史遗留负担。

### 5.2 当前已冻结、需由 `362` 正式承接的目标规则

以下结论已经在现有计划与实现中形成共识，但仍需要 `362` 作为正式业务 SSOT 收口：

- Person 应被描述为“人员主档 + 当前快照 + 操作历史 + 选择/解析能力”的业务域，而不是单纯的 staffing 前置 lookup 服务。
- `pernr -> person_uuid` 是 Person 对外的重要能力，但不是 Person 的全部业务含义。
- Contact Info 与 Employment Basics 应回归 Person 主档拥有，但仍保持 Pattern B 当前快照语义，而不是直接把整个域改造成 effective-dated 主档。
- Person 页面应从“最小 list/create”演进为“列表 + 详情 + 历史 + 任职只读组合”的正式对象页。
- Person 的 `active / inactive` 状态应成为可被 Org、Reporting、Assistant 复用的稳定业务信号，而不是仅存在于 schema 的内部枚举。

## 6. `362` 冻结的目标业务规则矩阵

| 场景 | 用户真正要做什么 | 核心业务规则 | 业务结果 |
| --- | --- | --- | --- |
| 新建人员 | 在租户内建立一个新的人员主档 | `pernr` 必须规范化且租户内唯一；`display_name` 必填；创建后形成稳定 `person_uuid` | Person 身份建立并可被下游引用 |
| 搜索人员 | 通过工号或姓名快速定位某人 | `persons:options` 用于联想；`persons:by-pernr` 用于精确解析；联想不等于精确真值 | 用户进入正确的人员上下文 |
| 查看当前主档 | 看清这个人现在是谁、状态如何 | Person 详情以当前快照为主；不得伪装成 effective-dated 版本页 | 当前主档可解释、可引用 |
| 修改基础信息 | 调整姓名、联系方式、雇佣基础属性等 | 字段可编辑性由服务端决议；更新改变当前快照并留痕 | 主档当前值被受控更新 |
| 激活/停用人员 | 改变某个人是否可被继续引用 | 状态是主档生命周期，不等于删除；inactive Person 的下游可用性必须有明确解释 | Person 在下游选择中的资格发生变化 |
| 精确解析 pernr | 把自然键稳定映射到身份锚点 | `00000103` 与 `103` 命中同一人；非法工号必须 fail-closed；跨租户不可泄漏 | 下游拿到唯一 `person_uuid` |
| 在 Person 页看任职 | 查看这个人有哪些 assignment | Person 只读组合 Staffing 事实；不拥有任职写入；时间视图要显式 | 用户能从人出发看任职，但边界不漂移 |
| 查看历史 | 理解这个人最近被谁改过什么 | Person 的主历史以操作/审计历史为主，不冒充有效期版本史 | 主档变更证据可追溯 |
| 理解不可操作原因 | 明白为什么不能编辑/不能引用/不能访问 | 区分对象只读、动作禁用、直接无权访问；错误反馈必须可解释 | 用户知道下一步该如何处理 |

## 7. Person 域边界与跨模块约束

### 7.1 Person 拥有的内容

- 人员稳定身份；
- `pernr` 规范化、唯一性与精确解析真值；
- 当前主档快照；
- 当前 active/inactive 生命周期；
- 人员选择/联想/精确解析投影；
- 人员主档操作历史与审计解释。

### 7.2 Person 不拥有的内容

- Assignment / Position 的主写模型：由 `364`/Staffing 拥有；
- 组织层级、组织负责人业务规则：由 `361`/Org 拥有；
- Job Catalog 分类事实：由 `363`/JobCatalog 拥有；
- 审批请求与审批生命周期：由 `370` 拥有；
- 报表工作台、导出任务与运营统计：由 `380` 拥有；
- Assistant 的对话理解、澄清与动作编排：由 `390` 拥有。

### 7.3 跨模块调用原则

- Staffing 只能把 Person 当作身份与选择真值来源，不得自己解析 `pernr` 作为写侧真值。
- Org 可以引用 active Person 作为负责人来源，但不拥有 Person 生命周期规则。
- Person 页面可以组合展示 AssignmentSummary，但不得把任职写入口偷偷搬进 Person 域。
- Workflow、Reporting、Assistant 只能消费 Person 公开事实与受控动作，不得重写 Person 主档语义。

## 8. 作为其他子计划的业务需求输入

### 8.1 对 `351 / 352 / 353` 的输入

- [ ] Person 业务页必须挂在稳定 tenant app 路由组下，标准入口为 `/app/person/persons`。
- [ ] Person 必须采用 `L1 + D1 + H2` 页面模式，而不是照搬 effective-dated 对象的双栏版本页。
- [ ] Person 表单必须消费服务端字段决议，不得自行猜测哪些字段可改。
- [ ] Person 核心表单默认不是 effective-dated 表单；若页面出现 `as_of`，必须解释它作用于组合下游事实，而不是作用于 Person 主档版本。
- [ ] Person 页必须区分“对象只读”“动作禁用”“直接无权访问”，并对 inactive / not found / cross-tenant fail-closed 给出明确反馈。

### 8.2 对 `370`（Workflow / Audit / Integration）的输入

- [ ] Workflow 至少要区分：新建人员、主档更新、人员状态变更三类 Person 请求。
- [ ] 审批/审计摘要至少应包含：`person_uuid`、`pernr`、`display_name`、当前状态、被修改字段、是否影响下游引用资格。
- [ ] Person 历史页以操作历史和 before/after 证据为主，不应被 Workflow 反向改写成伪有效期历史。
- [ ] 外部集成消费 Person 时，必须显式声明读取的是“当前快照”还是“操作历史”。

### 8.3 对 `380`（Data Workbench / Reporting）的输入

- [ ] 查询工作台必须支持按 `pernr / display_name / status` 检索 Person。
- [ ] 与 Staffing 组合报表时，必须显式声明任职侧的时间视图，不得把 Person 当前快照和 Assignment 历史混成一套模糊时间语义。
- [ ] 数据质量检查至少要能发现：非法 pernr、重复 canonical pernr、inactive Person 被错误引用、跨租户误读等问题。
- [ ] Person 导出至少要支持：`person_uuid / pernr / display_name / status / created_at / updated_at`，以及受控扩展后的主档字段。

### 8.4 对 `390`（Assistant）的输入

- [ ] Assistant 至少需要稳定支持：搜索人员、精确解析工号、创建人员、查看人员详情、解释人员状态与引用资格。
- [ ] Assistant 的确认摘要必须显式包含：目标人员、目标工号、当前状态、要修改的字段、是否影响负责人/任职等下游引用。
- [ ] 当用户输入带前导 0 的工号时，Assistant 必须沿用 Person 的 canonical 规则，不得各自发明解析逻辑。
- [ ] 当存在无权限、未命中、inactive 等情况时，Assistant 必须复用 Person 的共享边界，不得绕过或美化失败。

## 9. 与现有计划的关系

- `300` 提供“业务规则优先”和核心 HR 四域拆分的上层思想。
- `322` 提供 Person 必须采用的 Pattern B 与共享时间语言。
- `351/352/353` 提供壳层、页面模式和表单交互语言；`362` 只定义 Person 必须呈现什么。
- `360` 拥有核心 HR 业务域总拆分；`362` 是其中 `M2: Person` 的正式详细设计。
- `361` 提供 Org 对负责人引用和组织边界的上游约束。
- `364` 提供 Staffing 对 `person_uuid` 写侧锚点和任职边界的下游约束。
- `370/380/390` 只能消费 `362` 冻结的 Person 语义，不能重新定义人员主档。

## 10. 实施步骤

1. [ ] 冻结 Person 域业务词汇表：`Person / PersonIdentity / PersonProfileSnapshot / PersonLifecycle / PersonContactInfo / PersonEmploymentBasics / PersonSelectionOption / PersonOperationHistory / AssignmentSummary`。
2. [ ] 将 `016/027/061/063/069` 与当前代码中的稳定事实映射到本计划，形成“现行规则 / 目标规则”双视图。
3. [ ] 以本计划为依据，收敛 Person 页面的 IA：从最小 list/create 样板推进到正式的列表/详情/历史/任职组合结构。
4. [ ] 以本计划为输入，推动 Org / Staffing / Reporting / Assistant 显式消费 Person 的身份、状态和 lookup 语义，而不是各自实现一套解析逻辑。
5. [ ] 对 Contact Info、Employment Basics、主档操作历史等扩展能力，在不破坏 Pattern B 的前提下拆分后续子计划或实施批次。

## 11. 验收标准

- [ ] Person 域的业务对象、业务场景、边界与不变量已经可以脱离当前实现细节独立理解。
- [ ] `pernr -> person_uuid`、当前主档快照、active/inactive 生命周期与 lookup 语义已经被明确冻结为 Person 域真值。
- [ ] 当前最小身份锚点与目标 Person 主档蓝图被明确区分，不再混杂为“Person 只等于 lookup 服务”。
- [ ] `362` 与 `300/322/351/352/353/360/364` 口径一致，不引入新的时间语义漂移、边界越权或第二事实源。
- [ ] `351/352/353/370/380/390` 已可直接引用本计划作为 Person 域输入，而不是各自重写人员主档语义。

## 12. 关联文档

- [DEV-PLAN-016](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/016-greenfield-hr-modules-skeleton.md)
- [DEV-PLAN-021](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md)
- [DEV-PLAN-022](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/022-authz-casbin-toolchain.md)
- [DEV-PLAN-027](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/027-person-minimal-identity-for-staffing.md)
- [DEV-PLAN-061](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md)
- [DEV-PLAN-063](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/063-test-tp060-03-person-and-assignments.md)
- [DEV-PLAN-069](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/069-remove-payroll-attendance.md)
- [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md)
- [DEV-PLAN-322](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/322-effective-date-history-and-interval-integrity-detailed-design.md)
- [DEV-PLAN-351](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/351-product-shell-and-route-information-architecture-detailed-design.md)
- [DEV-PLAN-352](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/352-list-detail-history-page-patterns-detailed-design.md)
- [DEV-PLAN-353](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md)
- [DEV-PLAN-360](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/360-core-hr-domains-plan.md)
- [DEV-PLAN-364](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/364-staffing-position-assignment-business-rules-and-detailed-design.md)
