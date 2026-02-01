# DEV-PLAN-026A 执行日志

> 目的：记录 DEV-PLAN-026A 的落地变更与可复现验证入口。

## 变更摘要

- OrgUnit 标识规范落地：`org_id` 8 位 `int4`、`tenant_uuid`/`event_uuid`/`request_code` 命名统一
- 事件幂等键改为 UUID v7，新增 `uuidv7` 生成器
- `full_name_path` 预计算并落地，读侧不再祖先链 JOIN
- Go/SQL/迁移与测试全量对齐，覆盖新增校验与错误分支

## 本地验证

- 已通过：`go fmt ./...`
- 已通过：`go vet ./...`
- 已通过：`make check lint`
- 已通过：`make test`
- 已通过：`make check doc`
