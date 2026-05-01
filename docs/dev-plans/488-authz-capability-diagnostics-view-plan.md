# DEV-PLAN-488：授权项诊断视图方案

**状态**: 规划中（2026-05-01 10:31 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：后置新增一个只读诊断视图，用于查看 authz capability registry 中未进入普通功能授权项候选列表的能力及原因，避免不可分配、停用、无覆盖、系统内部能力混入角色配置主路径。
- **关联模块/目录**：`apps/web/src/**`、`internal/server/**`、`pkg/authz/**`、`scripts/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-482`、`DEV-PLAN-482A`、`DEV-PLAN-483`、`DEV-PLAN-484`、`DEV-PLAN-485`
- **用户入口/触点**：授权管理菜单中的 `授权项诊断` 只读页面、482 authz capability options/diagnostics 查询接口、484 单一覆盖事实聚合能力

### 0.1 Simple > Easy 三问

1. **边界**：488 只拥有“为什么某个 registry authz capability 不在普通功能授权项候选中”的后置只读诊断视图；482 继续拥有 registry 字段模型与普通 options API 主契约；484 继续拥有唯一服务端覆盖事实聚合源和 CI 门禁；485 继续拥有 API 正向目录 facade。
2. **不变量**：角色定义页和普通功能授权项默认列表只能展示 `enabled + assignable + tenant_api + 当前 tenant API 覆盖` 的 authz capability。诊断视图不得成为角色保存候选源，也不得放宽服务端保存校验。
3. **可解释**：授权管理员或开发排查人员能看到某个 authz capability 被排除的明确原因，例如停用、不可分配、非 tenant surface、无当前 API 覆盖或仅存在于 registry/policy 中。

## 1. 背景

`DEV-PLAN-482` 已冻结功能授权项 options API 默认口径：只输出启用、可分配、tenant API surface 且有当前实现覆盖的能力。这个口径适合角色定义和普通功能授权项页面，因为管理员看到的每一项都应该可授予、可执行。

488 不应成为首批授权管理闭环的前置条件。首批顺序应先完成 484 覆盖事实枚举与 lint，再让 482 options、482A `关联 API` 弹窗、485 `API 授权目录` 和 490 CubeBox API tool builder 消费同一聚合源；488 只在这些闭环稳定后提供治理排查视图。

但排查时还需要回答另一类问题：

1. registry 中哪些能力因为 `assignable=false` 没有进入候选项。
2. 哪些能力处于 `disabled` 或 `deprecated`。
3. 哪些 authz capability 没有当前 tenant API 覆盖。
4. 哪些能力属于 `superadmin` 或 `internal_system` surface。
5. 哪些 policy/registry 漂移已经被 484 lint 阻断，但需要在页面上辅助定位。

这些信息如果直接混进普通功能授权项页面，会让角色配置主路径变成诊断台，并可能误导管理员分配不可执行或系统内部能力。因此需要独立诊断视图。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 新增授权管理只读页面 `授权项诊断`，与 `功能授权项`、`API 授权目录` 分离。
2. [ ] 定义诊断查询口径：可返回普通候选项之外的 `disabled`、`deprecated`、`assignable=false`、非 `tenant_api` surface、无覆盖能力，并给出明确排除原因。
3. [ ] 明确诊断视图消费 482 registry 与 484 单一覆盖事实聚合结果，不复制 lint 逻辑，不替代 CI 门禁。
4. [ ] 明确角色定义页、角色保存校验和普通功能授权项默认列表不消费诊断全集。
5. [ ] 定义最小测试与验收，防止诊断字段回流到角色配置主路径。

### 2.2 非目标

1. 不新增 DB 表、迁移或在线 registry 编辑能力；首期仍以 `pkg/authz` 静态 registry 为 SoT。
2. 不提供启用、停用、修复、刷新、覆盖重算或 policy 修改按钮。
3. 不把无覆盖 authz capability 变成可分配能力；无覆盖的 `assignable=true` 仍必须按 `DEV-PLAN-484` 导致 lint 失败。
4. 不在角色定义页、普通功能授权项主表或保存 payload 中暴露不可分配、停用、无覆盖、内部能力。
5. 不把 API method/path 当成 authz capability key；API 只作为覆盖证据展示，普通功能授权项的反向 `关联 API` 弹窗仍按 482A，正向 API 目录仍按 485。
6. 不展示 CubeBox executor key；当前授权事实统一基于 HTTP API、route requirement 与 authz capability registry。

## 3. 页面与交互

### 3.1 菜单与页面

- 菜单：`授权管理 > 授权项诊断`
- 页面标题：`授权项诊断`
- 页面属性：只读诊断，不提供新增、编辑、删除、修复、刷新、导出首期能力。
- 权限保护：首期使用已登记并有覆盖的 `iam.authz:read`；若实际实现需要展示 `superadmin` 或 `internal_system` 详情，应在实现计划中评估是否升级到 `iam.authz:admin` 或 superadmin-only。

### 3.2 表格列契约

| 列 | 含义 | 示例 |
| --- | --- | --- |
| 授权项标识 | `object:action` authz capability key | `orgunit.orgunits:read` |
| 资源名称 | registry 展示标签 | `组织管理` |
| 操作 | action 展示标签或 action | `查看` |
| 归属模块 | `owner_module` | `orgunit` |
| 状态 | `enabled` / `disabled` / `deprecated` | `enabled` |
| 可分配 | `assignable` | `否` |
| Surface | `tenant_api` / `superadmin` / `internal_system` | `internal_system` |
| 当前覆盖 | 是否有当前 tenant API 覆盖 | `否` |
| 诊断原因 | 排除或异常原因 | `not_assignable` |

主表不常驻展示 API method/path。点击授权项标识可沿用 `DEV-PLAN-482A` 的“关联 API”弹窗展示覆盖 API；如果没有覆盖，弹窗展示空态，不把无覆盖当成可分配。

### 3.3 过滤与搜索

首期支持：

1. 按 `authz_capability_key`、资源名称、`owner_module` 搜索。
2. 按状态、可分配、surface、当前覆盖、诊断原因筛选。
3. 默认筛选建议为“仅显示异常/被排除项”，避免和普通功能授权项重复；用户可切换查看全部 registry entries。

## 4. 数据契约

### 4.1 数据来源

服务端聚合来源：

1. 482 authz capability registry：`authz_capability_key/object/action/owner_module/resource_label/action_label/scope_dimension/assignable/status/surface/sort_order`。
2. 484 单一覆盖事实聚合结果：`authz_capability_key -> covered_routes[]` 与是否具备当前 tenant API 覆盖。
3. 484 policy/route/tool overlay 聚合结果：仅用于派生诊断原因，不在 488 中重新解析源文件或重新实现 lint。

### 4.2 建议 Endpoint

优先复用 482 endpoint 的同源服务层，但不要让角色定义页使用诊断全集。

建议新增专用查询面：

```text
GET /iam/api/authz/capability-diagnostics
```

该 endpoint 必须受 `iam.authz:read` 或更严格授权项保护。若实现阶段决定复用 `GET /iam/api/authz/capabilities` 的 `include_disabled/include_uncovered` 参数，也必须在路由/handler 层区分调用场景，确保角色定义页和普通功能授权项默认列表无法通过前端参数切到诊断全集。

查询参数：

| 参数 | 说明 |
| --- | --- |
| `q` | 可选，按 authz capability key、资源标签、动作标签搜索 |
| `owner_module` | 可选，按模块过滤 |
| `status` | 可选，`enabled` / `disabled` / `deprecated` |
| `assignable` | 可选，按可分配状态过滤 |
| `surface` | 可选，按 surface 过滤 |
| `covered` | 可选，按当前 tenant API 覆盖状态过滤 |
| `diagnostic_reason` | 可选，按诊断原因过滤 |

响应示例：

```json
{
  "capabilities": [
    {
      "authz_capability_key": "iam.internal_jobs:admin",
      "object": "iam.internal_jobs",
      "action": "admin",
      "owner_module": "iam",
      "resource_label": "内部任务",
      "action_label": "管理",
      "scope_dimension": "none",
      "assignable": false,
      "status": "enabled",
      "surface": "internal_system",
      "covered": false,
      "diagnostic_reasons": ["not_assignable", "non_tenant_surface", "uncovered"],
      "sort_order": 9000
    }
  ],
  "registry_rev": "20260429-static"
}
```

### 4.3 诊断原因

首期原因码：

| 原因码 | 含义 |
| --- | --- |
| `candidate` | 满足普通功能授权项候选口径 |
| `disabled` | `status=disabled` |
| `deprecated` | `status=deprecated` |
| `not_assignable` | `assignable=false` |
| `non_tenant_surface` | `surface` 不是 `tenant_api` |
| `uncovered` | 没有当前 tenant API 覆盖 |
| `policy_only` | policy 引用存在但没有当前实现覆盖，正常应被 484 lint 阻断 |
| `registry_only` | registry entry 存在但没有 route/policy/tool overlay 关联证据 |

说明：

1. `candidate` 不是异常，只表示该项会进入普通功能授权项默认列表。
2. `uncovered` 与 `assignable=true/status=enabled/surface=tenant_api` 同时出现时，属于 484 必须阻断的空壳能力；页面可展示诊断，但不能让 CI 放行。
3. `policy_only` 和 `registry_only` 是排查辅助，不得作为允许漂移存在的理由。

## 5. 与现有计划的分工

| 计划 | Owner |
| --- | --- |
| `DEV-PLAN-480` | 授权体系蓝图与授权管理 UI 总体边界 |
| `DEV-PLAN-482` | authz capability registry 字段模型、普通功能授权项 options API、角色保存 authz capability 校验 |
| `DEV-PLAN-482A` | 普通 `功能授权项` 主页面与反向 `关联 API` 弹窗 |
| `DEV-PLAN-483` | canonical `object:action` 单主源、旧权限语言硬删除 |
| `DEV-PLAN-484` | route/tool overlay/registry/policy 覆盖事实与 CI 反漂移门禁 |
| `DEV-PLAN-485` | API 授权目录，从 API 角度查看 method/path 到授权项的绑定 |
| `DEV-PLAN-488` | 授权项诊断视图，从 authz capability 角度查看未入候选项原因 |

488 只消费 484 的单一覆盖事实聚合结果，不复制一套覆盖判断。488 的页面存在不改变 482 默认 options API 的候选项语义，也不改变 483 对旧 key 的硬删除要求。488 是 484+482A+485 首批闭环之后的治理增强，不作为功能授权项页面、关联 API 弹窗或 API 授权目录的前置交付。

## 6. 实施切片

### 6.1 P0：契约冻结

1. [ ] 488 文档加入 AGENTS Doc Map。
2. [ ] 480/482/483/484/485 引用 488，明确授权项诊断不属于普通功能授权项候选列表，也不属于 API 授权目录，且不作为 482A/485 首批闭环前置。
3. [ ] 冻结诊断原因码和页面只读边界。

### 6.2 P1：诊断数据聚合

1. [ ] 复用 482 registry 枚举能力。
2. [ ] 复用 484 单一覆盖事实聚合能力；若缺诊断字段，先补 484 聚合输出，不在 488 新建第二套枚举。
3. [ ] 增加诊断原因派生函数，覆盖候选项、停用、废弃、不可分配、非 tenant surface、无覆盖、policy-only、registry-only。
4. [ ] 补 `pkg/authz` 或服务层黑盒表驱动测试。

### 6.3 P2：服务端 API

1. [ ] 新增 `GET /iam/api/authz/capability-diagnostics`，或在实现前明确复用 482 endpoint 的受控诊断参数方案。
2. [ ] endpoint 受已登记并有覆盖的 `iam.authz:read` 或更严格授权项保护。
3. [ ] 支持搜索和基础筛选。
4. [ ] 补 handler/API 测试，覆盖默认候选与诊断全集不会混淆。

### 6.4 P3：前端页面

1. [ ] 新增授权管理菜单 `授权项诊断`。
2. [ ] 新增只读列表页，按列契约展示数据。
3. [ ] 支持搜索与筛选。
4. [ ] 点击授权项标识可打开“关联 API”弹窗；无覆盖时展示空态，不提供修复入口。

### 6.5 P4：测试与门禁

1. [ ] 前端测试覆盖普通功能授权项默认列表不显示不可分配、停用、无覆盖、内部能力。
2. [ ] 前端测试覆盖授权项诊断能显示这些被排除项及诊断原因。
3. [ ] 服务端测试覆盖角色保存仍拒绝未知、禁用、不可分配、非 tenant surface、无覆盖 key。
4. [ ] 命中 Authz/UI/文档时按 AGENTS 触发器运行对应门禁。

## 7. 验收标准

1. [ ] `授权管理 > 功能授权项` 默认只展示 `enabled + assignable + tenant_api + 当前 tenant API 覆盖` authz capability。
2. [ ] `授权管理 > 授权项诊断` 可查看普通候选项之外的不可分配、停用、废弃、无覆盖、内部 surface authz capability，并展示明确诊断原因。
3. [ ] 角色定义页不能通过前端参数切换到诊断全集。
4. [ ] 角色保存接口继续拒绝未知、禁用、不可分配、非 tenant surface、无覆盖 key。
5. [ ] `assignable=true/status=enabled/surface=tenant_api` 但无当前 API 覆盖时，484 lint 仍失败；诊断视图不能替代 CI 阻断。
6. [ ] 页面不展示 executor key，不把 API path 当 authz capability key。
7. [ ] `make check doc`、Authz/UI 命中门禁通过。

## 8. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 诊断全集混入角色候选项 | 管理员可分配停用/无覆盖/内部能力 | 角色定义页只能使用 482 默认候选口径，服务端二次校验 |
| 用诊断页替代 lint | 空壳 authz capability 被展示但 CI 放行 | 484 lint 仍是阻断 owner，488 只读展示 |
| 诊断页抢跑成为前置 | 482A/485 尚未闭环就先实现 488，导致排查视图倒逼三套解析 | 488 后置；没有 484 单一聚合源时不得实现诊断事实枚举 |
| 诊断页变成 registry 管理台 | 页面出现启用、修复、编辑 policy 按钮 | 首期只读；在线管理另起计划 |
| 复制 API 目录 | 主表常驻 method/path，和 485 重复 | API method/path 只进关联 API 弹窗；正向目录归 485 |
| 泄露内部 surface | 普通 tenant 管理员看到 superadmin/internal 细节 | 实现阶段按权限分级；必要时升级到 `iam.authz:admin` 或 superadmin-only |

## 9. 验证记录

- 2026-05-01 10:31 CST：创建方案文档，待实施阶段按命中范围运行 `make check doc`、Authz/UI 相关门禁与前端测试。
