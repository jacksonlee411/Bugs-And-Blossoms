# DEV-PLAN-010 Readiness（证据记录）

> 目的：把 “P0-Ready” 的关键结论固化为可审计证据（时间戳/环境/命令/结果）。
> 本文件为模板；每次完成一个里程碑，在对应小节补齐证据。

## 1. 基本信息

- repo: Bugs-And-Blossoms
- 分支保护：main 禁止直推/禁止 force-push/必须 PR，并冻结 required checks（GitHub 侧配置）

## 2. Required Checks（不出现 skipped）

- `Code Quality & Formatting`：`make check fmt` / `make check lint`
- `Unit & Integration Tests`：`make test`
- `Routing Gates`：`make check routing`
- `E2E Tests`：`make e2e`

证据（贴运行时间与结论链接/截图均可）：
- 日期：2026-01-06
- 运行环境：GitHub Actions（Quality Gates）
- 结论：PR 合并均为 4/4 checks 成功（不出现 skipped）
  - PR-3（Routing Gates）：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/3
  - PR-5（UI shell）：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/5
  - PR-6（最小登录）：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/6
  - PR-7（DB gates）：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/7
  - PR-8（sqlc/authz toolchain）：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/8
  - PR-10（orgunit P0 slice）：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/10

## 3. UI 壳（用户可见性）

证据：
- PR-5：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/5
- PR-80（Astro build + go:embed Phase 0 收口）：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/80
- 路由入口：`/app`（壳）+ 导航占位页（Org/JobCatalog/Staffing/Person）
- 说明：`/app` 已收口为 Astro build 产物 + `go:embed`（`internal/server/assets/astro/app.html`），并按 `DEV-PLAN-018` 注入 `__BB_AS_OF__`；证据与闭环见 `docs/dev-plans/009m6-phase1-astro-build-phase0-execution-plan.md` 与本文件 §17。

## 4. 最小登录链路

证据：
- PR-6：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/6
- 行为：Host→tenant（fail-closed）；未登录重定向 `/login`；登录后进入 `/app`

## 5. Routing Gates

证据：
- PR-3：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/3
- allowlist SSOT：`config/routing/allowlist.yaml`
- 本地门禁：`make check routing`

## 6. DB/迁移闭环（至少 iam）

证据：
- PR-7：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/7
- 入口：
  - `make iam plan`
  - `make iam lint`（atlas migrate validate）
  - `make iam migrate up`（goose + `cmd/dbtool rls-smoke`）

## 7. sqlc / Authz 工具链

证据：
- PR-8：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/8
- sqlc：`make sqlc-generate`（导出 `internal/sqlc/schema.sql` + 生成物提交）
- authz：`make authz-pack && make authz-test && make authz-lint`（生成 `config/access/policy.csv(.rev)` 并纳入一致性门禁）

## 8. 本地开发一键启动（避免端口/环境漂移）

证据：
- PR-25：新增 `make dev` / `make dev-up` / `make dev-server`（自动加载 `.env.local`/`env.local`/`.env`）
- 日期：2026-01-06
- 结论：
  - `make dev-up` 后 postgres/redis 可用（端口来自 `.env.local`）
  - `make dev-server` 启动的 server 不会回落到默认 DB 端口（避免出现 `127.0.0.1:5438 connect: connection refused`）

## 9. 浏览器验证脚本（本地可复现）

目的：把“从启动到可见业务操作”的最小闭环固化为可复现步骤。

- 启动：
  - `make dev`（或分别运行 `make dev-up` + `make dev-server`）
- 打开并登录：
  - `http://localhost:8080/login`（点击 Login 按钮）
- 访问 OrgUnit（单链路 current）：
  - `http://localhost:8080/org/nodes?as_of=2026-01-06`（读取 current；失败/为空显式报错并引导修复/重试）

## 10. DEV-PLAN-009M1（SetID + JobCatalog 纵切片）

证据：
- 日期：2026-01-06
- 本地门禁：`make preflight`（全绿）
- 新增表/迁移（红线）手工确认：用户已在对话中确认追认（2026-01-06）
- DB 闭环（Atlas+Goose + smoke）：
  - `make iam plan && make iam lint && make iam migrate up`（含 `rls-smoke`）
  - `make orgunit plan && make orgunit lint && make orgunit migrate up`（含 `orgunit-smoke`）
  - `make jobcatalog plan && make jobcatalog lint && make jobcatalog migrate up`（含 `jobcatalog-smoke`）
- 端到端（HTTP/curl，可复现）：
  - 前置：`make dev-up`（需要本机 `.env.local`/`env.local`/`.env` 提供 `DB_PORT` 等；本仓库 `.env.local` 已被 `.gitignore` 忽略）
  - 启动：`make dev-server`
  - 登录：`curl -i -X POST -H 'Host: localhost:8080' -c /tmp/bb_cookies.txt http://127.0.0.1:8080/login`（拿到 `session=ok`）
  - SetID/BU 创建：`curl -X POST -H 'Host: localhost:8080' -b /tmp/bb_cookies.txt -d 'action=create_setid&setid=S2601&name=Smoke+SetID' http://127.0.0.1:8080/org/setid`（303）
  - BU 创建：`curl -X POST -H 'Host: localhost:8080' -b /tmp/bb_cookies.txt -d 'action=create_bu&business_unit_id=BU901&name=Smoke+BU' http://127.0.0.1:8080/org/setid`（303）
  - Mappings 保存：`curl -X POST -H 'Host: localhost:8080' -b /tmp/bb_cookies.txt -d 'action=save_mappings&map_BU000=SHARE&map_BU901=S2601' http://127.0.0.1:8080/org/setid`（303）
  - JobCatalog 解析验证：`curl -H 'Host: localhost:8080' -b /tmp/bb_cookies.txt 'http://127.0.0.1:8080/org/job-catalog?as_of=2026-01-01&business_unit_id=BU901'`（页面显示 `Resolved SetID: S2601`）
  - Job Family Group 创建：`curl -X POST -H 'Host: localhost:8080' -b /tmp/bb_cookies.txt -d 'action=create_job_family_group&effective_date=2026-01-01&business_unit_id=BU901&code=JC901&name=Smoke+Group&description=' 'http://127.0.0.1:8080/org/job-catalog?business_unit_id=BU901&as_of=2026-01-01'`（303）
  - 列表读取验证：同 GET 页面可见 `JC901 / Smoke Group` 行（写入→列表读取闭环）

## 11. DEV-PLAN-009M2（Person Identity + Staffing 纵切片）

证据：
- 日期：2026-01-07
- 合并记录：PR #43 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/43
- 运行模式：
  - `RLS_ENFORCE=enforce`（对齐 `.env.example`；DB role 为非 superuser）
  - `AUTHZ_MODE=enforce`（默认 enforce；见 `pkg/authz`）

DB 闭环（迁移 + smoke）：
- `make person migrate up`（含 `person-smoke`）
- `make staffing migrate up`（含 `staffing-smoke`）

端到端（HTTP/curl，可复现）：
- 前置启动：
  - `make dev-up`
  - `make iam migrate up && make orgunit migrate up && make jobcatalog migrate up && make person migrate up && make staffing migrate up`
  - `make dev-server`
- 登录拿 cookie（注意 tenant 解析依赖 Host；`127.0.0.1` 会 fail-closed）：
  - `curl -i -X POST -H 'Host: localhost:8080' -c /tmp/bb_m2_cookies.txt http://127.0.0.1:8080/login`
- 创建 Person（pernr 1-8 位数字；含前导 0 同值）：
  - `curl -i -X POST -H 'Host: localhost:8080' -b /tmp/bb_m2_cookies.txt -d 'pernr=101&display_name=Smoke+Person+101' http://127.0.0.1:8080/person/persons`
- 确保存在 OrgUnit（用于 Position 的 `org_unit_id` 输入来源）：
  - 打开 `http://localhost:8080/org/nodes?as_of=2026-01-07`，若为空则创建 1 条 OrgUnit；随后从 `http://localhost:8080/org/positions?as_of=2026-01-07` 的下拉选项中取任一 `org_unit_id`。
- 创建 Position：
  - `curl -i -X POST -H 'Host: localhost:8080' -b /tmp/bb_m2_cookies.txt -d 'effective_date=2026-01-07&org_unit_id=<uuid>&name=Smoke+Position+101' http://127.0.0.1:8080/org/positions?as_of=2026-01-07`
  - 验证列表：`http://localhost:8080/org/positions?as_of=2026-01-07` 可见新行（包含 `position_id`）。
- 创建/更新 Assignment（primary upsert；写侧权威输入为 `person_uuid`，UI 允许输入 pernr 并解析）：
  - `curl -i -X POST -H 'Host: localhost:8080' -b /tmp/bb_m2_cookies.txt -d 'effective_date=2026-01-07&pernr=101&position_id=<uuid>' http://127.0.0.1:8080/org/assignments?as_of=2026-01-07`
  - 验证时间线：`http://localhost:8080/org/assignments?as_of=2026-01-07&pernr=101` 可见新增行，且 UI 只展示 `effective_date`（不展示 `end_date`）。
- Person read API（精确解析）：
  - `curl -i -H 'Host: localhost:8080' -b /tmp/bb_m2_cookies.txt 'http://127.0.0.1:8080/person/api/persons:by-pernr?pernr=101'`

失败路径（最小 2 条）：
- 非法 pernr：
  - `curl -i -H 'Host: localhost:8080' -b /tmp/bb_m2_cookies.txt 'http://127.0.0.1:8080/person/api/persons:by-pernr?pernr=BAD'`（400 `PERSON_PERNR_INVALID`）
- 403（授权可拒绝）：
  - 运行态行为由 `AUTHZ_MODE=enforce` + policy 决定；单测覆盖见 `internal/server/authz_middleware_test.go` 的 `TestWithAuthz_ForbiddenWhenEnforced`（403）。

## 12. DEV-PLAN-009M3（Phase 5：E2E 真实化 + 可排障门禁）

证据：
- 日期：2026-01-07
- 合并记录：PR #49 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/49
- 本地门禁：`make preflight`（全绿）
- E2E 入口（SSOT）：`make e2e`（Playwright 真实浏览器 smoke；fail-fast；failure artifact）

复现（本地）：
- 运行：`make e2e`
  - 默认使用：`E2E_BASE_URL=http://localhost:8080`（强约束：必须 `localhost`，禁止 `127.0.0.1`，对齐 Host→tenant fail-closed）。
  - 默认运行态：`AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`，且禁止 `AUTHZ_UNSAFE_ALLOW_DISABLED=1`。
  - DB runtime 角色：`app_runtime`（非 superuser 且 `NOBYPASSRLS`；E2E 脚本会断言）。
- 若本机 dev Postgres volume 早于该里程碑创建（未执行 init scripts），可能缺少 `app_runtime`：先运行 `make dev-reset`（会清空 dev volume）再重跑 `make e2e`。

失败时证据落点：
- Playwright 产物：`e2e/test-results/**`、`e2e/playwright-report/**`（trace/screenshot/video retain-on-failure）
- Server/SuperAdmin/Kratos 启动日志：`e2e/_artifacts/server.log`、`e2e/_artifacts/superadmin.log`、`e2e/_artifacts/kratosstub.log`

## 13. DEV-PLAN-009M4（Phase 2：SuperAdmin 控制面 + Tenant Console MVP）

证据：
- 日期：2026-01-07
- 合并记录：PR #55 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/55
- 本地门禁：`make preflight`（全绿，含 `make e2e`）
- E2E smoke：`e2e/tests/m3-smoke.spec.js`（superadmin→创建 tenant/domain→tenant app 登录→访问受保护页面）

复现（本地）：
- 一键：`make preflight`（会自动跑 e2e）
- 仅 e2e：`make e2e`
  - server：`http://localhost:8080`
  - superadmin：`http://localhost:8081`（Phase 0 BasicAuth；dev 默认 `admin/admin`，见 `Makefile` 的 `dev-superadmin`）

失败时证据落点：
- Playwright 产物：`e2e/test-results/**`、`e2e/playwright-report/**`
- server/superadmin/kratos 日志：`e2e/_artifacts/server.log`、`e2e/_artifacts/superadmin.log`、`e2e/_artifacts/kratosstub.log`

## 14. DEV-PLAN-009M5（Phase 2：AuthN 真实化：Kratos + 本地会话 sid/sa_sid）

证据：
- 日期：2026-01-08
- 合并记录：
  - PR #58 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/58
  - PR #60 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/60
  - PR #61 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/61
  - PR #62 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/62
  - PR #63 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/63
- 本地门禁：`make preflight`（全绿，含 `make e2e`；coverage 门禁 100%）
- E2E smoke：`e2e/tests/m3-smoke.spec.js`
  - superadmin：`/superadmin/login`（Kratos 认人 → `sa_sid`）
  - tenant app：`/login`（Kratos 认人 → `sid`）

复现（本地）：
- 一键：`make preflight`
- 仅 e2e：`make e2e`
  - server：`http://localhost:8080`
  - superadmin：`http://localhost:8081`（E2E 脚本仍启用 BasicAuth 外层保护；E2E 会先走 `/superadmin/login` 建立 `sa_sid`）
  - Kratos stub：
    - public：`http://127.0.0.1:4433`（`KRATOS_PUBLIC_URL`）
    - admin：`http://127.0.0.1:4434`（`E2E_KRATOS_ADMIN_URL`）
  - superadmin login identity（默认）：
    - email：`admin+<runID>@example.invalid`（默认用 runID 做唯一化，避免本地多次运行时与既有 principal 的 `kratos_identity_id` 绑定冲突；可用 `E2E_SUPERADMIN_EMAIL` 覆盖为固定值）
    - identifier：`sa:<email>`
    - password：`E2E_SUPERADMIN_LOGIN_PASS`（未设置时回退 `E2E_SUPERADMIN_PASS`）

失败时证据落点：
- Playwright 产物：`e2e/test-results/**`、`e2e/playwright-report/**`
- server/superadmin/kratos 日志：`e2e/_artifacts/server.log`、`e2e/_artifacts/superadmin.log`、`e2e/_artifacts/kratosstub.log`

## 15. DEV-PLAN-021（RLS 强租户隔离：No Tx, No RLS）

证据：
- 日期：2026-01-08
- 合并记录：PR #66 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/66
- 运行模式：`RLS_ENFORCE=enforce`（运行态 DB role 非 superuser 且 `NOBYPASSRLS`）
- DB 闭环（迁移 + smoke；fail-closed/隔离/tenant mismatch）：
  - `make iam migrate up`（含 `cmd/dbtool rls-smoke`）
  - `make orgunit migrate up`（含 `cmd/dbtool orgunit-smoke`）
  - `make jobcatalog migrate up`（含 `cmd/dbtool jobcatalog-smoke`）
  - `make person migrate up`（含 `cmd/dbtool person-smoke`）
  - `make staffing migrate up`（含 `cmd/dbtool staffing-smoke`）

复现（本地）：
- 一键：`make preflight`
- 仅 DB/RLS：按上面模块逐个执行 `make <module> migrate up`

## 16. DEV-PLAN-022（Authz：Casbin 工具链 + 403 契约 + enforce/shadow）

证据：
- 日期：2026-01-08
- 合并记录：PR #67 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/67
- policy SSOT：`config/access/policies/**` → `config/access/policy.csv`（pack 产物）
- 本地门禁：
  - `make authz-pack && make authz-test && make authz-lint`
  - `make preflight`（全绿，E2E 默认 `AUTHZ_MODE=enforce`）

## 17. DEV-PLAN-009M6（Phase 1：Astro build + go:embed Shell）

证据：
- 日期：2026-01-08
- 合并记录：
  - PR #76 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/76
  - PR #77 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/77
  - PR #78 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/78
  - PR #79 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/79
  - PR #80 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/80
- 本地验证（全绿）：
  - Go：`go fmt ./...`、`go vet ./...`、`make check lint`、`make test`
  - Routing：`make check routing`
  - UI build：`make css`（生成 `internal/server/assets/astro/**`；生成后 `git status --short` 为空）
  - E2E：`make e2e`
  - Docs/Stopline：`make check doc`、`make check no-legacy`
- CI（不出现 skipped）：PR #80 的 Quality Gates 4/4 全绿（Gate-1 命中 UI 变更时执行 `make css` 并通过 `assert-clean`）

## 18. DEV-PLAN-041（Payroll P0-1：Pay Period & Payroll Run）

证据（Milestone 1-2：Schema SSOT + 迁移闭环）：
- 日期：2026-01-08
- 合并记录：PR #82 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/82
- 产出：
  - Schema SSOT：`modules/staffing/infrastructure/persistence/schema/00004_staffing_payroll_tables.sql`
  - 迁移：`migrations/staffing/20260108131301_staffing_payroll_tables.sql` + `migrations/staffing/atlas.sum`
  - sqlc 生成物：`internal/sqlc/schema.sql`、`modules/iam/infrastructure/sqlc/gen/models.go`
- 本地验证：
  - `make staffing plan`（No drift）
  - `make staffing lint`（atlas migrate validate）
  - `make staffing migrate up`（含 `cmd/dbtool staffing-smoke`）
  - `make sqlc-generate` 后 `git status --short` 为空
- CI（Quality Gates）：PR #82 4/4 全绿（包含 `assert-clean`）

证据（Milestone 3：Kernel submit 函数）：
- 日期：2026-01-08
- 合并记录：PR #84 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/84
- 产出：
  - Schema SSOT：`modules/staffing/infrastructure/persistence/schema/00005_staffing_payroll_engine.sql`
  - 迁移：`migrations/staffing/20260108133000_staffing_payroll_engine.sql` + `migrations/staffing/atlas.sum`
- 本地验证：
  - `make staffing plan && make staffing lint && make staffing migrate up`
  - `make sqlc-generate` 后 `git status --short` 为空

证据（Milestone 4：Server Store + handlers）：
- 日期：2026-01-08
- 合并记录：PR #85 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/85
- 产出：
  - Server：`internal/server/payroll.go`、`internal/server/payroll_handlers.go`
  - Tests：`internal/server/payroll_store_test.go`、`internal/server/payroll_handlers_test.go`
- 本地验证：
  - `go fmt ./... && go vet ./... && make check lint && make test`
- CI（Quality Gates）：PR #85 4/4 全绿

证据（Milestone 5：Routing/Authz + 路由接入）：
- 日期：2026-01-08
- 合并记录：PR #86 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/86
- 产出：
  - Routing allowlist：`config/routing/allowlist.yaml`
  - Authz registry/route map：`pkg/authz/registry.go`、`internal/server/authz_middleware.go`
  - Authz policy SSOT：`config/access/policies/00-bootstrap.csv`（生成：`config/access/policy.csv`、`config/access/policy.csv.rev`）
  - Server route wiring（UI + internal API）：`internal/server/handler.go`
- 本地验证：
  - `go fmt ./... && go vet ./... && make check lint && make test`
  - `make check routing`
  - `make authz-pack && make authz-test && make authz-lint`
- CI（Quality Gates）：PR #86 4/4 全绿

证据（Milestone 6：Tests（状态机/幂等/RLS fail-closed））：
- 日期：2026-01-08
- 合并记录：PR #89 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/89
- 产出：
  - Kernel（幂等修复）：`modules/staffing/infrastructure/persistence/schema/00005_staffing_payroll_engine.sql`
  - 迁移：`migrations/staffing/20260108180000_staffing_payroll_run_event_idempotency.sql` + `migrations/staffing/atlas.sum`
  - DB Smoke：`cmd/dbtool/main.go`（`staffing-smoke` 覆盖 overlap/invalid transition/finalized readonly/idempotent finalize/RLS）
  - sqlc 生成物：`internal/sqlc/schema.sql`
- 本地验证：
  - Go：`go fmt ./... && go vet ./... && make check lint && make test`
  - DB：`make staffing plan && make staffing lint && make staffing migrate up`（含 `cmd/dbtool staffing-smoke`）
  - `make sqlc-generate` 后 `git status --short` 为空
- CI（Quality Gates）：PR #89 4/4 全绿（E2E 路径包含 `make staffing migrate up`，并运行扩展后的 `staffing-smoke`）

## 19. DEV-PLAN-042（Payroll P0-2：Payslip & Pay Items）

证据（Milestone 1-5：Schema/Kernel/Server/Routing/Authz）：
- 日期：2026-01-09
- 合并记录：
  - PR #92 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/92
  - PR #93 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/93
  - PR #94 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/94
  - PR #95 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/95

证据（Milestone 6：Tests（pro-rate/FTE/舍入点/失败路径/RLS fail-closed））：
- 日期：2026-01-09
- 合并记录：PR #96 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/96
- 产出：
  - DB Smoke：`cmd/dbtool/main.go`（`staffing-smoke` 覆盖 pro-rate/FTE/舍入点/失败路径/RLS fail-closed）
- 本地验证：
  - `go fmt ./... && go vet ./... && make check lint && make test`
- CI（Quality Gates）：PR #96 4/4 全绿

## 20. DEV-PLAN-051（考勤 Slice 4A：Punch Ledger）

证据：
- 日期：2026-01-09
- 合并记录（按 `docs/dev-plans/051-attendance-slice-4a-punch-ledger.md` §8.2 每步一个 PR）：
  - Step1 Routing：PR #102 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/102
  - Step2 Authz：PR #103 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/103
  - Step3 DB（表 + RLS + kernel）：PR #104 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/104
  - Step4 sqlc（生成物清洁）：PR #105 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/105
  - Step5 Go（store + handlers + nav + route wiring）：PR #106 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/106
  - Step6 Tests（RLS fail-closed / 跨租户隔离 / 幂等冲突）：PR #108 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/108
- 本地门禁（结论：全绿）：
  - Go：`go fmt ./... && go vet ./... && make check lint && make test`
  - DB（闭环）：`make staffing plan && make staffing lint && make staffing migrate up`
- CI（Quality Gates）：上述 PR 均为 4/4 checks 全绿（不出现 skipped）
- 新增表/迁移（红线）手工确认：已在对话中确认（2026-01-09）

## 21. DEV-PLAN-052（考勤 Slice 4B：日结果计算闭环（标准班次））

证据：
- 日期：2026-01-09
- 合并记录（按 `docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md` §8.2 每步一个 PR）：
  - Step1 Routing：PR #110 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/110
  - Step2 Authz：PR #112 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/112
  - Step3 DB（表 + RLS + kernel）：PR #113 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/113
  - Step4 sqlc（生成物清洁）：PR #115 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/115
  - Step5 Go（store + handlers + nav + API）：PR #117 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/117
  - Step6 Tests（RLS fail-closed / 跨租户隔离 / 日结果核心用例）：PR #123 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/123
  - Step7 E2E（smoke：补卡 → 日结果可见/可解释）：PR #125 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/125
- 本地门禁（结论：全绿）：
  - 2026-01-09T09:51:35Z：`go fmt ./... && go vet ./... && make check lint && make test`
- CI（Quality Gates）：上述 PR 均为 4/4 checks 全绿（不出现 skipped）

## 22. DEV-PLAN-043（Payroll P0-3：社保政策（单城市）配置与扣缴计算）

证据：
- 日期：2026-01-09
- 合并记录：PR #111 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/111
- 新增表/迁移（红线）手工确认：已在对话中确认（2026-01-09）
- 产出：
  - Schema SSOT：
    - `modules/staffing/infrastructure/persistence/schema/00006_staffing_payroll_social_insurance_tables.sql`
    - `modules/staffing/infrastructure/persistence/schema/00007_staffing_payroll_social_insurance_engine.sql`
  - 迁移（Atlas+Goose）：
    - `migrations/staffing/20260109100000_staffing_payroll_p0_3_social_insurance_schema.sql`
    - `migrations/staffing/20260109101000_staffing_payroll_p0_3_social_insurance_kernel.sql`
    - `migrations/staffing/20260109102000_staffing_payroll_run_events_run_state_default.sql`
    - `migrations/staffing/20260109103000_staffing_payroll_run_events_calc_finish_ensure_payslips.sql`
    - `migrations/staffing/atlas.sum`
  - DB Smoke：`cmd/dbtool/main.go`（`staffing-smoke` 覆盖 policy/no-overlap/single-city/舍入/fail-closed/RLS）
- 本地门禁（结论：全绿）：
  - DB：`make staffing plan && make staffing lint && make staffing migrate up`
  - sqlc：`make sqlc-generate`（生成后 `git status --short` 为空）
  - Authz：`make authz-pack && make authz-test && make authz-lint`
  - Go：`go fmt ./... && go vet ./... && make check lint && make test`
  - Stopline：`make check routing && make check no-legacy && make check doc`
- CI（Quality Gates）：PR #111 4/4 全绿（coverage 门禁 100%）
  - Step3 DB（表 + RLS + kernel）：PR #113 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/113
  - Step4 sqlc（生成物清洁）：PR #115 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/115
