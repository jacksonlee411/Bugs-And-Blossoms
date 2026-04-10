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
