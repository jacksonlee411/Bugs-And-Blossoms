# DEV-PLAN-066 记录：TP-060-06——考勤 4F（外部对接 + 身份映射）执行日志

**状态**：已完成（2026-01-11）

> 对应计划：`docs/dev-plans/066-test-tp060-06-attendance-4f-integrations-identity-mapping.md`  
> 上游套件：`docs/dev-plans/060-business-e2e-test-suite.md`

## 执行结论

- `/org/attendance-integrations`：映射管理可用；pending/active/disabled/ignored 状态机可控；422 错误可判定。
- 外部事件摄入（本地模拟）：已验证 unmapped 不落库、mapped 可写入 punches，且 punches/daily results 可见。
- 安全与隔离：viewer 角色 403；缺 tenant context fail-closed；跨租户不可串数（以 `PERSON_NOT_FOUND` 与 punches 空结果留证）。

## 环境与关键变量（本次执行）

- `Host=t-060.localhost`，`AS_OF_BASE=2026-01-11`，`WORK_DATE=2026-01-11`
- `T060_TENANT_ID=f88b6771-3643-4a26-82f1-3612c5e7e004`
- `T060B_TENANT_ID=759542a3-8e7f-46ac-a3fe-bc43bed99b21`
- E08：`E08_PERSON_UUID=799522ec-fa32-400e-ad7c-9056bc6fa7f6`（pernr=108）
- E09：`E09_PERSON_UUID=41a5f905-9958-441d-85d1-9c79414fb977`（pernr=109）
- 外部用户 ID：
  - mapped：`E08_DINGTALK_USER_ID=dt-e08-060`
  - mapped：`E09_WECOM_USER_ID=wc-e09-060`
  - unmapped：`UNMAPPED_DINGTALK_USER_ID=dt-unmapped-060`

## 关键步骤与证据（命令口径）

### 1) TimeProfile（必要前置）

- 配置：`effective_date=2026-01-01 shift=09:00-18:00 name=TP060-default`
- DB 留证（event）：
  - `SELECT ... FROM staffing.time_profile_events WHERE tenant_id='<T060_TENANT_ID>'` → `id=92`

### 2) 映射可见性（UI）

- `GET /org/attendance-integrations?as_of=2026-01-11`：
  - Pending：`DINGTALK dt-unmapped-060`，`seen_count=4`，`last_seen_at` 随摄入更新
  - Active：`DINGTALK dt-e08-060 -> E08`，`seen_count=7`，`last_seen_at` 随摄入更新
  - Active：`WECOM wc-e09-060 -> E09`

### 3) 状态机覆盖（UI 操作）

- 对 `WECOM wc-e09-060` 执行一次完整流转并回到 active：
  - `disable → enable → unlink → ignore → unignore → link`
  - 每步 `POST /org/attendance-integrations` 均返回 `303`

### 4) 安全与隔离

- Authz：viewer 对 `POST /org/attendance-integrations` → `403`
- fail-closed：不带 tenant host 访问 `/org/attendance-integrations` → `404 tenant not found`
- 跨租户（可选但建议，已执行）：
  - 在 `T060B` 创建 `pernr=201` 得 `T060B_PERSON_UUID=24355e0b-2a1e-4b75-a5c7-726c4a3757bc`
  - 在 `T060` 查询 `/person/api/persons:by-pernr?pernr=201` → `404 PERSON_NOT_FOUND`
  - 在 `T060` 查询 punches（person_uuid=`T060B_PERSON_UUID`）→ `count=0`

### 5) 外部事件摄入（本地模拟，走同一 kernel 写入口）

> 说明：本次未使用真实 DingTalk/WeCom 凭证启动 `cmd/attendance-integrations`，而是在本地用 Go 调用 `attendanceintegrations.IngestExternalPunch` + `PGStore.SubmitTimePunch`（最终落到 `staffing.submit_time_punch_event(...)`，满足 One Door）。

- unmapped（两次）：`provider=DINGTALK external_user_id=dt-unmapped-060`
  - `request_id=tp060-06-unmapped-1/2` → `outcome=unmapped`
  - DB 断言（必须）：`staffing.time_punch_events` 对上述 `request_id` 计数为 `0`
- mapped（两次 + 幂等重放）：`provider=DINGTALK external_user_id=dt-e08-060`
  - `request_id=tp060-06-e08-1` → `outcome=ingested event_db_id=406`
  - `request_id=tp060-06-e08-2` → `outcome=ingested event_db_id=407`
  - 幂等重放：再次投递 `tp060-06-e08-1` 返回同一 `event_db_id=406`
- punches 可见：
  - `GET /org/api/attendance-punches?person_uuid=<E08_PERSON_UUID>&from=2026-01-11T00:00:00Z&to=2026-01-12T00:00:00Z`
  - 断言：至少两条记录，`source_provider=DINGTALK`，`punch_type=RAW`
- daily results 可见：
  - `GET /org/api/attendance-daily-results?person_uuid=<E08_PERSON_UUID>&from_date=2026-01-11&to_date=2026-01-11`
  - 断言：存在一条 `work_date=2026-01-11`，且 `input_punch_count=2`、`first_in_time/last_out_time` 与 punches 对齐（UTC）。

## ENV_DRIFT（必须记录）

- 缺少真实 DingTalk/WeCom 平台凭证，未能按 `DEV-PLAN-056` 以“真实外部事件来源”启动 Worker 并留证（`cmd/attendance-integrations`）。

## 数据保留（强制）

- 本次执行创建的 identity links、punch events、daily results 与 time profile 配置需保留用于后续回归与排障；不得执行 `make dev-reset`/`docker compose down -v`。

