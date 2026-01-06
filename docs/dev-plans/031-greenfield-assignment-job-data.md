# DEV-PLAN-031：任职记录（Job Data / Assignments）全新实现（Staffing，事件 SoT + 同步投射）

**状态**: 草拟中（2026-01-05 03:38 UTC）

## 1. 背景与上下文 (Context)

本计划目标是**全新实现**“任职记录（Job Data / Assignments）”能力（Greenfield），并对齐：
- 技术路线（`DEV-PLAN-026/029/030`）：**DB=Projection Kernel（权威）**、同步投射（同事务 delete+replay）、One Door Policy（唯一写入口）、Valid Time=DATE、`daterange` 统一使用左闭右开 `[start,end)`。
- DDD/分层与模块骨架（`DEV-PLAN-015`、`DEV-PLAN-016`）：采用 4 模块（`orgunit/jobcatalog/staffing/person`），其中任职记录归属 `modules/staffing`。

本仓库当前尚未落盘任职记录与 Position 的实现（`/org/assignments` 仍为 placeholder），因此本计划按 Greenfield 口径推进：不做“存量盘点/承接/退场策略”。若未来需要承接外部存量系统或历史实现，必须另立 dev-plan 并明确迁移/回滚策略（对齐 `AGENTS.md` 的 No Legacy 与 Contract First）。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标
- [ ] 在 `modules/staffing` 内全新实现任职记录：**事件 SoT（`*_events`）+ 同步投射（`*_versions`）**，并由 DB Kernel 强制关键不变量。
- [ ] UI/列表展示层：任职记录**仅显示生效日期（effective date）**；底层仍使用 `daterange` 的左闭右开 `[start,end)` 表达有效期（不改为闭区间）。
- [ ] 以清晰契约替代“隐式耦合”：
  - 写路径输入统一使用 `person_uuid`（对齐 016），不再以 pernr 作为写侧主键；
  - person 的 pernr→uuid 解析由 `modules/person` 提供 options/read API，`staffing` 不直读 `persons` 表（Person Identity 合同见 `DEV-PLAN-027`）。
- [ ] 对齐 `DEV-PLAN-009M2` 的 M2 MVP：先交付 `primary` + `CREATE`（可选 `UPDATE`）的最小闭环，并把 `TRANSFER/TERMINATE/RESCIND/DELETE-SLICE` 等复杂动作显式延后（避免实现期补丁式堆叠）。
- [ ] 实现需满足 015/016 的 DDD 分层与 One Door Policy：Go=Facade（鉴权/事务/调用/错误映射），DB=Kernel（裁决/投射/重放）。

### 2.2 非目标（明确不做）
- 不做任何存量系统迁移/兼容/cutover（Greenfield）；如需承接外部存量必须另立 dev-plan。
- 不引入 `effseq`，同一实体同日最多一条事件（对齐 026/030/029）。
- 不在本计划内实现“跨模块异步事件/旧 outbox/audit/settings 支撑能力”的兼容；如需要另立计划。

## 2.3 工具链与门禁（SSOT 引用）
- DDD 分层框架（Greenfield）：`docs/dev-plans/015-ddd-layering-framework.md`
- Greenfield HR 模块骨架（4 模块）：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- Kernel 边界与 daterange 口径：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/032-effective-date-day-granularity.md`
- 分层/依赖门禁：`.gocleanarch.yml`（入口：`make check lint`）
- 命令入口与 CI：`Makefile`、`.github/workflows/quality-gates.yml`

## 3. 现状功能盘点（本仓库不适用）

本仓库当前未包含任何“存量任职记录实现”可供盘点；本计划按 Greenfield 口径直接落地新实现。
若未来需要盘点/承接外部存量系统或历史实现，必须另立 dev-plan 并明确迁移、回滚与门禁（对齐 `AGENTS.md` 的 No Legacy / Contract First）。

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

### 5.2 目录骨架（对齐 015/016）
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

### 5.3 Kernel 边界（与 026-029 同构）
- **DB = Projection Kernel（权威）**：插入事件（幂等）→ 同事务全量重放生成 versions → 裁决不变量。
- **Go = Command Facade**：鉴权/事务边界 + 调 Kernel + 错误映射到 `pkg/serrors`。

## 6. 方案（新实现方式：事件 SoT + 同步投射）

### 6.1 领域建模：以“时间线聚合（timeline aggregate）”作为写侧单位

本计划将“任职记录”建模为 `person_uuid + assignment_type` 的时间线聚合（至少覆盖 `primary`；扩展类型可选）：
- 聚合标识：`assignment_timeline_id`（可用 uuid，或从 `tenant_id+person_uuid+assignment_type` 派生稳定 uuid）
- 时间线内的每个有效片段（version slice）记录岗位、组织、FTE、就业状态等业务属性。

优势：
- 与“同日最多一条事件/无 overlap/gapless/最后一段无穷”天然对齐。
- 转移/终止/再入职等都成为“时间线上的事件”，由 Kernel 统一裁决。

### 6.2 数据模型（草案；DDL 以实施时 schema SSOT 为准）

> 注意：本节仅为目标合同草案，不在本计划内落盘；新增表/迁移须另开实施计划并获得手工确认（见仓库约束）。

**Write Side（SoT）**
- `assignment_timeline_events`
  - `tenant_id uuid`
  - `event_id uuid`（幂等键，unique）
  - `timeline_id uuid`
  - `event_type text`（例如 `CREATE/UPDATE/TRANSFER/TERMINATE/CORRECT/RESCIND`，以合同冻结）
  - `effective_date date`（同日唯一：`(tenant_id,timeline_id,effective_date)` unique）
  - `payload jsonb`（变化字段、reason_code/note 等）
  - `request_id text`、`initiator_id uuid`、`transaction_time timestamptz`

**Read Model（Projection）**
- `assignment_timeline_versions`
  - `tenant_id uuid`
  - `timeline_id uuid`
  - `validity daterange`（`[start,end)`，最后一段 `upper_inf=true`）
  - `position_id uuid`（Staffing 内部实体/引用）
  - `org_unit_id uuid`（引用 orgunit）
  - `allocated_fte numeric`
  - `employment_status text`（`active/inactive/...`）
  - `meta jsonb`（可选：保存必要的 label 快照）
  - 约束：
    - `EXCLUDE USING gist (tenant_id, timeline_id, validity &&)`（no-overlap）
    - gapless（commit-time gate）：相邻切片严丝合缝，最后一段 infinity

### 6.3 Kernel 入口（唯一写入口）
- `submit_assignment_timeline_event(...)`：
  - 幂等：同 `event_id` 重试不重复写入
  - 插入事件后：`DELETE FROM assignment_timeline_versions WHERE tenant_id=? AND timeline_id=?`，再基于 events 全量重放
  - 同事务内执行 `validate_assignment_timeline_*`（gapless/no-overlap/跨聚合不变量等）

### 6.4 只展示生效日期（UI/读接口形状）
- Timeline 列表行只渲染：
  - `effective_date = lower(validity)`
  - 以及岗位/组织/职类等**业务属性**（允许显示）
  - 不展示 `end_date`/`upper(validity)`（避免闭区间混用）
- as-of 查询仍使用 `validity @> $as_of::date` 保证语义一致。

## 7. 功能映射：存量能力 → 新方案

> 本节把 §3 的存量能力逐项映射到新方案的实现方式（保留/替代/不做）。

1) 创建任职（CreateAssignment）
- 新方案：通过 `submit_assignment_timeline_event(event_type='CREATE', effective_date=...)` 创建/更新时间线的第一个版本或新版本。
- 输入主键：使用 `person_uuid`（pernr 仅用于 UI 查询与展示，不进入写侧合同）。

2) 更新任职（UpdateAssignment）
- 新方案：统一为事件写入（例如 `event_type='UPDATE'`），由 Kernel 决定切片 split/截断。
- Go 不再“先查当前 slice 再补丁式截断”；避免第二套时间线算法。

3) Correct（就地更正）
- 新方案：定义为 `event_type='CORRECT'`，但仍遵循“同日唯一”规则；若需要同日多次修正，必须提升为不同的业务事件（本计划不引入 effseq）。

4) Rescind（撤销）
- 新方案：定义为 `event_type='RESCIND'`，由 Kernel 将某日之后的状态切片化为 `inactive` 或回滚到前态；具体语义需在子域实现计划中冻结（避免隐式复杂分支）。

5) Delete slice + stitch
- 新方案：不暴露“直接删 versions”的能力；如需“删除某日变更”，通过 `RESCIND` 或 `CORRECT` 事件表达（One Door Policy）。

6) Transfer / Termination（transition）
- 新方案：不再依赖独立的 `org_personnel_events` 表作为裁决入口；转移/终止应成为 assignment timeline 的事件类型（或成为 staffing 内的人员事件，但必须同构为事件 SoT）。

7) Primary gap-free / no-overlap
- 新方案：以 versions 的 `daterange [)` + DB gate 强制；展示层只用 `effective_date`。

8) 与 Position/OrgUnit/Job Catalog 的 join 与显示
- 新方案：读侧允许 join，但写侧必须避免跨模块“隐式查询”：
  - orgunit/jobcatalog 的 label 建议走 read API 或 `pkg/orglabels` 类共享投射能力；
  - 如确需快照，为保证历史一致性，可把必要 label 写入 versions 的 `meta`（需在实现计划中明确范围，避免无限膨胀）。

## 8. 里程碑与验收（Plan → Implement 的承接）

1. [ ] 冻结事件类型枚举、payload 合同、错误契约（SQLSTATE/constraint/stable code）。
2. [ ] 冻结 `modules/staffing` 的路由（UI+API）与输入输出契约（只展示 effective_date）。
3. [ ] 冻结 DB Kernel：`submit_*_event` + `replay_*_versions` + `validate_*` 的职责矩阵（对齐 026-029）。
4. [ ] 定义最小测试集：
   - 同日唯一
   - no-overlap
   - gapless（最后一段 infinity）
   - 转移/终止/撤销（失败路径可解释）
5. [ ] 通过相关门禁（引用 `AGENTS.md` 触发器矩阵）。
