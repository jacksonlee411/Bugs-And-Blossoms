# DEV-PLAN-486：CubeBox Executor 路线历史对照（已停止）

**状态**: 已停止（2026-04-30 CST；当前 PoR 转入 `DEV-PLAN-490`）

## 0. 适用范围与评审分级

- **评审分级**：`T1`（文档收敛）
- **范围一句话**：记录 CubeBox executor 路线为何不再作为当前实施 owner；本文件不得再作为新增 UI、授权目录、覆盖门禁或业务工具契约的依据。
- **关联模块/目录**：`modules/cubebox/**`、`modules/orgunit/presentation/cubebox/**`、`internal/server/cubebox_query_flow.go`、`internal/server/cubebox_orgunit_executors.go`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-480`、`DEV-PLAN-484`、`DEV-PLAN-485`、`DEV-PLAN-490`
- **用户入口/触点**：无新增用户入口；不规划 executor key 展示、executor 目录或 executor 管理页。

### 0.1 Simple > Easy 三问

1. **边界**：486 不再拥有 CubeBox runtime 授权、executor requirement、executor catalog 或模块边界整改的实施切片；当前业务工具路线由 `DEV-PLAN-490` 统一承接。
2. **不变量**：CubeBox 不是独立授权主体；当前路线要求业务工具调用回到现有 HTTP API 契约，以当前用户和当前租户执行。
3. **可解释**：文档中出现 executor 只表示历史实现与风险来源；任何当前可执行需求必须引用 `DEV-PLAN-490`，不得引用 486 作为实施依据。

## 1. 背景

486 创建时，CubeBox 查询链已经存在 `ExecutionRegistry` 与 `orgunit.details/list/search/audit` executor。原始方案试图把 executor 提升为一等授权入口，并补齐 per-step authorizer、executor requirement、模块归属和覆盖门禁。

后续评审明确：当前不走 executor 路线。继续修补 executor 会保留第二套业务工具契约，并让 API 授权目录、功能授权项和 CubeBox 运行时继续分叉。当前 PoR 是 `DEV-PLAN-490`：CubeBox 业务查询/操作以现有 HTTP API 为唯一业务工具契约，executor payload 不再作为事实源。

## 2. 当前决策

1. 不新增 `ExecutorAuthorizationRequirement`、per-step executor authorizer 或 executor requirement 覆盖门禁。
2. 不在功能授权项、API 授权目录或关联 API 弹窗展示 executor key。
3. 不新增 executor 管理页、executor 目录、executor 类型列或 executor 专属用户入口。
4. 不把 `ReadAPICatalog` / `apis.md` 继续收敛为 executor catalog；命名与知识包后续按 `DEV-PLAN-490` 收敛到 API tool overlay / API call plan。
5. 当前实现中已经存在的业务 executor 只作为迁移前技术债处理；删除 active runtime 中的业务 executor 执行入口由 `DEV-PLAN-490` 的 P3 切片承接，不以长期停用、空 registry 或兼容封装替代删除。

## 3. 保留的历史判断

以下判断仍可作为 490 的问题背景，但不是 486 的实施要求：

1. 只校验 `cubebox.conversations:use` 不能代表用户拥有业务数据权限。
2. executor payload、页面 API DTO、知识包字段和测试同时维护同类字段，会造成事实源漂移。
3. 业务工具实现长期堆在 `internal/server` 会模糊模块边界。
4. CubeBox 对业务数据的访问必须继承当前用户权限、RLS、数据范围和字段裁剪。

## 4. 停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| executor 路线回流 | 新增 executor requirement、executor authorizer 或 executor catalog | 停止实施，回到 `DEV-PLAN-490` 的 API-first 工具契约 |
| UI 泄露内部工具键 | 功能授权项或 API 授权目录展示 executor key | 删除该展示；用户可见面只展示 HTTP API 授权事实 |
| 双运行面扩大 | 同一能力同时新增 HTTP API 工具和 executor 工具 | 只保留 490 派生的 HTTP API 工具面，删除 active runtime 的业务 executor 可执行入口 |
| 486 被当成当前 owner | 新计划引用 486 作为实施依据 | 改为引用 `DEV-PLAN-490`，486 仅作历史对照 |

## 5. 验证记录

- 2026-04-29 23:08 CST：创建方案文档，记录当时 CubeBox executor 已实际进入主链路但缺 per-step 授权与模块边界治理。
- 2026-04-30 CST：按最新产品与架构决策停止 executor 路线；当前业务工具契约转入 `DEV-PLAN-490` API-first 方案，486 仅保留历史对照与反回流停止线。
