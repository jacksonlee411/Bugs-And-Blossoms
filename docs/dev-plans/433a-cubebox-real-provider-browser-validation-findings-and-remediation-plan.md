# DEV-PLAN-433A：CubeBox 真实 Provider 浏览器验证发现与修复方案

**状态**: 当前范围已重新验收通过；真实浏览器 success/interrupted 证据与自动化验证已完成，破坏性 credential 页面用例仍待补证（2026-04-22 21:39 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`（命中权限、失败恢复、多模型治理、真实 provider 外部依赖与用户可见链路）
- **范围一句话**：记录 `DEV-PLAN-433` 真实 provider 浏览器验证中暴露的问题，冻结修复方案、owner 边界、停止线与验收证据，确保 CubeBox 的真实 provider 主链、settings/verify、历史恢复、compact、权限 gating 和 fail-closed 行为可被真实用户页面链路验证。
- **关联模块/目录**：`docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`、`docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`、`docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`、`apps/web/src/pages/cubebox`、`internal/server`、`modules/cubebox`
- **关联计划/标准**：`DEV-PLAN-004M1`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-022`、`DEV-PLAN-300`、`DEV-PLAN-433`、`DEV-PLAN-435`、`DEV-PLAN-437A`
- **用户入口/触点**：登录页、主应用壳层 CubeBox 右侧抽屉、settings 弹窗、`turns:stream`、`settings/verify`、历史恢复、compact、停止按钮
- **证据记录 SSOT**：实施与复验证据统一回填 `docs/dev-records/DEV-PLAN-437-READINESS.md` 的 CubeBox Phase 记录段；若 433A 后续独立成批交付，再拆出 `docs/dev-records/DEV-PLAN-433A-READINESS.md`，不得把浏览器截图、网络证据和修复结论散落在 PR 评论或本地临时文件。

### 0.1 Simple > Easy 三问

1. **边界**：本计划只处理本次真实页面验证暴露出的 `433` 运行时闭环缺口与必要的前端权限/状态修复；不重新定义完整模型配置管理面，不引入 fallback/failover/quota/default model。
2. **不变量**：真实 provider 主链必须保持单链路，错误终态也必须进入 canonical event log；页面权限 gating 不能只依赖构建期 `VITE_PERMISSIONS`；settings/verify 的 health 回写必须与真实 turn 主链可解释地一致。
3. **可解释**：reviewer 必须能在 5 分钟内说明失败 turn 为什么不会在恢复后卡成 streaming、verify 为什么与 turn 成败口径一致、viewer 为什么看不到不可用 settings 操作、compact 后为什么下一轮仍走 `openai-chat-completions`。

## 1. 背景

2026-04-22 对 `DEV-PLAN-433` 真实 provider 主链进行了浏览器级验证：从真实登录页进入主应用壳层，打开 CubeBox 右侧抽屉，抓取 `/internal/cubebox/conversations`、`/internal/cubebox/turns:stream`、`/internal/cubebox/settings`、`/internal/cubebox/settings/verify` 网络请求，并观察 UI 状态。

验证确认了首轮主链已经能走真实 provider：

- `turn.started` 带 `trace_id`、`provider_id=openai-compatible`、`provider_type=openai-compatible`、`model_slug`、`runtime=openai-chat-completions`。
- `gpt-5.2` 配置下，`turns:stream` 可返回 `turn.started -> turn.user_message.accepted -> turn.agent_message.delta -> turn.agent_message.completed -> turn.completed`。
- provider disabled 时，页面和 SSE 只返回仓内错误语义，未泄露上游原始错误。
- compact 后下一轮 turn 仍可走 `openai-chat-completions`。

同时暴露出几个必须收口的问题：失败 turn 终态未落库导致历史恢复卡 streaming、settings/verify 与真实 turn 成功口径不一致、viewer 仍可打开 settings UI、compact prompt view 仍写 `model=deterministic-runtime`、部分页面按钮状态因历史失败会话恢复不完整而阻断后续验证。

## 2. 真实验证发现

| ID | 严重级别 | 发现 | 证据摘要 | 影响 |
| --- | --- | --- | --- | --- |
| `433A-F01` | `P0` | 失败 turn 的 `turn.error` / `turn.completed` 未落库 | 失败会话 replay 仅有 `conversation.loaded`、`turn.started`、`turn.user_message.accepted`，没有 terminal events；页面恢复后显示“流式处理中” | 历史恢复、重开抽屉、新建、发送、compact、停止验证均可能被卡住 |
| `433A-F02` | `P0` | settings/verify 与真实 turn 主链结果不一致 | `gpt-5.2` 的真实 turn 可 delta/completed，但 `/settings/verify` 返回 `422 failed / provider verify failed` 并写回 failed health | 管理面 health 误导用户，真实可用模型被标为失败 |
| `433A-F03` | `P1` | viewer 可见并可打开 settings 弹窗 | viewer 后端 `/settings` 与 `/settings/verify` 返回 `403 forbidden`，但 UI 仍显示 settings 按钮和表单 | 前端 gating 与后端 Authz 不一致，用户可见不可用入口 |
| `433A-F04` | `P1` | compact prompt view 仍标记 `model=deterministic-runtime` | compact response 的 system context 中 `model=deterministic-runtime`，但后续 turn 实际为 `runtime=openai-chat-completions` | 审计/上下文语义与真实运行时不一致 |
| `433A-F05` | `P2` | 停止按钮冒烟受失败恢复状态影响 | 失败历史恢复后新建/发送按钮 disabled，停止按钮可见但无法形成完整中断验证闭环 | 中断链路仍需在终态落库修复后补完整页面验证 |
| `433A-F06` | `P2` | `secret_ref` 无法解析 / credential inactive 未做破坏性页面验证 | 现有 UI 轮换 credential 会新增版本且停用旧版本，未在本次验证中修改原始 credential 链路 | fail-closed 覆盖还缺 credential 侧受控用例 |

## 3. 根因与修复方向

### 3.1 `433A-F01`：失败终态未落库

当前 `GatewayService.writeTerminalError` 只通过 sink 写出 fallback SSE，并未通过 store 追加 canonical event。函数签名上游已把 `store` 传入 `writeConfigError` / `writeProviderError`，但 terminal 写入路径未使用 store，导致客户端能看到错误 SSE，而恢复读取看不到 `turn.error` / `turn.completed`。

修复方向：

- 将 terminal error 写入收敛为与正常事件相同的 append-first 流程：先 `AppendEvent`，再写 SSE。
- `turn.error` 与随后 `turn.completed(status=failed)` 必须都落库。
- 如果 terminal event 落库失败，才允许写出 `event_log_write_failed` fallback SSE；不得把 provider/config 错误当成只写网络、不写事件的特殊分支。
- 增加失败恢复测试：失败 turn replay 后 reducer 状态必须为 `error`，按钮不得卡在 streaming。

冻结契约：

- terminal 写入由一个内部 helper 承接，例如 `appendTerminalEvents`；调用方只能提供 `turn_id`、`trace_id`、`provider_id`、`provider_type`、`model_slug`、`runtime`、`latency_ms`、错误码与 terminal status，不能绕过 helper 单独写 `turn.error` 或 `turn.completed`。
- `turn.error` 与 `turn.completed(status=failed)` 必须在同一 store append 事务或同一不可分割 append 调用中完成；如果当前 store 只能逐条 append，则必须先补 store 层批量 append 或在 service 层把第二条失败映射为 `event_log_write_failed`，不得留下只有 `turn.error` 没有 `turn.completed` 的可恢复状态。
- SSE 写出必须发生在事件落库成功之后；若 SSE 写失败但事件已落库，恢复链路以 event log 为准，不再补写第二套网络状态。
- 若 event log 写入失败，不允许伪造 canonical terminal event；只能写出不入库的 `event_log_write_failed` fallback SSE，并在服务端受控日志记录 `trace_id` 与仓内错误码。
- `turn.error`、`turn.completed` 的 sequence 必须沿用 canonical event log 的递增顺序，恢复时 reducer 看到 `turn.completed(status=failed)` 后必须关闭 streaming 状态。

### 3.2 `433A-F02`：verify 与 turn 口径不一致

真实 turn 使用同一 provider/model 能成功流式返回 delta，但 `VerifyActiveModel` 对同一 active model 返回 failed health。说明 verify 对上游 stream 的判断、错误分类或 health prompt / timeout 策略与主链不一致。

修复方向：

- `VerifyActiveModel` 必须复用同一 `ProviderAdapter` 与同一错误归一化路径。
- verify 应读取 stream 直到出现第一段非空 delta、明确 done、明确错误或达到短超时；不得因首包为空或 provider 的非内容事件误判为失败。
- verify 失败时的 `error_summary` 只允许仓内归一化枚举，不写上游原始 body。
- 对同一 fake provider 增加测试矩阵：首包空 delta 后第二包内容、done without content、401、429、5xx、invalid stream。
- 真实 provider smoke 中要求：同一 active model、同一 provider adapter、同一 stream parser、同一错误归一化路径下，已成功返回 delta/completed 的 turn 不得被 verify 记录为泛化的 `failed/provider verify failed`；若 verify 与 turn 结果差异可被解释，必须在 health 中用 `degraded` 或 `failed` 加仓内归一化原因表达。

一致性边界：

- 本计划不要求真实 provider 在任意时间点对 turn 与 verify 绝对同结果；网络抖动、上游限流、短超时、health prompt 被拒绝等可以导致差异。
- 必须消除的是本仓内部判断链路差异：verify 不得使用第二套 provider client、第二套 SSE parser、第二套错误码或第二套模型配置读取逻辑。
- health 状态允许值首轮收敛为 `healthy`、`degraded`、`failed`、`unknown`；`error_summary` 只能使用仓内枚举或 i18n key，如 `provider_stream_timeout`、`provider_auth_failed`、`provider_rate_limited`、`provider_stream_invalid`、`provider_empty_response`。
- `latency_ms` 记录 verify 请求开始到判定 health 结果的服务端壁钟时间；不得复用上一轮 turn 的 latency。

### 3.3 `433A-F03`：settings 权限 gating 不一致

当前前端权限判断来自 `AppPreferencesProvider` 的构建期环境变量，默认 `*`；viewer 登录后仍可看到 settings 按钮。后端 Authz 正确返回 403，但用户入口不应可见或应禁用。

修复方向：

- 前端会话上下文必须从后端会话 / principal 能力读取权限，不能只用 `VITE_PERMISSIONS` 决定真实用户权限。
- `CubeBoxPanel` 的 settings 按钮应基于真实权限隐藏；若后端返回 403，settings dialog 不能展示完整可编辑表单。
- 在 `435` 管理面正式封板前，最小修复可采用后端提供的 session capabilities 只读接口或现有 session payload 扩展，但不得引入 legacy fallback。

最小权限契约：

- 后端 owner：`internal/server` 只负责把当前 session principal 经 Authz 计算后的能力返回给前端；权限判定事实仍归 Casbin/Authz，前端不得自行推导角色。
- 前端 owner：`apps/web` 只消费能力布尔值控制入口显隐和按钮禁用；后端 API 仍必须独立执行 Authz，不能因前端隐藏按钮而放松 403。
- 最小 DTO 采用能力形状，不新增第二套角色模型：

```ts
type CubeBoxCapabilities = {
  conversation: {
    read: boolean
    use: boolean
  }
  settings: {
    read: boolean
    verify: boolean
    select: boolean
    update: boolean
    rotate: boolean
    deactivate: boolean
  }
}
```

- `settings.read=false` 时，主壳层 CubeBox settings 入口隐藏或禁用；直接打开 dialog 或 API 返回 403 时，页面只显示无权限占位，不加载 provider、credential 或 health 明细。
- 能力加载失败时按 fail-closed 处理：conversation 入口可按已存在会话读取错误展示空态，但 settings 管理入口不得默认可见。
- `VITE_PERMISSIONS` 只允许继续作为本地开发假 principal 的测试夹具，不得参与真实登录用户的 production 权限判定；实施 PR 必须标出退场点或限定使用条件。

### 3.4 `433A-F04`：compact context 运行时标识不一致

compact 当前仍由旧 deterministic 口径填充 `CanonicalContext.Model`。即使 compact 后下一轮 turn 可走真实 provider，prompt view 中的 model 字段仍会误导审计和上下文重放。

修复方向：

- compact context 必须从当前 active model runtime config 派生 `provider_id`、`provider_type`、`model_slug`、`runtime`，并沿用 `DEV-PLAN-437A` 的字段分离口径；不得把 `runtime/provider/model` 拼成只给 prompt view 使用的第二套字符串。
- 若 runtime config 不可用，应 fail-closed 或写明确的 `runtime=unavailable` / `model_slug=unavailable` 仓内状态，且该状态不得被下一轮 turn 当成可执行 runtime。
- compact 后下一轮 turn 必须继续使用当前 active model；不得由 compact 触发 deterministic 回流。

prompt view 兼容规则：

- 若旧 prompt view 仍只有 `CanonicalContext.Model` 单字段，则它只能作为展示层派生字段，格式由 `runtime + ":" + provider_id + "/" + model_slug` 生成；canonical event、store、reducer 和下一轮 turn 读取都不得依赖该展示字段。
- `turn.context_compacted` 仍按 `DEV-PLAN-437A` 保持 `summary_id`、`source_range` 最小 payload；runtime/model metadata 可挂在 compact item 的 metadata 中，但不得改写事件名或新增只供恢复页使用的 compact DTO。

### 3.5 `433A-F05`：中断链路补验

当前中断验证被失败历史恢复状态阻断，必须在 `F01` 修复后重跑。

修复方向：

- 使用真实可流式输出较长内容的 provider/model 触发 streaming。
- 点击“停止”必须发出 `/internal/cubebox/turns/{turn_id}:interrupt`。
- SSE 或 replay 中必须出现 `turn.interrupted` 和 `turn.completed(status=interrupted)`，页面状态收敛为“已中断”。

### 3.6 `433A-F06`：credential fail-closed 受控验证

credential 轮换目前会改变真实环境版本，直接做 destructive UI 验证风险较高。

修复方向：

- 增加测试专用 provider 或测试专用 credential，不复用生产样式 `openai-compatible` 主配置。
- 覆盖 `secret_ref` 非 `env://`、env 缺失、credential inactive 三种路径。
- 验证页面只看到 `ai_model_secret_missing` 对应仓内文案，不泄露 `secret_ref` 原始解析错误和上游响应。

测试数据治理：

- 本计划不新增数据库表；如需测试 provider/credential，只允许新增或更新测试租户内的测试数据。
- 测试 provider id 建议使用 `openai-compatible-test-invalid`，不得覆盖真实 `openai-compatible` 主配置。
- 破坏性用例执行前必须记录原始 active model、provider enabled 状态、credential active 状态；执行后必须恢复，复验恢复后的真实 provider turn 仍成功。
- `secret_ref` 破坏性用例只能使用无真实密钥含义的测试值，如 `env://CUBEBOX_TEST_MISSING_KEY` 或仓内非法 scheme；不得提交真实 key、真实 `secret_ref` 或上游原始错误 body。

## 4. 实施步骤

1. [x] P0：修复 terminal error 落库路径
   - `writeConfigError` / `writeProviderError` / `writeTerminalError` 必须通过 `AppendEvent` 写入 `turn.error` 与 `turn.completed(status=failed)`。
   - 引入单一 terminal append helper，保证 `turn.error` 与 `turn.completed(status=failed)` 的写入不可被调用方拆散。
   - 增加 store append 失败的 fallback 测试，确保 fallback 仅用于事件落库失败。
   - 增加 replay/reducer 测试：失败 turn 恢复后状态为 `error`，不是 `streaming`。

2. [x] P0：修复 settings/verify 与真实 turn 主链不一致
   - 统一 provider adapter、stream parser 与错误归一化。
   - 补齐 health 测试矩阵。
   - 真实 provider smoke 中记录 verify 前后 health，并与 turn 结果做一致性断言。
   - 真实 provider smoke 已执行；当前同一 active model 下 verify 与 turn 都通过同一真实 provider adapter 超时，并归一化为仓内 `provider_stream_timeout` / `ai_model_provider_unavailable`。

3. [x] P1：修复 viewer settings 可见性
   - 冻结并实现 `CubeBoxCapabilities` 最小能力 DTO。
   - viewer 不应看到 settings 入口；若直接访问 settings API，后端仍保持 403。
   - 管理动作 `verify/select/update/rotate/deactivate` 继续按后端 Authz fail-closed。

4. [x] P1：修复 compact context 中的 runtime/model 字段
   - compact context 从 active runtime config 派生 `provider_id`、`provider_type`、`model_slug`、`runtime` 分离字段。
   - compact 后下一轮 turn 继续验证 `runtime=openai-chat-completions`。

5. [~] P2：补完整中断与历史恢复验证
   - 成功 turn、失败 turn、中断 turn 都必须可恢复且按钮状态正确。
   - 重开抽屉后最新会话不丢，历史列表可选中并恢复。
   - 失败 turn 已通过真实浏览器 + DB replay 验证可恢复为 terminal `failed`；成功 turn 与中断 turn 仍受真实上游超时阻塞，待 provider 连通后补验。

6. [~] P2：补 credential fail-closed 受控用例
   - 使用测试专用 provider/credential。
   - 覆盖 disabled provider、inactive credential、invalid `secret_ref`、missing env。
   - 结束后恢复原始 active model 与 provider enabled 状态。
   - 自动化已覆盖 invalid/missing secret、provider disabled、credential missing/inactive；破坏性页面验证仍需测试专用 provider/credential 环境。

7. [x] P2：回填证据记录
   - 将修复前后 Playwright 网络证据、SSE lifecycle 摘要、settings/verify 前后 health、权限矩阵和 fail-closed 用例结果回填 `docs/dev-records/DEV-PLAN-437-READINESS.md`。
   - 证据只记录仓内错误码、trace_id、runtime、provider_id、model_slug、latency_ms 和截图路径；不得记录真实密钥、完整上游 body 或原始 `secret_ref` 解析细节。
   - 本轮已回填自动化证据与真实浏览器 Playwright 网络证据；证据只保留脱敏后的 runtime、provider、model、health 与 lifecycle 摘要。

### 4.1 本轮代码实施摘要

- `GatewayService` 统一承接真实 provider stream、deterministic fixture 测试路径、terminal error append-first 和 lifecycle payload；真实 provider 配置失败时 `runtime=unavailable`，不会标成 `deterministic-fixture`。
- `Store.AppendEvents` 提供同事务批量事件写入，保证 `turn.error` 与 `turn.completed(status=failed)` 不被拆散。
- `ModelVerificationService` 复用 `ProviderAdapter` / stream parser / 错误归一化，并通过 `RecordModelHealthCheck` 写回真实 health。
- `/internal/cubebox/capabilities` 提供基于当前 session principal 与 Authz 的最小能力 DTO；前端 settings 入口 fail-closed 隐藏，不再依赖构建期权限作为真实用户管理权限。
- compact canonical context 已拆出 `provider_id`、`provider_type`、`model_slug`、`runtime`，旧 `model` 仅作为展示派生字段。
- 未新增数据库表；未恢复旧 `/app/cubebox` 整页入口；未引入 fallback/failover/default model。

### 4.2 真实浏览器复验摘要（2026-04-22）

- **时间语义说明**：以下 `4.2` 条目保留 `2026-04-22 20:17 CST` 复验当时的浏览器/网络证据快照。当前运行时基线已切换并冻结为 `provider_id=openai-compatible`、`provider_type=codex`、`base_url=https://code2026.pumpkinai.vip/v1`、`enabled=true`、`secret_ref=env://OPENAI_API_KEY`、`model_slug=gpt-5.2`；当前基线以 `DEV-PLAN-433` 为准，不应用本节旧快照回写当前配置判断。
- **环境**：已执行 `make dev-down && make dev-up`、`make iam migrate up && make orgunit migrate up`，并启动 `make dev-kratos-stub` 与 `make dev-server`；`http://localhost:8080/health` 返回 `ok`。
- **登录与入口**：Playwright 真实浏览器访问 `http://localhost:8080/app/login`，`admin@localhost` 登录后进入 `/app`；主壳层 CubeBox 按钮可见，右侧抽屉 `role=complementary` 可见；旧整页入口 `a[href*="cubebox"]` 计数为 `0`。
- **权限与 capabilities**：`GET /internal/cubebox/capabilities` 返回 `200`，tenant-admin 的 `conversation.read/use` 与 `settings.read/verify/select/update/rotate/deactivate` 均为 `true`。
- **settings 读写**：通过浏览器同源 session 执行 `GET /internal/cubebox/settings`、`POST /settings/providers`、`POST /settings/credentials`、`POST /settings/selection` 均成功；active model 回显为 `provider_id=openai-compatible`、`provider_type=openai-compatible`、`base_url=https://api.openai.com/v1`。
- **verify 回写**：`POST /internal/cubebox/settings/verify` 返回 `200`；health 从 `validated_at=2026-04-22T12:01:03Z / latency_ms=30012 / error_summary=provider_stream_timeout` 更新为 `validated_at=2026-04-22T12:17:30Z / latency_ms=30002 / error_summary=provider_stream_timeout`，说明 verify 走真实 provider 校验并写回 health。
- **真实 turn 主链**：`POST /internal/cubebox/turns:stream` 返回 `200 text/event-stream`；SSE 包含 `turn.started -> turn.user_message.accepted -> turn.error -> turn.completed(status=failed)`，`turn.started/error/completed` 均带 `runtime=openai-chat-completions`、`trace_id`、`provider_id=openai-compatible`、`provider_type=openai-compatible`、`model_slug=gpt-4.1`。
- **DB replay 证据**：新会话 `conv_4a32db7ea99e4fb5b171bdd0e137ad4d` 已落库 sequence 1-5：`conversation.loaded`、`turn.started`、`turn.user_message.accepted`、`turn.error(code=ai_model_provider_unavailable, latency_ms=30013)`、`turn.completed(status=failed, latency_ms=30013)`。
- **收尾状态**：复验结束后已按要求保留真实 credential 引用，并将 active selection 恢复为 `model_slug=gpt-5.2`；provider 仍为 `openai-compatible`、`base_url=https://api.openai.com/v1`、`enabled=true`，active credential version 为 `4` 且未输出真实 key。
- **当前阻塞**：真实 provider 外网/上游链路当前 30 秒超时，未拿到成功 `turn.agent_message.delta`；本轮不能判定“成功 turn”通过，但可以确认未回退 deterministic fixture，且失败路径 fail-closed、terminal event 与 health 回写已正确收敛。

### 4.3 真实 success/interrupted 补证摘要（2026-04-22 21:32-21:33 CST）

- **当前 settings 实时回显**：浏览器同源 `GET /internal/cubebox/settings` 返回 `provider.id=openai-compatible`、`provider.provider_type=openai-compatible`、`provider.display_name=codex`、`base_url=https://code2026.pumpkinai.vip/v1`、`selection.model_slug=gpt-5.2`、`health.status=healthy`、`latency_ms=3377`、`validated_at=2026-04-22T12:32:45Z`。这说明当前 UI 回显事实已切到 `pumpkinai` 基线，但 `provider_type/display_name` 与 `433/5D.1` 的冻结口径需要并行记录，不能混写成单字段事实。
- **真实 success 页面证据**：`POST /internal/cubebox/turns:stream => 200`，request body 为 `{"conversation_id":"conv_1db58863b0764207a855cb84ca33e47c","prompt":"请用中文写一句简短问候，只回复一句。","next_sequence":642}`；页面最终显示用户消息 `请用中文写一句简短问候，只回复一句。`、助手消息 `你好，祝你今天一切顺利。`，两条状态均为 `completed`。
- **真实 interrupted 页面证据**：在新会话 `conv_b223bd5689fb48b3ab2c7434ce952318` 中发起长输出请求后，先出现 streaming 状态，再触发 `POST /internal/cubebox/turns/turn_000037:interrupt?conversation_id=conv_b223bd5689fb48b3ab2c7434ce952318 => 200`，request body 为 `{"reason":"user_requested"}`；页面最终显示 `已中断`，消息状态为 `interrupted`。
- **真实 interrupted replay 证据**：同源 `GET /internal/cubebox/conversations/conv_b223bd5689fb48b3ab2c7434ce952318` 最后事件包含 `turn.interrupted(reason=user_requested, runtime=openai-chat-completions, provider_id=openai-compatible, provider_type=openai-compatible, model_slug=gpt-5.2, latency_ms=28356)` 与 `turn.completed(status=interrupted, latency_ms=28376)`，两者共享同一 `trace_id`。

## 5. 验收标准

| 验收项 | 必须证据 |
| --- | --- |
| 登录与会话 | 浏览器登录返回 `204`，后续 `/internal/cubebox/**` 无 401 |
| 入口与页面 | 主壳层右侧抽屉可打开，`a[href*="cubebox"]` 旧整页入口计数为 0 |
| 成功 turn | SSE 包含 `turn.started -> turn.agent_message.delta -> turn.agent_message.completed -> turn.completed(status=completed)` |
| 真实 provider | `runtime=openai-chat-completions`，不等于 `deterministic-fixture` |
| 失败 turn | SSE 与 replay 均包含 `turn.error` 和 `turn.completed(status=failed)`，页面恢复后不显示 streaming |
| verify | `settings/verify` 写回 health；重新打开 settings 可见一致的 `status`、`latency_ms`、`validated_at`、`error_summary` |
| verify / turn 一致性 | 同 active model 下 verify 与 turn 复用 adapter、stream parser、错误归一化；差异必须落到 `degraded` / `failed` 加仓内原因 |
| 配置即时生效 | 修改 active model 后，下一次 turn 的 `model_slug` 立即变化，无需刷新页面 |
| fail-closed | disabled provider / invalid secret / inactive credential 只显示仓内错误语义 |
| 历史恢复 | 重开抽屉后会话、消息、terminal 状态不丢 |
| compact | compact 后下一轮 turn 仍走 `openai-chat-completions`，compact context 使用 `provider_id`、`provider_type`、`model_slug`、`runtime` 分离字段且不写 deterministic runtime |
| 权限 | tenant-admin 可用 settings；tenant-viewer 可用 conversation read/use，但 settings 入口隐藏或禁用，直接 API 为 403 |
| 证据记录 | 复验结果回填 `docs/dev-records/DEV-PLAN-437-READINESS.md`，并同时包含 UI 表现与网络证据 |

## 5A. 2026-04-22 重新验收记录

本次重新验收针对 `433A` 已实现范围重跑自动化与门禁，并据此重新裁决“当前是否可算开发完成”。

本次实际执行命令：

- `go test ./modules/cubebox ./internal/server`
- `pnpm -C apps/web exec vitest run src/pages/cubebox/api.test.ts src/pages/cubebox/reducer.test.ts src/pages/cubebox/CubeBoxProvider.test.tsx src/pages/cubebox/CubeBoxPanel.test.tsx src/pages/cubebox/CubeBoxPanel.restore.test.tsx`
- `make check doc`
- `make check chat-surface-clean`
- `make check routing`
- `make authz-test`

结果摘要：

- Go 测试通过，证明 `AppendEvents` append-first、`settings/verify` 错误归一化、`runtime=unavailable` fail-closed、terminal fallback only-on-append-failure 等路径仍成立。
- 前端 CubeBox 测试通过，证明 failed turn replay、settings 权限显隐、compact item 恢复、credential deactivate 错误回显与 settings API 交互没有回归。
- `chat-surface-clean`、`routing`、`authz-test`、`doc` 全部通过，说明 `433A` 的权限 gating、路由和文档口径未发生反漂移。

当前验收裁决：

- 已通过验收：
  - `F01`：失败 turn 的 `turn.error` 与 `turn.completed(status=failed)` 已稳定落库，恢复后不再留下 dangling streaming。
  - `F02`：`settings/verify` 已复用真实 provider adapter / stream parser / 错误归一化，并真实写回 health。
  - `F03`：settings 入口已按 `/internal/cubebox/capabilities` fail-closed 控制，viewer 不再依赖构建期权限看到可操作入口。
  - `F04`：compact context 已使用 `provider_id / provider_type / model_slug / runtime` 分离字段，不再把 deterministic runtime 当成当前真实运行时。
  - `F05`：真实 success/interrupted 页面与 replay 证据已补齐；当前可确认 success、stop/interrupt 与终态恢复行为都走真实 provider 主链，而非 deterministic fixture。
  - 自动化与门禁证据已重跑通过，可继续把 `433A` 视为“当前实现范围已完成”。
- 仍待补证但不推翻当前完成裁决：
  - `F06`：测试专用 provider/credential 的破坏性 fail-closed 页面用例仍未完成，不应误记为已补齐。

结论：

- `DEV-PLAN-433A` 当前应记为“当前范围已重新验收通过”，不是“全部收尾完成”。
- 后续只需补测试专用 credential 破坏性页面样本，无需重新打开已通过的 `F01-F05` 主修复面。

## 6. 测试与覆盖率

- **覆盖率口径**：沿用仓库 Go / 前端现有门禁，不在本计划降低阈值或扩大排除项。
- **文档门禁**：仅修改本计划时至少执行 `make check doc`。
- **Go 门禁**：命中 `internal/server`、`modules/cubebox` 或 Go 测试时执行 `go fmt ./... && go vet ./... && make check lint && make test`。
- **前端门禁**：命中 `apps/web` 的 CubeBox UI、capabilities 读取或 reducer 时执行对应前端测试；若触及 MUI/presentation assets 或生成物，按仓库入口补跑 `make generate && make css` 并确认生成物状态。
- **后端测试重点**：
  - `GatewayService` 正常流、provider 错误、config 错误、store append 失败。
  - `ModelVerificationService` health success/degraded/failed 矩阵。
  - `Store` replay 包含 terminal events。
  - Authz：viewer settings/verify 继续 403。
- **前端测试重点**：
  - reducer replay failed turn 后状态为 `error`。
  - settings 入口按真实权限隐藏或禁用。
  - failed / completed / interrupted 三种 terminal 状态恢复后按钮状态正确。
- **浏览器 smoke**：
  - 使用 Playwright 走真实登录页和右侧抽屉。
  - 同时抓 `/internal/cubebox/conversations`、`turns:stream`、`settings`、`settings/verify`。
  - 真实 provider 连通性仍作为 readiness/smoke 证据，不作为 required gate 的唯一前置；required gate 继续使用 fake provider / mock SSE。
  - 每个 smoke 结论必须以“UI 表现 + 网络证据”双证据记录，不得仅凭 toast 或文案判定通过。

## 7. 停止线

- 不新增数据库表，除非用户手工确认。
- 不为 fail-closed 用例修改真实主 provider / credential 的不可恢复状态；破坏性验证只能使用测试租户或测试 provider/credential。
- 不引入 fallback / failover / route alias / quota / default model。
- 不把真实密钥、完整上游错误 body 或 `secret_ref` 解析细节输出到前端、SSE 或日志证据。
- 不通过降低覆盖率、跳过失败恢复、删除错误用例来规避 `F01/F02`。
- 不恢复旧 `/app/cubebox` 整页入口。
- 不让 deterministic fixture 成为真实 provider 失败时的 runtime fallback。
- 不新增 `vendor`、`channel`、`endpoint config`、`model runtime string` 等与 `DEV-PLAN-435` / `DEV-PLAN-437A` 冲突的平行命名。

## 8. 关联文档

- `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`
- `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
- `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`
