# DEV-PLAN-170A：Org 变更日志页 UI 外观对齐 Capability Key（仅页面壳层改造）

**状态**: 已实施（2026-02-24 13:30 UTC，评审收口版）

## 1. 背景与问题
- DEV-PLAN-170 已完成详情主视图（Profile）壳层对齐；170A 聚焦同页 Audit 视图，继续对齐 Capability Key 治理台的信息架构与阅读节奏。
- 评审指出三个关键缺口：范围边界不清、顶部上下文未体现“当前选中审计事件”、`as_of` 文案存在硬编码 i18n 漂移风险。
- 本次收口在“不改审计行为契约”的前提下，完成壳层体验与可维护性同步提升。

## 2. 目标与非目标
### 2.1 目标
- [x] 将 Audit 视图改造成治理台式外观（上下文区 + 双栏内容区 + 辅助区）。
- [x] 顶部上下文区展示“当前选中审计事件”的定位信息（event type / event UUID / tx time）。
- [x] 保持行为契约不变（事件选择、URL 恢复、差异展示、raw payload 展示、加载更多）。
- [x] 消除新增壳层中的 i18n 硬编码，统一走 `messages.ts`。

### 2.2 非目标
- 不变更后端 API 契约、路由映射、capability_key 语义。
- 不新增/删除审计字段，不改 `before_snapshot/after_snapshot` 对比逻辑。
- 不改写入弹窗行为（add/insert/correct/delete）。

## 3. 评审问题与整改决策
1. **范围边界风险（170A 只应改 Audit）**
   决策：170A 文档明确“实施增量以 audit 上下文和审计壳层为核心”；Profile 保持 170 的既有交付，不在 170A 再扩展行为修改。
2. **顶部上下文误用 profile 版本事件类型**
   决策：新增 audit 上下文摘要逻辑，`tab=audit` 时统一基于 `selectedAuditEvent` 渲染。
3. **`as_of` 硬编码**
   决策：替换为 i18n key（`org_filter_as_of`），并补齐审计事件 UUID 的 i18n key（`org_audit_event_uuid`）。

## 4. 改造范围（UI Shell Only）
- `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
  - 顶部上下文区改为 tab-aware：Profile 与 Audit 使用不同摘要来源。
  - Audit 上下文新增 tx_time / rescinded 状态 Chip（仅展示，不改行为）。
  - 审计详情中的 `event_uuid` 文案改为 i18n key。
- `apps/web/src/i18n/messages.ts`
  - 新增 `org_audit_event_uuid`（en/zh）。
- `e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`
  - 增补上下文摘要断言（`org-context-summary`）以防回归。

## 5. 不变量（冻结）
1. [x] 不修改审计查询调用链：`listOrgUnitAudit`。
2. [x] 不修改事件选择与 URL 同步契约：`tab=audit`、`audit_event_uuid`。
3. [x] 保留 `data-testid`：`org-audit-<event_uuid>`。
4. [x] 保留差异表、撤销信息、raw payload 字段语义与展示内容。
5. [x] 保留“加载更多”行为与分页上限递增逻辑。

## 6. 视觉与交互口径
### 6.1 页面结构层
- 三段式壳层：顶部上下文区、中部双栏区、底部辅助区。
- Audit 上下文区统一显示：as_of、effective_date（audit 下取选中事件 effective_date）、状态、选中事件摘要。

### 6.2 信息优先级
- 统一优先级：错误 > 警示 > 加载信息 > 帮助提示。
- 将加载/错误提示收敛到固定容器，减少阅读跳跃。

### 6.3 i18n 与可测性
- 不允许新增硬编码业务文案（包括 `as_of`、event UUID 标签）。
- 顶部摘要引入稳定测试锚点：`data-testid="org-context-summary"`。

## 7. 验收标准
- [x] Audit 视图外观与 Capability Key 风格对齐，行为不变。
- [x] `tab=audit` 与 `audit_event_uuid` URL 恢复行为保持不变。
- [x] 顶部上下文在 audit 下显示当前选中事件信息，不再复用 profile 版本事件类型。
- [x] `org-audit-*` 定位能力保持不变（E2E 选择器稳定）。
- [x] `as_of` 与 event UUID 标签走 i18n，不再硬编码。

## 8. 风险与缓解
- **风险 1：上下文摘要切换逻辑引入 tab 间误判**
  缓解：使用 `detailTab + selectedAuditEvent` 的显式分支，并在 E2E 增加上下文断言。
- **风险 2：i18n key 漏配导致文案回退**
  缓解：en/zh 同步补齐 `org_audit_event_uuid`。
- **风险 3：后续壳层改动再次漂移为硬编码**
  缓解：将 `org-context-summary` 和 i18n key 作为评审检查项。

## 9. 关联文档
- `docs/dev-plans/170-org-form-ui-shell-alignment-with-capability-key.md`
- `docs/dev-records/dev-plan-170-execution-log.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/002-ui-design-guidelines.md`
