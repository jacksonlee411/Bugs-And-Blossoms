# DEV-PLAN-200 M10C 证据：AI/UI 等价执行一致性

**对应计划**: `DEV-PLAN-210`  
**记录时间**: 2026-02-28 20:20 UTC

## 1. 等价执行冻结结论

1. AI 与 UI 必须物化为同构提交命令：`intent/request_id/trace_id/policy_version/composition_version`。  
2. 同 actor、同输入下，AI/UI 的 allow/deny、错误码、版本冲突判定必须一致。  
3. 禁止 `ai_*` 专用业务写入口与差异化提交语义。  
4. explain 输出口径一致，支持双入口结果对比与回放。

## 2. 对比矩阵（冻结）

| 输入条件 | 入口 | 预期结果 |
| --- | --- | --- |
| 同 actor + 同 intent + 同 payload | UI | 基线结果 |
| 同 actor + 同 intent + 同 payload | AI | 与 UI 完全一致 |
| 版本漂移后提交 | UI / AI | 同错误码阻断 |

## 3. 门禁与验证记录

1. [X] `make authz-pack && make authz-test && make authz-lint` -> PASS  
2. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
3. [X] `make check no-legacy` -> PASS  
4. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.2（步骤 12/14）、6.4（条目 10）、9.2（条目 24）。  
- 满足 `DEV-PLAN-210` 验收标准：AI/UI 等价且可回放。
