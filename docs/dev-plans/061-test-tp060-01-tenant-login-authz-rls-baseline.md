# DEV-PLAN-061：全链路业务测试子计划 TP-060-01——租户/登录/权限/隔离基线

**状态**: 已完成（2026-01-10 13:18 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 本文按 `docs/dev-plans/000-docs-format.md` 组织，聚焦“平台先行”的基线验收；不复制命令矩阵与工具细节（SSOT：`AGENTS.md`）。

## 1. 背景

`DEV-PLAN-009` 路线图明确“平台先行”：Tenancy/AuthN → RLS（fail-closed）→ Authz（可拒绝），且任何业务能力必须在 UI 可见可操作（`AGENTS.md` §3.8）。本子计划用于验证：
- tenant 解析与登录链路是否稳定可复现；
- RLS/Authz 是否按契约 fail-closed/可拒绝；
- 跨租户隔离是否成立（Host/Session/数据均不串）。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [X] 可通过 SuperAdmin 创建 2 个租户（T060/T060B）并记录 tenant_id。
- [X] 固定测试日期（Valid Time）：`AS_OF_BASE=2026-01-01`（后续步骤统一使用该值，避免执行期漂移）。
- [X] Tenant App 在**正确 tenant host**下可登录（`GET /login`=200，`POST /login`=302），并可访问 `GET /app?as_of=2026-01-01`。
- [X] Tenancy 必须 fail-closed：
  - [X] 错误 Host（不存在的 tenant domain）访问 `GET /login` 必须 `404`（必要时用 `Accept: application/json` 获取稳定错误码 `tenant_not_found`）。
  - [X] 未登录访问 `GET /app?as_of=...` 必须 `302 Location=/login`。
- [X] 跨租户隔离成立：
  - [X] session 不可“带着 cookie 切租户”：携带 `T060` 的 `sid` 访问 `T060B` 必须清 cookie 并 `302 Location=/login`。
  - [X] 数据不可跨租户读取：在 `T060` 调用 `GET /person/api/persons:by-pernr?pernr=201` 必须 `404`（JSON 错误码 `PERSON_NOT_FOUND`）。
- [X] Authz 可拒绝（对齐 `DEV-PLAN-022` 的“只读角色”契约）：只读角色对任一 POST/ADMIN 动作必须 403。
- [X] UI Shell 最小可发现：导航入口可见，en/zh 切换可用（抽样 2 页）。

### 2.2 非目标

- 不在本子计划内验证 Org/Staffing/Payroll/Attendance 的业务正确性（由后续子计划承接）。

### 2.3 工具链与门禁（SSOT 引用）

> 目的：声明“本子计划执行/维护会命中哪些门禁”，并给出单一事实源链接；不在本文复制命令矩阵。

- 触发器（按需勾选本计划命中的项）：
  - [X] Authz（策略 pack/test/lint；SSOT：`AGENTS.md`、`docs/dev-plans/022-authz-casbin-toolchain.md`）
  - [ ] 路由治理（allowlist/分类/responder；SSOT：`AGENTS.md`、`docs/dev-plans/017-routing-strategy.md`）
  - [X] E2E（Playwright；SSOT：`AGENTS.md`、`Makefile` 的 `e2e` 入口）
  - [X] 文档门禁（本文件变更；SSOT：`AGENTS.md`、`Makefile` 的 `doc` 入口）
- SSOT 链接：
  - 触发器矩阵与本地必跑：`AGENTS.md`
  - 命令入口与脚本实现：`Makefile`
  - CI 门禁定义：`.github/workflows/quality-gates.yml`
  - 路由 allowlist（事实源）：`config/routing/allowlist.yaml`
  - Authz policy 产物（事实源）：`config/access/policy.csv`、`config/access/policy.csv.rev`
  - Readiness 证据口径：`docs/dev-records/DEV-PLAN-010-READINESS.md`

## 3. 契约引用（SSOT）

- 路线图与不变量：`docs/dev-plans/009-implementation-roadmap.md`、`AGENTS.md`
- Tenancy/AuthN：`docs/dev-plans/019-tenant-and-authn.md`
- RLS（No Tx, No RLS）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- Authz（Casbin）：`docs/dev-plans/022-authz-casbin-toolchain.md`
- SuperAdmin：`docs/dev-plans/023-superadmin-authn.md`
- 路由治理：`docs/dev-plans/017-routing-strategy.md`
- UI Shell/i18n：`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`、`docs/dev-plans/020-i18n-en-zh-only.md`

### 3.1 接口/路由清单（本子计划使用）

> 说明：为避免“执行期猜测”，本节把关键断言落到可复现的 URL/Method/期望（状态码/重定向/清 cookie）。如需稳定错误码，建议用 `Accept: application/json` 获取 JSON error envelope（UI 路由默认返回 HTML，见 `internal/routing/responder.go`）。

| 场景 | Host | Method | Path | 期望（最小） | 证据口径 |
| --- | --- | --- | --- | --- | --- |
| tenant 正常解析 + 登录页可达 | `t-060.localhost:8080` | GET | `/login` | 200 | 浏览器 Network 状态码或 `curl -i` |
| tenant 不存在必须 fail-closed | `t-060-nope.localhost:8080` | GET | `/login` | 404（可选：JSON `code=tenant_not_found`） | `curl -i -H 'Accept: application/json' ...` |
| 未登录访问受保护页 | `t-060.localhost:8080` | GET | `/app?as_of=2026-01-01` | 302 `Location=/login` | Network/`curl -i` 记录 `Location` |
| 登录成功建立 session | `t-060.localhost:8080` | POST | `/login` | 302 `Location=/app?as_of=...` + `Set-Cookie: sid=...` | Network/`curl -i` 记录 `Set-Cookie` |
| 跨租户 Host/Session 隔离 | `t-060b.localhost:8080` | GET | `/app?as_of=2026-01-01` | 302 `Location=/login` + 清 `sid` cookie | 记录 `Location` + `Set-Cookie`（MaxAge<0） |
| 跨租户数据隔离（确定性断言） | `t-060.localhost:8080` | GET | `/person/api/persons:by-pernr?pernr=201` | 404 JSON `code=PERSON_NOT_FOUND` | 记录响应 JSON |
| Authz 可拒绝（只读 403） | `t-060.localhost:8080` | POST | `/org/nodes?tree_as_of=2026-01-01` | 403（可选：JSON `code=forbidden`） | 浏览器提示/或 `Accept: application/json` |

## 4. 数据准备要求（最小集）

### 4.1 租户（必须）

- `T060`：hostname `t-060.localhost`
- `T060B`：hostname `t-060b.localhost`

> 说明：tenant 解析依赖 Host（或 `X-Forwarded-Host`）；手工浏览器测试建议使用 `http://t-060.localhost:8080` / `http://t-060b.localhost:8080` 访问（对齐 `docs/dev-plans/060-business-e2e-test-suite.md` §4.1）。

### 4.2 账号（必须）

按 `docs/dev-plans/060-business-e2e-test-suite.md` §4.2/§4.4 创建并首次登录，至少包括：
- SuperAdmin（BasicAuth + Kratos 登录）
- Tenant Admin（T060/T060B）
- Tenant Viewer（T060/T060B，若环境支持）

### 4.3 跨租户断言数据（必须）

按 `docs/dev-plans/060-business-e2e-test-suite.md` §5.9（060-DS2）在 `T060B` 创建：
- Person：`pernr=201`（记录 `T060B_PERSON_UUID`；用于跨租户断言）

### 4.4 Authz 只读角色（用于 403 验证）

> 目标：在 `AUTHZ_MODE=enforce` 下，用“只读角色”稳定复现 403，验证“可拒绝”而非“隐式放行”。角色契约见 `docs/dev-plans/022-authz-casbin-toolchain.md`。

- 期望存在：只读角色 `tenant-viewer`（以 `config/access/policy.csv` 中的 `role:*` 为准）。
- 期望能力：只读角色对 `read` 允许、对 `admin` 拒绝（至少覆盖 `orgunit.orgunits` 的 read 与 admin）。
- 若环境缺少“角色分配入口/流程”（无法把某个 principal 设为只读角色）：
  - 本步标记为阻塞，并在“问题记录”中登记为 `CONTRACT_MISSING/ENV_DRIFT`（说明缺口落点：角色分配入口/role slug 命名/policy 缺失）。

### 4.5 数据保留（强制）

- 本子计划创建的 `T060/T060B`、tenant users identities、以及 `T060B pernr=201`（060-DS2）必须保留，供后续子计划复用（SSOT：`docs/dev-plans/060-business-e2e-test-suite.md` §5.0）。
- 禁止在执行完本子计划后清理租户/清库；若必须重置环境，需在问题记录登记 `ENV_DRIFT` 并重建数据集后再继续。

## 5. 测试步骤（执行时勾选）

> 执行记录要求：每步至少记录 `Host/AS_OF_BASE/AUTHZ_MODE/RLS_ENFORCE`；若涉及重定向/清 cookie，必须同时记录 `Location` 与 `Set-Cookie`；失败必须填“问题记录”表。

1. [X] **运行态确认（硬要求）**
   - 固定：`AS_OF_BASE=2026-01-01`。
   - 记录：`AUTHZ_MODE`、`RLS_ENFORCE`、访问 Host（禁止 `127.0.0.1` 作为常规访问 Host；负向用例除外）。
   - （可选）记录：`config/access/policy.csv.rev` 的值（便于定位策略漂移）。
2. [X] **SuperAdmin：创建 T060/T060B**
   - 在 `/superadmin/tenants` 创建两条 tenant/domain，并记录两条 `tenant_id`。
3. [X] **Kratos：创建 identities（按 SSOT 口径）**
   - 参照 `e2e/tests/m3-smoke.spec.js` 的 `POST /admin/identities` 形状创建：
     - superadmin：`identifier=sa:<email>`
     - tenant app：`identifier=<tenant_id>:<email>` 且 traits 含 `tenant_id`（只读账号额外要求：`role_slug=tenant-viewer`）
4. [X] **Tenant App：登录与导航可发现**
   - 访问 `http://t-060.localhost:8080/login` 登录成功（302）并记录 `Set-Cookie: sid=...`。
   - 访问 `http://t-060.localhost:8080/app?as_of=2026-01-01`，确保 UI 壳可见。
   - 抽样 2 页验证中英文切换与导航入口可见（建议：`/org/nodes?tree_as_of=2026-01-01` 与 `/person/persons?as_of=2026-01-01`）。
5. [X] **Tenancy fail-closed（错误 Host）**
   - 访问 `http://t-060-nope.localhost:8080/login` 必须 404（可选：用 `Accept: application/json` 断言 `code=tenant_not_found`）。
6. [X] **跨租户隔离（Host/Session）**
   - 在同一浏览器会话内，从 `t-060.localhost` 切换到 `t-060b.localhost` 访问 `/app?as_of=2026-01-01`：
     - 期望：302 `Location=/login`，并清 `sid` cookie（MaxAge<0）。
7. [X] **跨租户隔离（数据）**
   - 在 `T060` 下调用：`GET /person/api/persons:by-pernr?pernr=201`
     - 期望：404 JSON `code=PERSON_NOT_FOUND`（确定性断言）。
   - （可选，用户可见性证据）在 `T060` 打开 `http://t-060.localhost:8080/person/persons?as_of=2026-01-01`，确认列表不出现 `pernr=201`。
8. [X] **Authz 可拒绝（只读 403）**
   - 前置：存在“只读角色”且可分配到某个 principal（见 §4.4）；否则将本步标记为阻塞并记录问题。
   - 以只读角色访问 `GET /org/nodes?tree_as_of=2026-01-01`：
     - 期望：200（read 允许）。
   - 以只读角色提交任一 POST/ADMIN 动作（示例：`POST /org/nodes?tree_as_of=2026-01-01` 创建节点）：
     - 期望：403（可选：用 `Accept: application/json` 断言 `code=forbidden`）。

## 6. 验收证据（最小）

- `make preflight`（包含 `make e2e`）通过：2026-01-10 13:18 UTC（关键环境：`AUTHZ_MODE=enforce`、`RLS_ENFORCE=enforce`、`TRUST_PROXY=1`；E2E 用例：`e2e/tests/tp060-01-tenant-login-authz-rls-baseline.spec.js`）。
- SuperAdmin tenants 列表：两条租户的 `tenant_id/hostname` 截图或文本记录。
- Tenant App 登录成功：`Set-Cookie: sid=...` + `GET /app?as_of=2026-01-01` 页面截图（或等效证据）。
- Tenancy fail-closed：错误 Host 的 404 证据（可选：JSON `code=tenant_not_found`）。
- 跨租户隔离（Host/Session）：`t-060b` 下的 302 `Location=/login` + 清 cookie（`Set-Cookie` MaxAge<0）证据。
- 跨租户隔离（数据）：`GET /person/api/persons:by-pernr?pernr=201` 的 404 JSON `code=PERSON_NOT_FOUND` 证据。
- Authz 403：只读角色一次 POST 被拒绝的证据（HTTP 403；可选：JSON `code=forbidden`）。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
