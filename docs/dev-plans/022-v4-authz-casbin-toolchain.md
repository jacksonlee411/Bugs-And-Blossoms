# DEV-PLAN-022：V4 Authz（Casbin）工具链与实施方案（Greenfield）

**状态**: 草拟中（2026-01-05 08:57 UTC）

> 适用范围：本仓库（`Bugs-And-Blossoms`）的 **V4 Greenfield implementation**。如未来拆分到独立仓库，本计划冻结的 Authz 契约与工具链口径仍应保持一致，避免“同一概念两套权威表达”。  
> 本文冻结 V4 的授权（Authz）契约与工具链口径：`subject/object/action/domain` 命名、policy 的 SSOT 与发布方式、CI 门禁，以及与 `DEV-PLAN-021/019`（RLS/AuthN/Tenancy）的边界关系，避免实现期“各模块各写一套”导致漂移。

## 1. 背景与上下文 (Context)

- V4 选择 Greenfield 全新实施（路线图见 `DEV-PLAN-009`），不承担存量 `user.Can`/旧权限映射的兼容包袱。
- 本仓库已具备最小 Casbin 工具链与门禁：`config/access/*` + `scripts/authz/*` + `make authz-pack/authz-test/authz-lint`（CI 入口见 `.github/workflows/quality-gates.yml`）。V4 的主体模型在 `DEV-PLAN-019` 已选定为 `principal`（而非 `user`），需要尽早冻结命名与边界，避免后续改名或双轨并存。
- V4 同时选定：RLS 做强租户隔离（`DEV-PLAN-021`），Casbin 做“是否允许做事”（Authz），两者边界必须明确：**RLS 圈地 != Casbin 授权**。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标

- [ ] 冻结 V4 Authz 合同：`subject/object/action/domain` 的命名规范与不变量（可用于 code review 与测试断言）。
- [ ] 给出 V4 的 policy SSOT 与发布口径（Git 管理 vs Apply API），并明确“生产可复现”的约束。
- [ ] 给出模块级接入模板（controller/service 如何调用 `authz.Authorize`，以及 403/forbidden payload 口径）。
- [ ] 形成 V4 的工具链门禁清单（触发器、CI 入口、生成物与验收标准），避免实现期临时拼装。

### 2.2 非目标（明确不做）

- 不在本计划内交付企业 SSO 或多 IdP 编排；相关工作属于 AuthN/SSO 范畴（参考 `DEV-PLAN-019` 的 AuthN/Session 边界）。
- 不在本计划内引入“复杂 ABAC DSL/表达式”或策略编排语言；仅保留最小 ABAC 字段（如确需）。
- 不在本计划内迁移/兼容旧系统（或旧仓库）的 policy/roles 到 V4；V4 以最小角色集重新定义。
- 不在本计划内新增数据库表存储 policy（避免把配置与数据耦合、并触发迁移门禁）；V4 baseline 以文件策略为主。

## 3. 工具链与门禁（SSOT 引用）

> 本计划不复制命令矩阵；触发器与门禁以 SSOT 为准。

- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- Casbin 模型：`config/access/model.conf`
- Policy 碎片（SSOT）：`config/access/policies/**`
- Pack 产物：`config/access/policy.csv`、`config/access/policy.csv.rev`
- 打包/校验脚本：`scripts/authz/pack.sh`、`scripts/authz/test.sh`、`scripts/authz/lint.sh`
- V4 Tenancy/AuthN 与主体模型（principal）：`docs/dev-plans/019-tenant-and-authn-v4.md`
- V4 RLS 强租户隔离：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog-v4.md`
- V4 技术栈与工具链版本（Casbin 版本基线等）：`docs/dev-plans/011-v4-tech-stack-and-toolchain-versions.md`

## 4. 关键决策（ADR 摘要）

### 4.1 继续采用 Casbin（选定）

- **选定**：V4 继续采用 Casbin 作为授权引擎（以 role 为主体的 RBAC + domains；ABAC 仅在明确需要时引入），沿用本仓库 `config/access/model.conf` 的“输入四元组”思路（`sub/dom/obj/act`）。
- **理由**：工具链与门禁已具备（pack/lint/test）；V4 更需要“冻结契约、防漂移”，而非替换引擎。

### 4.2 Domain 语义（选定：tenant UUID / global）

- **选定**：Casbin `domain` 仅有两类：
  - 租户域：`strings.ToLower(tenant_id.String())`
  - 全局域：`global`
- **禁止**：把模块名、路由 segment、hostname 写入 domain（避免把“部署形态/路由形态”误当成授权边界）。

### 4.3 Subject 语义（选定：role 为授权主体；principal 为审计标识）

> `DEV-PLAN-019` 选定“本地主体”为 `principal`。但 V4 Greenfield 若把“principal↔role 绑定”也写进 Casbin policy（`g`/`g2`），会导致“创建用户/租户 = 修改 policy 文件”的运维耦合。为保持简单性：**V4 MVP 的 Casbin 授权以 role 为主体**；principal 仍保留为审计/日志/诊断的稳定标识。

- **审计标识（principal id，非 Casbin Enforce 输入）**：
  - tenant principal：`tenant:{tenant_id}:principal:{principal_id}`
  - global principal（仅控制面）：`global:principal:{principal_id}`
- **授权主体（Effective Subject，Casbin Enforce 输入）**：
  - role：`role:{slug}`（全小写，slug 作为稳定标识）
- **V4 MVP 约束（选定以保持简单）**：每个 session 恰好一个 `role_slug`（不做多角色 OR 判定）；如未来需要“多角色/继承/组”，另起子计划并给出边界与回滚。
- **不做兼容**：不支持 `tenant:{id}:user:{id}` 作为 V4 输入格式；如未来需要兼容旧系统迁移，另起计划并明确双轨期限。

### 4.4 Object 命名（选定：module.resource）

- **选定**：object 采用 `module.resource`（全小写）。
- **V4 模块建议前缀**（与 `DEV-PLAN-016/019` 对齐）：
  - `iam.*`（tenancy/authn/session/principal 等平台域）
  - `orgunit.*`
  - `jobcatalog.*`
  - `staffing.*`
  - `person.*`
  - `superadmin.*`（仅控制面；与 tenant app 隔离）

### 4.5 Action 命名（选定：最小动词集合）

- **选定**：action 以最小集合起步，避免同义动词泛滥：
  - MVP 允许：`read/admin/debug`
  - 预留（非 MVP）：`create/update/delete`
  - 管理：`admin`
  - 诊断（仅 debug/诊断端点）：`debug`
- 后续新增 action 必须在对应 dev-plan 中声明（并补齐 policy + 测试），不得在代码里“随手造词”。

### 4.6 Policy SSOT 与发布方式（选定：Git 管理 + pack）

- **选定（V4 baseline）**：policy 以 Git 管理为 SSOT：
  - 源文件：`config/access/policies/**`（按模块拆分）
  - 生成物：`config/access/policy.csv` 与 `config/access/policy.csv.rev`（由 pack 生成，必须提交）
- **暂不纳入**：管理员在线 Apply 作为 V4 MVP 的必选能力。若未来引入，必须补齐“容器内写文件的持久化策略、审计与回滚”，另起子计划（暂定命名：DEV-PLAN-022A）。

### 4.7 与 RLS 的边界（选定：分层防御）

- RLS（`DEV-PLAN-021`）：只负责“同租户可见性”（圈地），不表达“是否允许操作”。
- Casbin：只负责“是否允许做事”（按 subject/object/action/domain），不得替代 tenant 解析或 RLS 注入。
- **禁止**：为了 superadmin 跨租户需求放宽 RLS policy；跨租户必须走控制面边界与专用 DB role（对齐 `DEV-PLAN-019`）。

### 4.8 运行态模式（选定：`AUTHZ_MODE` + `authz_flags.yaml`，无 segments）

> 目标：给实现一个单一可解释的“开关”，并且行为可测、可回滚。

- **选定**：三态模式：`disabled|shadow|enforce`。默认从 `config/access/authz_flags.yaml` 读取，环境变量 `AUTHZ_MODE` 可覆盖配置文件。
- **SSOT**：`config/access/authz_flags.yaml` 仅允许包含 `mode` 字段；**禁止**出现 `segments` 等扩展字段（避免“写了但运行时不生效”的漂移）。
- **安全约束（选定）**：`AUTHZ_MODE=disabled` 属于危险开关；除本地排障外不得使用。实现侧必须要求同时显式设置 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 才允许启动，否则 fail-fast（避免误配导致“全放行”）。
- 行为合同：

| mode | denied 时的行为 | 记录缺口（missing policy） | 典型用途 |
| --- | --- | --- | --- |
| `disabled` | 不做授权判断 | 否 | 本地排障/短期止血 |
| `shadow` | 不中断请求（继续执行） | 是（日志/诊断） | 新模块接入期 |
| `enforce` | 直接拒绝（统一 403） | 是（日志/诊断） | 默认（生产） |

### 4.9 V4 最小角色与策略包（选定：3 角色 + 只用 `read/admin/debug`）

> 目标：V4 先把“能跑通的最小权限闭环”冻结为可执行规格；后续新增能力必须显式扩展本表。

- **选定角色**：
  - `role:superadmin`（控制面）
  - `role:tenant_admin`（租户管理员）
  - `role:tenant_viewer`（租户只读）
- **动作口径（MVP）**：只允许使用 `read/admin/debug`；`create/update/delete` 保留但不在 MVP 使用（避免早期拆得过细造成策略爆炸与漂移）。

策略矩阵（MVP，建议从此起步）：

| object（module.resource） | `tenant_viewer` | `tenant_admin` | `superadmin` |
| --- | --- | --- | --- |
| `orgunit.nodes` | `read` | `read, admin` | — |
| `jobcatalog.catalog` | `read` | `read, admin` | — |
| `staffing.positions` | `read` | `read, admin` | — |
| `staffing.assignments` | `read` | `read, admin` | — |
| `person.persons` | `read` | `read, admin` | — |
| `superadmin.tenants` | — | — | `read, admin` |
| `superadmin.authz`（可选） | — | — | `debug` |

## 5. 本仓库落地形态（目录与产物）

> 说明：本仓库已存在 `config/access/*` 与 `scripts/authz/*` 的最小闭环；本节描述“V4 需要补齐/冻结”的增量目标。

- `pkg/authz/**`：V4 authz 门面（输入规范化、enforcer 构造、授权调用、缺口诊断、403 输出适配）。
- `config/access/model.conf`：Casbin 模型。
- `config/access/policies/**`：策略碎片（模块维度）。
- `config/access/policy.csv`、`config/access/policy.csv.rev`：聚合产物（pack 生成）。
- `scripts/authz/**`：pack/lint/verify 辅助脚本（以 `Makefile` 为入口）。
- `config/access/authz_flags.yaml`：运行态 mode（本计划新增；仅允许 `mode`）。

### 5.1 Authz API 与 403 输出契约（冻结）

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
- HTTP 状态码固定为 `403`；禁止在各模块自造“看起来不一样”的 forbidden payload。
- 默认不在响应体回显 `subject/domain/object/action`（减少内部策略细节泄露面），但必须在日志中记录以便补齐策略。

**日志与缺口诊断契约（冻结）**
- 记录字段（最小集）：`principal_id`、`role_slug`、`tenant_id`（如有）、`domain`、`object`、`action`、`mode`、`decision`（allow/deny）、`policy_rev`（`config/access/policy.csv.rev` 的值）。
- 缺口（missing policy）的口径：指“在当前 `subject/domain/object/action` 下无匹配策略导致 deny”；与“enforcer/加载失败”等系统错误区分开。

**Policy 形态约束（MVP）**
- 禁止使用 Casbin `g/g2`（MVP 不做 role 继承/组/多角色）；如确需，必须先扩展模型、补齐 lint/test，并另起 dev-plan 声明迁移与回滚。
- policy 中 `act` 仅允许 `read|admin|debug`；出现 `create/update/delete` 视为“未按本计划冻结的契约扩展”，必须先走 dev-plan。

### 5.2 授权主流程与失败路径（10 句可复述）

> 目标：reviewer 能用 5 分钟复述“为什么会放行/为什么会拒绝”，而不需要翻多处实现细节。

1. 请求进入后，先由 AuthN/session 中间件解析 `principal_id`（或 superadmin principal），并明确是否为匿名。
2. tenant app 的请求必须先解析 tenant（Host → tenant_id，fail-closed），并把 `tenant_id` 放入上下文（对齐 `DEV-PLAN-019`）。
3. 计算 Casbin `domain`：tenant app 用 `strings.ToLower(tenant_id.String())`；superadmin 控制面固定为 `global`。
4. 计算 Casbin `subject`（Effective Subject）：从 session 读取 `role_slug`，映射为 `role:{slug}`（MVP 单角色）。
5. 计算 `object/action`：由模块级 helper 把路由/handler 映射为固定的 `module.resource` + `read/admin/debug`（对齐 §4.9）。
6. 调用 `authz.Authorize(ctx, Request{subject, domain, object, action})` 得到 allow/deny（由 `AUTHZ_MODE` 决定是否阻断）。
7. `AUTHZ_MODE=enforce` 且 deny：直接返回统一 403（payload/组件口径统一），并记录缺口（missing policy）。
8. `AUTHZ_MODE=shadow` 且 deny：不阻断请求，但记录缺口（日志/诊断），用于补齐策略与收敛 object/action 漂移。
9. `AUTHZ_MODE=disabled`：跳过授权判断（仅用于本地排障/短期止血）；不得在生产作为常态。
10. 无论 Casbin 是否放行，RLS 仍是租户数据隔离的最终兜底；任何跨租户旁路必须走控制面边界与专用 DB role（`DEV-PLAN-021/019`）。

## 6. 实施步骤（Plan → Implement）

1. [ ] 冻结 contracts：在本仓库落地 `pkg/authz` 的 V4 版本（principal 作为审计标识 + role 作为 Effective Subject），并补齐单测覆盖命名规范与 normalize 行为。
2. [ ] 落地 policy SSOT：在现有 `config/access/model.conf`、`config/access/policies/**`、pack 生成物基础上，补齐 CI“生成物必须已提交”的检查（防止漏提交）；同时新增 `config/access/authz_flags.yaml`（仅 `mode`）。
3. [ ] 接入最小授权点：
   - [ ] `modules/iam`：tenant console（创建/禁用租户、绑定域名、bootstrap）—— 仅 superadmin 可用。
   - [ ] HR 4 模块 UI/API（`orgunit/jobcatalog/staffing/person`）的 read/admin 最小集。
4. [ ] 统一 403/forbidden 输出契约：控制器侧不自造 JSON/HTML；统一走 `pkg/serrors`/通用组件（沿用现仓库口径）。
5. [ ] 建立可复用的“模块 authz helpers”模板，避免各模块自写 subject/domain/object/action 推导逻辑。
6. [ ] 强化脚本门禁：
   - [ ] `scripts/authz/lint.sh` 增加 MVP 约束校验（禁止 `g,` 行；`act` 仅允许 `read|admin|debug`）。
   - [ ] `scripts/authz/test.sh`（或 CI 步骤）在 `make authz-pack` 后执行 `git diff --exit-code -- config/access/policy.csv config/access/policy.csv.rev`，确保生成物与源码一致且已提交。
7. [ ] 文档与门禁对齐：确保 `AGENTS.md` 与 `docs/dev-plans/012-v4-ci-quality-gates.md` 所述 Authz gates 与实际 `Makefile/scripts/authz/*` 一致，避免“文档说一套、CI 跑一套”。

## 7. 测试与覆盖率（Go 代码门禁）

> 对齐 `DEV-PLAN-019` 的 100% 覆盖率要求：Authz 代码必须通过“可测性设计”达成，而不是靠豁免目录。

- 覆盖率口径（待本仓库 SSOT 冻结）：默认 line coverage；统计范围应包含 `pkg/authz` 与各模块的授权接入层。
- 排除项原则：仅允许排除生成代码/第三方；不允许排除“难测的业务分支”。
- 最小用例集（必须覆盖）：
  - `role_slug → Effective Subject` 的映射规则（MVP 单角色）
  - subject/domain/object/action 的 normalize 与不变量校验
  - `AUTHZ_MODE` 三态行为（disabled/shadow/enforce）
  - allow/deny 与错误映射（含 missing policy 的诊断信息，若保留）
  - tenant app 与 superadmin 的边界：不同 domain、不同 object 前缀不可互相放行

## 8. 风险与缓解

- **命名漂移**（principal vs user）：缓解——本计划在 §4.3 冻结为 principal，并要求 helpers 模板统一推导。
- **策略分散**（每模块自造 object/action）：缓解——本计划冻结 module 前缀与 action 最小集合；新增必须走 dev-plan。
- **运维复杂度**（apply 在线写文件）：缓解——V4 baseline 不纳入 apply；需要时另起子计划（暂定命名：DEV-PLAN-022A）并补齐持久化与回滚。

## 9. 验收标准（本计划完成定义）

- [ ] V4 的 `subject/object/action/domain` 命名规范在文档与代码中一致，并有测试兜底。
- [ ] policy SSOT 清晰：策略碎片可追踪、聚合产物可复现、CI 能阻止漏提交。
- [ ] 最小授权闭环可演示：登录（019）→ 进入租户 → 访问受保护页面/API → 无权返回统一 403。
- [ ] 403 输出契约符合 §5.1：统一 403，响应体不回显 `subject/domain/object/action`，但日志具备补齐策略所需信息。
- [ ] policy 契约符合 §5.1：MVP 不出现 `g,` 行，且 `act` 仅包含 `read|admin|debug`。
- [ ] `AUTHZ_MODE` 三态行为符合 §4.8：`disabled|shadow|enforce` 可测；`AUTHZ_MODE=disabled` 必须由 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 显式解锁，否则 fail-fast；`authz_flags.yaml` 不允许出现 `segments`。
- [ ] 触发器与门禁在本仓库可执行（以 SSOT：`Makefile`/CI workflow 为准），且 pack 产物一致性通过 `git diff --exit-code -- config/access/policy.csv config/access/policy.csv.rev` 证明。

## 10. Simple > Easy Review（DEV-PLAN-003）

### 10.1 边界
- Authz 只做“是否允许做事”；RLS 只做“同租户可见性”；不要互相越界代偿。

### 10.2 不变量
- domain 只能是 tenant UUID / global。
- subject 必须是 principal/role 的规范形式，禁止模块自造。

### 10.3 停止线（命中即打回）
- [ ] 在模块里手写 subject/domain/object/action 推导（应复用 `pkg/authz` helpers）。
- [ ] 用放宽 RLS policy 实现跨租户控制面需求（必须走控制面边界与专用 role）。
- [ ] 任何模块自造 forbidden payload（应统一走全局 responder/组件）；或在 403 响应体回显 `subject/domain/object/action`。
- [ ] policy 中出现 `g,`/`g2,`（MVP 不做 role 继承/组/多角色）。
- [ ] 在 policy 或代码中使用 `create/update/delete` 作为实际授权动作，而未先更新本 dev-plan 并补齐 lint/test。
- [ ] `AUTHZ_MODE=disabled` 未被 `AUTHZ_UNSAFE_ALLOW_DISABLED=1` 显式解锁却仍能启动（必须 fail-fast）。
- [ ] 引入 `segments` 或其他“看起来可配置但运行时不生效”的 flags 扩展（必须先实现并加测试，再引入配置）。
- [ ] 引入第二套 policy SSOT（例如同时以 DB 与 Git 为权威）而无明确迁移与回滚计划。
