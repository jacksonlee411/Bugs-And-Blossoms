# DEV-PLAN-290 执行日志（260-M5 真实 Case 验收与证据固化）

> 归档说明（2026-04-12）：本记录已自 `docs/dev-records/` 迁入 `docs/archive/dev-records/`，仅保留为历史执行证据，不再作为活体入口。

**状态**: 已完成（2026-03-09 01:55 CST；`290A` 已完成根因修复并回灌 `tp290-e2e-001~004`，Case 1~4 全部通过 `266/280` stopline，`260-M5` 已形成可复核封板输入）

## 1. 执行范围（与 290 对齐）
1. [X] 按 `Case Matrix v1` 执行 Case 1~4 真实验收（固定输入向量，不临场改词）。
2. [X] 每个 Case 固化页面、DOM、网络、trace、phase 断言证据。
3. [X] 每个 Case 同步断言 `280` 硬门槛：单正式入口、官方消息树唯一落点、无外挂回执、DTO-only。
4. [X] 输出面向 `285` 的 `260-M5` 收口结论。

## 2. 固定输入口径
1. [X] 唯一输入口径：`docs/archive/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md` 第 4.1 节 `Case Matrix v1`。
2. [X] 默认确认词口径：`确认` / `是的`。
3. [X] Case 4 未触发候选顺序不稳定，沿用矩阵默认输入 `选第2个`，无需切换候选编码。

## 3. 执行记录
1. [X] 启动门槛已满足：`292 -> 288 -> 290` 前置链路前两段已关闭，`tp288-e2e-001/002` 已通过。
2. [X] 2026-03-08：已完成 `290` 文档与 `271/280` 对齐改写，并把 `280` 硬门槛写入 `290` 验收条款。
3. [X] 2026-03-08：已创建 `docs/dev-records/assets/dev-plan-290/`，并落地 `tp290-case-matrix-v1.md` 与 `tp290-real-case-evidence-index.json`。
4. [X] 2026-03-08 首轮结果：Case 1 `passed`；Case 2/3/4 因 `pending placeholder bubble (binding_key=::::)` 导致 `single_assistant_bubble=false` 与 `official_message_tree_only=false`，当前不得判定 `290` 通过。
5. [X] 2026-03-08 20:42 CST：已按最新代码多轮重跑 `tp290` 并覆盖证据资产，结论保持不变：Case 2/3/4 仍存在 `pending placeholder bubble (binding_key=::::)`。
6. [X] 2026-03-09 01:54 CST：合入 `290A` 根因修复后执行“`make librechat-web-build` -> 重启 server/superadmin/kratosstub -> `tp290` 单 spec 复跑”，Case 1~4 全部通过；Case 2/3/4 不再出现 `binding_key=::::` placeholder，`single_assistant_bubble=true`、`official_message_tree_only=true`。

## 4. 命令与结果
1. [X] 前置验证命令：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --grep "tp288-e2e-002"`，结果通过。
2. [X] 前置验证命令：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js`，结果 `2 passed`。
3. [X] 首轮命令：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290-librechat-real-case-matrix.spec.js --workers=1 --trace on`，结果 `4 passed`（测试执行通过，但按 phase 断言 Case 2/3/4 未满足 stopline）。
4. [X] 用例发现命令：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test --list tests/tp290-librechat-real-case-matrix.spec.js`，结果命中 `tp290-e2e-001~004`。
5. [X] 文档门禁命令：`make check doc`，结果 `[doc] OK`（2026-03-08 CST）。
6. [X] 多轮复跑命令（同口径）：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290-librechat-real-case-matrix.spec.js --workers=1 --trace on`，结果均为 `4 passed`（测试执行通过，但 phase 断言 Case 2/3/4 仍未满足 stopline）。
7. [X] 回灌复跑命令：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp290-librechat-real-case-matrix.spec.js --workers=1 --trace on`，结果 `4 passed`，且 phase 断言全通过（2026-03-09 01:54 CST）。

## 5. 证据资产索引
1. [X] 证据根目录：`docs/dev-records/assets/dev-plan-290/`
2. [X] 固定命名文件（每个 Case）：
   - `case-{id}-page.png`
   - `case-{id}-dom.json`
   - `case-{id}-network.har`
   - `case-{id}-trace.zip`
   - `case-{id}-phase-assertions.json`
3. [X] 总索引：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`
4. [X] 矩阵副本：`docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`
5. [X] 首轮证据覆盖状态：
   - Case 1：`passed`
   - Case 2：`failed`（pending placeholder bubble）
   - Case 3：`failed`（pending placeholder bubble）
   - Case 4：`failed`（pending placeholder bubble）
6. [X] 最新复跑证据覆盖状态（2026-03-09 01:54 CST）：
   - Case 1：`passed`
   - Case 2：`passed`
   - Case 3：`passed`
   - Case 4：`passed`

## 6. 时效与失效规则
1. [ ] 若 `240C/240D/240E` 发生影响性合入（运行时 gate、路由/认证链路、错误码语义、MCP 写能力准入、fail-closed），本日志中的历史证据视为失效，必须重跑并刷新索引。
2. [ ] 若 `288` 未形成可复核复跑证据，本计划不得标记“已完成”。
3. [ ] 若出现与 `280` 主计划冲突的验收结论（双入口、外挂回执、前端重算 FSM），该轮 Case 结果直接作废。

## 7. 与 260 历史日志关系
1. [X] 本日志是 `260-M5` 唯一执行日志入口。
2. [ ] `docs/archive/dev-records/dev-plan-260-execution-log.md` 仅保留历史阶段记录与本日志链接，不再混写新口径执行细节。
