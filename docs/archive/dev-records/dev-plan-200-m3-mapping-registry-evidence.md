# DEV-PLAN-200 M3 证据：Surface/Intent 映射注册表唯一命中

**对应计划**: `DEV-PLAN-203`  
**记录时间**: 2026-02-28 17:00 UTC

## 1. 契约冻结结论

1. 运行时必须先命中 `SurfaceIntentCapabilityRegistry`，再进入 SetID 与候选决议。  
2. 同时段同 `mapping_scope + surface + intent` 仅允许单一激活映射；缺失或歧义均 fail-closed。  
3. `tenant > global` 仅用于映射覆盖，不改变 tenant-only 数据读取边界。  
4. `mapping_missing`/`mapping_ambiguous` 为稳定错误码，禁止回退 legacy 路径。

## 2. 验证矩阵（冻结）

| 输入 | 期望 |
| --- | --- |
| 仅 tenant 命中 | allow，返回唯一 capability |
| tenant + global 同时命中 | allow，选择 tenant |
| 仅 global 命中 | allow，但后续数据仍 tenant-only |
| 0 命中 | deny，`mapping_missing` |
| 同层多命中 | deny，`mapping_ambiguous` |

## 3. 门禁与验证记录

1. [X] `make check capability-route-map` -> PASS  
2. [X] `make check routing` -> PASS  
3. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
4. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 2.2、6.1（步骤 3）、9.1（映射唯一且完整）与 9.2（条目 9）目标。  
- 与 `DEV-PLAN-203` 目标、门禁与验收标准一致。
