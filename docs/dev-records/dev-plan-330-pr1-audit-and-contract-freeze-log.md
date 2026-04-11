# DEV-PLAN-330 PR-1 记录：策略主链审计与契约冻结

**状态**: 已完成（2026-04-11 13:53 CST）

## 1. 范围

1. [X] 固化 `DEV-PLAN-330` 的实施批次与 PR 映射，避免后续把 `R0-R7` 留到实现阶段临场拆分。
2. [X] 盘点当前策略主链的核心热点，明确：
   - 哪些路径已经读 `SetID Strategy Registry`
   - 哪些路径仍读 `tenant_field_policies`
   - 哪些链路仍按单轴 `business_unit_node_key` 或等价 BU 上下文裁决
3. [X] 固化 PR-1 stopline，作为 `PR-2 ~ PR-6` 的入口条件。
4. [X] 将本记录接入仓库级文档地图，满足可发现性要求。

## 2. 执行入口

1. [X] 时间戳采集

```bash
date '+%Y-%m-%d %H:%M %Z'
```

2. [X] 策略主链热点检索

```bash
rg -n "setid_strategy_registry|tenant_field_policies|ResolveSetID|resolveFieldDecisionFromItems|staffing.assignment_create.field_policy" internal modules pkg docs
```

3. [X] 旧层 / 新层 / 单轴查询盘点

```bash
rg -n "ResolveTenantFieldPolicy|ResolveSetIDStrategyFieldDecision|resolveFieldDecision|resolved_setid|setid_source|business_unit_org_code|business_unit_node_key|business_unit_id" internal/server modules/orgunit pkg
```

4. [X] 路由 capability / authz 归属盘点

```bash
rg -n "staffing.assignment_create.field_policy|ObjectOrgSetIDCapability|capabilityAuthzRequirementForBinding" internal/server
```

## 3. 审计结论总览

### 3.1 路由 capability 与鉴权 object 仍处于双语义状态

1. [X] `SetID Strategy Registry` 与 explain / internal evaluate 相关路由仍绑定到 `staffing.assignment_create.field_policy`，见 [capability_route_registry.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/capability_route_registry.go#L108)。
2. [X] 同一批路由的 authz object 却统一落到 `authz.ObjectOrgSetIDCapability`，见 [capability_route_registry.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/capability_route_registry.go#L398)。
3. [X] 该分裂已满足 `DEV-PLAN-330 §3.2A` 的“语义未收口”定义，PR-1 不改行为，仅将其冻结为后续 `M3` 的明确输入。

### 3.2 动态字段策略当前并非唯一 PDP

1. [X] `internal/server` 侧仍保留一套运行时选择链，核心函数是 [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go#L1152) 的 `resolveFieldDecisionFromItems(...)`。
2. [X] `modules/orgunit/infrastructure` 侧仍保留另一套 `ResolveSetIDStrategyFieldDecision(...)`，见 [orgunit_pg_store.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/orgunit_pg_store.go#L439)。
3. [X] 第二套实现仍以 `businessUnitID` 变量名承载 BU 上下文，并直接按 `org_applicability + business_unit_node_key` 取候选，见 [orgunit_pg_store.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/orgunit_pg_store.go#L463)。
4. [X] 该实现未把 `resolved_setid` 纳入输入，也未读取 `priority_mode / local_override_mode`，说明其仍是单轴、弱模式兑现的过渡态实现。
5. [X] `/internal/rules/evaluate` 还存在一条基于 registry 列表构造 candidate 再做 CEL eligibility 判断的 sidecar 评估链，见 [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go#L190) 与 [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go#L280)。

### 3.3 仓库内已有 SetID 解析能力，但尚不存在统一 `Context Resolver`

1. [X] explain 链路在 [setid_explain_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_explain_api.go#L73) 先做 `org_code -> org_node_key`，再在 [setid_explain_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_explain_api.go#L130) 解析 `resolved_setid`。
2. [X] internal evaluate 链路在 [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go#L138) 与 [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go#L179) 重复执行相近上下文解析。
3. [X] 字段启用候选接口也会单独解析 SetID，见 [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go#L255) 与 [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go#L284)。
4. [X] 代码层面尚无单独的 `PolicyContext` 类型或“唯一 Context Resolver 边界”，因此 PR-2 必须先落该边界，再谈 schema/query/PDP 切换。

### 3.4 `tenant_field_policies` 旧层仍然存在真实读路径

1. [X] `internal/server` 仍保留旧层查询与解析入口：
   - 列表查询见 [orgunit_field_metadata_store.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_store.go#L367)
   - 单条解析见 [orgunit_field_metadata_store.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_store.go#L416)
2. [X] `modules/orgunit/infrastructure` 也仍保留旧层解析入口，见 [orgunit_pg_store.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/orgunit_pg_store.go#L335)。
3. [X] API 写入口虽已显式 `write_disabled`，但表、store、read path 仍在仓库内持续存在，见 [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go#L809)。
4. [X] 同时字段配置页预览已开始直接消费 registry 决议，而不只读旧层：
   - 静态字段决议预览见 [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go#L211)
   - 字段策略 resolve preview 见 [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go#L850)
5. [X] 结论：旧层已降级，但尚不能宣称“完全退场”；PR-4/PR-6 之前必须继续视其为兼容层审计对象。

### 3.5 Registry schema / API 仍是单轴契约

1. [X] 现有表只显式承载 `business_unit_node_key`，未承载 `resolved_setid`，见 [00020_orgunit_setid_strategy_registry_schema.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql#L9)。
2. [X] 现有唯一键也仅覆盖 `tenant_uuid + capability_key + field_key + org_applicability + business_unit_node_key + effective_date`，见 [00020_orgunit_setid_strategy_registry_schema.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql#L66)。
3. [X] 当前 API 请求体只显式表达 `business_unit_org_code`，未显式表达 `resolved_setid exact / wildcard`，见 [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go#L87)。
4. [X] 当前 PG store upsert / conflict key 仍按单轴落库，见 [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go#L314)。

### 3.6 Happy path Registry consumer 清单已明确

| 类别 | 当前入口 | 现状说明 | 后续动作 |
| --- | --- | --- | --- |
| 用户可见 create 决议 | [orgunit_create_field_decisions_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_create_field_decisions_api.go#L32) | 直接调用 registry store 决议 create 所需字段 | 迁入统一 `PolicyContext -> PDP` |
| explain | [setid_explain_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_explain_api.go#L49) | 先自行解析 org/setid，再调 registry 决议 | 复用统一 resolver |
| Assistant dry-run enrich | [assistant_create_policy_precheck.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/assistant_create_policy_precheck.go#L12) | 直接调 registry 决议 create 字段 | 按 `DEV-PLAN-350` 降为统一投影消费者 |
| 字段启用/预览 | [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go#L211) | 已开始用 registry 做动态镜像/预览 | 后续继续统一主链 |
| 写服务默认值 | [orgunit_write_service.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/services/orgunit_write_service.go#L1612) | 仍走模块内第二套 resolver/store | 收敛到唯一 PDP |
| 内部规则评估 | [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go#L120) | 从 registry list 派生候选并做 sidecar 评估 | 需要并入 explain/预检统一链 |

### 3.7 错误码 canonical 仍未切到 lower snake_case

1. [X] registry 侧常量仍以大写 legacy 形式定义，见 [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go#L30)。
2. [X] 写服务与用户提示层也仍消费大写错误码，见 [orgunit_write_service.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/services/orgunit_write_service.go#L1612) 与 [orgunit_nodes.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_nodes.go#L145)。
3. [X] 这意味着 PR-5 之前不能宣称 `DEV-PLAN-330 §4.2.7` 的错误码契约已落地。

## 4. PR-1 冻结结论

1. [X] `PR-1` 的定位是 `R0 + M1 起步 + M6 证据前置`，不改 schema、不切查询、不替换 PDP。
2. [X] 后续批次的入口条件冻结为：
   - `PR-2` 先落统一 `Context Resolver`
   - `PR-3` 再做双轴 schema 与回填
   - `PR-4` 先补唯一 PDP 与前置测试，再谈切主链
3. [X] 在 `PR-2` 完成前，任何“直接在 API/store/PDP 内补一点 `resolved_setid` 字段”的做法，都视为偏离 `330` 实施顺序。
4. [X] 在 `PR-4` 完成前，任何“schema 已双轴但运行时仍单轴”的状态，都视为 stopline。

## 5. PR-1 Stopline

1. [X] 若仓库中仍不存在统一 `Context Resolver`，不得进入双轴主查询切换。
2. [X] 若 [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go#L1152) 与 [orgunit_pg_store.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/orgunit_pg_store.go#L439) 两套 PDP 同时存在，则不得宣称 `330` 已进入验收阶段。
3. [X] 若 `tenant_field_policies` 仍参与 happy path 字段裁决，必须先在实施记录中明确它是兼容读还是正式读；未说明即 stopline。
4. [X] 若主写 API 仍无法显式表达 `resolved_setid=exact / wildcard`，则 schema 演化不得宣称完成。
5. [X] 若正式错误码仍以大写 `FIELD_POLICY_*` 作为输出契约，则 `330` 不得验收通过。
6. [X] 若 route capability 仍挂 `staffing.assignment_create.field_policy`，而 authz object 与页面归属仍按 org SetID governance 叙事，则 `M3` 不得跳过。

## 6. 交付物

1. [X] 主计划补充 PR 映射与 PR-1 已交付清单：
   [330-strategy-module-architecture-and-design-convergence-plan.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/330-strategy-module-architecture-and-design-convergence-plan.md)
2. [X] 本审计记录：
   [dev-plan-330-pr1-audit-and-contract-freeze-log.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-records/dev-plan-330-pr1-audit-and-contract-freeze-log.md)
3. [X] 仓库级文档地图新增索引：
   [AGENTS.md](/home/lee/Projects/Bugs-And-Blossoms/AGENTS.md)
