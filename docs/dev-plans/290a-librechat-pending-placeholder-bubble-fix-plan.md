# DEV-PLAN-290A：pending placeholder bubble 修复专项（Case 2 优先路径）

**状态**: 规划中（2026-03-08 CST；由 `DEV-PLAN-290` 多轮复跑失败触发，当前问题聚焦 `binding_key=::::` 的遗留 assistant 气泡）

## 1. 背景
1. [ ] `DEV-PLAN-290` 已多轮重跑 `tp290-e2e-001~004`，Playwright 进程结果为 `4 passed`，但业务 stopline 仍未通过。
2. [ ] 当前稳定复现问题为 `pending placeholder bubble`：出现 `binding_key=::::` 的 assistant 气泡，导致 `single_assistant_bubble=false` 与 `official_message_tree_only=false`。
3. [ ] 失败集中在 Case 2/3/4；Case 1 已通过，因此本计划按“先 Case 2、再扩展 Case 3/4”的顺序推进。
4. [ ] `DEV-PLAN-271` 的 `S5` 仍被该问题阻塞；在缺口未修复前不得判定 `290` 通过，也不得进入 `285` 封板。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 修复 `binding_key=::::` 对应的 pending placeholder bubble 生成/残留链路。
2. [ ] 先在 Case 2 路径达成“同轮单 assistant 气泡 + 官方消息树唯一落点”，再复用到 Case 3/4。
3. [ ] 产出可复核证据，支撑 `DEV-PLAN-290` 重跑后关闭 stopline。
4. [ ] 保持 `280` 的单链路约束：不引入旧桥、双入口、双消息落点或前端业务重算。

### 2.2 非目标
1. [ ] 不调整 `260` 的 FSM/DTO 契约定义。
2. [ ] 不在本计划内扩展新业务能力，仅修复现有正式链路中的气泡一致性缺口。
3. [ ] 不以“放宽断言/修改 stopline”替代真实修复。
4. [ ] 不将 `290` 的验收职责迁入本计划；`290A` 仅负责修复与修复证据。

## 3. 范围与依赖
1. [ ] 作用范围：`/app/assistant/librechat` 正式入口的消息落点与绑定键生成/关联路径。
2. [ ] 依赖输入：`DEV-PLAN-290` 最新失败证据（`case-2/3/4-phase-assertions.json`、trace、DOM 快照）。
3. [ ] 执行顺序：`290A` 修复完成后，必须回到 `290` 重跑 Case Matrix，禁止只凭局部验证宣称通过。
4. [ ] 影响判定：若修复涉及路由/认证/fail-closed 语义，需按 `271` 规则重刷受影响证据。

## 4. 根因假设与排查矩阵
1. [ ] 假设 H1：占位气泡创建后未在正式消息到达时正确替换/合并，导致遗留 `binding_key=::::`。
2. [ ] 假设 H2：turn/request 绑定键在某些阶段未注入或注入时机晚于渲染，触发默认空键占位。
3. [ ] 假设 H3：消息树归并逻辑在 Case 2 的确认/提交阶段发生重复 append，形成“正式消息 + 占位消息”并存。
4. [ ] 假设 H4：retry/stream 事件在结束态清理未命中，留下 pending 节点。

## 5. 实施步骤（先 Case 2）
1. [ ] **步骤 A：证据回放与最小定位（Case 2）**  
   固定复现命令与输入序列，确认 `binding_key=::::` 的首次出现阶段、组件路径与事件来源。
2. [ ] **步骤 B：绑定键与占位生命周期加固（Case 2）**  
   修复 `placeholder -> bound message` 的替换/合并逻辑，确保 pending 节点在成功落地后被清理。
3. [ ] **步骤 C：单轮唯一气泡约束（Case 2）**  
   在同轮消息归并处加入一致性防护，阻断“空键占位 + 正式消息”并存。
4. [ ] **步骤 D：回归扩展到 Case 3/4**  
   复用 Case 2 修复路径验证缺字段补全与候选选择链路，确认无回归。
5. [ ] **步骤 E：回灌 `290` 验收**  
   重跑 `tests/tp290-librechat-real-case-matrix.spec.js --workers=1 --trace on`，刷新 `290` 证据与索引。

## 6. 停止线（Fail-Closed）
1. [ ] 若修复方案需要恢复旧桥职责（`bridge.js`、`data-assistant-dialog-stream`）才可通过，立即判失败并回退方案。
2. [ ] 若出现双正式入口、双消息落点或页面级业务 FSM 重算，立即判失败。
3. [ ] 若 Case 2 通过但 Case 3/4 回归失败，不得关闭 `290A`。
4. [ ] 若仅测试“通过数”上升但 phase 断言仍失败，不得关闭 `290A`。

## 7. 验收标准
1. [ ] Case 2/3/4 均不再出现 `binding_key=::::` 的 pending placeholder bubble。
2. [ ] `single_assistant_bubble=true` 与 `official_message_tree_only=true` 在 Case 2/3/4 全部成立。
3. [ ] `DEV-PLAN-290` 重跑结论更新为 Case 1~4 全通过，且 stopline 通过。
4. [ ] 修复后证据时间戳晚于最近一次影响性合入，且索引已刷新。

## 8. 测试与门禁（SSOT 引用）
1. [ ] 复跑命令入口以 `Makefile`、`AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 最低执行：`tp290` 用例重跑（`--workers=1 --trace on`）+ 文档门禁 `make check doc`。
3. [ ] 若修复触达更广范围，按触发器补跑对应专项门禁（routing/no-legacy 等）。

## 9. 交付物
1. [ ] 本计划文档：`docs/dev-plans/290a-librechat-pending-placeholder-bubble-fix-plan.md`。
2. [ ] 修复实现与测试补充（代码与测试文件按实际变更登记）。
3. [ ] 修复证据清单（建议目录）：`docs/dev-records/assets/dev-plan-290a/`。
4. [ ] 回灌结果：`DEV-PLAN-290` 证据索引与执行日志刷新记录。

## 10. 关联文档
- `docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`
- `docs/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- `docs/dev-records/dev-plan-290-execution-log.md`
- `AGENTS.md`
