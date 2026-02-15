# DEV-PLAN-092：MUI X 升级子计划 P1（壳与导航迁移）

**状态**: 已完成（2026-02-12 07:25 UTC）

> 本计划承接 `DEV-PLAN-090` 的 §5.2（Phase 1），聚焦主框架壳、导航体系、全局入口与基础观测能力。

## 1. 子计划范围

- 对应 `DEV-PLAN-090` 步骤：5~7。
- 时间窗口：1~2 周。
- 输出形态：可用于承载业务模块的统一 Shell 与导航体系。

## 2. 核心目标（DoD）

- [x] 主导航、顶部栏、全局搜索入口完成迁移。
- [x] 权限菜单裁剪规则落地，避免“无权限入口可见”。
- [x] 主题与语言切换在新壳内可用（en/zh）。
- [x] 页面级埋点基线建立，覆盖核心交互事件。

## 3. 非目标

- 不迁移业务详情页逻辑。
- 不在本阶段实现复杂报表或分析图表。
- 不在本阶段引入页面级个性化布局配置。

## 4. 实施步骤

1. [x] App Shell 迁移
   - 完成统一布局：侧边导航、顶部栏、内容区。
   - 冻结导航配置模型：`icon/route/permission/order/keywords`（`src/navigation/config.tsx`）。
2. [x] 权限与菜单联动
   - 菜单渲染策略落地：无权限默认隐藏。
   - 开发环境调试模式：`VITE_NAV_DEBUG=true` 时显示“因权限隐藏”的禁用菜单项。
3. [x] 全局搜索入口
   - 支持“导航项 + 常用页面”搜索（`Ctrl/Cmd + K`）。
   - 预留 provider 扩展接口（`src/search/globalSearch.ts`），后续可接业务搜索 API。
4. [x] 主题与 i18n
   - 接入主题切换（浅色/深色）并持久化。
   - 接入 en/zh 壳层文案切换并持久化。
5. [x] 埋点体系基线
   - 冻结事件词表：`nav_click/filter_submit/detail_open/bulk_action`。
   - 字段标准落位：`tenant/module/page/action/result/latencyMs` + `metadata`（`src/observability/tracker.ts`）。

## 5. 验收标准

- [x] 任意已接入页面均在统一 Shell 下运行。
- [x] 权限用户视角下导航可见项正确，越权入口不可见（同时提供 NoAccess 页面兜底）。
- [x] 语言与主题切换在刷新后行为一致。
- [x] 埋点数据可在开发环境观测并可追溯到页面行为（`window.__WEB_MUI_UI_EVENTS__`）。

## 6. 风险与缓解

- 风险：导航与权限模型耦合过深，后续扩展困难。  
  缓解：导航配置与权限判断拆层，页面级通过 `RequirePermission` 做最小守卫。
- 风险：埋点字段口径不一致，后续无法分析。  
  缓解：已冻结事件词表与字段模型，新增事件需按同一结构扩展。
- 风险：壳层首包偏大。  
  缓解：P093/P095 引入路由级代码分割并按模块懒加载。

## 7. 执行记录（2026-02-12）

- 已执行并通过：
  - `pnpm -C apps/web lint`
  - `pnpm -C apps/web typecheck`
  - `pnpm -C apps/web test`
  - `pnpm -C apps/web build`
  - `pnpm -C apps/web check`

## 8. 关联计划

- 总方案：`docs/dev-plans/090-mui-x-frontend-upgrade-plan.md`
- i18n：`docs/dev-plans/020-i18n-en-zh-only.md`
- 路由治理：`docs/dev-plans/017-routing-strategy.md`
