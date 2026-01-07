# DEV-PLAN-014：多工作区并行开发指引（3 worktree 模式 + 共享本地 infra）

**状态**: 草拟中（2026-01-05 09:50 UTC）

> 适用范围：**全新实施的新代码仓库（Greenfield）**，但本指引可复用于现仓库的多 worktree 工作方式。  
> 目标：在一台开发机上维持 **3 个并行 worktree**（常见：main/feature-a/feature-b 或 main/feature/review），并与 `DEV-PLAN-011` 的本地开发/部署口径对齐（Docker/compose/端口与版本基线）。

## 1. 背景与上下文 (Context)

- Greenfield 采用全新实施（路线图见 `DEV-PLAN-009`），研发阶段常见需要同时打开多个分支：一边开发、一边 review/复现、另一边保持 main 同步。
- 多 worktree 的核心收益是：**不需要频繁切分支**，每个分支拥有独立工作目录与生成物，降低上下文切换与“误改错分支”的风险。
- 本地基础设施（Postgres/Redis）不应为每个 worktree 各起一套：资源浪费且端口/配置管理复杂。默认采用 **共享 infra**（由 `compose.dev.yml` + `Makefile` 的 `dev-*` 入口定义）。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标

- [ ] 给出“3 worktree 并行开发”的推荐拓扑、目录命名与日常工作流。
- [ ] 共享一套本地 Postgres/Redis（固定端口），并说明如何避免端口冲突与破坏性操作。
- [ ] 当确需同时运行多个 server 时，给出最小的端口/环境变量分配口径（避免 cookie/Host/tenant 相关 drift）。
- [ ] 明确与 `DEV-PLAN-011` 的对齐点：本地启动与 Docker 部署的边界、风险与建议做法。

### 2.2 非目标（本计划不做）

- 不覆盖“每个 worktree 独立一套 DB/Redis 并行运行”的完整方案（如确需隔离，请在团队内先冻结口径再补充单独的 dev-plan）。
- 不在本文内定义“私人 docker 命令口径”；共享 infra 的启动/停止以 `Makefile` 的 `dev-*` 为准，避免 drift。

### 2.3 工具链与门禁（SSOT 引用）

> 触发器与命令入口以 SSOT 为准，避免本文复制导致 drift。

- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口：`Makefile`
- 本地服务编排：`compose.dev.yml`
- 示例环境变量：`.env.example`
- 技术栈/部署口径：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`

## 3. 推荐拓扑：3 worktree 角色分工

> 一句话：**main 只做同步与基线验证；两个工作 worktree 用于开发与 review/复现。**

- `wt-main`（基线/同步）：
  - 永远保持干净（无本地改动），只做 `git pull --ff-only`、跑门禁、验证合并后的 main。
  - 推荐由它“管理共享 infra”（启动/停止 Postgres/Redis），避免在多个目录里同时操作 docker。
- `wt-dev-a`（开发 A）：当前主要开发分支（feature/bugfix）。
- `wt-dev-b`（开发 B / review）：用于并行任务、代码 review、复现线上/CI 问题，避免打断 A 的上下文。

## 4. 创建 worktree（一次性）

> 说明：以下为示例命令；实际目录命名以团队约定为准。关键点是：所有 worktree 共享同一 `.git` 存储（省空间，且便于并行）。

1. 以 main 为基线创建两个并行目录：
   - `git worktree add ../repo-wt-dev-a -b feature/<topic-a> origin/main`
   - `git worktree add ../repo-wt-dev-b -b feature/<topic-b> origin/main`
2. 查看当前 worktree 列表：
   - `git worktree list`
3. 完成任务后的清理（建议在合并后执行）：
   - `git worktree remove ../repo-wt-dev-a`
   - `git branch -d feature/<topic-a>`

### 4.1 日常工作流（推荐）

- `wt-main`：
  - 定期同步：`git pull --ff-only`（保持目录干净）。
  - 统一管理共享 infra：启动/停止 Postgres/Redis，避免在多个 worktree 同时操作 docker。
  - 合并前/合并后按触发器矩阵跑门禁（入口见 `AGENTS.md`；推荐 `make preflight` 对齐 CI）。
- `wt-dev-a` / `wt-dev-b`：
  - 用于功能开发/复现/Review；需要更新基线时先 `git fetch origin`，再按团队口径 rebase/merge。
  - 命中生成物/迁移等触发器时，在“产生改动的 worktree”内完成生成与提交，避免跨 worktree 误提交与漂移。

## 5. 共享本地基础设施（Postgres/Redis）

> 默认方案：所有 worktree 共享一套 Postgres/Redis，端口约定为 `5438/6379`（见 `compose.dev.yml` 的默认值）。

### 5.1 为什么能“从任意目录启动同一套 infra”

关键前提：必须固定 compose project name；否则 `docker compose` 会按目录推导 project，导致不同 worktree 误起多套或端口冲突。

本仓库通过 `Makefile` 强制使用统一的 `DEV_COMPOSE_PROJECT`（默认：`bugs-and-blossoms-dev`），从而保证“同一份 `compose.dev.yml` + 同一 project name”指向同一套容器。

### 5.2 启动/停止（推荐由 wt-main 执行）

- 启动（共享 infra）：`make dev-up`
- 停止（非破坏，保留数据卷）：`make dev-down`
- 查看当前状态：`make dev-ps`

**禁止**：在共享 infra 模式下随意执行 `make dev-reset` 或 `docker compose ... down -v`（会清空共享数据卷，相当于重置所有 worktree 的本地数据）。

## 6. 每个 worktree 的本地环境变量（避免冲突）

### 6.1 `.env` / `.env.local` 约定

- 每个 worktree 建议维护自己的 `.env.local`（或 `env.local`，不提交），避免“改错目录导致另一分支配置被污染”。
- 环境变量基线以 `.env.example` 为参考；`make dev-server` 会按优先级加载：`.env.local` → `env.local` → `.env`。

### 6.2 只运行一个 server（默认推荐）

最简单的并行方式是“多 worktree 并行写代码，但同一时刻只跑一个 server”。此时各 worktree 可以共享默认：
- `HTTP_ADDR=:8080`
- `DB_HOST=127.0.0.1`、`DB_PORT=5438`、`DB_NAME=bugs_and_blossoms`、`DB_USER=app`、`DB_PASSWORD=app`
- 若命中启用 RLS 的 Greenfield 表：`RLS_ENFORCE=enforce`

### 6.3 同时运行多个 server（可选）

当确需同时跑 2~3 个 server（例如对比两条分支的行为），需要为每个 worktree 分配不同 `HTTP_ADDR`。另外，HTTP cookie **不区分端口**：如果多个 server 共享同一 hostname，登录态可能互相覆盖/污染（当前 `session`/`lang` cookie 名称为硬编码）。

示例（分别在各自 worktree 执行）：
- `wt-main`：`HTTP_ADDR=:8080 make dev-server`
- `wt-dev-a`：`HTTP_ADDR=:8082 make dev-server`
- `wt-dev-b`：`HTTP_ADDR=:8083 make dev-server`

#### 6.3.1 推荐：同 hostname，换浏览器 profile/容器（零配置）

- 方式 A：Chrome/Edge 使用不同 Profile；或 Firefox Container Tabs。
- 方式 B：一个用普通窗口，另一个用无痕窗口（会话隔离最简单，但关闭即丢）。

> 适用原因：cookie 不区分端口，且当前 cookie 名称（`session`/`lang`）为硬编码（见 `internal/server/handler.go`）。

#### 6.3.2 备选：不同 hostname（cookie 天然隔离，但需同步 tenants 配置）

当前租户解析使用 `Host`（去掉端口）并在 tenants 配置中查找（见 `internal/server/tenants.go`、默认 `config/tenants.yaml`）。因此若你用多个 hostname（例如 `a.localhost`/`b.localhost`），必须同时：

1) 在 `/etc/hosts` 映射到 `127.0.0.1`；  
2) 在各 worktree 的 `.env.local` 指定 `TENANTS_PATH` 指向一个本地 tenants 文件，并在其中加入这些 hostname。

示例（`config/tenants.local.yaml`，仅本机使用，不提交）：

```yaml
version: 1
tenants:
  - id: 00000000-0000-0000-0000-000000000001
    domain: localhost
    name: Local Tenant
  - id: 00000000-0000-0000-0000-000000000001
    domain: a.localhost
    name: Local Tenant
```

### 6.4 与 RLS 的对齐（只在命中 Greenfield 表时）

`DEV-PLAN-021/011` 已明确：访问启用 RLS 的 Greenfield 表时，`RLS_ENFORCE` 必须为 `enforce`，且 `DB_USER` 必须为非 superuser（否则 Postgres 会绕过 RLS）。  
多 worktree 并行时，务必在**每个 worktree 的 `.env/.env.local`**保持一致，避免“一个 worktree enforce、另一个 disabled”导致行为分叉。

## 7. 数据库迁移：协作规则（避免互相踩踏）

共享 DB 的代价是：任一 worktree 的迁移都会影响其他 worktree。为减少互相踩踏，建议遵循：

- 单写者原则（推荐）：约定由 `wt-main`（或指定的一个 worktree）执行迁移；其他 worktree 只在需要时执行 `make <module> migrate up` 同步到最新（示例：`make iam migrate up`）。
- 避免破坏性回滚：不要在共享 DB 上频繁 `migrate down`/重置；需要验证“回滚链路”时，使用独立 DB（另起一套 Postgres / 另一个 compose project / 单独环境）单独验证。
- 变更数据库 schema/迁移时：对齐 `AGENTS.md` 触发器矩阵与 `DEV-PLAN-011` 的“可复现”要求；并在 PR 前按门禁组合验证。

## 8. 与 `DEV-PLAN-011` 的部署口径对齐（本地 → Docker）

### 8.1 本地开发对齐点

- Postgres 主版本对齐：本地 compose 使用 `postgres:17`（与 `DEV-PLAN-011` 一致）。
- 本地编排 SSOT：`compose.dev.yml`；不要在不同 worktree 各自维护“私人 compose”。

### 8.2 Docker 部署对齐点（避免在本机多套 stack 互撞）

`DEV-PLAN-011` 定义了“本地 → Docker”的部署口径；但当前仓库尚未落地 `compose.yml` 等完整 stack 编排文件（以仓库现状为准）。在一台开发机上同时跑多套“应用+数据库”的 compose stack，容易导致：
- 端口冲突（应用端口、Postgres/Redis 端口）；
- 多实例同时执行迁移（若镜像入口包含自动迁移），造成不可预期的竞态。

建议：
- 本地并行开发优先使用“共享 infra + 多 worktree +（必要时）多端口 server”；
- 需要做“接近生产的 Docker 演练”时，单独选择一套端口/DB（或单独一台环境）进行验证，不要与共享 dev infra 混跑。

## 9. 验收标准（本计划完成定义）

- [ ] 在两个不同 worktree 目录中分别执行 `make dev-up` 后，`make dev-ps` 显示同一套容器（同一 `DEV_COMPOSE_PROJECT`）。
- [ ] 3 worktree 模式下，能在不切分支的前提下完成：开发/复现/基线验证。
- [ ] 如需并行运行多 server，按 §6.3 分配 `HTTP_ADDR` 后可同时启动且互不抢占端口，并按 §6.3.1 或 §6.3.2 保证会话 cookie 不互相污染。
- [ ] 指引中涉及的本地/部署口径不与 `DEV-PLAN-011` 冲突。

## 10. 常见问题与排查（Troubleshooting）

- 另一个 worktree 执行 `make dev-up` 报端口占用：通常表示你起了第二套 compose project，或已有其他进程占用了 `5438/6379`；先 `make dev-ps`/`docker ps` 确认当前容器与端口占用，并确保各 worktree 使用同一 `DEV_COMPOSE_PROJECT`。
- 多个 server 并行时登录态“串号”：优先按 §6.3.1 使用不同浏览器 profile/容器；若采用不同 hostname，则按 §6.3.2 同步配置 `TENANTS_PATH` 与 `/etc/hosts`；必要时清理浏览器对应站点的 cookies。
- 多个 server 并行时提示 `tenant not found`：检查当前访问的 hostname 是否在 tenants 配置中；默认只有 `localhost`（见 `config/tenants.yaml`），若用了自定义 hostname 需要按 §6.3.2 配置 `TENANTS_PATH`。
- 误执行 `make dev-reset` / `docker compose ... down -v`：重新 `make dev-up` 后，按模块执行 `make <module> migrate up` 恢复迁移（若你本地依赖了未提交的数据，需自行重建）。
