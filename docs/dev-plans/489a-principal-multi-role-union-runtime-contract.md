# DEV-PLAN-489A：Principal 多角色 Union 运行时契约修订方案

**状态**: 已实施运行时门面、DB union cutover 与专用反回流门禁，并完成 480A 组合运行时验收（2026-05-02 CST；E2E 待后续补齐）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：正式将 480 系列普通 tenant 授权从“每个 session 单 `role_slug`”升级为“principal 角色集合 union”，冻结 session subject set、审计字段、scope 合并规则与反回流门禁；本计划不拥有角色定义主表、角色 capability 主表或用户授权表的新增迁移。
- **关联模块/目录**：`modules/iam/**`、`modules/orgunit/**`、`pkg/authz/**`、`internal/server/**`、`apps/web/src/**`、`config/access/**`、`scripts/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-480A`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-484`、`DEV-PLAN-487`、`DEV-PLAN-489`、`DEV-PLAN-490`

### 0.1 Simple > Easy 三问

1. **边界**：489A 只修订运行时授权主体和多角色 union 语义。角色定义保存仍归 `DEV-PLAN-487`；用户授权保存、`principal_role_assignments` 与组织范围绑定仍归 `DEV-PLAN-489`；489A 不直接新增 DB 表或迁移。
2. **不变量**：普通 tenant 运行时授权必须以 `principal_id + tenant_id + assigned role_slugs[]` 作为可信输入；能力授权为已分配角色在 487 角色定义能力集合上的并集；组织范围为 principal 组织范围绑定的并集；不得选择“当前角色”、不得取第一行、不得从 `iam.principals.role_slug` 推导普通 tenant 在线授权。
3. **可解释**：一个请求进入后，session 只证明“谁在当前租户内发起请求”；IAM scope/authz provider 读取该 principal 的角色集合与组织范围；若任一已分配角色授予 `object:action`，能力层允许，随后按 capability 的 `scope_dimension` 注入 principal 组织范围；审计记录完整角色集合和实际命中的角色。

## 1. 背景与冲突修订

`DEV-PLAN-019`、`DEV-PLAN-022` 和早期 480 口径冻结了“每个 session 恰好一个 `role_slug`”。`DEV-PLAN-481` 与 `DEV-PLAN-489` 随后把用户授权页设计为可添加多个角色行，并计划 `iam.principal_role_assignments` 与 `roles: []` 保存 payload。若不修订运行时语义，会出现两套权威来源：

1. `iam.principals.role_slug` 表示当前有效角色。
2. `iam.principal_role_assignments` 表示授权角色集合。

489A 选择正式升级为 principal 多角色 union，消除“当前角色”概念，不引入多角色选择器、角色优先级或 deny override。

## 2. 冻结契约

### 2.1 Session Authz Context

普通 tenant 受保护请求的运行时上下文必须收敛为：

```text
tenant_id
principal_id
assigned_role_slugs[]   -- 从 iam.principal_role_assignments 读取，排序、去重、同租户校验
```

约束：

1. `assigned_role_slugs[]` 为空时 fail-closed。
2. `assigned_role_slugs[]` 中任一角色不存在、未启用或跨 tenant 时 fail-closed。
3. `iam.principals.role_slug` 不再作为普通 tenant EHR 在线授权的权威来源；它只能作为迁移前既有字段、seed/backfill 输入或待收敛兼容面，不得参与 489A cutover 后的运行时裁决。
4. 匿名、登录页与 superadmin 控制面仍使用各自专用 subject 口径，不通过 `principal_role_assignments` 表达。

### 2.2 Effective Subject Set

489A 不引入 Casbin `g/g2`、角色继承或 principal 作为 policy subject。运行时 subject 从单值升级为集合：

```text
effective_subjects = [
  "role:{role_slug_1}",
  "role:{role_slug_2}",
  ...
]
```

若内部仍使用 Casbin，`Enforce` 可以对每个 `role:{slug}` 执行一次并由统一授权门面做确定性 union；若内部直接读取 487 DB role capability 集合，也必须保持相同外部语义。handler、业务模块和 CubeBox API runner 只能调用统一门面，不得自行循环角色或读取 IAM 表。

### 2.3 Capability Union

能力授权规则：

```text
CapabilitiesForPrincipal(principal)
  = DISTINCT UNION(
      role_authz_capabilities(role_slug)
      FOR role_slug IN principal_role_assignments(principal)
    )
```

对 `object:action` 的判断：

1. 任一已分配角色授予该 capability，即能力层允许。
2. 没有任何已分配角色授予该 capability，则拒绝。
3. 不支持 deny rule、角色优先级、角色排除或“更窄角色覆盖更宽角色”；如后续需要，必须另起计划。
4. 不允许 DB 角色定义与 `config/access/policy.csv` 对普通 tenant role 做 OR；CSV 只能继续承接 bootstrap/static/system surface。

### 2.4 Scope Merge

组织范围仍是 principal 维度，不按角色分别配置。对需要 `scope_dimension=organization` 的 capability：

```text
OrgScopesForPrincipal(principal, capability)
  = UNION(principal_org_scope_bindings(principal))
```

约束：

1. 多个组织范围取并集；`include_descendants=true` 包含节点及 subtree，`false` 只包含节点本身。
2. capability 被角色集合 union 授予，但 principal 没有任何组织范围时 fail-closed。
3. 用户授权保存时，只要已分配角色集合中任一 capability 的 `scope_dimension=organization`，就必须至少保存一条组织范围。
4. 首批不支持“角色 A 只能看组织 X、角色 B 只能看组织 Y”的按角色范围矩阵；如需按角色绑定 scope，必须另起计划并修改 481/489 UI。
5. 不保存隐式全租户范围；全租户访问必须由显式租户根组织加 `include_descendants=true` 表达。

### 2.5 审计与诊断

运行时授权日志、拒绝诊断和后续 readiness 证据必须至少能追踪：

| 字段 | 语义 |
| --- | --- |
| `tenant_id` | 当前租户域 |
| `principal_id` | 审计主体 |
| `role_slugs` | 当前 principal 已分配角色集合，排序后记录 |
| `matched_role_slugs` | 本次 `object:action` 命中的角色集合；拒绝时为空 |
| `object` / `action` | authz capability key 的两个维度 |
| `decision` | `allow` / `deny` |
| `authz_source` | `db_role_union`、`static_system_policy` 等稳定来源标签 |
| `scope_dimension` | `none` / `organization` |
| `scope_node_keys` | 实际注入的组织范围节点集合；无组织范围时为空 |

对外 403 响应仍由统一 responder 渲染，不在响应体泄露完整角色集合或策略细节。

## 3. 反回流门禁

489A 实施时必须新增或扩展门禁，建议入口为 `make check authz-role-union`，并纳入 `make preflight`。门禁至少阻断：

1. 普通 tenant 运行时授权路径直接读取 `iam.principals.role_slug` 做 allow/deny。
2. 出现 `current_role_slug`、`primary_role_slug`、`default_role_slug`、`active_role_slug` 等“当前角色”字段或 payload。
3. 对 `principal_role_assignments` 使用 `LIMIT 1`、`roles[0]`、“第一行角色”或排序后取单个角色参与授权。
4. 普通 tenant role 同时从 DB role definition 与 policy CSV 任一命中即放行。
5. 在 489 表中复制角色名称、角色描述或 capability 集合作为第二套角色事实源。
6. 从前端 query、localStorage、prompt、CubeBox context 或 policy CSV 推导组织范围。

实施前允许 019/022 的历史单角色文字作为 baseline 说明保留，但所有 480 系列新实现、测试和门禁必须引用 489A 的多角色 union 口径。

## 4. 实施步骤

### 4.1 P0：文档修订

1. [X] 新建 489A 文档，冻结 principal 多角色 union、session subject set、审计字段、scope 合并和反回流门禁。
2. [X] 修订 `DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-487`、`DEV-PLAN-489`，把普通 tenant 运行时授权语义指向 489A。
3. [X] 将 489A 加入 `AGENTS.md` Doc Map。

### 4.2 P1：运行时门面

1. [X] 定义统一 Authz/Scope provider 输入：`tenant_id`、`principal_id`、`assigned_role_slugs[]`、`object`、`action`、`effective_date`。
2. [X] handler 与业务模块不得接收单个 `role_slug` 作为普通 tenant 授权参数。
3. [X] CubeBox API-first runner 复用同一授权门面，不实现独立角色 union。

### 4.3 P2：IAM SoT 与迁移

1. [X] 由 `DEV-PLAN-489` 实施 `principal_role_assignments` 与 `principal_org_scope_bindings`；新增表前已获得用户手工确认。
2. [X] 由 `DEV-PLAN-487` 提供角色定义摘要与 role capability 读取接口，489A 不直接读取 `role_authz_capabilities`。
3. [X] 制定并实施 `iam.principals.role_slug` 到 `principal_role_assignments` 的一次性 seed/backfill 策略；cutover 后普通 tenant 授权不再回读单字段。

### 4.4 P3：测试与门禁

1. [X] 补授权门面测试：多角色任一命中允许、全不命中拒绝、空角色集合拒绝。
2. [X] 补 scope provider 测试：多组织范围 union、缺失组织范围 fail-closed、无隐式全租户。
3. [X] 补专用反回流门禁测试：`make check authz-role-union` 已阻断普通 tenant 单 `role_slug`/`roles[0]` 回流、DB+CSV OR 放行、普通 tenant CSV role grant 与当前角色字段回流。
4. [X] 补运行时闭环证据：角色能力来自 487 DB SoT，principal 角色集合来自 489 `principal_role_assignments`，组织范围来自 489 scope provider，普通 API 与 CubeBox API-first 不回读 CSV、`iam.principals.role_slug` 或 `roles[0]`。
5. [ ] 更新 readiness 记录，登记专用反回流门禁、`make authz-pack && make authz-test && make authz-lint`、相关 Go/UI 测试结果；本轮证据先记录于 487/489/489A 计划验证段与最终说明。

## 5. ADR 摘要

- **决策 1：多角色 union，不做当前角色选择**
  - 用户授权页允许多个角色行，运行时必须体现这些角色的并集。
  - 当前角色选择器会把同一用户的授权变成会话状态，难以审计且与 481/489 的保存模型冲突。

- **决策 2：仍不引入 Casbin g/g2**
  - 多角色 union 由统一授权门面聚合，不用 Casbin role inheritance 表达。
  - 角色定义事实源仍是 487 DB role definition；policy CSV 不成为普通 tenant role 的第二放行来源。

- **决策 3：组织范围按 principal 合并**
  - 首批 UI 只有“角色”和“组织范围”两个页签，没有按角色配置范围的矩阵。
  - 因此组织范围是 principal 维度事实，运行时按并集裁剪。

- **决策 4：审计记录角色集合与命中角色**
  - 只记录 principal 不足以解释 union 放行原因。
  - 只记录单个 role 会掩盖多角色命中或错误配置。

## 6. 验收标准

1. [X] 019/022 不再被解读为阻止 480 系列多角色 union；单角色口径仅作为历史 baseline 或非 480 surface 说明。
2. [X] 480/487/489 明确引用 489A，且不存在“当前 session 有效角色 = 单 `role_slug`”作为普通 tenant EHR 运行时授权 SSOT 的表述。
3. [X] 489 的 `roles: []`、`principal_role_assignments`、`CapabilitiesForPrincipal` 与 489A 的 union 语义一致。
4. [X] 实施新增表前已按 AGENTS 要求获得用户手工确认。
5. [X] 489A 作为 480 系列后端运行时授权交付的一部分，已与 487/489 同步满足 480A 的组合闭环口径；不得仅凭授权门面或反回流门禁单点宣称完整用户可见闭环。
6. [X] `make check doc` 通过。

## 7. 本轮验证记录

- 2026-05-02 CST：已实施普通 tenant runtime 多角色 union 门面。`withAuthz`、session capabilities、CubeBox orgunit executor 和 orgunit scope provider 均以 `tenant_id + principal_id` 回源 DB runtime store；不再使用 `RoleSlug`/`roles[0]`/CSV 普通 tenant grant 做运行时 allow。
- 2026-05-02 CST：已新增并接线专用 `make check authz-role-union` 反回流门禁，纳入 `make preflight` 与 CI Gate-1，阻断单角色/当前角色/CSV role grant 回流。
- 已验证：`go test ./...`、`go vet ./...`、`make check lint`、`make authz-pack && make authz-test && make authz-lint`、`make check routing`、`make check error-message`、`make check doc`、`make check authz-role-union`。
- 补充复核：`go test ./internal/server ./internal/routing ./pkg/authz`、`make test`、`make check no-legacy && make check chat-surface-clean && make check no-scope-package && make check granularity && make check request-code`、`make check root-surface`。
- 待补：E2E 与用户授权 UI 保存交互。
