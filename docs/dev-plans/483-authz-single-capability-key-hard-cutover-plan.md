# DEV-PLAN-483：权限标识单主源与前端 permissionKey 硬删除方案

**状态**: 规划中（2026-05-01 10:31 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：把权限标识收敛为唯一 `object:action` authz capability key，硬删除前端 `permissionKey`、`module.verb` 别名、policy-only 权限和未实现能力；不提供兼容映射、双字段、过渡窗口或旧 key 自动转换。
- **关联模块/目录**：`pkg/authz/**`、`config/access/**`、`scripts/authz/**`、`internal/server/**`、`internal/superadmin/**`、`apps/web/src/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-480A`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-482A`、`DEV-PLAN-484`、`DEV-PLAN-487`、`DEV-PLAN-488`
- **用户入口/触点**：功能授权项、授权项诊断、角色定义页、用户授权页、导航入口、页面守卫、所有受保护 HTTP API 与 CubeBox API-first 工具链

### 0.1 Simple > Easy 三问

1. **边界**：`pkg/authz` 的 authz capability registry 是唯一权限标识事实源；policy 只表达授予结果；route 与 CubeBox API tool overlay 只消费 authz capability key；前端不再定义本地权限语言，并必须从服务端会话/权限摘要获取当前用户 authz capability key。route/tool overlay/registry/policy 的覆盖门禁由 `DEV-PLAN-484` 承接。
2. **不变量**：系统内唯一可保存、可传输、可展示给管理员的权限标识是 `object:action`；任何 `orgunit.read`、`dict.admin`、`foundation.read`、`approval.read`、`cubebox.conversations.read` 这类旧 key 都不是权限标识。
3. **可解释**：管理员看到的“授权项标识”就是运行时鉴权使用的 key；一个 key 可以保护多个 API，但 API 路由不能发明第二个 key；没有当前 HTTP API 或明确 surface 承接的 key 不进入权限系统。

## 1. 背景与问题

当前实现暴露出三类漂移：

1. 后端 Casbin 使用 `object/action`，而前端导航和页面守卫使用 `permissionKey`，例如 `orgunit.read`、`dict.admin`。
2. 有些前端 key 只是本地 UI 守卫，例如 `foundation.read`、`approval.read`，没有对应后端 API 权限实现。
3. policy 中存在未找到当前 tenant API route requirement 的权限，例如 `iam.ping:read`、`iam.session:read`、`org.share_read:read`；这些 policy-only 权限的覆盖门禁由 `DEV-PLAN-484` 承接。

这些问题会让权限管理员无法判断“分配这个权限后用户能做什么”，也会让审计、测试和后续角色管理出现 UI、policy、route、CubeBox tool overlay 各写一套的风险。

本计划是 `DEV-PLAN-482` 的硬切换补充：482 冻结 authz capability registry 和 options API；483 专门冻结旧权限语言的删除要求、无兼容要求和反漂移验收。

## 2. 核心目标

1. [ ] 冻结唯一权限标识：`AuthzCapabilityKey = object + ":" + action`，例如 `orgunit.orgunits:read`。
2. [ ] 删除前端独立 `permissionKey` 语言；前端若需要守卫，只能使用同一个 canonical authz capability key。
3. [ ] 删除或改造所有 policy-only 权限；policy 中不得存在没有当前 route/CubeBox API tool overlay/superadmin surface 承接的 key。
4. [ ] 删除 `module.verb`、点号 action、构建期权限、示例权限和历史共享读权限等旧表达。
5. [ ] 对齐 `DEV-PLAN-484` 覆盖门禁与 `DEV-PLAN-487` 角色保存 API，使 registry、policy、route requirement、CubeBox API tool overlay、角色定义 payload、前端路由守卫只能使用同一套 key；483 本身只拥有旧 key 和旧字段硬删除。

## 3. 非目标

1. 不引入 DB schema、迁移或在线 registry 管理页；本计划只冻结硬切换契约和实施要求。
2. 不设计旧 key 兼容、别名表、自动转换、灰度窗口或 deprecated registry entry。
3. 不把 API path 当成权限标识；API path 只是某个 authz capability key 的覆盖证据。
4. 不把 superadmin 权限混入 HRMS tenant 权限管理员页面；superadmin 可以继续使用同一 `object:action` 格式，但属于独立 surface。
5. 不恢复 SetID、scope/package、org_level/scope_type/scope_key 或历史共享读语义。

## 4. 不可变要求

### 4.1 唯一标识格式

唯一格式：

```text
<object>:<action>
```

示例：

| 合法 key | 含义 |
| --- | --- |
| `orgunit.orgunits:read` | 查看组织 |
| `orgunit.orgunits:admin` | 管理组织 |
| `iam.dicts:admin` | 管理字典 |
| `iam.dict_release:admin` | 发布字典 |
| `cubebox.conversations:use` | 使用 CubeBox 会话 |

禁止格式：

| 禁止 key | 原因 |
| --- | --- |
| `orgunit.read` | 前端旧别名，不是 Casbin object/action |
| `orgunit.admin` | 前端旧别名 |
| `dict.admin` | 前端旧别名 |
| `dict.release.admin` | 前端旧别名 |
| `cubebox.conversations.read` | 点号 action 旧表达；必须是 `cubebox.conversations:read` |
| `foundation.read` | UI 本地守卫，无后端授权实现 |
| `approval.read` | 当前无对应后端 API 权限实现 |

术语约束：

1. 本计划中的 capability key 只表示授权项标识，即 authz capability key；新代码与新接口字段优先显式命名为 `authz_capability_key(s)` 或 `requiredCapabilityKey`。
2. 历史业务策略 `capability_key`、SetID 策略 key、字段策略 key 不属于前端权限语言，不得被迁移成 `requiredCapabilityKey`。
3. 前端字段命名可改为 `requiredCapabilityKey`，但该字段值必须是 `object:action`；字段改名不能让旧业务策略 key 或旧 `module.verb` key 合法化。

### 4.2 授权项标识与 API 的关系

授权项标识不是 API 地址。它是授权 ID，由 `authz_object` 与 `authz_action` 组合而成；API 是被访问的接口入口。

正确关系：

```text
API Route Requirement = method + route -> authz_object + authz_action
Authz Capability Key  = authz_object + ":" + authz_action

一个 API route -> 绑定一个授权项标识 / authz capability key
一个授权项标识 -> 可以保护多个 API route
```

示例：

| 授权项标识 | 关联 API |
| --- | --- |
| `orgunit.orgunits:read` | `GET /org/api/org-units` |
| `orgunit.orgunits:read` | `GET /org/api/org-units/details` |
| `orgunit.orgunits:read` | `GET /org/api/org-units/audit` |
| `orgunit.orgunits:admin` | `POST /org/api/org-units` |
| `orgunit.orgunits:admin` | `POST /org/api/org-units/rename` |
| `orgunit.orgunits:admin` | `POST /org/api/org-units/move` |

当前代码示例：

```text
GET /org/api/org-units
-> object = orgunit.orgunits
-> action = read
-> key    = orgunit.orgunits:read
```

反例：

```text
key = /org/api/org-units
```

要求：

1. 每个受保护 API route 必须映射到且只映射到一个 registry 中存在的 authz capability key。
2. 一个 authz capability key 可以保护多个 route，例如 `orgunit.orgunits:read` 可保护 list/search/details/audit 等读取 API。
3. `assignable=true` 的 authz capability 必须有当前可运行的 tenant API 承接；没有实现面的能力不得进入角色候选项。
4. policy 中每条授权记录必须能在 registry 和当前实现面中找到证据；找不到就删除，不保留空壳。覆盖证据校验以 `DEV-PLAN-484` 为准。
5. UI 功能授权项主表列名统一使用“授权项标识”；普通功能授权项主页面与“关联 API”弹窗由 `DEV-PLAN-482A` 承接。API method/path 只允许出现在“关联 API”弹窗或 `DEV-PLAN-485` 的 `API 授权目录` 中；不可分配、停用、无覆盖、内部 surface 等诊断信息只进入 `DEV-PLAN-488` 的授权项诊断视图；不得把 API 地址和 key 混在同一列。

### 4.3 前端使用要求

前端只能消费 canonical authz capability key：

1. 删除 `permissionKey` 这个概念和类型字段；若组件需要守卫，字段命名应表达“需要的 capability”，例如 `requiredCapabilityKey`，且值必须是 `object:action`。
2. 删除 `VITE_PERMISSIONS` 与空权限默认 `*` 的构建期权限模型。
3. 导航、页面守卫、按钮状态、CubeBox 设置入口不得继续使用旧 `permissionKey` 语言；若需要权限判断，使用 canonical key，并在删除旧构建期权限 fallback 的同一切片接入真实会话 authz capability 来源 `GET /iam/api/me/capabilities`。
4. 前端不维护旧 key 到新 key 的映射表。
5. 前端测试不得继续断言 `orgunit.read`、`dict.admin` 等旧 key。

硬切换的替代数据源要求：

1. 删除 `VITE_PERMISSIONS` 和默认 `*` 的同一实施切片，必须提供服务端当前用户 authz capability 来源；当前落地为 session-authenticated bootstrap endpoint `GET /iam/api/me/capabilities`，响应字段为 `authz_capability_keys`，且只返回 canonical `object:action` key。
2. 缺少服务端 authz capability 摘要、摘要加载失败或摘要为空时，前端导航与页面守卫必须 fail-closed；不得临时恢复 `*` 或允许全部可见。
3. 前端守卫字段可以命名为 `requiredCapabilityKey` 或等价名称，但值必须来自 canonical `object:action`；字段改名不能替代值校验。
4. 如果某页面首期尚无后端授权实现，该页面不能靠本地 key 留在功能授权项或导航权限体系中；应移除权限语义或补齐受保护 API/registry 覆盖。
5. `/iam/api/me/capabilities` 只作为会话 bootstrap/self endpoint；它先经过租户与会话校验，但不作为可分配 authz capability surface，也不要求自身再绑定一个业务 authz capability，避免 authz capability 摘要读取出现循环依赖。

### 4.4 服务端保存与校验要求

服务端不接受旧 key：

1. 角色定义、角色 fixture、权限 options response 只出现 `object:action`。
2. 提交 `orgunit.read`、`dict.admin`、`cubebox.conversations.read` 等旧 key 时，直接返回明确错误，例如 `invalid_capability_key`。
3. 服务端不做 normalize，不把点号 action 转成冒号 action。
4. 未登记、禁用、未实现、不可分配的 key 均不能保存到角色定义。

## 5. 当前清理清单

### 5.1 前端旧 key

| 现有 key / surface | 处理要求 |
| --- | --- |
| `foundation.read` | 删除权限语义；若 Foundation demo 保留，它不能作为功能授权项 |
| `approval.read` | 删除权限语义；审批模块未落地前不进入功能授权项 |
| `orgunit.read` | 替换为 `orgunit.orgunits:read`，不保留别名 |
| `orgunit.admin` | 替换为 `orgunit.orgunits:admin`，不保留别名 |
| `dict.admin` | 替换为 `iam.dicts:admin`，不保留别名 |
| `dict.release.admin` | 替换为 `iam.dict_release:admin`，不保留别名 |
| `cubebox.conversations.read` | 替换为 `cubebox.conversations:read`，不保留别名 |
| `cubebox.conversations.use` | 替换为 `cubebox.conversations:use`，不保留别名 |
| `permissionKey` prop/type | 删除；改为 canonical authz capability key 语义 |
| `VITE_PERMISSIONS` | 删除；不得继续作为真实权限输入 |
| `VITE_AUTHZ_CAPABILITY_KEYS` | 禁止新增构建期替代变量；当前用户 authz capability 只来自 `GET /iam/api/me/capabilities` |

### 5.2 policy-only / 未绑定 key

| 当前 key | 处理要求 |
| --- | --- |
| `iam.ping:read` | 若无当前受保护 route 承接，则从 policy/registry 删除；不得作为示例权限保留 |
| `iam.session:read` | 当前 tenant route 使用 `iam.session:admin`；若无 read route 承接，则删除 |
| `org.share_read:read` | 历史共享读/SetID-scope 语义残留；删除 |

### 5.3 superadmin key

| 当前 key | 处理要求 |
| --- | --- |
| `superadmin.session:read/admin` | 保留同一 `object:action` 格式，但归属 `superadmin_route` surface |
| `superadmin.tenants:read/admin` | 保留同一 `object:action` 格式，但不进入 HRMS tenant 权限管理员页面 |

## 6. 无兼容要求

本计划明确禁止：

1. 禁止 alias map，例如 `orgunit.read -> orgunit.orgunits:read`。
2. 禁止 API 同时接收 `permissionKey` 和 `capabilityKey`。
3. 禁止 response 同时返回旧 key 和新 key。
4. 禁止在 registry 中登记旧 key 并标记 `deprecated`。
5. 禁止前端在运行时把旧 key normalize 成新 key。
6. 禁止 `VITE_PERMISSIONS`、`*` 或空权限默认全权限作为 fallback。
7. 禁止 policy 中保留无 route/CubeBox API tool overlay/superadmin surface 的 key。
8. 禁止为了不改测试而保留旧 key fixture。

## 7. 实施切片

### 7.1 P0：契约冻结

1. [ ] 483 被 AGENTS Doc Map 收录。
2. [ ] 480/481/482 引用 483，明确旧 `permissionKey` 和旧 key 无兼容删除。
3. [ ] 设计图页面文案统一称为“功能授权项”，主表列名统一称为“授权项标识”，值只展示 `object:action`；普通功能授权项主页面与点击授权项标识后打开的“关联 API”弹窗按 `DEV-PLAN-482A`；全量 HTTP API 正向查看面统一命名为“API 授权目录”。
4. [ ] 设计图如需展示不可分配、停用、无覆盖或内部 surface capability，必须进入 `DEV-PLAN-488` 的“授权项诊断”视图，不得混入功能授权项默认列表。

### 7.2 P1：后端 registry 与 policy 硬清理

1. [ ] `pkg/authz` 增加 canonical key 构造、解析和 registry 校验函数。
2. [ ] registry 中每条 entry 使用 `object/action` 派生 key，不手写第二套 key。
3. [ ] 删除 `iam.ping:read`、`iam.session:read`、`org.share_read:read` 等无当前实现面 key，或在同一 PR 中补齐当前实现面；不得 policy-only。
4. [ ] `superadmin.*` 保留同格式，但标记或分类为独立 superadmin surface，不进入 tenant options。

### 7.3 P2：route / tool overlay / policy 反漂移门禁

1. [ ] route / CubeBox API tool overlay / policy / registry 覆盖关系按 `DEV-PLAN-484` 落入 authz lint。
2. [ ] `assignable=true` 的 registry entry 覆盖证据按 `DEV-PLAN-484` 校验。
3. [ ] 旧 key 正则命中 `apps/web/src/**`、role fixture、policy、测试 payload 时门禁失败。

### 7.4 P3：前端权限语言硬切换

1. [ ] 删除 `permissionKey` 类型字段、`VITE_PERMISSIONS` 解析和默认 `*` 行为。
2. [ ] 同一切片接入服务端当前用户 authz capability 摘要来源 `GET /iam/api/me/capabilities`；缺摘要或加载失败时 fail-closed。
3. [ ] 导航和路由守卫只使用 canonical authz capability key。
4. [ ] 旧 key 测试全部改为 canonical key；不得新增兼容测试。
5. [ ] 前端测试覆盖无 authz capability 摘要时导航/页面守卫不会默认全量可见。

### 7.5 P4：角色定义与功能授权项消费

1. [ ] 481 角色定义页只从 482 options API 获取 canonical capability。
2. [ ] 角色保存 payload 只提交 `object:action`。
3. [ ] 未知、禁用、不可分配、未实现或旧格式 key 均阻断保存。
4. [ ] 功能授权项默认只展示 `enabled + assignable + tenant_api + 当前实现覆盖` 的 authz capability。
5. [ ] 授权项诊断按 `DEV-PLAN-488` 展示普通候选项之外的 authz capability，但不得放宽角色保存校验。

## 8. 验收标准

1. [ ] `apps/web/src/**` 不再出现 `permissionKey`、`VITE_PERMISSIONS`、`foundation.read`、`approval.read`、`orgunit.read`、`orgunit.admin`、`dict.admin`、`dict.release.admin` 作为权限判断。
2. [ ] 前端导航、页面守卫和按钮状态使用的 key 与后端 registry key 完全相同。
3. [ ] 删除构建期权限 fallback 的同一 PR 中，前端已接入服务端当前用户 authz capability 摘要 `GET /iam/api/me/capabilities`，响应字段为 `authz_capability_keys`；缺摘要时 fail-closed。
4. [ ] policy、route requirement、CubeBox API tool overlay 与 registry 的覆盖关系按 `DEV-PLAN-484` 校验，任意 key 不在 registry 时 authz lint 失败。
5. [ ] registry 中 `assignable=true` 但无当前实现面的 key 按 `DEV-PLAN-484` 导致 authz lint 失败。
6. [ ] 角色保存提交旧 key 时失败，且服务端不返回替换建议或自动修正结果。
7. [ ] HRMS tenant 功能授权项不展示 superadmin key、不展示 policy-only key、不展示 UI 本地守卫 key。
8. [ ] 授权项诊断可展示被排除 key 的诊断原因，但这些 key 不得成为前端守卫、角色候选项或保存 payload 的兼容来源。

## 9. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 为减少改动保留 alias | 旧 key 和新 key 同时存在 | 停止实现，删除 alias 后再继续 |
| 前端先改名但值仍是旧 key | `requiredCapabilityKey="orgunit.read"` | 门禁必须按值检查，不只检查字段名 |
| policy-only key 继续保留 | 管理员看到不可执行权限 | policy/registry lint 失败 |
| superadmin 混入 tenant 功能授权项 | HRMS 管理员看到租户管理后台权限 | options API 默认过滤 superadmin surface |
| 空权限默认全权限 | 未配置环境变量时 UI 全量可见 | 删除 `*` fallback，缺摘要时 fail-closed |
| 删除旧权限语言后无服务端来源 | 导航全关或开发者重新加本地 fallback | 483 P3 必须同切片接入服务端 authz capability 摘要 |
| 业务策略 key 被误当权限 key | `org.orgunit_create.field_policy` 等历史 key 出现在导航守卫或角色 payload | 前端守卫和保存 API 只接受 registry 中的 `object:action` authz capability key |

## 10. 验证记录

- 待实施阶段按命中范围运行：`make check doc`、`go fmt ./... && go vet ./... && make check lint && make test`、`make authz-pack && make authz-test && make authz-lint`、前端测试与 E2E。
