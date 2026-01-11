# DEV-PLAN-060：全链路业务测试案例套件（009/039/051-056 覆盖）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 假设：系统已按对应契约文档实现全部功能。本文只定义“全链路业务测试套件”的框架、数据集与记录要求，不替代各模块 dev-plan 的实现合同。

## 1. 背景

本仓库按 Greenfield 的“切片式交付 + 门禁阻断漂移”推进（见 `docs/dev-plans/009-implementation-roadmap.md`、`docs/dev-plans/039-payroll-social-insurance-implementation-roadmap.md`、`docs/dev-plans/051-056`）。因此需要一套“全链路业务测试案例套件”，用于在 **不回退/不走双链路** 的前提下验证：
- 系统功能是否覆盖完整业务域（组织/职位分类/SetID/职位/任职/人员/薪酬/社保/个税/考勤）；
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
- **平台先行**：Tenancy/AuthN → RLS 圈地 → Casbin 授权边界，且 fail-closed（`docs/dev-plans/019-tenant-and-authn.md`、`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`）。
- **用户可见性原则**：每个切片交付必须有页面入口与可操作链路（`AGENTS.md` §3.8）。
- **主数据纵切片顺序**：`SetID => JobCatalog => Position => Assignments`（`docs/dev-plans/028-setid-management.md`～`docs/dev-plans/031-greenfield-assignment-job-data.md`）。

因此：本套件以“租户与权限基线 → 主数据 → 人员任职 → 考勤 → 薪酬（含社保/个税/回溯）”为主链路顺序。

### 3.2 `DEV-PLAN-039`（薪酬社保实施路线图）

测试侧关键信号：
- 以 `DEV-PLAN-041`～`DEV-PLAN-046` 为切片序列；每切片均要求 UI 可发现/可操作。
- 核心端到端闭环：社保政策（单城市）→ pay period → payroll run → calculate → payslip → finalize（定稿后只读），并覆盖 Retro 与净额保证（仅 IIT）。

因此：本套件将薪酬拆为两个子计划：`041-043`（主流程/工资条/社保）与 `044-046`（个税/回溯/税后发放）。

### 3.3 `DEV-PLAN-051`～`DEV-PLAN-056`（考勤切片）

测试侧关键信号：
- 4A 以 **punch 事件**作为可重放输入底座（One Door、append-only），提供 `/org/attendance-punches` 可操作入口。
- 4B 输出日结果读模（标准班次）并可解释；不提供 UI 手工范围重算入口（Option A）。
- 4C 提供 TimeProfile 与 HolidayCalendar 配置入口；驱动日结果可解释。
- 4D 以时间银行/累加器做月度聚合与 trace。
- 4E 统一承接更正与审计/重算（bounded replay），并要求在日结果详情页完成最小纠错链路。
- 4F 通过外部身份映射将钉钉/企微事件纳入同口径 punches/daily results，并提供 `/org/attendance-integrations` 映射管理 UI。

因此：本套件将考勤拆为三个子计划：`4A`（输入）、`4B-4E`（结果/配置/更正）、`4F`（集成）。

## 4. 测试环境与登录信息（dev 默认）

> 安全原则：真实环境密码不得写入仓库；本节仅用于本地/dev/测试环境的固定测试账号（或由 Kratos stub 动态创建）。如与实际环境不符，按“问题记录”登记为 `ENV_DRIFT`。

### 4.1 访问地址

- Tenant App（业务端服务地址）：`http://localhost:8080`
  - 注意：tenant 解析依赖 **Host**（或 `X-Forwarded-Host`），因此手工浏览器测试建议直接使用 tenant hostname 访问：
    - T060：`http://t-060.localhost:8080/login`
    - T060B：`http://t-060b.localhost:8080/login`
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
- Host/tenant 解析必须严格：**禁止用 `127.0.0.1` 作为访问 Host**（对齐 `docs/dev-records/DEV-PLAN-010-READINESS.md` 与 E2E）。
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
   - 使用 tenant hostname 访问 `/login`，用第 4.2 的账号登录至少一次，以确保系统侧已创建/更新 `iam.principals` 并建立 session。

4) **角色准备（用于 403 验证）**
   - 若环境支持 tenant 角色区分：为 “Tenant Viewer” 账号分配 `role_slug=tenant-viewer`（对齐 `docs/dev-plans/022-authz-casbin-toolchain.md`）。
   - 若环境不支持角色区分/无法分配 `tenant-viewer`：在 TP-060-01 中将“只读 403 验证”记录为 `CONTRACT_MISSING`（并附上阻塞原因与建议的契约/实现落点）。

## 5. 基线数据集（060-DS1/DS2）

> 说明：060-DS1 是“业务全链路可复现”的主数据集（含 10 员工差异）；060-DS2 为跨租户隔离验证的最小补充数据集。各子计划在其“数据准备”小节中声明所需子集与增量数据。

### 5.0 数据保留与复用（强制）

> 目的：TP-060-* 是“纵切片串联”的测试套件；前一子计划产出的数据会成为后一子计划的输入，因此**必须保留**，不能“跑完就清理”。

- **必须保留**：执行 TP-060-* 过程中创建/变更的测试数据必须保留（含租户、账号/角色、OrgUnit、SetID/BU/mapping、JobCatalog、Position、Person、Assignment、Punches、Daily Results、TimeProfile/HolidayCalendar、Payroll Period/Run/Payslip、Recalc Requests、Balances 等）。
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
| L1 | Plant | 制造/仓储（用于班次/加班差异） |

### 5.3 SetID 与 Business Unit

| 对象 | 值 | 备注 |
| --- | --- | --- |
| SetID | `S2601` | 示例 SetID |
| BU | `BU000` | 共享（映射到 `SHARE`） |
| BU | `BU901` | 中国区业务单元 |
| Mapping | `BU000 -> SHARE` | 不得隐式回退（对齐 `DEV-PLAN-028`） |
| Mapping | `BU901 -> S2601` | 用于 JobCatalog 解析样板 |

### 5.4 职位分类（JobCatalog，按 `setid=S2601` + `as_of=2026-01-01`）

| 对象 | code | name | 备注 |
| --- | --- | --- | --- |
| Job Family Group | `JFG-ENG` | Engineering | 示例 |
| Job Family Group | `JFG-SALES` | Sales | 示例 |

备注：
- Job Catalog 的权威作用域为 `setid`；本套件使用 `BU901 -> S2601` 的 SetID 解析链路作为 UI 入口（对齐 `docs/dev-plans/028-setid-management.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`）。
- 可选扩展（若系统已实现）：补齐 `job_families/job_levels/job_profiles` 并验证“写入→as_of 读取→UI 可见”的闭环；若未实现则记录为 `SCOPE_GAP`（不得在测试中隐式补口径）。

### 5.5 职位（Positions，`as_of=2026-01-01`）

| Position | OrgUnit | 备注 |
| --- | --- | --- |
| P-ENG-01 | R&D | 研发岗位 |
| P-ENG-02 | R&D | 研发岗位（用于 FTE/薪资差异） |
| P-SALES-01 | Sales | 销售岗位 |
| P-HR-01 | HQ | HR 岗位 |
| P-FIN-01 | HQ | 财务岗位 |
| P-OPS-01 | Ops | 运营岗位 |
| P-PLANT-01 | Plant | 班次/加班样例 |
| P-PLANT-02 | Plant | 班次样例 |
| P-SUPPORT-01 | Ops | 支持岗位 |
| P-MGR-01 | HQ | 管理岗（用于净额保证/回溯样例） |

### 5.6 考勤配置（TimeProfile / HolidayCalendar）

- TimeProfile（**租户默认**，effective-dated，单条时间线）：
  - `TP-DEFAULT`: 标准班次 09:00-18:00（午休 60m，Asia/Shanghai）
- HolidayCalendar（按月覆盖项）：
  - 将 `2026-01-01` 标记为 Holiday（用于 300% 加班样例；对齐 `DEV-PLAN-053`/`DEV-PLAN-054`）。

### 5.7 薪酬配置（Payroll/SI/IIT）

- Pay Group：`monthly`（P0 冻结为自然月；对齐 `DEV-PLAN-042`）
- Pay Period：
  - `PP-2026-01`：`[2026-01-01, 2026-02-01)`
  - `PP-2026-02`：`[2026-02-01, 2026-03-01)`
- 社保政策（单城市，P0）：`city_code=CN-310000`（示例：上海，需全大写/trim）
  - 为保证断言可判定，建议采用如下测试政策值（对齐 `DEV-PLAN-043` 的“clamp + 舍入合同”）：
    - `base_floor=5000.00`
    - `base_ceiling=30001.00`
    - PENSION：`employer_rate=0.160000`、`employee_rate=0.080000`、`rounding_rule=HALF_UP`、`precision=2`
    - MEDICAL：`employer_rate=0.095530`、`employee_rate=0.020070`、`rounding_rule=CEIL`、`precision=2`
    - 其他险种：可设为 0 以简化对账（但仍需满足 schema/枚举约束）
- IIT（居民综合所得累计预扣）：按 `DEV-PLAN-044` 算法与 balances 口径执行。

### 5.8 员工数据（10 人，均需存在 Person + Assignment）

> 备注：本表以“测试意图”为主；具体字段落点以 `docs/dev-plans/027-person-minimal-identity-for-staffing.md`、`docs/dev-plans/031-greenfield-assignment-job-data.md`、`docs/dev-plans/042/044` 为准。

| 编号 | pernr | 姓名 | FTE | 岗位 | 入职生效日 | 月薪（base_salary） | 关键差异（用于覆盖） |
| --- | --- | --- | --- | --- | --- | --- | --- |
| E01 | `101` | Alice Zhang | 1.0 | P-ENG-01 | 2026-01-01 | 20,000.00 | 标准链路：工资条=基本工资 + 正常出勤 |
| E02 | `102` | Bob Li | 1.0 | P-SALES-01 | 2026-01-01 | 80,000.00 | 社保基数上限 clamp + 加班/时间银行 |
| E03 | `00000103` | Carol Wu | 1.0 | P-ENG-02 | 2026-01-01 | 3,000.00 | pernr 前导 0 解析一致性 + 社保基数下限 clamp + 缺卡纠错 |
| E04 | `104` | David Chen | 0.5 | P-ENG-02 | 2026-01-01 | 20,000.00 | FTE 0.5 的 pro-rate 口径 |
| E05 | `105` | Erin Sun | 1.0 | P-MGR-01 | 2026-01-01 | 30,000.00 | 回溯：已定稿后提交更早生效的加薪/调岗触发 recalc request |
| E06 | `106` | Frank Zhou | 1.0 | P-FIN-01 | 2026-01-15 | 25,000.00 | IIT：年中入职 first_tax_month 口径 + SAD 录入/留抵（可选） |
| E07 | `107` | Grace Xu | 1.0 | P-MGR-01 | 2026-01-01 | 35,000.00 | 税后发放（仅 IIT）净额保证：target_net=20,000.00 |
| E08 | `108` | Henry Gao | 1.0 | P-PLANT-01 | 2026-01-01 | 12,000.00 | 钉钉 Stream 外部事件（RAW）+ 映射 active |
| E09 | `109` | Ivy He | 1.0 | P-PLANT-02 | 2026-01-01 | 12,000.00 | 企微 Poller 外部事件（RAW）+ pending→active→disabled 状态流转 |
| E10 | `110` | Jack Lin | 1.0 | P-SUPPORT-01 | 2026-01-01 | 15,000.00 | 时间银行：调休/累加器（earned/used）+ 月度 trace |

### 5.9 第二租户数据（060-DS2，用于跨租户隔离验证）

- Tenant：`T060B`（name：`Tenant 060B`）
- Hostname（示例）：`t-060b.localhost`
  - 手工测试建议：在本机 `hosts` 中绑定 `127.0.0.1 t-060b.localhost`；或在反代层注入 `X-Forwarded-Host: t-060b.localhost`。
- 最小数据（用于“跨租户不可见”断言）：
  - 创建 1 个 Person：`pernr=201`、`display_name=Tenant060B Person 201`；记录其 `person_uuid` 为 `T060B_PERSON_UUID`。
  - （可选）创建 1 个 Position + 1 条 Assignment（用于跨租户 positions/assignments 的不可见验证）。

## 6. 覆盖矩阵（功能 × 子测试计划）

| 功能域 | 覆盖点 | 子计划 |
| --- | --- | --- |
| 租户/登录 | superadmin 创建租户与域名；tenant app 登录 | TP-060-01 |
| 权限/隔离 | Authz 403；RLS fail-closed；跨租户不可见 | TP-060-01 |
| 组织架构 | OrgUnit 树/新增/查询 | TP-060-02 |
| SetID | SetID/BU/mapping；JobCatalog 解析 setid | TP-060-02 |
| 职位分类 | Job family group 创建与查询（可选扩展：families/levels/profiles） | TP-060-02 |
| 职位 | Position 创建与列表 | TP-060-02 |
| 人员 | Person 创建/查询；pernr 解析一致性 | TP-060-03 |
| 任职记录 | Assignment timeline；仅展示 effective_date；变更触发回溯 | TP-060-03、TP-060-08 |
| 考勤 | punches（手工/导入/外部）→ daily results → time profile/holiday → time bank → corrections | TP-060-04/05/06 |
| 薪酬 | pay period/run → payslip → 社保 → IIT/balances → retro → net guarantee | TP-060-07/08 |

## 7. 子测试计划（框架）

> 约定：每个子计划都必须在“契约引用”列出对应 dev-plan；测试发现的问题必须记录在本子计划的“问题记录”表中（见第 9 节规则）。

### TP-060-01：租户/登录/权限/隔离基线（平台先行）

**子计划文档**：`docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`

**契约引用**
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/023-superadmin-authn.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- `docs/dev-plans/020-i18n-en-zh-only.md`

**数据准备**
- 按 §4.4 创建 `T060 / t-060.localhost` 与 `T060B / t-060b.localhost`，并创建第 4.2 的账号（含 tenant users 的 Kratos identity 与首次登录）。
- 按 060-DS2 在 `T060B` 创建 Person `pernr=201` 并记录 `T060B_PERSON_UUID`（用于跨租户不可见断言）。

**核心验收点（高层）**
- tenant app 在正确 Host 下可登录并进入 `/app?as_of=...`；错误 Host/缺失 Host 必须 fail-closed。
- **跨租户隔离（Host/Session）**：在 `t-060.localhost` 登录后，直接切换到 `t-060b.localhost` 访问 `/app?as_of=...` 必须 fail-closed（不得“带着同一 session 自动切租户”）；反向同理。
- **跨租户隔离（数据）**：在 `T060` 下用 `T060B_PERSON_UUID` 访问任一“按 person_uuid 定位”的页面（示例：`/org/attendance-daily-results/{person_uuid}/{work_date}?as_of=...`）不得读到数据（404/空/稳定错误码均可，但不得泄漏 B 租户数据）。
- **Authz 可拒绝**：`role_slug=tenant-viewer` 对 GET 可访问，对任一 POST/ADMIN 动作必须 403（至少覆盖 1 个页面）；若无法创建/分配 `tenant-viewer`，按 §9 记录为 `CONTRACT_MISSING` 并标注阻塞点。
- UI Shell：导航可发现（Org/Person/Staffing/Attendance/Payroll 入口）与 en/zh 文案不缺漏（抽样 2 页）。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-02：主数据（组织架构 + SetID + 职位分类 + 职位）

**子计划文档**：`docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`

**契约引用**
- `docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- `docs/dev-plans/028-setid-management.md`
- `docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- `docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`

**数据准备**
- 按 060-DS1 建立 OrgUnit 树（`/org/nodes?as_of=2026-01-01`）。
- 建立 SetID/BU/mapping（`/org/setid`）。
- 在 `BU901` 下建立 JobCatalog（`/org/job-catalog?business_unit_id=BU901&as_of=2026-01-01`）。
- 建立 10 个职位（`/org/positions?as_of=2026-01-01`）。

**核心验收点（高层）**
- OrgUnit：新增节点后树与详情可见；`as_of` 改变时口径符合日粒度有效期。
- SetID：mapping 保存后，JobCatalog 页面可显示“resolved setid”，且缺映射必须 fail-closed（不允许默认洞）。
- JobCatalog：至少 1 个实体“写入→列表可见”闭环；BU 变更与 as_of 变更口径一致。
- Position：新增职位后列表可见；职位引用 OrgUnit 的输入/下拉来源可靠。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-03：人员与任职（Person + Assignments）

**子计划文档**：`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`

**契约引用**
- `docs/dev-plans/027-person-minimal-identity-for-staffing.md`
- `docs/dev-plans/031-greenfield-assignment-job-data.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`

**数据准备**
- 创建 10 个 Person（`/person/persons?as_of=2026-01-01`），包含 `E03 pernr=00000103`。
- 为 10 人创建/更新 Assignment（`/org/assignments?as_of=2026-01-01&pernr=...`），绑定到 10 个职位，并设置 `base_salary/allocated_fte`（对齐 `DEV-PLAN-042` 的输入语义）。

**核心验收点（高层）**
- pernr 校验：仅允许 1-8 位数字字符串；前导 0 的解析一致性可验证（同一人可用不同输入形式定位，但不得产生重复人）。
- Assignment：timeline 可见；UI 仅展示 `effective_date`（不展示 `end_date`）。
- 有效期：提交更早生效的变更后，按 `as_of` 读取可见历史版本（Valid Time=day）。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-04：考勤 4A（Punch Ledger：手工补卡 + 最小导入）

**子计划文档**：`docs/dev-plans/064-test-tp060-04-attendance-4a-punch-ledger.md`

**契约引用**
- `docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`

**数据准备**
- 确保 10 个员工存在（060-DS1）。
- 在 `2026-01-02` 为 E01/E03/E10 录入 punches（`/org/attendance-punches?as_of=2026-01-02`）：
  - E01：IN 09:00 / OUT 18:00
  - E03：仅 IN（制造缺卡）
  - E10：通过 CSV 粘贴导入 1 天 2 条记录（验证导入链路）

**核心验收点（高层）**
- punches 列表按人员/日期范围可查；新增后立即可见。
- 只读角色对 punches 的 POST 必须 403；未注入 tenant context 必须 fail-closed。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-05：考勤 4B-4E（日结果/配置/时间银行/更正与审计）

**子计划文档**：`docs/dev-plans/065-test-tp060-05-attendance-4b-4e-results-config-bank-corrections.md`

**契约引用**
- `docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`
- `docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`
- `docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`
- `docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`

**数据准备**
- 配置默认 TimeProfile（`/org/attendance-time-profile?as_of=2026-01-01`）。
- 配置 HolidayCalendar：将 `2026-01-01` 标记为 Holiday（`/org/attendance-holiday-calendar?as_of=2026-01-01&month=2026-01`）。
- 准备 punches：
  - E02：`2026-01-01` Holiday 日加班（08:00-20:00，用于 300%/时间银行）
  - E03：保留缺卡（来自 TP-060-04）

**核心验收点（高层）**
- 日结果列表/详情可见（`/org/attendance-daily-results`）。
  - 断言 1（可判定）：`work_date=2026-01-02` 下，E01（已录入 09:00 IN / 18:00 OUT）应为 `PRESENT`。
  - 断言 2（可判定，4E）：进入 E01 的详情页作废（void）`18:00 OUT` 后，summary 必须变为 `EXCEPTION` 且包含 `MISSING_OUT`；punch 审计表中该记录标记为 `VOIDED`。
- HolidayCalendar 覆盖项可追溯：日结果能解释 holiday/overtime 字段变化，并可跳转回 punches。
- 时间银行页面可见（`/org/attendance-time-bank`）：月度汇总与 trace 链接到日结果详情。
- 更正与审计：在日结果详情页作废错误 punch → 结果变更可见；bounded replay/权限口径符合契约（4B 不提供 UI 手工范围重算入口，4E 统一承接纠错与重算）。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-06：考勤 4F（钉钉/企微外部对接 + 身份映射）

**子计划文档**：`docs/dev-plans/066-test-tp060-06-attendance-4f-integrations-identity-mapping.md`

**契约引用**
- `docs/dev-plans/056-attendance-slice-4f-dingtalk-wecom-integration.md`
- `docs/dev-plans/027-person-minimal-identity-for-staffing.md`

**数据准备**
- 为 E08/E09 建立外部身份映射（`/org/attendance-integrations`）：
  - E08：DINGTALK userId → person_uuid（active）
  - E09：WECOM userid → person_uuid（pending → active → disabled）
- 运行外部摄入（Worker 或等效模拟）：产生 `punch_type=RAW` 的外部 punch 事件。

**核心验收点（高层）**
- `/org/attendance-integrations` 页面可发现、可操作（创建/禁用/状态可见，含 `last_seen_at/seen_count`）。
- 外部事件进入后，在 `/org/attendance-punches` 与 `/org/attendance-daily-results` 可见且与手工事件同口径。
- 未授权访问 integrations 必须 403；缺少 tenant context 必须 fail-closed。

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-07：薪酬 041-043（主流程 + 工资条 + 社保）

**子计划文档**：`docs/dev-plans/067-test-tp060-07-payroll-041-043-run-payslip-si.md`

**契约引用**
- `docs/dev-plans/039-payroll-social-insurance-implementation-roadmap.md`
- `docs/dev-plans/041-payroll-p0-slice-pay-period-and-payroll-run.md`
- `docs/dev-plans/042-payroll-p0-slice-payslip-and-pay-items.md`
- `docs/dev-plans/043-payroll-p0-slice-social-insurance-policy-and-calculation.md`

**数据准备**
- 确保 10 人 assignment 已具备 `base_salary/allocated_fte`（060-DS1）。
- 配置社保政策（单城市）至少 1 个有效版本（`/org/payroll-social-insurance-policies`）。
- 创建 pay periods（`/org/payroll-periods`）与 2026-01 payroll run（`/org/payroll-runs`）。

**核心验收点（高层）**
- pay period/run：可创建、可计算、可定稿；定稿后只读（再次计算/定稿失败且有稳定错误码）。
- payslips：run 下工资条列表/详情可见（`/org/payroll-runs/{run_id}/payslips`）；`gross_pay/net_pay/employer_total` 与明细可对账。
- 社保：险种明细可见（个人扣款 + 企业成本）；对 E02/E03 分别覆盖基数上限/下限 clamp；社保政策若在 period 内变更必须 fail-closed。
  - 断言 1（可判定，042）：在 `PP-2026-01` 的工资条中，E04 的 `EARNING_BASE_SALARY` 明细金额应为 `10,000.00`（`base_salary=20,000.00` × `allocated_fte=0.5`，且有效期覆盖整月）。
  - 断言 2（可判定，043；按 §5.7 测试政策值）：
    - E03（`gross_pay=3,000.00`，clamp 到 `base_floor=5,000.00`）：
      - PENSION：employee=`400.00`，employer=`800.00`
      - MEDICAL：employee=`100.35`，employer=`477.65`
    - E02（`gross_pay=80,000.00`，clamp 到 `base_ceiling=30,001.00`）：
      - PENSION：employee=`2,400.08`，employer=`4,800.16`
      - MEDICAL：employee=`602.13`（CEIL），employer=`2,866.00`（CEIL）

**问题记录**
| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

### TP-060-08：薪酬 044-046（个税累计预扣 + 回溯 + 税后发放）

**子计划文档**：`docs/dev-plans/068-test-tp060-08-payroll-044-046-iit-retro-net-guarantee.md`

**契约引用**
- `docs/dev-plans/044-payroll-p0-slice-iit-cumulative-withholding-and-balances.md`
- `docs/dev-plans/045-payroll-p0-slice-retroactive-accounting.md`
- `docs/dev-plans/046-payroll-p0-slice-net-guaranteed-iit-tax-gross-up.md`

**数据准备**
- 在 `PP-2026-01` 已定稿前后准备：
  - E06：专项附加扣除（SAD）月度合计录入（internal API 或 UI 承载，按 `DEV-PLAN-044` 约定）。
  - E07：录入净额保证工资项（`target_net=20,000.00`，仅 IIT 税后）。
- `PP-2026-02`：创建新 run 用于承载回溯差额与继续累计。
- 回溯触发：在 `PP-2026-01` 定稿后，为 E05 提交“更早 effective_date 的 assignment 变更”（例如 2026-01-15 生效加薪），触发 recalc request。

**核心验收点（高层）**
- IIT：工资条展示 IIT 明细与税后实发；balances 读口径为 O(1)；年中入职的累计减除费用月数口径正确（E06）。
  - 断言 1（可判定，044）：对 E06，执行 `GET /org/api/payroll-balances?person_uuid=<E06_UUID>&tax_year=2026`：
    - `PP-2026-01` 定稿后：`first_tax_month=1`、`last_tax_month=1`、`ytd_standard_deduction="5000.00"`
    - `PP-2026-02` 定稿后：`last_tax_month=2`、`ytd_standard_deduction="10000.00"`
- 回溯：触发后 UI 可见 recalc request（`/org/payroll-recalc-requests`）；执行 apply 后，差额以 pay items 结转到 `PP-2026-02`，且可追溯 origin（不覆盖已定稿工资条）。
- 税后发放（仅 IIT）：E07 的净额保证项满足 `net_after_iit == target_net`（精确到分），并可解释 `gross_amount/iit_delta`。
  - 断言 2（可判定，046）：E07 的净额保证工资项 `target_net=20,000.00` 计算后，该项 `net_after_iit` 必须精确等于 `20,000.00`（分位一致），且工资条展示 `gross_amount/iit_delta`。

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

- 路线图：`docs/dev-plans/009-implementation-roadmap.md`、`docs/dev-plans/039-payroll-social-insurance-implementation-roadmap.md`
- 主数据：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/028-setid-management.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/031-greenfield-assignment-job-data.md`
- 平台：`docs/dev-plans/019-tenant-and-authn.md`、`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/017-routing-strategy.md`、`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`、`docs/dev-plans/020-i18n-en-zh-only.md`
- 薪酬：`docs/dev-plans/040-payroll-social-insurance-module-design-blueprint.md`、`docs/dev-plans/041-046`、`docs/dev-plans/043-045`
- 考勤：`docs/dev-plans/050-hrms-attendance-blueprint.md`、`docs/dev-plans/051-056`
