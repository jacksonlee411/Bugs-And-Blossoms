# DEV-PLAN-200 M9A 证据：严格结构化输出（constrained decode）

**对应计划**: `DEV-PLAN-208`  
**记录时间**: 2026-02-28 19:20 UTC

## 1. 约束冻结结论

1. 模型输出阶段强制 `strict=true`，不满足 schema 的产物不得进入 lint/commit。  
2. schema 使用最小字段集与 `additionalProperties=false`，防止隐式扩展字段。  
3. 类型错误、必填缺失、枚举越界均返回稳定错误码并 fail-closed。  
4. decode 失败路径输出 machine-readable explain，便于审计与回归统计。

## 2. 失败路径样本

| 场景 | 结果 | 错误码 |
| --- | --- | --- |
| 缺少必填字段 | deny | `ai_plan_schema_constrained_decode_failed` |
| 字段类型不匹配 | deny | `ai_plan_schema_constrained_decode_failed` |
| 额外未声明字段 | deny | `ai_plan_schema_constrained_decode_failed` |

## 3. 门禁与验证记录

1. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
2. [X] `make check error-message` -> PASS  
3. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.2（步骤 6）、7（条目 19）与 9.2（条目 17）。  
- 满足 `DEV-PLAN-208`“非法结构 lint 前拦截”目标。
