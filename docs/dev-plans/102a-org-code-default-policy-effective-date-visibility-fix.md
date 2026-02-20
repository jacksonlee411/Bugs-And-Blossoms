# DEV-PLAN-102A：Org Code 默认规则“保存后无变化”生效日错位调查与收敛方案

**状态**: 已完成（2026-02-20 08:10 UTC）

## 1. 背景
- 现象：在“字段配置”页面把 `org_code` 默认规则改为 `next_org_code('ORG',6)` 并保存后，页面看起来“没有变化”。
- 影响：管理员误判“保存失败”，并可能在新建组织时继续手工输入 `org_code`，造成规则能力不可发现。

## 2. 调查发现（代码证据）

### 2.1 主因：策略生效日与页面 as_of 错位
- 策略弹窗默认把 `enabled_on` 设为 `max(todayUtc, asOf)`，会把“过去视图日”自动抬到“今天”：`apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx:618`。
- 列表读取策略时使用当前 URL 的 `as_of` 发起查询：`apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx:460`。
- 后端解析策略要求 `enabled_on <= as_of` 才命中：`internal/server/orgunit_field_metadata_store.go:451`。
- 结果：若页面 `as_of=2026-01-01`，保存发生在 `2026-02-20`，则新策略 `enabled_on=2026-02-20`，当前 `as_of` 仍查不到新策略，表现为“保存后无变化”。

### 2.2 次因：规则语法口径前后不一致（单引号/双引号）
- API 保存阶段只做 CEL 编译校验，`next_org_code('ORG',6)` 可通过。
- 写入执行阶段的规则解析正则仅接受双引号：`next_org_code("ORG", 6)`（`modules/orgunit/services/orgunit_write_service.go:58`、`modules/orgunit/services/orgunit_write_service.go:1680`）。
- 结果：即使策略已保存，创建组织时仍可能触发 `DEFAULT_RULE_EVAL_FAILED`。

## 3. 根因结论
1. **可见性根因**：`enabled_on` 默认值与当前 `as_of` 发生时间错位，导致读取口径下命中旧策略。
2. **稳定性根因**：规则“保存校验”和“运行解析”语法不一致，用户输入单引号时存在运行时失败风险。

## 4. 收敛方案（执行结果）

### 4.1 UI 可见性收敛（优先）
1. [X] 保存策略成功后，若 `enabled_on > as_of`，自动把 URL `as_of` 推进到 `enabled_on`，并提示“已切换到策略生效日”。
2. [X] 在策略弹窗中对“生效日晚于当前视图日”增加显式提示，避免误判“未生效=未保存”。
3. [X] 在列表中保留并强化 `pending` 语义（按 `as_of` 未生效时显示待生效状态）。

### 4.2 规则语法口径收敛（同批）
1. [X] 统一规则规范为 `next_org_code("PREFIX", WIDTH)`（文案、placeholder、校验说明一致）。
2. [X] 解析器仅支持双引号写法；保存阶段对单引号写法返回 `FIELD_POLICY_EXPR_INVALID`，保持单一口径。
3. [X] 补齐测试：覆盖双引号成功与单引号拒绝场景。

## 5. 验收标准
- [X] 在 `as_of` 早于当天的页面中保存策略后，不再出现“保存后无变化”的误导体验。
- [X] 新建组织在 `org_code` 自动生成场景下，仅 `next_org_code("ORG",6)` 作为有效写法；单引号写法在保存阶段明确拒绝。
- [X] 字段配置页、创建弹窗、后端写入链路对默认规则的语义完全一致。

## 6. 落地变更（代码）

- 前端（MUI）：
  - `apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx`：策略保存成功后，若 `enabled_on > as_of` 自动推进 URL `as_of` 并展示切换提示；策略弹窗增加“晚于当前视图日”提示；规则输入说明统一为 `next_org_code("PREFIX", WIDTH)`。
  - `apps/web/src/pages/org/orgUnitFieldPolicyAsOf.ts`：新增 `as_of` 推进判定函数，收敛比较逻辑。
  - `apps/web/src/pages/org/orgUnitFieldPolicyAsOf.test.ts`：补齐前端判定逻辑测试。
  - `apps/web/src/i18n/messages.ts`：新增提示文案（en/zh 同步）。
- 后端（Go）：
  - `modules/orgunit/services/orgunit_write_service.go`：`parseNextOrgCodeRule` 收敛为仅双引号写法。
  - `modules/orgunit/services/orgunit_write_service_policy_defaults_test.go`：补齐“单引号拒绝”测试。
  - `internal/server/orgunit_field_metadata_api.go` / `internal/server/orgunit_field_policy_api_test.go`：保存校验阶段拒绝单引号写法，避免运行时口径漂移。

## 7. 关联 SSOT
- `docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- `docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`
- `AGENTS.md`
- `docs/dev-records/dev-plan-102a-execution-log.md`
