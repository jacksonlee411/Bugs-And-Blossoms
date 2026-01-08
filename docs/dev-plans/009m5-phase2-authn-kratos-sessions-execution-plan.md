# DEV-PLAN-009M5：Phase 2 下一大型里程碑执行计划（AuthN 真实化：Kratos + 本地会话 sid/sa_sid）

**状态**: 已完成（2026-01-08 02:01 UTC）

> 本文是 `DEV-PLAN-009` 的执行计划补充（里程碑拆解）。假设 `DEV-PLAN-009M4`（SuperAdmin 控制面 + Tenant Console MVP）已完成，本里程碑聚焦 `DEV-PLAN-009` 的 **Phase 2：平台与安全硬化** 中仍缺失的关键出口：将 tenant app 的“占位登录”升级为 **真实认证与会话（Kratos 认人 → 本地 session）**，并推进 SuperAdmin Phase 1（`sa_sid`）以获得更强的可审计主体。
>
> 本文不替代 `DEV-PLAN-019/021/022/023` 的合同；任何契约/数据模型变更必须先更新对应 dev-plan 再写代码。

## 1. 里程碑定义（M5）

### 1.1 输入事实（基于已合并实现）

- `DEV-PLAN-009M1/009M2/009M3/009M4` 已完成：
  - Phase 4 业务纵切片可演示（OrgUnit/JobCatalog/Staffing/Person）。
  - Phase 5 质量门禁已真实化（`make preflight`/`make e2e`）。
  - SuperAdmin 边界已具备最小可用 Tenant Console（BasicAuth Phase 0）。
- tenant 解析：运行态 SSOT 已切换到 DB（`iam.tenant_domains.hostname`），并禁止 runtime fallback 到 `config/tenants.yaml`（单一事实源）。
- tenant app 登录态：当前仍为占位实现（`POST /login` 设置 cookie `session=ok`，无 `sessions/principals` 事实源）；无法满足“可审计主体/可回收会话/可过期”的平台硬化目标。
- SuperAdmin 审计：已具备 `iam.superadmin_audit_logs`（actor 目前为 BasicAuth username）。

### 1.2 目标（对齐 `DEV-PLAN-009` Phase 2 出口条件）

> 目标：把“平台与安全”从占位链路升级为可长期演进的主链路；不以“第二条兼容通道/回退”换取短期可用。

- **AuthN 真实化（tenant app）**：
  - tenant app 的登录从 “cookie 占位” 升级为：`Host 解析 tenant（fail-closed）→ Kratos 认人 → 本地 session（sid）→ RLS 圈地 → Casbin 管事`（对齐 `DEV-PLAN-019`）。
  - 运行态 session 事实源收敛为 DB（禁止并存 `session=ok` 与真实 session 的双链路）。
- **控制面 Phase 1（SuperAdmin）**：
  - 在保留 “环境级保护可选” 的前提下，引入独立 cookie `sa_sid` 与控制面本地 session（对齐 `DEV-PLAN-023` Phase 1），让审计主体从 BasicAuth username 升级为可稳定引用的 `superadmin_principal_id`。
- **Bootstrap 可复现**：
  - 能在“没有任何 tenant/domain/principal/session”时完成最小 bootstrap：创建 tenant + domain + 第一个 tenant admin principal（及其登录凭据）+ 第一个 superadmin principal（如启用 Phase 1）。
  - 任何 bootstrap 必须可被 CI/E2E 复现；不得在迁移/日志中写入明文密码/secret/token/cookie。
- **门禁与证据闭环**：
  - `make preflight` 全绿（含 `make e2e`）。
  - 至少 1 条 E2E smoke 覆盖“superadmin → 创建 tenant/domain → bootstrap tenant admin → tenant app 登录（真实凭据）→ 访问受保护页面”。
  - 在 `DEV-PLAN-010` 固化最短复现步骤与结果，并在 `DEV-PLAN-009` 路线图 Phase 2 出口条件中登记证据（勾选）。

### 1.3 依赖（SSOT 引用）

- 路线图与出口口径：`docs/dev-plans/009-implementation-roadmap.md`
- Tenancy/AuthN：`docs/dev-plans/019-tenant-and-authn.md`
- RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
- SuperAdmin：`docs/dev-plans/023-superadmin-authn.md`
- Routing：`docs/dev-plans/017-routing-strategy.md`
- Atlas+Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
- CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`、`.github/workflows/quality-gates.yml`
- 证据记录：`docs/dev-records/DEV-PLAN-010-READINESS.md`
- 停止线：`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`

## 2. 非目标（本执行计划不做）

- 不在本里程碑内交付企业 SSO/MFA/SCIM/目录同步（如需，另立 dev-plan）。
- 不在本里程碑内实现多角色、多租户委派、复杂 RBAC 管理 UI；MVP 仍以“tenant admin / superadmin”两类 role 为主（对齐 `DEV-PLAN-022`）。
- 不在本里程碑内引入“兼容登录链路/回退通道”（例如 `AUTHN_MODE=legacy`、读写双事实源、`session=ok` 兜底）。

## 3. 不变量与停止线（对齐 `AGENTS.md` + `DEV-PLAN-004M1`）

### 3.1 单一会话事实源（禁止双链路）

- tenant app 的运行态会话事实源必须唯一：**只允许** `sid`（DB session）作为登录态判断依据；不得保留 `session=ok` 作为兼容或 fallback。
- SuperAdmin 的运行态会话事实源必须唯一：**只允许** `sa_sid`（DB session）；BasicAuth/IP allowlist 仅作为部署侧“外层保护”，不得作为应用内身份事实源。

### 3.2 Cookie 与边界隔离（冻结）

- `sid` 与 `sa_sid` 必须为 host-only cookie（不得设置 apex Domain），且 cookie 名不可复用。
- tenant app 与 superadmin 之间不得共享 middleware/会话表/认证 cookie（边界清晰可解释）。

### 3.3 RLS 与 DB role（fail-closed）

- `sessions`/`superadmin_sessions` 不启用 RLS（避免“先有 tenant 才能取 session”的循环），但 tenant-scoped 业务表保持 RLS 强隔离。
- tenant app runtime role 必须为非 superuser 且 `NOBYPASSRLS`；SuperAdmin 的旁路能力只能存在于 superadmin 边界（专用 role/连接池），不得在 tenant app 中出现。

### 3.4 审计（跨租户写必审计）

- 所有跨租户写操作必须写入审计日志；审计失败必须 fail-closed（拒绝写入）。
- 审计 payload 必须过滤敏感信息（密码、token、cookie、Kratos 凭据等）。

### 3.5 停止线（命中即拒绝）

- 引入任何 legacy/回退/双事实源（包括 `session=ok` 与 `sid` 并存，或 superadmin 同时把 BasicAuth 与 sa_sid 作为身份事实源）。
- tenant app 获取到 bypass pool/role 或在 tenant app 中出现跨租户写入口。
- 为“让登录可用”而放宽 RLS/降低 `AUTHZ_MODE`（必须在强约束模式下交付）。

## 4. Done 口径（验收/关闭条件）

### 4.1 Tenant App（AuthN + Session）

- [ ] `GET /login`：在 tenant 已解析前提下可访问；未解析 tenant 必须 fail-closed（404）。
- [ ] `POST /login`：通过 Kratos 完成认证并创建本地 `sid` 会话；成功后进入 `/app`。
- [ ] `POST /logout`：删除/失效化 `sid` 会话并清 cookie；幂等。
- [ ] `sid` 无效/过期：统一回到 `/login`（不得“隐式继续”或“默认租户”）。

### 4.2 SuperAdmin（Phase 1）

- [ ] `GET/POST /superadmin/login`：创建 `sa_sid` 会话；`sa_sid` 无效时必须回到登录页（或 401）。
- [ ] Tenant Console 仍满足 009M4 的 MVP（列表/创建/启停/绑定域名），且审计主体可稳定落到 `superadmin_principal_id`（不再仅靠 BasicAuth username）。

### 4.3 门禁与证据

- [ ] 本地：`make preflight` 全绿。
- [ ] E2E：新增/更新 smoke 覆盖“superadmin → 创建租户/domain → bootstrap tenant admin → tenant app 登录 → 业务 smoke”。
- [ ] 证据固化：更新 `DEV-PLAN-010`（新增 009M5 小节）；并在 `DEV-PLAN-009` Phase 2 出口条件中勾选并链接证据。

## 5. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 required checks 全绿且不 `skipped` 后合并；`main` 禁止直推与 force-push。

### PR-0：合同回填与范围冻结（文档优先）

- [x] 在 `DEV-PLAN-019` 冻结 `sid` 合同：token 形态/哈希存储/TTL/吊销/失败口径/跨租户绑定断言（见 `DEV-PLAN-019` §4.3/§5.2/§4.5）。（#58）
- [x] 在 `DEV-PLAN-023` 冻结 `sa_sid` 合同：token 形态/哈希存储/TTL/失败口径（见 `DEV-PLAN-023` §6.2/§5）。（#58）
- [x] 冻结 bootstrap 凭据注入策略：本机 `.env.local`/CI secrets；禁止迁移/日志/审计落明文（见 `DEV-PLAN-019` §6.3、`DEV-PLAN-023` §9）。（#58）
- [x] 冻结 Kratos dev/CI 拓扑：dev 走 `compose.dev.yml`、CI 走 service；镜像版本 pin 见 `DEV-PLAN-011` 的 Kratos 项；明确 unit/integration 使用 stub/real 的边界与失败产物落点。（#58）
- [x] 冻结落位与依赖方向：`modules/iam/**` 承载 principal/session/Kratos client；`internal/{server,superadmin}` 只做 wiring；禁止 HR 模块依赖 iam domain 类型（对齐 `DEV-PLAN-015/019`）。（#58）
- [x] 门禁对齐：在 `DEV-PLAN-012` 中确认本里程碑命中项的触发器覆盖到位（特别是：迁移闭环、authz pack、routing gates、e2e、doc gate）。（#58）

### PR-1：IAM 数据模型（principals/sessions）与迁移闭环（需要用户确认的新表）

- [x] **红线**：新增表前必须获得用户手工确认（按 `AGENTS.md`）。（#60）
- [x] 新增最小表（建议落在 `iam` schema）。（#60）
  - tenant app：`principals`、`sessions`
  - superadmin：`superadmin_principals`、`superadmin_sessions`（或等价拆分）
- [x] 迁移闭环：`make iam plan && make iam lint && make iam migrate up`（含 smoke）；sqlc 生成物一致性按门禁收口。（#60）

### PR-2：tenant app 本地 session（sid）与中间件收口（移除占位 session=ok）

- [x] `/login`/`/logout` 改为创建/失效化 DB session（`sid`），并在中间件中以 DB session 作为唯一登录态判断依据。（#61）
- [x] 明确 cookie 属性（host-only/httpOnly/sameSite）；无效/过期统一跳转 `/login`。（#61）
- [x] 跨租户绑定断言：`session.tenant_id` 必须与 `Host → tenant_id` 一致；不一致清 cookie 并回到 `/login`（fail-closed）。（#61）
- [x] 单测覆盖：session 校验、过期、登出幂等、fail-closed（覆盖率门禁保持 100%）。（#61）

### PR-3：Kratos 集成（tenant app）

- [x] 增加最小 Kratos 客户端抽象与实现（测试用 stub server 做契约测试；运行态可配置 Kratos endpoint）。（#62）
- [x] `POST /login`：Kratos login flow → whoami → upsert principal → create `sid` session。（#62）
- [x] 开发/CI：提供可复现的 Kratos 启动方式（E2E/CI 使用 `kratosstub`；运行态可用 `KRATOS_PUBLIC_URL` 指向真实 Kratos；避免“本机手工跑”）。（#62）

### PR-4：SuperAdmin Phase 1（sa_sid）+ 审计主体升级

- [x] 新增 `GET/POST /superadmin/login`、`POST /superadmin/logout`；引入 `sa_sid`（host-only）。(#63)
- [x] 审计升级：将 actor 从“字符串”升级为可稳定引用的 principal（必要时保留旧字段但不得作为唯一事实源）。(#63)
- [x] 保持旁路能力仅存在于 superadmin 边界；tenant app 不得获取 bypass pool/role。(#63)

### PR-5：E2E 与 readiness 收口

- [x] 更新/新增 E2E：覆盖本里程碑最小链路（含失败产物与日志落点）。（#64）
- [x] 更新 `DEV-PLAN-010`（009M5 小节）与 `DEV-PLAN-009` Phase 2 出口条件勾选与证据链接。（#64）

## 6. 本地验证（SSOT 引用）

- 一键对齐 CI：`make preflight`
- E2E：`make e2e`
- DB 闭环（按模块）：`make iam plan && make iam lint && make iam migrate up`

## 7. Simple > Easy Review（DEV-PLAN-003，自评要点）

- 结构：tenant app 与 superadmin 通过“cookie/会话表/路由/DB pool”四层隔离，避免隐式共享导致串租户风险。
- 演化：先把 session 事实源与 fail-closed 行为收口，再引入 Kratos；避免“先接 IdP 再补会话/审计”造成返工。
- 回滚：回滚只能走“PR 回滚/环境级保护/只读停写”，禁止引入 runtime legacy 分支。
