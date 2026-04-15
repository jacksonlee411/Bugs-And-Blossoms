# DEV-PLAN-380A Readiness

**状态**: 进行中（2026-04-15；`iam.cubebox_*` schema / migration / sqlc / `cmd/dbtool` 工具链已落地并完成本轮仓内验证，`make test` 覆盖门禁已通过至 `98.10%`，真实 assistant 历史数据回填与本地文件索引导入的实库执行证据待下一轮在目标数据集上补齐）

## 1. 本轮交付范围

1. [X] `iam.cubebox_*` schema SSOT 已新增：
   - `modules/iam/infrastructure/persistence/schema/00011_iam_cubebox_conversations.sql`
   - `modules/iam/infrastructure/persistence/schema/00012_iam_cubebox_tasks.sql`
   - `modules/iam/infrastructure/persistence/schema/00013_iam_cubebox_files.sql`
2. [X] Goose migration 已新增：
   - `migrations/iam/20260414100000_iam_cubebox_conversations.sql`
   - `migrations/iam/20260414101000_iam_cubebox_tasks.sql`
   - `migrations/iam/20260414102000_iam_cubebox_files.sql`
3. [X] `sqlc` 已扩展为 `cubebox` 独立生成包，并通过 schema consistency 校验：
   - `modules/cubebox/infrastructure/sqlc/queries/*.sql`
   - `modules/cubebox/infrastructure/sqlc/gen/*`
   - `sqlc.yaml`
   - `internal/sqlc/schema.sql`
4. [X] `modules/cubebox` 正式 PG store 骨架已落地：
   - `modules/cubebox/module.go`
   - `modules/cubebox/infrastructure/persistence/store.go`
5. [X] `cmd/dbtool` 已新增 `380A` 承诺的四个入口：
   - `cubebox-backfill-assistant`
   - `cubebox-verify-backfill`
   - `cubebox-import-local-files`
   - `cubebox-verify-file-import`
6. [X] `migrations/iam/atlas.sum` 已刷新，新增 `cubebox` 三条 migration hash：
   - `20260414100000_iam_cubebox_conversations.sql`
   - `20260414101000_iam_cubebox_tasks.sql`
   - `20260414102000_iam_cubebox_files.sql`

## 2. 关键代码落点

1. [X] `cmd/dbtool` 入口与实现：
   - [main.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/main.go)
   - [cubebox_backfill.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/cubebox_backfill.go)
   - [cubebox_backfill_test.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/cubebox_backfill_test.go)
2. [X] `modules/cubebox` PG repository 骨架：
   - [store.go](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/infrastructure/persistence/store.go)
3. [X] `cubebox` sqlc query / 生成物：
   - [conversations.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/queries/conversations.sql)
   - [tasks.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/queries/tasks.sql)
   - [files.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/queries/files.sql)
   - [models.go](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen/models.go)
4. [X] `iam` schema / migration / atlas hash：
   - [00011_iam_cubebox_conversations.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/iam/infrastructure/persistence/schema/00011_iam_cubebox_conversations.sql)
   - [00012_iam_cubebox_tasks.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/iam/infrastructure/persistence/schema/00012_iam_cubebox_tasks.sql)
   - [00013_iam_cubebox_files.sql](/home/lee/Projects/Bugs-And-Blossoms/modules/iam/infrastructure/persistence/schema/00013_iam_cubebox_files.sql)
   - [atlas.sum](/home/lee/Projects/Bugs-And-Blossoms/migrations/iam/atlas.sum)

## 3. 本轮验证记录

### 3.1 命令、时间与结果

环境：

- 仓库：`/home/lee/Projects/Bugs-And-Blossoms`
- 时间：`2026-04-15 08:07:01 CST`
- 本轮目标：验证 `380A` 的 schema / sqlc / dbtool / `iam` 闭环，不在本轮伪造真实 assistant 历史数据或本地文件索引样本

执行结果：

1. [X] `go test ./cmd/dbtool/...`
   - 结果：通过
   - 说明：覆盖了 `cubebox` dbtool 的最小纯函数测试
2. [X] `make sqlc-generate`
   - 结果：通过
3. [X] `make sqlc-verify-schema`
   - 结果：通过
   - 说明：临时库应用 migration 与导出 schema 后，`[sqlc-verify] OK: schema is consistent`
4. [X] `./scripts/db/run_atlas.sh migrate hash --dir file://migrations/iam`
   - 结果：通过
   - 说明：刷新了 `migrations/iam/atlas.sum`，解决新增 `cubebox` migration 后的 checksum mismatch
5. [X] `make iam plan`
   - 结果：通过
   - 说明：`[db-plan] OK: no drift`
6. [X] `make iam lint`
   - 结果：通过
   - 说明：在刷新 `atlas.sum` 后恢复正常
7. [X] `make iam migrate up`
   - 结果：通过
   - 说明：三条 `iam_cubebox_*` migration 成功执行，随后 `rls-smoke` 通过
8. [X] `go test ./internal/server/...`
   - 结果：通过
   - 说明：`internal_rules_evaluate_api.go` 等本轮补测热点已收口
9. [X] `make test`
   - 结果：通过
   - 说明：总覆盖率门禁已过线，`[coverage] OK: total 98.10% >= threshold 98.00%`

### 3.2 迁移与 schema consistency 关键信息

1. [X] `make sqlc-verify-schema` 期间，`iam` 迁移链成功应用到临时库，新增 migration 依次为：
   - `20260414100000_iam_cubebox_conversations.sql`
   - `20260414101000_iam_cubebox_tasks.sql`
   - `20260414102000_iam_cubebox_files.sql`
2. [X] `migrations/iam/atlas.sum` 已包含上述三条 migration 的 hash。
3. [X] `make iam migrate up` 在真实本地数据库上执行成功，并附带 `rls-smoke` 通过。

## 4. `cmd/dbtool` 工具能力冻结结果

### 4.1 assistant backfill / verify

1. [X] `cubebox-backfill-assistant`
   - tenant 级显式事务
   - 显式注入 `app.current_tenant`
   - 基础实体链采用 `ON CONFLICT DO UPDATE` 可纠偏重跑
   - append-only 三张表采用 tenant 级 `DELETE + 全量重放`
2. [X] `cubebox-verify-backfill`
   - 校验逐表计数
   - 校验主键集合
   - 校验 `request_id / workflow_id / status / task_type / dispatch_status / last_error_code` 保真
   - 校验历史新增 nullable 快照列在回填后保持 `NULL`
   - 校验 append-only 三张表关键字段集合一致

### 4.2 本地文件索引导入 / verify

1. [X] `cubebox-import-local-files`
   - 读取 `.local/cubebox/files/index.json`
   - 校验 `file_id / tenant_id / file_name / media_type / size_bytes / sha256 / storage_key / uploaded_by / uploaded_at`
   - 校验 `objects/<storage_key>` 实体文件存在、大小一致、摘要一致
   - 若存在 `conversation_id`，要求目标 `cubebox_conversations` 中可映射
   - `conversation_id` 只写 `cubebox_file_links(link_role='conversation_attachment')`
2. [X] `cubebox-verify-file-import`
   - 对比 `index.json` 有效记录数与 `cubebox_files` 行数
   - 对比 `conversation_id` 记录数与 `cubebox_file_links` 行数
   - 对比文件元数据与 link 存在性

## 5. mismatch / stopline / 快照空值统计

### 5.1 本轮实际结果

1. [X] schema drift mismatch：为空
   - 证据：`make iam plan` 通过
2. [X] migration checksum mismatch：已清零
   - 证据：刷新 `migrations/iam/atlas.sum` 后 `make iam lint` 通过
3. [X] sqlc schema mismatch：为空
   - 证据：`make sqlc-verify-schema` 通过
4. [ ] assistant 历史数据 backfill mismatch：本轮未执行真实 tenant 数据回填
   - 说明：工具已具备，待在目标数据集上执行 `cubebox-backfill-assistant` / `cubebox-verify-backfill`
5. [ ] 本地文件索引导入 mismatch：本轮未执行真实 `.local/cubebox/files/index.json` 导入
   - 说明：工具已具备，待在目标索引数据集上执行 `cubebox-import-local-files` / `cubebox-verify-file-import`

### 5.2 历史新增快照列空值统计

1. [X] 工具逻辑已冻结：
   - `knowledge_snapshot_digest`
   - `route_catalog_version`
   - `resolver_contract_version`
   - `context_template_version`
   - `reply_guidance_version`
   - `policy_context_digest`
   - `effective_policy_version`
   - `resolved_setid`
   - `setid_source`
   - `precheck_projection_digest`
   - `mutation_policy_version`
2. [ ] 实际空值统计结果：待在真实 tenant 数据上执行 `cubebox-verify-backfill` 后补录

### 5.3 append-only 重跑前后对比

1. [X] 重跑语义已冻结为 tenant 级显式重建，不是追加补写
2. [ ] 实际“重跑前后计数 / 关键字段集合对比结果”：待在真实 tenant 数据上执行 committed backfill + verify 后补录

## 6. 当前结论

1. [X] `380A` 的 M1 / M2 / M3 / M4 所需的代码与工具链主体已落地。
2. [X] `iam` schema、Goose migration、Atlas checksum、`sqlc` 生成与 schema consistency、本地 `iam migrate up` 均已通过。
3. [X] `cmd/dbtool` 四个承诺入口已进入仓库，并具备最小测试覆盖。
4. [X] 代码侧测试与覆盖门禁已收口：`go test ./internal/server/...` 与 `make test` 均已通过，总覆盖率为 `98.10%`。
5. [ ] `380A` 仍未完成“真实历史数据回填 + verify 结果入档”的最终 readiness 关闭；该部分需要在目标数据库与本地文件索引样本上继续执行。

## 7. 下一步

1. [ ] 在目标数据库上执行 `cmd/dbtool cubebox-backfill-assistant --tenant <tenant-id>` 与 `cmd/dbtool cubebox-verify-backfill --tenant <tenant-id>`，补齐真实 mismatch/空值统计证据。
2. [ ] 在真实 `.local/cubebox/files/index.json + objects/` 上执行 `cmd/dbtool cubebox-import-local-files --tenant <tenant-id>` 与 `cmd/dbtool cubebox-verify-file-import --tenant <tenant-id>`，补齐文件面 stopline 证据。
3. [ ] 补齐 `go vet ./... && make check lint` 的本轮验证记录，并在需要时追加到 readiness 证据。
4. [ ] 在实数据迁移证据补齐后，回写 `docs/dev-plans/380a-cubebox-postgresql-data-plane-and-migration-contract.md` 的 M1~M4 / 9.2 / 9.4 状态。

## 8. 2026-04-15 覆盖率收口补记

1. [X] 补测并收口 [internal/server/internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go)：
   - `handleInternalRulesEvaluateAPI`
   - `resolveInternalRulesEvaluation`
   - `internalRuleDecisionFromError`
   - `internalRuleCandidateFromResolution`
   - `internalRuleBriefExplain`
2. [X] 本轮覆盖率结论：
   - `go tool cover -func=coverage/coverage.out | tail -n 1`
   - 输出：`total: (statements) 98.1%`
