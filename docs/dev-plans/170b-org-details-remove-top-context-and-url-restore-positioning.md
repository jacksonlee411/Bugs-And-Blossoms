# DEV-PLAN-170B：Org 详情页移除顶部上下文区与 URL 恢复定位替代方案

**状态**: 已完成（2026-02-25 00:07 UTC）

## 1. 背景与问题
- `DEV-PLAN-170/170A` 在 `OrgUnitDetailsPage` 引入了顶部上下文区（Chip + `org-context-summary`）以对齐治理台壳层。
- 实际评审发现“同一字段多点重复”问题明显：`effective_date/status/event_type/event_uuid/tx_time` 在顶部、左栏、右栏重复出现，增加阅读噪声。
- 现有 URL 恢复 E2E 依赖 `org-context-summary`，若移除顶部上下文区，需要提供新的“恢复后可定位”锚点方案，避免可测性退化。
- 结论：170B 是对 170A 的**UI 信息分层纠偏**，纠偏对象是“顶部上下文承载定位”的实现策略，而不是否定 170A 的 URL 恢复目标本身。

## 2. 目标与非目标
### 2.1 目标
- [X] 完全移除详情页（Profile/Audit）顶部上下文区（包含 Chip 行与 `org-context-summary` 文本）。
- [X] 收敛信息来源，消除关键字段重复展示。
- [X] 保持 URL 恢复行为不变：`tab/effective_date/audit_event_uuid` 仍可直接回放并准确定位。
- [X] 提供不依赖顶部摘要的新定位方案，并更新 E2E 断言。
- [X] 完成对 170A 的纠偏闭环：保留其“可恢复、可定位”验收目标，替换其“顶部摘要作为定位锚点”的实现路径。
- [X] 将“变更日志-事件详情”仅做字段排版对齐“版本详情”（统一字段栅格、标签/值对齐与行间距）。
- [X] 版本详情页面显示全部业务字段（内置字段 + 扩展字段），仅排除元数据字段。

### 2.2 非目标
- 不改后端 API、路由映射、capability_key、写入弹窗行为与契约。
- 不改 `listOrgUnitVersions/listOrgUnitAudit` 的分页与选择逻辑。
- 不新增业务字段，不变更 diff 生成与 raw payload 内容。
- 不做事件详情容器重组、交互流程调整或视觉主题改版（仅字段排版优化）。

## 3. 去重后的信息分层（目标形态）
1. **列表层（左栏）**：只承担“当前选中项”的定位入口，不承担完整上下文摘要。
2. **详情层（右栏）**：作为字段事实唯一展示层（尤其是 `event_uuid/event_type/tx_time/status`）。
3. **页面壳层（Header/Tabs）**：只保留导航与动作，不承载业务字段摘要。

> 约束：同一字段最多在“列表层 + 详情层”出现一次语义重复；禁止再加全局第三处摘要。

## 3B. 版本详情字段完整性口径（新增）
- **展示范围（必须）**：`org_unit` 业务字段 + `ext_fields` 扩展字段全部展示。
- **元数据排除（不展示）**：仅排除技术/审计元数据（例如 event_id/event_uuid、request_id、tx_time、snapshot、审计关联字段等）。
- **一致性要求**：版本详情与审计事件详情对齐后，字段分组顺序保持稳定，避免“同字段在两处命名/位置不一致”。

## 3C. 标题与内容边界（新增）
- **版本详情标题**：固定为 `版本详情（当前选中） <effective_date>`（示例：`版本详情（当前选中） 2026-01-01`）。
- **版本详情内容**：仅显示版本详情字段，不混入事件详情信息。
- **事件详情标题**：固定为 `事件详情（当前选中）`。
- **事件详情内容**：仅显示事件详情字段，不混入版本详情信息。

## 3A. 对 170A 的纠偏清单（新增）
1. **保留的 170A 契约**
   - 保留 URL 恢复契约：`tab=audit` + `audit_event_uuid`；
   - 保留“恢复后必须有稳定可测定位点”的验收要求。
2. **回退的 170A 实现**
   - 回退 `org-context-summary` 作为主定位锚点；
   - 回退顶部 context Chip 对审计事件定位信息的承载。
3. **替代实现（170B 生效后）**
   - 左栏选中态（`org-audit-*` / `org-version-*`）作为入口锚点；
   - 右栏事件事实字段（UUID）作为结果锚点；
   - 必要时自动滚动到左栏选中项，保障恢复后“可见定位”。

## 4. URL 恢复后的定位替代方案
### 4.1 方案 A（采纳）：左栏选中锚点 + 右栏唯一事实锚点
- 恢复后定位以左栏 `ListItemButton.selected` 为主锚点（版本：`org-version-*`；审计：`org-audit-*`）。
- 进入 `tab=audit` 且存在 `audit_event_uuid` 时，自动将选中审计项滚动到可视区域中部（`scrollIntoView`）。
- 右栏增加稳定测试锚点（例如 `data-testid="org-audit-selected-event"` 与 `data-testid="org-audit-selected-event-uuid"`），作为恢复结果的事实确认点。
- E2E 从“顶部摘要包含 UUID”改为“左栏选中 + 右栏 UUID 精确匹配”双断言。

### 4.2 方案 B（备选）：URL hash 锚点
- 将 `audit_event_uuid` 同步到 hash（如 `#audit:<uuid>`）并据此定位。
- 不采纳原因：会引入 query/hash 双状态源，复杂度高，且与现有 URL 契约不一致。

### 4.3 方案 C（备选）：恢复后 Toast 提示
- 恢复后弹出“已定位到事件 XXX”提示，不改页面结构。
- 不采纳原因：提示为瞬时信息，不是稳定锚点，不适合作为测试与运维排障依据。

## 5. 改造范围
- `apps/web/src/pages/org/OrgUnitDetailsPage.tsx`
  - 删除顶部上下文区。
  - 增补审计恢复定位的滚动与右栏测试锚点。
- `e2e/tests/tp060-04-orgunit-details-two-pane.spec.js`
  - 用新锚点替换 `org-context-summary` 相关断言。
- `docs/dev-plans/170a-org-audit-log-ui-shell-alignment-with-capability-key.md`
  - 补充“被 170B 纠偏的实现项”说明，避免后续按 170A 旧实现回滚。
- `docs/dev-records/dev-plan-170-execution-log.md`
  - 记录 170B 改造与回归证据（时间/命令/结果）。

## 6. 实施步骤
1. [X] 删除顶部上下文区（Profile/Audit 共用区域）并清理无用状态（`contextSummary/contextEffectiveDate`）。
2. [X] 保持现有 URL 恢复主链路不变，补充“审计选中项自动滚动到可视区”行为。
3. [X] 版本详情补齐业务字段展示（内置 + 扩展；元数据排除），并固定字段分组顺序。
4. [X] 调整标题与内容边界：版本详情与事件详情标题按 3C 执行，且两者内容不互相混入。
5. [X] 在右栏详情增加稳定定位锚点（事件容器 + UUID 字段）。
6. [X] 更新 E2E：从摘要断言迁移到“左栏选中 + 右栏事实字段”断言。
7. [X] 执行并记录门禁：`make check doc`、`pnpm -C e2e exec playwright test tests/tp060-04-orgunit-details-two-pane.spec.js`、`make preflight`。

## 7. 验收标准
- [X] 页面不再出现顶部上下文区（无 `org-context-summary`、无该区 Chip 行）。
- [X] URL 直达恢复仍可定位正确版本/审计事件，且刷新后保持一致。
- [X] 审计恢复定位具备可见选中态与右栏事实锚点，E2E 稳定通过。
- [X] 无新增重复信息回归（`status/effective_date/event_uuid/event_type/tx_time` 不再三处重复）。
- [X] 审计事件详情区仅完成字段排版对齐（字段栅格、标签/值对齐、行间距），不引入容器与交互改造。
- [X] 版本详情区完整展示全部业务字段（内置 + 扩展），且不展示元数据字段。
- [X] 标题与内容边界满足：`版本详情（当前选中） <effective_date>` 仅承载版本详情；`事件详情（当前选中）` 仅承载事件详情。

## 8. 风险与缓解
- **风险 1：移除摘要后首屏“当前上下文”感知下降**  
  缓解：强化左栏选中态与右栏标题语义（“已选版本/已选事件”）。
- **风险 2：滚动定位在异步加载下失效**  
  缓解：在列表数据加载完成且目标节点存在后触发滚动，并在 E2E 覆盖 reload 场景。
- **风险 3：测试选择器再次出现歧义**  
  缓解：新增语义化 `data-testid` 到右栏事实字段，避免依赖模糊文案匹配。

## 9. 关联文档
- `docs/dev-plans/170-org-form-ui-shell-alignment-with-capability-key.md`
- `docs/dev-plans/170a-org-audit-log-ui-shell-alignment-with-capability-key.md`
- `docs/dev-records/dev-plan-170-execution-log.md`
- `docs/dev-plans/002-ui-design-guidelines.md`
