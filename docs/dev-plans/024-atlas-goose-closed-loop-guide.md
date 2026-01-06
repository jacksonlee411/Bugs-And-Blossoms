# DEV-PLAN-024：Atlas + Goose 闭环指引（模块级 schema→迁移→门禁）

**状态**: 草拟中（2026-01-05 08:30 UTC）

## 1. 背景与上下文 (Context)

`DEV-PLAN-009`～`DEV-PLAN-023` 已冻结实施方向：Greenfield（全新实施）、DB Kernel（权威）、Go Facade（编排）、One Door Policy（唯一写入口），并在仓库内已有可复用的 Atlas+Goose 基线（`iam`/`orgunit`）。

为保证新模块（见 `DEV-PLAN-016`）在最早期即可做到 **“schema 变更可规划、迁移可生成、可回滚、CI 可拦截漂移”**，本计划提供一份可执行的 Atlas+Goose 闭环指引，并冻结“模块级受控目录”的统一口径。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标
- [ ] **闭环统一**：对模块提供统一的“Schema SSOT → Atlas plan/lint → Goose apply”闭环指引与验收标准。
- [ ] **模块级隔离**：每个模块拥有独立迁移目录与独立 goose 版本表，避免跨模块串号与门禁误触发。
- [ ] **命名与目录冻结**：冻结 `atlas.hcl` env 命名、migrations 目录命名、goose 版本表命名，降低实施期沟通成本。
- [ ] **Makefile/CI 对齐**：新增模块的入口与 CI 门禁以 `Makefile`/`.github/workflows/quality-gates.yml` 为 SSOT，对齐现有 `iam`/`orgunit` 样板。
- [ ] **失败路径明确**：常见故障（hash 漂移、stub 缺失、版本表冲突、破坏性变更等）提供明确处理路径，避免“凭经验修库”。

### 2.2 非目标（明确不做）
- 不在本计划内新增/修改任意业务表结构（`CREATE TABLE` 等 schema 变更由各子域实现计划承接）。
- 不在本计划内把所有模块强行合并到同一个 migrations/env（避免漂移与门禁耦合）。
- 不在本计划内引入新的迁移工具或替换 Atlas/Goose（版本冻结见 `DEV-PLAN-011`）。

## 2.3 工具链与门禁（SSOT 引用）
> 本文不复制仓库“触发器矩阵/命令清单/CI 脚本”，仅给出入口与约束；细节以 SSOT 为准。

- 触发器矩阵与红线：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- Atlas 配置（可选/辅助）：`atlas.hcl`（当前门禁以 `scripts/db/*.sh` 的显式参数为准）
- Atlas runner：`scripts/db/run_atlas.sh`（自动安装到 `bin/atlas`）
- Goose runner：`scripts/db/run_goose.sh`（自动安装到 `bin/goose`）
- DB 闭环脚本：`scripts/db/plan.sh`、`scripts/db/lint.sh`、`scripts/db/migrate.sh`
- 现有样板（代码即样板）：`migrations/iam` + `modules/iam/infrastructure/persistence/schema`；`migrations/orgunit` + `modules/orgunit/infrastructure/persistence/schema`
- 模块边界：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`

## 3. 统一闭环（架构与关键决策）

### 3.1 单一流水线（选定）
**选定**：采用与现有 `iam`/`orgunit` 同构的单一流水线：

```mermaid
flowchart TD
  Schema[编辑 Schema SSOT\nmodules/<module>/.../schema/*.sql] --> Plan[make <module> plan\natlas schema diff（migrations→schema）]
  Plan --> Diff[atlas migrate diff\n生成 goose 迁移（写入 migrations/<module>）]
  Diff --> Hash[atlas migrate hash\n更新 migrations/<module>/atlas.sum]
  Hash --> Lint[make <module> lint\natlas migrate validate]
  Lint --> Goose[make <module> migrate up/down\ngoose apply/rollback]
  CI[CI quality-gates] --> Plan
  CI --> Lint
  CI --> Goose
```

约束：
- Schema SSOT 与 migrations 必须保持一致；CI 通过 `plan`/`lint` + `migrate up`（必要时对关键模块叠加 smoke）证明闭环可用。
- goose 只负责 apply/rollback；迁移内容由 Schema SSOT + Atlas 约束驱动产生与校验。

### 3.2 模块级受控目录（选定）
**选定**：每个模块独立一套：
- `migrations/<module>/`（goose 格式）+ `atlas.sum`
- （可选）`atlas.hcl` env：`<module>_dev` / `<module>_ci`（用于手工运行 Atlas）
- goose 版本表：`goose_db_version_<module>`（必须独立）

动机：
- 降低耦合：任一模块的 schema/迁移改动只触发其自身门禁；
- 消除串号：goose 版本表隔离后，不同 migrations 目录即使使用相同 version_id 也不会互相污染；
- 更易审计：每套目录可独立回滚与核对（对齐 dev-record 的记录口径）。

### 3.3 命名约定（冻结）
> `<module>` 采用 `DEV-PLAN-016` 的模块名（`orgunit/jobcatalog/staffing/person`）。平台能力（例如 `iam`）的边界与数据所有权见 `DEV-PLAN-019`；如需为平台模块启用 Atlas+Goose，同样按本节规则扩展。

| 项 | 约定 |
| --- | --- |
| Schema SSOT | `modules/<module>/infrastructure/persistence/schema/*.sql` |
| 依赖 stub（预留） | `modules/<module>/infrastructure/atlas/core_deps.sql`（当前门禁脚本不读取；见 §3.4） |
| migrations 目录 | `migrations/<module>/` |
| Atlas env（可选） | `<module>_dev` / `<module>_ci`（用于手工运行 Atlas；门禁以脚本显式参数为准） |
| Atlas dev URL（门禁使用） | `ATLAS_DEV_URL=docker://postgres/17/dev?search_path=public`（可覆盖；`make <module> plan/lint` 会使用） |
| 目标库连接（goose apply） | `DATABASE_URL=...` 或 `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE`（见 `scripts/db/db_url.sh`） |
| goose 版本表（必须） | `goose_db_version_<module>`（由 `scripts/db/migrate.sh` 固定传入） |

### 3.4 跨模块依赖（冻结口径）
**冻结**：业务模块默认 **禁止跨模块 FK**；跨模块只存 ID（例如 `tenant_id`），不在 DB 层强绑定到别的模块表（对齐 `DEV-PLAN-016` 的边界原则）。

例外（必须显式论证）：
- 只有当某个 FK 是“必须的业务不变量/监管约束”时，才允许作为例外；并且必须在该子域 dev-plan 中明确：
  - 依赖顺序（例如 CI/本地必须先 `make iam migrate up`）
  - 工具链可复现方案（Atlas validate 的 dev DB 如何具备依赖对象）
  - 回滚影响面与停止线

关于 stub：
- 目前 `scripts/db/plan.sh`/`scripts/db/lint.sh` **不会读取** `modules/<module>/infrastructure/atlas/core_deps.sql`，因此该文件路径仅做目录预留，并不构成可用方案；若未来确需 stub，必须另开 dev-plan 落地脚本/CI 支持后才能使用。

关于 `tenants/tenant_domains`：
- 推荐：除 `iam`（见 `DEV-PLAN-019`）外，业务模块默认只保存 `tenant_id` 并依赖 RLS/`assert_current_tenant` 做 fail-closed，不对 `tenants` 做 FK（减少跨模块 DB 耦合与 stub 漂移）。
- 若某模块坚持对 `tenants` 建 FK：必须在该模块的实现 dev-plan 中明确“依赖顺序（先 apply `iam` 迁移）+ 工具链可复现策略 + 回滚影响面”，不得仅靠“本地库刚好有 tenants 表”隐式通过。

停止线：
- [ ] 任何因为“图省事”在多个模块重复维护同一份完整外部 schema（易 drift）；若出现，必须先收敛依赖策略再继续扩表。
- [ ] 任何把平台表（例如 `tenants`）的定义复制进业务模块 schema SSOT（形成双权威表达）。

### 3.5 新增表红线（仓库级合约）
- [ ] 任何将引入新表的迁移（出现 `CREATE TABLE`）在落地前必须获得人工确认（仓库红线，见 `AGENTS.md`）。

### 3.6 迁移版本号（`version_id`）与文件名规则（冻结）
为降低并行开发下的冲突与回滚误操作风险，**新建模块**的 goose 迁移文件名统一采用：
- `YYYYMMDDHHMMSS_<slug>.sql`（UTC 时间戳，14 位，作为 goose `version_id`）
- 示例：`20260105083000_orgunit_baseline.sql`

约束：
- 同一模块内 `version_id` 必须严格递增（按时间戳天然满足）。
- 禁止在不同模块共享同一个 goose 版本表；否则仅按 `version_id` 记账会导致串号（已在 3.2 冻结）。
- 现仓库既有目录（例如 `migrations/person` 的 `00001_...`）不强制回迁；本规则仅约束新建模块，避免在旧资产上制造额外 churn。

### 3.7 与现有样板的对照（避免“再发明一套”）
- `iam`：`modules/iam/infrastructure/persistence/schema` + `migrations/iam`；本地 `make iam plan && make iam lint && make iam migrate up`；CI 见 `.github/workflows/quality-gates.yml` 的 “DB Gates”。
- `orgunit`：`modules/orgunit/infrastructure/persistence/schema` + `migrations/orgunit`；本地 `make orgunit plan && make orgunit lint && make orgunit migrate up`；CI 同上。
- 脚本实现：`scripts/db/plan.sh`（schema drift 检测）、`scripts/db/lint.sh`（migrations validate）、`scripts/db/migrate.sh`（goose up/down + smoke）。

## 4. 推荐目录结构（模板）

以 `orgunit` 为例（其余模块同构）：
```
modules/orgunit/
  infrastructure/
    atlas/
      core_deps.sql                # 预留：当前 toolchain 不读取；默认禁止跨模块 FK（见 §3.4）
    persistence/
      schema/
        orgunit-schema.sql         # Schema SSOT（可按对象拆多个 sql）

migrations/orgunit/
  20260105083000_orgunit_baseline.sql        # goose Up/Down（首个 baseline；UTC 时间戳 version_id）
  20260105083010_orgunit_migration_smoke.sql # 可选但推荐：验证 up/down/redo 链路
  atlas.sum                        # 由 atlas migrate hash 维护
```

## 5. 接入落地步骤（给“新增模块闭环”用）

> 目标：把某个模块接入到“可 plan/lint/apply + CI 门禁”的状态；模板以仓库现有 `iam`/`orgunit` 为参照。

### 5.1 建议接入顺序（路线图）
为减少跨模块依赖与“先有鸡还是先有蛋”，建议接入顺序为：
1. `iam`：先落 `tenants/tenant_domains/sessions` 等平台表（见 `DEV-PLAN-019`），并把其 Atlas+Goose 闭环跑通。
2. `orgunit` / `jobcatalog` / `staffing`：按业务优先级逐个接入；每个模块独立 migrations/env/版本表。
3. `person`（identity）：按 `DEV-PLAN-027` 的契约落地；是否复用现有 `migrations/person` 取决于“是否新建 person 模块与新表”，需在实现计划中明确。

### 5.2 接入落地步骤
1. [ ] 准备 Schema SSOT：在 `modules/<module>/infrastructure/persistence/schema/` 创建/维护 schema SQL（SSOT）。
2. [ ] （默认不需要）跨模块 FK 默认禁止；如未来要支持跨模块依赖 stub，仅预留 `modules/<module>/infrastructure/atlas/core_deps.sql`（当前 toolchain 不读取，需另开 dev-plan 落地）。
3. [ ] 初始化 migrations 目录：创建 `migrations/<module>/`、baseline 迁移与 `atlas.sum`（由 Atlas 生成/维护）。
4. [ ] （可选）扩展 `atlas.hcl`：为 `<module>` 增加 `<module>_dev/<module>_ci` env，并绑定 `src=modules/<module>/.../schema` 与 `migration.dir=migrations/<module>`（便于手工运行 Atlas；门禁以脚本显式参数为准）。
5. [ ] 扩展 `Makefile`：在 `.PHONY` 加入 `<module>` 并添加空 target（`<module>:` → `@:`），使 `make <module> plan|lint|migrate up` 可用。
6. [ ] 扩展 CI：在 `.github/workflows/quality-gates.yml` 的 “DB Gates” step 中加入该模块的：
   - `make <module> plan`
   - `make <module> lint`
   - `make <module> migrate up`
   > 当前 CI 以 `paths.outputs.db == true` 粗粒度触发（避免漏跑）；Atlas/Goose 由 `scripts/db/run_atlas.sh`/`scripts/db/run_goose.sh` 自动安装，无需额外 install target。
7. [ ] 记录 readiness：在对应 dev-record 中登记命令与结果（时间戳/环境/结论），作为可追溯证据。

## 6. 日常开发闭环（给“改 schema 的开发者”用）

> 命令入口以 `Makefile` 为准；下述为“行为顺序”说明。当前仓库未提供 `diff/hash` 的 `make` 包装命令；生成迁移时建议使用 `scripts/db/run_atlas.sh` 并显式指定 `--dir/--dir-format/--to/--dev-url`（避免隐式参数漂移）。

1. [ ] 修改 Schema SSOT（必要时同步更新 `core_deps.sql`）。
2. [ ] 运行 `<module> plan`（drift 检测），确认 migrations 与 Schema SSOT 一致。
3. [ ] 用 Atlas 生成迁移：执行 `./scripts/db/run_atlas.sh migrate diff --dir "file://migrations/<module>" --dir-format goose --dev-url "${ATLAS_DEV_URL:-docker://postgres/17/dev?search_path=public}" --to "file://modules/<module>/infrastructure/persistence/schema" <slug>`。
4. [ ] 更新迁移 hash：执行 `./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/<module>" --dir-format goose`，提交 `atlas.sum`。
5. [ ] 运行 `<module> lint`（`atlas migrate validate`），确保 hash 与 SQL 语义可验证（`atlas migrate lint` 在 v0.38+ 为 Pro）。
6. [ ] 运行 `<module> migrate up`（goose apply）并做最小验证；需要回滚时用 `<module> migrate down`（建议 `GOOSE_STEPS=1`）。
7. [ ] 再次运行 `<module> plan`，期望输出为 No Changes（闭环收敛）。

## 7. 常见故障与处理（失败路径）

- [ ] `atlas migrate validate` 报 `atlas.sum` 不一致：运行 `./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/<module>" --dir-format goose` 后重新 validate，并提交 `atlas.sum`。
- [ ] `goose` 执行了“别的模块”的迁移：检查是否误用迁移目录（应为 `migrations/<module>`）或误用版本表（应为 `goose_db_version_<module>`）；建议优先使用 `make <module> migrate up|down`（内部固定 `-dir/-table`）。
- [ ] Atlas plan/lint 报引用表不存在（FK 依赖缺失）：优先移除跨模块 FK（默认口径）；如确需 FK，必须先补齐“依赖顺序 + 工具链可复现方案”，并在子域 dev-plan 论证后再落地。
- [ ] plan 输出出现大规模 drop/create：优先检查是否连接到错误 DB，或 DB 已被手工修改导致 drift；禁止用“手工改库”去对齐 schema，应回到迁移闭环。
- [ ] 需要破坏性变更（drop column/table）：先在子域 dev-plan 明确回滚与数据迁移策略，并通过 lint 的破坏性规则；禁止绕过门禁强推。

## 8. 验收标准 (Acceptance Criteria)

### 8.1 文档门禁（本计划交付）
- [ ] `make check doc` 通过。
- [ ] `AGENTS.md` Doc Map 已包含 `DEV-PLAN-024` 链接。

### 8.2 工具链门禁（模块接入后必须满足）
对任一接入模块 `<module>`：
- [ ] CI 的 “DB Gates”（`paths.outputs.db == true`）包含该模块的 `make <module> plan/lint/migrate up` 且可跑通（见 `.github/workflows/quality-gates.yml`）。
- [ ] goose 使用独立版本表：`goose_db_version_<module>`。
- [ ] 生成物无漂移：CI 的 `./scripts/ci/assert-clean.sh` 为 OK（`atlas.sum` 等生成物必须提交）。
- [ ] migrations 文件名符合 3.6 的 `version_id` 规则（新建模块）。

## 9. Simple > Easy Review（DEV-PLAN-003）

### 9.1 结构（减少耦合）
- 通过：按模块分套（独立 env/目录/版本表），把门禁耦合从“全仓库”降到“单模块”。
- 通过：goose runner 复用 `scripts/db/run_goose.sh`，避免脚本复制与参数漂移。

### 9.2 演化（确定性与可复现）
- 通过：命名约定冻结 + SSOT 引用明确（`Makefile`/CI/`scripts/db/*.sh`），避免“口头流程”。
- 风险：跨模块 FK 会迫使工具链复杂化（stub/dev DB/依赖顺序）；已冻结“默认禁止 FK + 例外需 dev-plan 论证”的停止线。

### 9.3 认知（Simple > Easy）
- 通过：把复杂度集中在一条闭环流水线（plan→diff→hash→lint→apply），不引入第二套迁移系统。
- 风险：若在 Makefile/CI 之外私自拼接 Atlas 命令，会造成不可复现；本文已明确入口与停止线。

### 9.4 维护（可替换性）
- 通过：闭环拆解成可替换环节（Schema SSOT、Atlas 校验、Goose apply），未来若替换执行器（非本计划）也有清晰边界。

## 10. 决策记录与待决策事项

> 目的：把关键口径显式化，避免在实现期“撞出来”；同时保留历史问题的可追溯性。

### 10.1 已冻结决策
- [x] **跨模块 FK：默认禁止**。跨模块仅存 ID；如确需 FK，必须在子域 dev-plan 论证“依赖顺序 + 工具链可复现方案 + 回滚影响面”，并落地对应脚本/CI 支持后方可启用（见 §3.4）。
- [x] **`core_deps.sql`：仅预留，不作为现行方案**。当前 `scripts/db/plan.sh`/`scripts/db/lint.sh` 不读取该文件，避免形成“第二权威表达”；后续若要支持，需另开计划实现（见 §3.4/§5.2）。
- [x] **门禁入口：以 `make <module> plan|lint|migrate` 与 `scripts/db/*.sh` 为 SSOT**。`atlas.hcl` 仅作手工辅助，防止出现多入口与参数漂移。
- [x] **CI DB Gates：先维持粗粒度触发**（`paths.outputs.db == true`），优先 fail-safe 避免漏跑；当模块数/耗时达到优化阈值再升级到模块级 filter（见 §5.2/§8.2）。
- [x] **smoke：默认只要求 `migrate up`**；仅对关键平台模块或首个垂直切片模块强制额外 smoke，其它模块按需引入，避免脚本/测试入口泛滥。

### 10.2 历史待决策问题（已收敛）
- [x] 跨模块依赖/FK 是否“允许/禁止/有限允许”？—— 已冻结为“默认禁止；例外需 dev-plan 论证并落地可复现闭环”（见 §3.4/§10.1）。
- [x] Atlas 执行入口是否收敛到 `atlas.hcl` env？—— 已冻结为“门禁 SSOT 以 `scripts/db/*.sh` 显式参数为准；`atlas.hcl` 仅作手工辅助”（见 §3.2/§10.1）。
- [x] CI DB Gates 触发粒度：粗粒度还是模块级 filter？—— 已冻结为“先维持粗粒度（避免漏跑），达到优化阈值再升级”（见 §5.2/§8.2/§10.1）。
- [x] 是否为每个模块定义最小 smoke（类似 `cmd/dbtool`）？—— 已冻结为“默认只要求 `migrate up`；关键模块/首个垂直切片模块可额外 smoke”（见 §8.2/§10.1）。
