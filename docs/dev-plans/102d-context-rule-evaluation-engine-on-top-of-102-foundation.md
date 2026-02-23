# DEV-PLAN-102D：基于 102 基线的 Context + Rule + Eval 动态隔离与配置安全实施方案

**状态**: 草拟中（2026-02-22 12:50 UTC）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102/102B/102C` 的延续方案，目标是在既有“显式时间上下文 + tenant-only 运行时 + 能力差距评估”基础上，落地 **Context + Rule + Eval** 规则执行内核。
- 本计划聚焦“运行时机制与验收口径”，不替代 `070B` 的迁移责任，也不回退到 legacy 双链路。
- **定位冻结**：102D 是“总装编排 PoR”，负责执行顺序、跨计划集成与统一验收；不覆盖 102C1/102C2/102C3 的细则 SSOT。
- 冲突优先级：
  1. 时间口径以 `DEV-PLAN-102B`/`STD-002` 为准；
  2. tenant-only 与发布路径以 `DEV-PLAN-070B` 为准；
  3. 能力目标与差距优先级以 `DEV-PLAN-102C` 为准；
  4. 授权上下文与拒绝码以 `DEV-PLAN-102C1` 为准；
  5. 个性化注册表字段以 `DEV-PLAN-102C2` 为准；
  6. explain 字段与分级暴露以 `DEV-PLAN-102C3` 为准；
  7. 本计划仅补充“如何把上述契约在 Go + PostgreSQL + CEL 中编排落地”。

## 1. 背景与问题陈述
- `102B` 已完成“`as_of/effective_date` 显式必填”收敛，回放稳定性有了统一时间语义。
- `102C` 已识别关键差距：上下文化安全、BU 个性化策略注册、命中可解释性。
- 当前缺口：仍缺少一个统一、可复用的“规则执行内核”，将组织上下文、角色约束、业务资格过滤、冲突决议收敛到同一执行框架。
- 目标是借鉴 Workday 原则：**不依赖 SetID 作为运行时唯一过滤器**，而是以上下文求值实现动态隔离和配置决策。

## 2. 目标与非目标
### 2.1 核心目标
- [ ] 建立统一 `EvaluationContext` 契约（Worker/User/Org/System/Time），作为 API 与规则引擎之间的稳定边界。
- [ ] 建立 CEL 规则执行链路：解析、编译缓存、执行、冲突决议、解释输出（explainability）。
- [ ] 建立“硬隔离 + 软规则”双层模型：RLS 负责硬边界，CEL 负责资格与可见性细分。
- [ ] 在至少 2 个业务场景落地（建议：配置下拉过滤、策略命中决策），形成端到端闭环。
- [ ] 输出门禁与测试契约，阻断“隐式默认 today / 绕过 RLS / 未解释命中”的回漂。
- [ ] 冻结跨计划职责边界，避免与 102C1/102C2/102C3 形成并行或冲突契约。

### 2.2 非目标
- 不重做 `070B` 已覆盖的“global_tenant 退出与发布链路”。
- 不引入 feature flag 双实现、fallback 读链路或兼容别名窗口（遵循 No Legacy）。
- 不在本计划直接扩展到所有业务域；先完成样板域，再按模块复制。
- 不改变 `102B` 已冻结的时间参数语义（只复用，不重新定义）。
- 不重写 102C1/102C2/102C3 已冻结的字段清单、错误码、解释分级与治理流程。

## 3. 架构原则（延续 102 基线）
1. **Time Explicitness First**：所有规则评估必须接收显式 `as_of`/`effective_date`。
2. **No Tx, No RLS**：访问业务表必须显式事务 + 租户注入，缺失即 fail-closed。
3. **One Door**：写入仍走 DB Kernel 事件入口；规则引擎只负责决策，不绕过写入契约。
4. **Deterministic Replay**：同一 `(tenant, context, rule_version, date)` 在不同执行日得到相同业务结果。
5. **Explainability by Default**：每次命中需输出“命中链路”而非黑盒结果。

## 4. 目标架构（Go + PostgreSQL + CEL）
### 4.1 执行链路
1. API/Service 构建 `EvaluationContext`（含组织路径、操作者安全组、业务对象属性、显式日期），并执行“客户端输入最小化 + 服务端权威回填”。
2. Repository 按 `tenant_id + module + status + effective_date` 粗筛候选规则。
3. Rule Engine 执行 CEL（编译缓存 + 程序缓存），得到 `true/false` 与可解释中间结果。
4. Resolver 按固定排序（`priority ASC, effective_date DESC, rule_id ASC`）决议最终命中。
5. 输出业务结果 + explain payload（命中规则、拒绝原因、时间上下文、策略版本）。

### 4.2 数据层设计（草案）
> 说明：以下为候选模型，进入实施前若涉及新建表，需先获得用户确认。

- 规则定义：`cfg_rule_definitions`
  - 关键字段：`tenant_id`, `module`, `rule_code`, `cel_expression`, `priority`, `status`, `version`, `effective_date`, `expired_date`
- 规则绑定：`cfg_rule_bindings`
  - 关键字段：`rule_id`, `target_type`, `target_key`, `scope_path`, `weight`
- 解释快照：`cfg_rule_eval_explains`
  - 关键字段：`request_id`, `tenant_id`, `rule_id`, `eval_context_hash`, `eval_result`, `explain_json`, `as_of_date`

### 4.3 组织层级与路径
- 组织层级优先复用现有组织模型；若需要路径索引，采用 `ltree` + GiST/GIN 索引。
- 路径片段使用稳定标识（不可变 code/UUID），避免组织改名导致规则语义漂移。
- 所有路径匹配必须受租户边界约束，禁止跨租户路径命中。

### 4.4 安全模型分层
- **硬边界（DB）**：RLS 保证“租户可见范围”与“组织基础边界”。
- **软边界（Engine）**：CEL 在硬边界内做资格过滤（如国家、岗位、人员类型、BU 上下文）。
- 任一层失败即拒绝，且统一输出可审计错误码（fail-closed）。

### 4.5 CEL 运行时规范
- 变量命名冻结：`worker`, `user`, `org`, `system`, `request`。
- 禁止非确定性函数（如当前时间、随机数、外部 I/O）。
- 自定义函数白名单（示例）：`is_subordinate(pathA, pathB)`, `in_group(user, group)`。
- 编译/执行错误不回退默认命中；直接返回规则无效并记录审计。

### 4.6 上下文可信边界（冻结）
- 客户端可传（最小业务参数）：`module`, `target_id/target_type`, `as_of/effective_date`。
- 服务端权威回填（禁止客户端直传覆盖）：`tenant_id`, `actor_id`, `actor_scope`, `roles/groups`, `business_unit_id`, `setid`, `capability_key`。
- 若客户端传入与服务端推导冲突的上下文字段，统一 fail-closed 并记录审计。
- 对外业务接口不得暴露“可任意传入完整 context”的通用能力。

### 4.7 capability_key 机制冻结（防止退化为 scope）
1. `capability_key` 只表达“业务能力动作”（做什么），不表达“上下文切片”（在哪个租户/BU/SetID/国家）。
2. `capability_key` 不是隔离键；隔离始终由 `tenant + setid + business_unit + as_of` 等上下文字段承担。
3. 同一 `capability_key` 在不同 `setid/BU` 的差异通过策略数据实现，不通过拼接 key 变体实现。
4. 业务专用 API 默认不要求前端手填 `capability_key`，由服务端按“路由+动作”映射推导；仅内部治理入口允许显式录入。
5. explain/audit 必须保留 `capability_key` 作为“判定对象锚点”，但不得把 `setid/BU` 编入 key。

### 4.8 capability_key 命名规范（冻结）
- 语法：`<module>.<capability>[.<action>]`，全部小写字母/数字/下划线，段间用 `.`。
- 合法示例：`staffing.assignment_create`、`comp.allowance_select`、`jobcatalog.profile_defaults`。
- 非法示例：`staffing.assignment_create.bu_a`、`jobcatalog.setid_s2601`、`comp.allowance.cn`。
- 命名禁词（作为上下文变量，禁止出现在 key）：`setid`、`bu`、`scope`、`tenant`、地区/国家代码、组织编码。

## 5. API 与契约（首批样板）
### 5.1 规则命中查询
- [ ] `POST /internal/rules/evaluate`（**第一阶段仅内部/BFF 使用**）
  - 输入：`capability_key`, `module`, `target`, `as_of/effective_date`（上下文由服务端回填，不接受客户端直传完整 context）
  - 输出：`matched_rules[]`, `selected_rule`, `explain`
- [ ] 对外优先采用业务专用接口（如 allowances/comp-plans），不直接暴露通用规则评估入口。

### 5.2 业务资格过滤（示例）
- [ ] `GET /api/allowances?target_worker_id=...&as_of=YYYY-MM-DD`
  - 行为：后端推导 `capability_key` 后加载候选津贴规则，执行 CEL，返回可选项。
- [ ] `GET /api/comp-plans?worker_id=...&as_of=YYYY-MM-DD`
  - 行为：后端推导 `capability_key` 后按优先级返回首个命中或命中集合（按场景配置）。

### 5.3 错误码口径
- 缺失时间参数：沿用 `102B`（`invalid_as_of` / `invalid_effective_date`）。
- 授权与上下文拒绝码：沿用 `102C1`（不在 102D 重定义）。
- 个性化注册表相关键与模式：沿用 `102C2`（不在 102D 重定义）。
- explain 分级与内部告警码：沿用 `102C3`（`brief/full` 与可见范围由 102C3 冻结）。
- 仅引擎内部保留实现细节错误（如编译失败），对外映射遵循上述 SSOT 口径。

### 5.4 外部开放前置条件（未来阶段）
- 仅当 102C1/102C2/102C3 已落地并通过验收，方可评估外部开放通用评估能力。
- 若开放，必须维持“客户端最小参数 + 服务端权威回填”模型，不开放完整 context 直传。
- 对外返回默认 `brief explain`，`full explain` 仅限审计/管理员场景并走显式授权。

### 5.5 capability_key 注册与映射（新增冻结）
- 新能力必须先在策略注册表登记 `capability_key`，再进入编码与联调（承接 102C2）。
- 服务端维护“路由/动作 -> capability_key”单点映射表；禁止分散在 handler 中硬编码字符串。
- 若路由映射缺失或重复映射冲突，统一 fail-closed（`AUTHZ_CONTEXT_POLICY_MISSING` 或等价冻结码）。
- 禁止运行时拼接 `capability_key`（例如 `capability_key + "_" + setid`）。

### 5.6 用户可见性与可操作交付（冻结）
- 本计划的用户可见性交付由业务专用页面承载，不新增“仅后端可用但无入口”的长期能力。
- 第一阶段必须至少提供 1 条可发现入口（导航/页面按钮）触发规则决策能力，候选：
  - `allowances` 页面：在“津贴选择”下拉中应用规则过滤；
  - `comp-plans` 页面：在“薪酬方案选择”中应用规则命中。
- UI 入口需满足：
  - 可发现：页面路由可进入，入口文案与权限可见性明确；
  - 可操作：用户可实际提交并得到规则筛选结果；
  - 可解释：失败/拒绝时显示与 102C1/102C3 对齐的简版原因信息（`brief explain`）。
- 若阶段内只能后端先行，必须同步提供页面占位或已接入路由，并在验收中标记“上线可见入口”。

## 6. 实施步骤与里程碑
1. [ ] **M1 契约冻结**：冻结 `EvaluationContext` 字段、规则排序规则、错误码、explain 输出结构，并冻结“路由/动作 -> capability_key”映射口径。
   - 备注：字段与错误码细则以 102C1/102C2/102C3 为准，102D 只冻结编排顺序与集成接口。
2. [ ] **M2 数据层准备**：完成规则元数据 DDL 评审（含索引、RLS、归档策略）与注册表/映射持久化方案评审；涉及新表前提交人工确认。
3. [ ] **M3 引擎内核**：实现 CEL 编译缓存、执行器、冲突决议器、解释器；落地单元测试。
4. [ ] **M4 样板场景接入**：至少接入 2 条业务链路（资格过滤 + 命中决策），并通过业务专用接口对外提供能力。
5. [ ] **M5 用户入口接入**：至少 1 个页面入口完成接入（路由/按钮/表单），用户可完成一次规则驱动操作。
6. [ ] **M6 门禁与观测**：接入 `make check` 级门禁（规则 lint、确定性回放、命中解释必填）与用户链路回归测试。
7. [ ] **M7 验收收口**：完成评分卡回填（承接 `102C`），并形成 `dev-records` 证据。

## 7. 测试与覆盖率
- 覆盖率口径：沿用仓库现行 Go 覆盖率门禁口径（引用 `Makefile`/CI SSOT）。
- 必测项：
  - [ ] 编译缓存命中/失效测试（表达式版本变化后自动失效）。
  - [ ] 同上下文跨日期重放一致性测试（显式日期相同则结果一致）。
  - [ ] RLS + CEL 双层拒绝路径测试（任一层拒绝都不可放行）。
  - [ ] 冲突规则确定性排序测试（优先级、同级 tie-break）。
  - [ ] explain 输出完整性测试（必须包含 rule_id、version、as_of、decision_reason）。
  - [ ] UI 端到端样板测试：用户从页面入口完成“查询候选 -> 规则过滤 -> 提交/确认”至少 1 条成功链路。
  - [ ] UI 拒绝样板测试：规则拒绝时页面可展示 `brief explain` 且不泄露 `full explain` 敏感信息。
  - [ ] capability_key 反漂移测试：禁止 key 包含上下文变量（setid/bu/scope/tenant 等）与动态拼接。

## 7.1 反漂移门禁（capability_key 专项）
- [ ] 新增静态检查：阻断 `capability_key` 字面量包含上下文禁词。
- [ ] 新增代码检查：阻断运行时字符串拼接生成 `capability_key`。
- [ ] 新增注册一致性检查：业务路由映射到的 `capability_key` 必须在注册表存在且唯一。
- [ ] 新增契约检查：对外/内部 API、服务层、SQL 迁移中不得新增 `scope_code/scope_package` 语义入口。

## 8. 风险与缓解
- **R1：规则执行性能抖动**
  - 缓解：DB 粗筛 + CEL 精筛；表达式编译缓存；热点规则预热。
- **R2：规则可写但不可解释**
  - 缓解：将 explain 结构设为强制输出字段，缺失即测试失败。
- **R3：规则绕过硬隔离**
  - 缓解：把 RLS 置于 CEL 前置条件；服务层禁止直接访问未注入租户上下文的查询。
- **R4：与既有 SetID 能力语义冲突**
  - 缓解：第一阶段仅在 tenant-only 基线内增量接入，不重写既有发布链路。

## 9. 验收标准
- [ ] 能在同租户跨 BU 场景下稳定复现“不同上下文命中不同策略”，且解释链路可读。
- [ ] 所有样板接口在缺失显式时间参数时 fail-closed，不存在默认 today。
- [ ] 规则冲突行为可预测（同输入命中稳定），并有审计证据可追踪。
- [ ] 不引入 legacy 分支，不新增双链路读写入口。
- [ ] 文档、测试、实现三者口径一致，并通过 `make check doc` 与对应质量门禁。
- [ ] 102D 与 102C1/102C2/102C3 无冲突定义；评审可明确“单一细则来源”。
- [ ] 通用规则评估能力在第一阶段仅内部可用，不暴露可伪造上下文的外部入口。
- [ ] 至少 1 条用户可见、可操作端到端链路通过验收（含页面入口、交互操作、结果反馈）。
- [ ] `capability_key` 注册表与路由映射具备持久化可用性（重启后不丢失，且映射冲突可阻断）。

## 10. 依赖与引用
- `docs/archive/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `AGENTS.md`
