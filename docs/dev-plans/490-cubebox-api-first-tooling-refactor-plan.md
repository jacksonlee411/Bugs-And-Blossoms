# DEV-PLAN-490：CubeBox 统一 API 工具化重构方案

**状态**: 规划中；484/485 与 487/489/489A 当前用户 capability + org scope 后端裁剪已落地，490 overlay/API_CALLS runtime 硬切换仍未实施（2026-05-02 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：将 CubeBox 业务查询/操作工具面从当前 executor payload + knowledge pack 双轨，硬切换为“现有 HTTP API 是唯一业务工具契约”，由 484 单一覆盖事实聚合源、485 API 授权目录投影、490 最小 CubeBox 可调用标记、当前用户权限、API schema 和 observation adapter 共同约束模型调用；本轮不保留 `ReadPlan` / `executor_key` 兼容执行面，写入确认机制暂缓，不进入本轮实施。
- **关联模块/目录**：`modules/cubebox/**`、`modules/orgunit/presentation/cubebox/**`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_api.go`、`internal/server/orgunit_api.go`、`internal/server/authz_middleware.go`、`apps/web/src/api/**`、`pkg/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-460`、`DEV-PLAN-480`、`DEV-PLAN-480A`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-486`
- **用户入口/触点**：CubeBox 对话流式入口、API 授权目录页面的 `丘宝可调用` 列、服务端 API tool builder、现有业务 HTTP API、功能授权项的“关联 API”

### 0.1 Simple > Easy 三问

1. **边界**：490 只拥有 CubeBox 统一使用现有业务 HTTP API 的工具化重构和 `method/path -> cubebox_callable` 最小标记；480/484 继续拥有授权语义与覆盖门禁；485 继续拥有当前覆盖 API 正向目录页面；486 作为 executor 路线的历史对照方案，不作为本路线实施 owner。
2. **不变量**：CubeBox 不是独立授权主体；所有 API 调用都以当前用户、当前租户、当前 session 执行，并且必须经过与普通 HTTP API 等价的 PEP/authz/RLS/数据范围路径。API tool builder 不是授权来源，也不是第二套 API 目录；route/authz/capability 字段必须从 484 单一覆盖事实聚合源派生，485 只是 API 视角投影；active runtime 不得同时接受 `READ_PLAN` 与 `API_CALLS` 两种业务执行计划。
3. **可解释**：任意 CubeBox 工具调用都能追溯到一个现有 HTTP API route、该 route 的 `object/action` requirement、当前用户授权决策、请求 schema、响应 schema和 observation 投影。

## 1. 背景

当前 CubeBox 已经通过 `ExecutionRegistry` 与 `orgunit.details/list/search/audit` 实际执行组织查询。但实现形态存在双轨问题：

1. 页面使用 HTTP API DTO，例如 `/org/api/org-units`、`/org/api/org-units/details`、`/org/api/org-units/search`、`/org/api/org-units/audit`。
2. CubeBox 使用 executor key、executor payload、knowledge pack `apis.md` 和 query flow 测试维护相似字段。
3. `has_children`、`full_name_path`、`path_org_codes`、`has_more` 等字段同时出现在页面 API、executor payload、知识包和测试里，但没有一个共同的 API schema 事实源。
4. executor 为了给模型提供 observation，实际在复刻页面 API 的一部分读契约，形成第二套实现和第二套字段契约风险。

因此，本计划提出另一条路线：**业务工具调用统一回到现有 HTTP API 契约**。CubeBox 不再维护业务 executor payload 作为事实源，而是通过 484 单一覆盖事实聚合源与 485 API 授权目录投影筛选可调用 API，并把 API response 投影为模型 observation。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 冻结 CubeBox API-first 工具路线：业务查询/操作以现有 HTTP API 为唯一业务契约，不再新增平行业务 executor payload 契约。
2. [ ] 在 484 单一覆盖事实聚合源经 485 API 授权目录投影后的 API 条目上叠加最小 CubeBox 工具标记，列出可供 CubeBox 使用的现有 HTTP API；`method/path/object/action/authz_capability_key` 不在 490 中重复维护。484 聚合源已预留 CubeBox API tool overlay 空集合扩展点，490 必须接入该扩展点。
3. [ ] CubeBox planner 输出 API call plan，而不是 executor key plan；后端只允许调用 485/490 派生出的可调用工具条目。
4. [ ] API 调用必须以当前用户身份经过现有 route/service authz、RLS、数据范围和字段裁剪。
5. [ ] 写入 API 首期不对 planner 开放；“提案 + 用户确认 + 当前用户提交”机制暂缓，后续另起计划冻结 UI 和契约。
6. [ ] 删除现有 orgunit executor 业务执行面，避免 HTTP API 与 executor 双读契约；迁移完成后 `executor_key` 不再是 planner、知识包、运行时 catalog 或用户可见目录中的业务工具主键。

### 2.2 非目标

1. 不让模型自由拼任意 HTTP request；只能选择 485/490 派生工具条目中登记且当前用户可用的 API。
2. 不为 CubeBox 单独授权，不引入 CubeBox service account、独立角色或独立 policy。
3. 不新增 `/api/ai/**` 或 CubeBox 专用业务 HTTP API 作为第二套入口。
4. 不在本计划内重做全部业务 API schema 生成体系；首期可用代码内静态 CubeBox tool overlay 起步，但 overlay 只能引用 484 单一覆盖事实聚合源和 485 API 授权目录投影中已知的 HTTP API。
5. 不把 CubeBox tool overlay 当作 capability registry；授权事实仍来自 route requirement、PDP、RLS 和业务读路径。
6. 不在首期支持所有 HTTP route；首批仅覆盖现有 orgunit 只读查询闭环。
7. 不保留 `READ_PLAN` / 裸 `ReadPlan` / `executor_key` 的运行时兼容窗口；迁移 PR 内可为编译和测试临时调整旧类型，但合入时 active runtime 必须只接受 `API_CALLS`。
8. 不引入通用 JSON Schema 平台、OpenAPI 生成链或 API 网关；首期 request schema 采用代码内最小结构，足够表达 GET/query 参数校验即可。

## 3. 核心原则

### 3.1 权限继承，不做 CubeBox 单独授权

CubeBox 的权限完全来自当前操作它的用户：

1. 当前用户没有某 API 的 route/service 权限，CubeBox 也不能调用。
2. 当前用户有读权限但数据范围受限，CubeBox 只能看到裁剪后的结果。
3. 首期不向 planner 暴露写 API；写入确认机制暂缓，不能作为本轮 UI 或 runtime 验收项。
4. 审计 actor/principal 仍是当前用户，CubeBox 只作为 `channel/source/tool`。

### 3.2 Tool Overlay 是工具标记，不是授权来源

API tool builder 的职责：

1. 从 484 单一覆盖事实聚合源经 485 API 授权目录投影后的 API 条目中筛选 `cubebox_callable=true` 的现有 HTTP API。
2. 告诉模型可用 API 的业务用途。
3. 提供请求参数 schema、默认值和参数约束。
4. 规定 response 如何投影为 observation。
5. 首期仅开放只读工具；写入/危险操作的确认机制暂缓，不能进入 485 主表列或本轮运行时契约。

API tool builder 不得：

1. 赋予额外权限。
2. 覆盖 route requirement。
3. 绕过业务 API 参数校验。
4. 让未登记 path 进入模型调用面。
5. 复制维护 `method/path/object/action/capability_key` 事实源。
6. 重新发明 `调用策略=只读/写入需确认` 一类分类语言；API 的读写语义来自现有 `method`、`action` 和 capability key，写入确认暂缓到后续计划。

当前 487/489/489A 已证明普通 HTTP API 能按 current principal capability union 与 org scope 裁剪；490 后续只继承该结果，不新增 CubeBox 专用授权或第二套范围判断。active runtime 仍待从 executor 路线硬切换到 API-first `API_CALLS`。

### 3.3 API 是唯一业务工具契约

统一关系：

```text
页面 UI       -> HTTP API -> shared business read/write path
CubeBox plan -> HTTP API -> shared business read/write path -> observation adapter
```

不再保留：

```text
页面 UI       -> HTTP API DTO
CubeBox plan -> executor payload DTO
```

### 3.4 Observation Adapter 只能投影，不承载业务规则

API response 面向页面，可能字段较多。CubeBox 仍需要 observation adapter，但它只能做：

1. 字段选择。
2. 大结果摘要。
3. 候选项结构化。
4. 内部字段过滤。
5. 模型上下文预算裁剪。

它不得：

1. 改写业务语义。
2. 重新做数据范围。
3. 重新计算字段权限。
4. 重新实现分页、搜索、排序、默认值。

### 3.5 硬切换，不做长期兼容过渡

490 的实施是执行契约切换，不是给旧 executor 链路外包一层 API 适配。允许在同一实施 PR 内为了编译顺序短暂出现新旧类型并存，但 PR 完成时必须满足：

1. planner system prompt 只要求 `API_CALLS` / `CLARIFY` / `DONE` / `NO_QUERY`，不再提示或接受 `READ_PLAN`。
2. `DecodePlannerOutcome` 不再接受裸 `ReadPlan`、`READ_PLAN` envelope 或 `steps[].executor_key`。
3. active runtime 不再调用 `ExecutionRegistry.ExecutePlan` 执行业务查询。
4. 知识包不再以 `apis.md` + `executor_key` 声明业务工具；若保留模块知识文件，只能引用 API tool overlay / API schema 的派生事实。
5. 测试不再把 `executor_key` 作为业务工具成功路径；历史 executor 测试要么删除，要么迁到明确归档/历史校验范围，不能参与 active runtime 验收。

### 3.6 旧执行面残留分级

490 实施前必须先对旧词汇命中做分级，避免反回流扫描不可执行。合入完成态按下表处理：

| 命中面 | 处理口径 |
| --- | --- |
| planner prompt、provider outcome decoder、query loop 正向路径中的 `READ_PLAN` / 裸 `ReadPlan` / `steps[].executor_key` | 必须删除或替换为 `API_CALLS` / `APICallPlan` / `method + path`；旧格式只能作为负向测试输入并返回 terminal error |
| `ExecutionRegistry.ExecutePlan`、orgunit 业务 executor 注册、`modules/cubebox/read_executor.go` 正向执行路径 | 必须删除 active runtime 职责；若保留通用 helper，必须改名到 API tool 语义，不得还能按 `executor_key` 执行业务 |
| `ReadAPICatalog`、knowledge pack `apis.md` 中的 `executor_key` 工具声明 | 必须改为 `APIToolOverlay` / `APIToolBuilder` 或删除；knowledge pack 不再是业务工具事实源 |
| active tests 的成功路径 fixture | 必须改为 `API_CALLS`；旧 `READ_PLAN` / `executor_key` 只能出现在负向反回流测试 |
| 用户输出泄露防线中的历史禁词 | 可以保留 `ReadPlan` / `executor_key` 作为“禁止外泄词”负向列表，但不得出现在 planner 指令、工具目录或成功路径断言中 |
| docs/archive、历史计划说明、490 迁移清单 | 可以保留历史词汇；反回流扫描必须把这些路径列为允许命中，而不是把它们当 active runtime 证据 |

## 4. 目标架构

```text
HTTP POST /internal/cubebox/turns:stream
  -> route authz: cubebox.conversations:use
  -> build current-user API tools from 484 coverage facts / 485 API catalog entries + 490 cubebox_callable overlay
  -> planner outputs API call plan
  -> API tool runner validates method/path/params against derived tool schema
  -> runner invokes existing HTTP API through the same-route PEP adapter as current user
  -> existing API route/service authz and business read/write path decide
  -> observation adapter projects API response
  -> planner continues or narrator answers
```

首期推荐 API call plan 结构：

```json
{
  "outcome": "API_CALLS",
  "calls": [
    {
      "id": "step-1",
      "method": "GET",
      "path": "/org/api/org-units",
      "params": {
        "as_of": "2026-04-29",
        "parent_org_code": "100000"
      },
      "depends_on": []
    }
  ]
}
```

为避免模型自由拼 path，`method + path` 必须完全匹配 485/490 派生出的可调用工具条目；参数必须通过 schema 校验。

### 4.1 Same-route PEP Adapter 边界

API runner 不得直接调用 store、repository、业务 helper、业务 handler 内部函数或旧 executor。`same-route PEP adapter` 不是“把当前 principal 塞进 context 后调用业务函数”的轻量包装，而是 CubeBox 调用现有 HTTP API 的唯一策略执行边界。首期只允许两种实现形态：

1. **同栈 HTTP 形态**：runner 构造受控的内部 HTTP request，进入与用户请求相同的 `withTenantAndSession -> withAuthz -> router -> handler` 链路；上下文、tenant、principal、session、request metadata 只能来自当前 turn 的已认证请求。
2. **显式 route PEP facade 形态**：若因测试或性能原因选择 in-process 调用，必须先抽出可测试的 route PEP facade，复用 `authzRequirementForRoute`、489A 多角色 union、487 DB role capability SoT、tenant/RLS 注入与普通 route responder 语义；业务 handler/helper/store 只能在该 facade 完成同等 PEP 决策后被调用。

两种形态都必须满足以下等价条件：

1. **同身份**：使用当前 request 中的 principal、tenant、session、roles/capability union 和 request metadata；不得使用 CubeBox service account、系统角色或新建授权主体。
2. **同 route requirement**：按 `method + path` 查同一份 route requirement；未知 path、无 requirement、非 `tenant_api` surface 或未标记 `cubebox_callable` 均 fail-closed。
3. **同授权裁决**：能力判断必须复用普通 HTTP API 当前路径的 489A 多角色 union 与 487 DB role capability SoT；不得从 planner、overlay、knowledge pack 或前端能力集合推断 allow/deny。
4. **同租户与 RLS**：进入业务读路径前必须具备普通 API 相同的显式事务、tenant 注入、RLS 与服务层数据范围裁剪；list/search 不扩大范围，details/audit 越界按普通 API 契约拒绝或隐藏。
5. **同 responder 语义**：adapter 捕获普通 API 的 status/error code，并按本计划错误映射转为 CubeBox observation 或 terminal error；不得吞掉 403/404/422 后 fallback 到普通聊天。
6. **同参数入口**：runner 只能把 schema 校验后的 query/body 参数传给该 API；不得额外补写业务参数、scope、org root、role 或 capability。

禁止形态：

1. runner 直接调用 `handleOrgUnits*`、`listOrgUnitListPage`、`searchNodeByVisibility`、`ensurePrincipalOrgScopeAllows`、store 或 repository。
2. runner 自行复制 `withAuthz` 的判断分支，或只检查 `method/path` 与 `cubebox_callable` 后跳过 route requirement。
3. runner 在 adapter 外补写 scope、role、capability、org root、父级范围或分页业务默认值。
4. 用测试 helper、空 registry、feature flag 或“当前只注册 API 工具”保留可恢复的 executor 执行路径。

### 4.2 APICallPlan 最小解码契约

首期 planner 输出只接受 JSON envelope，不接受裸计划：

1. `outcome` 只能是 `API_CALLS`、`CLARIFY`、`DONE`、`NO_QUERY`。
2. `API_CALLS.calls` 必须是 1..N 的线性步骤；`calls[0].depends_on=[]`，后续步骤只能依赖紧邻前一步。
3. `method` 首期只允许 `GET`；`path` 必须与派生可调用工具条目的 canonical path 完全一致。
4. `params` 只能包含 request schema 声明的 query 参数；未知字段、错误类型、缺必填、非法日期或非法枚举均 fail-closed。
5. `body` 首期不允许出现；写入 API 与 body schema 后续另起计划。
6. envelope 与 call 对象都禁止未知字段；旧 `plan`、`steps`、`executor_key`、`intent` 等 ReadPlan 字段出现时返回 terminal error，不做转换。

## 5. API Tool Overlay 契约

### 5.1 来源分层

490 不新增独立 API catalog 事实源。服务端 API tool entry 由两部分合成：

1. `APIAuthzCatalogEntry`：来自 484 单一覆盖事实聚合源经 485 API 授权目录投影后的 API 条目，字段由 routing、route requirement、capability registry、policy 覆盖事实和 CubeBox overlay 校验派生。
2. `APIToolOverlay`：490 只拥有 CubeBox 工具面增量字段。

为防止 490 runtime overlay 与 484 lint overlay 分叉，overlay 必须采用**同源定义、双投影**：

1. `APIToolOverlayDefinition` 是 490 唯一可维护的静态定义源，保存完整运行时增量字段：`method/path`、`cubebox_callable`、`operation_id`、用途摘要、request schema、response schema 引用与 observation projection。
2. `CoverageOverlay` 从同一个 `APIToolOverlayDefinition` 派生为 484 覆盖事实需要的最小结构：`method/path/cubebox_callable/surface`，并接入 `ListAuthzToolOverlayCoverage()`；不得在 484 或 lint 侧另写一份 path 清单。
3. `RuntimeToolOverlay` 从同一个 `APIToolOverlayDefinition` 派生，再与 484/485 的 `APIAuthzCatalogEntry` join 成 planner 可见工具；不得绕过 484/485 直接用 overlay definition 生成可调用 path。
4. 同一个 `method/path` 在 overlay definition 中只能出现一次；重复、引用不存在 route、引用非 `tenant_api` route、引用无 requirement route、引用未注册 capability 或引用 write/dangerous API 都必须 fail-closed。
5. 485 主表只消费派生后的 `cubebox_callable`；`operation_id`、schema 与 projection 不进入 485 主表列，也不成为 API 授权目录的新事实源。

`APIAuthzCatalogEntry` 字段：

| 字段 | 含义 |
| --- | --- |
| `method` | HTTP method |
| `path` | HTTP route path |
| `owner_module` | 归属模块 |
| `access_control` | 受保护 / allowlist / public 等 |
| `authz_object` / `authz_action` | 来自 route requirement |
| `capability_key` | `object:action` 派生 |

`APIToolOverlay` 首期字段：

| 字段 | 含义 |
| --- | --- |
| `method` / `path` | 引用 485 已存在 API；不得引用不存在的 HTTP API |
| `cubebox_callable` | 是否进入 CubeBox 可调用 HTTP API 工具面；同时作为 485 主表唯一 CubeBox 相关列 |
| `operation_id` | 稳定操作 ID，例如 `orgunit.list`，只作为 catalog 内部引用，不作为第二业务实现 |
| `intent_summary` | 给模型的简短用途 |
| `request_schema` | 最小 query/body 参数 schema；首期只允许 GET/query |
| `response_schema_ref` | 响应 schema 或 DTO 引用 |
| `observation_projection` | response 到 observation 的投影规则 |

读写语义不得以 `调用策略` 主表列重复表达。首期只开放只读 API；写入确认契约暂缓，不作为本轮 tool overlay 字段、UI 列或运行时验收项。

### 5.2 首批 OrgUnit API

首批只纳入只读 API：

| API | operation_id | kind | capability |
| --- | --- | --- | --- |
| `GET /org/api/org-units` | `orgunit.list` | `read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/details` | `orgunit.details` | `read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/search` | `orgunit.search` | `read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/audit` | `orgunit.audit` | `read` | `orgunit.orgunits:read` |

首期不纳入 orgunit 写 API。写入 API 开放与确认流程暂缓，后续另起计划处理。

### 5.3 当前用户可用性过滤

对 planner 暴露可调用工具时，服务端应按当前用户权限过滤或标记：

1. 无 `object/action` 权限的 API 默认不进入 planner 可调用工具集。
2. 若为了解释“为什么不能做”保留不可用 API，只能展示为不可调用项，不能生成 call。
3. 数据范围不在 catalog 层提前展开；仍由 API 调用后的业务读路径裁剪。

### 5.4 与 API 授权目录页面的关系

485 `API 授权目录` 页面主表只增加 `丘宝可调用` 一列，对应服务端 `cubebox_callable` 字段。不得新增 `调用策略`、`只读/写入策略` 或其他重复读写语义的主表列：

1. 是否只读已由 `方法`、`操作`、`授权项标识` 明确表达。
2. CubeBox 是否可调用由 `丘宝可调用` 表达。
3. 写入确认暂缓；API 授权目录主表不得预留确认策略列。

### 5.5 Request Schema 最小结构

首期不引入通用 JSON Schema 或 OpenAPI 生成链。`request_schema` 使用代码内最小结构，服务 API call plan 校验与 prompt/tool 描述：

| 字段 | 含义 |
| --- | --- |
| `param_source` | 首期固定为 `query` |
| `body_allowed` | 首期固定为 `false` |
| `allow_unknown_params` | 首期固定为 `false` |
| `parameters[].name` | query 参数名，必须与现有 HTTP API 入参一致 |
| `parameters[].type` | `string` / `bool` / `int` / `date` |
| `parameters[].required` | 是否必填 |
| `parameters[].default` | runner 可在调用前填入的执行默认值，例如分页 |
| `parameters[].enum` | 可选枚举值 |
| `parameters[].min/max` | 可选数值边界，主要用于 `page` / `page_size` |

首批规则：

1. `date` 只能是 `YYYY-MM-DD`，并保持 Valid Time day 粒度。
2. `bool` 与 `int` 在 planner JSON 中必须是对应 JSON 类型；runner 编码 query string 时再转成现有 HTTP API 接收格式。
3. `page` / `page_size` 属于执行控制参数，不得作为业务缺参追问；缺省时由 schema default 填入。
4. `all_org_units=true` 只能表示“当前用户授权范围内的全部组织”，不得扩大 489 scope provider 输出。
5. observation projection 只能消费 API response；不得因 schema 缺字段而回退到旧 executor payload。

首期 OrgUnit list 存在历史分页口径差异，必须显式冻结：

1. planner-facing schema 使用用户可见 1 基页码：`page=1` 表示第一页，默认 `page=1`、`page_size=100`。
2. HTTP API `/org/api/org-units` 现有 query 使用 0 基 `page` 与 `pageSize`/`size` 口径时，runner 必须在 adapter 参数编码层完成转换；转换前后都要保留 schema 校验，不得把转换逻辑写进 observation adapter 或业务 handler。
3. `request_schema.parameters[].http_name` 可用于表达 planner 参数名与 HTTP query 参数名不同的情况；未设置时默认等于 `name`。
4. 分页转换只属于执行控制参数转换，不得改变业务过滤语义、scope provider 输入、排序、搜索或 `all_org_units` 语义。

### 5.6 Runner Error Contract

API runner 对 planner 和用户只暴露受控结果，不暴露内部 API raw error。首期映射如下：

| 来源 | 处理 |
| --- | --- |
| 未知 `method/path`、未标记 `cubebox_callable`、write/dangerous API、旧 `READ_PLAN` / `executor_key` 字段 | terminal error；不重试、不转换、不 fallback 到普通聊天 |
| request schema 错误：未知参数、缺必填、类型错误、非法日期、非法枚举、body 出现 | terminal error；提示用户补充或改写查询，不把错误参数继续传给 API |
| API 返回 `401/403` 或 route/service authz 拒绝 | terminal authorization error；不得隐藏为普通聊天回答，也不得换 executor/API 重试 |
| API 返回范围外 `404` 或业务契约选择隐藏资源存在性 | terminal not-found/forbidden 语义；不得继续尝试绕路查询父级或全量列表 |
| API 返回 `400/422` 业务校验错误 | terminal validation error；后续若要把特定错误转为 `CLARIFY`，必须先在 490 或后续计划中列明错误码白名单 |
| API 返回 `5xx`、timeout、adapter 内部错误 | terminal system error；不得把部分失败结果交给 narrator 编造成成功回答 |
| API 成功但列表为空 | 成功 observation，narrator 可回答“未查到匹配结果” |

多步计划中任一步 terminal 后，当前 turn 立即停止执行后续 API call；不得把后续步骤当作 fallback。

## 6. 与 486 的关系

`DEV-PLAN-486` 原本是 executor 路线的整改方案，目标是让当前 executor 成为一等授权入口。该路线已停止；490 是当前 PoR，目标是删除业务 executor 作为平行执行面。

当前取舍：

1. 486 中“per-step executor authorizer”不作为主线实施。
2. 486 中关于“当前用户权限、模块边界、命名混乱、第二套实现风险”的问题判断继续有效。
3. 490 用 484 单一覆盖事实聚合源、485 API 授权目录投影、490 tool overlay、当前用户 API authz 和 observation adapter 解决这些问题。
4. 486 仅保留为历史对照，不得与 490 同时实施成双运行面。

## 7. 实施切片

### 7.1 P0：契约冻结

1. [ ] 490 文档加入 AGENTS Doc Map。
2. [ ] 480/486/485 引用 490，明确当前业务工具契约 owner 已从 executor registry 转为 484 单一覆盖事实聚合源 + 485 API 授权目录投影 + 490 最小 tool overlay。
3. [ ] 冻结首批仅覆盖 orgunit 只读 API，不纳入写入 API。
4. [ ] 冻结“不自由拼 API”：planner 只能输出派生可调用工具条目中存在的 method/path。
5. [ ] 冻结硬切换口径：本轮完成时 active runtime 不得继续接受 `READ_PLAN`、裸 `ReadPlan` 或 `executor_key` 业务执行计划。
6. [ ] 冻结 same-route PEP adapter、最小 request schema、runner error contract 与旧执行面残留分级。

### 7.2 P1：API Tool Overlay 最小实现

1. [ ] 在服务端新增静态 CubeBox tool overlay，首批引用四个 orgunit 只读 API。
2. [ ] overlay 必须采用同源定义、双投影：同一份 `APIToolOverlayDefinition` 同时派生 484 `CoverageOverlay` 与 490 `RuntimeToolOverlay`；不得在 lint、485 页面和 runner 中各自维护 path 清单。
3. [ ] overlay 只保存 `cubebox_callable`、`operation_id`、用途摘要、schema 引用和 observation projection；`object/action/authz_capability_key` 从 484 单一覆盖事实聚合源派生。
4. [ ] 为每个条目定义最小 request schema、参数默认值、planner-facing 到 HTTP query 的参数名/分页转换和 observation projection；首期只允许 GET/query、`body_allowed=false`、`allow_unknown_params=false`。
5. [ ] 增加 overlay 校验：method/path 必须存在、非 allowlist 受保护 route 必须有 requirement、capability 必须存在 registry；重复定义、引用不存在 API、引用非 `tenant_api` route 或 write/dangerous API 时 fail-closed。

### 7.3 P2：API Call Plan 与 Runner

1. [ ] 新增 `APICallPlan` 解码与校验，并删除 active runtime 中的业务 `ReadPlan` 解码/执行入口；不得以“双解码”“兼容裸 ReadPlan”“ReadPlan 转 APICallPlan”等方式形成第二业务执行面。
2. [ ] API runner 校验 method/path 完全匹配派生后的可调用工具条目。
3. [ ] API runner 校验 query/body 参数 schema，不允许未知参数。
4. [ ] API runner 通过 same-route PEP adapter 以当前用户上下文调用现有 HTTP API，并证明经过同一 route requirement、489A union、RLS/tenant 注入和服务层数据范围裁剪；adapter 只能采用同栈 HTTP 形态或显式 route PEP facade 形态。
5. [ ] API runner 不直接调用业务 handler 内部函数、store/helper，不自行补 scope、role、capability 或业务默认范围。
6. [ ] 实现 runner error contract：旧计划、未知 path、schema 错误、403/404/422/5xx 均按冻结规则 terminal 或 observation，不 fallback。
7. [ ] 更新 planner prompt、outcome decoder、query loop 和工作结果结构，使多步查询只围绕 API call result 继续推进。

### 7.4 P3：OrgUnit 查询链迁移

1. [ ] 将 `orgunit.list/details/search/audit` planner 示例改为 API call plan。
2. [ ] 删除对应业务 executor 注册与执行实现，不再构造独立 executor payload；不得仅靠 feature flag、空 registry 或“暂不注册”长期保留 executor 可恢复路径。
3. [ ] `has_children`、`full_name_path`、`path_org_codes`、`has_more` 等字段只以 HTTP API response schema 为事实源。
4. [ ] 保留现有多步规划、working results、candidate clarification 和 narration 行为。
5. [ ] 删除或改造 `internal/server/cubebox_orgunit_executors.go`、`modules/cubebox/read_executor.go`、`modules/cubebox/read_api_catalog.go` 的 active runtime 职责；若有通用类型仍被其他非业务路径使用，必须重命名到 API tool 语义，不能继续暴露 executor 契约。

### 7.5 P4：写入 API 确认机制（暂缓）

1. [ ] 暂缓，不进入本轮实施切片、测试或验收。
2. [ ] 后续若恢复，必须另起计划冻结 proposal schema、UI 展示、确认动作、幂等语义和 One Door 调用边界。
3. [ ] 暂缓期间，write/dangerous API 不对 planner 开放。

### 7.6 P5：文档与命名收敛

1. [ ] 删除 `ReadAPICatalog` 事实源语义，替换为表达 overlay/builder 语义的 `APIToolOverlay` / `APIToolBuilder`。
2. [ ] 将 `modules/orgunit/presentation/cubebox/apis.md` 改为 API tool overlay / API schema 派生说明，或直接删除该文件并同步 `LoadKnowledgePack` 校验；不得继续要求 `executor_key`。
3. [ ] 删除 executor payload 事实源语义，避免 `executor_key` 成为业务工具主键；active runtime、prompt、知识包和测试成功路径都不得再依赖 `executor_key`。
4. [ ] 更新功能授权项“关联 API”弹窗：首期只展示 HTTP API，不展示无意义 `类型=API` 列。
5. [ ] 增加硬门禁 `make check cubebox-api-first`，按 3.6 残留分级扫描 active runtime、prompt、知识包和 active tests；`READ_PLAN`、裸 `ReadPlan`、`ReadAPICatalog`、`ExecutionRegistry.ExecutePlan` 或 `executor_key` 不得作为业务工具契约保留，允许命中必须是归档文档、历史说明、490 迁移清单、负向测试或用户输出泄露防线。
6. [ ] `make preflight` 或至少 CI Gate-1 必须接入 `make check cubebox-api-first`；未接入前，490 实施 PR 不得宣称反回流门禁完成。

## 8. 测试与覆盖率

### 8.1 覆盖口径

本计划不新增覆盖率阈值；按仓库既有 Go、Authz、Routing、UI 门禁执行。新增测试围绕稳定职责，不新增补洞式测试文件。

### 8.2 必补测试

1. API tool overlay 校验：引用 route 存在、requirement 存在、capability 存在、schema 不允许未知参数。
2. API call plan 校验：未知 method/path、未知参数、缺必填参数、错误类型参数均 fail-closed。
3. API runner：通过 same-route PEP adapter 以当前用户身份调用现有 API，权限不足返回 terminal error，并证明没有绕过同一 route requirement、489A union、RLS/tenant 注入或 scope provider；直接调用业务 handler/store/helper 的测试实现不得作为通过证据。
4. OrgUnit 迁移：list/details/search/audit 通过 API path 获取与当前页面 API 一致的字段。
5. 多步查询：search 后 details、list 后继续查 children、result_list 补 `full_name_path` 等行为保持。
6. 写入确认暂缓：本轮只验证 write/dangerous API 不进入 planner 可调用工具集。
7. 反回流测试：planner 输出 `READ_PLAN`、裸 `ReadPlan`、`steps[].executor_key`、未知 method/path 或旧 executor payload 时均 fail-closed，不 fallback 到旧执行链。
8. 错误映射测试：schema 错误、403、范围外 404、422、5xx/timeout 和空列表分别命中 runner error contract。
9. 分页语义测试：planner-facing `page=1/page_size=100` 被编码为普通 `/org/api/org-units` 接受的 HTTP query，并与页面第一页返回一致；不得出现 0/1 基偏移。
10. Overlay 同源测试：484 `ToolOverlays` 与 runtime tool builder 来自同一 `APIToolOverlayDefinition`；故意新增只在一侧出现的 path 必须触发测试或 lint 失败。

### 8.3 验证命令

实施阶段按命中范围运行：

1. Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
2. Routing：`make check routing`
3. Authz：`make authz-pack && make authz-test && make authz-lint`
4. 文档：`make check doc`
5. UI / `.templ` / MUI assets 命中时：`make generate && make css`，并检查 `git status --short`
6. 关键用户闭环命中时：`make e2e`
7. 反回流门禁：`make check cubebox-api-first`

## 9. 验收标准

1. [ ] CubeBox orgunit 查询不再依赖业务 executor payload 作为事实源。
2. [ ] planner 只能输出 `API_CALLS`，并且只能调用 485/490 派生可调用工具中的 method/path。
3. [ ] 四个 orgunit 只读能力通过现有 HTTP API 契约执行，并继承当前用户权限。
4. [ ] 页面和 CubeBox 对 `has_children`、`full_name_path`、`path_org_codes`、`has_more` 的依赖来自同一 API response schema。
5. [ ] 无权限用户无法借 CubeBox 调用对应 API；拒绝为 terminal error，不 fallback 到普通聊天。
6. [ ] 写入 API 暂不对 planner 开放；确认流程不进入本轮验收。
7. [ ] 不新增 `/api/ai/**` 或 CubeBox 专用业务 API。
8. [ ] 484 单一覆盖事实聚合源与 485 API 授权目录投影仍能追踪 API route 到 authz capability key。
9. [ ] active runtime 不再接受 `READ_PLAN`、裸 `ReadPlan` 或 `steps[].executor_key`；旧格式输入返回 terminal error。
10. [ ] orgunit 业务 executor 不再作为可执行入口存在；同一能力不能同时通过 HTTP API tool 和 executor tool 执行。
11. [ ] 知识包、prompt 和 active tests 不再把 `executor_key`、`ReadAPICatalog` 或 executor payload 作为业务工具契约。
12. [ ] API runner 的 same-route PEP adapter、request schema 和错误映射均有测试覆盖；无法证明同路径授权/RLS/scope 的实现不得合入。
13. [ ] `make check cubebox-api-first` 已接入并能阻断 active runtime、prompt、知识包和 active tests 中旧执行面作为成功路径回流。
14. [ ] `APIToolOverlayDefinition` 是唯一 overlay 定义源；484 coverage overlay、485 `cubebox_callable` 和 runtime tool builder 均由它派生，未出现第二 path 清单。
15. [ ] OrgUnit list 的 planner-facing 分页参数到 HTTP query 的转换有测试覆盖，第一页语义与普通页面 API 一致。

## 10. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 把统一 API 误解为自由拼 HTTP | 模型生成未登记 path 或任意 query/body | runner 只接受派生可调用工具条目的 method/path 与 schema 参数 |
| tool overlay 变成第二授权源 | overlay 允许但用户无权限仍执行 | 所有调用仍走当前用户 route/service authz |
| tool overlay 变成第二 API 目录 | 490 手写 method/path/object/action/authz_capability_key，与 484/485 漂移 | 490 只保存 tool overlay，route/authz/capability 字段从 484 单一覆盖事实聚合源派生，485 只是 API 视角投影 |
| 页面 API 被 AI 需求污染 | 为模型随意改页面 response | observation adapter 做投影，API schema 变更需同时评估页面与 CubeBox |
| `调用策略` 回流 | API 授权目录主表出现 `调用策略=只读`，重复 method/action 语义 | 主表只保留 `丘宝可调用`，写入确认暂缓到后续计划 |
| AI 专用 API 回流 | 新增 `/api/ai/orgunit-tree` 聚合口 | 停止实施，回到现有业务 API 或正式业务 API 设计 |
| 写入 API 提前开放 | 模型直接 POST 写 API | write/dangerous API 暂不进入 planner 可调用工具集；确认机制后续另起计划 |
| 保留 executor 与 API 双链路 | 同一能力 API 和 executor 同时可执行 | 不允许合入；必须删除业务 executor 执行入口，active runtime 只能保留 API tool |
| 长期兼容 `READ_PLAN` | outcome decoder 同时接受 `READ_PLAN` 和 `API_CALLS`，或把旧 plan 转成 API call | 不允许合入；旧格式只能返回 terminal error |
| 只停用不删除旧 executor | 通过 feature flag、空 registry 或测试 helper 仍能恢复业务 executor | 停止实施，删除 active runtime 可恢复路径后再继续 |
| in-process adapter 绕过普通 API PEP | runner 直接调用 handler 内部 helper、store 或补写 scope | 停止实施；必须补 same-route PEP adapter 或改为真实 HTTP route 调用 |
| request schema 过重或不明 | 为首期引入 OpenAPI/JSON Schema 平台，或 schema 无法判断未知参数 | 首期只用最小代码内 schema；未知参数与 body 默认拒绝 |
| API 错误被普通聊天吞掉 | 403/404/422/5xx 后 narrator 编造成成功回答 | runner error contract 必须 terminal 或受控 observation，不允许 fallback |
| overlay 同源破裂 | 484 lint overlay、485 页面标记、runner 可调用工具各自维护 path | 停止实施；回到单一 `APIToolOverlayDefinition` 并由同一 helper 派生 coverage/runtime 投影 |
| 反回流只靠人工扫描 | PR 记录了 `rg` 结果但没有 `make check` 门禁 | 不允许宣称完成；必须补 `make check cubebox-api-first` 并接入 CI/preflight |
| 分页语义偏移 | planner `page=1` 调用普通 API 时落到第二页或返回不一致 | 停止实施；补 planner-facing 到 HTTP query 的转换测试后再继续 |

## 11. 验证记录

- 2026-04-29 23:40 CST：创建方案文档，冻结 CubeBox API-first 工具化重构方向。待实施阶段按命中范围运行文档、Go、Routing、Authz、UI 与 E2E 门禁。
- 2026-05-01 18:58 CST：登记前置状态：484 覆盖事实聚合已预留 CubeBox API tool overlay 空集合扩展点，490 后续必须接入同一聚合源；485 API 授权目录、490 overlay 与 API-first active runtime 仍未实施。
- 2026-05-01 23:10 CST：补齐文档状态登记；485 API 授权目录已落地并按当前覆盖普通授权 API 口径复验，490 tool overlay、planner/runner API-first runtime 与 executor 删除仍未实施。
- 2026-05-02 CST：登记运行时授权前置状态；487/489/489A 已使普通 HTTP API 按 current principal capability union 与 org scope 裁剪，CubeBox orgunit executor 当前也复用同一 scope provider。490 overlay、planner/runner `API_CALLS` hard cutover 与业务 executor 删除仍未实施。
- 2026-05-02 CST：随 481/489 A/B 组织范围 E2E 复验，当前 CubeBox orgunit executor 路径与普通 orgunit API 返回范围一致；该证据只证明 scope provider 已复用，不代表 490 API-first overlay、`API_CALLS` runtime hard cutover 或 executor 删除已完成。
- 2026-05-03 CST：根据 490 方案评审补强实施契约：新增旧执行面残留分级、same-route PEP adapter 边界、APICallPlan 最小解码契约、最小 request schema 与 runner error contract；这些修订用于约束后续实现，不代表 overlay/API_CALLS runtime 已实施。
- 2026-05-03 CST：根据架构一致性收敛评审继续修正方案：将 same-route PEP adapter 收敛为同栈 HTTP 或显式 route PEP facade 两种可合入形态；冻结 `APIToolOverlayDefinition` 同源双投影；把旧执行面扫描提升为 `make check cubebox-api-first` 硬门禁；补充 OrgUnit list 1 基 planner-facing 分页到普通 HTTP API query 的转换契约。此为方案修订，仍不代表 490 runtime 已实施。
