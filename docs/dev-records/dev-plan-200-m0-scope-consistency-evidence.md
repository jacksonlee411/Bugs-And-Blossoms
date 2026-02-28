# DEV-PLAN-200 M0 证据：跨层作用域一致性（Mapping × Dict × SetID）

**对应计划**: `DEV-PLAN-201`  
**记录时间**: 2026-02-28 14:45 UTC

## 1. 冻结结论（契约）

1. `mapping_scope` 仅用于 capability 映射覆盖决议，不授予跨租户数据读取权限。  
2. 无论命中 `tenant mapping` 还是 `global mapping`，L2/L4 读取边界均为 tenant-only。  
3. 命中 `global mapping` 且租户基线缺失时，必须 fail-closed：`dict_baseline_not_ready`。  
4. explain 最小字段冻结：`mapping_scope + resolved_setid + setid_source + data_scope_decision`。

## 2. 作用域矩阵（冻结）

| mapping 命中 | tenant dict baseline | 结果 | 错误码 |
| --- | --- | --- | --- |
| tenant | 有 | allow | - |
| global | 有 | allow | - |
| tenant | 无 | deny | `dict_baseline_not_ready` |
| global | 无 | deny | `dict_baseline_not_ready` |

## 3. 门禁与验证记录

1. [X] `make check capability-route-map` -> PASS  
2. [X] `make check capability-key` -> PASS（无变更文件，规则扫描通过）  
3. [X] `make check no-legacy` -> PASS  
4. [X] `make check doc` -> PASS  
5. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）

## 4. 覆盖项对应关系

- 覆盖 `DEV-PLAN-200` 第 2.2A、5.1A（第 7 条）、9.1（scope consistency 行）、9.2（第 10 条）对应目标。  
- 与 `DEV-PLAN-201` 的目标、步骤、验收标准逐项对齐，无新增 legacy 语义。
