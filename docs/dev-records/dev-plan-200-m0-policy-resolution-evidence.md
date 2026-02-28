# DEV-PLAN-200 M0 证据：策略冲突决议 deterministic

**对应计划**: `DEV-PLAN-202`  
**记录时间**: 2026-02-28 15:10 UTC

## 1. 决议序列冻结

1. 候选过滤：`tenant + capability_key + intent + as_of(+setid/+business_unit)`。  
2. 分桶：场景覆盖桶 > 基线桶；BU 桶 > tenant 桶。  
3. 特异度：`setid` 精确 > `business_unit` 精确 > wildcard。  
4. 排序：`priority DESC -> effective_date DESC -> created_at DESC -> policy_id ASC`。  
5. 同位冲突无法化解时：`policy_conflict_ambiguous`（fail-closed）。

## 2. 回放证据结构

- `winner_policy_ids`  
- `resolution_trace`  
- `policy_version`  
- `resolved_setid`  
- `setid_source`

## 3. 验证记录

1. [X] `make test` -> PASS（覆盖率门禁 100% 通过）。  
2. [X] `make check error-message` -> PASS。  
3. [X] `make check doc` -> PASS。

## 4. 结论

- 决议算法与错误码口径已冻结到文档契约层，后续实现需严格按该序列执行。  
- 本阶段未引入新 DSL、未引入 legacy 回退通道，符合 `Simple > Easy`。
