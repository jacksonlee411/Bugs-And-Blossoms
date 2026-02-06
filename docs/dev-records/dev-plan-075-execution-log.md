# DEV-PLAN-075 执行日志

> 目的：记录 DEV-PLAN-075 的落地变更与可复现验证入口。

## 变更摘要

- 更正规则补齐上级组织有效性校验：更正生效日时要求父级在该日有效，且不早于父级最早生效日。
- `/org/nodes` 记录动作补齐体验：insert/add 默认日期与范围提示，最早记录允许回溯时使用更正生效日路径。
- 新增 OrgUnit 生效日更正写入口（store）与覆盖性单测，补齐回溯/异常分支。

## 本地验证

- 已通过：`go fmt ./...`
- 已通过：`go vet ./...`
- 已通过：`make check lint`
- 已通过：`make test`（coverage 100%）
- 已通过：`make check doc`
- 已通过：`make orgunit plan`
- 已通过：`make orgunit lint`
- 已通过：`make orgunit migrate up`

## CI 证据

- PR #TBD
