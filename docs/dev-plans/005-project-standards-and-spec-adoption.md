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
- `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`

**与现有计划关系**

- 本条标准生效后，`DEV-PLAN-070/071/071A/071B/102/102B` 及其相关测试计划（如 `DEV-PLAN-063`）必须对齐统一口径。
- 本条标准仅冻结“目标口径”，不在本文件内展开迁移步骤与排期（迁移执行由后续实施计划承接）。

### STD-003：ID/UUID/Code 命名与内外标识边界标准（冻结）

**决策（Normative）**

1. 对于行业有明确标准或约定俗成的命名，优先遵循行业标准与社区共识，不另造别名（例如：`request_id`、`trace_id`）。
2. 标识命名统一遵循后缀语义：UUID 使用 `_uuid`，结构性数字标识使用 `_id`，业务编码使用 `_code`；技术主键可保留 `id`。
3. OrgUnit 领域标识边界冻结：对外契约仅使用 `org_code`，内部结构关系仅使用 `org_id`（8 位 `int4`，`10000000~99999999`）。
4. 边界解析规则冻结：请求进入服务边界时必须先做 `org_code -> org_id` 解析；对外响应回写标识时必须使用 `org_code`。
5. `org_code` 规则冻结：仅做 `upper` 归一化（不 trim）；长度 `1~64`；允许空格、`\t`、中文标点与全角字符；禁止全空白。
6. 命名收敛遵循 No Legacy：禁止新旧字段双轨并存；旧字段与新字段同时出现时必须 `400 invalid_request` fail-closed。
7. 幂等命名与追踪命名以 `STD-001` 为准：业务幂等统一 `request_id`，Tracing 统一 `trace_id`；历史 `request_code` 不再作为新增契约命名。

**适用范围**

- API 契约（JSON/query/form 字段命名与错误语义）；
- UI 表单与展示字段命名；
- DB Schema、SQL 函数、事件 payload、sqlc 模型；
- 服务层/控制器/测试命名与门禁规则。

**禁止事项**

1. 禁止新增 `*_id`/`*_uuid`/`*_code` 语义错位字段（例如 UUID 字段继续命名为 `*_id`）。
2. 禁止在外部接口暴露内部结构标识 `org_id`。
3. 禁止继续引入或回填历史命名 `request_code` 作为幂等字段。
4. 禁止通过兼容别名窗口保留旧字段（含 `org_unit_id` 对外兼容透传）。
5. 禁止对行业标准命名再造同义字段（如同时引入 `traceId`/`trace_id`、`request_code`/`request_id`）。

**参考规范**

- `docs/archive/dev-plans/026a-orgunit-id-uuid-code-naming.md`
- `docs/archive/dev-plans/026b-orgunit-external-id-code-mapping.md`
- `docs/archive/dev-plans/072-repo-wide-id-code-naming-convergence.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`

**与现有计划关系**

- `DEV-PLAN-072` 自本标准生效后归档，规范性内容由 `STD-003` 接管；`DEV-PLAN-072` 仅作为历史过程记录保留。
- 既有模块在命名收敛实施时，均以 `STD-003 + STD-001` 组合口径为准。

### STD-004：版本标记最小暴露标准（冻结）

**决策（Normative）**

1. 在 Greenfield 单实现前提下，用户可见面与对外契约默认不得暴露实现版本标记（例如页面文案、URL、路由参数、公开字段命名）。
2. 开发侧命名同样遵循“去噪优先”：文件名、包名、类型名、函数名与脚本入口不应通过版本后缀表达实现阶段。
3. 不再将“版本标记扫描”作为 CI 强制门禁；改为契约评审与代码评审中的人工阻断项。

**适用范围**

- UI 文案、导航、按钮、错误提示；
- 路由路径、query 参数、重定向地址、API 字段命名；
- 仓库内文件/目录命名、Go 标识符与脚本目标命名；
- 文档对外示例与可复制命令。

**禁止事项**

1. 禁止新增对外契约字段/路径中的实现版本标记（包括 URL/query/JSON 字段）。
2. 禁止通过“短期版本后缀”形成长期对外别名窗口。
3. 禁止在同一语义下并存“无版本名 + 版本名”双命名，造成调用方分叉。

**参考规范**

- `docs/archive/dev-plans/004-remove-version-marker-repo-wide.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`

**与现有计划关系**

- `DEV-PLAN-004` 自本标准生效后归档，规范性条款由 `STD-004` 接管；`DEV-PLAN-004` 仅保留历史实施与证据价值。
- 若后续确需对外引入显式版本号，必须先在 dev-plan 中说明业务必要性与退场策略，并通过评审后实施。

### STD-005：Tenant App 前端单链路与入口唯一性标准（冻结）

**决策（Normative）**

1. Tenant App 用户 UI 唯一入口冻结为 `/app/**`；唯一登录页面冻结为 `/app/login`。
2. 租户应用侧不再提供 `GET /login` HTML 页面，且禁止 `/login -> /app/login` 的兼容别名窗口。
3. 前端工程目录唯一冻结为 `apps/web`；go:embed 产物目录唯一冻结为 `internal/server/assets/web/**`；静态资源 URL 前缀冻结为 `/assets/web/`。
4. UI 构建与产物一致性统一通过 `make css` + `assert-clean` 门禁收敛；触发器必须覆盖 `apps/web/**` 与 `internal/server/assets/web/**`。
5. 删除后的旧 UI 家族（例如 `/ui/*`、`/lang/*`、旧 server-rendered HTML 路由）不得以任何 alias/rewrite/backdoor 方式复活。

**适用范围**

- 路由 allowlist、route_class 分类与中间件放行规则；
- 前端 router/base path、静态资源路径与构建脚本；
- CI path filters、`make` 门禁入口与 E2E 断言；
- 文档中的示例 URL、登录入口描述与可执行操作说明。

**禁止事项**

1. 禁止新增或恢复 `GET /login`（tenant app）及其 302 兼容跳转窗口。
2. 禁止重新引入 `apps/web-mui`、`/assets/web-mui/`、`internal/server/assets/web-mui/**` 等技术后缀双轨。
3. 禁止为 tenant app 新增第二套 UI 运行链路（如旧局部渲染链路/旧 Shell 并行保活）。

**参考规范**

- `docs/archive/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`
- `docs/archive/dev-plans/103a-dev-plan-103-closure-p3-p6-apps-web-rename.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`

**与现有计划关系**

- `DEV-PLAN-103/103A` 的规范性条款由本标准接管；两份文档保留为历史实施与证据上下文。
- 后续涉及 tenant app 登录入口、UI 构建目录或静态资源前缀的改动，必须先对齐本标准，再进入实现评审。

### STD-006：未登录/失效会话返回语义标准（按 route_class 冻结）

**决策（Normative）**

1. `route_class=ui` 在缺失/失效/跨租户会话时，统一返回 `302 Location=/app/login`（必要时清理 `sid`）。
2. `route_class=internal_api/public_api/webhook` 在缺失/失效/跨租户会话时，统一返回 JSON `401 unauthorized`；禁止返回 `302` 与 HTML 页面。
3. 返回语义以 allowlist/classifier 产生的 `route_class` 为主判定，不得依赖 `Accept` 头做反向兜底。
4. 会话失效与租户不匹配必须 fail-closed：清理 `sid` 后按上述 route_class 语义返回，不得“自动切租户/默认租户继续执行”。
5. 匿名可达的会话创建入口统一为 `POST /iam/api/sessions`（tenant app）；其放行应受 route_class 与显式路由注册约束。

**适用范围**

- tenancy/session middleware、global responder、authn 边界；
- API client 对 `401` 的前端跳转策略与错误处理；
- E2E 与集成测试中的未登录、跨租户、会话失效断言；
- 文档中的登录流程、返回码与重定向语义描述。

**禁止事项**

1. 禁止 API 在未登录时返回 `302` 到登录页或返回 HTML 错误页。
2. 禁止 UI 受保护路径在未登录时返回“静默 200/空页”而非明确跳转到 `/app/login`。
3. 禁止在各模块自行定义与 route_class 冲突的未登录处理分支，造成语义漂移。

**参考规范**

- `docs/archive/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`

**与现有计划关系**

- 既有 AuthN/路由/E2E 文档（含 `DEV-PLAN-019/061/022/060/017`）若仍使用 `/login` 或旧返回语义，需按本标准收敛并更新验收断言。
- 本标准仅冻结口径；具体迁移步骤由后续实施计划承接。

### STD-007：Valid Time 区间建模与更正边界标准（冻结）

**决策（Normative）**

1. 业务有效期的数据库区间表达统一采用半开区间 `[start, end)`；使用 `daterange` 时必须保持该语义一致。
2. 同一业务键（至少含 `tenant_id`，以及必要的 `setid/business_key`）在 Valid Time 上必须满足 no-overlap（有效区间不可重叠）。
3. 若实体使用 `effective_date + end_date`（闭区间）建模，做唯一性/冲突校验时必须转换为半开区间：`daterange(effective_date, end_date + 1, '[)')`。
4. 有效期更正（effective_date correction）必须满足相邻边界约束：`prev < new < next`；缺失一侧边界时视为单侧无界。
5. 同一业务键同日唯一必须成立：更正/插入后不得与既有记录发生“同键同日冲突”。
6. `gapless` 与 `last infinite` 仅在实体契约显式要求时启用，不作为全仓默认强制项。

**适用范围**

- Effective-dated 的 schema 设计（`versions` 表、`daterange`/`effective_date,end_date`）；
- 写入链路中的冲突校验、修正插入规则与回放一致性；
- SQL 约束、迁移脚本、服务层校验与测试断言；
- 文档中的有效期建模约定与示例。

**禁止事项**

1. 禁止同一语义在不同模块混用闭区间/半开区间且无显式转换规则。
2. 禁止在 `effective_date + end_date` 模型里直接用闭区间做冲突校验而不执行 `end_date + 1` 转换。
3. 禁止更正链路绕过相邻边界校验（例如允许 `new <= prev` 或 `new >= next`）。
4. 禁止通过“默认 today/取最近一条”掩盖有效期冲突与缺失参数问题。

**参考规范**

- `docs/dev-plans/032-effective-date-day-granularity.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`

**与现有计划关系**

- `STD-002` 继续负责“as_of/effective_date 输入语义与显式化”；本标准负责“Valid Time 区间建模与更正边界”。
- `DEV-PLAN-032` 保留为实现指南与示例来源；规范性口径以本标准为准。

## 后续扩展待办

1. [ ] 新增“STD-001 落地执行计划”（门禁、接口、DB/代码命名、迁移窗口与回滚策略）。
2. [ ] 新增“STD-002 落地执行计划”（以 `DEV-PLAN-102B` 为主计划，覆盖文档/实现/测试/门禁收口）。
3. [ ] 建立“标准变更记录模板”（记录版本、影响面、验收证据）。
4. [ ] 将标准检查接入 CI（新增或修订对应 `make check` 门禁）。
5. [ ] 新增“STD-007 落地执行计划”（区间约束、修正边界校验、冲突门禁与回放一致性测试）。

## 交付物

1. 本标准文档：`docs/dev-plans/005-project-standards-and-spec-adoption.md`
2. AGENTS 文档地图新增入口（确保可发现性）
