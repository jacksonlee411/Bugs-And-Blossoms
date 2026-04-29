# DEV-PLAN-482：EHR Capability Registry 与角色能力候选项方案

**状态**: 规划中（2026-04-29 08:01 CST）

## 1. 背景

`DEV-PLAN-480` 已冻结 EHR 授权体系蓝图，`DEV-PLAN-481` 已冻结角色定义与用户授权的极简交互边界。二者都依赖一个前提：角色定义页面能从统一事实源拿到“所有可配置 capability”，并能由服务端校验提交的 key。

当前仓库还没有这个专门事实源：

- `pkg/authz/registry.go` 只有 role、object、action 常量，缺少结构化 capability 元数据。
- `config/access/policies/**` 只能表达“某角色已经被授予什么”，不能反推出“所有可选能力”。
- `apps/web` 的 `permissionKey` 来自构建期/本地配置，不能作为授权事实源或角色能力候选源。
- 历史 `Capability Registry` / capability key 下拉方案已归档，且绑定 SetID / scope/package 历史语义，不得作为当前实现前提。

因此需要一个独立方案承接：全量 capability registry、候选项 options API 与 capability key 校验规则。482 不拥有角色定义页面本身；角色基础信息、保存按钮和角色编辑工作流继续归属 `DEV-PLAN-481`。现有前端 `permissionKey`、旧 key 的硬删除要求由 `DEV-PLAN-483` 承接；新增 API/executor 必然进入权限目录、policy-only 权限与覆盖证据门禁由 `DEV-PLAN-484` 承接。482 不提供兼容映射。

## 2. 目标

1. [ ] 冻结功能权限标识 / capability key 格式：统一为 `object:action`，例如 `orgunit.orgunits:read`；不得新增 `orgunit.view` 这类 `module.verb` 兼容别名。
2. [ ] 冻结 `Capability Registry` 的最小元数据，使 UI 能展示资源、动作、中文/英文标签、范围维度、启停状态。
3. [ ] 定义服务端 options API，使 `DEV-PLAN-481` 的角色定义页可从该 API 获取全部启用且可分配的 capability。
4. [ ] 定义 capability key 校验契约：角色保存提交的 key 必须存在于 registry 且处于可分配状态。
5. [ ] 定义 registry 校验基础，供 `DEV-PLAN-484` 校验 policy、route authz、executor requirement、role definition 与 registry 不得漂移。
6. [ ] 对齐 `DEV-PLAN-483/484`：registry 与 options API 只输出 canonical `object:action`，不输出旧 `permissionKey` 或别名，且不输出无当前实现覆盖的 assignable capability。

## 3. 非目标

1. 不在本计划中新建 DB 表、迁移或在线 capability registry 管理页；如后续需要 DB SoT，必须另起计划并获得用户手工确认。
2. 不把 `config/access/policies/**` 当成候选项来源；policy 只表达授权结果，不表达可选全集。
3. 不把前端 `permissionKey`、`VITE_PERMISSIONS` 或导航配置当成授权事实源。
4. 不恢复 SetID、scope/package、legacy capability key 或历史兼容别名。
5. 不把组织范围、字段策略、有效期、冲突检测放回角色定义页；这些边界继续以 `DEV-PLAN-480/481` 为准。
6. 不维护旧 key 到新 key 的映射；旧 key 的删除与反回流验收以 `DEV-PLAN-483` 为准。
7. 不在 482 内重复定义覆盖门禁；新增 API/executor 与 registry/policy/options 的覆盖校验以 `DEV-PLAN-484` 为准。

## 4. 事实源设计

### 4.1 Registry 归属

首期 registry 归属 `pkg/authz`，以代码内结构化表作为 SoT，复用现有 object/action 常量并补齐元数据。这样可以先服务校验、路由/策略 lint 和 options API，不引入 DB schema。

后续若要让管理员在线维护 registry，必须另起 DB/API 方案；本计划只冻结首期静态 registry。

### 4.2 Capability Entry

最小字段：

| 字段 | 含义 |
| --- | --- |
| `key` | 固定为 `object:action`，由 `object` 与 `action` 派生，不手写第二套 |
| `object` | Casbin object/resource，例如 `orgunit.orgunits` |
| `action` | Casbin action，例如 `read`、`admin`、`use` |
| `owner_module` | 归属模块，例如 `orgunit`、`iam`、`cubebox` |
| `resource_label` / `action_label` | UI 展示标签；实现时可用 i18n key 或服务端本地化字段 |
| `scope_dimension` | `none` 或 `organization`；用户授权页据此判断是否需要组织范围 |
| `assignable` | 是否允许出现在角色定义候选项中 |
| `status` | `enabled`、`disabled`、`deprecated` |
| `sort_order` | UI 分组和排序 |

说明：

1. `key` 是功能权限标识，不是 API 地址。
2. 一个 `key` 可以覆盖多个 HTTP API route 或 executor；具体覆盖关系由 route/executor requirement 提供实现证据。
3. 482 的 options API 默认返回 capability 元数据；若 UI 需要展示 API，应通过 `DEV-PLAN-484` 定义的覆盖接口证据读取或展开，不得把 route path 放进 `key` 字段。

派生规则：

1. `key = object + ":" + action`。
2. key 不允许手工覆盖。
3. 同一个 `object/action` 只能有一条 registry entry。
4. `assignable=false` 的能力可用于系统内部或超级管理员场景，但不进入普通角色定义候选项。

## 5. Options API

### 5.1 Endpoint

建议首期新增：

`GET /iam/api/authz/capabilities`

查询参数：

| 参数 | 说明 |
| --- | --- |
| `q` | 可选，按 key、资源标签、动作标签搜索 |
| `owner_module` | 可选，按模块过滤 |
| `scope_dimension` | 可选，按范围维度过滤 |
| `include_disabled` | 默认 `false`；仅授权管理员诊断场景允许开启 |

响应示例：

```json
{
  "capabilities": [
    {
      "key": "orgunit.orgunits:read",
      "object": "orgunit.orgunits",
      "action": "read",
      "owner_module": "orgunit",
      "resource_label": "组织管理",
      "action_label": "查看",
      "label": "组织管理 / 查看",
      "scope_dimension": "organization",
      "assignable": true,
      "status": "enabled",
      "sort_order": 100
    }
  ],
  "registry_rev": "20260429-static"
}
```

### 5.2 权限保护

该 endpoint 本身必须受授权保护。首期建议使用 `iam.authz:read`；角色保存和 registry 诊断类写操作如后续出现，应使用 `iam.authz:admin` 或更明确的 object/action。

## 6. 候选项消费契约

482 只定义候选项来源、选择器行为和服务端校验规则。`DEV-PLAN-481` 的角色定义页消费这些契约；482 不新增单独的角色定义界面，不定义角色名称、slug、描述或保存按钮。

角色定义页不从 policy CSV 反推候选项，必须从 options API 获取候选 capability。

推荐交互：

1. 能力数量较少时：按资源分组的矩阵，行是资源，列是动作，选中后形成 capability key 集合。
2. 能力数量较多时：使用可搜索 `Autocomplete`，展示 `资源 / 操作 / 功能权限标识`，支持按模块或范围维度筛选。
3. 已保存角色中出现未知、禁用或废弃 key 时：由 481 页面以警告 chip 展示，并由服务端阻止保存，直到管理员移除或替换。
4. 消费方不允许 freeSolo 手输 capability key；管理员只能选择 registry 返回的候选项。

## 7. 校验与门禁

### 7.1 服务端校验

角色保存时必须调用或复用 482 的 capability 校验规则：

1. capability key 格式为 `object:action`。
2. key 存在于 registry。
3. entry `status=enabled` 且 `assignable=true`。
4. 同一角色内 key 不重复。
5. 包含 `scope_dimension=organization` 的角色，在用户授权保存时必须有组织范围；角色定义页不手工维护 `scope_required`。

### 7.2 反漂移门禁

后续实现时应按 `DEV-PLAN-484` 增加或扩展 authz lint，至少覆盖：

1. `config/access/policies/**` 中每个 object/action 必须存在于 registry。
2. `internal/server` route requirement 中每个 object/action 必须存在于 registry。
3. `modules/cubebox` executor requirement 中每个 object/action 必须存在于 registry。
4. 角色定义 fixture / API payload 中每个 capability key 必须存在于 registry。
5. registry 中不得出现 `module.verb` 兼容别名或 SetID/scope/package 历史字段。
6. `enabled + assignable` 的 capability 必须具备当前 tenant API 或 internal executor 覆盖证据。

## 8. 实施切片

### 8.1 P0：契约冻结

1. [ ] 482 文档作为 capability registry 与角色候选项 SSOT 被 AGENTS Doc Map 收录。
2. [ ] 480/481 引用 482，明确角色定义页候选源不是 policy CSV，也不是前端 permissionKey。
3. [ ] 482 引用 484，明确覆盖门禁与空壳 capability 阻断不由 482 重复承接。
4. [ ] 明确首期不建 DB 表、不做在线 registry 管理。

### 8.2 P1：Registry 与校验

1. [ ] 在 `pkg/authz` 增加结构化 capability registry。
2. [ ] 增加 `ParseCapabilityKey`、`CapabilityKey(object, action)`、`LookupCapability`、`ListCapabilities` 等纯函数。
3. [ ] 增加角色 capability 校验函数，覆盖未知 key、禁用 key、重复 key、旧格式 key。
4. [ ] 补 `pkg/authz` 黑盒表驱动测试。

### 8.3 P2：Options API

1. [ ] 新增 `GET /iam/api/authz/capabilities`。
2. [ ] endpoint 受 `iam.authz:read` 保护。
3. [ ] 支持搜索与基础过滤，默认只返回 `enabled + assignable`。
4. [ ] 补 `internal/server` handler、authz requirement 与响应测试。

### 8.4 P3：481 页面消费契约

1. [ ] 481 的角色定义页从 options API 拉取候选 capability。
2. [ ] 481 页面使用资源-操作矩阵或可搜索 Autocomplete 展示全部可选项。
3. [ ] 481 的保存 payload 只提交 capability keys。
4. [ ] 481 UI 测试覆盖加载候选、搜索选择、未知 key 警告、禁用 key 阻断保存。

### 8.5 P4：门禁补强

1. [ ] 按 `DEV-PLAN-484` 扩展 authz lint，检查 policy、route requirement、executor requirement、role fixture 均引用 registry 已登记 object/action，并检查 assignable capability 覆盖证据。
2. [ ] 把旧格式 `module.verb` 与 SetID/scope/package 历史字段加入反回流检查。
3. [ ] 将门禁纳入 `make authz-lint` 或 `make check authz` 对应入口，避免新增独立漂移脚本无人运行。

## 9. 验收标准

1. [ ] 481 角色定义页的能力候选项可覆盖 registry 中全部 `enabled + assignable` capability。
2. [ ] 从 policy CSV 删除某条授权记录不会导致该 capability 从候选项消失。
3. [ ] registry 新增一个 `enabled + assignable` capability 后，options API 与 481 角色定义页可发现该项。
4. [ ] 未登记、禁用、废弃、旧格式 capability key 均不能被角色保存。
5. [ ] route authz、policy、CubeBox executor requirement 与 registry 漂移时，authz lint 失败。

## 10. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 把 policy 当候选源 | 只能选到已授权项，未分配能力不可发现 | options API 必须只读 registry |
| 前端手输 key 回流 | UI 可输入未知 capability | 481 消费方禁止 freeSolo，服务端二次校验 |
| registry 过早 DB 化 | 需要新增表和迁移 | 本计划停止，另起 DB 方案并获得用户确认 |
| 与 480/481 边界混淆 | 角色页出现组织范围或字段策略 | 回退到 481：角色只定义功能权限 |
| 历史 key 兼容 | `orgunit.view`、SetID/scope/package 字段回流 | lint 阻断，不提供兼容别名 |

## 11. 验证记录

- 待实施阶段按命中范围运行：`make check doc`、`go fmt ./... && go vet ./... && make check lint && make test`、`make authz-pack && make authz-test && make authz-lint`、前端测试与 E2E。
