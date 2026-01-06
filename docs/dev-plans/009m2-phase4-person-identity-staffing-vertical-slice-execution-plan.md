# DEV-PLAN-009M2：Phase 4 下一大型里程碑执行计划（Person Identity + Staffing 首个可见样板闭环）

**状态**: 草拟中（2026-01-06 15:01 UTC）

> 本文是 `DEV-PLAN-009` 的执行计划补充（里程碑拆解）。假设 `DEV-PLAN-009M1`（SetID + JobCatalog 首个可见样板闭环）已完成，本里程碑聚焦 `DEV-PLAN-009` Phase 4 剩余关键出口：`DEV-PLAN-027(Person Identity)` + `DEV-PLAN-030(Position)` + `DEV-PLAN-031(Assignments)` 的**首个 UI 可见且可操作闭环**。  
> 本文不替代 `DEV-PLAN-027/030/031` 的合同；任何契约变更必须先更新对应 dev-plan 再写代码。

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
  - OrgUnit / JobCatalog（业务组合前置）：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
  - Tenancy/AuthN：`docs/dev-plans/019-tenant-and-authn.md`
  - RLS（No Tx, No RLS）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
  - Authz（统一 403 + policy SSOT）：`docs/dev-plans/022-authz-casbin-toolchain.md`
  - Routing：`docs/dev-plans/017-routing-strategy.md`
  - UI Shell/IA（导航入口与 as-of 约束）：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
  - Atlas+Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
  - 门禁入口与触发器：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`

### 1.1 前置假设（M2 以此为输入）

- [ ] `DEV-PLAN-009M1` 已完成：JobCatalog 至少一个实体已具备“解析→写入→列表读取”闭环（为 Assignment/Position 的选择项与引用提供可用主数据）。
- [ ] OrgUnit 已具备至少一条可用链路（树/详情可读），可为 Position 提供 `org_unit_id` 的输入来源（对齐 `DEV-PLAN-026/018`）。
- [ ] 租户登录与会话可用（对齐 `DEV-PLAN-019`），且运行态 `RLS_ENFORCE=enforce`、`AUTHZ_MODE=enforce`（或按 `DEV-PLAN-022` 的 shadow→enforce 节奏推进，但本里程碑最终必须能 enforce）。

### 1.2 最小用户故事（必须在 UI 可演示）

> 说明：本节只冻结“用户动作链路”，不冻结具体字段细节；字段/事件类型等契约必须回写到 `DEV-PLAN-027/030/031` 后再实现。

1) Position：创建并在列表可见
- [ ] 用户从导航进入 `Staffing -> Positions`（对齐 `DEV-PLAN-018` IA）。
- [ ] 用户在该页完成一次“创建/更新”动作，提交成功后回到列表，能看到新条目（同一 `as_of` 视图内可复现）。

2) Assignment：选择人员 + 选择岗位，创建并在时间线/列表可见
- [ ] 用户从导航进入 `Staffing -> Assignments`。
- [ ] 用户在表单里通过 `pernr` 完成“人员解析/选择”（实际调用 `modules/person` 的 read API）。
- [ ] 用户选择一个 position（可用 options endpoint 或列表选择），提交创建成功后在时间线/列表可见。
- [ ] 任职记录展示**只包含** `effective_date`（不出现 `end_date`），且跨页面 `as_of` 不丢。

### 1.3 产物清单（本里程碑必须交付）

- [ ] Person：`persons:by-pernr` 与 `persons:options` 两个 read 能力可被 UI 复用（`DEV-PLAN-027`）。
- [ ] Staffing：Position 与 Assignments 的 Kernel 最小闭环（events + versions + submit_*_event + replay + query）（`DEV-PLAN-030/031`）。
- [ ] UI：`/org/positions` 与 `/org/assignments` 两条 UI 链路可操作（`DEV-PLAN-018`），并落入 allowlist（`DEV-PLAN-017`）。
- [ ] Authz：补齐 `staffing.positions`、`staffing.assignments`、`person.persons` 的最小策略碎片并 pack（`DEV-PLAN-022`）。

### 1.4 路由清单（UI / internal API / options）

> 说明：路由命名空间与返回契约以 `DEV-PLAN-017` 为 SSOT；本节仅列出本里程碑最小“需要存在且可测”的路由集合（便于拆 PR 与做 routing gates 断言）。

**UI（HTML/HTMX）**
- [ ] `GET /org/positions?as_of=YYYY-MM-DD`
- [ ] `GET /org/positions/form?as_of=YYYY-MM-DD`（HTMX partial）
- [ ] `POST /org/positions?as_of=YYYY-MM-DD`（HTMX；创建/更新最小动作）
- [ ] `GET /org/assignments?as_of=YYYY-MM-DD&person_uuid=<uuid>`（时间线/列表；`person_uuid` 为主）
- [ ] `GET /org/assignments/form?as_of=YYYY-MM-DD`（HTMX partial）
- [ ] `POST /org/assignments?as_of=YYYY-MM-DD`（HTMX；创建/更新最小动作）

**internal API（JSON-only）**
- [ ] `GET /org/api/positions?as_of=YYYY-MM-DD`
- [ ] `POST /org/api/positions`（body 以 `DEV-PLAN-030` 冻结的最小合同为准；写动作映射为 `admin`）
- [ ] `GET /org/api/assignments?as_of=YYYY-MM-DD&person_uuid=<uuid>`
- [ ] `POST /org/api/assignments`（body 以 `DEV-PLAN-031` 冻结的最小合同为准；写动作映射为 `admin`）

**Person read API（供 UI 选择/解析复用；JSON-only）**
- [ ] `GET /person/api/persons:options?q=<pernr_or_name>&limit=<n>`
- [ ] `GET /person/api/persons:by-pernr?pernr=<digits_max8>`

### 1.5 Authz object/action 映射（MVP：read/admin）

> 说明：object/action 命名与 registry 规则以 `DEV-PLAN-022` 为 SSOT；本节冻结“本里程碑需要哪些 object/action”，避免实现期各处手写字符串漂移。

**objects（固定）**
- [ ] `staffing.positions`
- [ ] `staffing.assignments`
- [ ] `person.persons`

**actions（MVP 固定）**
- [ ] `read`：列表/详情/选项/解析等只读行为
- [ ] `admin`：任何会触发写入（submit_*_event）的行为（含表单页、提交、patch/correct 等）

**路由 → object/action（最小映射表，必须可复述）**
- [ ] `GET /org/positions` → `staffing.positions/read`
- [ ] `GET /org/positions/form`、`POST /org/positions` → `staffing.positions/admin`
- [ ] `GET /org/api/positions` → `staffing.positions/read`
- [ ] `POST /org/api/positions` → `staffing.positions/admin`
- [ ] `GET /org/assignments` → `staffing.assignments/read`
- [ ] `GET /org/assignments/form`、`POST /org/assignments` → `staffing.assignments/admin`
- [ ] `GET /org/api/assignments` → `staffing.assignments/read`
- [ ] `POST /org/api/assignments` → `staffing.assignments/admin`
- [ ] `GET /person/api/persons:options`、`GET /person/api/persons:by-pernr` → `person.persons/read`

### 1.6 UI 交互细节（确保“复用 person 解析”而非 staffing 自己解析）

**人员选择（必须）**
- [ ] Assignment 表单提供 Person Picker：
  - 输入框支持输入 `pernr`（含前导 0），由前端触发 `GET /person/api/persons:options` 获取候选（联想）。
  - 选择候选后，将 `person_uuid` 写入 hidden input，提交写请求时只提交 `person_uuid`（不提交 pernr 给写侧）。
- [ ] 若允许“直接输入 pernr 并精确解析”：在提交前通过 `GET /person/api/persons:by-pernr` 将 pernr 解析为 `person_uuid` 并写入 hidden input；解析失败必须给出可见错误（400/404 对齐 `DEV-PLAN-027` 的错误契约）。

**列表筛选（建议）**
- [ ] `GET /org/assignments` 以 `person_uuid` 作为筛选参数的权威输入；如 UI 支持在 URL 上使用 `pernr`，必须先在浏览器侧解析为 `person_uuid` 再跳转（避免 staffing 服务端解析 pernr 形成边界漂移）。

**as-of（强制）**
- [ ] Positions/Assignments 相关链接与表单提交必须保持 `as_of` query 参数不丢（对齐 `DEV-PLAN-018`）。

### 1.7 Schema/迁移（需你手工确认的新表清单，供 PR-3 使用）

> 红线提醒：在落盘任何 `CREATE TABLE` 前必须先获得你手工确认。本节用于提前明确“预计新增哪些表”，以便你审批时有稳定清单。

- [ ] `positions`
- [ ] `position_events`
- [ ] `position_versions`
- [ ] `assignments`
- [ ] `assignment_events`
- [ ] `assignment_versions`

### 1.8 MVP 冻结点（PR-1 必须在合同里关掉的“最小功能面”）

> 目标：把实现范围收敛到“能演示的一条链路”，避免一次性把 030/031 的全部复杂动作（transfer/rescind/delete-slice 等）塞进 M2。

**Position（M2 MVP）**
- [ ] 写动作：仅要求支持 `CREATE`（可选支持 `UPDATE`）；`reports_to_position_id` 在 M2 可固定为 `NULL`（先不交付汇报线编辑 UI），从而把“无环”不变量约束面缩到最小（仍需禁止自指）。
- [ ] 必填输入：`org_unit_id`（来自 OrgUnit options/选择）；其余字段可按合同给默认（例如 `capacity_fte=1.0`、`lifecycle_status=active`）。
- [ ] 读模型：支持 as-of 列表（`validity @> as_of`）。

**Assignments（M2 MVP）**
- [ ] 写动作：仅要求支持 `CREATE`（可选支持 `UPDATE`）；不交付 `TRANSFER/TERMINATE/RESCIND/DELETE-SLICE` 等复杂动作。
- [ ] 写侧输入：只接收 `person_uuid`（不得接收 pernr 作为写侧权威输入）；`pernr` 只用于 UI 解析与展示（对齐 `DEV-PLAN-027/031`）。
- [ ] 约束面：先只交付 `assignment_type=primary`（或默认 primary），并在同一 `as_of` 下保证“每人最多 1 条 primary”（精确口径在 `DEV-PLAN-031` 里冻结）。
- [ ] 读模型：支持按 `person_uuid + as_of` 获取时间线/列表，且 UI 仅展示 `effective_date`。

**Person Identity（M2 MVP）**
- [ ] `persons:by-pernr`：400 `PERSON_PERNR_INVALID`、404 `PERSON_NOT_FOUND`、200 返回 `person_uuid/pernr/display_name/status`（对齐 `DEV-PLAN-027`）。
- [ ] `persons:options`：用于 UI 联想，返回最小字段 `person_uuid/pernr/display_name`（对齐 `DEV-PLAN-027`）。

### 1.9 每个 PR 的“最小门禁口径”（避免只跑一半）

> 入口与完整矩阵以 `AGENTS.md` 为 SSOT；本节只列出 M2 高概率命中项（不复制命令细节）。

- [ ] Go 代码（fmt/vet/lint/test）
- [ ] Routing（allowlist/route_class/responder gates）
- [ ] Authz（policy SSOT + pack + lint/test）
- [ ] sqlc（生成物一致性 + 工作区干净）
- [ ] Atlas+Goose（按模块 plan/lint/migrate up 闭环）

## 2. 非目标（本里程碑不做）

- 不做 `modules/org` 存量 Position/Assignments 的迁移/兼容/灰度/双写（如需承接存量退场策略，必须另立 dev-plan）。
- 不引入 `effseq`：同一实体同日最多一条事件（对齐 `DEV-PLAN-030/031`）。
- 不引入第二写入口：所有写入必须遵守 One Door（DB Kernel 的 `submit_*_event(...)`）（对齐 `AGENTS.md` 与 `DEV-PLAN-030/031`）。
- 不引入 legacy 回退通道/双链路（对齐 `DEV-PLAN-004M1`）。

## 3. Done 口径（验收/关闭条件）

### 3.1 端到端（用户可见）

- [ ] 在 tenant app 中，存在明确入口页面（导航可达），用户能完成至少一次闭环：
  - 创建/更新一个 Position（或等价写入动作）→ 列表可见；
  - 创建/更新一条 Assignment（或等价写入动作）→ 列表/时间线可见。
- [ ] `pernr -> person_uuid` 的解析在该闭环中被实际调用（不是仅有工具函数/文档描述），且 `modules/staffing` 不直读 `persons` 表做解析（对齐 `DEV-PLAN-027/016`）。
- [ ] Assignments UI **仅展示** `effective_date`（`lower(validity)`），不展示 `end_date`/`upper(validity)`（对齐 `DEV-PLAN-031/018/032`）。
- [ ] `as_of` 参数在页面间可复现透传（对齐 `DEV-PLAN-018` 的 as-of 交互约束）。

### 3.2 安全与门禁（不可漂移）

- [ ] No Tx, No RLS：访问 Greenfield 表的路径必须在事务内注入 `app.current_tenant`，缺失上下文 fail-closed（对齐 `DEV-PLAN-021`）。
- [ ] 授权可拒绝：Position/Assignments 的 UI/API 路径接入统一 403 契约与策略 SSOT（对齐 `DEV-PLAN-022/017`）。
- [ ] 触发器矩阵命中项均通过本地门禁（按 `AGENTS.md`）：涉及 Go/路由/sqlc/迁移/authz/UI 生成物时，不允许“只跑一半”。

### 3.3 证据固化

- [ ] 将验证步骤与结果补到 `docs/dev-records/DEV-PLAN-010-READINESS.md`（作为本里程碑的可复现证据入口）。

## 4. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 required checks 全绿且不 `skipped` 后合并；本文只拆解“可合并节奏”，不定义业务字段细节。

### PR 发起前的文档更新规则（强制）

- [ ] 每完成一个 PR 的开发任务，在发起 PR 之前，必须先在“该 PR 命中的 dev-plan 合同文档”中登记本次已完成事项：将对应条目从 `[ ]` 更新为 `[X]`，并补充必要证据（命令+时间戳或 PR 链接）。
- [ ] 本规则适用于所有被本里程碑命中的合同文档（至少 `DEV-PLAN-027/030/031/022/021/010`）。

### PR-1：收口合同与范围（不引入新功能）

- [ ] 补齐/确认 `DEV-PLAN-027/030/031` 本里程碑所需的最小合同：边界、不变量、验收口径、失败路径（不在本文定义字段细节）。
- [ ] 若会新增路由/授权/迁移/sqlc/生成物：先在对应 dev-plan 中记录触发器与门禁命中点（SSOT 引用）。

### PR-2：Person Identity 可复用解析闭环（027）

- [ ] 落地 `persons:by-pernr`（精确解析）与 `persons:options`（用于 UI 选择/联想）的最小实现与测试（对齐 `DEV-PLAN-027`）。
- [ ] 冻结 `pernr` 规范化口径（前导 0 同值，canonical 存储）并在输入校验与查询路径一致使用（对齐 `DEV-PLAN-027`）。
- [ ] 门禁：命中项按 `AGENTS.md` 执行；若涉及路由 allowlist，确保 `make check routing` 通过。

### PR-3：Staffing 模块 DB 闭环入口（024）+（需要用户确认的新表）

- [ ] 若 `staffing` 模块尚未具备：补齐 `make staffing plan/lint/migrate up` 的模块级闭环入口（对齐 `DEV-PLAN-024`）。
- [ ] **红线**：本 PR 预计会引入 Position/Assignments 的新表（`CREATE TABLE`）。在落盘任何新表前，必须先获得用户手工确认再继续。
  - [ ] 审批材料（必须提供）：本 PR 将新增的表清单（见 §1.7）+ 每张表的用途一句话 + 是否启用 RLS（默认 tenant-scoped 表启用）。
  - [ ] 审批后执行顺序：先 `make staffing plan` 给出变更摘要，再落盘迁移并 `make staffing migrate up` 做 smoke。

### PR-4：Position Kernel 最小闭环（030）

- [ ] 依据 `DEV-PLAN-030` 合同实现 Kernel 的最小闭环（事件 SoT + 同事务同步投射 + 读模型查询）。
- [ ] 读写路径均在 tenant 事务内执行并注入租户上下文（对齐 `DEV-PLAN-021`）。
- [ ] 如命中 sqlc：执行 `make sqlc-generate`，并确保生成物提交且工作区干净（对齐 `DEV-PLAN-025`）。

### PR-5：Assignments Kernel 最小闭环（031）

- [ ] 依据 `DEV-PLAN-031` 合同实现 Assignment 写入口与读模型（事件 SoT + 同事务同步投射）。
- [ ] 写侧输入使用 `person_uuid`；pernr 仅用于 UI/筛选与展示，解析责任在 `modules/person`（对齐 `DEV-PLAN-027/031`）。
- [ ] UI/读侧严格对齐 “仅展示 effective_date” 的数据形状与渲染（对齐 `DEV-PLAN-031/018/032`）。

### PR-6：UI 可见样板闭环（030 + 031 + 018）

- [ ] 在 tenant app 中提供 Position 与 Assignments 的最小页面闭环（创建/更新 → 列表可见），并确保导航可达（对齐 `DEV-PLAN-018` IA）。
- [ ] 在 Assignment 表单中实际复用 Person Identity 解析（options + by-pernr 任一或组合），避免 `staffing` 自己解析。
- [ ] 若历史已存在同名路由/handler：本 PR 必须保证“单一路径单一实现”，避免双入口/双实现并存（对齐 No Legacy 原则）。

### PR-7：Authz 最小可拒绝（022）

- [ ] 为 Position/Assignments 的 UI/API 路径补齐最小 policy（`config/access/policies/**`），并提交 pack 产物（`config/access/policy.csv(.rev)`）。
- [ ] 门禁：`make authz-pack && make authz-test && make authz-lint`。

### PR-8：Readiness 证据补齐（010）

- [ ] 更新 `docs/dev-records/DEV-PLAN-010-READINESS.md`：记录从启动到完成本里程碑闭环的浏览器验证脚本与结果（包含时间戳与链接）。

## 5. 本地验证（SSOT 引用）

- 一键对齐 CI（推荐）：`make preflight`（入口以 `Makefile` 为准；不要在本文复制脚本细节）。
- 质量门禁：按 `AGENTS.md` 的触发器矩阵执行（Go/路由/sqlc/迁移/authz/UI 生成物）。

## 6. Readiness 证据模板（PR-8 填入 `DEV-PLAN-010`，本节仅提供结构）

> 目的：让 reviewer/未来自己可以“照着做一遍复现”，而不是只看截图或口头描述。

### 6.1 环境与版本
- [ ] 记录：当前 commit/PR 链接、运行模式（`AUTHZ_MODE`、`RLS_ENFORCE`）、tenant、测试账号/角色（不记录密钥）。

### 6.2 手工验收脚本（最小可复现）
- [ ] 登录 tenant app。
- [ ] 进入 `Staffing -> Positions`：创建 1 条 Position，返回列表可见。
- [ ] 进入 `Staffing -> Assignments`：在表单内输入 pernr 并解析出 person；选择刚创建的 position；创建 1 条 Assignment；返回时间线/列表可见。
- [ ] 断言 UI 不展示 `end_date`（只展示 `effective_date`），并在跳转/刷新后 `as_of` 不丢。

### 6.3 失败路径（至少 2 条）
- [ ] 输入非法 pernr：`persons:by-pernr` 返回 400（可见错误提示）。
- [ ] policy 缺失/权限不足：相关页面/提交返回统一 403（对齐 `DEV-PLAN-022/017`），并能从日志定位缺口（不在响应体泄露策略细节）。

## 7. 风险与缓解（提前声明，避免实现期“补丁式绕过”）

- [ ] 新表/RLS：schema 落盘需要你手工确认；确认前不得创建表；确认后按 `DEV-PLAN-024` 模块闭环执行并留证据。
- [ ] Person `pernr` 约束与存量数据：若发现存在非数字/超长/前导 0 冲突，必须先在 `DEV-PLAN-027` 中给出修复策略再落约束（可采用 `NOT VALID` 渐进）。
- [ ] 边界漂移：`modules/staffing` 不得 import `modules/person/**` 做解析；解析责任必须通过 Person read API 或浏览器侧完成（对齐 `DEV-PLAN-016`）。

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
- [ ] 评审结论写入规则：本节结论用于驱动 `PR-1` 的文档收口；实现若偏离必须先回写对应 dev-plan 再落代码（对齐 `AGENTS.md` Contract First）。

### 8.2 Stopline（命中即拒绝，作为 PR Review 阻断项）

- [ ] One Door：任何写入不得绕过 `submit_*_event(...)` 形成第二写入口（对齐 `AGENTS.md`、`DEV-PLAN-030/031/025`）。
- [ ] No Tx, No RLS：访问启用 RLS 的 tenant-scoped 表必须显式事务 + 注入 `app.current_tenant`，缺上下文必须 fail-closed（对齐 `DEV-PLAN-021`）。
- [ ] No Legacy：不得引入 legacy/双链路/回退通道；同一路径不得并存两套实现（对齐 `DEV-PLAN-004M1`）。
- [ ] Routing：新增路由必须纳入 allowlist/route_class 并通过 routing gates；不得新增非版本化 `/api/*`（对齐 `DEV-PLAN-017`）。
- [ ] Authz：路径必须“可拒绝”（统一 403 + policy SSOT + pack 产物）；object/action 不得在模块内手写拼装（对齐 `DEV-PLAN-022`）。
- [ ] 生成物：sqlc/authz/迁移/UI 等生成物必须提交且工作区干净（对齐 `AGENTS.md` stopline）。
- [ ] 新建表：落盘任何 `CREATE TABLE` 前必须先获得用户手工确认（对齐 `AGENTS.md` 红线；见 §1.7/PR-3）。

### 8.3 Simple > Easy 评审意见（按合同逐项）

#### 8.3.1 `DEV-PLAN-027`（Person Identity）

- 通过：Person 收敛为“身份锚点 + 解析能力”，避免 staffing 直读 `persons` 表形成隐式耦合（对齐 `DEV-PLAN-016/031`）。
- 通过：`pernr` 规则（digits<=8 + 前导 0 同值）与 `persons:by-pernr` 的错误契约清晰，可被 UI 复用。
- 警告（需收口到合同）：`persons` 是否纳入 RLS（或至少在读接口上强制 tenant 过滤/事务约束）在实现期必须定案；否则容易出现“读接口 fail-open/跨租户误查”的隐患（对齐 `DEV-PLAN-021` 的 fail-closed 精神）。
- [ ] `PR-1` 要求：在 `DEV-PLAN-027` 明确 M2 对 Person read API 的最小运行时约束（tenant 边界、authz object、失败路径、是否要求显式事务）。
- [ ] `PR-1` 要求：明确 `persons:options` 的“联想不等于精确解析”边界，避免实现期用 options 代替 by-pernr 造成“同值/歧义”漂移。

#### 8.3.2 `DEV-PLAN-030`（Position）

- 通过：Kernel 边界（DB=裁判/投射，Go=Facade）与 `[start,end)`/同日唯一/全量 replay 的范式清晰（可解释、可验证）。
- 警告（范围过大）：`reports_to` 无环、占编等高级不变量会显著扩大实现与测试面；若不冻结 MVP，容易走向“Easy：先塞字段再补规则”。
- [ ] `PR-1` 要求：在 `DEV-PLAN-030` 明确 M2 MVP（见 §1.8）：先不交付 `reports_to_position_id` 编辑 UI（可默认 NULL），把无环约束的交付面收敛到“禁止自指/允许空”。
- [ ] `PR-1` 要求：冻结 M2 的最小事件类型集合与输入必填项（至少 `CREATE`；其余作为非目标），并声明对应失败路径与验收口径。

#### 8.3.3 `DEV-PLAN-031`（Assignments）

- 通过：明确“写侧使用 `person_uuid`、UI 仅展示 `effective_date`、Valid Time=DATE”这三个关键不变量（对齐 `AGENTS.md` 与 `DEV-PLAN-032`）。
- 警告（容易纠缠）：文档包含大量存量 `modules/org` 任职能力盘点；若实现时直接复用旧 service/旧表，会触发 No Legacy/One Door stopline。
- [ ] `PR-1` 要求：在 `DEV-PLAN-031` 明确 M2 MVP 动作集（见 §1.8）：只交付 `CREATE`（可选 `UPDATE`），不交付 `TRANSFER/TERMINATE/RESCIND/DELETE-SLICE` 等复杂动作。
- [ ] `PR-1` 要求：明确 “pernr 仅用于 UI/筛选与展示，解析责任属于 person 模块” 的实现约束与验证方式（至少一条测试/门禁断言，防止 staffing 直读 persons 表）。

#### 8.3.4 `DEV-PLAN-022`（Authz）

- 通过：subject/domain/object/action 的冻结口径清晰；强调 object/action registry 单点权威，符合“Simple：一种权威表达”。
- [ ] `PR-1` 要求：在 `DEV-PLAN-022` 或其对应实现 checklist 中补齐本里程碑 objects（`staffing.positions`、`staffing.assignments`、`person.persons`）与路由映射的最小验收点（本计划 §1.5 已给出映射表，需回写到合同或 registry 计划中）。
- [ ] `PR-1` 要求：明确“表单页/写提交”归类为 `admin` 的口径，避免实现期出现“页面可打开但提交 403/反之”的漂移。

#### 8.3.5 `DEV-PLAN-021`（RLS）

- 通过：fail-closed + `assert_current_tenant` 的思路能把“串租户/漏注入”从隐患变成显式错误，符合 stopline 要求。
- [ ] `PR-1` 要求：将 `positions/*` 与 `assignments/*`（见 §1.7）纳入 `DEV-PLAN-021` 的“应启用 RLS 表清单/模板”，并声明最小测试点（缺 tenant 注入必失败、跨租户不可见）。

#### 8.3.6 `DEV-PLAN-017/018`（Routing + UI Shell）

- 通过：路由分类/全局 responder/as-of 透传的治理思路清晰，能阻断“每个模块自造错误返回/参数口径”。
- 警告（需澄清语义）：本里程碑使用 `/org/positions`、`/org/assignments` 路由前缀（对齐 `DEV-PLAN-018` IA），但实现必须保持代码模块边界为 `modules/staffing`（路由命名空间 ≠ Go 模块边界），避免跨模块 import 漂移（对齐 `DEV-PLAN-016`）。
- [ ] `PR-1` 要求：在对应合同（建议 `DEV-PLAN-018` 或 `DEV-PLAN-017`）补一条“UI 路由前缀与代码模块归属的映射解释”，用于 review 时快速判断是否发生边界漂移。

#### 8.3.7 `DEV-PLAN-024/025`（DB 闭环 + sqlc）

- 通过：模块级 Atlas+Goose 闭环与 sqlc 的“生成物+门禁”策略，能把确定性变成 CI 阻断（避免实现期 drift）。
- [ ] `PR-1` 要求：明确 staffing 模块是否会命中 sqlc（预期会），以及 schema 导出/生成物路径的权威入口（引用 `Makefile`/CI SSOT，不复制命令）。
