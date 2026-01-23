# DEV-PLAN-069：移除薪酬社保与考勤（文档/代码/测试/数据库）

**状态**: 草拟中（2026-01-22 09:38 UTC — 按 DEV-PLAN-001 细化到可直接落地）

## 1. 背景与上下文 (Context)

### 1.1 需求来源

仓库当前处于开发早期，数据库仅存在测试数据，系统未上线。现需**彻底移除**：

- 薪酬社保（Payroll + Social Insurance，覆盖 `DEV-PLAN-039`～`046`、`DEV-PLAN-067`～`068`）
- 考勤（Time & Attendance，覆盖 `DEV-PLAN-050`～`056`、`DEV-PLAN-064`～`066`）

并保证 `iam/orgunit/jobcatalog/person/staffing(核心)` 等其它模块不受影响、门禁不退化。

### 1.2 现状落点（实现在哪里）

本仓库并不存在独立的 `modules/payroll` / `modules/attendance`；相关能力主要以“切片/特性”形式分散在：

- HTTP/UI（server）：`internal/server/*attendance*`、`internal/server/*payroll*`、以及导航/路由/鉴权映射
- 考勤对接（worker + lib）：`cmd/attendance-integrations/**`、`internal/attendanceintegrations/**`
- DB（主要在 staffing/person schema 下）：
  - `modules/staffing/infrastructure/persistence/schema/*.sql`（Schema SSOT）
  - `migrations/staffing/*.sql`（Goose 迁移）
  - `modules/person/infrastructure/persistence/schema/*.sql` + `migrations/person/*.sql`（外部身份映射表）
- E2E：`e2e/tests/tp060-04-*`、`tp060-05-*`、`tp060-07-*`、`tp060-08-*`

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 目标（Goals）

- [ ] **彻底删除文档**：删除 payroll/attendance 系列 dev-plan + 相关 dev-record（含 `.docx`），并清理所有引用，避免死链/误导。
- [ ] **彻底删除痕迹**：除 `DEV-PLAN-069`（及其在 `AGENTS.md` 的 Doc Map 条目）外，仓库中不应再出现 payroll/attendance 相关词汇、对象名、路由名与实现痕迹；`docs/dev-records/` 不保留历史痕迹（必须清除/改写）。
- [ ] **彻底删除实现**：删除 payroll/attendance 的路由入口、UI 页面、handler/store/worker/lib、以及相关 Go 测试；`go test ./...` 通过。
- [ ] **彻底删除测试套件**：删除 payroll/attendance 的 E2E spec 与其子计划文档；`make e2e` 在剩余用例下通过且不为 0 tests。
- [ ] **彻底删除数据库对应内容**：
  - 删除 `staffing` 下所有 payroll/attendance 表/函数/RLS policy；
  - 删除 `person.external_identity_links`（考勤集成映射专用表）；
  - 同时移除任职（Assignment）中为 payroll 引入的输入字段（`base_salary`、`currency`）。
- [ ] **不影响其它模块**：保留并验证 `iam/orgunit/jobcatalog/person/staffing(核心)` 的路由、鉴权、RLS/事务与 smoke/E2E 能力。
- [ ] **门禁全绿**：最终 `make preflight` 通过；并满足生成物门禁（`make sqlc-generate` 后工作区干净）。

### 2.2 非目标（Non-Goals）

- 不保留任何“占位页面/占位路由/兼容别名/legacy 回退通道”（对齐 `DEV-PLAN-004M1`）。
- 不引入新模块或新增业务能力；仅做删除与收敛。
- 不做在线数据迁移/保留承诺（仅测试数据；允许环境级重置）。

### 2.3 工具链与门禁（SSOT 引用）

> 入口与脚本实现以 SSOT 为准；本文只声明本计划命中哪些门禁，并给出可复现的执行点。

- [X] 文档门禁：`make check doc`（SSOT：`AGENTS.md`、`scripts/doc/check.sh`）
- [X] Go 门禁：`go fmt ./... && go vet ./... && make check lint && make test`（SSOT：`AGENTS.md`、`Makefile`）
- [X] 路由门禁：`make check routing`（SSOT：`docs/dev-plans/017-routing-strategy.md`）
- [X] Authz：`make authz-pack && make authz-test && make authz-lint`（SSOT：`docs/dev-plans/022-authz-casbin-toolchain.md`）
- [X] DB 闭环：`make <module> plan && make <module> lint && make <module> migrate up`（SSOT：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`）
- [X] sqlc：`make sqlc-generate`（生成物必须提交；SSOT：`AGENTS.md`、`scripts/sqlc/*.sh`）
- [X] E2E：`make e2e`（SSOT：`Makefile`、`scripts/e2e/run.sh`）

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 关键决策（已确认，冻结）

1. [X] **迁移策略：双模块重基线（staffing + person）**  
   - `staffing` 与 `person` 均执行“删旧迁移 + 以 Schema SSOT 重新生成基线”，不新增 drop migration；确保仓库不再残留 payroll/attendance SQL 文本。  
   - 所有已有环境必须执行一次性环境级重置（`make dev-reset` 或等价方式），不支持在线/增量迁移（测试数据、未上线）。
2. [X] **共享字段/约束：一律移除**  
   `staffing` 中凡“仅为 payroll/attendance 服务”的字段/约束/表/函数一律移除；不为“未来可能用得上”保留结构。  
   例外：若发现对象已被 `staffing` 核心能力实际使用，必须在 §8 “盘点清单”中写明保留理由与依赖点。
3. [X] **UI 与路由入口：不保留占位**  
   直接移除入口；旧路径 404；不做 alias/redirect/compat window。
4. [X] **文档痕迹：全仓清理（包含 dev-records）**  
   `docs/dev-records/` 同样属于清理范围：不保留 payroll/attendance 的历史痕迹；对非 payroll/attendance 专用的记录，必须“改写”去除相关字样与上下文引用（保留其它模块证据价值）。

### 3.2 变更策略（分层剥离，先收口入口再删内核）

按“用户可见入口 → 授权/路由治理 → 业务实现 → DB”顺序剥离，避免出现：

- 入口仍可访问但后端已删（500）
- DB 已删但代码仍在查询（运行时崩溃/测试不稳定）
- allowlist/authz 漂移导致路由门禁失败

### 3.3 保护面（必须保持不受影响）

- 核心模块：`iam`、`orgunit`、`jobcatalog`、`person`、`staffing`（除被删除能力外）
- 平台不变量：One Door、显式事务 + 租户注入（No Tx, No RLS）、fail-closed、路由治理门禁、Authz 工具链
- 既有 E2E：至少保留 `m3-smoke`、`tp060-01/02/03` 等非 payroll/attendance 用例

## 4. 数据模型与约束 (Data Model & DB)

### 4.1 需要删除的 DB 对象（清单）

#### 4.1.1 `staffing`：考勤（Attendance）

删除以下表（含 RLS policy / indexes / constraints）：

- `staffing.time_punch_events`
- `staffing.time_punch_void_events`
- `staffing.attendance_recalc_events`
- `staffing.daily_attendance_results`
- `staffing.time_bank_cycles`
- `staffing.time_profile_events`
- `staffing.time_profile_versions`
- `staffing.holiday_day_events`
- `staffing.holiday_days`

删除以下函数（及其间接依赖）：

- `staffing.submit_time_punch_event(...)`
- `staffing.submit_time_punch_void_event(...)`
- `staffing.submit_attendance_recalc_event(...)`
- `staffing.replay_time_profile_versions(...)`
- `staffing.submit_time_profile_event(...)`
- `staffing.submit_holiday_day_event(...)`
- `staffing.recompute_time_bank_cycle(...)`
- `staffing.get_time_profile_for_work_date(...)`
- `staffing.recompute_daily_attendance_result(...)`
- `staffing.recompute_daily_attendance_results_for_punch(...)`

#### 4.1.2 `staffing`：薪酬社保（Payroll + SI + IIT + Retro）

薪酬相关 DB 对象通过“删除 payroll schema SSOT 文件 + migrations 重基线”整体移除（§4.2/§4.3）。范围包括但不限于：

- pay period / payroll run / payslip / payslip item
- social insurance policy + payslip SI items
- IIT（含专项附加扣除、累计预扣算法落库部分）
- payroll recalc（retro）相关表与函数
- payslip item inputs（net-guaranteed IIT 等）

同时必须移除 `staffing` 内任何对 payroll 内核函数的引用（例如 `staffing.maybe_create_payroll_recalc_request_from_assignment_event(...)`）。

#### 4.1.3 `staffing.assignment_versions`：移除 payroll 输入字段

删除以下列与约束，并同步删除 payload 允许键与校验逻辑：

- 列：`assignment_versions.base_salary`
- 列：`assignment_versions.currency`
- 约束：`assignment_versions_base_salary_check`
- 约束：`assignment_versions_currency_check`

保留并继续支持：

- `assignment_versions.allocated_fte`（已用于 capacity 语义）
- `assignment_versions.profile`（若后续发现仅为 payroll 服务，则同样应移除；以盘点为准）

#### 4.1.4 `person`：考勤集成外部身份映射表

删除以下表（含 RLS policy / indexes / constraints）：

- `person.external_identity_links`

删除前需盘点该表的全部引用点（Go/SQL/文档/E2E）；若发现非考勤依赖，按“彻底清除”原则优先移除依赖能力，不新建替代表；如确需新增表，必须先获得用户确认。

并删除相关写入口（例如 `internal/attendanceintegrations` 的 touch/link）。

### 4.2 Schema SSOT 变更点（可直接按文件落地）

#### 4.2.1 删除 payroll schema SSOT 文件（`modules/staffing/.../schema`）

- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00004_staffing_payroll_tables.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00005_staffing_payroll_engine.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00006_staffing_payroll_social_insurance_tables.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00007_staffing_payroll_social_insurance_engine.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00008_staffing_iit_deduction_claims.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00009_staffing_payroll_balances.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00010_staffing_payroll_iit_engine.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00011_staffing_payroll_recalc_tables.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00012_staffing_payroll_recalc_engine.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00013_staffing_payroll_item_inputs.sql`
- [ ] 删除 `modules/staffing/infrastructure/persistence/schema/00014_staffing_payroll_item_inputs_engine.sql`

#### 4.2.2 修改 staffing 表（`modules/staffing/.../schema/00002_staffing_tables.sql`）

- [ ] 从 `staffing.assignment_versions` 移除 `base_salary` / `currency` 相关 `ALTER TABLE ... ADD COLUMN ...` 与对应约束
- [ ] 删除所有 attendance 表（time_punch/time_profile/holiday/time_bank/daily_results/recalc）与对应 RLS policy 段落

#### 4.2.3 修改 staffing 内核（`modules/staffing/.../schema/00003_staffing_engine.sql`）

- [ ] 删除所有 attendance 相关函数（§4.1.1 列表）
- [ ] 在 `replay_assignment_versions` 及相关写入口中移除 `base_salary` / `currency` 的 payload 解析与写入
- [ ] 删除所有对 payroll 钩子函数的调用（`maybe_create_payroll_recalc_request_from_assignment_event`）

#### 4.2.4 修改 person schema SSOT（`modules/person/.../schema/00002_person_persons.sql`）

- [ ] 删除 `person.external_identity_links` 的建表、索引与 RLS policy 段落

### 4.3 migrations 变更点（重基线，避免遗留 SQL 文本）

#### 4.3.1 `staffing` migrations：重建为“无 payroll/attendance”的新基线

- [ ] 删除 `migrations/staffing/*.sql` 与 `migrations/staffing/atlas.sum`
- [ ] 以更新后的 Schema SSOT 生成新迁移（Goose 格式），并生成新的 `atlas.sum`

参考 `DEV-PLAN-024`，建议使用 Atlas 在空目录上“从 0 生成基线迁移”（示例 slug 仅供参考）：

```bash
./scripts/db/run_atlas.sh migrate diff \
  --dir "file://migrations/staffing" --dir-format goose \
  --dev-url "${ATLAS_DEV_URL:-docker://postgres/17/dev}" \
  --to "file://modules/staffing/infrastructure/persistence/schema" \
  staffing-rebaseline-069

./scripts/db/run_atlas.sh migrate hash \
  --dir "file://migrations/staffing" --dir-format goose
```

#### 4.3.2 `person` migrations：重建为“无 external_identity_links”的新基线

- [ ] 删除 `migrations/person/*.sql` 与 `migrations/person/atlas.sum`
- [ ] 以更新后的 Schema SSOT 生成新迁移（Goose 格式），并生成新的 `atlas.sum`

参考 `DEV-PLAN-024`，示例：

```bash
./scripts/db/run_atlas.sh migrate diff \
  --dir "file://migrations/person" --dir-format goose \
  --dev-url "${ATLAS_DEV_URL:-docker://postgres/17/dev}" \
  --to "file://modules/person/infrastructure/persistence/schema" \
  person-rebaseline-069

./scripts/db/run_atlas.sh migrate hash \
  --dir "file://migrations/person" --dir-format goose
```

## 5. 接口契约 (API Contracts)

### 5.1 删除的 UI 路由（server allowlist 对齐）

从 `config/routing/allowlist.yaml` 与 server router 中移除：

- `/org/attendance-punches`（GET/POST）
- `/org/attendance-integrations`（GET/POST）
- `/org/attendance-daily-results`（GET）
- `/org/attendance-daily-results/{person_uuid}/{work_date}`（GET/POST）
- `/org/attendance-time-bank`（GET）
- `/org/attendance-time-profile`（GET/POST）
- `/org/attendance-holiday-calendar`（GET/POST）
- `/org/payroll-periods`（GET/POST）
- `/org/payroll-runs`（GET/POST）
- `/org/payroll-runs/{run_id}`（GET）
- `/org/payroll-runs/{run_id}/calculate`（POST）
- `/org/payroll-runs/{run_id}/finalize`（POST）
- `/org/payroll-runs/{run_id}/payslips`（GET）
- `/org/payroll-runs/{run_id}/payslips/{payslip_id}`（GET）
- `/org/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items`（POST）
- `/org/payroll-social-insurance-policies`（GET/POST）
- `/org/payroll-recalc-requests`（GET）
- `/org/payroll-recalc-requests/{recalc_request_id}`（GET）
- `/org/payroll-recalc-requests/{recalc_request_id}/apply`（POST）

同时删除 UI 导航入口与文案（`internal/server/handler.go` 的 nav + i18n key）。

### 5.2 删除的 Internal API 路由（server allowlist 对齐）

- `/org/api/attendance-punches`（GET/POST）
- `/org/api/attendance-daily-results`（GET）
- `/org/api/attendance-punch-voids`（POST）
- `/org/api/attendance-recalc`（POST）
- `/org/api/payroll-periods`（GET/POST）
- `/org/api/payroll-runs`（GET/POST）
- `/org/api/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items`（POST）
- `/org/api/payroll-balances`（GET）
- `/org/api/payroll-iit-special-additional-deductions`（POST）
- `/org/api/payroll-social-insurance-policies`（GET/POST）
- `/org/api/payroll-recalc-requests`（GET）
- `/org/api/payroll-recalc-requests/{recalc_request_id}`（GET/POST）
- `/org/api/payslips`（GET）
- `/org/api/payslips/{payslip_id}`（GET）

### 5.3 变更的接口（任职输入收敛）

任职相关入口保持，但移除 payroll 输入字段：

- `POST /org/assignments`：表单移除 `base_salary`（以及任何 `currency` 输入）
- `POST /org/api/assignments`：移除 JSON 字段 `base_salary`
- `Correct/Rescind` 的 replacement_payload：移除 `base_salary`/`currency` 的处理与允许键

同时更新对应契约文档（至少 `docs/dev-plans/031-greenfield-assignment-job-data.md`）以移除 payroll 字段定义。

## 6. 核心逻辑与实现步骤 (Execution Plan)

> 本节按“可直接照做”的粒度列出要改哪些文件、如何验证、以及每步的失败路径。

### 6.1 Milestone A：文档收敛（删除 + 清理引用）

1. [ ] 删除 dev-plan 文档（见 §8.1 清单）
2. [ ] 删除 payroll/attendance 专用 dev-record 文档/资产（见 §8.2.1）
3. [ ] 新增 `docs/dev-records/dev-plan-069-execution-log.md`，并在 `AGENTS.md` Doc Map 添加链接
4. [ ] 更新引用入口文档（至少）：
   - [ ] `AGENTS.md`：移除 039-046/050-056/064-068 的 Doc Map 链接；新增 `DEV-PLAN-069` 执行记录链接
   - [ ] `docs/dev-plans/009-implementation-roadmap.md`：移除相关里程碑/引用
   - [ ] `docs/dev-plans/060-business-e2e-test-suite.md`：移除相关用例引用
   - [ ] `docs/dev-records/DEV-PLAN-010-READINESS.md`：移除 payroll/attendance readiness 段落（避免误导）
   - [ ] `docs/dev-plans/031-greenfield-assignment-job-data.md`：移除 `base_salary/currency` 的契约定义与任何 payroll 语义描述
   - [ ] `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`：移除 `base_salary/currency` 与 payroll 依赖引用（保持 tp060-03 作为 Person+Assignments 的核心闭环）
   - [ ] `docs/dev-records/dev-plan-031-execution-log.md`：改写移除 payroll/attendance 字样（保留 Assignments M4 证据）
   - [ ] `docs/dev-records/dev-plan-063-execution-log.md`：改写移除 payroll/attendance 字样（保留 Person+Assignments 证据）
5. [ ] 对 `docs/dev-records/` 做“无痕清理”扫尾：
   - [ ] 删除 payroll/attendance 专用记录（见 §8.2.1）
   - [ ] 改写其它记录中出现的 payroll/attendance 字样与上下文（见 §8.2.2；以 `rg` 盘点为准）
6. [ ] 验证：`make check doc`

### 6.2 Milestone B：E2E 套件收敛（删除 spec + 清理引用）

1. [ ] 删除 E2E spec：
   - `e2e/tests/tp060-04-attendance-punch-ledger.spec.js`
   - `e2e/tests/tp060-05-attendance-4b-4e.spec.js`
   - `e2e/tests/tp060-07-payroll-041-043.spec.js`
   - `e2e/tests/tp060-08-payroll-044-046.spec.js`
2. [ ] 验证：`make e2e`（确保仍能发现 tests，且剩余用例通过）

### 6.3 Milestone C：路由与 UI 入口收敛（先让入口消失）

1. [ ] 更新 `config/routing/allowlist.yaml`：移除 §5.1/§5.2 的全部条目
2. [ ] 同步清理路由治理断言与测试（含 route_class/responder 映射或分类配置），确保门禁基线不漂移
3. [ ] 更新 server 导航与翻译键（`internal/server/handler.go`）：移除 payroll/attendance nav 项与 i18n 文案
4. [ ] 更新 router wiring：移除 payroll/attendance 的 handler 注册（`internal/server/handler.go` 及相关测试）
5. [ ] 调整 `HandlerOptions`/默认 wiring：删除 payroll/attendance 的 store 依赖，并移除 `attendanceConfigStore` 的强制校验（避免“入口已删但 handler 初始化仍失败”）
6. [ ] 验证：`make check routing`

### 6.4 Milestone D：Authz 收敛（对象、策略、路由映射）

1. [ ] 更新对象注册（`pkg/authz/registry.go`）：移除所有 `staffing.attendance-*` 与 `staffing.payroll-*` / `staffing.payslips`
2. [ ] 更新路由到权限映射（`internal/server/authz_middleware.go` + tests）：删除 payroll/attendance 分支
3. [ ] 更新策略源（`config/access/policies/00-bootstrap.csv`）：删除 payroll/attendance 相关 `p, ...` 行；运行 `make authz-pack` 以重建 `config/access/policy.csv` 与 `config/access/policy.csv.rev`
4. [ ] 验证：`make authz-pack && make authz-test && make authz-lint`

### 6.5 Milestone E：Go 实现删除（handler/store/worker/lib/dbtool）

1. [ ] 删除 server 侧实现与测试（文件名包含 payroll/attendance 的一组；见 §8.3）
   - [ ] 同步删除 external identity link store（`internal/server/external_identity_links.go` + tests），并从 `PersonStore` 接口中移除 ExternalIdentity* 方法（避免残留 DB 依赖）
2. [ ] 删除考勤对接 worker 与库：
   - [ ] `cmd/attendance-integrations/**`
   - [ ] `internal/attendanceintegrations/**`
3. [ ] 删除薪酬算法包：
   - [ ] `pkg/payroll/**`
4. [ ] 调整任职输入（移除 base_salary/currency）：
   - [ ] `internal/server/staffing_handlers.go`（UI 表单与 replacement_payload）
   - [ ] `modules/staffing/presentation/controllers/assignments_api.go`
   - [ ] `modules/staffing/domain/ports/assignment_store.go`
   - [ ] `modules/staffing/services/assignments_facade.go`
   - [ ] `modules/staffing/infrastructure/persistence/assignment_pg_store.go`
   - [ ] 相关 Go tests / E2E（例如 `e2e/tests/tp060-03-person-and-assignments.spec.js` 若仍写 base_salary）
5. [ ] 调整 `cmd/dbtool` 的 `staffing-smoke`：移除所有 payroll 相关 smoke（避免依赖被删除的表/函数/包）
6. [ ] 验证：`go fmt ./... && go vet ./... && make check lint && make test`

### 6.6 Milestone F：DB 重基线（staffing/person）+ sqlc 生成物收敛

1. [ ] 更新 Schema SSOT（§4.2）
2. [ ] 重建 `migrations/staffing`（§4.3.1），并更新 `atlas.sum`
3. [ ] 重建 `migrations/person`（§4.3.2），并更新 `atlas.sum`
4. [ ] 清理 sqlc 源文件与配置：更新 `sqlc.yaml` 的 schema/queries 列表，删除或改写涉及 payroll/attendance 的 SQL 查询文件（若存在），避免生成失败
5. [ ] 一次性环境级重置（推荐使用 repo 提供入口）：
   - [ ] `make dev-reset`（会 drop docker volumes；仅测试环境）
6. [ ] 模块闭环验证：
   - [ ] `make staffing plan && make staffing lint && make staffing migrate up`
   - [ ] `make person plan && make person lint && make person migrate up`
7. [ ] `make sqlc-generate`，并确认 `git status --short` 为空

### 6.7 Milestone G：全仓库扫尾（无残留 + CI 对齐）

1. [ ] 关键词扫尾（强制，无痕）：除以下文件外，仓库中不应再出现 `payroll|payslip|social_insurance|iit|attendance|time_punch|time_bank|dingtalk|wecom`：
   - `docs/dev-plans/069-remove-payroll-attendance.md`
   - `AGENTS.md`（仅允许保留 `DEV-PLAN-069` 的 Doc Map 条目文字）
2. [ ] 运行：`make preflight`

### 6.8 验收标准（Definition of Done）

- [ ] 目标文档已删除且无死链：§8.1 与 §8.2.1 清单全部从仓库移除；§8.2.2 清单完成改写；`AGENTS.md` Doc Map 与 roadmap/e2e 套件文档无引用残留
- [ ] 仓库无痕：Milestone G 的关键词扫尾通过（`docs/dev-records/` 不保留 payroll/attendance 历史痕迹）
- [ ] 目标代码已删除：§8.3 清单对应实现与依赖均移除；`go test ./...` 通过
- [ ] 目标路由已删除：§5.1/§5.2 列表全部不可达（404），且 `make check routing` 通过
- [ ] Authz 已收敛：无 payroll/attendance 对象与策略残留；`make authz-pack && make authz-test && make authz-lint` 通过
- [ ] DB 已收敛：
  - `staffing` 不再包含 payroll/attendance 的表/函数/策略，并移除 `base_salary/currency`；`make staffing plan/lint/migrate up` 通过
  - `person` 不再包含 `person.external_identity_links`；`make person plan/lint/migrate up` 通过
- [ ] sqlc 源文件与生成物无漂移：`sqlc.yaml`/queries 无 payroll/attendance 残留，`make sqlc-generate` 后 `git status --short` 为空
- [ ] CI 对齐：`make preflight` 通过

### 6.9 运维与监控（Ops & Monitoring）

本计划为删除与收敛，不引入任何运维/监控开关；对齐 `AGENTS.md` “早期阶段避免过度运维与监控”的约束。

## 7. 风险与失败路径 (Risks & Mitigations)

- 风险：`staffing/person` 重基线后，本地数据库迁移版本表与文件不一致，导致 migrate 失败。
  - 缓解：强制一次性 `make dev-reset`（或手工重建数据库）；把执行记录写入 §9。
- 风险：`staffing` 内核存在 payroll 钩子调用（例如 `maybe_create_payroll_recalc_request_from_assignment_event`），删库后会在写入时触发运行时错误。
  - 缓解：在删 DB 前先移除内核钩子调用，并用 `make staffing migrate up` 的 smoke + 单测覆盖验证写链路。
- 风险：allowlist/authz/路由 wiring 三者漂移导致 `make check routing` 或 authz 测试失败。
  - 缓解：按 Milestone C→D 顺序逐步收敛，并在每步后跑对应门禁。

## 8. 附录：待删除/修改清单（落地用）

### 8.1 待删除：dev-plan 文档

- `docs/dev-plans/039-payroll-social-insurance-implementation-roadmap.md`
- `docs/dev-plans/040-payroll-social-insurance-module-design-blueprint.md`
- `docs/dev-plans/041-payroll-p0-slice-pay-period-and-payroll-run.md`
- `docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`
- `docs/dev-plans/043-payroll-p0-slice-social-insurance-policy-and-calculation.md`
- `docs/dev-plans/044-payroll-p0-slice-iit-cumulative-withholding-and-balances.md`
- `docs/dev-plans/045-payroll-p0-slice-retroactive-accounting.md`
- `docs/dev-plans/046-payroll-p0-slice-net-guaranteed-iit-tax-gross-up.md`
- `docs/dev-plans/050-hrms-attendance-blueprint.md`
- `docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`
- `docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`
- `docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`
- `docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`
- `docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`
- `docs/dev-plans/056-attendance-slice-4f-dingtalk-wecom-integration.md`
- `docs/dev-plans/064-test-tp060-04-attendance-4a-punch-ledger.md`
- `docs/dev-plans/065-test-tp060-05-attendance-4b-4e-results-config-bank-corrections.md`
- `docs/dev-plans/066-test-tp060-06-attendance-4f-integrations-identity-mapping.md`
- `docs/dev-plans/067-test-tp060-07-payroll-041-043-run-payslip-si.md`
- `docs/dev-plans/068-test-tp060-08-payroll-044-046-iit-retro-net-guarantee.md`

### 8.2 待删除：dev-record 文档/资产
#### 8.2.1 待删除：payroll/attendance 专用记录

- `docs/dev-records/HRMS考勤蓝图设计.docx`
- `docs/dev-records/dev-plan-053-execution-log.md`
- `docs/dev-records/dev-plan-054-execution-log.md`
- `docs/dev-records/dev-plan-055-execution-log.md`
- `docs/dev-records/dev-plan-056-execution-log.md`
- `docs/dev-records/dev-plan-066-execution-log.md`

#### 8.2.2 待改写：清除 payroll/attendance 历史痕迹（保留其它模块证据）

- `docs/dev-records/dev-plan-031-execution-log.md`
- `docs/dev-records/dev-plan-063-execution-log.md`
- 以及 `docs/dev-records/` 下所有通过 `rg` 盘点命中的其它文件（以 Milestone A 的盘点结果为准）

### 8.3 待删除：Go 实现（server）

> 实际删除以编译与测试通过为准；以下为“命名上明确属于 payroll/attendance”的基线清单。

- `internal/server/attendance.go`
- `internal/server/attendance_config.go`
- `internal/server/attendance_config_handlers.go`
- `internal/server/attendance_config_handlers_test.go`
- `internal/server/attendance_config_store_test.go`
- `internal/server/attendance_daily_results_placeholders_test.go`
- `internal/server/attendance_db_integration_test.go`
- `internal/server/attendance_handlers.go`
- `internal/server/attendance_handlers_test.go`
- `internal/server/attendance_integrations_handlers.go`
- `internal/server/attendance_integrations_handlers_test.go`
- `internal/server/attendance_store_test.go`
- `internal/server/attendance_time_bank_store_test.go`
- `internal/server/attendance_time_bank_test.go`
- `internal/server/handler_attendance_config_routes_test.go`
- `internal/server/handler_attendance_config_test.go`
- `internal/server/handler_attendance_time_bank_authz_test.go`
- `internal/server/payroll.go`
- `internal/server/payroll_handlers.go`
- `internal/server/payroll_handlers_test.go`
- `internal/server/payroll_net_guaranteed_iit_db_integration_test.go`
- `internal/server/payroll_store_test.go`
- `internal/server/external_identity_links.go`
- `internal/server/external_identity_links_test.go`

### 8.4 待删除：Go 实现（worker/lib/pkg）

- `cmd/attendance-integrations/`
- `internal/attendanceintegrations/`
- `pkg/payroll/`

### 8.5 待修改：关键收口点（路由/鉴权/任职输入/DB）

- 路由 allowlist：`config/routing/allowlist.yaml`
- Authz 策略源：`config/access/policies/00-bootstrap.csv`
- Authz 对象注册：`pkg/authz/registry.go`
- Authz 路由映射：`internal/server/authz_middleware.go`
- UI 导航：`internal/server/handler.go`
- PersonStore 契约收敛（移除 ExternalIdentity*）：`internal/server/person.go`（以及相关 stub tests）
- 任职输入（移除 `base_salary/currency`）：`internal/server/staffing_handlers.go` + `modules/staffing/**`
- dbtool smoke 收敛：`cmd/dbtool/main.go`
- Schema SSOT：`modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`、`modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`、`modules/person/infrastructure/persistence/schema/00002_person_persons.sql`
- migrations 重基线：`migrations/staffing/`、`migrations/person/`
- sqlc 源配置：`sqlc.yaml`（以及 `modules/**/infrastructure/sqlc/queries/*.sql` 若存在）

## 9. Readiness / 执行记录（实施时补齐）

> 按 `DEV-PLAN-000`：记录写入 `docs/dev-records/dev-plan-069-execution-log.md`，每条包含时间戳、命令与结果；全部完成后更新状态为“已完成”。

1. [ ] 记录：`make check doc`
2. [ ] 记录：`make e2e`
3. [ ] 记录：`make check routing`
4. [ ] 记录：`make authz-pack && make authz-test && make authz-lint`
5. [ ] 记录：Go 门禁（fmt/vet/lint/test）
6. [ ] 记录：DB（`make staffing plan && make staffing lint && make staffing migrate up`；`make person plan && make person lint && make person migrate up`）
7. [ ] 记录：`make sqlc-generate`（并确认工作区干净）
8. [ ] 记录：`make preflight`
