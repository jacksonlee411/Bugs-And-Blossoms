# DEV-PLAN-320 执行日志：org_node_key cutover stopline explain

**状态**: 已完成 1 次本地 stopline explain 采集（含 `target-shadow` consumer chain 基线，2026-04-10 CST）

## 1. 范围

1. [X] 使用当前 `org_id` source 运行库采集 Org / SetID / Staffing 主链路 explain 基线。
2. [X] 使用 committed `org_node_key` rehearsal target 库采集 Org `target-real` explain。
3. [X] 为 `SetID` / `Staffing` 补充 `target-shadow` explain：
   - dedicated target 内维护最小 shadow 表
   - 按 `org_code -> org_node_key` 导入当前态样本
   - 明确只用于 stopline 对比，不冒充 consumer runtime 已完成 cutover
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
  --target-url "postgres://app:<redacted>@127.0.0.1:5438/bugs_and_blossoms_orgnode_rehearsal?sslmode=disable" \
  --as-of 2026-01-01 \
  --output-dir .local/orgunit-stopline/2026-01-01
```

3. [X] 证据归档
   - 归档目录：`docs/dev-records/assets/dev-plan-320-stopline/`
   - 汇总报告：`report.md` / `report.json`
   - 样本选择：`samples.json`
   - explain 原始 JSON：`source-real-*.explain.json`、`target-real-*.explain.json`、`target-shadow-*.explain.json`

## 3. 关键结果

### 3.1 source-real

1. [X] `org-roots`: `0.705 ms`
2. [X] `org-children`: `0.654 ms`
3. [X] `org-details`: `0.877 ms`
4. [X] `org-search`: `0.512 ms`
5. [X] `org-subtree-filter`: `0.150 ms`
6. [X] `org-ancestor-chain`: `0.573 ms`
7. [X] `org-full-name-rebuild`: `0.586 ms`
8. [X] `org-move`: `5.489 ms`
9. [X] `setid-resolve`: `0.374 ms`
10. [X] `staffing-by-org`: `0.239 ms`

### 3.2 target-real

1. [X] `org-roots`: `0.383 ms`
2. [X] `org-children`: `0.564 ms`
3. [X] `org-details`: `0.401 ms`
4. [X] `org-search`: `0.256 ms`
5. [X] `org-subtree-filter`: `0.247 ms`
6. [X] `org-ancestor-chain`: `0.529 ms`
7. [X] `org-full-name-rebuild`: `4.181 ms`
8. [X] `org-move`: `0.582 ms`

### 3.3 target-shadow

1. [X] `setid-resolve`: `0.900 ms`
2. [X] `staffing-by-org`: `0.272 ms`

## 4. Stopline 判断

1. [X] 本次本地样本中未出现“大范围 `Seq Scan` 且无法修复”的 stopline 迹象。
2. [X] `ltree` / 祖先链相关 explain 仍以 index / bitmap scan / nested loop 为主。
3. [X] `org-move` 与 `full_name_path` 重建已采到 shared buffers 指标，可作为后续正式切窗前后的对比基线。
4. [X] `SetID` / `Staffing` 已补齐 `target-shadow` explain 基线，且未出现 `Seq Scan`。
5. [ ] `SetID` / `Staffing` 的 consumer runtime `target-real` explain 仍未完成。
   - 当前仓库 schema/runtime 仍以 `org_id` 为内部列口径。
   - 本次 `target-shadow` 仅解决 stopline 证据缺口，不等于 P4 consumer cutover 已闭环。

## 5. 本次修复

1. [X] 修复 [orgunit_stopline_capture.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_stopline_capture.go)
   - source `org-move` explain 原先仍向 `GENERATED ALWAYS` 列 `path_ids` 显式写值，首次采集会直接失败。
   - 已改为只写当前 schema 允许的列，stopline 采集可稳定完成。
2. [X] 为 `target-shadow` 增加最小 shadow 表与样本导入
   - `SetID` / `Staffing` 当前态样本由 source 库导出。
   - 通过 `org_code -> org_node_key` 映射写入 dedicated target 的 shadow 表后，再执行 explain。
3. [X] 修复 stopline stage 路由
   - 新增 `target-shadow` 后，采集端必须显式路由到 target 连接，否则会误在 source 库执行并因缺少 `org_node_key` 列失败。

## 6. 结论

1. [X] `DEV-PLAN-320` 的 Org 主读链路与写热点，已经具备一组可复现、可归档的本地 stopline explain 基线。
2. [X] `SetID` / `Staffing` 也已经具备 `source-real + target-shadow` 的补充证据。
3. [X] 当前本地 evidence 不构成 9.5 第 5 条的直接阻塞。
4. [ ] 在正式 cutover 前，仍需补齐“consumer runtime 真实 target-real 的 SetID / Staffing explain”这一缺口。
