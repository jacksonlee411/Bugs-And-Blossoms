# DEV-PLAN-073 执行日志

> 目的：记录 DEV-PLAN-073 的落地变更与可复现验证入口。

## 变更摘要

- PR #271：新增 `/org/nodes/children`、`/org/nodes/details`、`/org/nodes/search` 读服务与路由，补齐 allowlist/Authz 与单测覆盖
- PR #272：Shoelace Tree 资源接入与 `/org/nodes` UI 对接（树/详情/搜索事件桥接），Astro Shell 与样式更新，build-astro 产物同步至 `internal/server/assets/`
- PR #273：新增 DEV-PLAN-073 执行日志，并补齐 AGENTS Doc Map 链接
- PR #275：扩展 OrgUnit Internal API 与独立详情页的契约/里程碑定义
- PR #276：实现 OrgUnit Internal API（`/org/api/org-units` CRUD），补齐路由 allowlist/Authz 映射与服务层/路由层覆盖率

## 本地验证

- 已通过：`go fmt ./...`
- 已通过：`go vet ./...`
- 已通过：`make test`
- 已通过：`make check routing`
- 已通过：`make generate`
- 已通过：`make css`
- 已通过：`make check doc`
- 未通过：`make check lint`（go-cleanarch 拉取 GitHub 失败，本地网络问题；CI 已通过）

## CI 证据

- PR #271：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/271
- PR #272：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/272
- PR #273：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/273
- PR #275：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/275
- PR #276：https://github.com/jacksonlee411/Bugs-And-Blossoms/pull/276
