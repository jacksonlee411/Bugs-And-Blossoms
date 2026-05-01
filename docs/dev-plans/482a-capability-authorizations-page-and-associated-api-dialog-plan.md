# DEV-PLAN-482A：功能授权项主页面与关联 API 弹窗实施方案

**状态**: 规划中（2026-05-01 11:18 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：新增授权管理下的 `功能授权项` 只读主页面，展示可分配且有当前实现覆盖的 authz capability，并定义点击授权项标识后打开的 `关联 API` 弹窗；页面不承担 registry 在线编辑、角色保存或 API 正向目录职责。
- **关联模块/目录**：`apps/web/src/**`、`internal/server/**`、`pkg/authz/**`、`scripts/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-483`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-488`
- **用户入口/触点**：授权管理菜单中的 `功能授权项` 页面、482 authz capability options API、484 单一覆盖事实聚合能力、基于同一聚合结果暴露的 authz capability 反向 API 查询面

### 0.1 Simple > Easy 三问

1. **边界**：482A 只拥有普通 tenant 功能授权项页面和反向 `关联 API` 弹窗；482 继续拥有 registry 字段模型、options API 和 authz capability key 校验；484 继续拥有唯一服务端覆盖事实聚合源与 CI 反漂移门禁；485 继续拥有全量 API 正向目录 facade；488 继续拥有后置的不可分配、停用、内部 surface 和无覆盖能力诊断视图。
2. **不变量**：普通功能授权项默认列表只能展示 `enabled + assignable + tenant_api + 当前 tenant API 覆盖` 的 authz capability。API method/path 只能在 `关联 API` 弹窗里展示，不进入主表常驻列，也不能被当作授权项标识。
3. **可解释**：管理员在 `功能授权项` 页面能回答“系统当前有哪些可授予的功能权限”；点击某个授权项标识后，弹窗只回答“这个授权项由哪些当前 HTTP API 覆盖”。

## 1. 背景

`DEV-PLAN-482` 已冻结 authz capability registry、角色候选项 options API 和服务端 authz capability key 校验；`DEV-PLAN-484` 已冻结 route/tool overlay/registry/policy 覆盖门禁；`DEV-PLAN-485` 已冻结从 API 角度查看全量 HTTP API 授权归属的 `API 授权目录` 页面。

但普通用户可见的 `功能授权项` 页面本身仍缺少清晰 owner：

1. 482 定义了候选项 API，但明确不拥有页面实现。
2. 484 只定义 UI 原则和门禁，不定义页面路由、主表列、空态、筛选或前端组件落点。
3. 485 是 API 正向目录，不能替代 authz capability 视角的普通授权项页面。
4. 488 是后置诊断视图，不能混入角色配置主路径，也不能作为 482A/485 首批闭环前置。

因此需要 482A 承接一个小实施切片，把 `功能授权项` 页面和 `关联 API` 弹窗从原则变成可交付契约。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 新增授权管理只读页面 `功能授权项`。
2. [ ] 定义主页面路由、菜单落点、表格列、搜索筛选、加载态、空态和错误态。
3. [ ] 定义页面默认只消费 482 普通 options 口径：`enabled + assignable + tenant_api + 当前 tenant API 覆盖`。
4. [ ] 定义 `关联 API` 弹窗的数据契约、触发方式、展示列、空态和测试切片。
5. [ ] 明确页面不得展示 executor key，不得把 API method/path 常驻在主表，也不得从 route、policy CSV、导航配置或本地常量反推候选项。

### 2.2 非目标

1. 不新增 DB 表、迁移或在线 registry 编辑能力；首期仍以 482 的静态 registry 和服务端聚合结果为事实源。
2. 不提供新增、编辑、删除、启用、停用、修复、刷新覆盖事实或导出功能。
3. 不承担角色定义保存、角色持久化或用户授权组织范围；这些分别由 `DEV-PLAN-481/487/489` 承接。
4. 不展示不可分配、停用、废弃、非 tenant surface 或无覆盖 authz capability；这些归 `DEV-PLAN-488` 的 `授权项诊断`。
5. 不做 API 正向目录；全量 HTTP API method/path 主表归 `DEV-PLAN-485`。
6. 不展示 CubeBox executor key；当前授权事实统一基于 HTTP API、route requirement 与 authz capability registry。

## 3. 页面与交互

### 3.1 菜单与页面

- 菜单：`授权管理 > 功能授权项`
- 页面标题：`功能授权项`
- 页面副标题：`查看当前可分配功能权限及其授权项标识`
- 页面属性：只读，不提供新增、编辑、删除、修复、刷新、导出首期能力。
- 权限保护：首期使用已登记并有覆盖的 `iam.authz:read`。

### 3.2 主表列契约

主表只展示 authz capability 语义，不常驻展示 API method/path。

| 列 | 含义 | 示例 |
| --- | --- | --- |
| 资源名称 | registry 展示标签 | `组织管理` |
| 操作 | action 展示标签或 action | `查看` |
| 授权项标识 | `object:action` authz capability key，可点击打开 `关联 API` 弹窗 | `orgunit.orgunits:read` |
| 归属模块 | `owner_module` | `orgunit` |
| 范围维度 | `none` / `organization` | `organization` |
| 当前覆盖 | 是否有当前 tenant API 覆盖；普通列表必须为 `是` | `是` |
| 状态 | 普通列表必须为 `enabled` | `enabled` |

说明：

1. 主表不得新增 `API 路径`、`方法`、`executor key`、`调用策略`、`route source`、`lint 状态` 等诊断列。
2. `授权项标识` 是 authz capability key，不是 API path。点击该 key 打开 `关联 API` 弹窗。
3. 如果实现阶段希望把 `可分配` 或 `surface` 放进主表，必须保持普通列表中它们固定为 `是` / `tenant_api`，不得借此混入诊断全集。

### 3.3 搜索与筛选

首期支持：

1. 搜索框按 `key`、资源名称、操作标签、`owner_module` 搜索。
2. 筛选器支持归属模块、范围维度。
3. 不提供 `include_disabled`、`include_uncovered`、`surface=internal_system` 等诊断开关；诊断需求去 `授权项诊断`。

### 3.4 状态

| 状态 | 行为 |
| --- | --- |
| 加载态 | 表格区域显示骨架或加载进度，不展示伪数据 |
| 空态 | 展示“暂无可分配功能授权项”，不提供创建按钮 |
| 搜索无结果 | 展示“没有匹配的功能授权项”，保留清除筛选入口 |
| 错误态 | 展示服务端明确错误信息；不得回退到前端本地权限列表 |

## 4. 关联 API 弹窗

### 4.1 触发方式

用户点击主表中的 `授权项标识` 后打开弹窗。

- 弹窗标题：`关联 API`
- 弹窗副标题或上下文：展示当前 authz capability key，例如 `orgunit.orgunits:read`
- 弹窗属性：只读；不提供编辑 route、修复 registry、刷新覆盖或跳转修改入口。

### 4.2 展示列

| 列 | 含义 | 示例 |
| --- | --- | --- |
| 方法 | HTTP method | `GET` |
| API 路径 | HTTP route path | `/org/api/org-units` |
| 访问控制 | `protected` 等服务端分类 | `受保护` |
| 归属模块 | route 或 registry owner module | `orgunit` |
| 丘宝可调用 | 该 HTTP API 是否进入 CubeBox HTTP API 工具面 | `是` |

说明：

1. 弹窗只展示当前 authz capability key 关联的 API，不展示全量 API。
2. 弹窗不得展示 executor key、tool executor 名称或第二业务工具 key。
3. 弹窗不得把 method/path 回写到主表的 `授权项标识`。

### 4.3 弹窗空态

普通 `功能授权项` 默认列表理论上只展示有覆盖 authz capability，因此弹窗通常不应为空。若服务端返回空数组，弹窗展示“暂无关联 API”，并保留关闭按钮；这属于数据异常或竞态，不得在前端伪造 API 行。

## 5. 数据契约

### 5.1 主列表数据

主列表优先消费 482 定义的：

```text
GET /iam/api/authz/capabilities
```

默认查询不传诊断参数，服务端只返回：

```text
enabled + assignable + tenant_api + 当前 tenant API 覆盖
```

页面不得从前端路由、导航配置、policy CSV、静态 fixture 或 `permissionKey` 拼装候选项。

### 5.2 关联 API 反向查询

首期必须复用 484 的单一覆盖事实聚合能力，并在服务端提供一个 authz capability 反向查询口径。endpoint 形态可以在实施 PR 中固定，但覆盖关系的 join 只能发生在 484 同源聚合层：

1. 复用 485 endpoint facade：

```text
GET /iam/api/authz/api-catalog?authz_capability_key={key}
```

2. 新增更窄的反向 endpoint facade：

```text
GET /iam/api/authz/capabilities/{authz_capability_key}/apis
```

选择规则：

1. 无论选择哪个 endpoint，响应都必须由服务端按 `authz_capability_key` 过滤，前端不得拉全量后本地筛选。
2. 窄 endpoint 只能包装 484 单一覆盖事实聚合能力，不得复制第二套 route/registry/policy/CubeBox overlay 关联判断。
3. endpoint 必须受 `iam.authz:read` 或更严格已登记 capability 保护。

建议响应字段：

```json
{
  "authz_capability_key": "orgunit.orgunits:read",
  "apis": [
    {
      "method": "GET",
      "path": "/org/api/org-units",
      "access_control": "protected",
      "owner_module": "orgunit",
      "cubebox_callable": true
    }
  ]
}
```

## 6. 与现有计划的分工

| 计划 | Owner |
| --- | --- |
| `DEV-PLAN-480` | 授权体系蓝图与授权管理 UI 总体边界 |
| `DEV-PLAN-481` | 角色定义与用户授权交互边界 |
| `DEV-PLAN-482` | authz capability registry 字段模型、普通 options API、角色保存 authz capability 校验 |
| `DEV-PLAN-482A` | `功能授权项` 只读主页面与 `关联 API` 反向弹窗 |
| `DEV-PLAN-483` | canonical `object:action` 单主源、旧权限语言硬删除 |
| `DEV-PLAN-484` | route/tool overlay/registry/policy 覆盖事实与 CI 反漂移门禁 |
| `DEV-PLAN-485` | `API 授权目录`，从 API 角度查看 method/path 到授权项的绑定 |
| `DEV-PLAN-488` | `授权项诊断`，查看未进入普通候选项的能力及原因 |

482A 只消费 482 的普通 options API 和 484 的单一覆盖事实聚合能力。485 可提供 API 视角 facade，但不得成为 482A 之外的第二套覆盖事实来源；482A 不复制 registry、route、policy 或 CubeBox overlay 解析。

## 7. 实施切片

### 7.1 P0：契约冻结

1. [ ] 482A 文档加入 AGENTS Doc Map。
2. [ ] 480/482/484/485/488 引用 482A，明确普通 `功能授权项` 页面与 `关联 API` 弹窗 owner 不再散落。
3. [ ] 冻结主表不展示 API method/path、弹窗不展示 executor key 的约束。

### 7.2 P1：服务端查询聚合

1. [ ] 复用 482 registry/options 能力输出普通功能授权项列表。
2. [ ] 复用 484 单一覆盖事实聚合能力，支持按 `authz_capability_key` 反向查询关联 API。
3. [ ] 补服务层或 handler 黑盒测试，覆盖有覆盖、多 API 共享同一 key、未知 key、无覆盖 key。

### 7.3 P2：服务端 API

1. [ ] 确认主列表 endpoint 使用 `GET /iam/api/authz/capabilities` 默认口径。
2. [ ] 固定 `关联 API` 查询使用 `api-catalog?authz_capability_key=` 或窄 endpoint 二选一；二者都只能是 484 聚合结果的服务端过滤 facade。
3. [ ] endpoint 受已登记并有覆盖的 `iam.authz:read` 或更严格授权项保护。
4. [ ] handler/API 测试覆盖搜索筛选、默认不返回诊断全集、反向 API 查询不展示 executor key。

### 7.4 P3：前端页面

1. [ ] 新增授权管理菜单 `功能授权项`。
2. [ ] 新增只读主页面，按列契约展示数据。
3. [ ] 支持搜索、归属模块筛选、范围维度筛选、加载态、空态、搜索无结果和错误态。
4. [ ] 点击 `授权项标识` 打开 `关联 API` 弹窗。
5. [ ] 弹窗按列契约展示 method/path/access_control/owner_module/cubebox_callable，不展示 executor key。

### 7.5 P4：测试与门禁

1. [ ] 前端测试覆盖主列表渲染、搜索筛选、空态、错误态。
2. [ ] 前端测试覆盖点击授权项标识打开 `关联 API` 弹窗，并验证 method/path 只出现在弹窗中。
3. [ ] 前端测试覆盖不可分配、停用、无覆盖或内部 surface 项不会出现在普通功能授权项默认列表。
4. [ ] 服务端测试覆盖 options 默认口径与关联 API 反向查询。
5. [ ] 命中 Authz/UI/文档时按 AGENTS 触发器运行对应门禁。

## 8. 验收标准

1. [ ] 授权管理菜单中存在 `功能授权项` 页面。
2. [ ] 页面默认只展示 `enabled + assignable + tenant_api + 当前 tenant API 覆盖` authz capability。
3. [ ] 主表展示资源名称、操作、授权项标识、归属模块、范围维度、当前覆盖和状态，不常驻展示 method/path。
4. [ ] 点击授权项标识能打开 `关联 API` 弹窗。
5. [ ] `关联 API` 弹窗展示当前 capability 关联的 method/path/access_control/owner_module/cubebox_callable。
6. [ ] 页面和弹窗均不展示 executor key，也不从前端本地常量、policy CSV 或导航配置反推授权项。
7. [ ] `API 授权目录` 仍由 485 作为 API 视角 facade 承接；`授权项诊断` 仍由 488 后置承接；三者共享 484 单一覆盖事实聚合源。
8. [ ] `make check doc`、Authz/UI 命中门禁通过。

## 9. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 功能授权项页面变成诊断台 | 停用、无覆盖、内部 surface 项进入普通列表 | 普通列表只能消费 482 默认 options 口径，诊断去 488 |
| 主表堆 API 细节 | method/path 常驻主表，和 485 重复 | API method/path 只能在 `关联 API` 弹窗展示 |
| 前端本地拼候选项 | UI 与 registry/options 漂移 | 页面只能消费服务端 options API |
| 弹窗复制覆盖判断 | 反向 API 查询和 485/484 口径不同 | 必须复用 484 单一覆盖事实聚合源；485 只是正向目录 facade |
| executor 路线回流 | 弹窗出现 executor key 或第二工具 key | 当前授权事实只展示 HTTP API method/path，不展示 executor key |

## 10. 验证记录

- 2026-05-01 11:18 CST：创建方案文档，待实施阶段按命中范围运行 `make check doc`、Authz/UI 相关门禁与前端测试。
