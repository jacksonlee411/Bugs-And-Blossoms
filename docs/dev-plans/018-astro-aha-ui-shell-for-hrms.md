# DEV-PLAN-018：引入 Astro（AHA Stack）到 HTMX + Alpine 的 HRMS UI 方案（026-031）

**状态**: 执行中（Phase 0：Astro build + go:embed Shell，2026-01-08 10:24 UTC）

## 1. 背景与上下文 (Context)

仓库当前 UI 技术栈为 **Templ + HTMX + Alpine.js + Tailwind CSS**（版本与 SSOT 见 `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`）。与此同时，`DEV-PLAN-026`～`DEV-PLAN-031` 定义了 HRMS（Greenfield）的核心模块与契约：
- 内核边界：DB=Projection Kernel（权威），Go=Command Facade（编排），One Door Policy（唯一写入口）（见 026/030/029/031）。
- Valid Time=DATE；时间戳仅用于 Audit/Tx Time。读模型使用 `daterange` 且统一 `[start,end)`（左闭右开）（口径见 `AGENTS.md` 与 `docs/dev-plans/032-effective-date-day-granularity.md`，并对齐 026/030/029/031）。
- UI 合同：任职记录（Job Data / Assignments）**仅显示 `effective_date`**（不展示 `end_date`），但底层继续沿用 `daterange [start,end)`（见 031）。
- Greenfield 模块划分（仅实现 026-031 所覆盖模块）：`orgunit/jobcatalog/staffing/person`（见 016）。

本计划目标是在不改变“HTMX + Alpine 的交互范式”的前提下，引入 **Astro（AHA Stack：Astro + HTMX + Alpine）**，用于：
- 统一页面壳（shell）、导航与信息架构（IA）
- 统一 UI 组件、视觉规范与布局系统（Design System）
- 降低页面级模板复杂度与重复，提高可复用性

### 1.1 适用范围：Greenfield（同仓内引入 Astro 工程）
本计划按 “Greenfield、同仓实现” 口径制定：
- 在本仓库 `apps/web` 建立 Astro 工程，作为 UI 壳与静态资源的 build 层（对齐 `DEV-PLAN-011` 的 SSOT 口径）。
- 不做 feature flag 分流；不在同一路由下并存两套壳；回退以**发布版本回滚**（上一版构建产物/上一版镜像）为唯一手段。

### 1.2 现状（可验证，2026-01-06）
> 目的：满足 `DEV-PLAN-003` Stage 1 的“准确描述现状”最低要求，避免用“想象中的架构”驱动计划。

- Go 侧已存在一个最小 Shell（SSR）与 HTMX 装配点：`/app` -> `<aside#nav hx-get="/ui/nav">`、`<header#topbar hx-get="/ui/topbar">`、`<div#flash hx-get="/ui/flash">`、`<main#content>`（见 `internal/server/handler.go`）。
- `config/routing/allowlist.yaml` 已将 `/ui/nav`、`/ui/topbar`、`/ui/flash` 纳入 allowlist（对齐 `DEV-PLAN-017` 的路由治理）。
- `apps/web` 当前为占位目录（`.gitkeep`），尚未落盘 Astro 工程与可复现的 `package.json`/lockfile（对齐 `DEV-PLAN-011` 的“落盘并冻结版本”要求）。
- `make generate` / `make css` 当前为 placeholder（见 `Makefile`）；本计划会把 “Astro build + CSS 输出” 的入口收敛到 `Makefile`（不在本文复制命令细节）。

### 1.3 现状图（请求流）
```mermaid
flowchart LR
  B[Browser] -->|GET /app| S[Go Server]
  S -->|HTML Shell| B
  B -->|HTMX GET /ui/nav| S
  B -->|HTMX GET /ui/topbar| S
  B -->|HTMX GET /ui/flash| S
  B -->|click nav -> HTMX GET {page}| S
  S -->|HTML partial swap| B
```

### 1.4 风险点与验证手段（最小集合）
- **as-of 漂移**（链接/partial 丢失 `as_of`）：用 E2E 断言 URL 始终包含 `as_of` 且跨页面不丢（入口见 `DEV-PLAN-012`）。
- **双权威表达**（Astro 维护一份导航、服务端又维护一份）：冻结为“服务端渲染 `/ui/nav`”，Astro 只提供容器与样式（见 §4.3/§6 Phase 1）。
- **静态资源不一致**（Astro build 产物未被打包进镜像/二进制）：冻结为 go:embed，且在构建门禁中校验产物存在（入口以 `Makefile`/CI 为准）。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标
- [ ] 只实现 026-031 的 4 个模块的 UI：`OrgUnit`、`Job Catalog`、`Staffing`、`Person`；左侧导航栏布局与目前一致。
- [ ] 模块为一级菜单；模块下子模块为二级菜单；不引入更多层级。
- [ ] 在 HTMX + Alpine 的基础上引入 Astro：Astro 负责页面壳/组件编译与静态资源组织；交互仍以 HTMX 为主、Alpine 为辅。
- [ ] 明确 i18n、Authz、as-of（有效日期）等全局 UI 能力在新架构下的边界与集成方式。
- [ ] 给出可执行的落地步骤与验收标准（避免 “Easy but not Simple”）。

### 2.2 非目标（明确不做）
- 不在本计划内替换 DB Kernel/领域实现（026-031 的后端契约不在此计划内变更）。
- 不在本计划内把系统改成 SPA；不引入前端状态管理框架（React/Vue/Redux 等）。
- 不在本计划内把系统改造成“前端渲染为主”；业务 HTML 仍以服务端渲染 + HTMX swap 为主。

### 2.3 工具链与门禁（SSOT 引用）
- DDD 分层框架：`docs/dev-plans/015-ddd-layering-framework.md`
- HR 模块骨架（4 模块）：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- 任职记录 UI 合同（仅显示 effective_date、保持 `[start,end)`）：`docs/dev-plans/031-greenfield-assignment-job-data.md`
- 路由治理与门禁：`docs/dev-plans/017-routing-strategy.md`（入口：`make check routing`）
- 分层/依赖门禁：`.gocleanarch.yml`（入口：`make check lint`）
- 样式与生成入口：`Makefile`（Tailwind/生成物以 SSOT 为准）
- 文档门禁：`make check doc`

## 3. 信息架构（IA）与导航（左侧布局不变）

### 3.1 一级菜单（模块）
仅提供 4 个一级模块（与 016 对齐）：
- `OrgUnit`（组织架构）
- `Job Catalog`（职位分类）
- `Staffing`（职位 + 任职）
- `Person`（人员）

### 3.2 二级菜单（子模块）
二级菜单以“页面入口”维度定义，且不跨模块混放：

**OrgUnit**
- 组织结构（Tree）：`/org/nodes`
- 组织节点详情（Panel/Details）：`/org/nodes/{id}`（可复用现有信息架构）

**Job Catalog**
- 职类总览（Overview，默认重定向到 family groups）：`/org/job-catalog`
- 职类组（Job Family Groups）：`/org/job-catalog/family-groups`
- 职类（Job Families）：`/org/job-catalog/families`
- 职级（Job Levels）：`/org/job-catalog/levels`
- 职位模板（Job Profiles）：`/org/job-catalog/profiles`

**Staffing**
- 职位（Positions）：`/org/positions`
- 任职记录（Job Data / Assignments）：`/org/assignments`

> 说明：`/org/*` 是 UI 命名空间（信息架构/导航维度），不是 Go 模块边界；`/org/positions` 与 `/org/assignments` 的实现归属 `modules/staffing`，Authz object 使用 `staffing.positions`/`staffing.assignments`（对齐 `DEV-PLAN-016/022`）。

**Person**
- 人员列表（Persons）：`/person/persons`
- 人员详情（Person Details）：`/person/persons/{person_uuid}`

> 说明：路由沿用 016 的“人机入口稳定”建议（仍使用 `/org/*` 与 `/person/*`），但导航以“模块”维度组织，解决当前“Person 里挂 org/assignments”造成的 IA 混乱。

### 3.3 全局 as-of（有效日期）交互（统一入口）
为对齐 valid-time 语义，页面壳提供一个全局 “As-of 日期”控件：
- **冻结口径：Query 参数** `?as_of=YYYY-MM-DD`（禁止混用 `hx-include` 作为第二套透传口径）。
- `as_of` 缺省行为：对所有“业务内容 UI 路由”（含 `/app` 与模块页面），若未提供 `as_of`，服务端必须 **302 重定向** 到同一路径并补齐 `?as_of=<CURRENT_DATE(UTC)>`，从而让 URL 可分享/可复现。
- `as_of` 校验：若 `as_of` 非法（格式不为 `YYYY-MM-DD` 或不可解析），返回 **400**（UI 显示错误，且不做“猜测性纠正”）。
- 透传规则：`/ui/nav`、`/ui/topbar`、以及所有模块内链接都必须保留当前 `as_of`（使 URL 可分享/可复现）。
- 对任职记录页面：仅展示 `effective_date`（即 `lower(validity)`）；as-of 用于筛选/定位快照，不引入 `end_date` 展示（对齐 031）。

### 3.4 As-of 控件交互（HTMX，冻结口径）
- `/ui/topbar` 渲染一个 `method="GET"` 的日期表单（`name="as_of"`），提交后以 **GET + `hx-push-url=true`** 更新当前页面 URL，并刷新 `#content`（不需要引入 SPA 或前端状态管理）。
- `q`（搜索）与其他筛选条件必须与 `as_of` 共存于 query string；任何刷新/分页/详情跳转都不得丢失 `as_of`。
- Job Catalog 默认页：`/org/job-catalog` 必须保留 `as_of` 并重定向到 `/org/job-catalog/family-groups?as_of=...`（或等价的默认 section）。

## 4. UI 技术架构：Astro + HTMX + Alpine（AHA）

### 4.1 总体原则
- **Astro = 壳与组件编译层**：负责 layout/导航组件/页面框架/静态资源与 build pipeline；尽量不承载业务数据渲染。
- **HTMX = 业务交互与数据驱动渲染**：业务页面内容以 server-rendered HTML partial 为主，依赖 `hx-get/hx-post/hx-target` 做局部刷新。
- **Alpine = 局部状态与微交互**：导航折叠、快捷键、弹窗、表单局部校验提示等；不做跨页面状态管理。

### 4.2 “壳（Shell）”与“内容（Content）”分离
引入一个统一的 App Shell：
- 左侧导航（与现有布局一致）
- 顶部栏（As-of 日期、搜索入口、用户/租户/语言）
- 主内容区（由 HTMX 拉取模块内容并 swap）

核心约束：**Shell 负责结构与导航，Content 负责业务**。Shell 允许是 Astro 产物；Content 仍可由 Go（Templ/handlers）渲染，逐步迁移不强制一次到位。

### 4.3 Authz / i18n 集成方式（不把动态信息固化到 Astro）
为避免“静态壳无法感知用户权限/语言”的矛盾：
- 导航与页面标题的最终渲染仍由服务端输出 HTML（可复用现有本地化与权限判定），Astro 壳只提供容器与样式。
- Astro 壳在加载时通过 HTMX 拉取：
  - `/ui/nav`：当前用户可见的导航 HTML（含二级菜单）
  - `/ui/topbar`：包含 As-of 控件与用户信息
  - `/ui/flash`：统一错误/成功提示（统一出口，避免在 JS 分支里散落 toast/alert）

> 这保持了“权威表达在服务端”的简单性：权限/语言不在前端复制一套判断逻辑。

### 4.4 Shell/Partial 的最小契约（冻结）
为降低“壳与内容互相猜测”的偶然复杂度，冻结如下最小契约：
- Shell 必须包含固定 ID 的挂载点：`#nav`、`#topbar`、`#flash`、`#content`。
- Shell 必须在用户已登录的上下文中触发加载：`hx-get="/ui/nav?as_of=__BB_AS_OF__"`、`hx-get="/ui/topbar?as_of=__BB_AS_OF__"`、`hx-get="/ui/flash"`（其中 `__BB_AS_OF__` 由 `/app` handler 在返回 Shell 时注入；契约 URL 路径不变；见 §4.5）。
- 内容区页（任意模块页面）必须满足：同一 URL 同时支持 “全页访问” 与 “HTMX partial（`Hx-Request: true`）” 两种模式（路由/协商口径对齐 `DEV-PLAN-017`）。
- `as_of` 作为 URL 状态：Shell 与 partial 的所有 HTMX 请求与链接必须保留 `as_of`（见 §3.3）。

### 4.5 Astro build 产物交付方式（冻结：go:embed）
为保持发布物简单（Simple）并减少部署侧偶然复杂度，冻结如下交付方式：
- Astro build 产物在构建阶段被复制到 Go 可 `go:embed` 的稳定目录（建议：`internal/server/assets/astro/**`）。
- Go server 负责静态资源分发（仍以 `/assets/*` 命名空间提供），避免运行时依赖额外 volume/旁路静态服务器。
- `apps/web` 的 `package.json` + lockfile 作为 UI 依赖版本的 SSOT（见 `DEV-PLAN-011`）。
- 门禁与入口：UI build 的唯一入口为 `make css`（SSOT），CI Gate-1 命中 `ui` 触发器时必须执行并由 `assert-clean` 阻断生成物漂移（见 `DEV-PLAN-012`）。

#### 4.5.1 Shell 产物映射（冻结）
- 静态资源 URL 前缀固定为 `/assets/astro/`（由 Go `/assets/*` 分发，确保路径稳定且不依赖 `/app` 相对路径）。
- 复制规则（冻结，作为 CI 可验证契约）：
  - `apps/web/dist/index.html` → `internal/server/assets/astro/app.html`
  - `apps/web/dist/**`（除 `index.html` 外）→ `internal/server/assets/astro/**`（保持相对路径）

#### 4.5.2 占位符注入（冻结）
- Shell 模板唯一占位符：`__BB_AS_OF__`（表示当前 URL 的 `as_of`）。
- Go `/app` handler 只允许做“最小占位符注入”，不得在 handler 内重写页面结构：
  - 先按 §3.3 执行 `as_of` 的缺省/校验口径（302 补齐；非法则 400）。
  - 将 `internal/server/assets/astro/app.html` 中所有 `__BB_AS_OF__` 以 URL query 语义替换为当前 `as_of`（例如用于 `hx-get="/ui/nav?as_of=__BB_AS_OF__"`）。
- 失败行为（fail-fast）：若模板缺失 `__BB_AS_OF__` 或替换后仍残留该 token，返回 500 并显式报错；禁止回退到 Go 拼接壳/双壳并存。

## 5. 页面与组件规范（只覆盖 026-031 模块）

### 5.1 通用页面框架
- 主内容区统一：`PageHeader`（标题+说明+操作按钮） + `AsOfBar`（如该页需要） + `ContentPanel`（列表/表单/详情）。
- 所有列表页：
  - 支持 `as_of`（必带；缺省会被重定向补齐）与 `q`（搜索，可选）
  - 列表行点击打开右侧/下方详情面板（HTMX swap），减少全页跳转

### 5.2 任职记录（Assignments）UI 合同落地
对齐 031 的强约束：
- 列表/时间线**只显示 `effective_date`**（生效日期），不显示 `end_date`。
- 时间线分段用 “事件/动作”标识（CREATE/UPDATE/TRANSFER/TERMINATE…），但不引入 `effseq`。
- 任何“删除某日变更”类操作不允许直接操作 versions；必须走事件入口（One Door Policy 对齐 026-029/031）。

### 5.3 Job Catalog / Positions / OrgUnit 的一致性体验
统一约定：
- 所有有效期类对象：同样用 as-of 控制当前视图，不在 UI 混入 end_date。
- 所有“选项下拉”（组织节点、职位、职位模板等）：统一使用 HTMX options endpoint + 输入搜索（避免把大字典塞到前端）。

## 6. 落地步骤（可执行）

### Phase 0：先打通 AHA 基础链路（最小可运行）
1. [X] 在 `apps/web` 初始化 Astro 工程，落盘 `package.json` 与 lockfile，并按 `DEV-PLAN-011` 冻结 Node/pnpm/Astro 版本口径。（009M6 PR-1）
2. [X] 定义 App Shell（Astro）：包含 §4.4 的四个固定挂载点，并以 HTMX 拉取 nav/topbar/flash 的方式装配动态上下文。（009M6 PR-1）
3. [ ] 按 §4.5 落地 Astro build 产物复制到 `internal/server/assets/astro/**` 的 pipeline，并由 Go server 在 `/assets/*` 下提供静态资源。
4. [ ] 将 `/app` 的 Shell 渲染切换为 Astro 产物（不在 Go handler 内拼接整页 HTML），但继续保留 `/ui/nav`、`/ui/topbar`、`/ui/flash` 的服务端权威渲染。
5. [ ] 后端提供最小 UI partial：`/ui/nav`、`/ui/topbar`、`/ui/flash` 与一个占位内容页（例如 `/app/home`），验证：
   - 静态资源可用（Astro build 产物）
   - HTMX swap 正常
   - Alpine 初始化不与 HTMX 冲突
6. [ ] 更新路由 allowlist 以覆盖新增的 UI 路由（尤其 `/org/job-catalog/*`）并通过路由门禁（入口：`make check routing`）。

### Phase 1：按模块逐个接入内容（不改 Shell）
6. [ ] 按 016 的 4 模块顺序接入页面与二级菜单入口（未实现模块不出现在导航中）：
   - OrgUnit（`/org/nodes`）
   - JobCatalog（`/org/job-catalog/*`，见 §3.2）
   - Staffing（`/org/positions`、`/org/assignments`）
   - Person（`/person/persons`，并通过 HTMX 组合 Staffing 的任职时间线）
7. [ ] 导航 SSOT：二级菜单由服务端单点维护并渲染 `/ui/nav`（Astro 不维护第二份导航规则；避免出现“第二套权威表达”）。

### Phase 2：硬化与验收（对齐 026-031 契约）
8. [ ] 全局 as-of 透传严格执行 Query 参数口径（见 §3.3）；`/ui/nav`、`/ui/topbar`、模块链接一律保留 `as_of`。
9. [ ] 任职记录页严格执行：只展示 `effective_date`，不展示 `end_date`（对齐 031），但底层有效期仍为 `daterange [start,end)`。
10. [ ] E2E：为“导航层级 + as-of 参数透传 + 任职仅展示 effective_date”补齐可视化验收用例（入口与门禁以 `DEV-PLAN-012` 为准）。

### 回退策略（Greenfield 口径）
11. [ ] 回退以“发布版本回滚”为唯一手段：上一版构建产物/上一版镜像；不在运行时引入旧页面并存或 feature flag 分流。

## 7. 验收标准（Acceptance Criteria）
- [ ] 左侧导航布局与现有一致；一级仅 4 模块；二级菜单与 §3.2 完全一致。
- [ ] 任职记录页面不展示 `end_date`，只展示 `effective_date`；且 `as_of` 作为 query 参数在页面间保持一致透传（URL 可分享可复现）。
- [ ] `as_of` 缺省时会被 302 补齐（`/app` 与任意模块页面均满足），`as_of` 非法时返回 400 且可被 UI 展示。
- [ ] 不引入 SPA；所有业务交互仍可用 HTMX 解释（5 分钟可复述：入口 → 请求 → swap → 失败提示）。
- [ ] 权限与本地化不在前端复制实现：导航与操作按钮可随用户权限变化。
- [ ] Astro build 产物被纳入 `go:embed`，发布产物中不依赖额外静态目录挂载即可访问 Shell 与静态资源。
- [ ] 路由门禁通过：新增的 UI 路由已登记 allowlist 且 `make check routing` 通过。
- [ ] 文档与门禁：本计划加入 Doc Map，且 `make check doc` 通过。

## 8. Simple > Easy Review（DEV-PLAN-003）

### 8.1 边界
- Astro 只负责 Shell/组件编译；业务内容仍由服务端 HTML partial 提供，避免引入“第二套前端权威表达”。

### 8.2 不变量
- 有效期语义：Valid Time=DATE，`daterange [start,end)`；Assignments 仅显示 `effective_date`（不展示 end_date）。

### 8.3 可解释性
- 主流程：加载 Shell → HTMX 拉取 nav/topbar → 用户点击二级菜单 → HTMX 拉取模块内容 → swap 更新。
- 失败路径：统一走 `/ui/flash` 或现有错误反馈组件，不散落在多处 JS 分支。
