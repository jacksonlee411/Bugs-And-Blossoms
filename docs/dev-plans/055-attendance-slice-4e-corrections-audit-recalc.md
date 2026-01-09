# DEV-PLAN-055：考勤 Slice 4E——纠错与审计闭环（更正事件 + 重算）

**状态**: 草拟中（2026-01-09 14:28 UTC）

> 目标：按 `docs/dev-plans/001-technical-design-template.md` 补齐到“无需再做设计决策即可开工”的细化程度（Level 4-5）。

## 1. 背景与上下文 (Context)

- **需求来源**：`docs/dev-plans/050-hrms-attendance-blueprint.md`（Slice 4E）。
- **上游依赖**：
  - `docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`（输入 SoT：`staffing.time_punch_events` + kernel `staffing.submit_time_punch_event(...)`）。
  - `docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`（日结果读模：`staffing.daily_attendance_results`；并已明确 Option A：4B 不提供 UI 手工重算入口）。
  - `docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`（TimeProfile/HolidayCalendar SSOT + 日结果追溯锚点）。
  - `docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`（额度/周期累加器；要求“日结果重算联动周期重算”）。
- **范围定位**：对齐 `docs/dev-plans/009-implementation-roadmap.md` Phase 4 的“业务垂直切片：业务 + UI 同步交付”。
- **模块/落点（选定）**：
  - DB：`staffing` schema（Schema SSOT：`modules/staffing/infrastructure/persistence/schema/*.sql`；迁移：`migrations/staffing/*`）。
  - App：tenant app `/org/*` UI（在现有日结果详情页补齐“纠错+审计+重算”入口）；代码落点：`internal/server/*`（与 punches/daily results 同构）。
- **业务价值**：
  - 考勤天然是“持续纠错 + 追溯重算”的系统：补打卡、撤销/作废、规则/日历调整都会改变历史结果；没有“可重算 + 可追溯”的能力就无法合规解释与对账。
  - 本切片把“纠错/重算”收敛为 **事件化写入（One Door）+ 同事务同步投射（写后读强一致）+ 可见审计链路**，避免通过 UPDATE/手工脚本“修表”形成第二写入口。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标（Done 的定义）

- [ ] **One Door（更正/重算写入口唯一）**：
  - 作废打卡（void）必须通过 DB Kernel `staffing.submit_time_punch_void_event(...)`。
  - 手工/管理重算必须通过 DB Kernel `staffing.submit_attendance_recalc_event(...)`（记录可审计事件 + 同事务执行 bounded replay）。
  - 应用层禁止直写 `staffing.time_punch_void_events` / `staffing.attendance_recalc_events` / `staffing.daily_attendance_results` / `staffing.time_bank_cycles`。
- [ ] **打卡撤销不更新 SoT**：不 UPDATE/DELETE `staffing.time_punch_events`；作废以新事件表达（append-only），并在计算/展示时排除已作废 punches。
- [ ] **bounded replay（可界定的重算边界）**：
  - 作废 punch：按 punch 的 `punch_time` 归属，复用 `staffing.recompute_daily_attendance_results_for_punch(...)`，仅重算 `local_date(punch_time)-1` 与 `local_date(punch_time)` 两天（对齐 4B 的跨天口径）。
  - 手工重算：仅支持 **按人 + 日期范围**，并在 DB 侧强限制范围大小（MVP：最多 31 天（含首尾；`range_days = (to_date - from_date) + 1`）），避免全量历史重放。
- [ ] **额度联动（对齐 4D）**：任何导致 `daily_attendance_results` 重算的路径，都必须在同一事务内联动更新 `time_bank_cycles`（实现方式对齐 4D：在 `recompute_daily_attendance_result(...)` 内部调用 `recompute_time_bank_cycle(...)`）。
- [ ] **UI 可见可操作（至少一条端到端链路）**：在 `/org/attendance-daily-results/{person_uuid}/{work_date}` 日结果详情页：
  - 展示“审计链路”：该日计算窗口、输入 punches（含作废标记）、本次计算锚点（`computed_at` + watermarks + TimeProfile/Holiday 锚点）、手工重算事件（如有）。
  - 提供“作废 punch”操作入口，并在提交后可见结果变化（`computed_at` 更新 + 日结果字段变化）。
  - 提供“重算本日/范围重算（按人）”入口，用于规则/日历变更后的回填。
- [ ] **可拒绝（安全）**：RLS `ENABLE + FORCE`；`AUTHZ_MODE=enforce` 下，未授权 read/admin 必须统一 403 拒绝（对齐 `docs/dev-plans/021-...`、`docs/dev-plans/022-...`）。

### 2.2 非目标（Out of Scope）

- 不实现复杂审批流、多角色签核、申诉工单系统；本切片只交付“可纠错、可追溯、可重算”的技术底座与最小 UI 入口。
- 不引入异步队列/后台调度器来跑大规模重算（对齐 `AGENTS.md` §3.6）；MVP 仅支持按人小范围同步重算。
- 不提供“编辑 punch”或 “unvoid（撤销作废）”能力；更正口径为：**作废旧 punch + 新增正确 punch**。
- 不支持 `DAY_RESULT_ADJUSTED`（直接改日结果）写入口：避免绕过规则导致权威表达分裂；如确需必须另立 dev-plan 并明确不变量与审计口径。
- 不实现跨租户、跨组织的大范围 backfill 工具（如“全租户全员重算”）；若后续需要，必须以明确的限流/分批/可中断/可追溯为前置契约另立计划。

### 2.3 工具链与门禁（SSOT 引用）

- **触发器清单（本计划命中）**：
  - [X] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [X] DB 迁移 / Schema（Atlas+Goose，`make staffing plan && make staffing lint && make staffing migrate up`）
  - [X] sqlc（`make sqlc-generate`；并确保 `git status --short` 为空）
  - [X] 路由治理（`make check routing`；必要时更新 `config/routing/allowlist.yaml`）
  - [X] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [ ] E2E（如将本切片纳入 smoke：`make e2e`）

- **SSOT 链接**：
  - 触发器矩阵与红线：`AGENTS.md`
  - CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
  - RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
  - 路由：`docs/dev-plans/017-routing-strategy.md`
  - Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
  - 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
  - 上游切片：`docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`
  - 蓝图：`docs/dev-plans/050-hrms-attendance-blueprint.md`

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 架构图 (Mermaid)

```mermaid
graph TD
  UI1[UI: /org/attendance-daily-results/{person}/{work_date}] --> H1[Handler: GET/POST]
  H1 --> S1[Store: tx + SET LOCAL app.current_tenant]

  S1 --> Q1[Query: daily result + audit timeline]
  Q1 --> DR[(staffing.daily_attendance_results)]
  Q1 --> P[(staffing.time_punch_events)]
  Q1 --> PV[(staffing.time_punch_void_events)]
  Q1 --> RE[(staffing.attendance_recalc_events)]

  H1 -->|POST: void| K1[DB Kernel: submit_time_punch_void_event]
  K1 --> PV
  K1 --> PRJ1[Projection: recompute_daily_attendance_results_for_punch]
  PRJ1 --> DR

  H1 -->|POST: recalc| K2[DB Kernel: submit_attendance_recalc_event]
  K2 --> RE
  K2 --> PRJ2[Projection: recompute_daily_attendance_result (loop)]
  PRJ2 --> DR

  DR -->|4D hook| TB[(staffing.time_bank_cycles)]
```

### 3.2 关键设计决策（ADR 摘要）

- **撤销/作废采用独立事件 SoT（选定）**：新增 `staffing.time_punch_void_events`（append-only），用于表达“某条 punch 已作废”；不更新 `staffing.time_punch_events`。
- **“有效 punches”由计算时过滤（选定）**：修改 `staffing.recompute_daily_attendance_result(...)`，在扫描 punches 时排除 `time_punch_void_events` 命中的事件（见 §4.2.1）。
- **手工重算也事件化（选定）**：新增 `staffing.attendance_recalc_events` 记录“谁在何时对谁的哪段日期做了重算”；并在 kernel 同事务执行 bounded replay。
- **bounded replay 的硬边界（选定）**：手工重算范围由 DB 强限制（MVP：31 天/人/次（含首尾）），避免 UI 层绕过限制造成长事务或锁放大。
- **审计链路不引入独立系统（选定）**：审计视图基于事件表（punch + void + recalc）与读模字段（`computed_at/input_watermark_*/*_last_event_id`）组合展示；不引入额外审计组件（对齐 `AGENTS.md` §3.6）。
- **时区口径（沿用 4B/4C）**：`work_date date` 与 punches 归属统一按 `Asia/Shanghai` 解释；UI 展示同口径。

## 4. 数据模型与约束 (Data Model & Constraints)

> 红线：新增数据库表（`CREATE TABLE`）与对应迁移落地前，必须获得你手工确认（`AGENTS.md` §3.2）。

### 4.1 Schema 定义（SQL；落地到 `modules/staffing/infrastructure/persistence/schema/*.sql`）

#### 4.1.0 新增表清单（需手工确认）

- `staffing.time_punch_void_events`
- `staffing.attendance_recalc_events`

**建表批准记录（落迁移前必须完成）**
- [x] 已批准新增以上 2 张表（批准人：shangmeilin；时间：2026-01-09；证据：本对话确认“你获得全部授权”，按 DEV-PLAN-055 实施）。

#### 4.1.1 `staffing.time_punch_void_events`（打卡作废事件 SoT，append-only）

```sql
CREATE TABLE IF NOT EXISTS staffing.time_punch_void_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,

  -- 目标 punch（引用 punches SoT）
  target_punch_event_db_id bigint NOT NULL,
  target_punch_event_id uuid NOT NULL,

  -- 审计扩展字段：必须是 object（禁止 array/scalar）
  payload jsonb NOT NULL DEFAULT '{}'::jsonb, -- 例如：{"reason":"误打卡","source":"ui"}

  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT time_punch_void_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT time_punch_void_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT time_punch_void_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT time_punch_void_events_target_unique UNIQUE (tenant_id, target_punch_event_db_id)
);

CREATE INDEX IF NOT EXISTS time_punch_void_events_person_created_idx
  ON staffing.time_punch_void_events (tenant_id, person_uuid, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS time_punch_void_events_target_idx
  ON staffing.time_punch_void_events (tenant_id, target_punch_event_db_id);

ALTER TABLE staffing.time_punch_void_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.time_punch_void_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.time_punch_void_events;
CREATE POLICY tenant_isolation ON staffing.time_punch_void_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
```

#### 4.1.2 `staffing.attendance_recalc_events`（按人按日期范围的手工重算事件 SoT）

```sql
CREATE TABLE IF NOT EXISTS staffing.attendance_recalc_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  from_date date NOT NULL,
  to_date date NOT NULL,

  -- 审计扩展字段：必须是 object（禁止 array/scalar）
  payload jsonb NOT NULL DEFAULT '{}'::jsonb, -- 例如：{"reason":"TimeProfile 更新回填","source":"ui"}

  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT attendance_recalc_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT attendance_recalc_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT attendance_recalc_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT attendance_recalc_events_date_range_check CHECK (to_date >= from_date),
  -- 范围天数口径：range_days = (to_date - from_date) + 1；MVP 上限 31 天（含首尾）
  CONSTRAINT attendance_recalc_events_range_size_check CHECK ((to_date - from_date) <= 30)
);

CREATE INDEX IF NOT EXISTS attendance_recalc_events_person_range_idx
  ON staffing.attendance_recalc_events (tenant_id, person_uuid, from_date, to_date, id);

CREATE INDEX IF NOT EXISTS attendance_recalc_events_created_idx
  ON staffing.attendance_recalc_events (tenant_id, created_at DESC, id DESC);

ALTER TABLE staffing.attendance_recalc_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.attendance_recalc_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.attendance_recalc_events;
CREATE POLICY tenant_isolation ON staffing.attendance_recalc_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
```

### 4.2 Kernel：同步投射与 bounded replay（SQL；落地到 `modules/staffing/infrastructure/persistence/schema/*.sql`）

#### 4.2.1 修改 `staffing.recompute_daily_attendance_result(...)`：窗口口径单一化 + 忽略已作废 punches

##### 4.2.1A 新增 helper：`staffing.get_time_profile_for_work_date(...)`（单一事实源：shift + window）

> 目的：避免在 kernel 与 UI 各自硬编码 punches 扫描窗口（例如 6h/12h）导致口径漂移；窗口边界只在该函数内定义，其他代码只消费输出。

```sql
CREATE OR REPLACE FUNCTION staffing.get_time_profile_for_work_date(
  p_tenant_id uuid,
  p_work_date date
)
RETURNS TABLE (
  shift_start_local time,
  shift_end_local time,
  late_tolerance_minutes int,
  early_leave_tolerance_minutes int,
  overtime_min_minutes int,
  overtime_rounding_mode text,
  overtime_rounding_unit_minutes int,
  time_profile_last_event_id bigint,
  shift_start timestamptz,
  shift_end timestamptz,
  window_start timestamptz,
  window_end timestamptz
)
LANGUAGE plpgsql
AS $$
DECLARE
  v_tz text := 'Asia/Shanghai';
  v_window_before interval := interval '6 hours';
  v_window_after interval := interval '12 hours';
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_work_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'work_date is required';
  END IF;

  SELECT
    tp.shift_start_local,
    tp.shift_end_local,
    tp.late_tolerance_minutes,
    tp.early_leave_tolerance_minutes,
    tp.overtime_min_minutes,
    tp.overtime_rounding_mode,
    tp.overtime_rounding_unit_minutes,
    tp.last_event_id
  INTO
    shift_start_local,
    shift_end_local,
    late_tolerance_minutes,
    early_leave_tolerance_minutes,
    overtime_min_minutes,
    overtime_rounding_mode,
    overtime_rounding_unit_minutes,
    time_profile_last_event_id
  FROM staffing.time_profile_versions tp
  WHERE tp.tenant_id = p_tenant_id
    AND tp.lifecycle_status = 'active'
    AND tp.validity @> p_work_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_TIME_PROFILE_NOT_CONFIGURED_AS_OF',
      DETAIL = format('tenant_id=%s as_of=%s', p_tenant_id, p_work_date);
  END IF;

  shift_start := (p_work_date + shift_start_local) AT TIME ZONE v_tz;
  shift_end := (p_work_date + shift_end_local) AT TIME ZONE v_tz;
  window_start := shift_start - v_window_before;
  window_end := shift_end + v_window_after;

  RETURN NEXT;
END;
$$;
```

##### 4.2.1B 在 `recompute_daily_attendance_result(...)` 中使用 helper 获取 window

> 说明：`recompute_daily_attendance_result(...)` 不再在函数体内硬编码/计算 punches window；改为只消费 `get_time_profile_for_work_date(...)` 的输出，从源头消除口径漂移。

```sql
SELECT
  shift_start_local,
  shift_end_local,
  late_tolerance_minutes,
  early_leave_tolerance_minutes,
  overtime_min_minutes,
  overtime_rounding_mode,
  overtime_rounding_unit_minutes,
  time_profile_last_event_id,
  shift_start,
  shift_end,
  window_start,
  window_end
INTO
  v_shift_start_local,
  v_shift_end_local,
  v_late_tolerance_min,
  v_early_tolerance_min,
  v_overtime_min_minutes,
  v_overtime_rounding_mode,
  v_overtime_rounding_unit_minutes,
  v_time_profile_last_event_id,
  v_shift_start,
  v_shift_end,
  v_window_start,
  v_window_end
FROM staffing.get_time_profile_for_work_date(p_tenant_id, p_work_date);
```

##### 4.2.1C punches 扫描：排除已作废事件

把 §4B 的 punches 扫描从“全量事件”改为“排除已作废事件”：

```sql
-- 原：FROM staffing.time_punch_events
-- 新：增加 NOT EXISTS 过滤
SELECT e.id, e.punch_time, e.punch_type
FROM staffing.time_punch_events e
WHERE e.tenant_id = p_tenant_id
  AND e.person_uuid = p_person_uuid
  AND e.punch_time >= v_window_start
  AND e.punch_time < v_window_end
  AND NOT EXISTS (
    SELECT 1
    FROM staffing.time_punch_void_events v
    WHERE v.tenant_id = e.tenant_id
      AND v.target_punch_event_db_id = e.id
  )
ORDER BY e.punch_time ASC, e.id ASC;
```

> 说明：`daily_attendance_results.input_*` 水位线随之收敛为“有效 punches 的水位线”，审计链路用事件表展示“被作废的 punches”。

#### 4.2.2 新增 Kernel：`staffing.submit_time_punch_void_event(...)`（作废某条 punch，并触发重算）

> 目标：提供唯一写入口，插入 `time_punch_void_events`（append-only）并同事务触发 `recompute_daily_attendance_results_for_punch(...)`。

```sql
CREATE OR REPLACE FUNCTION staffing.submit_time_punch_void_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_target_punch_event_id uuid,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_target staffing.time_punch_events%ROWTYPE;
  v_existing_by_event staffing.time_punch_void_events%ROWTYPE;
  v_existing_by_target staffing.time_punch_void_events%ROWTYPE;
  v_payload jsonb;
  v_void_db_id bigint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_target_punch_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'target_punch_event_id is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  SELECT * INTO v_target
  FROM staffing.time_punch_events
  WHERE tenant_id = p_tenant_id
    AND event_id = p_target_punch_event_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_TIME_PUNCH_EVENT_NOT_FOUND',
      DETAIL = format('tenant_id=%s target_event_id=%s', p_tenant_id, p_target_punch_event_id);
  END IF;

  INSERT INTO staffing.time_punch_void_events (
    event_id,
    tenant_id,
    person_uuid,
    target_punch_event_db_id,
    target_punch_event_id,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    v_target.person_uuid,
    v_target.id,
    v_target.event_id,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT DO NOTHING
  RETURNING id INTO v_void_db_id;

  IF v_void_db_id IS NULL THEN
    -- 1) event_id 幂等重放
    SELECT * INTO v_existing_by_event
    FROM staffing.time_punch_void_events
    WHERE event_id = p_event_id;

    IF FOUND THEN
      IF v_existing_by_event.tenant_id <> p_tenant_id
        OR v_existing_by_event.person_uuid <> v_target.person_uuid
        OR v_existing_by_event.target_punch_event_db_id <> v_target.id
        OR v_existing_by_event.target_punch_event_id <> v_target.event_id
        OR v_existing_by_event.payload <> v_payload
        OR v_existing_by_event.request_id <> p_request_id
        OR v_existing_by_event.initiator_id <> p_initiator_id
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
          DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_by_event.id);
      END IF;
      RETURN v_existing_by_event.id;
    END IF;

    -- 2) 目标 punch 已被作废（幂等：返回既有 void id）
    SELECT * INTO v_existing_by_target
    FROM staffing.time_punch_void_events
    WHERE tenant_id = p_tenant_id
      AND target_punch_event_db_id = v_target.id
    LIMIT 1;

    IF FOUND THEN
      RETURN v_existing_by_target.id;
    END IF;

    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'void insert failed';
  END IF;

  PERFORM staffing.recompute_daily_attendance_results_for_punch(p_tenant_id, v_target.person_uuid, v_target.punch_time);

  RETURN v_void_db_id;
END;
$$;
```

#### 4.2.3 新增 Kernel：`staffing.submit_attendance_recalc_event(...)`（按人按日期范围手工重算）

> 目标：把“重算”变成可追溯的事件，并在同事务内执行 bounded replay（按人按天循环调用 `recompute_daily_attendance_result`）。

```sql
CREATE OR REPLACE FUNCTION staffing.submit_attendance_recalc_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_from_date date,
  p_to_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_existing staffing.attendance_recalc_events%ROWTYPE;
  v_payload jsonb;
  v_recalc_db_id bigint;
  v_d date;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_from_date IS NULL OR p_to_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'from_date/to_date is required';
  END IF;
  IF p_to_date < p_from_date THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'to_date must be >= from_date';
  END IF;
  -- 范围天数口径：range_days = (to_date - from_date) + 1；MVP 上限 31 天（含首尾）
  IF (p_to_date - p_from_date) > 30 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'date range too large (max 31 days)';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  INSERT INTO staffing.attendance_recalc_events (
    event_id,
    tenant_id,
    person_uuid,
    from_date,
    to_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_person_uuid,
    p_from_date,
    p_to_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_recalc_db_id;

  IF v_recalc_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.attendance_recalc_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.from_date <> p_from_date
      OR v_existing.to_date <> p_to_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  v_d := p_from_date;
  WHILE v_d <= p_to_date LOOP
    PERFORM staffing.recompute_daily_attendance_result(p_tenant_id, p_person_uuid, v_d);
    v_d := v_d + 1;
  END LOOP;

  RETURN v_recalc_db_id;
END;
$$;
```

### 4.3 迁移策略（Atlas+Goose；按 `DEV-PLAN-024` 闭环）

- **Up（预期）**：
  1. 新增 `staffing.time_punch_void_events`、`staffing.attendance_recalc_events`（含索引、RLS policy）。
  2. 修改 `staffing.recompute_daily_attendance_result`：过滤 voided punches。
  3. 新增 kernel：`staffing.submit_time_punch_void_event`、`staffing.submit_attendance_recalc_event`。
- **Down（预期）**：删除函数与表（生产通常不执行破坏性 down；仅用于本地/测试环境）。

## 5. 接口契约 (API Contracts)

### 5.1 UI：`GET/POST /org/attendance-daily-results/{person_uuid}/{work_date}`（route_class=ui）

- **Path 参数**：
  - `person_uuid`：uuid
  - `work_date`：`YYYY-MM-DD`
- **Query 参数**：
  - `as_of`：沿用 shell 顶栏（非业务必需）；缺省当天 UTC date。

#### 5.1.1 GET（页面内容：新增“审计 + 纠错/重算”区块）

- **基础信息（沿用 4B/4C）**：展示 `ruleset_version/day_type/status/flags/first_in_time/last_out_time/*minutes/input_watermark_*/time_profile_last_event_id/holiday_day_last_event_id/computed_at`。
- **审计区块（本切片新增）**：
  - 计算窗口（按 `Asia/Shanghai`）：`work_date` 对应的 `shift_start/shift_end/window_start/window_end`（由 `staffing.get_time_profile_for_work_date(...)` 计算；与 kernel 共用，避免口径漂移）。
  - punches 列表（窗口内，按 `punch_time ASC,id ASC`）：展示 `event_id/punch_time(beijing)/type/source/tx_time`，并标记是否已作废；若已作废同时展示对应 void 事件的 `event_id/created_at` 与 `payload.reason`（如有）。
  - 手工重算事件（如有）：展示最近 N 条 `attendance_recalc_events`（过滤条件：`from_date <= work_date <= to_date`，按 `created_at DESC`）。
- **纠错入口（本切片新增）**：对 punches 列表中“未作废”的行提供 “Void” 按钮（见 POST）。
- **重算入口（本切片新增）**：提供“重算本日”按钮与“范围重算（按人）”表单（见 POST）。

#### 5.1.2 POST（Form Data；页面内 op 分支）

- **op=void_punch（作废一条 punch）**：
  - `op`（Required）：`void_punch`
  - `target_punch_event_id`（Required，uuid）：要作废的 punch 的 `event_id`
  - `reason`（Optional，string）：写入 void event 的 `payload.reason`
  - **成功**：303 redirect 回 GET 同页；刷新后日结果与审计链路更新。
  - **失败**：200 返回同页并展示可读错误信息。
  - **幂等语义（选定）**：若目标 punch 已作废，视为 no-op（返回成功并复用既有 void 事件；不创建新 void 事件）。
- **op=recalc_day（重算本日）**：
  - `op`（Required）：`recalc_day`
  - `reason`（Optional，string）：写入 recalc event 的 `payload.reason`
  - **成功**：303 redirect 回 GET 同页；刷新后 `computed_at` 更新。
- **op=recalc_range（按人范围重算）**：
  - `op`（Required）：`recalc_range`
  - `from_date` / `to_date`（Optional，`YYYY-MM-DD`；缺省为 `work_date`；闭区间；MVP 最大跨度 31 天（含首尾；即 `to_date - from_date <= 30`））
  - `reason`（Optional，string）
  - **成功**：303 redirect 回 GET（建议 redirect 到 `from_date=work_date&to_date=work_date`，并在页面提示“已重算 N 天”）。

> 更正（replace）口径（MVP）：先在详情页作废错误 punch，再跳转 `/org/attendance-punches?person_uuid=...&from_date=work_date&to_date=work_date` 补打一条正确 punch。

### 5.2 Internal API（可选但建议）：`POST /org/api/attendance-punch-voids`（route_class=internal_api）

> 用途：E2E/自动化验收与调试；不作为公共 API 承诺。

- **请求（application/json）**：
  ```json
  {
    "event_id": "uuid (optional; 用于幂等重放)",
    "target_punch_event_id": "uuid",
    "payload": {"reason":"..."}
  }
  ```
- **响应（201）**：返回 void 事件（至少包含 `event_id/target_punch_event_id/created_at`）。
  ```json
  {
    "event_id": "uuid",
    "target_punch_event_id": "uuid",
    "created_at": "2026-01-01T00:00:00Z"
  }
  ```
- **错误码**：
  - 400：参数无效
  - 404：目标 punch 不存在（`STAFFING_TIME_PUNCH_EVENT_NOT_FOUND`）
  - 409：幂等键冲突（`STAFFING_IDEMPOTENCY_REUSED`）
  - 403：Authz deny（统一 responder）

### 5.3 Internal API（可选但建议）：`POST /org/api/attendance-recalc`（route_class=internal_api）

- **请求（application/json）**：
  ```json
  {
    "event_id": "uuid (optional; 用于幂等重放)",
    "person_uuid": "uuid",
    "from_date": "2026-01-01",
    "to_date": "2026-01-31",
    "payload": {"reason":"TimeProfile update backfill"}
  }
  ```
- **响应（201）**：返回 recalc 事件（至少包含 `event_id/person_uuid/from_date/to_date/created_at`）。
  ```json
  {
    "event_id": "uuid",
    "person_uuid": "uuid",
    "from_date": "2026-01-01",
    "to_date": "2026-01-31",
    "created_at": "2026-01-01T00:00:00Z"
  }
  ```
- **错误码**：400（参数/范围无效）、409（幂等键冲突）、403（Authz deny）。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)

### 6.1 作废 punch（UI / Internal API 共用）

1. 解析输入：`target_punch_event_id` 必填；`reason` 可选。
2. 生成幂等键：`event_id`（uuid v4）；`request_id = event_id::text`。
3. 开启事务。
4. 注入租户：`SELECT set_config('app.current_tenant', $1, true)`（必须在事务内）。
5. 调用 Kernel：`SELECT staffing.submit_time_punch_void_event(...)`。
6. 事务提交：提交成功即意味着：
   - void 事件已写入 SoT；
   - 受影响日结果（`work_date-1` 与 `work_date`）已同步重算（写后读强一致）。

### 6.2 手工重算（按人按日期范围）

1. 解析输入：`person_uuid` 必填；`from_date/to_date` 缺省为当前 `work_date`；`to_date>=from_date`。
2. 约束：DB kernel 强限制范围（MVP：最大 31 天（含首尾；`range_days = (to_date - from_date) + 1`））；超过直接报错。
3. 开启事务 + 注入租户。
4. 调用 Kernel：`SELECT staffing.submit_attendance_recalc_event(...)`。
5. DB 内部按日循环调用 `recompute_daily_attendance_result`，并依赖 4D 的“联动 hook”同步更新 `time_bank_cycles`。
6. 提交事务后，UI 刷新即可读到最新结果。

### 6.3 审计链路（事件链展示口径）

#### 6.3.1 punches 输入窗口（单一事实源）

对齐 `staffing.recompute_daily_attendance_result(...)`，并避免“UI/Kernel 两处硬编码 window 常量”：

```sql
SELECT shift_start, shift_end, window_start, window_end
FROM staffing.get_time_profile_for_work_date($1::uuid, $2::date);
```

#### 6.3.2 punches + voids 列表查询（窗口内）

```sql
SELECT
  p.event_id::text,
  p.punch_time,
  p.punch_type,
  p.source_provider,
  p.payload,
  p.transaction_time,

  (v.id IS NOT NULL) AS is_voided,
  v.event_id::text AS void_event_id,
  v.created_at AS void_created_at,
  v.payload->>'reason' AS void_reason
FROM staffing.time_punch_events p
LEFT JOIN staffing.time_punch_void_events v
  ON v.tenant_id = p.tenant_id
  AND v.target_punch_event_db_id = p.id
WHERE p.tenant_id = $1::uuid
  AND p.person_uuid = $2::uuid
  AND p.punch_time >= $3::timestamptz
  AND p.punch_time < $4::timestamptz
ORDER BY p.punch_time ASC, p.id ASC;
```

#### 6.3.3 手工重算事件（覆盖该 work_date）

```sql
SELECT
  event_id::text,
  from_date,
  to_date,
  payload,
  created_at
FROM staffing.attendance_recalc_events
WHERE tenant_id = $1::uuid
  AND person_uuid = $2::uuid
  AND from_date <= $3::date
  AND to_date >= $3::date
ORDER BY created_at DESC, id DESC
LIMIT 50;
```

#### 6.3.4 规则/日历追溯锚点

- 从 `daily_attendance_results.time_profile_last_event_id/holiday_day_last_event_id` 展示（可选：进一步 join `time_profile_events/holiday_day_events` 拿到 `event_id`），用于解释“本次结果用的是哪版规则/日历”。

## 7. 安全与鉴权 (Security & Authz)

### 7.1 RLS（强租户隔离）

- 新表 `time_punch_void_events`、`attendance_recalc_events` 必须启用 `ENABLE + FORCE` + `tenant_isolation` policy（见 §4.1）。
- Go 侧所有读写必须事务内注入 `app.current_tenant`；缺失注入应 fail-closed（对齐 `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`）。

### 7.2 Authz（Casbin）

- **object（沿用 + 扩展）**：`staffing.attendance-daily-results`
  - `GET /org/attendance-daily-results/{person_uuid}/{work_date}`：`read`
  - `POST /org/attendance-daily-results/{person_uuid}/{work_date}`：`admin`（本切片新增）
- **object（沿用）**：`staffing.attendance-punches`
  - `/org/attendance-punches`、`/org/api/attendance-punches` 仍按 4A 的 read/admin 映射。
- **internal_api（建议）**：
  - `POST /org/api/attendance-punch-voids`：`staffing.attendance-punches` + `admin`
  - `POST /org/api/attendance-recalc`：`staffing.attendance-daily-results` + `admin`
- **policy（bootstrap 期）**：为 `role:tenant-admin` 增补上述 object 的 `admin` 允许项；必须跑 `make authz-pack && make authz-test && make authz-lint`。

## 8. 依赖与里程碑 (Dependencies & Milestones)

### 8.1 依赖

- 4A punches SoT + kernel 已存在：`staffing.time_punch_events` / `staffing.submit_time_punch_event`。
- 4B 日结果重算函数已存在：`staffing.recompute_daily_attendance_result` / `recompute_daily_attendance_results_for_punch`。
- 4C TimeProfile/HolidayCalendar 已存在且能提供 as-of 规则与追溯锚点。
- 4D（若已落地）：`recompute_daily_attendance_result` 内部已联动 `recompute_time_bank_cycle`；若未落地，需先合并 4D 的联动改造再做本切片。

### 8.2 里程碑（可直接开工的拆解）

1. [x] 文档确认 + 新增表批准记录固化：确认本切片范围/不变量，并在 §4.1.0 登记“新增表已获手工批准”。
2. [x] 路由：
   - 允许 `/org/attendance-daily-results/{person_uuid}/{work_date}` 支持 POST（保持 `route_class=ui`）。
   - （建议）新增 `/org/api/attendance-punch-voids`、`/org/api/attendance-recalc`（`route_class=internal_api`）。
   - 更新 `config/routing/allowlist.yaml` 并跑 `make check routing`。
3. [x] Authz：更新 `pkg/authz/registry.go`（如需新增 object）；更新 `internal/server/authz_middleware.go` 进行路由映射；更新 `config/access/policies/00-bootstrap.csv`；跑 `make authz-pack authz-test authz-lint`。
4. [ ] DB：按 §4.1/§4.2 落地表 + kernel（新增表/迁移前需手工确认）；跑 `make staffing plan && make staffing lint && make staffing migrate up`。
5. [ ] sqlc：运行 `make sqlc-generate`，并确保生成物提交且 `git status --short` 为空。
6. [ ] Go：
   - store：在 `internal/server/attendance.go` 增补 void/recalc 的 store 方法（显式 tx + tenant 注入 + 调 kernel）。
   - handler/UI：在 `internal/server/attendance_handlers.go` 的日结果详情页实现 POST（void/recalc），并补齐审计区块（事件链展示）。
7. [ ] 测试：补齐本计划覆盖（见 §9）。
8. [ ] （可选）E2E：把“作废 punch → 日结果变化可见”纳入 smoke（`make e2e`）。
9. [ ] 证据：按 `docs/dev-records/` 口径登记关键门禁执行记录（时间戳/命令/结论）。

## 9. 测试与验收标准 (Acceptance Criteria)

### 9.1 验收清单

- [ ] 端到端：
  1) 先补打卡形成日结果；2) 在日结果详情页作废其中一条 punch；3) 刷新后日结果更新且 `computed_at` 更新；审计区块能看到“被作废 punch + void 事件”。
- [ ] 可追溯：同一天的事件链可视化（punches + void +（如有）recalc events），且 `computed_at/input_watermark_*/*_last_event_id` 可解释。
- [ ] 一致性：作废/重算在事务提交后立即一致（写后读强一致）；无“半更新”状态。
- [ ] One Door：无任何绕过 kernel 的直写路径；更正/重算只通过 `submit_*_event` 入口。
- [ ] 安全：RLS fail-closed + 跨租户隔离；`AUTHZ_MODE=enforce` 下未授权 admin 被 403 拒绝。

### 9.2 建议的测试用例（落点建议）

- `internal/server/authz_middleware_test.go`：覆盖新路由与方法映射（detail POST、internal api POST）。
- `internal/server/handler_test.go`：覆盖 detail GET 200、detail POST 303（成功）/200（失败）。
- `internal/server/attendance_db_integration_test.go`（DB 行为）：
  - 作废后重算：同一 work_date 的 worked_minutes/flags 发生预期变化（例如作废 OUT → `MISSING_OUT`）。
  - 作废过滤：`recompute_daily_attendance_result` 不再计入被作废的 punch。
  - 幂等：重复提交同一 void event_id 不改变结果；event_id 重用但 payload 不同 → `STAFFING_IDEMPOTENCY_REUSED`。
  - fail-closed：缺失 `app.current_tenant` 注入时读取/写入新表必须失败。

## 10. 运维与监控 (Ops & Monitoring)

- **不引入额外运维组件**（对齐 `AGENTS.md` §3.6）。
- **最小日志字段（建议）**：`request_id`、`tenant_id`、`principal_id`、`person_uuid`、`work_date`、`op`、`target_punch_event_id`、`event_id`。

## 11. 开放问题

- [ ] 是否需要支持 “unvoid（撤销作废）”：若需要，应以事件化表达（VOID/UNVOID）并重新定义“有效 punches”的折叠规则（last-write-wins vs. 仅允许一次 void）。
- [ ] 是否需要支持“按租户批量重算”：若需要，必须先给出分批/限流/可中断/可追溯的契约与门禁策略，避免引入隐式后台任务与运维负担。
