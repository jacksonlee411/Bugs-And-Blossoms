# DEV-PLAN-291：237 升级兼容回归前置专项（供 285 封板复核）

**状态**: 规划中（2026-03-08 CST；可与 `290` 并行准备，但当前尚未形成供 `285` 直接引用的升级兼容证据）

## 1. 背景
1. [ ] `DEV-PLAN-271` 的 `S5` 明确要求补齐 `237` 的 source/runtime compatibility 回归后，才能进入 `285` 封板。
2. [ ] 当前 `237` 范围覆盖升级流程与回归闭环，作为前置专项拆分可降低与 `260/266` 验收并行时的冲突风险。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 形成可执行的升级兼容回归矩阵（source、patch stack、runtime、入口边界、关键交互路径）。
2. [ ] 固化升级回放流程与失败 stopline，确保“可预期、可回滚、可审计”。
3. [ ] 输出一组可直接供 `285` 使用的前置证据与复核清单。

### 2.2 非目标
1. [ ] 不在本计划内直接宣称封板完成（封板由 `DEV-PLAN-285` 承担）。
2. [ ] 不改写 `237` 主计划业务契约，仅做前置专项执行与证据收敛。
3. [ ] 不承担 `260` 业务 Case 验收。

## 3. 顺序与依赖
1. [ ] 可与 `260-M5` 验收并行准备，但最终通过判定应先于 `285` 启动。
2. [ ] 若回归发现旧入口、旧桥职责或双链路回流，必须回退到对应计划修复，不得带缺口进入封板。
3. [ ] 当前优先级低于 `288` 的最近战术阻塞，但必须在 `285` 启动前完成；不得因其“可并行准备”而被长期顺延。

### 3.1 输入冻结（291-v1）
1. [ ] **source 基线**：`third_party/librechat-web/UPSTREAM.yaml`（`repo/ref/commit/imported_at/rollback_ref`）作为来源冻结输入。
2. [ ] **patch stack 基线**：`third_party/librechat-web/patches/series` 作为唯一 patch 顺序入口；禁止在执行期临时散改顺序。
3. [ ] **runtime 基线**：`deploy/librechat/versions.lock.yaml` 与 `deploy/librechat/runtime-status.json` 作为运行态版本与健康事实输入。
4. [ ] **入口边界基线**：`/app/assistant/librechat`（正式入口）+ `/assets/librechat-web/**`（正式静态前缀）+ `292` compat façade 路径作为回归边界输入。

### 3.2 进入与退出条件（供 285 引用）
1. [ ] 进入条件：`292` 已完成；`288` 已进入默认基线复跑阶段；`285` 尚未启动封板执行。
2. [ ] 退出条件：输出 `291` 专属“回归矩阵 + 执行报告 + 风险清单 + stopline 检查结果”，并可被 `285` 直接引用，无需再补设计。

## 4. 实施步骤
1. [ ] 冻结回归矩阵（v1）：明确每个检查项的命令入口、通过标准、证据文件名。
2. [ ] 执行 source/patch 回放：按 `make librechat-web-verify` 与 `make librechat-web-build` 回放来源与 patch stack 兼容性。
3. [ ] 执行 runtime compatibility：按 `assistant-runtime-up/status/down` 流程核验运行态健康与版本锁一致性。
4. [ ] 执行入口/职责 stopline 检查：校验无旧入口正式职责、无旧桥职责回流、无双入口/双回执语义。
5. [ ] 汇总产物：形成可被 `285` 直接消费的前置通过清单与风险列表。

### 4.1 回归矩阵（291-v1）
| 维度 | 检查项 | 命令入口（固定） | 通过标准 | 证据落盘 |
| --- | --- | --- | --- | --- |
| Source 元数据 | 来源仓库/ref/commit/rollback_ref 可读且一致 | `make librechat-web-verify` | 校验脚本返回 0，且无 source 缺失提示 | `docs/dev-records/assets/dev-plan-291/291-source-verify.log` |
| Patch Stack 回放 | patch 顺序可重放、构建产物可生成 | `make librechat-web-build` | 构建返回 0，产物稳定输出到 `internal/server/assets/librechat-web` | `docs/dev-records/assets/dev-plan-291/291-web-build.log` |
| Runtime 兼容 | 运行态组件健康、版本锁可复核 | `make assistant-runtime-up && make assistant-runtime-status` | runtime status 为 healthy，且版本信息与锁文件一致 | `docs/dev-records/assets/dev-plan-291/291-runtime-status.json` |
| 路由与入口边界 | 正式入口/正式静态前缀/compat API 分类与保护链一致 | `make check routing` | 路由门禁通过，无分类漂移 | `docs/dev-records/assets/dev-plan-291/291-routing-check.log` |
| Legacy 回流阻断 | 无旧桥正式职责、无双链路回流 | `make check no-legacy` | 门禁通过，无回流命中 | `docs/dev-records/assets/dev-plan-291/291-no-legacy.log` |

### 4.2 Stopline 检查（强制）
1. [ ] **旧桥职责回流**：不得恢复 `bridge.js`、`data-assistant-dialog-stream`、`assistantDialogFlow`、`assistantAutoRun` 的正式职责语义。
2. [ ] **旧入口职责回流**：`/assistant-ui/*` 不得恢复为正式交互入口；`/app/assistant` 不得恢复为正式聊天承载面。
3. [ ] **双口径回流**：不得出现“双正式入口、双正式消息落点、双正式验收口径”。
4. [ ] **检查命令（固定）**：`make check no-legacy`、`make check routing`；未通过即 `291` 失败，不得进入 `285`。

### 4.3 产物清单（291 完成判定）
1. [ ] `docs/dev-records/assets/dev-plan-291/291-upgrade-compat-matrix.md`
2. [ ] `docs/dev-records/assets/dev-plan-291/291-execution-report.md`
3. [ ] `docs/dev-records/assets/dev-plan-291/291-risk-list.md`
4. [ ] 关键命令日志与状态快照（见 4.1 的证据落盘列）

## 5. 验收标准
1. [ ] `291-v1` 回归矩阵全部完成且每项均有“命令结果 + 证据文件 + 结论”三元记录。
2. [ ] `237` 对应 source/runtime compatibility 条目在 `291` 中具备可复核证据，不再停留在抽象描述。
3. [ ] stopline 检查无命中（无旧入口/旧桥职责回流、无双口径回流）。
4. [ ] 输出的通过清单可被 `285` 直接引用，无需在封板阶段补充命令或判定标准。

## 6. 测试与门禁（SSOT 引用）
1. [ ] 触发器与门禁命令以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [ ] 文档改动至少通过 `make check doc`。
3. [ ] `291` 固定命令基线（按执行顺序）：
   - [ ] `make librechat-web-verify`
   - [ ] `make librechat-web-build`
   - [ ] `make assistant-runtime-up`
   - [ ] `make assistant-runtime-status`
   - [ ] `make check routing`
   - [ ] `make check no-legacy`
   - [ ] `make assistant-runtime-down`

## 7. 交付物
1. [ ] 本计划文档：`docs/dev-plans/291-librechat-237-upgrade-compatibility-readiness-plan.md`。
2. [ ] 升级兼容回归矩阵与执行结果：`docs/dev-records/assets/dev-plan-291/291-upgrade-compat-matrix.md`。
3. [ ] 执行报告：`docs/dev-records/assets/dev-plan-291/291-execution-report.md`。
4. [ ] 风险清单：`docs/dev-records/assets/dev-plan-291/291-risk-list.md`。
5. [ ] 面向 `285` 的前置通过清单（可直接引用到 `285 §2.3/§3`）。

## 8. 关联文档
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/285-librechat-cutover-regression-and-closure-plan.md`
- `AGENTS.md`
