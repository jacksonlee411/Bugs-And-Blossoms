# DEV-PLAN-025A：sqlc schema 导出一致性加固（取消夜间校验，PR 即时阻断）

**状态**: 已完成（2026-02-27 08:25 UTC）

## 1. 背景

`DEV-PLAN-025` 已确定“全局 `internal/sqlc/schema.sql` + `make sqlc-generate` + 生成物一致性门禁”的主路径，但当前仍存在以下执行风险：

- `scripts/sqlc/export-schema.sh` 采用模块名单硬编码，新增模块时存在静默漏导出风险。
- `scripts/ci/paths-filter.sh` 的 sqlc 触发器未覆盖 `scripts/sqlc/**`，改导出逻辑可能漏跑 sqlc gate。
- 文档与脚本口径有残留差异（例如导出脚本路径与触发器描述不完全一致）。

本计划用于在不引入“夜间异步兜底”的前提下，把一致性校验收敛为 **PR 即时阻断**。

## 2. 目标与非目标

### 2.1 目标（In Scope）

- [X] 保留“静态导出（schema SSOT → `internal/sqlc/schema.sql`）”为主路径，不增加日常开发依赖。
- [X] 新增“DB 一致性校验”并纳入 PR 阻断门禁（命中 sqlc/db 触发器时必须通过）。
- [X] 取消并禁止“夜间校验后再补救”的质量模型，避免滞后发现问题。
- [X] 收敛触发器与脚本路径口径，确保改动命中即触发对应 gate。
- [X] 将模块发现机制改为自动发现，消除硬编码模块清单。

### 2.2 非目标（Out of Scope）

- 不改变 `DEV-PLAN-025` 的 SQL-first / One Door / No Tx, No RLS 总体原则。
- 不引入第二套持久化主链路（如 SQLX/ORM 并行主路径）。
- 不在本计划中新增业务表结构或变更业务 API 契约。

## 3. 关键决策（ADR 摘要）

### 3.1 决策 A（选定）：静态导出继续作为主路径

- 继续使用 `scripts/sqlc/export-schema.sh` 生成 `internal/sqlc/schema.sql`，用于 sqlc 编译输入。
- 该路径保持“快、可复现、无额外运行时依赖”，适合作为默认开发路径。

### 3.2 决策 B（选定）：取消夜间校验，改为 PR 即时阻断

- **不采用**“仅夜间跑一致性校验”的模式。
- 对命中 sqlc/db 触发器的 PR，必须在 CI 中执行 DB 一致性校验并阻断失败。
- 任何“白天可合并、夜间再报警”的滞后质量模型均视为不满足本计划目标。

### 3.3 决策 C（选定）：自动模块发现替代硬编码模块清单

- `export-schema.sh` 不再写死 `iam/orgunit/jobcatalog/staffing/person`。
- 由脚本自动扫描 `modules/*/infrastructure/persistence/schema` 并按稳定顺序导出。

### 3.4 决策 D（选定）：比对规则规范化

- DB 导出与静态导出比对前进行统一规范化（排序与噪音剔除规则固定）。
- 规范化规则作为脚本内显式实现，不允许在 CI 中临时拼接 ad-hoc 过滤命令。

## 4. 实施步骤

1. [X] **文档口径收敛（025/012/AGENTS）**
   - 在 `DEV-PLAN-025` 中明确：静态导出是主路径；DB 一致性校验是 PR 阻断门禁，不存在夜间兜底模式。
   - 对齐 `DEV-PLAN-012` 的 gate 描述，补充 sqlc/db 命中时的一致性校验入口与阻断语义。
   - 对齐 `AGENTS.md` 触发器矩阵与文档地图引用。

2. [X] **脚本加固**
   - 改造 `scripts/sqlc/export-schema.sh`：自动发现模块 + 稳定排序导出。
   - 新增 `scripts/sqlc/verify-schema-consistency.sh`：执行“静态导出 vs DB 导出”规范化比对，失败即非 0 退出。

3. [X] **CI 门禁接入（阻断式）**
   - 在 `.github/workflows/quality-gates.yml` 中新增/接入 `sqlc schema consistency` gate（命中触发器时执行）。
   - 保证该 gate 为必过项；失败即 PR 不可合并。
   - 明确与 `make sqlc-generate`、`assert-clean` 的执行顺序与职责边界，避免重复或互相覆盖。

4. [X] **触发器修正**
   - 扩展 `scripts/ci/paths-filter.sh` 的 sqlc 触发器覆盖：
     - `scripts/sqlc/**`
     - `sqlc.yaml`
     - `internal/sqlc/**`
     - `modules/**/infrastructure/sqlc/**`
     - `modules/**/infrastructure/persistence/schema/**`

5. [X] **测试与回归**
   - 完成脚本语法校验与本地门禁验证（`make sqlc-generate`、`bash -n ...`、`make check doc`、`make sqlc-verify-schema` 的 fail-fast 验证）。
   - 记录执行证据到 `docs/dev-records/dev-plan-025a-execution-log.md`（时间戳、命令、结果、失败样例与修复结果）。

## 5. 停止线（命中即打回）

- [ ] 仍保留硬编码模块清单，导致新增模块无法自动纳入导出。
- [ ] 仍存在“夜间校验”或“异步报警后补救”作为主质量模型。
- [ ] 命中 sqlc/db 触发器时，PR 未执行或未阻断 DB 一致性校验。
- [ ] 通过临时忽略规则、手工修改生成物等方式让门禁“假绿”。

## 6. 验收标准（DoD）

- [X] `export-schema.sh` 已改为自动发现模块，且导出结果可复现。
- [X] `paths-filter` 已覆盖 `scripts/sqlc/**`，改动导出逻辑必触发 sqlc 相关 gate。
- [X] CI 已具备阻断式 `sqlc schema consistency` gate；命中触发器时失败可稳定拦截。
- [X] `DEV-PLAN-025` / `DEV-PLAN-012` / `AGENTS.md` 已完成口径收敛，无双轨描述。
- [X] 明确取消夜间校验方案，仓库中不存在对该方案的依赖与验收前提。

## 7. 依赖与引用（SSOT）

- `docs/dev-plans/025-sqlc-guidelines.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/011-tech-stack-and-toolchain-versions.md`
- `docs/dev-records/dev-plan-025a-execution-log.md`
- `AGENTS.md`
- `Makefile`
- `.github/workflows/quality-gates.yml`
