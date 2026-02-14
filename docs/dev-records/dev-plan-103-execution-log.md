# DEV-PLAN-103 执行日志

**状态**: 实施中（2026-02-14）

**关联文档**:
- `docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`

## 路由盘点（旧 UI → MUI / API）

> 事实源：`internal/server/handler.go`、`config/routing/allowlist.yaml`、`internal/server/authz_middleware.go`、`e2e/tests/**`

### 旧 UI（计划删除）
- `/login`（HTML 登录页）
- `/ui/nav`、`/ui/topbar`、`/ui/flash`（HTMX 装配端点）
- `/lang/en`、`/lang/zh`（服务端语言 cookie）
- `/org/nodes*`（OrgUnit 旧树 + 详情 + 搜索）
- `/org/setid`（SetID 旧 HTML 页面）
- `/org/job-catalog`（JobCatalog 旧 HTML 页面）
- `/org/positions`、`/org/assignments`（Staffing 旧 HTML 页面）
- `/person/persons`（Person 旧 HTML 页面）

### 新入口（保留/强化）
- UI：`/app/**`（MUI SPA，含 `/app/login`）
- AuthN：`POST /iam/api/sessions`（JSON 登录，设置 `sid` cookie）
- API：`/org/api/**`、`/person/api/**`、`/jobcatalog/api/**`（逐步补齐，作为 UI 契约）

## 已完成事项
- 2026-02-14：建立执行日志；落地 UI build 门禁收敛（`make css` → `scripts/ui/build-web.sh`，Vite `base=/assets/web/`，服务端 `webMUIIndexPath=assets/web/index.html`）。

## 进行中事项
- 2026-02-14：迁移 E2E 与后端路由/allowlist/authz，移除 Astro/HTMX 运行路径并用 MUI 页面替代旧 HTML 页面。

