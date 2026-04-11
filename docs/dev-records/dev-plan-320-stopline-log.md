# DEV-PLAN-320 执行日志：org_node_key cutover stopline explain

**状态**: 已完成 3 次本地 stopline explain 采集（最近一次：2026-04-11 CST；`target-real` 已覆盖 Org + SetID + committed Staffing，`target-shadow` 已移除）

## 1. 范围

1. [X] 使用当前 `org_id` source 运行库采集 Org / SetID / Staffing 主链路 explain 基线。
2. [X] 使用 committed `org_node_key` rehearsal target 库采集 Org + SetID + committed `staffing.position_versions` 的 `target-real` explain。
3. [X] 将 SetID explain 从 `target-shadow` 升级为 dedicated target 内真实 `orgunit.setid_binding_versions` 链路：
   - 按 `org_code -> org_node_key` 导入当前态样本
   - 不再依赖 `stopline` shadow 表
   - 同时保留“这不等于 P3 正式 runtime 切主”的边界说明
4. [X] 将 explain 原始 JSON、样本选择与汇总报告归档到仓库内证据目录。

## 2. 执行入口

1. [X] 单测回归

```bash
go test ./cmd/dbtool -count=1
```

2. [X] stopline 采集

```bash
go run ./cmd/dbtool orgunit-stopline-capture \
  --source-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable" \
  --target-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms_orgnode_rehearsal_20260411?sslmode=disable" \
  --as-of 2026-04-11 \
  --output-dir .local/orgunit-stopline/2026-04-11-targetreal-setid
```

3. [X] 证据归档
   - 归档目录：`docs/dev-records/assets/dev-plan-320-stopline/`
   - 汇总报告：`report.md` / `report.json`
   - 样本选择：`samples.json`
   - explain 原始 JSON：`source-real-*.explain.json`、`target-real-*.explain.json`

## 3. 关键结果

### 3.1 source-real

1. [X] `org-roots`: `0.243 ms`
2. [X] `org-children`: `0.227 ms`
3. [X] `org-details`: `0.222 ms`
4. [X] `org-search`: `0.174 ms`
5. [X] `org-subtree-filter`: `0.054 ms`
6. [X] `org-ancestor-chain`: `0.202 ms`
7. [X] `org-full-name-rebuild`: `0.162 ms`
8. [X] `org-move`: `2.313 ms`
9. [X] `setid-resolve`: `0.184 ms`
10. [X] `staffing-by-org`: `0.247 ms`

### 3.2 target-real

1. [X] `org-roots`: `0.181 ms`
2. [X] `org-children`: `0.165 ms`
3. [X] `org-details`: `0.159 ms`
4. [X] `org-search`: `0.186 ms`
5. [X] `org-subtree-filter`: `0.168 ms`
6. [X] `org-ancestor-chain`: `0.160 ms`
7. [X] `org-full-name-rebuild`: `1.051 ms`
8. [X] `org-move`: `0.240 ms`
9. [X] `setid-resolve`: `0.393 ms`
   - 证据：`docs/dev-records/assets/dev-plan-320-stopline/target-real-setid-resolve.explain.json`
   - 口径：在 dedicated target 的 `orgunit.setid_binding_versions` 内导入当前态样本，不再依赖 `stopline` shadow 表
10. [X] `staffing-by-org`: `0.107 ms`

## 4. Stopline 判断

1. [X] 本次本地样本中未出现“大范围 `Seq Scan` 且无法修复”的 stopline 迹象。
2. [X] `ltree` / 祖先链相关 explain 仍以 index / bitmap scan / nested loop 为主。
3. [X] `org-move` 与 `full_name_path` 重建已采到 shared buffers 指标，可作为后续正式切窗前后的对比基线。
4. [X] `Staffing` 的 consumer/runtime `target-real` explain 已完成，且未出现 `Seq Scan`。
   - 证据：`docs/dev-records/assets/dev-plan-320-stopline/target-real-staffing-by-org.explain.json`
   - 口径：使用 committed `staffing.position_versions`，当前态样本通过 `org_code -> org_node_key` 导入 dedicated target。
5. [X] `SetID` 的 consumer/runtime `target-real` explain 已完成，且未出现 `Seq Scan`。
   - 证据：`docs/dev-records/assets/dev-plan-320-stopline/target-real-setid-resolve.explain.json`
   - 口径：使用 dedicated target 的真实 `orgunit.setid_binding_versions` 当前态样本，不再依赖 `target-shadow`
6. [X] stopline 证据层面已不再存在 `P3` 仓库侧缺口。
   - 后续剩余事项是按 choreography 执行正式维护窗口，而不是继续补 stopline/runtime 证据。

## 5. 本次修复

1. [X] 修复 [orgunit_stopline_capture.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_stopline_capture.go)
   - source `org-move` explain 原先仍向 `GENERATED ALWAYS` 列 `path_ids` 显式写值，首次采集会直接失败。
   - 已改为只写当前 schema 允许的列，stopline 采集可稳定完成。
2. [X] 为 `target-real` 增加 committed Staffing explain bootstrap 与样本导入
   - 在 dedicated target 内自动 bootstrap `modules/staffing/infrastructure/persistence/schema/00001-00002`
   - 将 `Staffing` 当前态样本按 `org_code -> org_node_key` 映射写入 committed `staffing.positions / position_events / position_versions`
3. [X] 为 SetID 增加 `target-real` explain bootstrap 与样本导入
   - 在 dedicated target 的 `orgunit.setid_binding_versions` 中导入当前态样本
   - `target-real setid-resolve` 已替代 `target-shadow setid-resolve`
4. [X] 将 stopline source 采样同步到最新 Staffing schema
   - `staffing.position_versions` 已切到 `org_node_key`，采样 SQL 不能再引用 `org_unit_id`
   - 已改为通过 `orgunit.decode_org_node_key(...)` 与 source-real 的 `orgunit` 当前态做联查

## 6. 结论

1. [X] `DEV-PLAN-320` 的 Org 主读链路与写热点，已经具备一组可复现、可归档的本地 stopline explain 基线。
2. [X] `Staffing` 与 `SetID` 都已具备 `source-real + target-real` 的补充证据。
3. [X] 当前本地 evidence 不构成 9.5 第 5 条的直接阻塞。
4. [X] stopline 证据已不再阻塞进入正式维护窗口。
   - `target-real` explain、target runtime overlay 安装验证与 current runtime Gate 已在 2026-04-11 同步闭环。
