# DEV-PLAN-200 M0 证据：`allowed_value_codes` 集合语义

**对应计划**: `DEV-PLAN-202`  
**记录时间**: 2026-02-28 15:10 UTC

## 1. 集合语义冻结

1. `allowed_value_codes` 是最终可选集，不是候选池事实源。  
2. 候选池唯一来源：L2 Dict（tenant-only）。  
3. 求值顺序：先 `priority_mode`（层级），后 `local_override_mode`（补充/覆盖）。  
4. 不变量：`allowed_value_codes ⊆ candidate_pool`。  
5. 一致性阻断：  
   - `required=true` 且 DICT 字段时，不允许最终可选集为空；  
   - `default_value` 非空时必须属于最终可选集。

## 2. 组合矩阵（冻结）

| priority_mode | local_override_mode | 合法性 |
| --- | --- | --- |
| blend_custom_first | allow/no_override/no_local | 合法 |
| blend_deflt_first | allow/no_override/no_local | 合法 |
| deflt_unsubscribed | allow/no_override | 合法 |
| deflt_unsubscribed | no_local | 非法（fail-closed） |

## 3. 验证记录

1. [X] `make test` -> PASS（覆盖率门禁 100% 通过）。  
2. [X] `make check error-message` -> PASS。  
3. [X] `make check doc` -> PASS。

## 4. 结论

- `allowed_value_codes` 与 L2 候选池边界已收敛，避免“候选池/最终可选”语义混叠。  
- 非法组合与一致性违约均以 fail-closed 方式阻断。
