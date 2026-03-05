# DEV-PLAN-239D：Ubuntu 本地开发环境搭建方案（Clone 后首日）

**状态**: 进行中（2026-03-05 07:25 UTC）

## 1. 背景与目标

- 项目已在 Ubuntu 本机完成 clone，需要一套可复现、可验收的本地开发环境搭建流程。
- 本方案目标是完成“可开发 + 可运行 + 可验证”的最小闭环，且与仓库既有 SSOT 对齐，不引入个人口径漂移。
- 历史开发环境以 WSL 为主；本次明确切换为**原生 Ubuntu**，需要补齐环境差异校验与停止线。

## 1.1 本次冻结决策（针对 239D）

1. [X] 本机为原生 Ubuntu，未安装 Docker Desktop。
2. [X] 运行时数据库账号强制使用 `app_runtime`（严格对齐 E2E/RLS）。
3. [X] 本地环境文件统一使用 `.env`（不使用 `.env.local` 作为本方案主入口）。

## 2. 范围与非目标

### 2.1 范围（本方案覆盖）
1. [ ] 安装并校验本地工具链（Go/Node/Docker/Make 等）。
2. [ ] 启动本地基础设施（PostgreSQL/Redis）。
3. [ ] 执行模块迁移并启动应用服务。
4. [ ] 完成健康检查与最小门禁验证。

### 2.2 非目标（本方案不覆盖）
1. [ ] 不包含生产环境部署与运维编排。
2. [ ] 不包含业务数据初始化脚本的定制化扩展。
3. [ ] 不包含新增 schema/迁移/sqlc 改造。

## 3. 事实源（SSOT）

- `AGENTS.md`：触发器矩阵、门禁入口、开发红线。
- `Makefile`：本地启动、迁移、测试、门禁统一入口。
- `.tool-versions` / `.nvmrc` / `go.mod`：本地工具链版本口径。
- `compose.dev.yml` / `.env.example`：本地 infra 与默认环境变量。
- `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`：技术栈版本冻结口径。
- `docs/dev-plans/014-parallel-worktrees-local-dev-guide.md`：本地 worktree 与共享 infra 约定。

## 4. 实施步骤（Ubuntu）

### 4.0 Phase 0：WSL -> Ubuntu 差异校验（先做）
1. [X] 校验 Docker context 指向本机：`docker context ls` 与 `docker context show`（期望 `default`）。
2. [X] 校验当前用户可直接使用 Docker：`id -nG | grep -w docker`。
3. [X] 记录主机信息：`uname -a`、`lsb_release -a`（用于后续问题回溯）。

### 4.1 Phase A：主机依赖与版本校验
1. [X] 安装基础命令：`make`、`git`、`curl`、`python3`、`gcc`、`docker`、`docker compose plugin`。
2. [X] 安装 Go `1.26.0`，并执行 `go version` 校验。
3. [X] 安装 Node `20.19.0`（或 20.x 且与 `.nvmrc` 一致），启用 `corepack`。
4. [X] 执行 `docker --version`、`docker compose version`、`node -v`、`go version` 并留存结果。

### 4.2 Phase B：仓库初始化
1. [X] 复制环境模板：`cp .env.example .env`（仅本机使用，`.env` 已在 `.gitignore`）。
2. [X] 编辑 `.env` 并冻结关键值：
   - `DB_USER=app_runtime`
   - `DB_PASSWORD=app`
   - `RLS_ENFORCE=enforce`
   - `AUTHZ_MODE=enforce`
3. [X] 导出 infra 环境文件口径：`export DEV_INFRA_ENV_FILE=.env`。
4. [X] 校验 Go 版本门禁：`make check go-version`。
5. [X] 预热工具链：`go mod download`。
6. [X] 预热前端依赖并构建静态资源：`make css`。

### 4.3 Phase C：本地基础设施与数据库闭环
1. [X] 启动依赖服务：`DEV_INFRA_ENV_FILE=.env make dev-up`。
2. [X] 校验容器状态：`DEV_INFRA_ENV_FILE=.env make dev-ps`（确认 postgres/redis healthy）。
3. [X] 使用 admin 连接执行迁移（对齐 E2E 的 migrate 口径）：  
   - `DATABASE_URL=postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable make iam migrate up`  
   - `DATABASE_URL=postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable make orgunit migrate up`  
   - `DATABASE_URL=postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable make jobcatalog migrate up`  
   - `DATABASE_URL=postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable make person migrate up`  
   - `DATABASE_URL=postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable make staffing migrate up`
4. [X] 执行授权产物收敛：`make authz-pack`。
5. [X] 校验 `app_runtime` 角色旗标（期望 `f|f|t` = 非 superuser / 非 bypassrls / 可登录）：  
   - `docker compose -p "${DEV_COMPOSE_PROJECT:-bugs-and-blossoms-dev}" --env-file ".env" -f compose.dev.yml exec -T postgres psql -h localhost -U app -d postgres -tAc "SELECT (CASE WHEN rolsuper THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolbypassrls THEN 't' ELSE 'f' END) || '|' || (CASE WHEN rolcanlogin THEN 't' ELSE 'f' END) FROM pg_roles WHERE rolname='app_runtime';"`
6. [X] 运行 RLS 烟测（runtime 账号）：  
   - `go run ./cmd/dbtool rls-smoke --url "postgres://app_runtime:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable"`

### 4.4 Phase D：服务启动与最小可用验证
1. [X] 启动主服务：`DEV_SERVER_ENV_FILE=.env make dev-server`（默认 `:8080`）。
2. [ ] 可选启动 SuperAdmin：`DEV_SUPERADMIN_ENV_FILE=.env make dev-superadmin`（默认 `:8081`）。
3. [X] 验证健康检查：`curl -fsS http://127.0.0.1:8080/health`。
4. [ ] 若需本地登录链路联调，另起终端运行：`make dev-kratos-stub`。

### 4.5 Phase E：首轮质量门禁（最小集）
1. [X] `go fmt ./...`
2. [X] `go vet ./...`
3. [X] `make check lint`
4. [X] `make test`
5. [ ] 可选：`make preflight`（与 CI 一键对齐）

## 5. 验收标准（完成定义）

1. [X] `make dev-up` 后本地 PostgreSQL/Redis 稳定可用，`make dev-ps` 状态正常。
2. [X] 五个模块迁移全部执行成功，且无报错回滚。
3. [X] `app_runtime` 角色旗标校验通过（`rolsuper=false`、`rolbypassrls=false`、`rolcanlogin=true`）。
4. [X] `rls-smoke`（runtime URL）通过。
5. [X] `make dev-server` 可启动，`/health` 返回成功。
6. [X] 最小质量门禁（fmt/vet/lint/test）全部通过。
7. [X] 工作区无未预期生成物漂移（`git status --short` 仅包含预期改动）。

## 6. 风险与处置

- 端口冲突（5438/6379/8080/8081）：优先停止冲突进程或在 `.env` 调整端口。
- Docker 权限问题：确认当前用户已加入 `docker` 组并重新登录会话。
- Docker context 异常：若 `docker context show` 非 `default`，先切回 `docker context use default`。
- 迁移连接失败：先确认 `make dev-ps` 健康，再重试 `make <module> migrate up`。
- Node/pnpm 漂移：统一通过 `corepack prepare pnpm@10.24.0 --activate`。

## 7. 执行记录（待实施后补充）

1. [ ] 在 `docs/dev-records/` 新增 `dev-plan-239d-execution-log.md`，记录时间、命令、结果。
2. [ ] 全部勾选完成后，将本计划状态更新为 `已完成` 并写入完成时间戳。
