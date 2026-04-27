# DEV-PLAN-477：CubeBox `api_key -> executor_key` 契约改名专项方案

**状态**: 已完成（2026-04-27 23:27 CST；查询链内部执行键、`source_executor_key` 回放口径、测试与活体文档已收口）

## 0. 范围一句话

完成 `CubeBox` 查询链内部执行键从 `api_key` / `source_api_key` 到 `executor_key` / `source_executor_key` 的一次性收口，移除运行时 legacy alias，统一测试与活体文档口径，并明确真实 provider secret 面不在本专项。

## 1. 背景

1. 查询链历史上把内部执行注册表键叫作 `api_key`，容易与真实 provider 密钥混淆。
2. 2026-04-27 之前，主运行时、知识包和 planner prompt 已基本切到 `executor_key`，但仍残留 `source_api_key` replay alias、测试文案和活体计划漂移。
3. 本专项的目标不是重命名真实凭据，而是关闭查询链内部 contract 的最后尾巴，避免双字段长期并存。

## 2. 已完成范围

1. [x] 运行时 contract 收口：
   - `ReadPlan`、执行注册表、knowledge pack、working results、planner/narrator prompt 已以 `executor_key` 为唯一正式字段。
   - `QueryEntity` decoder 已删除 `source_api_key -> source_executor_key` 的 replay alias；当前 canonical replay 只接受 `source_executor_key`。
2. [x] 测试与错误文案收口：
   - `modules/cubebox/query_entity_test.go` 删除 legacy alias 回放断言。
   - `modules/cubebox/read_executor_test.go`、`internal/server/cubebox_orgunit_executors_test.go` 统一改成 `executor_key` 文案。
   - narrator prompt 测试继续固定“内部字段不得外泄”，同时覆盖历史 `api_key` 与当前 `executor_key`。
3. [x] 活体文档回写：
   - `DEV-PLAN-468` 当前正式 contract 统一改写为 `executor_key`。
   - `DEV-PLAN-468-READINESS` 中涉及当前字段的描述同步为 `source_executor_key` / `executor_key`。
   - `AGENTS.md` 文档地图已标注本专项完成。
4. [x] 历史/安全边界保留：
   - `internal/server/cubebox_query_flow.go` 的泄露防线继续同时拦截 `api_key` 与 `executor_key`，用于阻断历史字段名、内部字段名和提示注入，不表示恢复双字段 contract。
   - `docs/dev-records/assets/dev-plan-468/*.json` 保留历史样本中的 `source_api_key`，仅作证据归档，不再代表当前契约。

## 3. 明确不改面

1. [x] `modules/cubebox/gateway.go`、`modules/cubebox/health.go`、`internal/server/cubebox_query_flow.go` 中 `ProviderChatRequest.APIKey`
2. [x] 环境变量 `CUBEBOX_OPENAI_API_KEY`
3. [x] runtime/config/API DTO 中 `secret_ref`
4. [x] 任何真实外部 provider secret / credential 命名

## 4. 验收口径

1. [x] 当前运行时与测试不再依赖 `source_api_key` replay alias。
2. [x] 活体计划不再把 `api_key` 当成当前正式执行字段；若仍提到 `api_key`，仅用于历史问题说明或泄露防线说明。
3. [x] 用户可见回答仍不得泄露 `executor_key`；泄露防线对历史 `api_key` 也继续 fail-closed。
4. [x] provider secret 命名未受本专项影响。

## 5. 验证命令

本次收口按下列命令验证：

```bash
gofmt -w modules/cubebox/query_entity.go \
  modules/cubebox/query_entity_test.go \
  modules/cubebox/read_executor_test.go \
  internal/server/cubebox_orgunit_executors_test.go \
  internal/server/cubebox_query_flow_test.go
go test ./modules/cubebox ./internal/server
make check doc
rg -n "source_api_key|api_key|source_executor_key|executor_key" \
  modules/cubebox \
  internal/server \
  docs/dev-plans/468-cubebox-session-continuity-and-model-autonomy-improvement-plan.md \
  docs/dev-plans/477-cubebox-api-key-to-executor-key-contract-rename-plan.md \
  docs/dev-records/DEV-PLAN-468-READINESS.md
```

说明：

1. `rg` 扫描允许命中真实 provider secret 面、narrator 泄露防线和历史证据文件，但不允许再把 `api_key` / `source_api_key` 作为当前查询链正式 contract。
2. 如后续重新引入内部执行键别名，必须新开计划评审，不得在当前主线上静默恢复兼容窗口。

## 6. 当前状态

1. [x] `DEV-PLAN-477` owner 已关闭。
2. [x] 查询链内部正式字段统一为 `executor_key` / `source_executor_key`。
3. [x] 剩余与 CubeBox 查询链相关的主线不再属于 477，而转回 `DEV-PLAN-468` 的 `P2-2` per-api 授权补强等后续 owner。
