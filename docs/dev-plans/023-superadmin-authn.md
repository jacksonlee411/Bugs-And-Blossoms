# DEV-PLAN-023：SuperAdmin 控制面认证与会话（与租户登录链路解耦）

**状态**: 草拟中（2026-01-05 08:05 UTC）

> 适用范围：**全新实现的新代码仓库（Greenfield）**。  
> 上游依赖：`DEV-PLAN-019`（租户管理与登录认证总体方案）、`DEV-PLAN-021`（RLS 推进）、`DEV-PLAN-019D`（控制面边界与回滚理念）。  
> 目标：把 “SuperAdmin 控制面” 的认证/会话/旁路能力收口成一个清晰、可回滚、可审计的边界，避免在早期引入第二套隐式认证模型。

## 1. 背景与现状（输入）

本仓库当前仅具备 tenant app 的最小可登录链路，并 **尚未落地** 独立 superadmin 二进制/路由：
- routing allowlist 已预留 entrypoint：`superadmin`（对齐 `DEV-PLAN-017`），但目前仅用于 ops 占位。
- tenant app 当前最小实现使用 cookie `session=ok` 作为登录态占位（目标态术语仍为 `sid`）。
- `DEV-PLAN-009M4` 将在此基础上补齐：superadmin 边界 + Tenant Console MVP + Tenancy SSOT 切换（DB `tenant_domains.hostname`）。

Greenfield 将重构租户/认证/RLS，并要求：
- tenant app：`Host 解析 tenant（fail-closed）→ Kratos 认人 → 本地 session（sid）→ RLS 圈地 → Casbin 管事`（见 `DEV-PLAN-019`）。
- 控制面：跨租户高风险操作必须在独立边界，且 RLS 旁路必须显式（见 `DEV-PLAN-019D/019A`、`DEV-PLAN-021`）。

## 2. 目标与非目标

### 2.1 目标（Goals）
- [ ] **认证链路解耦**：SuperAdmin 不复用 tenant app 的登录态 cookie（术语：`sid`；本仓库当前实现为 `session` 占位）；Phase 0 使用环境级保护，Phase 1 使用独立 cookie 名与独立 session 事实源。
- [ ] **显式旁路**：SuperAdmin 访问启用 RLS 的业务表时，必须通过独立 DB role/连接池（或 BYPASSRLS role）实现旁路；tenant app 永远不可获得该连接。
- [ ] **可回滚**：认证故障时，能降级到“环境级保护 + 只读/停写”，而不是引入 legacy 分支（对齐 `DEV-PLAN-004M1`）。
- [ ] **可审计**：所有跨租户写操作必须记录审计事件（最小字段即可：who/when/what/target_tenant）。
- [ ] **Bootstrap 可用**：在没有任何租户/用户数据时，也能创建第一个 superadmin 并登录进入控制面。

### 2.2 非目标（Non-Goals）
- 不在本计划内交付：企业 SSO（Jackson）、MFA、SCIM、复杂权限委派。
- 不在本计划内解决：运营监控/告警体系（保持早期阶段“不过度运维”原则）。

## 3. 关键决策（ADR 摘要）

### 3.1 决策 1：分阶段落地（Phase 0→Phase 1）
- 选择：
  - Phase 0（`DEV-PLAN-009M4`）：使用环境级保护/BasicAuth，不引入 `sa_sid` 与 `superadmin_sessions`（避免在 tenant app 尚未具备真实 `sessions`/`principal` 时先造第二套会话系统）。
  - Phase 1（后续里程碑）：引入独立会话 cookie `sa_sid`（host-only），并落地 `superadmin_sessions`/`superadmin_principals`。
- 理由：把复杂度与风险按阶段拆开：MVP 先确保“边界隔离 + 审计 + 可回滚”，再引入可用性更强但成本更高的控制面会话。

### 3.2 决策 2：SuperAdmin 不依赖 Host→tenant 解析
- 选择：控制面不做 tenant 解析；跨租户操作必须显式携带 `target_tenant_id`（路径或表单字段）。
- 理由：SuperAdmin 的本质是“跨租户操作”，把 tenant 解析留在 tenant app，可避免串租户风险模型混杂。

### 3.3 决策 3：`sessions`（tenant app）不启用 RLS；控制面会话独立
- 选择：tenant app 的 `sessions` 不启用 RLS（避免“先有 tenant 才能取 session”的循环）；控制面 Phase 1 另建 `superadmin_sessions`，同样不启用 RLS。
- 理由：会话查找必须发生在 tenant 注入之前；RLS 适用于业务表，而非 session 表。

### 3.4 决策 4：SuperAdmin Identity 与 Tenant Identity 共享 Kratos（但 identifier 命名空间隔离）
- 选择：复用同一 Kratos 实例，superadmin 使用独立 identifier 命名空间 `sa:{lower(email)}`；tenant app 仍使用 `{tenant_id}:{lower(email)}`（见 `DEV-PLAN-019`）。
- 理由：避免再引入第二个 Kratos 部署组件；同时用命名空间消除“同一 email 多租户”与 superadmin 的冲突与歧义。

## 4. 架构与边界

### 4.1 组件图（Mermaid）

> 注：下图展示 Phase 1 的目标态（`sa_sid`）；`DEV-PLAN-009M4` 的 Phase 0 将以 BasicAuth 作为控制面入口保护。tenant app cookie 当前实现为 `session`（目标态术语为 `sid`）。

```mermaid
flowchart LR
  subgraph TenantApp[Tenant App]
    B1[Browser] -->|sid| A1[HTTP Server]
    A1 -->|session->principal->tenant| M1[Middleware]
    M1 -->|set_config app.current_tenant| R1[RLS Enforce]
  end

  subgraph SuperAdmin[SuperAdmin Server]
    B2[Browser] -->|sa_sid| A2[HTTP Server]
    A2 --> M2[SuperAdmin Middleware]
    M2 -->|explicit bypass pool| DB2[(DB: bypass role)]
    M1 -->|tenant pool| DB1[(DB: tenant role)]
  end
```

### 4.2 模块落位（对齐 `DEV-PLAN-015/016/019`）
- 推荐放在平台 `modules/iam` 内：
  - `modules/iam/domain/superadmin/`：`SuperAdminPrincipal`、`SuperAdminSession`、审计事件值对象。
  - `modules/iam/presentation/superadmin/`：控制面登录页与租户管理 UI/API。
  - `modules/iam/infrastructure/persistence/`：superadmin repo 使用 **bypass 连接池**。
- HR 业务域模块不依赖任何 superadmin 类型；仅依赖 `pkg/**` 的 tenancy/rls/authz 契约。

## 5. 数据模型（新仓库建议）

> 本节是目标态 schema 草案；新增表落盘前需用户确认（本仓库规则）。注：`DEV-PLAN-009M4` 已对其 MVP 需要的新增表/迁移做了预先批准（2026-01-07）。

- `superadmin_principals`
  - `id uuid pk`
  - `email text not null unique`
  - `display_name text null`
  - `status text not null`（`active|disabled`）
  - `kratos_identity_id uuid null unique`（若采用 Kratos；若 MVP 先 BasicAuth，可为空）
  - `created_at/updated_at timestamptz`

- `superadmin_sessions`
  - `token text pk`
  - `principal_id uuid not null`
  - `expires_at timestamptz not null`
  - `ip text null`、`user_agent text null`
  - `created_at timestamptz not null`

- `superadmin_audit_logs`（最小审计）
  - `id uuid pk`
  - `actor text not null`（Phase 0：BasicAuth username；Phase 1：也可写 `principal_id`）
  - `principal_id uuid null`
  - `action text not null`（例如 `tenant.create` / `tenant.disable`）
  - `target_tenant_id uuid null`
  - `payload jsonb not null`（必须过滤 secret/token/cookie）
  - `created_at timestamptz not null`

## 6. 认证方案（分阶段，避免“一口吃成胖子”）

### 6.1 Phase 0（MVP）：环境级保护 + 只读/停写开关
- 入口保护：反代 BasicAuth / 内网访问 / IP allowlist（三选一或组合）。
- 优点：实现最小、回滚最直接；适合 Greenfield 早期。
- 缺点：无法获得强身份（principal_id）；审计粒度通常只能到 BasicAuth username。
- 停止线：进入生产口径前，跨租户写操作必须具备可审计主体（至少 actor=BasicAuth username；如需更强身份与会话管理则推进 Phase 1）。

### 6.2 Phase 1（推荐）：Kratos 认人 + 控制面本地会话（`sa_sid`）
- SuperAdmin 登录：
  - `GET /superadmin/login`
  - `POST /superadmin/login`：Kratos login flow（server-side）→ whoami → upsert `superadmin_principals` → create `superadmin_sessions` → set `sa_sid`
- 登出：
  - `POST /superadmin/logout`：删除 `superadmin_sessions`（可选同时调用 Kratos logout）
- 重要约束：
  - SuperAdmin 入口必须是独立 host（例如 `superadmin.<apex>`），避免与 tenant 域名混用。
  - `sa_sid` cookie 必须 host-only；不得设置为 apex Domain，避免被租户站点携带。
  - Kratos identifier 必须采用 `sa:{lower(email)}`；不得复用 tenant 的 `{tenant_id}:{email}` 规则。

### 6.3 Phase 2（可选）：接入企业 SSO（Jackson）
单独出计划（例如 019B），并在 Tenant Console 中管理 superadmin 的 SSO 连接与回滚策略；不在本计划内展开。

## 7. 路由、失败路径与回滚

### 7.1 路由（最小集）
- `GET /superadmin/login`
- `POST /superadmin/login`
- `POST /superadmin/logout`
- `GET /superadmin/tenants`
- `POST /superadmin/tenants`
- `POST /superadmin/tenants/{tenant_id}/disable|enable`

### 7.2 失败路径（必须显式）
- `sa_sid` 无效/过期：统一 302 到 `/superadmin/login`（或 401 for API）。
- bypass DB pool 不可用：控制面进入“只读/停写”模式，并给出明确错误提示；不得自动降级为 tenant pool（避免旁路消失导致误判）。
  - audit log 写入失败：跨租户写操作必须 fail-closed（拒绝写入），不得“先写再说”。

### 7.3 回滚策略
- Phase 1 回滚到 Phase 0：
  - 禁用 `/superadmin/login`（或让其恒返回 404），仅保留环境级保护入口。
  - 所有写操作强制 `SUPERADMIN_WRITE_MODE=disabled`（kill switch）。
  - 数据不回滚：保留 `superadmin_principals/sessions/audit_logs`，仅停止链路使用。

## 8. 测试与验收（100% 覆盖率门禁）

- 单元测试：
  - `sa_sid` cookie 生成与属性（host-only、httpOnly、sameSite）。
  - session 校验与过期、登出幂等。
  - 写操作必须产生日志（audit log）且过滤敏感字段。
- 集成测试：
  - bypass pool 与 tenant pool 严格分离：tenant app 代码路径不可拿到 bypass pool。
  - 当启用 RLS 的业务表被访问时：tenant pool 必须被圈地；bypass pool 可以跨租户读写（仅在 superadmin 路径）。

验收标准（最小集）：
- [ ] tenant app 的 `sid` 不能访问 superadmin 路由（必须 302/401）。
- [ ] superadmin 的 `sa_sid` 不能访问 tenant app 的登录态路由（必须 401）。
- [ ] 所有跨租户写操作都能在 `superadmin_audit_logs` 找到对应记录，且 `payload` 不含 secret/token/cookie。
- [ ] 任一旁路能力（bypass pool / write mode）在关闭时必须 fail-closed。

## 9. Bootstrap（避免“第一天就锁死”）
- Phase 0：通过部署侧 BasicAuth/allowlist 进入控制面，无需创建 superadmin principal。
- Phase 1：提供一次性 CLI（例如 `cmd/superadmin bootstrap`）：
  - 创建首个 `superadmin_principals`（email + display_name）。
  - （可选）创建对应 Kratos identity（identifier `sa:{email}`），并输出一次性初始密码/恢复链接（不得落日志）。
  - 记录可审计痕迹（时间、操作者、目标 email），但不得记录敏感凭据。

## 10. Simple > Easy Review（DEV-PLAN-003，自评）

### 结构（解耦/边界）
- 通过：用独立 cookie + 独立 session 表把控制面与数据面解耦；旁路能力仅在 superadmin 边界存在。
- 警告：若试图复用 tenant app 的 session/middleware，会快速引入“隐式 tenant 依赖”，应坚持边界隔离。

### 演化（规格/确定性）
- 通过：分阶段落地（Phase 0→1），每步都有可验证输出与回滚开关。

### 认知（本质/偶然复杂度）
- 通过：复杂度来自真实风险（跨租户旁路/审计/回滚），没有为“未来可能”引入多余抽象。

### 维护（可理解/可解释）
- 通过：控制面主流程可一句话描述：`sa_sid` 认证 → bypass pool 执行跨租户操作 → 写入审计。

## 11. 停止线（命中即打回）
- [ ] 控制面复用 tenant app 的 `sid` cookie 或 session 表。
- [ ] bypass pool 不可用时自动降级到 tenant pool（属于安全降级错误）。
- [ ] 跨租户写操作未写入审计日志仍然成功。
