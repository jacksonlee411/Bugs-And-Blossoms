# DEV-PLAN-091：MUI X 升级子计划 P0（基座准备与许可落地）

**状态**: 已完成（2026-02-12 07:07 UTC）

> 本计划承接 `DEV-PLAN-090` 的 §5.1（Phase 0），聚焦“先把地基打稳”：版本、许可、工程骨架、平台组件、API 基础能力。

## 1. 子计划范围

- 对应 `DEV-PLAN-090` 步骤：1~4。
- 时间窗口：1~2 周。
- 输出形态：可运行的 React + MUI X 基座工程（不含业务页面迁移）。

## 2. 核心目标（DoD）

- [x] 冻结 React/MUI/MUI X 版本策略与许可策略（含 Pro/Premium 边界说明）。
- [x] 新前端工程可独立启动、构建、测试，并纳入统一门禁入口。
- [x] 平台组件层最小集合可用：`AppShell / PageHeader / FilterBar / DataGridPage / DetailPanel`。
- [x] 统一 API 客户端能力落地：鉴权注入、错误归一、重试策略、请求追踪 ID 透传。

## 3. 非目标

- 不迁移任何业务模块页面。
- 不在本阶段追求视觉细节对标 Workday。
- 不在本阶段引入 Premium 专属功能（仅完成能力边界与许可决策）。

## 4. 实施步骤

1. [x] 版本与许可冻结
   - 输出《版本与许可决策表》，并写入 `apps/web-mui/README.md`。
   - 许可边界冻结为：P091 仅启用 MUI X Community（MIT）；Pro/Premium 作为后续法务/采购决策点。

   | 组件 | 版本 | 许可策略 |
   | --- | --- | --- |
   | React | 19.2.4 | OSS |
   | MUI Core (`@mui/material`) | 7.3.7 | MIT |
   | MUI X Data Grid (`@mui/x-data-grid`) | 8.27.0 | MIT（Community） |
   | MUI X Tree View (`@mui/x-tree-view`) | 8.27.0 | MIT（Community） |
   | MUI X Date Pickers (`@mui/x-date-pickers`) | 8.27.0 | MIT（Community） |
   | MUI X Pro/Premium | 未启用 | 待法务/采购确认 |

2. [x] 工程骨架初始化
   - 初始化目录：`apps/web-mui`。
   - 建立脚本：`dev/build/test/lint/typecheck/check`。
   - 建立 `README.md`、`.env.example`、Vite/TypeScript/ESLint/Vitest 基础配置。

3. [x] 平台组件层落位
   - 实现组件骨架：`AppShell / PageHeader / FilterBar / DataGridPage / DetailPanel`。
   - 建立示例页面：`FoundationDemoPage`（树 + 表 + 详情侧栏）。
   - 组件约束（P091 冻结）：
     - `AppShell`：统一顶部栏 + 左侧导航 + 内容区布局。
     - `PageHeader`：页面标题/副标题/操作区统一结构。
     - `FilterBar`：筛选项容器，响应式横竖布局。
     - `DataGridPage`：统一 DataGrid 容器与空态。
     - `DetailPanel`：统一右侧抽屉详情面板。

4. [x] API 基础能力落位
   - 封装统一 HTTP 客户端：`src/api/httpClient.ts`。
   - 错误归一：`src/api/errors.ts`（状态码到稳定错误码映射）。
   - 请求上下文透传：`Authorization` / `X-Tenant-ID` / `X-Request-ID`。
   - 重试策略：默认 1 次，仅网络异常或 5xx 重试。

5. [x] 基础测试与质量门禁
   - 建立最小单元/组件测试：
     - `src/api/httpClient.test.ts`
     - `src/components/PageHeader.test.tsx`
   - 完成 `pnpm -C apps/web-mui check` 全链路通过（lint/typecheck/test/build）。

## 5. 验收标准

- [x] 本地可执行：安装依赖、启动开发环境、构建成功、测试通过。
- [x] 至少 1 个示例页面完整使用平台组件层渲染。
- [x] API 客户端具备统一错误处理，不允许页面各自散落 try/catch 分支。
- [x] 版本与许可文档评审通过，作为后续阶段唯一事实源。

## 6. 风险与缓解

- 风险：Pro/Premium 许可尚未最终确认，后续高级能力启用存在采购前置。  
  缓解：P091 已冻结 Community 基线，不阻塞 P092/P093 开发；在进入 Pro 依赖前设置 stopline。
- 风险：基座默认打包体积偏大（当前 Vite 输出存在 chunk size 警告）。  
  缓解：在 P092/P095 引入路由级代码分割与按需加载策略。

## 7. 执行记录（2026-02-12）

- 已执行并通过：
  - `pnpm -C apps/web-mui lint`
  - `pnpm -C apps/web-mui typecheck`
  - `pnpm -C apps/web-mui test`
  - `pnpm -C apps/web-mui build`
  - `pnpm -C apps/web-mui check`

## 8. 关联计划

- 总方案：`docs/dev-plans/090-mui-x-frontend-upgrade-plan.md`
- 质量门禁：`docs/dev-plans/012-ci-quality-gates.md`
- 路由治理：`docs/dev-plans/017-routing-strategy.md`
- i18n：`docs/dev-plans/020-i18n-en-zh-only.md`
