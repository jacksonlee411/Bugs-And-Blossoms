# DEV-PLAN-009M3：Phase 5 下一大型里程碑执行计划（质量收口：E2E 真实化 + 可排障门禁）

**状态**: 草拟中（2026-01-07 01:36 UTC）

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
  - Routing：`docs/dev-plans/017-routing-strategy.md`
  - Tenancy/AuthN（Host→tenant fail-closed + 登录入口）：`docs/dev-plans/019-tenant-and-authn.md`
  - Atlas+Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
  - Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`

## 2. 非目标（本执行计划不做）

- 不新增业务功能/业务字段；不新增或调整任何业务数据契约（这些应在 `DEV-PLAN-026/029/030/031/027` 里按合同推进）。
- 不引入“第二套 E2E 框架”或“第二套 UI 构建流水线”；E2E 与门禁入口必须收敛到 `Makefile`/CI workflow（SSOT）。
- 不引入 legacy/双链路/回退通道；不为“让 E2E 通过”绕过 One Door / No Tx, No RLS stopline。

## 3. Done 口径（验收/关闭条件）

### 3.1 E2E（真实且可复现）
- [ ] CI 的 `E2E Tests` job 运行 **真实 E2E**（非 no-op/placeholder），并且：
  - [ ] 至少包含 1 条可稳定复现的 smoke 场景：`/login` → 登录成功 → `/app` 壳加载成功 → 至少一个业务页面可打开且可执行一次最小操作（建议复用 `DEV-PLAN-009M1` 或 `DEV-PLAN-009M2` 的“写入→列表读取”闭环之一，避免新增“专用测试功能”）。
  - [ ] 失败时默认产出可用 artifact（trace/screenshot/video/日志，具体落点以 `DEV-PLAN-012` 的 SSOT 为准）。
- [ ] 本地可通过单一入口复现同一 E2E（入口以 `Makefile` 为准；不要在本文复制脚本细节）。
- [ ] `make e2e` 不再因 `apps/web` 存在而 no-op；placeholder 逻辑被移除或被替换为真实执行（以 `Makefile` 为 SSOT）。

### 3.2 门禁与漂移阻断（可解释）
- [ ] “生成物漂移”能被 CI 明确阻断并给出可定位的差异证据（对齐 `DEV-PLAN-012` 的 artifact 约束；SSOT：`.github/workflows/quality-gates.yml` + `scripts/ci/assert-clean.sh`）。
- [ ] DB/迁移闭环失败可被定位：Atlas plan/lint 与 goose/smoke 的失败输出在 CI 中可审计（SSOT：`Makefile` + `scripts/db/**`）。
- [ ] Routing 漂移可被定位：`make check routing` 的失败信息能指向具体 allowlist/分类漂移点（SSOT：`scripts/routing/**`）。

### 3.3 证据固化
- [ ] 将本里程碑 E2E 与门禁收口的“可复现步骤 + 结果链接”写入 `docs/dev-records/DEV-PLAN-010-READINESS.md`（或在 `DEV-PLAN-012` 增设对应 readiness 记录文件，并在本文引用其 SSOT 路径）。

## 4. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 GitHub Actions required checks 全绿且不 `skipped` 后合并；`main` 禁止直推与 force-push。

### PR-0：文档回填（对齐“009m2 已完成”的输入事实）
- [X] 在 `docs/dev-records/DEV-PLAN-010-READINESS.md` 补齐 `DEV-PLAN-009M2` 的最小可复现证据入口（不要求复制全部细节，但必须能复跑）。
- [X] 在 `docs/dev-plans/009-implementation-roadmap.md` Phase 4 出口条件中补齐对应勾选与证据链接（使路线图状态与实现事实一致）。
- [X] 将 `docs/dev-plans/009m2-phase4-person-identity-staffing-vertical-slice-execution-plan.md` 的状态更新为 `已完成` 并补时间戳（以 readiness 证据为前置）。

### PR-1：E2E 框架落地（Playwright/等价）与最小 smoke
- [ ] 调整 `make e2e`：移除/替换 placeholder（当前会因 `apps/web` 目录存在而 no-op），并将 E2E 唯一入口收敛为真实执行。
- [ ] 落地 E2E 测试工程（目录/依赖/配置），并提供一个最小 smoke（建议优先复用 `DEV-PLAN-009M2` 的最短闭环）：
  - [ ] 打开 `http://localhost:8080/login`（或 CI 内部等价地址）并完成登录。
  - [ ] 验证跳转到 `/app` 并能看到 `/app/home` 的模块入口（避免只验证 200）。
- [ ] 明确“测试数据准备”的唯一入口（建议通过既有 UI 表单或既有迁移/seed 工具；禁止新增隐藏后门）。

### PR-2：CI 编排与排障证据（让失败可定位）
- [ ] 更新 `.github/workflows/quality-gates.yml` 的 `E2E Tests` job：为 E2E 提供最小运行环境（启动 server、准备 DB、准备必要迁移/数据；方式以 SSOT 为准）。
- [ ] 默认上传 E2E 失败 artifact（trace/screenshot/video/日志等），并在失败输出中包含“如何在本地复现”的最短指引（引用 `Makefile` 入口）。

### PR-3：选择一个业务闭环纳入 E2E（避免只测登录）
- [ ] 选择并固化一个“业务闭环”作为 E2E 场景（按现状建议优先顺序如下）：
  - [ ] 首选：复用 `DEV-PLAN-009M2`（Person Identity + Staffing）：创建 Person → 创建 Position → 创建/更新 Assignment，并断言 UI 仅展示 `effective_date`（可直接把 `DEV-PLAN-010` 中的 M2 证据脚本翻译为浏览器操作）。
  - [ ] 复用 `DEV-PLAN-009M1`：SetID + JobCatalog（解析→写入→列表读取）。
- [ ] 场景必须走真实 UI 交互（浏览器操作），并在失败时能定位到“哪一步失败”（而非只有最终断言）。

### PR-4：收口与门禁联动（把“能跑”变成“可长期演进”）
- [ ] 将 E2E 入口纳入 `make preflight`（如果当前 `preflight` 未覆盖或覆盖但为 placeholder，则收口成真实执行）。
- [ ] 对齐 `DEV-PLAN-012`：更新其状态/待办项，使“门禁结构”与“实现现状”一致（例如 E2E 已真实化、artifact 已落地等）。

## 5. 本地验证（SSOT 引用）

- 一键启动：`make dev`（入口与环境加载规则以 `Makefile`/`DEV-PLAN-010` 为准）。
- 质量门禁：优先 `make preflight`（入口与 required checks 口径以 `AGENTS.md`/`DEV-PLAN-012` 为准）。
- E2E：以 `make e2e` 为唯一入口（具体实现与依赖在落地后由 `Makefile` 与 E2E 工程 README 作为 SSOT）。
