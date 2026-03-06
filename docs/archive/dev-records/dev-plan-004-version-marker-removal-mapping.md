# DEV-PLAN-004 记录：全仓去除版本标记——映射表（已冻结）

**状态**: 已冻结（2026-01-06 11:32 UTC）

> 本记录用于支撑 `docs/archive/dev-plans/004-remove-version-marker-repo-wide.md` 的 PR-1（盘点与映射表冻结）。
>
> 重要：为满足“最终全仓扫描命中为 0”的验收口径，本文避免直接写出版本标记字面量；统一用占位符 `<ver>` 表示“`v`/`V` + `4` 的紧邻组合”。

## 1. 扫描摘要（可复现）

以下命令以“字符串拼接”方式构造搜索模式，避免把目标字面量写进脚本/文档：

```bash
PATTERN_LOWER=$(printf '%s%s' v 4)
PATTERN_UPPER=$(printf '%s%s' V 4)
rg -n "(?i)(${PATTERN_LOWER}|${PATTERN_UPPER})" -S --hidden --glob '!.git/**' --glob '!**/*_templ.go'
rg --files --hidden --glob '!.git/**' | rg -S "(?i)(${PATTERN_LOWER}|${PATTERN_UPPER})"
```

本次盘点结果（2026-01-06 09:25 UTC）：
- 命中行：895
- 文件名命中：27（包含文档、迁移、Go 代码与 SQL schema 文件）
- 高风险命中主题（必须在后续 PR 原子切换，不允许混跑窗口）：
  - URL/query 参数：`/org/nodes?read=<ver>&as_of=...`（用户可见/对外契约）
  - 锁 key 前缀：`org:<ver>:...`（运行时语义承载；影响互斥/一致性）
  - 迁移文件名与 `atlas.sum` 引用（工具链一致性）

## 2. 文件名重命名映射（冻结候选）

### 2.1 `docs/dev-plans/**`（文件名）

| 类别 | 旧路径（含 `<ver>`） | 新路径（不含版本标记） | 风险等级 | 备注 |
| --- | --- | --- | --- | --- |
| docs-file | `docs/dev-plans/009-<ver>-implementation-roadmap.md` | `docs/dev-plans/009-implementation-roadmap.md` | low | 更新 `AGENTS.md` Doc Map 与全仓引用 |
| docs-file | `docs/dev-plans/010-<ver>-p0-prerequisites-contract.md` | `docs/dev-plans/010-p0-prerequisites-contract.md` | low | 同上 |
| docs-file | `docs/dev-plans/011-<ver>-tech-stack-and-toolchain-versions.md` | `docs/dev-plans/011-tech-stack-and-toolchain-versions.md` | low | 同上 |
| docs-file | `docs/dev-plans/012-<ver>-ci-quality-gates.md` | `docs/dev-plans/012-ci-quality-gates.md` | low | 同上 |
| docs-file | `docs/dev-plans/014-<ver>-parallel-worktrees-local-dev-guide.md` | `docs/dev-plans/014-parallel-worktrees-local-dev-guide.md` | low | 同上 |
| docs-file | `docs/dev-plans/017-<ver>-routing-strategy.md` | `docs/dev-plans/017-routing-strategy.md` | low | 同上 |
| docs-file | `docs/dev-plans/018-astro-aha-ui-shell-for-hrms-<ver>.md` | `docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md` | low | 同上 |
| docs-file | `docs/dev-plans/019-tenant-and-authn-<ver>.md` | `docs/dev-plans/019-tenant-and-authn.md` | low | 同上 |
| docs-file | `docs/dev-plans/021-pg-rls-for-org-position-job-catalog-<ver>.md` | `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md` | low | 同上 |
| docs-file | `docs/dev-plans/022-<ver>-authz-casbin-toolchain.md` | `docs/dev-plans/022-authz-casbin-toolchain.md` | low | 同上 |
| docs-file | `docs/dev-plans/023-superadmin-authn-<ver>.md` | `docs/dev-plans/023-superadmin-authn.md` | low | 同上 |
| docs-file | `docs/dev-plans/024-<ver>-atlas-goose-closed-loop-guide.md` | `docs/dev-plans/024-atlas-goose-closed-loop-guide.md` | low | 同上 |
| docs-file | `docs/dev-plans/025-sqlc-guidelines-for-<ver>.md` | `docs/dev-plans/025-sqlc-guidelines.md` | low | 同上 |
| docs-file | `docs/dev-plans/026-org-<ver>-transactional-event-sourcing-synchronous-projection.md` | `docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md` | low | 同上 |
| docs-file | `docs/dev-plans/028-<ver>-setid-management.md` | `docs/archive/dev-plans/028-setid-management.md` | low | 同上 |
| docs-file | `docs/dev-plans/029-job-catalog-<ver>-transactional-event-sourcing-synchronous-projection.md` | `docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md` | low | 同上 |
| docs-file | `docs/dev-plans/030-position-<ver>-transactional-event-sourcing-synchronous-projection.md` | `docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md` | low | 同上 |
| docs-file | `docs/dev-plans/031-greenfield-assignment-job-data-<ver>.md` | `docs/dev-plans/031-greenfield-assignment-job-data.md` | low | 同上 |

### 2.2 `migrations/orgunit/**`（迁移文件名）

| 类别 | 旧路径（含 `<ver>`） | 新路径（不含版本标记） | 风险等级 | 备注 |
| --- | --- | --- | --- | --- |
| migration-file | `migrations/orgunit/20260106080000_orgunit_<ver>_schema.sql` | `migrations/orgunit/20260106080000_orgunit_schema.sql` | high | 同步更新 `migrations/orgunit/atlas.sum`、以及任何引用该文件名的脚本/文档 |
| migration-file | `migrations/orgunit/20260106083000_orgunit_<ver>_engine.sql` | `migrations/orgunit/20260106083000_orgunit_engine.sql` | high | 同上；且该文件内含锁 key 前缀（见 3.2） |
| migration-file | `migrations/orgunit/20260106090000_orgunit_<ver>_read.sql` | `migrations/orgunit/20260106090000_orgunit_read.sql` | high | 同上 |
| migration-file | `migrations/orgunit/20260106090500_orgunit_<ver>_unbounded_validity.sql` | `migrations/orgunit/20260106090500_orgunit_unbounded_validity.sql` | high | 同上 |

### 2.3 `modules/orgunit/**`（SQL schema 文件名）

| 类别 | 旧路径（含 `<ver>`） | 新路径（不含版本标记） | 风险等级 | 备注 |
| --- | --- | --- | --- | --- |
| sql-file | `modules/orgunit/infrastructure/persistence/schema/00002_orgunit_<ver>_org_schema.sql` | `modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql` | medium | 同步更新引用（如工具链、sqlc/schema、文档） |
| sql-file | `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_<ver>_engine.sql` | `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` | medium | 该文件内含锁 key 前缀（见 3.2） |
| sql-file | `modules/orgunit/infrastructure/persistence/schema/00004_orgunit_<ver>_read.sql` | `modules/orgunit/infrastructure/persistence/schema/00004_orgunit_read.sql` | medium | 同步更新引用 |

### 2.4 `internal/server/**`（Go 文件名）

| 类别 | 旧路径（含 `<ver>`） | 新路径（不含版本标记） | 风险等级 | 备注 |
| --- | --- | --- | --- | --- |
| go-file | `internal/server/orgunit_<ver>_snapshot.go` | `internal/server/orgunit_snapshot.go` | low | 同步更新 package 内引用与测试文件名 |
| go-file | `internal/server/orgunit_<ver>_snapshot_test.go` | `internal/server/orgunit_snapshot_test.go` | low | 同上 |

## 3. 字面量/契约替换映射（冻结候选）

### 3.1 URL/query：read 模式

| 类别 | 旧值（含 `<ver>`） | 新值（不含版本标记） | 风险等级 | 备注 |
| --- | --- | --- | --- | --- |
| url | `read=<ver>` | `read=current` | high | 需同步更新：服务端解析、UI 链接、重定向 Location、测试断言、readiness 文档 |
| ui-copy | `Use <ver> read` | `Use current read` | medium | 当前页面文案为英文；保持风格一致即可 |
| ui-copy | `rename/move/disable 仅支持 <ver> 模式` | `rename/move/disable 仅支持 current 模式` | medium | 若不希望暴露实现细节，可改为“仅支持 current 模式”或“仅支持 current read” |

### 3.2 锁 key 前缀（运行时语义承载）

| 类别 | 旧值（含 `<ver>`） | 新值（不含版本标记） | 风险等级 | 备注 |
| --- | --- | --- | --- | --- |
| lock-key | `org:<ver>:<tenant_id>:<scope>` | `org:write-lock:<tenant_id>:<scope>` | high | 必须“同 PR 原子切换”：迁移 SQL、sqlc schema、以及所有文档/示例；不允许旧/新前缀混跑 |

## 4. 需确认（冻结前必须定稿）

以下两项一旦确定，后续 PR 不允许再变更方向（避免漂移）：

1. [X] `read=` 的“current”命名接受（并作为默认值）。
2. [X] 锁 key 前缀 `org:write-lock:` 接受（并要求同 PR 原子切换：迁移/SSOT/sqlc/文档一致）。
