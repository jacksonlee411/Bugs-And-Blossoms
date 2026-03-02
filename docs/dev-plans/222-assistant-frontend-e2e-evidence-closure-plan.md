# DEV-PLAN-222：Assistant 前端交互与 E2E 证据闭环实施计划

**状态**: 规划中（2026-03-02 05:37 UTC）

## 1. 背景
- `DEV-PLAN-220A` 识别 FE/E2E 缺口：高风险门控不足、状态按钮可用性未收敛、dry-run diff 展示不足、220 专项 E2E 未落地。
- 本计划承接 `DEV-PLAN-221`，聚焦“可见、可测、可验收”。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 完成前端状态机按钮门控（risk/state 双门控）。
2. [ ] 完成事务面板 `plan/diff/explain/risk_tier` 的可视化收口。
3. [ ] 落地 220 核心 E2E 用例（至少 `TC-220-E2E-101/102/103/104` + 关键阻断场景）。
4. [ ] 落地 `postMessage` origin/schema 安全校验与自动化测试。

### 2.2 非目标
1. [ ] 不在本计划引入数据库持久化（由 `DEV-PLAN-223` 负责）。
2. [ ] 不在本计划引入多模型配置治理（由 `DEV-PLAN-224` 负责）。

## 3. 实施范围
- 前端：`apps/web/src/pages/assistant/AssistantPage.tsx` 与相关 API/UI 测试。
- E2E：`e2e/tests/tp220-assistant-*.spec.js`。
- 安全：iframe 交互消息校验逻辑与测试样例。

## 4. 实施步骤
1. [ ] 定义前端状态矩阵：`validated/confirmed/committed/canceled/expired` 对应按钮启停规则。
2. [ ] 实现高风险门槛：`risk_tier=high` 未确认不可提交；提示文案与错误码一致。
3. [ ] 补齐 dry-run diff 展示组件与断言点（主键字段优先）。
4. [ ] 落地 E2E：
   - `tp220-assistant-low-risk-commit.spec.js`
   - `tp220-assistant-high-risk-role-drift.spec.js`
   - `tp220-assistant-create-department.spec.js`
   - `tp220-assistant-parent-candidate-confirm.spec.js`
5. [ ] 落地 postMessage 安全校验（白名单 origin + schema 校验 + 丢弃策略）与 FE/E2E 测试。

## 5. 测试与验收
1. [ ] 对齐 `TC-220-FE-003/005/006` 与 `TC-220-E2E-101/102/103/104`。
2. [ ] e2e 与 error-message 相关门禁通过（以仓库 SSOT 门禁为准）。
3. [ ] 验收标准：在 `/app/assistant` 可稳定复现“提案->确认->提交/阻断”用户旅程。

## 6. 风险与缓解
- **E2E 易脆弱**：增加稳定选择器与固定数据夹具，避免文案耦合。
- **UI 与后端状态不一致**：以前端仅消费后端状态机为准，禁止本地推断提交结果。
- **安全校验绕过**：默认拒绝（fail-closed），仅白名单来源放行。

## 7. 交付物
1. [ ] Assistant 页面交互收敛代码。
2. [ ] `tp220` 系列 E2E 用例与执行证据。
3. [ ] `DEV-PLAN-222` 执行记录文档（实施时新增到 `docs/dev-records/`）。

## 8. 关联文档
- `docs/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
- `docs/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
- `docs/dev-plans/221-assistant-p1-blocker-closure-plan.md`
- `AGENTS.md`
