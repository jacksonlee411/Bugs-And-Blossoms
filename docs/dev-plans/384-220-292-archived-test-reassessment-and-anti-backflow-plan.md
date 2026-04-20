# DEV-PLAN-384：220-292 归档后测试资产重评估与防回流专项方案

**状态**: 规划中（2026-04-17 09:07 CST）

> 本文聚焦测试资产治理，不重开 `220-292` 的功能设计本身。目标是把仍挂在活体测试、活体文档、活体门禁中的历史 Assistant / LibreChat 语义全部重新裁决、迁移或归档，阻断“过期内容因为测试仍然通过而回流成现行契约”。

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：针对已归档的 `DEV-PLAN-220 ~ DEV-PLAN-292`，全面重评估仍在活体测试链路中的 E2E / 前端 / Go 测试 / 文档引用 / 证据资产，完成“重新归属、迁移改名、归档移出、门禁防回流”的专项收口。
- **关联模块/目录**：
  - `e2e/tests`
  - `scripts/e2e`
  - `apps/web/src/**/*.test.*`
  - `internal/**/*_test.go`
  - `modules/**/*_test.go`
  - `pkg/**/*_test.go`
  - `docs/dev-plans/060-business-e2e-test-suite.md`
  - `docs/dev-plans/064a-test-tp060-05-assistant-conversation-intent-and-tasks.md`
  - `docs/archive/dev-plans/380g-cubebox-regression-gates-and-final-closure-plan.md`
  - `docs/dev-records/**`
- **关联计划/标准**：
  - `AGENTS.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/060-business-e2e-test-suite.md`
  - `docs/dev-plans/064a-test-tp060-05-assistant-conversation-intent-and-tasks.md`
  - `docs/dev-plans/300-test-system-investigation-report.md`
  - `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
  - `docs/archive/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
  - `docs/archive/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/archive/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
  - `docs/archive/dev-plans/380f-librechat-vendored-runtime-and-deploy-retirement-plan.md`
  - `docs/archive/dev-plans/380g-cubebox-regression-gates-and-final-closure-plan.md`
  - `docs/archive/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
  - `docs/archive/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
  - `docs/archive/dev-plans/221-assistant-p1-blocker-closure-plan.md`
  - `docs/archive/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
  - `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
  - `docs/archive/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
  - `docs/archive/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
  - `docs/archive/dev-plans/226-test-guide-tg004-gate-caliber-change-approval.md`
  - `docs/archive/dev-plans/230-librechat-project-level-integration-plan.md`
  - `docs/archive/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
  - `docs/archive/dev-plans/232-librechat-official-runtime-baseline-plan.md`
  - `docs/archive/dev-plans/233-librechat-single-source-config-convergence-plan.md`
  - `docs/archive/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
  - `docs/archive/dev-plans/239-librechat-chat-write-path-recovery-and-runtime-stability-plan.md`
  - `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
  - `docs/archive/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
  - `docs/archive/dev-plans/281-librechat-web-ui-source-vendoring-and-mainline-freeze-plan.md`
  - `docs/archive/dev-plans/283-librechat-formal-entry-cutover-plan.md`
  - `docs/archive/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
  - `docs/archive/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`
  - `docs/archive/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
  - `docs/archive/dev-plans/290b-librechat-live-intent-action-chain-evidence-plan.md`
  - `docs/archive/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`
  - `docs/archive/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- **用户入口/触点**：
  - `make e2e`
  - `pnpm --dir e2e exec playwright test`
  - PR/CI 的测试通过结果
  - `TP-060-05` 与 `380G` 的验收证据

### 0.1 Simple > Easy 三问

1. **边界**：归档计划编号不是活体测试的 owner；任何仍需保留的断言都必须重新归属到当前活体计划或稳定产品契约。
2. **不变量**：活体测试发现路径中不得继续保留 `220-292` 的占位壳、历史编号、历史 happy-path 契约或“只是 skip 掉所以没关系”的遗留文件。
3. **可解释**：reviewer 必须能在 5 分钟内说清每个活体测试为什么还存在、现在由谁负责、验证的是当前 `CubeBox` 还是历史 `LibreChat` 退役语义。

### 0.2 现状研究摘要

- `docs/archive/dev-plans/220-292` 已整体归档，但当前 `make e2e` 仍通过 `scripts/e2e/run.sh` 直接执行 `pnpm exec playwright test`，对 `e2e/tests/*.spec.js` 没有 archive 过滤。
- 当前活体发现路径里仍存在一批历史编号测试：
  - `e2e/tests/tp220-assistant.spec.js`
  - `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js`
  - `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js`
  - `e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js`
  - `e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js`
- 当前活体发现路径里还存在“历史占位但仍被发现”的文件：
  - `e2e/tests/tp284-librechat-send-render-takeover.prep.spec.js`
  - `e2e/tests/tp288-librechat-real-entry-evidence.spec.js`
  - `e2e/tests/tp290-librechat-real-case-matrix.spec.js`
- 前端与 Go 测试中仍有历史样例编号直接暴露，例如：
  - `apps/web/src/pages/cubebox/CubeBoxPage.test.tsx`
  - `internal/server/assistant_model_gateway_more_test.go`
- 活体测试总纲/子计划仍直接把 `220-225` 当作当前测试契约来源：
  - `docs/dev-plans/060-business-e2e-test-suite.md`
  - `docs/dev-plans/064a-test-tp060-05-assistant-conversation-intent-and-tasks.md`

## 1. 背景与上下文（Context）

- `DEV-PLAN-380/380C/380E/380F/380G` 已作为历史切换批次，将当时的正式产品面、正式 API 面和退役语义收敛到 `CubeBox` 主链与 `LibreChat` 退役断言；现均只作为归档证据引用。
- 但测试资产并没有同步完成“owner 重写”：
  - 一部分仍以 `tp220/tp283/tp288b/tp290b` 编号继续承担活体验收；
  - 一部分虽然 `skip`，却仍在 `playwright test --list` 与发现路径中出现；
  - 一部分活体文档仍把已归档计划当作当前测试 authority。
- 这会造成三类风险：
  1. **契约回流**：旧路由、旧术语、旧 happy-path 因“测试还绿着”而被误解成仍然有效。
  2. **责任漂移**：reviewer 无法判断某条测试今天到底由 `TP-060-05`、`380G` 还是历史归档计划负责。
  3. **门禁失真**：CI 虽然通过，但通过的是历史编号或历史占位壳，不代表当前产品契约真的被覆盖。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 核心目标

- [ ] 对所有命中 `220-292` 的活体测试资产完成 inventory freeze，并形成“资产 -> 处置方式 -> 新 owner”的唯一矩阵。
- [ ] 将仍有现行价值的断言迁移到当前活体 owner，优先收敛到 `TP-060-05` 的长期业务验收口径。
- [ ] 将仅保留历史价值的资产移出活体发现路径，转入 archive / dev-record evidence，而不是继续留在 `e2e/tests`。
- [ ] 对“历史占位但被发现”的脚本建立 stopline 与门禁，阻断后续通过 `skip` 或 `.prep.spec.js` 形式继续回流。
- [ ] 更新活体测试文档，使 `060/064A/380G` 不再把 `220-292` 直接当作当前测试 authority，而是改为“历史背景 + 当前 owner”双栏表达。

### 2.2 非目标（Out of Scope）

- 不在本文重新设计 `CubeBox` 的业务流程、状态机或后端实现。
- 不为了保留历史可追溯性而继续保留 `220-292` 作为活体测试文件名或活体计划编号。
- 不把所有历史 evidence 直接物理删除；允许保留为归档证据，但必须离开活体执行/发现路径。
- 不通过“继续 skip 旧测试”来满足治理目标；skip 只能是迁移中的短暂中间态，不能是长期解法。

### 2.3 用户可见性交付

- **用户可见入口**：PR/CI 的 `make e2e`、`playwright test --list`、`TP-060-05` 与 `380G` 验收证据。
- **最小可操作闭环**：
  - reviewer 可以直接看出哪些测试覆盖当前 `CubeBox` 主链；
  - `make e2e` 不再列出 `tp220-292` 历史编号；
  - 历史 `LibreChat` / `assistant` 路径只保留“退役/redirect/410”语义，不再通过旧 happy-path 测试回流。

### 2.4 工具链与门禁（SSOT 引用）

- **当前命中触发器**：
  - [ ] Go 测试
  - [ ] `apps/web/**` 测试
  - [X] E2E
  - [X] 文档
  - [X] 质量门禁/CI 接线
- **当前必须对齐的现有入口**：
  - `make e2e`
  - `make check doc`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `scripts/e2e/run.sh`
- **本文拟新增的专项门禁（命名可在实施时微调，但语义冻结）**：
  - `make check no-archived-test-plan-ref`
  - `make check no-historical-e2e-placeholder`
  - `make check current-test-owner-docs`

### 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 当前问题 | 目标口径 |
| --- | --- | --- | --- |
| `e2e/tests` | 长期业务验收与退役负向断言 | 历史编号/占位壳仍活体发现 | 只保留当前 owner 的活体用例 |
| `apps/web` 测试 | 页面状态、API client、纯转换器 | 测试样例仍带旧 TP 编号 | 改为中性 fixture / 当前业务语义 |
| Go 测试 | 协议映射、状态机、错误码 | 历史场景编码泄露到样例值 | 改为中性命名，不再借历史计划编号 |
| `docs/dev-records` / evidence 资产 | 历史证据留存 | 与活体测试边界不清 | 只做 evidence，不参与活体发现 |

## 3. 当前资产盘点与初步分类

| 资产 | 当前角色 | 当前问题 | 初步处置建议 |
| --- | --- | --- | --- |
| `e2e/tests/tp220-assistant.spec.js` | 活体 E2E | 旧编号仍承担当前入口/redirect 断言 | 重写并迁入 `TP-060-05` 当前主链套件 |
| `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js` | 活体 E2E | 仍以 cutover 计划编号承担稳定产品断言 | 将仍需长期保留的断言吸收进 `TP-060-05`；剩余一次性 cutover 证据移至 readiness |
| `e2e/tests/tp288b-librechat-live-task-receipt-contract.spec.js` | 活体 E2E | receipt contract 仍挂历史 plan owner | 若仍验证当前正式 contract，则改名并迁入 `TP-060-05`；否则转 evidence-only |
| `e2e/tests/tp290b-librechat-live-intent-action-chain.spec.js` | 活体 E2E | 当前主链 happy-path 仍用历史编号 | 改名并迁入 `TP-060-05` |
| `e2e/tests/tp290b-librechat-live-intent-action-negative.spec.js` | 活体 E2E | 当前负向 contract 仍用历史编号 | 改名并迁入 `TP-060-05` |
| `e2e/tests/tp284-librechat-send-render-takeover.prep.spec.js` | 占位壳 | `prep` 占位仍在活体发现路径 | 立即归档或删除，不得继续位于 `e2e/tests` |
| `e2e/tests/tp288-librechat-real-entry-evidence.spec.js` | skip 历史脚本 | 历史 evidence 仍被 Playwright 发现 | 转 archive / evidence-only，退出活体发现 |
| `e2e/tests/tp290-librechat-real-case-matrix.spec.js` | skip 历史脚本 | 历史 matrix 仍被 Playwright 发现 | 转 archive / evidence-only，退出活体发现 |
| `apps/web/src/pages/cubebox/CubeBoxPage.test.tsx` | 前端单测 | fixture 直接携带 `TP290B...` 样例值 | 改为中性业务 fixture，不再表达历史 owner |
| `internal/server/assistant_model_gateway_more_test.go` | Go 测试 | 样例值直接携带 `TP290B...` | 改为中性 fixture，不再表达历史 owner |
| `docs/dev-plans/060-business-e2e-test-suite.md` | 活体测试总纲 | 仍写“220-225 覆盖” | 更新为当前 `CubeBox` / `TP-060-05` owner 口径 |
| `docs/dev-plans/064a-test-tp060-05-assistant-conversation-intent-and-tasks.md` | 活体测试子计划 | 仍把 `220-225` 作为当前契约引用 | 改写为“历史背景 + 当前活体 owner/SSOT”口径 |
| `docs/dev-records/assets/dev-plan-288b/**`、`docs/dev-records/assets/dev-plan-290b/**` | 证据资产 | 当前与活体测试边界不清 | 保留为 readiness/evidence，仅禁止再作为活体测试 owner |

## 4. 重评估裁决规则（Decision Rules）

### 4.1 单条测试的四问

1. **它验证的是不是当前产品行为？**
   - 若否：归档或删除。
2. **它现在是否有明确活体 owner？**
   - 若否：必须迁移到 `TP-060-05` 或其他当前活体计划，再保留。
3. **它是否还依赖旧路由/旧 happy-path 作为正向成功语义？**
   - 若是：必须改写为 `CubeBox` 当前主链，或改为 retired negative assertion。
4. **它是否只是历史占位壳/历史 evidence？**
   - 若是：不得继续留在 `e2e/tests` 活体发现路径。

### 4.2 处置分类（唯一四分法）

- **A. Delete**：功能已消失且不再需要长期证据；删除源文件与引用。
- **B. Archive as Evidence**：只保留历史证据价值；转入 archive / dev-record，不参与 `make e2e`。
- **C. Rewrite and Re-own**：断言仍有现行价值，但必须改名、改 owner、改文案、改路由语义。
- **D. Neutralize Fixtures**：保留测试层级与覆盖目标，但移除历史 TP 编号、历史名词和旧 owner 暗示。

### 4.3 当前推荐 owner 口径

- **长期业务验收**：
  - 优先并入 `TP-060-05`
  - 文件名与测试标题不再使用 `tp220-292`
- **一次性切换/封板证据**：
  - 写入 `380G` readiness / `docs/dev-records/**`
  - 不再保留为长期活体 Playwright case
- **纯历史留痕**：
  - 移至 archive / evidence-only
  - 从 `e2e/tests` 与活体 docs 中剥离

## 5. 实施步骤（Implementation Slices）

### 5.1 Inventory Slice

1. [ ] 对以下范围执行一次 inventory freeze：
   - `e2e/tests`
   - `apps/web/src/**/*.test.*`
   - `internal/**/*_test.go`
   - `modules/**/*_test.go`
   - `pkg/**/*_test.go`
   - 活体测试文档与 dev-record assets
2. [ ] 形成唯一映射表：
   - 旧资产路径
   - 当前状态（active / skip / placeholder / evidence）
   - 处置分类（A/B/C/D）
   - 新 owner
   - 实施人
   - 验证命令

### 5.2 Migration Slice

1. [ ] 将当前仍需保留的 `CubeBox` 正向主链、receipt contract、negative contract 迁入新的活体命名体系。
2. [ ] 将历史 `LibreChat` formal entry / retired prefix 断言拆成两类：
   - 当前稳定产品必须长期成立的 redirect / 410 / retired negative assertion
   - 只属于 cutover 阶段的一次性证据
3. [ ] 删除或归档 `prep` / `skip historical` 占位脚本，确保它们不再被 Playwright 发现。
4. [ ] 将前端 / Go 测试中的历史 `TP220-292` 样例编号中性化。

### 5.3 Docs and Owner Slice

1. [ ] 更新 `DEV-PLAN-060`，移除“220-225 为当前测试覆盖 owner”的表达。
2. [ ] 更新 `DEV-PLAN-064A`，改为：
   - 历史背景可引用 archive
   - 当前活体验收 owner 必须引用 `TP-060-05` 与 `CubeBox` 现行契约
3. [ ] 视迁移结果更新 `380G`，说明哪些已从一次性 cutover case 收敛为长期活体业务测试。

### 5.4 Gate Slice

1. [ ] 新增 `no-archived-test-plan-ref`：
   - 扫描活体测试与活体测试文档
   - 阻断 `tp220-292`、`DEV-PLAN-220-292` 在活体 owner 语义中的继续出现
2. [ ] 新增 `no-historical-e2e-placeholder`：
   - 阻断 `.prep.spec.js`
   - 阻断 `test.skip(true, "historical...")` 一类长期占位壳留在 `e2e/tests`
3. [ ] 新增 `current-test-owner-docs`：
   - 阻断活体测试总纲/子计划把 archive plan 直接写成当前验收 authority
4. [ ] 若需要，调整 `scripts/e2e/run.sh` 或目录布局，保证 archive 资产天然不参与发现。

### 5.5 Regression and Readiness Slice

1. [ ] 记录迁移前后的 `playwright test --list` 对比。
2. [ ] 执行迁移后的活体测试：
   - `make e2e`
   - 命中的前端/Go 测试
3. [ ] 回写 `docs/dev-records/DEV-PLAN-384-READINESS.md`。

## 6. 命名与归属收敛原则

1. **活体 E2E 文件名** 不得再使用 `tp220-292` 作为前缀。
2. **活体测试标题** 不得再把归档计划编号当作当前 owner。
3. **长期业务验收** 应收敛到当前产品语义：
   - `cubebox`
   - `conversation`
   - `turn`
   - `confirm/commit`
   - `retired route`
4. **允许出现历史路径字符串**，但仅限：
   - 断言某路径已 redirect / 410 / gone
   - 断言错误码如 `librechat_retired`、`assistant_api_gone`
5. **不允许继续以 `LibreChat` 作为活体测试 owner 命名**；若只是验证退役负向语义，也应以当前 owner 命名并在断言中体现旧路径。

## 7. Stopline（强制）

- [ ] 活体发现路径 `e2e/tests` 中不得再存在 `tp220-292` 文件。
- [ ] 活体发现路径中不得再存在 `.prep.spec.js`、历史占位壳、长期 `test.skip(true, "...historical...")`。
- [ ] 活体 happy-path 测试不得再以内网旧聊天入口或历史对话前端作为成功契约。
- [ ] 活体文档不得再把 archive plan 写成当前测试 authority。
- [ ] 历史 evidence 只允许存在于 archive / `docs/dev-records`，不得继续伪装成活体测试。

## 8. 验收标准与 Readiness

### 8.1 验收标准

- [ ] 所有命中 `220-292` 的活体测试资产均已完成 A/B/C/D 分类。
- [ ] `pnpm --dir e2e exec playwright test --list` 不再列出 `tp220-292` 与历史占位壳。
- [ ] `make e2e` 只验证当前 `CubeBox` 主链与当前退役负向语义。
- [ ] 活体测试文档 `060/064A/380G` 已完成 owner 口径更新。
- [ ] 新专项门禁已接线，并能阻断归档测试资产回流。

### 8.2 Readiness 最低输出

- [ ] 新建 `docs/dev-records/DEV-PLAN-384-READINESS.md`
- [ ] 记录 inventory freeze 结果
- [ ] 记录旧资产 -> 新 owner 映射
- [ ] 记录 `playwright test --list` 迁移前后对比
- [ ] 记录命中的门禁与测试结果
- [ ] 记录仍保留为 evidence-only 的资产清单

## 9. 关联事实源

1. `AGENTS.md`
2. `docs/dev-plans/012-ci-quality-gates.md`
3. `docs/dev-plans/060-business-e2e-test-suite.md`
4. `docs/dev-plans/064a-test-tp060-05-assistant-conversation-intent-and-tasks.md`
5. `docs/dev-plans/300-test-system-investigation-report.md`
6. `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
7. `docs/archive/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
8. `docs/archive/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
9. `docs/archive/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
10. `docs/archive/dev-plans/380f-librechat-vendored-runtime-and-deploy-retirement-plan.md`
11. `docs/archive/dev-plans/380g-cubebox-regression-gates-and-final-closure-plan.md`
12. `docs/archive/dev-plans/220-chat-assistant-upgrade-implementation-plan.md`
13. `docs/archive/dev-plans/220a-chat-assistant-gap-assessment-and-closure-plan.md`
14. `docs/archive/dev-plans/221-assistant-p1-blocker-closure-plan.md`
15. `docs/archive/dev-plans/222-assistant-frontend-e2e-evidence-closure-plan.md`
16. `docs/archive/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
17. `docs/archive/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
18. `docs/archive/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
19. `docs/archive/dev-plans/230-librechat-project-level-integration-plan.md`
20. `docs/archive/dev-plans/231-librechat-prerequisites-contract-and-gates-plan.md`
21. `docs/archive/dev-plans/232-librechat-official-runtime-baseline-plan.md`
22. `docs/archive/dev-plans/233-librechat-single-source-config-convergence-plan.md`
23. `docs/archive/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
24. `docs/archive/dev-plans/239-librechat-chat-write-path-recovery-and-runtime-stability-plan.md`
25. `docs/archive/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
26. `docs/archive/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
27. `docs/archive/dev-plans/281-librechat-web-ui-source-vendoring-and-mainline-freeze-plan.md`
28. `docs/archive/dev-plans/283-librechat-formal-entry-cutover-plan.md`
29. `docs/archive/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
30. `docs/archive/dev-plans/288b-librechat-tp288-async-task-receipt-and-live-evidence-plan.md`
31. `docs/archive/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
32. `docs/archive/dev-plans/290b-librechat-live-intent-action-chain-evidence-plan.md`
33. `docs/archive/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`
34. `docs/archive/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
