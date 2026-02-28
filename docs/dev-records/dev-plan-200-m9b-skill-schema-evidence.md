# DEV-PLAN-200 M9B 证据：Skill I/O 契约与 schema 严格校验

**对应计划**: `DEV-PLAN-209`  
**记录时间**: 2026-02-28 19:45 UTC

## 1. 契约冻结结论

1. Skill 执行必须声明并校验 `input_schema/output_schema`，违约即 fail-closed。  
2. 仅 `status=published` 且版本未废弃的 Skill 可进入执行链路。  
3. `SkillExecutionResult` 必须固化 `input_hash/output_hash/output_schema_valid`。  
4. 未注册或废弃版本分别返回 `skill_not_registered/skill_version_deprecated`。

## 2. 失败路径矩阵

| 场景 | 结果 | 错误码 |
| --- | --- | --- |
| 未注册 Skill | deny | `skill_not_registered` |
| 使用废弃版本 | deny | `skill_version_deprecated` |
| 输入 schema 违约 | deny | `skill_input_schema_invalid` |
| 输出 schema 违约 | deny | `skill_output_schema_violation` |

## 3. 门禁与验证记录

1. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
2. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.6（条目 3/6/8）与 9.2（条目 28/29）。  
- 与 `DEV-PLAN-209` 验收标准一致：schema 违约不进入提交链路。
