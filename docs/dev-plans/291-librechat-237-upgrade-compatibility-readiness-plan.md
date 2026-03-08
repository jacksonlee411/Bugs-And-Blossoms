# DEV-PLAN-291：237 升级兼容回归前置专项（供 285 封板复核）

**状态**: 规划中（2026-03-08 CST；可与 `290` 并行推进，本文档已细化为可直接执行与交接的 `291-v1` 口径）

## 1. 背景
1. [ ] `DEV-PLAN-271` 的 `S5` 明确要求补齐 `237` 的 source/runtime compatibility 回归后，才能进入 `285` 封板。
2. [ ] `291` 在 `271 §5.8` 中属于 `P1` 结构性并行项：可并行准备，但不得以“可并行”长期顺延。
3. [ ] `285` 的直接前置要求中已明确包含 `291` 通过清单；若 `291` 不可复核，`285` 必须维持未启动状态。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 形成可执行的升级兼容回归矩阵（source、patch stack、runtime、入口边界、stopline）。
2. [ ] 固化升级回放流程与失败 stopline，确保“可预期、可回滚、可审计”。
3. [ ] 输出一组可直接供 `285` 引用的前置证据与复核清单，不在封板阶段临时补规则。
4. [ ] 形成 `291-v1` 证据索引，保证每个检查项都能定位到“命令输出 + 状态快照 + 结论”。

### 2.2 非目标
1. [ ] 不在本计划内直接宣称封板完成（封板由 `DEV-PLAN-285` 承担）。
2. [ ] 不改写 `237` 主计划业务契约，仅做前置专项执行与证据收敛。
3. [ ] 不承担 `260` 业务 Case 验收（由 `290` 承担）。
4. [ ] 不以“局部日志存在”替代“回归矩阵逐项通过”。

## 3. 顺序、依赖与并行规则
1. [ ] 可与 `260-M5` 验收（`290`）并行推进，但 `291` 通过判定必须早于 `285` 启动。
2. [ ] 若回归发现旧入口、旧桥职责或双链路回流，必须回退到对应实现计划修复，不得带缺口进入封板。
3. [ ] `291` 执行完成后，才允许在 `285` 中将 `237` 前置标记为“已齐备”。

### 3.1 输入冻结（291-v1）
1. [ ] **source 基线**：`third_party/librechat-web/UPSTREAM.yaml`（`repo/ref/commit/imported_at/rollback_ref`）为唯一来源输入。
2. [ ] **patch stack 基线**：`third_party/librechat-web/patches/series` 为唯一 patch 顺序入口；执行期禁止临时散改顺序。
3. [ ] **runtime 基线**：`deploy/librechat/versions.lock.yaml` 与 `deploy/librechat/runtime-status.json` 为版本与健康事实输入。
4. [ ] **入口边界基线**：`/app/assistant/librechat`（正式入口）+ `/assets/librechat-web/**`（正式静态前缀）+ `292` compat facade 路径。

### 3.2 进入与退出条件（供 285 引用）
1. [ ] 进入条件：`292` 已完成；`288` 已进入默认基线复跑阶段；`285` 尚未启动封板执行。
2. [ ] 退出条件：输出 `291` 专属“回归矩阵 + 执行报告 + 风险清单 + stopline 检查结果 + 285 交接清单”。

### 3.3 并行变更复跑规则（对齐 271-S5）
1. [ ] `291` 执行期间若合入影响路由分类、认证链路、运行时 gate、no-legacy 规则的变更，已有 `291` 证据立即失效。
2. [ ] 证据失效后必须重跑受影响项并刷新索引时间戳，未重跑不得判定 `291` 通过。
3. [ ] 若发生“`291` 通过后再次合入影响性变更”，`285` 不得引用旧版 `291` 结论。

## 4. 实施步骤（291-v1）
1. [ ] **批次 A：矩阵冻结**  
   产出 `291-upgrade-compat-matrix.md`，固定检查项 ID、命令入口、通过标准、证据文件名。
2. [ ] **批次 B：回放执行**  
   按固定顺序执行 source/patch/runtime/routing/no-legacy 检查，并采集原始日志与状态快照。
3. [ ] **批次 C：stopline 复核**  
   对旧入口、旧桥、双口径回流执行显式检查并记录“命中/未命中 + 证据定位”。
4. [ ] **批次 D：结论收敛与交接**  
   生成执行报告、风险清单、`285` 可直接引用的前置通过清单。

### 4.1 回归矩阵（291-v1）
| ID | 维度 | 检查项 | 命令入口（固定） | 通过标准 | 证据落盘 |
| --- | --- | --- | --- | --- | --- |
| R1 | Source 元数据 | `UPSTREAM.yaml` 的 `repo/ref/commit/imported_at/rollback_ref` 完整且一致 | `make librechat-web-verify` | 返回码为 0，且无 source 缺失/冲突提示 | `docs/dev-records/assets/dev-plan-291/291-source-verify.log` |
| R2 | Patch Stack 回放 | patch 顺序可重放且构建成功 | `make librechat-web-build` | 返回码为 0，产物稳定输出至 `internal/server/assets/librechat-web` | `docs/dev-records/assets/dev-plan-291/291-web-build.log` |
| R3 | Runtime 启动健康 | 运行态服务可启动且健康 | `make assistant-runtime-up && make assistant-runtime-status` | status 为 healthy，关键组件无 crash/restart loop | `docs/dev-records/assets/dev-plan-291/291-runtime-status.json` |
| R4 | Runtime 版本锁一致 | runtime 版本与 lock 文件一致 | `make assistant-runtime-status` | 状态输出中的版本与 `versions.lock.yaml` 一致 | `docs/dev-records/assets/dev-plan-291/291-runtime-version-check.md` |
| R5 | 路由与入口边界 | 正式入口/静态前缀/compat API 分类与保护链一致 | `make check routing` | 门禁通过，无分类漂移 | `docs/dev-records/assets/dev-plan-291/291-routing-check.log` |
| R6 | Legacy 回流阻断 | 无旧桥正式职责、无双链路回流 | `make check no-legacy` | 门禁通过，无回流命中 | `docs/dev-records/assets/dev-plan-291/291-no-legacy.log` |
| R7 | 认证/启动兼容前置复核 | `292` 兼容层仍可支撑 runtime 启动链 | `make assistant-runtime-up && make assistant-runtime-status` | 不出现 auth/startup compatibility 回归 | `docs/dev-records/assets/dev-plan-291/291-auth-startup-compat-check.md` |
| R8 | 运行态清理闭环 | 回归后可正常下线，避免脏运行态影响后续 | `make assistant-runtime-down` | 返回码为 0，清理结果可复核 | `docs/dev-records/assets/dev-plan-291/291-runtime-down.log` |

### 4.2 Stopline 检查（强制）
1. [ ] **旧桥职责回流**：不得恢复 `bridge.js`、`data-assistant-dialog-stream`、`assistantDialogFlow`、`assistantAutoRun` 的正式职责语义。
2. [ ] **旧入口职责回流**：`/assistant-ui/*` 不得恢复为正式交互入口；`/app/assistant` 不得恢复为正式聊天承载面。
3. [ ] **双口径回流**：不得出现“双正式入口、双正式消息落点、双正式验收口径”。
4. [ ] **命令门禁（固定）**：`make check routing`、`make check no-legacy`；任一失败即 `291` 失败，不得进入 `285`。

### 4.3 证据与索引结构（291-v1）
1. [ ] 证据根目录固定为：`docs/dev-records/assets/dev-plan-291/`。
2. [ ] 索引文件固定为：`docs/dev-records/assets/dev-plan-291/291-evidence-index.json`。
3. [ ] 索引条目字段固定为：`id`、`command`、`executed_at`、`exit_code`、`artifacts[]`、`result`、`owner`、`notes`。
4. [ ] 任一矩阵项若缺少索引条目或 `artifacts[]` 为空，视为未完成。

### 4.4 291 -> 285 交接包（固定）
1. [ ] `291-upgrade-compat-matrix.md`：回归矩阵与逐项结论。
2. [ ] `291-execution-report.md`：本次执行环境、命令摘要、失败与修复说明。
3. [ ] `291-risk-list.md`：按 `高/中/低` 分级的遗留风险及处置建议。
4. [ ] `291-handoff-to-285.md`：供 `285 §2.3/§3` 直接引用的通过清单与未决项。

## 5. 验收标准
1. [ ] `291-v1` 回归矩阵（R1~R8）全部完成，且每项均具备“命令结果 + 证据文件 + 结论”三元记录。
2. [ ] `237` 对应 source/runtime compatibility 条目在 `291` 中具备可复核证据，不再停留在抽象描述。
3. [ ] stopline 无命中（无旧入口/旧桥职责回流、无双口径回流）。
4. [ ] 交接包可被 `285` 直接引用，不需要在封板阶段新增命令或补判定标准。
5. [ ] 若 `291` 证据时间早于最近一次影响性合入，则本次结论失效，必须重跑后再判定通过。

## 6. 测试与门禁（SSOT 引用）
1. [ ] 触发器与门禁命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 文档改动至少通过 `make check doc`。
3. [ ] `291` 固定执行顺序：`librechat-web-verify -> librechat-web-build -> assistant-runtime-up -> assistant-runtime-status -> check routing -> check no-legacy -> assistant-runtime-down`。

## 7. 交付物
1. [ ] 本计划文档：`docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`。
2. [ ] 升级兼容回归矩阵：`docs/dev-records/assets/dev-plan-291/291-upgrade-compat-matrix.md`。
3. [ ] 执行报告：`docs/dev-records/assets/dev-plan-291/291-execution-report.md`。
4. [ ] 风险清单：`docs/dev-records/assets/dev-plan-291/291-risk-list.md`。
5. [ ] 证据索引：`docs/dev-records/assets/dev-plan-291/291-evidence-index.json`。
6. [ ] 面向 `285` 的交接清单：`docs/dev-records/assets/dev-plan-291/291-handoff-to-285.md`。

## 8. 关联文档
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `docs/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`
- `AGENTS.md`
