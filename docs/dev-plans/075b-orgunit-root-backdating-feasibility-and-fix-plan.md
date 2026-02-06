# DEV-PLAN-075B：Root Unit A 生效日回溯可行性调查与修复方案

**状态**: 已完成（2026-02-06 12:21 UTC）

## 1. 背景
- 需求问题：调查是否可以在页面 `http://localhost:8080/org/nodes?tree_as_of=2026-02-06` 将 **Root Unit A** 的生效日期修改为 `2026-01-01`。
- 关联上下文：`DEV-PLAN-075` 已明确目标口径为“支持受控回溯（区间内前后调整）”。

## 2. 调查范围与方法
- 文档口径核对：`DEV-PLAN-075` 与执行日志是否一致。
- 写入链路核对：UI 页面 -> Internal API -> WriteService -> PG Store -> DB Kernel 函数。
- 数据库事实核对：`modules/orgunit/infrastructure/persistence/schema/*.sql` 与 `migrations/orgunit/*.sql` 是否一致。
- 运行态可用性核对：本地 `localhost:8080` 与 `127.0.0.1:5438` 是否可连通。

## 3. 关键发现（Findings）

### 3.1 文档目标与实现入口是支持回溯
- `DEV-PLAN-075` 结论已选“支持更早生效（受控回溯）”。
- 页面详情编辑生效日走 `/org/api/org-units/corrections`。
- 路由与服务链路：
  - `internal/server/orgunit_nodes.go`（前端 JS 提交 corrections）
  - `internal/server/handler.go`（注入 `OrgUnitWriteService`）
  - `modules/orgunit/services/orgunit_write_service.go`（`Correct`）
  - `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`（`SubmitCorrection`）

### 3.2 存在“Schema 与 Migration 漂移”
- **Schema（目标口径）**：`00003_orgunit_engine.sql` 的 `submit_org_event_correction(...)` 使用前后邻接边界（`v_prev_effective`/`v_next_effective`）校验，允许区间内向前调整。
- **Migration（实际落库路径）**：`20260204070612_orgunit_event_corrections_enable.sql` 中 `submit_org_event_correction(...)` 仍包含“`v_new_effective < v_target.effective_date` 直接拒绝”的旧规则。
- 结论：如果环境通过 `make orgunit migrate up` 从 `migrations/orgunit` 建库，运行时函数将阻止向前回溯。

### 3.3 对“Root Unit A 改到 2026-01-01”的可行性结论
- 在 **迁移链路建成的当前数据库** 上，结论为：**通常不可行**（会命中 `EFFECTIVE_DATE_OUT_OF_RANGE` 或同类冲突）。
- 在 **手工应用 schema 新版函数** 的环境上，才可能按“受控回溯”通过。

### 3.4 本地运行态限制
- 调查时本机 `localhost:8080` 不可连接。
- 调查时本机 `127.0.0.1:5438`（PostgreSQL）无响应。
- 因此本次结论基于代码/迁移静态核对；未完成该 URL 的在线实操验证。

### 3.5 附带风险（非本问题主因）
- `internal/server/orgunit_nodes.go` 中 `CorrectNodeEffectiveDate(...)` 的 `set_config` SQL 字符串存在未加引号写法（`set_config(app.current_tenant, ...)`），可能导致该旧链路报 SQL 错。
- 该问题主要影响“`insert_record` 对最早记录走 correction 的分支”，不是详情页 corrections API 的主链路。

## 4. 根因总结（Root Cause）
- 根因不是“075 方案本身错误”，而是 **“方案已更新，但 migration 中 DB Kernel 函数未同步到最终口径”**。
- 项目运行时依赖 `migrations/orgunit`，因此真实行为仍受旧函数约束。

## 5. 修复方案（Proposed Fix）

### 5.1 必做：补一条新迁移，修正 DB Kernel 函数
1. 新增一条 `migrations/orgunit/` 迁移（仅函数替换，不改历史文件）。
2. 在迁移里 `CREATE OR REPLACE FUNCTION orgunit.submit_org_event_correction(...)`，对齐 `00003_orgunit_engine.sql` 的区间校验逻辑：
   - 使用 `org_events_effective` 解析目标事件有效日期与生效 payload。
   - 以 `v_prev_effective` / `v_next_effective` 约束回溯范围。
   - 保留同日冲突校验与上级有效性校验。
3. 保持 One Door：仍通过 DB Kernel 函数写入 current/history + replay，不引入第二写入口。

### 5.2 建议：顺手修复旧 UI store 链路的 SQL 字符串
- 将 `internal/server/orgunit_nodes.go` 中 `set_config(app.current_tenant, ...)` 改为 `set_config('app.current_tenant', ...)`，避免最早记录 insert->correction 分支潜在故障。

### 5.3 验证与验收
- 本地最小验证：
  - `make orgunit migrate up`
  - `go test ./internal/server -run "TestHandleOrgNodes_RecordActions/insert_record_backdate_earliest_uses_correction" -count=1`
  - 补充 DB 层集成验证：构造“最早记录向前回溯”的 correction SQL，用例验证通过。
- 页面回归（目标场景）：
  1. 打开 `/org/nodes?tree_as_of=2026-02-06`。
  2. 选中 Root Unit A。
  3. 在详情编辑里将生效日期改为 `2026-01-01` 并保存。
  4. 期望：成功并回显新版本；若越界/冲突，返回清晰错误码与文案。

## 6. 影响评估
- 影响层级：OrgUnit correction 写入逻辑（DB Kernel）+ 旧 UI store 的 correction 辅助链路。
- 向后兼容：
  - 不改变 API 路由；
  - 不新增表结构；
  - 仅修正函数语义到既定契约。

## 7. 实施步骤（待执行）
1. [x] 新增 migration：对齐 `submit_org_event_correction` 区间回溯规则（`migrations/orgunit/20260206115000_orgunit_correction_backdating_range_fix.sql`）。
2. [x] 修复 `internal/server/orgunit_nodes.go` 的 `set_config('app.current_tenant', ...)` 字符串。
3. [x] 补齐/更新相关测试（新增 `TestOrgUnitPGStore_UsesQuotedCurrentTenantKey`，并回归 `insert_record_backdate_earliest_uses_correction` 关键分支）。
4. [x] 本地执行门禁并记录证据到 `docs/dev-records/`（见 `docs/dev-records/dev-plan-075b-execution-log.md`）。

## 8. 关联文档
- `docs/dev-plans/075-orgunit-effective-date-backdating-assessment.md`
- `docs/dev-records/dev-plan-075-execution-log.md`
- `docs/dev-plans/073-orgunit-crud-implementation-status.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`
