# DEV-PLAN-103A 执行日志

**状态**: 已完成（2026-02-15 20:52 UTC）

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
| `/login`（HTML 登录页） | 不可达（allowlist 不含；路由未注册），且无“兼容别名窗口”（tenant app 不提供 `/login` HTML） | C | `/login`（SPA，浏览器 URL `/app/login`） | `POST /iam/api/sessions` | n/a | allowlist：`config/routing/allowlist.yaml` 未列 `GET /login`；中间件：`internal/server/handler.go`（对非 `/app/**` UI path passthrough，不做 `/login` alias）；单测：`internal/server/tenancy_middleware_test.go` |
| `/ui/nav` | 已移除（无路由注册/无实现；allowlist 不含） | C | n/a | n/a | n/a | `internal/server/**` 未发现 `/ui/nav` 引用 |
| `/ui/topbar` | 已移除（无路由注册/无实现；allowlist 不含） | C | n/a | n/a | n/a | `internal/server/**` 未发现 `/ui/topbar` 引用 |
| `/ui/flash` | 已移除（无路由注册/无实现；allowlist 不含） | C | n/a | n/a | n/a | `internal/server/**` 未发现 `/ui/flash` 引用 |
| `/lang/en` | 已移除（无路由注册；无残留死代码/测试） | C | n/a | n/a | n/a | 门禁：`make check no-legacy` + `make preflight`（见下表） |
| `/lang/zh` | 已移除（无路由注册；无残留死代码/测试） | C | n/a | n/a | n/a | 门禁：`make check no-legacy` + `make preflight`（见下表） |
| `/org/nodes*`（树/详情/搜索） | 不可达（allowlist 不含；路由未注册），且旧 HTML/HTMX 交互链路已清理（仅保留 JSON API + store） | B | `/org/units`、`/org/units/:orgCode` | `/org/api/org-units*` | `orgunit.read` | 事实源：`internal/server/orgunit_nodes.go`（JSON API + store）；门禁：`make preflight`（E2E 通过） |
| `/org/snapshot` | 已移除（相关 store + option + 单测已删除） | A | n/a（当前 MUI 未提供对应入口） | n/a | n/a | 删除：`internal/server/orgunit_snapshot.go`、`internal/server/orgunit_snapshot_test.go` |
| `/org/setid`（旧 HTML） | 不可达（allowlist 不含；路由未注册），且旧 HTML/HTMX 交互链路已清理（仅保留 JSON API + MUI） | A | `/org/setid` | `/org/api/setids`、`/org/api/setid-bindings` | `orgunit.read` | 事实源：`apps/web/**`（MUI）；`internal/server/setid.go`（JSON API） |
| `/org/job-catalog`（旧 HTML） | 不可达（allowlist 不含；路由未注册），且旧 HTML 交互链路已清理（仅保留 JSON API + MUI） | A | `/jobcatalog` | `/jobcatalog/api/catalog`、`/jobcatalog/api/catalog/actions`、`/org/api/owned-scope-packages` | `jobcatalog.read` | 事实源：`apps/web/**`（MUI）；`internal/server/jobcatalog.go`（JSON API） |
| `/org/positions`（旧 HTML） | 不可达（allowlist 不含；路由未注册），且旧 HTML/HTMX 交互链路已清理（仅保留 JSON API + MUI） | A | `/staffing/positions` | `/org/api/positions`、`/org/api/positions:options` | `staffing.positions.read` | 事实源：`apps/web/**`（MUI）；`internal/server/staffing_handlers.go`（JSON API） |
| `/org/assignments`（旧 HTML） | 不可达（allowlist 不含；路由未注册），且旧 HTML 交互链路已清理（仅保留 JSON API + MUI） | A | `/staffing/assignments` | `/org/api/assignments` | `staffing.assignments.read` | 事实源：`apps/web/**`（MUI）；`internal/server/staffing_handlers.go`（JSON API） |
| `/person/persons`（旧 HTML） | 不可达（allowlist 不含；路由未注册），且旧 HTML handler 已移除 | A | `/person/persons` | `/person/api/persons` | `person.read` | `internal/server/person.go` 已移除旧 HTML handler；`internal/server/person_test.go` 同步收口 |

## 2) 关键命令执行记录（以 SSOT 入口为准）

> 说明：命令入口以 `AGENTS.md` 为 SSOT；此处仅记录“实际执行的时间、结果与关键输出摘要”。

| 时间（UTC） | 命令 | 结果 | 备注 |
| --- | --- | --- | --- |
| 2026-02-15 17:19 UTC | `make css` | ✅ | 产物同步到 `internal/server/assets/web/**` |
| 2026-02-15 17:26 UTC | `make preflight` | ✅ | 含 E2E（Playwright 7 tests）通过 |
| 2026-02-15 20:41 UTC | `make preflight` | ✅ | `make check no-legacy` ✅；`make test`（100% coverage）✅；E2E（Playwright 7 tests）✅ |
| 2026-02-15 20:49 UTC | `make check doc` | ✅ | 文档门禁通过（回写 `DEV-PLAN-103/103A`） |
| 2026-02-15 20:52 UTC | `make preflight` | ✅ | 合并前再次对齐 CI（含 E2E 7 tests） |

## 3) 变更清单（PR 维度）

### PR-103A-1（P3 收尾）
- 变更点：
  - 移除 Person 页面“伪需求参数”输入（As-of ignored），避免与 `DEV-PLAN-102` 时间上下文口径冲突。
- 验证点：
  - Person 页面移除 “As-of (ignored)” 输入：`apps/web/src/pages/person/PersonsPage.tsx`
  - UI 产物更新：`internal/server/assets/web/**`
  - `make css`：通过（见上表）

### PR-103A-2（旧 UI 残留清理）
- 变更点：
  - 中间件对非 `/app/**` UI path passthrough，避免 `/login` 等旧 URL 出现“兼容别名窗口”。
  - 清理旧 UI 残留死代码/测试：移除旧 HTML/HTMX 渲染辅助、OrgUnit Snapshot 残留、以及若干旧 UI 相关测试（保持 JSON API 与 `/app/**` 入口不变）。
- 验证点：
  - Go 单测覆盖 `/login` 不提供 HTML/不引入 alias：`internal/server/tenancy_middleware_test.go`、`internal/server/handler_test.go`
  - `make preflight`：通过（见上表）

### PR-103A-3（P6 改名 apps/web-mui → apps/web）
- 变更点：
  - 前端工程目录统一为 `apps/web/`；更新构建脚本与 CI 触发器引用。
- 验证点：
  - `make css`、`make preflight`：通过（见上表）
