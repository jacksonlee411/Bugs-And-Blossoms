# DEV-PLAN-485：API 访问入口目录页面方案

**状态**: 规划中（2026-04-29 22:55 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：新增一个只读菜单与页面，用于查看全部 HTTP API 访问入口，以及每个 API 绑定的授权资源、操作和授权项标识；页面不承担编辑、修复或运行时授权裁决职责。
- **关联模块/目录**：`apps/web/src/**`、`internal/server/**`、`internal/routing/**`、`pkg/authz/**`、`scripts/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-482`、`DEV-PLAN-483`、`DEV-PLAN-484`
- **用户入口/触点**：授权管理菜单中的 `API 访问入口` 页面、服务端 API 访问入口列表接口、功能授权项页面中的“关联的访问入口”弹窗跳转关系

### 0.1 Simple > Easy 三问

1. **边界**：485 只拥有“API 访问入口目录”的只读用户入口与查询 API；484 继续拥有 route/executor/registry/policy 覆盖门禁；482 继续拥有 capability registry 与功能授权项 options API。
2. **不变量**：API path/method 不是授权项标识；每个受保护 API 的 `method + path` 必须能追溯到一个 `authz_object + authz_action`，并派生出一个 `object:action` 授权项标识；公开或 allowlist API 必须明确展示为未绑定授权项且说明公开原因。
3. **可解释**：管理员能从 API 角度回答“这个接口受哪个授权资源和哪个授权项控制”，也能从功能授权项页面反向查看该授权项关联了哪些 API。

## 1. 背景

`DEV-PLAN-482/483/484` 已把角色候选能力收敛为功能授权项，并要求授权项标识与 API method/path 分离。当前功能授权项页面只回答“某个授权项关联哪些访问入口”，不能从 API 角度查看全量清单。

授权管理员和开发排查还需要一个正向目录：

1. 查看系统中有哪些 HTTP API。
2. 查看每个 API 绑定到哪个授权资源、操作和授权项标识。
3. 识别公开/allowlist API 与受保护 API 的差异。
4. 在不阅读路由代码、不解析 policy CSV 的情况下，快速确认某个 API 的授权归属。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 新增菜单 `API 访问入口`，与 `功能授权项` 并列作为授权管理下的只读治理页面。
2. [ ] 新增只读 API 访问入口列表接口，返回全量 HTTP API 的 method、path、访问控制状态、授权资源、操作、授权项标识与归属模块。
3. [ ] 页面以表格展示全部 API；`方法`、`API 路径`、`访问控制`、`资源名称`、`资源对象`、`操作`、`授权项标识` 必须分列展示，不得把不同字段塞进同一列。
4. [ ] 页面支持按路径/授权项标识搜索，并按方法、归属模块、访问控制状态、授权资源筛选。
5. [ ] 从 API 访问入口页面可以跳转或定位到对应功能授权项；从功能授权项页面“关联的访问入口”弹窗可以追溯回 API 访问入口页面的同一条 API。

### 2.2 非目标

1. 不新增 DB 表、迁移或在线编辑能力；API 访问入口目录首期由路由注册、route requirement、allowlist 与 capability registry 派生。
2. 不把 CubeBox executor 混入本页；executor 仍在功能授权项的“关联的访问入口”弹窗或后续专门 executor 目录中展示。
3. 不把 API route path 当成授权项标识，也不允许前端从 path 反推 `object/action`。
4. 不在页面中提供“刷新目录”、`registry_rev`、覆盖 lint 状态面板或修复按钮；这些属于开发诊断和 CI 门禁，不是管理员主路径。
5. 不改变 484 的覆盖门禁；本页消费覆盖事实，不重新实现第二套漂移校验。

## 3. 用户可见性交付

### 3.1 菜单与页面

- 菜单：`授权管理 > API 访问入口`
- 页面标题：`API 访问入口`
- 页面副标题：`查看 API 与授权资源、授权项标识的绑定关系`
- 页面属性：只读，不提供新增、编辑、删除、刷新、导出首期能力。

### 3.2 表格列契约

表格必须按字段分列，避免把不同语义塞到一个列中。

| 列 | 含义 | 示例 |
| --- | --- | --- |
| 方法 | HTTP method | `GET` |
| API 路径 | HTTP route path | `/org/api/org-units` |
| 访问控制 | `受保护` / `公开` / `登录握手` / `静态资源` 等 | `受保护` |
| 资源名称 | capability registry 中的资源展示名 | `组织管理` |
| 资源对象 | Casbin object/resource | `orgunit.orgunits` |
| 操作 | Casbin action | `read` |
| 授权项标识 | `object:action` capability key | `orgunit.orgunits:read` |
| 归属模块 | owner module / surface | `orgunit` |

公开或 allowlist API 的 `资源名称`、`资源对象`、`操作`、`授权项标识` 为空，`访问控制` 必须展示明确分类，不能用空白静默表达。

### 3.3 交互

1. 搜索框按 `method`、`path`、`resource_object`、`capability_key`、资源名称搜索。
2. 筛选器首期支持：方法、访问控制、归属模块、授权资源。
3. 点击 `授权项标识` 跳转或定位到 `功能授权项` 页面对应授权项。
4. 点击 API 行可打开只读详情抽屉，展示 route source、route_class、allowlist reason、requirement source 等诊断字段；这些字段不得常驻占据主表。

## 4. 数据契约

### 4.1 数据来源

本页只消费服务端聚合后的目录，不从前端本地路由、导航配置、policy CSV 或硬编码常量拼装。

服务端聚合来源：

1. Routing 事实：`method/path/route_class/surface/owner_module`。
2. Authz route requirement：`method/path -> authz_object/authz_action`。
3. Capability registry：`object/action -> capability_key/resource_label/action_label/assignable/status`。
4. Routing allowlist：公开、登录握手、静态资源、health 等分类原因。
5. `DEV-PLAN-484` 覆盖事实：用于保证受保护 API 与 registry 不漂移。

### 4.2 建议 Endpoint

首期建议新增：

```text
GET /iam/api/authz/api-access-entries
```

该 endpoint 必须受 `iam.authz:read` 或后续冻结的更明确授权项保护。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `q` | 可选，按 method、path、资源对象、授权项标识、资源名称搜索 |
| `method` | 可选，按 HTTP method 过滤 |
| `access_control` | 可选，`protected` / `allowlisted` / `public` 等 |
| `owner_module` | 可选，按归属模块过滤 |
| `resource_object` | 可选，按授权资源对象过滤 |
| `capability_key` | 可选，按授权项标识过滤 |

响应示例：

```json
{
  "api_entries": [
    {
      "method": "GET",
      "path": "/org/api/org-units",
      "access_control": "protected",
      "owner_module": "orgunit",
      "route_class": "tenant_api",
      "resource_label": "组织管理",
      "resource_object": "orgunit.orgunits",
      "action": "read",
      "capability_key": "orgunit.orgunits:read",
      "capability_status": "enabled",
      "assignable": true
    },
    {
      "method": "GET",
      "path": "/healthz",
      "access_control": "allowlisted",
      "owner_module": "platform",
      "route_class": "health",
      "allowlist_reason": "health_check"
    }
  ]
}
```

## 5. 与现有计划的分工

| 计划 | Owner |
| --- | --- |
| `DEV-PLAN-482` | capability registry、功能授权项 options API、授权项标识校验 |
| `DEV-PLAN-483` | `object:action` 单主源、旧 `permissionKey` 与旧 key 硬删除 |
| `DEV-PLAN-484` | route/executor/registry/policy 覆盖事实与反漂移门禁 |
| `DEV-PLAN-485` | API 访问入口目录页面与只读查询 API |

485 不复制 484 的 lint 逻辑；如果覆盖事实缺失，应先补 484 的枚举能力，再由 485 消费。

## 6. 实施切片

### 6.1 P0：契约冻结

1. [ ] 485 文档加入 AGENTS Doc Map。
2. [ ] 480/482/483/484 引用 485，明确“全量 API 正向目录”不属于功能授权项页面。
3. [ ] 冻结 UI 列契约：`方法`、`API 路径`、`访问控制`、`资源名称`、`资源对象`、`操作`、`授权项标识`、`归属模块` 分列展示。

### 6.2 P1：覆盖事实读取接口

1. [ ] 复用或补齐 484 的 route requirement 枚举能力。
2. [ ] 提供服务端聚合函数，输出 API access entry 列表。
3. [ ] 对 allowlist/public route 输出明确 `access_control` 与原因，不静默空字段。

### 6.3 P2：服务端 API

1. [ ] 新增 `GET /iam/api/authz/api-access-entries`。
2. [ ] endpoint 受 `iam.authz:read` 或后续冻结的授权项保护。
3. [ ] 支持搜索和基础筛选。
4. [ ] 补 handler/API 测试，覆盖受保护 API、allowlist API、未知 requirement fail-closed。

### 6.4 P3：前端页面

1. [ ] 新增授权管理菜单 `API 访问入口`。
2. [ ] 新增只读列表页，按列契约展示数据。
3. [ ] 支持搜索和筛选。
4. [ ] `授权项标识` 可跳转或定位到 `功能授权项` 页面。
5. [ ] 主页面不显示 `registry_rev`、刷新按钮、覆盖 lint 状态或数量 chip。

### 6.5 P4：测试与门禁

1. [ ] 前端测试覆盖搜索、筛选、空公开字段展示、授权项跳转。
2. [ ] 服务端测试覆盖 route/requirement/registry 聚合。
3. [ ] 命中 Authz/Routing/UI 时按 AGENTS 触发器运行对应门禁；实际执行记录进入 dev-record 或 PR 说明。

## 7. 验收标准

1. [ ] 授权管理菜单中存在 `API 访问入口` 页面。
2. [ ] 页面能展示全部 HTTP API，包括受保护 API 与公开/allowlist API。
3. [ ] 受保护 API 行展示资源名称、资源对象、操作与授权项标识。
4. [ ] 公开/allowlist API 行不伪造授权项标识，并明确展示访问控制分类。
5. [ ] 表格不出现 `类型 / 方法`、`资源 / 授权项` 这类多字段合并列。
6. [ ] 前端不从本地常量、导航配置或 policy CSV 反推 API 授权归属。
7. [ ] `make check doc`、Authz/Routing/UI 命中门禁通过。

## 8. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 把 API 目录做成功能授权项子表 | 功能授权项页面再次堆满 API 诊断信息 | 必须独立菜单与页面，功能授权项只保留反向弹窗 |
| 前端自行拼装 API 归属 | UI 与 route requirement 漂移 | 页面只能消费服务端聚合 API |
| 合并字段节省列数 | `API · GET`、`资源 / 授权项` 回流 | 表格字段必须分列展示 |
| 把 allowlist 当未配置错误 | health/login/static 被误标红 | allowlist/public 必须有明确分类和原因 |
| 复制 484 lint 逻辑 | 两套覆盖判断漂移 | 485 只消费 484 的覆盖事实或同一枚举函数 |

## 9. 验证记录

- 2026-04-29 22:55 CST：创建方案文档。待实施阶段按命中范围运行 `make check doc`、Routing/Authz/UI 相关门禁与前端测试。
