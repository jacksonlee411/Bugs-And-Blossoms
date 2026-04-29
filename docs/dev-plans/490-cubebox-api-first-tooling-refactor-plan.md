# DEV-PLAN-490：CubeBox 统一 API 工具化重构方案

**状态**: 规划中（2026-04-29 23:40 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：将 CubeBox 业务查询/操作工具面从当前 executor payload + knowledge pack 双轨，重构为“现有 HTTP API 是唯一业务工具契约”，由服务端 API catalog、当前用户权限、API schema、写入确认和 observation adapter 共同约束模型调用。
- **关联模块/目录**：`modules/cubebox/**`、`modules/orgunit/presentation/cubebox/**`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_api.go`、`internal/server/orgunit_api.go`、`internal/server/authz_middleware.go`、`apps/web/src/api/**`、`pkg/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-015`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-460`、`DEV-PLAN-480`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-486`
- **用户入口/触点**：CubeBox 对话流式入口、服务端 API catalog、现有业务 HTTP API、功能授权项的“关联的访问入口”、后续 API 访问入口目录

### 0.1 Simple > Easy 三问

1. **边界**：490 只拥有 CubeBox 统一使用现有业务 HTTP API 的工具化重构；480/484 继续拥有授权语义与覆盖门禁；485 继续拥有全量 HTTP API 正向目录页面；486 作为 executor 路线的对照方案，不作为本路线默认实施 owner。
2. **不变量**：CubeBox 不是独立授权主体；所有 API 调用都以当前用户、当前租户、当前 session 执行。API catalog 不是授权来源，而是模型可调用 API 的 schema、分类、确认策略和 observation 规则。
3. **可解释**：任意 CubeBox 工具调用都能追溯到一个现有 HTTP API route、该 route 的 `object/action` requirement、当前用户授权决策、请求 schema、响应 schema和 observation 投影。

## 1. 背景

当前 CubeBox 已经通过 `ExecutionRegistry` 与 `orgunit.details/list/search/audit` 实际执行组织查询。但实现形态存在双轨问题：

1. 页面使用 HTTP API DTO，例如 `/org/api/org-units`、`/org/api/org-units/details`、`/org/api/org-units/search`、`/org/api/org-units/audit`。
2. CubeBox 使用 executor key、executor payload、knowledge pack `apis.md` 和 query flow 测试维护相似字段。
3. `has_children`、`full_name_path`、`path_org_codes`、`has_more` 等字段同时出现在页面 API、executor payload、知识包和测试里，但没有一个共同的 API schema 事实源。
4. executor 为了给模型提供 observation，实际在复刻页面 API 的一部分读契约，形成第二套实现和第二套字段契约风险。

因此，本计划提出另一条路线：**业务工具调用统一回到现有 HTTP API 契约**。CubeBox 不再维护业务 executor payload 作为事实源，而是通过服务端受控 API catalog 选择和调用现有 API，并把 API response 投影为模型 observation。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 冻结 CubeBox API-first 工具路线：业务查询/操作以现有 HTTP API 为唯一业务契约，不再新增平行业务 executor payload 契约。
2. [ ] 新增服务端 API catalog，列出可供 CubeBox 使用的 API、method/path、请求 schema、响应摘要、读写分类、确认策略、`object/action` requirement 和 observation 投影规则。
3. [ ] CubeBox planner 输出 API call plan，而不是 executor key plan；后端只允许调用 catalog 中登记的 API。
4. [ ] API 调用必须以当前用户身份经过现有 route/service authz、RLS、数据范围和字段裁剪。
5. [ ] 写入 API 只能进入“提案 + 用户确认 + 当前用户提交”流程，不允许模型直接提交正式写入。
6. [ ] 逐步删除或降级现有 orgunit executor 业务实现，避免 HTTP API 与 executor 双写读契约。

### 2.2 非目标

1. 不让模型自由拼任意 HTTP request；只能选择 catalog 中登记且当前用户可用的 API。
2. 不为 CubeBox 单独授权，不引入 CubeBox service account、独立角色或独立 policy。
3. 不新增 `/api/ai/**` 或 CubeBox 专用业务 HTTP API 作为第二套入口。
4. 不在本计划内重做全部业务 API schema 生成体系；首期可用代码内静态 catalog 起步。
5. 不把 API catalog 当作 capability registry；授权事实仍来自 route requirement、PDP、RLS 和业务读路径。
6. 不在首期支持所有 HTTP route；首批仅覆盖现有 orgunit 只读查询闭环。

## 3. 核心原则

### 3.1 权限继承，不做 CubeBox 单独授权

CubeBox 的权限完全来自当前操作它的用户：

1. 当前用户没有某 API 的 route/service 权限，CubeBox 也不能调用。
2. 当前用户有读权限但数据范围受限，CubeBox 只能看到裁剪后的结果。
3. 当前用户有写权限，也必须先确认，不能由模型直接提交。
4. 审计 actor/principal 仍是当前用户，CubeBox 只作为 `channel/source/tool`。

### 3.2 API Catalog 是工具说明书，不是授权来源

API catalog 的职责：

1. 告诉模型可用 API 的业务用途。
2. 提供请求参数 schema、默认值和参数约束。
3. 标记 API 是 read、non-mutating action、write、dangerous admin、system/internal 等分类。
4. 规定 response 如何投影为 observation。
5. 规定写入/危险操作是否需要确认。

API catalog 不得：

1. 赋予额外权限。
2. 覆盖 route requirement。
3. 绕过业务 API 参数校验。
4. 让未登记 path 进入模型调用面。

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
  -> build current-user API catalog
  -> planner outputs API call plan
  -> API tool runner validates method/path/params against catalog schema
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

为避免模型自由拼 path，`method + path` 必须完全匹配 catalog 条目；参数必须通过 schema 校验。

## 5. API Catalog 契约

### 5.1 Catalog Entry

首期字段：

| 字段 | 含义 |
| --- | --- |
| `method` | HTTP method |
| `path` | HTTP route path |
| `operation_id` | 稳定操作 ID，例如 `orgunit.list`，只作为 catalog 内部引用，不作为第二业务实现 |
| `owner_module` | 归属模块 |
| `intent_summary` | 给模型的简短用途 |
| `request_schema` | query/body 参数 schema |
| `response_schema_ref` | 响应 schema 或 DTO 引用 |
| `operation_kind` | `read` / `non_mutating_action` / `write` / `dangerous_admin` / `system` |
| `confirmation_required` | 写入或危险操作必须为 true |
| `authz_object` / `authz_action` | 来自 route requirement |
| `capability_key` | `object:action` 派生 |
| `observation_projection` | response 到 observation 的投影规则 |

### 5.2 首批 OrgUnit API

首批只纳入只读 API：

| API | operation_id | kind | capability |
| --- | --- | --- | --- |
| `GET /org/api/org-units` | `orgunit.list` | `read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/details` | `orgunit.details` | `read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/search` | `orgunit.search` | `read` | `orgunit.orgunits:read` |
| `GET /org/api/org-units/audit` | `orgunit.audit` | `read` | `orgunit.orgunits:read` |

首期不纳入 orgunit 写 API。写入 API 进入 P4 确认流程后再开放。

### 5.3 当前用户可用性过滤

对 planner 暴露 catalog 时，服务端应按当前用户权限过滤或标记：

1. 无 `object/action` 权限的 API 默认不进入 planner 可调用 catalog。
2. 若为了解释“为什么不能做”保留不可用 API，只能展示为不可调用项，不能生成 call。
3. 数据范围不在 catalog 层提前展开；仍由 API 调用后的业务读路径裁剪。

## 6. 与 486 的关系

`DEV-PLAN-486` 是 executor 路线的整改方案，目标是让当前 executor 成为一等授权入口。490 是 API-first 路线，目标是删除业务 executor 作为平行执行面。

默认取舍：

1. 如果采纳 490，486 中“per-step executor authorizer”不作为主线实施。
2. 486 中关于“当前用户权限、模块边界、命名混乱、第二套实现风险”的问题判断继续有效。
3. 490 用 API catalog + 当前用户 API authz + observation adapter 解决这些问题。
4. 486 可保留为历史对照或应急备选，不得与 490 同时实施成双运行面。

## 7. 实施切片

### 7.1 P0：契约冻结

1. [ ] 490 文档加入 AGENTS Doc Map。
2. [ ] 480/486/485 引用 490，明确若走 API-first 路线，业务工具契约 owner 从 executor registry 转为 API catalog。
3. [ ] 冻结首批仅覆盖 orgunit 只读 API，不纳入写入 API。
4. [ ] 冻结“不自由拼 API”：planner 只能输出 catalog 中存在的 method/path。

### 7.2 P1：API Catalog 最小实现

1. [ ] 在服务端新增静态 API catalog 结构，首批登记四个 orgunit 只读 API。
2. [ ] catalog 条目引用 route requirement，确保 `object/action` 与 484 覆盖事实一致。
3. [ ] 为每个条目定义 request schema、参数默认值和 observation projection。
4. [ ] 增加 catalog 校验：method/path 必须存在、非 allowlist 受保护 route 必须有 requirement、capability 必须存在 registry。

### 7.3 P2：API Call Plan 与 Runner

1. [ ] 新增 `APICallPlan` 解码与校验，替代或并行封装现有 `ReadPlan`。
2. [ ] API runner 校验 method/path 完全匹配 catalog。
3. [ ] API runner 校验 query/body 参数 schema，不允许未知参数。
4. [ ] API runner 以当前用户上下文调用现有 HTTP API 或等价 in-process route adapter。
5. [ ] API runner 不直接调用 store/helper。

### 7.4 P3：OrgUnit 查询链迁移

1. [ ] 将 `orgunit.list/details/search/audit` planner 示例改为 API call plan。
2. [ ] 删除或停用对应业务 executor 实现，不再构造独立 executor payload。
3. [ ] `has_children`、`full_name_path`、`path_org_codes`、`has_more` 等字段只以 HTTP API response schema 为事实源。
4. [ ] 保留现有多步规划、working results、candidate clarification 和 narration 行为。

### 7.5 P4：写入 API 确认机制

1. [ ] 为 write/dangerous API 定义 proposal schema。
2. [ ] planner 对写入只能输出 proposal，不直接执行 API。
3. [ ] UI 展示动作、影响对象、关键字段、有效日期、request_code/幂等键。
4. [ ] 用户确认后，以当前用户身份调用现有业务 API / One Door。
5. [ ] 用户确认不替代后端权限、参数和业务不变量校验。

### 7.6 P5：文档与命名收敛

1. [ ] 将 `ReadAPICatalog` 更名或替换为 `APIToolCatalog`。
2. [ ] 将 `modules/orgunit/presentation/cubebox/apis.md` 改为从 API catalog 派生或仅保留语义说明。
3. [ ] 删除 executor payload 事实源语义，避免 `executor_key` 成为业务工具主键。
4. [ ] 更新功能授权项“关联的访问入口”弹窗：首期只展示 HTTP API，不展示无意义 `类型=API` 列。

## 8. 测试与覆盖率

### 8.1 覆盖口径

本计划不新增覆盖率阈值；按仓库既有 Go、Authz、Routing、UI 门禁执行。新增测试围绕稳定职责，不新增补洞式测试文件。

### 8.2 必补测试

1. API catalog 校验：登记 route 存在、requirement 存在、capability 存在、schema 不允许未知参数。
2. API call plan 校验：未知 method/path、未知参数、缺必填参数、错误类型参数均 fail-closed。
3. API runner：以当前用户身份调用现有 API，权限不足返回 terminal error。
4. OrgUnit 迁移：list/details/search/audit 通过 API path 获取与当前页面 API 一致的字段。
5. 多步查询：search 后 details、list 后继续查 children、result_list 补 `full_name_path` 等行为保持。
6. 写入确认：写 API 在未确认前不执行；确认后仍走现有 API 和 One Door。

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
2. [ ] planner 只能调用 API catalog 中登记的 method/path。
3. [ ] 四个 orgunit 只读能力通过现有 HTTP API 契约执行，并继承当前用户权限。
4. [ ] 页面和 CubeBox 对 `has_children`、`full_name_path`、`path_org_codes`、`has_more` 的依赖来自同一 API response schema。
5. [ ] 无权限用户无法借 CubeBox 调用对应 API；拒绝为 terminal error，不 fallback 到普通聊天。
6. [ ] 写入 API 未进入确认流程前不对 planner 开放直接执行。
7. [ ] 不新增 `/api/ai/**` 或 CubeBox 专用业务 API。
8. [ ] 484/485 覆盖事实仍能追踪 API route 到 capability key。

## 10. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 把统一 API 误解为自由拼 HTTP | 模型生成未登记 path 或任意 query/body | runner 只接受 catalog method/path 与 schema 参数 |
| API catalog 变成第二授权源 | catalog 允许但用户无权限仍执行 | 所有调用仍走当前用户 route/service authz |
| 页面 API 被 AI 需求污染 | 为模型随意改页面 response | observation adapter 做投影，API schema 变更需同时评估页面与 CubeBox |
| AI 专用 API 回流 | 新增 `/api/ai/orgunit-tree` 聚合口 | 停止实施，回到现有业务 API 或正式业务 API 设计 |
| 写入绕过确认 | 模型直接 POST 写 API | write/dangerous kind 无确认不得执行 |
| 保留 executor 与 API 双链路 | 同一能力 API 和 executor 同时可执行 | 迁移完成后停用业务 executor 入口，避免双运行面 |

## 11. 验证记录

- 2026-04-29 23:40 CST：创建方案文档，冻结 CubeBox API-first 工具化重构方向。待实施阶段按命中范围运行文档、Go、Routing、Authz、UI 与 E2E 门禁。
