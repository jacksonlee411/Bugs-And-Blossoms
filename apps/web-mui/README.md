# apps/web-mui

`DEV-PLAN-091` 的前端基座工程（React + MUI Core + MUI X）。

## 快速开始

```bash
pnpm install
pnpm dev
```

## 常用命令

```bash
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm check
```

## 版本与许可决策（P091）

| 组件 | 版本 | 许可策略 |
| --- | --- | --- |
| React | 19.2.4 | OSS |
| MUI Core (`@mui/material`) | 7.3.7 | MIT |
| MUI X Data Grid (`@mui/x-data-grid`) | 8.27.0 | MIT（Community） |
| MUI X Tree View (`@mui/x-tree-view`) | 8.27.0 | MIT（Community） |
| MUI X Pro/Premium | 未启用 | 待法务/采购确认后按需接入 |

> 本阶段先以 Community 能力完成基座搭建，Pro/Premium 作为后续里程碑决策点。

## 当前交付

- `AppShell / PageHeader / FilterBar / DataGridPage / DetailPanel` 平台组件骨架。
- 统一 API 客户端（鉴权 header、租户 header、request-id、基础重试、错误归一）。
- Foundation Demo 页面：树 + 表 + 详情侧栏。

## P092 壳层能力

- 统一导航配置（`icon/route/permission/order/keywords`）。
- 权限菜单联动：无权限菜单默认隐藏；开发环境支持 `VITE_NAV_DEBUG=true` 显示隐藏项。
- 全局搜索入口（`Ctrl/Cmd + K`）：支持导航项与常用页面搜索，预留多 provider 扩展。
- 主题与语言切换：`light/dark`、`en/zh`，状态持久化到 `localStorage`。
- 埋点基线：`nav_click/filter_submit/detail_open/bulk_action` 字段模型，开发环境可在 `window.__WEB_MUI_UI_EVENTS__` 观察。

## 环境变量

```bash
VITE_API_BASE_URL=http://localhost:8080
VITE_API_TIMEOUT_MS=10000
VITE_TENANT_ID=demo-tenant
VITE_PERMISSIONS=*
VITE_NAV_DEBUG=false
```
