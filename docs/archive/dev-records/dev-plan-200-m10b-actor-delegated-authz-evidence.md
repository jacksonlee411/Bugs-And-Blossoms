# DEV-PLAN-200 M10B 证据：Actor-Delegated 授权一致性

**对应计划**: `DEV-PLAN-210`  
**记录时间**: 2026-02-28 20:20 UTC

## 1. 授权边界冻结结论

1. AI 不作为独立授权主体，所有请求绑定 `actor_id + tenant_id + role_set`。  
2. 提交前必须执行实时 re-auth，快照过期/角色漂移即拒绝。  
3. 授权顺序冻结：`Actor Bind -> MapRouteToObjectAction -> Require -> Pre-Commit Re-Auth -> One Door`。  
4. 授权拒绝统一 403 合同，日志记录诊断字段。

## 2. 角色回归样本

| 角色 | AI 代操 | 人工直操 | 结果 |
| --- | --- | --- | --- |
| 系统配置管理员 | 同输入 | 同输入 | 一致 |
| HR 专业用户 | 同输入 | 同输入 | 一致 |
| 普通员工 | 同输入 | 同输入 | 一致 |
| 经理 | 同输入 | 同输入 | 一致 |

## 3. 门禁与验证记录

1. [X] `make authz-pack && make authz-test && make authz-lint` -> PASS  
2. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
3. [X] `make check no-legacy` -> PASS  
4. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 2.3、6.2（步骤 11/13）、9.2（条目 22/23/27）。  
- 满足 `DEV-PLAN-210`“提交瞬时 re-auth + 授权顺序冻结”目标。
