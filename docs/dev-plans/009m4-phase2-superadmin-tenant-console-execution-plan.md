# DEV-PLAN-009M4：Phase 2 下一大型里程碑执行计划（SuperAdmin 控制面 + Tenant Console MVP）

**状态**: 草拟中（2026-01-07 13:05 UTC）

> 本文是 `DEV-PLAN-009` 的执行计划补充（里程碑拆解）。在 `DEV-PLAN-009M3`（Phase 5：E2E 真实化）已完成的基础上，本里程碑聚焦 `DEV-PLAN-009` 的 **Phase 2：平台与安全硬化** 中仍缺失的关键出口：**控制面边界（SuperAdmin）**，并交付最小可用的 Tenant Console（创建/启停租户、绑定域名、bootstrap）。
>
> 本文不替代 `DEV-PLAN-019/023` 的合同；任何契约/数据模型变更必须先更新对应 dev-plan 再写代码。

## 1. 里程碑定义（M4）

### 1.1 输入事实（基于已合并实现）

- `DEV-PLAN-009M1/009M2/009M3` 已完成：Phase 4 业务纵切片可演示；Phase 5 E2E 已真实化并纳入 required checks。
- tenant app 当前具备 Host→tenant fail-closed 与最小登录链路，但 tenant 解析 SSOT 仍为配置文件（`config/tenants.yaml`），控制面（superadmin）仍为占位（allowlist 仅预留 entrypoint）。
- RLS/Authz 已按 enforce 口径运行；但缺少“跨租户控制面”的明确边界与审计闭环（对齐 `DEV-PLAN-023` 的目标）。

### 1.2 目标（对齐 `DEV-PLAN-009` Phase 2 出口条件 #3）

- **控制面边界可用**：交付独立 superadmin 服务边界（独立二进制/路由 entrypoint/独立 cookie/独立 DB role/连接池），并具备最小审计，避免把跨租户能力泄漏到 tenant app。
- **Tenant Console MVP 可用**：SuperAdmin 能创建/启停租户、绑定域名（hostname），并完成最小 bootstrap，使 tenant app 能以新建租户域名完成登录与访问。
- **Tenancy SSOT 收敛**：将 tenant 解析 SSOT 收敛为 `tenant_domains.hostname`（对齐 `DEV-PLAN-019`），并移除运行态对 `config/tenants.yaml` 的依赖（不允许 runtime 双事实源/回退）。
- **门禁与证据闭环**：路由 allowlist、Authz policy、迁移闭环、E2E 与 readiness 证据必须同步更新，确保 drift 可被 CI 阻断且可复现。

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
- 停止线：`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`

## 2. 非目标（本执行计划不做）

- 不在本里程碑内引入企业 SSO/MFA/SCIM；不在本里程碑内完成 Kratos 全量集成（若需要，单独拆里程碑，避免把“身份系统接入”与“控制面边界”耦合成不可控的大 PR）。
- 不新增 HR 业务域功能与业务数据契约（OrgUnit/JobCatalog/Staffing/Person 仍以现有闭环为基线）。
- 不通过“放宽 RLS policy”来满足 superadmin 跨租户访问需求；旁路必须在连接/role 层完成（对齐 `DEV-PLAN-021/023`）。

## 3. 不变量与停止线（对齐 `DEV-PLAN-003` + `AGENTS.md`）

### 3.1 控制面边界（必须冻结）

- **Cookie 隔离**：tenant app 只能使用 `sid`；superadmin 只能使用 `sa_sid`；两者不得复用同名 cookie/session 表。
- **路由隔离**：superadmin 路由必须落在 allowlist 的 `superadmin` entrypoint 下，并由 routing gates 阻断漂移；tenant app 不得暴露跨租户控制入口。
- **DB 连接池隔离**：tenant app 使用非 superuser 且 `NOBYPASSRLS` 的 runtime role；superadmin 使用专用 role/连接池承载跨租户能力（若需要 BYPASSRLS，必须在 superadmin 边界显式化）。
- **审计先行**：任何跨租户写操作必须写入 audit log；审计失败必须 fail-closed（拒绝写入）。

### 3.2 Tenancy SSOT（禁止双事实源）

- **唯一 SSOT**：tenant 解析只允许读取 `tenant_domains.hostname`；不得在运行态保留 `config/tenants.yaml` 的兜底/回退路径。
- **fail-closed**：tenant 未解析时，`/login` 与任何需要 tenant 语义的入口必须拒绝（404/401），不得默认租户。

### 3.3 停止线（命中即拒绝）

- 引入 legacy/双链路/回退通道（包括 runtime fallback 到 `config/tenants.yaml`）。
- superadmin 复用 tenant app 的 cookie/session 或共享同一 middleware 导致边界不清。
- 为满足控制面需求而放宽 RLS policy（必须走专用连接池/role）。
- 跨租户写未写审计仍然成功（必须 fail-closed）。

## 4. Done 口径（验收/关闭条件）

### 4.1 控制面可用（可审计）

- [ ] 存在独立二进制（建议：`cmd/superadmin`），可独立启动并通过 `/health`。
- [ ] allowlist `superadmin` entrypoint 覆盖 superadmin 路由最小集，并通过 `make check routing`。
- [ ] superadmin 与 tenant app：
  - [ ] cookie 与会话事实源隔离（`sid` 无法访问 superadmin；`sa_sid` 无法访问 tenant app 的登录态路由）。
  - [ ] DB role/连接池隔离（tenant app 无法获得 bypass role）。
- [ ] superadmin 执行跨租户写操作时写入审计记录，且不包含 secret/token/cookie。

### 4.2 Tenant Console MVP（可 bootstrap）

- [ ] `GET /superadmin/tenants`：可列出租户。
- [ ] `POST /superadmin/tenants`：可创建租户（含 primary domain）。
- [ ] `POST /superadmin/tenants/{tenant_id}/disable|enable`：可启停租户。
- [ ] 提供 bootstrap 入口（建议：一次性 CLI），确保“没有任何数据时”也能创建第一个租户与域名，并能在 tenant app 使用该域名完成登录。

### 4.3 门禁与证据（可复现）

- [ ] 本地：`make preflight` 全绿。
- [ ] CI：四大 required checks 全绿且不出现 `skipped`（本里程碑所有 PR 必须满足）。
- [ ] E2E：至少新增 1 条 smoke 覆盖“控制面边界”或“tenant bootstrap→登录→访问”的最小链路（失败产出 artifact）。
- [ ] 证据固化：`DEV-PLAN-010` 增补本里程碑可复现步骤与结果。

## 5. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 required checks 全绿且不 `skipped` 后合并；`main` 禁止直推与 force-push。

### PR 发起前的文档更新规则（强制）

- [ ] 每完成一个 PR 的开发任务，在发起 PR 之前，必须先在“该 PR 命中的 dev-plan 合同文档”与本文中登记已完成事项（`[ ]`→`[X]`），并补证据（命令/时间戳/PR 链接）。
- [ ] 若某个 dev-plan 文档的待办条目全部完成，必须同步更新其头部 `**状态**` 为 `已完成（YYYY-MM-DD HH:MM UTC）`。

### PR-0：合同对齐与范围冻结（文档优先）

- [ ] 在 `DEV-PLAN-019/023/022/017/021` 中补齐本里程碑 MVP 所需的“边界/不变量/验收/失败路径”（仅冻结口径，不在本文复制细节）。
- [ ] 在 `DEV-PLAN-012` 中确认 Gate 4（E2E）与 routing/authz/db gate 的触发器覆盖本里程碑新增内容（避免 CI 漂移）。

### PR-1：superadmin 二进制骨架 + routing entrypoint 落地

- [ ] 新增 `cmd/superadmin`（独立启动入口）与最小 handler（至少 `/health`）。
- [ ] 更新 allowlist：填充 `superadmin` entrypoint 的最小路由集（不与 `server` 混用），并确保 routing gates 通过。
- [ ] 最小本地入口（建议）：`make dev-superadmin`（仅作为入口，不在 PR-1 扩展控制面能力）。

### PR-2：Tenancy 与控制面最小 schema（需要用户确认的新表）

- [ ] 新增 Tenancy 控制面表（至少 `tenants`、`tenant_domains`），并按合同冻结唯一性/规范化规则（hostname 全局唯一等）。
- [ ] 新增 superadmin 会话/审计表（例如 `superadmin_sessions`、`superadmin_audit_logs` 的最小集）。
- [ ] 迁移闭环：补齐 `make iam plan/lint/migrate up` 的闭环验证与 smoke（对齐 `DEV-PLAN-024`）。
- [ ] **红线**：上述新增表/迁移落盘前，必须获得用户手工确认。

### PR-3：tenant 解析 SSOT 切换到 DB（移除 runtime fallback）

- [ ] tenant app 的 Host→tenant 解析改为读取 `tenant_domains.hostname`（fail-closed；`/login` 同样必须先解析 tenant）。
- [ ] 移除运行态对 `config/tenants.yaml` 的依赖（可保留为样例文件，但不得被运行态读取）。
- [ ] 提供 dev/test 的确定性 bootstrap（例如迁移内置默认 `localhost` 租户/域名，或一次性 CLI/seed 步骤），确保 `make e2e` 可稳定运行。

### PR-4：superadmin AuthN/Session（`sa_sid`）与 DB role/pool 隔离

- [ ] superadmin 使用独立 cookie（`sa_sid`）与独立 session 事实源。
- [ ] 显式 superadmin DB 连接池/role（如需旁路，必须只在 superadmin 边界存在），并用测试证明 tenant app 无法取得该连接。
- [ ] 失败路径：session 无效统一跳转/拒绝；旁路池不可用必须 fail-closed（不得降级为 tenant pool）。

### PR-5：Tenant Console MVP（列表/创建/启停/域名绑定）+ 审计

- [ ] 实现 `GET/POST /superadmin/tenants`、`POST /superadmin/tenants/{tenant_id}/disable|enable` 等最小集（以 `DEV-PLAN-019` 为合同）。
- [ ] 所有跨租户写操作写 audit log；审计失败拒绝写入。

### PR-6：E2E 与门禁收口（把“能用”变成“可长期演进”）

- [ ] E2E 新增 smoke：至少覆盖“superadmin→创建租户/domain→tenant app 登录并访问受保护页面”的最小链路，或覆盖“cookie 隔离”的负路径。
- [ ] 更新 readiness：在 `DEV-PLAN-010` 固化最短复现步骤与 artifact 位置。

## 6. 本地验证（SSOT 引用）

- 一键对齐 CI：`make preflight`
- E2E：`make e2e`
- DB 闭环（按模块）：`make iam plan && make iam lint && make iam migrate up`

## 7. Simple > Easy Review（DEV-PLAN-003，自评要点）

- 结构：控制面与数据面通过“二进制/路由/cookie/DB pool”四层隔离，避免隐式耦合。
- 演化：先冻结边界与停止线，再落地最小可用 Tenant Console；Kratos/SSO 另起里程碑，避免把可回滚边界变成不可回滚大改。
- 维护：确保“单一 SSOT + fail-closed + 可审计”的主线可在 5 分钟内讲清楚，且 CI 能阻断漂移。
