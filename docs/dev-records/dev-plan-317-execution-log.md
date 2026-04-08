# DEV-PLAN-317 执行日志：View As Of 页面时间语义回归与验收

**状态**: 已完成（2026-04-09 CST）

## 1. 执行范围

1. [X] 汇总 `DEV-PLAN-312/314/315/316` 已落地页面、工具态组件与门禁，形成统一回归矩阵。
2. [X] 以“最小直接测试 / 页面级交互测试 / 门禁脚本”三层方式固化专项验收责任。
3. [X] 补齐 `DEV-PLAN-316` 的工具态时间语义回归样板，并把结果纳入 `317` 验收基线。
4. [X] 输出一份可直接复用的专项完成判定清单，供 `DEV-PLAN-311` 收尾使用。

## 2. 回归矩阵

### 2.1 最小直接测试

1. [X] [readViewState.test.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/readViewState.test.ts)
   - current/history 解析
   - `requestedAsOf` 收敛
   - trim / omit-empty 基线
2. [X] [orgReadNavigation.test.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/orgReadNavigation.test.ts)
   - URL 搜索参数构造
   - current 模式下 `as_of` 省略
3. [X] [SetIDExplainPanel.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/components/SetIDExplainPanel.test.tsx)
   - 工具态时间文案已从 `as_of` 收口
   - `initialAsOf` 仍会正确透传到 explain 请求
   - 宿主可显式传入任务态 label/hint

### 2.2 页面级交互测试

1. [X] [OrgUnitsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.test.tsx)
   - `default current`
   - history 显式写入 URL `as_of`
2. [X] [OrgUnitDetailsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.test.tsx)
   - 单历史锚点
   - 写态日期 sticky
   - 写成功后不自动跳日
3. [X] [OrgUnitFieldConfigsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitFieldConfigsPage.test.tsx)
   - current 模式不显式显示 `as_of`
   - current/history 跳转 SetID Registry 时 query 行为正确
4. [X] [StaffingViewAsOfPages.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/staffing/StaffingViewAsOfPages.test.tsx)
   - `AssignmentsPage` history 不覆盖 `effective_date`
   - `PositionsPage` history 不覆盖 `effective_date`
5. [X] [JobCatalogPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/jobcatalog/JobCatalogPage.test.tsx)
   - `default current`
   - create dialog 不继承 history `as_of`
6. [X] [DictConfigsPage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/dicts/DictConfigsPage.test.tsx)
   - `default current`
   - history 模式 URL 传递
   - release 区“发布时点”不回流浏览态
7. [X] [SetIDGovernancePage.test.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/SetIDGovernancePage.test.tsx)
   - registry 默认日期不受浏览态 history 串线影响
   - explain 子区携带任务态时间说明
   - registry 相关日期标签已收口

### 2.3 门禁与显式日期合同

1. [X] `./scripts/ci/check-as-of-explicit.sh`
   - 禁止 070/071 runtime scope 内隐式 today fallback
   - 联动执行 `check-view-as-of-frontend.sh`
2. [X] `./scripts/ci/check-view-as-of-frontend.sh`
   - 阻断 page-local date fallback 与读写日期串线
   - 工具态 allowlist 仅允许经 `DEV-PLAN-316` 登记的对象
3. [X] 当前 allowlist 事实源： [view-as-of-frontend-allowlist.txt](/home/lee/Projects/Bugs-And-Blossoms/scripts/ci/view-as-of-frontend-allowlist.txt)
   - 仅登记 [SetIDExplainPanel.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/components/SetIDExplainPanel.tsx)
   - 注释已收敛为“允许保留工具态显式时间能力，但必须使用任务态文案，且不得回流宿主页浏览/写态”

## 3. 317 对应的完成判定

1. [X] `default current` 已被 `OrgUnitsPage`、`JobCatalogPage`、`DictConfigsPage`、`AssignmentsPage`、`PositionsPage` 等样本覆盖。
2. [X] “history 不改写写态”已被 `OrgUnitDetailsPage`、`AssignmentsPage`、`PositionsPage`、`JobCatalogPage`、`SetIDGovernancePage` 覆盖。
3. [X] “写成功后不跳日”已被 `OrgUnitDetailsPage` 样板覆盖，并继续作为 `312` 代表性行为存在。
4. [X] “工具态显式时间不回流宿主”已被 `SetIDExplainPanel`、`DictConfigsPage release`、`SetIDGovernancePage explain/registry` 覆盖。
5. [X] `315` 的 helper 与门禁不再悬空，而是被直接测试与脚本验证共同承接。

## 4. 执行命令与结果

1. [X] 前端回归测试

```bash
pnpm vitest run \
  src/pages/org/readViewState.test.ts \
  src/pages/org/orgReadNavigation.test.ts \
  src/pages/org/OrgUnitsPage.test.tsx \
  src/pages/org/OrgUnitFieldConfigsPage.test.tsx \
  src/pages/org/OrgUnitDetailsPage.test.tsx \
  src/pages/staffing/StaffingViewAsOfPages.test.tsx \
  src/pages/jobcatalog/JobCatalogPage.test.tsx \
  src/pages/dicts/DictConfigsPage.test.tsx \
  src/pages/org/SetIDGovernancePage.test.tsx \
  src/components/SetIDExplainPanel.test.tsx
```

结果：

- [X] `10` 个测试文件通过
- [X] `35` 个测试用例通过

2. [X] 类型检查

```bash
pnpm typecheck
```

3. [X] 门禁脚本

```bash
./scripts/ci/check-as-of-explicit.sh
./scripts/ci/check-view-as-of-frontend.sh
```

结果：

- [X] `as-of-explicit` 通过
- [X] `view-as-of-frontend` 通过

## 5. 结论

1. [X] `DEV-PLAN-317` 已把 `312/314/315/316` 的关键行为收敛为统一可回归基线，而不再是分散在各页的局部说明。
2. [X] `DEV-PLAN-316` 新增的工具态时间语义样板已并入统一验收，避免后续再次回退成技术字段名 `as_of`。
3. [X] `DEV-PLAN-311` 视角下，View As Of 前端专项已经进入“可持续回归、可交接验收”的状态；后续新增页面若命中该主题，应复用本日志中的矩阵与判定清单，而不是重新发明口径。
