# DEV-PLAN-009M3：Phase 5 下一大型里程碑执行计划（质量收口：E2E 真实化 + 可排障门禁）

**状态**: 已完成（2026-01-07 12:50 UTC）

> 本文是 `DEV-PLAN-009` 的执行计划补充（里程碑拆解）。假设 `DEV-PLAN-009M2`（Person Identity + Staffing 首个可见样板闭环）已完成，本里程碑聚焦 `DEV-PLAN-009` 的 **Phase 5：质量收口**：把当前“能跑”的状态，推进到“可长期演进且可被 CI 明确阻断漂移/易排障”的状态。  
> 本文不替代 `DEV-PLAN-012`（CI 质量门禁）的合同；任何门禁结构/口径的变更必须先更新对应 dev-plan 再实现。

## 1. 里程碑定义（M3）

- 基于现状的关键结论（输入事实）：
  - `DEV-PLAN-009M1` 与 `DEV-PLAN-009M2` 已完成并在 `DEV-PLAN-010` 中固化了可复现脚本，因此 Phase 4 的“业务垂直切片”已具备可演示闭环；当前最大短板在 Phase 5：`E2E Tests` 仍为 placeholder，无法阻断回归。
  - 当前 UI 为 server-rendered（Go + HTMX），`apps/web` 仅有 `.gitkeep`；但 `make e2e` 的实现会因为 `apps/web` 目录存在而直接 no-op，导致 CI 的 `E2E Tests` required check 失去实际约束力。

- 目标（对齐 `DEV-PLAN-009` Phase 5 出口条件）：
  - E2E Tests 从 placeholder 升级为 **真实的浏览器自动化 smoke**（面向现有 Go UI，而非依赖 `apps/web`），至少覆盖一条“`/login` → `/app` → 业务页面可见/可操作”的链路，并能在失败时产出可用的排障证据（trace/screenshot 等）。
  - 质量门禁的“漂移阻断”具备明确可解释性：生成物漂移/路由漂移/迁移闭环失败时，CI 日志与 artifact 足够定位问题来源（而不是“红了但不知道为什么”）。

- 依赖（SSOT 引用）：
  - 路线图与出口口径：`docs/dev-plans/009-implementation-roadmap.md`
  - CI 门禁结构与验收：`docs/dev-plans/012-ci-quality-gates.md`、`.github/workflows/quality-gates.yml`
  - 本地入口与触发器矩阵：`AGENTS.md`、`Makefile`
  - 工具链版本口径（Node/Playwright/Go 等）：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
  - 证据记录（可复现步骤/结果链接）：`docs/dev-records/DEV-PLAN-010-READINESS.md`
  - 评审与停止线（约束性引用）：`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
  - Routing：`docs/dev-plans/017-routing-strategy.md`
  - Tenancy/AuthN（Host→tenant fail-closed + 登录入口）：`docs/dev-plans/019-tenant-and-authn.md`
  - RLS（No Tx, No RLS + fail-closed + NOBYPASSRLS）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
  - Atlas+Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
  - Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
  - E2E 业务断言口径（`effective_date`）：`docs/dev-plans/031-greenfield-assignment-job-data.md`、`docs/dev-plans/032-effective-date-day-granularity.md`
  - Phase 4 复用输入（smoke 场景来源）：`docs/dev-plans/009m2-phase4-person-identity-staffing-vertical-slice-execution-plan.md`

## 2. 非目标（本执行计划不做）

- 不新增业务功能/业务字段；不新增或调整任何业务数据契约（这些应在 `DEV-PLAN-026/029/030/031/027` 里按合同推进）。
- 不引入“第二套 E2E 框架”或“第二套 UI 构建流水线”；E2E 与门禁入口必须收敛到 `Makefile`/CI workflow（SSOT）。
- 不引入 legacy/双链路/回退通道；不为“让 E2E 通过”绕过 One Door / No Tx, No RLS stopline。

## 3. 不变量与停止线（对齐 `DEV-PLAN-003` + `AGENTS.md`）

> 评审定位：Stage 2（Plan）为主。目标是把实现期最容易“为了过测而引入偶然复杂度”的决策前置冻结，避免回到 Easy（堆叠分支/后门/第二入口）。

### 3.1 运行态契约（必须写死）

- **Host/tenant**：E2E 与手工验证必须使用 `http://localhost:8080`（而非 `127.0.0.1`）；对齐 `DEV-PLAN-019`：tenant 解析 SSOT 为 `tenant_domains.hostname`，且 `/login` 也必须先解析 tenant（未解析即 404，fail-closed）。
- **强约束模式**：E2E 必须在 `AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce` 下运行；不得通过降级模式让测试变绿（对齐 `DEV-PLAN-022/021`）。
- **危险解锁开关**：CI/E2E 不得设置 `AUTHZ_UNSAFE_ALLOW_DISABLED=1`（该开关仅用于本地短期排障，且必须显式解锁；对齐 `DEV-PLAN-022` 的 fail-fast 约束）。
- **DB 用户**：E2E/CI 的 `DB_USER` 必须为非 superuser 且不得拥有 `BYPASSRLS`（避免绕过 RLS；对齐 `DEV-PLAN-021` 口径）。
- **确定性 DB**：E2E 必须对 DB 状态有确定性假设（推荐：CI 每次使用全新 Postgres service + migrate up；本地如需 reset 必须走明确的破坏性入口，不允许隐式清库）。

### 3.2 数据准备单一权威（禁止测试后门）

- E2E 的业务数据准备只允许走**既有用户可见入口**（现有 UI 表单链路）或既有 `cmd/dbtool` smoke/迁移闭环；不得新增“测试专用写入 API/隐藏路由/直写表脚本”。
- 写入必须继续遵守 One Door（`submit_*_event(...)`）；不得为测试直接写 read model 表。

### 3.3 停止线（命中即拒绝）

- 引入第二套 E2E 框架/第二套入口（违反 SSOT）。
- 让 required check “成功但不跑测试”：在 `E2E Tests` job **命中触发器**时，`make e2e` 仍 no-op/placeholder/0 tests 却退出 0；仅允许在 paths-filter 未命中时 step-level no-op（对齐 `DEV-PLAN-012` 的 required checks 口径）。
- 通过 `AUTHZ_MODE=disabled`/`RLS_ENFORCE=disabled` 绕过约束，或在 CI 中设置 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 让 disabled 可用。
- 为测试新增 legacy 分支/回退通道/双链路。

## 4. Done 口径（验收/关闭条件）

### 4.1 E2E（真实且可复现）
- [ ] CI 的 `E2E Tests` job 运行 **真实 E2E**（非 no-op/placeholder），并且：
  - [ ] 至少包含 1 条可稳定复现的 smoke 场景（首选复用 `DEV-PLAN-009M2`）：`/login` → `/app` → Person→Position→Assignment 的“写入→列表读取”闭环，并断言 Assignments UI 仅展示 `effective_date`（对齐 `DEV-PLAN-031/032`）。
  - [ ] 失败时默认产出可用 artifact（trace/screenshot/video/日志，具体落点以 `DEV-PLAN-012` 的 SSOT 为准）。
- [ ] 本地可通过单一入口复现同一 E2E（入口以 `Makefile` 为准）。
- [ ] `make e2e` 不再因 `apps/web` 存在而 no-op；placeholder 逻辑被移除或被替换为真实执行（以 `Makefile` 为 SSOT）。
- [ ] 当 E2E 依赖缺失/未发现测试用例时：应 fail-fast 并输出明确指引（而不是静默成功）。

### 4.2 门禁与漂移阻断（可解释）
- [ ] “生成物漂移”能被 CI 明确阻断并给出可定位的差异证据（对齐 `DEV-PLAN-012` 的 artifact 约束；SSOT：`.github/workflows/quality-gates.yml` + `scripts/ci/assert-clean.sh`）。
- [ ] DB/迁移闭环失败可被定位：Atlas plan/lint 与 goose/smoke 的失败输出在 CI 中可审计（SSOT：`Makefile` + `scripts/db/**`）。
- [ ] Routing 漂移可被定位：`make check routing` 的失败信息能指向具体 allowlist/分类漂移点（SSOT：`scripts/routing/**`）。

### 4.3 证据固化（可审计）
- [ ] 将本里程碑 E2E 与门禁收口的“可复现步骤 + 结果链接”写入 `docs/dev-records/DEV-PLAN-010-READINESS.md`（或在 `DEV-PLAN-012` 增设对应 readiness 记录文件，并在本文引用其 SSOT 路径）。
- [ ] E2E 失败时最小证据集（至少）：
  - [ ] Playwright 报告/trace（或等价）、至少 1 张 screenshot
  - [ ] server 启动日志（含监听地址、关键 env）
  - [ ] 迁移执行日志（模块 migrate up / smoke 输出）

## 5. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 GitHub Actions required checks 全绿且不 `skipped` 后合并；`main` 禁止直推与 force-push。

### PR-0：文档回填（对齐“009m2 已完成”的输入事实）
- [X] 在 `docs/dev-records/DEV-PLAN-010-READINESS.md` 补齐 `DEV-PLAN-009M2` 的最小可复现证据入口（不要求复制全部细节，但必须能复跑）。
- [X] 在 `docs/dev-plans/009-implementation-roadmap.md` Phase 4 出口条件中补齐对应勾选与证据链接（使路线图状态与实现事实一致）。
- [X] 将 `docs/dev-plans/009m2-phase4-person-identity-staffing-vertical-slice-execution-plan.md` 的状态更新为 `已完成` 并补时间戳（以 readiness 证据为前置）。

### PR-1：E2E 框架落地（Playwright/等价）与最小 smoke
- [X] 调整 `make e2e`：移除 placeholder/no-op，入口收敛为 `scripts/e2e/run.sh`；无用例时 fail-fast（拒绝 “0 tests 退出 0”）。
- [X] 落地 E2E 测试工程（`e2e/`），并提供最小 smoke（真实浏览器链路）：
  - [X] `http://localhost:8080/login` 登录并跳转 `/app`。
  - [X] `/app` 首页可见（避免只验证 200）。
- [X] 测试数据准备只走既有用户可见入口（UI 表单 + 迁移/smoke），不新增测试后门写入通道。

### PR-2：CI 编排与排障证据（让失败可定位）
- [X] 更新 `.github/workflows/quality-gates.yml` 的 `E2E Tests` job：命中触发器时运行 `make e2e`，并补齐 Go/Node 工具链准备。
- [X] CI 运行态契约显式化并由脚本 fail-fast：
  - [X] `E2E_BASE_URL` 必须使用 `http(s)://localhost...`（禁止 `127.0.0.1`）。
  - [X] 默认 `AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`；禁止 `AUTHZ_UNSAFE_ALLOW_DISABLED=1`。
  - [X] DB runtime 角色强约束：非 superuser 且 `NOBYPASSRLS`（通过 dev postgres init + E2E 脚本断言）。
- [X] 默认上传 E2E 失败 artifact：CI failure 时上传 `e2e/test-results/**`、`e2e/playwright-report/**`；失败输出包含本地最短复现入口（`make e2e`）与 server log 路径。

### PR-3：选择一个业务闭环纳入 E2E（避免只测登录）
- [X] 固化 1 条“业务闭环” E2E 场景（复用 `DEV-PLAN-009M2`）：
  - [X] 创建 OrgUnit（复用已有 root，避免 `ORG_ROOT_ALREADY_EXISTS`）。
  - [X] 创建 Person → 创建 Position → 创建 Assignment，并断言 UI 仅展示 `effective_date`（不出现 `end_date`）。
- [X] 场景走真实 UI 交互（浏览器操作），失败时具备 trace/screenshot/video + server log 证据。

### PR-4：收口与门禁联动（把“能跑”变成“可长期演进”）
- [X] `make preflight` 已包含 `make e2e`；本次将其从 placeholder 收口为真实执行。
- [X] 对齐 `DEV-PLAN-012`：补齐 Gate 4（E2E）的“实现现状/证据落点/停止线”登记（见对应 dev-plan 变更）。

## 6. 本地验证（SSOT 引用）

- 一键启动：`make dev`（入口与环境加载规则以 `Makefile`/`DEV-PLAN-010` 为准）。
- 质量门禁：优先 `make preflight`（入口与 required checks 口径以 `AGENTS.md`/`DEV-PLAN-012` 为准）。
- E2E：以 `make e2e` 为唯一入口（实现落地后由 `Makefile` 与 E2E 工程文档作为 SSOT）。

## 7. 实施登记（Execution Log）

> 说明：本节只登记“已完成/落地”的关键项与 SSOT 落点；详细实现差异以对应 PR/commit 为准。

- 2026-01-07：完成 `make e2e` 真实化（Playwright smoke + fail-fast + artifact）
  - 合并记录：PR #49 https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/49
  - 入口收口：`Makefile` → `scripts/e2e/run.sh`
  - E2E 工程：`e2e/`（`playwright.config.mjs` + `tests/m3-smoke.spec.js`）
  - CI artifact（failure）：`.github/workflows/quality-gates.yml` 上传 `e2e/test-results/**`、`e2e/playwright-report/**`
  - DB runtime 角色约束：`compose.dev.yml` + `scripts/dev/postgres-init/*`
