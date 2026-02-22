# DEV-PLAN-005：项目标准与外部规范采纳清单

**状态**: 规划中（2026-02-22 02:32 UTC）

## 背景

仓库已进入多模块并行演进阶段，接口命名、可观测性和错误语义若缺少统一标准，容易在不同模块重复分叉，增加联调、排障与门禁治理成本。

为避免标准漂移，新增本文件作为“项目采用规范与标准”的统一入口（SSOT），后续新增标准均在本文件追加，不再分散在临时文档中。

## 目标

1. 固化本项目对外部规范的采纳结论，形成统一口径。
2. 给出可执行的命名与语义边界，支持后续代码/门禁落地。
3. 为后续标准扩展提供稳定模板（持续编号追加）。

## 标准编排规则（持续扩展）

1. 标准条目使用 `STD-XXX` 编号（从 `STD-001` 起）。
2. 每条标准至少包含：决策、适用范围、禁止事项、参考规范。
3. 当新标准与历史计划文档冲突时，以本文件最新已批准条目为准，并要求相关 dev-plan 同步修订。
4. 标准条目只定义“应当是什么”，具体实施拆分到独立执行计划（例如 `DEV-PLAN-109A` 的后续修订计划）。

## 标准清单

### STD-001：业务幂等与链路追踪命名标准（第一条，冻结）

**决策（Normative）**

1. 业务幂等统一使用 `request_id`。
2. Tracing 统一使用 `trace_id`。
3. `request_id` 与 `trace_id` 语义必须严格分离：
   - `request_id` 仅用于“业务幂等去重/重试一致性”；
   - `trace_id` 仅用于“链路追踪/可观测性关联”。

**适用范围**

- API 请求/响应契约（JSON 字段、校验文案、错误明细字段）；
- 服务层入参、领域错误映射、审计与日志上下文字段；
- 前端调用约定、网关/中间件、测试数据命名与门禁规则。

**禁止事项**

1. 禁止在新增业务写入契约中继续引入 `request_code`。
2. 禁止把 tracing 字段命名为 `request_id`（追踪场景统一改为 `trace_id`）。
3. 禁止同一语义在不同层出现双命名并存（如同一接口同时出现 `request_id` 与 `request_code` 表达幂等）。

**参考规范**

- Google AIP-155（Request identification / idempotency）
- W3C Trace Context（`trace-id` 传播）
- OpenTelemetry Trace 语义（`trace_id`）

**与现有计划关系**

- 本条标准生效后，涉及幂等命名的既有计划（含 `DEV-PLAN-109/109A`）需要按本标准修订为一致口径。
- 本条标准仅冻结“目标口径”，不在本文件内展开迁移步骤与排期。

### STD-002：`as_of` 与 `effective_date` 时间语义标准（冻结）

**决策（Normative）**

1. `as_of` 仅表示**读模型切片时点**，中文统一翻译为**“查询时点”**；必须显式提供；缺失/非法统一返回 `400 invalid_as_of`（message：`as_of required`）。
2. `effective_date` 仅表示**写入生效日**，中文统一翻译为**“生效日期”**；必须显式提供；缺失/非法统一返回 `400 invalid_effective_date`（message：`effective_date required`）。
3. 业务有效时间统一使用 `date`（`YYYY-MM-DD`，日粒度）；禁止用时分秒表达业务生效语义。
4. 业务时间不得由服务端默认 today（`time.Now().UTC()` / `current_date`）推断；必须由请求显式传入并透传到 service/store/kernel。
5. 审计/事务时间与业务有效时间严格分离：`created_at` / `updated_at` / `transaction_time` 可用 `timestamptz`，不得替代业务生效时间。
6. bootstrap/backfill 必须显式提供 `effective_date`，且需通过 root BU 在该日期有效性校验；不允许固定常量日期兜底。
7. 同一输入（`tenant + route + payload/query + as_of/effective_date`）在不同运行日必须可重放且结果一致（审计时间戳字段除外）。

**适用范围**

- API 契约（query/body 字段、错误码、返回语义）；
- UI 路由与页面请求参数（URL/payload）；
- 服务层与控制器参数校验；
- SQL/Kernel 函数、迁移与回填脚本；
- 自动化测试、静态门禁与文档契约。

**禁止事项**

1. 禁止 `if asOf == "" { asOf = time.Now().UTC().Format("2006-01-02") }` 这类隐式回填。
2. 禁止 `if req.EffectiveDate == "" { req.EffectiveDate = ... }` 这类隐式回填。
3. 禁止以 `as_of` 回填 `effective_date`（或反向混用）。
4. 禁止在 070/071 业务有效期判断中引入 `current_date` 作为隐式输入。
5. 禁止把业务有效期字段建模为 `timestamptz`。
6. 禁止“缺失参数自动补齐后继续执行”的 fail-open 行为。

**参考规范**

- `AGENTS.md`（仓库级时间语义总则）
- `docs/dev-plans/032-effective-date-day-granularity.md`
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`

**与现有计划关系**

- 本条标准生效后，`DEV-PLAN-070/071/071A/071B/102/102B` 及其相关测试计划（如 `DEV-PLAN-063`）必须对齐统一口径。
- 本条标准仅冻结“目标口径”，不在本文件内展开迁移步骤与排期（迁移执行由后续实施计划承接）。

## 后续扩展待办

1. [ ] 新增“STD-001 落地执行计划”（门禁、接口、DB/代码命名、迁移窗口与回滚策略）。
2. [ ] 新增“STD-002 落地执行计划”（以 `DEV-PLAN-102B` 为主计划，覆盖文档/实现/测试/门禁收口）。
3. [ ] 建立“标准变更记录模板”（记录版本、影响面、验收证据）。
4. [ ] 将标准检查接入 CI（新增或修订对应 `make check` 门禁）。

## 交付物

1. 本标准文档：`docs/dev-plans/005-project-standards-and-spec-adoption.md`
2. AGENTS 文档地图新增入口（确保可发现性）
