# DEV-PLAN-170：Org 详情页 UI 外观对齐 Capability Key（仅页面壳层改造）

**状态**: 已实施（2026-02-24 10:57 UTC）

## 1. 背景与问题
- 当前 Org 组织详情页（`OrgUnitDetailsPage`）与 SetID Capability Governance 页面在信息编排与视觉层级上差异较大，用户跨页面切换时心智负担偏高。
- Org 写入链路与能力判定契约已稳定（`create-field-decisions` / `write-capabilities` / `policy_version` / `writeOrgUnit(intent+patch)`），本计划不触碰这些行为。
- 范围收敛：本次仅对齐“组织详情页面壳层外观”，**不改任何弹窗外观**。

## 2. 目标与非目标
### 2.1 目标
- [x] 将 Org 组织详情页改造成与 Capability Key 页面一致的治理台式信息结构（统一顶部上下文区、分区内容区、底部辅助信息区）。
- [x] 保持现有业务行为与门禁语义不变（fail-closed、字段可编辑判定、deny reasons、policy_version 校验）。
- [x] 提升详情页可读性与一致性，降低学习与排障成本。

### 2.2 非目标
- 不变更任何后端 API 契约、路由映射、capability_key 语义。
- 不新增写入意图，不改 Org 业务规则，不引入 legacy 双链路。
- **不改动新建弹窗与详情页写入弹窗的 UI 外观与交互结构**。

## 3. 改造范围（UI Shell Only）
- `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
  - 仅调整“详情页主视图（非弹窗）”的布局分区、信息层级、视觉容器。
- 允许最小范围复用/新增页面壳层样式组件（仅用于详情页）。
- 明确排除：
  - `apps/web/src/pages/org/OrgUnitsPage.tsx` 新建组织弹窗；
  - `apps/web/src/pages/org/OrgUnitDetailsPage.tsx` 内 add_version / insert_version / correct / delete 等写入弹窗。

## 4. 不变量（冻结）
1. [x] 不修改详情页写入相关调用链：`getOrgUnitWriteCapabilities`、`writeOrgUnit`。
   - 说明：`getOrgUnitCreateFieldDecisions` 属于新建链路，不在本计划范围内（本计划不改新建弹窗）。
2. [x] 不修改提交 payload 结构：`intent/org_code/effective_date/target_effective_date/policy_version/request_id/patch`。
3. [x] 保留全部 fail-closed 行为与能力判定语义（含 deny reasons 展示口径）。
4. [x] 保留现有弹窗入口、弹窗行为、字段校验与按钮语义（本计划不触碰弹窗实现）。

## 5. 视觉方案（对齐口径）
### 5.1 页面结构层
- 统一采用“三段式页面壳层”：
  1) 顶部上下文区（组织标识、as_of、状态与能力摘要）；
  2) 中部详情区（基础信息 + 扩展字段分组）；
  3) 底部辅助区（审计/提示/帮助信息）。

### 5.2 信息优先级
- 统一优先级：错误 > 拒绝原因 > 加载信息 > 帮助提示。
- 将分散提示收敛到统一容器，避免页面内提示跳读。

### 5.3 组件语义
- 按 Capability Key 页面风格统一容器、标题层级、间距与分隔。
- 仅调整详情页主视图样式，不迁移弹窗内部表单结构。

## 6. 实施步骤
1. [x] 冻结详情页壳层线框与分区清单（不含弹窗）。
2. [x] 改造 `OrgUnitDetailsPage` 主视图区布局与视觉容器。
3. [x] 回归详情页四类状态：加载、拒绝、可读、提交后刷新展示。
4. [x] 核验弹窗零改动（外观/结构/行为保持现状）。
5. [x] 完成门禁与证据记录（见 `docs/dev-records/dev-plan-170-execution-log.md`）。

## 7. 验收标准
- [x] 组织详情页视觉结构与 Capability Key 页面对齐，且不改变业务行为。
- [x] 详情页提示信息完整、优先级清晰、无信息缺失。
- [x] 详情页相关写入弹窗外观与行为保持不变。
- [x] 弹窗回归用例通过（至少）：`e2e/tests/tp060-02-orgunit-record-wizard.spec.js`、`e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`。
- [x] 关键门禁通过：`make check lint && make test && make check routing && make check capability-route-map && make check error-message`。

## 8. 风险与缓解
- **风险 1：页面改造时误改弹窗逻辑**  
  缓解：代码改动限制在详情页主视图容器，弹窗组件仅做引用不重构。
- **风险 2：视觉统一后页面信息拥挤**  
  缓解：按优先级分区，限制同屏 Alert 数量。
- **风险 3：范围回涨到“全表单改造”**  
  缓解：以本计划范围为准，弹窗改造另立计划。

## 9. 关联文档
- `docs/dev-records/dev-plan-170-execution-log.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/161a-setid-capability-registry-editable-and-maintainable.md`
- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/002-ui-design-guidelines.md`
