# 请总是用中文回复。

# AGENTS.md（主干 SSOT）

本文件是仓库内“如何开发/如何验证/如何组织文档与规则”的**主干入口**。优先阅读本文件，并通过链接跳转到其他专题文档；避免在多个文档里复制同一套规则，减少漂移。

本仓库为 Greenfield 的 **implementation repo**（仓库名：`Bugs-And-Blossoms`）。Greenfield 早期执行顺序与并行策略历史来源见 `docs/archive/dev-plans/009-implementation-roadmap.md`；当前现行 owner 以 `docs/dev-plans/440-complete-setid-removal-plan.md`、`docs/dev-plans/450-direct-removal-of-jobcatalog-staffing-person-modules-plan.md` 及其他活体 dev-plan 为准。`docs/dev-plans/` 为契约文档 SSOT（同时通过 `docs/dev-plans` 入口可达），P0-Ready 的证据记录在 `docs/dev-records/`。

## 0. TL;DR（最常见变更要跑什么）

- Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
- 新建 Go 模块后（防止 `go mod init` 默认回退）：执行 `go get go@1.26.0`（或 `go mod edit -go=1.26.0`）
- 禁止 legacy（单链路原则）：`make check no-legacy`（或直接跑 `make preflight`）
- 历史对话面清场门禁（阻断旧 `assistant` / `LibreChat` / `CubeBox` 运行面与兼容语义回流）：`make check chat-surface-clean`
- 禁止新增 scope/package 漂移：`make check no-scope-package`
- 颗粒度层次门禁（阻断 org_level/scope_type/scope_key 回流）：`make check granularity`
- DDD 分层 P0 反漂移门禁（阻断 `internal/server` 扩散与 `infrastructure -> services` 回流）：`make check ddd-layering-p0`
- DDD 分层 P2 组合根门禁（模块扩张时要求 `module.go/links.go` 承接职责）：`make check ddd-layering-p2`
- 业务幂等字段命名收敛：`make check request-code`
- `.templ`/MUI Web UI/presentation assets 相关：`make generate && make css`，然后 `git status --short` 必须为空
- 多语言 JSON：`make check tr`
- 提交前 Go 版本门禁：`make check go-version`（或直接跑 `make preflight`）
- 错误提示收敛门禁（禁止泛化失败文案直出）：`make check error-message`
- 发 PR 前一键对齐 CI（推荐）：`make preflight`
- 发 PR 规则（强制）：PR 源分支只能是 `wt-dev-main` / `wt-dev-a` / `wt-dev-b`（CI 门禁：`make check pr-branch`）
- DB Schema/迁移（Atlas+Goose，按模块）：`make <module> plan && make <module> lint && make <module> migrate up`
- sqlc：`make sqlc-generate`，然后 `git status --short` 必须为空；命中 DB 触发器时补跑 `make sqlc-verify-schema`
- Routing：`make check routing`
- Authz：`make authz-pack && make authz-test && make authz-lint`
- E2E：`make e2e`
- 文档新增/整理：`make check doc`

> 说明：命令入口与门禁结构以 `docs/dev-plans/012-ci-quality-gates.md` 为准；本文件只维护“入口与触发器”，尽量不复制脚本内部实现。

## 1. 事实源（不要复制细节，统一引用）

- 规则入口：`AGENTS.md`
- 计划/规范文档：`docs/dev-plans/`
- 实施路线图【归档 / 历史来源】：`docs/archive/dev-plans/009-implementation-roadmap.md`
- Readiness 证据记录：`docs/dev-records/`

## 2. 变更触发器矩阵（与 CI 对齐）

| 你改了什么 | 本地必跑 | 备注 |
| --- | --- | --- |
| 任意 Go 代码 | `go fmt ./... && go vet ./... && make check lint && make test` | 不要仅跑 `gofmt`/`go test`，它们覆盖不到 CI lint |
| `go.mod` / `.tool-versions`（Go 版本口径） | `make check go-version` | 防止 `go mod init` 默认回退，保持 `1.26.x` |
| `.templ` / MUI Web UI / presentation assets | `make generate && make css` + `git status --short` | 生成物必须提交，否则 CI 会失败 |
| 多语言 JSON | `make check tr` | |
| DB Schema/迁移（Atlas+Goose，按模块） | `make <module> plan && make <module> lint && make <module> migrate up` | 模块级闭环见 `DEV-PLAN-024` |
| sqlc（schema/queries/config） | `make sqlc-generate` + `git status --short`（命中 DB 触发器再跑 `make sqlc-verify-schema`） | 规范与 stopline 见 `DEV-PLAN-025/025A` |
| Routing（allowlist/分类/responder） | `make check routing` | 口径见 `DEV-PLAN-017` |
| Authz（Casbin） | `make authz-pack && make authz-test && make authz-lint` | 口径见 `DEV-PLAN-022` |
| E2E（Playwright） | `make e2e` | 门禁结构见 `DEV-PLAN-012`；数据库依赖口径冻结为 Docker / compose，E2E 不得把宿主机 `psql` 等工具作为唯一前置条件 |
| 新增/调整文档 | `make check doc` | 门禁见“文档收敛与门禁” |
| 引入/修改“回退通道/双链路/legacy 分支” | `make check no-legacy` | 禁止 legacy（见 `DEV-PLAN-004M1`） |
| 历史对话面 / 旧路由 / 旧兼容语义相关改动 | `make check chat-surface-clean` | 硬删除后唯一反回流门禁（见 `DEV-PLAN-436`） |
| 新增 scope/package 语义引用（`scope_code/scope_package/scope_subscription/package_id`） | `make check no-scope-package` | 增量反漂移门禁（承接 `DEV-PLAN-102C6`） |
| 颗粒度层次/旧 scope 相关新增（`org_level/scope_type/scope_key`） | `make check granularity` | 颗粒度治理门禁（承接 `DEV-PLAN-180`） |
| DDD 分层相关新增漂移（`internal/server` 扩散模块实现、`modules/*/infrastructure -> services` 回流） | `make check ddd-layering-p0` | P0 止血门禁（承接 `DEV-PLAN-015B/015C`） |
| 模块分层扩张且组合根需同步承接（`module.go/links.go` 不得继续空壳） | `make check ddd-layering-p2` | P2 组合根门禁（承接 `DEV-PLAN-015B/015Z4`） |
| 幂等与追踪命名（request_id / trace_id） | `make check request-code` | 规则见 `DEV-PLAN-109A` |
| 错误提示契约（错误码→明确提示） | `make check error-message` | 规则见 `DEV-PLAN-140` |

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
- 模块边界：现行业务域保留 `orgunit`，平台模块保留 `iam`；跨模块优先通过 `pkg/**` 与 HTTP/JSON API 组合，避免 Go 代码跨模块 import（`DEV-PLAN-015/016/019`）。
- SetID：现行删除与收口以 `DEV-PLAN-440` 为唯一 PoR；历史 SetID 方案仅允许作为 archive/历史来源或待归档调查材料保留，不得再作为当前实现前提、当前用户入口或回退依据。
- No Legacy：禁止引入“legacy 分支/回退通道/双链路”（包括 `read=legacy`、兼容别名窗口、旧实现兜底等）；回滚只能走“环境级保护 + 只读/停写/修复后重试”，并必须有门禁阻断（`DEV-PLAN-004M1`）。

### 3.8 用户可见性原则（避免“僵尸功能”）

- 新增功能必须**可发现、可操作**：应在 UI 页面上可见（导航入口/按钮/表单/列表/详情等）并可完成至少一条端到端操作；否则视为“未交付”。
- 若某能力短期必须是“后端先行”（API/内核/工具链）：必须同时提供明确的用户入口规划与验收方式（例如对应页面占位、路由入口、或被现有页面实际调用），避免长期积累隐形/重复/无人使用的功能分支。

### 3.9 死分支与覆盖率处理原则（强制）

- 当前覆盖率门禁下，**死分支优先删除，不允许为凑覆盖率长期保留可证明不可达的分支**。
- 对覆盖率缺口，必须先分类："可构造场景的真实分支" → 补测试；"可证明不可达的死分支" → 删除或前移为更早的不变量约束；不得默认通过改门禁口径、加排除项或伪造 fallback 来绕过。
- 若某分支当前看似不可达但承担明确业务兜底语义，必须保留，并通过更小职责拆分、纯函数化、接口隔离或可注入依赖提升可测性；不得因测试困难直接删除。
- 删除死分支时，必须满足：① 能说明不可达原因；② 删除后不改变对外契约；③ 相关测试与文档同步更新。
- 未经用户明确批准，不得通过降低阈值、扩大 coverage 排除项、缩小统计范围来替代“删死分支/补测试”。

### 3.10 测试设计原则（DEV-PLAN-300/301）

- 测试设计与新增测试文件前，必须先对齐 `docs/dev-plans/300-test-system-investigation-report.md` 与 `docs/archive/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`；其中 `300` 是测试问题基线，`301` 作为首轮分层整改历史方案来源。
- 严禁继续以 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 一类“补洞式”命名追加同质测试；新增测试应围绕稳定职责、明确场景与子测试组织，而不是围绕 coverage 缺口堆文件。
- 优先补“最小而稳定”的直接测试：纯函数、解析/归一化链路、错误映射、默认值策略、边界日期/空值/非法输入；先把职责拆小、把逻辑做纯，再补测试，不要把复杂装配硬塞进测试里。
- Go 测试分层遵循 `DEV-PLAN-301` 的历史收敛口径：`pkg/**` 优先承载纯函数与工具层边界测试，`modules/*/services` 优先承载业务规则与默认值策略测试，`internal/server` 只保留路由、协议解析、错误映射、租户/鉴权/RLS 适配与跨模块编排测试。
- 对导出边界、纯函数、解析器、validator，优先考虑黑盒测试（`package xxx_test`）；只有在验证未导出不变量或明确内部状态推进时，才保留白盒测试，并需能说明理由。
- 并行、`t.Setenv`、fuzz、benchmark 的使用必须遵循 `DEV-PLAN-300/301` 引用的 Go 官方实践：仅在隔离成立时启用并行；修改环境变量或全局状态的测试不得与 parallel 混用；对开放输入空间的解析/归一化路径优先评估 fuzz；仅对热点纯函数补 benchmark。
- 前端测试同样遵循“职责下沉、最小边界、避免补洞式堆叠”的原则：优先测试可提纯的小函数、小状态机、小转换器；仅在纯函数无法覆盖关键用户行为时，再增加页面级交互测试。

### 3.11 缓存默认方案与外部依赖准入

- 缓存默认工具链冻结为 **Go 原生 + pgx + PostgreSQL**（优先 request-scope 复用与进程内短 TTL，回源 PostgreSQL）。
- 原则：先使用“原生与扩展”，避免过早引入外部缓存基础设施或第三方缓存库。
- 仅当存在明确必要（性能预算/停止线无法满足，且已有压测与回归证据）时，才允许申请启用 `Redis` / `Ristretto` / `BigCache` 等外部缓存方案。
- 启用外部缓存前必须完成：用户审批 + 契约文档更新（`docs/dev-plans/`）+ 失效/一致性与回退策略评审。

## 4. 架构与目录约束（DDD + CleanArchGuard）

每个模块遵循 DDD 分层，依赖约束由仓库内的架构约束配置定义。

```
modules/{module}/
├── domain/
├── infrastructure/
├── services/
└── presentation/
```

更完整的分层/边界说明以 `docs/dev-plans/015-ddd-layering-framework.md` 与 `docs/archive/dev-plans/016-greenfield-hr-modules-skeleton.md` 为准（由本文件引用，不在多处复制）。

## 5. 实施工作流（入口与 SSOT）

- 本地 worktree 约定：`Bugs-And-Blossoms` 工作区固定使用 `wt-dev-main` 分支进行长期开发（跟踪 `origin/wt-dev-main`），不要在该目录切换分支。
- 本地 worktree 约定：`Bugs-And-Blossoms-wt-dev-a` 工作区固定使用 `wt-dev-a` 分支进行长期开发（跟踪 `origin/wt-dev-a`），不要在该目录切换分支。
- 本地 worktree 约定：`Bugs-And-Blossoms-wt-dev-b` 工作区固定使用 `wt-dev-b` 分支进行长期开发（跟踪 `origin/wt-dev-b`），不要在该目录切换分支。
- PR 规则（强制门禁）：禁止创建/使用临时分支；所有 PR 的源分支必须是 `wt-dev-main` / `wt-dev-a` / `wt-dev-b`（CI 门禁：`make check pr-branch`）。
- 避免分叉（每次都做）：在对应 worktree 目录中，始终先同步 `origin/main` 再开发/发 PR/合并后回写。
  - 开工前：`git fetch origin && git status --porcelain=v1` 必须为空，然后 `git merge origin/main`
  - 发 PR 前：再次 `git fetch origin && git merge origin/main`，跑 `make preflight`，再 `git push origin <wt-dev-*>` 并从该分支发起 PR
  - PR 合并后：立刻在“发起该 PR 的固定分支”执行 `git fetch origin && git merge origin/main && git push origin <wt-dev-*>`；并建议另外两个固定分支也各同步一次，避免下次切换时累积冲突
- GitHub 推送链路约定：若 `https://github.com` 的 443 连通性不稳定或 `git push origin <wt-dev-*>` 长时间无响应，优先改用 **SSH over `ssh.github.com:443`** 这一已验证成功的链路，不要反复卡在 HTTPS 重试。
  - 优先命令：`git push 'ssh://git@ssh.github.com:443/<owner>/<repo>.git' <wt-dev-*>`
  - 认证顺序：优先复用本机已有 GitHub SSH key；若账号已通过 `gh auth` 登录且具备 `admin:public_key` scope，可临时生成本机 key、通过 `gh api user/keys` 登记后完成推送；任务结束后应删除不再需要的临时 key。
- 主线原则：以 `origin/main` 为唯一主线；所有 worktree 分支都应以 `origin/main` 为基线并定期同步，避免把 `origin/wt-dev-*` 当作集成主线。
- 合并建议：固定 worktree 分支（`wt-dev-*`）向 `origin/main` 合并时优先使用 **merge commit**（GitHub: Create a merge commit），以便后续能通过快进/常规 merge 顺滑同步；`squash`/`rebase` 仅适用于“短生命周期分支”（本仓库日常已禁止创建临时分支），避免出现“内容已进 main 但 hash 不同”的残留分叉。
- P0 前置条件实施方案（契约优先）：`docs/dev-plans/010-p0-prerequisites-contract.md`
- 路线图（执行顺序/并行，归档历史来源）：`docs/archive/dev-plans/009-implementation-roadmap.md`
- 版本与工具链基线：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- Atlas + Goose（模块级闭环）：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc（规范与门禁）：`docs/dev-plans/025-sqlc-guidelines.md`
- Tenancy/AuthN（Kratos + session）：`docs/dev-plans/019-tenant-and-authn.md`
- RLS 强租户隔离【归档 / 历史合同】：`docs/archive/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- Authz（Casbin）：`docs/dev-plans/022-authz-casbin-toolchain.md`
- Routing 策略与门禁：`docs/dev-plans/017-routing-strategy.md`
- UI Shell（历史，已被 DEV-PLAN-103 替代，已归档）：`docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- i18n（仅 en/zh）：`docs/dev-plans/020-i18n-en-zh-only.md`
- Docs 治理：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
- CI 质量门禁：`docs/dev-plans/012-ci-quality-gates.md`
- 时间口径（现行）：`docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`；其中 070/071/028 的 SetID 历史语义仅可作为历史来源引用，不得再作为当前实现前提；现行删除 owner 见 `docs/dev-plans/440-complete-setid-removal-plan.md`

## 6. 文档收敛与门禁（New Doc Gate）

目标：防止文档熵增；新增文档必须可发现、可归类、可维护。

- 仓库根目录禁止新增 `.md`（白名单：`AGENTS.md`）。
- 仓库级文档分类：
  - 计划/契约：`docs/dev-plans/`（遵循 `docs/dev-plans/000-docs-format.md`）
  - 证据/记录：`docs/dev-records/`（按 `DEV-PLAN-010` 的 readiness 要求固化证据）
  - 历史归档：`docs/archive/`（已作废/已替代/仅过程性且不再具参考意义的文档）
- 归档规则（强制）：已经作废、已被替代、已不具有参考意义的过程性开发计划文档，必须转入 `docs/archive/dev-plans/`（例如：`DEV-PLAN-018`、`DEV-PLAN-026` 系列）。
- 命名（新增文件）：
  - 统一使用：`kebab-case.md`
- 可发现性：新增仓库级活体文档（计划/规范/指南/runbook/索引）必须在本文件的“文档地图（Doc Map）”中新增链接；执行日志/Readiness 证据/过程性 `dev-record` 不逐条纳入 `AGENTS.md` 文档地图，应通过 `docs/dev-records/README.md`、`docs/archive/dev-records/README.md` 或对应计划文档的关联章节发现。
- 门禁：`make check doc`（执行阶段由 CI 触发，仅在文档/资源变更时运行）。

## 7. 文档地图（Doc Map）

- 文档归档入口：`docs/archive/README.md`
- 当前执行日志 / Readiness 证据入口：`docs/dev-records/README.md`（执行日志与过程性证据不逐条纳入本节；请通过目录入口或对应计划文档的关联章节查找）
- 历史执行记录归档：`docs/archive/dev-records/README.md`
- 文档规范：`docs/dev-plans/000-docs-format.md`
- 技术设计模板：`docs/dev-plans/001-technical-design-template.md`
- DEV-PLAN-002：UI 设计规范（React + MUI Core + MUI X / Material Design Web）：`docs/dev-plans/002-ui-design-guidelines.md`
- Valid Time（日粒度 Effective Date）：`docs/dev-plans/032-effective-date-day-granularity.md`
- DEV-PLAN-004【归档】：全仓去除版本标记（命名降噪 + 避免对外契约污染，规范已并入 `DEV-PLAN-005/STD-004`）：`docs/archive/dev-plans/004-remove-version-marker-repo-wide.md`
- DEV-PLAN-004M1：禁止 legacy（单链路原则）——清理、门禁与迁移策略：`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- DEV-PLAN-005：项目标准与外部规范采纳清单（规范入口，持续扩展）：`docs/dev-plans/005-project-standards-and-spec-adoption.md`
- Valid Time（日粒度 Effective Date）：`docs/dev-plans/032-effective-date-day-granularity.md`
- DEV-PLAN-060：全链路业务测试案例套件（009/026-031/220-225 覆盖）：`docs/dev-plans/060-business-e2e-test-suite.md`
- DEV-PLAN-061：全链路业务测试子计划 TP-060-01——租户/登录/权限/隔离基线：`docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`
- DEV-PLAN-062【归档 / 历史合同】：全链路业务测试子计划 TP-060-02（含 SetID 主链样本；现行删除 owner 见 `DEV-PLAN-440`）：`docs/archive/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`
- DEV-PLAN-063【归档 / 历史合同】：全链路业务测试子计划 TP-060-03——人员与任职（Person + Assignments）：`docs/archive/dev-plans/063-test-tp060-03-person-and-assignments.md`
- DEV-PLAN-381【归档】：CubeBox capability 与 functional area 历史来源专项调查：`docs/archive/dev-plans/381-cubebox-capability-and-functional-area-lineage-investigation.md`
- DEV-PLAN-382【归档】：Capability Functional Area 治理影响面专项调查：`docs/archive/dev-plans/382-capability-functional-area-governance-impact-investigation.md`
- DEV-PLAN-383【归档】：Functional Area 与 DDD 模块并行第二维度风险专项调查与收敛建议：`docs/archive/dev-plans/383-functional-area-vs-ddd-module-second-axis-investigation-and-remediation-plan.md`
- DEV-PLAN-384：220-292 归档后测试资产重评估与防回流专项方案：`docs/dev-plans/384-220-292-archived-test-reassessment-and-anti-backflow-plan.md`
- DEV-PLAN-390：历史文档入口已失效；涉及 SetID 与 scope/package/subscription 根删除的现行 owner 统一以 `DEV-PLAN-440` 为准。
- DEV-PLAN-391：DEV-PLAN-390 执行排序、命中清单与分批落地方案：`docs/dev-plans/391-dev-plan-390-execution-sequencing-and-hit-list.md`
- DEV-PLAN-391A：阶段 A / PR-A：`authz requirement` 单主源与 390 反回流门禁冻结：`docs/dev-plans/391a-phase-a-authz-requirement-single-source-and-anti-backflow-gates-freeze.md`
- DEV-PLAN-391B：阶段 B / PR-B：治理 runtime 主切断与 legacy surface 成组下线：`docs/dev-plans/391b-phase-b-main-cutover-governance-runtime-and-legacy-surface-removal-plan.md`
- DEV-PLAN-391B1【归档】：阶段 B 尾扫：OrgUnit 字段决策与 SetID Binding 最终 runtime 切断：`docs/archive/dev-plans/391b1-phase-b-orgunit-field-decision-and-setid-binding-final-runtime-cutover.md`
- DEV-PLAN-391C：阶段 C / PR-C：schema/sqlc/错误码/测试/文档封板收口：`docs/dev-plans/391c-phase-c-sealing-sweep-for-schema-sqlc-errors-tests-and-docs.md`
- DEV-PLAN-410：基于实体授权的配置差异化设计原则：`docs/dev-plans/410-entity-authorization-based-configuration-differentiation-principle.md`
- DEV-PLAN-411：基于实体授权差异化配置的 UI 配置模式：`docs/dev-plans/411-ui-configuration-pattern-for-entity-based-differentiated-config.md`
- DEV-PLAN-420：规则函数框架与客户自定义函数方案：`docs/dev-plans/420-rule-function-framework-and-custom-functions-plan.md`
- DEV-PLAN-430：IDE 式对话助手重做架构方案：`docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- DEV-PLAN-431：Codex UI 协议、状态机与右悬挂壳层复用/重构方案：`docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- DEV-PLAN-431A：CubeBox 页面设计契约（承接 DEV-PLAN-431）：`docs/dev-plans/431a-cubebox-page-design-contract.md`
- DEV-PLAN-432：Codex 会话持久化、索引与恢复语义复用/重构方案：`docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- DEV-PLAN-433：Bifrost 主参考的 AI 网关复用/重构方案：`docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- DEV-PLAN-434：Codex 上下文管理与压缩机制复用/重构方案：`docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- DEV-PLAN-435：Bifrost 主参考的模型配置 UI 与管理权限复用/重构方案：`docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`
- DEV-PLAN-436：CubeBox 历史对话面彻底删除与仓面清场方案：`docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`
- DEV-PLAN-440：彻底删除 SetID 的全仓收口方案（SetID 根删除唯一 PoR）：`docs/dev-plans/440-complete-setid-removal-plan.md`
- DEV-PLAN-441：旧策略模块残余清理方案：`docs/dev-plans/441-legacy-strategy-module-residue-cleanup-plan.md`
- DEV-PLAN-450：直接切除 jobcatalog / staffing / person 三模块方案（保留 orgunit）：`docs/dev-plans/450-direct-removal-of-jobcatalog-staffing-person-modules-plan.md`
- DEV-PLAN-440 Readiness：当前命中、停止线与分阶段 owner：`docs/dev-records/DEV-PLAN-440-READINESS.md`
- SetID 相关历史研究/中间方案说明：`070A`、`102C*`、`015Z*`、`161A`、`163A`、`185`、`191`、`203` 等文档仅可作为 archive 历史来源或调查记录引用；凡涉及 SetID 根删除、入口是否保留、现行主流程是否仍依赖 SetID，一律以 `DEV-PLAN-440` 为准。
- DEV-PLAN-400：CodeFlow 辅助源码分析与爆炸半径评估落地方案：`docs/dev-plans/400-codeflow-assisted-source-analysis-and-impact-radius-plan.md`
- DEV-PLAN-069【归档 / 历史删除方案】：移除薪酬社保与考勤（文档/代码/测试/数据库）：`docs/archive/dev-plans/069-remove-payroll-attendance.md`
- DEV-PLAN-070【归档】：SetID 绑定组织架构重构方案（时间口径已由 DEV-PLAN-102B 接管）：`docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- DEV-PLAN-070A【归档 / 历史来源】：全局共享租户模式 vs 天然租户隔离模式专项调查（SetID/Scope Package，不作为现行实现依据）：`docs/archive/dev-plans/070a-setid-global-share-vs-tenant-native-isolation-investigation.md`
- DEV-PLAN-070B：取消共享租户（global_tenant）并收敛为租户本地发布方案（以字典配置模块为样板）：`docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- DEV-PLAN-070B1：字典基线发布 UI 可视化操作方案（承接 DEV-PLAN-070B）：`docs/dev-plans/070b1-dict-release-ui-operations-plan.md`
- DEV-PLAN-070B-T：070B 系列目标达成测试方案（字典租户本地发布）：`docs/dev-plans/070b-t-dict-tenant-release-test-plan.md`
- DEV-PLAN-071【归档】：SetID Scope Package 订阅蓝图（时间口径已由 DEV-PLAN-102B 接管）：`docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- DEV-PLAN-071A【归档】：基于 Package 的配置编辑与订阅显式化：`docs/archive/dev-plans/071a-package-selection-ownership-and-subscription.md`
- DEV-PLAN-071B【归档】：字段配置/字典配置与 SetID 边界实施方案：`docs/archive/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- DEV-PLAN-072【归档】：全仓 ID/Code 命名与对外标识收敛（规范已并入 `DEV-PLAN-005/STD-003`）：`docs/archive/dev-plans/072-repo-wide-id-code-naming-convergence.md`
- DEV-PLAN-073：OrgUnit CRUD 实现清单（页面与 API）：`docs/dev-plans/073-orgunit-crud-implementation-status.md`
- DEV-PLAN-073A【归档】：组织架构树运行态问题记录（Shoelace 资源加载失败）：`docs/archive/dev-plans/073a-orgunit-tree-runtime-issue.md`
- DEV-PLAN-074：OrgUnit Details 集成更新能力与 UI 优化方案：`docs/dev-plans/074-orgunit-details-update-ui-optimization.md`
- DEV-PLAN-075：OrgUnit 生效日期不允许回溯的限制评估：`docs/dev-plans/075-orgunit-effective-date-backdating-assessment.md`
- DEV-PLAN-075A【归档】：OrgUnit 记录新增/插入 UI 与可编辑字段问题记录：`docs/archive/dev-plans/075a-orgunit-records-ui-and-editing-issues.md`
- DEV-PLAN-075B：Root Unit A 生效日回溯可行性调查与修复方案：`docs/dev-plans/075b-orgunit-root-backdating-feasibility-and-fix-plan.md`
- DEV-PLAN-075C：OrgUnit 删除记录/停用语义混用调查与收敛方案：`docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- DEV-PLAN-075D【归档】：OrgUnit 页面状态字段与有效/无效显式切换：`docs/archive/dev-plans/075d-orgunit-status-field-active-inactive-selector.md`
- DEV-PLAN-075E【归档】：OrgUnit 同日状态修正（生效日不变）方案（模块标准已并入 `DEV-PLAN-108`）：`docs/archive/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
- DEV-PLAN-076：OrgUnit 版本切换导致选中组织丢失问题与修复方案：`docs/dev-plans/076-orgunit-version-switch-selection-retention.md`
- DEV-PLAN-077：OrgUnit Replay 写放大评估与收敛方案：`docs/dev-plans/077-orgunit-replay-write-amplification-assessment-and-mitigation.md`
- DEV-PLAN-078：OrgUnit 写模型替代方案对比与决策建议：`docs/dev-plans/078-orgunit-write-model-alternatives-comparison-and-decision.md`
- DEV-PLAN-080：OrgUnit 审计链收敛（方向 1：单一审计链）：`docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
- DEV-PLAN-080A：OrgUnit before_snapshot/after_snapshot 机制调查与收敛修复方案：`docs/dev-plans/080a-orgunit-audit-snapshot-mechanism-and-fix.md`
- DEV-PLAN-080B【归档】：OrgUnit 生效日更正失败（orgunit_correct_failed）专项调查与修复方案（错误码提取规范已并入 `DEV-PLAN-111`）：`docs/archive/dev-plans/080b-orgunit-correction-failure-investigation-and-remediation.md`
- DEV-PLAN-080C：OrgUnit 审计快照 presence 表级强约束（INSERT 即写齐）方案：`docs/dev-plans/080c-orgunit-audit-snapshot-presence-table-constraint-plan.md`
- DEV-PLAN-080D【归档】：OrgUnit 变更日志“已撤销事件未标识”专项调查与收敛方案（审计可读性契约已并入 `DEV-PLAN-080`）：`docs/archive/dev-plans/080d-orgunit-audit-rescinded-event-visibility-investigation.md`
- DEV-PLAN-081：OrgUnit Details 记录版本选择器双栏化（左生效日期 / 右详情）：`docs/dev-plans/081-orgunit-records-version-selector-two-pane-alignment.md`
- DEV-PLAN-082【归档】：Org 模块业务字段修改规则全量调查（排除元数据）：`docs/archive/dev-plans/082-org-module-field-mutation-rules-investigation.md`
- DEV-PLAN-083【归档】：Org 白名单模型扩展性改造（规则单点化 + 能力矩阵外显）：`docs/archive/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- DEV-PLAN-083A【归档】：OrgUnit Append 写入动作能力外显与策略单点扩展（create / event_update）：`docs/archive/dev-plans/083a-orgunit-append-actions-capabilities-policy-extension.md`
- DEV-PLAN-083B：Org 变更能力模型后置收口（承接 083/083A）：`docs/dev-plans/083b-org-mutation-capabilities-post-083a-closure-plan.md`
- DEV-PLAN-084【归档】：Org 模块组织树“下级可展开指示符”缺失问题分析与收敛方案（树可展开契约已并入 `DEV-PLAN-073`）：`docs/archive/dev-plans/084-orgunit-tree-expand-indicator-visibility.md`
- DEV-PLAN-090：前端框架升级为 MUI X（对标 Workday UX）方案：`docs/dev-plans/090-mui-x-frontend-upgrade-plan.md`
- DEV-PLAN-091【归档】：MUI X 升级子计划 P0（基座准备与许可落地，阶段交付已完成）：`docs/archive/dev-plans/091-mui-x-phase0-foundation-and-license-plan.md`
- DEV-PLAN-092【归档】：MUI X 升级子计划 P1（壳与导航迁移，阶段交付已完成）：`docs/archive/dev-plans/092-mui-x-phase1-shell-navigation-plan.md`
- DEV-PLAN-093【归档】：MUI X 升级子计划 P2（高价值模块迁移；阶段交付完成并封板）：`docs/archive/dev-plans/093-mui-x-phase2-high-value-modules-plan.md`
- DEV-PLAN-094【归档】：MUI X 升级子计划 P3（长尾迁移与收口；历史阶段文档含旧路径口径，现行规范以 `DEV-PLAN-005/STD-005` 为准）：`docs/archive/dev-plans/094-mui-x-phase3-long-tail-convergence-plan.md`
- DEV-PLAN-095【归档】：MUI X 升级子计划 P4（稳定化与性能压测；阶段收尾后不再单列实施）：`docs/archive/dev-plans/095-mui-x-phase4-stability-performance-plan.md`
- DEV-PLAN-096【归档】：Org 模块全量迁移至 MUI X 与统一体验收口方案（阶段收口并封板）：`docs/archive/dev-plans/096-org-module-full-migration-and-ux-convergence-plan.md`
- DEV-PLAN-097【归档】：OrgUnit 详情从抽屉（Drawer）迁移为独立页面（page pattern 已沉淀为历史记录）：`docs/archive/dev-plans/097-orgunit-details-drawer-to-page-migration.md`
- DEV-PLAN-098【归档】：组织架构模块架构评估——多类型宽表预留字段 + 元数据驱动（实施已由 `DEV-PLAN-100` 承接）：`docs/archive/dev-plans/098-org-module-wide-table-metadata-driven-architecture-assessment.md`
- DEV-PLAN-099【归档】：OrgUnit 信息页双栏化（左生效日期/修改时间，右侧详情；口径已并入 `DEV-PLAN-096`）：`docs/archive/dev-plans/099-orgunit-details-two-pane-info-audit-mui.md`
- DEV-PLAN-100：Org 模块宽表预留字段 + 元数据驱动落地实施计划与路线图（承接 DEV-PLAN-098）：`docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- DEV-PLAN-100A：Org 模块宽表元数据落地 Phase 0：契约冻结与就绪检查：`docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`
- DEV-PLAN-100B：Org 模块宽表元数据落地 Phase 1：Schema 与元数据骨架（最小数据库闭环）：`docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`
- DEV-PLAN-100C：Org 模块宽表元数据落地 Phase 2：Kernel/Projection 扩展（保持 One Door）：`docs/dev-plans/100c-org-metadata-wide-table-phase2-kernel-projection-extension-one-door.md`
- DEV-PLAN-100D：Org 模块宽表元数据落地 Phase 3：服务层与 API（读写可用）：`docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- DEV-PLAN-100D2：Org 模块宽表元数据落地 Phase 3 修订：契约对齐与 API 实现收口（为 100E/101 做准备）：`docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
- DEV-PLAN-100E：Org 模块宽表元数据落地 Phase 4A：OrgUnit 详情页扩展字段展示与 Capabilities 驱动编辑（MUI）：`docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- DEV-PLAN-100E1：OrgUnit Mutation Policy 单点化 + 更正链路支持 `patch.ext`（作为 DEV-PLAN-100E 前置）：`docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`
- DEV-PLAN-100G：Org 模块宽表元数据落地 Phase 4C：OrgUnit 列表扩展字段筛选/排序 + i18n 收口（闭环收口，MUI）：`docs/dev-plans/100g-org-metadata-wide-table-phase4c-orgunits-list-ext-query-i18n-closure.md`
- DEV-PLAN-100H：Org 模块宽表元数据落地 Phase 5：稳定性/性能/异常与运维收口：`docs/dev-plans/100h-org-metadata-wide-table-phase5-stability-performance-ops-closure.md`
- DEV-PLAN-101【归档】：OrgUnit 字段配置管理页（MUI）IA 与组件级方案（承接 DEV-PLAN-100）：`docs/archive/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- DEV-PLAN-101B【归档】：OrgUnit PLAIN 扩展字段编辑能力收敛（新建/插入记录/修正）：`docs/archive/dev-plans/101b-orgunit-plain-ext-fields-editability-convergence.md`
- DEV-PLAN-101I【归档】：OrgUnit 生效日期记录新增/插入（MUI）操作口径与约束说明：`docs/archive/dev-plans/101i-orgunit-effective-date-record-add-insert-ui-and-constraints.md`
- DEV-PLAN-105：全模块字典配置模块（DICT 值配置 + 生效日期 + 变更记录）：`docs/dev-plans/105-dict-config-platform-module.md`
- DEV-PLAN-105A：字典配置模块验证问题调查与修复方案（承接 DEV-PLAN-105）：`docs/dev-plans/105a-dict-config-validation-issues-investigation.md`
- DEV-PLAN-105B：Dict Code（字典本体）新增与治理方案（承接 DEV-PLAN-105/105A）：`docs/dev-plans/105b-dict-code-management-and-governance.md`
- DEV-PLAN-106：Org 模块扩展字段启用方式改造（DICT 全量引用 + 自定义 PLAIN 字段）：`docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md`
- DEV-PLAN-106A：Org 扩展字段启用增强（字典字段作为 Field Key + 启用时自定义描述）：`docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md`
- DEV-PLAN-106B：OrgUnit 更正语义收敛（生效日更正粘性 + 后续更正兼容，根因修复）：`docs/dev-plans/106b-orgunit-corrections-effective-date-sticky-semantics.md`
- DEV-PLAN-107：OrgUnit 扩展字段槽位扩容（总计 135 槽；按类型合理分布；新增 numeric）：`docs/dev-plans/107-orgunit-ext-field-slots-expand-to-100.md`
- DEV-PLAN-108：Org 模块 CRUD UI 按钮整合与统一字段变更规则（用户操作视角）：`docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- DEV-PLAN-108A：Org 新建组织弹窗支持 DICT 扩展字段（下拉选择）：`docs/dev-plans/108a-org-create-dialog-dict-ext-fields-support.md`
- DEV-PLAN-108B：Org 新建组织弹窗 DICT 扩展字段实现（承接 108A）：`docs/dev-plans/108b-org-create-dialog-dict-ext-fields-implementation.md`
- DEV-PLAN-109【归档】：Org 模块幂等命名收敛与门禁（历史阶段封板，按 STD-001 修订）：`docs/archive/dev-plans/109-request-code-unification-and-gate.md`
- DEV-PLAN-109A：`request_id`（幂等）+ `trace_id`（Tracing）全仓收敛与防扩散：`docs/dev-plans/109a-request-code-total-convergence-and-anti-drift.md`
- DEV-PLAN-110：启用字段表单增强：自定义（直接值）+ 值类型选择 + 自定义字段名称：`docs/dev-plans/110-orgunit-field-configs-custom-direct-value-form.md`
- DEV-PLAN-111：前端错误信息准确化与字段级提示收敛方案：`docs/dev-plans/111-frontend-error-message-accuracy-and-field-level-hints.md`
- DEV-PLAN-120：Org 字段默认值（Go+PG+CEL）规则引擎落地路线图：`docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`
- DEV-PLAN-126：Go 1.26 升级与现代化改造计划（No-Compat）：`docs/dev-plans/126-go-1-26-upgrade-and-modernization-plan.md`
- DEV-PLAN-130：Org 组织树初始化问题收敛与自举修复方案：`docs/dev-plans/130-orgunit-tree-initialization-recovery-and-bootstrap.md`
- DEV-PLAN-140：全仓错误提示明确化与质量门禁：`docs/dev-plans/140-error-message-clarity-and-gates.md`
- DEV-PLAN-150 ~ DEV-PLAN-160【归档】：Capability Key / Functional Area / Policy Activation 主链文档已由 `DEV-PLAN-450` 宣告退役；现仅作为仓内历史来源保留，不再作为现行实现依据：`docs/archive/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md` ~ `docs/archive/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- DEV-PLAN-161【历史来源】：Org 新建表单动态策略落地（Capability 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- DEV-PLAN-161A【归档 / 历史来源】：SetID Capability Registry 可编辑与可维护化（Capability 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/161a-setid-capability-registry-editable-and-maintainable.md`
- DEV-PLAN-162：OrgUnit 新增版本后组织类型回退为“单位”问题调查：`docs/dev-plans/162-orgunit-add-version-dict-ext-not-applied-investigation.md`
- DEV-PLAN-163【归档 / 历史来源】：Capability Key 表单字段下拉化收敛方案（Strategy Registry 历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/163-capability-key-form-dropdown-convergence.md`
- DEV-PLAN-163A【归档 / 历史来源】：SetID Governance 其余三页签字段下拉化收敛方案：`docs/archive/dev-plans/163a-setid-governance-other-tabs-dropdown-convergence.md`
- DEV-PLAN-164：组织类型策略控制范围与继承缺口分析（OrgType）：`docs/dev-plans/164-org-type-policy-control-gap-analysis.md`
- DEV-PLAN-165【归档 / 历史来源】：字段配置页与 Strategy capability_key 的对应关系调查与页面定位重评（Capability 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- DEV-PLAN-170【归档 / 历史来源】：Org 详情页 UI 外观对齐 Capability Key（Capability 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/170-org-form-ui-shell-alignment-with-capability-key.md`
- DEV-PLAN-170A【归档 / 历史来源】：Org 变更日志页 UI 外观对齐 Capability Key（Capability 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/170a-org-audit-log-ui-shell-alignment-with-capability-key.md`
- DEV-PLAN-170B：Org 详情页移除顶部上下文区与 URL 恢复定位替代方案（170A 纠偏计划）：`docs/dev-plans/170b-org-details-remove-top-context-and-url-restore-positioning.md`
- DEV-PLAN-180：项目颗粒度层次统一与治理（Field/Form/Module/SetID/Tenant/Server）：`docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md`
- DEV-PLAN-181【归档 / 历史来源】：OrgUnit Details 三类表单到 Capability Key 映射落地（Capability 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/181-orgunit-details-form-capability-mapping-implementation.md`
- DEV-PLAN-182【归档 / 历史来源】：BU 策略“全 CRUD 默认生效”与场景覆盖收敛方案（Capability/SetID 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/182-bu-policy-baseline-and-intent-override-unification.md`
- DEV-PLAN-183【归档 / 历史来源】：Capability Key 配置可发现性与对象/意图显式建模方案（Capability 主链历史方案；现行删除 owner 见 `DEV-PLAN-450`）：`docs/archive/dev-plans/183-capability-key-object-intent-discoverability-and-modeling.md`
- DEV-PLAN-184：字段配置与策略规则双层 SoT 收敛方案（Static Metadata vs Dynamic Policy）：`docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md`
- DEV-PLAN-185【归档 / 历史来源】：字段配置页字典值列表 SetID 列展示与主数据取数控制策略收敛：`docs/archive/dev-plans/185-field-config-dict-values-setid-column-and-master-data-fetch-control.md`
- DEV-PLAN-191【归档 / 历史来源】：`/app/org/setid` 导航与页面设计优化方案（历史页面方案；现行入口删除 owner 见 `DEV-PLAN-440`）：`docs/archive/dev-plans/191-setid-governance-navigation-and-layout-optimization.md`
- DEV-PLAN-200【归档 / 历史来源】：组合优先的积木式页面与功能架构蓝图（Field Config × Dict × CRUD Pattern × Strategy）：`docs/archive/dev-plans/200-composable-building-block-architecture-blueprint.md`
- DEV-PLAN-201【归档 / 历史来源】：200蓝图 Phase 0 边界冻结与跨层作用域一致性基线：`docs/archive/dev-plans/201-blueprint-phase0-boundary-and-scope-consistency-freeze.md`
- DEV-PLAN-202【归档 / 历史来源】：200蓝图 Phase 0 策略决议确定性与 allowed_value_codes 语义收敛：`docs/archive/dev-plans/202-blueprint-policy-resolution-and-allowed-values-determinism.md`
- DEV-PLAN-203【归档 / 历史来源】：200蓝图 Phase 1 运行时读路径（映射注册表 + SetID 硬前置）：`docs/archive/dev-plans/203-blueprint-runtime-read-path-mapping-and-setid-preresolve.md`
- DEV-PLAN-204【归档 / 历史来源】：200蓝图 Phase 1 组合 DTO、Explain 与版本快照协议：`docs/archive/dev-plans/204-blueprint-composition-dto-and-explain-versioning.md`
- DEV-PLAN-205【归档 / 历史来源】：200蓝图 Phase 1 页面职责收敛（Static Metadata × Dynamic Policy）：`docs/archive/dev-plans/205-blueprint-page-responsibility-convergence-static-dynamic-sot.md`
- DEV-PLAN-206【归档 / 历史来源】：200蓝图 Phase 2 CRUD 模板统一与双版本提交收口：`docs/archive/dev-plans/206-blueprint-crud-template-and-double-version-submit-cutover.md`
- DEV-PLAN-207【归档 / 历史来源】：200蓝图 Phase 2 性能停止线与反 N+1 门禁收口：`docs/archive/dev-plans/207-blueprint-performance-gates-and-n-plus-one-prevention.md`
- DEV-PLAN-208【归档 / 历史来源】：200蓝图 Phase 3 Req2Config 只读编排与严格结构化输出：`docs/archive/dev-plans/208-blueprint-req2config-readonly-and-strict-decode.md`
- DEV-PLAN-209【归档 / 历史来源】：200蓝图 Phase 3 Skill 契约化与工具白名单治理：`docs/archive/dev-plans/209-blueprint-skill-manifest-tool-whitelist-and-risk-tier.md`
- DEV-PLAN-210【归档 / 历史来源】：200蓝图 Phase 4 会话事务提交与委托授权同构收口：`docs/archive/dev-plans/210-blueprint-conversation-transaction-and-actor-delegated-authz.md`
- DEV-PLAN-211【归档 / 历史来源】：200蓝图 Phase 5 自建 Temporal M10D0 最小化落地：`docs/archive/dev-plans/211-blueprint-temporal-m10d0-minimal-orchestration-foundation.md`
- DEV-PLAN-212【归档 / 历史来源】：200蓝图 Phase 6 评测门禁与触发式 Temporal 平台化验收：`docs/archive/dev-plans/212-blueprint-eval-gates-and-triggered-temporal-productionization.md`
- Assistant / LibreChat / 旧 CubeBox `220-383` 系列与 `380A-380G` 子计划：已完成历史归档治理并迁入 `docs/archive/dev-plans/`；相关执行记录已按落地情况迁入 `docs/archive/dev-records/`。这些文档仅保留为历史证据，不再作为现行实现前提、编排入口或完成定义；当前对话助手重做主线请改看 `DEV-PLAN-430`、`DEV-PLAN-431`、`DEV-PLAN-431A`、`DEV-PLAN-432`、`DEV-PLAN-433`、`DEV-PLAN-434`、`DEV-PLAN-435`。
- DEV-PLAN-300：全仓测试体系问题调查记录：`docs/dev-plans/300-test-system-investigation-report.md`
- DEV-PLAN-301【归档 / 历史来源】：Go 测试分层整治与官方最佳实践落地方案（首轮分层整改历史方案）：`docs/archive/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- DEV-PLAN-302【归档 / 历史来源】：`internal/server` 残留 `gap/coverage` 测试文件收口计划（首轮尾项清零历史方案）：`docs/archive/dev-plans/302-internal-server-residual-gap-coverage-closure-plan.md`
- DEV-PLAN-330【归档 / 历史来源】：策略模块架构混乱调查与收口方案（旧策略模块历史架构调查入口；现行残余清理以 `DEV-PLAN-441`、SetID 根删除以 `DEV-PLAN-440`、三模块/Capability 主链删除以 `DEV-PLAN-450` 为准）：`docs/archive/dev-plans/330-strategy-module-architecture-and-design-convergence-plan.md`
- DEV-PLAN-303：全仓残留 `gap/coverage` 测试尾项清零计划：`docs/dev-plans/303-repo-final-gap-coverage-test-tail-closure-plan.md`
- DEV-PLAN-310：全项目 view/as_of 时间语义专项检视与最小收敛方案：`docs/dev-plans/310-project-wide-view-as-of-semantics-review-and-minimal-convergence-plan.md`
- DEV-PLAN-311【归档 / 历史来源】：View As Of 页面改造矩阵与 OrgUnitDetails 样板实施计划（含已删除页面的历史矩阵；现行页面边界以当前活体模块为准）：`docs/archive/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- DEV-PLAN-312：View As Of 收口实施计划——详情页单历史锚点与 A 类页面读写解耦：`docs/dev-plans/312-view-as-of-implementation-plan-details-single-history-anchor-and-a-pages-read-write-decoupling.md`
- DEV-PLAN-313【归档 / 历史来源】：View As Of 后端并行收口计划——显式日期契约、无 fallback、统一错误语义（含已删除接口的历史收口记录）：`docs/archive/dev-plans/313-view-as-of-backend-parallel-convergence-plan-explicit-date-contract-and-no-fallback.md`
- DEV-PLAN-314【归档 / 历史来源】：View As Of P1 页面批量收口计划——Assignments / Positions / JobCatalog / DictConfigs：`docs/archive/dev-plans/314-view-as-of-p1-pages-batch-cutover-plan-assignments-positions-jobcatalog-dicts.md`
- DEV-PLAN-315：View As Of 最小 helper 与反回流门禁计划：`docs/dev-plans/315-view-as-of-minimal-helper-and-anti-regression-gates-plan.md`
- DEV-PLAN-316【归档 / 历史来源】：View As Of 工具态页面收口计划——Explain / Release / Governance 子区统一任务态时间语义（含已删除页面/工具区的历史收口记录）：`docs/archive/dev-plans/316-view-as-of-tooling-pages-convergence-plan.md`
- DEV-PLAN-317：View As Of 页面时间语义回归与验收计划：`docs/dev-plans/317-view-as-of-regression-and-acceptance-plan.md`
- DEV-PLAN-320：Org 域 8 位非纯数字 `org_node_key` 一步切换方案（不扩大到全对象）：`docs/dev-plans/320-org-node-key-cutover-plan-no-global-expansion.md`
- DEV-PLAN-102【归档】：全项目 as_of 时间上下文收敛与批判（承接 DEV-PLAN-076，现行口径以 `DEV-PLAN-102B`/`STD-002` 为准）：`docs/archive/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- DEV-PLAN-102A【归档】：Org Code 默认规则“保存后无变化”生效日错位调查与收敛方案（表达式口径一致性已并入 `DEV-PLAN-120`）：`docs/archive/dev-plans/102a-org-code-default-policy-effective-date-visibility-fix.md`
- DEV-PLAN-102B：070/071 时间口径强制显式化与历史回放稳定性收敛：`docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- DEV-PLAN-102C【归档 / 历史来源】：SetID 对标 Workday 的集团共享与业务单元个性化差距评估（承接 102/102B）：`docs/archive/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- DEV-PLAN-102C1【归档 / 历史来源】：SetID 上下文化安全模型（承接 102C，避免与 070B 重复）：`docs/archive/dev-plans/102c1-setid-contextual-security-model.md`
- DEV-PLAN-102C2【归档 / 历史来源】：BU 个性化策略注册表（承接 102C，避免与 070B/102C1 重复）：`docs/archive/dev-plans/102c2-bu-personalization-strategy-registry.md`
- DEV-PLAN-102C3【归档 / 历史来源】：SetID 配置命中可解释性（Explainability）方案（承接 102C，避免与 070B/102C1/102C2 重复）：`docs/archive/dev-plans/102c3-setid-configuration-hit-explainability.md`
- DEV-PLAN-102C4【归档 / 历史来源】：BU 流程个性化样板（承接 102C，避免与 070B/102C1/102C2/102C3 重复）：`docs/archive/dev-plans/102c4-bu-process-personalization-pilot.md`
- DEV-PLAN-102C5【归档 / 历史来源】：102C1-102C3 UI 专项方案（SetID 上下文化安全 + 策略注册表 + 命中解释）：`docs/archive/dev-plans/102c5-ui-design-for-setid-context-security-registry-explainability.md`
- DEV-PLAN-102C6【归档 / 历史来源】：彻底删除 scope_code + package，收敛到 capability_key + setid（历史阶段方案；Capability 主链已退役，凡涉现行删除排序以 `DEV-PLAN-440/450` 为准）：`docs/archive/dev-plans/102c6-remove-scope-code-and-converge-to-capability-key-plan.md`
- DEV-PLAN-102C-T【归档 / 历史来源】：102C1-102C3 测试方案（同租户跨 BU 字段差异）：`docs/archive/dev-plans/102c-t-test-plan-for-c1-c3-bu-field-variance.md`
- DEV-PLAN-102D：基于 102 基线的 Context + Rule + Eval 动态隔离与配置安全实施方案：`docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md`
- DEV-PLAN-102D-T：102D 动态规则引擎测试方案（用户可见性 + 内部评估链路）：`docs/dev-plans/102d-t-context-rule-eval-user-visible-test-plan.md`
- DEV-PLAN-103【归档】：移除旧前端链路，前端收敛为 MUI X（React SPA；规范已并入 `DEV-PLAN-005/STD-005/STD-006`）：`docs/archive/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`
- DEV-PLAN-103A【归档】：DEV-PLAN-103 收尾（P3 业务页闭环 + P6 工程改名：去技术后缀）：`docs/archive/dev-plans/103a-dev-plan-103-closure-p3-p6-apps-web-rename.md`
- DEV-PLAN-104【归档 / 历史来源】：Job Catalog（职位分类）页面 UI 优化方案（信息架构收敛：上下文工具条 + Tabs + DataGrid + Dialog）：`docs/archive/dev-plans/104-jobcatalog-ui-optimization.md`
- DEV-PLAN-104A【归档 / 历史来源】：Job Catalog UI 优化补充修订（对齐 DEV-PLAN-002）：`docs/archive/dev-plans/104a-jobcatalog-ui-optimization-alignment-with-dev-plan-002.md`
- DEV-PLAN-026A【归档】：OrgUnit 8位编号与 UUID/Code 命名规范：`docs/archive/dev-plans/026a-orgunit-id-uuid-code-naming.md`
- DEV-PLAN-026B【归档】：OrgUnit 外部ID兼容（org_code 映射）方案：`docs/archive/dev-plans/026b-orgunit-external-id-code-mapping.md`
- DEV-PLAN-026C【归档】：OrgUnit 外部ID兼容（org_code 映射）评审与修订方案：`docs/archive/dev-plans/026c-orgunit-external-id-code-mapping-review-and-revision.md`
- DEV-PLAN-026D【归档】：OrgUnit 增量投射方案（减少全量回放写放大）：`docs/archive/dev-plans/026d-orgunit-incremental-projection-plan.md`
- P0 前置条件实施方案（契约优先）：`docs/dev-plans/010-p0-prerequisites-contract.md`
- AI 驱动开发评审清单（Simple > Easy）：`docs/dev-plans/003-simple-not-easy-review-guide.md`
- Org（事务性事件溯源 + 同步投射，已归档）：`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- Position（事务性事件溯源 + 同步投射，已归档）：`docs/archive/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
- Job Catalog（事务性事件溯源 + 同步投射，已归档）：`docs/archive/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- PostgreSQL RLS 强租户隔离【归档 / 历史合同】：`docs/archive/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- DDD 分层框架（对齐 CleanArchGuard + DB Kernel）：`docs/dev-plans/015-ddd-layering-framework.md`
- DEV-PLAN-015A：DDD 分层框架履职缺口评估（承接 DEV-PLAN-015）：`docs/dev-plans/015a-ddd-layering-framework-implementation-gap-assessment.md`
- DEV-PLAN-015B：DDD 分层框架收口整改路线图（P0/P1/P2，承接 DEV-PLAN-015A）：`docs/dev-plans/015b-ddd-layering-framework-remediation-roadmap.md`
- DEV-PLAN-015C：DDD 分层框架 P0 反漂移门禁实施计划（承接 DEV-PLAN-015B）：`docs/dev-plans/015c-ddd-layering-framework-p0-anti-drift-gate-plan.md`
- DEV-PLAN-015D【归档 / 历史来源】：Staffing Assignment 分层回流修复（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015d-staffing-assignment-layering-reversal-fix-plan.md`
- DEV-PLAN-015E【归档 / 历史来源】：Person 默认装配向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015e-person-default-wiring-module-side-plan.md`
- DEV-PLAN-015F【归档 / 历史来源】：Person 模块 Composition Root 最小化落地（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015f-person-module-composition-root-minimalization-plan.md`
- DEV-PLAN-015G【归档 / 历史来源】：JobCatalog 内存 Store 向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015g-jobcatalog-memory-store-module-side-plan.md`
- DEV-PLAN-015H【归档 / 历史来源】：JobCatalog PG Store 向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015h-jobcatalog-pg-store-module-side-plan.md`
- DEV-PLAN-015I【归档 / 历史来源】：Staffing Assignment 组合根最小化收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015i-staffing-assignment-composition-root-plan.md`
- DEV-PLAN-015J【归档 / 历史来源】：JobCatalog Server 侧冗余构造包装消除（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015j-jobcatalog-server-constructor-elimination-plan.md`
- DEV-PLAN-015K【归档 / 历史来源】：Person Server 侧冗余构造包装消除（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015k-person-server-constructor-elimination-plan.md`
- DEV-PLAN-015L【归档 / 历史来源】：JobCatalog Test-Only Wrapper 从生产代码移除（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015l-jobcatalog-test-only-wrapper-elimination-plan.md`
- DEV-PLAN-015M【归档 / 历史来源】：JobCatalog Normalize Wrapper 从生产代码移除（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015m-jobcatalog-normalize-wrapper-elimination-plan.md`
- DEV-PLAN-015N【归档 / 历史来源】：Person Normalize Wrapper 从生产代码移除（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015n-person-normalize-wrapper-elimination-plan.md`
- DEV-PLAN-015O【归档 / 历史来源】：Staffing Assignment 默认 PG 装配向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015o-staffing-assignment-pg-default-wiring-plan.md`
- DEV-PLAN-015P【归档 / 历史来源】：Staffing Assignment 默认 Memory 装配向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015p-staffing-assignment-memory-default-wiring-plan.md`
- DEV-PLAN-015Q【归档 / 历史来源】：Staffing Assignment 内存实现向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015q-staffing-assignment-memory-implementation-module-side-plan.md`
- DEV-PLAN-015R【归档 / 历史来源】：Staffing Position 领域类型与 Port 契约前移到模块侧（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015r-staffing-position-domain-contract-plan.md`
- DEV-PLAN-015S【归档 / 历史来源】：Staffing Position 默认 Memory 装配向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015s-staffing-position-memory-default-wiring-plan.md`
- DEV-PLAN-015T【归档 / 历史来源】：Staffing Position 内存实现向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015t-staffing-position-memory-implementation-module-side-plan.md`
- DEV-PLAN-015U【归档 / 历史来源】：Staffing Memory 兼容壳移出生产代码（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015u-staffing-memory-compatibility-shell-test-only-plan.md`
- DEV-PLAN-015V【归档 / 历史来源】：Staffing PG Assignment 薄委派移出生产代码（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015v-staffing-pg-assignment-wrapper-test-only-plan.md`
- DEV-PLAN-015W【归档 / 历史来源】：Staffing Position PG 实现与默认装配向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015w-staffing-position-pg-module-side-wiring-plan.md`
- DEV-PLAN-015X：OrgUnit Write Service 默认装配向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/dev-plans/015x-orgunit-write-service-module-wiring-plan.md`
- DEV-PLAN-015Y【归档 / 历史来源】：JobCatalog 视图服务适配入口向模块侧收口（承接 DEV-PLAN-015B P1）：`docs/archive/dev-plans/015y-jobcatalog-view-service-adapter-module-side-plan.md`
- DEV-PLAN-015Z：DDD 分层框架收尾盘点与封板清单（承接 DEV-PLAN-015B）：`docs/dev-plans/015z-ddd-layering-framework-closure-summary-and-backlog.md`
- DEV-PLAN-015Z1【归档 / 历史来源】：OrgUnit SetID Memory Store 向模块侧收口（承接 DEV-PLAN-015Z）：`docs/archive/dev-plans/015z1-orgunit-setid-memory-module-side-plan.md`
- DEV-PLAN-015Z2【归档 / 历史来源】：OrgUnit SetID PG 默认装配入口向模块侧收口（承接 DEV-PLAN-015Z1）：`docs/archive/dev-plans/015z2-orgunit-setid-pg-module-entry-plan.md`
- DEV-PLAN-015Z3【归档 / 历史来源】：OrgUnit SetID PG 实现向模块侧收缩为 Server 薄壳（承接 DEV-PLAN-015Z2）：`docs/archive/dev-plans/015z3-orgunit-setid-pg-server-thin-shell-plan.md`
- DEV-PLAN-015Z4：DDD 分层 P2 组合根门禁封板（承接 DEV-PLAN-015Z）：`docs/dev-plans/015z4-ddd-layering-p2-gate-closure-plan.md`
- DEV-PLAN-015Z5：IAM Dict Store 向模块侧收缩为 Server 薄壳（承接 DEV-PLAN-015Z）：`docs/dev-plans/015z5-iam-dict-server-thin-shell-plan.md`
- DEV-PLAN-015Z6【归档 / 历史来源】：OrgUnit SetID Store 向模块侧收缩为 Server 薄壳（承接 DEV-PLAN-015Z）：`docs/archive/dev-plans/015z6-orgunit-setid-server-thin-shell-plan.md`
- DEV-PLAN-015AA：IAM Dict Store 向模块侧收口（承接 DEV-PLAN-015Z）：`docs/dev-plans/015aa-iam-dict-store-module-side-plan.md`
- Greenfield HR 模块骨架与契约（已归档，OrgUnit/JobCatalog/Staffing/Person 历史骨架）：`docs/archive/dev-plans/016-greenfield-hr-modules-skeleton.md`
- 任职记录（Job Data / Assignments，已归档）：`docs/archive/dev-plans/031-greenfield-assignment-job-data.md`
- Person 最小身份锚点（Pernr 1-8 位数字字符串，已归档）：`docs/archive/dev-plans/027-person-minimal-identity-for-staffing.md`
- 引入 Astro（AHA Stack）到 HRMS UI（历史，已被 DEV-PLAN-103 替代，已归档）：`docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- 技术栈与工具链版本冻结：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- 租户管理与登录认证：`docs/dev-plans/019-tenant-and-authn.md`
- SuperAdmin 控制面认证与会话：`docs/dev-plans/023-superadmin-authn.md`
- 多语言（仅 en/zh）：`docs/dev-plans/020-i18n-en-zh-only.md`
- Atlas + Goose 闭环指引：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 工具链使用指引与规范：`docs/dev-plans/025-sqlc-guidelines.md`
- DEV-PLAN-025A：sqlc schema 导出一致性加固（取消夜间校验，PR 即时阻断）：`docs/dev-plans/025a-sqlc-schema-export-consistency-hardening.md`
- Authz（Casbin）工具链与实施方案：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 多工作区并行开发指引（3 worktree）：`docs/dev-plans/014-parallel-worktrees-local-dev-guide.md`
- 全局路由策略统一（UI/API/Webhooks）：`docs/dev-plans/017-routing-strategy.md`
- 文档创建与过程治理规范：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
- CI 质量门禁（Quality Gates）：`docs/dev-plans/012-ci-quality-gates.md`
- SetID 管理（Greenfield，已归档）：`docs/archive/dev-plans/028-setid-management.md`
- DEV-PLAN-009M1【归档】：Phase 4 下一大型里程碑执行计划（SetID + JobCatalog 首个可见样板闭环）：`docs/archive/dev-plans/009m1-phase4-setid-jobcatalog-vertical-slice-execution-plan.md`
- DEV-PLAN-009M2【归档】：Phase 4 下一大型里程碑执行计划（Person Identity + Staffing 首个可见样板闭环）：`docs/archive/dev-plans/009m2-phase4-person-identity-staffing-vertical-slice-execution-plan.md`
- DEV-PLAN-009M3【归档】：Phase 5 下一大型里程碑执行计划（质量收口：E2E 真实化 + 可排障门禁）：`docs/archive/dev-plans/009m3-phase5-quality-hardening-e2e-execution-plan.md`
- DEV-PLAN-009M4【归档】：Phase 2 下一大型里程碑执行计划（SuperAdmin 控制面 + Tenant Console MVP）：`docs/archive/dev-plans/009m4-phase2-superadmin-tenant-console-execution-plan.md`
- DEV-PLAN-009M5【归档】：Phase 2 下一大型里程碑执行计划（AuthN 真实化：Kratos + 本地会话 sid/sa_sid）：`docs/archive/dev-plans/009m5-phase2-authn-kratos-sessions-execution-plan.md`
- DEV-PLAN-009M6【归档】：Phase 1 追加里程碑执行计划（历史：补齐 DEV-PLAN-018 Phase 0，已由 DEV-PLAN-103 收口）：`docs/archive/dev-plans/009m6-phase1-astro-build-phase0-execution-plan.md`
- Greenfield 全新实施路线图（009-031，归档历史来源）：`docs/archive/dev-plans/009-implementation-roadmap.md`
