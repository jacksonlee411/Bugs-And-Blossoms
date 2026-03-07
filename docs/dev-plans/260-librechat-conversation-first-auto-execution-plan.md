# DEV-PLAN-260：AI对话真实业务闭环主计划（多轮补全 / 候选确认 / 提交回执）

**状态**: 进行中（2026-03-08 CST；已完成 P0 契约冻结与 `DEV-PLAN-289` 的 `M2~M4` 实施收口，当前剩余焦点为 `DEV-PLAN-290` 的真实 Case 证据封板）

> 历史执行记录仍保留在 `docs/archive/dev-records/dev-plan-260-execution-log.md`，但其“已完成”只代表旧口径阶段性实现；**不再等同于当前真实需求已达成**。

## 1. 背景与重开原因
- 用户已明确指出：239A 及其后续落地**偏离真实需求**，没有真正实现“通过 AI 对话形式完成多轮补充 / 确认信息、自动执行操作，并通过对话告诉用户结果”的闭环。
- 当前问题不是单一的文案或截图问题，而是**主计划与子问题边界混乱**：
  1. 旧 `260` 更像一次阶段性收口记录，但没有把“真实业务闭环”与“业务事实源 / 官方 UI 承载面 / 单通道回写”分层冻结；
  2. `266` 主要解决官方 UI 单通道与气泡内回写问题，**不能单独代表业务对话闭环**；
  3. `223` 已承担会话持久化与审计，但在本轮重评估前尚未显式承接 `phase` 级 FSM 与最小 DTO 快照；
  4. `280/284` 已冻结前端降权方向：vendored UI 只能消费后端 DTO，不得继续通过页面 helper / adapter 重算业务阶段与确认约束。
- 本次重开后，计划分工冻结为：
  - **260 = 主计划**：定义真实业务对话闭环、FSM、确认语义、补全语义、候选语义、自动执行时机与最小 DTO 契约。
  - **223 = 业务事实源子计划**：负责把 `conversation_id/turn_id/request_id/trace_id + phase + 交互快照 + 审计状态转移` 持久化，并保证可恢复、可回放。
  - **266 = UI 单通道前置子计划**：保证这些对话都发生在官方 UI 的同一聊天流、同一气泡内，不再出现外置容器和官方 `Connection error`。
  - **280/284 = 承载面与源码接管计划**：负责 vendored UI、send/store/render 接管与前端降权，不重定义业务语义本身。

## 2. 唯一目标口径（以用户真实案例为准）

### 2.1 验证入口
- 验证入口按用户当前口径冻结为：`http://localhost:8080/app/assistant/librechat`
- `/app/assistant` 仅保留日志与审计记录，**不是** `260` 的交互验收入口。
- 若运行态存在路由别名或 iframe 落点差异，**以用户实际可见的独立页体验为准**，不得以技术内部路径差异规避验收。

### 2.2 真实 Case（必须 100% 达成）
1. [ ] **Case 1：通道连通（前置）**
   - 输入：`你好`
   - 预期：
     - `/app/assistant/librechat` 页面不白屏、官方输入框可用、可正常发消息；
     - 同轮满足 `266` 共通 stopline：无官方原始发送、无官方 `Connection error`、无页面外回复容器；
     - 助手回复进入官方聊天流内。
2. [ ] **Case 2：一句话自动执行（完整信息）**
   - 输入：`在 AI治理办公室 下新建 人力资源部2，生效日期 2026-01-01`
   - 预期：
     - AI 先在对话中返回准备提交的信息摘要；
     - AI 在对话中询问用户是否确认提交；
     - 用户通过对话输入确认；
     - 系统自动执行 `create -> confirm -> commit`；
     - AI 通过对话告诉用户已提交成功。
3. [ ] **Case 3：信息不充分 -> 对话补全**
   - 第一句：`在 AI治理办公室 下新建 人力资源部239A补全`
   - 第二句：`生效日期 2026-03-25`
   - 预期：
     - 第一句先提示缺少字段；
     - AI 在对话中明确指出缺的是哪个字段，并引导用户补全；
     - 用户第二句通过对话补充确切信息；
     - AI 在对话中给出准备提交的信息并要求确认；
     - 用户确认后，系统自动执行并提交成功；
     - AI 通过对话告诉用户已提交成功。
4. [ ] **Case 4：多候选确认（对话内完成）**
   - 第一句：`在 共享服务中心 下新建 239A候选验证部，生效日期 2026-03-26`
   - 第二句：`选第2个`（或候选编码）
   - 预期：
     - AI 发现系统中有多个匹配后，在对话中以用户友好形式列出候选并编号；
     - 用户通过对话反馈“选第N个/编码”完成候选选择；
     - AI 再次确认用户选择的是哪个具体候选项；
     - 用户通过对话确认“是的”；
     - 系统自动执行并提交成功；
     - AI 通过对话告诉用户已提交成功。

### 2.3 Case 1~4 共通 UI / 体验前提（继承 266）
1. [ ] Case 1~4 的真实页面验收，必须同时满足 `DEV-PLAN-266` 第 6.6 节“用户可见交互与体验变化”与第 7 节“验收标准（硬门槛）”。
2. [ ] 同一轮用户输入只允许一条有效发送通道；不得再出现“官方原始发送 + 本仓桥接请求”双链路并存。
3. [ ] 同一轮 assistant 最终回复只能出现一次，且必须位于官方聊天流内部。
4. [ ] 页面外 bridge 容器、overlay、notice 不得承担用户可见业务回执职责。
5. [ ] 任一 Case 若出现官方 `Connection error`、双写、串泡或外挂回执，则该 Case 直接判失败，即使业务语义本身正确。

## 3. 主从关系冻结（260 主计划 / 223 事实源 / 266 前置子计划）
1. [ ] **260 主计划职责**：
   - 定义 Case 1~4 的业务 FSM；
   - 定义哪些轮次等待补全、等待候选、等待二次确认、等待提交确认；
   - 定义用户输入如何驱动 `create / confirm / commit`；
   - 定义最终成功 / 失败回执的业务语义；
   - 冻结前后端共用的最小 DTO 字段集合与验收口径。
2. [ ] **223 子计划职责**：
   - 将 `conversation_id/turn_id/request_id/trace_id + phase + 交互快照 + 审计状态转移` 作为唯一业务事实源落盘；
   - 保证服务重启后仍可恢复 Case 2~4 的待补全 / 待候选 / 待确认上下文；
   - 为 `280/284` 提供稳定 DTO rebuild 能力。
3. [ ] **266 前置子计划职责**：
   - 收掉官方原始发送链路；
   - 保证所有业务回执都写入官方 UI 同一聊天流、同一气泡体系；
   - 移除页面外外挂容器；
   - 消除官方 `Connection error` 干扰。
4. [ ] **280/284 承载面职责**：
   - 保证 vendored UI 是唯一正式聊天承载面；
   - 在 send/store/render 控制点消费后端 DTO；
   - 不得重新定义业务 FSM、候选裁决或提交约束。
5. [ ] **边界冻结**：
   - 未完成 `266`，不得宣称 `260` 用户体验达成；
   - 未完成 `223` 的 `phase/DTO` 持久化与恢复，`260` 不得宣称具备稳定可恢复实现；
   - 即使 `266/280/284` 完成，若 `260` 的业务 FSM/确认语义未完成，也不得宣称 Case 2~4 达成；
   - `260` 任一 Case 的通过，必须同时满足 `223` 的事实源不变量与 `266` 的单通道、气泡内回写、无外挂容器、无官方原始错误体验等前置门槛。

## 4. 目标与非目标

### 4.1 核心目标
1. [ ] 所有业务闭环步骤都必须通过**对话**完成，不得依赖页面外提示、浮层、表单按钮或隐藏状态提示来完成业务确认。
2. [ ] 正常、缺字段、多候选、提交成功、提交失败五类结果，都必须通过 AI 对话返回给用户。
3. [ ] 写入动作仍保持 One Door：只允许走既有 `/internal/assistant/*` 与 DB Kernel 提交链路。
4. [ ] 用户可见业务文案必须来自真实大模型，不允许本地模板 / fallback 冒充。
5. [ ] 业务 FSM 与状态推进以后端为 SSOT；前端只消费后端返回的 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` DTO，不得在 vendored UI、`LibreChatPage`、页面 helper 或 adapter 内重算业务语义。
6. [ ] `223` 必须能把上述 DTO 从持久化事实源稳定重建出来，保证重启恢复后 Case 2~4 仍按同一语义继续推进。
7. [ ] Case 1~4 除业务语义达成外，还必须同时满足单通道、官方气泡内回写、同轮唯一回复、无外挂回复容器、无官方原始 `Connection error` 干扰。

### 4.2 非目标
1. [ ] `260` 本身不定义 schema/迁移/sqlc 实施细节；凡为 `phase/DTO` 持久化新增的表列与回填策略，统一由 `DEV-PLAN-223` 承接。
2. [ ] 不引入 legacy 双链路、兼容快路径或第二业务写入口。
3. [ ] 不在本计划内定义 LibreChat UI 承载实现细节；UI 主架构与源码纳管以 `280` 为准，send/store/render 接管以 `284` 为准，260 只冻结业务语义、FSM、DTO 契约与验收口径。
4. [ ] 不以“局部单测通过”“页面外出现提示”或“接口返回成功”作为 Case 2~4 达成依据。

### 4.3 P0 契约冻结（用于推进 284）
1. [X] 冻结统一 DTO 字段命名：`phase / missing_fields / candidates / pending_draft_summary / selected_candidate_id / commit_reply / error_code`。
2. [X] 冻结阶段转移与 guard 条件（见 5.4），作为 `284` send/store/render 接管时的唯一业务推进依据。
3. [X] 冻结 DTO 字段适用矩阵（见 5.5），禁止前端为“补齐显示”自行推导业务字段。
4. [X] 冻结接口契约矩阵（见 6.1），`284` patch 与测试只允许消费该契约。
5. [X] 冻结前端降权 stopline（见 5.6）：vendored UI 只做渲染与事件分发，不重算业务语义。
6. [X] 冻结命名退化禁止项：`draft`、`commit-reply`、`commitReply` 仅可作为历史术语出现在归档说明，不得作为现行契约字段名。

## 5. 业务状态机（FSM）冻结

### 5.1 运行态阶段
> 下述结构是**后端事实源 + DTO 口径**，不是前端本地真相模型。

```ts
interface DialogFlowSnapshotDTO {
  phase:
    | 'idle'
    | 'await_missing_fields'
    | 'await_candidate_pick'
    | 'await_candidate_confirm'
    | 'await_commit_confirm'
    | 'committing'
    | 'committed'
    | 'failed'
  conversation_id: string
  turn_id: string
  pending_draft_summary?: string
  missing_fields?: string[]
  candidates?: AssistantCandidateOption[]
  selected_candidate_id?: string
  commit_reply?: {
    outcome: 'success' | 'failure'
    message: string
  }
  error_code?: string
}
```

### 5.2 阶段语义
1. [ ] `idle`
   - 仅表示当前没有待补全 / 待选择 / 待确认上下文。
2. [ ] `await_missing_fields`
   - AI 必须明确告诉用户缺哪些字段；
   - 用户补充后，系统重新生成草案；
   - 不允许在该阶段直接 `commit`。
3. [ ] `await_candidate_pick`
   - AI 必须以编号列表形式给出候选；
   - 用户可通过“选第N个/候选编码”反馈选择；
   - 选择后转 `await_candidate_confirm`。
4. [ ] `await_candidate_confirm`
   - AI 必须复述用户选中的候选具体内容；
   - 用户确认后才能执行 `confirm(candidate_id)`。
5. [ ] `await_commit_confirm`
   - AI 必须展示准备提交的摘要；
   - 只有用户明确确认后才能执行 `commit`。
6. [ ] `committing`
   - 后台正在提交；
   - 完成后转 `committed` 或 `failed`。
7. [ ] `committed`
   - AI 必须通过对话明确告诉用户提交成功。
8. [ ] `failed`
   - AI 必须通过对话解释失败原因与下一步建议；
   - 不允许仅在页面外给 notice/alert。

### 5.3 不变量
1. [ ] `phase != await_candidate_confirm && phase != await_commit_confirm` 时，确认词不得触发写入。
2. [ ] `selected_candidate_id` 为空时，不得进入 `await_candidate_confirm`。
3. [ ] `pending_draft_summary` 为空时，不得进入 `await_commit_confirm`。
4. [ ] 任意 `confirm/commit` 失败后必须转入 `failed` 并在对话中回执。
5. [ ] 任意阶段若用户可见业务回执不在聊天流内，则整轮验收判失败。
6. [ ] 任意轮用户发送不得触发双链路；若官方原始发送实际发出，则该轮验收直接失败。
7. [ ] 任意轮 assistant 最终回复只能出现一次，且必须能与同轮 `conversation_id/turn_id/request_id` 一一对应。
8. [ ] 页面外 bridge 容器、overlay、notice 不得承担用户可见业务回执职责。
9. [ ] DTO 中的 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` 必须能由 `223` 的持久化事实源稳定重建，不能只存在于前端运行时内存。

### 5.4 阶段转移表（P0 冻结）
| 当前 phase | 用户输入 / 动作 | Guard（必须满足） | 下一 phase | 接口 |
| --- | --- | --- | --- | --- |
| `idle` | `POST .../turns`（信息完整） | 无缺字段、无候选冲突、已形成草案 | `await_commit_confirm` | `POST /internal/assistant/conversations/{conversation_id}/turns` |
| `idle` | `POST .../turns`（信息缺失） | 存在缺字段 | `await_missing_fields` | 同上 |
| `idle` | `POST .../turns`（多候选） | 存在候选冲突 | `await_candidate_pick` | 同上 |
| `await_missing_fields` | `POST .../turns`（用户补全） | 缺字段已补齐 | `await_commit_confirm` 或 `await_candidate_pick` | 同上 |
| `await_candidate_pick` | 用户“选第N个/候选编码” | 可唯一定位候选 | `await_candidate_confirm` | `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:reply`（或等价回复动作） |
| `await_candidate_confirm` | `:confirm(candidate_id)` | `selected_candidate_id` 非空 | `await_commit_confirm` | `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:confirm` |
| `await_commit_confirm` | `:commit` | `pending_draft_summary` 非空且用户明确确认 | `committing` | `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:commit` |
| `committing` | 提交完成 | 后端提交结果成功 | `committed` | 内部状态推进 |
| `committing` | 提交完成 | 后端提交结果失败 | `failed` | 内部状态推进 |

### 5.5 DTO 字段适用矩阵（P0 冻结）
| 字段 | 适用 phase | 约束 |
| --- | --- | --- |
| `phase` | 全部 | 必填，后端事实源唯一真相 |
| `missing_fields` | `await_missing_fields` | 非空列表；其他 phase 不得伪造 |
| `candidates` | `await_candidate_pick` | 非空列表；候选顺序稳定可追溯 |
| `selected_candidate_id` | `await_candidate_confirm`、`await_commit_confirm`、`committing`、`committed`、`failed`（若由候选链路进入） | 进入 `await_candidate_confirm` 前必须已有值 |
| `pending_draft_summary` | `await_commit_confirm`、`committing`、`committed`、`failed`（若草案已生成） | 进入 `await_commit_confirm` 前必须已有值 |
| `commit_reply` | `committed`、`failed` | 仅提交结束后出现 |
| `error_code` | `failed`（或接口错误响应） | 与错误码契约一致，不得由前端拼装 |

### 5.6 前端降权 Stopline（P0 冻结）
1. [X] 禁止在 vendored UI、`LibreChatPage`、页面 helper、adapter 中根据自然语言文本重算 `phase`。
2. [X] 禁止前端根据局部上下文自行推断 `selected_candidate_id`、`pending_draft_summary`、`commit_reply`。
3. [X] 禁止在前端把确认词直接映射为 `:commit`；是否允许提交只能由后端 phase + guard 决策。
4. [X] 前端允许职责仅限：DTO 渲染、事件分发、协议适配、可观测埋点。
5. [X] 若任一 stopline 被破坏，则 `260/284` 同时判未达成。

## 6. 内部调用序列（冻结）
1. [ ] **Case 2**：
   - `POST /conversations/:id/turns`
   - 返回 `await_commit_confirm` 草案 DTO
   - 等待用户确认
   - `:confirm`
   - `:commit`
   - 返回 `commit_reply`
2. [ ] **Case 3**：
   - `turns(首轮缺字段)`
   - 返回 `await_missing_fields` DTO
   - 等待用户补全
   - `turns(补全后草案)`
   - 返回 `await_commit_confirm` DTO
   - 等待用户确认
   - `:confirm`
   - `:commit`
   - 返回 `commit_reply`
3. [ ] **Case 4**：
   - `turns(候选列表)`
   - 返回 `await_candidate_pick` DTO
   - 等待用户选择候选
   - 返回 `await_candidate_confirm` DTO
   - AI 二次确认用户选中项
   - `:confirm(candidate_id)`
   - 返回 `await_commit_confirm` DTO
   - 等待提交确认
   - `:commit`
   - 返回 `commit_reply`

### 6.1 接口契约矩阵（P0 冻结）
| 接口 | 请求关键字段 | 响应关键字段（最小集合） | 说明 |
| --- | --- | --- | --- |
| `POST /internal/assistant/conversations/{conversation_id}/turns` | `request_id`、`trace_id`、`user_input` | `phase` + 适用子集（`missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code`） | 创建用户回合并推进到下一业务阶段 |
| `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:reply`（或等价动作） | `request_id`、`trace_id`、`user_input` | 同上 | 处理“补全字段”“选择候选”“确认文本”等回合内回复 |
| `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:confirm` | `request_id`、`trace_id`、`candidate_id`（候选链路时） | `phase` + 适用子集 | 候选确认或提交前确认 |
| `POST /internal/assistant/conversations/{conversation_id}/turns/{turn_id}:commit` | `request_id`、`trace_id` | `phase`、`commit_reply`、`error_code`（失败时） | 执行最终提交 |
| `GET /internal/assistant/conversations/{conversation_id}` | 无 | 当前会话 `phase` + 最近回合可恢复 DTO 快照 | 重启恢复与 UI 回读唯一入口 |

错误码口径（P0 最小集合）：
1. `conversation_state_invalid`
2. `idempotency_key_conflict`
3. `request_in_progress`
4. `tenant_mismatch`

## 7. 实施分解

### 7.1 M1：业务语义重新冻结（主计划）
1. [ ] 将 Case 1~4 作为唯一业务验收契约写入测试与执行日志模板。
2. [ ] 统一确认词、候选选择词、补全语义解析规则。
3. [ ] 明确“哪些回复必须等待用户下一轮输入，哪些回复可以自动推进”。
4. [ ] 与 `223` 对齐 DTO 白名单字段，冻结 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code` 的业务含义。

### 7.2 M2：事实源与编排收口
1. [ ] 将 FSM 推进、候选裁决、提交约束冻结到后端事实源与服务层，不再定义或保留“共享 FSM helper 作为正式业务真相”的口径。
2. [ ] 删除页面级分叉编排，避免 `/app/assistant` 再次演化成第二交互入口。
3. [ ] 保证 Case 2~4 在运行态中严格按后端持久化 `phase` 推进，而不是按前端本地状态推进。
4. [ ] `GET conversation` 或等价恢复接口必须能稳定重建 DTO，供 `280/284` 直接消费。

### 7.3 M3：对话文案与模型链路收口
1. [ ] 所有业务回执统一走真实大模型回复链路。
2. [ ] 缺字段提示、多候选提示、确认提示、成功/失败回执，都必须由对话消息返回。
3. [ ] 禁止页面外 notice/alert 承担业务确认职责。
4. [ ] 禁止以前端 fallback 文案、局部模板或 DOM 注入替代真实业务回执。

### 7.4 M4：依赖 266 / 280 / 284 完成 UI 与承载面前置收口
1. [ ] 以 `266` 第 6.6 节与第 7 节为 readiness：只有当单通道、气泡内回写、无外挂容器、无官方原始错误体验全部达成后，260 才能进入最终 Case 通过判定。
2. [ ] 以 `280/284` 为承载面 readiness：只有当 vendored UI 成为正式承载面，且 send/store/render 已消费后端 DTO，260 才能宣称具备稳定实现底座。
3. [ ] 将官方原始发送链路收掉，并把“官方原始发送未实际发出”作为 `260` Case 1~4 的共通前置断言。
4. [ ] 保证所有业务回执落到官方 UI 同一聊天流气泡中，并把“同轮唯一 assistant 回复”作为 `260` Case 1~4 的共通前置断言。
5. [ ] 彻底去掉外挂容器与官方错误气泡干扰；若 `266/280/284` 任一回归退化，则 `260` 不得封板。

### 7.5 M5：真实验收与证据固化
1. [ ] 用真实页面按 Case 1~4 顺序逐条验收。
2. [ ] 每个 Case 必须保存页面全图、对话局部图、同轮 trace / 网络证据，并额外证明：无官方 `Connection error`、无页面外挂回复容器、同轮仅一份 assistant 回复。
3. [ ] 执行记录写回 `docs/archive/dev-records/dev-plan-260-execution-log.md` 新章节，明确区分“旧 260 验收记录”与“本次重开后的真实需求验收记录”。
4. [ ] 证据中必须额外证明：同轮 `conversation_id/turn_id/request_id` 能关联回持久化事实源与唯一 assistant 气泡。

## 8. 验收标准（硬门槛）
1. [ ] Case 1~4 必须全部在**AI 对话中**闭环，不得借助页面外提示补齐业务流程。
2. [ ] Case 2 必须是“先草案、后确认、再提交”，不得首轮自动 `commit`。
3. [ ] Case 3 必须是“先缺字段提示、再补全、再确认、再提交”，不得跳过确认。
4. [ ] Case 4 必须是“先候选列表、再选择、再二次确认、再提交”，不得选中后直接提交。
5. [ ] 成功与失败回执都必须由真实大模型生成，并显示在聊天流气泡内。
6. [ ] Case 1~4 的每一轮都必须同时满足 `266` 第 6.6 节“用户可见交互与体验变化”与第 7 节“验收标准（硬门槛）”。
7. [ ] 任一 Case 如出现双链路、官方 `Connection error`、页面外挂容器承担回复或同轮多份 assistant 回复，则该 Case 直接判失败。
8. [ ] `223` 若无法恢复对应轮次的 `phase/DTO` 上下文，或 `280/284` 仍需前端本地补算业务语义，则 `260` 不得宣布达成。
9. [ ] `266` 未完成或回归退化前，不得宣布 `260` 用户体验达成。

## 9. 测试与门禁
- 触发器与门禁以 `AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile` 为 SSOT。
- `260` 当前最低验证集：
  1. [ ] `go test ./internal/server -run 'TestAssistantUIProxy|TestAssistantReply|TestAssistantRenderReply' -count=1`
  2. [ ] `pnpm --dir apps/web test -- src/pages/assistant/AssistantPage.test.tsx src/pages/assistant/LibreChatPage.test.tsx`
  3. [ ] 增补后端 DTO / 恢复契约测试，证明 `GET conversation` 或等价恢复接口可稳定返回 `phase/missing_fields/candidates/pending_draft_summary/selected_candidate_id/commit_reply/error_code`。
  4. [ ] 旧桥专属 E2E 已由 `DEV-PLAN-282` 删除；正式入口 E2E 由 `DEV-PLAN-283/285` 重新补齐。
  5. [ ] 补充“AI 对话独立页真实 Case 1~4”专属 E2E；每个 Case 必须同时断言 `266` 的共通 stopline：无官方原始发送、无官方错误气泡、无外挂回复容器、同轮唯一 assistant 气泡。
  6. [ ] `make check doc`

### 9.1 `260 -> 284` 交接包（P0）
1. [X] 阶段转移与 guard：见 5.4。
2. [X] DTO 字段适用矩阵：见 5.5。
3. [X] 接口契约矩阵与错误码最小集合：见 6.1。
4. [X] 前端降权 stopline：见 5.6。

## 10. 交付物
1. [ ] 主计划文档：`docs/dev-plans/260-librechat-conversation-first-auto-execution-plan.md`
2. [ ] 业务事实源子计划：`docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
3. [ ] 前置子计划：`docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
4. [ ] 更新后的执行日志：`docs/archive/dev-records/dev-plan-260-execution-log.md`
5. [ ] 真实用例证据目录：`docs/archive/dev-records/assets/dev-plan-260/`
6. [ ] 相关后端 / Web / E2E 用例补强。

## 11. 关联文档
- `docs/archive/dev-plans/239a-librechat-dialog-auto-execution-and-standalone-page-plan.md`
- `docs/archive/dev-plans/239b-239a-direct-validation-report-and-implementation-gaps.md`
- `docs/dev-plans/223-assistant-conversation-persistence-and-audit-closure-plan.md`
- `docs/dev-plans/263-librechat-gpt52-assistant-dialogue-response-implementation-plan.md`
- `docs/dev-plans/264-librechat-gpt52-reply-single-pipeline-and-real-evidence-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/284-librechat-source-level-send-and-render-takeover-plan.md`
- `docs/archive/dev-records/dev-plan-260-execution-log.md`
- `AGENTS.md`
