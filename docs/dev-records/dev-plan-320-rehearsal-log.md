# DEV-PLAN-320 执行日志：org_node_key cutover rehearsal

**状态**: 已完成 4 次本地 source/target rehearsal（最近一次：2026-04-11 CST）

## 1. 范围

1. [X] 为 `cmd/dbtool` 增加 target bootstrap 子命令，能够在专用 target 库内落纯 `org_node_key` schema。
2. [X] 新增 rehearsal 编排脚本，串联 `export -> check -> bootstrap -> import -> verify`。
3. [X] 用当前 dev 运行库作为 source、专用 rehearsal 库作为 target，完成一轮真实导出/导入/核对闭环。
4. [X] 固化 rehearsal 的关键前提，避免后续把 compat 运行库误当作 target 库原地导入。

## 2. 关键结论

1. [X] `DEV-PLAN-320` 的 rehearsal 不能在当前 dev/E2E 运行库原地完成。
   - source 库当前仍是旧 `org_id` 运行库 + runtime compat 适配，只能负责“当前态导出”。
   - target 必须是专用 `org_node_key` 目标库，用于 schema bootstrap、import 与 verify。
2. [X] source 导出必须使用 owner / bypass-RLS 连接。
   - 使用 `app_runtime` 连接导出时会直接命中 `app.current_tenant` 缺失错误。
   - 使用 `app` 连接可成功跨租户导出全量快照。
3. [X] 在专用 target 库内执行 committed import + standalone verify 可完整覆盖 7.3/7.4 所要求的结构核对闭环。

## 3. 新增入口

1. [X] [orgunit_snapshot_bootstrap.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_snapshot_bootstrap.go)
   - 新增 `orgunit-snapshot-bootstrap-target`
   - 负责创建扩展、`orgunit` schema、`assert_current_tenant(...)` 前置，并顺序应用 `00023-00025`
2. [X] [orgunit-node-key-rehearsal.sh](/home/lee/Projects/Bugs-And-Blossoms/scripts/db/orgunit-node-key-rehearsal.sh)
   - 串联 `orgunit-snapshot-export`
   - `orgunit-snapshot-check`
   - `orgunit-snapshot-bootstrap-target`
   - `orgunit-snapshot-import`
   - `orgunit-snapshot-verify`

## 4. 本地执行

1. [X] `go test ./cmd/dbtool -count=1`
   - 结果：通过
2. [X] 建立专用 target 库

```bash
docker exec bugs-and-blossoms-dev-postgres-1 \
  psql -U app -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS bugs_and_blossoms_orgnode_rehearsal;" \
  -c "CREATE DATABASE bugs_and_blossoms_orgnode_rehearsal OWNER app;"
```

3. [X] 执行 rehearsal

```bash
./scripts/db/orgunit-node-key-rehearsal.sh \
  --source-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable" \
  --target-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms_orgnode_rehearsal?sslmode=disable" \
  --as-of 2026-01-01 \
  --snapshot .local/orgunit-node-key-rehearsal/rehearsal-2026-01-01.json \
  --import-mode commit
```

4. [X] 关键输出
   - `orgunit-snapshot-export`: `tenants=316`
   - `orgunit-snapshot-check`: `nodes=1120`
   - `orgunit-snapshot-bootstrap-target`: `applied_files=3`
   - `orgunit-snapshot-import`: `tenants=316`
   - `orgunit-snapshot-verify`: `tenants=316`

## 5. 目标库核对

在 `bugs_and_blossoms_orgnode_rehearsal` 上执行：

```sql
SELECT count(*) AS tenant_count FROM orgunit.org_trees;
SELECT count(*) AS version_rows FROM orgunit.org_unit_versions;
SELECT count(*) AS code_rows FROM orgunit.org_unit_codes;
SELECT count(*) AS event_rows FROM orgunit.org_events;
SELECT count(*) AS registry_rows FROM orgunit.org_node_key_registry;
```

结果：

1. [X] `tenant_count = 316`
2. [X] `version_rows = 1120`
3. [X] `code_rows = 1120`
4. [X] `event_rows = 1120`
5. [X] `registry_rows = 1120`

## 6. 本次固化的操作约束

1. [X] 正式切窗前，source/target 必须明确分离，不能把 compat 运行库直接拿来做 import / verify。
2. [X] source 导出连接必须提前按 owner / bypass-RLS 口径准备好，不能在维护窗口内再临时切换账号。
3. [X] target rehearsal 库必须具备 DDL 权限，能应用 `00023-00025` 并创建 `orgunit_kernel`/RLS 相关对象。
4. [X] committed import 的 rehearsal 结果更适合作为 cutover 证据；若仅跑 `--import-mode dry-run`，则 verify 只发生在同事务内，不替代 committed target 验证。

## 7. 2026-04-10 第二轮实库 rehearsal（SetID strategy registry 闭环）

### 7.1 本轮目标

1. [X] 将 SetID strategy registry 的 rehearsal 子链路补成真实 source/target committed 闭环。
2. [X] 证明 fresh target 路径无需手工预置，单条脚本即可自动完成 target bootstrap。
3. [X] 将 SetID strategy registry 的 target schema stopline、fresh target-only 约束与 validate 链路纳入实跑证据。

### 7.2 执行前调整

1. [X] 收紧 [orgunit_setid_strategy_registry_snapshot.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_setid_strategy_registry_snapshot.go)
   - target schema 仍保留 `business_unit_id` / 旧约束 / 旧数字正则时，`import` / `verify` 直接 fail-closed。
   - `import` 改为 fresh target-only；若目标租户已有任意现存记录，不再“先删后灌”，而是直接 stopline。
2. [X] 扩展 [orgunit_snapshot_bootstrap.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_snapshot_bootstrap.go)
   - 新增 `--include-setid-strategy-registry`
   - 在启用 SetID rehearsal/validate 时，自动补齐 `00020-00022`，不再要求手工预置 target registry schema。
3. [X] 更新 [orgunit-node-key-rehearsal.sh](/home/lee/Projects/Bugs-And-Blossoms/scripts/db/orgunit-node-key-rehearsal.sh)
   - 命中 `--rehearse-setid-strategy-registry` / `--validate-setid-strategy-registry` 时，自动启用完整 target bootstrap。

### 7.3 执行入口

1. [X] 单测与文档门禁

```bash
go test ./cmd/dbtool -count=1
make check doc
```

2. [X] 创建 fresh target 库

本轮实际使用：

- source：`postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable`
- target：`postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms_rehearsal_20260410_autobootstrap?sslmode=disable`

3. [X] 执行 committed rehearsal（不传 `--skip-bootstrap`）

```bash
./scripts/db/orgunit-node-key-rehearsal.sh \
  --source-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable" \
  --target-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms_rehearsal_20260410_autobootstrap?sslmode=disable" \
  --as-of 2026-04-10 \
  --rehearse-setid-strategy-registry \
  --validate-setid-strategy-registry
```

### 7.4 关键输出

1. [X] Org snapshot
   - `orgunit-snapshot-export`: `tenants=410`
   - `orgunit-snapshot-check`: `nodes=1241`
   - `orgunit-snapshot-bootstrap-target`: `applied_files=6`
   - `orgunit-snapshot-import`: `tenants=410`
   - `orgunit-snapshot-verify`: `tenants=410`
2. [X] SetID strategy registry
   - `orgunit-setid-strategy-registry-export`: `rows=2162`
   - `orgunit-setid-strategy-registry-check`: `rows=2162`
   - `orgunit-setid-strategy-registry-import`: `tenants=1081 rows=2162`
   - `orgunit-setid-strategy-registry-verify`: `tenants=1081 rows=2162`
   - `orgunit-setid-strategy-registry-validate`: `rows=2162 business_unit_rows=0`
3. [X] 产物归档
   - `.local/orgunit-node-key-rehearsal/orgunit-snapshot-20260410T080716Z.json`
   - `.local/orgunit-node-key-rehearsal/setid-strategy-registry-20260410T080716Z.json`

### 7.5 本轮结论

1. [X] fresh target 路径已形成单命令闭环；不再需要先手工执行 `00023-00025` 或 `00020-00022`。
2. [X] SetID strategy registry 的 `source export -> snapshot check -> target import -> target verify -> stopline validate` 已在真实 source/target DB 上跑通。
3. [X] 320 要求的 target schema stopline 已前移到 `import` / `verify` 本身，而非仅靠独立 `validate` 命令兜底。
4. [ ] 当前 source 实库的 registry 当前态全部为 `tenant` 作用域。
   - 本轮真实数据结果：`business_unit_rows=0`
   - 因此“`business_unit_node_key` 可唯一落点 / 不可落点 / 歧义落点”的真实数据分支尚未被 source 实库命中。

## 8. 下一轮受控 rehearsal 方案（已执行，结果见第 9 节）

### 8.1 目标

1. [X] 在不污染当前 source-real 的前提下，显式命中 `business_unit` 作用域策略链路。
2. [X] 验证 `business_unit_node_key` 在 target 当前态下的三类结果：
   - 唯一落点：允许导入并通过 verify / validate
   - 无法落点：必须 stopline
   - 歧义落点：必须 stopline

### 8.2 建议环境

1. [X] 使用独立 `rehearsal/source` 库复制当前 source 基线，而不是直接改 `source-real`。
2. [X] 使用独立 `rehearsal/target` fresh 库，继续沿用当前脚本自动 bootstrap。

### 8.3 最小数据集

1. [X] `pass` 用例：构造 1 条 `business_unit` 策略，其 `business_unit_org_code` 能在 target 当前态唯一映射到 1 个 `business_unit_node_key`。
2. [X] `unresolved` 用例：构造 1 条 `business_unit` 策略，其 `business_unit_org_code` / `business_unit_node_key` 在 target 当前态无对应节点。
3. [X] `ambiguous` 用例：构造 1 条 `business_unit` 策略，其 `business_unit_node_key` 在 target 当前态命中多条当前有效记录。

### 8.4 验收口径

1. [X] `pass` 用例：
   - `export/check/import/verify/validate` 全部通过
2. [X] `unresolved` 用例：
   - `import` 或 `validate` 必须返回 `business_unit_node_key_unresolved`
3. [X] `ambiguous` 用例：
   - `import` 或 `validate` 必须返回 `business_unit_node_key_ambiguous`
4. [X] 任一失败场景都不得通过恢复旧列名、旧正则、旧接口或关闭 fresh target-only 门禁绕过。

## 9. 2026-04-10 第三轮实库 rehearsal（`business_unit` 受控三分支）

### 9.1 目标

1. [X] 在不污染 `source-real` 的前提下，显式命中 SetID strategy registry 的 `business_unit` 作用域。
2. [X] 对同一条受控策略分别验证：
   - `pass`：target 当前态唯一落点，允许 `import / verify / validate`
   - `unresolved`：target 当前态无落点，`import` 必须 stopline
   - `ambiguous`：target 当前态多落点，`import` 必须 stopline

### 9.2 执行方式

1. [X] 使用新脚本：
   - `scripts/db/orgunit-setid-strategy-registry-business-unit-rehearsal.sh`
2. [X] 脚本执行口径：
   - 从当前 `source-real` clone 出独立 `rehearsal/source`
   - 先导出 `orgunit-snapshot`
   - 通过 fresh `probe target` 先导入 Org 当前态，解析本轮 target-space 的 `org_node_key`
   - 再把该 key 回填到受控 `business_unit` 策略行，导出 registry snapshot
   - 对 `pass / unresolved / ambiguous` 三个 fresh target 分别执行 bootstrap -> org import -> registry import/verify/validate
3. [X] 本地执行命令：

```bash
scripts/db/orgunit-setid-strategy-registry-business-unit-rehearsal.sh \
  --source-url "$(./scripts/db/db_url.sh migration)" \
  --as-of 2026-04-10 \
  --case all
```

### 9.3 实跑结果

1. [X] 受控 seed 摘要
   - `tenant_uuid=00000000-0000-0000-0000-000000000001`
   - `org_code=1`
   - `org_id=10000000`
   - `predicted_target_org_node_key=AAAAAAAB`
2. [X] `pass`
   - `orgunit-setid-strategy-registry-import`: 通过
   - `orgunit-setid-strategy-registry-verify`: 通过
   - `orgunit-setid-strategy-registry-validate`: `business_unit_rows=1`
3. [X] `unresolved`
   - 命中预期 stopline 片段：`count=0 want=1`
4. [X] `ambiguous`
   - 命中预期 stopline 片段：`count=2 want=1`
5. [X] fresh target-only 约束未被绕过
   - 全程继续通过 `orgunit-snapshot-bootstrap-target --include-setid-strategy-registry`
   - 未恢复旧列名、旧约束、旧接口或 compat 双写

### 9.4 结论

1. [X] 320 在 SetID strategy registry 上要求的 `business_unit` 三类 target 当前态分支，已经通过受控实库 rehearsal 全部命中。
2. [X] 这补齐了“真实 source 当前态 `business_unit_rows=0` 无法自然覆盖”的证据缺口。
3. [ ] 仍未改变的一点是：source-real 当前自然数据样本仍全部为 `tenant` 作用域；这不再阻塞 320 的工具链 readiness 判断，但正式切主前仍应继续观察是否出现真实 `business_unit` 当前态样本。

## 10. 2026-04-11 第四轮实库 rehearsal（fresh target 复跑 + P3 准备度复核）

### 10.1 本轮目标

1. [X] 在 fresh target 上复跑 committed `source -> target` rehearsal，确认最新 stopline/consumer 改动未破坏导入闭环。
2. [X] 继续复跑 SetID strategy registry committed rehearsal，确认 `--rehearse-setid-strategy-registry` / `--validate-setid-strategy-registry` 在 `as_of=2026-04-11` 口径下稳定通过。
3. [X] 把 P3“正式切主准备”仍未完成的阻塞显式落档，避免把 rehearsal readiness 误写成“可直接切主”。

### 10.2 执行入口

1. [X] 创建 fresh target 库

```bash
docker exec bugs-and-blossoms-dev-postgres-1 \
  psql -U app -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS bugs_and_blossoms_orgnode_rehearsal_20260411;" \
  -c "CREATE DATABASE bugs_and_blossoms_orgnode_rehearsal_20260411 OWNER app;"
```

2. [X] 执行 committed rehearsal

```bash
./scripts/db/orgunit-node-key-rehearsal.sh \
  --source-url "$(./scripts/db/db_url.sh migration)" \
  --target-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms_orgnode_rehearsal_20260411?sslmode=disable" \
  --as-of 2026-04-11 \
  --rehearse-setid-strategy-registry \
  --validate-setid-strategy-registry
```

### 10.3 关键输出

1. [X] Org snapshot
   - `orgunit-snapshot-export`: `tenants=410`
   - `orgunit-snapshot-check`: `nodes=1241`
   - `orgunit-snapshot-bootstrap-target`: `applied_files=6`
   - `orgunit-snapshot-import`: `tenants=410`
   - `orgunit-snapshot-verify`: `tenants=410`
2. [X] SetID strategy registry
   - `orgunit-setid-strategy-registry-export`: `rows=2162`
   - `orgunit-setid-strategy-registry-check`: `rows=2162`
   - `orgunit-setid-strategy-registry-import`: `tenants=1081 rows=2162`
   - `orgunit-setid-strategy-registry-verify`: `tenants=1081 rows=2162`
   - `orgunit-setid-strategy-registry-validate`: `rows=2162 business_unit_rows=0`
3. [X] 产物归档
   - `.local/orgunit-node-key-rehearsal/orgunit-snapshot-20260410T225940Z.json`
   - `.local/orgunit-node-key-rehearsal/setid-strategy-registry-20260410T225940Z.json`

### 10.4 P3 准备度复核

1. [X] fresh target 路径与 committed import/verify 依然稳定，可继续作为 P3 正式切主前的 rehearsal 证据。
2. [X] SetID strategy registry 的 source/target committed 闭环依然稳定，但 source-real 当前自然数据仍为 `business_unit_rows=0`。
3. [ ] P3 正式切主本身仍未就绪；Org kernel 仍保留大量 `org_id` 运行路径。
   - 结构与事件账本仍以旧键为核心：`modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql`
   - 内核 replay / mutation / move / snapshot 主链仍大量使用 `org_id` / `parent_id`：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`
   - read model 仍输出 `org_id` / `parent_id`：`modules/orgunit/infrastructure/persistence/schema/00004_orgunit_read.sql`
   - SetID runtime schema / engine 仍以 `org_id` 为账本口径：`modules/orgunit/infrastructure/persistence/schema/00005_orgunit_setid_schema.sql`、`modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`
   - compat 解析层仍依赖 `org_id` 回读：`pkg/orgunit/resolve.go`
   - `internal/server` 仍存在 `org_id` 中心解析与 metadata compat：`internal/server/orgunit_nodes.go`、`internal/server/orgunit_field_metadata_store.go`

### 10.5 本轮结论

1. [X] 2026-04-11 的 fresh target 复跑证明：P2 工具链、SetID registry rehearsal 与后续 stopline 采集仍然稳定。
2. [X] 这足以支撑“P3 正式切主准备已推进到可复跑、可核对、可留证”的判断。
3. [ ] 但这仍不是“可直接切主”：在正式维护窗口前，必须继续收口 Org kernel source-real 的 `org_id` 主链与 SetID 的真实 `target-real` runtime。

## 11. 2026-04-11 用户可见验收与 LibreChat live 证据补齐

### 11.1 本轮目标

1. [X] 将 `DEV-PLAN-060` 主链路从“代码已收口”推进到“用户可见契约已收口”。
2. [X] 复跑 LibreChat formal/live 证据，确认 `tp288b` receipt contract 与 `tp290b` intent-action chain 不再阻塞。
3. [X] 为 320 留下“P5 用户可见验收已补齐，但最终 Gate 尚未全量复跑”的仓库内记录。

### 11.2 实际执行

1. [X] 复跑 LibreChat receipt contract

```bash
./scripts/e2e/run.sh tests/tp288b-librechat-live-task-receipt-contract.spec.js
```

2. [X] 复跑 LibreChat intent-action chain

```bash
./scripts/e2e/run.sh tests/tp290b-librechat-live-intent-action-chain.spec.js
```

3. [X] 复跑用户可见主链

```bash
./scripts/e2e/run.sh \
  tests/m3-smoke.spec.js \
  tests/tp060-02-master-data.spec.js \
  tests/tp060-03-person-and-assignments.spec.js \
  tests/tp070b-dict-release-ui.spec.js
```

### 11.3 关键结果

1. [X] `tp288b` 已转为 `passed`
   - 证据：`docs/dev-records/assets/dev-plan-288b/tp288b-live-evidence-index.json`
   - 结果：receipt `task_id -> poll_uri -> conversation refresh` 闭环成立，`final_task_status=succeeded`
2. [X] `tp290b` 已转为 `passed`
   - 证据：`docs/dev-records/assets/dev-plan-290b/tp290b-live-evidence-index.json`
   - 结果：`runtime_admission_gate=passed`，Case 1-4 全部 `passed`，`tp290b-data-baseline.json` 同步转为 `passed`
3. [X] 用户可见主链复跑通过
   - 结果：`m3-smoke`、`tp060-02`、`tp060-03`、`tp070b` 全部通过
4. [X] 文档契约已同步收口
   - 证据：`docs/dev-plans/060-business-e2e-test-suite.md`、`docs/dev-plans/062-test-tp060-02-master-data-org-setid-jobcatalog-position.md`、`docs/dev-plans/063-test-tp060-03-person-and-assignments.md`、`docs/dev-plans/064a-test-tp060-05-assistant-conversation-intent-and-tasks.md`
   - 结果：`SetID / Staffing / Assistant` 对外链路明确只允许 `org_code`，不再接受或回写 `org_unit_id / org_node_key`

### 11.4 本轮结论

1. [X] 320 的 `P5` 中“用户可见验收”部分已完成。
2. [X] 320 的 LibreChat live 证据不再阻塞后续收口。
3. [X] 截至第 11 节收尾时，320 仍未进入正式维护窗口：当时还需要 `P3` 正式切主准备完成。
   - 该阻塞已在第 14 节关闭。

## 12. 2026-04-11 最终 Gate 复跑与维护窗口判断

### 12.1 本轮目标

1. [X] 修复 `sqlc-verify-schema` 在 fresh replay 下的真实阻塞，避免只靠“数据库未就绪”掩盖问题。
2. [X] 完成最终 Gate 复跑：`go test`、`org-node-key-backflow`、`error-message`、`doc`、sqlc 一致性、E2E。
3. [X] 收敛 `internal/server` 已知脆弱测试，避免 `assistant_model_timeout` 偶发超时继续污染 Gate。
4. [X] 把“是否进入正式维护窗口”写成仓库内明确判断，而不是口头结论。

### 12.2 实际执行

1. [X] 修复 `sqlc-verify-schema` fresh replay 兼容

```bash
./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/staffing" --dir-format goose
DB_HOST=127.0.0.1 DB_PORT=5438 DB_USER=app DB_PASSWORD=app DB_ADMIN_DB=postgres make sqlc-verify-schema
```

2. [X] 收敛 `internal/server` 脆弱测试

```bash
go test ./internal/server -run 'TestAssistantModelGatewayMoreBranches/adapter_nil_ctx_and_nil_client_provider_unavailable' -count=20
go test ./internal/server -count=1
```

3. [X] 复跑最终 Gate

```bash
go test ./...
make check org-node-key-backflow
make check error-message
make check doc
make e2e
```

### 12.3 关键结果

1. [X] `sqlc-verify-schema` 已恢复为真实 schema 一致性门禁
   - 修复点：
     - `scripts/sqlc/verify-schema-consistency.sh` 在无宿主机 `psql/pg_isready` 时自动回退到 `postgres:17` 容器客户端
     - `migrations/staffing/20260411083000_staffing_position_events_org_node_key_constraint.sql` 改为仅在旧列存在时才执行 `org_unit_id -> org_node_key` 回填
   - 结果：fresh DB replay 不再因不存在的 `org_unit_id` 直接失败，`make sqlc-verify-schema` 通过
2. [X] `internal/server` 的已知偶发超时测试已收紧为确定性失败场景
   - 修复点：`assistant_reply_more_test.go` 不再依赖自签 TLS 握手去命中 `provider_unavailable`，改为本地关闭端口的确定性拒绝连接场景
   - 结果：目标子测试 `-count=20` 稳定通过，`go test ./internal/server -count=1` 通过
3. [X] 最终 Gate 全绿
   - `go test ./...`
   - `make check org-node-key-backflow`
   - `make check error-message`
   - `make check doc`
   - `make sqlc-verify-schema`
   - `make e2e`（31 passed）

### 12.4 维护窗口判断

1. [X] `P5` 已完成
   - 用户可见主链、LibreChat live、sqlc 一致性与最终 Gate 都已收口
2. [X] 截至第 12 节 Gate 复跑收尾时，2026-04-11 仍不能进入正式维护窗口
   - 当时阻塞：`P3` 的 Org kernel source-real -> target-real 正式切主准备仍缺
   - 该阻塞已在第 14 节通过 target runtime overlay 安装验证与 runtime 主链收口关闭
3. [X] reopen 条件无需改写
   - 当前保持 `DEV-PLAN-320` 既有 choreography：`停写 -> 快照 -> 导入/核对 -> 后端 -> 前端 -> smoke -> reopen`
   - 结论是不进入窗口，而不是放宽 reopen 条件或引入 compat/legacy 兜底

## 13. 2026-04-11 SetID `target-real` explain 补齐

### 13.1 本轮目标

1. [X] 把 SetID stopline 从 `target-shadow` 升到真正的 `target-real`。
2. [X] 修复 stopline source 采样仍引用 `staffing.position_versions.org_unit_id` 的旧口径。
3. [X] 将新 explain 证据归档到 `docs/dev-records/assets/dev-plan-320-stopline/`，并同步回写 320 文档。

### 13.2 实际执行

1. [X] 更新 [orgunit_stopline_capture.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_stopline_capture.go)
   - SetID explain 改为在 dedicated target 的 `orgunit.setid_binding_versions` 内导入当前态样本
   - source Staffing 采样改为使用 `orgunit.decode_org_node_key(...)` 适配当前 `org_node_key` schema
   - committed staffing target bootstrap 改为可重复执行，避免已有 target 表时重复应用 `00002` 失败
2. [X] 回归单测

```bash
go test ./cmd/dbtool -count=1
```

3. [X] 重跑 stopline capture

```bash
go run ./cmd/dbtool orgunit-stopline-capture \
  --source-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable" \
  --target-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms_orgnode_rehearsal_20260411?sslmode=disable" \
  --as-of 2026-04-11 \
  --output-dir .local/orgunit-stopline/2026-04-11-targetreal-setid
```

### 13.3 关键结果

1. [X] `target-real setid-resolve` 已生成
   - 证据：`docs/dev-records/assets/dev-plan-320-stopline/target-real-setid-resolve.explain.json`
   - 指标：`execution_time_ms=0.393`，`shared_hit_blocks=33`，`shared_read_blocks=3`
2. [X] `target-shadow-setid-resolve.explain.json` 已退出当前口径
   - 当前归档目录只保留 `source-real-*.explain.json` 与 `target-real-*.explain.json`
3. [X] 320 文档中的“SetID target-real explain 未完成”缺口已关闭
   - 证据：`docs/dev-records/dev-plan-320-stopline-log.md`
   - 结果：当前剩余主缺口收敛为 `P3` Org kernel 正式切主与维护窗口执行

### 13.4 本轮结论

1. [X] 320 的第 2 步“consumer runtime target-real explain 证据”已完成。
2. [X] 正式维护窗口尚未执行；但阻塞已不再是 `P3/P6` 的仓库侧准备缺口。

## 14. 2026-04-11 P3 仓库侧收口与维护窗口 readiness review

### 14.1 本轮目标

1. [X] 修正 `00023-00032` 作为 target runtime overlay 的真实安装断点，并验证 fresh DB 可顺序安装。
2. [X] 把 Org / SetID / Staffing / `internal/server` 的核心运行时调用切到 `org_node_key` / `char(8)` 主链。
3. [X] 收敛 `sqlc` 导出边界，避免把 target bootstrap 误导出到 current runtime schema。
4. [X] 复跑最终 Gate，明确仓库内是否已经具备进入正式维护窗口的条件。

### 14.2 实际执行

1. [X] 修正 target runtime overlay
   - 更新 `modules/orgunit/infrastructure/persistence/schema/00023_orgunit_org_node_key_schema.sql`
   - 更新 `modules/orgunit/infrastructure/persistence/schema/00026_orgunit_org_node_key_engine.sql`
   - 更新 `modules/orgunit/infrastructure/persistence/schema/00028_orgunit_org_node_key_submit_allocator.sql`
   - 更新 `modules/orgunit/infrastructure/persistence/schema/00030_orgunit_setid_org_node_key_schema.sql`
2. [X] 补 source compat runtime 对齐
   - 新增 `migrations/orgunit/20260411120000_orgunit_setid_org_node_key_runtime_compat.sql`
   - 将 `pkg/setid/setid.go`、`modules/orgunit/infrastructure/persistence/setid_pg_store.go`、`internal/server/orgunit_nodes.go` 切到 `char(8)` 调用签名
   - 将 `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql` 与 `migrations/staffing/20260411083000_staffing_position_events_org_node_key_constraint.sql` 改为 `org_node_key` 主链
3. [X] 校正 sqlc 导出边界
   - 更新 `scripts/sqlc/export-schema.sh`
   - 将 `orgunit 00023-00032` 明确排除出 `internal/sqlc/schema.sql`
4. [X] 执行验证

```bash
go test ./pkg/setid ./modules/orgunit/infrastructure/persistence ./internal/server -count=1
go test ./modules/staffing/... -count=1

docker run --rm --network host -e PGPASSWORD=app postgres:17 \
  psql "postgres://app:app@127.0.0.1:5438/<fresh-db>?sslmode=disable" \
  -v ON_ERROR_STOP=1 -f modules/orgunit/infrastructure/persistence/schema/00001_orgunit_schema.sql
# …顺序应用 00001-00032，最终通过

./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/orgunit" --dir-format goose
./scripts/db/run_atlas.sh migrate hash --dir "file://migrations/staffing" --dir-format goose
make sqlc-generate

go test ./...
make check org-node-key-backflow
make check error-message
make check doc
DB_HOST=127.0.0.1 DB_PORT=5438 DB_USER=app DB_PASSWORD=app DB_ADMIN_DB=postgres make sqlc-verify-schema
make e2e
```

### 14.3 关键结果

1. [X] `00023-00032` 已通过 fresh DB 顺序安装验证
   - 首个真实断点是 `00023` 先删 `org_path_ids()`、后删依赖表；修正顺序后全套通过
2. [X] source compat runtime 与 target runtime 已形成明确分层
   - source-real：继续通过 compat migration 承接维护窗口前运行
   - target-real：通过 `00023-00032` 提供纯 `org_node_key` runtime
3. [X] Org / SetID / Staffing / `internal/server` 的核心运行时调用已切到 `org_node_key` / `char(8)` 主链
4. [X] `sqlc-verify-schema` 已恢复为 current runtime 一致性校验
   - `internal/sqlc/schema.sql` 不再混入 target bootstrap overlay
5. [X] 最终 Gate 复跑通过
   - `go test ./...`
   - `make check org-node-key-backflow`
   - `make check error-message`
   - `make check doc`
   - `make sqlc-verify-schema`
   - `make e2e`

### 14.4 本轮结论

1. [X] `P3` 的仓库侧正式切主准备已经完成。
2. [X] `P6` 的最终 Gate / readiness review 已完成。
3. [X] 当前仓库状态已具备进入正式维护窗口条件。
4. [X] reopen 条件保持不变
   - 继续执行 `停写 -> 快照 -> 导入/核对 -> 后端 -> 前端 -> smoke -> reopen`
   - 若任一步失败，仍只允许整窗回滚
