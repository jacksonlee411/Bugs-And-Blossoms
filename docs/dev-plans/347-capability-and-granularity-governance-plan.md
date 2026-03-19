# DEV-PLAN-347：Capability 与颗粒度治理子计划（Capability Key / Route & Action Mapping / Granularity）

**状态**: 规划中（2026-03-18 CST）

## 1. 背景与定位

本计划是 [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md) 的子计划，同时承接：

- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 对角色/权限矩阵与高风险能力分层的冻结；
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 对 `capability_key + context + as_of` 决议协议的冻结；
- [DEV-PLAN-353](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md) 对权限感知交互与提交语义一致性的要求；
- 现仓 [DEV-PLAN-150](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md) 与 [DEV-PLAN-180](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md) 的治理经验。

`347` 的目标是把“能力命名、路由映射、颗粒度词汇”收敛成一个最小且稳定的平台底座，避免后续计划继续出现：

- 同语义多套命名（capability/scope/package 并存）；
- 路由与能力映射分散在多个模块；
- 字段/能力/组织上下文边界混用；
- 门禁只能增量扫描、无法阻断结构漂移。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 冻结 `capability_key` 最小命名合同（禁止上下文编码、禁止运行时拼接）。
- [ ] 冻结双映射单点注册合同：`route -> capability_key` 与 `assistant_action_id -> capability_key`。
- [ ] 冻结最小颗粒度词汇表：`Tenant / OrgContext / Capability / Field / Time / Version`。
- [ ] 冻结跨计划通用 fail-closed 规则：缺映射、冲突映射、上下文缺失、未注册 key 一律拒绝。
- [ ] 冻结“权限单轨”不变量：Assistant 不拥有独立权限体系，只能消费操作者同源授权决策。
- [ ] 冻结统一 capability 目录最小字段：`capability_type / required_permission_bundle / risk_level / approval_required / receipt_type`。
- [ ] 将 capability 与颗粒度治理检查统一接入门禁，支撑 `342/345/353/360/390` 一致消费。

### 2.2 非目标

- [ ] 本计划不直接实现完整动态关系引擎或复杂规则执行内核。
- [ ] 本计划不替代 [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md) 的角色矩阵与授权语义。
- [ ] 本计划不替代 [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md) 的配置/策略主蓝图。
- [ ] 本计划不新增 scope/package/legacy alias 兼容窗口。
- [ ] 本计划不新增 `assistant_role / assistant_permission / assistant_policy` 等并行权限体系。

## 3. 范围

- `capability_key` 命名与注册规则
- 路由与 Assistant 动作到 capability 映射注册表
- 统一 capability 目录字段合同
- 颗粒度词汇表与边界说明
- fail-closed 拒绝码与最小 Explain 字段
- 门禁接线（capability-key / capability-route-map / granularity / assistant-action-capability-map）

## 4. 关键设计决策

### 4.1 能力键只表达动作语义（选定）

- `capability_key` 只表达“做什么”，不编码租户、BU、SetID、地域等上下文。
- 上下文由独立字段承载，评估时显式输入。
- 任何运行时拼接 `capability_key` 的实现视为违规。

### 4.2 入口映射单点注册（Route/Action 双映射，选定）

- `route -> capability_key` 与 `assistant_action_id -> capability_key` 必须来自同一事实源（或同源编译产物）。
- 禁止在 controller/handler/assistant runtime 内隐式推导 capability。
- 映射缺失、重复、指向未注册 key 时直接 fail-closed。
- Assistant 的动作授权以 `assistant_action_id` 命中 capability，不以自由文本意图直连授权判定。

### 4.3 颗粒度词汇固定且可审计（选定）

- 最小层次冻结为：
  - `Tenant`（隔离边界）
  - `OrgContext`（业务上下文）
  - `Capability`（动作语义）
  - `Field`（字段行为）
  - `Time`（as_of/effective_date）
  - `Version`（policy/version anchor）
- 禁止把服务器部署形态、路由层级等实现细节冒充业务颗粒度。

### 4.4 权限单轨与审计分轨（选定）

- 权限真值仅有一套：操作者在 `342` 定义的角色/权限包矩阵。
- Assistant 是代操作通道，不是新身份；不得新增 Assistant 专属权限包作为放行依据。
- `acting_channel=assistant` 只用于审计、回放与体验分析，不参与授权放行条件。
- UI/API/Assistant 命中同一 `capability_key` 时，授权结果必须一致。

### 4.5 先收敛治理，再扩展能力（选定）

- 第一阶段只做“命名 + 映射 + 门禁 +词汇”最小闭环。
- 动态关系、功能域开关等增强能力按后续独立里程碑推进，不作为当前前置阻塞。

## 5. 建议实施分期

1. [ ] `M1`：命名与词汇冻结  
   冻结 `capability_key` 规则、颗粒度词汇表与禁用术语。
2. [ ] `M2`：映射注册与拒绝语义冻结  
   冻结 `route -> capability` 与 `assistant_action_id -> capability` 注册表及缺失/冲突拒绝码。
3. [ ] `M3`：目录合同与权限单轨冻结  
   冻结统一 capability 目录字段合同，并明确 Assistant 不得引入并行权限体系。
4. [ ] `M4`：门禁与下游接线  
   将 `check capability-key / capability-route-map / granularity / assistant-action-capability-map` 与 `342/345/353/390` 的输入对齐。

## 6. 与其他子计划关系

- `342`：消费 `347` 的 capability 命名与映射底座，专注角色与权限语义。
- `345`：消费 `347` 的能力键和颗粒度词汇，专注配置/策略决议蓝图。
- `350`：消费 `347` 的 capability 与颗粒度词汇，把它们翻译为导航、页面主动作、字段状态与权限感知 UI 的统一产品语言，但不得重写语义边界。
- `353`：消费 `347` 的能力与字段边界，避免 UI 层重算权限语义。
- `360/370/380/390`：不得自行发明第二套 capability 命名与映射链路。
- `390`：必须消费 `347 + 342` 的单轨授权结果，不得通过 channel、prompt、模型分支形成第二放行条件。

## 7. 验收标准

- [ ] `capability_key` 命名规则与禁用模式已冻结且可被门禁检查。
- [ ] `route -> capability` 与 `assistant_action_id -> capability` 双映射注册表成为单一事实源，缺失/重复可被阻断。
- [ ] 颗粒度词汇在 `342/345/350/353` 中保持一致，不再混用 legacy 术语。
- [ ] fail-closed 规则对“缺映射、未注册、上下文缺失”可稳定拒绝并可解释。
- [ ] “权限单轨（Assistant=操作者同源授权）”已经固化，且无并行 Assistant 权限体系。
- [ ] 统一 capability 目录字段合同可被检查并被 `360/370/380/390` 直接消费。
- [ ] 现有治理门禁可直接作为 `347` 的执行入口并纳入 CI required checks。

## 8. 关联文档

- [DEV-PLAN-340](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/340-platform-and-iam-foundation-plan.md)
- [DEV-PLAN-342](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/342-authz-and-platform-permission-matrix-business-rules-plan.md)
- [DEV-PLAN-345](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/345-platform-configuration-and-policy-business-rules-blueprint.md)
- [DEV-PLAN-350](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/350-frontend-product-shell-and-interaction-system-plan.md)
- [DEV-PLAN-353](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/353-form-patterns-and-permission-aware-interaction-detailed-design.md)
- [DEV-PLAN-150](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md)
- [DEV-PLAN-180](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md)
