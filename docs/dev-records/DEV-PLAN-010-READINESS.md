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
- 访问 OrgUnit（默认 current 读取；可回退）：
  - `http://localhost:8080/org/nodes?as_of=2026-01-06`（默认读取优先 current，失败/为空自动回退 legacy）
  - 强制 legacy：`http://localhost:8080/org/nodes?read=legacy`
  - 强制 current：`http://localhost:8080/org/nodes?read=current&as_of=2026-01-06`
