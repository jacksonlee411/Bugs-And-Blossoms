# DEV-PLAN-200 M7 证据：Explain 可解释输出与审计可回放

**对应计划**: `DEV-PLAN-204`  
**记录时间**: 2026-02-28 17:30 UTC

## 1. Explain 结构冻结

1. explain 输出固定包含：`input_context + matched_records + resolution_trace + final_decisions + versions + resolved_setid`。  
2. 字段级决议固定包含：`source_layer + winner_policy_ids + resolved_setid + setid_source`。  
3. 响应与 explain 的 `resolved_setid`、`policy_version`、`composition_version` 必须一致。  
4. 版本冲突场景需输出可机器解析的冲突来源字段，支持审计回放。

## 2. 可回放证据口径

- 回放最小集：`request_id + trace_id + input_context + versions + resolution_trace`。  
- 排障最小集：`winner_policy_ids + filtered_out_codes + final_decisions`。  
- 审计最小集：`actor_id + tenant_id + intent + captured_at`（命名对齐 `STD-001`）。

## 3. 门禁与验证记录

1. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
2. [X] `make check error-message` -> PASS  
3. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.1（步骤 9）、7（条目 13）与 9.2（条目 11）。  
- 满足 `DEV-PLAN-204`“可解释 + 可回放 + 可比对”目标。
