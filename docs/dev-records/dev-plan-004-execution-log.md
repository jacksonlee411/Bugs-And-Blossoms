# DEV-PLAN-004 记录：全仓去除版本标记——执行日志

**状态**: 进行中（2026-01-06 09:25 UTC）

> 重要：为满足“最终全仓扫描命中为 0”的验收口径，本文避免直接写出版本标记字面量；统一用占位符 `<ver>` 表示“`v`/`V` + `4` 的紧邻组合”。

## PR-1：盘点与映射表草案

- **状态**：已完成（2026-01-06 09:25 UTC）
- **交付物**
  - 映射表草案：`docs/dev-records/dev-plan-004-version-marker-removal-mapping.md`
  - 计划文档进度更新：`docs/dev-plans/004-remove-version-marker-repo-wide.md`
  - Doc Map 增补入口：`AGENTS.md`
- **本地门禁**
  - `make check doc`：通过

## PR-2：重命名 dev-plan 文件名并修复全仓引用

- **状态**：已完成（2026-01-06 09:49 UTC）
- **范围**
  - `docs/dev-plans/**`：移除文件名中的版本标记（`git mv`）
  - 全仓：修复对旧路径的引用（`AGENTS.md` 与相关 dev-plan 文档）
- **本地门禁**
  - `make check doc`：通过

## PR-3：路由与 UI 文案清理

- **状态**：已完成（2026-01-06 10:01 UTC）
- **范围**
  - `/org/nodes`：`read` 模式从旧值切换为 `current`（默认值、链接、重定向、错误文案与测试一并更新）
  - OrgUnit Snapshot：路径从 `/org/.../snapshot` 切换为 `/org/snapshot`，并同步更新导航与 allowlist
  - Readiness：更新浏览器复现步骤中的 URL
- **本地门禁**
  - `go fmt ./...`：通过
  - `go vet ./...`：通过
  - `make check lint`：通过
  - `make test`：通过
  - `make check routing`：通过
  - `make check doc`：通过

## PR-4：锁 key 前缀去噪（原子切换）

- **状态**：已完成（2026-01-06 10:09 UTC）
- **范围**
  - OrgUnit：写入互斥锁 key 前缀去除版本标记，并同步更新 schema SSOT、迁移文件与文档示例
  - 工具链：更新迁移校验和（`atlas.sum`）并重新导出 sqlc schema
- **本地门禁**
  - `make sqlc-generate`：通过
  - `./scripts/db/run_atlas.sh migrate hash --dir file://migrations/orgunit --dir-format goose`：通过
  - `./scripts/db/lint.sh orgunit`：通过
  - `make orgunit plan`：通过
  - `make orgunit migrate up`：通过（已在本地环境运行）
