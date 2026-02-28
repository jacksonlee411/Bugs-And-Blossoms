# DEV-PLAN-200 M5 证据：跨层版本一致性（policy/composition/mapping）

**对应计划**: `DEV-PLAN-204`、`DEV-PLAN-206`  
**记录时间**: 2026-02-28 17:30 UTC

## 1. 契约冻结结论

1. 写入提交必须携带 `policy_version + composition_version + mapping_version`。  
2. `composition_version` 计算输入冻结为：`l1_snapshot_hash + l2_snapshot_hash + policy_version + mapping_version + resolved_setid + as_of + intent`。  
3. 任一版本字段缺失、过期或不一致均 fail-closed：`policy_version_conflict`/`composition_version_conflict`。  
4. `resolved_setid/as_of/intent` 变化必须触发版本冲突，阻断 TOCTOU。

## 2. 测试向量（冻结）

| 变更维度 | 预期 |
| --- | --- |
| 仅 `resolved_setid` 变化 | `composition_version` 改变 |
| 仅 `as_of` 变化 | `composition_version` 改变 |
| 仅 `intent` 变化 | `composition_version` 改变 |
| 输入完全一致 | hash 稳定一致 |

## 3. 门禁与验证记录

1. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
2. [X] `make check error-message` -> PASS  
3. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.1（步骤 7/8）、7（条目 6/7）与 9.2（条目 12/13）。  
- 与 `DEV-PLAN-204` 的 DTO 版本协议目标一致，可供 `DEV-PLAN-206` 直接复用。


## 5. 复用说明

- 该证据已在 `DEV-PLAN-206` 提交链路收口阶段复核，结论保持一致。
