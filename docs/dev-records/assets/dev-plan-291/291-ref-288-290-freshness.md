# DEV-PLAN-291 R9：`288/290` 引用证据新鲜度与有效性复核

- 执行时间：2026-03-08T13:41:02Z
- 最新相关代码提交时间上界：`2026-03-08T18:18:22+08:00`
- 结论：`failed`

## 引用证据
- `tp288`：`docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json`；文件时间：`2026-03-08T15:58:22.549111+08:00`；状态：`in_progress`
- `tp290`：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json`；文件时间：`2026-03-08T20:42:52.088357+08:00`；状态：`in_progress`

## 代码影响提交
- `vendored-ui`：`996c1112657c420140c5c9e01c559ea0293d3853 2026-03-08T18:18:22+08:00 fix(librechat): preserve formal retry bubbles and refresh dev login skill`
- `formal-entry`：`b09cf7d02b5fe6eaa319e8c9f0a4c306920800ae 2026-03-08T16:46:56+08:00 feat(assistant): sync 288/290 docs and vendored librechat runtime fixes`
- `compat-api`：`b09cf7d02b5fe6eaa319e8c9f0a4c306920800ae 2026-03-08T16:46:56+08:00 feat(assistant): sync 288/290 docs and vendored librechat runtime fixes`
- `runtime`：`0638265eccbcdd5b001fcb197034a7ed4e8cf809 2026-03-06T06:37:51+08:00 feat(assistant): 完成260对话闭环实施与运行态修复`
- `routing`：`b09cf7d02b5fe6eaa319e8c9f0a4c306920800ae 2026-03-08T16:46:56+08:00 feat(assistant): sync 288/290 docs and vendored librechat runtime fixes`

## 复核结论
- `tp288` 索引存在且时间晚于最新 formal-entry / vendored-ui 代码提交，可继续作为 `official_message_tree_only` 与 `no_native_send_post` 的最新引用输入。
- `tp290` 索引存在且时间晚于最新代码提交，但其当前结论仍为 Case 1 通过、Case 2/3/4 失败，无法证明 `single_assistant_bubble` 与 `dto_only` 已在完整 Case Matrix 上满足 `DEV-PLAN-280 §10.2` 硬门槛。
- 因此 `R9` 未通过，`291` 不能作为 `285` 的“通过前置件”，只能作为已执行且已固化阻塞事实的专项。
