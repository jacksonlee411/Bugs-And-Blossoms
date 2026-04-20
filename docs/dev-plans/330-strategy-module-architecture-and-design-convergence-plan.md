# DEV-PLAN-330：策略模块架构混乱调查与收口方案

**状态**: 已完成（2026-04-11，`PR-1 ~ PR-6` 已完成并封板）

## 1. 背景

近期对仓库内“策略模块”现状进行梳理后，确认当前实现并非“没有架构”，而是已经形成了较明确的目标分层：

1. 动态策略层：`SetID Strategy Registry`
2. 静态字段元数据层：`tenant_field_configs`
3. 写入动作策略层：`OrgUnit Mutation Policy`
4. 版本与激活层：`policy_version / activation`

但在具体落地上，仍存在多处“过渡态混杂”与语义未完全收口的问题，导致维护者和评审者很难快速回答以下问题：

1. 哪一层才是当前运行时唯一 SoT？
2. 哪一组 capability/object 才是路由与鉴权的正式语义？
3. 哪些策略字段只是登记存在，哪些字段真正参与运行时裁决？
4. 旧 `tenant_field_policies` 目前究竟是兼容壳、只读镜像来源，还是仍参与关键运行时？

本计划用于将上述调查发现收敛为正式整改方案，作为后续代码与文档收口的唯一事实源。

## 2. 调查范围

本次调查覆盖以下对象：

1. `SetID Strategy Registry` 路由、API、运行时决策与持久化实现。
2. `tenant_field_policies` / `field-configs` 与 Strategy Registry 的边界关系。
3. OrgUnit 写场景的 capability 绑定、基线 capability 与意图 capability 的组合版本。
4. `priority_mode / local_override_mode` 的 schema、API、文档与运行时使用情况。
5. `DEV-PLAN-320` 对 Strategy Registry 中 Org 引用语义的最新影响，包括 `business_unit_org_code` / `business_unit_node_key` 的分层口径，以及 consumer runtime 是否已完成真实 cutover。

## 3. 已确认结论

### 3.1 已确认：目标架构方向是清楚的

1. [X] 静态元数据 SoT 与动态策略 SoT 的分层方向已经明确，见 `DEV-PLAN-165/184`。
2. [X] OrgUnit 写场景已经采用“基线 capability + 意图 capability”的模型，见 `DEV-PLAN-182`。
3. [X] `policy_version` 已进入写前一致性校验链路，组合版本算法已成形。

### 3.1A 已确认：`DEV-PLAN-320` 已改变 Strategy Registry 的字段与语义口径

1. [X] `DEV-PLAN-320` 已冻结 `SetID Strategy Registry` 中 BU 作用域的目标 DB/内部运行时字段为 `business_unit_node_key`，不再应继续以 `business_unit_id` 统称该语义。
2. [X] 当前外部 API / 页面输入更准确的口径是 `business_unit_org_code`，经服务边界解析后进入内部 `business_unit_node_key`。
3. [X] 上述 schema / migration / API 口径已明显前进，但 `DEV-PLAN-320` 整体仍未完成；SetID / Staffing consumer runtime 的真实 `target-real` cutover 与 explain 证据仍未闭环。
4. [X] 因此，策略模块当前的真实状态不是“仍然纯旧态 `business_unit_id`”，而是“Strategy Registry 已朝 `business_unit_node_key` 收口，但仓库整体 consumer/runtime 仍处于过渡态”。

### 3.2 已确认：当前仍存在架构与设计混乱

#### A. 路由 capability 与鉴权 object 语义存在分裂

1. [X] `SetID Strategy Registry` 路由在 capability-route 层绑定的是 `staffing.assignment_create.field_policy`，见 `internal/server/capability_route_registry.go`。
2. [X] 同一组路由在 capability → authz object 映射时又统一落到 `org.setid_capability_config` 语义，见 `internal/server/capability_route_registry.go` 的 `capabilityAuthzRequirementForBinding(...)`。
3. [X] 结果是：路由“看起来属于 staffing capability”，但实际授权语义又更接近 org SetID 治理台，容易造成页面归属、审计归属、治理归属混乱。

#### B. 同一策略决策算法出现跨层重复实现

1. [X] `internal/server/setid_strategy_registry_api.go` 内存在 `resolveFieldDecisionFromItems(...)` 及其 lookup chain。
2. [X] `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go` 内又存在一套 `baseline capability + bucket` 的决策实现。
3. [X] 两套逻辑都在处理“意图覆盖 vs 基线兜底、BU vs tenant”的命中顺序，存在未来漂移风险。

#### C. `priority_mode / local_override_mode` 被建模为核心策略，但运行时未完全兑现

1. [X] schema、API、错误码、文档均将 `priority_mode / local_override_mode` 视为正式治理字段。
2. [X] 当前运行时主要完成了合法性校验、归一化、持久化与响应回传。
3. [X] 尚未确认其已完整参与“候选层合并 / 覆盖 / 解释”主流水线。
4. [X] 这会造成“UI 可配置、文档有定义、运行时未必按其裁决”的设计落差。

#### D. 旧 `tenant_field_policies` 已降级，但仍保留较强存在感

1. [X] `tenant_field_policies` 写入口已显式返回 `write_disabled`，表明其不再是动态策略主写入口。
2. [X] 但其 store、schema、resolve 接口与兼容查询路径仍然保留。
3. [X] 同时字段配置页又开始通过 Strategy Registry 决议做动态镜像。
4. [X] 结果是：代码读者很容易误判旧层是否仍为关键运行时依赖。

#### E. “策略”一词同时指代多种职责，命名负担偏高

1. [X] 当前至少同时存在：
   - SetID Strategy Registry
   - Tenant Field Policy
   - OrgUnit Mutation Policy
   - Policy Activation / Policy Version
2. [X] 这些概念都合法，但“都叫 policy/strategy”会提高理解成本。
3. [X] 若不补统一术语表，维护时容易把“字段动态策略”“写入动作策略”“版本激活策略”混为一谈。

#### F. SetID 对策略的影响机制在“目标设计”与“当前实现”之间存在表达落差

1. [X] 按统一模型目标方向，SetID 应作为动态策略差异化的核心上下文之一，并以 `resolved_setid` 形式进入 PDP 的规范化上下文；能力模型收敛目标仍是 `capability_key + setid` 语义，而不是旧的 `scope/package` 体系。
2. [X] 结合 `DEV-PLAN-320` 的最新实施状态，更准确的现状应表述为：
   - 外部输入通常以 `business_unit_org_code` 进入；
   - Strategy Registry 的 DB/API 主口径已收敛到 `business_unit_node_key`；
   - 但 consumer/runtime 仍有部分链路保留 legacy 8 位 `business_unit_id` 传递或解释习惯。
3. [X] 因而在当前 OrgUnit 动态字段策略运行时，很多决策主维度更接近：
   - `tenant + capability_key + field_key + business_unit_node_key + as_of`
   而不是目标态中“经上下文解析后的 `resolved_setid + business_unit_node_key` 双轴输入”。
4. [X] 这意味着当前代码里，SetID 对策略的影响在很多场景下仍主要通过 BU 上下文间接体现，尚未稳定收口为 PDP 的正式规范化输入之一。
5. [X] 同时，BU 这层上下文本身也需要分清三种口径，避免继续混称：
   - `business_unit_org_code`：外部请求/页面输入
   - `business_unit_node_key`：内部运行时 / DB 命中键
   - `business_unit_id`：仅可用于描述尚未切净的 legacy/compat 痕迹，不应再作为目标态术语
6. [X] 这种落差会导致实现阅读中的典型困惑：
   - 目标态要求 `resolved_setid` 进入 PDP；
   - 代码主路径看起来却更像 `capability_key + business_unit_node_key + as_of`；
   - 个别 explain / compat 细节里又还能看到 legacy `business_unit_id`；
   - 维护者难以判断“SetID 是正式运行时主键，还是由 BU 代理承载的治理语义”。
7. [X] 目前可以确认的职责分层是：
   - SetID/个性化模式：承担统一语义轴与共享复用治理语义；
   - BU/组织上下文：外部以 `business_unit_org_code` 输入，内部以 `business_unit_node_key` 命中，在目标态中承担特异度/本地覆盖语义；
   - capability_key：表达稳定能力，不允许编码 SetID/BU/tenant 上下文。

## 4. 问题定性

本计划将当前问题定性为：

1. **不是无架构**：已有明确目标分层。
2. **是过渡态混杂**：旧层未彻底退出，新层未完全单点化。
3. **是语义收口不足**：路由、鉴权、文档、运行时之间仍有命名与归属漂移。
4. **是实现重复**：关键裁决逻辑没有完全抽成唯一 PDP。
5. **是规范化上下文尚未彻底收口**：目标态应以 `PolicyContext(resolved_setid + business_unit_node_key + as_of + capability_key + field_key)` 进入 PDP，但当前实现仍主要停留在 BU 键直驱动的过渡态。

## 4.1 对“按组织个性化需求通过配置与策略解决”的满足程度评估

### 4.1.1 目标需求重述

`DEV-PLAN-330` 所对应的核心业务意图不是“单纯整理代码结构”，而是：

1. 让不同组织的个性化需求优先通过配置与策略表达，而不是继续通过条件分支或硬编码散落在服务端与前端。
2. 让这些差异化能力可解释、可审计、可版本化，并能在相同能力下按组织上下文稳定命中。
3. 让“组织差异”最终收敛为统一治理模型，而不是每个页面、每条链路各自演化一套特例。

### 4.1.2 当前满足程度（调查判断）

当前设计对上述目标的满足程度可定性为：

1. **方向正确，样板可用**。
2. **已从硬编码迈向策略驱动**。
3. **但尚未达到“系统性解决不同组织个性化需求”的完成态**。

本次调查的主观评级为：**约 60% 完成度**。

### 4.1.3 已满足的部分

1. [X] 系统已经具备动态策略注册表，可表达字段级差异：
   - `required`
   - `visible`
   - `maintainable`
   - `default_rule_ref / default_value`
   - `allowed_value_codes`
2. [X] OrgUnit 写场景已经引入“基线策略 + 意图覆盖”模型，而不是为每个写动作完全复制一套规则。
3. [X] `policy_version` 已进入写前校验链路，表明策略不再只是展示配置，而开始参与真实运行时治理。
4. [X] 至少在 OrgUnit create 样板链路上，配置与策略已经能真实影响用户可见行为，而不是停留在文档层。
5. [X] 承接 `DEV-PLAN-320`，Strategy Registry 的 schema/API 口径已经从旧 `business_unit_id` 明显收敛到 `business_unit_node_key`，说明策略模块至少在“Org 引用字段命名与内部键语义”上已经开始从旧态走向目标态。

### 4.1.4 尚未满足“系统性解决”的关键缺口

1. [X] **SetID 尚未以 `resolved_setid` 形式稳定进入当前 PDP 的正式输入**。
   - 目标态方向应是 `resolved_setid + business_unit_node_key` 的双轴上下文；
   - 当前不少实现更直接依赖 `business_unit_node_key + as_of`；
   - 外部入口则常以 `business_unit_org_code` 进入，再映射到内部命中键；
   - 结果是 SetID 在很多场景下仍通过 BU 间接代理，而不是以统一语义轴显式参与裁决。
2. [X] **共享复用能力还不够强**。
   - 若未来诉求是“多个组织共享同一 SetID 策略”，当前设计未必能自然复用；
   - 有退化为按 BU 重复配置的风险。
3. [X] **部分正式裁决字段虽已建模，但尚未完全兑现为运行时能力**。
   - `priority_mode / local_override_mode` 的目标态应是“有限枚举、矩阵冻结、fail-closed”的正式裁决维度；
   - 当前实现仍更接近“已登记、已校验、已回显、未完全进入主决策链”的状态。
4. [X] **历史事实源尚未完全退场**。
   - `tenant_field_policies` 虽已不再是动态策略主写入口，但仍保留较强存在感，影响“唯一策略 SoT”的确定性认知。
5. [X] **唯一 PDP 尚未完全成立**。
   - 关键决策逻辑仍有跨层重复实现，降低了长期稳定满足组织个性化需求的可信度。
6. [X] **版本与激活治理仍偏轻量**。
   - 当前 activation 更接近运行时治理底座而非成熟的持久策略治理体系。

### 4.1.5 主数据分享对策略的影响

1. [X] **主数据分享首先影响策略的输入边界，而不是直接改变策略语义**。
   - 根据 `DEV-PLAN-070A/070B` 的冻结方向，共享主数据的目标态已从“运行时共享读取”转向“发布到租户本地后由租户运行时消费”；
   - 这意味着策略模块不应继续承担“是否跨租户读取共享主数据”的职责，而应默认在租户本地基线上做裁决。
2. [X] **主数据分享决定策略是否能建立稳定的 SetID 上下文**。
   - 一旦共享改为 tenant-only 运行时，策略必须先解析 `resolved_setid`，再按 `tenant + resolved_setid + as_of` 取候选；
   - 若共享边界不收口，策略就会被迫混入 fallback、共享例外、数据来源判断，削弱 `capability_key + setid` 的统一性。
3. [X] **主数据分享影响策略的复用方式，而不是直接提供跨租户读特权**。
   - `mapping_scope=global` 更适合理解为“映射可复用”，而不是“数据可跨租户读取”；
   - 真正被复用的应是 capability 到策略的组织方式，而候选主数据仍应来自租户本地已发布基线。
4. [X] **主数据分享直接影响 explain 与审计叙事是否成立**。
   - 若共享主数据先发布再消费，则 explain 可以稳定回答：
     - 命中了哪个 `resolved_setid`；
     - `setid_source` 是什么；
     - 本次裁决的数据作用域是否 tenant-only；
   - 若仍保留运行时共享读取，explain 会被迫同时解释策略命中与共享来源切换，叙事复杂度显著升高。
5. [X] **主数据分享是否收口，是 330 方案能否真正满足“不同组织个性化需求”的前置条件之一**。
   - 若共享模型长期停留在运行时共享或混合态，策略平台容易退化成“按 BU 重复配置 + 局部例外补丁”；
   - 只有先把共享主数据稳定成“发布式共享、运行时 tenant-only”，策略模块才更可能自然承接“集团标准复用 + 组织个性化裁决”的双重目标。

### 4.1.6 结论

因此，本计划当前阶段更准确的定位应是：

1. **已经构建出“组织个性化配置/策略化”的可工作框架**。
2. **但尚未完成“组织个性化需求系统性平台化治理”的最终收口**。
3. 后续收口重点不应再停留在“继续增加策略字段”，而应转向：
   - 收口上下文主键；
   - 收口唯一 PDP；
   - 收口历史事实源；
   - 收口 explain 与版本语义；
   从而真正把“不同组织的个性化需求”稳定落到配置与策略，而不是继续由代码细节隐式承担。

## 4.2 统一模型（冻结建议）

### 4.2.1 模型总览

本计划将“策略模块”的目标架构正式冻结为一套统一治理与运行时裁决模型，而不是继续把它理解为单张表、单个 API 或单个页面配置项。

统一模型的正式表达为：

`静态字段定义 + 动态策略 + 动作策略 + 版本激活 -> 唯一 PDP -> 最终字段决策 + explain + 审计版本`

其核心意图是：

1. 外部链路只负责提供原始上下文与消费决策，不再各自实现一套策略理解逻辑。
2. `Context Resolver` 负责把原始请求上下文解析为规范化 `PolicyContext`。
3. 后端唯一 PDP 负责消费 `PolicyContext` 与多层事实源，输出唯一、稳定、可解释的字段决策。
4. explain 与审计不再是附属信息，而是统一模型的正式输出之一。
5. 双轴 PDP 的确定性排序、`allowed_value_codes` 集合语义与 mode 矩阵，不再由 `330` 另起平行规范；其基础算法以 `DEV-PLAN-200 §5.1A/§5.2` 与 `DEV-PLAN-202` 为 SSOT，`330` 在此之上冻结 Strategy Registry 的双轴记录契约、分桶顺序与失败语义。

### 4.2.2 模型分层

统一模型冻结为四层。即使后续更换具体表名、API 名或模块位置，也不得改变下表的职责边界：

| 层 | 唯一主写事实 | 主写入口 | 运行时消费方 | 冻结不变量 |
| --- | --- | --- | --- | --- |
| `Static Metadata SoT` | 字段定义、字段类型、候选来源、展示元数据 | 字段配置 / metadata 写入口 | 唯一 PDP 的静态读取阶段 | 不得主写 `required / visible / maintainable / default / allowed_value_codes / mode` 等动态裁决语义 |
| `Dynamic Policy SoT` | `required / visible / maintainable / default_rule_ref / default_value / allowed_value_codes / priority / priority_mode / local_override_mode`，以及其作用域轴 | `SetID Strategy Registry` | 唯一 PDP 的候选读取与裁决阶段 | 是字段动态行为的唯一主写层；不得由页面、adapter、store 或旧表形成第二主写 |
| `Mutation Policy` | 允许哪些写动作、允许哪些字段变化、哪些模板可提交 | 动作能力/变更模板/写动作策略入口 | 写前校验、mutation guard、提交模板 | 只决定“能不能写、能改哪些字段”，不得承担字段可见、默认值或候选值语义 |
| `Policy Activation` | 当前激活版本、`policy_version`、`effective_policy_version`、写前一致性校验口径 | 发布/激活/版本切换入口 | 唯一 PDP 的快照选择与写前版本校验 | 只决定“哪版生效、版本是否 stale”，不得携带字段裁决语义 |

补充冻结说明：

1. `Mutation Policy` 不是“动态字段策略的别名”，而是动作约束层；它保护写动作与字段变更边界，不负责字段值语义。
2. `Policy Activation` 不是“普通状态位”；它是统一模型中的正式事实层，负责把“哪版当前生效”和“客户端提交时带回的版本是否仍然一致”收口成唯一口径。
3. `effective_policy_version` 在目标态中属于 `Policy Activation` 的正式输出，用于承接 `DEV-PLAN-182` 中基线版本与意图版本的组合一致性语义。

### 4.2.3 统一维度

统一模型冻结以下 7 个维度：

1. `事实源维度`
   - 区分 `Static Metadata SoT`、`Dynamic Policy SoT`、`Mutation Policy`、`Policy Activation`
   - 先回答“谁是唯一主写入口”，再谈运行时消费
2. `字段维度`
   - 以 `field_key` 为核心
   - 结合字段类型、数据源、展示能力等静态定义
3. `上下文维度`
   - 外部请求上下文正式口径：
     - `tenant`
     - `capability_key`
     - `field_key`
     - `as_of`
     - `business_unit_org_code`
   - `business_unit_org_code` 属于原始请求输入，不直接进入 PDP。
   - 规范化 `PolicyContext` 作为 PDP 正式输入，其正式口径为：
     - `tenant`
     - `capability_key`
     - `field_key`
     - `as_of`
     - `resolved_setid`
     - `business_unit_node_key`
     - `setid_source`
   - `resolved_setid` 是统一模型的正式语义轴，`business_unit_node_key` 是正式特异度/本地覆盖轴。
   - `business_unit_id` 仅允许用于描述 legacy/compat 痕迹，不再作为目标态术语。
4. `能力与意图维度`
   - 区分稳定能力语义与具体写意图
   - 支持 `baseline capability + intent override capability`
   - 覆盖 `create / add_version / insert_version / correct` 等正式写场景
5. `裁决维度`
   - 正式承载：
     - baseline vs intent override
     - business_unit vs tenant
     - `priority`
     - `priority_mode`
     - `local_override_mode`
     - fail-closed
   - `priority_mode / local_override_mode` 属于正式裁决维度，但仅允许有限枚举、冻结矩阵与非法组合硬拒绝
   - 其职责是从多条候选中收敛出唯一结果，而不是允许多链路各自解释
6. `输出决策维度`
   - PDP 正式输出口径冻结为：
     - `visible`
     - `required`
     - `maintainable`
     - `default_rule_ref / default_value`
     - `allowed_value_codes`
     - `source_type`
7. `解释与审计维度`
   - explain / 审计正式输出口径冻结为：
     - `policy_version`
     - `effective_policy_version`
     - 命中来源
     - 拒绝原因
     - `resolved_setid`
     - `setid_source`

### 4.2.4 统一运行时主链

统一模型要求后端只保留一条正式运行时主链：

1. 外部输入：
   - `tenant`
   - `capability_key`
   - `field_key`
   - `as_of`
   - `business_unit_org_code`
2. `Context Resolver` 上下文解析：
   - `business_unit_org_code -> business_unit_node_key`
   - 按组织上下文解析 `resolved_setid / setid_source`
   - 组装规范化 `PolicyContext`
3. capability 链解析：
   - 从外部 `capability_key` 展开运行时 capability bucket
   - 形成 `intent override bucket -> baseline bucket` 的正式 lookup chain
4. 静态层读取：
   - 读取字段定义、类型、候选来源与结构边界
5. 动态层候选读取：
   - 读取 capability / context 下的动态策略候选
6. 唯一 PDP 裁决：
   - 以 `PolicyContext(resolved_setid + business_unit_node_key + as_of + capability_key + field_key)` 为正式输入
   - 按统一优先级链收敛基线/覆盖、SetID 语义轴、BU 特异度轴、模式与冲突
7. 字段决策输出：
   - 输出唯一字段决策，不允许再由页面、API adapter 或 store 各自追加第二套裁决
8. explain / version 输出：
   - 同步返回 explain、命中来源与版本签名，保证可追踪、可复算、可审计

统一模型下的正式决策边界冻结如下：

1. 字段定义不参与动态裁决，只提供候选与结构信息。
2. 动态策略负责 `required / visible / maintainable / default / allowed_value_codes / mode`。
3. 动作策略负责“允许哪些写动作、允许哪些字段变化”。
4. 版本激活负责当前生效版本与写前一致性校验。
5. `resolved_setid` 是 PDP 的正式语义输入，`business_unit_node_key` 是 PDP 的正式特异度/本地覆盖输入。
6. `priority_mode / local_override_mode` 是 PDP 的正式裁决维度，但必须受限于冻结矩阵与 fail-closed 语义。
7. 唯一 PDP 负责合并以上四层并输出唯一决策。
8. 前端只能消费决策与 explain，不得形成第二 PDP。

### 4.2.5 `PolicyContext` 与 Strategy Registry 记录/查询契约（冻结）

`resolved_setid` 既然已经进入 PDP 正式输入，就不能只停留在 explain 或中间变量层。`330` 在此冻结 Strategy Registry 的双轴记录与查询契约如下：

1. `PolicyContext` 是唯一正式运行时上下文对象；任一 API、服务层或 explain 链路都不得绕过 `Context Resolver` 自行拼接 PDP 输入。
2. Strategy Registry 的动态策略记录必须显式表达两条独立作用域轴：
   - `resolved_setid`：SetID 语义轴
   - `business_unit_node_key`：BU 特异度/本地覆盖轴
3. 以上两条轴既属于记录表达，也属于候选查询键；它们不是 explain 附属字段，也不是只在命中后回填的派生值。
4. 目标态允许的记录形状只冻结为三类：
   - `resolved_setid=exact` + `business_unit_node_key=exact`
   - `resolved_setid=exact` + `business_unit_node_key=wildcard`
   - `resolved_setid=wildcard` + `business_unit_node_key=wildcard`
5. 以下形状属于非法建模，必须 fail-closed：
   - `resolved_setid=wildcard` + `business_unit_node_key=exact`
   - 任何“只写 BU、不写 SetID，但又声称是正式本地覆盖”的记录
6. tenant 级策略不是“天然 setid 无关”。其正式口径冻结为：
   - 若策略只适用于某一 `resolved_setid`，必须显式记录 `resolved_setid=exact`
   - 若策略明确适用于租户内全部 SetID，必须显式记录 `resolved_setid=wildcard`
   - 禁止把“未填写 SetID”解释成“自动对所有 SetID 生效”
7. wildcard/空值语义冻结为：
   - 存储层若使用 `NULL`、空串或其他物理表示，只能表达“wildcard”
   - `NULL`、空串不得表达“未知”“待解析”“以后再补”
   - 运行时若拿不到唯一 `resolved_setid`，应返回上下文错误，而不是拿 wildcard 偷渡
8. 唯一 PDP 的最小候选查询契约冻结为：
   - `tenant`
   - `capability_bucket`
   - `field_key`
   - `as_of`
   - `resolved_setid` 的 exact/wildcard 匹配
   - `business_unit_node_key` 的 exact/wildcard 匹配
9. 逻辑冲突检测至少必须覆盖：
   - `tenant`
   - `capability_bucket`
   - `field_key`
   - 生效区间
   - `resolved_setid` 作用域
   - `business_unit_node_key` 作用域
   - `priority`
10. 禁止继续存在以下隐式契约：
    - 先按 `business_unit_node_key` 命中，再“猜一个 setid”补 explain
    - 直接拿 `business_unit_org_code` 或 legacy `business_unit_id` 作为 PDP 命中键
    - 将 `resolved_setid` 仅视作前端展示字段，而不进入记录与查询语义

### 4.2.5A 现状差距与既有表演化路径（冻结）

`330` 不仅冻结目标态，也冻结“如何从当前单轴表结构走到双轴契约”的最小演化路径，避免把 schema/index/backfill/query 切换留到实现阶段临场决定。

当前已知差距为：

1. [X] 现有 `orgunit.setid_strategy_registry` 已收口到 `business_unit_node_key`，但记录与唯一键仍未显式承载 `resolved_setid`。
2. [X] 现有 Registry 写 API 仍主要围绕 `business_unit_org_code -> business_unit_node_key` 工作，尚不能明确表达“`resolved_setid=exact` / `resolved_setid=wildcard`”。
3. [X] 现有 consumer/runtime 仍存在仅按 BU 轴取候选的过渡实现；若不先冻结切换顺序，后续极易演变成“文档双轴、运行时单轴、explain 事后补轴”的假收口。
4. [X] `Context Resolver` 虽已在统一模型中承担“外部输入 -> 规范化 `PolicyContext`”职责，但当前计划尚未把它提升为可单独验收的实施步骤，容易导致 schema/backfill/API/PDP 各自默认依赖一个尚未真正收口的解析层。
5. [X] 旧 `tenant_field_policies` 与其 consumer/read path 的退场审计若不前置，存在“Registry 已双轴化，但 happy path 仍暗中读旧层”的假收口风险。

基于以上现状，`330` 将既有表的最小演化步骤冻结为：

1. **R0：存量审计先行**
   - 先盘点现有 Registry 记录，逐条判定其应归属为：
     - `resolved_setid=exact + business_unit_node_key=exact`
     - `resolved_setid=exact + business_unit_node_key=wildcard`
     - `resolved_setid=wildcard + business_unit_node_key=wildcard`
   - 任一记录若无法被无歧义地归类到上述三种形状之一，必须 stopline；不得带着“以后运行时再猜”的记录继续切主链。
   - `R0` 还必须同时盘点所有仍参与 happy path 的 consumer/read/explain 路径，明确它们当前是读取：
     - Strategy Registry 新层
     - `tenant_field_policies` 旧层
     - BU-only 单轴 helper / 查询
   - 任一路径若在切主链后仍需要保留旧层读取，必须在本阶段说明其仅为显式兼容读证据还是仍属 happy path；未说清即 stopline。
2. **R1：Context Resolver 单点落地**
   - 必须先落一个可单独验收的 `Context Resolver`，作为所有后续 schema/backfill/API/PDP 切换的统一前提。
   - 该层的最小职责冻结为：
     - `business_unit_org_code -> business_unit_node_key`
     - `business_unit_node_key -> resolved_setid`
     - 输出 `setid_source`
     - 对歧义、缺失、非法 source fail-closed
   - explain、主写 API 校验、主查询与版本签名都必须复用同一 resolver，而不是各自复制解析逻辑。
   - 在 `R1` 完成前，不得开始双轴主查询切换；否则实现者仍会被迫在 store/API/PDP 内各自补 resolver 逻辑。
3. **R2：既有表增量扩展**
   - 在现有 `orgunit.setid_strategy_registry` 上增量引入 `resolved_setid` 作用域列与对应约束；本计划不要求新建表，也不允许通过平行新表制造第二事实源。
   - `resolved_setid` 的物理 wildcard 表达必须在实施计划中一次性冻结，并且只能表达 wildcard，不能表达“未知/待解析/兼容态”。
   - 现有唯一键与冲突检测键必须同步纳入 `resolved_setid`，使“双轴记录契约”在存储层成为硬约束，而不是 explain 附属信息。
   - 必须新增或重写约束，硬拒绝：
     - `resolved_setid=wildcard + business_unit_node_key=exact`
     - 未声明 `resolved_setid` 却试图保存 BU 本地覆盖
4. **R3：回填与校验**
   - 所有历史记录必须在切查询前完成 `resolved_setid` 回填或显式 wildcard 判定。
   - 无法确定唯一 `resolved_setid` 的历史记录必须阻断切换，不得默认写成 wildcard。
   - 回填完成后，必须有可复查证据证明：
     - 不存在非法形状记录
     - 不存在“BU exact 但 setid 缺失”的记录
     - 新唯一键与冲突检测维度已生效
5. **R4：测试与证据前置补齐**
   - 在切 Registry 主写 API 与双轴主查询之前，必须先补齐最小验证资产，至少包括：
     - `Context Resolver` 的成功/缺失/歧义/非法 source 测试
     - 双轴 bucket 顺序测试
     - mode matrix 测试
     - explain 回放样例
     - 旧层已被识别且不会继续参与 happy path 的证据
   - `R4` 的目标不是“全部测试收尾”，而是把会决定切主链正确性的证据前置到实现前半段。
   - 若未先补齐这些测试与回放证据，不得开始 `R5/R6` 的主链替换。
6. **R5：写 API 契约切换**
   - Registry 主写 API 必须显式表达 SetID 轴，至少能区分：
     - `resolved_setid=exact`
     - `resolved_setid=wildcard`
   - 禁止继续依赖“只给 `business_unit_org_code`，由服务层默认猜测 exact setid”的隐式建模。
   - 若某条策略 intended 为 tenant 全局，则必须由 API 显式声明 wildcard；不得把“字段缺省”解释成“适用于全部 SetID”。
7. **R6：查询与 PDP 切主链**
   - 所有候选查询、冲突检测、explain 复算与版本签名必须统一切到 `resolved_setid + business_unit_node_key` 双轴。
   - 在 R4 完成前，不得宣称 `330` 已完成；“schema 已加列但主查询仍按 BU 单轴”不构成收口。
   - `M3` 的 capability/authz 归属调整若会改变 capability bucket 语义，则必须先于 `R6` 完成；若只改变页面归属与 authz object 文案，则不得影响 `R6` 已冻结的 PDP bucket 顺序与输入语义。
8. **R7：旧实现退场**
   - 任一仍以 legacy `business_unit_id` 或“BU-only 命中”驱动正式裁决的实现，都必须在切主链时移除或降级为显式兼容读证据，不得继续作为 happy path。
   - 任一仍经由 `tenant_field_policies` 旧层参与正式字段裁决的 consumer/read path，也必须在本阶段移除或明确降级为非 happy path。
   - 若切换过程中出现风险，只允许采用仓库既定的环境级保护、只读/停写、修复后重试；不得引入兼容别名窗口、双输出口径或旧实现兜底。

### 4.2.6 双轴 PDP 的确定性裁决算法（冻结）

`330` 不再自造第二套冲突决议规则。双轴 PDP 的确定性算法以 `DEV-PLAN-200 §5.2` 与 `DEV-PLAN-202` 为 SSOT，`330` 在此冻结双轴输入下必须补充的顺序与触发条件：

1. capability 链先行：
   - 先查 `intent override bucket`
   - 未命中再查 `baseline bucket`
   - 不允许绕过 capability chain 直接把不同意图与基线候选混成一桶再按 `priority` 直排
2. 候选过滤冻结为：
   - 只读取 active 且生效区间覆盖 `as_of` 的记录
   - 只读取 `tenant + capability_bucket + field_key` 命中的记录
   - 只保留与当前 `PolicyContext` 的 `resolved_setid / business_unit_node_key` 形成 exact 或 wildcard 合法匹配的记录
   - 非法记录形状在进入排序前即拒绝
3. 双轴分桶顺序冻结为：
   - `intent + setid exact + business_unit exact`
   - `intent + setid exact + business_unit wildcard`
   - `intent + setid wildcard + business_unit wildcard`
   - `baseline + setid exact + business_unit exact`
   - `baseline + setid exact + business_unit wildcard`
   - `baseline + setid wildcard + business_unit wildcard`
4. PDP 必须选择“首个非空桶”继续裁决；不得跨桶只按 `priority` 直排，以免出现“tenant wildcard 意图桶”与“本地 baseline 桶”语义倒挂。
5. 桶内排序直接继承 `DEV-PLAN-200 §5.2`：
   - `priority DESC`
   - `effective_date DESC`
   - `created_at DESC`
   - `policy_id ASC`
6. 桶内裁决语义冻结为：
   - 标量语义（`visible / required / maintainable / default_*`）采用 first-winner
   - 集合语义（`allowed_value_codes`）必须按 `DEV-PLAN-202` 的 `priority_mode + local_override_mode` 矩阵求值
   - 一致性校验必须覆盖 `allowed_value_codes ⊆ candidate_pool`、`required=true` 时集合不可非法为空、`default_value` 必须落在最终 allowed 集合内
7. 正式失败触发条件冻结为：
   - `policy_missing`：所有合法桶均为空
   - `policy_conflict_ambiguous`：在首个非空桶内，经 `DEV-PLAN-202` 的 mode 矩阵、桶内排序和一致性约束后，仍无法得到唯一合法结果
8. 正式证据输出冻结为：
   - `matched_bucket`
   - `winner_policy_ids`
   - `resolution_trace`
   - `policy_version`
   - `effective_policy_version`
   - `resolved_setid`
   - `setid_source`
   - 命中来源与拒绝原因

### 4.2.7 失败语义矩阵（冻结）

错误码 canonical 口径以 `DEV-PLAN-200 §7.1` 的 lower snake_case 为准；若某些现有链路仍残留 legacy 大写错误码，应由对应实施计划一次性切换并清退，不得在 `330` 中形成第二主源或双输出目标态。

| 失败场景 | 正式错误码 | 是否允许 fallback | explain 最低输出 | 是否构成 stopline |
| --- | --- | --- | --- | --- |
| `business_unit_org_code` 无法解析为唯一 `business_unit_node_key` | `business_unit_context_invalid` | 否 | 原始 `business_unit_org_code`、解析阶段、拒绝原因 | 否 |
| 无法得到唯一 `resolved_setid` | `setid_binding_missing` 或 `setid_binding_ambiguous` | 否 | `business_unit_node_key`、`setid_source`、拒绝原因 | 是 |
| `setid_source` 非法、禁用或不受支持 | `setid_source_invalid` | 否 | `resolved_setid`、`setid_source`、拒绝原因 | 是 |
| 所有合法候选桶均无策略命中 | `policy_missing` | 否 | `PolicyContext`、查找过的 bucket、拒绝原因 | 是 |
| 首个非空桶内仍无法化解同位冲突 | `policy_conflict_ambiguous` | 否 | `PolicyContext`、命中记录、`resolution_trace` | 是 |
| `priority_mode` 非法值或 `priority_mode/local_override_mode` 非法组合 | `policy_mode_invalid` 或 `policy_mode_combination_invalid` | 否 | 命中记录、mode 值、拒绝原因 | 是 |
| 写入请求缺少 `policy_version` 或 `effective_policy_version` | `policy_version_required` | 否 | 写入上下文、缺失字段、拒绝原因 | 否 |
| 写入请求携带的版本 stale 或与当前激活版本不一致 | `policy_version_conflict` | 否 | 请求版本、当前激活版本、拒绝原因 | 否 |

补充冻结说明：

1. 本表中的 `fallback=否` 表示禁止回退到 legacy 表、旧 `business_unit_id`、前端二次裁决或隐式 wildcard 放行。
2. 本表中的 `stopline=是` 表示若该错误出现在主链路 happy path、门禁样例或基线验收路径中，则不得宣称 `330` 已完成收口。
3. `policy_version_required / policy_version_conflict` 虽属于正常 runtime 拒绝，但若主链路产物系统性缺失版本字段，仍属于实施 stopline。

## 5. 目标与非目标

### 5.1 核心目标

1. [X] 将第 `4.2` 节统一模型固定为 `DEV-PLAN-330` 的正式目标架构主口径，后续相关整改均已围绕该模型收口。
2. [X] 已冻结“策略模块”术语表，明确 `dynamic policy / static metadata / mutation policy / activation` 的唯一中文口径，以及每层的主写入口、消费方和不变量。
3. [X] `resolved_setid + business_unit_node_key` 双轴已正式写入 Strategy Registry 的记录/查询契约，终结“只靠 BU 命中、SetID 事后解释”的过渡态。
4. [X] 动态字段策略运行时已收口为唯一 PDP，并显式继承 `DEV-PLAN-200/202` 的确定性裁决算法。
5. [X] SetID Registry 路由的 capability 归属与 authz object 归属已对齐，双语义已消除。
6. [X] `priority_mode / local_override_mode` 已冻结为正式裁决维度，并以有限枚举、合法矩阵与 fail-closed 语义完成运行时兑现。
7. [X] 旧 `tenant_field_policies` 已收敛为明确退役路径；数据库历史结构仅保留迁移事实，不再继续作为“看起来还能写/还能裁决”的影子事实源。
8. [X] explain、错误码、版本语义、测试与门禁已补齐，策略模块主路径可追踪、可复算、可审计。

### 5.2 非目标

1. [X] 本计划未直接新增数据库表；如需新表或 destructive migration，仍必须另起计划并先获用户确认。
2. [X] 本计划未在第一阶段重写全部 UI 页面，仅聚焦架构收口与契约对齐。
3. [X] 本计划未引入 legacy 双链路、回退开关或第二写入口。
4. [X] 本计划未保留 API 兼容别名窗口、双错误码输出窗口或“旧主链继续 happy path、新主链逐步试运行”的并行运行策略；风险控制仅使用环境级保护与 fail-closed 语义。

### 5.3 实施批次与 PR 映射（冻结）

`DEV-PLAN-330` 的实施顺序自本次起冻结为 6 个连续批次，避免把 `R0-R7` 的拆分时点留到实现阶段临场决定：

| 批次 | 当前状态 | 对应里程碑 / R 步骤 | 目标 | 本批完成前不得声称完成的事项 |
| --- | --- | --- | --- | --- |
| `PR-1` 审计与契约冻结 | [X] 已完成 | `R0` + `M1` 起步 + `M6` 证据前置 | 固化现状审计、热点清单、PR 映射与 stopline | 不改 schema、不切主查询、不替换 PDP |
| `PR-2` `Context Resolver` 单点落地 | [X] 已完成 | `R1` + `6.2A` | 建立单一 `Context Resolver`，输出 `business_unit_node_key + resolved_setid + setid_source` | 未完成前不得开始双轴主查询切换 |
| `PR-3` Schema 双轴化与历史回填 | [X] 已完成 | `R2 + R3` + `6.2B` 记录契约 | 在现有表上引入 `resolved_setid`、约束、唯一键与回填证据 | 若仍存在非法形状或不可判定记录即 stopline |
| `PR-4` 唯一 PDP 与前置测试 | [X] 已完成 | `R4` + `6.2` + `6.5` | 抽单一 PDP，补 bucket/mode/explain 回放测试 | 未完成前不得切主链 |
| `PR-5` API / explain / version / 错误码切主链 | [X] 已完成 | `R5 + R6` + `6.2C` | 显式表达 SetID 轴，统一 explain/version，切 canonical 错误码 | “schema 双轴、查询单轴”不构成收口 |
| `PR-6` route/authz、旧层退场与门禁收尾 | [X] 已完成 | `R7` + `6.3 + 6.4 + 6.6` | 收 capability/authz 归属，冻结 `tenant_field_policies` 定位并完成门禁证据 | 不允许旧主链继续 happy path |

### 5.3A PR-1 已交付物（2026-04-11）

1. [X] 新增 PR-1 审计记录并固化现状热点、旧层清单与 stopline：
   `docs/dev-records/dev-plan-330-pr1-audit-and-contract-freeze-log.md`
2. [X] 将 `R0-R7` 与 `PR-1 ~ PR-6` 的实施顺序写回主计划，作为后续实施 SSOT。
3. [X] 明确后续批次必须先过 `Context Resolver -> schema/backfill -> 唯一 PDP -> API/explain cutover` 的顺序，不得跳步。
4. [X] 将新记录接入 `AGENTS.md` 文档地图，满足可发现性要求。

### 5.3B PR-2 已交付物（2026-04-11）

1. [X] 新增统一 `Context Resolver`，将 `business_unit_org_code -> business_unit_node_key -> resolved_setid -> setid_source` 固化为 `internal/server` 正式边界：
   `internal/server/setid_context_resolver.go`
2. [X] explain、`/internal/rules/evaluate`、OrgUnit 字段启用候选 / options 预览不再各自重复解析 SetID 上下文，已统一复用同一 Resolver。
3. [X] 新增 `Context Resolver` 直测，并保留原接口错误口径优先级（`org_code invalid/not_found` 仍先于 `setid_resolver_missing` 暴露）：
   `internal/server/setid_context_resolver_test.go`
4. [X] 新增 PR-2 实施记录并接入仓库文档地图，作为 `PR-3` 前的证据冻结点：
   `docs/dev-records/dev-plan-330-pr2-context-resolver-implementation-log.md`

### 5.3C PR-3 已交付物（2026-04-11）

1. [X] 在既有 `orgunit.setid_strategy_registry` 上增量引入 `resolved_setid`，冻结 wildcard 物理表达为 `''`，并将格式约束、合法 shape 约束、唯一键与查找索引同步纳入双轴契约：
   - `modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql`
2. [X] 新增 `PR-3` 迁移，完成历史记录回填与 stopline：
   - tenant 记录显式回填 `resolved_setid=''`
   - business_unit 记录按 `business_unit_node_key + effective_date` 回填 exact `resolved_setid`
   - 若无法得到唯一 `resolved_setid`，迁移抛出 `SETID_STRATEGY_RESOLVED_SETID_BACKFILL_BLOCKED`
   - downgrade 若会因移除 `resolved_setid` 造成旧唯一键坍缩则直接阻断
3. [X] Registry store、snapshot/validate dbtool 与相关测试夹具已同步到双轴 schema，避免“schema 双轴、测试夹具单轴”残留：
   - `internal/server/setid_strategy_registry_api.go`
   - `cmd/dbtool/orgunit_setid_strategy_registry_snapshot.go`
   - `cmd/dbtool/orgunit_setid_strategy_registry_validate.go`
4. [X] 新增 PR-3 实施记录并接入仓库文档地图，作为 `PR-4` 前的证据冻结点：
   `docs/dev-records/dev-plan-330-pr3-schema-dual-axis-and-backfill-implementation-log.md`

### 5.3D PR-4 已交付物（2026-04-11）

1. [X] 新增共享 PDP，将双轴 bucket 命中顺序、排序、mode 合并、默认值收敛与 explain trace 重算统一收口为单一运行时实现：
   - `pkg/fieldpolicy/setid_strategy_pdp.go`
   - `pkg/fieldpolicy/setid_strategy_pdp_test.go`
2. [X] `internal/server` 已切到共享 PDP 主链，`/internal/rules/evaluate` 不再维持独立 happy-path 裁决实现；遇到 `FIELD_POLICY_*` 解析错误时按 fail-closed 返回 `deny + reason_code`：
   - `internal/server/setid_strategy_registry_api.go`
   - `internal/server/internal_rules_evaluate_api.go`
   - `internal/server/internal_rules_evaluate_api_test.go`
3. [X] OrgUnit persistence 已切到共享 PDP，按 `resolved_setid + business_unit_node_key` 查询候选并复用同一裁决逻辑，移除 store 侧平行决策实现：
   - `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`
   - `modules/orgunit/infrastructure/persistence/orgunit_pg_store_policy_test.go`
4. [X] explain、assistant precheck、字段启用/metadata 预览链路已统一把 `resolved_setid + business_unit_node_key` 带入 PDP 输入，避免“解析上下文已统一，但裁决输入仍分叉”：
   - `internal/server/setid_explain_api.go`
   - `internal/server/assistant_create_policy_precheck.go`
   - `internal/server/orgunit_create_field_decisions_api.go`
   - `internal/server/orgunit_field_metadata_api.go`
5. [X] 新增 PR-4 实施记录并接入仓库文档地图，作为 `PR-5` 前的证据冻结点：
   `docs/dev-records/dev-plan-330-pr4-unique-pdp-and-prerequisite-tests-log.md`

### 5.3E PR-5 已交付物（2026-04-11）

1. [X] Strategy Registry 主写 / 主读 API 已显式表达 `resolved_setid_scope + resolved_setid`，tenant exact / wildcard 与 business_unit exact 的外部契约不再靠 BU 输入隐式猜测：
   - `internal/server/setid_strategy_registry_api.go`
   - `apps/web/src/api/setids.ts`
   - `apps/web/src/pages/org/SetIDGovernancePage.tsx`
2. [X] explain、`/internal/rules/evaluate`、OrgUnit create-field-decisions / write-capabilities / write submit 已统一输出与消费 `policy_version + effective_policy_version`，其中 `policy_version` 固定表示 intent active version，`effective_policy_version` 固定表示组合版本签名：
   - `internal/server/setid_explain_api.go`
   - `internal/server/internal_rules_evaluate_api.go`
   - `internal/server/orgunit_create_field_decisions_api.go`
   - `internal/server/orgunit_write_capabilities_api.go`
   - `internal/server/orgunit_write_api.go`
   - `apps/web/src/api/orgUnits.ts`
   - `apps/web/src/api/setids.ts`
3. [X] 唯一 PDP 的正式错误码已切为 lower snake_case canonical 主链，`policy_missing / policy_conflict_ambiguous / policy_mode_invalid / policy_mode_combination_invalid / policy_version_required / policy_version_conflict / policy_disable_not_allowed / policy_redundant_override` 已在实现、前端映射与错误目录对齐：
   - `pkg/fieldpolicy/setid_strategy_pdp.go`
   - `internal/server/orgunit_api.go`
   - `internal/server/orgunit_nodes.go`
   - `apps/web/src/errors/presentApiError.ts`
   - `config/errors/catalog.yaml`
4. [X] explain 已正式带出 `matched_bucket / winner_policy_ids / resolution_trace / effective_policy_version`，不再保留 `resolved_config_version` 这一过渡字段。
5. [X] 新增 PR-5 实施记录并接入仓库文档地图，作为 `PR-6` 前的证据冻结点：
   `docs/dev-records/dev-plan-330-pr5-api-explain-version-error-cutover-log.md`

### 5.3F PR-6 已交付物（2026-04-11）

1. [X] SetID 治理台相关 route-level capability 与 owner module 已统一收口到 `org.orgunit_write.field_policy / orgunit`，不再继续挂靠 `staffing.assignment_create.field_policy`：
   - `internal/server/capability_route_registry.go`
   - `config/capability/route-capability-map.v1.json`
2. [X] authz requirement、路由注册与 allowlist 已同步切主并退役旧 `field-policies*` public route；旧路由只保留负向测试中的 `404/未映射` 回归断言，不再保留 runtime/public 写入口：
   - `internal/server/authz_middleware.go`
   - `internal/server/handler.go`
   - `config/routing/allowlist.yaml`
3. [X] `tenant_field_policies` 旧兼容层已退出 runtime happy path：server 旧 API/store、module 旧 read helper、前端旧 helper 与页面旧 dialog 全部删除；DB 历史表仅保留为历史结构/迁移事实，不再代表正式主链：
   - `internal/server/orgunit_field_metadata_api.go`
   - `internal/server/orgunit_field_metadata_store.go`
   - `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`
   - `apps/web/src/api/orgUnits.ts`
   - `apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx`
4. [X] `make check no-legacy` 已补 retired runtime/public symbol 防回流扫描，阻断旧 public route、前端旧 helper 与旧 store API 回流：
   - `scripts/ci/check-no-legacy.sh`
5. [X] 新增 PR-6 实施记录并接入仓库文档地图，作为 `DEV-PLAN-330` 当前批次完成证据：
   `docs/dev-records/dev-plan-330-pr6-route-authz-legacy-retirement-and-gates-log.md`

## 6. 收口方案

### 6.1 M1：术语与职责冻结

1. [X] 已在文档中统一以下术语：
   - `Static Metadata SoT`
   - `Dynamic Policy SoT`
   - `Mutation Policy`
   - `Policy Activation`
2. [X] 已明确每个术语的：
   - 事实源
   - 主写入口
   - 运行时消费方
   - explain 责任
   - 冻结不变量
3. [X] 已同步冻结统一模型的正式维度与输入/输出语义，至少包括：
   - 事实源维度
   - 字段维度
   - 上下文维度
   - 能力与意图维度
   - 裁决维度
   - 输出决策维度
   - 解释与审计维度
4. [X] 已同步冻结 BU 上下文相关术语：
   - `business_unit_org_code`：仅用于外部 API / 页面输入与回显
   - `business_unit_node_key`：仅用于内部运行时 / DB / PDP 命中键
   - `business_unit_id`：仅允许用于描述 legacy/compat 痕迹，不再作为目标态术语
5. [X] 已明确 `resolved_setid` 属于正式语义轴，`business_unit_node_key` 属于正式特异度轴；二者都进入统一模型的正式上下文，而不是“术语说明”层装饰。
6. [X] 已冻结禁止继续使用会引发混淆的泛称来描述不同层，尤其禁止把 `business_unit_org_code` / `business_unit_node_key` / `business_unit_id` 混写成同一层语义。

### 6.2 M2：动态字段策略 PDP 单点化

1. [X] 已抽取唯一决策器，统一承载：
   - baseline vs intent override lookup chain
   - `PolicyContext(resolved_setid + business_unit_node_key + as_of + capability_key + field_key)`
   - `DEV-PLAN-200/202` 的确定性裁决算法
   - `priority_mode / local_override_mode`
   - conflict / missing / explain / version 输出
2. [X] `internal/server` 与 `modules/orgunit/infrastructure` 已不再各自维护平行决策逻辑。
3. [X] 该 PDP 的职责已显式对应第 `4.2.4` 节统一运行时主链与第 `4.2.6` 节双轴算法，而不是局部 lookup helper。
4. [X] 所有相关 API、服务层与 explain 链路已统一复用该 PDP。

### 6.2A M2 前置：`Context Resolver` 单点落地

1. [X] 已在切双轴 schema、主写 API 与 PDP 前先落单一 `Context Resolver`。
2. [X] 该 Resolver 的正式输入/输出已冻结为：
   - 输入：`tenant + capability_key + field_key + as_of + business_unit_org_code`
   - 输出：`business_unit_node_key + resolved_setid + setid_source`
3. [X] Resolver 的失败语义已直接对齐第 `4.2.7` 节：
   - `business_unit_context_invalid`
   - `setid_binding_missing`
   - `setid_binding_ambiguous`
   - `setid_source_invalid`
4. [X] explain、主写 API、双轴主查询、版本签名与回放工具已统一复用 Resolver 逻辑，不再重复复制。
5. [X] `Context Resolver` 已具备单独验收样例与回放证据，证明它不是“文档里的逻辑概念”，而是可复用的正式边界。

### 6.2B M2 补充：SetID 上下文收口

1. [X] 已冻结“SetID 如何影响策略”的正式口径：
   - `business_unit_org_code` 仅属于外部原始请求输入；
   - `Context Resolver` 必须将其解析为 `business_unit_node_key + resolved_setid + setid_source`；
   - `resolved_setid` 作为 PDP 的正式语义输入；
   - `business_unit_node_key` 作为 PDP 的正式特异度/本地覆盖输入。
2. [X] 已冻结 Strategy Registry 的正式记录契约：
   - `resolved_setid` 与 `business_unit_node_key` 都进入记录作用域
   - 只允许 `setid exact + bu exact`、`setid exact + bu wildcard`、`setid wildcard + bu wildcard`
   - `setid wildcard + bu exact` 为非法形状
3. [X] 已明确该问题属于统一模型“上下文维度 + 记录契约”的收口，而不是零散字段命名修补。
4. [X] explain、测试、版本签名与冲突复算已显式回显或使用：
   - `resolved_setid`
   - `setid_source`
   - `business_unit_node_key`
5. [X] 任何正式链路已不再把 `business_unit_org_code` 或 legacy `business_unit_id` 直接当作 PDP 命中键。
6. [X] 维护者现已能明确回答：
   - 哪一层负责从外部输入解析出 `PolicyContext`
   - `resolved_setid` 如何承担统一语义轴
   - `business_unit_node_key` 如何承担特异度/本地覆盖语义
   - wildcard 如何表达
   - 哪些 legacy 口径仍存在及其退出路径
7. [X] 该里程碑已按第 `4.2.5A` 节的顺序落地：
   - 先完成存量记录审计
   - 再完成 `Context Resolver` 单点落地
   - 再完成既有表增量扩展与约束
   - 再完成历史记录回填与非法形状清零
   - 再补齐切主链前置测试与回放证据
   - 再切 Registry 主写 API 的 SetID 轴表达
   - 最后切主查询、PDP、explain 与版本签名
8. [X] 已冻结“无法唯一确定 `resolved_setid` 的历史记录”即 stopline；未通过 wildcard、前端补参或旧查询兜底绕过。

### 6.2C M2 补充：失败语义与版本契约

1. [X] 失败语义已直接对齐第 `4.2.7` 节失败矩阵，不再由各接口散落定义。
2. [X] `policy_missing / policy_conflict_ambiguous / policy_mode_invalid / policy_version_required / policy_version_conflict` 等正式错误码已在文档、实现与用户提示层保持一一对应。
3. [X] `Policy Activation` 已成为正式消费层：
   - 负责当前激活版本
   - 负责 `policy_version / effective_policy_version`
   - 负责写前 stale 校验
4. [X] 缺上下文、缺策略、非法 mode、非法记录形状均不会 fallback 到 legacy 路径、前端二次裁决或“默认放行”。
5. [X] canonical 输出错误码已以 `DEV-PLAN-200 §7.1` 的 lower snake_case 为准；legacy 大写错误码仅作为迁移输入或局部兼容读处理。
6. [X] 错误码切换已具备单独 stopline：
   - API/前端/用户提示若仍把 legacy 大写错误码当正式输出，则 `330` 不得验收通过
   - 禁止出现“同一主链同时承诺大写与 lower snake_case 都是正式输出”的双口径窗口

### 6.3 M3：路由 capability 与鉴权归属对齐

1. [X] 已重新评估 `SetID Strategy Registry` 治理台路由归属；正式 route-level capability 不再继续挂在 `staffing.assignment_create.field_policy`。
2. [X] 已将治理台路由统一收口到 `org.orgunit_write.field_policy`，并同步使用 `orgunit` 作为 owner module / authz 归属。
3. [X] capability-route-map、authz requirement、页面跳转归属与 explain 入口已一次性对齐 canonical 主链。

### 6.4 M4：旧 `tenant_field_policies` 兼容层收边

1. [X] 已明确其最终定位为：数据库历史结构仅保留 `migration source only`，runtime/public 语义为 `fully retired`；不再保留 `read-only compatibility`。
2. [X] 已以前置审计结果为输入盘清 consumer/read/explain 路径，并在 `PR-6` 直接完成 runtime/public 退役，不再把该问题后置。
3. [X] server/module/public 旧读写路径已删除，`tenant_field_policies` 不再参与 happy path 正式字段裁决；仅保留历史结构与迁移事实层语义。
4. [X] 字段配置页动态镜像已统一经由 Strategy Registry / PDP 输出，不再绕回旧层，也不再保留旧 policy dialog / helper 写入口。

### 6.5 M5：`priority_mode / local_override_mode` 正式裁决维度兑现

1. [X] 已盘点当前真实使用场景与 explain 诉求。
2. [X] 已冻结其正式语义为：有限枚举、合法矩阵、非法组合 fail-closed；实现口径对齐 `DEV-PLAN-202`。
3. [X] 它们已正式进入第 `4.2.4` 节统一运行时主链并参与第 `4.2.6` 节唯一 PDP 裁决，而不是只停留在 schema/API 回显层。
4. [X] explain、测试矩阵与回放证据已能回答 mode 如何影响最终裁决结果。
5. [X] 在运行时兑现完成前，文档与 UI 未将其宣称为“已完全生效”的稳定能力；完成后已按正式主链口径收口。

### 6.6 M6：测试、证据与门禁

1. [X] 针对唯一 PDP 的确定性测试已在 `PR-4` 完成，并作为切主链前置条件而非收尾工作：
   - baseline/intent bucket 顺序
   - `resolved_setid exact/wildcard`
   - `business_unit_node_key exact/wildcard`
   - `policy_missing / policy_conflict_ambiguous`
   - mode matrix
2. [X] `Context Resolver` 已具备单独测试与回放证据，确保同输入得到同一 `PolicyContext` 输出。
3. [X] explain 证据已在 `PR-5` 完成补齐，确保同输入可复算。
4. [X] 失败语义回归已覆盖：
   - `business_unit_context_invalid`
   - `setid_binding_missing / setid_binding_ambiguous`
   - `policy_missing`
   - `policy_conflict_ambiguous`
   - `policy_mode_invalid`
   - `policy_version_required / policy_version_conflict`
5. [X] `Mutation Policy` 与 `Policy Activation` 已具备最小验证样例，边界未重新回流到 `Dynamic Policy SoT`。
6. [X] 已按 `AGENTS.md` 与 `DEV-PLAN-012` 收口相关门禁，`PR-6` 额外补齐 retired runtime/public symbol 防回流检查。
7. [X] 既有表演化证据已包含：
   - 存量记录分类结果
   - 旧层 consumer/read/explain 路径清单
   - 非法形状清零证据
   - `Context Resolver` 已成为唯一规范化入口的证据
   - `resolved_setid` 已进入唯一键/冲突检测键的证据
   - 双轴主查询已替换单轴 BU 查询的证据
8. [X] 错误码收口证据已包含：
   - canonical 输出为 lower snake_case 的 API 样例
   - 用户提示层与错误码一一对应的样例
   - legacy 大写错误码不再作为正式输出的回归样例

## 7. 验收标准

1. [X] 仓库内已能明确回答“动态字段策略的唯一 PDP 在哪里”。
2. [X] `Static Metadata SoT / Dynamic Policy SoT / Mutation Policy / Policy Activation` 四层都能明确说明主写入口、运行时消费方与冻结不变量。
3. [X] SetID Registry 路由的 capability 归属、authz object、页面定位三者一致。
4. [X] `priority_mode / local_override_mode` 已按有限枚举、冻结矩阵与 fail-closed 语义进入正式裁决维度，运行时地位不再模糊。
5. [X] `tenant_field_policies` 的兼容状态在代码、文档、页面层均表达一致。
6. [X] 同一上下文下的字段决策结果只由一条主路径给出，且 explain 可追踪。
7. [X] `resolved_setid` 已稳定进入 PDP 的正式输入与 Strategy Registry 的正式记录/查询契约，`business_unit_node_key` 已稳定进入 PDP 的正式特异度/本地覆盖输入，`business_unit_org_code` 不再直接参与命中。
8. [X] Strategy Registry 的作用域形状只存在本计划允许的三类，不再出现 `setid wildcard + bu exact` 这类语义不清记录。
9. [X] 同一 `PolicyContext` 输入下，双轴 PDP 的 bucket 顺序、桶内排序、mode 矩阵与错误码结果可重复复算，且与 `DEV-PLAN-200/202` 保持一致。
10. [X] 失败语义矩阵已冻结：缺上下文、缺 SetID、缺策略、mode 非法、版本 stale 都有稳定错误码、explain 最低输出与 fail-closed 语义。
11. [X] BU 上下文字段分层表达一致：外部只谈 `business_unit_org_code`，规范化上下文谈 `resolved_setid + business_unit_node_key`，legacy `business_unit_id` 仅在兼容说明中出现。
12. [X] 维护者能够按第 `4.2` 节统一模型完整解释任一字段决策：输入上下文、命中层次、裁决路径、最终输出与 explain 结果均可对应到同一模型。
13. [X] `orgunit.setid_strategy_registry` 现有表已完成双轴化：`resolved_setid` 已进入记录约束、唯一键/冲突检测键与主查询契约；不存在“schema 双轴、查询单轴”的过渡残留。
14. [X] Registry 主写 API 已能显式表达 `resolved_setid=exact / wildcard`，不再通过 BU 输入隐式猜测 SetID 作用域。
15. [X] 不存在 API 兼容别名窗口、双错误码正式输出窗口或“旧主链 happy path 保留”的并行目标态；若系统需要风险缓解，只通过环境级保护与 fail-closed 语义实现。

## 8. 风险与缓解

1. **R1：收口时误伤现有页面行为**
   - 缓解：先冻结术语、双轴契约与主路径，再做运行时替换；若上线风险升高，仅允许使用环境级保护、只读/停写与修复后重试，不允许使用 API 兼容别名窗口或旧实现兜底。
2. **R2：重复逻辑迁移时出现裁决差异**
   - 缓解：先补双轴 bucket tests、mode matrix tests 与 explain 回放证据，再抽单点实现。
3. **R3：SetID 轴被继续弱化为 explain 装饰字段**
   - 缓解：将 `resolved_setid` 直接纳入记录/查询契约、唯一键与主查询，并禁止 `setid wildcard + bu exact` 这类语义偷渡。
4. **R4：继续保留“文档比代码领先”的假收口**
   - 缓解：将 `priority_mode / local_override_mode`、失败矩阵、双轴 schema 演化与错误码 canonical 输出一并纳入 stopline，不允许长期停留在“仅文档冻结、主链未切”状态。

## 9. 门禁与验证（SSOT 引用）

按 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 执行，不在本文复制脚本实现。本批次整改已命中并通过：

1. [X] Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
2. [X] Routing / capability：`make check routing && make check capability-route-map && make check capability-key`
3. [X] 文档：`make check doc`
4. [X] Legacy 防回流：`make check no-legacy`

## 10. 关联文档

1. `docs/archive/dev-plans/102c2-bu-personalization-strategy-registry.md`
2. `docs/archive/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
3. `docs/dev-plans/182-bu-policy-baseline-and-intent-override-unification.md`
4. `docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md`
5. `docs/dev-plans/202-blueprint-policy-resolution-and-allowed-values-determinism.md`
6. `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
7. `docs/archive/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
8. `docs/dev-plans/320-org-node-key-cutover-plan-no-global-expansion.md`
9. `docs/dev-records/dev-plan-320-stopline-log.md`
10. `AGENTS.md`
