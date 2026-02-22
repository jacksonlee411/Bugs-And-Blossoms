# DEV-PLAN-031：任职记录（Job Data / Assignments）全新实现（Staffing，事件 SoT + 同步投射）

**状态**: 已评审（2026-01-11 13:30 UTC）— M2 MVP 已完成（证据：`docs/dev-records/DEV-PLAN-010-READINESS.md` 的 `DEV-PLAN-009M2` 小节），M3+ 扩展待续

## 1. 背景与上下文 (Context)

本计划目标是**全新实现**“任职记录（Job Data / Assignments）”能力（Greenfield），并对齐：
- 技术路线（`DEV-PLAN-026/029/030`）：**DB=Projection Kernel（权威）**、同步投射（同事务 delete+replay）、One Door Policy（唯一写入口）、Valid Time=DATE、`daterange` 统一使用左闭右开 `[start,end)`。
- DDD/分层与模块骨架（`DEV-PLAN-015`、`DEV-PLAN-016`）：采用 4 模块（`orgunit/jobcatalog/staffing/person`），其中任职记录归属 `modules/staffing`。

本仓库已落地 Assignments 的 M2 最小闭环（DB Kernel + UI + Internal API + Authz + Routing + E2E）。因此本文件从“草拟”升级为 Assignments 的合同 SSOT：冻结当前已实现的稳定合同，并将 `TRANSFER/TERMINATE/RESCIND/DELETE-SLICE` 等复杂动作显式延后（对齐 `DEV-PLAN-009M2`）。

本计划保持 Greenfield 口径：不做任何存量盘点/承接/退场策略。若未来需要承接外部存量系统或历史实现，必须另立 dev-plan 并明确迁移/回滚策略（对齐 `AGENTS.md` 的 No Legacy 与 Contract First）。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标
- [X] 在 `modules/staffing` 内全新实现任职记录：事件 SoT（`staffing.assignment_events`）+ 同步投射（`staffing.assignment_versions`），并由 DB Kernel 强制关键不变量（写入口：`staffing.submit_assignment_event`）。
- [X] UI/列表展示层：任职记录**仅显示生效日期（effective date）**；底层仍使用 `daterange` 的左闭右开 `[start,end)` 表达有效期（不改为闭区间）。
- [X] 以清晰契约替代“隐式耦合”：
  - 写路径输入统一使用 `person_uuid`（对齐 016），不再以 pernr 作为写侧主键；`pernr` 仅用于 UI 查询/展示。
  - pernr→uuid 解析由 Person Identity 提供（`persons:by-pernr`/`persons:options`），Assignments 写侧不直读 `persons` 表（Person Identity 合同见 `DEV-PLAN-027`）。
- [X] 对齐 `DEV-PLAN-009M2` 的 M2 MVP：先交付 `primary` + `CREATE/UPDATE` 的最小闭环，并把 `TRANSFER/TERMINATE/RESCIND/DELETE-SLICE` 等复杂动作显式延后（避免实现期补丁式堆叠）。
- [X] One Door Policy：Go（Facade/Handler）只提交事件，DB=Kernel（裁决/投射/重放）；应用层不得直写任一 SoT/versions/identity 表。

### 2.2 非目标（明确不做）
- 不做任何存量系统迁移/兼容/cutover（Greenfield）；如需承接外部存量必须另立 dev-plan。
- 不引入 `effseq`，同一实体同日最多一条事件（对齐 026/030/029）。
- 不在本计划内实现“跨模块异步事件/旧 outbox/audit/settings 支撑能力”的兼容；如需要另立计划。

## 2.3 工具链与门禁（SSOT 引用）
- DDD 分层框架（Greenfield）：`docs/dev-plans/015-ddd-layering-framework.md`
- Greenfield HR 模块骨架（4 模块）：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- Kernel 边界与 daterange 口径：`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/032-effective-date-day-granularity.md`
- 分层/依赖门禁：`.gocleanarch.yml`（入口：`make check lint`）
- 命令入口与 CI：`Makefile`、`.github/workflows/quality-gates.yml`

## 3. 实施现状（M2 已落地）

- DB Schema/Kernel SSOT：
  - `modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`
  - `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`
- 写入口（One Door）：`staffing.submit_assignment_event(...)`（同事务内写 events → `staffing.replay_assignment_versions(...)`）
- 读模型：`staffing.assignment_versions`，as-of：`validity @> $as_of::date`，展示：`effective_date = lower(validity)`
- HTTP：
  - UI：`GET/POST /org/assignments`
  - Internal API：`GET/POST /org/api/assignments`
- Person Identity：`GET /person/api/persons:by-pernr`、`GET /person/api/persons:options`
- 证据入口：`docs/dev-records/DEV-PLAN-010-READINESS.md` 的 `DEV-PLAN-009M2` 小节（含可复现 curl 与门禁记录）。
- 代码组织说明：Go Facade/Handler 当前位于 `internal/server/*`（composition root）；`modules/staffing` 的 Go 目录仍为骨架（后续可迁移落位，不改变合同）。

## 4. 核心设计约束（合同，必须遵守）

### 4.1 Valid Time 与 `daterange` 口径（强制）
- Valid Time 粒度：`date`（对齐 `DEV-PLAN-032`）。
- 所有有效期区间：使用 `daterange` 且统一左闭右开 `[start,end)`。
- 展示层：任职记录**仅展示 `effective_date`**（即 `lower(validity)`），不展示 `end_date`；避免把 `[start,end)` 再转回闭区间造成语义混乱。

### 4.2 One Door Policy（写入口唯一）
- 应用层不得直写任一 SoT/versions/identity 表；只能调用 DB Kernel 的 `submit_*_event(...)`。
- Kernel 内部函数（如 `apply_*_logic`）不得被应用角色直接执行。

## 5. 目标架构（modules/staffing，DB Kernel + Go Facade）

### 5.1 模块归属
任职记录归属 `modules/staffing`（对齐 `DEV-PLAN-016`：Position+Assignment 收敛以承载跨聚合不变量）。

### 5.2 目录骨架（对齐 015/016；目标）
```
modules/staffing/
  domain/ports/                        # AssignmentKernelPort（与 PositionKernelPort 同模块）
  domain/types/                        # 稳定枚举/错误码/输入 DTO（可选）
  services/                            # Facade：Tx + 调 Kernel + serrors 映射
  infrastructure/persistence/          # pgx adapter（调用 submit_*_event）
  infrastructure/persistence/schema/   # staffing-schema.sql（SSOT，含 assignment）
  presentation/controllers/            # /org/assignments（UI）与 /org/api/assignments（API）
  presentation/templates/
  presentation/locales/
  module.go
  links.go
```

> 现状（M2）：DB schema 已落位于 `modules/staffing/infrastructure/persistence/schema/*`；HTTP/Facade 当前在 `internal/server/*`。

### 5.3 Kernel 边界（与 026-029 同构）
- **DB = Projection Kernel（权威）**：插入事件（幂等）→ 同事务全量重放生成 versions → 裁决不变量。
- **Go = Command Facade**：鉴权/事务边界 + 调 Kernel + 错误映射到 `pkg/serrors`。

## 6. 方案（新实现方式：事件 SoT + 同步投射）

### 6.1 领域建模：以“时间线聚合（timeline aggregate）”作为写侧单位

本计划将“任职记录”建模为 `person_uuid + assignment_type` 的时间线聚合（至少覆盖 `primary`；扩展类型可选）：
- 聚合标识：`assignment_id`（uuid）。`staffing.assignments` 通过 `(tenant_id, person_uuid, assignment_type)` 的唯一约束将其绑定到某个时间线；提交事件时会对 id mismatch fail-closed（`STAFFING_ASSIGNMENT_ID_MISMATCH`）。
- 时间线内的每个有效片段（version slice）记录岗位、组织、FTE、就业状态等业务属性。

优势：
- 与“同日最多一条事件/无 overlap/gapless/最后一段无穷”天然对齐。
- 转移/终止/再入职等都成为“时间线上的事件”，由 Kernel 统一裁决。

### 6.2 数据模型（合同 SSOT；M2 已落盘）

> 说明：M2 的表/约束已落盘；后续若新增表/迁移，仍需遵守仓库红线并先获手工确认（见 `AGENTS.md`）。

**Identity（timeline anchor）**
- `staffing.assignments`：`(tenant_id, person_uuid, assignment_type)` unique；当前仅支持 `assignment_type='primary'`。

**Write Side（SoT）**
- `staffing.assignment_events`：
  - 幂等键：`event_id` unique
  - 同日唯一：`(tenant_id, assignment_id, effective_date)` unique（不引入 `effseq`）
  - `event_type ∈ {'CREATE','UPDATE'}`（M2）
  - `payload` 必须为 JSON object（字段合同见 §6.3）
  - `request_id`：`(tenant_id, request_id)` unique（幂等/审计关联）

**Read Model（Projection）**
- `staffing.assignment_versions`：
  - `validity daterange`：统一 `[start,end)`；末段 infinity
  - no-overlap：`EXCLUDE ... (tenant_id, assignment_id, validity &&)`
  - 交叉不变量（M2）：`EXCLUDE ... (tenant_id, position_id, validity &&) WHERE (status='active')`（同一时点一个 position 最多被一个 active assignment 占用）
  - gapless + 末段 infinity：由 `staffing.replay_assignment_versions` 在同事务内校验

### 6.3 Kernel 入口（唯一写入口，M2 合同）
- `staffing.submit_assignment_event(...)`：
  - 幂等：同 `event_id` 重试不重复写入；参数不一致时报 `STAFFING_IDEMPOTENCY_REUSED`
  - identity：`(tenant_id, person_uuid, assignment_type)` 唯一绑定到一个 `assignment_id`；不一致时报 `STAFFING_ASSIGNMENT_ID_MISMATCH`
  - 写后同步投射：同事务内调用 `staffing.replay_assignment_versions(p_tenant_id, p_assignment_id)`
  - payload（M2）：
    - `position_id`：CREATE 必填；UPDATE 可选
    - `status`：仅 UPDATE 生效（`active|inactive`）
    - `allocated_fte`：可选；(0,1]
    - `profile`：可选；JSON object

### 6.4 只展示生效日期（UI/读接口形状）
- Timeline 列表行只渲染：
  - `effective_date = lower(validity)`
  - 以及岗位/组织/职类等**业务属性**（允许显示）
  - 不展示 `end_date`/`upper(validity)`（避免闭区间混用）
- as-of 查询仍使用 `validity @> $as_of::date` 保证语义一致。

## 7. 动作映射（M2 已实现 / M3+ 待续）

> 本节把常见业务动作映射到事件模型（保留/替代/不做）。

1) 创建/更新 Primary Assignment（M2）
- 新方案：通过 `staffing.submit_assignment_event(event_type='CREATE'|'UPDATE', effective_date=...)` 写入；首次为 CREATE，后续为 UPDATE（“upsert”由 Go Facade 决定）。
- 输入主键：使用 `person_uuid`（pernr 仅用于 UI 查询与展示，通过 Person Identity 解析）。

2) 更新字段（M2）
- 新方案：统一为 `event_type='UPDATE'`，由 Kernel 重放生成切片。

3) Correct/Rescind/Transfer/Terminate（M3+）
- 当前未实现；后续若引入必须先冻结事件类型与 payload，并保持同日唯一（无 effseq）与 One Door。

4) Delete slice + stitch
- 不提供直接删 versions 的能力；如需撤销/更正，必须通过事件表达（One Door）。

5) Primary gap-free / no-overlap
- 由 `daterange [)` + exclude constraints + replay 校验强制；展示层只用 `effective_date`。

## 8. 里程碑与验收（Plan → Implement 的承接）

1. [X] 冻结事件类型枚举、payload 合同、错误契约（见 §6.3；证据：`docs/dev-records/DEV-PLAN-010-READINESS.md`）。
2. [X] 冻结路由（UI+API）与输入输出契约（只展示 effective_date；见 §3、§6.4）。
3. [X] 冻结 DB Kernel：`staffing.submit_assignment_event` + `staffing.replay_assignment_versions`（对齐 026-030）。
4. [X] 定义最小测试集（覆盖现状 + 待补齐项）：
   - 已覆盖：E2E（TP-060-03）断言 UI 仅展示 effective_date、disabled position 不可任职；单测覆盖 handler/store 的基本失败路径。
   - 待补齐：同日唯一/no-overlap/gapless/末段 infinity 的针对性负例（建议落在 `cmd/dbtool staffing-smoke` 或 db 单测）。
5. [X] 通过相关门禁（M2 证据见 `docs/dev-records/DEV-PLAN-010-READINESS.md` 的 `DEV-PLAN-009M2` 小节；触发器矩阵以 `AGENTS.md` 为准）。

## 9. 待开发（M3+，按 `DEV-PLAN-001` 细化）

> 本节把“尚未交付的内容”细化为可直接实现的设计与验收清单；实现时应优先保持合同简单（Simple > Easy），避免引入 `effseq`/第二写入口/legacy 回退通道。

### 9.1 M3-A：Upsert 可重复执行（Idempotency / Re-run）

#### 背景与问题
- 现状：同一 `assignment_id + effective_date` 再次提交会触发 `assignment_events_one_per_day_unique`，导致写入失败（不满足“可重复执行”口径）。
- 目标口径来源：TP-060-03 明确要求 `/org/assignments` 的 upsert 可重复执行（同一 `effective_date` 幂等不变或变更），且不得产生同日重复 slice（SSOT：`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`）。

#### 目标与非目标
- [ ] 重复提交“相同输入”（同 tenant + 同 person_uuid + 同 effective_date + 同 position_id + 同薪酬字段）应幂等成功：不新增事件、不改变 timeline。
- [ ] 重复提交“不同输入但 effective_date 相同”必须 fail-closed，并给出稳定错误码，明确引导用户改用新的 `effective_date` 表达更正（不支持同日多次修改；不引入 effseq）。
- 不做：为同一 `effective_date` 引入可编辑/覆盖旧事件的能力（事件 SoT 保持 append-only；不做 delete/upsert events）。

#### 方案（详细设计）
- 方案 A（推荐，最小改动）：**确定性 `event_id`（uuid）用于 Upsert**
  - Upsert 生成的 `event_id` 必须是确定性的：其输入只由稳定键构成（建议：`tenant_id + assignment_id + effective_date + assignment_type`；不包含 payload）。
  - 重复提交相同输入 → 复用同一 `event_id` → Kernel 走幂等分支返回既有事件；不触发 one-per-day unique。
  - 若用户试图在同一 `effective_date` 写入不同 payload：因为仍复用同一 `event_id`，Kernel 将抛出 `STAFFING_IDEMPOTENCY_REUSED`（稳定可定位），从而显式拒绝“同日改写”。
  - `request_id` 建议与 `event_id` 同源（或等值），确保重试链路一致。
  - 注意：该策略依赖 `staffing.submit_assignment_event` 的幂等比较字段包含 payload；因此能准确识别 drift。
- 方案 B（备选，Kernel 友好错误）：在 `staffing.submit_assignment_event` 内增加显式检查
  - 在写入事件前，在同一把 advisory lock 下查询是否已存在相同 `effective_date` 的事件。
  - 若存在且 `event_id != p_event_id`，则 `RAISE EXCEPTION MESSAGE='STAFFING_ASSIGNMENT_ONE_PER_DAY'`（稳定错误码）以替代 PG 约束报错（便于 UI/API 统一处理）。

#### 触发器与门禁（勾选本子项命中）
- [ ] Go 代码（store/handler/dbtool/test）：按 `AGENTS.md` 触发器矩阵执行
- [ ] E2E（若把 rerun 断言写进 TP-060-03）：`make e2e`

#### 验收标准（可直接编码/断言）
- [ ] 在同一 tenant 下，对同一 `person_uuid`、同一 `effective_date`、同一 payload 连续 upsert 两次：
  - HTTP：均成功（UI 为 303；Internal API 为 200/201 视实现而定）
  - DB：`staffing.assignment_events` 仅 1 条（同一 `event_id`）；`staffing.assignment_versions` 不变
- [ ] 若同一 `effective_date` 第二次提交的 payload 不同：返回 `STAFFING_IDEMPOTENCY_REUSED`（或 `STAFFING_ASSIGNMENT_ONE_PER_DAY`，取决于选定方案）

### 9.2 M3-B：Terminate / Deactivate（`status=inactive`）

#### 背景与问题
- Kernel 已支持 `payload.status`（`active|inactive`）并投射到 `assignment_versions.status`，但 UI/Internal API 尚无用户入口（目前只能通过 dbtool/直调 Kernel 演示）。

#### 目标与非目标
- [ ] 提供最小用户入口：把 primary assignment 标记为 `inactive`（Terminate/Deactivate），并在 Timeline 中可见。
- [ ] `inactive` slice 不占用 position（依赖既有约束：`assignment_versions_position_no_overlap WHERE status='active'`）。
- 不做：终止原因码、离职手续流、撤销（rescind）等复杂动作。

#### 接口契约（Internal API，M3 扩展）
- `POST /org/api/assignments?as_of=YYYY-MM-DD`
  - Request Body（JSON）新增可选字段：
    - `status`: `"active" | "inactive"`（可选；为空/缺失表示不变）
  - 其余字段保持 M2 合同：`effective_date/person_uuid/position_id/allocated_fte`
- 错误码：
  - 400：JSON/日期/字段格式错误（HTTP 层校验）
  - 422：`STAFFING_*`（Kernel fail-closed，例如 status 非法、position 不可用等）

#### UI 交互（M3）
- `/org/assignments` 的 Upsert 表单新增 `status` 下拉（默认 `active`）；提交仍走同一路径 `POST /org/assignments?as_of=...`。
- Timeline 列表维持“只展示 effective_date”不变，但允许展示 `status` 字段值（已存在列）。

#### 触发器与门禁（勾选本子项命中）
- [ ] Go 代码（handler/store/test）：按 `AGENTS.md` 触发器矩阵执行
- [ ] E2E（新增终止链路断言时）：`make e2e`

#### 验收标准
- [ ] UI：对同一 person 提交 `status=inactive` 后，Timeline 行显示 `status=inactive`，且不展示 `end_date`。
- [ ] DB：同一 position 在该日期之后可被其他 active assignment 占用（或可被 position disable 流程使用，取决于 Position M3 的交叉裁决）。

### 9.3 M3-C：质量补齐（Assignment 不变量的可复现负例）

#### 目标
- [ ] 为 Assignment 的关键不变量提供“可复现、可排障”的负例与证据入口（建议落在 `cmd/dbtool staffing-smoke` 或 DB 集成测试），避免仅依赖隐式约束。

#### 覆盖点（最小集）
- [ ] one-per-day：同一 `assignment_id` 同一 `effective_date` 不允许第二条事件（应输出稳定错误码或至少能定位到约束/错误来源）。
- [ ] 引用校验：active assignment 引用的 `position_id` 在 `effective_date` as-of 下必须存在且为 active：`STAFFING_POSITION_NOT_FOUND_AS_OF` / `STAFFING_POSITION_DISABLED_AS_OF`。
- [ ] Position 排他：同一时点一个 position 最多被一个 active assignment 占用（`assignment_versions_position_no_overlap`）。

#### 触发器与门禁（勾选本子项命中）
- [ ] Go 代码（dbtool/test）：按 `AGENTS.md` 触发器矩阵执行

### 9.4 M4+：Correct / Rescind / Delete-slice（需单独冻结合同）

> 说明：在“不引入 effseq + 同日唯一”的不变量下，若要对历史 `effective_date` 的事件做“更正/撤销”而不新增同日事件，直接往 `assignment_events` 增加新事件类型极易演化为隐式 `effseq`。因此本节选定：**不改变 `assignment_events` 的同日唯一约束与 append-only 口径**，而是通过“附属 SoT（correction/rescind）+ replay 统一解释”的方式实现。

#### 9.4.1 语义边界（合同冻结，已确认）
- [X] **同日唯一仍成立**：`staffing.assignment_events` 继续保持 `(tenant_id, assignment_id, effective_date)` 唯一；不引入 `effseq`。
- [X] **事件表 append-only**：`staffing.assignment_events` 不允许被应用层更新/删除；更正与撤销以“附属事件表”表达（同样只允许通过 DB Kernel 写入）。
- [X] **target 定位键**：以 `(tenant_id, assignment_id, target_effective_date)` 指向目标事件（等价于目标事件的同日唯一键）；避免引用 `bigserial id` 形成跨租户存在性侧信道。
- [X] **非目标**：不支持对同一 `target_effective_date` 进行“不同 replacement_payload 的多次更正”（否则会引入隐式序列）。需要再次修改时，必须使用新的 `effective_date` 表达变更（对齐“同日唯一”）。
- [X] **Correct（更正）**：对某一条既有 `assignment_event` 提交“payload 替换”，replay 时使用替换后的 payload 解释该事件。
  - 不新增 `assignment_events` 行，因此不触发同日唯一冲突。
  - 更正只改变“该事件对时间线的解释”，不改变其 `effective_date`。
  - 若未来需要支持 `effective_date` 调整，必须另立计划并遵循 `DEV-PLAN-032` 的有效期调整规则（前后边界、不冲突、关联有效性）；本计划明确不做。
- [X] **Rescind（撤销/作废）**：对某一条既有 `assignment_event` 标记为“在 replay 中忽略”，从而实现 delete-slice + stitch（删某片段并自动缝合）。
  - Rescind 不改变原事件行；replay 时跳过该事件。
- [X] **Rescind 限制**：只允许撤销 `event_type='UPDATE'`；禁止撤销首事件/CREATE（避免时间线失锚与隐式重建语义）。
- [X] **Correct vs Rescind**：若目标事件已 rescind，则 correct 必须 fail-closed（错误：`STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED`）；两者同时存在时，replay 以 `rescind > correct` 解释。
- [X] **禁止回退通道**：不允许引入 `read=legacy`、双写/兜底读等任何 legacy 分支（对齐 `DEV-PLAN-004M1`）。

#### 9.4.2 数据模型与约束（拟新增表；实施前需手工确认）

> 红线：新增表/迁移属于破坏性变更路径，实施前必须获得你的手工确认（见 `AGENTS.md`）。

> 约定：下述表内的 `id bigserial` 仅用于内部主键/索引；不得在任何对外接口/跨表引用中作为定位键。对外与跨表定位统一使用 `event_id` 与 `(tenant_id, assignment_id, target_effective_date)`。

1) `staffing.assignment_event_corrections`（append-only）
- 用途：对某条 `assignment_event` 的 payload 做“替换解释”。
- 字段（合同，类型/约束必须落地）：
  - `id bigserial primary key`
  - `event_id uuid not null`（幂等键，unique）
  - `tenant_id uuid not null`
  - `assignment_id uuid not null`
  - `target_effective_date date not null`（目标事件定位键）
  - `replacement_payload jsonb not null`（必须是 object）
  - `request_id text not null`（tenant 内 unique）
  - `initiator_id uuid not null`
  - `transaction_time timestamptz not null default now()`
  - `created_at timestamptz not null default now()`
- 约束：
  - `UNIQUE(event_id)`
  - `UNIQUE(tenant_id, request_id)`
  - `UNIQUE(tenant_id, assignment_id, target_effective_date)`（每个 target 最多 1 个更正；避免“更正的更正”引入隐式序列）
  - `CHECK(jsonb_typeof(replacement_payload)='object')`
- RLS：
  - `tenant_id = current_setting('app.current_tenant')::uuid`（USING/WITH CHECK）

2) `staffing.assignment_event_rescinds`（append-only）
- 用途：对某条 `assignment_event` 做撤销（replay 跳过）。
- 字段（合同，类型/约束必须落地）：
  - `id bigserial primary key`
  - `event_id uuid not null`（幂等键，unique）
  - `tenant_id uuid not null`
  - `assignment_id uuid not null`
  - `target_effective_date date not null`（目标事件定位键）
  - `payload jsonb not null default '{}'::jsonb`（可选：reason/note；必须是 object）
  - `request_id text not null`（tenant 内 unique）
  - `initiator_id uuid not null`
  - `transaction_time timestamptz not null default now()`
  - `created_at timestamptz not null default now()`
- 约束：
  - `UNIQUE(event_id)`
  - `UNIQUE(tenant_id, request_id)`
  - `UNIQUE(tenant_id, assignment_id, target_effective_date)`（每个 target 最多 1 个撤销）
  - `CHECK(jsonb_typeof(payload)='object')`
- RLS：同上（tenant 隔离，fail-closed）

#### 9.4.3 DB Kernel 写入口（One Door，拟新增函数）

> 原则：应用层不得直写 `*_events/*_versions/*_corrections/*_rescinds`；只能调用 Kernel 函数。两类新函数必须在同一事务内完成：插入 correction/rescind → `replay_assignment_versions`。

1) `staffing.submit_assignment_event_correction(...)`
- 输入（建议签名；以实施时 schema SSOT 为准）：
  - `p_event_id uuid`
  - `p_tenant_id uuid`
  - `p_assignment_id uuid`
  - `p_target_effective_date date`（目标事件定位键；同日唯一保证最多命中 1 条）
  - `p_replacement_payload jsonb`
  - `p_request_id text`
  - `p_initiator_id uuid`
- 行为：
  - 校验 tenant ctx（`staffing.assert_current_tenant`）
  - 在 assignment lock 下定位 target event（`tenant_id + assignment_id + target_effective_date`）
  - 若 target 不存在：`STAFFING_ASSIGNMENT_EVENT_NOT_FOUND`
  - 若 target 已 rescind：必须 fail-closed（`STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED`）
  - 插入 correction（按 `event_id` 幂等；按 `target` 二次幂等）：
    - 若同 target 已存在 correction：replacement_payload 相同则返回既有 id；否则报 `STAFFING_ASSIGNMENT_EVENT_ALREADY_CORRECTED`
  - 调用 `staffing.replay_assignment_versions(p_tenant_id, p_assignment_id)`；若 replay 失败则整事务回滚（fail-closed）

2) `staffing.submit_assignment_event_rescind(...)`
- 输入（建议签名）：
  - `p_event_id uuid`
  - `p_tenant_id uuid`
  - `p_assignment_id uuid`
  - `p_target_effective_date date`
  - `p_payload jsonb`
  - `p_request_id text`
  - `p_initiator_id uuid`
- 行为：
  - 与 correction 同构：定位 target → 插入 rescind → replay → 失败则回滚
  - 约束（建议）：
    - 只允许撤销 `event_type='UPDATE'`（禁止 CREATE）：`STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND`
  - 幂等口径（建议）：若同 target 已存在 rescind，直接返回既有 id（rescind payload 可不纳入幂等比较；如需纳入必须冻结规则）

#### 9.4.4 Replay 解释规则（`staffing.replay_assignment_versions` 的扩展）

> 选定：replay 的“输入事件流”是 `assignment_events` 在去除 rescind 后的序列；每条事件的 payload 为 `COALESCE(correction.replacement_payload, assignment_events.payload)`。

- [ ] 参考实现步骤（伪代码，消歧义）：
  1) 读取 `assignment_events`（按 `effective_date` 升序）并按 key 左连接 `assignment_event_rescinds` 与 `assignment_event_corrections`
  2) 过滤 rescinded 事件，得到 `filtered_events`
  3) 计算 `effective_payload = COALESCE(correction.replacement_payload, event.payload)`
  4) 基于 `filtered_events` 计算 `next_effective`（lead），构造 `validity=[effective_date,next_effective)`（末段 `infinity`）
  5) 生成 versions 并裁决不变量；失败则整体回滚（fail-closed）
- [ ] 事件过滤：replay 时跳过所有被 rescind 的 `assignment_events`。
- [ ] payload 替换：replay 时若存在 correction，则使用 replacement_payload；否则使用原 payload。
- [ ] lead/validity：`next_effective` 必须基于“过滤后的事件序列”计算，确保删除中间事件后自动 stitch。
- [ ] 关联键（强制）：correction/rescind 与目标事件的关联使用 `(tenant_id, assignment_id, target_effective_date)`（其中 `target_effective_date = assignment_events.effective_date`）；不得依赖跨租户可枚举的 `bigserial id`。
- [ ] 不变量继续由 replay 裁决：
  - `CREATE` 必须为首事件（基于过滤后的序列）
  - gapless + 末段 infinity
  - position as-of 存在且 active（当 status=active）
  - position 排他（同一时点最多一个 active assignment 占用）

#### 9.4.5 接口契约（Internal API + UI，建议形态）

> 说明：实现时可以沿用 “单一页面入口” 原则，把操作入口收敛在 `/org/assignments`；Internal API 建议增加专用 endpoint，避免把 `/org/api/assignments` 变成多动作大杂烩。

1) Internal API（建议新增）
- `POST /org/api/assignment-events:correct`
  - Request（JSON）：
    - `assignment_id`（必填；可通过 `GET /org/api/assignments?person_uuid=...` 获取）
    - `target_effective_date`（必填）
    - `replacement_payload`（必填，object；字段范围同 §6.3 payload）
  - Response：200/201 + 返回 `correction_event_id` 与 `target_effective_date`
  - 实施备注：需更新 `config/routing/allowlist.yaml`、`internal/server/authz_middleware.go`，并通过 `make check routing` 与 authz 门禁（入口见 `AGENTS.md`/`Makefile`）。
- `POST /org/api/assignment-events:rescind`
  - Request（JSON）：
    - `assignment_id`（必填；可通过 `GET /org/api/assignments?person_uuid=...` 获取）
    - `target_effective_date`（必填）
    - `payload`（可选，object；reason/note）
  - Response：200/201 + 返回 `rescind_event_id`
  - 实施备注：同上（路由 allowlist + authz 映射 + 门禁）。

2) UI（建议扩展 `/org/assignments`）
- Timeline 每行提供 “Rescind”/“Correct” 操作入口（按钮/表单），但仍保持“只展示 effective_date，不展示 end_date”合同不变。
- Correct 表单仅暴露“可理解的字段”（position/status/allocated_fte/profile），其结果映射为 replacement_payload。

#### 9.4.6 错误码（稳定、可定位）

> 口径：DB 通过 `RAISE EXCEPTION MESSAGE='STAFFING_*'` 输出稳定错误码；HTTP 层将 `STAFFING_*` 映射为 422（对齐既有模式）。

- [ ] `STAFFING_ASSIGNMENT_EVENT_NOT_FOUND`：target_effective_date 下无事件
- [ ] `STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED`：目标已被 rescind，禁止再 correct
- [ ] `STAFFING_ASSIGNMENT_EVENT_ALREADY_CORRECTED`：同 target 已存在不同的 replacement_payload
- [ ] `STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND`：禁止撤销首事件/CREATE（仅允许撤销 UPDATE）
- [ ] `STAFFING_IDEMPOTENCY_REUSED`：幂等键复用但参数不同（沿用既有语义）

#### 9.4.7 触发器与门禁（勾选本子项命中）
- [ ] DB 迁移 / Schema（Atlas+Goose，staffing）：`make staffing plan && make staffing lint && make staffing migrate up`（必要时设置 `DATABASE_URL=...`）
- [ ] 路由治理：新增/调整 internal endpoint 时执行 `make check routing` 并更新 `config/routing/allowlist.yaml`
- [ ] Authz（若新增动作/对象映射）：`make authz-pack && make authz-test && make authz-lint`
- [ ] Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- [ ] `.templ` / Tailwind（若修改 UI 片段/样式）：`make generate && make css`，并确保生成物提交且 `git status --short` 为空
- [ ] sqlc（若 schema/queries/config 变更）：`make sqlc-generate`，并确保生成物提交且 `git status --short` 为空
- [ ] E2E（若把 Correct/Rescind 写进 TP-060-*）：`make e2e`

#### 9.4.8 测试与验收标准（最小可复现）

- [ ] Rescind（delete-slice + stitch）：
  - 准备：同一 assignment 有两条事件 `2026-01-01`（CREATE）与 `2026-02-01`（UPDATE position_id）
  - 操作：rescind `target_effective_date=2026-02-01`
  - 断言：as-of `2026-02-15` 仅剩 `effective_date=2026-01-01` 的 slice；UI 不展示 `end_date`
- [ ] Correct（替换解释）：
  - 准备：`2026-01-01` CREATE 写入 `allocated_fte=0.5`
  - 操作：correct `target_effective_date=2026-01-01`，replacement_payload 将 `allocated_fte` 改为 `0.75`
  - 断言：DB 投射后的 `assignment_versions.allocated_fte` 在 `as_of=2026-01-15` 为 `0.75`（证据可落在 dbtool 或集成测试）
- [ ] fail-closed：
  - 若 correction/rescind 导致 replay 失败（例如让 active assignment 引用 disabled position），操作必须整体失败且不落盘 correction/rescind 记录
- [ ] 幂等：
  - 同一 `event_id` 重试成功且不重复生效；参数不一致必须 `STAFFING_IDEMPOTENCY_REUSED`
- [ ] target 不存在：
  - 对不存在的 `target_effective_date` 执行 correct/rescind：必须 `STAFFING_ASSIGNMENT_EVENT_NOT_FOUND`
- [ ] 不允许撤销 CREATE（当存在后续事件）：
  - 若 `2026-01-01` 为 CREATE 且存在 `2026-02-01` UPDATE：rescind `2026-01-01` 必须失败（`STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND`）
- [ ] correct 与 rescind 的优先级：
  - 若已 rescind 某条事件：再次对同 target 提交 correct 必须报错（`STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED`）

### 9.5 （可选）Go 分层落位（对齐 `DEV-PLAN-015/016`，不改外部行为）

#### 目标
- [ ] 将 `internal/server` 中 Assignments 的 Facade/Handler 迁移到 `modules/staffing`（`services/infrastructure/presentation`），`internal/server` 仅做 wiring；不改变 DB 合同与路由。

#### 验收标准
- [ ] `make check lint` 不新增 go-cleanarch 违规；现有单测/E2E 不回归。
