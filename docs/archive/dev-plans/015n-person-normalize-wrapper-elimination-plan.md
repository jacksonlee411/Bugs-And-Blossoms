# DEV-PLAN-015N：Person Normalize Wrapper 从生产代码移除（承接 DEV-PLAN-015B P1）

**状态**: 已完成（2026-04-09 10:58 CST）

## 背景

在 `DEV-PLAN-015K` 之后，`internal/server/person.go` 中仍保留一个极薄的转发函数：

1. [ ] `normalizePernr`

该函数当前只是在 `internal/server` 中把调用转发到 `modules/person/services.NormalizePernr`，本身并不承担独立的 server 规则职责。

## 目标与非目标

### 目标

1. [ ] 从生产代码移除 `internal/server/person.go` 中的 `normalizePernr` 包装。
2. [ ] 将生产调用点直接改为使用模块服务规则入口。
3. [ ] 保持现有 `internal/server` 测试与行为不变。

### 非目标

1. [ ] 本计划不修改 Person API 行为。
2. [ ] 本计划不调整 Person 模块侧 Normalize 规则实现。
3. [ ] 本计划不重组 Person 测试文件。

## 实施步骤

1. [X] 新建 `015N` 文档，冻结范围。
2. [X] 删除生产包装 `normalizePernr`。
3. [X] 更新生产调用点直接使用模块规则。
4. [X] 执行最小验证：`go test ./internal/server/...`、`make check lint`、`make check doc`（2026-04-09 10:58 CST，本地通过）

## 验收标准

1. [ ] `internal/server/person.go` 不再保留 `normalizePernr` 包装。
2. [ ] Person API 行为保持不变。
3. [ ] 相关测试与门禁通过。
