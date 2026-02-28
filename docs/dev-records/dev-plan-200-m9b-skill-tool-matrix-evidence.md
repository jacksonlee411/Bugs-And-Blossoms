# DEV-PLAN-200 M9B 证据：Skill 工具白名单与风险分级矩阵

**对应计划**: `DEV-PLAN-209`  
**记录时间**: 2026-02-28 19:45 UTC

## 1. 矩阵冻结结论

1. Skill 仅允许调用 manifest 中声明的 `allowed_tools`。  
2. 工具权限与 `risk_tier` 绑定：高风险必须 dry-run + 人工确认 + re-auth。  
3. 未声明工具调用必须拒绝：`skill_tool_not_allowed`。  
4. `SkillValidationReport` 发布前必须包含 tool whitelist 校验结论。

## 2. 风险分级策略（冻结）

| risk_tier | dry-run | 人工确认 | 提交前 re-auth |
| --- | --- | --- | --- |
| low | 必须 | 可选 | 必须 |
| medium | 必须 | 必须 | 必须 |
| high | 必须 | 必须 | 必须（且授权新鲜度校验） |

## 3. 门禁与验证记录

1. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
2. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.6（条目 4/7）、7（条目 20/21/22）与 9.2（条目 30）。  
- 满足 `DEV-PLAN-209` 验收标准：高风险未注册 Skill 拒绝、工具权限不漂移。
