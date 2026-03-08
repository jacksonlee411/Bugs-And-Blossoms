# DEV-PLAN-291 -> DEV-PLAN-285 交接清单

- 生成时间：2026-03-08 21:41:02 CST
- 当前结论：`不得作为 285 的通过前置件直接引用；只能作为已执行的阻塞事实引用。`

## 已通过项
- Source 元数据完整：见 `docs/dev-records/assets/dev-plan-291/291-source-verify.log`
- Patch Stack 可重放：见 `docs/dev-records/assets/dev-plan-291/291-web-build.log`
- Runtime 健康：见 `docs/dev-records/assets/dev-plan-291/291-runtime-status.json`
- Formal entry 受保护且可提供静态资源：见 `docs/dev-records/assets/dev-plan-291/291-formal-entry-go-test.log`
- Compat alias 与正式静态 API 基准同链：见 `docs/dev-records/assets/dev-plan-291/291-compat-alias-go-test.log`
- Routing / no-legacy 门禁通过：见 `docs/dev-records/assets/dev-plan-291/291-routing-check.log`、`docs/dev-records/assets/dev-plan-291/291-no-legacy.log`

## 未通过项
- `R9`：`288/290` 引用证据未能共同证明 `DEV-PLAN-280` 核心硬门槛已全部满足。
- 具体阻塞：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json` 仍记录 Case 2/3/4 失败；因此 `single_assistant_bubble` / `dto_only` 不能按 `291-v2` 口径判定为通过。

## Compat alias 状态
- `/app/assistant/librechat/api/**` 当前仍是 `DEV-PLAN-292` 允许的 compat alias。
- 本轮已确认其未形成第二正式 API 面，但在 `285` 启动前仍需保持“只限兼容、不扩语义”的边界。

## 对 285 的建议
- 在 `290` 完成重新复跑并满足完整 Case Matrix 通过前，不要将 `237` 前置标记为完成。
- 若 `290` 后续修复 pending placeholder bubble，需重新执行 `291` 的 `R9` 复核并刷新本交接单。
