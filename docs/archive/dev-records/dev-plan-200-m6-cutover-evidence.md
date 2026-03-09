# DEV-PLAN-200 M6 证据：No Legacy 单次切换与旧路径下线

**对应计划**: `DEV-PLAN-206`  
**记录时间**: 2026-02-28 18:25 UTC

## 1. 切换结论

1. create/add/insert/correct 提交统一走单一模板链路，无平行写入口。  
2. 提交前置固定为：映射决议 -> ResolveSetID -> 组合版本校验 -> One Door 提交。  
3. 切换后不保留 runtime legacy 回退分支；失败处置仅允许环境级保护（只读/停写/修复重试）。  
4. 版本冲突与上下文漂移均通过稳定错误码 fail-closed。

## 2. 切换剧本（冻结）

| 阶段 | 验证点 |
| --- | --- |
| 只读对照 | 新旧结果一致，禁止双写 |
| 预发验收 | 版本冲突/幂等/上下文漂移回归通过 |
| 正式切换 | 单次切换，不启用 fallback |
| 切换后收口 | 删除旧入口并由门禁阻断回流 |

## 3. 门禁与验证记录

1. [X] `make check no-legacy` -> PASS  
2. [X] `make test` -> PASS（覆盖率门禁输出：`OK: total 100.00% >= threshold 100.00%`）  
3. [X] `make check error-message` -> PASS  
4. [X] `make check doc` -> PASS

## 4. 覆盖关系

- 覆盖 `DEV-PLAN-200` 第 6.1（步骤 8）、9.1（No legacy 行）、9.2（条目 12/14）与 11（R4/R5）。  
- 满足 `DEV-PLAN-206` 验收标准：四类 intent 单链路提交、冲突稳定可复现、无 runtime 双链路。
