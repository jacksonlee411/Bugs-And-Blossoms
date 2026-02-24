# DEV-PLAN-170A：Org 变更日志页 UI 外观对齐 Capability Key（仅页面壳层改造）

**状态**: 规划中（2026-02-24 11:27 UTC）

## 1. 背景与问题
- DEV-PLAN-170 已完成详情主视图（Profile）壳层对齐，但同页的变更日志视图（Audit）在信息层级、容器语义与间距节奏上仍与 Capability Key 治理台存在差异。
- 变更日志是审计与追溯高频入口，当前布局可读性与跨页面一致性不足，影响定位效率。
- 本计划只做 Audit 视图壳层改造，不触碰审计数据、筛选/定位、URL 同步与写入行为。

## 2. 目标与非目标
### 2.1 目标
- [ ] 将 `OrgUnitDetailsPage` 的变更日志视图改造成与 Capability Key 页面一致的治理台式外观（上下文区 + 双栏内容区 + 辅助区）。
- [ ] 保持审计行为契约不变（事件选择、URL 恢复、差异展示、raw payload 展示、加载更多）。
- [ ] 提升审计信息浏览与问题回放效率，降低切页心智成本。

### 2.2 非目标
- 不变更任何后端 API 契约、路由映射、capability_key 语义。
- 不新增/删除审计字段，不改 `before_snapshot/after_snapshot` 对比逻辑。
- 不改详情页 Profile 视图、不改任何写入弹窗（add/insert/correct/delete）。

## 3. 改造范围（UI Shell Only）
- `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
  - 仅改 `tab === 'audit'` 分支的布局分区、视觉容器、标题层级、间距与提示信息容器。
- 允许最小范围复用页面壳层样式结构（与 DEV-PLAN-170 统一）。
- 明确排除：
  - Audit 数据查询与组装逻辑；
  - Profile 分支与写入弹窗实现；
  - API/路由/服务端改动。

## 4. 不变量（冻结）
1. [ ] 不修改审计查询调用链：`listOrgUnitAudit`。
2. [ ] 不修改事件选择与 URL 同步契约：`tab=audit`、`audit_event_uuid`。
3. [ ] 保留 `data-testid` 语义：`org-audit-<event_uuid>`，以保持 E2E 稳定。
4. [ ] 保留差异表、撤销信息、raw payload 的字段语义与展示内容，不做删减。
5. [ ] 保留“加载更多”行为与分页上限递增逻辑。

## 5. 视觉方案（对齐口径）
### 5.1 页面结构层
- 统一采用“三段式壳层”：
  1) 顶部上下文区（as_of、effective_date、当前审计定位信息）；
  2) 中部双栏区（左时间轴事件列表 / 右事件详情）；
  3) 底部辅助区（返回与帮助信息）。

### 5.2 信息优先级
- 统一优先级：错误 > 警示 > 加载信息 > 帮助提示。
- 将分散的加载/错误提示收敛到固定容器，减少阅读跳跃。

### 5.3 组件语义
- 对齐 Capability Key 口径：统一 `Paper` 分区、标题层级、间距与分隔。
- 保留现有交互节点（选中样式、load more、Accordion 展开）但升级壳层可读性。

## 6. 实施步骤
1. [ ] 冻结 Audit 视图壳层线框与分区清单（不改行为）。
2. [ ] 改造 `OrgUnitDetailsPage` 的 audit 分支容器结构与样式。
3. [ ] 回归 Audit 四类状态：加载、错误、有数据、无数据。
4. [ ] 核验 URL 恢复与事件选中链路零回归（含 reload 后恢复）。
5. [ ] 完成门禁与证据记录（若进入实施，补充到 `docs/dev-records/`）。

## 7. 验收标准
- [ ] 变更日志视图外观与 Capability Key 页面风格对齐，且行为不变。
- [ ] `tab=audit` 与 `audit_event_uuid` 的 URL 恢复行为保持不变。
- [ ] `org-audit-*` 定位能力保持不变（E2E 不因选择器漂移失败）。
- [ ] 回归用例通过（至少）：`e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`。
- [ ] 关键门禁通过：`make check lint && make test && make check routing && make check capability-route-map && make check error-message`。

## 8. 风险与缓解
- **风险 1：壳层改造影响事件定位/URL 恢复**  
  缓解：冻结 `data-testid` 与 query param 行为，回归 reload 场景。
- **风险 2：Audit 信息拥挤导致可读性下降**  
  缓解：分层展示与固定提示优先级，限制同屏告警数量。
- **风险 3：改造范围外溢到 Profile/弹窗**  
  缓解：提交评审按 `tab === 'audit'` 变更边界逐项核对。

## 9. 关联文档
- `docs/dev-plans/170-org-form-ui-shell-alignment-with-capability-key.md`
- `docs/dev-records/dev-plan-170-execution-log.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/002-ui-design-guidelines.md`
