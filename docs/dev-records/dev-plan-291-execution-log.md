# DEV-PLAN-291 执行日志（237 升级兼容回归前置专项）

**状态**: 已执行（2026-03-08 CST；`R1~R8/R10` 通过，`R9` 未通过；`291` 当前不能作为 `285` 的通过前置件）

## 1. 本轮执行摘要
1. [X] 已按 `291-v2` 固定顺序执行 `source -> build -> runtime -> version-lock -> formal-entry -> compat-alias -> routing -> no-legacy -> 引用新鲜度 -> runtime-down`。
2. [X] 已创建证据目录：`docs/dev-records/assets/dev-plan-291/`。
3. [X] 已生成矩阵、执行报告、风险清单、证据索引与 `285` 交接包。
4. [ ] `R9` 未通过：`290` 当前仍存在 Case 2/3/4 stopline 失败，阻断 `291` 作为通过件交接给 `285`。

## 2. 关键结论
1. [X] `237` 对应的 source/runtime/entry-boundary compatibility 已形成可复核证据。
2. [X] `292` compat alias 边界已被复核，未形成第二正式 API 面。
3. [ ] `280` 核心硬门槛无法仅凭当前 `288/290` 证据判定全部成立；`290` 仍需修复后重跑。

## 3. 证据索引
- `docs/dev-records/assets/dev-plan-291/291-upgrade-compat-matrix.md`
- `docs/dev-records/assets/dev-plan-291/291-execution-report.md`
- `docs/dev-records/assets/dev-plan-291/291-risk-list.md`
- `docs/dev-records/assets/dev-plan-291/291-evidence-index.json`
- `docs/dev-records/assets/dev-plan-291/291-handoff-to-285.md`
- `docs/dev-records/assets/dev-plan-291/291-ref-288-290-freshness.md`
