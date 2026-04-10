# DEV-PLAN-320 执行日志：org_node_key cutover stopline explain

**状态**: 已完成 2 次本地 stopline explain 采集（最近一次：2026-04-11 CST；`target-real` 已覆盖 Org + committed Staffing，`target-shadow` 仅保留 SetID）

## 1. 范围

1. [X] 使用当前 `org_id` source 运行库采集 Org / SetID / Staffing 主链路 explain 基线。
2. [X] 使用 committed `org_node_key` rehearsal target 库采集 Org + committed `staffing.position_versions` 的 `target-real` explain。
3. [X] 为仍未切到 `target-real` 的 SetID consumer 链路保留最小 `target-shadow` explain：
   - dedicated target 内维护最小 shadow 表
   - 按 `org_code -> org_node_key` 导入当前态样本
   - 明确只用于 SetID stopline 对比，不冒充 consumer runtime 已完成 cutover
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
  --output-dir .local/orgunit-stopline/2026-04-11
```

3. [X] 证据归档
   - 归档目录：`docs/dev-records/assets/dev-plan-320-stopline/`
   - 汇总报告：`report.md` / `report.json`
   - 样本选择：`samples.json`
   - explain 原始 JSON：`source-real-*.explain.json`、`target-real-*.explain.json`、`target-shadow-*.explain.json`

## 3. 关键结果

### 3.1 source-real

1. [X] `org-roots`: `0.329 ms`
2. [X] `org-children`: `0.361 ms`
3. [X] `org-details`: `0.351 ms`
4. [X] `org-search`: `0.166 ms`
5. [X] `org-subtree-filter`: `0.069 ms`
6. [X] `org-ancestor-chain`: `0.206 ms`
7. [X] `org-full-name-rebuild`: `0.591 ms`
8. [X] `org-move`: `4.067 ms`
9. [X] `setid-resolve`: `0.253 ms`
10. [X] `staffing-by-org`: `0.131 ms`

### 3.2 target-real

1. [X] `org-roots`: `0.201 ms`
2. [X] `org-children`: `0.128 ms`
3. [X] `org-details`: `0.126 ms`
4. [X] `org-search`: `0.079 ms`
5. [X] `org-subtree-filter`: `0.091 ms`
6. [X] `org-ancestor-chain`: `0.182 ms`
7. [X] `org-full-name-rebuild`: `1.080 ms`
8. [X] `org-move`: `0.145 ms`
9. [X] `staffing-by-org`: `0.091 ms`

### 3.3 target-shadow

1. [X] `setid-resolve`: `0.900 ms`
   - 2026-04-11 这一轮已不再保留 `target-shadow staffing-by-org`

## 4. Stopline 判断

1. [X] 本次本地样本中未出现“大范围 `Seq Scan` 且无法修复”的 stopline 迹象。
2. [X] `ltree` / 祖先链相关 explain 仍以 index / bitmap scan / nested loop 为主。
3. [X] `org-move` 与 `full_name_path` 重建已采到 shared buffers 指标，可作为后续正式切窗前后的对比基线。
4. [X] `Staffing` 的 consumer/runtime `target-real` explain 已完成，且未出现 `Seq Scan`。
   - 证据：`docs/dev-records/assets/dev-plan-320-stopline/target-real-staffing-by-org.explain.json`
   - 口径：使用 committed `staffing.position_versions`，当前态样本通过 `org_code -> org_node_key` 导入 dedicated target。
5. [X] `SetID` 的 `target-shadow` explain 继续可用，且未出现 `Seq Scan`。
6. [ ] `SetID` 的 consumer/runtime `target-real` explain 与 Org kernel 正式切主仍未完成。
   - 当前 source-real / 运行主链仍保留大量 `org_id` 内核路径。
   - 本次 `target-shadow` 仅剩 SetID 链路，用于 stopline 对比，不等于 P3/P6 已闭环。

## 5. 本次修复

1. [X] 修复 [orgunit_stopline_capture.go](/home/lee/Projects/Bugs-And-Blossoms/cmd/dbtool/orgunit_stopline_capture.go)
   - source `org-move` explain 原先仍向 `GENERATED ALWAYS` 列 `path_ids` 显式写值，首次采集会直接失败。
   - 已改为只写当前 schema 允许的列，stopline 采集可稳定完成。
2. [X] 为 `target-real` 增加 committed Staffing explain bootstrap 与样本导入
   - 在 dedicated target 内自动 bootstrap `modules/staffing/infrastructure/persistence/schema/00001-00002`
   - 将 `Staffing` 当前态样本按 `org_code -> org_node_key` 映射写入 committed `staffing.positions / position_events / position_versions`
3. [X] 为 `target-shadow` 收缩到 SetID-only
   - `Staffing` explain 已切到 `target-real`
   - `target-shadow` 只保留 SetID binding explain
4. [X] 修复 stopline stage 路由
   - 新增 `target-shadow` 后，采集端必须显式路由到 target 连接，否则会误在 source 库执行并因缺少 `org_node_key` 列失败。

## 6. 结论

1. [X] `DEV-PLAN-320` 的 Org 主读链路与写热点，已经具备一组可复现、可归档的本地 stopline explain 基线。
2. [X] `Staffing` 已具备 `source-real + target-real` 的补充证据；`SetID` 目前保留 `source-real + target-shadow` 证据。
3. [X] 当前本地 evidence 不构成 9.5 第 5 条的直接阻塞。
4. [ ] 在正式 cutover 前，仍需补齐“SetID consumer/runtime 真实 target-real explain”与“Org kernel source-real -> target-real 正式切主”这两个缺口。
