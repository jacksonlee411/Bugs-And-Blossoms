# DEV-PLAN-022：Authz（Casbin）工具链与实施方案（Greenfield）

**状态**: 部分完成（009M1：tenant app 最小可拒绝；2026-01-06 23:40 UTC）

> 适用范围：本仓库（`Bugs-And-Blossoms`）的 **Greenfield implementation**。如未来拆分到独立仓库，本计划冻结的 Authz 契约与工具链口径仍应保持一致，避免“同一概念两套权威表达”。
>
> 本文冻结授权（Authz）契约与工具链口径：`subject/object/action/domain` 命名、policy 的 SSOT 与发布方式、CI 门禁，以及与 `DEV-PLAN-021/019`（RLS/AuthN/Tenancy）的边界关系，避免实现期“各模块各写一套”导致漂移。

## 1. 背景与上下文 (Context)

- Greenfield 全新实施（路线图见 `DEV-PLAN-009`），不承担存量 `user.Can`/旧权限映射的兼容包袱。
- 本仓库已具备最小 Casbin 工具链与门禁：`config/access/*` + `scripts/authz/*` + `make authz-pack/authz-test/authz-lint`（CI 入口见 `.github/workflows/quality-gates.yml`）。
- 选定：RLS 做强租户隔离（`DEV-PLAN-021`），Casbin 做“是否允许做事”（Authz）。两者边界必须明确：**RLS 圈地 != Casbin 授权**。
- 主体模型在 `DEV-PLAN-019` 已选定为 `principal`（而非 `user`）。本计划必须冻结“principal（审计） vs role（授权）”的输入语义，避免后续改名或双轨并存。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标

- [X] 冻结 Authz 合同：`subject/object/action/domain` 的命名规范与不变量（用于 code review、lint、测试断言）。
- [X] 明确 policy SSOT 与发布口径（baseline = Git 管理 + pack），并定义“生产可复现”的约束。
- [X] 给出模块级接入模板（controller/service 如何调用 `pkg/authz` 门面、以及 403/forbidden 契约）。
- [X] 形成本仓库 Authz 工具链门禁清单（触发器、CI 入口、生成物与验收标准），避免实现期临时拼装。

009M1 落地（最小可拒绝）：
- `pkg/authz/*`：Authorizer 门面与 mode 语义（`disabled|shadow|enforce`）
- `internal/server/authz_middleware.go`：统一 403/forbidden 契约
- policy SSOT/生成物：`config/access/policies/00-bootstrap.csv` → `config/access/policy.csv`、`config/access/policy.csv.rev`
- 证据：`docs/dev-records/DEV-PLAN-010-READINESS.md`（第 10 节）

### 2.2 非目标（明确不做）

- 不在本计划内交付企业 SSO 或多 IdP 编排；相关工作属于 AuthN/SSO 范畴（参考 `DEV-PLAN-019` 的 AuthN/Session 边界）。
- 不在本计划内引入“复杂 ABAC DSL/表达式”或策略编排语言；仅保留最小 ABAC 字段（如确需）。
- 不在本计划内迁移/兼容旧系统（或旧仓库）的 policy/roles；以最小角色集重新定义。
- 不在本计划内新增数据库表存储 policy（避免把配置与数据耦合、并触发迁移门禁）；baseline 以文件策略为主。
- 不在本计划内引入 Casbin `g/g2`（role 继承/组/多角色）。如确需，必须另起 dev-plan 并给出迁移与回滚。

## 3. 工具链与门禁（SSOT 引用）

> 本计划不复制命令矩阵；触发器与门禁以 SSOT 为准。

- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`（说明见 `docs/dev-plans/012-ci-quality-gates.md`）
- Casbin 模型：`config/access/model.conf`
- Policy 碎片（SSOT）：`config/access/policies/**`
- Pack 产物：`config/access/policy.csv`、`config/access/policy.csv.rev`
- 打包/校验脚本：`scripts/authz/pack.sh`、`scripts/authz/test.sh`、`scripts/authz/lint.sh`
- Tenancy/AuthN 与主体模型（principal）：`docs/dev-plans/019-tenant-and-authn.md`
- RLS 强租户隔离：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- 路由治理与 responder 契约：`docs/dev-plans/017-routing-strategy.md`
- 技术栈与工具链版本（Casbin 版本基线等）：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- Simple > Easy 评审口径：`docs/dev-plans/003-simple-not-easy-review-guide.md`

## 4. 关键决策（ADR 摘要）

### 4.1 继续采用 Casbin（选定）

- **选定**：继续采用 Casbin 作为授权引擎（以 role 为主体的 RBAC + domains；ABAC 仅在明确需要时引入），沿用本仓库 `config/access/model.conf` 的“输入四元组”思路（`sub/dom/obj/act`）。
- **理由**：工具链与门禁已具备（pack/lint/test）；更需要“冻结契约、防漂移”，而非替换引擎。

### 4.2 Domain 语义（选定：tenant UUID / global）

- **选定**：Casbin `domain` 仅有两类：
  - 租户域：`strings.ToLower(tenant_id.String())`
  - 全局域：`global`
- **禁止**：把模块名、路由 segment、hostname 写入 domain（避免把“部署形态/路由形态”误当成授权边界）。

### 4.3 Subject 语义（选定：role 为授权主体；principal 为审计标识）

> `DEV-PLAN-019` 选定“本地主体”为 `principal`。但若把“principal↔role 绑定”写进 Casbin policy，会导致“创建用户/租户 = 修改 policy 文件”的运维耦合。为保持简单性：**MVP 的 Casbin 授权以 role 为主体**；principal 仅作为审计/日志/诊断标识。

- **审计标识（principal id，非 Casbin Enforce 输入）**：
  - tenant principal：`tenant:{tenant_id}:principal:{principal_id}`
  - global principal（仅控制面）：`global:principal:{principal_id}`
- **授权主体（Effective Subject，Casbin Enforce 输入）**：
  - `role:{slug}`（全小写，slug 作为稳定标识）
- **MVP 约束（选定）**：
  - 每个 session 恰好一个 `role_slug`（不做多角色 OR 判定）
  - 匿名请求使用 `role:anonymous`（由 AuthN/Session 层判定并注入）

### 4.4 Object 命名（选定：module.resource）

- **选定**：object 采用 `module.resource`（全小写）。
- **粒度（选定，MVP）**：以“业务资源级”作为 object 粒度（例如 `orgunit.orgunits`、`jobcatalog.catalog`），避免按 endpoint/page 细碎拆分导致策略爆炸与漂移。
- **禁止**：把 HTTP method/path 片段、页面组件名、query params 等写入 object（它们属于路由与展示层细节，不是稳定授权边界）。
- **模块建议前缀**（与 `DEV-PLAN-016/019` 对齐）：
  - `iam.*`（tenancy/authn/session/principal 等平台域）
  - `orgunit.*`
  - `jobcatalog.*`
  - `staffing.*`
  - `person.*`
  - `superadmin.*`（仅控制面；与 tenant app 隔离）

### 4.5 Action 命名（选定：最小动词集合）

- **选定**：action 以最小集合起步，避免同义动词泛滥：
  - MVP 允许：`read/admin/debug`
  - 预留（非 MVP）：`create/update/delete`（禁止在 MVP 的实际授权判断中启用）
  - 管理：`admin`
  - 诊断（仅 debug/诊断端点）：`debug`
- 后续新增 action 必须在对应 dev-plan 中声明（并补齐 policy + lint + 测试），不得在代码里“随手造词”。

### 4.6 Policy SSOT 与发布方式（选定：Git 管理 + pack）

- **选定（baseline）**：policy 以 Git 管理为 SSOT：
  - 源文件：`config/access/policies/**`（按模块拆分）
  - 生成物：`config/access/policy.csv` 与 `config/access/policy.csv.rev`（由 pack 生成，必须提交）
- **一致性门禁（选定）**：CI 在 `make authz-pack` 后必须执行：
  - `git diff --exit-code -- config/access/policy.csv config/access/policy.csv.rev`
- **暂不纳入**：管理员在线 Apply 作为 MVP 的必选能力。若未来引入，必须补齐“容器内写文件的持久化策略、审计与回滚”，另起子计划（暂定命名：DEV-PLAN-022A）。

### 4.7 与 RLS 的边界（选定：分层防御）

- RLS（`DEV-PLAN-021`）：只负责“同租户可见性”（圈地），不表达“是否允许操作”。
- Casbin：只负责“是否允许做事”（按 `subject/object/action/domain`），不得替代 tenant 解析或 RLS 注入。
- **禁止**：为了 superadmin 跨租户需求放宽 RLS policy；跨租户必须走控制面边界与专用 DB role（对齐 `DEV-PLAN-019`）。

### 4.8 运行态模式（选定：`AUTHZ_MODE` + `authz_flags.yaml`，无 segments）

> 目标：给实现一个单一可解释的“开关”，并且行为可测、可回滚。

- **选定**：三态模式：`disabled|shadow|enforce`。默认从 `config/access/authz_flags.yaml` 读取，环境变量 `AUTHZ_MODE` 可覆盖配置文件。
- **SSOT**：`config/access/authz_flags.yaml` 仅允许包含 `mode` 字段；**禁止**出现 `segments` 等扩展字段（避免“写了但运行时不生效”的漂移）。
- **安全约束（选定）**：`AUTHZ_MODE=disabled` 属于危险开关；除本地排障外不得使用。实现侧必须要求同时显式设置 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 才允许启动，否则 fail-fast（避免误配导致“全放行”）。
- **落点（选定）**：fail-fast 必须发生在所有 server/binary 的启动入口（不依赖“环境识别”），避免某个入口漏掉导致旁路。
- 行为合同：

| mode | denied 时的行为 | 记录缺口（missing policy） | 典型用途 |
| --- | --- | --- | --- |
| `disabled` | 不做授权判断 | 否 | 本地排障/短期止血（需 `AUTHZ_UNSAFE_ALLOW_DISABLED=1`） |
| `shadow` | 不中断请求（继续执行） | 是（日志/诊断） | 新模块接入期 |
| `enforce` | 直接拒绝（统一 403） | 是（日志/诊断） | 默认（生产） |

### 4.9 最小角色与策略包（选定：3 角色 + 只用 `read/admin/debug`）

> 目标：先把“能跑通的最小权限闭环”冻结为可执行规格；后续新增能力必须显式扩展本表。

- **选定角色**：
  - `role:superadmin`（控制面）
  - `role:tenant_admin`（租户管理员）
  - `role:tenant_viewer`（租户只读）
  - `role:anonymous`（匿名；仅用于**明确白名单**的必要资源）
- **动作口径（MVP）**：只允许使用 `read/admin/debug`。

策略矩阵（MVP，建议从此起步）：

| object（module.resource） | `tenant_viewer` | `tenant_admin` | `superadmin` | `anonymous` |
| --- | --- | --- | --- | --- |
| `orgunit.orgunits` | `read` | `read, admin` | — | — |
| `jobcatalog.catalog` | `read` | `read, admin` | — | — |
| `staffing.positions` | `read` | `read, admin` | — | — |
| `staffing.assignments` | `read` | `read, admin` | — | — |
| `person.persons` | `read` | `read, admin` | — | — |
| `superadmin.tenants` | — | — | `read, admin` | — |
| `superadmin.authz`（可选） | — | — | `debug` | — |
| `iam.ping`（示例） | — | — | `read` | `read` |

**匿名白名单（选定，MVP）**
- 仅允许 `role:anonymous` 访问“明确列入 policy 的 object/action”，不得因为“没有登录”而在 handler 内做隐式放行。
- MVP 默认仅允许：`iam.ping/read`。
- 当新增登录/会话创建等“必须匿名可达”的入口时：必须为其定义稳定 object（例如 `iam.session`/`iam.login` 等）并在 policy 中显式加入 `role:anonymous` 允许项；否则 `AUTHZ_MODE=enforce` 下应当拒绝（fail-closed）。

## 5. 本仓库落地形态（目录与产物）

> 说明：本仓库已存在 `config/access/*` 与 `scripts/authz/*` 的最小闭环；本节描述“需要补齐/冻结”的增量目标。
- `pkg/authz/**`：authz 门面（输入规范化、enforcer 构造、授权调用、缺口诊断、403 输出适配）。
- `config/access/model.conf`：Casbin 模型。
- `config/access/policies/**`：策略碎片（模块维度）。
- `config/access/policy.csv`、`config/access/policy.csv.rev`：聚合产物（pack 生成）。
- `scripts/authz/**`：pack/lint/test 辅助脚本（以 `Makefile` 为入口）。
- `config/access/authz_flags.yaml`：运行态 mode（本计划新增；仅允许 `mode`）。

### 5.1 Authz API 与 403/日志契约（冻结）

> 目标：让实现“对齐规格”而不是“撞出来”；让 reviewer 能只看本节就判断是否出现多套权威表达。

**输入四元组（Casbin Enforce 输入）**
- `subject`：`role:{slug}`（全小写；由 session 的 `role_slug` 映射得到；MVP 不支持多角色 OR 判定）
- `domain`：tenant UUID（lowercase string）或 `global`
- `object`：`module.resource`（全小写）
- `action`：MVP 仅允许 `read|admin|debug`（`create|update|delete` 保留但默认不启用）

**审计标识（不参与 Enforce）**
- `principal_id`：仅用于日志/审计/诊断；不得把 principal 作为 Casbin subject，以免引入“用户数据变更 = policy 文件变更”的运维耦合（见 §4.3）。

**推荐门面（供实现对齐；命名可调但语义必须一致）**
- `Authorize(ctx, Request) (Decision, error)`：纯判定 + 缺口诊断（支持 `disabled|shadow|enforce` 三态）。
- `Require(ctx, Request) error`：便于 handler 使用；在 `enforce` 且 deny 时返回一个可稳定映射为 HTTP 403 的错误（HTML/HTMX/JSON 的具体渲染由全局 responder 统一负责）。

**403 合同（对外）**
- HTTP 状态码固定为 `403`；禁止在各模块自造“看起来不一样”的 forbidden payload（由全局 responder/组件统一渲染，对齐 `DEV-PLAN-017`）。
- 默认不在响应体回显 `subject/domain/object/action`（减少内部策略细节泄露面），但必须在日志中记录以便补齐策略。

**日志与缺口诊断契约（冻结）**
- 记录字段（最小集）：`request_id`（或等价链路 id）、`method`、`path`、`principal_id`、`role_slug`、`tenant_id`（如有）、`domain`、`object`、`action`、`mode`、`decision`（allow/deny）、`policy_rev`（`config/access/policy.csv.rev` 的值）。
- 缺口（missing policy）的口径：指“在当前 `subject/domain/object/action` 下无匹配策略导致 deny”；与“enforcer/加载失败”等系统错误区分开。

**Policy 形态约束（MVP）**
- 禁止使用 Casbin `g/g2`（MVP 不做 role 继承/组/多角色）；如确需，必须先扩展模型、补齐 lint/test，并另起 dev-plan 声明迁移与回滚。
- policy 中 `act` 仅允许 `read|admin|debug`；出现 `create/update/delete` 视为“未按本计划冻结的契约扩展”，必须先走 dev-plan。

### 5.2 object/action 映射的权威来源（选定）

> 目标：防止“每个模块一套字符串拼装规则”造成漂移。

- **权威来源（选定）**：object/action 的“命名与映射表”以 `pkg/authz` 的集中 registry/常量为权威（代码中集中定义、集中注册）。
- **模块使用方式（选定）**：模块侧只能调用 helper（例如 `pkg/authz` 提供的 `RequireReadXxx/RequireAdminXxx` 或 `MapRouteToObjectAction`），禁止在模块内拼装 `module.resource` 或自造 action 字符串。
- **变更规则（选定）**：新增受保护入口（页面/API/HTMX）必须同时更新：
  - object/action registry（代码）
  - 对应 policy（`config/access/policies/**` + pack 产物）
  - lint/test（确保不出现 `g,` 行与非 MVP action）

### 5.3 授权主流程与失败路径（10 句可复述）

> 目标：reviewer 能用 5 分钟复述“为什么会放行/为什么会拒绝”，而不需要翻多处实现细节。

1. 请求进入后，先由 AuthN/session 中间件解析 `principal_id` 并确定 `role_slug`（匿名为 `anonymous`；superadmin 控制面使用其专用 principal）。
2. tenant app 的请求必须先解析 tenant（Host → `tenant_id`，fail-closed），并把 `tenant_id` 放入上下文（对齐 `DEV-PLAN-019`）。
3. 计算 Casbin `domain`：tenant app 用 `strings.ToLower(tenant_id.String())`；superadmin 控制面固定为 `global`。
4. 计算 Casbin `subject`（Effective Subject）：`role:{slug}`（MVP 单角色）。
5. 计算 `object/action`：由 `pkg/authz` registry/helper 映射得到（MVP action 仅 `read/admin/debug`）。
6. 调用 `authz.Authorize(ctx, Request{subject, domain, object, action})` 得到 allow/deny（由 `AUTHZ_MODE` 决定是否阻断）。
7. `AUTHZ_MODE=enforce` 且 deny：返回统一 403，并记录缺口（missing policy）。
8. `AUTHZ_MODE=shadow` 且 deny：不阻断请求，但记录缺口（日志/诊断），用于补齐策略与收敛 object/action 漂移。
9. `AUTHZ_MODE=disabled`：跳过授权判断（仅用于本地排障/短期止血；需 `AUTHZ_UNSAFE_ALLOW_DISABLED=1`）。
10. 无论 Casbin 是否放行，RLS 仍是租户数据隔离的最终兜底；任何跨租户旁路必须走控制面边界与专用 DB role（`DEV-PLAN-021/019`）。

## 6. 实施步骤（Plan → Implement）

1. [ ] 落地 `pkg/authz`：实现 helper（必要时集中 registry/常量）+ `Authorize/Require` 门面；实现 `AUTHZ_MODE` 三态与 `AUTHZ_UNSAFE_ALLOW_DISABLED` fail-fast（覆盖所有 server/binary 入口）。
2. [ ] 落地 `authz_flags.yaml`：新增 `config/access/authz_flags.yaml`（仅 `mode`），实现读取并允许 `AUTHZ_MODE` 覆盖配置；禁止 `segments` 等扩展字段。
3. [ ] 落地 policy SSOT：在现有 `config/access/model.conf`、`config/access/policies/**`、pack 生成物基础上，补齐 CI “生成物必须已提交”的检查（防止漏提交）。
4. [ ] 强化脚本门禁：
   - [ ] `scripts/authz/lint.sh` 增加 MVP 约束校验（禁止 `g,` 行；`act` 仅允许 `read|admin|debug`）。
   - [ ] `scripts/authz/test.sh`（或 CI 步骤）在 `make authz-pack` 后执行 `git diff --exit-code -- config/access/policy.csv config/access/policy.csv.rev`，确保生成物与源码一致且已提交。
5. [ ] 接入最小授权点：
   - [ ] `modules/iam`：tenant console（创建/禁用租户、绑定域名、bootstrap）—— 仅 superadmin 可用。
   - [ ] HR 4 模块 UI/API（`orgunit/jobcatalog/staffing/person`）的 read/admin 最小集。
6. [ ] 统一 403/forbidden 输出契约：控制器侧不自造 JSON/HTML；统一走全局 responder/通用组件（对齐 `DEV-PLAN-017`）；响应体不回显 `subject/domain/object/action`。
7. [ ] 落地匿名白名单（MVP）：保证 `role:anonymous` 仅访问 policy 明确列出的入口（至少 `iam.ping/read`）；任何新增匿名入口必须先定义稳定 object 并显式加 policy。
8. [ ] 文档与门禁对齐：确保 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 所述 Authz gates 与实际 `Makefile/scripts/authz/*` 一致，避免“文档说一套、CI 跑一套”。

## 7. 测试与覆盖率（Go 代码门禁）

> 对齐 `DEV-PLAN-019` 的 100% 覆盖率要求：Authz 代码必须通过“可测性设计”达成，而不是靠豁免目录。

- 覆盖率口径（待本仓库 SSOT 冻结）：默认 line coverage；统计范围应包含 `pkg/authz` 与各模块的授权接入层。
- 排除项原则：仅允许排除生成代码/第三方；不允许排除“难测的业务分支”。
- 最小用例集（必须覆盖）：
  - `role_slug → role:{slug}` 的映射规则（MVP 单角色）
  - subject/domain/object/action 的 normalize 与不变量校验
  - `AUTHZ_MODE` 三态行为（disabled/shadow/enforce）与 `AUTHZ_UNSAFE_ALLOW_DISABLED` fail-fast
  - allow/deny 与错误映射（含 missing policy 的诊断信息）
  - tenant app 与 superadmin 的边界：不同 domain、不同 object 前缀不可互相放行

## 8. 风险与缓解

- **命名漂移**（principal vs user）：缓解——本计划在 §4.3 冻结为 principal（审计）与 role（授权），并要求 helpers（必要时集中 registry/常量）统一推导。
- **策略分散**（每模块自造 object/action）：缓解——冻结 module 前缀与 action 最小集合；新增必须走 dev-plan；实现侧通过 stopline 阻断模块自拼字符串。
- **误配全放行**（`AUTHZ_MODE=disabled`）：缓解——必须 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 显式解锁，否则 fail-fast；并要求单测覆盖。
- **运维复杂度**（在线 apply/写文件）：缓解——baseline 不纳入 apply；需要时另起子计划（暂定命名：DEV-PLAN-022A）并补齐持久化与回滚。

## 9. 验收标准（本计划完成定义）

- [ ] `subject/object/action/domain` 命名规范在文档与代码中一致，并有测试兜底。
- [ ] policy SSOT 清晰：策略碎片可追踪、聚合产物可复现、CI 能阻止漏提交。
- [ ] 最小授权闭环可演示：登录（DEV-PLAN-019）→ 进入租户 → 访问受保护页面/API → 无权返回统一 403。
- [ ] 匿名入口符合 §4.9：`role:anonymous` 仅可访问白名单（MVP 至少 `iam.ping/read`），新增匿名入口必须显式加 policy 且走 registry。
- [ ] 403 输出契约符合 §5.1：统一 403，响应体不回显 `subject/domain/object/action`，但日志具备补齐策略所需信息（含 `policy_rev`、`request_id/method/path`）。
- [ ] policy 契约符合 §5.1：MVP 不出现 `g,` 行，且 `act` 仅包含 `read|admin|debug`。
- [ ] `AUTHZ_MODE` 三态行为符合 §4.8：`disabled|shadow|enforce` 可测；`AUTHZ_MODE=disabled` 必须由 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 显式解锁，否则 fail-fast；`authz_flags.yaml` 不允许出现 `segments`。
- [ ] 触发器与门禁在本仓库可执行（以 SSOT：`Makefile`/CI workflow 为准），且 pack 产物一致性通过 `git diff --exit-code -- config/access/policy.csv config/access/policy.csv.rev` 证明。

## 10. Simple > Easy Review（DEV-PLAN-003）

### 10.1 边界
- Authz 只做“是否允许做事”；RLS 只做“同租户可见性”；不要互相越界代偿。

### 10.2 不变量
- domain 只能是 tenant UUID / global。
- subject 必须是 `role:{slug}`；principal 只用于审计/日志，不参与 Enforce。
- object 必须是 `module.resource` 且粒度为业务资源级；action 在 MVP 只能是 `read|admin|debug`。
- `role:anonymous` 只能访问 policy 显式白名单。

### 10.3 停止线（命中即打回）
- [ ] 在模块里手写 `subject/domain/object/action` 推导（应复用 `pkg/authz` helpers；必要时集中 registry/常量）。
- [ ] 不经 `pkg/authz` 的常量/集中定义而直接在模块里手写 `module.resource` 或 action 字符串。
- [ ] 用放宽 RLS policy 实现跨租户控制面需求（必须走控制面边界与专用 role）。
- [ ] 任何模块自造 forbidden payload（应统一走全局 responder/组件）；或在 403 响应体回显 `subject/domain/object/action`。
- [ ] policy 中出现 `g,`/`g2,`（MVP 不做 role 继承/组/多角色）。
- [ ] 在 policy 或代码中使用 `create/update/delete` 作为实际授权动作，而未先更新本 dev-plan 并补齐 lint/test。
- [ ] `AUTHZ_MODE=disabled` 未被 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 显式解锁却仍能启动（必须 fail-fast）。
- [ ] 引入 `segments` 或其他“看起来可配置但运行时不生效”的 flags 扩展（必须先实现并加测试，再引入配置）。
- [ ] 引入第二套 policy SSOT（例如同时以 DB 与 Git 为权威）而无明确迁移与回滚计划。
