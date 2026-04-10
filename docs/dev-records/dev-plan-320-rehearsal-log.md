# DEV-PLAN-320 执行日志：org_node_key cutover rehearsal

**状态**: 已完成 1 次本地 source/target rehearsal（2026-04-10 CST）

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
