# DEV-PLAN-060：全链路业务测试案例套件（009/026-031/220-225 覆盖）

**状态**: 草拟中（2026-01-10 11:40 UTC；2026-04-20 起其中 SetID 主链口径降级为历史测试合同，现行删除 owner 以 `DEV-PLAN-440` 为准）

> 假设：系统已按对应契约文档实现全部功能。本文只定义“全链路业务测试套件”的框架、数据集与记录要求，不替代各模块 dev-plan 的实现合同。

## 1. 背景

本仓库按 Greenfield 的“切片式交付 + 门禁阻断漂移”推进（见 `docs/dev-plans/009-implementation-roadmap.md`、`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/archive/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`）。因此需要一套“全链路业务测试案例套件”，用于在 **不回退/不走双链路** 的前提下验证：
- 系统功能是否覆盖完整业务域（组织/职位分类/职位/任职/人员）；其中本文原有 SetID 主链条目自 2026-04-20 起仅作为历史样本保留，不再作为现行实现目标；
- Assistant 功能是否覆盖会话/意图/提交/任务编排的主链路（对齐 220-225）；
- 每条能力是否 **用户可见、可操作**（避免僵尸功能）；
- 行为是否与契约文档一致（Contract First：偏差必须先记录并回到契约处理）。

## 2. 目标与非目标

### 2.1 目标（本测试套件的 Done 定义）

- **功能全面性**：第 6 节覆盖矩阵中“功能点 × 子计划”全部有落点；每个功能点至少 1 条端到端链路（Create/Update → List/Detail 可见）。
- **用户可见可操作性**：每个子计划至少包含 1 条“页面入口可发现 + 表单/动作可提交 + 结果可见”的闭环。
- **契约遵从性**：每个子计划明确引用对应 dev-plan；测试发现的偏差必须按第 9 节“问题记录”在**对应子计划**中登记（含契约引用与处理建议）。

### 2.2 非目标（本套件不解决）

- 不做性能/压测、容量规划与长周期运维监控（对齐 `AGENTS.md` §3.6）。
- 不扩展契约范围：若执行中发现“功能存在但无契约文档约定”，按问题记录标记为 `CONTRACT_MISSING`，进入“先补契约”流程，而不是在测试文档里临时追加口径。

## 3. 路线图与计划分析（面向测试的结论）

### 3.1 `DEV-PLAN-009`（Greenfield 全新实施路线图）

测试侧关键信号：
- **平台先行**：Tenancy/AuthN → RLS 圈地 → Casbin 授权边界，且 fail-closed（`docs/dev-plans/019-tenant-and-authn.md`、`docs/archive/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`）。
- **用户可见性原则**：每个切片交付必须有页面入口与可操作链路（`AGENTS.md` §3.8）。
- **主数据纵切片顺序（历史口径）**：本文原先采用 `SetID => JobCatalog => Position => Assignments`。自 `DEV-PLAN-440` 生效起，该顺序不再作为现行实现顺序；涉及 SetID 的主数据链路改由 `DEV-PLAN-440` 统筹删除或重写。

因此：本套件以“租户与权限基线 → 主数据 → 人员任职”为主链路顺序。

## 4. 测试环境与登录信息（dev 默认）

> 安全原则：真实环境密码不得写入仓库；本节仅用于本地/dev/测试环境的固定测试账号（或由 Kratos stub 动态创建）。如与实际环境不符，按“问题记录”登记为 `ENV_DRIFT`。

### 4.1 访问地址

- Tenant App（业务端服务地址）：`http://localhost:8080`
  - 注意：tenant 解析依赖 **Host**（或 `X-Forwarded-Host`），因此手工浏览器测试建议直接使用 tenant hostname 访问：
    - T060：`http://t-060.localhost:8080/app/login`
    - T060B：`http://t-060b.localhost:8080/app/login`
  - Home：`http://<tenant-host>:8080/app?as_of=YYYY-MM-DD`
- SuperAdmin（控制面）：`http://localhost:8081`
  - 登录：`http://localhost:8081/superadmin/login`

### 4.2 账号与密码（示例）

| 入口 | 租户 | 账号（email） | 密码 | 备注 |
| --- | --- | --- | --- | --- |
| SuperAdmin（外层 BasicAuth） | — | `admin` | `admin` | dev 默认（见 `e2e/tests/m3-smoke.spec.js`） |
| SuperAdmin（Kratos 登录） | global | `admin+060@example.invalid` | `admin` | `identifier=sa:<email>`（见 §4.4） |
| Tenant Admin（业务端登录） | T060 | `tenant-admin@example.invalid` | `pw` | `identifier=<tenant_id>:<email>`（见 §4.4） |
| Tenant Viewer（只读，用于 403 验证） | T060 | `tenant-viewer@example.invalid` | `pw` | 需能分配 `role_slug=tenant-viewer`（见 §4.4；若不可用记录为 `CONTRACT_MISSING/ENV_DRIFT`） |
| Tenant Admin（业务端登录） | T060B | `tenant-admin-b@example.invalid` | `pw` | 同上（不同 tenant_id） |
| Tenant Viewer（只读，用于 403 验证） | T060B | `tenant-viewer-b@example.invalid` | `pw` | 同上 |

### 4.3 运行态硬要求（测试前置）

- `AUTHZ_MODE=enforce`（未授权必须 403）
- `RLS_ENFORCE=enforce`（未注入 tenant context 必须 fail-closed）
- Host/tenant 解析必须严格：**禁止用 `127.0.0.1` 作为访问 Host**（对齐 `docs/archive/dev-records/DEV-PLAN-010-READINESS.md` 与 E2E）。
- PostgreSQL 运行口径冻结为 **Docker / compose 内数据库**；`TP-060-*` 的 seed / 校验脚本不得把宿主机安装 `psql` 作为唯一前置条件。
- 若测试需要 SQL 直连/seed，必须优先使用 `docker compose exec postgres psql`（或等价容器内执行方式）；不得因宿主机缺少 `psql` 将业务用例判定为失败。
- `as_of` 为 UI Shell 的统一时间语义输入；所有业务页按契约要求显式传入或使用 UI 默认。

### 4.4 账号准备（SSOT：E2E smoke 口径）

> 目标：让第 4.2 的账号“可落地创建”，并与当前仓库的 Kratos stub/E2E 口径保持一致（避免环境漂移）。

1) **创建租户并记录 tenant_id**
   - 在 SuperAdmin：`/superadmin/tenants` 创建 `T060 / t-060.localhost` 与 `T060B / t-060b.localhost`。
   - 在 tenants 列表中记录每个租户展示的 `tenant_id`（uuid），后续创建 tenant 用户 identity 必须使用该值。

2) **创建 Kratos identities（管理员 API）**
   - SSOT：`e2e/tests/m3-smoke.spec.js`（包含默认端口与 `POST /admin/identities` 的请求形状）。
   - 端口（dev 默认，若环境不同按问题记录标记 `ENV_DRIFT`）：
     - Kratos admin：`http://127.0.0.1:4434`
   - identifier 规则（冻结）：
     - superadmin：`sa:<email>`
     - tenant app：`<tenant_id>:<email>`（其中 `<tenant_id>` 为上一步记录的 uuid）
   - traits 要求（最小）：
     - superadmin：`{ email }`
     - tenant app：`{ tenant_id, email }`

3) **首次登录以生成/更新 principals**
   - 使用 tenant hostname 访问 `/app/login`，并通过 `POST /iam/api/sessions` 完成一次登录，以确保系统侧已创建/更新 `iam.principals` 并建立 session。

4) **角色准备（用于 403 验证）**
   - 若环境支持 tenant 角色区分：为 “Tenant Viewer” 账号分配 `role_slug=tenant-viewer`（对齐 `docs/dev-plans/022-authz-casbin-toolchain.md`）。
   - 若环境不支持角色区分/无法分配 `tenant-viewer`：在 TP-060-01 中将“只读 403 验证”记录为 `CONTRACT_MISSING`（并附上阻塞原因与建议的契约/实现落点）。

## 5. 基线数据集（060-DS1/DS2）

> 说明：当前活体数据集只保留现行执行面所需的最小样本。历史上的 `JobCatalog/Position/Person/Assignment` 样本已退出当前态，只在归档子计划中保留。

### 5.0 数据保留与复用（强制）

> 目的：TP-060-* 是“纵切片串联”的测试套件；前一子计划产出的数据会成为后一子计划的输入，因此**必须保留**，不能“跑完就清理”。

- **必须保留**：执行当前活体 TP-060-* 过程中创建/变更的测试数据必须保留（含租户、账号/角色、OrgUnit 与现行 app 所需最小样本）。已被 `DEV-PLAN-450` 删除的 `JobCatalog/Position/Person/Assignment` 数据仅允许作为 archive 证据保留，不再作为当前态建数要求。
- **禁止自动清理**：测试脚本与手工步骤不得包含“跑完回滚/删除租户/清库”的自动清理逻辑。
- **需要重置时的口径**：若因环境漂移/破坏性变更必须重置（例如执行 `make dev-reset`），必须在对应子计划“问题记录”中登记为 `ENV_DRIFT`，并按本文的 060-DS1/DS2 重新建数再继续后续子计划。
- **重复执行口径**：优先“存在则复用、缺失则补齐”；若必须新增，使用可追溯命名（例如附加 `run_id`/日期后缀），并在证据中记录映射，避免后续子计划无法对齐。
- **自动化与手工的边界**：E2E 自动化可能创建临时 tenant（hostname 带 runID）用于验证路径；这些数据同样不清理，但不替代 060-DS1/DS2 的“固定可复现数据集”证据。

### 5.1 租户与域名

- Tenant：`T060`（name：`Tenant 060`）
- Hostname（示例）：`t-060.localhost`
  - 手工测试建议：在本机 `hosts` 中绑定 `127.0.0.1 t-060.localhost`；或在反代层注入 `X-Forwarded-Host: t-060.localhost`（与 E2E 一致）。

### 5.2 组织架构（OrgUnit）

| 层级 | 名称 | 备注 |
| --- | --- | --- |
| L0 | Bugs & Blossoms Co., Ltd. | 根节点 |
| L1 | HQ | 管理/财务/HR |
| L1 | R&D | 研发 |
| L1 | Sales | 销售 |
| L1 | Ops | 运营/支持 |
| L1 | Plant | 制造/仓储（用于岗位差异样例） |

### 5.3 历史主数据样本（已归档，不再作为当前态建数要求）

> 状态说明：历史上的 SetID / JobCatalog / Position / Person / Assignment 样本已分别由 `DEV-PLAN-440` 或 `DEV-PLAN-450` 退出当前执行面。本文不再展开这些样本明细，避免把 archive 内容误读为当前建数要求。

- 如需追溯历史样本，请查：
  - `TP-060-02`：`docs/archive/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`
  - `TP-060-03`：`docs/archive/dev-plans/063-test-tp060-03-person-and-assignments.md`

### 5.7 第二租户数据（060-DS2，用于跨租户隔离验证）

- Tenant：`T060B`（name：`Tenant 060B`）
- Hostname（示例）：`t-060b.localhost`
  - 手工测试建议：在本机 `hosts` 中绑定 `127.0.0.1 t-060b.localhost`；或在反代层注入 `X-Forwarded-Host: t-060b.localhost`。
- 最小数据（用于“跨租户不可见”断言）：
  - 创建 1 个 OrgUnit：`org_code=T060B-ROOT`、`name=Tenant060B Root Unit`；记录其 `org_code` 为 `T060B_ORG_CODE`。
  - 不再要求创建 `Person/Position/Assignment` 样本；相关旧断言已由 `DEV-PLAN-450` 下线。

## 6. 覆盖矩阵（功能 × 子测试计划）

| 功能域 | 覆盖点 | 子计划 |
| --- | --- | --- |
| 租户/登录 | superadmin 创建租户与域名；tenant app 登录 | TP-060-01 |
| 权限/隔离 | Authz 403；RLS fail-closed；跨租户不可见 | TP-060-01 |
| 组织架构 | OrgUnit 树/新增/查询；外部协议仅使用 `org_code` | 现行由 TP-060-01 与后续 orgunit 专项回归承接 |
| SetID | 历史主数据测试样本；当前剩余治理 owner 见 `DEV-PLAN-440` | TP-060-02（已归档，不再执行） |
| 职位分类 / 职位 / 人员 / 任职记录 | 历史三模块测试样本；当前删除 owner 见 `DEV-PLAN-450` | TP-060-02 / TP-060-03（已归档，不再执行） |

> 编号说明：`TP-060-04` 已被现有 OrgUnit 详情双栏回归用例占用（`e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`）。

## 7. 子测试计划（框架）

> 约定：每个子计划都必须在“契约引用”列出对应 dev-plan；测试发现的问题必须记录在本子计划的“问题记录”表中（见第 9 节规则）。

### TP-060-01：租户/登录/权限/隔离基线（平台先行）

**子计划文档**：`docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`

**契约引用**
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/archive/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/023-superadmin-authn.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- `docs/dev-plans/020-i18n-en-zh-only.md`

**数据准备**
- 按 §4.4 创建 `T060 / t-060.localhost` 与 `T060B / t-060b.localhost`，并创建第 4.2 的账号（含 tenant users 的 Kratos identity 与首次登录）。
- 按 060-DS2 在 `T060B` 创建 OrgUnit `org_code=T060B-ROOT` 并记录 `T060B_ORG_CODE`（用于跨租户不可见断言）。

**核心验收点（高层）**
- tenant app 在正确 Host 下可登录并进入 `/app?as_of=...`；错误 Host/缺失 Host 必须 fail-closed。
- **跨租户隔离（Host/Session）**：在 `t-060.localhost` 登录后，直接切换到 `t-060b.localhost` 访问 `/app?as_of=...` 必须 fail-closed（不得“带着同一 session 自动切租户”）；反向同理。
- **跨租户隔离（数据）**：在 `T060` 下以 `T060B_ORG_CODE` 请求第二租户 OrgUnit 数据不得读到内容（404/空/稳定错误码均可，但不得泄漏 B 租户数据）。
- **Authz 可拒绝**：`role_slug=tenant-viewer` 对 GET 可访问，对任一 POST/ADMIN 动作必须 403（至少覆盖 1 个页面）；若无法创建/分配 `tenant-viewer`，按 §9 记录为 `CONTRACT_MISSING` 并标注阻塞点。
- UI Shell：导航可发现（至少 `Org` 与首页入口）与 en/zh 文案不缺漏（抽样 2 页）。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-02【已归档 / 不再执行】：主数据（组织架构 + SetID + 职位分类 + 职位）

**子计划文档**：`docs/archive/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`

> 状态说明：该子计划自 `DEV-PLAN-440/450` 生效起已退出当前态 E2E 执行面，仅作为历史测试样本保留；其中 SetID 剩余治理 owner 见 `DEV-PLAN-440`，`jobcatalog/position` 删除 owner 见 `DEV-PLAN-450`。

**契约引用**
- `docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/archive/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- `docs/archive/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`
- `docs/dev-plans/320-org-node-key-cutover-plan-no-global-expansion.md`

**数据准备**
- 按 060-DS1 建立 OrgUnit 树（`/org/units?as_of=2026-01-01`）。
- 其余 SetID / JobCatalog / Position 样本细节以归档子计划为准，当前不在本总纲重复展开。

**核心验收点（高层）**
- OrgUnit：新增节点后树与详情可见；`as_of` 改变时口径符合日粒度有效期。
- 历史 SetID / JobCatalog / Position 验收细项仅保留在 archive，用于回溯当时测试口径，不再作为当前 E2E 总纲要求。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-03【已归档 / 不再执行】：人员与任职（Person + Assignments）

**子计划文档**：`docs/archive/dev-plans/063-test-tp060-03-person-and-assignments.md`

**契约引用**
- `docs/archive/dev-plans/027-person-minimal-identity-for-staffing.md`
- `docs/archive/dev-plans/031-greenfield-assignment-job-data.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`

> 状态说明：该子计划对应的 `person/assignments` 能力已由 `DEV-PLAN-450` 从当前仓库删除，以下内容仅保留为历史测试样本，不再作为现行 E2E 合同。

**数据准备**
- 历史 `Person/Assignment` 样本细节以归档子计划为准，当前不在本总纲重复展开。

**核心验收点（高层）**
- 历史 `Person/Assignment` 验收细项仅保留在 archive，用于回溯当时测试口径，不再作为当前 E2E 总纲要求。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## 9. 问题记录规则（必须写在对应子计划中）

> 规则：发现问题后，必须在“发生的子计划”下的“问题记录”表追加一行；补齐最小复现信息，确保后续能回到契约处理（“修实现”或“先改契约再修实现”）。

字段要求（最小）：
- `环境（Host/as_of/模式）`：至少包含 Host（tenant domain）、`as_of`、`AUTHZ_MODE/RLS_ENFORCE`。
- `期望（契约引用）`：必须给出对应 dev-plan 路径（必要时补到章节/小节）。
- `类型`：
  - `BUG`：实现不符合契约；
  - `CONTRACT_DRIFT`：实现与契约冲突且需要决定以谁为准；
  - `CONTRACT_MISSING`：功能存在但没有契约文档约定；
  - `ENV_DRIFT`：环境/入口/账号与本文假设不一致（例如端口/路径/登录方式变化）。

## 10. 参考（SSOT 链接）

- 路线图：`docs/dev-plans/009-implementation-roadmap.md`
- 主数据：`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`、`docs/archive/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`、`docs/archive/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`、`docs/archive/dev-plans/031-greenfield-assignment-job-data.md`
- 平台：`docs/dev-plans/019-tenant-and-authn.md`、`docs/archive/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/017-routing-strategy.md`、`docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`、`docs/dev-plans/020-i18n-en-zh-only.md`
