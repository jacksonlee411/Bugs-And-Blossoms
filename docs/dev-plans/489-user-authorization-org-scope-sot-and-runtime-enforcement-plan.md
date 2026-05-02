# DEV-PLAN-489：用户授权组织范围 SoT 与运行时强制实施方案

**状态**: 已实施后端 SoT、运行时强制、用户授权 UI 保存交互，并完成 480A 组合运行时与 A/B E2E 验收（2026-05-02 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结 `DEV-PLAN-481` 用户授权页两个页签背后的首批可保存闭环：principal 角色授权、组织范围 SoT、保存 API、运行时读取与 orgunit 服务端强制裁剪；本计划不拥有角色定义主表或角色 authz capability 主表，不直接提交迁移，新增 DB 表实施前必须再次获得用户手工确认。
- **关联模块/目录**：`modules/iam/**`、`modules/orgunit/**`、`pkg/authz/**`、`internal/server/**`、`apps/web/src/**`、`config/access/**`、`scripts/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-032`、`DEV-PLAN-480`、`DEV-PLAN-480A`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-487`、`DEV-PLAN-489A`、`DEV-PLAN-490`
- **用户入口/触点**：`授权管理 > 用户授权` 顶部用户选择器、`角色` 页签、`组织范围` 页签、统一 `保存` 按钮、orgunit 普通 API、CubeBox API-first orgunit 查询

### 0.1 Simple > Easy 三问

1. **边界**：481 只拥有用户授权 UI 与交互骨架；487 拥有角色定义保存、能力集合和运行时 capability 来源；489 只拥有首批 principal 角色授权、组织范围绑定、读取/保存 API、服务端校验和运行时组织范围强制；489A 拥有 principal 多角色 union、subject set、审计与 scope 合并规则；482 继续拥有 capability registry 与 `scope_dimension` 元数据；480 继续拥有授权体系蓝图与运行时原则。
2. **不变量**：包含 `scope_dimension=organization` capability 的角色集合被授予某用户时，该用户必须至少有一条组织范围绑定；缺失时保存失败，不得默认全租户。所有 orgunit 列表、搜索、详情、审计和 CubeBox API-first 调用都必须消费同一服务端 scope 事实。多角色能力判断按 489A 做 union，不选择当前角色。
3. **可解释**：管理员选择用户，添加一个或多个角色行，添加组织范围行，点击保存；服务端在同一事务中保存角色授权集合与组织范围，并在运行时按 489A 把角色能力 union 与当前用户组织范围注入 orgunit 查询。越界目标 fail-closed，前端本地状态、prompt、Casbin object 字符串都不能代替 SoT。

### 0.2 现状研究摘要

- `DEV-PLAN-480` 已冻结授权体系蓝图，明确 RLS 只负责跨租户隔离，数据范围必须在服务端读路径强制。
- `DEV-PLAN-481` 已冻结用户授权 UI：顶部用户选择器、`角色` / `组织范围` 两个页签、添加行、移除、统一保存、组织范围必填校验；但 481 明确不直接建表或冻结迁移 SQL。
- `DEV-PLAN-482` 已冻结 capability registry 字段，其中 `scope_dimension=organization` 是用户授权页判断是否必须配置组织范围的来源。
- `DEV-PLAN-487` 冻结角色定义保存 API、角色能力集合持久化与普通 tenant role 运行时 capability 来源；489 只引用当前有效角色定义，不拥有角色定义本身。
- `DEV-PLAN-489A` 冻结 principal 多角色 union 运行时语义：`roles: []` 不是展示集合，而是普通 tenant 授权 subject set 的来源；不得回退成 `roles[0]` 或当前角色。
- 当前 `config/access/policies/**` 与 Casbin 只覆盖 capability/API 级授权，不是在线用户授权记录、组织范围绑定或运行时数据范围 SoT。
- 最容易出错的位置：把组织范围只做成前端状态；把范围塞进 Casbin object/action；给缺失范围默认全租户；CubeBox 走 executor 或绕过 HTTP API 读路径；在角色定义页回流 `scope_required` 字段。

## 1. 背景与上下文

用户授权页已经有清晰 UI 方案，但“保存后真实生效”尚未冻结。若只实现两个页签的静态展示或前端状态，管理员会以为用户 B 已被限制在“鲜花事业部及下级”，而服务端 API / CubeBox 仍可能按全租户读取组织数据。这会把授权 UI 变成僵尸功能，并违反 480 的同租户内授权边界。

本计划把首批闭环压缩到一个可解释模型：

1. 角色定义回答“能做什么”。
2. 用户授权回答“谁拥有这些角色”。
3. 组织范围回答“这个用户在组织维度能力上能在哪里行使角色”。
4. 运行时 orgunit 读写路径强制消费该范围。

首批只做 principal/user 维度，不扩展 team、position、有效期、字段策略、冲突检测或策略预览。

## 2. 目标与非目标

### 2.1 核心目标

1. [X] 冻结用户授权组织范围 SoT：principal 角色授权与组织范围绑定归属 IAM 模块，orgunit 只提供组织节点与 subtree 解析能力；角色定义与角色 authz capability 集合继续归 `DEV-PLAN-487`。
2. [X] 冻结首批保存 API：用户授权页一次性提交角色行与组织范围行，服务端同事务保存并校验。
3. [X] 冻结必填校验：已分配角色集合按 489A union 后，任一 capability 包含 `scope_dimension=organization` 时，组织范围行必须非空。
4. [X] 冻结运行时强制：orgunit list/search/tree/detail/audit/write 和 CubeBox API-first orgunit 查询使用同一 scope provider。
5. [X] 冻结失败语义：缺失范围保存失败；越界访问 fail-closed；不能用前端隐藏、prompt 或 Casbin 字符串替代服务端裁剪。

### 2.2 非目标

1. 不在本计划直接新增 DB 表、迁移或 sqlc 生成物；后续实施新增表前必须再次获得用户手工确认。
2. 不实现 team、position、岗位、群组或委托授权。
3. 不实现有效期、字段级授权、字段脱敏、冲突检测、策略预览、授权摘要或保存审计解释 UI。
4. 不支持“按角色分别配置组织范围”；首批同一用户的一组组织范围适用于该用户所有需要 `organization` 范围的角色能力。
5. 不把组织范围写入 Casbin policy，不新增第二套 policy engine，不引入 OPA/CEL。
6. 不恢复 legacy、SetID、scope/package、org_level/scope_type/scope_key 或旧 permission key 语义。
7. 不让 CubeBox 成为独立授权主体；CubeBox 继续继承当前用户权限并复用 HTTP API 或等价 route/service path。

### 2.3 用户可见性交付

- **用户可见入口**：`授权管理 > 用户授权`。
- **最小可操作闭环**：
  1. 管理员选择用户 B。
  2. 在 `角色` 页签添加 `flower-hr`。
  3. 在 `组织范围` 页签添加 `鲜花事业部`，`包含下级组织` 默认选中。
  4. 点击 `保存`。
  5. 用户 B 调用 orgunit API 或通过 CubeBox 查询组织时，只能看到授权组织范围内的数据。
- **后端先行验收**：即使 UI 分批落地，服务端 API、保存校验、scope provider 和 orgunit 裁剪也必须可通过集成测试验证，不能只停留在页面状态。

## 2.4 工具链与门禁

- **命中触发器**：
  - [X] Go 代码
  - [X] `apps/web/**` / presentation assets / 生成物（错误映射与构建资产命中）
  - [X] i18n（仅 `en/zh`）
  - [X] DB Schema / Migration / Backfill / Correction
  - [ ] sqlc（本轮未新增 sqlc query/config）
  - [X] Routing / allowlist / responder / 相关路由注册/映射
  - [X] AuthN / Tenancy / RLS
  - [X] Authz（Casbin/bootstrap policy 与 runtime 分工）
  - [X] E2E
  - [X] 文档 / readiness / 证据记录（本计划创建命中）
  - [X] `request-code`、`error-message`、`ddd-layering-p0/p2`

实际命令入口以 `AGENTS.md`、`Makefile`、CI workflow 为准；执行结果写入 `docs/dev-records/DEV-PLAN-489-READINESS.md` 或实现 PR 说明。

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `pkg/authz` | authz capability key、scope dimension、角色是否需要组织范围的纯函数判断 | `pkg/authz/*_test.go` | 黑盒表驱动，覆盖无范围、organization 范围、未知 key |
| `modules/iam/services` | 用户授权保存校验、角色授权与组织范围绑定规则 | `modules/iam/services/*_test.go` | 不把业务规则堆在 handler |
| `modules/iam/infrastructure` | IAM 授权 SoT 读写、显式事务、tenant 注入、RLS | `modules/iam/infrastructure/**` | 后续实现命中 sqlc/PG 测试时补 |
| `modules/orgunit/services` | scope filter 注入后的 list/search/detail/audit/write 行为 | `modules/orgunit/services/*_test.go` | 业务路径只消费 scope provider，不读取前端状态 |
| `internal/server` | API payload 解析、错误映射、route authz、事务编排 | `internal/server/*_test.go` | 覆盖缺范围、未知用户、未知角色、越界访问 |
| `apps/web/src/**` | 两页签保存状态、服务端错误落点、切换用户未保存状态处理 | Vitest / Testing Library | 优先测状态转换和 API client |
| `E2E` | A/B 用户组织范围端到端验收 | `e2e/**` | 覆盖普通 API 与 CubeBox API-first 一致性 |

并行测试仅用于无共享 DB、无全局状态的纯函数；涉及 `t.Setenv`、共享 PG、session 或租户上下文的测试不得与 parallel 混用。

## 3. 架构与关键决策

### 3.1 5 分钟主流程

```mermaid
flowchart LR
  Admin[授权管理员] --> UI[用户授权页]
  UI --> API[PUT /iam/api/authz/user-assignments/{principal_id}]
  API --> IAM[iam user authorization service]
  IAM --> DB[(iam 授权 SoT)]
  Caller[当前用户或 CubeBox API runner] --> OrgAPI[orgunit HTTP API / service]
  OrgAPI --> PIP[iam scope provider]
  PIP --> DB
  OrgAPI --> OrgStore[orgunit store with scope filter]
  OrgStore --> Result[裁剪后的组织结果]
```

主流程：

1. 管理员打开用户授权页，选择 principal/user。
2. 页面加载该用户的角色授权行与组织范围行。
3. 管理员保存时，服务端校验所有角色存在、authz capability key 有效、角色集合 union 后是否需要 organization 范围、组织节点是否属于当前 tenant。
4. 服务端在同一事务中替换该用户首批授权集合，并返回保存后的版本。
5. 运行时 orgunit API 根据当前 session principal 读取 IAM scope provider 输出，把 scope filter 注入 orgunit 查询。
6. CubeBox API-first 工具链以当前用户调用同一 HTTP API 或等价 route/service path，因此自动复用同一 scope filter。

失败路径：

1. 缺少 `iam.authz:admin` 或后续更明确管理能力：保存 API 返回授权拒绝。
2. 角色包含 `scope_dimension=organization` capability 但无组织范围：保存失败，错误定位到 `org_scopes`。
3. 组织节点不存在、跨租户或不可解析：保存失败。
4. 运行时查询目标越界：list/search 返回裁剪后结果；detail/audit/write 返回 fail-closed 错误。

恢复语义：

1. 保存失败不写入部分角色或部分范围。
2. 管理员修正角色或组织范围后重试同一保存请求。
3. 若后续实施引入 `request_code` 幂等字段，必须按 `DEV-PLAN-109A` 命名收敛，不得新增 `request_id` 业务幂等字段。

### 3.2 模块归属与职责边界

- **IAM owner**：角色定义、角色 authz capability 集合、principal 角色授权、principal 组织范围绑定、scope provider。
- **OrgUnit owner**：组织节点、组织树、subtree 解析、带 scope filter 的业务查询。
- **Authz registry owner**：482 继续拥有 capability 元数据与 `scope_dimension`。
- **Server composition owner**：`internal/server` 只编排 HTTP payload、session/tenant/authz、事务边界和错误映射，不承载业务规则。
- **Frontend owner**：481 UI 消费 489 API，不从本地常量、policy CSV、导航或 prompt 推导授权事实。

### 3.3 落地形态决策

- **形态选择**：`A. Go DDD + PostgreSQL SoT`
- **选择理由**：用户授权保存是 IAM 管理配置，不是 orgunit 业务事件。首批不需要事件回放或 DB Kernel 投射；用 IAM service 编排显式事务、PG repo 与 RLS 更简单。若后续需要审计事件流，可另起计划加入 append-only audit，但不得改变本计划的运行时读取主链。

Go DDD 分工：

1. `modules/iam/domain`：principal 授权行、组织范围绑定、校验错误类型；角色定义实体复用 `DEV-PLAN-487`，不在 489 重复建模。
2. `modules/iam/services`：保存用户授权、读取 487 角色摘要、计算角色是否需要组织范围、组装 scope provider 输出。
3. `modules/iam/infrastructure`：PG 读写实现、显式事务、tenant 注入、RLS。
4. `modules/orgunit/services`：接收已解析 scope filter 并强制裁剪业务查询。

### 3.4 用户授权 DB SoT

本节只冻结用户授权与组织范围建议模型，不等同于允许立即新增表。实施迁移前必须再次获得用户手工确认。角色定义主表和角色 authz capability 集合由 `DEV-PLAN-487` 拥有，489 不重复创建 `authz_roles` / `authz_role_capabilities` / `role_authz_capabilities` 或同义表。

| 表 | 归属 | 首批字段要点 | 说明 |
| --- | --- | --- | --- |
| `iam.principal_role_assignments` | IAM | `tenant_uuid`、`principal_id`、`role_slug`、审计字段 | 用户角色授权行，首批只支持 principal |
| `iam.principal_authz_assignment_revisions` | IAM | `tenant_uuid`、`principal_id`、`revision`、审计字段 | 用户授权 replace-all 乐观锁版本；不复制角色或组织范围事实 |
| `iam.principal_org_scope_bindings` | IAM | `tenant_uuid`、`principal_id`、`org_node_key`、`include_descendants`、审计字段 | 用户组织范围行 |

约束：

1. 所有表必须 tenant-scoped，启用并强制 RLS。
2. `principal_role_assignments.role_slug` 必须引用或服务端校验命中 487 的当前有效角色定义；不得在 489 表中复制角色名称、描述或 capability 集合作为事实源。
3. 是否需要组织范围由 487 角色 authz capability 集合 + 482 registry `scope_dimension` 计算，不在 489 表中保存 `scope_required`。
4. `org_node_key` 使用 orgunit 现行 node key，不引入 org_level/scope_type/scope_key。
5. `principal_authz_assignment_revisions` 只承载该 principal 授权集合的 replace-all 乐观锁版本；角色集合仍以 `principal_role_assignments` 为 SoT，组织范围仍以 `principal_org_scope_bindings` 为 SoT。
6. 不保存“全租户”隐式范围。若未来需要全租户显式授权，应另起计划冻结表达方式；首批可通过选择租户根组织并勾选包含下级表达。
7. `principal_role_assignments` 是 489A 普通 tenant 多角色 union 的角色集合来源；保存与读取必须保留完整集合，不得取第一行或派生 current role。

### 3.5 API 契约

建议首批 endpoint：

```text
GET /iam/api/authz/user-assignments
GET /iam/api/authz/user-assignments?principal_id={uuid}
PUT /iam/api/authz/user-assignments/{principal_id}
```

不带 `principal_id` 的 `GET` 用于用户授权页顶部用户选择器，只返回当前 tenant 内 active principal 的轻量候选，不返回 `iam.principals.role_slug`，也不得作为运行时授权来源：

```json
{
  "principals": [
    {
      "principal_id": "11111111-1111-1111-1111-111111111111",
      "email": "user-b@example.invalid",
      "display_name": "王小花"
    }
  ]
}
```

`GET` 响应示例：

```json
{
  "principal_id": "11111111-1111-1111-1111-111111111111",
  "roles": [
    {
      "role_slug": "flower-hr",
      "display_name": "鲜花公司 HR",
      "description": "查看和维护授权组织范围内的组织数据",
      "requires_org_scope": true
    }
  ],
  "org_scopes": [
    {
      "org_node_key": "A2345678",
      "org_code": "FLOWERS",
      "org_name": "鲜花事业部",
      "include_descendants": true
    }
  ],
  "revision": 7
}
```

`PUT` 请求示例：

```json
{
  "roles": [
    {"role_slug": "flower-hr"}
  ],
  "org_scopes": [
    {"org_code": "FLOWERS", "include_descendants": true}
  ],
  "revision": 7
}
```

服务端规则：

1. Endpoint 必须受 `iam.authz:admin` 或后续冻结的更明确管理 capability 保护，并进入 registry、route requirement、policy 与 484 覆盖门禁。
2. `principal_id` 必须属于当前 tenant。
3. 每个 `role_slug` 必须存在且启用；`roles` 为空不得保存为可运行授权。
4. 保存采用 replace-all 语义：请求中的 `roles` 与 `org_scopes` 是该用户首批授权配置的完整集合。
5. 保存必须校验 `revision`；冲突返回 `stale_revision`，不得局部保存成功。
6. `org_scopes` 的用户可见输入使用 `org_code`；服务端在当前 tenant 内解析为 `org_node_key` 后写入 IAM SoT。响应可附带 `org_code` 用于 UI 回显，但运行时 SoT 仍是 `org_node_key`。
7. 组织范围缺失错误返回稳定 code，例如 `authz_org_scope_required`，并在 UI 映射到组织范围页签。

### 3.6 运行时 Scope Provider

建议服务端内部契约：

```go
type OrgScope struct {
    OrgNodeKey         string
    IncludeDescendants bool
}

type PrincipalScopeProvider interface {
    CapabilitiesForPrincipal(ctx context.Context, tenantID string, principalID string) ([]string, error)
    OrgScopesForPrincipal(ctx context.Context, tenantID string, principalID string, capabilityKey string) ([]OrgScope, error)
}
```

冻结不变量：

1. Scope provider 是组织范围事实的唯一运行时读取入口。
2. OrgUnit service 只接收 scope provider 解析后的 filter，不自行读取 IAM 表。
3. `scope_dimension=none` 的 capability 不要求组织范围。
4. `scope_dimension=organization` 的 capability 如果运行时读不到组织范围，必须 fail-closed。
5. 运行时不得从前端 query 参数、localStorage、prompt、CubeBox context 或 policy CSV 推导组织范围。
6. Scope provider 可以调用 487 的角色能力读取接口判断 authz capability 集合，但不得自行读取或复制 `role_authz_capabilities` 表实现第二套角色能力来源。
7. `CapabilitiesForPrincipal` 的语义必须与 489A 一致：对该 principal 的全部 `principal_role_assignments` 做 DISTINCT UNION；不得从 `iam.principals.role_slug`、`roles[0]` 或 current role 推导能力。

### 3.7 OrgUnit 裁剪契约

首批按以下语义强制：

1. `list/search/tree`：只返回授权范围内组织。多个 scope 取并集。
2. `include_descendants=true`：包含该组织节点及下级 subtree。
3. `include_descendants=false`：只包含该组织节点。
4. `detail/audit`：目标组织不在范围内时 fail-closed。
5. `write/admin`：若 action 为 `orgunit.orgunits:admin`，同样必须检查目标组织在范围内；没有组织范围不得写。
6. CubeBox API-first orgunit 查询必须与普通 HTTP API 结果一致。

首批建议 detail/audit/write 越界返回 `403`；如后续决定用 `404` 避免泄露资源存在性，必须在本计划更新后再实现。

## 4. UI 契约

481 的用户授权 UI 不变，489 只补保存和错误消费契约：

1. 顶部用户选择器先通过不带 `principal_id` 的 `GET` 加载服务端 principal 候选，再在切换后通过带 `principal_id` 的 `GET` 加载该用户授权事实。
2. `角色` 页签只维护角色行；角色说明来自服务端角色摘要或 role options API，不从 capability 常量拼装。
3. `组织范围` 页签只维护组织行；组织选择器复用 orgunit 服务端读路径。
4. `保存` 调用统一 `PUT`，覆盖两个页签。
5. `authz_org_scope_required` 错误必须把用户带到组织范围页签，并标记组织范围缺失。
6. 切换用户时若存在未保存变更，必须确认或丢弃，不得把 A 用户的范围保存到 B 用户。

## 5. 与现有计划分工

| 计划 | Owner |
| --- | --- |
| `DEV-PLAN-480` | 授权体系蓝图、RLS 与应用授权分工、运行时强制原则 |
| `DEV-PLAN-481` | 角色定义与用户授权 UI / 交互 SSOT |
| `DEV-PLAN-482` | capability registry、`scope_dimension`、候选项 options API |
| `DEV-PLAN-484` | registry / route / policy / CubeBox API tool overlay 覆盖门禁 |
| `DEV-PLAN-485` | API 授权目录只读页面 |
| `DEV-PLAN-487` | 角色定义保存 API、角色 authz capability 持久化、普通 tenant role 运行时能力来源 |
| `DEV-PLAN-489` | 用户授权保存 SoT、组织范围绑定、scope provider、orgunit 强制裁剪 |
| `DEV-PLAN-489A` | principal 多角色 union、subject set、审计字段、scope 合并和反回流门禁 |
| `DEV-PLAN-490` | CubeBox API-first 工具化，复用当前用户 HTTP API 授权与数据范围 |

## 6. 实施切片

### 6.1 P0：契约冻结

1. [X] 489 文档加入 AGENTS Doc Map。
2. [X] 480/481 引用 489，明确数据范围 SoT、保存模型、运行时强制由 489 承接。
3. [X] 确认首批只支持 principal/user，不扩 team/position/effective_date/字段策略。
4. [X] 实施前获得用户对新增 IAM 授权表的手工确认。

### 6.2 P1：Registry 与管理 capability

1. [X] 按 482/484 登记并覆盖 `iam.authz:read` 与 `iam.authz:admin` 或更明确管理 capability。
2. [X] 用户授权读取 API 使用 read capability，保存 API 使用 admin capability。
3. [X] route requirement、policy、registry 与 API catalog 全部可追溯。

### 6.3 P2：IAM SoT 与服务

1. [X] 新增 IAM 用户授权 SoT schema、RLS、迁移；不新增角色定义主表或角色 authz capability 主表。本轮未命中 sqlc。
2. [X] 通过 487 runtime store 读取角色定义摘要，实现用户授权读取、replace-all 保存；保存后的角色集合满足 489A 多角色 union 输入契约。
3. [X] 服务端校验角色存在、角色 authz capability 是否需要组织范围、组织范围必填、组织节点归属 tenant。
4. [X] 保存失败不得产生部分写入。

### 6.4 P3：Scope Provider 与 OrgUnit 强制裁剪

1. [X] 实现 principal scope provider。
2. [X] orgunit list/search/tree/detail/audit/write 统一注入 scope filter。
3. [X] 缺少组织范围或越界目标 fail-closed。
4. [X] CubeBox API-first orgunit 调用复用同一路径。

### 6.5 P4：用户授权 UI 保存闭环

1. [X] 用户选择器加载服务端授权事实。
2. [X] 两个页签共享统一 dirty state 和保存按钮。
3. [X] 服务端组织范围必填错误映射到组织范围页签。
4. [X] 保存成功后重新读取服务端事实。

### 6.6 P5：测试、门禁与证据

1. [X] 补 IAM handler/runtime store 测试。
2. [X] 补 orgunit scope 裁剪测试。
3. [X] 补 UI 保存交互测试。
4. [X] 补 E2E：A 全集团，B 仅鲜花事业部；普通 API 与 CubeBox 查询一致。
5. [X] 补闭环测试证据：角色能力来自 487 DB SoT，principal 角色集合来自 489 `principal_role_assignments`，能力判断按 489A union，组织范围来自 489 scope provider，普通 API 与 CubeBox API-first 不回读 CSV、`iam.principals.role_slug` 或 `roles[0]`。
6. [ ] 更新 `docs/dev-records/DEV-PLAN-489-READINESS.md` 记录命中命令和结果；本轮证据先记录于本计划验证段与最终说明。

## 7. 验收标准

1. [X] 用户授权页保存不是前端本地状态；刷新后仍能从服务端读取角色行与组织范围行。
2. [X] 包含 `scope_dimension=organization` capability 的角色授权缺少组织范围时保存失败，不默认全租户。
3. [X] 用户 B 只配置“鲜花事业部及下级”后，orgunit list/search/tree 只返回该范围内组织（服务端测试与 E2E 覆盖）。
4. [X] 用户 B 访问范围外 detail/audit/write 时 fail-closed（服务端测试与 E2E 覆盖）。
5. [X] CubeBox orgunit 查询与普通 HTTP API 使用同一权限和组织范围结果（服务端测试与 E2E 覆盖；490 API-first hard cutover 仍待后续）。
6. [X] 角色定义页不出现组织范围、`scope_required`、字段策略、有效期或策略预览。
7. [X] policy CSV、route requirement、capability registry 和 API catalog 能追溯到 `iam.authz:read/admin` 或更明确管理能力。
8. [X] 489 作为 480 系列后端运行时授权交付的一部分，已与 487/489A 以及 481 UI 保存交互同步满足 480A 的首批用户可见闭环口径；不得仅凭用户授权保存 API 或 scope provider 单点宣称完成。
9. [X] 命中的 Go/Authz/Routing/DB/UI/doc/E2E 门禁通过；sqlc 未命中。

## 8. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| UI 静态化 | 两页签可编辑但刷新丢失，运行时不生效 | 保存必须写服务端 SoT，E2E 验证 API 裁剪 |
| 默认全租户 | 缺组织范围也保存成功 | `scope_dimension=organization` 缺范围必须失败 |
| 重复角色 SoT | 489 新增或维护角色定义 / role authz capability 表 | 角色定义与能力集合只归 487；489 只保存 principal assignment 与 org scope |
| 单角色回流 | 489 保存 `roles: []`，运行时却只用 `roles[0]`、current role 或 `principals.role_slug` | 489A 要求完整角色集合 union；反回流门禁阻断单角色路径 |
| 组织范围塞进 Casbin | policy object/action 带组织节点 | 组织范围只在 IAM SoT 与 scope provider |
| OrgUnit 自读 IAM 表 | orgunit store 直接依赖 IAM schema | 由服务层注入 scope filter，避免跨模块耦合 |
| CubeBox 绕过裁剪 | 模型 executor 或旧 read plan 直读全量 orgunit | CubeBox API-first 复用 HTTP API / route-service path |
| 子计划单独宣布运行时完成 | 489 用户授权可保存，但 487 DB role cutover 或 489A union 尚未接入 | 只能声明 489 子能力完成；480 系列运行时闭环必须按 480A 的 487/489/489A 组合验收 |
| 过度设计 | 首批加入 effective_date、字段策略、team/position | 全部后移独立计划 |
| 新表未经确认 | 实施 PR 直接提交迁移 | 新增 DB 表前必须再次获得用户手工确认 |

## 9. Readiness 证据

后续实现 PR 需要新建或更新 `docs/dev-records/DEV-PLAN-489-READINESS.md`，至少记录：

- 文档变更：`make check doc`
- DB/schema 命中时：模块级 Atlas/Goose plan/lint/migrate 结果
- sqlc 命中时：`make sqlc-generate` 与生成物状态
- Authz 命中时：`make authz-pack && make authz-test && make authz-lint`
- Routing 命中时：`make check routing`
- Go 命中时：`go fmt ./... && go vet ./... && make check lint && make test`
- UI 命中时：`make generate && make css` 与 `git status --short`
- E2E 命中时：A/B 用户组织范围与 CubeBox API-first 一致性结果

### 9.1 本轮验证记录

- 2026-05-02 CST：后端 SoT 与运行时强制已实施。新增 `iam.principal_role_assignments`、`iam.principal_org_scope_bindings`，用户授权读取/保存 API，principal scope provider，orgunit HTTP 与 CubeBox orgunit executor 范围裁剪；普通 tenant runtime 不再从 CSV、`iam.principals.role_slug`、`roles[0]` 推导授权。
- 已验证：`go test ./...`、`go vet ./...`、`make check lint`、`make authz-pack && make authz-test && make authz-lint`、`make check routing`、`make check error-message`、`make iam plan && make iam lint`、`pnpm -C apps/web typecheck && pnpm -C apps/web test`、`make generate && make css`、`make check root-surface && make check no-legacy && make check doc`、`make check chat-surface-clean && make check no-scope-package && make check granularity && make check request-code`。
- 2026-05-02 CST：随 487/489A 完成 480A 后端运行时组合验收，并新增 `make check authz-role-union` 专用反回流门禁；补充验证通过 `go test ./internal/server ./internal/routing ./pkg/authz`、`make test`、`make check authz-role-union`。
- 2026-05-02 CST：用户授权 UI 保存交互已接入 489 API；组织范围缺失错误映射到组织范围页签。新增 `e2e/tests/dev481-authz-org-scope-runtime.spec.js`，覆盖 A 用户全范围、B 用户仅鲜花事业部及下级、普通 orgunit API 裁剪、范围外 detail fail-closed、CubeBox orgunit 查询与普通 API 一致。稳定门禁继续通过 `make preflight` / `make e2e`；真实模型验收改为显式 `make e2e-live`。
