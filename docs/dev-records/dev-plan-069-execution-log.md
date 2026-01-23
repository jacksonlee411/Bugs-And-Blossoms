# DEV-PLAN-069 执行日志

> 目的：记录 DEV-PLAN-069 的落地变更与可复现验证入口。

## 变更摘要

- 清理 069 范围内文档与记录，保留计划入口与 Doc Map 索引
- 清理 069 范围的 server/worker/lib/DB/测试，并补齐单测覆盖
- staffing/person 迁移基线补充扩展依赖（pgcrypto/btree_gist）并更新 atlas.sum
- staffing 内核补齐 Job Profile SetID 校验（跨 SetID 引用 fail-closed）
- tp060-03 幂等冲突用例改为“不同 payload”触发
- 集成测试隔离到独立测试库，避免污染主库函数

## 本地验证

- 已通过：`go fmt ./...`
- 已通过：`go vet ./...`
- 已通过：`make check lint`
- 已通过：`make test`
- 已通过：`make check doc`
- 已通过：`make check routing`
- 已通过：`make authz-pack`
- 已通过：`make authz-test`
- 已通过：`make authz-lint`
- 已通过：`make sqlc-generate`
- 已通过：`make staffing plan`
- 已通过：`make staffing lint`
- 已通过：`make staffing migrate up`
- 已通过：`make person plan`
- 已通过：`make person lint`
- 已通过：`make person migrate up`
- 已通过：`make e2e`
- 已通过：`make preflight`
