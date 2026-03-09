# DEV-PLAN-200 M10 证据：会话事务状态机与幂等提交

**对应计划**: `DEV-PLAN-210`  
**记录时间**: 2026-02-28 20:20 UTC

## 1. 状态机冻结结论

1. 状态流转固定：`draft -> proposed -> validated -> confirmed -> committed`。  
2. 仅 `confirmed` 可提交；`canceled/expired` 为终态，不可隐式恢复。  
3. 同 `conversation_id + turn_id + request_id` 仅允许幂等重试，不生成隐式新版本。  
4. 版本漂移（policy/composition/mapping）时回退 `validated` 并强制重确认。

## 2. 验证矩阵（冻结）

| 场景 | 预期 | 错误码 |
| --- | --- | --- |
| validated 直接提交 | deny | `conversation_confirmation_required` |
| canceled 后继续提交 | deny | `conversation_state_invalid` |
| 同 request_id 重试 | 幂等，无重复提交 | - |

## 3. 门禁与验证记录

1. [X] `make authz-pack && make authz-test && make authz-lint` -> PASS  
2. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
3. [X] `make check no-legacy` -> PASS  
4. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.3、9.2（条目 17/18）与 10（M10）。  
- 满足 `DEV-PLAN-210` 验收标准：未 confirmed 禁止提交、状态机可审计。
