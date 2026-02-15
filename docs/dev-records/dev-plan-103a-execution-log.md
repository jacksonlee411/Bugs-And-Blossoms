# DEV-PLAN-103A 执行日志

**状态**: 未开始（2026-02-15 13:46 UTC）

**关联文档**:
- `docs/dev-plans/103a-dev-plan-103-closure-p3-p6-apps-web-rename.md`
- `docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`

## 1) 旧 UI → MUI 映射表（事实源盘点 + 状态）

> 事实源（按优先级；可按实际补齐）：  
> 1) `config/routing/allowlist.yaml`（route_class / 可达性门禁）  
> 2) `internal/server/**`（旧 HTML/HTMX handler 与 redirect/表单 action 的实际残留面）  
> 3) `internal/server/authz_middleware.go`（路由 → 权限判定；用于识别“路由已删但授权分支仍保活”）  
> 4) `apps/**/src/router/index.tsx`（MUI SPA 路由）  
> 5) `apps/**/src/navigation/config.tsx`（导航入口 + permissionKey）  
> 6) `e2e/tests/**`（外部可见链路证据）

表格字段建议（可按实际调整）：旧路径/模式、旧状态（已移除/不可达/仍有死代码）、旧时间上下文（A/B/C）、新 MUI path（`/app` 内路由）、新 API（如有）、permissionKey、备注（证据/清理点）。

| 旧 UI 路由（或能力入口） | 旧状态 | 时间上下文（A/B/C） | 新 MUI path（/app 内） | 新 API（如有） | permissionKey | 备注/证据 |
| --- | --- | --- | --- | --- | --- | --- |
| `/login`（HTML 登录页） | 不可达（allowlist 不含；路由未注册），但存在风险残留（中间件仍特殊放行、旧表单渲染仍在） | C | `/login`（SPA，浏览器 URL `/app/login`） | `POST /iam/api/sessions` | n/a | allowlist：`config/routing/allowlist.yaml` 未列 `GET /login`；中间件放行：`internal/server/handler.go`；旧表单：`internal/server/handler.go` |
| `/ui/nav` | 已移除（无路由注册/无实现；allowlist 不含） | C | n/a | n/a | n/a | `internal/server/**` 未发现 `/ui/nav` 引用 |
| `/ui/topbar` | 已移除（无路由注册/无实现；allowlist 不含） | C | n/a | n/a | n/a | `internal/server/**` 未发现 `/ui/topbar` 引用 |
| `/ui/flash` | 已移除（无路由注册/无实现；allowlist 不含） | C | n/a | n/a | n/a | `internal/server/**` 未发现 `/ui/flash` 引用 |
| `/lang/en` | 不可达（allowlist 不含；路由未注册），但仍有死代码（Topbar 仍输出链接 + 单测保活） | C | n/a | n/a | n/a | `internal/server/handler.go`、`internal/server/handler_utils_test.go` |
| `/lang/zh` | 不可达（allowlist 不含；路由未注册），但仍有死代码（Topbar 仍输出链接 + 单测保活） | C | n/a | n/a | n/a | `internal/server/handler.go`、`internal/server/handler_utils_test.go` |
| `/org/nodes*`（树/详情/搜索） | 不可达（allowlist 不含；路由未注册），但旧 HTML/HTMX handler + 大量单测仍在 | B | `/org/units`、`/org/units/:orgCode` | `/org/api/org-units*` | `orgunit.read` | 旧实现：`internal/server/orgunit_nodes.go`、`internal/server/orgunit_nodes_test.go` |
| `/org/snapshot` | 不可达（allowlist 不含；路由未注册），但旧 HTML handler + 单测仍在 | A | n/a（当前 MUI 未提供对应入口） | n/a | n/a | 旧实现：`internal/server/orgunit_snapshot.go`、`internal/server/orgunit_snapshot_test.go` |
| `/org/setid`（旧 HTML） | 不可达（allowlist 不含；路由未注册），但旧 HTML/HTMX handler + 单测仍在（且存在指向旧路由的 hx-get/redirect） | A | `/org/setid` | `/org/api/setids`、`/org/api/setid-bindings` | `orgunit.read` | 旧实现：`internal/server/setid.go`、`internal/server/setid_test.go` |
| `/org/job-catalog`（旧 HTML） | 不可达（allowlist 不含；路由未注册），但旧 HTML handler + 单测仍在（redirect/表单 action 指向旧路由） | A | `/jobcatalog` | `/jobcatalog/api/catalog`、`/jobcatalog/api/catalog/actions`、`/org/api/owned-scope-packages` | `jobcatalog.read` | 旧实现：`internal/server/jobcatalog.go`、`internal/server/jobcatalog_test.go` |
| `/org/positions`（旧 HTML） | 不可达（allowlist 不含；路由未注册），但旧 HTML/HTMX handler + 单测仍在 | A | `/staffing/positions` | `/org/api/positions`、`/org/api/positions:options` | `staffing.positions.read` | 旧实现：`internal/server/staffing_handlers.go`、`internal/server/staffing_test.go` |
| `/org/assignments`（旧 HTML） | 不可达（allowlist 不含；路由未注册），但旧 HTML handler + 单测仍在 | A | `/staffing/assignments` | `/org/api/assignments` | `staffing.assignments.read` | 旧实现：`internal/server/staffing_handlers.go`、`internal/server/staffing_test.go` |
| `/person/persons`（旧 HTML） | 不可达（allowlist 不含；路由未注册），但旧 HTML handler + 单测仍在 | A | `/person/persons` | `/person/api/persons` | `person.read` | 旧实现：`internal/server/person.go`、`internal/server/person_test.go` |

## 2) 关键命令执行记录（以 SSOT 入口为准）

> 说明：命令入口以 `AGENTS.md` 为 SSOT；此处仅记录“实际执行的时间、结果与关键输出摘要”。

| 时间（UTC） | 命令 | 结果 | 备注 |
| --- | --- | --- | --- |
|  | `make css` |  |  |
|  | `make preflight` |  |  |

## 3) 变更清单（PR 维度）

### PR-103A-1（P3 收尾）
- 变更点：
- 验证点：

### PR-103A-2（旧 UI 残留清理）
- 变更点：
- 验证点：

### PR-103A-3（P6 改名 apps/web-mui → apps/web）
- 变更点：
- 验证点：
