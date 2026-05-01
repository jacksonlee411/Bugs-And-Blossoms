# DEV-PLAN-485：API 授权目录页面方案

**状态**: 规划中；482/483/484 P1 事实基础已可作为前置输入（2026-05-01 18:58 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：新增一个只读菜单与页面，用于查看全部 HTTP API 授权目录项、每个 API 绑定的授权资源/操作/授权项标识，以及该 API 是否进入 CubeBox 可调用 HTTP API 工具面；页面不承担编辑、修复或运行时授权裁决职责。
- **关联模块/目录**：`apps/web/src/**`、`internal/server/**`、`internal/routing/**`、`pkg/authz/**`、`scripts/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-480A`、`DEV-PLAN-482`、`DEV-PLAN-482A`、`DEV-PLAN-483`、`DEV-PLAN-484`、`DEV-PLAN-488`
- **用户入口/触点**：授权管理菜单中的 `API 授权目录` 页面、服务端 API 授权目录列表接口；功能授权项页面中的“关联 API”弹窗由 `DEV-PLAN-482A` 承接；二者都消费 `DEV-PLAN-484` 的单一覆盖事实聚合能力

### 0.1 Simple > Easy 三问

1. **边界**：485 只拥有“API 授权目录”的只读用户入口与查询 API facade；482A 拥有功能授权项主页面与反向“关联 API”弹窗；484 继续拥有 route/CubeBox API tool overlay/registry/policy 的唯一覆盖事实聚合源与覆盖门禁；482 继续拥有 authz capability registry 与功能授权项 options API；490 只提供 HTTP API 是否进入 CubeBox 工具面的最小标记。
2. **不变量**：API path/method 不是授权项标识；每个受保护 API 的 `method + path` 必须能追溯到一个 `authz_object + authz_action`，并派生出一个 `object:action` 授权项标识；公开或 allowlist API 必须明确展示为未绑定授权项且说明公开原因。
3. **可解释**：管理员能从 API 角度回答“这个接口受哪个授权资源和哪个授权项控制”；功能授权项页面的反向“关联 API”查看由 482A 承接，485 只把 484 覆盖事实呈现为 API 正向目录。

## 1. 背景

`DEV-PLAN-482/482A/483/484` 已把角色候选能力收敛为功能授权项，并要求授权项标识与 API method/path 分离。功能授权项页面只回答“当前可分配授权项有哪些，以及某个授权项关联哪些 API”；本计划从 API 角度查看全量清单。

授权管理员和开发排查还需要一个正向目录：

1. 查看系统中有哪些 HTTP API。
2. 查看每个 API 绑定到哪个授权资源、操作和授权项标识。
3. 识别公开/allowlist API 与受保护 API 的差异。
4. 在不阅读路由代码、不解析 policy CSV 的情况下，快速确认某个 API 的授权归属。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 新增菜单 `API 授权目录`，与 `功能授权项` 并列作为授权管理下的只读治理页面。
2. [ ] 新增只读 API 授权目录列表接口，返回全量 HTTP API 的 method、path、访问控制状态、授权资源、操作、授权项标识、归属模块与 `丘宝可调用` 标记。
3. [ ] 页面以表格展示全部 API；`方法`、`API 路径`、`访问控制`、`资源名称`、`资源对象`、`操作`、`授权项标识`、`归属模块`、`丘宝可调用` 必须分列展示，不得把不同字段塞进同一列。
4. [ ] 页面支持按路径/授权项标识搜索，并按方法、归属模块、访问控制状态、授权资源筛选。

### 2.2 非目标

1. 不新增 DB 表、迁移或在线编辑能力；API 授权目录首期由 484 单一覆盖事实聚合能力派生，底层事实包括路由注册、route requirement、allowlist、authz capability registry、policy 与 CubeBox overlay。
2. 不把 CubeBox executor 混入本页；本页只标记现有 HTTP API 是否可被 CubeBox 作为工具调用。当前已明确不走 executor 路线，不再规划 executor 目录或弹窗展示。
3. 不把 API route path 当成授权项标识，也不允许前端从 path 反推 `object/action`。
4. 不在页面中提供“刷新目录”、`registry_rev`、覆盖 lint 状态面板或修复按钮；这些属于开发诊断和 CI 门禁，不是管理员主路径。
5. 不改变 484 的覆盖门禁；本页消费 484 聚合后的覆盖事实，不重新实现第二套事实枚举或漂移校验。
6. 不新增 `调用策略`、`工具类型`、`只读/写入策略` 等 CubeBox 策略列；读写属性已由 `方法`、`操作` 和 `授权项标识` 表达，写入确认已按 `DEV-PLAN-490` 暂缓。
7. 不新增 API 行详情抽屉；`route source`、`requirement source` 等诊断字段不进入首期 UI。
8. 不展示 authz capability registry 中不可分配、停用、无覆盖或内部 surface 的授权项诊断全集；这些从 authz capability 角度出发的诊断视图归属 `DEV-PLAN-488`。

## 3. 用户可见性交付

### 3.1 菜单与页面

- 菜单：`授权管理 > API 授权目录`
- 页面标题：`API 授权目录`
- 页面副标题：`查看 API 路径与授权资源、操作、授权项标识的绑定关系`
- 页面属性：只读，不提供新增、编辑、删除、刷新、导出首期能力。

### 3.2 表格列契约

表格必须按字段分列，避免把不同语义塞到一个列中。

| 列 | 含义 | 示例 |
| --- | --- | --- |
| 方法 | HTTP method | `GET` |
| API 路径 | HTTP route path | `/org/api/org-units` |
| 访问控制 | `受保护` / `公开` / `登录握手` / `静态资源` 等 | `受保护` |
| 资源名称 | authz capability registry 中的资源展示名 | `组织管理` |
| 资源对象 | Casbin object/resource | `orgunit.orgunits` |
| 操作 | Casbin action | `read` |
| 授权项标识 | `object:action` authz capability key | `orgunit.orgunits:read` |
| 归属模块 | owner module / surface | `orgunit` |
| 丘宝可调用 | 该 HTTP API 是否进入 CubeBox 可调用工具面 | `是` |

公开或 allowlist API 的 `资源名称`、`资源对象`、`操作`、`授权项标识` 为空，`访问控制` 必须展示明确分类，不能用空白静默表达。

`丘宝可调用` 只表达该 HTTP API 是否进入 CubeBox 工具 allowlist，不表达当前用户是否有权限调用，也不重新表达 API 的读写类型。当前用户权限仍由现有 route/service authz、RLS、数据范围和字段裁剪决定。

### 3.3 交互

1. 搜索框按 `method`、`path`、`resource_object`、`authz_capability_key`、资源名称搜索。
2. 筛选器首期支持：方法、访问控制、归属模块、授权资源。

## 4. 数据契约

### 4.1 数据来源

本页只消费服务端聚合后的目录，不从前端本地路由、导航配置、policy CSV 或硬编码常量拼装。服务端目录 API 也不得自行解析 route/policy/registry；它必须调用 484 的单一覆盖事实聚合能力后按 API 视角投影。

484 单一覆盖事实聚合来源：

1. Routing 事实：`method/path/surface/owner_module`。
2. Authz route requirement：`method/path -> authz_object/authz_action`。
3. Authz capability registry：`object/action -> authz_capability_key/resource_label/action_label/assignable/status`。
4. Routing allowlist：公开、登录握手、静态资源、health 等分类原因。
5. Policy / DB role seed 覆盖事实：用于保证 registry 与授权授予面不漂移。
6. `DEV-PLAN-490` CubeBox HTTP API 工具面最小标记：`method/path -> cubebox_callable`，只能引用本目录已经存在的 HTTP API。

### 4.2 建议 Endpoint

首期建议新增：

```text
GET /iam/api/authz/api-catalog
```

该 endpoint 必须受 `iam.authz:read` 或后续冻结的更明确授权项保护。实现前必须先按 `DEV-PLAN-482/484` 登记 `iam.authz:read` registry entry、route requirement 与 policy 覆盖；不得只在本计划中引用一个未登记 object/action。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `q` | 可选，按 method、path、资源对象、授权项标识、资源名称搜索 |
| `method` | 可选，按 HTTP method 过滤 |
| `access_control` | 可选，`protected` / `allowlisted` / `public` 等 |
| `owner_module` | 可选，按归属模块过滤 |
| `resource_object` | 可选，按授权资源对象过滤 |
| `authz_capability_key` | 可选，按授权项标识过滤 |

响应示例：

```json
{
  "api_entries": [
    {
      "method": "GET",
      "path": "/org/api/org-units",
      "access_control": "protected",
      "owner_module": "orgunit",
      "resource_label": "组织管理",
      "resource_object": "orgunit.orgunits",
      "action": "read",
      "authz_capability_key": "orgunit.orgunits:read",
      "capability_status": "enabled",
      "assignable": true,
      "cubebox_callable": true
    },
    {
      "method": "GET",
      "path": "/healthz",
      "access_control": "allowlisted",
      "owner_module": "platform",
      "cubebox_callable": false
    }
  ]
}
```

## 5. 与现有计划的分工

| 计划 | Owner |
| --- | --- |
| `DEV-PLAN-482` | authz capability registry、功能授权项 options API、授权项标识校验 |
| `DEV-PLAN-482A` | 功能授权项主页面与反向 `关联 API` 弹窗 |
| `DEV-PLAN-483` | `object:action` 单主源、旧 `permissionKey` 与旧 key 硬删除 |
| `DEV-PLAN-484` | route/CubeBox API tool overlay/registry/policy 覆盖事实与反漂移门禁 |
| `DEV-PLAN-485` | API 授权目录页面与只读查询 API |
| `DEV-PLAN-488` | 授权项诊断视图，从 authz capability 角度查看未进入普通候选项的原因 |
| `DEV-PLAN-490` | CubeBox 复用现有 HTTP API 的工具面标记与 runtime 执行链 |

485 不复制 484 的 lint 逻辑，也不拥有独立覆盖事实枚举；如果覆盖事实缺失，应先补 484 的枚举能力，再由 485 消费。482A 的反向弹窗首期固定复用 485 endpoint 的 `authz_capability_key` 服务端过滤 facade，不再新增窄 endpoint；过滤前的数据仍必须来自 484 单一聚合源，不能让前端拉全量后自行筛选。若后续确有性能、权限边界或响应形态隔离需求，必须先更新 482A/485/480A 契约或另起计划说明原因。
485 也不复制 490 的 runtime 执行策略；主表只增加 `丘宝可调用` 一列，API 的读写语义继续由 `方法`、`操作` 与 `授权项标识` 表达。
485 不承接 488 的授权项诊断；485 从 API 角度展示 method/path 到授权项的绑定，488 从 authz capability 角度展示未入候选项原因。

## 6. 实施切片

### 6.1 P0：契约冻结

1. [ ] 485 文档加入 AGENTS Doc Map。
2. [ ] 480/482/483/484 引用 485，明确“全量 API 正向目录”统一命名为 `API 授权目录`，不属于功能授权项页面。
3. [ ] 冻结 UI 列契约：`方法`、`API 路径`、`访问控制`、`资源名称`、`资源对象`、`操作`、`授权项标识`、`归属模块`、`丘宝可调用` 分列展示。
4. [ ] 冻结禁止新增 `调用策略` 主表列；写入确认暂缓，不在 API 授权目录主表预留表达。

### 6.2 P1：覆盖事实读取接口

1. [ ] 复用 484 的单一覆盖事实聚合能力；484 聚合源基础已落地，若缺 485 所需投影字段，先补 484，不在 485 新建第二套枚举。
2. [ ] 提供 API 视角投影函数，输出 API 授权目录列表。
3. [ ] 对 allowlist/public route 输出明确 `access_control` 与原因，不静默空字段。
4. [ ] 叠加 490 的 `cubebox_callable` 标记；标记引用不存在的 `method/path` 时 fail-closed。

### 6.3 P2：服务端 API

1. [ ] 新增 `GET /iam/api/authz/api-catalog`。
2. [ ] endpoint 受已登记并有 policy 覆盖的 `iam.authz:read` 或后续冻结的授权项保护。
3. [ ] 支持搜索和基础筛选。
4. [ ] 补 handler/API 测试，覆盖受保护 API、allowlist API、未知 requirement fail-closed。

### 6.4 P3：前端页面

1. [ ] 新增授权管理菜单 `API 授权目录`。
2. [ ] 新增只读列表页，按列契约展示数据。
3. [ ] 支持搜索和筛选。
4. [ ] 主页面显示 `丘宝可调用`，不显示 `调用策略`、`registry_rev`、刷新按钮、覆盖 lint 状态或数量 chip。

### 6.5 P4：测试与门禁

1. [ ] 前端测试覆盖搜索、筛选、空公开字段展示和 `丘宝可调用` 列展示。
2. [ ] 服务端测试覆盖 route/requirement/registry 聚合。
3. [ ] 命中 Authz/Routing/UI 时按 AGENTS 触发器运行对应门禁；实际执行记录进入 dev-record 或 PR 说明。

## 7. 验收标准

1. [ ] 授权管理菜单中存在 `API 授权目录` 页面。
2. [ ] 页面能展示全部 HTTP API，包括受保护 API 与公开/allowlist API。
3. [ ] 受保护 API 行展示资源名称、资源对象、操作与授权项标识。
4. [ ] 公开/allowlist API 行不伪造授权项标识，并明确展示访问控制分类。
5. [ ] 表格不出现 `类型 / 方法`、`资源 / 授权项` 这类多字段合并列。
6. [ ] 表格只新增 `丘宝可调用` 这一 CubeBox 相关主列，不出现 `调用策略` 或其他重复读写语义的策略列。
7. [ ] 前端不从本地常量、导航配置或 policy CSV 反推 API 授权归属。
8. [ ] `make check doc`、Authz/Routing/UI 命中门禁通过。

## 8. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 把 API 目录做成功能授权项子表 | 功能授权项页面再次堆满 API 诊断信息 | 必须独立菜单与页面，功能授权项只保留反向弹窗 |
| 前端自行拼装 API 归属 | UI 与 route requirement 漂移 | 页面只能消费服务端聚合 API |
| 合并字段节省列数 | `API · GET`、`资源 / 授权项` 回流 | 表格字段必须分列展示 |
| 把 allowlist 当未配置错误 | health/login/static 被误标红 | allowlist/public 必须有明确分类和原因 |
| 复制 484 lint 逻辑 | 两套覆盖判断漂移 | 485 只消费 484 的单一覆盖事实聚合结果 |
| 把 CubeBox 策略做成主表分类 | `调用策略=只读` 重复 `方法/操作`，未来写入策略被提前固化 | 主表只保留 `丘宝可调用`，写入确认暂缓，不在本页预留策略分类 |
| 把授权项诊断塞进 API 目录 | API 页面展示不可分配/停用/无覆盖 authz capability 全集 | 从 authz capability 角度的诊断归 488，485 只做 API 正向目录 |

## 9. 验证记录

- 2026-04-29 22:55 CST：创建方案文档。待实施阶段按命中范围运行 `make check doc`、Routing/Authz/UI 相关门禁与前端测试。
- 2026-05-01 18:58 CST：登记前置状态：482 registry/options、483 canonical key 硬删除、484 覆盖事实聚合与 `make authz-lint` 基础已落地；485 页面和 `GET /iam/api/authz/api-catalog` 仍未实施。
