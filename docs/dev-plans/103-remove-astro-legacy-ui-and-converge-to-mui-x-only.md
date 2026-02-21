# DEV-PLAN-103：移除 Astro/旧局部渲染链路，前端收敛为 MUI X（React SPA）

**状态**: 已完成（2026-02-14）

## 1. 背景

当前仓库存在两套并行的 UI 形态与资产链路：

- 旧链路：Astro Shell + 旧局部渲染链路/Alpine（已移除；历史工程名曾为 `apps/web`；对应 `internal/server/assets/astro/**` + `/ui/*`）
- 新链路：React + MUI Core + MUI X（当前唯一 UI 工程：`apps/web` + `internal/server/assets/web/**`，入口 `/app`）

随着 `DEV-PLAN-090/091/092/094/096` 已把 `/app` 切换为 MUI SPA，继续保留 Astro/旧局部渲染链路会带来：

- 认知负担：两套路由/渲染/状态模型并存（Shell 注入、partial swap vs SPA router）。
- 契约漂移：时间上下文（`as_of/tree_as_of/effective_date`）跨层语义更容易回漂（承接 `DEV-PLAN-102`）。
- 工具链复杂：构建脚本与 CI 触发器需要覆盖两套工程与生成物，且容易漏触发。
- 资产膨胀：`shoelace/vendor` 等 vendored 资产进入 embed，增加体积与维护面。

因此本计划明确：**仅保留 MUI X（React SPA）作为唯一用户 UI**，并在仓库内 **彻底移除 Astro/旧局部渲染链路**，以获得更纯净、更可治理的架构边界。

## 2. 目标与非目标

### 2.1 核心目标（DoD）

- [x] UI 栈收敛：仓库内不再存在 Astro/旧局部渲染链路/Alpine/Shoelace 的运行路径与构建链路；唯一 UI 工程为 `apps/web`。
- [x] 入口收敛：`/app` 及其子路由是唯一应用入口；未登录时统一跳转到 MUI 登录页（`GET /app/login`）。
- [x] 路由收敛：移除旧的 server-rendered UI 路由（例如 `/org/*`、`/person/*` 的 HTML 页面），并用 MUI 页面替代；保留（并强化）JSON API 作为前后端契约。
- [x] 工具链/门禁收敛：CI 的 UI gate 与本地入口只围绕 `apps/web` 与 `internal/server/assets/web/**`（替代 `internal/server/assets/web-mui/**`）；不再构建/校验 Astro 产物。
- [x] 质量证据：更新 E2E/门禁断言，使其不再依赖 `/login` HTML 或 `/org/nodes` 等旧局部渲染链路页面；`make preflight` 可稳定通过（以 `AGENTS.md`/`Makefile`/CI workflow 为 SSOT）。

### 2.2 非目标

- 不在本计划内重写后端领域/DB Kernel/事件模型（保持 One Door 不变量）。
- 不在本计划内改动 RLS/Authz 的边界与对象命名（按既有契约执行）。
- 不在本计划内迁移或重写 SuperAdmin 控制面（`cmd/superadmin`）UI（除非其依赖 Astro/旧局部渲染链路；若有依赖需在实施盘点中明确）。
- 不引入 legacy 双链路回退（遵守 `DEV-PLAN-004M1` No Legacy）。

## 3. SSOT 引用（避免漂移）

- 触发器矩阵/本地必跑：`AGENTS.md`
- 命令入口：`Makefile`
- CI 质量门禁：`.github/workflows/quality-gates.yml`、`docs/dev-plans/012-ci-quality-gates.md`
- MUI X 总方案与子计划：`docs/dev-plans/090-mui-x-frontend-upgrade-plan.md`、`DEV-PLAN-091/092/094/096`
- 时间上下文收敛：`docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- 收尾与证据（承接本计划）：`docs/dev-plans/103a-dev-plan-103-closure-p3-p6-apps-web-rename.md`、`docs/dev-records/dev-plan-103a-execution-log.md`

## 4. 设计决策（本计划冻结）

### 4.1 唯一前端工程与构建产物

- 唯一前端工程：`apps/web`
- 唯一 UI 静态产物（入仓 + go:embed）：`internal/server/assets/web/**`（本计划内从 `internal/server/assets/web-mui/**` 改名）
- 唯一 UI 静态资源 URL 前缀：`/assets/web/`（本计划内从 `/assets/web-mui/` 改名；避免把实现细节写进 URL）
- Go 静态资源挂载：沿用 `/assets/*`（服务端 `FileServer`），但仅保留 MUI 所需的子树（`/assets/web/**`）。

### 4.2 登录与会话（MUI-only）

冻结目标行为（本计划选择：**MUI 登录页 + JSON 登录 API**）：

- 未登录访问任意 `/app/**`（除 `/app/login`）：302 跳转到 `/app/login`。
- `/app/login` 由 SPA 渲染登录页面（服务端仅返回 MUI `index.html`）。
- **不再提供** `GET /login` 的 HTML 页面（返回 404 或路由不存在均可；不引入 302 “兼容别名窗口”，遵守 No Legacy）。
- `POST /iam/api/sessions`：创建会话（JSON），成功时设置 `sid` cookie（HttpOnly）并返回 204（或 201）；失败返回 JSON 错误（422/403/500）。
- `POST /logout`：后端清理会话 cookie；前端负责跳转到 `/app/login`。

> 说明：本计划不强制“登录必须无刷新”，但要求“登录 UI 归属于 MUI SPA”，并且后端必须提供**可被 SPA 稳定消费**的登录 API（JSON 错误，不依赖 HTML/重定向）。

### 4.3 旧 UI 路由与能力迁移策略

本计划以“能力闭环”为切换条件：在对应的 MUI 页面/API/E2E 完成之前，不允许直接删除可用能力导致用户链路断裂（遵守“用户可见性原则”，见 `AGENTS.md`）。

### 4.4 未登录/失效会话的返回语义（UI vs API）

冻结目标行为（避免 SPA 在会话过期时“吃到 302 + HTML”）：

- 对 `internal_api/public_api/webhook`：
  - 缺失/失效/跨租户的 `sid`：返回 JSON `401`（并在必要时清 cookie），**不得** 302。
- 对 `ui`（主要是 `/app/**`）：
  - 缺失/失效/跨租户的 `sid`：302 跳转到 `/app/login`（并在必要时清 cookie）。

> 注：以 route_class（allowlist/classifier）作为判定依据优先于 `Accept` 头；`Accept` 仅作为补充协商信号。

## 5. 实施步骤（建议按 PR 拆分）

### P0：盘点与收敛前置（Stopline）

1. [x] 输出《旧 UI 路由清单》与《旧 UI → MUI 映射表》（以 allowlist/handler/authz/E2E 为事实源），并标注每条路由的迁移/删除状态；证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`：
   - UI 路由（HTML）：`/org/*`、`/person/*`、`/ui/*`、`/login`、`/lang/*` 等
   - API 路由（JSON）：`/org/api/*`、`/person/api/*` 等
   - 同步纳入事实源与收敛点：
     - allowlist：`config/routing/allowlist.yaml`
     - authz 映射：`internal/server/authz_middleware.go`
     - E2E 依赖：`e2e/tests/**`
2. [x] 在 `docs/dev-records/` 新建执行日志：`dev-plan-103-execution-log.md`（实施开始时落盘）。

### P1：CI/UI Build 门禁先修复（避免继续漂移）

3. [x] 调整 UI gate 的路径触发器，使其覆盖：
   - 源码：`apps/web/**`
   - 产物：`internal/server/assets/web/**`
   - E2E 触发器（避免“UI 变了但 E2E 不跑”）：同步覆盖 `apps/web/**`
4. [x] 将 `make css`（或新目标）收敛为“仅构建 MUI 产物并复制到 embed 目录”，不再构建 Astro；并确保 `assert-clean` 能阻断生成物漂移。
5. [x] 静态资源路径与 embed 目录改名（去除 `web-mui` 技术后缀）：
   - URL：`/assets/web-mui/` → `/assets/web/`
   - embed：`internal/server/assets/web-mui/**` → `internal/server/assets/web/**`
   - 同步调整：
     - `apps/web/vite.config.ts`（Vite `base`）
     - 服务端常量 `webMUIIndexPath`（路径与命名一起收敛）
     - 相关测试（E2E/Go 单测）与文档引用（dev-plan、README 等）

> 注：门禁结构与入口以 SSOT 为准（`AGENTS.md`/`Makefile`/CI workflow），本计划只冻结“触发范围必须闭合”这一契约。

### P2：登录入口 MUI 化（移除 `/login` HTML 依赖）

6. [x] 在 `apps/web` 新增路由 `/login`（实际 URL 为 `/app/login`），实现登录页面（表单提交走 JSON API）：
   - 表单字段：email/password
   - 提交策略（冻结）：`POST /iam/api/sessions`（JSON）
7. [x] 后端新增 JSON 登录 API：`POST /iam/api/sessions`：
   - 成功：设置 `sid` cookie（HttpOnly）并返回 204（或 201）
   - 失败：返回 JSON 错误（422/403/500），不得返回 HTML
8. [x] 后端会话中间件调整：
   - 未登录重定向目标从 `/login` 改为 `/app/login`
   - 放行 `/app/login` 与 `POST /iam/api/sessions`（不要求 sid），避免重定向循环
   - `internal_api` 缺失/失效会话返回 JSON 401（不得 302）
9. [x] 更新 E2E（至少 TP060-01 + m3-smoke）断言：不再依赖 `GET /login` 返回 HTML；会话失效/跨租户时对 API 断言 401 JSON，对 UI 断言 302 到 `/app/login`。

### P3：业务页面迁移到 MUI（直到旧 UI 可删除）

10. [x] 将仍在 server-rendered UI 下的能力迁移到 MUI（按模块闭环）；对“仅存旧 HTML 交互壳层”的残留能力做删除/不可达收口，确保业务入口在 MUI 中可发现、可操作（至少覆盖当前已实现能力）；证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`：
   - Org：补齐/巩固（承接 `DEV-PLAN-096`）
   - JobCatalog/Staffing/Person/SetID：为每个模块补齐 MUI 页面入口、API client、权限显隐与错误回显
11. [x] 按 `DEV-PLAN-102` 收敛时间参数：在 MUI 页面中冻结 A/B/C 类路由的时间上下文职责，避免“壳层强灌 as_of”复活；并移除 Person 页面“ignored as-of”等伪需求输入（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。
12. [x] 补齐/调整 API 契约（如需）：本计划未触发新增/调整对外 API 契约；若后续确需调整，先更新对应 dev-plan（Contract First），再落代码。

### P4：删除 Astro/旧局部渲染链路（真正收口）

13. [x] 删除旧 UI 运行路径：
   - 删除 `/ui/nav` `/ui/topbar` `/ui/flash` 等旧局部渲染链路装配端点
   - 删除旧 HTML 页面 handler（例如 `/org/nodes`、`/org/job-catalog`、`/org/positions`、`/org/assignments`、`/person/persons` 等）
14. [x] 删除 Astro 资产与构建链路：
   - 删除 Astro 工程目录（历史名曾为 `apps/web`）
   - 删除 `internal/server/assets/astro/**`、`internal/server/assets/shoelace/**`（若不再被任何路径引用）
   - 删除旧局部渲染链路相关 JS 库文件（如历史 `internal/server/assets/js/lib/*`）以及 `internal/server/assets/js/lib/alpine.min.js`（若不再被任何路径引用）
   - 删除旧 CSS/样式产物（例如 `internal/server/assets/app.css`，若仅供旧 UI）
   - 移除服务端 Astro Shell 注入代码（`renderAstroShellFromAssets`、`writeShell*` 等）
15. [x] 更新 allowlist/authz 路由映射：
   - allowlist：移除 `/ui/*`、`/lang/*`、旧 HTML 路由与 `/login`（GET），补齐 `/iam/api/sessions`
   - authz：移除旧 UI 路由的 `authzRequirementForRoute` 分支；补齐 `/iam/api/sessions`；确保 API 仍受控且 fail-closed

### P5：文档与版本冻结更新

16. [x] 更新技术栈冻结文档：从 `DEV-PLAN-011` 中移除 Astro 作为 UI SSOT 的描述，改为以 `apps/web/package.json` + lockfile 为唯一事实源。
17. [x] 更新仓库入口文档（`AGENTS.md`）：
   - 移除 Astro/旧局部渲染链路相关触发器描述（例如 `make css` 的“Tailwind/Astro”叙述），改为 MUI-only 的触发器口径
   - Doc Map 保留 `DEV-PLAN-018` 但标注“已被 103 替代（历史记录）”
18. [x] 为历史计划加“已被替代/不再适用”的显式说明（至少 `DEV-PLAN-018`），避免后续误用。

### P6（可选但推荐）：工程命名去技术后缀

19. [x] 在旧 UI 工程删除完成后，完成目录改名：前端工程目录统一为 `apps/web`（仅做机械改名 + 引用更新；不夹带功能改动）。

## 6. 验收标准

- [x] 仓库内不存在 Astro/旧局部渲染链路/Alpine/Shoelace 的运行路径与构建步骤；`internal/server/assets/astro/**` 已移除（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。
- [x] 未登录访问 `/app` 会跳转到 `/app/login`；`internal_api` 未登录返回 401 JSON（不 302）；登录后进入 MUI Shell；退出登录链路可用（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。
- [x] 不再提供 `GET /login` 的 HTML 页面；不会出现 `/login` → `/app/login` 的“兼容别名窗口”（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。
- [x] 旧的 server-rendered UI 路由不可达或已移除；业务能力在 MUI 页面可发现、可操作（至少覆盖现有已实现能力）（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。
- [x] MUI 静态资源前缀为 `/assets/web/`，embed 目录为 `internal/server/assets/web/**`，仓库内不再引用 `/assets/web-mui/` 与 `internal/server/assets/web-mui/**`（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。
- [x] CI UI gate 能在 `apps/web/**` 或 `internal/server/assets/web/**` 变更时触发，并能阻断生成物漂移（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。
- [x] E2E（至少 TP060-01 + m3-smoke）通过且不依赖旧 UI；整体门禁入口以 `make preflight` 对齐（证据入口见 `docs/dev-records/dev-plan-103a-execution-log.md`）。

## 7. 风险与缓解

- 风险：迁移面过大导致“删旧太早、能力断裂”。  
  缓解：以“能力闭环 + E2E 证明”为删除前置；先迁移、后删除。
- 风险：登录/重定向规则调整导致循环或越权。  
  缓解：为 `/app/login` 建立明确的中间件放行规则，并在单测/E2E 覆盖“未登录/跨租户/权限拒绝”路径。
- 风险：时间上下文语义在 SPA 内再次发散。  
  缓解：承接 `DEV-PLAN-102`，把路由分类与参数职责固化为契约矩阵与测试断言。
- 风险：引入“兼容别名窗口”变相形成 legacy 回退。  
  缓解：删除 `GET /login`（不做 302 兼容）；用 E2E 证明新入口可用后再删旧路由。

## 8. 交付物

- `docs/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`（本文件）
- `docs/dev-records/dev-plan-103-execution-log.md`（实施时创建）
