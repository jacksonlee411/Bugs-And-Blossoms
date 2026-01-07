# DEV-PLAN-009M4：Phase 2 下一大型里程碑执行计划（SuperAdmin 控制面 + Tenant Console MVP）

**状态**: 已完成（2026-01-07 18:31 UTC）

> 本文是 `DEV-PLAN-009` 的执行计划补充（里程碑拆解）。在 `DEV-PLAN-009M3`（Phase 5：E2E 真实化）已完成的基础上，本里程碑聚焦 `DEV-PLAN-009` 的 **Phase 2：平台与安全硬化** 中仍缺失的关键出口：**控制面边界（SuperAdmin）**，并交付最小可用的 Tenant Console（创建/启停租户、绑定域名、bootstrap）。
>
> 本文不替代 `DEV-PLAN-019/023` 的合同；任何契约/数据模型变更必须先更新对应 dev-plan 再写代码。

## 1. 里程碑定义（M4）

### 1.1 输入事实（基于已合并实现）

- `DEV-PLAN-009M1/009M2/009M3` 已完成：Phase 4 业务纵切片可演示；Phase 5 E2E 已真实化并纳入 required checks。
- tenant app 当前具备 Host→tenant fail-closed 与最小登录链路，但 tenant 解析 SSOT 仍为配置文件（`config/tenants.yaml`），控制面（superadmin）仍为占位（allowlist 仅预留 entrypoint）。
- RLS/Authz 已按 enforce 口径运行；但缺少“跨租户控制面”的明确边界与审计闭环（对齐 `DEV-PLAN-023` 的目标）。
- 本里程碑预计需要新增 Tenancy/控制面相关表；用户已在对话中预先同意（2026-01-07），后续实施不再需要逐次审批。

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

- **认证/会话隔离**：Phase 0 superadmin 仅允许环境级保护/BasicAuth；不得复用 tenant app 的登录态 cookie（当前为 `session`；目标态术语为 `sid`）。Phase 1 若引入控制面会话，cookie 名固定为 `sa_sid` 且与 tenant app 独立。
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

### 3.4 关键决策（本里程碑冻结，避免实现期“撞出来”）

> 说明：以下决策是对评审意见的落实；进入实现阶段后不得临时改口径（若确需改变，必须先更新本文与对应 dev-plan 再动代码）。

1) SuperAdmin AuthN（Phase 0，M4）
- 选定：`DEV-PLAN-009M4` 只落地 **环境级保护/BasicAuth**（`DEV-PLAN-023` Phase 0），不在本里程碑引入 `sa_sid`（避免在 tenant app 尚未具备真实 session/principal 时，先造第二套会话系统）。
- 约束：跨租户写操作必须有审计（actor 以 BasicAuth username 记录）；审计写入失败必须 fail-closed。
- 预留：`sa_sid` 作为 `DEV-PLAN-023` Phase 1 的后续里程碑（不在 M4 做）。

2) Cookie 命名（与当前实现对齐）
- 现状：tenant app 当前最小实现使用 cookie 名 `session`（见 `internal/server/handler.go`），尚未落地 `sessions` 表与 `sid` 语义。
- 选定：M4 **不改动** tenant app cookie 名；因此本文与 E2E/验收中的“tenant app 登录态 cookie”均以 `session` 为准。
- 迁移：当 `DEV-PLAN-019` 的真实 session/principal 落地后，再单独出变更把 `session` 迁移/收敛为 `sid`（需给出迁移/回滚与 E2E 证据）。

3) Tenancy SSOT 切换与 bootstrap（单一权威）
- 选定：tenant 解析 SSOT 收敛到 `tenant_domains.hostname`（`DEV-PLAN-019`），并通过 **一次性/幂等 CLI bootstrap** 提供确定性数据（不在迁移中硬编码 dev-only seed）。
- 约束：运行态禁止读取 `config/tenants.yaml` 作为兜底/回退；该文件最多保留为样例，不得成为运行态权威来源。

4) 本地/CI 拓扑（可复现）
- 选定：tenant app 与 superadmin 以两个独立进程运行：tenant app `:8080`，superadmin `:8081`；本地与 CI 均使用 `localhost` 访问（避免额外 DNS/证书复杂度）。
- 选定：多租户域名在本地/CI 使用 `*.localhost`（无需改 `/etc/hosts`）；E2E 可使用 `t-<id>.localhost` 作为新租户域名。

## 4. Done 口径（验收/关闭条件）

### 4.1 控制面可用（可审计）

- [ ] 存在独立二进制（建议：`cmd/superadmin`），可独立启动并通过 `/health`。
- [ ] allowlist `superadmin` entrypoint 覆盖 superadmin 路由最小集，并通过 `make check routing`。
- [ ] superadmin 与 tenant app：
  - [ ] superadmin 使用环境级保护/BasicAuth（Phase 0），tenant app 的登录态 cookie（当前为 `session`）不得作为 superadmin 的认证依据。
  - [ ] DB role/连接池隔离（tenant app 无法获得 bypass role）。
- [ ] superadmin 执行跨租户写操作时写入审计记录（actor=BasicAuth username），且不包含 secret/token/cookie。

### 4.2 Tenant Console MVP（可 bootstrap）

- [X] `GET /superadmin/tenants`：可列出租户。
- [X] `POST /superadmin/tenants`：可创建租户（含 primary domain）。
- [X] `POST /superadmin/tenants/{tenant_id}/disable|enable`：可启停租户。
- [X] 提供 bootstrap（选定：迁移 seed 默认 tenant/domain + superadmin 可创建），确保“没有任何数据时”也能创建第一个租户与域名，并能在 tenant app 使用该域名完成登录。

### 4.3 门禁与证据（可复现）

- [X] 本地：`make preflight` 全绿。
- [X] CI：四大 required checks 全绿且不出现 `skipped`（本里程碑所有 PR 必须满足）。
- [X] E2E：新增 1 条 smoke 覆盖“superadmin（BasicAuth）→创建租户/domain→tenant app（新域名）登录→访问受保护页面”的最小链路（失败产出 artifact）。
- [X] 证据固化：`DEV-PLAN-010` 增补本里程碑可复现步骤与结果。

## 5. 实施步骤（建议 PR 序列）

> 说明：每个 PR 都必须在 required checks 全绿且不 `skipped` 后合并；`main` 禁止直推与 force-push。

### PR 发起前的文档更新规则（强制）

- [ ] 每完成一个 PR 的开发任务，在发起 PR 之前，必须先在“该 PR 命中的 dev-plan 合同文档”与本文中登记已完成事项（`[ ]`→`[X]`），并补证据（命令/时间戳/PR 链接）。
- [ ] 若某个 dev-plan 文档的待办条目全部完成，必须同步更新其头部 `**状态**` 为 `已完成（YYYY-MM-DD HH:MM UTC）`。

### PR-0：合同对齐与范围冻结（文档优先）

- [X] 在 `DEV-PLAN-019/023/022/017/021` 中补齐本里程碑 MVP 所需的“边界/不变量/验收/失败路径”（仅冻结口径，不在本文复制细节）。
- [X] 在 `DEV-PLAN-012` 中确认 Gate 4（E2E）与 routing/authz/db gate 的触发器覆盖本里程碑新增内容（避免 CI 漂移）。

### PR-1：superadmin 二进制骨架 + routing entrypoint 落地

- [X] 新增 `cmd/superadmin`（独立启动入口）与最小 handler（至少 `/health`）。
- [X] 更新 allowlist：填充 `superadmin` entrypoint 的最小路由集（不与 `server` 混用），并确保 routing gates 通过。
- [X] 最小本地入口：`make dev-superadmin`。

### PR-2：Tenancy 与控制面最小 schema（需要用户确认的新表）

- [X] 新增 Tenancy 控制面表（`tenants`、`tenant_domains`），并按合同冻结唯一性/规范化规则（hostname 全局唯一等）。
- [X] 新增 superadmin 审计表（最小集：`superadmin_audit_logs`；Phase 0 不引入 `superadmin_sessions`）。
- [X] 迁移闭环：`make iam plan/lint/migrate up`（含 smoke）。
- [ ] **红线（已预先批准）**：上述新增表/迁移已获用户在对话中预先同意（2026-01-07），后续落盘不再需要逐次审批；但必须在本文与相关 dev-plan 中登记具体表/迁移与 PR 证据。

### PR-3：tenant 解析 SSOT 切换到 DB（移除 runtime fallback）

- [X] tenant app 的 Host→tenant 解析改为读取 `tenant_domains.hostname`（fail-closed；`/login` 同样必须先解析 tenant）。
- [X] 移除运行态对 `config/tenants.yaml` 的依赖（样例可保留，但运行态不得读取）。
- [X] 提供 dev/test 的确定性 bootstrap（迁移 seed 默认 `localhost` tenant/domain），确保 `make e2e` 稳定运行。

### PR-4：superadmin AuthN（Phase 0：环境级保护/BasicAuth）+ DB role/pool 隔离

- [X] superadmin 使用环境级保护/BasicAuth（不引入 `sa_sid` 与第二套 session 表）。
- [X] 显式 superadmin DB 连接池/role（仅在 superadmin 边界存在），并用测试证明 tenant app 无法取得该连接。
- [X] 失败路径：BasicAuth 缺失/错误统一拒绝；旁路池不可用 fail-closed（不得降级为 tenant pool）。

### PR-5：Tenant Console MVP（列表/创建/启停/域名绑定）+ 审计

- [X] 实现 `GET/POST /superadmin/tenants`、`POST /superadmin/tenants/{tenant_id}/disable|enable` 等最小集（以 `DEV-PLAN-019` 为合同）。
- [X] 所有跨租户写操作写 audit log；审计失败拒绝写入。

### PR-6：E2E 与门禁收口（把“能用”变成“可长期演进”）

- [X] E2E 新增 smoke：覆盖“superadmin（BasicAuth，:8081）→创建租户/domain（`t-<id>.localhost`）→tenant app（:8080，新域名）登录→访问受保护页面”的最小链路，并补齐负路径（无 BasicAuth 必须拒绝）。
- [X] 更新 readiness：在 `DEV-PLAN-010` 固化最短复现步骤与 artifact 位置。

## 6. 本地验证（SSOT 引用）

- 一键对齐 CI：`make preflight`
- E2E：`make e2e`
- DB 闭环（按模块）：`make iam plan && make iam lint && make iam migrate up`

## 7. Simple > Easy Review（DEV-PLAN-003，自评要点）

- 结构：控制面与数据面通过“二进制/路由/cookie/DB pool”四层隔离，避免隐式耦合。
- 演化：M4 只做 superadmin Phase 0（BasicAuth）与 Tenancy SSOT 切换；`sa_sid`/Kratos/SSO 另起里程碑，避免把可回滚边界变成不可回滚大改。
- 维护：确保“单一 SSOT + fail-closed + 可审计”的主线可在 5 分钟内讲清楚，且 CI 能阻断漂移。

## 8. 完成登记（证据）

- 日期：2026-01-07
- 本地门禁：`make preflight`（全绿，含 `make e2e`）
- E2E 关键产物：
  - server/superadmin 日志：`e2e/_artifacts/server.log`、`e2e/_artifacts/superadmin.log`
