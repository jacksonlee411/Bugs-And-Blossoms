# apps/web

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
| MUI X Community (`@mui/x-data-grid` / `@mui/x-tree-view` / `@mui/x-date-pickers`) | 8.27.0 | MIT |
| MUI X Pro (`@mui/x-data-grid-pro` / `@mui/x-tree-view-pro` / `@mui/x-date-pickers-pro`) | 8.27.0 | 商业授权（已采购） |
| MUI X Premium (`@mui/x-data-grid-premium`) | 8.27.0 | 商业授权（已采购） |

> 已完成 Pro/Premium 采购与授权：后续设计与实现按“最佳组件优先”选型，不再受许可限制。

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

## P093 高价值模块迁移（进行中）

- `/org/units`：组织树 + 列表 + URL 联动筛选 + 详情侧栏。
- `/people`：复杂筛选 + 批量动作（二次确认）+ 详情侧栏。
- `/approvals`：审批列表状态流转 + 操作反馈 + 详情联动。

## P094 收口（已完成）

- 新增统一树面板组件 `TreePanel`，替换页面内重复树实现。
- 统一列表页 URL 参数协议：`q/status/page/size/sort/order`，并落地分页/排序可复现。
- i18n 支持简单变量插值（如 `{count}`），收敛通用反馈文案 key。
- `DataGridPage` 容器样式收口到主题色板，修复暗色模式背景不一致。
- `DataGridPage` 空态/加载态 Overlay 统一使用 `text_no_data / text_loading` 口径。
- 删除不可达占位页与无引用文案键，降低重复资产。

## 环境变量

```bash
VITE_API_BASE_URL=http://localhost:8080
VITE_API_TIMEOUT_MS=10000
VITE_TENANT_ID=demo-tenant
VITE_PERMISSIONS=*
VITE_NAV_DEBUG=false
```

## 权限键映射补充（070B1）

- 字典页面访问：`dict.admin`（对应后端 `iam.dicts/admin`）。
- 字典发布（预检/执行）：`dict.release.admin`（对应后端 `iam.dict_release/admin`）。
- `/dicts` 路由继续用 `dict.admin` 控制页面可见性；页面内“预检发布/执行发布”按钮再做 `dict.release.admin` 细粒度控制。
