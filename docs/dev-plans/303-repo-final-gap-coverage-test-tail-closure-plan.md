# DEV-PLAN-303：全仓残留 `gap/coverage` 测试尾项清零计划

**状态**: 已完成（2026-04-08 CST）

执行补记（2026-04-08 CST）：

1. [X] 在 `DEV-PLAN-302` 完成后，对全仓再次盘点残留 `*_gap_test.go` / `*_coverage_test.go` 文件。
2. [X] 盘点结果确认：仓库仅剩 `internal/routing/error_catalog_coverage_test.go` 这 `1` 个尾项。
3. [X] 该文件已改为正式测试入口 `internal/routing/error_catalog_test.go`。
4. [X] 当前全仓已无残留 `*_gap_test.go` / `*_coverage_test.go` 文件。
5. [X] 验证已通过：
   - `rg --files . | rg '(gap|coverage)_test\\.go$'`
   - `go test ./internal/routing -count=1`
   - `make check doc`

## 背景

`DEV-PLAN-301` 与 `DEV-PLAN-302` 已分别完成首轮 Go 测试分层治理与 `internal/server` 残留收口。  
在 `302` 收尾后，对仓库进行全量复盘时确认，仍有 `1` 个非 `internal/server` 的遗留 `coverage` 命名测试文件未被清理：

1. [X] `internal/routing/error_catalog_coverage_test.go`

该文件本身已经承担正式校验职责，问题只在于命名仍保留“补洞式”历史痕迹，因此适合用最小变更直接收口。

## 目标

1. [X] 清零全仓最后一个 `gap/coverage` 命名测试文件。
2. [X] 将该测试保留为正式长期入口，而不是删除其校验能力。
3. [X] 让仓库级盘点结果与 `301/302/303` 文档状态一致。

## 非目标

1. [X] 不重开 `301` 或 `302` 范围。
2. [X] 不调整测试逻辑、覆盖率口径或门禁阈值。
3. [X] 不把本次尾项收口扩展成新的大规模测试重构。

## 实施

### 尾项收口

1. [X] `internal/routing/error_catalog_coverage_test.go` → `internal/routing/error_catalog_test.go`

执行说明：

1. [X] 仅做正式命名收口，测试内容与行为保持不变。
2. [X] `internal/routing` 目录下当前已不存在新的 `gap/coverage` 替代文件。

## 验收标准

1. [X] 全仓 `rg --files . | rg '(gap|coverage)_test\\.go$'` 结果为空。
2. [X] `internal/routing` 定向测试通过。
3. [X] 文档入口已补齐，仓库状态与文档结论一致。

