# DEV-PLAN-180：项目颗粒度层次统一与治理（Field/Form/Module/SetID/Tenant/Server）

**状态**: 已完成（2026-02-25 15:25 UTC）

## 1. 背景与问题
当前仓库在字段策略与能力治理上同时存在多套“颗粒度表达”：
- 旧链路：`scope_type + scope_key`（`GLOBAL/FORM`）驱动 `tenant_field_policies`；
- 新链路：`capability_key + org_applicability + business_unit_id + effective_date` 驱动 `setid_strategy_registry`；
- 讨论语境中常混用“字段/表单/模块/SetID/租户/服务器”，导致职责边界不清、策略来源不清、门禁口径不一致。

本计划采用“**目标态优先**”原则：先定义长期稳定层次，再定义迁移路径；不把临时兼容结构写入目标模型。
同时补齐“主流 HRM 普遍具备、但当前文档缺失”的**主体维度**（谁在执行策略）。

## 2. 目标与非目标
### 2.1 目标
- [x] 冻结项目统一颗粒度层次（目标态）与术语字典（唯一口径）。
- [x] 建立“层次职责 + 禁区 + 准入标准”三件套治理机制。
- [x] 建立从旧模型到目标态的退役时间表（含硬性停用日期）。
- [x] 建立面向未来的门禁：阻断新代码继续依赖旧颗粒度模型。

### 2.2 非目标
- 不在本计划内一次性重写所有运行时代码（按里程碑分批收敛）。
- 不引入任何 legacy 双链路保活策略。

## 3. 目标态颗粒度层次（长期模型，冻结建议）
目标态采用“双轴模型”：**主体轴（Who）** + **资源策略轴（What/Where）**。
同时显式声明“**对象/实例上下文**”为横切输入，不作为新层级：由后端权威上下文（RLS/查询条件）提供，禁止注入 capability 或 field 作为替代。

### 3.1 主体轴（Who）
1. **S1 主体标识（Principal）**
   - 定义：`principal_id`（审计与追踪标识）。
   - 作用：标记“谁发起”。

2. **S2 授权主体（Effective Subject）**
   - 定义：`role:{slug}`（当前仓库 MVP 口径）。
   - 作用：策略评估输入主体（可随后续计划演进为多角色模型）。

3. **S3 授权域（Domain）**
   - 定义：`tenant_uuid` 或 `global`。
   - 作用：授权边界域，不承载模块/路由/部署形态。
   - 约束：`global` 仅用于平台运维与跨租户审计能力，业务能力默认必须是 `tenant_uuid`。

> **主体轴推导规则（建议冻结）**
> - `principal_id` 为审计主键（不可变），`subject` 为授权评估主键（可变、可被角色切换影响）。
> - `principal -> subject` 必须由后端鉴权模块统一映射，不允许前端自报。
> - `subject` 允许多角色集合，但每次评估必须产出**确定的、排序稳定**的主体集合。
> - `subject` 必须与 `domain` 同时存在，禁止 `subject` 缺失或跨域注入。

### 3.2 资源策略轴（What/Where）
按“从粗到细”定义如下（业务层级仅 L1-L5）：

1. **L1 租户层（Tenant）**
   - 定义：`tenant_uuid`，系统强隔离边界。
   - 作用：数据隔离、权限边界、策略空间顶层命名域。

2. **L2 组织上下文层（Org Context）**
   - 定义：`org_applicability + business_unit_id`（字段已完成更名；取值如 `tenant` / `business_unit`）。
   - 作用：同租户内的上下文化差异策略（BU 差异等）。
   - 说明：SetID 语义在本层承载，不再平行派生独立层级。
   - 补充：`personalization_mode` 为**横切开关**（策略启用方式），不作为 L2 维度；其变化不得导致 L2 语义漂移。

3. **L3 能力层（Capability）**
   - 定义：`owner_module + capability_key`（稳定键，禁止上下文编码）。
   - 作用：表达业务动作/流程语义（create/add/insert/correct 等场景语义归入 capability，不再独立建 form 层）。
   - 补充：`owner_module` 必须来自能力注册表（`internal/server/capability_route_registry.go`）并与模块目录一致；禁止自由拼接。

4. **L4 字段层（Field）**
   - 定义：`field_key`。
   - 作用：`required/visible/maintainable/default/allowed_value_codes` 等字段级行为。

5. **L5 规则与取值层（Rule/Value）**
   - 定义：`default_rule_ref/default_value/allowed_value_codes` 等策略内容。
   - 作用：产出运行时最终判定值。
   - 规则执行语义（建议冻结）：
     1) `allowed_value_codes` 若存在则先裁剪候选集合；
     2) `default_rule_ref` 若存在则计算默认值；
     3) `default_value` 为最终兜底（仅当 rule 为空或失败时使用）；
     4) 同一 field 在同一 `policy_version` 下只能有一个“最终默认值来源”，冲突必须由后端报错并阻断。

> 横切维度（非新层级）：**有效时间**（`effective_date/end_date`）与**策略版本**（`policy_version`）。

### 3.3 现状快照（按层穷举）
以下为**项目现状**的可穷举元素/内容（数据驱动部分标注为“无静态枚举”）：

- S1 `principal_id`：格式约定（无静态枚举）。示例：`tenant:{tenant_id}:principal:{principal_id}` / `global:principal:{principal_id}`。参考 `docs/dev-plans/022-authz-casbin-toolchain.md`
- S2 `subject`（角色）：`role:anonymous` / `role:tenant-admin` / `role:tenant-viewer` / `role:superadmin`。来源 `config/access/policy.csv`
- S3 `domain`：请求域为 `tenant_uuid` 或 `global`，策略域允许 `*` 通配。参考 `docs/dev-plans/022-authz-casbin-toolchain.md`

- L1 租户层：无静态枚举（运行时租户数据）。
- L2 `org_applicability` 枚举：`tenant` / `business_unit`。参考 `modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql`
- L2 `business_unit_id` 约束：`^[0-9]{8}$`（仅当 `org_applicability=business_unit`）。参考 `modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql`
- L2 相关横切开关 `personalization_mode` 枚举：`tenant_only` / `setid`。参考 `modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql`

- L3 已注册能力：`staffing.assignment_create.field_policy` / `org.policy_activation.manage` / `org.orgunit_create.field_policy`。参考 `internal/server/capability_route_registry.go`
- L3 OwnerModule（已注册能力对应）：`staffing` / `orgunit`。参考 `internal/server/capability_route_registry.go`
- 模块目录（非能力枚举）：`iam` / `jobcatalog` / `orgunit` / `person` / `staffing`。参考 `modules/`

- L4 OrgUnit 核心字段枚举：`org_code` / `name` / `status` / `parent_org_code` / `manager_pernr` / `is_business_unit`。参考 `internal/server/orgunit_field_metadata_api.go`
- L4 create 场景接入 SetID 策略字段：`org_code` / `d_org_type`。参考 `modules/orgunit/services/orgunit_write_service.go`
- L4 扩展字段（`tenant_field_configs`）：无静态枚举（数据驱动）。

- L5 旧链路默认值模式：`default_mode` = `NONE` / `CEL`。参考 `modules/orgunit/infrastructure/persistence/schema/00018_orgunit_field_policies_schema.sql`
- L5 新链路规则字段：`default_rule_ref` / `default_value` / `allowed_value_codes`。参考 `modules/orgunit/infrastructure/persistence/schema/00021_orgunit_setid_strategy_registry_fields.sql`
- L5 规则执行要素（当前实现）：`next_org_code` + CEL 求值。参考 `modules/orgunit/services/orgunit_write_service.go`

- 迁移附录相关（旧 `scope_key`）：`orgunit.create_dialog` / `orgunit.details.add_version_dialog` / `orgunit.details.insert_version_dialog` / `orgunit.details.correct_dialog` / `GLOBAL + global`。参考 `modules/orgunit/infrastructure/persistence/schema/00018_orgunit_field_policies_schema.sql`
- 服务器层：无元素（硬禁区，不允许作为业务策略维度）。

### 3.4 命名收敛（`org_level` -> `org_applicability`，已完成）
**目标**：消除与 OrgUnit 领域字段 `org_level`（组织属性）混淆；对外统一语义为“**适用组织范围**”。

**目标字段与语义**
- 新字段名：`org_applicability`
- 枚举保持不变：`tenant` / `business_unit`
- 关联字段：`business_unit_id` 语义与约束不变

**变更范围（已完成）**
- DB：列名、约束名、索引名、SQL（含 sqlc schema）
- API/JSON：请求与响应字段名统一为 `org_applicability`
- 前端：表单字段、表格列名与文案统一为“适用组织范围”
- 文档与测试：全量替换术语

**切换策略（已完成）**
- 一次性切换，不保留双字段/双写/双读（禁止 legacy 回退）。
- 仅做列名迁移，不做数据转换。
- 前后端必须同版本部署，避免混用字段名。

**禁区**
- 不引入 `scope` 词，避免与旧 `scope_type/scope_key` 语义回潮。

## 4. 目标态治理规则（职责与禁区）
### 4.1 层次职责
- S1-S3：只承载“谁可以做”，不承载字段取值规则。
- L1/L2：只承载隔离与上下文，不承载字段值规则。
- L3：只承载业务能力语义，不承载租户/BU/SetID上下文值。
- L4/L5：只承载字段行为与规则，不承载路由或页面壳层语义。
- 对象/实例上下文（如 `orgunit_id`/`record_id`）只能来自后端权威上下文或查询条件，不允许塞入 L3/L4。

### 4.2 明确禁区（硬约束）
- 禁止把**服务器层（部署节点/进程/容器）**作为业务策略差异维度。
- 禁止在 `capability_key` 中编码 tenant/bu/setid/scope 等上下文。
- 禁止缺失主体轴输入直接做策略判定（必须具备 subject + domain）。
- 禁止新增以 `scope_key` 为主语义的策略写路径（仅允许迁移读取）。
- 禁止同一语义在两套 SoT 并行生效（`tenant_field_policies` 与 `setid_strategy_registry` 双写/双生效）。
- 禁止新增“仅前端可解释、后端不可权威解析”的策略维度。
- 禁止把对象/实例上下文作为 capability_key 或 field_key 的编码片段。

## 5. 未来导向准入标准（Gate Criteria）
任何新增“层级/维度/枚举值”必须同时满足：
1. [x] **必要性**：存在可验证用户价值，且不能由现有 L1-L5 表达。
2. [x] **稳定性**：有稳定键、可审计命名、禁止运行时拼接。
3. [x] **后端权威**：解析与冲突裁决由后端完成，前端仅展示。
4. [x] **冲突可判定**：有唯一性/优先级/时间重叠约束。
5. [x] **主体完备性**：能映射到 `subject + domain + capability` 的完整评估输入。
6. [x] **门禁可执行**：CI 可自动阻断（capability-key/route-map/no-legacy/专项检查）。
7. [x] **可退役**：提供生命周期定义（引入/过渡/冻结/退役）及时间窗。

## 6. 目标导向实施里程碑（含硬日期）
1. [x] **M1（2026-03-15）契约冻结（已提前完成）**
   - 冻结 L1-L5 术语字典、禁区、准入标准。
   - 冻结主体轴术语（S1-S3）与 `subject/domain/capability` 对齐口径。
   - 冻结 create/add/insert/correct 对应 capability 命名清单。

2. [x] **M2（2026-04-15）旧入口禁增量（已提前完成）**
   - 禁止新增 `tenant_field_policies` 写入场景。
   - 旧入口仅允许迁移性读取，不允许新业务接入。

3. [x] **M3（2026-05-15）运行时统一（已提前完成）**
   - create 已接入 capability 决策链路；add/insert/correct 当前无策略入口，落地时必须接入本链路。
   - capability 决策统一输出 `policy_version`（baseline）。

4. [x] **M4（2026-06-15）旧链路停写（已提前完成）**
   - 停止 `tenant_field_policies` 写入 API 与 UI 编辑入口。
   - 仅保留只读审计窗口（用于迁移核对）。

5. [x] **M5（2026-07-15）旧链路退役（已提前完成）**
   - 移除旧读取 fallback、下线 `scope_key` 运行时依赖。
   - 旧链路仅保留只读审计窗口，目标态成为唯一语义层次。

## 7. 验收标准（目标态）
- [x] 团队评审仅使用 L1-L5 术语，不再把 form/scope 作为目标态层级。
- [x] 所有策略决策都可落到 `subject + domain + capability + field + version` 五元证据链。
- [x] 新增策略能力必须归位到 L1-L5，且能标注唯一 SoT。
- [x] create/add/insert/correct 四类 intent 全部统一走 capability 决策与 `policy_version`。
- [x] CI 新增颗粒度门禁（建议：`make check granularity`），可阻断跨层越界与旧模型回流。
- [x] 代码仓库不再出现新增 `scope_key` 写路径。
- [x] 对象/实例上下文不再被编码进 capability/field（通过门禁与审计日志验证）。

## 8. 迁移附录（非目标态，仅过渡期参考）
### 8.1 已落地映射
- [X] `FORM + orgunit.create_dialog` -> `capability_key=org.orgunit_create.field_policy`

### 8.2 规划中映射（已冻结）
- [x] `FORM + orgunit.details.add_version_dialog` -> `capability_key=org.orgunit_add_version.field_policy`
- [x] `FORM + orgunit.details.insert_version_dialog` -> `capability_key=org.orgunit_insert_version.field_policy`
- [x] `FORM + orgunit.details.correct_dialog` -> `capability_key=org.orgunit_correct.field_policy`
- [x] `GLOBAL + global` -> 对应 capability 下 `org_applicability=tenant,business_unit_id=''` 兜底

> 本附录用于迁移，不构成目标态长期分层定义。

## 9. 风险与缓解
- **风险 1：短期学习成本上升**  
  缓解：提供 1 页术语速查表，并在评审模板强制填写“层级归属（L1-L5）”。
- **风险 2：迁移窗口内双语义混淆**  
  缓解：按硬日期推进“禁增量 -> 停写 -> 退役”，并配套门禁阻断回流。
- **风险 3：跨团队执行节奏不一致**  
  缓解：每个里程碑绑定唯一验收清单与执行责任人，逾期自动升级风险级别。

## 10. 关联文档
- `docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/164-org-type-policy-control-gap-analysis.md`
- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
