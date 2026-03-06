# DEV-PLAN-200 M2 证据：SetID 硬前置（先 ResolveSetID 再取数）

**对应计划**: `DEV-PLAN-203`  
**记录时间**: 2026-02-28 17:00 UTC

## 1. 契约冻结结论

1. 候选读取前必须先执行 `ResolveSetID(tenant, as_of, org_unit_id|business_unit_id, capability_key)`。  
2. `ResolveSetID` 失败时必须 fail-closed，不允许进入候选读取。  
3. 候选接口必须回显 `resolved_setid + setid_source`，并在 explain 中同口径输出。  
4. 禁止“先查候选再猜 setid”的逆序读取路径。

## 2. 校验场景（冻结）

| 场景 | 预期结果 | 错误码 |
| --- | --- | --- |
| 未执行 `ResolveSetID` 直接读候选 | deny | `setid_binding_missing` |
| `ResolveSetID` 未命中 | deny | `setid_not_found` |
| `ResolveSetID` 命中且租户基线存在 | allow | - |

## 3. 门禁与验证记录

1. [X] `make check capability-route-map` -> PASS  
2. [X] `make check routing` -> PASS  
3. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
4. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.1（步骤 4）、7（条目 9）、9.1（SetID 前置）与 9.2（条目 6）目标。  
- 与 `DEV-PLAN-203` 目标/步骤/验收逐项对齐，无新增 legacy 语义。
