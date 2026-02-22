# DEV-PLAN-109A：`request_id`（幂等）+ `trace_id`（Tracing）全仓收敛与防扩散

**状态**: 已完成（2026-02-22 09:35 UTC）

## 1. 背景与问题定义

`DEV-PLAN-005` 的 `STD-001` 已冻结仓库标准：

1. 业务幂等统一 `request_id`；
2. Tracing 统一 `trace_id`（传播遵循 W3C Trace Context，语义对齐 OpenTelemetry）。

但当前仓库仍是旧口径残留（`request_code` + `request_id` tracing 混用），形成“契约/实现/门禁”三层分叉，存在持续扩散风险。

### 1.1 基线快照（2026-02-22，工作区扫描）

1. `request_code`：约 1961 处匹配，约 162 个文件。
2. `request_id|RequestID|X-Request-ID`：约 861 处匹配，约 67 个文件。
3. `trace_id`：0 处匹配（基本未落地）。
4. 现有门禁 `check-request-code` 仍以“禁止 `request_id` 业务语义”为目标，与 `STD-001` 方向相反。

> 结论：若不做反向收敛，`STD-001` 无法成为可执行标准。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. [x] 业务幂等统一为 `request_id`（契约文档 + API + 服务 + schema 源 + 门禁）。
2. [x] Tracing 统一为 `trace_id`（错误 envelope、前端错误解析、日志字段、传播策略）。
3. [x] 阻断新增 `request_code` 业务语义与 `request_id` tracing 语义回潮。
4. [x] 历史资产可追溯迁移：不破坏历史迁移文件，但确保“新增不扩散”。
5. [x] 在 `docs/dev-records/` 固化执行证据与例外审批。

### 2.2 非目标（Stopline）

1. 不改动业务领域规则（仅命名与可观测性语义收敛）。
2. 不对已执行的历史 migration 文件做重写。
3. 不引入长期双字段兼容窗口（No Legacy）。

## 3. 统一命名词典（STD-001 落地版）

| 语义场景 | 唯一命名 | 说明 |
| --- | --- | --- |
| 业务幂等键（请求体/服务入参/DB 幂等语义） | `request_id` | 业务重试去重唯一键 |
| Tracing 语义字段（日志/错误 envelope/诊断） | `trace_id` | 追踪关联唯一键 |
| HTTP 追踪传播 | `traceparent`（W3C） | 系统内提取并暴露 `trace_id` |
| 旧业务命名 | `request_code` | 禁止新增；历史资产分阶段迁移 |
| 旧 tracing 命名 | `request_id` / `X-Request-ID` | 禁止继续作为 tracing 语义 |

## 4. 根因分析（为何 109 后仍不一致）

1. 109 的目标是 `request_id -> request_code`，与 `STD-001` 新标准方向相反。
2. 门禁规则与脚本命名绑定在旧语义上，只能防“`request_id` 回流”，不能防“`request_code` 扩散”。
3. tracing 未引入 `trace_id` 统一词典，`request_id` 被复用为 tracing 字段。
4. 070/071/071A 等契约文档与实现层曾出现双轨，缺少一次性反向收口计划。

## 5. 方案设计

### 5.1 契约层收口（先文档后代码）

1. 修订 070/071/071A：业务幂等字段统一为 `request_id`。
2. 修订 109（历史封板）与 109A（现行执行）的承接关系。
3. 统一文档中 tracing 表述：从 `request_id` 改为 `trace_id`，并注明 `traceparent` 传播。

### 5.2 API / 服务层收口（应用语义）

1. 请求/响应 JSON 字段：`request_code` -> `request_id`。
2. Go/TS 字段命名：`RequestCode/request_code` -> `RequestID/request_id`。
3. 校验文案：`request_code is required` -> `request_id is required`。
4. 历史错误码常量（如 `ORG_REQUEST_ID_CONFLICT`）可保留；语义解释统一到“业务幂等 request_id 冲突”。

### 5.3 Tracing 收口（可观测性语义）

1. 通用错误 envelope：`request_id` -> `trace_id`（`internal/routing/responder.go`）。
2. 前端错误适配：解析字段由 `request_id` 改为 `trace_id`（`apps/web/src/api/errors.ts`）。
3. HTTP 客户端默认追踪传播从 `X-Request-ID` 迁移到 `traceparent`（必要时由 SDK 生成 trace id 并组装）。
4. 日志字段统一使用 `trace_id`，禁止新增 tracing 场景 `request_id`。

### 5.4 DB / Schema 收口（语义一致）

1. `modules/**/schema/*.sql` 中函数参数/局部变量统一为 `p_request_id` / `v_request_id*`（业务幂等语义）。
2. 新增迁移将活跃表的业务幂等列从 `request_code` 迁移为 `request_id`（含唯一约束命名同步）。
3. `sqlc` 生成模型与查询参数同步收敛为 `request_id`。
4. 历史迁移文件保持不可变，只允许新增迁移做前向收敛。

### 5.5 门禁升级（Gate-C，反向重定义）

在现有 `scripts/ci/check-request-code.sh` 上升级（脚本路径可保留，规则语义必须反向）：

1. 业务实现路径阻断新增 `request_code` / `RequestCode` 业务命名。
2. tracing 场景阻断新增 `request_id` / `X-Request-ID`。
3. 放行白名单仅限历史资产与构建产物（必须可审计、只能减少不能增加）。
4. 增量 + 全量两种模式都要给出文件与行号。
5. 与 `make check request-code`、CI Quality Gates 对齐（后续可改名，但不得改变阻断能力）。

## 6. 实施步骤（Checklist）

1. [x] M1：完成 109/109A/070/071/071A 契约修订并评审通过。
2. [x] M2：服务端 API/服务层完成 `request_code -> request_id` 改造与测试对齐。
3. [x] M3：Tracing 链路完成 `request_id -> trace_id` 改造（envelope + 前端适配 + header 传播）。
4. [x] M4：Schema/SQL/sqlc 完成业务幂等语义收敛（前向迁移，不改历史迁移）。
5. [x] M5：Gate-C 升级完成并开启 blocking。
6. [x] M6：执行并登记证据：
   - `make check request-code`
   - `make check doc`
   - 涉及 Go 变更时：`go fmt ./... && go vet ./... && make check lint && make test`
7. [x] M7：新增执行日志：`docs/dev-records/dev-plan-109a-execution-log.md`。

## 7. 验收标准（DoD）

1. [x] 新增业务写接口不再出现 `request_code`。
2. [x] 新增 tracing 字段不再出现 `request_id` / `X-Request-ID`。
3. [x] 业务幂等统一 `request_id`，Tracing 统一 `trace_id`，并在门禁可阻断。
4. [x] 070/071/071A 与实现口径一致，不再双轨。
5. [x] 109A 证据记录可追溯（命令、结果、白名单审批）。

## 8. 风险与缓解

- **影响面广（接口+DB+前端）**：按 M1→M7 分段推进，每段必须可回归验证。
- **门禁误杀**：先 dry-run 校准白名单，再切 blocking。
- **追踪传播改造风险**：先在测试环境验证 `traceparent -> trace_id` 解析稳定性。
- **历史命名残留误导**：在文档与错误语义中统一解释，避免“字段名与语义反转”。

## 9. 关联文档

- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/109-request-code-unification-and-gate.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `AGENTS.md`
