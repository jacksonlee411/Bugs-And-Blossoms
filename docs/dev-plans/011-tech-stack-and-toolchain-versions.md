# DEV-PLAN-011：技术栈与工具链版本冻结（Stack & Tooling Decisions）

**状态**: 草拟中（2026-01-05 09:20 UTC）

> 本文是 Greenfield（全新实施，实施路线图见 `DEV-PLAN-009`）的“技术栈 + 工具链”决策与版本基线文档：**明确我们用什么、用哪个版本、以什么为事实源（SSOT）**，避免本地/CI/部署版本漂移导致不可复现。

## 1. 背景与上下文

- 现有仓库已经形成一套可工作的技术栈与门禁体系（见 `AGENTS.md`/`Makefile`/`.github/workflows/quality-gates.yml`）。
- Greenfield 选择“全新实施”而非改造/迁移旧功能：需要在最早期就把**版本与工具链口径**冻结下来，作为后续实施计划的统一依赖（见 `DEV-PLAN-009`）。

## 2. 决策范围与原则

### 2.1 范围

- 运行时与基础设施：Go、PostgreSQL、Redis、容器基底镜像。
- UI 技术栈：React（Vite）+ MUI Core + MUI X（唯一用户 UI；见 `DEV-PLAN-090/091/092/103`），前端工程 SSOT 为 `apps/web`。
- 数据与迁移工具链：sqlc、Atlas、Goose、SQL 格式化门禁（pg_format）。
- 授权/路由/事件：Casbin、Routing Gates、Transactional Outbox（能力复用口径）。
- 质量门禁与测试：golangci-lint、go-cleanarch、Go test、Playwright E2E。
- 开发体验：Air、DevHub、Node/pnpm（用于 MUI UI build 与 E2E）。
- 部署形态：Docker 镜像与 compose 拓扑（含 superadmin）。

### 2.2 原则（SSOT 与可复现）

- **事实源（SSOT）**：
  - 命令/脚本：`Makefile`
  - CI 门禁：`.github/workflows/quality-gates.yml`
  - 本地服务编排：`devhub.yml`、`compose*.yml`
  - 示例环境变量：`.env.example`
- 版本与依赖：`go.mod`、`e2e/pnpm-lock.yaml`、以及 MUI UI 工程的 `package.json` 与 lockfile（`apps/web/package.json` + `apps/web/pnpm-lock.yaml`；由 `DEV-PLAN-103` 收口）。
- **版本冻结粒度**：
  - 开发/构建工具优先固定到**精确版本**（例如 `v0.3.857`）。
  - 容器镜像至少固定到**主版本 tag**（例如 `postgres:17`）；生产环境建议进一步固定 digest（由部署侧落地）。

## 3. 版本基线（冻结清单）

> “版本”优先引用仓库内的可验证来源；若某项在仓库内仍是浮动（例如 `:latest`），在表中明确标注为“浮动”，并在第 7 节给出收敛计划。

### 3.1 运行时与基础设施

| 组件 | 基线版本 | 来源/说明 |
| --- | --- | --- |
| Go | `1.26.0` | `go.mod` + `.github/workflows/quality-gates.yml` |
| PostgreSQL | `17`（`postgres:17`） | `compose.dev.yml`/`compose.yml`/CI service |
| Redis | `latest`（`redis:latest`，浮动） | `compose.dev.yml`/CI service |
| Docker 基底（构建） | 暂未落地（仓库暂无 `Dockerfile`） | 待补齐容器化交付后冻结 |
| Docker 基底（运行） | `alpine:3.21` | `Dockerfile`/`Dockerfile.superadmin` |

### 3.1.1 PostgreSQL 扩展（Kernel 依赖）

> 说明：`DEV-PLAN-026/030/029` 的 Kernel DDL 大量依赖下表扩展；因此它们属于“运行时基线”，必须在 schema SSOT/迁移里显式创建，避免环境漂移。

| 扩展 | 基线 | 用途（摘要） | 来源/说明 |
| --- | --- | --- | --- |
| `pgcrypto` | enabled | `gen_random_uuid()` 默认值等 | `migrations/org/00001_org_baseline.sql`、`migrations/person/00001_person_baseline.sql`；以及 `DEV-PLAN-026/030/029` |
| `btree_gist` | enabled | `EXCLUDE USING gist` + `gist_uuid_ops`（no-overlap） | `migrations/org/00001_org_baseline.sql`；以及 `DEV-PLAN-026/030/029` |
| `ltree` | enabled（OrgUnit） | 路径（子树/祖先链）查询 | `migrations/org/00001_org_baseline.sql`；以及 `DEV-PLAN-026` |

### 3.2 UI 技术栈（React SPA：MUI X）

> 说明：`DEV-PLAN-103` 已移除 Astro/旧局部渲染链路/Alpine/Shoelace 的旧 UI 链路；仓库内唯一用户 UI 为 **React SPA（MUI X）**。  
> UI 依赖版本以 `apps/web/package.json` + `apps/web/pnpm-lock.yaml` 为 SSOT，本节仅做“可读性摘要”。

#### 3.2.1 UI Build / 依赖版本（Vite）

| 组件 | 基线版本 | 来源/说明 |
| --- | --- | --- |
| pnpm | `10.24.0` | `apps/web/package.json#packageManager`（同时用于 E2E） |
| Node.js | `20.x`（推荐） | UI build 与 E2E 共同依赖；建议补齐可复现 pin（例如 `.tool-versions`/`.nvmrc`/CI） |
| Vite | `7.3.1` | `apps/web/package.json` |
| React | `19.2.4` | `apps/web/package.json` |
| MUI Core | `7.3.7` | `apps/web/package.json` |
| MUI X（DataGrid/TreeView/DatePickers） | `8.27.0` | `apps/web/package.json` |

#### 3.2.2 产物交付（go:embed）

- 唯一 UI 静态产物目录（入仓 + go:embed）：`internal/server/assets/web/**`
- 唯一 UI 静态资源 URL 前缀：`/assets/web/`
- 构建入口（SSOT）：`make css` → `scripts/ui/build-web.sh`

### 3.3 数据访问 / Schema / 迁移 / 生成

| 组件 | 基线版本 | 来源/说明 |
| --- | --- | --- |
| sqlc（CLI） | `v1.30.0` | `go.mod` tool directives + `scripts/sqlc/generate.sh`（`go tool sqlc`） |
| sqlc（Go module） | `v1.30.0` | `go.mod`（工具依赖） |
| Atlas（CLI） | `v0.38.0` | `Makefile` 的 `ATLAS_VERSION`（源码构建安装） |
| Goose（CLI） | `v3.26.0` | `go.mod` tool directives + `scripts/db/run_goose.sh`（`go tool goose`） |
| pgx（PostgreSQL driver） | `v5.8.0` | `go.mod`（Kernel port/adapters 推荐使用 `pgx`；见 `DEV-PLAN-016/031`） |
| goimports（用于生成物整理） | `v0.38.0` | `go.mod` tool directives + `scripts/sqlc/generate.sh`（`go tool goimports`） |
| SQL 格式化（pg_format） | OS 包（未 pin） | CI 安装 `pgformatter`（Ubuntu apt），本地用 `make check sqlfmt` 对齐 |

### 3.4 Authz / Routing / Outbox（能力复用）

| 组件 | 基线版本 | 来源/说明 |
| --- | --- | --- |
| GraphQL（gqlgen） | 未引入 | 当前 `go.mod` 无该依赖（如启用需单独冻结版本） |
| Casbin | `v2.98.0` | `go.mod` |
| Routing Gates | 仓库内门禁（无外部版本） | `docs/dev-plans/018-routing-strategy.md` + `make check routing` |
| Transactional Outbox | 仓库内实现（无外部版本） | `docs/dev-plans/017-transactional-outbox.md` + `pkg/outbox/**` |

### 3.5 质量门禁与测试

| 组件 | 基线版本 | 来源/说明 |
| --- | --- | --- |
| golangci-lint | `v2.7.2` | CI 安装步骤（Quality Gates） |
| go-cleanarch | `v1.2.1` | `go.mod`（`make check lint` 会运行） |
| E2E：Playwright | `@playwright/test@1.55.1` | `e2e/pnpm-lock.yaml` |
| E2E：pnpm | `10.24.0` | `e2e/package.json#packageManager` |
| E2E：Node.js | `20.x`（推荐） | `README.MD`/`.devcontainer/devcontainer.json`；Playwright 最低 `>=18` |

### 3.6 开发体验（可选但推荐）

| 组件 | 基线版本 | 来源/说明 |
| --- | --- | --- |
| Air | `v1.61.5` | `docs/CONTRIBUTING.MD` 与 `.devcontainer/Dockerfile` |
| DevHub CLI | `v0.0.2` | `.devcontainer/Dockerfile`（`devhub.yml` 为编排 SSOT） |
| Docker Engine/Compose | `27.x`（推荐） | `docs/CONTRIBUTING.MD`（实际以团队统一口径为准） |

## 4. 工具链使用口径（统一）

### 4.1 本地命令入口

- 一切以 `Makefile` 为入口；不要绕过 Makefile 直接拼命令写在个人笔记里。
- 变更触发器矩阵与“改什么必须跑什么”：以 `AGENTS.md` 为准。

### 4.2 生成物与门禁

- `.templ` / UI（MUI 静态产物）/ sqlc 等生成物：**必须提交**，否则 CI 会失败。
- UI/路由/Authz/DB 等“治理型契约”：新增例外属于契约变更，必须先更新对应 dev-plan SSOT 再落代码。

### 4.3 RLS 与 DB Role（运行态契约）

> 说明：Kernel 默认启用 PostgreSQL RLS（fail-closed）；该契约会影响本地开发、CI 与部署口径，因此在工具链决策里显式写出。

- RLS 注入：事务内设置 `app.current_tenant`（`pkg/composables/rls.go`），policy 用 `current_setting('app.current_tenant')::uuid`（fail-closed），对齐 `DEV-PLAN-021`。
- 运行态开关：凡访问 Greenfield 表，`RLS_ENFORCE` 必须为 `enforce`（否则视为配置错误），对齐 `.env.example` 与 `DEV-PLAN-021`。
- DB 账号：应用侧 `DB_USER` 必须为非 superuser，且不可带 `BYPASSRLS`（建议显式 `NOBYPASSRLS`）；superadmin 若需旁路能力，使用独立 role/连接池（见 `DEV-PLAN-019` 的边界）。

## 5. 开发环境指引（本地）

> 目的：新人按此文档能完成“启动 + smoke”，细节以 `docs/CONTRIBUTING.MD`/`devhub.yml`/`Makefile` 为准。

1. 安装并确认版本：Go `1.26.0`、Node `20.x`（E2E/工具）、Docker/Compose（推荐 27.x）。
2. 初始化环境变量：复制 `.env.example` 为 `.env`（必要时使用 `make dev-env` 生成 `.env.local`）。
3. 启动依赖服务：使用 `compose.dev.yml` 启动 Postgres/Redis（端口默认 `5438/6379`，以 `devhub.yml` 为准）。
4. 初始化数据库：执行迁移与 seed（入口见 `Makefile`；常用组合见 `AGENTS.md` TL;DR）。
5. 启动开发服务：
   - 方式 A（推荐）：使用 DevHub（`make devtools`）按 `devhub.yml` 一键编排；
   - 方式 B：启动 `air -c .air.toml`；UI 变更后执行 `make css` 重新构建并同步 `internal/server/assets/web/**`（命令与端口以 SSOT 为准）。
6. RLS（若命中 Greenfield 表）：设置 `RLS_ENFORCE=enforce`，并确保 `DB_USER` 为非 superuser（否则 Postgres 会绕过 RLS）。
7. E2E（可选）：进入 `e2e/`，用 `pnpm` 安装依赖并运行 Playwright（要求本地 DB 与 Go server 已启动）。

## 6. 部署指引（Docker）

> 目的：明确部署形态与边界；具体运维细节以部署环境规范为准。

- 主应用镜像：`Dockerfile`（运行时基底 `alpine:3.21`）；入口会执行迁移/seed 并启动 server（现状）。
- Superadmin：独立镜像 `Dockerfile.superadmin`；部署与路由边界见 `docs/SUPERADMIN.md`。
- 生产 compose 参考：`compose.yml`（当前示例使用 `postgres:17`，并通过环境变量连接）。
- 版本 pin 建议：生产环境应把关键镜像（Postgres/Redis/应用镜像）进一步 pin 到 digest，避免“同 tag 不同内容”的不可复现。

## 7. 现状差异（Drift）与收敛计划（必做）

> 说明：本节用于把“当前仓库存在的版本漂移”显式化，并给出收敛动作；避免团队在实施过程中继续背负漂移成本。

1. [ ] `golangci-lint`：CI 使用 `v2.7.2`，但本地文档/部分 Dockerfile 仍引用 `v1.64.8` —— 统一到 `v2.7.2` 并更新相关资产。
2. [X] Tailwind CLI：已由 `DEV-PLAN-103` 收口（移除旧 UI 链路后不再作为 UI 工具链依赖）。
3. [X] sqlc：已收敛到 `go.mod` tool directives（`v1.30.0`）+ `go tool sqlc` 单一路径（`DEV-PLAN-126`）。
4. [X] goimports：已收敛到 `go.mod` tool directives（`v0.38.0`）+ `go tool goimports` 单一路径（`DEV-PLAN-126`）。
5. [ ] Redis 镜像：当前为 `redis:latest`（浮动）—— 为生产/CI 口径增加 pin 策略（至少固定 major/minor，推荐 digest）。
6. [ ] DevContainer：当前基底为 Go `1.23`（与 `go.mod` 不一致）—— 视团队是否继续使用 DevContainer，决定升级或移除（参考 `DEV-PLAN-002`）。
7. [X] Astro（AHA UI Shell，`DEV-PLAN-018`）：已被 `DEV-PLAN-103` 替代（前端收敛为 MUI X / React SPA），不再作为主 UI 方案。
8. [X] ORY Kratos（`DEV-PLAN-019/009M5`）：已确认为 AuthN 方案；镜像选定 `oryd/kratos:v25.4.0`（后续在 `compose.dev.yml`/CI service 固定到 digest），配置格式以官方 `kratos.yml`（YAML）为准，并要求本地/CI 启动口径可复现。
9. [ ] 100% 覆盖率门禁（`DEV-PLAN-019`）：新仓库需明确“覆盖率统计口径/排除项/生成物处理/CI 入口”，避免实现期临时拼装导致口径漂移。

## 8. 验收标准（本计划完成定义）

- [ ] 本文中的“版本基线”与仓库 SSOT（`go.mod`/`Makefile`/CI/workflows/compose）一致，且不再出现同一工具多版本并存而无说明。
- [ ] 新人按“第 5 节开发指引”可在本地完成：启动依赖服务 → 迁移+seed → 启动 server → 打开健康检查端点。
- [ ] 后续计划引用技术栈/工具链时，只引用本文或更细分的 SSOT 文档，不再在多个 dev-plan 里复制版本清单。
