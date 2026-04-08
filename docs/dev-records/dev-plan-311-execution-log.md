# DEV-PLAN-311 执行日志：View As Of 页面改造矩阵与样板收尾

**状态**: 已完成（2026-04-09 CST）

## 1. 执行范围

1. [X] 将 `DEV-PLAN-310` 的“product/UI current-by-default + service explicit-by-contract”原则下沉为页面级执行矩阵。
2. [X] 以 `OrgUnitDetailsPage` 作为首个样板，验证“单历史锚点 + 读写解耦 + 写后不跳日”模式可执行。
3. [X] 将后续页面与工具态能力拆分为 `312/313/314/315/316/317` 子计划承接，并完成闭环。
4. [X] 用统一回归矩阵确认该父计划不再只是设计文档，而是已经有稳定实现与证据。

## 2. 子计划收尾情况

1. [X] `DEV-PLAN-312`
   - 详情页单历史锚点与 A 类页面读写解耦
   - 文档状态已回写完成
2. [X] `DEV-PLAN-313`
   - 后端显式日期合同、无 fallback、统一错误语义
   - 文档状态已回写为已实施
3. [X] `DEV-PLAN-314`
   - `AssignmentsPage` / `PositionsPage` / `JobCatalogPage` / `DictConfigsPage`
   - 已完成 `default current + 读写解耦`
4. [X] `DEV-PLAN-315`
   - `readViewState` / `readNavigation`
   - 前端反回流门禁与 allowlist
5. [X] `DEV-PLAN-316`
   - 工具态显式时间文案与边界收口
   - `SetIDExplainPanel` / `DictConfigsPage release` / `SetIDGovernancePage`
6. [X] `DEV-PLAN-317`
   - 跨页面回归矩阵与验收基线
   - `35` 条前端回归测试与门禁脚本纳入统一证据

## 3. 父计划层面的完成判定

1. [X] 页面矩阵已稳定成立：
   - 业务浏览页：`default current`
   - 工具态页面/子区：保留显式时间，但只作为任务参数
2. [X] `OrgUnitDetailsPage` 已完成样板职责：
   - 旧 `as_of` 深链接归一化为单一 `effective_date`
   - 写态日期不会被历史查看覆盖
   - 写成功后不自动跳日
3. [X] 前端共享 helper 只保留最小纯函数边界，没有演化为新时间框架或全局 store。
4. [X] 页面级收口不再依赖“是否继续一刀切显式 `as_of`”的重复讨论，而是直接沿用父计划矩阵。

## 4. 关键证据

### 4.1 样板与页面级测试

1. [X] [OrgUnitDetailsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.test.tsx)
2. [X] [OrgUnitsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.test.tsx)
3. [X] [OrgUnitFieldConfigsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitFieldConfigsPage.test.tsx)
4. [X] [StaffingViewAsOfPages.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/StaffingViewAsOfPages.test.tsx)
5. [X] [JobCatalogPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/jobcatalog/JobCatalogPage.test.tsx)
6. [X] [DictConfigsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/dicts/DictConfigsPage.test.tsx)
7. [X] [SetIDGovernancePage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/SetIDGovernancePage.test.tsx)
8. [X] [SetIDExplainPanel.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/components/SetIDExplainPanel.test.tsx)

### 4.2 直接测试与门禁

1. [X] [readViewState.test.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/readViewState.test.ts)
2. [X] [orgReadNavigation.test.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/orgReadNavigation.test.ts)
3. [X] `./scripts/ci/check-as-of-explicit.sh`
4. [X] `./scripts/ci/check-view-as-of-frontend.sh`

### 4.3 统一验收记录

1. [X] [dev-plan-317-execution-log.md](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-records/dev-plan-317-execution-log.md)
   - 作为 `311` 父计划的统一回归与验收证据承接入口

## 5. 结论

1. [X] `DEV-PLAN-311` 已完成其父计划职责：矩阵、样板方向、减法原则、子计划拆分与验收口径均已落地。
2. [X] 当前不再存在需要继续挂在 `311` 父计划下的明显功能遗留项。
3. [X] 后续若新增命中该主题的页面，应直接复用 `311 + 317` 的矩阵与验收口径，而不是重开同类原则讨论。
