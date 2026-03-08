# DEV-PLAN-291 R9：`288/290` 引用证据新鲜度与有效性复核

- 执行时间：2026-03-08T17:54:16Z
- 最新相关代码提交时间上界：`2026-03-08T22:17:36+08:00`
- 结论：`failed`

## 引用证据
- `tp288`：`docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`；文件时间：`2026-03-08T22:51:45.930181+08:00`；状态：`completed`
- `tp290`：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`；文件时间：`2026-03-08T20:42:52.088357+08:00`；状态：`in_progress`

## 代码影响提交
- `vendored-ui`：`50c1d40a94e8a71c1ea1b7552b17e0878feb279d 2026-03-08T22:17:36+08:00 fix: dedupe stale assistant formal placeholders`
- `formal-entry`：`50c1d40a94e8a71c1ea1b7552b17e0878feb279d 2026-03-08T22:17:36+08:00 fix: dedupe stale assistant formal placeholders`
- `compat-api`：`50c1d40a94e8a71c1ea1b7552b17e0878feb279d 2026-03-08T22:17:36+08:00 fix: dedupe stale assistant formal placeholders`
- `runtime`：`50c1d40a94e8a71c1ea1b7552b17e0878feb279d 2026-03-08T22:17:36+08:00 fix: dedupe stale assistant formal placeholders`
- `routing`：`50c1d40a94e8a71c1ea1b7552b17e0878feb279d 2026-03-08T22:17:36+08:00 fix: dedupe stale assistant formal placeholders`

## 复核结论
- `tp288` 索引存在，且时间晚于最新相关代码提交；其当前状态已更新为 `completed`，可继续作为 `official_message_tree_only`、`single_assistant_bubble` 与 `no_native_send_post` 的最新引用输入。
- `tp290` 索引存在，但其当前结论仍为 Case 1 通过、Case 2/3/4 失败，无法证明 `single_assistant_bubble` 与 `dto_only` 已在完整 Case Matrix 上满足 `DEV-PLAN-280 §10.2` 硬门槛。
- 因此 `R9` 仍未通过，`291` 不能作为 `285` 的“通过前置件”，只能作为已执行且已固化阻塞事实的专项。
