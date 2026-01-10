# DEV-PLAN-056 记录：考勤 Slice 4F——生态集成闭环（钉钉 Stream / 企微 Poller）执行日志

**状态**: 进行中（2026-01-10）

> 本日志用于跟踪 `docs/dev-plans/056-attendance-slice-4f-dingtalk-wecom-integration.md` 的实施拆分（§8），确保每个 PR 可独立验收、门禁对齐、并在合并后回填完成情况。

## PR-0：计划文档收敛到可实施颗粒度 + 建表批准记录

- **状态**：已完成（2026-01-10）
- **范围**
  - 计划文档补齐到可直接实施颗粒度（对齐 `docs/dev-plans/001-technical-design-template.md`）
  - 按 `docs/dev-plans/003-simple-not-easy-review-guide.md` 与 `AGENTS.md` 原则评审并优化
  - 记录“新增表手工确认”（对齐 `AGENTS.md` §3.2 红线）
- **交付物**
  - 计划文档：`docs/dev-plans/056-attendance-slice-4f-dingtalk-wecom-integration.md`
  - 执行日志：`docs/dev-records/dev-plan-056-execution-log.md`
- **建表手工确认（已获得）**
  - 允许新增表：`person.external_identity_links`（用于外部身份映射）
- **本地门禁**
  - `make check doc`：通过

## PR-1：DB/Kernel — identity_links + RAW punch + request_id 幂等

- **状态**：已完成（2026-01-10）
- **范围**
  - Person：新增 `person.external_identity_links`（RLS + 最小排障摘要字段）
  - Staffing：扩展 `staffing.time_punch_events` allowlist（`punch_type=RAW`，`source_provider=DINGTALK/WECOM`）
  - Staffing Kernel：`staffing.submit_time_punch_event` 支持 `request_id` 幂等（允许 event_id 漂移但字段必须一致）
  - 日结果：`staffing.recompute_daily_attendance_result` 支持 RAW punch 交替解释
  - 工具链：修复 Atlas dev-url 默认值（避免仅 public schema 导致 drift 检测失效）
  - sqlc：更新 `internal/sqlc/schema.sql` 并重新生成
  - Tests：补齐 request_id 幂等路径的 DB 集成测试
- **本地门禁**
  - `make person plan && make person lint`：通过
  - `make staffing plan && make staffing lint`：通过
  - `make sqlc-generate`：通过（生成物已更新）
  - `go fmt ./... && go vet ./... && make check lint && make test`：通过
  - `make check doc`：通过
- **迁移验证（本地 DB）**
  - `DATABASE_URL=... make orgunit migrate up`：通过（满足 staffing smoke 的 orgunit 依赖）
  - `DATABASE_URL=... make staffing migrate up`：通过（含 `staffing-smoke`）
  - `DATABASE_URL=... make person migrate up`：通过（含 `person-smoke`）

## PR-2：UI + Authz + Routing — /org/attendance-integrations

- **状态**：待开始

## PR-3：Worker — DingTalk Stream（attendance_check_record）

- **状态**：待开始

## PR-4：Worker — WeCom Poller（滑动窗口 + 幂等）

- **状态**：待开始

## PR-5：Tests + Readiness（fixture/负例/门禁记录）

- **状态**：待开始
