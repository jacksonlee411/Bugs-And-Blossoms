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
- 路由入口：`/app`（壳）+ 导航占位页（Org/JobCatalog/Staffing/Person）

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
