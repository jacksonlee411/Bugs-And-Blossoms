# TP290 Case Matrix v1（冻结输入向量）

- 所属计划：`DEV-PLAN-290`
- 对齐基线：`DEV-PLAN-260` Case 1~4、`DEV-PLAN-271` S5-P0、`DEV-PLAN-280` 验收硬门槛
- 更新时间：`2026-03-08 CST`

| Case | 固定输入序列（按轮次） | 关键阶段断言（后端 phase） | 通过判定 |
| --- | --- | --- | --- |
| Case 1 | T1：`你好` | `idle -> idle`（不得进入提交链） | `/app/assistant/librechat` 可发送、可回包，且同轮满足 `266 stopline + 280 单入口约束` |
| Case 2 | T1：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`；T2：`确认` | `idle -> await_commit_confirm -> committing -> committed` | 对话内完成草案、确认、提交、成功回执 |
| Case 3 | T1：`在 AI治理办公室 下新建 人力资源部239A补全`；T2：`生效日期 2026-03-25`；T3：`确认` | `idle -> await_missing_fields -> await_commit_confirm -> committing -> committed` | 对话内提示缺字段、补全后确认并提交成功 |
| Case 4 | T1：`在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26`；T2：`选第2个`（或候选编码）；T3：`是的` | `idle -> await_candidate_pick -> await_candidate_confirm -> await_commit_confirm -> committing -> committed` | 对话内完成候选选择、二次确认并提交成功 |

## 补充规则
1. 默认确认词冻结为 `确认` / `是的`。
2. 仅当候选顺序在运行时不稳定时，Case 4 才允许改用候选编码；必须在执行日志记录原因与实际输入。
3. 任一 Case 输入向量偏离本矩阵且未在执行日志说明，不计入通过统计。
