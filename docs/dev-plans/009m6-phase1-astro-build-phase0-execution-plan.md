# DEV-PLAN-009M6：Phase 1 追加里程碑执行计划（补齐 DEV-PLAN-018 Phase 0：Astro build + go:embed Shell）

**状态**: 执行中（2026-01-08 10:24 UTC）

> 本文是 `DEV-PLAN-009` 的执行计划补充（里程碑拆解）。由于当前仓库的 UI Shell 仍主要由 Go handler 拼接输出，而 `DEV-PLAN-018` 已冻结“Shell=Astro build 产物 + go:embed + HTMX 装配动态上下文”的合同，本里程碑用于**补齐/收敛**到 `DEV-PLAN-018` 的 Phase 0（最小可运行）要求，并同步把 `Makefile`/CI 门禁从 placeholder 收口为可阻断漂移的真实入口。
>
> 本文不替代 `DEV-PLAN-018/011/012/017/020` 的合同；任何契约变更必须先更新对应 dev-plan 再写代码。

## 1. 里程碑定义（M6）

### 1.1 输入事实（基于当前仓库实现）

- `DEV-PLAN-018` Phase 0 明确要求：在 `apps/web` 初始化 Astro 工程、落盘 `package.json` + lockfile、构建产物复制到 `internal/server/assets/astro/**` 并由 Go `go:embed`（Shell HTML 固定路径、Go `/app` handler 只做最小占位符注入；占位符契约见 `DEV-PLAN-018` §4.5）。
- 当前仓库：
  - `apps/web` 为空（仅 `.gitkeep`），不存在 Astro 工程 SSOT（无 `apps/web/package.json` / `apps/web/pnpm-lock.yaml`）。
  - `internal/server/assets/astro/**` 不存在；`/app` Shell 仍由 Go 拼接输出。
  - `Makefile` 的 `make css` / `make generate` 为 no-op placeholder，CI Gate-1 未执行 UI build/生成物一致性。
- CI 门禁存在 Node 20 依赖（主要用于 E2E），但 **UI build** 尚未被 required checks 强制执行与验收。

### 1.2 目标（对齐 `DEV-PLAN-018` Phase 0）

- **Astro UI 工程落盘（SSOT）**：
  - 在 `apps/web` 初始化 Astro 工程，提交 `apps/web/package.json` 与 `apps/web/pnpm-lock.yaml`；
  - 版本口径按 `DEV-PLAN-011`：`packageManager` pin 为 `pnpm@10.24.0`，Node 基线对齐 CI（Node 20）。
- **Shell 产物 go:embed（冻结路径）**：
  - `internal/server/assets/astro/app.html` 为唯一可嵌入 Shell HTML；
  - Astro build 的静态资源一并落到 `internal/server/assets/astro/**`（对齐 `DEV-PLAN-018` §4.5：静态资源 URL 前缀固定为 `/assets/astro/`，且路径必须可由 Go `/assets/*` 提供）。
- **服务端装配（不复制权威判断）**：
  - `/ui/nav`、`/ui/topbar`、`/ui/flash` 仍由服务端渲染（保持 i18n/authz 的权威表达在服务端，避免 Astro 内复制第二套判断逻辑）。
  - `/app`（以及所有“全页访问”模式）使用 Astro Shell；Go 只做最小占位符注入，不在 handler 内重写页面结构（对齐 `DEV-PLAN-018`）。
- **门禁收口（Makefile + CI）**：
  - `make css` 不再是 placeholder：命中 UI 变更时，必须能在本地与 CI 生成 `internal/server/assets/astro/**` 并保证 `git status --porcelain` 为空。
  - CI Gate-1（Code Quality & Formatting）命中 UI 变更时必须执行 `make css`（以及必要的 Node/pnpm 安装步骤），并用 `assert-clean` 阻断生成物漂移。

### 1.3 依赖（SSOT 引用）

- UI Shell 合同：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- 工具链版本基线：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- CI 门禁结构：`docs/dev-plans/012-ci-quality-gates.md`、`.github/workflows/quality-gates.yml`
- Routing 策略与门禁：`docs/dev-plans/017-routing-strategy.md`、`config/routing/allowlist.yaml`
- i18n（仅 en/zh）：`docs/dev-plans/020-i18n-en-zh-only.md`
- 证据记录：`docs/dev-records/DEV-PLAN-010-READINESS.md`
- 停止线：`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`

### 1.4 已决事项（已确认采用建议项）

> 这些是会影响实现细节与门禁闭合的关键决策；当前已确认采用“建议项”（并已写入 `DEV-PLAN-018/012`）。

1. [X] 占位符注入 token 方案（Shell HTML）
   - 建议：单 token `__BB_AS_OF__`（只注入 `as_of`；对齐 `DEV-PLAN-018` §4.5.2）。
   - 备选：为每个动态属性单独 token（例如 `__BB_NAV_HX_GET__`），但注入点数量会膨胀。
2. [X] Shell 产物映射（Astro dist → go:embed）
   - 建议：`apps/web/dist/index.html` 复制为 `internal/server/assets/astro/app.html`；其余 `dist/**` 原样复制到 `internal/server/assets/astro/**`（对齐 `DEV-PLAN-018` §4.5.1）。
3. [X] CI `ui` 触发器口径（paths-filter）
   - 建议：`ui` 至少覆盖 `apps/web/` + `internal/server/assets/astro/`，确保“改源/改产物”都会触发 `make css` 并被 `assert-clean` 阻断漂移。
4. [X] Node 版本 pin 形式（本地可复现 vs 最小必要）
   - 建议：先以 CI Node 20 + `packageManager: pnpm@10.24.0` 为最小闭环；如需要更强一致性再补 `.nvmrc`/`.tool-versions`。
5. [X] `as_of` 缺省行为（URL 可分享/可复现）
   - 建议：按 `DEV-PLAN-018` §3.3 执行 302 补齐 `?as_of=<CURRENT_DATE(UTC)>`（URL 永远显式包含 `as_of`）。
   - 备选：缺省时直接在服务端回退到 `CURRENT_DATE(UTC)`（不 302），实现更省但 URL 不可复现且易产生“今天/明天不同结果”的隐性漂移。

## 2. 非目标（本执行计划不做）

- 不引入 SPA；仍以 “Astro Shell + HTMX partial + Alpine” 为主（对齐 `DEV-PLAN-018`）。
- 不在本里程碑内重写模块页面 UI（OrgUnit/JobCatalog/Staffing/Person 仍以现有服务端 HTML 输出为主）。
- 不在本里程碑内引入 Tailwind 的完整接入与设计系统扩展（仅为 Shell 提供最小可用样式与资源管线；如需扩展另立 dev-plan）。
- 不新增/升级 i18n 方案（仍只允许 `en/zh`）。

## 3. 不变量与停止线（对齐 `AGENTS.md` + `DEV-PLAN-018`）

### 3.1 单一 Shell 事实源（避免双实现）

- Shell HTML 的权威表达必须唯一：以 `internal/server/assets/astro/app.html` 为准；不得继续维护“Go 拼接版 Shell”作为 fallback/回退通道。
- Astro 工程的依赖版本事实源必须唯一：以 `apps/web/package.json` + `apps/web/pnpm-lock.yaml` 为准（对齐 `DEV-PLAN-011`）。

### 3.2 go:embed 交付方式（冻结）

- Astro build 产物必须复制到 `internal/server/assets/astro/**`，并通过 Go `/assets/*` 提供静态分发；不得引入运行时额外挂载目录或旁路静态服务器。

### 3.3 停止线（命中即拒绝）

- 引入任何 runtime legacy/回退通道（例如 `USE_LEGACY_SHELL=1`、双壳并存、或按条件回退到 Go 拼接版）。
- 在 Astro 内复制权限/语言判断逻辑（造成第二套权威表达）。
- UI 生成物未提交导致 `git status` 非空仍可通过 CI（必须被 `assert-clean` 阻断）。
- 命中 UI 变更但 CI 未执行 `make css`（例如触发器口径不闭合导致 UI build 被漏跑）。

## 4. Done 口径（验收/关闭条件）

### 4.1 UI 工程与产物

- [X] `apps/web/package.json` 与 `apps/web/pnpm-lock.yaml` 已提交，且 `packageManager` pin 为 `pnpm@10.24.0`。
- [X] `internal/server/assets/astro/app.html` 存在，且包含 `#nav`、`#topbar`、`#flash`、`#content` 四个挂载点，并在 `hx-get` 中使用 `as_of=__BB_AS_OF__`（对齐 `DEV-PLAN-018` §4.4/§4.5）。
- [X] `make css` 可在本地与 CI 生成/更新 `internal/server/assets/astro/**`，并保证 `git status --porcelain` 为空。

### 4.2 Server 行为

- [ ] `/app` 返回的全页 HTML 由 Astro Shell 提供（不再由 Go 拼接生成），并且在登录态下会触发 HTMX 拉取 `/ui/nav?as_of=...`、`/ui/topbar?as_of=...`、`/ui/flash`（对齐 `DEV-PLAN-018` §4.4）。
- [ ] `as_of` 口径对齐 `DEV-PLAN-018` §3.3：`/app`（以及最小占位内容页）缺省时 302 补齐 `?as_of=<CURRENT_DATE(UTC)>`；非法时 400 且可被 UI 展示（避免“猜测性纠正”）。
- [ ] 任意模块页面在非 HTMX 访问模式下仍可返回全页（使用 Astro Shell 作为外壳），并保持与 HTMX partial 的协商口径一致（对齐 `DEV-PLAN-017/018`）。

### 4.3 门禁与证据

- [ ] 本地：`make preflight` 全绿。
- [ ] CI：四大 required checks 全绿且不出现 `skipped`；UI 变更（`apps/web/**` 或 `internal/server/assets/astro/**`）时 Gate-1 必须执行 `make css` 并通过 `assert-clean`。
- [ ] 证据固化：更新 `DEV-PLAN-010`（新增 009M6 小节：命令/时间戳/结论/PR 链接），并在 `DEV-PLAN-018` 将 Phase 0 条目勾选为完成并更新状态。

## 5. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 required checks 全绿且不 `skipped` 后合并；`main` 禁止直推与 force-push。

### PR-0：合同回填与实施口径冻结（文档优先）

- [X] 在 `DEV-PLAN-018` 明确 Shell 的“产物映射 + 占位符注入”契约（`apps/web/dist/*` → `internal/server/assets/astro/**`、token `__BB_AS_OF__`、注入点/失败行为），避免实现期临时发明。
- [X] 在 `DEV-PLAN-011` 落定 Astro 基线版本（以 `apps/web/pnpm-lock.yaml` 进入主干为准），并明确 Node/pnpm 的 SSOT 与对齐方式。
- [X] 在 `DEV-PLAN-012` 补齐 UI Gate 的“必须执行项”与触发器说明（命中 `ui` 时 Gate-1 必须跑 `make css`；触发器需覆盖 `apps/web/**` 与 go:embed 产物目录）。

### PR-1：初始化 `apps/web` Astro 工程（SSOT 落盘）

- [X] 初始化 Astro 项目（最小集），提交：
  - `apps/web/package.json`（含 `packageManager: pnpm@10.24.0`）
  - `apps/web/pnpm-lock.yaml`
  - `apps/web/astro.config.*`（必要时）
  - 最小页面/模板：生成 `dist/index.html`（或等价入口），包含四个挂载点，并在 `hx-get` 中使用 `as_of=__BB_AS_OF__`（token 由 Go 注入）。
- [X] 确保 `apps/web/node_modules` 不进入仓库（gitignore）。

### PR-2：构建与复制管线（产物进入 `internal/server/assets/astro/**`）

- [X] 新增脚本（建议）：`scripts/ui/build-astro.sh`
  - 通过 corepack pin pnpm 版本（对齐 E2E 口径）；
  - `pnpm -C apps/web install --frozen-lockfile`
  - `pnpm -C apps/web build`
  - 按 `DEV-PLAN-018` §4.5.1 将 build 产物复制到 `internal/server/assets/astro/**`（`dist/index.html` → `app.html`；其余文件保持相对路径）。
- [X] Makefile 收口：`make css` 调用上述脚本，且输出稳定、可复现、可在 CI 运行；生成后 `git status --porcelain` 必须为空。

### PR-3：CI Gate-1 补齐 UI build（阻断生成物漂移）

- [X] 更新 `.github/workflows/quality-gates.yml`：
  - Code Quality & Formatting job 命中 UI 变更时安装 Node（setup-node）并执行 `make css`；
  - 仍保留 `assert-clean` 作为生成物一致性门禁。
- [X] 调整 paths-filter 的 UI 触发器口径：`ui` 至少覆盖 `apps/web/**` 与 `internal/server/assets/astro/**`（防止手改产物绕过 build）；不扩大到无关 generated 目录导致误触发。

### PR-4：Go Server 切换为 Astro Shell（移除 Go 拼接壳）

- [X] `/app` 读取并输出 `assets/astro/app.html`，并进行最小占位符注入（替换 `__BB_AS_OF__`；对齐 `DEV-PLAN-018` §4.5.2；不在 handler 内重写结构）。
- [X] `as_of` 缺省/校验行为对齐 `DEV-PLAN-018` §3.3（302 补齐；非法 400），并保证注入后的 HTMX 请求不会丢失 `as_of`。
- [X] `writeShell*`/全页模式收口：将“非 HTMX”页面的外壳渲染统一切换到 Astro Shell，确保“同一 URL 支持全页与 partial”。
- [X] 静态资源分发确保覆盖 Astro build 产物路径（仍在 `/assets/*` 命名空间下）。

### PR-5：测试与验收脚本收口（最小可复现）

- [X] 单测：覆盖 `/app` 全页响应中包含四个挂载点，并且在登录态会触发 `/ui/nav` 等 HTMX 拉取（只测“契约存在”，不测 Astro 内部实现）。
- [X] E2E（如需）：增加一个轻量断言（打开 `/app` 后壳加载成功，且能通过导航进入一个模块页）。

### PR-6：Readiness 证据登记与文档收口

- [ ] 更新 `docs/dev-records/DEV-PLAN-010-READINESS.md`：新增 009M6 证据（时间戳/命令/结果/PR）。
- [ ] 更新 `docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`：Phase 0 勾选完成并将状态更新为“已完成（UTC 时间戳）”。
- [ ] 更新 `docs/dev-plans/009-implementation-roadmap.md`：在 UI Shell 相关条目处补充“已按 Astro build Phase 0 收口”的证据链接（不改合同，只补证据）。

## 6. 本地验证（SSOT 引用）

- UI build（本里程碑新增）：`make css`（生成物必须提交；`git status --short` 为空）
- 一键对齐 CI：`make preflight`
- 路由门禁：`make check routing`
- E2E：`make e2e`

## 7. Simple > Easy Review（DEV-PLAN-003，自评要点）

- 结构：Astro 只负责 Shell/构建；导航/权限/语言仍以服务端渲染为权威，避免出现第二套判断逻辑。
- 演化：先让 build/产物/门禁成体系（可复现、可阻断），再谈 Tailwind/组件化扩展，避免“先堆样式再补管线”。
- 回滚：回滚只允许通过 PR 回滚/版本回滚；不引入 runtime fallback/双壳并存。
