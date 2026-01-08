# DEV-PLAN-019：租户管理与登录认证（Kratos 认人 → RLS 圈地 → Casbin 管事）

**状态**: 部分完成（009M5：tenant app Kratos + sid 会话已落地；2026-01-08 03:30 UTC）

> 适用范围：**全新实现的新代码仓库（Greenfield）**。本文总结现仓库在“租户/认证/会话/RLS/Authz”上的既有实现与已评审契约（`DEV-PLAN-019*`、`DEV-PLAN-021`），并给出最小可落地方案。  
> 对齐要求：`DEV-PLAN-015`（DDD 分层框架）、`DEV-PLAN-016`（HR 业务域 4 模块骨架）；本文引入一个 **平台 IAM/Tenancy 模块**，不计入 HR 业务域模块数量。

## 0. 实现注记（本仓库现状，避免 drift）

> 说明：本节用于描述 `Bugs-And-Blossoms` 当前已合并实现的“事实输入”，避免计划与实现口径漂移；目标态合同仍以本文后续章节为准。

- tenant 解析：运行态 SSOT 已切换到 DB（`iam.tenant_domains.hostname`），并禁止 runtime fallback 到 `config/tenants.yaml`（该文件仅可作为样例，不得被运行态读取）。
- tenant app 登录态：已落地 cookie `sid` + DB session（`iam.sessions`），并移除占位 `session=ok`。
- superadmin：已落地独立控制面边界与 Tenant Console MVP（Phase 0：环境级保护/BasicAuth）；Phase 1（`sa_sid` 本地会话）在 `DEV-PLAN-023`/`DEV-PLAN-009M5` 推进。
- 009M5 PR-0（#58）：冻结 `sid` 合同（§4.3/§4.5）与 Kratos 测试/拓扑口径（§9）。
- 009M5 PR-1（#60）：新增 `iam.principals`/`iam.sessions` 数据模型与迁移闭环（token 存 `sha256(sid)`；`sessions` 不启用 RLS）。
- 009M5 PR-2（#61）：tenant app `sid` 会话与中间件收口（以 DB session 为唯一登录态；跨租户绑定断言 fail-closed；移除占位 `session=ok`）。
- 009M5 PR-3（#62）：tenant app `/login` 真实接入 Kratos（login flow + whoami）；E2E/CI 引入 `kratosstub` 保证可复现。
- PR（#69）：对齐本文状态与实现引用（文档收敛，非代码变更）。

## 1. 现仓库实现总结（作为输入，不做兼容包袱）

### 1.1 租户（Tenancy）
- **数据模型**：`iam.tenants` + `iam.tenant_domains`（`hostname` 全局唯一；入库 `lowercase + trim + 去端口` 由 DB 约束兜底），并作为未登录态的 tenant 解析事实源（见 `internal/server/tenancy.go`、`internal/superadmin/handler.go`）。
- **tenant 上下文**：通过 `context` 注入 tenant（见 `internal/server/tenancy.go` 与 `currentTenant(...)`）；缺 tenant 即 fail-closed（404/错误页），避免“跨租户兜底查询”。
- **tenant 解析契约（已有）**：`Host → iam.tenant_domains.hostname`，找不到即 fail-closed（见本文 §4.1）。

### 1.2 认证与会话（AuthN + Session）
- **现状**：应用侧会话 cookie 为 `sid`，并以 `iam.sessions` 为唯一会话事实源（DB 存 `sha256(sid)`，不存明文）。
- **中间件链路**：`Host → tenant` → `sid` cookie → 查 session → 断言 tenant 一致 → 加载 principal → 注入运行态上下文；无效/过期/串租户统一清 cookie 并跳转 `/login`（fail-closed）。
- **已评审演进方向**：采用 **ORY Kratos** 作为 Headless Identity，应用保留 `/login` UI；主链路选择 “Kratos 认人 → 本地 session（`sid`）桥接”（见本文 §4.2/§6.1）。

### 1.3 数据隔离（RLS）
- **现状接口**：事务内设置 `app.current_tenant`（`SELECT set_config('app.current_tenant', $1, true)`），由 RLS policy 读取，实现 fail-closed（RLS 推进口径见 `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`；DB smoke 见 `cmd/dbtool`）。

### 1.4 授权（AuthZ）
- **系统级口径**：Casbin（“管事”）与 RLS（“圈地”）形成纵深防御；主体/role_slug/subject 的冻结口径以 `docs/dev-plans/022-authz-casbin-toolchain.md` 为准。

## 2. 目标与非目标（Greenfield 口径）

### 2.1 目标（Goals）
- [ ] **租户解析 fail-closed**：任何需要 tenant 语义的入口在 tenant 未解析时必须拒绝（404/401），不得回退跨租户逻辑。
- [ ] **统一认证入口**：仅实现一种主链路（推荐：Kratos password login），避免 legacy/多分支并存（对齐 `DEV-PLAN-004M1`）。
- [x] **统一会话模型**：以 `sid` session 作为唯一运行态会话来源，承载 `tenant_id` 与 `principal_id`；cookie 名为 `sid`（不得引入第二套会话事实源；对齐 `DEV-PLAN-009M4` stopline）。
- [ ] **RLS 默认开启（fail-closed）**：所有 tenant-scoped 表默认启用 RLS，并要求在事务内注入 `app.current_tenant`。
- [ ] **最小控制面（Tenant Console）**：提供 SuperAdmin 创建/禁用租户、绑定域名、初始化租户管理员的能力（可先无 DNS/HTTP verify）。
- [ ] **对齐 DDD 分层与共享准入**：IAM/Tenancy 作为平台模块按 `modules/{module}/{domain,infrastructure,services,presentation}` 落地；跨模块共用能力优先下沉 `pkg/**`（遵循 `DEV-PLAN-015`）。
- [ ] **测试覆盖率门禁**：新仓库按 100% 覆盖率门禁执行（遵循 `DEV-PLAN-000` 新增要求）。

### 2.2 非目标（Non-Goals）
- 不在本计划内交付：企业 SSO（Jackson）、MFA、SCIM、目录同步、复杂邀请/审批流。
- 不在本计划内兼容现仓库用户/租户数据；如需迁移，单独出计划（含风险评审与回滚）。

### 2.3 关键决策（ADR 摘要，避免实现阶段“撞出来”）
- **决策 1：采用 Kratos，但应用仍保留本地 session**
  - 选择：Kratos 负责“认人”，应用负责“发会话/圈地/管事”（tenant session（术语：`sid`） + RLS + Casbin）。
  - 理由：运行态只需要一个稳定的 session 事实源；RLS 依赖 `tenant_id`，因此应用必须可确定注入。
- **决策 2：tenant 解析 SSOT 为 `tenant_domains.hostname`**
  - 选择：解析只读 `tenant_domains`（hostname 全局唯一），`tenants.primary_domain` 仅作展示/缓存字段。
  - 理由：避免未来“从 primary_domain 切到 domains”的迁移与回滚复杂度；并明确多域名场景的权威来源。
- **决策 3：登录必须先确定 tenant（fail-closed）**
  - 选择：`/login` 入口也必须先解析 tenant，否则 404；禁止跨租户 email 查询/兜底。
  - 理由：消灭“同一邮箱多租户”歧义，避免串租户与安全事故。
- **决策 4：控制面与数据面隔离**
  - 选择：Tenant Console 在 superadmin server，数据面业务 server 永远不提供跨租户控制入口。
  - 理由：把高风险跨租户能力收口在单一边界，便于审计与回滚（对齐 `DEV-PLAN-023` 的边界与回滚口径）。

## 3. 模块与分层方案（对齐 015/016）

### 3.1 新增平台模块：`modules/iam`
> 说明：`DEV-PLAN-016` 的 4 个 HR 业务域模块保持不变；`iam` 属于平台能力（Identity & Access Management），为运行态提供“租户/认证/授权基础设施”。

- `modules/iam/domain/`：
  - 聚合：`tenant`（Tenant + Domain 列表/主域规则）、`principal`（租户内登录主体）、`session`（运行态会话）。
  - 值对象：`hostname`、`email`、`identity_mode`（仅 `kratos`）。
  - 端口（接口）：`TenantRepository`、`PrincipalRepository`、`SessionRepository`、`IdentityProvider`（Kratos client port）、`AuditSink`（可选）。
- `modules/iam/infrastructure/`：
  - Postgres repo（tenant/principal/session）。
  - Kratos client（实现 `IdentityProvider`，仅承载 login/whoami/logout 的最小子集）。
  - RLS 注入（复用 `pkg/` 统一入口）。
- `modules/iam/services/`：
  - `TenantService`：创建租户、绑定/切换主域、启停租户。
  - `AuthService`：登录/登出（使用 `IdentityProvider`），创建/销毁本地 session。
- `modules/iam/presentation/`：
  - Tenant App：`/login`、`/logout`、（可选）`/settings/account`。
  - SuperAdmin：`/superadmin/tenants/**`（Tenant Console MVP）。

### 3.2 `pkg/**` 下沉（跨模块共享）
- `pkg/tenancy`：Host 规范化、tenant 解析中间件、ctx 注入/读取（可直接复用 `composables` 的模式，但建议在新仓库中收敛命名）。
- `pkg/rls`：事务内注入 `app.current_tenant` 的统一入口（对齐现仓库运行态链路：事务内 `set_config('app.current_tenant', ...)`）。
- `pkg/http/middleware`：认证态注入（session→principal→tenant）与 fail-closed guard（对齐现仓库 `pkg/middleware/auth.go` 的语义）。

### 3.3 依赖方向（保证“可替换性/局部性”）
- HR 业务域模块（`orgunit/jobcatalog/staffing/person`）**不得依赖** `modules/iam` 的 domain 类型与 service；它们只依赖：
  - `pkg/tenancy` 提供的 `tenant_id`（以及可选的 `principal_id`）上下文读取；
  - `pkg/rls` 事务注入契约；
  - `pkg/authz`（如有）提供的鉴权门面。
- `modules/iam` 可以依赖 `pkg/**`，但不得反向依赖任何 HR 模块（对齐 `DEV-PLAN-015` 的依赖方向约束）。

## 4. 关键契约与不变量（避免后续试错）

### 4.1 Tenant 解析（Host → Tenant）
- **SSOT**：`tenant_domains.hostname`（`hostname` 全局唯一；`tenants.primary_domain` 仅用于展示/缓存）。
- **规范化**：`lowercase(hostname)` + 去端口；禁止空字符串；禁止 wildcard。
- **fail-closed**：
  - tenant 未解析：返回 `404`（未登录入口）或 `401`（已登录且 session 缺 tenant）。
  - 禁止按 email “全局查找用户”。
- **信任边界**：生产环境只信任反代写入的 host（建议使用 `X-Forwarded-Host` 白名单策略或由网关做 host 校验），禁止 Host header 注入导致“串租户”。

### 4.2 身份（Kratos）到本地主体（Principal）的映射
沿用已评审的 Headless IdP 方案（Kratos），但移除 legacy 分支（对齐 `DEV-PLAN-004M1`）：
- **identifier**：`{tenant_id}:{lower(email)}`（解决“同一 email 多租户”）。
- **traits（最小子集）**：`tenant_id`、`email`、（可选）`name`。
- **本地 principal**：以 `(tenant_id, email)` 唯一；并绑定 `kratos_identity_id`（全局唯一）用于防串号。

### 4.3 会话（Session）与运行态上下文（`sid`）
- **唯一运行态会话来源**：tenant app 只认 `sid`（cookie 或 `Authorization: Bearer <sid>`）；本仓库已移除占位 `session=ok`（#61，对齐 `DEV-PLAN-004M1` 的 No Legacy）。
- **token 形态（冻结）**：
  - `sid` 为随机高熵不透明字符串（推荐：`32 bytes random → base64url`，无 padding）。
  - DB 中只存 `sha256(sid)`，不存明文 token（避免“DB 泄露 = session 可重放”）。
- **生命周期（冻结）**：
  - `expires_at` 为绝对过期时间（建议默认 14d，支持配置覆盖）。
  - logout = 失效化 session（删除行或标记 revoke）；`principal=disabled` / 重置凭据时必须回收该 principal 的全部 session。
- **跨租户绑定（冻结，必须 fail-closed）**：
  - 请求链路必须先 `Host → tenant_id`（见 §4.1），再查 session（`sessions` 不启用 RLS）。
  - 必须断言：`session.tenant_id == tenant_id`；不一致视为无效 session：清 cookie 并返回 `401`（或 302 到 `/login`）；不得“自动切租户/默认租户”。
- **失败口径（冻结）**：
  - token 缺失/无效/过期 ⇒ 视为未登录；HTML：302 到 `/login`；API：401。
  - 禁止把密码、token、cookie、Kratos 凭据写入日志/审计（只记录 `principal_id/tenant_id/request_id` 等非敏感定位信息）。

### 4.4 授权（Casbin）与主体表达
- **审计标识（principal）**：`tenant:{tenant_id}:principal:{principal_id}`（用于日志/审计/诊断；不作为 MVP 的 Casbin enforce 输入）。
- **授权主体（effective subject）**：`role:{role_slug}`（MVP 单角色；口径见 `DEV-PLAN-022`）。
- **边界**：
  - AuthN（登录）只负责建立 `principal_id` 与 `tenant_id` 的可信来源；
  - AuthZ（Casbin）只负责“是否允许做事”，不得承担 tenant 解析与 session 校验职责；
  - DB（RLS）只负责“圈地”，不得放宽 policy 作为跨租户旁路。

### 4.5 `role_slug`（最小角色集，冻结）
- **权威来源**：`principals.role_slug`；禁止以“有 session 就默认是 admin”等隐式推导作为授权依据。
- **MVP 角色集**：
  - tenant app：`tenant-admin`（唯一可登录角色）
  - 未登录：`anonymous`（仅作为 Authz 输入，不落库）
  - superadmin：使用其专用 principal/session（见 `DEV-PLAN-023`），不得复用 tenant principal。
- **注入规则（冻结）**：session 中间件必须加载 principal 并注入 `principal_id/role_slug/tenant_id`；缺任一项即拒绝进入受保护路径（fail-closed）。

## 5. 数据模型（新仓库建议）

> 本节为目标态 schema 草案；是否启用 domain verify、是否引入企业 SSO、以及 superadmin 的更完整审计模型，后续按子计划扩展。

### 5.1 Tenant（控制面）
- `tenants`
  - `id uuid pk`
  - `name text not null`
  - `primary_domain text not null unique`（仅用于展示/缓存；解析 SSOT 见 `tenant_domains`）
  - `is_active bool not null default true`
  - `created_at/updated_at timestamptz`
- `tenant_domains`
  - `id uuid pk`
  - `tenant_id uuid not null`
  - `hostname text not null unique`
  - `is_primary bool not null default false`
  - `verified_at timestamptz null`（MVP 可不启用 verify，但字段保留）
  - 约束：同 tenant 至多一个 `is_primary=true`；且 `is_primary=true` 的那条必须与 `tenants.primary_domain` 一致（以事务保证）。

### 5.2 Principal / Session（数据面）
- `principals`
  - `id uuid pk`
  - `tenant_id uuid not null`
  - `email text not null`
  - `role_slug text not null`（MVP：`tenant_admin`）
  - `display_name text null`
  - `status text not null`（`active|disabled`）
  - `kratos_identity_id uuid not null unique`
  - `created_at/updated_at timestamptz`
  - 约束：`unique (tenant_id, email)`
- `sessions`
  - `token_sha256 bytea pk`（`sha256(sid)`；32 bytes）
  - `tenant_id uuid not null`
  - `principal_id uuid not null`
  - `expires_at timestamptz not null`
  - `ip text null`、`user_agent text null`
  - `created_at timestamptz not null`

### 5.3 RLS（必须）
- `tenants` / `tenant_domains`（控制面）默认不启用 RLS，由 superadmin server 保护访问边界。
- `sessions`（数据面）**不启用 RLS**：
  - 原因：`sid`→`session` 查询发生在 tenant 解析/注入之前；若对 `sessions` 启用 tenant RLS，将形成“先有 tenant 才能取 session / 先有 session 才能得 tenant”的环。
  - 约束：只允许按 `token` 精确查询；不得提供“按 tenant 列表 session”等接口（防止横向枚举）。
- `principals`（数据面）可启用 RLS：
  - 登录入口已通过 Host 解析 tenant；因此可以在查询 principal 之前先注入 `app.current_tenant`。
  - 若实现成本过高，可先不启用 RLS，但必须保持所有查询显式包含 `tenant_id`，并用测试覆盖“缺 tenant 即失败/不跨租户命中”。
- 推荐：**superadmin 使用独立 DB role/连接池**（旁路在连接层完成），tenant app 的 DB role 对业务表强制开启 RLS（对齐 `DEV-PLAN-021` 的口径）。

## 6. 路由与 UI（最小集）

### 6.1 Tenant App
- `GET /login`：渲染登录页（Host 解析 tenant；未解析返回 404）
- `POST /login`：Kratos login flow（server-side）→ whoami → upsert principal → create session → set tenant session cookie（术语：`sid`）（错误返回 422 并渲染表单错误）
- `POST /logout`：删除 session（可选调用 Kratos logout；无论是否存在 session 都应幂等成功）

### 6.2 SuperAdmin（Tenant Console MVP）
- `GET /superadmin/tenants`：列表
- `POST /superadmin/tenants`：创建租户（含 primary_domain）
- `GET /superadmin/tenants/{tenant_id}`：详情（基础信息 + is_active）
- `POST /superadmin/tenants/{tenant_id}/disable|enable`：启停

SuperAdmin 认证（MVP 选定，见 `DEV-PLAN-009M4`）：
- 采用 **环境级保护/BasicAuth**（可由反代提供，或由 superadmin 二进制内置 BasicAuth），以便保持“控制面边界”简单可回滚、且可被 E2E/CI 复现。
- 不在 Tenant Console MVP 内引入 `sa_sid` 与第二套会话表；`sa_sid` 作为 `DEV-PLAN-023` Phase 1 的后续能力单独推进。

### 6.3 Bootstrap（避免“第一天就锁死”）
- 最小要求：在没有任何 tenant/domain/principal 的情况下，能完成以下闭环：
  1) 创建第一个租户 + 主域名
  2) 创建/绑定第一个租户管理员（principal）
  3) 使用该租户管理员通过 `/login` 登录成功
- 建议方案（Greenfield）：提供一次性/幂等 CLI/脚本入口（例如扩展 `cmd/dbtool` 的 `tenancy-bootstrap` 子命令），用于生成初始租户与域名（以及后续的管理员/principal），并记录为可审计的“执行痕迹”（但不得输出明文密码/secret）。
- CI/E2E 凭据注入（冻结）：凭据只允许通过环境变量/本机 `.env.local` 注入；不得把明文密码/token 写入迁移、seed 文件或仓库；任何“输出一次性凭据”必须显式 opt-in 且默认关闭，避免出现在 CI 日志中。

## 7. 安全与失败路径（必须显式）
- **fail-closed**：tenant 未解析 / session 缺 tenant_id / RLS 注入失败 ⇒ 直接拒绝请求；不得“默认租户”。
- **cookie 策略**：默认 host-only cookie；如必须使用 apex domain，需显式配置且仍必须满足 `session.tenant_id == Host 解析 tenant_id` 的绑定断言（见 §4.3）。
- **敏感信息**：禁止把密码、token、cookie 写入日志；审计日志需过滤 secret。

## 8. 实施步骤（Greenfield 里程碑）
1. [ ] `modules/iam` 骨架落地（对齐 `DEV-PLAN-015` 分层与依赖方向），补齐端口与最小实体。
2. [ ] Tenant Console MVP：创建/禁用租户与主域名。
3. [ ] tenant 解析中间件：Host → tenant_id 注入（fail-closed）。
4. [ ] Kratos 集成：login flow + whoami 最小子集（`IdentityProvider` 实现）。
5. [ ] 本地 session：创建/校验/登出 + 中间件注入 principal/tenant。
6. [ ] RLS：tenant app 事务内注入 `app.current_tenant`；为 tenant-scoped 表编写 policy（fail-closed）。
7. [ ] Casbin：最小授权（superadmin/tenant admin）与路由保护。

## 9. 测试与验收（新仓库 100% 覆盖率门禁）
- 覆盖率口径与统计范围：按新仓库 SSOT（`Makefile`/CI workflow）执行与记录。
- 单元测试（domain/service）：
  - tenant 域名规范化、主域唯一性、启停行为。
  - principal upsert、防串号（kratos_identity_id 不一致）失败路径。
  - session 创建/过期/登出。
- 集成测试（infrastructure）：
  - Postgres repo + RLS 注入：缺 tenant 时必须失败（fail-closed）。
  - Kratos client：默认用 stub server 做契约测试（只覆盖最小端点）；E2E/CI 使用容器化 Kratos（dev：`compose.dev.yml`，CI：workflow service），失败产物统一落到既有 artifacts 目录（例如 `e2e/test-results/`、`e2e/playwright-report/`）。
- E2E（可选，若新仓库已有 e2e 体系）：登录→访问受保护页面→登出。

## 10. 回滚与停止线（避免把复杂度留给实现阶段）
- 回滚（Greenfield 口径）：
  - 禁止引入 “legacy 认证分支” 作为回滚；回滚应通过“保持控制面可用 + 修复配置/数据 + 重试”完成（对齐 `DEV-PLAN-004M1`）。
  - 对 Kratos 依赖的回滚：允许在本地/dev 通过切换到 stub IdP 或禁用外部调用来维持开发效率，但不得进入生产口径。
- 停止线（命中即打回）：
  - [ ] tenant 未解析时出现跨租户兜底查询（按 email 全局查 principal/user）。
  - [ ] session 中缺 tenant_id 仍允许进入 tenant-scoped 业务查询路径。
  - [ ] 通过放宽 RLS policy 解决 superadmin 跨租户读写需求（必须走显式旁路）。

## 11. Simple > Easy Review（DEV-PLAN-003，自评）
### 结构（解耦/边界）
- 通过：HR 业务域与 `modules/iam` 解耦（只依赖 `pkg/**` 的上下文契约），边界可替换。
- 警告：superadmin 与 tenant app 的“认证/会话”若共享同一套 middleware，易引入隐式耦合；实现时需保持两条路由链路清晰分层。

### 演化（规格/确定性）
- 通过：关键决策以 ADR 摘要固定（Kratos+本地 session、tenant_domains SSOT、fail-closed、控制面隔离）。
- 待补齐：落地前需把 `tenant_domains` 与 `tenants.primary_domain` 的一致性约束写成数据库级约束/事务算法（避免靠“代码约定”）。

### 认知（本质/偶然复杂度）
- 通过：复杂度直接对应不变量（先 tenant 后登录、session 必含 tenant、RLS fail-closed）。
- 警告：Bootstrap 如果仅靠 UI 交互可能演变成“试错流程”；因此明确要求 CLI bootstrap 与可审计痕迹。

### 维护（可理解/可解释）
- 通过：主流程可用一句话描述：`Host 解析 tenant → Kratos 认人 → 本地 session → RLS 圈地 → Casbin 管事`。
