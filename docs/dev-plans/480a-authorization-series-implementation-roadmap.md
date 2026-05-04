# DEV-PLAN-480A：480 系列授权子计划实施顺序路线图

**状态**: P1/P2 只读治理面、P3/P4 后端运行时闭环、481 UI 保存交互与 A/B 组织范围 E2E、P5 CubeBox API-first 首期硬切换及评审修复已完成；491 Phase A/B/C/D 已完成 selector facade、组件与主要组织选择入口接入；P6 授权项诊断仍待后续补齐，492 组织管理页浏览/编辑主树读取收敛与 SQL 级 scoped pagination 仍待后续（2026-05-04 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结 `DEV-PLAN-480` 系列授权子计划的实施顺序、依赖门槛和停止线，避免 capability、API 目录、角色、用户授权、CubeBox 工具化和诊断视图并行落地时形成重复事实源或半成品在线入口。
- **关联模块/目录**：`pkg/authz/**`、`config/access/**`、`scripts/authz/**`、`internal/server/**`、`modules/iam/**`、`modules/orgunit/**`、`modules/cubebox/**`、`apps/web/src/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-012`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-482A`、`DEV-PLAN-483`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-486`、`DEV-PLAN-487`、`DEV-PLAN-488`、`DEV-PLAN-489`、`DEV-PLAN-489A`、`DEV-PLAN-490`
- **用户入口/触点**：授权管理下的角色管理、用户授权、功能授权项、API 授权目录、后置授权项诊断、CubeBox API-first 工具链

### 0.1 Simple > Easy 三问

1. **边界**：480A 只规定实施顺序和跨计划依赖，不新增 registry 字段、DB schema、endpoint payload、UI 页面列或运行时授权语义。
2. **不变量**：所有依赖覆盖事实的页面、options、tool builder 和诊断视图必须消费 `DEV-PLAN-484` 的单一覆盖事实聚合源；普通 tenant role 运行时不得同时由 DB role SoT 与 policy CSV 放行；用户授权组织范围必须与运行时 scope provider 同闭环交付。
3. **可解释**：先完成权限标识和覆盖事实基础，再交付只读可视化；角色定义保存和用户授权范围作为运行时闭环推进；CubeBox 在 API 目录可追溯后切到 API-first；诊断页最后做。

## 1. 背景

480 系列已经把授权体系拆成多个子计划：

| 计划 | 职责 |
| --- | --- |
| `DEV-PLAN-480` | EHR 授权体系蓝图、分层原则、管理 UI 总边界 |
| `DEV-PLAN-481` | 角色定义与用户授权 UI / 交互边界 |
| `DEV-PLAN-482` | authz capability registry、options API、key 校验 |
| `DEV-PLAN-482A` | 普通功能授权项页面与反向关联 API 弹窗 |
| `DEV-PLAN-483` | canonical `object:action` 单主源和旧权限语言硬删除 |
| `DEV-PLAN-484` | 覆盖事实单一聚合源与 authz lint 门禁 |
| `DEV-PLAN-485` | API 授权目录正向只读页面 |
| `DEV-PLAN-486` | executor 路线已停止的活体警示 |
| `DEV-PLAN-487` | 角色定义 DB SoT、保存 API、普通 tenant role cutover |
| `DEV-PLAN-488` | 后置授权项诊断视图 |
| `DEV-PLAN-489` | 用户授权角色集合、组织范围 SoT、scope provider 与 orgunit 强制裁剪 |
| `DEV-PLAN-489A` | principal 多角色 union 运行时契约 |
| `DEV-PLAN-490` | CubeBox API-first 工具化，删除业务 executor 双链路 |

这些计划边界基本清楚，但如果并行实现顺序不受控，容易出现：

1. 482A、485、488、490 各自解析 route / registry / policy / CubeBox overlay，形成多套覆盖事实。
2. 487 暴露角色保存入口，但普通请求仍按 policy CSV 或单 `role_slug` 判定。
3. 489 用户授权页可保存角色和组织范围，但 orgunit / CubeBox 运行时仍按全租户读。
4. 488 诊断视图抢跑，把不可分配、停用、无覆盖或内部能力混进角色配置主路径。
5. 490 tool overlay 在 484 / 485 未完成前手写 `method/path/object/action/authz_capability_key`，成为第二 API 目录。

480A 的目标是把这些依赖关系前置冻结，作为 480 系列执行入口。

## 2. 目标与非目标

### 2.1 核心目标

1. [X] 冻结 480 系列实施阶段顺序和每阶段出入口条件。
2. [X] 明确哪些计划可并行，哪些计划必须等待前置门槛。
3. [X] 明确不得提前暴露的在线入口，避免半闭环功能进入用户可见面。
4. [X] 明确重复实现停止线：覆盖事实、角色能力来源、组织范围来源、CubeBox 工具来源都只能有一个 active runtime SoT。
5. [X] 明确并完成 487/489/489A 后端运行时闭环：角色定义保存生效、用户授权角色集合和组织范围进入服务端 SoT，orgunit 与 CubeBox orgunit 查询按服务端 scope provider 裁剪。
6. [X] 补齐 481 角色/用户授权 UI 保存交互与 A/B 组织范围 E2E；完成 490 CubeBox API-first 首期硬切换，488 授权项诊断视图仍待后续。

### 2.2 非目标

1. 480A 本身不新增 DB 表、迁移、sqlc query 或 API payload；涉及 schema 的 `DEV-PLAN-487/489` 已在实施前获得用户手工确认，后续新增 schema 仍必须再次确认。
2. 不替代各子计划的详细契约；字段、endpoint、UI 列、测试用例以对应子计划为准。
3. 不新增字段级授权、字段脱敏管理 UI、授权摘要页、权限核查页、角色复制、有效期、team/position 授权或写入 API 确认机制。
4. 不恢复 `permissionKey`、legacy key、SetID、scope/package、org_level/scope_type/scope_key、CubeBox executor 或 `READ_PLAN` 兼容窗口。

## 3. 实施顺序总览

```text
P0  文档与语义冻结
    -> P1  权限标识 / registry / 覆盖事实门禁
       -> P2  只读授权目录面
          -> P3  角色定义 DB SoT 与运行时准备
             -> P4  用户授权组织范围 SoT 与统一运行时 cutover
                -> P5  CubeBox API-first 迁移
                   -> P6  授权项诊断后置治理
```

关键门槛：

1. 没有 484 P1/P2 单一覆盖事实聚合源，不得实现 482A/485/488/490 的覆盖 join。
2. 487/489/489A 后端运行时闭环、481 UI 保存交互与 A/B 组织范围 E2E、490 API-first 首期硬切换及评审修复已完成；491 Phase A/B/C/D 的 selector facade、组件与主要组织选择入口接入已完成，但不得把 488 诊断视图、更广 491/492 联合 E2E、组织管理页浏览/编辑主树读取收敛或 492 SQL scoped pagination 一并宣称完成。
3. 489 scope provider 与 orgunit 裁剪已完成；用户授权 UI 保存交互已消费 489 API，不得落回前端本地状态。
4. 没有 485 API 授权目录基础投影和 490 overlay 校验，不得让 CubeBox planner 调用 API 工具。
5. 488 诊断视图必须后置到 484 + 482A + 485 首批闭环之后。
6. 487/489/489A 不得分别宣布运行时完成；角色定义 DB SoT、`principal_role_assignments`、scope provider 和多角色 union 必须作为同一个运行时闭环验收。

## 4. 阶段路线图

### 4.1 P0：文档与入口收敛

Owner：`DEV-PLAN-480/480A`

1. [X] 480A 加入 `AGENTS.md` Doc Map，作为 480 系列执行顺序入口。
2. [X] 480 系列子计划引用 480A 时，只引用实施顺序，不把具体契约复制进 480A。
3. [X] `DEV-PLAN-486` 继续作为 executor 路线停止警示；当前实现 owner 是 `DEV-PLAN-490`。
4. [X] 若发现 410/411 等相邻文档链接失效，应另行做 Doc Map 清理；不得把缺失文档当成 480 当前实现前提。

出入口条件：

- 出口：所有 480 系列实施 PR 能用 480A 判断“是否具备前置条件”。P0 已把 `DEV-PLAN-480A` 收入口文档地图与 480 系列关联计划入口；后续阶段仍以各 owner 子计划为详细契约。

### 4.2 P1：权限标识、registry 与覆盖事实基础

Owner：`DEV-PLAN-483/482/484`

建议顺序：

1. [X] `DEV-PLAN-483`：删除旧 `permissionKey`、`VITE_PERMISSIONS`、`module.verb`、policy-only key 和旧 key fallback；前端当前用户能力只来自服务端 canonical `authz_capability_keys`。
2. [X] `DEV-PLAN-482`：建立 `pkg/authz` 静态 authz capability registry、key 解析/构造/校验函数、默认 options API 口径。
3. [X] `DEV-PLAN-484`：建立单一覆盖事实聚合源，枚举 route requirement、registry、policy，并为 DB role seed 与 CubeBox API tool overlay 提供同一枚举接口；487/490 已接入同一聚合源，后续不得在 482A/485/488/490 中另起第二套 join。

并行规则：

- 482 的纯 registry 结构和 key 函数可与 483 并行。
- 482 options API 的“当前 tenant API 覆盖”过滤必须等待 484 聚合源。
- 任何前端页面不得在 P1 期间手写候选项或覆盖关系 fixture 作为事实源。

停止线：

1. 发现 `requiredCapabilityKey="orgunit.read"` 等旧值时停止，回到 483。
2. 发现 482A/485/488/490 自行解析 route/policy/registry 时停止，回到 484。
3. `assignable=true/status=enabled/surface=tenant_api` 但无当前 API 覆盖时必须 lint 失败，不能靠页面诊断放行。
4. route requirement 必须能反向匹配 allowlist/实际 route surface；不存在实际 route 的手写 requirement 不得计为当前 API 覆盖。
5. 普通 options API 不暴露诊断全集参数；诊断需求后置到 488 专用入口。

### 4.3 P2：只读授权目录面

Owner：`DEV-PLAN-482A/485`

建议顺序：

1. [X] `DEV-PLAN-482A`：实现 `功能授权项` 普通只读页面，只消费 482 默认 options 口径。
2. [X] `DEV-PLAN-485`：实现 `API 授权目录` 正向只读页面，当前覆盖的普通 tenant 授权 API 视角投影必须来自 484 聚合源。
3. [X] 482A 的 `关联 API` 弹窗固定复用 485 的 API catalog facade 和服务端过滤口径；首期不新增第二个窄 endpoint。具体 endpoint 与 payload 以 482A/485 为准，480A 只冻结“复用同一 facade、不新增第二事实源”的顺序约束。若后续确有性能、权限边界或响应形态隔离需求，必须先更新 482A/485/480A 契约或另起计划说明原因。

并行规则：

- 482A 和 485 前端可并行，但服务端都必须依赖同一个 484 聚合包/服务。
- 485 不拥有覆盖事实枚举；482A 不拥有 API 正向目录。

停止线：

1. 482A 主表常驻展示 API method/path 时停止，转回 485 或关联 API 弹窗。
2. 485 页面展示不可分配、停用、无覆盖或内部 surface 的 authz capability 全集时停止，转回 488。
3. 页面出现 executor key、tool executor 名称或 `调用策略` 主表列时停止，转回 486/490 边界。

### 4.4 P3：角色定义 DB SoT 与运行时准备

Owner：`DEV-PLAN-481/487`

建议顺序：

1. [X] `DEV-PLAN-481`：角色定义 UI 只提交基础信息、`role_slug`、`revision` 和 `authz_capability_keys`。
2. [X] `DEV-PLAN-487`：已获得新增 DB 表手工确认，落地 role definition / role authz capability DB SoT、保存 API、服务端校验。
3. [X] `DEV-PLAN-487` 提供角色定义摘要和 role capability 读取能力，并已供 489A union 与 489 scope provider 消费。
4. [X] P3 route 挂载为可调用 API 前已同步完成 P4 后端 cutover，未交付“保存成功但运行时不生效”的在线入口。

并行规则：

- 481 UI 保存交互已接入 487 API；后续不得用本地状态模拟 487 API。
- 487 API route 已在 P4 后端同步完成后挂载为可调用保存入口。
- 489A 的门面和反回流门禁已随 489 `principal_role_assignments` 事实源落地后统一启用。
- 487 的单独状态只能表示角色定义 DB SoT 子能力完成；480 系列运行时后端闭环以 487/489/489A 组合验收。

停止线：

1. 普通 tenant role 同时从 DB 和 policy CSV OR 放行时停止。
2. 保存 API 可被管理员调用，但业务请求仍按 CSV tenant policy 判定时停止。
3. 运行时出现 `current_role_slug`、`primary_role_slug`、`roles[0]` 参与普通 tenant allow/deny 时停止。

### 4.5 P4：用户授权组织范围与统一运行时 cutover

Owner：`DEV-PLAN-481/487/489/489A`

建议顺序：

1. [X] `DEV-PLAN-489`：已获得新增 DB 表手工确认，落地 `principal_role_assignments` 与 `principal_org_scope_bindings`。
2. [X] 用户授权保存 API 使用 replace-all 语义，同事务保存角色集合和组织范围。
3. [X] `DEV-PLAN-489A`：运行时授权门面按 489 `principal_role_assignments` 做 role capability union，不读 `roles[0]` 或 `iam.principals.role_slug`。
4. [X] `DEV-PLAN-487` 普通 tenant role cutover：能力授权只读 DB role SoT；policy CSV 仅保留 bootstrap/static/system surface。
5. [X] 保存时按 489A union 后的角色 capability 集合判断是否需要 `scope_dimension=organization`。
6. [X] 实现 principal scope provider；orgunit list/search/tree/detail/audit/write 统一消费 scope filter。
7. [X] 481 用户授权 UI 接入 489 API；组织范围保存失败会定位到组织范围页签。

并行规则：

- 用户授权 UI 骨架可与 489 服务端并行；可保存 UI 交付必须消费已落地的 scope provider 与 orgunit 裁剪。
- orgunit 不直接读取 IAM 表；IAM scope provider 输出 filter，orgunit service 强制消费。
- 489 的“完成”不能只看用户授权保存 API；必须证明 487 角色定义 DB SoT、489 `principal_role_assignments`、489A 多角色 union、scope provider 和 orgunit 裁剪在同一运行时路径中生效。

停止线：

1. 缺组织范围默认保存为全租户时停止。
2. 用户授权页刷新后丢失服务端事实时停止。
3. CubeBox 或普通 orgunit API 绕过 scope provider 读全量组织时停止。
4. 489 新增角色定义或 role capability 主表时停止，角色定义事实只归 487。

### 4.6 P5：CubeBox API-first 工具化

Owner：`DEV-PLAN-490`，`DEV-PLAN-486` 仅作停止警示

前置条件：

1. 484 单一覆盖事实聚合源已可枚举 route requirement，并提供 CubeBox API tool overlay 的同源枚举扩展点；490 overlay 已按该扩展点接入，后续不得由 490 之外的代码手写工具目录。
2. 485 API 授权目录基础投影已可从 484 覆盖事实输出现有 HTTP API 与 authz capability 绑定；`cubebox_callable` 等 CubeBox 增量标记已由 490 overlay 合成展示。
3. 当前用户 capability 与 orgunit scope 裁剪已可通过普通 HTTP API 验证。

建议顺序：

1. [X] 490 APIToolOverlay 只保存 CubeBox 增量字段；具体字段以 `DEV-PLAN-490` 为准。
2. [X] HTTP API 标识、route requirement 与 authz capability 绑定从 484/485 派生，不在 490 手写第二事实源。
3. [X] planner 只输出 `API_CALLS`；runner 只接受已派生可调用工具条目的 method/path 和 schema 参数。
4. [X] 删除 active runtime 业务 executor 执行入口，不保留 `READ_PLAN` / 裸 `ReadPlan` / `executor_key` 兼容窗口。

停止线：

1. 490 中新增 `/api/ai/**` 或 CubeBox 专用业务 API 时停止。
2. active runtime 同时接受 `READ_PLAN` 与 `API_CALLS` 时停止。
3. API runner 直接调用 store/helper 绕过现有 HTTP API 或等价 route/service authz path 时停止。
4. `APIToolOverlay` 保存或维护 route requirement 或 authz capability 绑定作为事实源时停止。

### 4.7 P6：授权项诊断后置治理

Owner：`DEV-PLAN-488`

前置条件：

1. 484 P1/P2 已完成并进入 lint。
2. 482A 功能授权项页面和 485 API 授权目录已消费同一覆盖事实源。
3. 角色保存 API 已在服务端拒绝未知、禁用、不可分配、非 tenant surface、无覆盖 key。

建议顺序：

1. [ ] 新增只读 `授权项诊断` 页面或受控诊断 endpoint。
2. [ ] 诊断原因只从 482 registry 与 484 覆盖事实派生。
3. [ ] 诊断页默认不进入角色定义候选项，也不允许前端参数把角色定义页切到诊断全集。

停止线：

1. 488 重新解析 route/policy/registry/CubeBox overlay 时停止。
2. 诊断页出现启用、修复、编辑 policy、刷新覆盖事实按钮时停止。
3. 诊断视图被用来替代 484 lint 阻断时停止。

## 5. 首批交付组合

### 5.1 可先交付的最小只读组合

首批只读治理面可以按以下组合验收。P1 基础已完成；P2 页面已落地且命中门禁已通过：

1. [X] 483 旧 key 删除完成。
2. [X] 482 registry/options 默认口径可用。
3. [X] 484 单一覆盖事实聚合和 lint 可用。
4. [X] 482A 功能授权项页面可展示当前可分配且有覆盖的授权项。
5. [X] 485 API 授权目录可从 API 角度展示 method/path 到授权项绑定。

该组合作为 P2 首批只读治理面验收时，不要求角色在线保存、不要求用户授权组织范围保存、不要求 CubeBox API-first 完整切换；当前这些后续闭环已分别由 P3/P4/P5 继续推进并完成首期范围。

### 5.2 可交付的最小运行时授权组合

角色与用户授权运行时必须作为闭环验收：

1. [X] 487 角色定义 DB SoT 与保存 API 可用。
2. [X] 489A 多角色 union 授权门面可用。
3. [X] 487 普通 tenant role cutover 完成，不再 CSV fallback。
4. [X] 489 用户授权角色集合与组织范围 SoT 可保存。
5. [X] 489 scope provider 与 orgunit 裁剪生效。
6. [X] A/B/descendant 组织范围服务端测试与 E2E 通过：全范围用户可见根范围，受限用户只可见指定节点及下级。

完成口径：

1. 487、489、489A 可以按职责拆 PR，但不能分别宣布“480 系列运行时授权已完成”。
2. 运行时闭环验收必须同时证明：角色能力来自 487 DB SoT，principal 角色集合来自 489 `principal_role_assignments`，能力判断按 489A 多角色 union，组织范围来自 489 scope provider，普通 API 和 CubeBox API-first 不再回读 policy CSV、`iam.principals.role_slug` 或 `roles[0]`。
3. 本轮已满足后端运行时闭环、481 用户可见保存交互、A/B 组织范围 E2E 与 490 API-first 首期硬切换；对外仍不能宣称 488 诊断视图已完成。

### 5.3 CubeBox 最小 API-first 组合

CubeBox 切换必须等待：

1. [X] 484/485 可追踪 API 到 authz capability。
2. [X] 490 overlay 引用的 HTTP API 全部存在并有 requirement。
3. [X] 当前用户 capability 与组织范围裁剪通过普通 HTTP API 生效。
4. [X] active runtime 不再接受 executor 业务计划。
5. [X] 评审修复已补齐 `all_org_units=true` 透传、API result 候选投影、search 多候选澄清与唯一 scope 可见候选返回路径；知识包示例 `depends_on` 不再跨 turn 引用。

## 6. 跨计划重复实现检查清单

实施或评审 480 系列 PR 时，必须主动检查：

1. [ ] 是否新增了第二套 authz capability key 格式、别名或旧 key normalize。
2. [ ] 是否从 policy CSV、导航配置、前端常量或知识包反推候选授权项。
3. [ ] 是否在 482A、485、488、490 中复制 route/registry/policy join。
4. [ ] 是否把 API path 当作授权项标识。
5. [ ] 是否让 `assignable=true` 的空壳 capability 进入普通候选项。
6. [ ] 是否让角色定义 payload 接收组织范围、字段策略、复制语义或 scope_required。
7. [ ] 是否让普通 tenant role DB SoT 与 policy CSV 同时放行。
8. [ ] 是否从 `iam.principals.role_slug`、`roles[0]` 或 current role 推导普通 tenant 授权。
9. [ ] 是否把组织范围放进 Casbin object/action、前端 query、prompt 或 CubeBox context。
10. [ ] 是否保留 CubeBox executor 与 HTTP API tool 双执行面。
11. [ ] 是否把 491/492 后续范围误标成 480A/P5 已完成。

命中任一项时，当前 PR 应停止合入，回到对应 owner 计划收敛。

## 7. 验收标准

1. [X] 480A 被 `AGENTS.md` Doc Map 收录，并位于 480 与各子计划之间。
2. [X] 480 系列实施 PR 可引用 480A 判断前置依赖和停止线。
3. [X] 482A/485/488/490 的实施顺序明确依赖 484 单一覆盖事实聚合源。
4. [X] 487/489/489A 的实施顺序明确要求同一运行时闭环验收，禁止任一子计划单独宣布运行时完成或暴露半成品在线入口。
5. [X] 488 明确后置，不作为 482A/485 首批闭环前置。
6. [X] 文档变更通过 `make check doc`。

## 8. 验证记录

- 2026-05-01 15:48 CST：创建 480A 路线图文档，待本轮文档同步后运行 `make check doc`。
- 2026-05-01 16:56 CST：启动并完成 P0 文档与入口收敛；确认 `AGENTS.md` Doc Map 已将 480A 放在 480 与各子计划之间，并为 480 系列子计划补充 480A 作为实施顺序入口。同步确认 `DEV-PLAN-486` 仍为 executor 路线停止警示，当前实现 owner 继续指向 `DEV-PLAN-490`。发现 AGENTS 中 410/411 链接对应文件当前缺失，按 P0 边界仅记录为后续 Doc Map 清理事项，不作为 480 当前实现前提。
- 2026-05-01 16:56 CST：`make check doc` 通过。
- 2026-05-01 18:22 CST：P1 已完成，`DEV-PLAN-483/482/484` 分别落地旧权限语言硬删除、静态 authz capability registry/options API、单一覆盖事实聚合源与 `make authz-lint` 门禁；已执行 `make authz-pack && make authz-test && make authz-lint`、Go fmt/vet/lint/test、前端相关测试、`make generate`、`make css`、文档/路由/root/no-legacy/chat-surface/no-scope/granularity/diff check 等门禁。
- 2026-05-01 18:58 CST：完成 480A 与 `DEV-PLAN-480/482/483/484` 文档登记收敛；P1 基础项在首批只读治理面组合中标记完成，482A/485 页面、487/489/489A 运行时闭环、488 诊断和 490 API-first 仍保持未完成。
- 2026-05-01 19:05 CST：本轮文档登记收敛后再次运行 `make check doc`，通过。
- 2026-05-01 19:54 CST：根据待提交评审补强 P1 停止线：覆盖事实必须由 allowlist route 与 route requirement 交集产生；482 普通 options API 拒绝诊断参数；`iam.authz:read` 首期 bootstrap policy 仅授予 `tenant-admin`。
- 2026-05-01 21:22 CST：P2 只读授权目录面落地并通过命中门禁；482A 功能授权项页面、关联 API 弹窗和 485 API 授权目录均复用 484 覆盖事实聚合源，未新增第二套 route/policy/registry join。
- 2026-05-01 22:03 CST：按 P2 停止线修正 485 普通目录口径，API 授权目录仅展示当前覆盖的 `enabled + assignable + tenant_api` 授权 API；浏览器复验 `GET /iam/api/authz/api-catalog` 返回 `status=200,count=46,badPaths=[],accessControls=["protected"],missingCapabilityKeyCount=0,nonAssignableCount=0`，未展示 health/static/internal no-requirement route、不可分配项或 executor key。
- 2026-05-01 23:10 CST：补齐 480A 与相关子计划文档状态登记；P2 只读治理面标记完成，P3/P4/P5/P6 仍保持未完成边界。
- 2026-05-02 CST：P3/P4 后端运行时闭环已实施。487 新增角色定义 DB SoT 与保存 API，489 新增 principal role assignment / org scope SoT 与保存 API，489A runtime 按 principal 多角色 union 授权；普通 tenant role 不再从 policy CSV、`iam.principals.role_slug` 或 `roles[0]` 放行；orgunit HTTP 与 CubeBox orgunit executor 均通过服务端 scope provider 裁剪。
- 2026-05-02 CST：新增 `make check authz-role-union` 专用反回流门禁并接入 `make preflight` 与 CI Gate-1；删除不可达的单 `RoleSlug` orgunit 判权死分支。已验证：`go test ./internal/server ./internal/routing ./pkg/authz`、`make test`、`make check lint`、`make authz-pack && make authz-test && make authz-lint`、`make check authz-role-union`、`make check routing`、`make check error-message`、`make check root-surface`、`make check doc`、`make check no-legacy && make check chat-surface-clean && make check no-scope-package && make check granularity && make check request-code`。
- 2026-05-02 CST：481 角色定义 / 用户授权 UI 保存交互已按 `designs/480.pen` 接入 487/489 API，并补齐 `dev481` A/B 组织范围 E2E：A 用户全范围，B 用户仅鲜花事业部及下级；覆盖普通 orgunit API 裁剪、范围外 detail fail-closed、CubeBox orgunit 查询与普通 API 结果一致。稳定门禁继续走 `make preflight` / `make e2e`；真实模型验收改为显式 `make e2e-live`。490 API-first hard cutover 与 488 诊断仍待后续。
- 2026-05-03 CST：完成 P5（DEV-PLAN-490 API-first 硬切换）首期实现：四个 orgunit 只读 API 进入 CubeBox API tool overlay，`cubebox_callable` 由 overlay 投影到 485 API 授权目录；query flow active runtime 切为 `cubebox-query-api-calls`，planner/decoder/runner 不再接受旧 `READ_PLAN`、裸 `ReadPlan` 或 `executor_key` 成功路径；新增 `make check cubebox-api-first` 并接入 `make preflight`。P6（DEV-PLAN-488 授权项诊断）成为后续治理顺序。
- 2026-05-03 CST：同步 480A 与 `DEV-PLAN-480/484/485/486/488/490` 当前进度状态，消除“490 未实施 / P5 待后续”漂移；本次文档状态同步后 `make check doc` 通过。
- 2026-05-03 CST：完成 P5 评审修复登记：普通 Web API 与 CubeBox runner 均保留并校验 `all_org_units=true`，语义仍为当前调用者可见范围内全部组织；`orgunit.search` 对多候选返回澄清候选，scope 过滤后唯一候选直接返回该可见候选；API observation 投影 `PresentedCandidates`，knowledge pack `depends_on` 仅允许同一 `API_CALLS` envelope 内引用。491 selector/UI 当时仍按后续计划推进，不并入 P5 完成口径。随后 2026-05-04 已完成 491 Phase A/B/C/D selector facade、组件与用户授权页、创建/详情上级组织选择入口接入，但该进展仍不并入 P5 完成口径。已验证 Go fmt/vet/lint/test、`make check cubebox-api-first`、authz 三件套、routing/doc/no-legacy/chat-surface/root-surface/no-scope/granularity/request-code/go-version/error-message/authz-role-union。
- 2026-05-04 CST：同步 491 Phase D 与 492 readiness 状态：主要组织选择入口已统一走 `OrgUnitTreeField`/`orgUnitSelector`，safe path 深层/跨分支测试已补；488 诊断、更广 491/492 联合 E2E、组织管理页浏览/编辑主树读取收敛和 492 SQL 级 scoped pagination 仍保持后续范围。
