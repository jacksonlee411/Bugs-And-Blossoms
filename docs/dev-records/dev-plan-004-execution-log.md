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
