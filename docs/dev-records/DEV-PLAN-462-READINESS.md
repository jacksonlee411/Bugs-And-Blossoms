# DEV-PLAN-462 Readiness

## 状态

- `462`：已完成
- 完成日期：`2026-04-23`
- 关联 owner：`434`、`460`、`461`

## 结论

`CubeBox` 已形成 `434` 会话级上下文压缩、`460` 数字助手权限边界、`461` 查询回答级统一收敛、`462` 上位协同方案的连续叙事。`462` 所要求的“统一预算、统一降级、统一挂载点、canonical context reinjection 方法论分层”已在文档与实现层闭环；其中查询回答预算现已明确落实为“统一行数预算 + 整次 answer 字符预算 + 禁止原始 payload 直出”。

## 完成证据

### 1. 会话级压缩 owner 已完成

- `DEV-PLAN-434` 已封板，承接 prompt view replacement、recent user messages 保留、summary prefix、canonical context reinjection、manual/pre-turn compact 与 `turn.context_compacted` 最小事件语义。

### 2. 数字助手定位与权限边界已冻结

- `DEV-PLAN-460` 已明确并封板：
  - `CubeBox` 不是独立授权主体
  - 查询/操作完全继承当前用户、当前租户、当前 session
  - 文档不是授权来源
  - 正式业务写入必须显式确认并回到现有 One Door

### 3. 查询回答已收敛到统一入口

- 通用回答总线统一走：
  - [internal/server/cubebox_query_flow.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go)
- 统一预算与收敛规则已落地：
  - `limitQueryAnswerLines(...)`
  - `limitQueryAnswerChars(...)`
  - `cubeboxQueryAnswerMaxChars`
  - `cubeboxQueryAnswerMaxLines`
  - `cubeboxQueryListSummaryMaxItems`
  - `cubeboxQuerySummaryFallbackListNotice`
  - `cubeboxQuerySummaryFallbackOmitted`
- 通用回退不再直出原始 `result.Payload` JSON；当 `SummaryRenderer` 缺失且 `result_focus` 未命中时，会先回退到稳定元信息摘要，再回到受控提示语句：
  - [internal/server/cubebox_query_flow.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go)
- 能力专属摘要唯一挂载点已落地：
  - [modules/cubebox/read_executor.go](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/read_executor.go)
  - 通过 `RegisteredExecutor.SummaryRenderer`
- `orgunit.list` 专属摘要已从回答总线移到能力侧挂载：
  - [internal/server/cubebox_orgunit_executors.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_orgunit_executors.go)
- 无专属摘要器时会自动回退到通用 `result_focus` 摘要：
  - [internal/server/cubebox_query_flow_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow_test.go)
- 大 payload、超长 renderer 输出、多 step 累积超预算的统一收敛测试已补齐：
  - [internal/server/cubebox_query_flow_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow_test.go)
  - [internal/server/cubebox_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_api_test.go)

### 4. 失败语义已对齐 `461`

- `knowledge_pack_invalid`：
  - [modules/cubebox/knowledge_pack.go](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/knowledge_pack.go)
  - [modules/cubebox/knowledge_pack_test.go](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/knowledge_pack_test.go)
- `api_catalog_drift_or_executor_missing`：
  - [modules/cubebox/read_executor.go](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/read_executor.go)
  - [modules/cubebox/read_executor_test.go](/home/lee/Projects/Bugs-And-Blossoms/modules/cubebox/read_executor_test.go)
  - [internal/server/cubebox_query_flow.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_query_flow.go)
  - [internal/server/cubebox_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/cubebox_api_test.go)
- responder 用户提示已存在：
  - [internal/routing/responder.go](/home/lee/Projects/Bugs-And-Blossoms/internal/routing/responder.go)

### 5. `462` 要求的后移项没有提前变成首期前置

仓内没有把以下能力写成 `462` 首期封板前置：

- remote compaction
- model downshift compact
- 独立 summary model
- 完整 telemetry 数据面
- memory pipeline
- compacted prompt 全量 UI 可视化

## 本次封板范围

- 封板 `462` 上位方案文档
- 收口 `461` Step 6：
  - 统一摘要预算
  - 能力侧摘要挂载点
  - 通用回退路径
  - `api_catalog_drift_or_executor_missing` 运行时错误码
  - 回归测试
- 同步把 `460` 状态收口为已完成，作为 `462` 依赖闭环的一部分

## 未纳入本次范围

- remote compaction
- model downshift compact
- 独立 summary model
- memory pipeline
- 完整 telemetry 数据面
- compacted prompt UI 全量可视化

以上继续后移，由 `434` 或后续 owner 计划承接，不属于 `462` 首期封板前提。

## 验证

已执行：

```bash
go test ./modules/cubebox/... ./internal/server/... ./internal/routing/...
```
