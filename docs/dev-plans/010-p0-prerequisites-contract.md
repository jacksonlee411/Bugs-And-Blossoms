# DEV-PLAN-010：P0 前置条件实施方案（契约文档优先，避免实施漂移）

**状态**: 已完成（2026-01-06）

> 适用范围：Greenfield 全新实施（从 0 开始）在进入 P0（第一条业务垂直切片）前的“工程/治理/工具链/UI 骨架”准备工作。  
> 说明：本仓库即 implementation repo；契约文档 SSOT 位于 `docs/dev-plans/`，证据记录位于 `docs/dev-records/`。

## 1. 背景与上下文 (Context)

- **需求来源**：`docs/dev-plans/009-implementation-roadmap.md`（Phase 0/1 的前置条件收口）。
- **当前痛点**：Greenfield 从 0 开始最容易出现“先写代码再补规约/门禁”的漂移：版本不可复现、迁移/生成物口径各写一套、路由/授权边界随人演化，最终产出大量不可见的“僵尸功能”。
- **业务价值**：把 P0 之前必须具备的契约（SSOT/门禁/壳/最小登录/迁移闭环）提前固化，使后续 `DEV-PLAN-011`～`DEV-PLAN-031` 的实施只是在“既定轨道”上累积业务能力，而非重复发明基础设施。

### 1.1 术语与定义（本计划口径）

- **implementation repo**：新代码仓库（真正写代码/跑 CI/跑 DB 的地方）。
- **plans repo**：本仓库（只存放 dev-plans 文档）。
- **P0**：在 implementation repo 中交付第一条“用户可见/可操作”的端到端垂直切片（最小链路：确定租户 → 登录 → 进入 UI 壳 → 完成一个业务动作并在页面可见）。
- **P0-Ready**：进入 P0 之前必须满足的前置条件（本计划的“完成定义”）。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标

- [ ] **仓库初始化（bootstrap）可复现**：implementation repo 的基础结构、入口、文档与规则落点清晰，reviewer 能在 5 分钟内找到“规则/门禁/路线图/关键 SSOT”。
- [ ] **SSOT 不漂移**：命令入口统一 `Makefile`；CI 门禁统一 `.github/workflows/*`；规则入口统一 `AGENTS.md`（对齐 `docs/dev-plans/012-ci-quality-gates.md`）。
- [ ] **门禁先行**：CI 至少具备四大 required checks 的“外壳”，且不会出现 `skipped` 结论。
- [ ] **用户可见骨架先行**：UI 壳（含 4 模块入口 + 占位页）尽早落地，后续能力只能挂到这些入口上（对齐用户可见性原则）。
- [ ] **平台最小闭环**：至少具备“租户解析（fail-closed）→ 登录 → session → 进入壳”的最小链路（`docs/dev-plans/019-tenant-and-authn.md`）。
- [ ] **DB/迁移闭环可复制**：至少平台模块（`iam`）具备 Atlas+Goose 模块级闭环与 smoke（`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`），并有 RLS fail-closed 证据（`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`）。
- [ ] **证据可审计**：P0-Ready 的每项关键结论（命令/时间戳/环境）有 readiness 记录入口（实现仓库落地）。

### 2.2 非目标（Out of Scope）

- 不在本计划内实现任何业务模块的“真实业务功能”（由 `DEV-PLAN-026`～`DEV-PLAN-031` 承接）。
- 不在本计划内交付完整 Authz 策略体系（Casbin 的收口可与 P0 并行，但必须在首批策略落地前完成；见 `docs/dev-plans/022-authz-casbin-toolchain.md`）。
- 不在本计划内解决部署/发布/CD（只聚焦 PR/merge 的 CI 门禁与本地可复现）。

## 2.3 工具链与门禁（SSOT 引用）

> 目的：本文不复制脚本细节；只声明本计划在 implementation repo 实施时会命中哪些触发器，并引用 SSOT。

- **本仓库（plans repo）**：
  - [X] 文档新增/调整（本计划交付物为文档）。
- **implementation repo（预计命中）**：
  - [ ] Go 代码（lint/test/coverage）
  - [ ] `.templ` / Tailwind / Astro UI（生成物一致性）
  - [ ] 多语言 JSON（仅 `en/zh`）
  - [ ] 路由治理（allowlist + routing gates）
  - [ ] Authz（Casbin：pack/lint/test）
  - [ ] DB 迁移 / Schema（Atlas+Goose，按模块）
  - [ ] sqlc（生成与一致性门禁）
  - [ ] E2E（Playwright smoke）

- **SSOT 链接（本仓库路径）**：
  - 规则入口：`AGENTS.md`
  - 路线图：`docs/dev-plans/009-implementation-roadmap.md`
  - 版本基线：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
  - 路由契约：`docs/dev-plans/017-routing-strategy.md`
  - CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
  - UI 壳：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
  - Tenancy/AuthN：`docs/dev-plans/019-tenant-and-authn.md`
  - RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
  - Atlas+Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
  - Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`

## 3. 架构与关键决策 (Architecture & Decisions)

### 3.1 总体架构图（从契约到可见交付）

```mermaid
flowchart TD
  Plans[dev-plans: Contract SSOT] --> Make[Makefile: 单一入口]
  Make --> CI[CI: Quality Gates (required checks)]
  CI --> Merge[允许合并]
  Merge --> UI[UI: 可见/可操作入口]
  UI --> E2E[E2E smoke: 可视化证据]
```

### 3.2 关键设计决策（ADR 摘要）

#### ADR-010-01：dev-plan 的 SSOT 放哪里（必须明确，避免双轨漂移）

- **选定：选项 A** —— **dev-plans 与代码同仓**（implementation repo 内 `docs/dev-plans/` 为 SSOT）。
  - 优点：单 PR 评审；变更与实现同步；最小漂移风险。
  - 缺点：需要把本仓库的 `docs/dev-plans/*` 复制进新仓库并维护目录结构。
- 备选（未采用）：选项 B —— **继续以 plans repo 为 SSOT**（代码 PR 只引用外部链接）。
  - 优点：文档与代码分离。
  - 缺点：天然双 PR；易出现“计划已改/实现没改（或反之）”；CI 难以自动阻断漂移。
- 交付要求：implementation repo 的 `AGENTS.md` 必须明确写出本决策，并在 PR 流程里固化“计划先行/同步阻断”的规则。

#### ADR-010-02：CI required checks 必须稳定且不 `skipped`（选定）

- 选定：required checks 以 `docs/dev-plans/012-ci-quality-gates.md` 为准，job 名称冻结；路径未命中时只能在 job 内 no-op，不能 job-level 跳过。

#### ADR-010-03：用户可见性作为“前置条件”的验收约束（选定）

- 选定：UI 壳与模块占位页属于 P0-Ready 的硬性出口；后续任何能力不得以“只有后端无入口”的方式长期存在。

#### ADR-010-04：路由治理与 allowlist entrypoint key（选定）

- 选定：allowlist SSOT 存在且 entrypoint key 冻结为 `server`（tenant app）与 `superadmin`（控制面），并由 routing gates 阻断漂移（见 `docs/dev-plans/017-routing-strategy.md`）。

## 4. 交付物与约束（Repo Skeleton as Contract）

> 本节的路径均指 implementation repo；目的是把“初始化仓库”从口头约定变成可评审的合同。

### 4.1 P0-Ready 最小目录/文件清单（建议形态）

```
.
├── AGENTS.md
├── Makefile
├── .github/workflows/quality-gates.yml
├── config/
│   ├── routing/allowlist.yaml
│   └── access/               # Authz SSOT（可先空壳，但目录预留）
├── docs/
│   ├── docs/dev-plans/            # 若 ADR-010-01 选 A：这里为 SSOT
│   └── dev-records/          # readiness 证据记录
├── modules/
│   ├── iam/                  # 平台模块（Tenancy/AuthN/session）
│   ├── orgunit/
│   ├── jobcatalog/
│   ├── staffing/
│   └── person/
├── apps/web/                 # Astro UI（若按 018）
└── scripts/                  # db/routing/authz/sqlc 等脚本入口（由 Makefile 调用）
```

约束：
- `AGENTS.md` 必须是规则入口（含触发器矩阵、Doc Map、红线）。
- `.github/workflows/quality-gates.yml` 必须只编排 `Makefile` 入口（对齐 012）。
- `config/routing/allowlist.yaml` 必须存在（allowlist 不可用必须 fail-fast；对齐 017）。

### 4.2 Makefile 最小接口（对开发者的“契约 API”）

> 目标：让“本地 = CI”成立，避免每个人都发明一套脚本。

- 必须提供（名称建议与 `docs/dev-plans/012-ci-quality-gates.md` 对齐）：
  - `make preflight`（本地聚合入口）
  - `make check lint` / `make check fmt`
  - `make test`（含覆盖率门禁口径；100% 口径由 SSOT 固化）
  - `make check routing`
  - `make e2e`（smoke）
  - `make check doc`（若 dev-plans 同仓）
  - `make check tr`（i18n：en/zh）
  - 开发环境（本地一键启动，避免环境变量漂移）：
    - `make dev-up` / `make dev-down`
    - `make dev-server`：必须自动加载 `.env.local`（优先）/`env.local`/`.env`，避免 DB 端口回落到默认值（例如 `5438`）导致“连接拒绝”
  - DB（按模块）：`make <module> plan|lint|migrate up`（至少 `iam` 先跑通；对齐 024）
  - 生成物（按需）：`make sqlc-generate`、`make authz-pack/authz-test/authz-lint`

停止线：
- 禁止在 CI YAML 里直接拼接 Atlas/sqlc/authz/templ 命令串绕过 Makefile（对齐 012）。

### 4.3 CI 最小接口（required checks 的外部稳定面）

- required checks 的 job 名称必须冻结（对齐 `docs/dev-plans/012-ci-quality-gates.md`），且合并保护规则以此为准。
- required checks 不得因路径不命中而 `skipped`；允许 job 内 no-op 并返回成功结论。
- 对齐 `DEV-PLAN-012`，对外暴露的四个 required checks（job name）应稳定为：
  - `Code Quality & Formatting`
  - `Unit & Integration Tests`
  - `Routing Gates`
  - `E2E Tests`

### 4.4 UI 最小接口（用户可见性）

- UI 壳必须可运行并包含 4 模块入口（`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md` 的 IA）。
- 未交付模块必须以占位页承载，并明确“未来将交付的能力范围/验收方式”，作为后续唯一挂载点。

## 5. 接口契约 (API Contracts)

> 本计划只冻结“前置条件层”的接口；具体业务 API/UI 由 `DEV-PLAN-026`～`DEV-PLAN-031` 等业务计划定义。

### 5.1 路由与命名空间（高层约束）

- 路由命名空间、返回契约、allowlist 与 routing gates 以 `docs/dev-plans/017-routing-strategy.md` 为 SSOT；P0-Ready 阶段至少应覆盖 allowlist 健康检查与全局 404/405/500 的 responder 契约。

### 5.2 最小登录链路（P0-Ready 阶段可演示）

- tenant 解析必须 fail-closed（见 `docs/dev-plans/019-tenant-and-authn.md`）。
- 登录成功后必须能进入 UI 壳（对齐用户可见性：能看到导航与占位页）。

## 6. 核心流程与算法 (Business Logic & Algorithms)

### 6.1 required checks “不跳过”原则（CI 伪代码）

```
job(required_check):
  changed = paths_filter()
  if not changed:
    print("no relevant changes; no-op")
    exit 0
  run("make <gate>")
```

约束：不得使用 job-level `if:` 让 job 产生 `skipped` 结论（对齐 012）。

## 7. 安全与鉴权 (Security & Authz)

- **fail-closed**：tenant 未解析 / session 缺 tenant_id / RLS 注入缺失 ⇒ 必须拒绝请求（对齐 019/021）。
- **控制面边界**：tenant app 与 superadmin 必须是显式隔离边界（路由前缀、cookie、连接池/role），不得用放宽 RLS policy 代偿（对齐 023/021）。
- **环境暴露基线**：生产默认不暴露 dev/test/playground 端点（对齐 017）。

## 8. 依赖与里程碑 (Dependencies & Milestones)

> 形式：用 PR 序列落地；每个 PR 都必须有清晰的“可演示出口 + 门禁证据”。

### 8.1 串行关键路径（优先保证不阻塞）

`ADR-010-01(SSOT 落点明确) => 011(版本基线) => 012(CI 门禁骨架) => 017(routing SSOT+gates) => 018(UI 壳) => 019(最小登录) => 进入 P0 垂直切片`

### 8.2 推荐 PR 拆分（可并行，但需按关键路径收口）

1. [ ] PR-0：仓库 bootstrap（mono-repo + `apps/web`；README/AGENTS/Doc Map/目录骨架）+ 固化 `docs/dev-plans/` 为 SSOT（ADR-010-01）。
2. [ ] PR-1：对齐 `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`（版本 pin、依赖锁定、基础 Makefile 入口）。
3. [ ] PR-2：对齐 `docs/dev-plans/012-ci-quality-gates.md`（CI required checks 骨架；job 名称冻结；job 不跳过）。
4. [ ] PR-3：对齐 `docs/dev-plans/017-routing-strategy.md`（allowlist SSOT + 最小 routing gates + 本地入口）。
5. [ ] PR-4：对齐 `docs/dev-plans/015-ddd-layering-framework.md`/`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`（`modules/*` 骨架 + 依赖门禁配置）。
6. [ ] PR-5：对齐 `docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`/`docs/dev-plans/020-i18n-en-zh-only.md`（UI 壳 + i18n + 占位页；为 P0 的 `orgunit` 预留入口）。
7. [ ] PR-6：对齐 `docs/dev-plans/019-tenant-and-authn.md`（tenant 解析 + 登录最小闭环，进入壳即可）。
8. [ ] PR-7：对齐 `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`/`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`（`iam` Atlas+Goose 闭环 + RLS fail-closed 最小测试）。
9. [ ] PR-8：对齐 `docs/dev-plans/025-sqlc-guidelines.md`/`docs/dev-plans/022-authz-casbin-toolchain.md`（sqlc 与 Authz 工具链收口；可与 P0 并行，但必须在首批 schema/策略合入前完成）。

## 9. 测试与验收标准 (Acceptance Criteria)

### 9.1 P0-Ready（进入 P0 前必须满足）

- [ ] implementation repo 内 SSOT 落点清晰：规则入口、门禁入口、计划文档入口均可发现且不重复（完成 ADR-010-01）。
- [ ] CI required checks 名称稳定且不出现 `skipped`（未命中触发器时以 no-op 返回成功结论；对齐 012）。
- [ ] UI 壳可打开且 4 模块入口可见；未交付模块以占位页承载（对齐用户可见性原则）。
- [ ] 最小登录链路可演示：确定租户 → 登录 → 进入壳（对齐 019 的 fail-closed 约束）。
- [ ] routing gates 能阻断 allowlist 缺失/entrypoint 缺失/返回契约漂移（对齐 017）。
- [ ] 至少平台模块具备迁移闭环与 RLS fail-closed 证据（对齐 024/021）。

### 9.2 Readiness 记录（证据要求）

- [ ] implementation repo 新增 `docs/dev-records/DEV-PLAN-010-READINESS.md`（或等价落点），记录每条关键结论的：时间戳、环境、命令入口（Makefile 目标）、结果与链接。

### 9.3 5 分钟验收叙事（用于评审/演示）

- [ ] 运行一次 `make preflight`，四大 required checks 在本地均有可复现入口（对齐 012）。
- [ ] 打开 UI 壳：能看到 4 模块入口与占位页（对齐用户可见性原则）。
- [ ] 走一遍最小链路：确定租户 → `/login` → 登录成功 → 进入壳（对齐 019，tenant 解析 fail-closed）。

## 10. 运维与监控 (Ops & Monitoring)

- 早期阶段不做过度运维；但应具备最小健康检查端点与环境暴露基线（见 017/011）。

## 11. 回滚与停止线（最小处置）

### 11.1 回滚（Greenfield 口径）

- 回滚以“回退 PR/回退发布版本”为主；禁止引入并存双实现作为回滚手段（对齐 018 的回退策略）。

### 11.2 停止线（命中即打回）

- [ ] 在 CI YAML 里直接拼接 Atlas/sqlc/authz/templ 命令绕过 Makefile（012）。
- [ ] 新增路由但未更新 allowlist SSOT，或 allowlist 不可用时静默降级（017）。
- [ ] 开始落盘 schema/写入口，但迁移闭环与门禁尚未就位（024）。
- [ ] 访问 tenant-scoped 表但不在事务内注入 RLS，或把 policy 改成可缺省 tenant（021）。
- [ ] 新能力无 UI 入口/无占位页/长期不可见（违反用户可见性原则）。

## 12. 未决问题（需要在 PR-0 明确）

1. [X] ADR-010-01：dev-plan SSOT 放置策略选 A（同仓）—— 已批准。
2. [X] 仓库形态：mono-repo + `apps/web`（对齐 `docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`）—— 已批准。
3. [X] P0 第一条业务垂直切片：`orgunit` —— 已批准。
4. [X] implementation repo 命名/权限/分支保护：`jacksonlee411/Bugs-And-Blossoms`（public），`main` 禁止直推/禁止 force-push/必须 PR，并冻结 required checks：`Code Quality & Formatting` / `Unit & Integration Tests` / `Routing Gates` / `E2E Tests`。

## 13. 参考（本仓库路径）

- `docs/dev-plans/001-technical-design-template.md`
- `docs/dev-plans/009-implementation-roadmap.md`
- `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- `docs/dev-plans/015-ddd-layering-framework.md`
- `docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- `docs/dev-plans/025-sqlc-guidelines.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/012-ci-quality-gates.md`

## 14. Simple > Easy Review（DEV-PLAN-003）

### 结构（解耦/边界）
- 通过：明确区分 `plans repo` 与 `implementation repo`，并把 SSOT/入口（AGENTS/Makefile/CI）定位为可替换边界。
- 通过：把“初始化仓库”提升为可评审合同（目录骨架 + Makefile/CI 最小接口），避免实现期各自发明。
- 警告：Makefile 目标面较宽；需确保“必需入口最小化”，其余按触发器渐进接入，避免早期过度堆叠。

### 演化（规格/确定性）
- 通过：包含 ADR（SSOT 同仓）、PR 序列、P0-Ready 验收、Readiness 证据与停止线，满足“规格驱动而非对话驱动”。
- 需关注：PR-0 必须同步落地分支保护规则（required checks 名称冻结），否则“门禁先行”无法被强制执行。

### 认知（本质/偶然复杂度）
- 通过：复杂度直接服务于不变量（no skipped、fail-closed、routing/authn/rls 边界、用户可见性），未引入历史兼容负担。
- 警告：E2E/生成物门禁在早期容易被“先 no-op”滥用；需在 Readiness 里记录何时从 no-op 升级为真实执行的里程碑（与 `DEV-PLAN-009` 路线图同步）。

### 维护（可理解/可解释）
- 通过：具备可解释叙事（Mermaid + CI 伪代码 + 关键路径），reviewer 可在 5 分钟内复述“为什么这样做”。
- 建议：在 implementation repo 的 `DEV-PLAN-010-READINESS.md` 中固定“演示脚本与证据链接”，让后续新人按证据复现。

结论：通过（带警告与执行约束；阻塞项=无）。
