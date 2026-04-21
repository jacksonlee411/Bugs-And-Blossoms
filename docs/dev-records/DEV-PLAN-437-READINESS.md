# DEV-PLAN-437 Readiness

## 目标

- 作为 `DEV-PLAN-437` 的执行证据入口，记录 CubeBox 快速开工路线图的阶段状态、命中的 owner 计划、执行命令与可审计证据。
- 将“快速开工”收敛为一组可验证阶段，而不是口头上的优先级建议。
- 首轮先固化 `Phase A / PR-437A` 的 readiness 清单，为后续 `PR-437B` 到 `PR-437E` 预留统一记录结构。

## 当前状态

- 日期：2026-04-21
- owner：`DEV-PLAN-437`
- 当前结论：`PR-437A` 已完成其文档层收敛范围：`430` 已回链、`431/433/434` 已补齐首轮最小上游冻结、共享 companion doc `DEV-PLAN-437A` 已创建，且 `chat-surface-clean` 已显式列出批准的新主线路径 `/app/cubebox`、`/internal/cubebox`、`modules/cubebox`。但由于当前仓库尚未命中这些新路径，`Phase A` 仍保持“进行中”，等待 `PR-437B` 首次实际命中活体路径后再做最终闭环确认。

## 阶段总览

| 阶段 | 对应 PR | 主要 owner 计划 | 目标 | 当前状态 |
| --- | --- | --- | --- | --- |
| `Phase A` | `PR-437A` | `436`、`430`、`431`、`433`、`434` | 开工门禁、最小上游冻结、共享 canonical contract、deterministic provider 口径 | `进行中` |
| `Phase B` | `PR-437B` | `431`、`433` | 第一条可运行对话竖切 | `未开始` |
| `Phase C` | `PR-437C` | `432`、`431` | 会话持久化与恢复 | `未开始` |
| `Phase D` | `PR-437D` | `434`、`431` | 压缩最小闭环 | `未开始` |
| `Phase E` | `PR-437E` | `435`、`433` | 管理面与权限闭环 | `未开始` |

## Phase A / PR-437A

### 目标

- 把 CubeBox 从“文档已拆分但仍不好开工”推进到“具备首轮实现前置条件”。
- 只冻结最小必要项，不要求 `431-435` 全量映射一次性补齐。
- 为 `PR-437B` 的第一条可运行竖切提供单一共享输入。

### 勾选项

1. [x] `chat-surface-clean` 已补充显式批准的新主线路径清单：`/app/cubebox`、`/internal/cubebox`、`modules/cubebox`；当前文档层 allowlist 与路线图口径一致。
2. [x] `430` 已回填“按阶段快速开工”的引用，且不与 `437` 路线图冲突。
3. [x] `431`、`433`、`434` 已补齐首轮会使用到的上游 `commit SHA` 与最小文件级映射对象。
4. [x] 已形成共享 canonical contract，明确：
   - conversation / turn / item 命名
   - SSE event envelope
   - `turn.agent_message.delta` / `turn.completed` / `turn.error` / `turn.interrupted`
   - compact / token usage 事件名
   - reducer 输入与 reconstruction 输出 shape
5. [x] deterministic provider / mock SSE / fake provider 口径已冻结，不把真实外部模型调用作为 merge 前置条件。

### 当前证据

- 路线图文档：
  - `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`
- 共享 companion doc：
  - `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
- 文档地图更新：
  - `AGENTS.md`
- `430` 回链与首轮冻结：
  - `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `431` 最小映射冻结：
  - `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- `433` 最小映射冻结：
  - `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `434` 最小映射冻结：
  - `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `432` 消费 shared contract 的恢复输出对齐说明：
  - `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- 门禁 allowlist 更新：
  - `scripts/ci/check-chat-surface-clean.sh`

### 待补证据位

| 证据主题 | 目标文件 / 产物 | 期望证据形态 | 当前状态 |
| --- | --- | --- | --- |
| 反回流门禁更新 | `scripts/ci/check-chat-surface-clean.sh`、相关文档 | diff + 命令结果 | `已补并验证通过；待 PR-437B 首次命中新路径时再次确认` |
| `430` 回链 | `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md` | 文档 diff | `已补` |
| `431` 最小映射冻结 | `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md` | `commit SHA` + 文件级映射 | `已补` |
| `433` 最小映射冻结 | `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md` | `commit SHA` + 文件级映射 | `已补` |
| `434` 最小映射冻结 | `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md` | `commit SHA` + 文件级映射 | `已补` |
| 共享 canonical contract | `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md` | 文档 diff + owner 对齐说明 | `已补` |
| deterministic provider 口径 | `433` / companion doc / fixture 方案文档 | 文档 diff + fixture 路径 | `口径已冻结；代码/fixture 仍待 PR-437B 实现` |

### 命令记录

本阶段已进入文档与门禁修改；执行命令结果如下：

| 日期 | 命令 | 结果 | 备注 |
| --- | --- | --- | --- |
| `2026-04-21` | `bash scripts/ci/check-chat-surface-clean.sh` | `通过` | 输出批准的新主线路径清单，并返回 `[chat-surface-clean] OK` |

### Phase A 完成判定

同时满足以下条件时，`Phase A / PR-437A` 才可标记完成：

1. 上述 5 个勾选项全部完成。
2. `chat-surface-clean` 可通过且显式批准的新主线路径与 `437` 路线图一致。
3. reviewer 能指出首轮竖切具体依赖哪份 shared contract，而不是继续依赖口头约定。
4. `PR-437B` 已具备可直接开工条件，不再被“先补完整映射表”阻塞。

## 后续阶段预留

### Phase B / PR-437B 预留证据

- 第一条可运行竖切：
  - 抽屉入口
  - 统一 store / reducer
  - deterministic SSE 回复
  - stop / interrupt
- 待补命令：
  - `pnpm --dir apps/web check`
  - 命中 Go 时 `go fmt ./... && go vet ./... && make check lint && make test`
  - 命中门禁时 `make check chat-surface-clean`

### Phase C / PR-437C 预留证据

- 会话持久化与恢复：
  - append-only message log
  - conversation list/read/resume/archive/rename
  - reconstruction fixture

### Phase D / PR-437D 预留证据

- 压缩最小闭环：
  - manual compact
  - pre-turn auto compact
  - canonical context reinjection
  - prompt shape snapshot

### Phase E / PR-437E 预留证据

- 管理面与权限闭环：
  - provider / active model / credential / health UI
  - Authz 矩阵
  - validation / readiness / masking / rotation 证据

## 当前风险

- `437A` 已存在，但若后续实现没有真正消费该 companion doc，`431/432/434` 仍可能在代码层重新分叉。
- 若 `435` 抢在 `433` 运行时对象命名前先做管理面，会重新引入 Slice 2 / Slice 5 双主参考或双命名问题。
- 若继续把“完整映射表冻结”当作普遍前置，路线图虽然存在，开工节奏仍会停在文档层。

## 当前裁决

- `DEV-PLAN-437` 已具备 readiness 文档入口。
- `PR-437A` 已完成文档层冻结与门禁语义收敛，但 `Phase A` 仍待 `PR-437B` 首次命中新主线路径时完成最后的“活体路径命中确认”。
- 下一步可进入 `PR-437B`：抽屉壳层、统一 reducer/store、deterministic provider 与 SSE 最小竖切；届时应顺带再次执行 `make check chat-surface-clean`，验证新路径在真实代码落地后仍与 allowlist 口径一致。

## 关联文档

- `docs/dev-plans/437-cubebox-implementation-roadmap-for-fast-start.md`
- `docs/dev-plans/437a-cubebox-phase-a-canonical-conversation-contract.md`
- `docs/dev-plans/430-cubebox-ide-conversation-assistant-rebuild-architecture-plan.md`
- `docs/dev-plans/431-codex-ui-protocol-and-shell-reuse-plan.md`
- `docs/dev-plans/432-codex-session-persistence-reuse-plan.md`
- `docs/dev-plans/433-bifrost-centric-ai-gateway-reuse-and-reconstruction-plan.md`
- `docs/dev-plans/434-codex-context-management-and-compaction-reuse-plan.md`
- `docs/dev-plans/435-bifrost-centric-model-config-ui-and-admin-governance-plan.md`
- `docs/dev-plans/436-cubebox-historical-surface-hard-delete-plan.md`
