# DEV-PLAN-066：全链路业务测试子计划 TP-060-06——考勤 4F（外部对接 + 身份映射）

**状态**: 已完成（2026-01-11；证据：`docs/dev-records/dev-plan-066-execution-log.md`）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`（租户/登录/隔离基线）、`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（人员存在）与 `docs/dev-plans/064-test-tp060-04-attendance-4a-punch-ledger.md`（punches UI 可用），并确保 `docs/dev-plans/065-test-tp060-05-attendance-4b-4e-results-config-bank-corrections.md` 的日结果链路可用（便于验证外部事件 → 日结果）。
> 执行日志：`docs/dev-records/dev-plan-066-execution-log.md`

> 说明：本子计划是 “业务 E2E 证据” 的契约文档，目标是把 TP-060-06 细化到“无需猜测/无需二次设计即可执行”的颗粒度（参照 `docs/dev-plans/001-technical-design-template.md` 的口径）。

## 1. 背景与上下文（Context）

`DEV-PLAN-056` 已实现考勤 Slice 4F：将钉钉/企微事件摄入收敛为“外部身份映射（provider + external_user_id → person_uuid）+ Worker 规范化 + 调用 kernel 写入 punches（One Door）”，最终在 `/org/attendance-*` 页面与手工 punches 同口径可见。

本子计划（TP-060-06）聚焦验证以下端到端性质：
- **可发现性**：`/org/attendance-integrations` 页面入口可见，且映射可操作（避免“僵尸集成”）。
- **状态机可控**：pending/active/disabled/ignored 的状态流转符合契约，且 UI 反馈可判定。
- **外部事件落地**：外部事件进入后（至少 1 条），`/org/attendance-punches` 能看到 `Source=DINGTALK|WECOM`，`/org/attendance-daily-results` 能看到对应 `work_date` 的结果（与 4B/4E 同口径）。
- **安全与隔离**：Authz 必须 403 可拒绝；缺 tenant context 必须 fail-closed；跨租户不得串数。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [X] `/org/attendance-integrations` 页面可发现、可操作：支持 `link/disable/enable/unlink/ignore/unignore`，并可见 `last_seen_at/seen_count`。
- [X] 覆盖状态机（至少 1 次完整流转）：`pending → active → disabled → active → unlink(pending) → ignored → pending → active`。
- [X] 外部事件摄入与可见性（至少 1 条可复现证据）：
  - [X] 外部事件进入后，`/org/attendance-punches` 可见一条 `Source=DINGTALK|WECOM` 的 punch（DingTalk 期望 `punch_type=RAW`；WeCom 可能为 `IN/OUT/RAW`）。
  - [X] 对应 `work_date` 的 `/org/attendance-daily-results` 可见该人员的日结果（最小口径：列表出现一行，并可点进详情；不要求分钟级精确对账）。
- [X] “未映射行为”可判定：对一个**未映射**的外部用户（provider + external_user_id）摄入事件后：
  - [X] `person.external_identity_links`（通过 integrations UI）出现 `status=pending`，且 `seen_count` 增长；
  - [X] `staffing.time_punch_events` 不应新增该外部用户对应的 punches（通过 punches UI 断言“无新增记录”）。
- [X] 安全与隔离：
  - [X] Authz：只读角色对 `/org/attendance-integrations` 任一 POST 必须 403。
  - [X] fail-closed：缺 tenant context 访问 integrations/punches/daily results 不得泄漏数据（允许 404/403/500/稳定错误码）。
  - [X] （可选但建议）跨租户隔离：用 `T060B_PERSON_UUID` 在 `T060` 下访问不得读到数据（与 TP-060-01 口径一致）。

### 2.2 非目标

- 不覆盖“对外回写平台回执/对账闭环”（如需另立计划）。
- 不在本子计划内要求“真实平台凭证托管/多租户多企业配置中心”；按 `DEV-PLAN-056` MVP 形态执行（单进程/单租户 Worker）。
- 不在本子计划内验证钉钉/企微平台侧的业务判定字段（迟到/早退/正常等）；这些字段不作为权威输入。

## 2.3 工具链与门禁（SSOT 引用）

> 目的：声明本子计划执行/修复时可能命中的门禁入口；命令细节以 `AGENTS.md`/`Makefile` 为准。

- 触发器清单（勾选本计划命中的项；执行记录见 §9.1）：
  - [ ] E2E（`make e2e`；若补齐 TP-060-06 自动化用例）
  - [ ] Authz（`make authz-pack && make authz-test && make authz-lint`；若权限口径 drift）
  - [ ] 路由治理（`make check routing`；若 `/org/attendance-integrations` 路由或 allowlist drift）
  - [X] 文档（本文件变更：`make check doc`）
  - [ ] Go 代码（仅当为修复而改 Go：`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] DB/迁移（仅当为修复而改 DB：按模块 `make <module> plan/lint/migrate ...`）

- SSOT 链接：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口与脚本实现：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`

## 3. 契约引用（SSOT）

- 测试套件总纲与数据集：`docs/dev-plans/060-business-e2e-test-suite.md`
- 4F（生态集成闭环）：`docs/dev-plans/056-attendance-slice-4f-dingtalk-wecom-integration.md`
- Person identity：`docs/dev-plans/027-person-minimal-identity-for-staffing.md`
- Punches（4A）/日结果（4B）/更正审计（4E）：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`、`docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`、`docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`
- RLS/Authz：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`

## 4. 架构与关键决策（面向测试的结论）

### 4.1 手工执行 vs 自动化回归（两条链路的职责分工）

- **手工执行（060-DS1 固定数据集）**：在 `T060 / t-060.localhost` 下完成映射与外部事件验证，并保留 identity links 与 punches 供后续回归与排障。
- **自动化回归（隔离数据）**：若补齐 E2E 用例，应使用 `runID` 创建独立 tenant 与数据，保证可重复跑；但不替代 060-DS1 的“固定可复现证据”。

### 4.2 外部事件来源（执行时二选一，并在证据中记录）

- **路径 A（真实外部对接）**：运行 `cmd/attendance-integrations` Worker，并由钉钉 Stream/企微打卡产生真实外部事件。
- **路径 B（环境无外部凭证时）**：本子计划允许将“外部事件进入”标记为阻塞，并在 §10 记录为 `ENV_DRIFT`，但**不得**用 SQL 直改业务读模表冒充摄入结果（One Door / No Legacy）。

> 说明：路径 B 不是“跳过验收”，而是显式记录环境能力缺失；一旦具备凭证/沙箱环境，应回到路径 A 补齐证据。

## 5. 数据模型与约束（Data Model & Constraints）

> 本节仅摘录 TP-060-06 执行必需的最小合同，避免与 `DEV-PLAN-056` 复制漂移。

### 5.1 外部身份映射表（事实源：`DEV-PLAN-056` §4.1）

- 表：`person.external_identity_links`（RLS 强隔离；主键：`tenant_id + provider + external_user_id`）
- provider allowlist：`DINGTALK|WECOM`
- status allowlist：`pending|active|disabled|ignored`
- 强约束：
  - `status in (pending, ignored)` 时 `person_uuid IS NULL`
  - `status in (active, disabled)` 时 `person_uuid IS NOT NULL`
- 观测字段：`last_seen_at`、`seen_count`（验收以“外部事件到达后的增长（delta）”为口径；UI 可能创建初始记录，但不应制造“外部事件到达”假象）。

### 5.2 Punch 事件（事实源：`DEV-PLAN-056` §4.2/§4.3）

- `staffing.time_punch_events.source_provider` allowlist：`MANUAL|IMPORT|DINGTALK|WECOM`
- `staffing.time_punch_events.punch_type` allowlist：`IN|OUT|RAW`
  - DingTalk：事件不提供 IN/OUT 语义，`punch_type=RAW`；由日结果计算按时间序列交替解释。
  - WeCom：`上班打卡→IN`、`下班打卡→OUT`、其它→`RAW`（见 `DEV-PLAN-056`）。

### 5.3 幂等（事实源：`DEV-PLAN-056` §6.3）

- 外部事件幂等键：`tenant_id + request_id` unique；重复投递不得产生重复 punches。

## 6. 页面/接口契约（用于断言）

### 6.1 UI：`GET/POST /org/attendance-integrations`

- 查询参数：`as_of=YYYY-MM-DD`（必填；仅作为 UI Shell 时间语义输入，不影响 identity_links 的 Tx time）。
- 期望可见区块：Link 表单 + 四个列表（Pending/Active/Disabled/Ignored）。
- 列表列：`provider/external_user_id/status/person/last_seen_at(UTC)/seen_count/actions`。
- POST 表单字段（`op` 分支）：
  - `op=link`：`provider/external_user_id/person_uuid`
  - `op=disable|enable|ignore|unignore|unlink`：`provider/external_user_id`
- 成功：`303` 重定向回 `GET /org/attendance-integrations?as_of=...`；失败：`422` 并在页面显示错误信息。

### 6.2 Worker：`cmd/attendance-integrations`（执行路径 A）

- 运行方式：与 `cmd/server` 分离的单租户进程（对齐 `DEV-PLAN-056` §10）。
- 必需环境变量：
  - DB：`DATABASE_URL`（或 `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE`）
  - `TENANT_ID=<T060_TENANT_ID>`
  - DingTalk（可选）：`DINGTALK_CLIENT_ID`、`DINGTALK_CLIENT_SECRET`、`DINGTALK_CORP_ID`
  - WeCom（可选）：`WECOM_CORP_ID`、`WECOM_CORP_SECRET`（可选：`WECOM_INTERVAL_SECONDS/WECOM_LOOKBACK_SECONDS`）
- 日志最小字段（用于留证）：`tenant_id/provider/request_id/external_user_id/outcome(ingested|unmapped|ignored|disabled)`。

### 6.3 结果可见性（用于断言）

- punches：`GET /org/attendance-punches?as_of=YYYY-MM-DD&person_uuid=<...>&from_date=YYYY-MM-DD&to_date=YYYY-MM-DD`
  - 断言：结果表的 `Source` 列显示 `DINGTALK|WECOM`，且对应时间点存在记录。
- daily results：`GET /org/attendance-daily-results?as_of=YYYY-MM-DD&work_date=YYYY-MM-DD`
  - 断言：该人员行存在（最小），并能点击进入详情页：`/org/attendance-daily-results/{person_uuid}/{work_date}?as_of=...`。

## 7. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- 记录 `T060_TENANT_ID`（uuid）：用于 Worker 的 `TENANT_ID=<...>`（来源：SuperAdmin tenants 列表；对齐 TP-060-01）。
- 运行态硬要求：`AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`（对齐 `docs/dev-plans/060-business-e2e-test-suite.md` §4.3）
- 日期口径（执行时填写）：
  - `AS_OF_BASE=<YYYY-MM-DD>`（integrations/punches UI 的 `as_of`；建议使用执行当天的日期，避免与真实外部事件日期脱节）
  - `WORK_DATE=<YYYY-MM-DD>`（外部事件 punch 的北京日期；建议从 Worker 日志或 punches 列表时间反推后记录）
- 人员（060-DS1）：E08/E09 Person 必须存在，并记录：
  - `E08_PERSON_UUID`（用于 DingTalk 映射）
  - `E09_PERSON_UUID`（用于 WeCom 映射）
- 外部用户 ID（执行时填写；真实平台通常不可控）：
  - `E08_DINGTALK_USER_ID=<DINGTALK userId>`（用于映射到 `E08_PERSON_UUID`）
  - `E09_WECOM_USER_ID=<WECOM userid>`（用于映射到 `E09_PERSON_UUID`）
  - `UNMAPPED_DINGTALK_USER_ID=<DINGTALK userId>`（用于“未映射”断言：确保此 userId 在执行前未被 link）

> 获取外部 userId 的建议方式（任选其一，并在证据中记录来源）：
> - 从平台侧（钉钉/企微）后台/人员列表获取；或
> - 先触发一次外部事件让其出现在 integrations Pending 列表，再从页面中读取 `external_user_id`。

### 7.1 数据保留（强制）

- 本子计划创建的外部身份映射与外部摄入事件必须保留，用于后续回归与排障（例如验证 `seen_count/last_seen_at` 累积与状态流转），不得跑完清理（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。

## 8. 测试步骤（执行时勾选）

> 执行记录要求：每步至少记录 `Host/as_of/AUTHZ_MODE/RLS_ENFORCE`；涉及外部事件需记录 `provider/external_user_id/request_id`（或等效可追溯信息）。失败必须填写 §10 问题记录。

### 8.1 运行态确认（硬要求）

1. [ ] 记录：`Host=t-060.localhost`、`T060_TENANT_ID`、`AS_OF_BASE`、`WORK_DATE`、`AUTHZ_MODE`、`RLS_ENFORCE`。
2. [ ] 打开 integrations 页面：`/org/attendance-integrations?as_of=<AS_OF_BASE>`，断言页面可见且包含 Pending/Active/Disabled/Ignored 四个区块与表格列（provider/external_user_id/status/person/last_seen_at/seen_count）。

### 8.2 映射创建（link）与可见性

> 两种执行方式二选一：
> - 方式 A（已知 external_user_id）：直接在 Link 表单创建 active 映射；
> - 方式 B（未知 external_user_id）：先触发外部事件生成 Pending 行，再在 Pending 行内完成 Link。

1. [ ] E08（DingTalk）映射：`provider=DINGTALK` + `external_user_id=<E08_DINGTALK_USER_ID>` → `person_uuid=<E08_PERSON_UUID>`
   - 断言：记录出现在 Active 表；person 列显示 E08（或其 pernr/uuid）。
2. [ ] E09（WeCom）映射：`provider=WECOM` + `external_user_id=<E09_WECOM_USER_ID>` → `person_uuid=<E09_PERSON_UUID>`
   - 断言：记录出现在 Active 表。
3. [ ] 负例（必须）：提交空 `external_user_id` 或空 `person_uuid`（或 provider 非法）
   - 断言：返回 422 且页面展示错误信息（例如 `external_user_id is required` / `person_uuid is required` / `provider must be DINGTALK|WECOM`）。

### 8.3 状态机覆盖（UI 操作）

> 目标：完成一次完整流转并留下页面证据。

以 E09（WECOM）为例（若环境已有记录可复用）：
1. [ ] active → disabled：点击 `Disable`
2. [ ] disabled → active：点击 `Enable`
3. [ ] active → pending：点击 `Unlink`（断言：出现在 Pending 表，person 列为 `-`）
4. [ ] pending → ignored：点击 `Ignore`（断言：出现在 Ignored 表）
5. [ ] ignored → pending：点击 `Unignore`
6. [ ] pending → active：在 Pending 行内选择 person 并点击 `Link`
7. [ ] 负例（建议）：对不存在的 `provider/external_user_id` 执行 disable/enable/unlink
   - 断言：422 且提示 `identity link not found (or ...)`（或等效稳定错误信息）。

### 8.4 外部事件摄入（路径 A：真实外部对接）

> 若无外部凭证或无法触发外部事件：跳过本节，并在 §10 记录 `ENV_DRIFT`（说明缺少的环境能力与复现阻塞点）。

1. [ ] 启动 Worker（示例）：`TENANT_ID=<T060_TENANT_ID> DATABASE_URL=... go run ./cmd/attendance-integrations`
2. [ ] **未映射断言（必须，关键闭环）**：触发一条来自 DingTalk 的外部事件，且 `external_user_id=<UNMAPPED_DINGTALK_USER_ID>`（确保执行前未被 link）。
   - 记录：Worker 日志中的 `provider/external_user_id/request_id/outcome`（期望 `outcome=unmapped`）。
   - 断言 A（UI 可见）：刷新 `/org/attendance-integrations?as_of=<AS_OF_BASE>`，Pending 表出现：`provider=DINGTALK` + `external_user_id=<UNMAPPED_DINGTALK_USER_ID>`。
   - 断言 B（计数增长，delta）：再次触发同一用户的第二条外部事件后，`seen_count` 必须增加，`last_seen_at` 必须更新。
   - 断言 C（可判定，不落库）：可选 DB probe（建议）
     - 在 DB 内查询（示例，需满足 RLS 租户注入）：
       ```sql
       BEGIN;
       SELECT set_config('app.current_tenant', '<T060_TENANT_ID>', true);
       SELECT count(*) FROM staffing.time_punch_events
       WHERE tenant_id = '<T060_TENANT_ID>'::uuid AND request_id = '<UNMAPPED_REQUEST_ID>';
       ROLLBACK;
       ```
     - 期望：`0`（未映射不得写入 punches）。
3. [ ] **已映射摄入断言（必须）**：触发外部事件命中已映射用户（E08 或 E09）。建议同一 work_date 触发两次（形成一对 punches，便于日结果出现）。
   - 记录：两次事件各自的 `request_id` 与日志 `outcome`（期望 `outcome=ingested`）。
   - 断言 A（integrations delta）：目标行 `seen_count/last_seen_at` 发生增长/更新。
   - 断言 B（punches 可见）：打开 punches 页面（目标 `person_uuid` + `from_date/to_date=<WORK_DATE>`），结果表出现 `Source=DINGTALK|WECOM` 的记录（DingTalk 期望 `Type=RAW`）。
   - 断言 C（daily results 可见）：打开 `/org/attendance-daily-results?as_of=<WORK_DATE>&work_date=<WORK_DATE>`，能看到目标人员行，且可进入详情页。
   - 排障口径（必须记录）：若 punches 已出现但 daily results 为空，允许在详情页执行一次 `Recalc This Day`（`op=recalc_day`）后重试；若必须手工重算才出现，记录为 `BUG/CONTRACT_DRIFT`（契约引用：`DEV-PLAN-052/055`）。
4. [ ] **disabled 不摄入断言（必须）**：将 E08 或 E09 映射置为 `disabled`，再触发同一 `external_user_id` 的外部事件。
   - 记录：Worker 日志 `outcome`（期望 `disabled`）与该次 `request_id`。
   - 断言 A（integrations delta）：`seen_count/last_seen_at` 仍应更新（表示事件到达但被禁用）。
   - 断言 B（不落库）：可选 DB probe（建议）
     - `SELECT count(*) FROM staffing.time_punch_events WHERE tenant_id='<T060_TENANT_ID>'::uuid AND request_id='<DISABLED_REQUEST_ID>';`（同上，需 `set_config('app.current_tenant', ...)`）
     - 期望：`0`。
5. [ ] **幂等断言（建议，优先 WeCom）**：保持 WeCom Poller 运行至少 2 个 interval（lookback 覆盖同一条 check-in），观察同一 `request_id` 多次出现。
   - 断言 A（DB）：对该 `request_id`，`staffing.time_punch_events` 计数始终为 `1`（同上，需 `set_config('app.current_tenant', ...)`）。
   - 断言 B（UI）：punches 列表不出现重复行（同一时间点/同一 source 的重复事件）。

### 8.5 安全与隔离（必须）

1. [ ] Authz（403）：以只读用户对 `/org/attendance-integrations?as_of=<AS_OF_BASE>` 执行任一 POST（例如 link/disable）
   - 断言：403（对齐 TP-060-01 的“tenant-viewer 只读”口径；若无法分配只读角色，记录为 `CONTRACT_MISSING/ENV_DRIFT`）。
2. [ ] fail-closed（缺 tenant context）：用非租户 host 访问（示例）：`http://127.0.0.1:8080/org/attendance-integrations?as_of=<AS_OF_BASE>`
   - 断言：不得展示任何租户数据（允许 404/403/500/稳定错误码）。
3. [ ] （可选）跨租户隔离：在 `T060` 下使用 `T060B_PERSON_UUID` 打开 punches 或 daily results 页面
   - 断言：不得泄漏 T060B 数据（空/404/稳定错误码均可）。

## 9. 验收证据与执行记录（最小）

- integrations 页面证据：E08/E09 的映射记录可见，且状态机流转证据（至少包含 pending/active/disabled/ignored 四种状态的截图或等效证据）。
- Worker 日志证据（路径 A）：至少 1 条 `outcome=unmapped` 与 1 条 `outcome=ingested`（需包含 `provider/external_user_id/request_id`）。若无外部事件能力，必须在问题记录标注 `ENV_DRIFT`。
- `seen_count/last_seen_at` 证据：至少 1 次“外部事件到达后增长（delta）”的证据（路径 A）。
- punches 证据：至少 1 条 `Source=DINGTALK|WECOM` 的 punch 记录（含时间、Type、Source、EventID）。
- daily results 证据：对应 `work_date` 的列表可见 + 详情页可打开（最小）。
- 安全证据：只读角色一次 403（若可执行）+ 缺 tenant context 的 fail-closed 负例。

### 9.1 执行记录（Readiness/可复现记录）

> 说明：此处只记录“本次执行实际跑了什么、结果如何”；命令入口以 `AGENTS.md` 为准。执行时把 `[ ]` 改为 `[X]` 并补齐时间戳与结果摘要。

- [ ] 文档门禁：`make check doc`
- [ ] （可选）E2E：`make e2e`（若补齐 TP-060-06 自动化用例）
- [ ] （可选）Authz：`make authz-pack && make authz-test && make authz-lint`
- [ ] （可选）路由治理：`make check routing`

## 10. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
