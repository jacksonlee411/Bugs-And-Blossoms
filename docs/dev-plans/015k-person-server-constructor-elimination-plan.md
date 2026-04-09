# DEV-PLAN-015K：Person Server 侧冗余构造包装消除（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 10:42 CST）

## 背景

在 `DEV-PLAN-015E/015F` 完成后：

1. [ ] `internal/server/handler.go` 已直接使用 `modules/person` 的模块入口。
2. [ ] `internal/server/person.go` 中残留的 `newPersonPGStore` / `newPersonMemoryStore` 已不再承担生产装配职责。

与 `015J` 的 JobCatalog 情况相同，这两个函数继续留在生产代码中会放大 `internal/server` 仍是模块装配入口的错觉。

## 目标与非目标

### 目标

1. [ ] 从生产代码移除 `internal/server/person.go` 中的冗余构造包装。
2. [ ] 将测试兼容入口移动到测试侧 helper。
3. [ ] 继续收缩 `internal/server` 在 Person 模块上的装配表面积。

### 非目标

1. [ ] 本计划不改动 Person API handler。
2. [ ] 本计划不修改 Person 模块侧 store 实现。
3. [ ] 本计划不整理 Person 测试命名与组织方式。

## 实施步骤

1. [X] 新建 `015K` 文档，冻结范围。
2. [X] 删除 `internal/server/person.go` 中的生产构造包装。
3. [X] 在测试 helper 中保留兼容构造入口。
4. [X] 执行最小验证：`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 10:42 CST，本地通过）

## 验收标准

1. [ ] 生产代码中的 `internal/server/person.go` 不再承载 Person store 构造包装。
2. [ ] `internal/server` 现有 Person 测试仍能通过。
3. [ ] 相关 lint 与文档门禁通过。
