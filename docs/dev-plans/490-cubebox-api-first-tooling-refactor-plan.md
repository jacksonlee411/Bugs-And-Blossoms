# DEV-PLAN-490：CubeBox 统一 API 工具化重构方案

**状态**: 规划中（2026-04-29 23:40 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：将 CubeBox 业务查询/操作工具面从当前 executor payload + knowledge pack 双轨，重构为“现有 HTTP API 是唯一业务工具契约”，由 485 API 授权目录聚合事实、490 最小 CubeBox 可调用标记、当前用户权限、API schema 和 observation adapter 共同约束模型调用；写入确认机制暂缓，不进入本轮实施。
- **关联模块/目录**：`modules/cubebox/**`、`modules/orgunit/presentation/cubebox/**`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_api.go`、`internal/server/orgunit_api.go`、`internal/server/authz_middleware.go`、`apps/web/src/api/**`、`pkg/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-460`、`DEV-PLAN-480`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-486`
- **用户入口/触点**：CubeBox 对话流式入口、API 授权目录页面的 `丘宝可调用` 列、服务端 API tool builder、现有业务 HTTP API、功能授权项的“关联 API”

### 0.1 Simple > Easy 三问

1. **边界**：490 只拥有 CubeBox 统一使用现有业务 HTTP API 的工具化重构和 `method/path -> cubebox_callable` 最小标记；480/484 继续拥有授权语义与覆盖门禁；485 继续拥有全量 HTTP API 正向目录页面；486 作为 executor 路线的对照方案，不作为本路线默认实施 owner。
2. **不变量**：CubeBox 不是独立授权主体；所有 API 调用都以当前用户、当前租户、当前 session 执行。API tool builder 不是授权来源，也不是第二套 API 目录；route/authz/capability 字段必须从 484 覆盖事实或 485 API 授权目录聚合派生。
3. **可解释**：任意 CubeBox 工具调用都能追溯到一个现有 HTTP API route、该 route 的 `object/action` requirement、当前用户授权决策、请求 schema、响应 schema和 observation 投影。

## 1. 背景

当前 CubeBox 已经通过 `ExecutionRegistry` 与 `orgunit.details/list/search/audit` 实际执行组织查询。但实现形态存在双轨问题：

1. 页面使用 HTTP API DTO，例如 `/org/api/org-units`、`/org/api/org-units/details`、`/org/api/org-units/search`、`/org/api/org-units/audit`。
2. CubeBox 使用 executor key、executor payload、knowledge pack `apis.md` 和 query flow 测试维护相似字段。
3. `has_children`、`full_name_path`、`path_org_codes`、`has_more` 等字段同时出现在页面 API、executor payload、知识包和测试里，但没有一个共同的 API schema 事实源。
4. executor 为了给模型提供 observation，实际在复刻页面 API 的一部分读契约，形成第二套实现和第二套字段契约风险。

因此，本计划提出另一条路线：**业务工具调用统一回到现有 HTTP API 契约**。CubeBox 不再维护业务 executor payload 作为事实源，而是通过 485 API 授权目录聚合事实筛选可调用 API，并把 API response 投影为模型 observation。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 冻结 CubeBox API-first 工具路线：业务查询/操作以现有 HTTP API 为唯一业务契约，不再新增平行业务 executor payload 契约。
2. [ ] 在 485 API 授权目录聚合事实上叠加最小 CubeBox 工具标记，列出可供 CubeBox 使用的现有 HTTP API；`method/path/object/action/capability_key` 不在 490 中重复维护。
3. [ ] CubeBox planner 输出 API call plan，而不是 executor key plan；后端只允许调用 485/490 派生出的可调用工具条目。
4. [ ] API 调用必须以当前用户身份经过现有 route/service authz、RLS、数据范围和字段裁剪。
5. [ ] 写入 API 首期不对 planner 开放；“提案 + 用户确认 + 当前用户提交”机制暂缓，后续另起计划冻结 UI 和契约。
6. [ ] 逐步删除或降级现有 orgunit executor 业务实现，避免 HTTP API 与 executor 双写读契约。

### 2.2 非目标

1. 不让模型自由拼任意 HTTP request；只能选择 485/490 派生工具条目中登记且当前用户可用的 API。
2. 不为 CubeBox 单独授权，不引入 CubeBox service account、独立角色或独立 policy。
3. 不新增 `/api/ai/**` 或 CubeBox 专用业务 HTTP API 作为第二套入口。
4. 不在本计划内重做全部业务 API schema 生成体系；首期可用代码内静态 CubeBox tool overlay 起步，但 overlay 只能引用 485/484 已知 HTTP API。
5. 不把 CubeBox tool overlay 当作 capability registry；授权事实仍来自 route requirement、PDP、RLS 和业务读路径。
6. 不在首期支持所有 HTTP route；首批仅覆盖现有 orgunit 只读查询闭环。

## 3. 核心原则

### 3.1 权限继承，不做 CubeBox 单独授权

CubeBox 的权限完全来自当前操作它的用户：

1. 当前用户没有某 API 的 route/service 权限，CubeBox 也不能调用。
2. 当前用户有读权限但数据范围受限，CubeBox 只能看到裁剪后的结果。
3. 首期不向 planner 暴露写 API；写入确认机制暂缓，不能作为本轮 UI 或 runtime 验收项。
4. 审计 actor/principal 仍是当前用户，CubeBox 只作为 `channel/source/tool`。

### 3.2 Tool Overlay 是工具标记，不是授权来源

API tool builder 的职责：

1. 从 485 API 授权目录聚合事实中筛选 `cubebox_callable=true` 的现有 HTTP API。
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

## 4. 目标架构

```text
HTTP POST /internal/cubebox/turns:stream
  -> route authz: cubebox.conversations:use
  -> build current-user API tools from 485 API access entries + 490 cubebox_callable overlay
  -> planner outputs API call plan
  -> API tool runner validates method/path/params against derived tool schema
  -> runner invokes existing HTTP API handler or equivalent in-process route adapter as current user
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

## 5. API Tool Overlay 契约

### 5.1 来源分层

490 不新增独立 API catalog 事实源。服务端 API tool entry 由两部分合成：

1. `APIAuthzCatalogEntry`：来自 485 API 授权目录聚合，字段由 routing、route requirement、capability registry 和 484 覆盖事实派生。
2. `APIToolOverlay`：490 只拥有 CubeBox 工具面增量字段。

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
| `request_schema` | query/body 参数 schema |
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

## 6. 与 486 的关系

`DEV-PLAN-486` 原本是 executor 路线的整改方案，目标是让当前 executor 成为一等授权入口。该路线已停止；490 是当前 PoR，目标是删除业务 executor 作为平行执行面。

当前取舍：

1. 486 中“per-step executor authorizer”不作为主线实施。
2. 486 中关于“当前用户权限、模块边界、命名混乱、第二套实现风险”的问题判断继续有效。
3. 490 用 485 API 授权目录聚合事实、490 tool overlay、当前用户 API authz 和 observation adapter 解决这些问题。
4. 486 仅保留为历史对照，不得与 490 同时实施成双运行面。

## 7. 实施切片

### 7.1 P0：契约冻结

1. [ ] 490 文档加入 AGENTS Doc Map。
2. [ ] 480/486/485 引用 490，明确当前业务工具契约 owner 已从 executor registry 转为 485 API 授权目录聚合事实 + 490 最小 tool overlay。
3. [ ] 冻结首批仅覆盖 orgunit 只读 API，不纳入写入 API。
4. [ ] 冻结“不自由拼 API”：planner 只能输出派生可调用工具条目中存在的 method/path。

### 7.2 P1：API Tool Overlay 最小实现

1. [ ] 在服务端新增静态 CubeBox tool overlay，首批引用四个 orgunit 只读 API。
2. [ ] overlay 只保存 `cubebox_callable`、`operation_id`、用途摘要、schema 引用和 observation projection；`object/action/capability_key` 从 485/484 聚合事实派生。
3. [ ] 为每个条目定义 request schema、参数默认值和 observation projection。
4. [ ] 增加 overlay 校验：method/path 必须存在、非 allowlist 受保护 route 必须有 requirement、capability 必须存在 registry；引用不存在的 API 时 fail-closed。

### 7.3 P2：API Call Plan 与 Runner

1. [ ] 新增 `APICallPlan` 解码与校验，替代现有业务 `ReadPlan` 执行面；迁移期兼容不得形成第二执行入口。
2. [ ] API runner 校验 method/path 完全匹配派生后的可调用工具条目。
3. [ ] API runner 校验 query/body 参数 schema，不允许未知参数。
4. [ ] API runner 以当前用户上下文调用现有 HTTP API 或等价 in-process route adapter。
5. [ ] API runner 不直接调用 store/helper。

### 7.4 P3：OrgUnit 查询链迁移

1. [ ] 将 `orgunit.list/details/search/audit` planner 示例改为 API call plan。
2. [ ] 删除或停用对应业务 executor 实现，不再构造独立 executor payload。
3. [ ] `has_children`、`full_name_path`、`path_org_codes`、`has_more` 等字段只以 HTTP API response schema 为事实源。
4. [ ] 保留现有多步规划、working results、candidate clarification 和 narration 行为。

### 7.5 P4：写入 API 确认机制（暂缓）

1. [ ] 暂缓，不进入本轮实施切片、测试或验收。
2. [ ] 后续若恢复，必须另起计划冻结 proposal schema、UI 展示、确认动作、幂等语义和 One Door 调用边界。
3. [ ] 暂缓期间，write/dangerous API 不对 planner 开放。

### 7.6 P5：文档与命名收敛

1. [ ] 将 `ReadAPICatalog` 更名或替换为表达 overlay/builder 语义的 `APIToolOverlay` / `APIToolBuilder`。
2. [ ] 将 `modules/orgunit/presentation/cubebox/apis.md` 改为从派生工具条目提取或仅保留语义说明。
3. [ ] 删除 executor payload 事实源语义，避免 `executor_key` 成为业务工具主键。
4. [ ] 更新功能授权项“关联 API”弹窗：首期只展示 HTTP API，不展示无意义 `类型=API` 列。

## 8. 测试与覆盖率

### 8.1 覆盖口径

本计划不新增覆盖率阈值；按仓库既有 Go、Authz、Routing、UI 门禁执行。新增测试围绕稳定职责，不新增补洞式测试文件。

### 8.2 必补测试

1. API tool overlay 校验：引用 route 存在、requirement 存在、capability 存在、schema 不允许未知参数。
2. API call plan 校验：未知 method/path、未知参数、缺必填参数、错误类型参数均 fail-closed。
3. API runner：以当前用户身份调用现有 API，权限不足返回 terminal error。
4. OrgUnit 迁移：list/details/search/audit 通过 API path 获取与当前页面 API 一致的字段。
5. 多步查询：search 后 details、list 后继续查 children、result_list 补 `full_name_path` 等行为保持。
6. 写入确认暂缓：本轮只验证 write/dangerous API 不进入 planner 可调用工具集。

### 8.3 验证命令

实施阶段按命中范围运行：

1. Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
2. Routing：`make check routing`
3. Authz：`make authz-pack && make authz-test && make authz-lint`
4. 文档：`make check doc`
5. UI / `.templ` / MUI assets 命中时：`make generate && make css`，并检查 `git status --short`
6. 关键用户闭环命中时：`make e2e`

## 9. 验收标准

1. [ ] CubeBox orgunit 查询不再依赖业务 executor payload 作为事实源。
2. [ ] planner 只能调用 485/490 派生可调用工具中的 method/path。
3. [ ] 四个 orgunit 只读能力通过现有 HTTP API 契约执行，并继承当前用户权限。
4. [ ] 页面和 CubeBox 对 `has_children`、`full_name_path`、`path_org_codes`、`has_more` 的依赖来自同一 API response schema。
5. [ ] 无权限用户无法借 CubeBox 调用对应 API；拒绝为 terminal error，不 fallback 到普通聊天。
6. [ ] 写入 API 暂不对 planner 开放；确认流程不进入本轮验收。
7. [ ] 不新增 `/api/ai/**` 或 CubeBox 专用业务 API。
8. [ ] 484/485 覆盖事实仍能追踪 API route 到 capability key。

## 10. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 把统一 API 误解为自由拼 HTTP | 模型生成未登记 path 或任意 query/body | runner 只接受派生可调用工具条目的 method/path 与 schema 参数 |
| tool overlay 变成第二授权源 | overlay 允许但用户无权限仍执行 | 所有调用仍走当前用户 route/service authz |
| tool overlay 变成第二 API 目录 | 490 手写 method/path/object/action/capability_key，与 485/484 漂移 | 490 只保存 tool overlay，route/authz/capability 字段从 485/484 派生 |
| 页面 API 被 AI 需求污染 | 为模型随意改页面 response | observation adapter 做投影，API schema 变更需同时评估页面与 CubeBox |
| `调用策略` 回流 | API 授权目录主表出现 `调用策略=只读`，重复 method/action 语义 | 主表只保留 `丘宝可调用`，写入确认暂缓到后续计划 |
| AI 专用 API 回流 | 新增 `/api/ai/orgunit-tree` 聚合口 | 停止实施，回到现有业务 API 或正式业务 API 设计 |
| 写入 API 提前开放 | 模型直接 POST 写 API | write/dangerous API 暂不进入 planner 可调用工具集；确认机制后续另起计划 |
| 保留 executor 与 API 双链路 | 同一能力 API 和 executor 同时可执行 | 迁移完成后停用业务 executor 入口，避免双运行面 |

## 11. 验证记录

- 2026-04-29 23:40 CST：创建方案文档，冻结 CubeBox API-first 工具化重构方向。待实施阶段按命中范围运行文档、Go、Routing、Authz、UI 与 E2E 门禁。
