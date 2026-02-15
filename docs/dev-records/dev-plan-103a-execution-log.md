# DEV-PLAN-103A 执行日志

**状态**: 未开始（2026-02-15 13:46 UTC）

**关联文档**:
- `docs/dev-plans/103a-dev-plan-103-closure-p3-p6-apps-web-rename.md`
- `docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`

## 1) 旧 UI → MUI 映射表（事实源盘点 + 状态）

> 事实源（按优先级）：  
> 1) `internal/server/handler.go`（路由注册）  
> 2) `config/routing/allowlist.yaml`（route_class / 可达性门禁）  
> 3) `apps/**/src/router/index.tsx`（MUI SPA 路由）  
> 4) `apps/**/src/navigation/config.tsx`（导航入口 + permissionKey）  
> 5) `e2e/tests/**`（外部可见链路证据）

表格字段建议（可按实际调整）：旧路径/模式、旧状态（已移除/不可达/仍有死代码）、旧时间上下文（A/B/C）、新 MUI path（`/app` 内路由）、新 API（如有）、permissionKey、备注（证据/清理点）。

| 旧 UI 路由（或能力入口） | 旧状态 | 时间上下文（A/B/C） | 新 MUI path（/app 内） | 新 API（如有） | permissionKey | 备注/证据 |
| --- | --- | --- | --- | --- | --- | --- |
| `/login`（HTML 登录页） |  |  | `/login`（SPA，浏览器 URL `/app/login`） | `POST /iam/api/sessions` | n/a |  |
| `/ui/nav` |  | C | n/a | n/a | n/a |  |
| `/ui/topbar` |  | C | n/a | n/a | n/a |  |
| `/ui/flash` |  | C | n/a | n/a | n/a |  |
| `/lang/en` |  | C | n/a | n/a | n/a |  |
| `/lang/zh` |  | C | n/a | n/a | n/a |  |
| `/org/nodes*`（树/详情/搜索） |  | B | `/org/units`、`/org/units/:orgCode` | `/org/api/org-units*` | `orgunit.read` |  |
| `/org/snapshot` |  | A | （按实际填写：若无对应入口填 n/a） | （按实际填写） | （按实际填写） |  |
| `/org/setid`（旧 HTML） |  |  | `/org/setid` | `/org/api/setids`、`/org/api/setid-bindings` | `orgunit.read` |  |
| `/org/job-catalog`（旧 HTML） |  |  | `/jobcatalog` | `/jobcatalog/api/catalog`、`/jobcatalog/api/catalog/actions`、`/org/api/owned-scope-packages` | `jobcatalog.read` |  |
| `/org/positions`（旧 HTML） |  |  | `/staffing/positions` | `/org/api/positions`、`/org/api/positions:options` | `staffing.positions.read` |  |
| `/org/assignments`（旧 HTML） |  |  | `/staffing/assignments` | `/org/api/assignments` | `staffing.assignments.read` |  |
| `/person/persons`（旧 HTML） |  |  | `/person/persons` | `/person/api/persons` | `person.read` |  |

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
