# 请总是用中文回复。

# AGENTS.md（主干 SSOT）

本文件是仓库内“如何开发/如何验证/如何组织文档与规则”的**主干入口**。优先阅读本文件，并通过链接跳转到其他专题文档；避免在多个文档里复制同一套规则，减少漂移。

本仓库为 Greenfield 的 **implementation repo**（仓库名：`Bugs-And-Blossoms`）。执行顺序与并行策略以 `docs/dev-plans/009-implementation-roadmap.md` 为准；`docs/dev-plans/` 为契约文档 SSOT（同时通过 `docs/dev-plans` 入口可达），P0-Ready 的证据记录在 `docs/dev-records/`。

## 0. TL;DR（最常见变更要跑什么）

- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 禁止 legacy（单链路原则）：`make check no-legacy`（或直接跑 `make preflight`）
- `.templ`/MUI Web UI/presentation assets 相关：`make generate && make css`，然后 `git status --short` 必须为空
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
| `.templ` / MUI Web UI / presentation assets | `make generate && make css` + `git status --short` | 生成物必须提交，否则 CI 会失败 |
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
- 模块边界：业务域 4 模块（orgunit/jobcatalog/staffing/person）+ 平台模块 iam；跨模块优先通过 `pkg/**` 与 HTTP/JSON API 组合，避免 Go 代码跨模块 import（`DEV-PLAN-015/016/019`）。
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
- UI Shell（历史，已被 DEV-PLAN-103 替代）：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
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
- DEV-PLAN-070：SetID 绑定组织架构重构方案：`docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- DEV-PLAN-070 执行日志：`docs/dev-records/dev-plan-070-execution-log.md`
- DEV-PLAN-071：SetID Scope Package 订阅蓝图：`docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- DEV-PLAN-071A：基于 Package 的配置编辑与订阅显式化：`docs/dev-plans/071a-package-selection-ownership-and-subscription.md`
- DEV-PLAN-072：全仓 ID/Code 命名与对外标识收敛：`docs/dev-plans/072-repo-wide-id-code-naming-convergence.md`
- DEV-PLAN-073：OrgUnit CRUD 实现清单（页面与 API）：`docs/dev-plans/073-orgunit-crud-implementation-status.md`
- DEV-PLAN-073A：组织架构树运行态问题记录（Shoelace 资源加载失败）：`docs/dev-plans/073a-orgunit-tree-runtime-issue.md`
- DEV-PLAN-074：OrgUnit Details 集成更新能力与 UI 优化方案：`docs/dev-plans/074-orgunit-details-update-ui-optimization.md`
- DEV-PLAN-074 执行日志：`docs/dev-records/dev-plan-074-execution-log.md`
- DEV-PLAN-075：OrgUnit 生效日期不允许回溯的限制评估：`docs/dev-plans/075-orgunit-effective-date-backdating-assessment.md`
- DEV-PLAN-075 执行日志：`docs/dev-records/dev-plan-075-execution-log.md`
- DEV-PLAN-075A：OrgUnit 记录新增/插入 UI 与可编辑字段问题记录：`docs/dev-plans/075a-orgunit-records-ui-and-editing-issues.md`
- DEV-PLAN-075A 执行日志：`docs/dev-records/dev-plan-075a-execution-log.md`
- DEV-PLAN-075B：Root Unit A 生效日回溯可行性调查与修复方案：`docs/dev-plans/075b-orgunit-root-backdating-feasibility-and-fix-plan.md`
- DEV-PLAN-075B 执行日志：`docs/dev-records/dev-plan-075b-execution-log.md`
- DEV-PLAN-075C：OrgUnit 删除记录/停用语义混用调查与收敛方案：`docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- DEV-PLAN-075D：OrgUnit 页面状态字段与有效/无效显式切换：`docs/dev-plans/075d-orgunit-status-field-active-inactive-selector.md`
- DEV-PLAN-075E：OrgUnit 同日状态修正（生效日不变）方案：`docs/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
- DEV-PLAN-075D 执行日志：`docs/dev-records/dev-plan-075d-execution-log.md`
- DEV-PLAN-075E 执行日志：`docs/dev-records/dev-plan-075e-execution-log.md`
- DEV-PLAN-076：OrgUnit 版本切换导致选中组织丢失问题与修复方案：`docs/dev-plans/076-orgunit-version-switch-selection-retention.md`
- DEV-PLAN-077：OrgUnit Replay 写放大评估与收敛方案：`docs/dev-plans/077-orgunit-replay-write-amplification-assessment-and-mitigation.md`
- DEV-PLAN-077 记录：OrgUnit Replay 写放大基线测量：`docs/dev-records/dev-plan-077-write-amplification-baseline.md`
- DEV-PLAN-078 执行日志：`docs/dev-records/dev-plan-078-execution-log.md`
- DEV-PLAN-078：OrgUnit 写模型替代方案对比与决策建议：`docs/dev-plans/078-orgunit-write-model-alternatives-comparison-and-decision.md`
- DEV-PLAN-080：OrgUnit 审计链收敛（方向 1：单一审计链）：`docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
- DEV-PLAN-080A：OrgUnit before_snapshot/after_snapshot 机制调查与收敛修复方案：`docs/dev-plans/080a-orgunit-audit-snapshot-mechanism-and-fix.md`
- DEV-PLAN-080B：OrgUnit 生效日更正失败（orgunit_correct_failed）专项调查与修复方案：`docs/dev-plans/080b-orgunit-correction-failure-investigation-and-remediation.md`
- DEV-PLAN-080C：OrgUnit 审计快照 presence 表级强约束（INSERT 即写齐）方案：`docs/dev-plans/080c-orgunit-audit-snapshot-presence-table-constraint-plan.md`
- DEV-PLAN-080D：OrgUnit 变更日志“已撤销事件未标识”专项调查与收敛方案：`docs/dev-plans/080d-orgunit-audit-rescinded-event-visibility-investigation.md`
- DEV-PLAN-081：OrgUnit Details 记录版本选择器双栏化（左生效日期 / 右详情）：`docs/dev-plans/081-orgunit-records-version-selector-two-pane-alignment.md`
- DEV-PLAN-082：Org 模块业务字段修改规则全量调查（排除元数据）：`docs/dev-plans/082-org-module-field-mutation-rules-investigation.md`
- DEV-PLAN-083：Org 白名单模型扩展性改造（规则单点化 + 能力矩阵外显）：`docs/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- DEV-PLAN-090：前端框架升级为 MUI X（对标 Workday UX）方案：`docs/dev-plans/090-mui-x-frontend-upgrade-plan.md`
- DEV-PLAN-091：MUI X 升级子计划 P0（基座准备与许可落地）：`docs/dev-plans/091-mui-x-phase0-foundation-and-license-plan.md`
- DEV-PLAN-092：MUI X 升级子计划 P1（壳与导航迁移）：`docs/dev-plans/092-mui-x-phase1-shell-navigation-plan.md`
- DEV-PLAN-093：MUI X 升级子计划 P2（高价值模块迁移）：`docs/dev-plans/093-mui-x-phase2-high-value-modules-plan.md`
- DEV-PLAN-094：MUI X 升级子计划 P3（长尾迁移与收口）：`docs/dev-plans/094-mui-x-phase3-long-tail-convergence-plan.md`
- DEV-PLAN-095：MUI X 升级子计划 P4（稳定化与性能压测）：`docs/dev-plans/095-mui-x-phase4-stability-performance-plan.md`
- DEV-PLAN-096：Org 模块全量迁移至 MUI X 与统一体验收口方案：`docs/dev-plans/096-org-module-full-migration-and-ux-convergence-plan.md`
- DEV-PLAN-097：OrgUnit 详情从抽屉（Drawer）迁移为独立页面（对齐 MUI CRUD Dashboard）：`docs/dev-plans/097-orgunit-details-drawer-to-page-migration.md`
- DEV-PLAN-098：组织架构模块架构评估——多类型宽表预留字段 + 元数据驱动（V2.0）：`docs/dev-plans/098-org-module-wide-table-metadata-driven-architecture-assessment.md`
- DEV-PLAN-099：OrgUnit 信息页双栏化（左生效日期/修改时间，右侧详情）——对齐示例：`docs/dev-plans/099-orgunit-details-two-pane-info-audit-mui.md`
- DEV-PLAN-100：Org 模块宽表预留字段 + 元数据驱动落地实施计划与路线图（承接 DEV-PLAN-098）：`docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- DEV-PLAN-100A：Org 模块宽表元数据落地 Phase 0：契约冻结与就绪检查：`docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`
- DEV-PLAN-100B：Org 模块宽表元数据落地 Phase 1：Schema 与元数据骨架（最小数据库闭环）：`docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`
- DEV-PLAN-100C：Org 模块宽表元数据落地 Phase 2：Kernel/Projection 扩展（保持 One Door）：`docs/dev-plans/100c-org-metadata-wide-table-phase2-kernel-projection-extension-one-door.md`
- DEV-PLAN-100C 执行日志：`docs/dev-records/dev-plan-100c-execution-log.md`
- DEV-PLAN-100D：Org 模块宽表元数据落地 Phase 3：服务层与 API（读写可用）：`docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- DEV-PLAN-100D2：Org 模块宽表元数据落地 Phase 3 修订：契约对齐与 API 实现收口（为 100E/101 做准备）：`docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
- DEV-PLAN-100D2 执行日志：`docs/dev-records/dev-plan-100d2-execution-log.md`
- DEV-PLAN-100E：Org 模块宽表元数据落地 Phase 4A：OrgUnit 详情页扩展字段展示与 Capabilities 驱动编辑（MUI）：`docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- DEV-PLAN-100E1：OrgUnit Mutation Policy 单点化 + 更正链路支持 `patch.ext`（作为 DEV-PLAN-100E 前置）：`docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
- DEV-PLAN-101：OrgUnit 字段配置管理页（MUI）IA 与组件级方案（承接 DEV-PLAN-100）：`docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- DEV-PLAN-102：全项目 as_of 时间上下文收敛与批判（承接 DEV-PLAN-076）：`docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- DEV-PLAN-102 执行日志：`docs/dev-records/dev-plan-102-execution-log.md`
- DEV-PLAN-103：移除 Astro/HTMX，前端收敛为 MUI X（React SPA）：`docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`
- DEV-PLAN-103 执行日志：`docs/dev-records/dev-plan-103-execution-log.md`
- DEV-PLAN-080 执行日志：`docs/dev-records/dev-plan-080-execution-log.md`
- DEV-PLAN-073 执行日志：`docs/dev-records/dev-plan-073-execution-log.md`
- DEV-PLAN-071 执行日志：`docs/dev-records/dev-plan-071-execution-log.md`
- DEV-PLAN-072 记录：命名收敛差异清单与映射表：`docs/dev-records/dev-plan-072-naming-convergence-mapping.md`
- DEV-PLAN-026A：OrgUnit 8位编号与 UUID/Code 命名规范：`docs/dev-plans/026a-orgunit-id-uuid-code-naming.md`
- DEV-PLAN-026A 执行日志：`docs/dev-records/dev-plan-026a-execution-log.md`
- DEV-PLAN-026B：OrgUnit 外部ID兼容（org_code 映射）方案：`docs/dev-plans/026b-orgunit-external-id-code-mapping.md`
- DEV-PLAN-026C：OrgUnit 外部ID兼容（org_code 映射）评审与修订方案：`docs/dev-plans/026c-orgunit-external-id-code-mapping-review-and-revision.md`
- DEV-PLAN-026C 执行日志：`docs/dev-records/dev-plan-026c-execution-log.md`
- DEV-PLAN-026D：OrgUnit 增量投射方案（减少全量回放写放大）：`docs/dev-plans/026d-orgunit-incremental-projection-plan.md`
- DEV-PLAN-026D 执行日志：`docs/dev-records/dev-plan-026d-execution-log.md`
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
- 引入 Astro（AHA Stack）到 HRMS UI（历史，已被 DEV-PLAN-103 替代）：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
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
- DEV-PLAN-009M6：Phase 1 追加里程碑执行计划（历史：补齐 DEV-PLAN-018 Phase 0，已由 DEV-PLAN-103 收口）：`docs/dev-plans/009m6-phase1-astro-build-phase0-execution-plan.md`
- Greenfield 全新实施路线图（009-031）：`docs/dev-plans/009-implementation-roadmap.md`
