# DEV-PLAN-240F 运行时差量复核报告

- 生成时间：2026-03-09 12:31:22 CST
- 结论：`passed`
- 方法：不复制 `288/290/291` 的全量回归，只复核其交接件、索引、新鲜度与关键运行时边界是否相互一致。

## 差量项 1：正式入口与历史别名边界
- `internal/server/assistant_ui_proxy.go` 显示 `/assistant-ui` 仅允许 `GET/HEAD`，并统一 `302 -> /app/assistant/librechat`；`POST` 等写方法返回 `405`。
- `e2e/tests/tp283-librechat-formal-entry-cutover.spec.js` 已固化上述别名边界断言。
- 结论：未发现第二正式入口或别名扩权为正式写入口的回退。

## 差量项 2：单消息落点与单 assistant 气泡
- `docs/dev-records/assets/dev-plan-266/tp288-handoff-to-285.md` 已固化：
  - `native_send_emitted=0`
  - `official_message_tree_only=true`
  - `single_assistant_bubble=true`
- `docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json` 显示 Case 1~4 全部通过，且仍满足 `266` 共通 stopline 与 `280` 硬门槛。
- 结论：未发现双发送、双回复、外挂消息容器或第二消息事实源回退。

## 差量项 3：DTO-only 与前端降权
- `docs/archive/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md` 已冻结：前端只消费 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` DTO，不再承担业务语义重算。
- `290` 的 Case Matrix 继续以 DTO 驱动的 phase/FSM 为唯一验收口径。
- 结论：`240F` 未发现需要在正式入口恢复 helper 业务推进或页面补丁式状态机的证据。

## 差量项 4：receipt -> poll -> refresh 与人工接管
- `docs/archive/dev-plans/240d-assistant-durable-execution-and-manual-takeover-plan.md` 已完成正式 cutover：
  - `:commit` 返回 `202 + receipt`
  - 客户端轮询 `task`
  - `status=succeeded` 后刷新 conversation
  - `manual_takeover_required` 已具备可见性与最小操作面
- `240F` 本轮未发现任何“同步直返 conversation 重新成为正式主链”的回退证据。
- 结论：`240D` 的正式任务消费语义仍成立。

## 差量项 5：升级兼容与引用新鲜度
- `docs/dev-records/assets/dev-plan-291/291-evidence-index.json` 显示 `R1~R10` 全部通过。
- `docs/dev-records/assets/dev-plan-291/291-ref-288-290-freshness.md` 明确记录 `tp288/tp290` 已在 `240D-03/04` cutover 后重跑，当前仍有效。
- 结论：`291` 仍可作为 `285` 的升级兼容前置件直接引用。

## 综合结论
- [X] `240F` 所需的关键运行时边界之间不存在冲突。
- [X] 本次实施未发现必须回退到 `240C/240D/288/290/291` 再修复的新增缺口。
- [X] 截至 `2026-03-09 12:31:22 CST`，`240F` 可直接进入交接 `285` 阶段。
