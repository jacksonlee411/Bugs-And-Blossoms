# DEV-PLAN-291：237 升级兼容回归前置专项（供 285 封板复核）

> 归档说明（2026-04-12）：本文件已自 `docs/dev-plans/` 迁入 `docs/archive/dev-plans/`，仅保留为历史参考，不再作为现行 SSOT。

**状态**: 已完成（2026-03-09 11:00 CST；已在 `240D-03/04` cutover 后按 `291-v2` 重跑固定命令链与新鲜度复核；`R1~R10` 继续全部通过，`291` 仍可作为 `285` 的升级兼容前置件）

## 1. 背景
1. [X] `DEV-PLAN-271` 的 `S5` 明确要求补齐 `237` 的 source/runtime compatibility 回归后，才能进入 `285` 封板。
2. [X] `291` 在 `271 §5.8` 中属于 `P1` 结构性并行项：可并行准备，但不得以“可并行”长期顺延。
3. [X] `285` 的直接前置要求中已明确包含 `291` 通过清单；若 `291` 不可复核，`285` 必须维持未启动状态。

## 2. 目标与非目标
### 2.1 目标
1. [X] 形成可执行的升级兼容回归矩阵，覆盖 `source / patch stack / runtime / 正式入口边界 / stopline` 五类检查。
2. [X] 固化升级回放流程、兼容别名边界与失败 stopline，确保“可预期、可回滚、可审计”。
3. [X] 产出一组可直接供 `285` 引用的前置证据与复核清单，不把规则补写推迟到封板阶段。
4. [X] 将 `280` 的核心硬门槛纳入 `291` 的可复核输入：`291` 不重复执行 `288/290` 的业务与 UI Case，但必须显式引用并校验其最新证据是否仍然有效。
5. [X] 形成 `291-v2` 证据索引，保证每个检查项都能定位到“命令输出 + 状态快照/日志 + 结论 + 引用证据”。

### 2.2 非目标
1. [X] 不在本计划内直接宣称封板完成（封板由 `DEV-PLAN-285` 承担）。
2. [X] 不改写 `237` 主计划业务契约，仅做前置专项执行与证据收敛。
3. [X] 不承担 `260` 业务 Case 1~4 的真实验收执行（由 `290` 承担）；`291` 只校验 `290` 证据的新鲜度与可引用性。
4. [X] 不重复承担 `288` 的 message-tree / single-bubble E2E 执行；`291` 只将其作为 `280` 硬门槛引用输入。
5. [X] 不把 `292` 允许的 formal-entry 侧 compat alias 错判为第二正式 API 面；其边界必须按 `292` 固定口径复核。
6. [X] 不以“局部日志存在”替代“回归矩阵逐项通过”。

## 3. 顺序、依赖与并行规则
1. [X] 可与 `260-M5` 验收（`290`）并行推进，但 `291` 通过判定必须早于 `285` 启动。
2. [X] 若回归发现旧入口、旧桥职责或双链路回流，必须回退到对应实现计划修复，不得带缺口进入封板。
3. [X] 若最新 `288/290` 证据无法证明 `280` 的核心硬门槛仍成立，`291` 不得判定通过。
4. [X] `291` 执行完成后，才允许在 `285` 中将 `237` 前置标记为“已齐备”。

### 3.1 输入冻结（291-v2）
1. [X] **source 基线**：`third_party/librechat-web/UPSTREAM.yaml`（`repo/ref/commit/imported_at/rollback_ref`）为唯一来源输入。
2. [X] **patch stack 基线**：`third_party/librechat-web/patches/series` 为唯一 patch 顺序入口；执行期禁止临时散改顺序。
3. [X] **runtime 基线**：`deploy/librechat/versions.lock.yaml` 与 `deploy/librechat/runtime-status.json` 为版本与健康事实输入。
4. [X] **入口边界基线**：`/app/assistant/librechat`（正式用户入口）+ `/assets/librechat-web/**`（正式静态前缀与相对 API 基准）+ `292` 允许的 `/app/assistant/librechat/api/**` compat alias。
5. [X] **引用证据基线**：`docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json` 与 `docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json` 为 `280` 核心硬门槛的最新引用证据输入。

### 3.2 进入与退出条件（供 285 引用）
1. [X] 进入条件：`292` 已完成；`283` 正式入口切换已完成；`288` 已形成默认基线证据；`285` 尚未启动封板执行。
2. [X] 退出条件：输出 `291` 专属“回归矩阵 + 执行报告 + 风险清单 + stopline 检查结果 + 285 交接清单”，并显式引用最新 `288/290` 证据与其新鲜度结论。

### 3.3 并行变更复跑规则（对齐 271-S5）
1. [X] `291` 执行期间若合入影响路由分类、认证链路、正式入口/静态前缀、compat alias、运行时 gate、no-legacy 规则、vendored UI 构建产物的变更，已有 `291` 证据立即失效。
2. [X] 若合入影响消息渲染主路径、DTO 语义、single-bubble / official-message-tree 结论的变更，则不仅 `291` 证据失效，`288/290` 的被引用结论也必须按 `271-S5` 一并重跑或重新确认新鲜度。
3. [X] 证据失效后必须重跑受影响项并刷新索引时间戳，未重跑不得判定 `291` 通过。
4. [X] 若发生“`291` 通过后再次合入影响性变更”，`285` 不得引用旧版 `291` 结论。

## 4. 实施步骤（291-v2）
1. [X] **批次 A：矩阵冻结**  
   产出 `291-upgrade-compat-matrix.md`，固定检查项 ID、命令入口、通过标准、证据文件名与 `288/290` 引用位点。
2. [X] **批次 B：回放执行**  
   按固定顺序执行 source / patch / runtime / 正式入口边界 / compat alias / routing / no-legacy 检查，并采集原始日志与状态快照。
3. [X] **批次 C：stopline 复核**  
   对旧入口、旧桥、双口径回流与 `280` 硬门槛证据新鲜度执行显式检查，并记录“命中/未命中 + 证据定位”。
4. [X] **批次 D：结论收敛与交接**  
   生成执行报告、风险清单、`285` 可直接引用的前置通过清单与 `288/290` 证据引用结论。

### 4.1 回归矩阵（291-v2）
| ID | 维度 | 检查项 | 命令入口（固定） | 通过标准 | 证据落盘 |
| --- | --- | --- | --- | --- | --- |
| R1 | Source 元数据 | `UPSTREAM.yaml` 的 `repo/ref/commit/imported_at/rollback_ref` 完整且一致 | `make librechat-web-verify` | 返回码为 0，且无 source 缺失/冲突提示 | `docs/archive/dev-records/assets/dev-plan-291/291-source-verify.log` |
| R2 | Patch Stack 回放 | patch 顺序可重放且构建成功，且产物可追踪 | `make librechat-web-build` | 返回码为 0，产物稳定输出至 `internal/server/assets/librechat-web`，并补录产物清单/关键文件摘要 | `docs/archive/dev-records/assets/dev-plan-291/291-web-build.log` |
| R3 | Runtime 启动健康 | 运行态服务可启动且健康 | `make assistant-runtime-up && make assistant-runtime-status` | status 为 healthy，关键组件无 crash/restart loop | `docs/archive/dev-records/assets/dev-plan-291/291-runtime-status.json` |
| R4 | Runtime 锁文件一致性 | `versions.lock.yaml` 与 compose 解析结果一致 | `docker compose -p ${LIBRECHAT_COMPOSE_PROJECT:-bugs-and-blossoms-librechat} --env-file deploy/librechat/.env -f deploy/librechat/docker-compose.upstream.yaml -f deploy/librechat/docker-compose.overlay.yaml config --format json` | `versions.lock.yaml` 中的服务/tag/digest 完整，且 compose 配置中的镜像引用与锁文件口径一致；不再以 `runtime-status.json` 伪装版本校验 | `docs/archive/dev-records/assets/dev-plan-291/291-runtime-version-check.md` |
| R5 | 正式入口与静态前缀 | `/app/assistant/librechat` 与 `/assets/librechat-web/**` 的正式入口/静态前缀边界稳定 | `go test ./internal/server -run 'TestLibreChatWebUI'` | 测试通过，正式入口、受保护静态资源、SPA fallback 与登录保护链一致 | `docs/archive/dev-records/assets/dev-plan-291/291-formal-entry-go-test.log` |
| R6 | Compat alias 一致性 | `292` 允许的 `/app/assistant/librechat/api/**` 仍与 `/assets/librechat-web/api/**` 同 handler/DTO/会话链 | `go test ./internal/server -run 'TestLibreChatVendoredCompatAPI'` | 测试通过；compat alias 仅限兼容，不构成第二事实源、第二写入口或第二正式 API 面 | `docs/archive/dev-records/assets/dev-plan-291/291-compat-alias-go-test.log` |
| R7 | 路由与入口边界 | 正式入口/静态前缀/compat API 分类与保护链一致 | `make check routing` | 门禁通过，无分类漂移 | `docs/archive/dev-records/assets/dev-plan-291/291-routing-check.log` |
| R8 | Legacy 回流阻断 | 无旧桥正式职责、无双链路回流 | `make check no-legacy` | 门禁通过，无回流命中 | `docs/archive/dev-records/assets/dev-plan-291/291-no-legacy.log` |
| R9 | `280` 硬门槛引用新鲜度 | 最新 `288/290` 证据仍可证明官方消息树唯一落点、单轮唯一 assistant 回复、DTO-only | `test -f docs/dev-records/assets/dev-plan-266/tp288-real-entry-evidence-index.json && test -f docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json` | 两份索引均存在，且在 `291-execution-report.md` 中明确记录“引用文件 + 生成时间 + 最近影响性合入 + 是否仍有效”；若结论过期则 `291` 失败 | `docs/archive/dev-records/assets/dev-plan-291/291-ref-288-290-freshness.md` |
| R10 | 运行态清理闭环 | 回归后可正常下线，避免脏运行态影响后续 | `make assistant-runtime-down` | 返回码为 0，清理结果可复核 | `docs/archive/dev-records/assets/dev-plan-291/291-runtime-down.log` |

### 4.2 Stopline 检查（强制）
1. [X] **旧桥职责回流**：不得恢复 `bridge.js`、`data-assistant-dialog-stream`、`assistantDialogFlow`、`assistantAutoRun` 的正式职责语义。
2. [X] **旧入口职责回流**：`/assistant-ui/*` 不得恢复为正式交互入口；`/app/assistant` 不得恢复为正式聊天承载面。
3. [X] **双口径回流**：不得出现“双正式入口、双正式静态前缀、双正式消息落点、双正式验收口径”；`292` 允许的 `/app/assistant/librechat/api/**` compat alias 只有在与 `/assets/librechat-web/api/**` 同一 handler/DTO/会话链时才不构成违规。
4. [X] **`280` 硬门槛引用缺失**：若最新 `288/290` 证据不能证明 `official_message_tree_only / single_assistant_bubble / dto_only` 仍成立，`291` 立即失败。
5. [X] **命令门禁（固定）**：`go test ./internal/server -run 'TestLibreChatWebUI|TestLibreChatVendoredCompatAPI'`、`make check routing`、`make check no-legacy`；任一失败即 `291` 失败，不得进入 `285`。

### 4.3 证据与索引结构（291-v2）
1. [X] 证据根目录固定为：`docs/archive/dev-records/assets/dev-plan-291/`。
2. [X] 索引文件固定为：`docs/archive/dev-records/assets/dev-plan-291/291-evidence-index.json`。
3. [X] 索引条目字段固定为：`id`、`command`、`executed_at`、`exit_code`、`artifacts[]`、`result`、`owner`、`notes`。
4. [X] 任一矩阵项若缺少索引条目或 `artifacts[]` 为空，视为未完成。
5. [X] `R9` 必须额外记录 `referenced_artifacts[]`，至少包含 `tp288-real-entry-evidence-index.json` 与 `tp290-real-case-evidence-index.json` 的路径、生成时间与有效性结论。

### 4.4 291 -> 285 交接包（固定）
1. [X] `291-upgrade-compat-matrix.md`：回归矩阵与逐项结论。
2. [X] `291-execution-report.md`：本次执行环境、命令摘要、失败与修复说明。
3. [X] `291-risk-list.md`：按 `高/中/低` 分级的遗留风险及处置建议。
4. [X] `291-handoff-to-285.md`：供 `285 §2.3/§3` 直接引用的通过清单与未决项，必须包含 `292` compat alias 当前状态、`288/290` 引用证据结论与是否仍需重跑。

## 5. 验收标准
1. [X] `291-v2` 回归矩阵（R1~R10）全部完成，且每项均具备“命令结果 + 证据文件 + 结论”记录；`R9` 额外具备“引用证据 + 新鲜度结论”。
2. [X] `237` 对应 source/runtime/entry-boundary compatibility 条目在 `291` 中具备可复核证据，不再停留在抽象描述。
3. [X] stopline 无命中（无旧入口/旧桥职责回流、无双口径回流、无 `280` 核心硬门槛证据缺失）。
4. [X] `292` 允许的 compat alias 边界已被显式复核：既不被误判为第二正式 API 面，也不被扩权为新的正式语义。
5. [X] 交接包可被 `285` 直接引用，不需要在封板阶段新增命令、补判定标准或重新解释 `288/290` 证据关系。
6. [X] 若 `291` 证据或其引用的 `288/290` 证据时间早于最近一次影响性合入，则本次结论失效，必须重跑或刷新引用后再判定通过。

## 6. 测试与门禁（SSOT 引用）
1. [X] 触发器与门禁命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [X] 文档改动至少通过 `make check doc`。
3. [X] `291` 固定执行顺序：`librechat-web-verify -> librechat-web-build -> assistant-runtime-up -> assistant-runtime-status -> compose config version-lock check -> go test ./internal/server -run 'TestLibreChatWebUI' -> go test ./internal/server -run 'TestLibreChatVendoredCompatAPI' -> check routing -> check no-legacy -> 288/290 引用新鲜度复核 -> assistant-runtime-down`。

## 7. 交付物
1. [X] 本计划文档：`docs/archive/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`。
2. [X] 升级兼容回归矩阵：`docs/archive/dev-records/assets/dev-plan-291/291-upgrade-compat-matrix.md`。
3. [X] 执行报告：`docs/archive/dev-records/assets/dev-plan-291/291-execution-report.md`。
4. [X] 风险清单：`docs/archive/dev-records/assets/dev-plan-291/291-risk-list.md`。
5. [X] 证据索引：`docs/archive/dev-records/assets/dev-plan-291/291-evidence-index.json`。
6. [X] 面向 `285` 的交接清单：`docs/archive/dev-records/assets/dev-plan-291/291-handoff-to-285.md`。
7. [X] `280` 核心硬门槛引用结论：`docs/archive/dev-records/assets/dev-plan-291/291-ref-288-290-freshness.md`。

## 8. 关联文档
- `docs/archive/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/archive/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/archive/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/archive/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/archive/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- `AGENTS.md`
