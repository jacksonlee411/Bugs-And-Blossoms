# DEV-PLAN-200：组合优先的积木式页面与功能架构蓝图（Field Config × Dict × CRUD Pattern × Strategy）

**状态**: 规划中（2026-02-28 12:20 UTC，已纳入 `allowed_value_codes × SetID × Dict` 与“自建 Temporal 编排决策”收敛补丁）

## 1. 背景与问题定义

当前字段治理与策略治理正在从“双写/双事实源”向“双层 SoT”收敛（承接 `DEV-PLAN-165/184`）。在此基础上，需要进一步回答一个结构性问题：

1. 页面到底是“继承出来的差异”，还是“由积木组合出来的差异”？
2. 字段配置、字典配置、CRUD 行为模板、策略差异化各自的边界是什么？
3. 如何在保持统一交互与可维护性的同时，支持 capability/context 驱动的差异化行为？
4. 如何把“需求 -> 配置 -> 提交”变成可被 AI 自动化编排、但仍安全可控的过程？

本蓝图采用“组合优于继承”的原则，给出可执行的分层架构、AI 编排契约与对话式事务模型。

## 2. 术语与边界冻结（先统一语言）

### 2.1 Surface / Intent

1. **Surface（页面壳）**：如 `create_dialog/details_dialog/list_page`，只负责承载与交互容器。  
2. **Intent（功能意图）**：如 `create_org/add_version/insert_version/correct`，只负责行为语义。

冻结规则：

- Surface 不直接定义业务规则；
- Intent 不直接定义 UI 结构；
- 两者通过 `capability_key` 与策略层关联。

### 2.2 Surface/Intent/Capability 注册事实源

新增并冻结 `SurfaceIntentCapabilityRegistry`（命名可在实现阶段按项目规范微调）：

1. [ ] 注册键：`surface + intent + effective_date(+end_date)`。
2. [ ] 注册值：`capability_key + mapping_version + status`。
3. [ ] 约束：同一时段同一 `surface+intent` 只能命中一个激活映射；禁止默认回退 capability。
4. [ ] 运行时：组合流水线必须先查注册表；缺失或多命中直接 fail-closed（`mapping_missing/mapping_ambiguous`）。
5. [ ] 门禁：把 `surface/intent -> capability_key` 完整性纳入强制检查，阻断未注册路径上线。

### 2.3 AI 编排权限主体冻结（Actor-Delegated Authorization）

1. [ ] AI 编排不引入独立“AI 超级账号”或并行授权体系。
2. [ ] AI 所有计划与提交都必须绑定发起人 `actor_id`，并继承其租户、角色与授权边界。
3. [ ] 权限判定沿用现有 subject/domain/object/action（Casbin + RLS）链路，不得旁路。
4. [ ] 角色分层至少覆盖：系统配置管理员、HR 专业用户、普通员工、经理。
5. [ ] 同一请求内禁止“身份漂移”：`actor_id/tenant_id/role_set` 发生变化即 fail-closed。
6. [ ] AI 仅是交互入口，不是授权主体；同一 `actor` 的同一意图在 AI 与 UI 两个入口的授权结果必须一致。

## 3. 目标与非目标

### 3.1 目标

1. [ ] 冻结“积木式架构”四层边界：Static Metadata / Dict Options / CRUD Pattern / Dynamic Policy。
2. [ ] 建立统一行为模板：create/add/insert/correct/delete 遵循同一提交流水线与错误契约。
3. [ ] 保障差异化仅由策略层表达（`capability_key + context`），UI 不承载信任判定。
4. [ ] 形成可门禁、可回放、可解释的实施路线（含跨层版本一致性与 fail-closed）。
5. [ ] 冻结性能预算与停止线，阻断“组合架构落地后退化为 N+1 查询”。
6. [ ] 冻结策略冲突决议算法，确保同输入同输出（deterministic）。
7. [ ] 增加“需求到系统配置实现（Req2Config）”的 AI 编排契约，限制 AI 只生成结构化计划，不直接写库。
8. [ ] 建立对话式事务处理模型，确保多轮对话下提交仍保持 One Door、幂等、可回放、可取消与 fail-closed。

### 3.2 非目标

1. 不在本计划内重写 One Door 写入内核。
2. 不新增 legacy 回退链路或双路并行判定。
3. 不一次性覆盖所有业务模块；优先在 Org 模块完成蓝图落地与证据闭环。

## 4. 核心原则

1. **Composition over Inheritance**：页面不继承页面；以可组合积木描述结构与行为。
2. **Single Responsibility per Layer**：每层只负责一种变化维度，不跨层主写同一语义。
3. **Pattern First**：CRUD 行为先统一模板，再通过策略做上下文差异化。
4. **Policy as Data**：策略是可版本化数据，不是散落在 UI/Handler 的条件分支。
5. **Fail-Closed**：缺上下文、缺策略、版本冲突、非法组合一律拒绝。
6. **Deterministic Resolution**：策略冲突决议必须可复算、可解释、可审计。
7. **No Tx, No RLS**：组合读写链路必须显式事务与租户注入，违者拒绝。
8. **AI as Planner, Kernel as Judge**：AI 只负责提案与编排，最终合法性由规则引擎/内核校验并裁决。
9. **Actor-Delegated Authorization**：AI 仅代操作者执行，权限完全等同操作者，不得额外提权。

## 5. 总体架构蓝图

### 5.1 四层 SoT 分层

#### L1：静态积木层（Static Metadata SoT）

事实源：Field Config。职责：定义“页面可由哪些积木构成”。

- 字段基础属性：`field_key/value_type/enabled_on/disabled_on`
- 展示与编排属性：label、排序、分组、可筛选/可排序
- 数据源类型声明：`data_source_type/data_source_config`

> 该层决定“能拼什么”，不决定“此刻允许怎么拼”。

#### L2：字典属性层（Dict Options SoT）

事实源：Dict Config（含发布口径）。职责：提供积木属性可选值池（候选集）。

- 字典值生命周期与发布状态
- 候选值来源与取数口径
- 不承载 required/default/maintainable 等业务行为规则

> 该层决定“候选池是什么”，不决定“当前场景最终可选什么”。

#### L3：行为模板层（CRUD Pattern SoT）

事实源：统一写入模式与 API 契约（One Door + 意图模型）。职责：定义所有写动作共享流程骨架。

- 统一 intent 模型：`create_org/add_version/insert_version/correct/...`
- 统一校验顺序：值决议 -> required -> allowed -> 写入
- 统一错误码映射与用户可见提示口径
- 统一 request_id/trace_id/policy_version/composition_version 协议

> 该层决定“怎么拼”，不决定“拼成哪种业务差异”。

#### L4：策略差异层（Dynamic Policy SoT）

事实源：Strategy Registry（capability/context 生效）。职责：按上下文裁剪行为差异。

- `visible/required/maintainable/default_rule_ref/default_value`
- `allowed_value_codes`（仅表达“最终可选裁剪结果”，不作为字典候选池事实源）
- `priority_mode/local_override_mode`
- 生效区间与版本：`effective_date/end_date/policy_version`

> 该层决定“在当前 capability/context 下允许拼成什么样”。

### 5.1A `allowed_value_codes` 语义收敛（对齐 SetID × Dict 规范）

1. [ ] **定位冻结**：`allowed_value_codes` 仅是运行时“最终可选集”结果，不是候选池主事实；候选池唯一事实源仍是 L2（dict registry + dict values）。
2. [ ] **SetID 前置冻结**：必须先执行 `ResolveSetID(tenant, as_of, org_unit_id|business_unit_id, capability_key)`，得到 `resolved_setid` 后才允许查询候选；禁止“查完再猜 setid”。
3. [ ] **分层求值冻结**：集合求值采用“先层级后优先级”——先按 `priority_mode` 形成 `custom/DEFLT/SHARE` 层顺序，再由 `local_override_mode` 决定是否允许 local 补充/覆盖。
4. [ ] **租户边界冻结**：字典读取保持 tenant-only；缺基线/未命中必须 fail-closed（不允许 global fallback）。
5. [ ] **一致性不变量**：`allowed_value_codes ⊆ L2 候选池`；若字段 `required=true` 且为 DICT，不允许最终可选集为空；`default_value` 非空时必须命中最终可选集。
6. [ ] **可解释性冻结**：字段级决策必须可回显 `resolved_setid + setid_source + winner_policy_ids + resolution_trace`，便于排障与审计。

### 5.2 策略冲突决议算法（冻结）

1. [ ] **候选过滤**：按 `tenant + capability_key + intent + as_of(+setid/+business_unit)` 过滤可用策略；空集即 `policy_missing`。
2. [ ] **分桶决议**：先按“场景覆盖桶 > 基线桶、BU 桶 > tenant 桶”确定命中桶，禁止跨桶只按 `priority` 直排导致语义倒挂。
3. [ ] **上下文特异度排序**：在命中桶内按 `setid` 精确 > `business_unit` 精确 > wildcard；仅保留最高特异度分组。
4. [ ] **优先级排序**：在最高特异度分组内按 `priority DESC -> effective_date DESC -> created_at DESC -> policy_id ASC`。
5. [ ] **冲突处理**：
   - 标量语义（`visible/required/maintainable/default_*`）使用第一名策略（winner-takes-first）；
   - 集合语义（`allowed_value_codes`）按 `priority_mode + local_override_mode` 决议（先层级再补充/覆盖），并执行子集校验（必须属于 L2 候选池）。
6. [ ] **一致性阻断**：若 `required=true` 且集合结果为空，或 `default_value` 不在集合内，返回稳定错误码并阻断提交。
7. [ ] **歧义阻断**：若同位冲突仍无法化解，返回 `policy_conflict_ambiguous`。
8. [ ] **证据输出**：输出 `winner_policy_ids + resolution_trace + policy_version + resolved_setid + setid_source`。

## 6. 双流水线设计（运行时组合 + AI 编排）

### 6.1 运行时组合执行流水线

1. **事务与隔离前置**：进入显式事务，注入 tenant/RLS；缺租户上下文立即拒绝。
2. **解析上下文**：`tenant + surface + intent + as_of + (org_unit_id|business_unit_id)`。
3. **映射决议**：读取 `SurfaceIntentCapabilityRegistry`，得到唯一 `capability_key + mapping_version`。
4. **SetID 决议（硬前置）**：执行 `ResolveSetID(...)` 得到 `resolved_setid`；失败直接 fail-closed（不得继续取数）。
5. **读取 L1/L2/L4**：基于 `resolved_setid + as_of` 加载静态积木、tenant-only 候选池、命中策略（含版本）。
6. **组合决议**：结构（visible）、交互（maintainable）、值（default/allowed）一次完成；`allowed_value_codes` 仅作最终裁剪结果输出。
7. **版本快照计算**：生成 `composition_version`（由 L1/L2/L4/mapping 快照指纹计算）。
8. **写入提交**：执行 L3 模板校验与 One Door 提交，强制校验 `policy_version + composition_version`。
9. **输出 explain**：返回命中链路、拒绝原因、最终决策快照（含 `resolved_setid/setid_source`、版本与冲突证据）。

### 6.2 Req2Config 编排流水线（AI 自动化）

1. [ ] **Turn Capture**：接收对话输入，生成 `conversation_id + turn_id + trace_id`。
2. [ ] **Actor Context Bind**：绑定 `actor_id + tenant_id + role_set + authz_snapshot_version`，缺失或不一致即拒绝。
3. [ ] **Intent Parse**：解析 `surface + intent + expected_effect`；槽位不全则返回补充问题。
4. [ ] **Constraint Bind**：绑定现行 SoT（L1/L2/L4 + mapping）。
5. [ ] **Plan Generate**：输出 `ConfigDeltaPlan`（仅声明“改什么”，不声明“怎么写库”）。
6. [ ] **Schema-Constrained Decode**：模型输出阶段即启用严格结构约束（`strict=true`）；不满足 schema 直接拒绝进入 lint/commit。
7. [ ] **Static Lint**：执行命名、范围、No Legacy、capability 映射完整性检查。
8. [ ] **Dry Run Compose**：执行组合快照模拟，输出 `DryRunResult`（diff、错误、性能估计）。
9. [ ] **Risk Classify**：对计划按影响面与写入风险分级（`low/medium/high`），生成 `risk_tier + approval_policy`。
10. [ ] **Human/Policy Confirm**：`high` 强制人工确认；`low/medium` 可按预授权策略确认；无确认令牌禁止提交。
11. [ ] **Authz Gate (Casbin Enforce)**：按 `DEV-PLAN-022` 冻结口径计算并校验 `subject/domain/object/action`（经 `pkg/authz` registry/helper），执行 `Require(...)`；拒绝时统一 403 且 fail-closed。
12. [ ] **User-Equivalent Command Materialize**：将已确认计划编译为“与 UI 提交同构”的标准写入命令（同 `intent/request_id/trace_id/policy_version/composition_version` 契约）。
13. [ ] **Headless Execute**：不依赖页面点击，但必须走与 UI 完全一致的应用服务 -> L3 模板校验 -> One Door 提交流程；禁止 `ai_*` 专用写入路径与旁路提交。
14. [ ] **Evidence Persist**：运行时审计固化 `input/output/explain/version/hash/risk_tier/actor_id`；`docs/dev-records/` 仅用于阶段性 Readiness 证据归档。

### 6.2A 编排运行时可靠性契约（Durable Orchestration）

1. [ ] **Timeout Budget**：为 parse/plan/lint/dry-run/commit 定义独立超时预算，超时 fail-closed 并返回可复算错误码。
2. [ ] **Idempotent Retry**：同一 `conversation_id + turn_id + request_id` 仅允许幂等重试；禁止生成新的隐式计划版本。
3. [ ] **Checkpoint/Resume**：在 `proposed/validated/confirmed` 状态落盘检查点，进程重启后可恢复继续。
4. [ ] **Background Execution**：超过交互时限的 dry-run/评测任务转后台执行，前台仅返回任务句柄与轮询接口。
5. [ ] **Dead Letter & Manual Takeover**：重试耗尽或检查点损坏进入人工接管队列，禁止自动绕过提交。

### 6.3 对话式事务状态机（Conversation Transaction）

状态机冻结：

`draft -> proposed -> validated -> confirmed -> committed`  
`draft/proposed/validated/confirmed -> canceled`  
`draft/proposed/validated/confirmed -> expired`

规则：

1. [ ] 仅 `confirmed` 可进入 `committed`。
2. [ ] `canceled/expired` 为终态，不允许隐式恢复；需新建 turn。
3. [ ] 同一 `conversation_id + turn_id + request_id` 重试必须幂等。
4. [ ] 提交前若版本漂移（`policy_version/composition_version/mapping_version`）则回到 `validated` 并要求重确认。
5. [ ] 跨 turn 合并时必须显式展示聚合 diff，禁止静默覆盖旧意图。

### 6.4 AI 信任边界与安全约束

1. [ ] **Least Privilege**：AI 服务无数据库写权限，仅可调用解析/规划/dry-run 接口。
2. [ ] **Prompt Injection 防护**：用户文本不得直接拼接执行语句；必须结构化解析 + 白名单校验。
3. [ ] **PII 最小化**：对话日志默认脱敏；审计保存哈希与结构化摘要。
4. [ ] **Explain 必达**：每次计划与提交都返回 machine-readable explain。
5. [ ] **Fail-Closed**：解析失败、约束不全、冲突未解、确认缺失均拒绝提交。
6. [ ] **Tool Permission Matrix**：按 `capability_key + risk_tier` 绑定可调用工具白名单，禁止未注册工具与参数越权。
7. [ ] **Connector/Egress 限制**：外部连接器按租户与用途隔离，默认禁止任意外联与跨租户数据拼接。
8. [ ] **No AI Principal**：AI 不作为独立授权主体；所有敏感操作必须落在操作者权限上下文内执行。
9. [ ] **No AI Write Bypass**：AI 不得拥有独立业务写接口；仅允许通过“用户等价命令”进入与 UI 同构的提交链路。
10. [ ] **User-Equivalent Outcome**：同一 `actor + intent + input` 在 AI 与 UI 两入口必须得到一致的 allow/deny、错误码与版本冲突判定。
11. [ ] **Unified Forbidden Contract**：授权拒绝统一走全局 403/responder；响应体不回显 `subject/domain/object/action`，但日志必须记录缺口诊断字段。

### 6.5 编排引擎决策事项（自建 Temporal，Go SDK）

决策冻结：

1. [ ] `Req2Config` 编排流水线（6.2 + 6.2A）采用 **自建 Temporal（Go SDK）** 作为工作流引擎。
2. [ ] 运行时组合流水线（6.1）**不迁移到 Temporal**，继续保持现有同步事务链路与 One Door 提交。
3. [ ] Temporal 仅承载“编排与状态机”，不承载业务授权裁决；授权仍由 Casbin + RLS + One Door 内核裁决。
4. [ ] Workflow 业务主键冻结：`conversation_id + turn_id + request_id`，用于幂等重试与恢复。
5. [ ] 强制启用 Workflow 版本治理（变更标记/兼容窗口），禁止无版本策略直接发布导致非确定性回放失败。

自建边界（最小可用）：

1. [ ] 环境隔离：按环境独立 Namespace/集群边界，禁止测试流量进入生产 Namespace。
2. [ ] 安全基线：启用 mTLS、最小化服务账户权限、审计日志保留与密钥轮换。
3. [ ] 持久化基线：Temporal 元数据与历史库高可用部署，RPO/RTO 与平台基线对齐。
4. [ ] 可观测基线：队列积压、任务失败率、Workflow 超时率、重试耗尽率纳入告警。
5. [ ] 灾备演练：按季度执行恢复演练（含 checkpoint 恢复与 dead-letter 人工接管路径）。

## 7. 数据与接口契约

1. [ ] 定义 `PageCompositionSnapshot`（L1+L2）DTO，显式包含 `l1_snapshot_hash + l2_snapshot_hash`。
2. [ ] 定义 `IntentDecisionSnapshot`（L4）DTO，显式包含 `policy_version + winner_policy_ids`。
3. [ ] 定义 `MappingSnapshot` DTO，显式包含 `mapping_version`。
4. [ ] 定义 `ComposedFieldDecision` DTO，字段级输出必须带 `source_layer + resolved_setid + setid_source + winner_policy_ids`。
5. [ ] 定义 `AllowedValueDecision` DTO：`candidate_pool_hash + priority_mode + local_override_mode + allowed_value_codes + filtered_out_codes`。
6. [ ] 冻结 `composition_version` 计算：`hash(l1_snapshot_hash, l2_snapshot_hash, policy_version, mapping_version)`。
7. [ ] 统一写入请求携带 `intent + policy_version + composition_version + resolved_setid`；缺失、过期、冲突均拒绝。
8. [ ] 冻结“候选池 vs 最终可选”二段式语义，禁止 UI 合并为单语义。
9. [ ] 冻结“主数据先 ResolveSetID 再取数”的接口契约；候选接口必须回显 `resolved_setid`（或每项 `setid`）。
10. [ ] 定义 `RequirementIntentSpec`：`conversation_id + turn_id + tenant + surface + intent + constraints + expected_outcome`。
11. [ ] 定义 `ConfigDeltaPlan`：仅允许表达 L1/L2/L4 变更提案，禁止 SQL/表名/未注册 capability。
12. [ ] 定义 `DryRunResult`：`composed_diff + validation_errors + estimated_queries + estimated_tx + would_commit=false`。
13. [ ] explain 结构冻结：`input_context + matched_records + resolution_trace + final_decisions + versions + resolved_setid`。
14. [ ] 定义 `PlanRiskAssessment`：`risk_tier + impacted_surface_count + impacted_field_count + requires_human_confirm`。
15. [ ] 定义 `OrchestrationCheckpoint`：`conversation_state + schema_version + resume_token + expires_at`。
16. [ ] 定义 `AsyncTaskReceipt`：`task_id + task_type + submitted_at + status + poll_uri`（用于后台 dry-run/评测）。
17. [ ] 定义 `ActorAuthSnapshot`：`actor_id + tenant_id + role_set + authz_snapshot_version + delegated_by_ai=true`。

### 7.1 错误码冻结

1. [ ] 组合/策略错误：`mapping_missing`、`mapping_ambiguous`、`policy_missing`、`policy_conflict_ambiguous`、`policy_version_conflict`、`composition_version_conflict`、`allowed_value_out_of_pool`、`allowed_value_required_empty_forbidden`、`default_value_not_in_allowed`。
2. [ ] SetID/上下文错误：`SETID_BINDING_MISSING`、`SETID_NOT_FOUND`、`SETID_DISABLED`、`CAPABILITY_CONTEXT_MISMATCH`。
3. [ ] 字典边界错误：`dict_baseline_not_ready`（tenant-only 下缺基线）、`dict_not_found`、`dict_value_not_found_as_of`。
4. [ ] 对话事务错误：`CONVERSATION_TURN_INCOMPLETE`、`CONVERSATION_STATE_INVALID`、`CONVERSATION_CONFIRMATION_REQUIRED`、`CONVERSATION_VERSION_DRIFT`。
5. [ ] AI 边界错误：`AI_PLAN_SCHEMA_INVALID`、`AI_PLAN_SCHEMA_CONSTRAINED_DECODE_FAILED`、`AI_PLAN_BOUNDARY_VIOLATION`。
6. [ ] 编排运行时错误：`ORCHESTRATION_TIMEOUT`、`ORCHESTRATION_RETRY_EXHAUSTED`、`ORCHESTRATION_CHECKPOINT_CORRUPTED`、`ORCHESTRATION_ASYNC_TASK_FAILED`。
7. [ ] 委托授权错误：`AI_ACTOR_CONTEXT_MISSING`、`AI_ACTOR_AUTHZ_DENIED`、`AI_ACTOR_ROLE_DRIFT_DETECTED`。
8. [ ] Temporal 引擎错误：`TEMPORAL_WORKFLOW_NON_DETERMINISTIC`、`TEMPORAL_TASK_QUEUE_BACKLOG_LIMIT`、`TEMPORAL_NAMESPACE_UNAVAILABLE`。

## 8. 性能与缓存策略

### 8.1 性能调查发现（2026-02-28）

1. 字段配置页存在按字段逐次决议的 N+1 风险。  
2. Strategy 决议当前为“单次决议 = 一次事务 + 一次查询”形态。  
3. 字段选项接口为逐字段读取，且字典读取链路默认开事务。  
4. create 链路当前字段数较少，短期可控，扩展后会线性放大。  
5. 现有索引可支撑 key 过滤，但不自动解决应用层 N+1。

### 8.2 结论与停止线

1. [ ] 单次“页面组合快照”请求：事务次数 `<= 3`。
2. [ ] 单次“页面组合快照”请求：SQL 查询数 `<= 10`。
3. [ ] 禁止主加载链路出现“按字段逐个调用策略决议接口”。
4. [ ] 策略决议必须支持批量输入（`field_key IN (...)`）与单次返回。
5. [ ] 字典候选值采用懒加载；若预取必须按 `dict_code` 批量聚合。
6. [ ] 双版本校验额外成本：P95 增量 `<= 5ms`。
7. [ ] 在 `docs/dev-records/` 固化压测证据：P50/P95、QPS、查询计数、事务计数。

### 8.3 缓存工具链默认方案（冻结）

1. [x] 默认缓存工具链：**Go 原生 + pgx + PostgreSQL**。
2. [x] 优先级：request-scope 复用 > 进程内短 TTL > PostgreSQL 回源；先原生、后扩展。
3. [x] `Redis` / `Ristretto` / `BigCache` 等外部缓存库不作为默认依赖。
4. [x] 仅当停止线无法满足且证据完备时，才可申请启用外部缓存。
5. [x] 启用外部缓存前置条件：用户审批 + `docs/dev-plans/` 契约更新 + 一致性/失效/回退评审。

## 9. 门禁、验收与证据

按 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 执行，不在本文复制命令矩阵。

### 9.1 门禁映射（可执行化）

| 约束 | 门禁/测试 | 证据归档 |
| --- | --- | --- |
| Surface/Intent 映射唯一且完整 | `make check capability-route-map` + 映射注册表一致性测试（M3） | `docs/dev-records/dev-plan-200-m3-mapping-registry-evidence.md` |
| 策略冲突决议 deterministic | 单元测试（冲突矩阵）+ 集成回放测试 | `docs/dev-records/dev-plan-200-m0-policy-resolution-evidence.md` |
| SetID 前置决议（先解析再取数） | 集成测试：候选接口未先 `ResolveSetID` 直接失败；回显 `resolved_setid/setid_source` | `docs/dev-records/dev-plan-200-m2-setid-pre-resolve-evidence.md` |
| `allowed_value_codes` 集合语义收敛 | 单元测试：`priority_mode + local_override_mode` 组合矩阵；`required/default` 一致性阻断 | `docs/dev-records/dev-plan-200-m0-allowed-value-semantics-evidence.md` |
| 字典 tenant-only 边界 | 集成测试：缺基线 `dict_baseline_not_ready`；未命中不回退 global | `docs/dev-records/dev-plan-200-m2-dict-tenant-only-evidence.md` |
| No Tx, No RLS（组合链路） | 集成测试：缺 tenant/缺 tx 必须 fail-closed | `docs/dev-records/dev-plan-200-m5-tx-rls-evidence.md` |
| 跨层版本一致性（TOCTOU 阻断） | 写入冲突测试：`policy_version/composition_version` 过期拒绝 | `docs/dev-records/dev-plan-200-m5-version-consistency-evidence.md` |
| 禁止 legacy 回退 | `make check no-legacy` | `docs/dev-records/dev-plan-200-m6-cutover-evidence.md` |
| 性能停止线 | 查询/事务计数回归 + 压测 | `docs/dev-records/dev-plan-200-m8-performance-evidence.md` |
| AI 计划产物边界（不直写库） | `ConfigDeltaPlan` schema 校验 + 静态 lint（禁止 SQL/未注册 key） | `docs/dev-records/dev-plan-200-m9-ai-plan-boundary-evidence.md` |
| AI 严格结构化输出（constrained decode） | 严格 schema 解码测试（非法字段/缺字段/类型错误必须拒绝） | `docs/dev-records/dev-plan-200-m9a-ai-constrained-decode-evidence.md` |
| 编排可靠性（超时/重试/恢复） | 编排运行时测试：timeout、重试幂等、checkpoint 恢复 | `docs/dev-records/dev-plan-200-m10a-orchestration-durability-evidence.md` |
| AI 授权主体一致性（代操作者执行） | 授权回归测试：AI 代系统配置管理员/HR/普通员工/经理执行，结果与人工直操一致 | `docs/dev-records/dev-plan-200-m10b-actor-delegated-authz-evidence.md` |
| Casbin 工具链与执行顺序冻结 | `make authz-pack && make authz-test && make authz-lint` + 集成测试：`Actor Bind -> MapRouteToObjectAction -> Require -> One Door` 顺序不可旁路 | `docs/dev-records/dev-plan-200-m10b1-authz-toolchain-sequence-evidence.md` |
| AI/UI 等价执行一致性 | 同 actor、同 intent、同输入下，AI 编排与 UI 提交的 allow/deny、错误码、版本冲突结果必须一致 | `docs/dev-records/dev-plan-200-m10c-ai-ui-equivalent-execution-evidence.md` |
| 自建 Temporal 平台可用性 | 演练测试：worker 滚动升级、history 回放、任务队列积压告警、死信人工接管与灾备恢复 | `docs/dev-records/dev-plan-200-m10d-self-host-temporal-evidence.md` |
| 评测回归门禁（planner 质量） | 固定样本集评测 + 回归阈值（准确率/拒绝率/误放行率） | `docs/dev-records/dev-plan-200-m11-eval-gate-evidence.md` |
| 对话式事务一致性 | 会话状态机测试 + 幂等/重放测试（同 `request_id` 仅一次提交） | `docs/dev-records/dev-plan-200-m10-conversation-transaction-evidence.md` |

> 说明：若现有 CI 无对应检查项，由 M3/M5/M8/M9/M9A/M10/M10A/M10B/M10B1/M10C/M10D/M11 增补自动化门禁并接入 `make preflight`。

### 9.2 验收标准

1. [ ] 任一字段语义仅存在一个主写层，不再出现跨层双写。
2. [ ] 页面可见差异由组合结果驱动，不再由页面分支硬编码。
3. [ ] create/add/insert/correct 使用同一行为模板，差异仅由策略表达。
4. [ ] `allowed_value_codes` 始终是字典候选池子集，且集合求值遵循 `priority_mode + local_override_mode`（先层级后优先级）。
5. [ ] 字段级输出可见 `resolved_setid + setid_source`，并与候选接口回显一致。
6. [ ] 主数据候选链路全部满足“先 ResolveSetID 再取数”；不存在“查完再猜 setid”。
7. [ ] tenant-only 边界成立：缺基线返回 `dict_baseline_not_ready`，且不回退 global。
8. [ ] 当字段 `required=true` 且为 DICT 时，最终可选集不得为空；`default_value` 非空时必须命中最终可选集。
9. [ ] `surface+intent` 到 `capability_key` 命中唯一映射；缺失/歧义均 fail-closed。
10. [ ] 缺策略/缺上下文/版本冲突均 fail-closed，错误码稳定且可解释。
11. [ ] 写入提交必须通过 `policy_version + composition_version` 双版本校验，阻断 TOCTOU。
12. [ ] 质量门禁通过，且无 legacy 回退路径。
13. [ ] 组合快照满足性能预算与停止线，无字段级 N+1 查询回流。
14. [ ] AI 仅输出 `RequirementIntentSpec/ConfigDeltaPlan`，无直写数据库能力；越权产物被门禁拒绝。
15. [ ] 对话式事务遵循状态机，未 `confirmed` 的计划不得提交；取消后不得隐式恢复。
16. [ ] 同一 `conversation_id + turn_id + request_id` 重试幂等，提交结果可回放且审计可追溯。
17. [ ] AI 计划生成阶段启用严格 schema 约束（`strict=true`），非法结构在进入 lint 前即被拒绝。
18. [ ] 编排链路具备超时预算、检查点恢复与后台任务能力；重试耗尽进入人工接管，不得自动提交。
19. [ ] 计划评测门禁通过（固定回归集），关键质量指标在阈值内且证据已归档。
20. [ ] AI 编排权限与操作者完全一致：系统配置管理员、HR 专业用户、普通员工、经理四类角色均通过“AI 代操=人工直操”一致性验证。
21. [ ] AI 与 UI 提交路径保持同构：不新增 `ai_*` 业务写语义；同输入下授权结果、错误码与版本冲突判定一致。
22. [ ] 自建 Temporal 仅用于编排链路（6.2/6.2A），6.1 运行时组合链路不迁移；边界与回归证据齐备。
23. [ ] 自建 Temporal 通过平台可用性验收：滚动升级、回放兼容、队列积压告警、死信接管、灾备恢复可演练。
24. [ ] AI 编排授权执行顺序冻结并可验证：`Actor Context Bind -> MapRouteToObjectAction -> authz.Require(enforce) -> One Door`；任何旁路均被门禁阻断。

## 10. 实施里程碑（按依赖顺序）

1. [ ] **M0 契约冻结补丁**：冲突决议算法、`allowed_value_codes` 集合语义、跨层版本协议、映射注册表契约冻结。
2. [ ] **M1 术语与边界冻结**：Surface/Intent 与四层 SoT 文档冻结。
3. [ ] **M2 组合 DTO 落地**：新增组合快照/决策对象与只读接口（批量决议），补齐 `resolved_setid/setid_source` 回显。
4. [ ] **M3 映射注册表接入**：运行时强制映射决议，阻断未注册 `surface+intent`。
5. [ ] **M4 页面收敛**：字段配置页仅静态主写；策略页仅动态主写；字典页仅候选池主写。
6. [ ] **M5 CRUD 模板收口**：统一提交流程并接入双版本校验，固化“先 ResolveSetID 再取数”执行门禁。
7. [ ] **M6 切换演练与上线切换**：仅做单次切换，不保留 runtime legacy 回退路径。
8. [ ] **M7 explain 与审计闭环**：补齐组合决议可解释证据。
9. [ ] **M8 门禁与性能收口**：阻断双写回流、语义跨层污染与 N+1 回流。
10. [ ] **M9 Req2Config 编排链路**：落地 `RequirementIntentSpec -> ConfigDeltaPlan -> DryRunResult` 只读闭环。
11. [ ] **M9A 严格结构化输出**：落地 constrained decode（strict schema）与拒绝路径证据。
12. [ ] **M10 对话式事务提交**：落地会话状态机（draft/proposed/validated/confirmed/committed），接入双版本校验与 One Door 提交。
13. [ ] **M10A 编排可靠性**：落地 timeout/retry/checkpoint/background 运行时能力与人工接管。
14. [ ] **M10B 委托授权收口**：落地 `ActorAuthSnapshot`、角色一致性回归与“AI 非独立主体”门禁。
15. [ ] **M10B1 Casbin 工具链收口**：冻结 `Actor Bind -> MapRouteToObjectAction -> Require -> One Door` 执行顺序，并接入 `make authz-pack/authz-test/authz-lint`。
16. [ ] **M10C AI/UI 同构收口**：落地“用户等价命令”入口与同 actor 一致性回归门禁。
17. [ ] **M10D 自建 Temporal 平台化**：落地集群、Namespace 隔离、worker 版本治理、可观测与灾备演练。
18. [ ] **M11 评测与审批分级门禁**：落地 planner 回归评测与 `risk_tier` 驱动确认策略。

## 11. 迁移与切换剧本（No Legacy）

1. [ ] **R0 基线盘点**：盘点现有页面/接口路径，识别所有 `surface+intent` 与现行 capability 映射。
2. [ ] **R1 只读对照（测试环境）**：新组合链路仅用于对照与证据，不作为线上回退路径。
3. [ ] **R2 数据与契约补齐**：补齐映射注册表、冲突决议参数与版本字段，完成历史数据修复。
4. [ ] **R3 预发验收**：按第 9 节门禁与验收标准完成全量验证。
5. [ ] **R4 单次切换上线**：按发布窗口切换到组合链路，不保留运行时双链路。
6. [ ] **R5 下线旧路径**：删除旧调用入口与兼容分支，并以门禁阻断回流。
7. [ ] **失败处置**：仅允许环境级保护（只读/停写/修复后重试），不允许启用 legacy 兜底。

## 12. 风险与缓解

1. **概念迁移成本高**：通过术语冻结与页面来源标识降低认知切换成本。
2. **历史数据不一致**：先只读镜像与差异巡检，再执行分批迁移与阻断门禁。
3. **策略扩散到 UI**：以组合 DTO 单点出参约束前端，只消费结果不重算规则。
4. **能力映射漂移**：以映射注册表 + 强制门禁阻断未注册路径。
5. **组合层性能退化**：通过事务/查询预算、压测与查询计数门禁提前阻断。
6. **版本漂移引发 TOCTOU**：通过双版本校验阻断。
7. **冲突算法实现偏差**：以冲突矩阵测试 + explain 回放证据校准实现。
8. **AI 幻觉导致错误配置提案**：通过 schema 校验、静态 lint、dry-run 与人工确认四级拦截。
9. **多轮对话语义漂移**：通过状态机 + turn 级 diff 展示 + 版本漂移重确认阻断。
10. **编排长尾故障导致挂起**：通过超时预算、checkpoint 恢复、后台任务与人工接管收口。
11. **低质量计划回流生产**：通过固定评测集与回归阈值门禁，阻断质量退化版本发布。
12. **AI 隐式提权或角色漂移**：通过 Actor 绑定、提交前实时授权复核与角色漂移检测 fail-closed。
13. **同 code 跨 SetID 语义混淆**：通过字段级 `resolved_setid/setid_source` 可解释输出与集合语义门禁阻断误判。
14. **自建 Temporal 运维复杂度上升**：通过边界限域（仅 6.2/6.2A）、SRE 基线、演练门禁与容量预算收口。
15. **Workflow 代码演进导致非确定性回放失败**：通过 worker 版本治理、兼容发布与回放测试阻断。

## 13. 关联文档

- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- `docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md`
- `docs/dev-plans/183-capability-key-object-intent-discoverability-and-modeling.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- `docs/dev-plans/109a-request-code-total-convergence-and-anti-drift.md`
- `docs/dev-plans/140-error-message-clarity-and-gates.md`
- `docs/dev-plans/155-capability-key-m3-evaluation-context-cel-kernel.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/182-bu-policy-baseline-and-intent-override-unification.md`
- `docs/dev-plans/185-field-config-dict-values-setid-column-and-master-data-fetch-control.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/105-dict-config-platform-module.md`
- `docs/dev-plans/105b-dict-code-management-and-governance.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
