# 请总是用中文回复。

# AGENTS.md（主干 SSOT）

本文件是仓库内“如何开发/如何验证/如何组织文档与规则”的**主干入口**。优先阅读本文件，并通过链接跳转到其他专题文档；避免在多个文档里复制同一套规则，减少漂移。

本仓库为 Greenfield 的 **implementation repo**（仓库名：`Bugs-And-Blossoms`）。执行顺序与并行策略以 `docs/dev-plans/009-implementation-roadmap.md` 为准；`docs/dev-plans/` 为契约文档 SSOT（同时通过 `docs/dev-plans` 入口可达），P0-Ready 的证据记录在 `docs/dev-records/`。

## 0. TL;DR（最常见变更要跑什么）

- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 禁止 legacy（单链路原则）：`make check no-legacy`（或直接跑 `make preflight`）
- `.templ`/Tailwind/Astro UI 相关：`make generate && make css`，然后 `git status --short` 必须为空
- 多语言 JSON：`make check tr`
- 发 PR 前一键对齐 CI（推荐）：`make preflight`
- 发 PR 规则（强制）：PR 源分支只能是 `wt-dev-main` / `wt-dev-a` / `wt-dev-b`（CI 门禁：`make check pr-branch`）
- DB Schema/迁移（Atlas+Goose，按模块）：`make <module> plan && make <module> lint && make <module> migrate up`
- sqlc：`make sqlc-generate`，然后 `git status --short` 必须为空
- Routing：`make check routing`
- Authz：`make authz-pack && make authz-test && make authz-lint`
- E2E：`make e2e`
- 文档新增/整理：`make check doc`

> 说明：命令入口与门禁结构以 `docs/dev-plans/012-ci-quality-gates.md` 为准；本文件只维护“入口与触发器”，尽量不复制脚本内部实现。

## 1. 事实源（不要复制细节，统一引用）

- 规则入口：`AGENTS.md`
- 计划/规范文档：`docs/dev-plans/`
- 实施路线图：`docs/dev-plans/009-implementation-roadmap.md`
- Readiness 证据记录：`docs/dev-records/`

## 2. 变更触发器矩阵（与 CI 对齐）

| 你改了什么 | 本地必跑 | 备注 |
| --- | --- | --- |
| 任意 Go 代码 | `go fmt ./... && go vet ./... && make check lint && make test` | 不要仅跑 `gofmt`/`go test`，它们覆盖不到 CI lint |
| `.templ` / Tailwind / Astro UI / presentation assets | `make generate && make css` + `git status --short` | 生成物必须提交，否则 CI 会失败 |
| 多语言 JSON | `make check tr` | |
| DB Schema/迁移（Atlas+Goose，按模块） | `make <module> plan && make <module> lint && make <module> migrate up` | 模块级闭环见 `DEV-PLAN-024` |
| sqlc（schema/queries/config） | `make sqlc-generate` + `git status --short` | 规范与 stopline 见 `DEV-PLAN-025` |
| Routing（allowlist/分类/responder） | `make check routing` | 口径见 `DEV-PLAN-017` |
| Authz（Casbin） | `make authz-pack && make authz-test && make authz-lint` | 口径见 `DEV-PLAN-022` |
| E2E（Playwright） | `make e2e` | 门禁结构见 `DEV-PLAN-012` |
| 新增/调整文档 | `make check doc` | 门禁见“文档收敛与门禁” |
| 引入/修改“回退通道/双链路/legacy 分支” | `make check no-legacy` | 禁止 legacy（见 `DEV-PLAN-004M1`） |

## 3. 开发与编码规则（仓库级合约）

### 3.1 基本编码风格

- DO NOT COMMENT EXCESSIVELY：用清晰、可读的代码表达意图，不要堆注释。
- 错误处理遵循项目标准错误类型。
- UI 交互优先复用组件与既有交互模式。
- UI 主题色约定：丘比蓝，统一使用 `#09a7a3`（优先以全局 CSS 变量承载）。
- NEVER read `*_templ.go`（templ 生成文件不可读且无意义）。
- 不要手动对齐缩进：用 `go fmt`/`templ fmt`/已有工具完成格式化。

### 3.2 工具使用红线

- DO NOT USE `sed` 做文件内容修改。
- 未经用户明确批准，禁止通过 `git checkout --` / `git restore` / `git reset` / `git clean` 丢弃或回退未提交改动。
- 新增数据库表（新建迁移中的 `CREATE TABLE` 或 schema 中新增表）前，必须获得用户手工确认。
- `GITHUB_TOKEN` 等密钥只允许放在本机 `.env.local`，不得提交到仓库（CI 使用 GitHub Secrets）。

### 3.3 契约文档优先（Contract First）

- 新增或调整功能（尤其是 API/数据库/鉴权/交互契约变化）前，必须在 `docs/dev-plans/` 新建或更新相应计划文档（遵循 `docs/dev-plans/000-docs-format.md`，可基于 `docs/dev-plans/001-technical-design-template.md`）。
- 代码变更应是对文档契约的履行：文档是“意图”，代码是“实现”；若实现过程中发生范围/契约变化，应先更新计划文档再改代码。
- 例外：仅修复拼写/格式、或不改变外部行为的极小重构，可不强制新增计划文档；但一旦涉及迁移、权限、接口、数据契约，必须按本条执行。

### 3.4 AI 驱动开发：简单而非容易（Simple > Easy）

使用 AI 辅助时，优先追求“简单（Simple）”而不是“容易（Easy）”：先写清边界、不变量、失败路径与验收标准（建议以 dev-plan/Spec 固化），再实现；拒绝补丁式堆叠分支、复制粘贴与相似文件增殖；任何新抽象必须可在 5 分钟内解释清楚、具备可替换性，并能对应到明确的业务约束（评审清单见 `docs/dev-plans/003-simple-not-easy-review-guide.md`）。

### 3.5 时间语义（Valid Time vs Audit/Tx Time）

- 将“业务生效日期/有效期（Valid Time）”从 `timestamptz`（秒/微秒级）收敛为 **day（date）粒度**，对齐 SAP HCM（`BEGDA/ENDDA`）与 PeopleSoft（`EFFDT/EFFSEQ`）的 HR 习惯；同时明确 **时间戳（秒/微秒级）仅用于操作/审计时间（Audit/Tx Time）**（如 `created_at/updated_at/transaction_time`）。

### 3.6 运维与监控（早期阶段）

关于运维与监控，不需要引入开关切换。本项目仍处于初期，未发布上线，避免过度运维和监控。

### 3.7 Greenfield 不变量（009-031）

- One Door：写入必须走 DB Kernel 的 `submit_*_event(...)`（事件 SoT + 同事务同步投射），避免出现第二写入口（`DEV-PLAN-026/030/029/031`）。
- No Tx, No RLS：访问 Greenfield 表必须显式事务 + 租户注入，且 fail-closed（`DEV-PLAN-021/019/025`）。
- 路由治理：命名空间/route_class/全局 responder 契约统一，并由门禁阻断漂移（`DEV-PLAN-017/012`）。
- 授权边界：RLS 圈地 ≠ Casbin 授权；subject/domain/object/action 命名冻结（`DEV-PLAN-021/022/019`）。
- i18n：仅 `en/zh`，语言写入口唯一；不做业务数据多语言（`DEV-PLAN-020`）。
- 模块边界：业务域 4 模块（orgunit/jobcatalog/staffing/person）+ 平台模块 iam；跨模块优先通过 `pkg/**` 与 HTTP/HTMX 组合，避免 Go 代码跨模块 import（`DEV-PLAN-015/016/019`）。
- SetID：record group 为稳定枚举；映射无缺省洞；不得模块自造回退规则（`DEV-PLAN-028`）。
- No Legacy：禁止引入“legacy 分支/回退通道/双链路”（包括 `read=legacy`、兼容别名窗口、旧实现兜底等）；回滚只能走“环境级保护 + 只读/停写/修复后重试”，并必须有门禁阻断（`DEV-PLAN-004M1`）。

### 3.8 用户可见性原则（避免“僵尸功能”）

- 新增功能必须**可发现、可操作**：应在 UI 页面上可见（导航入口/按钮/表单/列表/详情等）并可完成至少一条端到端操作；否则视为“未交付”。
- 若某能力短期必须是“后端先行”（API/内核/工具链）：必须同时提供明确的用户入口规划与验收方式（例如对应页面占位、路由入口、或被现有页面实际调用），避免长期积累隐形/重复/无人使用的功能分支。

## 4. 架构与目录约束（DDD + CleanArchGuard）

每个模块遵循 DDD 分层，依赖约束由仓库内的架构约束配置定义。

```
modules/{module}/
├── domain/
├── infrastructure/
├── services/
└── presentation/
```

更完整的分层/边界说明以 `docs/dev-plans/015-ddd-layering-framework.md` 与 `docs/dev-plans/016-greenfield-hr-modules-skeleton.md` 为准（由本文件引用，不在多处复制）。

## 5. 实施工作流（入口与 SSOT）

- 本地 worktree 约定：`Bugs-And-Blossoms` 工作区固定使用 `wt-dev-main` 分支进行长期开发（跟踪 `origin/wt-dev-main`），不要在该目录切换分支。
- 本地 worktree 约定：`Bugs-And-Blossoms-wt-dev-a` 工作区固定使用 `wt-dev-a` 分支进行长期开发（跟踪 `origin/wt-dev-a`），不要在该目录切换分支。
- 本地 worktree 约定：`Bugs-And-Blossoms-wt-dev-b` 工作区固定使用 `wt-dev-b` 分支进行长期开发（跟踪 `origin/wt-dev-b`），不要在该目录切换分支。
- PR 规则（强制门禁）：禁止创建/使用临时分支；所有 PR 的源分支必须是 `wt-dev-main` / `wt-dev-a` / `wt-dev-b`（CI 门禁：`make check pr-branch`）。
- 避免分叉（每次都做）：在对应 worktree 目录中，始终先同步 `origin/main` 再开发/发 PR/合并后回写。
  - 开工前：`git fetch origin && git status --porcelain=v1` 必须为空，然后 `git merge origin/main`
  - 发 PR 前：再次 `git fetch origin && git merge origin/main`，跑 `make preflight`，再 `git push origin <wt-dev-*>` 并从该分支发起 PR
  - PR 合并后：立刻在“发起该 PR 的固定分支”执行 `git fetch origin && git merge origin/main && git push origin <wt-dev-*>`；并建议另外两个固定分支也各同步一次，避免下次切换时累积冲突
- 主线原则：以 `origin/main` 为唯一主线；所有 worktree 分支都应以 `origin/main` 为基线并定期同步，避免把 `origin/wt-dev-*` 当作集成主线。
- 合并建议：固定 worktree 分支（`wt-dev-*`）向 `origin/main` 合并时优先使用 **merge commit**（GitHub: Create a merge commit），以便后续能通过快进/常规 merge 顺滑同步；`squash`/`rebase` 仅适用于“短生命周期分支”（本仓库日常已禁止创建临时分支），避免出现“内容已进 main 但 hash 不同”的残留分叉。
- P0 前置条件实施方案（契约优先）：`docs/dev-plans/010-p0-prerequisites-contract.md`
- 路线图（执行顺序/并行）：`docs/dev-plans/009-implementation-roadmap.md`
- 版本与工具链基线：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- Atlas + Goose（模块级闭环）：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc（规范与门禁）：`docs/dev-plans/025-sqlc-guidelines.md`
- Tenancy/AuthN（Kratos + session）：`docs/dev-plans/019-tenant-and-authn.md`
- RLS 强租户隔离：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- Authz（Casbin）：`docs/dev-plans/022-authz-casbin-toolchain.md`
- Routing 策略与门禁：`docs/dev-plans/017-routing-strategy.md`
- UI Shell（Astro AHA）：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- i18n（仅 en/zh）：`docs/dev-plans/020-i18n-en-zh-only.md`
- Docs 治理：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
- CI 质量门禁：`docs/dev-plans/012-ci-quality-gates.md`
- SetID：`docs/dev-plans/028-setid-management.md`

## 6. 文档收敛与门禁（New Doc Gate）

目标：防止文档熵增；新增文档必须可发现、可归类、可维护。

- 仓库根目录禁止新增 `.md`（白名单：`AGENTS.md`）。
- 仓库级文档分类：
  - 计划/契约：`docs/dev-plans/`（遵循 `docs/dev-plans/000-docs-format.md`）
  - 证据/记录：`docs/dev-records/`（按 `DEV-PLAN-010` 的 readiness 要求固化证据）
- 命名（新增文件）：
  - 统一使用：`kebab-case.md`
- 可发现性：新增仓库级文档必须在本文件的“文档地图（Doc Map）”中新增链接。
- 门禁：`make check doc`（执行阶段由 CI 触发，仅在文档/资源变更时运行）。

## 7. 文档地图（Doc Map）

- 文档规范：`docs/dev-plans/000-docs-format.md`
- 技术设计模板：`docs/dev-plans/001-technical-design-template.md`
- Valid Time（日粒度 Effective Date）：`docs/dev-plans/032-effective-date-day-granularity.md`
- DEV-PLAN-004：全仓去除版本标记（命名降噪 + 避免对外契约污染）：`docs/dev-plans/004-remove-version-marker-repo-wide.md`
- DEV-PLAN-004M1：禁止 legacy（单链路原则）——清理、门禁与迁移策略：`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- DEV-PLAN-004 记录：全仓去除版本标记——映射表（草案）：`docs/dev-records/dev-plan-004-version-marker-removal-mapping.md`
- DEV-PLAN-004 记录：全仓去除版本标记——执行日志：`docs/dev-records/dev-plan-004-execution-log.md`
- Valid Time（日粒度 Effective Date）：`docs/dev-plans/032-effective-date-day-granularity.md`
- DEV-PLAN-060：全链路业务测试案例套件（009/039/051-056 覆盖）：`docs/dev-plans/060-business-e2e-test-suite.md`
- DEV-PLAN-061：全链路业务测试子计划 TP-060-01——租户/登录/权限/隔离基线：`docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`
- DEV-PLAN-062：全链路业务测试子计划 TP-060-02——主数据（组织架构 + SetID + JobCatalog + 职位）：`docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`
- DEV-PLAN-063：全链路业务测试子计划 TP-060-03——人员与任职（Person + Assignments）：`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`
- DEV-PLAN-069：移除薪酬社保与考勤（文档/代码/测试/数据库）：`docs/dev-plans/069-remove-payroll-attendance.md`
- DEV-PLAN-069 执行日志：`docs/dev-records/dev-plan-069-execution-log.md`
- P0 前置条件实施方案（契约优先）：`docs/dev-plans/010-p0-prerequisites-contract.md`
- AI 驱动开发评审清单（Simple > Easy）：`docs/dev-plans/003-simple-not-easy-review-guide.md`
- Org（事务性事件溯源 + 同步投射）：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- Position（事务性事件溯源 + 同步投射）：`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
- Job Catalog（事务性事件溯源 + 同步投射）：`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- PostgreSQL RLS 强租户隔离（Org/Position/Job Catalog）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- DDD 分层框架（对齐 CleanArchGuard + DB Kernel）：`docs/dev-plans/015-ddd-layering-framework.md`
- Greenfield HR 模块骨架与契约（OrgUnit/JobCatalog/Staffing/Person）：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- 任职记录（Job Data / Assignments）（事件 SoT + 同步投射）：`docs/dev-plans/031-greenfield-assignment-job-data.md`
- Person 最小身份锚点（Pernr 1-8 位数字字符串）：`docs/dev-plans/027-person-minimal-identity-for-staffing.md`
- 引入 Astro（AHA Stack）到 HRMS UI：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- 技术栈与工具链版本冻结：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- 租户管理与登录认证：`docs/dev-plans/019-tenant-and-authn.md`
- SuperAdmin 控制面认证与会话：`docs/dev-plans/023-superadmin-authn.md`
- 多语言（仅 en/zh）：`docs/dev-plans/020-i18n-en-zh-only.md`
- Atlas + Goose 闭环指引：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 工具链使用指引与规范：`docs/dev-plans/025-sqlc-guidelines.md`
- Authz（Casbin）工具链与实施方案：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 多工作区并行开发指引（3 worktree）：`docs/dev-plans/014-parallel-worktrees-local-dev-guide.md`
- 全局路由策略统一（UI/HTMX/API/Webhooks）：`docs/dev-plans/017-routing-strategy.md`
- 文档创建与过程治理规范：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
- CI 质量门禁（Quality Gates）：`docs/dev-plans/012-ci-quality-gates.md`
- SetID 管理（Greenfield）：`docs/dev-plans/028-setid-management.md`
- DEV-PLAN-009M1：Phase 4 下一大型里程碑执行计划（SetID + JobCatalog 首个可见样板闭环）：`docs/dev-plans/009m1-phase4-setid-jobcatalog-vertical-slice-execution-plan.md`
- DEV-PLAN-009M2：Phase 4 下一大型里程碑执行计划（Person Identity + Staffing 首个可见样板闭环）：`docs/dev-plans/009m2-phase4-person-identity-staffing-vertical-slice-execution-plan.md`
- DEV-PLAN-009M3：Phase 5 下一大型里程碑执行计划（质量收口：E2E 真实化 + 可排障门禁）：`docs/dev-plans/009m3-phase5-quality-hardening-e2e-execution-plan.md`
- DEV-PLAN-009M4：Phase 2 下一大型里程碑执行计划（SuperAdmin 控制面 + Tenant Console MVP）：`docs/dev-plans/009m4-phase2-superadmin-tenant-console-execution-plan.md`
- DEV-PLAN-009M5：Phase 2 下一大型里程碑执行计划（AuthN 真实化：Kratos + 本地会话 sid/sa_sid）：`docs/dev-plans/009m5-phase2-authn-kratos-sessions-execution-plan.md`
- DEV-PLAN-009M6：Phase 1 追加里程碑执行计划（补齐 DEV-PLAN-018 Phase 0：Astro build + go:embed Shell）：`docs/dev-plans/009m6-phase1-astro-build-phase0-execution-plan.md`
- Greenfield 全新实施路线图（009-031）：`docs/dev-plans/009-implementation-roadmap.md`
