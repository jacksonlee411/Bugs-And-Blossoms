# DEV-PLAN-330：策略模块架构混乱调查与收口方案

**状态**: 规划中（2026-04-10 07:37 CST）

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

## 3. 已确认结论

### 3.1 已确认：目标架构方向是清楚的

1. [X] 静态元数据 SoT 与动态策略 SoT 的分层方向已经明确，见 `DEV-PLAN-165/184`。
2. [X] OrgUnit 写场景已经采用“基线 capability + 意图 capability”的模型，见 `DEV-PLAN-182`。
3. [X] `policy_version` 已进入写前一致性校验链路，组合版本算法已成形。

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

1. [X] 按现行契约方向，SetID 应作为动态策略差异化的核心上下文之一；能力模型收敛目标是 `capability_key + setid`，而不是旧的 `scope/package` 体系。
2. [X] 但在当前 OrgUnit 动态字段策略运行时，很多决策主维度更直接使用的是 `tenant + capability_key + field_key + business_unit_id + as_of`。
3. [X] 这意味着当前代码里，SetID 对策略的影响在很多场景下是“通过 BU 上下文间接影响”，而不是“处处显式以 setid 为一等入参”。
4. [X] 这种落差会导致实现阅读中的典型困惑：
   - 文档说是 `capability_key + setid`；
   - 代码看起来却更像 `capability_key + business_unit_id + as_of`；
   - 维护者难以判断“SetID 是正式运行时主键，还是由 BU 代理承载的治理语义”。
5. [X] 目前可以确认的职责分层是：
   - SetID/个性化模式：决定某能力是否允许做差异化；
   - BU/组织上下文：在当前实现中经常承担运行时命中的直接分流键；
   - capability_key：表达稳定能力，不允许编码 SetID/BU/tenant 上下文。

## 4. 问题定性

本计划将当前问题定性为：

1. **不是无架构**：已有明确目标分层。
2. **是过渡态混杂**：旧层未彻底退出，新层未完全单点化。
3. **是语义收口不足**：路由、鉴权、文档、运行时之间仍有命名与归属漂移。
4. **是实现重复**：关键裁决逻辑没有完全抽成唯一 PDP。
5. **是上下文主键尚未彻底收口**：SetID 与 `business_unit_id` 在“谁是正式运行时主键”上仍处于过渡态。

## 5. 目标与非目标

### 5.1 核心目标

1. [ ] 冻结“策略模块”术语表，明确 `dynamic policy / static metadata / mutation policy / activation` 的唯一中文口径。
2. [ ] 让动态字段策略运行时只保留一个 PDP，消除跨层重复实现。
3. [ ] 对齐 SetID Registry 路由的 capability 归属与 authz object 归属，消除双语义。
4. [ ] 明确 `priority_mode / local_override_mode` 的命运：
   - 要么进入正式运行时裁决；
   - 要么从主契约降级为保留字段；
   - 要么删除并补迁移。
5. [ ] 将旧 `tenant_field_policies` 收敛为明确的兼容层或彻底退役路径，避免继续作为“看起来还能写/还能裁决”的影子事实源。
6. [ ] 补齐 explain、测试与门禁，使策略模块的主路径可追踪、可复算、可审计。

### 5.2 非目标

1. [ ] 本计划不直接新增数据库表；如需新表或 destructive migration，必须另起计划并先获用户确认。
2. [ ] 本计划不在第一阶段重写全部 UI 页面，只聚焦架构收口与契约对齐。
3. [ ] 本计划不引入 legacy 双链路、回退开关或第二写入口。

## 6. 收口方案

### 6.1 M1：术语与职责冻结

1. [ ] 在文档中统一以下术语：
   - `Static Metadata SoT`
   - `Dynamic Policy SoT`
   - `Mutation Policy`
   - `Policy Activation`
2. [ ] 明确每个术语的：
   - 事实源
   - 主写入口
   - 运行时消费方
   - explain 责任
3. [ ] 禁止继续使用会引发混淆的泛称来描述不同层。

### 6.2 M2：动态字段策略 PDP 单点化

1. [ ] 抽取唯一决策器，统一承载：
   - baseline vs intent override lookup chain
   - BU vs tenant 优先级
   - SetID 与 `business_unit_id` 的职责边界
   - `priority_mode / local_override_mode`
   - conflict / missing / explain 输出
2. [ ] `internal/server` 与 `modules/orgunit/infrastructure` 不得再各自维护平行决策逻辑。
3. [ ] 所有相关 API 与服务层统一复用该 PDP。

### 6.2A M2 补充：SetID 上下文收口

1. [ ] 冻结“SetID 如何影响策略”的正式口径：
   - 是运行时显式主键；
   - 或是通过 `business_unit_id -> setid` 映射间接承载；
   - 不允许文档与代码长期各说一套。
2. [ ] 若继续采用“BU 代理承载 SetID 差异”的实现，必须在文档、API 契约、explain 字段中显式说明。
3. [ ] 若转向 `capability_key + setid` 直接决策，则必须补齐：
   - SetID 解析前置；
   - explain 回显 `resolved_setid / setid_source`；
   - 相关测试与版本签名输入。
4. [ ] 无论采用哪种路径，都必须让维护者能明确回答：
   - SetID 是否为正式运行时输入；
   - `business_unit_id` 与 SetID 是并列上下文还是代理关系；
   - 哪一层负责从 BU 解析出 SetID。

### 6.3 M3：路由 capability 与鉴权归属对齐

1. [ ] 重新评估 `SetID Strategy Registry` 是否应继续挂在 `staffing.assignment_create.field_policy`。
2. [ ] 若该页面本质是治理台，应改为对齐 org SetID governance capability。
3. [ ] capability-route-map、authz requirement、页面文案、explain 归属必须一次性一起收口。

### 6.4 M4：旧 `tenant_field_policies` 兼容层收边

1. [ ] 明确其最终定位：
   - `read-only compatibility`
   - `migration source only`
   - `fully retired`
2. [ ] 若仍保留读路径，必须在代码与文档中显式标注其兼容属性。
3. [ ] 若字段配置页读取动态镜像，应统一经由 Strategy Registry / PDP 输出，不再绕回旧层。

### 6.5 M5：`priority_mode / local_override_mode` 落地或降级

1. [ ] 盘点当前真实使用场景与 explain 诉求。
2. [ ] 若保留为核心契约，必须接入主决策链并补齐回归测试。
3. [ ] 若短期不进入主决策链，应调整文档与 UI 表达，避免误导用户认为其已生效。

### 6.6 M6：测试、证据与门禁

1. [ ] 针对唯一 PDP 增加确定性测试：
   - BU/tenant
   - baseline/intent
   - conflict/missing
   - mode matrix
2. [ ] 补 explain 证据，确保同输入可复算。
3. [ ] 按 `AGENTS.md` 与 `DEV-PLAN-012` 收口相关门禁。

## 7. 验收标准

1. [ ] 仓库内能明确回答“动态字段策略的唯一 PDP 在哪里”。
2. [ ] SetID Registry 路由的 capability 归属、authz object、页面定位三者一致。
3. [ ] `priority_mode / local_override_mode` 的运行时地位不再模糊。
4. [ ] `tenant_field_policies` 的兼容状态在代码、文档、页面层均表达一致。
5. [ ] 同一上下文下的字段决策结果只由一条主路径给出，且 explain 可追踪。
6. [ ] “SetID 如何影响策略”在文档、API、运行时与 explain 中表达一致，不再出现“文档写 setid、代码主要看 BU”的长期歧义。

## 8. 风险与缓解

1. **R1：收口时误伤现有页面行为**
   - 缓解：先冻结术语与主路径，再做运行时替换；对外 API 保持兼容窗口。
2. **R2：重复逻辑迁移时出现裁决差异**
   - 缓解：先补 golden tests，再抽单点实现。
3. **R3：继续保留“文档比代码领先”的假收口**
   - 缓解：将 `priority_mode / local_override_mode` 明确纳入 stopline，不允许长期停留在“仅 schema/API 存在”状态。

## 9. 门禁与验证（SSOT 引用）

按 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 执行，不在本文复制脚本实现。预计后续整改将命中：

1. [ ] Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
2. [ ] Routing / capability：`make check routing && make check capability-route-map && make check capability-key`
3. [ ] 文档：`make check doc`
4. [ ] Legacy 防回流：`make check no-legacy`

## 10. 关联文档

1. `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
2. `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
3. `docs/dev-plans/182-bu-policy-baseline-and-intent-override-unification.md`
4. `docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md`
5. `docs/dev-plans/202-blueprint-policy-resolution-and-allowed-values-determinism.md`
6. `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
7. `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
8. `AGENTS.md`
