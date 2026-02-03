# DEV-PLAN-026D 执行日志

> 目的：记录 DEV-PLAN-026D 的实施与验证结果。

## 变更摘要

- submit_org_event 改为增量投射，按事件类型分发 apply_* 并局部重算 full_name_path。
- 新增 orgunit.rebuild_full_name_path_subtree 与 orgunit.assert_org_unit_validity，用于局部路径与有效期校验。
- replay 入口权限收敛，仅 orgunit_kernel 可执行。
- 新增增量写入 vs 全量 replay 一致性集成测试。

## 本地验证

- 已通过（2026-02-03 08:58 UTC）：
  - `GOCACHE=/tmp/go-build-cache go fmt ./...`
  - `GOCACHE=/tmp/go-build-cache go vet ./...`
  - `GOCACHE=/tmp/go-build-cache GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache make check lint`
  - `GOCACHE=/tmp/go-build-cache make test`
  - `make check doc`
  - `make sqlc-generate`
  - `./scripts/db/run_atlas.sh migrate hash --dir file://migrations/orgunit --dir-format goose`
  - `make orgunit plan`
  - `make orgunit lint`
  - `make orgunit migrate up`

## 迁移与生成物

- 新增迁移：`migrations/orgunit/20260203113000_orgunit_incremental_projection.sql`
- atlas.sum 已更新。

## 备注

- 若后续发现全量 replay 与增量写入差异，可先用 replay 运维修复入口校准。
