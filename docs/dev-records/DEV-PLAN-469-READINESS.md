# DEV-PLAN-469 Readiness

## 说明

- 本文件用于登记 `DEV-PLAN-469` 的阶段性实现状态、自动化验证、浏览器 smoke 与后续待补证据。
- 当前只记录已经实际完成的 `Phase 1 / No-Summary Baseline` 收口，不提前宣称 `Phase 2 / 模型摘要 fallback` 已落地。
- 2026-04-26 起，`Phase 2 / Model Summary Fallback` 与 `Phase 3 / Remote Compact Capability` 暂停实施，不再作为当前主线。

## 2026-04-25 实施记录

### Phase 1 / No-Summary Baseline 收口

- Go 主链已从 `CompactConversation(...)` 收口为 `PrepareConversationPromptView(...)`：
  - store 在事务内读取 canonical events
  - 重建完整历史视图
  - 重新注入 canonical context
  - 返回 provider prompt view
- pre-turn auto 主链继续只走 prompt view 准备，不再向 provider prompt view 注入本地拼接 `summary_text`。
- gateway pre-turn 失败文案已改为“会话上下文准备失败”，避免把当前主链误称为“会话压缩失败”。
- `turn.context_compacted` 在当前阶段不再作为运行时产物伪造写入；仅保留历史回放兼容语义。
- Web UI 已把当前运行时表述从“压缩摘要”收口为“历史上下文”，避免把历史兼容项误写成现行产品能力。

### 自动化验证

- 命令：`go test ./modules/cubebox ./internal/server`
- 结果：通过
- 覆盖重点：
  - prompt view builder 改为完整历史视图 + canonical context
  - 历史 `turn.context_compacted` 不再回放进 provider prompt view
  - store `PrepareConversationPromptView(...)` 不再写运行时 compact event
  - gateway / API stub / query flow 对新返回类型与失败文案保持一致

- 命令：`pnpm --dir apps/web exec vitest run src/pages/cubebox/reducer.test.ts src/pages/cubebox/CubeBoxPanel.restore.test.tsx src/pages/cubebox/CubeBoxPanel.test.tsx`
- 结果：通过
- 覆盖重点：
  - 历史 `turn.context_compacted` 在前端仍可回放
  - timeline item 已从 `compact_item` 收口为 `history_context_item`
  - UI 文案已从“压缩摘要”改为“历史上下文”

- 命令：`pnpm --dir apps/web exec tsc -b --pretty false`
- 结果：通过

- 命令：`make css`
- 结果：通过
- 说明：已同步重建 `internal/server/assets/web/**` 嵌入式前端产物。

- 命令：`make check doc`
- 结果：通过

### 浏览器 smoke

- 状态：通过（2026-04-25 23:45 CST）。
- 环境：
  - 使用 `tools/codex/skills/bugs-and-blossoms-dev-login/scripts/start_dev_runtime.sh` 重启本地运行时。
  - 运行时入口：`http://localhost:8080/app`
  - 登录账号：`admin@localhost`
- 验证点：
  - 主应用壳层可正常打开，CubeBox 抽屉可见且可展开。
  - CubeBox 会话时间线、输入框、发送按钮与基本发送链路正常。
  - 输入 `查一下 100000 在 2026-04-25 的组织详情` 后，页面进入流式处理中并最终返回组织详情回答。
  - live flow 未出现把当前运行时能力误表述为“压缩摘要”的可见文案。
- 非阻断现象：
  - 浏览器控制台仅见 `GET /favicon.ico -> 404`，不影响本次 CubeBox 验证。

## 当前结论

- `469 Phase 1 / No-Summary Baseline` 已完成代码与页面层收口。
- 当前 provider prompt view 已回到“完整历史视图 + canonical context + 当前 user input”基线。
- `469` 当前不再推进 `Phase 2 / Model Summary Fallback` 与 `Phase 3 / Remote Compact Capability`；两者自 2026-04-26 起暂停实施，待未来单独重启。

## 若未来重启需补证据

- `Phase 2 / Model Summary Fallback` 的契约冻结、prompt shape fixture 与真实 provider smoke。
- `remote compact capability` 的 adapter 能力验证与错误映射证据。
