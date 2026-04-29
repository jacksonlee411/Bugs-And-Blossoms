# DEV-PLAN-486：CubeBox Executor 授权与模块边界整改方案

**状态**: 规划中（2026-04-29 23:08 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：把当前已经真实执行业务读取的 CubeBox executor 从“查询链内部工具”提升为一等授权入口，补齐 per-step 授权、executor requirement、模块归属、命名收敛与共享读路径边界。
- **关联模块/目录**：`modules/cubebox/**`、`modules/orgunit/presentation/cubebox/**`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_orgunit_executors.go`、`internal/server/cubebox_api.go`、`pkg/authz/**`、`scripts/authz/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-015`、`DEV-PLAN-022`、`DEV-PLAN-480`、`DEV-PLAN-482`、`DEV-PLAN-484`、`DEV-PLAN-485`
- **用户入口/触点**：CubeBox 对话流式入口、模型生成的 `ReadPlan`、executor registry、功能授权项的“关联的访问入口”弹窗、后续 executor 覆盖门禁

### 0.1 Simple > Easy 三问

1. **边界**：486 只拥有 CubeBox executor 运行时整改和模块边界收敛；484 继续拥有 route/executor/registry/policy 覆盖门禁；485 只拥有 HTTP API 正向目录页面。
2. **不变量**：能使用 `cubebox.conversations:use` 只表示能进入对话，不表示能执行任何业务 executor；每个 executor step 必须在执行前按其 `object/action` requirement 授权，缺 requirement、缺 authorizer、deny 或 authorizer error 都 fail-closed。
3. **可解释**：任意 executor key 都能回答“它代理哪个业务能力、由哪个模块拥有、执行前检查哪个 capability、调用哪条业务读路径、失败时如何终止”。

## 1. 背景

当前 CubeBox 已经实际使用 executor，而不是只在文档中预留：

1. `buildDefaultCubeboxQueryFlow(...)` 会注册 orgunit executors，并创建 `ExecutionRegistry`。
2. `/internal/cubebox/turns:stream` 会优先进入 `queryFlow.TryHandle(...)`。
3. 当 planner 输出 `READ_PLAN` 时，query flow 调用 `ExecutionRegistry.ExecutePlan(...)`。
4. 当前已注册的 orgunit executor 包括 `orgunit.details`、`orgunit.list`、`orgunit.search`、`orgunit.audit`。

这条链路证明 executor 已经是实际业务数据入口。但当前实现仍存在几个结构性缺口：

1. HTTP route 只校验 `cubebox.conversations:use`，未对每个业务 executor step 校验其代理的业务 capability。
2. `RegisteredExecutor` 没有 `object/action` requirement，无法被 484 的覆盖门禁完整枚举。
3. orgunit executor 实现在 `internal/server`，模块语义和 server 组合层混在一起。
4. 运行时事实源叫 executor registry，但周边仍使用 `ReadAPICatalog`、`apis.md` 等 API 命名，持续制造“executor 是 HTTP API”的误解。
5. executor 直接调用 orgunit store/helper 读路径，而不是 HTTP API；这个方向可以保留，但必须保证与普通 API 共享同一套服务端读路径、数据范围和字段裁剪。

## 2. 目标与非目标

### 2.1 核心目标

1. [ ] 给 CubeBox `RegisteredExecutor` 增加授权 requirement 元数据，统一表达 `authz_object + authz_action`。
2. [ ] 给 `ExecutionRegistry.ExecutePlan(...)` 注入 executor authorizer，并在每个 step 执行前 fail-closed 授权。
3. [ ] 将现有 `orgunit.*` executor 绑定到 `orgunit.orgunits:read`，并补缺 requirement、deny、authorizer error、多 step 中途拒绝测试。
4. [ ] 将 executor requirement 纳入 484 的覆盖事实枚举，确保 capability registry、policy、executor requirement 不漂移。
5. [ ] 收敛 orgunit executor 的模块归属，使业务 executor 声明与知识包归属于 orgunit 模块边界，`internal/server` 只负责组合装配和 authz adapter。
6. [ ] 将 `ReadAPICatalog` / `apis.md` 命名逐步收敛为 executor/tool catalog 语义，避免把 executor 展示为 HTTP API。

### 2.2 非目标

1. 不让模型直接调用 HTTP API path，不把 `/org/api/...` 暴露为 planner 可自由拼接的执行面。
2. 不新增 DB 表、迁移、在线 executor 管理页或在线 capability registry 管理。
3. 不在本计划内实现复杂 ABAC、字段级授权配置 UI 或组织范围 SoT；executor 授权只覆盖 capability/API 层，数据范围仍由业务读路径承接。
4. 不新增第二套 orgunit 查询 endpoint，不为了 CubeBox 创建专用 HTTP API。
5. 不改变 485 的 HTTP API 访问入口目录页面边界；executor 展示只进入功能授权项的“关联的访问入口”或后续专门 executor 目录。

## 3. 决策

### 3.1 Executor 继续保留，不退回 HTTP API

保留 executor 是正确方向：模型只能选择已登记 `executor_key`，不能自由拼 HTTP path、SQL 或内部函数。executor 负责参数白名单、线性步骤、observation 裁剪、候选澄清和终止错误。

但 executor 必须补齐运行时授权。否则它就是绕过普通 route PEP 的第二读入口。

### 3.2 Executor Requirement 是运行时强制项

新增最小结构：

```go
type ExecutorAuthorizationRequirement struct {
	Object string
	Action string
}

type RegisteredExecutor struct {
	ExecutorKey    string
	RequiredParams []string
	OptionalParams []string
	RuntimeHints   QueryRuntimeHints
	Authorization  ExecutorAuthorizationRequirement
	Executor       ReadExecutor
}
```

运行时规则：

1. `NewExecutionRegistry` 注册时校验 `ExecutorKey`、`Executor`、`Authorization.Object`、`Authorization.Action` 都非空。
2. `ExecutePlan` 每个 step 在 `ValidateParams` 和 `Execute` 前调用 authorizer。
3. 缺 authorizer、缺 requirement、authorizer error、deny 都返回 terminal error，不 fallback 到普通聊天链路。
4. authorizer 只判断 capability；组织范围、对象实例、字段裁剪仍由业务读路径强制。

### 3.3 Server 只做组合根，不持有业务 executor 语义

目标边界：

1. `modules/cubebox` 拥有通用 executor registry、plan 执行、authorizer interface、tool catalog 投影。
2. `modules/orgunit/presentation/cubebox` 拥有 orgunit executor 声明、知识包、参数契约和业务 observation 形状。
3. `internal/server` 只负责把 orgunit store、cubebox runtime、provider adapter 和 authz adapter 组装起来。

允许过渡期保留 `internal/server/cubebox_orgunit_executors.go`，但新增 executor 不得继续默认落在 `internal/server`。

### 3.4 运行时事实源优先于 Markdown

executor registry 是执行事实源。知识包只用于模型语义提示，不是执行授权来源。

整改方向：

1. 参数目录优先由 registry 派生。
2. `apis.md` 可短期保留作为模型知识包文件名，但正文必须明确其列出的是 executor/tool catalog，不是 HTTP API。
3. 后续有窗口时将 `ReadAPICatalog` 更名为 `ExecutorCatalog` 或 `ToolCatalog`，并一次性更新 prompt、测试和文档。

## 4. 目标架构

```text
HTTP POST /internal/cubebox/turns:stream
  -> route authz: cubebox.conversations:use
  -> planner outputs READ_PLAN with executor_key
  -> ExecutionRegistry resolves RegisteredExecutor
  -> per-step authorizer checks executor.Authorization object/action
  -> executor validates params
  -> module-owned business read path applies tenant/scope/field rules
  -> observation/narration result
```

关键关系：

```text
executor_key       -> executor requirement          -> capability key
orgunit.details    -> orgunit.orgunits + read        -> orgunit.orgunits:read
orgunit.list       -> orgunit.orgunits + read        -> orgunit.orgunits:read
orgunit.search     -> orgunit.orgunits + read        -> orgunit.orgunits:read
orgunit.audit      -> orgunit.orgunits + read        -> orgunit.orgunits:read
```

## 5. 数据与授权契约

### 5.1 Authorizer Interface

建议在 `modules/cubebox` 定义最小接口，避免直接 import `pkg/authz`：

```go
type ExecutionAuthorizer interface {
	AuthorizeExecution(ctx context.Context, request ExecuteAuthorizationRequest) (ExecuteAuthorizationDecision, error)
}

type ExecuteAuthorizationRequest struct {
	TenantID       string
	PrincipalID    string
	ConversationID string
	ExecutorKey    string
	Object         string
	Action         string
}

type ExecuteAuthorizationDecision struct {
	Allowed bool
	Reason  string
}
```

`internal/server` 提供 adapter，把该请求映射到现有 authz PDP/Casbin。

### 5.2 失败语义

| 场景 | 行为 |
| --- | --- |
| executor 缺 requirement | 注册失败；如运行时发现则 terminal error |
| registry 缺 authorizer | terminal error |
| authorizer error | terminal error |
| authorizer deny | terminal error，不继续普通聊天 |
| 多 step 中第 N 步 deny | 整个执行链终止，不返回前序结果 |
| 业务读路径范围拒绝 | 沿用业务 terminal error 或 forbidden 映射 |

### 5.3 覆盖事实

486 实施后，484 的 executor 覆盖枚举至少能输出：

| 字段 | 含义 |
| --- | --- |
| `type` | 固定 `executor` |
| `executor_key` | `orgunit.list` |
| `owner_module` | `orgunit` |
| `resource_object` | `orgunit.orgunits` |
| `action` | `read` |
| `capability_key` | `orgunit.orgunits:read` |
| `status` | `registered` / `disabled` 等后续状态 |

## 6. 实施切片

### 6.1 P0：契约冻结

1. [ ] 486 文档加入 AGENTS Doc Map。
2. [ ] 480/484/485 引用 486，明确 CubeBox executor 实际入口整改 owner。
3. [ ] 冻结当前 orgunit executor 与 capability 的映射：四个 `orgunit.*` 只读 executor 均绑定 `orgunit.orgunits:read`。

### 6.2 P1：Executor 授权元数据与运行时 fail-closed

1. [ ] 在 `modules/cubebox` 增加 `ExecutorAuthorizationRequirement` 与 authorizer interface。
2. [ ] 扩展 `RegisteredExecutor`，要求注册时必须携带 `Authorization`。
3. [ ] 扩展 `ExecutionRegistry`，支持注入 authorizer。
4. [ ] `ExecutePlan` 每个 step 执行前调用 authorizer。
5. [ ] 补测试覆盖 allow、deny、缺 authorizer、缺 requirement、authorizer error、多 step 中途 deny。

### 6.3 P2：orgunit executor 绑定 requirement

1. [ ] `orgunit.details` 绑定 `orgunit.orgunits + read`。
2. [ ] `orgunit.list` 绑定 `orgunit.orgunits + read`。
3. [ ] `orgunit.search` 绑定 `orgunit.orgunits + read`。
4. [ ] `orgunit.audit` 绑定 `orgunit.orgunits + read`。
5. [ ] API 层流式入口测试覆盖用户只有 `cubebox.conversations:use` 但无 `orgunit.orgunits:read` 时被拒绝。

### 6.4 P3：模块边界收敛

1. [ ] 为 orgunit 提供模块级 CubeBox executor registration 函数或 provider，避免新增业务 executor 继续堆进 `internal/server`。
2. [ ] `internal/server` 改为组合模块 provider，不直接持有 orgunit executor 业务语义。
3. [ ] 保持现有 orgunit query behavior 不变，迁移只改变所有权边界与授权强制点。

### 6.5 P4：命名与目录收敛

1. [ ] 将运行时代码中的 `ReadAPICatalog` 迁移为 `ExecutorCatalog` 或 `ToolCatalog`。
2. [ ] 将 prompt 中“已登记只读 API”改为“已登记只读 executor/tool”。
3. [ ] 更新 `modules/orgunit/presentation/cubebox/apis.md` 正文，必要时另起文件名迁移；若迁移文件名，必须同步 `LoadKnowledgePack` 和知识包校验。
4. [ ] 确保用户可见回答继续不泄露 `executor_key`、plan、payload、params 等内部字段。

### 6.6 P5：484 覆盖门禁接入

1. [ ] 484 的 executor requirement 枚举读取 `RegisteredExecutor.Authorization`。
2. [ ] 缺 requirement、registry 外 object/action、assignable capability 无 API/executor 覆盖时 lint 失败。
3. [ ] 功能授权项“关联的访问入口”弹窗可展示 executor key；当前首期若页面只展示 HTTP API，则不展示 `类型=API` 的无意义列。

## 7. 测试与覆盖率

### 7.1 覆盖口径

本计划不新增覆盖率阈值；按仓库现有 Go、Authz、UI 门禁执行。新增测试应围绕稳定职责，不新增 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 这类补洞式文件。

### 7.2 必补测试

1. `modules/cubebox`：executor registry 注册校验、per-step authorizer allow/deny/error、缺 requirement fail-closed、多 step 中途拒绝。
2. `internal/server`：authz adapter 将 principal/tenant/object/action 正确传入 PDP；`/internal/cubebox/turns:stream` 在业务 executor deny 时返回 terminal error。
3. `modules/orgunit` 或模块级 adapter：四个 orgunit executor 均带 requirement，且原有 list/search/details/audit 行为不变。
4. `scripts/authz` 或 authz lint：executor requirement 可被枚举并与 capability registry 交叉校验。

### 7.3 验证命令

实施阶段按命中范围运行：

1. Go 代码：`go fmt ./... && go vet ./... && make check lint && make test`
2. Authz：`make authz-pack && make authz-test && make authz-lint`
3. 文档：`make check doc`
4. 若触及前端功能授权项弹窗：按 AGENTS UI 触发器运行前端测试、`make generate && make css` 与相关 E2E。

## 8. 验收标准

1. [ ] `cubebox.conversations:use` 不再隐含任何业务 executor 权限。
2. [ ] 任一 executor 缺 authorization requirement 时无法进入可执行 registry。
3. [ ] 用户缺 `orgunit.orgunits:read` 时，`orgunit.details/list/search/audit` 均 fail-closed。
4. [ ] 多 step plan 中任一步授权拒绝时，整个执行链终止，前序结果不被 narrator 输出。
5. [ ] 484 能枚举 executor requirement，并阻断 registry 外 object/action 与空壳 assignable capability。
6. [ ] 新增业务 executor 的默认位置是模块边界，不是 `internal/server` 业务语义堆叠。
7. [ ] prompt、知识包和代码命名不再把 executor 误称为 HTTP API。

## 9. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 只在 route 层校验 CubeBox use | 能聊天即能查业务数据 | 必须实现 per-step executor authorizer |
| executor 继续无 requirement | 484 无法覆盖 executor | 缺 requirement 注册失败 |
| 为规避授权改让模型调 HTTP API | 模型拼 path/query，扩大攻击面 | 模型只能输出已登记 executor key |
| 模块迁移过度重构 | 行为变化与边界迁移混杂 | P3 只迁移所有权边界，不改业务查询行为 |
| 命名迁移破坏知识包 | prompt/校验找不到文件 | 文件名迁移必须单独小步，先改正文语义 |
| 把字段/组织范围塞进本计划 | 486 膨胀为 ABAC/字段安全工程 | 数据范围和字段授权继续由 480 后续子计划承接 |

## 10. 验证记录

- 2026-04-29 23:08 CST：创建方案文档，记录当前 CubeBox executor 已实际进入主链路但缺 per-step 授权与模块边界治理。待实施阶段按命中范围运行文档、Go、Authz 与 UI 门禁。
