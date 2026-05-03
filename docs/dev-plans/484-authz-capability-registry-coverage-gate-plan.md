# DEV-PLAN-484：Authz Capability Registry 覆盖门禁方案

**状态**: P0/P1/P2/P4 覆盖事实聚合与 authz-lint 门禁基础已落地；P3 中 482A/485 下游 UI 展示测试已完成，487 role seed 与 488 诊断仍待实施（2026-05-01 23:10 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结新增模块、功能、HTTP API 与 CubeBox API tool overlay 必然进入 authz capability registry 和功能授权项的覆盖门禁；任何未声明、未登记、未覆盖或 policy-only 的权限漂移都必须在 CI 被阻断。
- **关联模块/目录**：`pkg/authz/**`、`config/access/**`、`scripts/authz/**`、`internal/server/**`、`internal/superadmin/**`、`modules/cubebox/**`、`apps/web/src/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-480A`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-482A`、`DEV-PLAN-483`、`DEV-PLAN-485`、`DEV-PLAN-487`、`DEV-PLAN-488`
- **用户入口/触点**：功能授权项、授权项诊断、角色定义页 authz capability 候选项、所有受保护 HTTP API、CubeBox API-first 工具链

### 0.1 Simple > Easy 三问

1. **边界**：484 拥有“覆盖关系、唯一服务端覆盖事实枚举源和反漂移门禁”；482 继续拥有 authz capability registry 数据结构与 options API；483 继续拥有旧 `permissionKey` 和旧 key 硬删除；482A/485/488 只能把 484 的同一聚合结果包装成页面/弹窗查询，不得自行解析 route、policy、registry 或 CubeBox overlay。
2. **不变量**：一个受保护 API 或 CubeBox API tool overlay 必须绑定且只绑定一个 `authz_object + authz_action`；每个 requirement 必须存在于 registry；每个 `assignable=true` authz capability 必须有当前实现覆盖证据。
3. **可解释**：管理员在功能授权项看到的授权项标识一定能追溯到至少一个当前可运行 API；新增模块如果忘记登记权限或登记了空壳权限，CI 失败。

### 0.2 当前门禁基线与剩余缺口

P1/P2 已将 `make authz-lint` 从基础 policy 文件格式检查扩展为覆盖事实门禁：当前可枚举 route requirement、authz capability registry、policy、allowlist，并为 DB role authz capability seed 与 CubeBox API tool overlay 预留同一聚合结构；尚未落地的 DB role seed 与 CubeBox overlay 当前按空集合处理。后续 482A/485/487/488/490 需要覆盖事实时，必须复用本计划已完成的同一聚合源，不得重新解析 route、policy、registry 或另写 join。

## 1. 背景

`DEV-PLAN-482` 已冻结 authz capability registry 与角色能力候选项，`DEV-PLAN-483` 已冻结 canonical `object:action` 与旧权限语言硬删除。但“新增模块、功能、API 必然进入功能授权项”是另一类问题：

1. API 可能新增了 route，但没有声明 authz requirement。
2. route 或 CubeBox API tool overlay 可能声明了 object/action，但 registry 没有对应 authz capability entry。
3. registry 可能新增了 `assignable=true` authz capability，但没有任何 API 覆盖，导致功能授权项出现空壳能力。
4. policy 可能引用 registry 外 key，或保留没有当前实现面的 policy-only 权限。
5. 前端可能绕过 options API 写死本地权限项。

这些都不能靠评审记忆解决，必须由 lint/CI 交叉校验。

## 2. 核心目标

1. [X] 冻结 API / CubeBox API tool overlay 到 authz capability key 的覆盖关系契约。
2. [X] 冻结 authz capability registry、route requirement、tool overlay、policy、前端消费之间的交叉校验规则。
3. [X] 定义 `assignable=true` authz capability 的实现覆盖证据要求，阻断空壳权限进入功能授权项。
4. [X] 定义新增模块/API 的开发模板要求，使新增功能天然携带权限元数据。
5. [X] 定义门禁入口，优先并入 `make authz-lint` 或现有 `make check` 链路，不新增无人运行的孤立脚本。
6. [X] 冻结覆盖事实的唯一服务端枚举源，供 lint、482 options 覆盖过滤、482A 关联 API、485 API 授权目录、490 CubeBox API tool builder 和 488 授权项诊断共同消费。

## 3. 非目标

1. 不改变 482 的 registry 字段模型和 options API 主契约；484 只规定覆盖证据和门禁。482 options API 默认输出口径必须与本计划一致：`enabled + assignable + tenant_api + 当前 tenant API 覆盖`。
2. 不恢复旧 `permissionKey`、`module.verb`、SetID、scope/package 或 legacy 别名；这些删除要求仍由 483 承接。
3. 不把 API route path 当成授权项标识；API route 只是覆盖证据。
4. 不要求每个 route 都有独立 authz capability key；一个 authz capability key 可以覆盖多个 route。
5. 不在本计划中新建 DB 表、迁移或在线功能授权项管理页。
6. 不在本计划中定义授权项诊断页面；诊断视图由 `DEV-PLAN-488` 承接，484 只提供覆盖事实和阻断规则。

## 4. 核心关系

```text
API Route Requirement = method + route -> authz_object + authz_action
Tool Overlay          = method + route -> cubebox_callable
Authz Capability Key  = authz_object + ":" + authz_action
```

关系基数：

```text
一个受保护 API -> 绑定一个 authz capability key
一个 authz capability key -> 可以覆盖多个 API
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

### 4.1 单一服务端聚合源

484 的 P1 产物必须是一个服务端可复用的覆盖事实聚合能力，后续实现可在命名上落为包、服务或函数，但职责必须唯一。该聚合能力是以下消费者的共同输入：

1. `make authz-lint` / authz 覆盖门禁。
2. 482 options API 的“当前 tenant API 覆盖”过滤。
3. 482A `关联 API` 弹窗的 authz capability 反向 API 查询。
4. 485 `API 授权目录` 的正向 API 列表。
5. 490 CubeBox API tool builder 的 `method/path -> cubebox_callable` 引用校验与工具筛选。
6. 488 `授权项诊断` 的诊断原因派生。

禁止的实现形态：

1. 482A、485、488 各自读取 route switch、policy CSV、registry 常量或 CubeBox overlay 后再自行 join。
2. 前端为了弹窗或目录拉取全量 route/policy 数据并本地筛选。
3. 485 目录服务成为 484 之外的第二个事实源；485 只能是 API 视角 facade。
4. 488 诊断服务重新实现 lint 判断；诊断只能解释 484 聚合事实和 lint 结果，不能让漂移通过 CI。

## 5. 门禁规则

### 5.1 Route 覆盖

1. 每个非 allowlist 的 HTTP route 必须声明 authz requirement。
2. allowlist 只能包含 health、静态资源、登录握手或明确公开入口；allowlist 必须集中维护并接受 lint。
3. route requirement 中的 `object/action` 必须存在于 482 registry。
4. 同一个 route/method 不得声明多个 requirement；需要多能力组合时必须另起子计划冻结组合语义，不在默认路径里临时扩展。
5. “当前 tenant API 覆盖”只能由实际 allowlist route 与 route requirement 的交集派生；单独存在于手写 requirement 表、但不存在于 allowlist/实际 route surface 的条目不得计为覆盖。

### 5.2 CubeBox API Tool Overlay 覆盖

1. 每个 CubeBox API tool overlay 只能引用已登记的 HTTP API `method/path`。
2. 被引用 API 的 route requirement 中的 `object/action` 必须存在于 registry。
3. overlay 引用不存在的 API、allowlist API 或缺 requirement 的受保护 API 时 lint 和运行时都 fail-closed。
4. overlay 的业务数据范围仍由被调用业务 API 保证；overlay 不表达新的 authz capability。

### 5.3 Registry 覆盖

1. registry 中每个 `assignable=true` 且 `status=enabled` 的 authz capability 必须至少有一个当前 tenant API 覆盖；覆盖必须来自 allowlist route 与 route requirement 的同一 `method/path` 交集。
2. 无覆盖但必须保留的系统内部 authz capability 必须设为 `assignable=false`，并标明 surface，例如 `superadmin`、`internal_system` 或后续冻结的专用分类。
3. `deprecated`、`disabled`、`assignable=false`、非 `tenant_api` surface 或无当前 tenant API 覆盖的 authz capability 默认不得进入 HRMS tenant 功能授权项。
4. registry 不得登记 API path 作为 key。
5. `iam.authz:read` 必须作为 tenant API authz capability 登记并覆盖保护 482 capabilities endpoint 与 485 API catalog endpoint；首期 bootstrap policy 仅授予 `tenant-admin`，`DEV-PLAN-487` 角色定义在线写入必须登记并覆盖 `iam.authz:admin` 或更明确 action。

### 5.4 Policy 覆盖

1. policy 中每个 object/action 必须存在于 registry。
2. policy 中普通 tenant 权限不得引用无当前 tenant API 覆盖的 authz capability。
3. superadmin policy 可以使用同一 `object:action` 格式，但必须归属 superadmin surface，不进入 HRMS tenant 功能授权项。
4. policy-only 权限必须删除或在同一实施切片中补齐当前实现面；不得作为示例权限保留。
5. 487 切换普通 tenant role 到 DB role authz capability SoT 后，普通 tenant role 的 authz capability seed 同样必须引用 registry 内 key；policy CSV 不得作为普通 tenant role 的 fallback 覆盖。

### 5.5 前端消费

1. 功能授权项 UI 只消费 482 options API，不从 route、policy CSV、导航配置或本地常量反推候选项；482 options API 默认只输出 `enabled + assignable + tenant_api + 当前实现覆盖` 的 authz capability。
2. 角色定义页保存 payload 只提交 `authz_capability_keys`；487 保存 API 必须做服务端二次校验，不信任前端候选项。
3. 点击功能授权项中的授权项标识时，可以打开标题为“关联 API”的弹窗展示 API method/path；主表不得常驻展示 method/path，也不能把 method/path 放进 authz capability key 列。该页面与弹窗实施 owner 为 `DEV-PLAN-482A`。当前已明确不走 executor 路线，弹窗不得规划 executor key 展示。
4. 前端不得新增 hardcoded authz capability candidate list；测试 fixture 如需模拟候选项，必须复用 registry/options response shape。
5. 当前覆盖 API 正向查看面归属 `DEV-PLAN-485` 的 `API 授权目录` 页面；普通功能授权项主页面与点击授权项标识后的反向“关联 API”弹窗归属 `DEV-PLAN-482A`。
6. 不可分配、停用、无覆盖、内部 surface 等诊断信息归属 `DEV-PLAN-488` 的授权项诊断视图；不得混入普通功能授权项默认列表或角色定义候选项。

## 6. 新增模块/API 开发模板要求

新增受保护 HTTP API 时，同一 PR 必须包含：

1. route 注册。
2. route authz requirement：`method + route -> object + action`。
3. registry entry 或复用已有 registry entry 的明确证据。
4. policy 更新或证明无需新增 policy。
5. route requirement lint / handler 测试覆盖。
6. 如该 authz capability `assignable=true`，确认 options API 和功能授权项能发现它。

新增 CubeBox API tool overlay 时，同一 PR 必须包含：

1. tool overlay registration。
2. 被引用 HTTP API 已存在 route authz requirement。
3. registry entry 或复用已有 registry entry 的明确证据。
4. 未知 method/path、缺 requirement、权限拒绝的 fail-closed 测试。
5. 如该 authz capability `assignable=true`，确认 options API 和功能授权项能发现它。

## 7. 实施切片

### 7.1 P0：契约冻结

1. [X] 484 被 AGENTS Doc Map 收录。
2. [X] 480/482/483 引用 484，覆盖门禁 owner 不再散落。
3. [X] 现有 route/CubeBox API tool overlay/registry/policy/front-end 权限语义按 482/483/484 分工重新标注。
4. [X] 在 482A/483/485/487/490 任一依赖覆盖事实的实施 PR 合入前，先完成本计划 P1/P2，使 `make authz-lint` 真正覆盖 route/registry/policy/tool overlay/DB role authz capability 交叉校验；482 registry 纯结构化落地可先行，但 options API 的覆盖过滤必须等 P1/P2 可用。
5. [X] 488 授权项诊断视图后置到 484 P1/P2 与 482A/485 首批闭环之后；如提前实现服务端雏形，也只能复用本计划 P1/P2 覆盖事实或同一枚举函数，不得复制第二套覆盖判断。

### 7.2 P1：提取覆盖事实

1. [X] 提供单一服务端覆盖事实枚举包或聚合函数，作为 lint、482 options 覆盖过滤、API 授权目录、关联 API 弹窗、CubeBox API tool builder 和授权项诊断的共同输入。
2. [X] route requirement 枚举输出 `method/path/object/action/surface`。
3. [X] CubeBox API tool overlay 枚举输出 `method/path/cubebox_callable/surface`。
4. [X] registry 枚举输出 `authz_capability_key/object/action/assignable/status/surface`。
5. [X] policy 枚举输出 `subject/domain/object/action`。
6. [ ] 487 实施后提供普通 tenant role authz capability seed/DB 定义枚举能力，输出 `tenant/role_slug/authz_capability_key/system_managed`；当前 484 聚合结构已预留 role seed 空集合扩展点。
7. [X] 482/482A/485/488/490 不得重新解析 route switch、policy CSV 或 registry 常量来判断覆盖关系；需要页面、options 或工具数据时只能调用本节同源聚合能力。

### 7.3 P2：覆盖 lint

1. [X] route requirement 缺失、重复、未登记，或没有对应 allowlist route 覆盖时 lint 失败。
2. [X] tool overlay 引用未知 route、缺 requirement 或 registry 外 object/action 时 lint 失败。
3. [X] policy 引用 registry 外 key 时 lint 失败。
4. [X] `enabled + assignable` registry entry 无 allowlist/route requirement 交集形成的 tenant API 覆盖时 lint 失败。
5. [X] HRMS tenant options API 输出 superadmin/internal-only authz capability 或无覆盖 authz capability 时 lint 或测试失败。
6. [X] `iam.authz:read` 缺 registry、缺 route requirement、缺 policy 或无 endpoint 覆盖时 lint 失败。
7. [X] 授权项诊断视图展示无覆盖或 policy-only 信息时，lint 阻断语义不变；页面存在不能让漂移通过 CI。

### 7.4 P3：开发模板与测试

1. [X] 新增模块/API 的脚手架或检查清单包含 authz requirement 与 registry entry。
2. [X] 增加最小表驱动测试，覆盖有效复用、多 route 共享同一 key、空壳 authz capability、policy-only key。
3. [X] 前端功能授权项测试按 `DEV-PLAN-482A` 覆盖“授权项标识列”和“关联 API”弹窗分离。

### 7.5 P4：CI 串联

1. [X] 覆盖 lint 并入 `make authz-lint` 或现有 `make check` 入口。
2. [X] `make preflight` 覆盖该门禁。
3. [X] CI Gate-1/质量门禁执行同一入口，避免本地与 CI 漂移。

## 8. 验收标准

1. [X] 新增受保护 API 但未声明 route requirement 时，lint 失败。
2. [X] route requirement 或 tool overlay 引用 registry 外 object/action 时，lint 失败。
3. [X] registry 新增 `enabled + assignable` authz capability 但没有 allowlist/route requirement 交集形成的 API 覆盖时，lint 失败。
4. [X] policy 引用 registry 外 key 或 policy-only key 时，lint 失败。
5. [X] 功能授权项 options API 只输出 `enabled + assignable + tenant_api + 当前实现覆盖` 的 HRMS tenant authz capability。
6. [X] UI 中“授权项标识”和“关联 API”弹窗分离展示，不把 API path 当 key，且不展示 executor key。
7. [ ] 授权项诊断如展示无覆盖、不可分配、停用或内部 surface authz capability，必须消费 484 覆盖事实且不能替代 lint 阻断。

## 9. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 靠人工评审记住登记 registry | 新 API 可运行但功能授权项不可见 | 必须补 route/tool overlay/registry 交叉 lint |
| 空壳 authz capability 进入目录 | 管理员能分配不可执行权限 | `assignable=true` 无覆盖即失败 |
| 每个 API 都新建 key | `GET list/details/audit` 各自发明 key | 回到 `object/action` 语义，复用同一 read key |
| 只检查字段名不检查值 | `requiredCapabilityKey=\"orgunit.read\"` 回流 | lint 必须检查 key 值格式与 registry |
| 手写 requirement 自证覆盖 | 假 route requirement 让 `assignable=true` capability 通过 | 覆盖事实必须反向校验 allowlist route；无实际 route surface 不计覆盖 |
| superadmin 混入 tenant 目录 | HRMS 管理员看到后台租户权限 | surface 分类和 options API 过滤必须阻断 |
| 诊断视图变成门禁替代品 | 页面能看到问题但 CI 仍放行 | 488 只读展示，484 lint 继续阻断 |
| 覆盖事实多套实现 | 482A、485、488 页面与 lint 对同一 route 得出不同授权项 | 484 P1 单一枚举源是前置条件；页面只能消费同源聚合函数 |

## 10. 验证记录

- 2026-05-01 18:22 CST：P1/P2/P4 已落地，新增 `CollectAuthzCoverageFactsWithAllowlist`、route/allowlist/registry/policy/tool overlay/role seed 聚合结构与 `cmd/authz-lint` 门禁；CubeBox tool overlay 与 DB role seed 当前按空集合扩展点处理，后续 490/487 必须接入同一聚合源；已执行 `make authz-pack && make authz-test && make authz-lint`、Go fmt/vet/lint/test、路由与文档相关门禁。
- 2026-05-01 18:58 CST：补齐 P0/P3/诊断后置登记；确认 482A/485 UI 展示测试、487 role seed 接入和 488 诊断页面仍未实施，不把页面存在性作为 484 完成条件。
- 2026-05-01 19:54 CST：根据待提交评审补强覆盖事实口径：tenant API covered keys 改为从 allowlist route 与 route requirement 的交集派生，并新增 route requirement 反向校验，避免手写 requirement 自证覆盖。
- 2026-05-01 22:03 CST：482A/485 已消费同一 484 覆盖事实聚合源；API catalog 已收敛为仅展示当前覆盖的 `enabled + assignable + tenant_api` 授权 API，浏览器复验 `count=46,badPaths=[],accessControls=["protected"],missingCapabilityKeyCount=0,nonAssignableCount=0`。
- 2026-05-01 23:10 CST：补齐文档状态登记；确认 P3 中 482A/485 UI 展示测试已完成，487 role seed 枚举接入与 488 授权项诊断页面仍未实施。
