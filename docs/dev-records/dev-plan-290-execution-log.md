# DEV-PLAN-290 执行日志（260-M5 真实 Case 验收与证据固化）

**状态**: 准备中（2026-03-08 CST；`289/292` 已完成，`288` 已完成默认基线复跑并通过 `tp288-e2e-001/002`，当前进入 Case Matrix v1 执行准备）

## 1. 执行范围（与 290 对齐）
1. [ ] 按 `Case Matrix v1` 执行 Case 1~4 真实验收（固定输入向量，不临场改词）。
2. [ ] 每个 Case 固化页面、DOM、网络、trace、phase 断言证据。
3. [ ] 输出面向 `285` 的 `260-M5` 收口结论。

## 2. 固定输入口径
1. [ ] 唯一输入口径：`docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md` 第 4.1 节 `Case Matrix v1`。
2. [ ] 默认确认词口径：`确认` / `是的`。
3. [ ] Case 4 仅在候选顺序不稳定时允许使用候选编码，且必须记录原因。

## 3. 执行记录
1. [X] 启动门槛已满足：`292 -> 288 -> 290` 前置链路前两段已关闭，`tp288-e2e-001/002` 已通过。
2. [ ] 待执行 Case 1~4 首轮真实验收并补记结论与异常记录。

## 4. 命令与结果
1. [X] 前置验证命令：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js --grep "tp288-e2e-002"`，结果通过。
2. [X] 前置验证命令：`pnpm --dir /home/lee/Projects/Bugs-And-Blossoms/e2e exec playwright test tests/tp288-librechat-real-entry-evidence.spec.js`，结果 `2 passed`。
3. [ ] `290` 正式验收命令待补记（Case 1~4、退出码、关键日志摘要）。

## 5. 证据资产索引
1. [ ] 证据根目录：`docs/dev-records/assets/dev-plan-290/`
2. [ ] 固定命名文件（每个 Case）：
   - `case-{id}-page.png`
   - `case-{id}-dom.json`
   - `case-{id}-network.har`
   - `case-{id}-trace.zip`
   - `case-{id}-phase-assertions.json`
3. [ ] 总索引：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`
4. [ ] 矩阵副本：`docs/dev-records/assets/dev-plan-290/tp290-case-matrix-v1.md`

## 6. 时效与失效规则
1. [ ] 若 `240C/240D/240E` 发生影响性合入（运行时 gate、路由/认证链路、错误码语义、MCP 写能力准入、fail-closed），本日志中的历史证据视为失效，必须重跑并刷新索引。
2. [ ] 若 `288` 未形成可复核复跑证据，本计划不得标记“已完成”。

## 7. 与 260 历史日志关系
1. [ ] 本日志是 `260-M5` 唯一执行日志入口。
2. [ ] `docs/archive/dev-records/dev-plan-260-execution-log.md` 仅保留历史阶段记录与本日志链接，不再混写新口径执行细节。
