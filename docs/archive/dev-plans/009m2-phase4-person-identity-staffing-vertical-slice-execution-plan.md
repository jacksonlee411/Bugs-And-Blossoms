# [Archived] DEV-PLAN-009M2：Phase 4 下一大型里程碑执行计划（Person Identity + Staffing 首个可见样板闭环）

**状态**: 已完成（2026-01-07 01:22 UTC）

> 本文是 `DEV-PLAN-009` 的执行计划补充（里程碑拆解）。假设 `DEV-PLAN-009M1`（SetID + JobCatalog 首个可见样板闭环）已完成，本里程碑聚焦 `DEV-PLAN-009` Phase 4 剩余关键出口：`DEV-PLAN-027(Person Identity)` + `DEV-PLAN-030(Position)` + `DEV-PLAN-031(Assignments)` 的**首个 UI 可见且可操作闭环**。  
> 本文不替代 `DEV-PLAN-027/030/031` 的合同；任何契约变更必须先更新对应 dev-plan 再写代码。

## 0. 实施登记（完成情况评估）

> 结论：本里程碑的“Person Identity + Staffing（Position/Assignments）首个 UI 可见闭环”已在主干落地，并满足 Phase 4 出口条件 #2/#4/#5 的最小口径。

- [X] 合并记录：PR #43 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/43（含 person+staffing schema/migrations、server UI/internal API、Authz policy）。
- [X] Person Identity：`pernr -> person_uuid` 精确解析与联想 options 已落地（`GET /person/api/persons:by-pernr`、`GET /person/api/persons:options`）。
- [X] Staffing：Position/Assignments 写入走 DB Kernel（`submit_position_event`/`submit_assignment_event`），并在同一事务内同步投射到 `*_versions`，提供 as-of 列表读取。
- [X] Assignments UI：只展示 `effective_date`（`lower(validity)`），不展示 `end_date`/`upper(validity)`。
- [X] Routing/Authz：相关路由已纳入 allowlist 且具备最小可拒绝策略（`staffing.positions|staffing.assignments|person.persons`）。
- [X] 证据入口：`docs/dev-records/DEV-PLAN-010-READINESS.md`（新增 `DEV-PLAN-009M2` 小节，记录可复现脚本与结果）。

## 1. 里程碑定义（M2）

- 目标（对齐 `DEV-PLAN-009` Phase 4 出口条件 #2/#4/#5）：
  - Person Identity：`pernr -> person_uuid` 的**精确解析**可被页面/表单实际复用（`DEV-PLAN-027`）。
  - Staffing：在 Position 与 Assignments 上形成“**写入 → 列表读取**”的 UI 可见闭环（`DEV-PLAN-030/031` + `DEV-PLAN-018`）。
  - Assignments UI：严格执行“**仅展示 `effective_date`**”（`DEV-PLAN-031/018/032`），不展示 `end_date`（避免把 `[start,end)` 转回闭区间）。

- 依赖（SSOT 引用）：
  - 路线图与出口口径：`docs/dev-plans/009-implementation-roadmap.md`
  - Person Identity：`docs/dev-plans/027-person-minimal-identity-for-staffing.md`
  - Position：`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
  - Assignments：`docs/dev-plans/031-greenfield-assignment-job-data.md`
  - OrgUnit / JobCatalog（业务组合前置）：`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
  - Tenancy/AuthN：`docs/dev-plans/019-tenant-and-authn.md`
  - RLS（No Tx, No RLS）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
  - Authz（统一 403 + policy SSOT）：`docs/dev-plans/022-authz-casbin-toolchain.md`
  - Routing：`docs/dev-plans/017-routing-strategy.md`
  - UI Shell/IA（导航入口与 as-of 约束）：`docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
  - Atlas+Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
  - 门禁入口与触发器：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`

### 1.1 前置假设（M2 以此为输入）

- [X] `DEV-PLAN-009M1` 已完成：JobCatalog 至少一个实体已具备“解析→写入→列表读取”闭环（为 Assignment/Position 的选择项与引用提供可用主数据）。
- [X] OrgUnit 已具备至少一条可用链路（树/详情可读），可为 Position 提供 `org_unit_id` 的输入来源（对齐 `DEV-PLAN-026/018`）。
- [X] 租户登录与会话可用（对齐 `DEV-PLAN-019`）；运行态 `RLS_ENFORCE=enforce`（本仓库 `.env.example`/`.env.local` 对齐），`AUTHZ_MODE` 默认 `enforce`（见 `pkg/authz`）。

### 1.2 最小用户故事（必须在 UI 可演示）

> 说明：本节只冻结“用户动作链路”，不冻结具体字段细节；字段/事件类型等契约必须回写到 `DEV-PLAN-027/030/031` 后再实现。

1) Position：创建并在列表可见
- [X] 用户从导航进入 `Staffing -> Positions`（对齐 `DEV-PLAN-018` IA；当前 nav 指向 `/org/positions`）。
- [X] 用户在该页完成一次“创建/更新”动作，提交成功后回到列表，能看到新条目（同一 `as_of` 视图内可复现）。

2) Assignment：选择人员 + 选择岗位，创建并在时间线/列表可见
- [X] 用户可从 `Positions` 页面内链接进入 `Assignments`（`/org/positions` → `/org/assignments`）。
- [X] 用户在表单里通过 `pernr` 完成“人员解析/选择”（实现：后端复用 `PersonStore.FindPersonByPernr`；`persons:by-pernr/options` API 已提供）。
- [X] 用户选择一个 position（列表/下拉选择），提交创建成功后在时间线/列表可见。
- [X] 任职记录展示**只包含** `effective_date`（不出现 `end_date`），且跨页面 `as_of` 不丢。

### 1.3 产物清单（本里程碑必须交付）

- [X] Person：`persons:by-pernr` 与 `persons:options` 两个 read 能力可被 UI 复用（`DEV-PLAN-027`）。
- [X] Staffing：Position 与 Assignments 的 Kernel 最小闭环（events + versions + submit_*_event + replay + query）（`DEV-PLAN-030/031`）。
- [X] UI：`/org/positions` 与 `/org/assignments` 两条 UI 链路可操作（`DEV-PLAN-018`），并落入 allowlist（`DEV-PLAN-017`）。
- [X] Authz：补齐 `staffing.positions`、`staffing.assignments`、`person.persons` 的最小策略碎片并 pack（`DEV-PLAN-022`）。

### 1.4 路由清单（UI / internal API / options）

> 说明：路由命名空间与返回契约以 `DEV-PLAN-017` 为 SSOT；本节仅列出本里程碑最小“需要存在且可测”的路由集合（便于拆 PR 与做 routing gates 断言）。

**UI（页面交互）**
- [X] `GET /org/positions?as_of=YYYY-MM-DD`
- [X] `GET /org/positions/form?as_of=YYYY-MM-DD`（实现说明：不单独提供 `/form`；同一路径根据 HX 请求返回 partial）
- [X] `POST /org/positions?as_of=YYYY-MM-DD`（创建最小动作；同一路径支持 HX partial）
- [X] `GET /org/assignments?as_of=YYYY-MM-DD&person_uuid=<uuid>`（时间线/列表；`person_uuid` 为主；UI 也支持 `pernr` 作为便捷入口）
- [X] `GET /org/assignments/form?as_of=YYYY-MM-DD`（实现说明：不单独提供 `/form`；同一路径根据 HX 请求返回 partial）
- [X] `POST /org/assignments?as_of=YYYY-MM-DD`（创建/更新最小动作；同一路径支持 HX partial）

**internal API（JSON-only）**
- [X] `GET /org/api/positions?as_of=YYYY-MM-DD`
- [X] `POST /org/api/positions`（body 以 `DEV-PLAN-030` 冻结的最小合同为准；写动作映射为 `admin`）
- [X] `GET /org/api/assignments?as_of=YYYY-MM-DD&person_uuid=<uuid>`
- [X] `POST /org/api/assignments`（body 以 `DEV-PLAN-031` 冻结的最小合同为准；写动作映射为 `admin`）

**Person read API（供 UI 选择/解析复用；JSON-only）**
- [X] `GET /person/api/persons:options?q=<pernr_or_name>&limit=<n>`
- [X] `GET /person/api/persons:by-pernr?pernr=<digits_max8>`

### 1.5 Authz object/action 映射（MVP：read/admin）

> 说明：object/action 命名与 registry 规则以 `DEV-PLAN-022` 为 SSOT；本节冻结“本里程碑需要哪些 object/action”，避免实现期各处手写字符串漂移。

**objects（固定）**
- [X] `staffing.positions`
- [X] `staffing.assignments`
- [X] `person.persons`

**actions（MVP 固定）**
- [X] `read`：列表/详情/选项/解析等只读行为
- [X] `admin`：任何会触发写入（submit_*_event）的行为（含表单页、提交、patch/correct 等）

**路由 → object/action（最小映射表，必须可复述）**
- [X] `GET /org/positions` → `staffing.positions/read`
- [X] `GET /org/positions/form`、`POST /org/positions` → `staffing.positions/admin`（实现说明：不单独提供 `/form`；同一路径支持 HX partial）
- [X] `GET /org/api/positions` → `staffing.positions/read`
- [X] `POST /org/api/positions` → `staffing.positions/admin`
- [X] `GET /org/assignments` → `staffing.assignments/read`
- [X] `GET /org/assignments/form`、`POST /org/assignments` → `staffing.assignments/admin`（实现说明：不单独提供 `/form`；同一路径支持 HX partial）
- [X] `GET /org/api/assignments` → `staffing.assignments/read`
- [X] `POST /org/api/assignments` → `staffing.assignments/admin`
- [X] `GET /person/api/persons:options`、`GET /person/api/persons:by-pernr` → `person.persons/read`

### 1.6 UI 交互细节（确保“复用 person 解析”而非 staffing 自己解析）

**人员选择（已落地：服务端解析，复用 Person Identity）**
- [X] Assignment 表单提供 `pernr` 输入；服务端复用 Person Identity（`persons:by-pernr` + `pernr` 规范化）解析为 `person_uuid`，并以 `person_uuid` 作为写侧权威输入（表单 hidden input + 写入路径仅接收 `person_uuid`）。
- [X] `GET /person/api/persons:options` / `GET /person/api/persons:by-pernr` 已提供；当前 UI 未实现浏览器侧联想 picker，但 API 已可供后续增强复用。

**列表筛选（建议）**
- [X] `GET /org/assignments` 支持以 `person_uuid` 作为权威筛选参数；同时允许 `pernr` 作为便捷入口（服务端解析后查询，不直读表做“第二解析”）。

**as-of（强制）**
- [X] Positions/Assignments 相关链接与表单提交保持 `as_of` query 参数不丢（对齐 `DEV-PLAN-018`）。

### 1.7 Schema/迁移（需你手工确认的新表清单，供 PR-3 使用）

> 红线提醒：在落盘任何 `CREATE TABLE` 前必须先获得你手工确认。本节用于提前明确“预计新增哪些表”，以便你审批时有稳定清单。

- [X] `positions`（实际落点：`staffing.positions`）
- [X] `position_events`（实际落点：`staffing.position_events`）
- [X] `position_versions`（实际落点：`staffing.position_versions`）
- [X] `assignments`（实际落点：`staffing.assignments`）
- [X] `assignment_events`（实际落点：`staffing.assignment_events`）
- [X] `assignment_versions`（实际落点：`staffing.assignment_versions`）

### 1.8 MVP 冻结点（PR-1 必须在合同里关掉的“最小功能面”）

> 目标：把实现范围收敛到“能演示的一条链路”，避免一次性把 030/031 的全部复杂动作（transfer/rescind/delete-slice 等）塞进 M2。

**Position（M2 MVP）**
- [X] 写动作：支持 `CREATE`（Position 写入）；`reports_to_position_id` 在 M2 固定为 `NULL`（未交付汇报线编辑 UI）。
- [X] 必填输入：`org_unit_id`（来自 OrgUnit 读模型；UI 下拉选择）；其余字段走合同默认。
- [X] 读模型：支持 as-of 列表（`validity @> as_of`）。

**Assignments（M2 MVP）**
- [X] 写动作：支持 `CREATE`（首条）+ `UPDATE`（同一人重复提交时 upsert）；未交付复杂动作（TRANSFER/TERMINATE/…）。
- [X] 写侧输入使用 `person_uuid`；UI 可输入 `pernr` 但会在提交前解析为 `person_uuid`（解析逻辑复用 Person Identity）。
- [X] 约束面：仅交付 `assignment_type=primary`，并保证“每人最多 1 条 primary”（DB unique + upsert）。
- [X] 读模型：支持按 `person_uuid + as_of` 获取时间线/列表，且 UI 仅展示 `effective_date`。

**Person Identity（M2 MVP）**
- [X] `persons:by-pernr`：400 `PERSON_PERNR_INVALID`、404 `PERSON_NOT_FOUND`、200 返回 `person_uuid/pernr/display_name/status`（对齐 `DEV-PLAN-027`）。
- [X] `persons:options`：用于 UI 联想，返回最小字段 `person_uuid/pernr/display_name`（对齐 `DEV-PLAN-027`）。

### 1.9 每个 PR 的“最小门禁口径”（避免只跑一半）

> 入口与完整矩阵以 `AGENTS.md` 为 SSOT；本节只列出 M2 高概率命中项（不复制命令细节）。

- [X] Go 代码（fmt/vet/lint/test）
- [X] Routing（allowlist/route_class/responder gates）
- [X] Authz（policy SSOT + pack + lint/test）
- [X] sqlc（生成物一致性 + 工作区干净）
- [X] Atlas+Goose（按模块 plan/lint/migrate up 闭环）

## 2. 非目标（本里程碑不做）

- 不做 `modules/org` 存量 Position/Assignments 的迁移/兼容/灰度/双写（如需承接存量退场策略，必须另立 dev-plan）。
- 不引入 `effseq`：同一实体同日最多一条事件（对齐 `DEV-PLAN-030/031`）。
- 不引入第二写入口：所有写入必须遵守 One Door（DB Kernel 的 `submit_*_event(...)`）（对齐 `AGENTS.md` 与 `DEV-PLAN-030/031`）。
- 不引入 legacy 回退通道/双链路（对齐 `DEV-PLAN-004M1`）。

## 3. Done 口径（验收/关闭条件）

### 3.1 端到端（用户可见）

- [X] 在 tenant app 中，存在明确入口页面（导航可达），用户能完成至少一次闭环：
  - 创建/更新一个 Position（或等价写入动作）→ 列表可见；
  - 创建/更新一条 Assignment（或等价写入动作）→ 列表/时间线可见。
- [X] `pernr -> person_uuid` 的解析在该闭环中被实际调用（不是仅有工具函数/文档描述）。
- [X] Assignments UI **仅展示** `effective_date`（`lower(validity)`），不展示 `end_date`/`upper(validity)`（对齐 `DEV-PLAN-031/018/032`）。
- [X] `as_of` 参数在页面间可复现透传（对齐 `DEV-PLAN-018` 的 as-of 交互约束）。

### 3.2 安全与门禁（不可漂移）

- [X] No Tx, No RLS：访问 Greenfield 表的路径在事务内注入 `app.current_tenant`（对齐 `DEV-PLAN-021`）。
- [X] 授权可拒绝：Position/Assignments 的 UI/API 路径接入统一 403 契约与策略 SSOT（对齐 `DEV-PLAN-022/017`）。
- [X] 触发器矩阵命中项均通过本地门禁（按 `AGENTS.md`）：涉及 Go/路由/sqlc/迁移/authz 时，不允许“只跑一半”。

### 3.3 证据固化

- [X] 将验证步骤与结果补到 `docs/dev-records/DEV-PLAN-010-READINESS.md`（作为本里程碑的可复现证据入口）。

## 4. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 required checks 全绿且不 `skipped` 后合并；本文只拆解“可合并节奏”，不定义业务字段细节。
>
> 实际落地：本里程碑已在 PR #43 合并完成（docs + db/migrations + server UI/internal API + authz policy），因此下述“建议 PR 分解”统一以 `[X]` 标记，并以 PR #43 作为合并证据入口。

### PR 发起前的文档更新规则（强制）

- [X] 每完成一个 PR 的开发任务，在发起 PR 之前，在“该 PR 命中的 dev-plan 合同文档”中登记本次已完成事项（证据见 PR #43）。
- [X] 本规则适用于所有被本里程碑命中的合同文档（至少 `DEV-PLAN-027/030/031/022/021/010`）。

### PR-1：收口合同与范围（不引入新功能）

- [X] 补齐/确认 `DEV-PLAN-027/030/031` 本里程碑所需的最小合同：边界、不变量、验收口径、失败路径（证据见 PR #43）。
- [X] 若命中路由/授权/迁移/sqlc/生成物：在对应 dev-plan 中记录触发器与门禁命中点（证据见 PR #43）。

### PR-2：Person Identity 可复用解析闭环（027）

- [X] 落地 `persons:by-pernr`（精确解析）与 `persons:options`（联想/options）的最小实现与测试（对齐 `DEV-PLAN-027`）。
- [X] 冻结 `pernr` 规范化口径（前导 0 同值，canonical 存储）并在输入校验与查询路径一致使用（对齐 `DEV-PLAN-027`）。
- [X] 门禁：命中项按 `AGENTS.md` 执行；路由已纳入 allowlist 且 `make check routing` 可阻断漂移（证据见 PR #43）。

### PR-3：Staffing 模块 DB 闭环入口（024）+（需要用户确认的新表）

- [X] 补齐 `make staffing plan/lint/migrate up` 的模块级闭环入口（对齐 `DEV-PLAN-024`）。
- [X] **红线（已执行）**：Position/Assignments 新表已随 PR #43 落盘；如需追溯审批讨论/确认记录，请以 PR #43 为入口。
  - [X] 审批材料：新增表清单见本文 §1.7（以及对应迁移文件）。
  - [X] 执行顺序：模块闭环入口与 smoke 已可复现（证据见 `DEV-PLAN-010` readiness 记录）。

### PR-4：Position Kernel 最小闭环（030）

- [X] 依据 `DEV-PLAN-030` 合同实现 Kernel 的最小闭环（事件 SoT + 同事务同步投射 + 读模型查询）。
- [X] 读写路径在 tenant 事务内执行并注入租户上下文（对齐 `DEV-PLAN-021`）。
- [X] 命中 sqlc：生成物已纳入一致性门禁（对齐 `DEV-PLAN-025`，证据见 PR #43）。

### PR-5：Assignments Kernel 最小闭环（031）

- [X] 依据 `DEV-PLAN-031` 合同实现 Assignment 写入口与读模型（事件 SoT + 同事务同步投射）。
- [X] 写侧输入使用 `person_uuid`；`pernr` 仅用于 UI/筛选与展示，解析逻辑复用 Person Identity（对齐 `DEV-PLAN-027/031`）。
- [X] UI/读侧严格对齐 “仅展示 effective_date” 的数据形状与渲染（对齐 `DEV-PLAN-031/018/032`）。

### PR-6：UI 可见样板闭环（030 + 031 + 018）

- [X] 在 tenant app 中提供 Position 与 Assignments 的最小页面闭环（创建/更新 → 列表可见），并确保入口可达（对齐 `DEV-PLAN-018` IA）。
- [X] 在 Assignment 表单中复用 Person Identity 解析（实现为服务端解析；options/by-pernr API 也已提供）。
- [X] 同一路径单一实现（No Legacy），避免双入口/双实现并存（证据见 PR #43）。

### PR-7：Authz 最小可拒绝（022）

- [X] 为 Position/Assignments 的 UI/API 路径补齐最小 policy（`config/access/policies/**`），并提交 pack 产物（`config/access/policy.csv(.rev)`）。
- [X] 门禁：`make authz-pack && make authz-test && make authz-lint`（证据见 PR #43）。

### PR-8：Readiness 证据补齐（010）

- [X] 更新 `docs/dev-records/DEV-PLAN-010-READINESS.md`：记录从启动到完成本里程碑闭环的可复现脚本与结果（包含时间戳与链接）。

## 5. 本地验证（SSOT 引用）

- 一键对齐 CI（推荐）：`make preflight`（入口以 `Makefile` 为准；不要在本文复制脚本细节）。
- 质量门禁：按 `AGENTS.md` 的触发器矩阵执行（Go/路由/sqlc/迁移/authz/UI 生成物）。

## 6. Readiness 证据（已写入 `DEV-PLAN-010`）

> 目的：让 reviewer/未来自己可以“照着做一遍复现”，而不是只看截图或口头描述。

### 6.1 环境与版本
- [X] 记录：见 `docs/dev-records/DEV-PLAN-010-READINESS.md` 的 `DEV-PLAN-009M2` 小节。

### 6.2 手工验收脚本（最小可复现）
- [X] 登录 tenant app。
- [X] 进入 `Staffing -> Positions`：创建 1 条 Position，返回列表可见。
- [X] 进入 `Staffing -> Assignments`：输入 pernr 并解析出 person；选择 position；创建/更新 1 条 Assignment；返回时间线/列表可见。
- [X] 断言 UI 不展示 `end_date`（只展示 `effective_date`），并在跳转/刷新后 `as_of` 不丢。

### 6.3 失败路径（至少 2 条）
- [X] 输入非法 pernr：`persons:by-pernr` 返回 400（契约与实现一致；证据见 `DEV-PLAN-010` readiness 记录）。
- [X] policy 缺失/权限不足：统一 403（实现由 `withAuthz` enforce；覆盖见 `internal/server/authz_middleware_test.go`）。

## 7. 风险与缓解（提前声明，避免实现期“补丁式绕过”）

- [X] 新表/RLS：已落盘并启用 RLS（见模块迁移与 smoke）；确认/讨论记录如需追溯以 PR #43 为入口。
- [X] Person `pernr` 约束：实现已限制为 1-8 位数字并统一规范化；冲突策略以 `DEV-PLAN-027` 为准（本里程碑未引入存量迁移）。
- [X] 边界漂移：解析逻辑集中于 Person Identity（不在 staffing 表/SQL 中“自造解析”路径）；路由命名空间与模块归属映射在 `DEV-PLAN-018` 已固化。

## 8. 计划评审（对齐 `DEV-PLAN-003` + `AGENTS.md`）

> 评审定位：Stage 2（Plan）为主——检查边界/不变量/失败路径/验收与门禁证据是否足够“可执行且确定”，避免实现期靠试错堆叠（对齐 `DEV-PLAN-003`）。

### 8.1 评审范围（本里程碑涉及的合同）

- [X] `DEV-PLAN-027`（Person Identity）
- [X] `DEV-PLAN-030`（Position）
- [X] `DEV-PLAN-031`（Assignments）
- [X] `DEV-PLAN-022`（Authz）
- [X] `DEV-PLAN-021`（RLS）
- [X] `DEV-PLAN-017`（Routing）
- [X] `DEV-PLAN-018`（UI Shell/IA/as-of）
- [X] `DEV-PLAN-024`（Atlas+Goose）
- [X] `DEV-PLAN-025`（sqlc）
- [X] 评审结论写入规则：合同/评审对齐已随本里程碑落地并回填（证据见 PR #43）。

### 8.2 Stopline（命中即拒绝，作为 PR Review 阻断项）

- [X] One Door：写入走 `submit_*_event(...)`，未引入第二写入口。
- [X] No Tx, No RLS：访问启用 RLS 的 tenant-scoped 表在事务内注入 `app.current_tenant`。
- [X] No Legacy：未引入 legacy/双链路/回退通道；同一路径未并存两套实现。
- [X] Routing：新增路由已纳入 allowlist/route_class 且通过 routing gates。
- [X] Authz：路径可拒绝（统一 403 + policy SSOT + pack 产物）。
- [X] 生成物：sqlc/authz/迁移等生成物已纳入一致性门禁且可阻断漂移。
- [X] 新建表：Position/Assignments 新表已落盘；确认/讨论记录如需追溯以 PR #43 为入口。

### 8.3 Simple > Easy 评审意见（按合同逐项）

#### 8.3.1 `DEV-PLAN-027`（Person Identity）

- 通过：Person 收敛为“身份锚点 + 解析能力”，避免 staffing 直读 `persons` 表形成隐式耦合（对齐 `DEV-PLAN-016/031`）。
- 通过：`pernr` 规则（digits<=8 + 前导 0 同值）与 `persons:by-pernr` 的错误契约清晰，可被 UI 复用。
- 警告（需收口到合同）：`persons` 是否纳入 RLS（或至少在读接口上强制 tenant 过滤/事务约束）在实现期必须定案；否则容易出现“读接口 fail-open/跨租户误查”的隐患（对齐 `DEV-PLAN-021` 的 fail-closed 精神）。
- [X] `PR-1` 要求：Person read API 的 tenant 边界/失败路径/约束已在合同中明确（见 `DEV-PLAN-027`）。
- [X] `PR-1` 要求：`persons:options` 的“联想≠精确解析”边界已在合同中明确（见 `DEV-PLAN-027`）。

#### 8.3.2 `DEV-PLAN-030`（Position）

- 通过：Kernel 边界（DB=裁判/投射，Go=Facade）与 `[start,end)`/同日唯一/全量 replay 的范式清晰（可解释、可验证）。
- 警告（范围过大）：`reports_to` 无环、占编等高级不变量会显著扩大实现与测试面；若不冻结 MVP，容易走向“Easy：先塞字段再补规则”。
- [X] `PR-1` 要求：M2 MVP 已按“reports_to 默认 NULL + 不交付编辑 UI”落地（见本文 §1.8 与实现）。
- [X] `PR-1` 要求：最小事件类型集合已冻结并落地（Position：`CREATE`；Assignments：`CREATE/UPDATE` 作为 upsert）。

#### 8.3.3 `DEV-PLAN-031`（Assignments）

- 通过：明确“写侧使用 `person_uuid`、UI 仅展示 `effective_date`、Valid Time=DATE”这三个关键不变量（对齐 `AGENTS.md` 与 `DEV-PLAN-032`）。
- 警告（容易纠缠）：文档包含大量存量 `modules/org` 任职能力盘点；若实现时直接复用旧 service/旧表，会触发 No Legacy/One Door stopline。
- [X] `PR-1` 要求：M2 MVP 动作集已收敛为 `CREATE` +（可选）`UPDATE`（作为 upsert），未交付复杂动作。
- [X] `PR-1` 要求：已明确并落地 “pernr 仅用于 UI/筛选与展示，解析责任属于 person identity” 的实现约束（见本文 §1.6/§1.8 与实现）。

#### 8.3.4 `DEV-PLAN-022`（Authz）

- 通过：subject/domain/object/action 的冻结口径清晰；强调 object/action registry 单点权威，符合“Simple：一种权威表达”。
- [X] `PR-1` 要求：objects（`staffing.positions`、`staffing.assignments`、`person.persons`）与最小映射已补齐（见 `DEV-PLAN-022` 与本计划 §1.5）。
- [X] `PR-1` 要求：“表单页/写提交”归类为 `admin` 的口径已落地（见 `authzRequirementForRoute` 与 policy）。

#### 8.3.5 `DEV-PLAN-021`（RLS）

- 通过：fail-closed + `assert_current_tenant` 的思路能把“串租户/漏注入”从隐患变成显式错误，符合 stopline 要求。
- [X] `PR-1` 要求：`staffing.positions/assignments` 相关表已纳入 RLS 口径并落地（见 `DEV-PLAN-021` 的表清单与迁移实现）。

#### 8.3.6 `DEV-PLAN-017/018`（Routing + UI Shell）

- 通过：路由分类/全局 responder/as-of 透传的治理思路清晰，能阻断“每个模块自造错误返回/参数口径”。
- 警告（需澄清语义）：本里程碑使用 `/org/positions`、`/org/assignments` 路由前缀（对齐 `DEV-PLAN-018` IA），但实现必须保持代码模块边界为 `modules/staffing`（路由命名空间 ≠ Go 模块边界），避免跨模块 import 漂移（对齐 `DEV-PLAN-016`）。
- [X] `PR-1` 要求：已在 `DEV-PLAN-018` 固化“UI 路由命名空间 ≠ Go 模块边界”的映射解释（`/org/positions`、`/org/assignments` 归属 `modules/staffing`）。

#### 8.3.7 `DEV-PLAN-024/025`（DB 闭环 + sqlc）

- 通过：模块级 Atlas+Goose 闭环与 sqlc 的“生成物+门禁”策略，能把确定性变成 CI 阻断（避免实现期 drift）。
- [X] `PR-1` 要求：staffing 模块命中 DB 门禁与（如命中）sqlc 生成物一致性门禁；权威入口以 `Makefile`/CI SSOT 为准（证据见 PR #43）。
