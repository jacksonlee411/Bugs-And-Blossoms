# DEV-PLAN-484：Authz Capability Registry 覆盖门禁方案

**状态**: 规划中（2026-05-01 08:14 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结新增模块、功能、HTTP API 与 CubeBox API tool overlay 必然进入 capability registry 和功能授权项的覆盖门禁；任何未声明、未登记、未覆盖或 policy-only 的权限漂移都必须在 CI 被阻断。
- **关联模块/目录**：`pkg/authz/**`、`config/access/**`、`scripts/authz/**`、`internal/server/**`、`internal/superadmin/**`、`modules/cubebox/**`、`apps/web/src/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-483`、`DEV-PLAN-485`
- **用户入口/触点**：功能授权项、角色定义页 capability 候选项、所有受保护 HTTP API、CubeBox API-first 工具链

### 0.1 Simple > Easy 三问

1. **边界**：484 只拥有“覆盖关系和反漂移门禁”；482 继续拥有 capability registry 数据结构与 options API；483 继续拥有旧 `permissionKey` 和旧 key 硬删除。
2. **不变量**：一个受保护 API 或 CubeBox API tool overlay 必须绑定且只绑定一个 `authz_object + authz_action`；每个 requirement 必须存在于 registry；每个 `assignable=true` capability 必须有当前实现覆盖证据。
3. **可解释**：管理员在功能授权项看到的授权项标识一定能追溯到至少一个当前可运行 API；新增模块如果忘记登记权限或登记了空壳权限，CI 失败。

### 0.2 当前门禁缺口

当前 `make authz-lint` 只做基础 policy 文件格式检查，尚不能枚举 route requirement、capability registry、policy 与 CubeBox API tool overlay 的覆盖关系。因此 482/483/485/490 的实施 PR 在合入前，必须先完成本计划 P1/P2 覆盖事实提取与 lint 串联；否则“覆盖门禁”只是文档约束，不能作为已具备的 CI 保护。

## 1. 背景

`DEV-PLAN-482` 已冻结 capability registry 与角色能力候选项，`DEV-PLAN-483` 已冻结 canonical `object:action` 与旧权限语言硬删除。但“新增模块、功能、API 必然进入功能授权项”是另一类问题：

1. API 可能新增了 route，但没有声明 authz requirement。
2. route 或 CubeBox API tool overlay 可能声明了 object/action，但 registry 没有对应 capability entry。
3. registry 可能新增了 `assignable=true` capability，但没有任何 API 覆盖，导致功能授权项出现空壳能力。
4. policy 可能引用 registry 外 key，或保留没有当前实现面的 policy-only 权限。
5. 前端可能绕过 options API 写死本地权限项。

这些都不能靠评审记忆解决，必须由 lint/CI 交叉校验。

## 2. 核心目标

1. [ ] 冻结 API / CubeBox API tool overlay 到 capability key 的覆盖关系契约。
2. [ ] 冻结 capability registry、route requirement、tool overlay、policy、前端消费之间的交叉校验规则。
3. [ ] 定义 `assignable=true` capability 的实现覆盖证据要求，阻断空壳权限进入功能授权项。
4. [ ] 定义新增模块/API 的开发模板要求，使新增功能天然携带权限元数据。
5. [ ] 定义门禁入口，优先并入 `make authz-lint` 或现有 `make check` 链路，不新增无人运行的孤立脚本。

## 3. 非目标

1. 不改变 482 的 registry 字段模型和 options API 主契约；484 只规定覆盖证据和门禁。482 options API 默认输出口径必须与本计划一致：`enabled + assignable + tenant_api + 当前 tenant API 覆盖`。
2. 不恢复旧 `permissionKey`、`module.verb`、SetID、scope/package 或 legacy 别名；这些删除要求仍由 483 承接。
3. 不把 API route path 当成授权项标识；API route 只是覆盖证据。
4. 不要求每个 route 都有独立 capability key；一个 capability key 可以覆盖多个 route。
5. 不在本计划中新建 DB 表、迁移或在线功能授权项管理页。

## 4. 核心关系

```text
API Route Requirement = method + route -> authz_object + authz_action
Tool Overlay          = method + route -> cubebox_callable
Capability Key        = authz_object + ":" + authz_action
```

关系基数：

```text
一个受保护 API -> 绑定一个 capability key
一个 capability key -> 可以覆盖多个 API
```

示例：

| 实现入口 | Requirement | 授权项标识 |
| --- | --- | --- |
| `GET /org/api/org-units` | `orgunit.orgunits + read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/details` | `orgunit.orgunits + read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/audit` | `orgunit.orgunits + read` | `orgunit.orgunits:read` |
| `POST /org/api/org-units` | `orgunit.orgunits + admin` | `orgunit.orgunits:admin` |
| `POST /org/api/org-units/rename` | `orgunit.orgunits + admin` | `orgunit.orgunits:admin` |
| `POST /org/api/org-units/move` | `orgunit.orgunits + admin` | `orgunit.orgunits:admin` |

## 5. 门禁规则

### 5.1 Route 覆盖

1. 每个非 allowlist 的 HTTP route 必须声明 authz requirement。
2. allowlist 只能包含 health、静态资源、登录握手或明确公开入口；allowlist 必须集中维护并接受 lint。
3. route requirement 中的 `object/action` 必须存在于 482 registry。
4. 同一个 route/method 不得声明多个 requirement；需要多能力组合时必须另起子计划冻结组合语义，不在默认路径里临时扩展。

### 5.2 CubeBox API Tool Overlay 覆盖

1. 每个 CubeBox API tool overlay 只能引用已登记的 HTTP API `method/path`。
2. 被引用 API 的 route requirement 中的 `object/action` 必须存在于 registry。
3. overlay 引用不存在的 API、allowlist API 或缺 requirement 的受保护 API 时 lint 和运行时都 fail-closed。
4. overlay 的业务数据范围仍由被调用业务 API 保证；overlay 不表达新的 capability。

### 5.3 Registry 覆盖

1. registry 中每个 `assignable=true` 且 `status=enabled` 的 capability 必须至少有一个当前 tenant API 覆盖。
2. 无覆盖但必须保留的系统内部 capability 必须设为 `assignable=false`，并标明 surface，例如 `superadmin`、`internal_system` 或后续冻结的专用分类。
3. `deprecated`、`disabled`、`assignable=false`、非 `tenant_api` surface 或无当前 tenant API 覆盖的 capability 默认不得进入 HRMS tenant 功能授权项。
4. registry 不得登记 API path 作为 key。
5. `iam.authz:read` 必须作为 tenant API capability 登记并覆盖保护 482 capabilities endpoint 与 485 API catalog endpoint；如出现在线写入能力，再登记并覆盖 `iam.authz:admin` 或更明确 action。

### 5.4 Policy 覆盖

1. policy 中每个 object/action 必须存在于 registry。
2. policy 中普通 tenant 权限不得引用无当前 tenant API 覆盖的 capability。
3. superadmin policy 可以使用同一 `object:action` 格式，但必须归属 superadmin surface，不进入 HRMS tenant 功能授权项。
4. policy-only 权限必须删除或在同一实施切片中补齐当前实现面；不得作为示例权限保留。

### 5.5 前端消费

1. 功能授权项 UI 只消费 482 options API，不从 route、policy CSV、导航配置或本地常量反推候选项；482 options API 默认只输出 `enabled + assignable + tenant_api + 当前实现覆盖` 的 capability。
2. 角色定义页保存 payload 只提交 capability keys。
3. 点击功能授权项中的授权项标识时，可以打开标题为“关联 API”的弹窗展示 API method/path；主表不得常驻展示 method/path，也不能把 method/path 放进 key 列。当前已明确不走 executor 路线，弹窗不得规划 executor key 展示。
4. 前端不得新增 hardcoded capability candidate list；测试 fixture 如需模拟候选项，必须复用 registry/options response shape。
5. 全量 HTTP API 正向查看面归属 `DEV-PLAN-485` 的 `API 授权目录` 页面；功能授权项页面只保留点击授权项标识后的反向“关联 API”弹窗。

## 6. 新增模块/API 开发模板要求

新增受保护 HTTP API 时，同一 PR 必须包含：

1. route 注册。
2. route authz requirement：`method + route -> object + action`。
3. registry entry 或复用已有 registry entry 的明确证据。
4. policy 更新或证明无需新增 policy。
5. route requirement lint / handler 测试覆盖。
6. 如该 capability `assignable=true`，确认 options API 和功能授权项能发现它。

新增 CubeBox API tool overlay 时，同一 PR 必须包含：

1. tool overlay registration。
2. 被引用 HTTP API 已存在 route authz requirement。
3. registry entry 或复用已有 registry entry 的明确证据。
4. 未知 method/path、缺 requirement、权限拒绝的 fail-closed 测试。
5. 如该 capability `assignable=true`，确认 options API 和功能授权项能发现它。

## 7. 实施切片

### 7.1 P0：契约冻结

1. [ ] 484 被 AGENTS Doc Map 收录。
2. [ ] 480/482/483 引用 484，覆盖门禁 owner 不再散落。
3. [ ] 现有 route/CubeBox API tool overlay/registry/policy/front-end 权限语义按 482/483/484 分工重新标注。
4. [ ] 在 482/483/485/490 任一实施 PR 合入前，先完成本计划 P1/P2，使 `make authz-lint` 真正覆盖 route/registry/policy/tool overlay 交叉校验。

### 7.2 P1：提取覆盖事实

1. [ ] 提供 route requirement 枚举能力，输出 `method/path/object/action/surface`。
2. [ ] 提供 CubeBox API tool overlay 枚举能力，输出 `method/path/cubebox_callable/surface`。
3. [ ] 提供 registry 枚举能力，输出 `key/object/action/assignable/status/surface`。
4. [ ] 提供 policy 枚举能力，输出 `subject/domain/object/action`。

### 7.3 P2：覆盖 lint

1. [ ] route requirement 缺失、重复、未登记时 lint 失败。
2. [ ] tool overlay 引用未知 route、缺 requirement 或 registry 外 object/action 时 lint 失败。
3. [ ] policy 引用 registry 外 key 时 lint 失败。
4. [ ] `enabled + assignable` registry entry 无 tenant API 覆盖时 lint 失败。
5. [ ] HRMS tenant options API 输出 superadmin/internal-only capability 或无覆盖 capability 时 lint 或测试失败。
6. [ ] `iam.authz:read` 缺 registry、缺 route requirement、缺 policy 或无 endpoint 覆盖时 lint 失败。

### 7.4 P3：开发模板与测试

1. [ ] 新增模块/API 的脚手架或检查清单包含 authz requirement 与 registry entry。
2. [ ] 增加最小表驱动测试，覆盖有效复用、多 route 共享同一 key、空壳 capability、policy-only key。
3. [ ] 前端功能授权项测试覆盖“授权项标识列”和“关联 API”弹窗分离。

### 7.5 P4：CI 串联

1. [ ] 覆盖 lint 并入 `make authz-lint` 或现有 `make check` 入口。
2. [ ] `make preflight` 覆盖该门禁。
3. [ ] CI Gate-1/质量门禁执行同一入口，避免本地与 CI 漂移。

## 8. 验收标准

1. [ ] 新增受保护 API 但未声明 route requirement 时，lint 失败。
2. [ ] route requirement 或 tool overlay 引用 registry 外 object/action 时，lint 失败。
3. [ ] registry 新增 `enabled + assignable` capability 但没有 API 覆盖时，lint 失败。
4. [ ] policy 引用 registry 外 key 或 policy-only key 时，lint 失败。
5. [ ] 功能授权项 options API 只输出 `enabled + assignable + tenant_api + 当前实现覆盖` 的 HRMS tenant capability。
6. [ ] UI 中“授权项标识”和“关联 API”弹窗分离展示，不把 API path 当 key，且不展示 executor key。

## 9. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 靠人工评审记住登记 registry | 新 API 可运行但功能授权项不可见 | 必须补 route/tool overlay/registry 交叉 lint |
| 空壳 capability 进入目录 | 管理员能分配不可执行权限 | `assignable=true` 无覆盖即失败 |
| 每个 API 都新建 key | `GET list/details/audit` 各自发明 key | 回到 `object/action` 语义，复用同一 read key |
| 只检查字段名不检查值 | `requiredCapabilityKey=\"orgunit.read\"` 回流 | lint 必须检查 key 值格式与 registry |
| superadmin 混入 tenant 目录 | HRMS 管理员看到后台租户权限 | surface 分类和 options API 过滤必须阻断 |

## 10. 验证记录

- 2026-04-29 21:20 CST：创建方案文档，待后续实施阶段按命中范围运行 `make check doc`、`make authz-pack && make authz-test && make authz-lint`、前端测试与 E2E。
