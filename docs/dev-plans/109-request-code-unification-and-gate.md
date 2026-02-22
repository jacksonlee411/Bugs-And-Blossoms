# DEV-PLAN-109：Org 模块幂等命名收敛与门禁（历史阶段封板，按 STD-001 修订）

**状态**: 已完成（2026-02-22 02:32 UTC — 第一阶段封板，现行口径由 DEV-PLAN-005/109A 接管）

## 1. 背景

`DEV-PLAN-109` 首次落地时（2026-02-18）将 Org 写入链路收敛到 `request_code`，并上线了对应门禁。

2026-02-22 起，`DEV-PLAN-005` 的 `STD-001` 冻结了新的仓库级标准：

1. 业务幂等统一 `request_id`（参考 Google AIP-155）；
2. Tracing 统一 `trace_id`（参考 W3C Trace Context / OpenTelemetry）。

因此，本计划中的“`request_code` 作为业务幂等唯一命名”已不再是现行规范。

## 2. 本文在修订后的定位

1. 保留 109 第一阶段执行事实与可复用资产（门禁结构、执行证据）。
2. 明确哪些条款已经失效，防止后续继续引用旧口径。
3. 将后续“反向收敛与防扩散”实施统一承接到 `DEV-PLAN-109A`。

## 3. 已失效条款（仅作历史记录）

以下条款自 `DEV-PLAN-005` 生效后视为失效，不再作为实现依据：

1. 业务幂等统一为 `request_code`；
2. tracing 沿用 `request_id` / `X-Request-ID`；
3. `check-request-code` 仅以“拦截 `request_id` 业务语义”为目标。

## 4. 保留资产（可复用）

1. 门禁接入结构（`make check request-code` + CI job）可复用，但规则语义需按 109A 改造。
2. “先增量阻断，再全量阻断”的门禁演进路径可复用。
3. 执行证据仍可追溯：`docs/dev-records/dev-plan-109-execution-log.md`。

## 5. 当前有效口径（引用 STD-001）

| 语义场景 | 当前有效命名 | 说明 |
| --- | --- | --- |
| 业务幂等键（请求体/服务入参/DB 幂等语义） | `request_id` | 业务唯一命名 |
| Tracing 上下文 | `trace_id` | 追踪唯一命名 |
| 旧命名 `request_code` | 禁止新增 | 历史资产按 109A 迁移 |

## 6. 与 109A 的承接关系

1. 109 作为“历史阶段封板文档”，不再新增实施步骤。
2. 109A 作为“现行实施文档”，负责从 `request_code/request_id(Tracing)` 收敛到 `request_id/trace_id`。
3. 109A 完成后，109 保留为历史事实与经验沉淀。

## 7. 验收标准（修订后）

1. 109 的历史定位与失效条款已明示，不再误导后续实现。
2. 现行规范入口统一指向 `DEV-PLAN-005` 与 `DEV-PLAN-109A`。
3. 109 的历史执行记录保持可追溯。

## 8. 关联文档

- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/109a-request-code-total-convergence-and-anti-drift.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-records/dev-plan-109-execution-log.md`
- `AGENTS.md`
