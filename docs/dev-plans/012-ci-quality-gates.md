# DEV-PLAN-012：CI 质量门禁（Quality Gates：Lint/Tests/Routing/E2E）

**状态**: 执行中（门禁能力已基本落地；GitHub required checks 处于恢复接线阶段）（2026-04-09 CST 修订）

## 1. 背景与上下文 (Context)

`DEV-PLAN-009`～`DEV-PLAN-031` 为 Greenfield 实施阶段定义了“尽早冻结工程契约”的方向，这一方向本身没有变化。但当前仓库已不再处于“从 0 设计 CI 门禁”的阶段，而是进入了另一种更现实的状态：

- `Makefile`、`scripts/ci/*`、`scripts/e2e/*`、`config/coverage/policy.yaml` 等门禁能力已经大量存在；
- 多个专项门禁已经形成单独脚本与 `make check ...` 入口；
- 但 `.github/workflows/quality-gates.yml` 当前仍处于占位/禁用态，四个 required checks 尚未恢复为真正的质量约束。

因此，本计划的定位从“为新仓建立一套 CI 门禁”收敛为：

**保留四大 Quality Gates 母法不变，把已经存在的本地门禁能力重新接回 GitHub required checks，并确保本地/CI/文档三者重新对齐。**

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标

- [ ] **恢复四大 required checks 的真实执行**：CI 对外继续暴露并冻结以下四个 job 名称：
  - Code Quality & Formatting
  - Unit & Integration Tests
  - Routing Gates
  - E2E Tests
- [ ] **保持 `Makefile`/脚本单主源**：CI 只负责编排环境与调用 `make ...`，不在 workflow 中重新发明第二套命令串。
- [ ] **恢复 required checks 的真实约束力**：不得长期停留在 “temporarily disabled” 占位态；不得通过 `skipped` 掩盖门禁缺失。
- [ ] **保留并执行 100% 覆盖率策略**：覆盖率口径、阈值与排除项继续由单主源配置文件与脚本定义，CI 只恢复强制执行，不新增第二套阈值来源。
- [ ] **生成物与专项门禁继续纳入统一框架**：sqlc/Authz/Atlas/Assistant 配置/域名白名单/错误提示/时间参数显式化等门禁继续通过 Gate-1 编排，不各自旁路。

### 2.2 非目标（明确不做）

- 不在本计划内引入第二套迁移系统或第二套测试框架；`Atlas+Goose`、现有 Go test 与 Playwright 继续作为唯一主链。
- 不在本计划内重新定义每个模块的测试用例明细；各模块行为验收仍由各自 dev-plan 承接。
- 不在本计划内提供 CD/发布流水线；本计划仅处理 PR/merge 的质量门禁。
- 不在本计划内为了“先恢复 CI”而降低覆盖率阈值、扩大排除项或把 required checks 改成软约束。

## 2.3 工具链与门禁（SSOT 引用）

> 本计划不复制命令细节；命令、脚本与触发器以仓库事实源为准。

- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 版本基线：`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- 多语言门禁（仅 en/zh）：`docs/dev-plans/020-i18n-en-zh-only.md`
- Atlas+Goose 闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 门禁：`docs/dev-plans/025-sqlc-guidelines.md`
- Authz 门禁：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 路由门禁：`docs/dev-plans/017-routing-strategy.md`
- 覆盖率与测试分层：`docs/dev-plans/300-test-system-investigation-report.md`、`docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`

## 3. 当前实现快照（2026-04-09）

### 3.1 已存在且可复用的门禁能力

- [X] `Makefile` 已提供统一入口：`preflight`、`check fmt`、`check lint`、`test`、`check routing`、`e2e` 等。
- [X] `scripts/ci/test.sh` 已固定走覆盖率单主源入口。
- [X] `scripts/ci/coverage.sh` + `config/coverage/policy.yaml` 已落地 100% line coverage 策略。
- [X] `scripts/e2e/run.sh` 已是实际 E2E 入口，不再是 placeholder/no-op。
- [X] `check go-version`、`check error-message`、`check assistant-config-single-source`、`check assistant-domain-allowlist` 等专项门禁已具备脚本与 `Makefile` 接线。
- [X] `preflight` 已能作为“本地严格版对齐 CI”的聚合入口。

### 3.2 当前最大缺口

- [ ] `.github/workflows/quality-gates.yml` 仍处于占位禁用态：
  - 四个 job 名称已保留；
  - 但真实门禁步骤尚未恢复；
  - 部分步骤以 `if: false` 挂起；
  - 当前 required checks 只能证明“workflow 仍存在”，不能证明“质量约束已执行”。

### 3.3 本计划当前判断

因此，`DEV-PLAN-012` 当前不是“继续扩张新 gate”的计划，而是：

1. 恢复真实执行；
2. 维持单主源；
3. 把已经存在的能力稳定接回 CI；
4. 在恢复过程中保留四大 checks 的外部命名与职责分层。

## 4. 总体方案：四大 Quality Gates 母法保持不变

### 4.1 Required Checks：四大门禁（稳定、可预测）

CI workflow 继续以一个聚合工作流 `Quality Gates` 对外暴露四个稳定的 required checks：

1. Code Quality & Formatting
2. Unit & Integration Tests
3. Routing Gates
4. E2E Tests

约束：

- required checks 的 job 名称冻结，避免保护规则失效；
- required checks 的 job 不得长期通过占位输出来“假绿”；
- required checks 不得通过 job-level `if:` 变成 `skipped`；
- “按需执行”只能在 job 内部控制步骤，而不是让整个 job 消失。

### 4.2 “本地 = CI”单一入口原则（Makefile 驱动）

继续沿用以下原则：

- CI 只负责安装依赖、准备环境、调用 `make ...`、收集 artifact；
- 所有实际门禁逻辑必须留在 `Makefile` 与脚本中；
- 禁止在 workflow 中重新拼 Atlas/sqlc/Authz/coverage/E2E 命令串。

当前主入口继续以以下目标为主：

- `make preflight`
- `make check fmt`
- `make check lint`
- `make test`
- `make check routing`
- `make e2e`

### 4.3 Paths-Filter：作为恢复后的执行优化，而不是当前事实

paths-filter 与 no-op 机制仍然是本计划的正确目标形态：

- 命中变更时 Full Run；
- 未命中时在 job 内执行 no-op 并退出 0；
- required checks 始终给出稳定结论，而不是 `skipped`。

但截至当前，仓库现实尚未恢复到这一阶段。本计划先恢复真实执行，再在第二阶段细化 paths-filter，以免继续维持“设计上很完整、运行上全禁用”的反差。

## 5. Gate 结构与当前接线口径

### 5.1 Gate 1：Code Quality & Formatting（首个恢复目标）

Gate-1 继续承载静态检查与生成物一致性，不跑长期运行态。当前应纳入并恢复执行的内容包括：

- Go：`make check fmt`、`make check lint`
- Docs：`make check doc`
- Go 版本：`make check go-version`
- 错误提示：`make check error-message`
- No-Legacy：`make check no-legacy`
- Assistant 配置单主源：`make check assistant-config-single-source`
- Assistant 域名白名单：`make check assistant-domain-allowlist`
- scope/package、granularity、capability、request-code、as-of-explicit、dict-tenant-only 等专项门禁
- sqlc / Authz / Atlas 等生成物与一致性门禁
- UI build 相关生成物一致性
- `git status --porcelain` 为空的统一断言

判断：

- 这是当前最适合优先恢复的 required check；
- 依赖最少；
- 能最快把“占位态 CI”恢复为真实约束。

### 5.2 Gate 2：Unit & Integration Tests（第二阶段恢复）

Gate-2 继续负责：

- Go tests
- 100% coverage policy
- 需要数据库的集成测试初始化
- 覆盖率与测试日志 artifact

当前策略：

- 覆盖率单主源已存在，不再新增第二套配置；
- `test.sh -> coverage.sh -> config/coverage/policy.yaml` 的调用链继续作为唯一实现；
- 恢复重点在 CI 环境接线，而不是重新设计覆盖率规则。

### 5.3 Gate 3：Routing Gates（第三阶段恢复）

Gate-3 继续以 `make check routing` 为唯一入口，负责阻断：

- allowlist 缺失/漂移
- 路由分类不一致
- responder 契约漂移

该 gate 相对独立，适合作为测试恢复后的第三步接回。

### 5.4 Gate 4：E2E Tests（最后恢复，但恢复后必须真跑）

Gate-4 继续以 `make e2e -> scripts/e2e/run.sh` 为唯一入口，负责：

- 跑最小稳定 smoke 集
- 产出报告、trace、screenshot、video artifact
- 缺依赖/0 tests/运行态缺口时 fail-fast

约束保持不变：

- 命中 Full Run 时不得 no-op；
- 不得用占位输出代替真实执行；
- 不得以 `skipped` 规避 required check 约束。

## 6. 分阶段恢复计划（代替“新仓建设清单”）

### 6.1 Phase A：恢复 Code Quality & Formatting

目标：

- 把最稳定、依赖最少的 Gate-1 恢复为真实执行；
- 去掉 workflow 中的 `temporarily disabled` 占位步骤；
- 优先接回已在 `Makefile` 中稳定存在的专项门禁。

入口条件：

- 当前 `Makefile` 与脚本入口可本地稳定执行。

出口条件：

- `Code Quality & Formatting` job 不再是占位态；
- 至少覆盖 `check go-version`、`check error-message`、`check assistant-config-single-source`、`check assistant-domain-allowlist`、`check doc`、`check fmt`、`check lint`；
- workflow 不再通过 `if: false` 挂起这些步骤。

### 6.2 Phase B：恢复 Unit & Integration Tests

目标：

- 接回 `make test`；
- 在 CI 中恢复覆盖率与测试结果的真实约束；
- 保持 100% 覆盖率单主源。

出口条件：

- `Unit & Integration Tests` job 不再输出占位信息；
- CI 真实执行 `make test`；
- 覆盖率失败能阻断 PR。

### 6.3 Phase C：恢复 Routing Gates

目标：

- 接回 `make check routing`；
- 让路由 allowlist/分类/responder 漂移重新受到 required check 约束。

### 6.4 Phase D：恢复 E2E Tests

目标：

- 接回 `make e2e`；
- 上传 Playwright artifact；
- 保持 fail-fast 纪律，不允许 0 tests 假绿。

出口条件：

- `E2E Tests` job 真实跑最小稳定集；
- 缺依赖或用例缺失返回非 0；
- failure artifact 可用于排障。

## 7. 失败路径与排障（Fail-Fast & Debuggability）

- [ ] 生成物漂移：CI 必须打印 `git status --porcelain`，必要时上传 diff artifact。
- [ ] 测试/覆盖率失败：CI 必须保留足够日志，至少能区分编译失败、测试失败、覆盖率阈值失败。
- [ ] DB/生成物失败：Atlas/goose/sqlc/Authz 的关键输出应可回溯。
- [ ] E2E 失败：继续保留 Playwright trace/screenshot/video/report artifact。
- [ ] 恢复阶段不得通过“先临时 echo 一下”或 “if: false” 维持绿灯。

## 8. 测试与覆盖率（100% 门禁继续作为单主源）

> 本节从“待建设”更新为“已落地单主源，待恢复 CI 强制执行”。

- **覆盖率口径**：[X] line coverage。
- **策略文件**：[X] `config/coverage/policy.yaml` 已存在，当前阈值为 `100`。
- **执行入口**：[X] `scripts/ci/test.sh` 与 `scripts/ci/coverage.sh` 已固定为唯一执行链路。
- **统计范围**：[X] 仅统计手写 Go 代码；排除项由策略文件定义。
- **排除规则约束（停止线）**：[ ] 不允许为了恢复 CI 而扩大排除项或降低阈值。
- **当前恢复任务**：[ ] 把这套已存在的覆盖率策略重新接回 `Unit & Integration Tests` required check。

## 9. 验收标准 (Acceptance Criteria)

- [ ] `.github/workflows/quality-gates.yml` 继续对外暴露四个稳定的 required checks，名称冻结。
- [ ] 四个 required checks 均不再处于 “temporarily disabled” 占位态。
- [ ] `Makefile` 继续作为 CI 的唯一业务入口；workflow 中不存在第二套独立命令链。
- [ ] `Code Quality & Formatting` 在 CI 中真实执行，不再依赖 `if: false` 步骤占位。
- [ ] `Unit & Integration Tests` 在 CI 中真实执行，并继续执行 100% coverage policy。
- [ ] `Routing Gates` 在 CI 中真实执行并阻断路由契约漂移。
- [ ] `E2E Tests` 在 CI 中真实执行最小稳定集，失败时具备可用 artifact。
- [ ] required checks 恢复为真实约束后，再进入 paths-filter/no-op 优化阶段；不得在恢复前以 `skipped` 作为替代。
- [ ] 覆盖率策略、E2E 入口、专项脚本继续维持单主源，不因恢复 CI 而产生双轨实现。

## 10. 参考与链接 (Links)

- `docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`
- `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/019-tenant-and-authn.md`
- `docs/dev-plans/020-i18n-en-zh-only.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- `docs/dev-plans/025-sqlc-guidelines.md`
- `docs/dev-plans/025a-sqlc-schema-export-consistency-hardening.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- `Makefile`
- `config/coverage/policy.yaml`
- `scripts/ci/test.sh`
- `scripts/ci/coverage.sh`
- `scripts/e2e/run.sh`
- `.github/workflows/quality-gates.yml`
