# DEV-PLAN-075C：OrgUnit 删除记录/停用语义对齐与最小化落地方案

**状态**: 进行中（2026-02-07 修订版，P0/P1/P2 已完成）

## 背景
- 当前 OrgUnit 详情页存在“删除记录”入口，但实际执行是 `DISABLE`，与用户“删除错误数据”的预期不一致。
- 团队在 2026-02-06 的确认口径：
  1) 删除要支持“删除错误数据”；
  2) 不新增审批链；
  3) 不新增独立后台入口；
  4) 优先简单、避免额外认知负担；
  5) 第一阶段不新增表。

## 现状事实（SSOT）
1. `action=delete_record` 当前落到停用逻辑。
   - 证据：`internal/server/orgunit_nodes.go:1822`、`internal/server/orgunit_nodes.go:2147`。
2. OrgUnit 事件源是 append-only + One Door（`submit_org_event(...)`）。
   - 证据：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md:35`、`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md:111`。
3. `org_unit_versions.last_event_id` 引用 `org_events(id)`，直接物理删事件存在一致性风险。
   - 证据：`modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql:105`。
4. `replay_org_unit_versions(...)` 依赖事件流重建 versions/tree/code。
   - 证据：`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql:702`。

## 本轮决策（2026-02-06）

### 1) 语义收敛（对用户）
- `停用/启用`：继续是状态变更（`DISABLE/ENABLE`）。
- `删除记录`：定义为“删除错误数据”，但技术实现为**撤销目标事件（rescind）**。
- `删除组织`：定义为“删除错误建档”，技术实现为**批量撤销该 org_id 的事件**。

### 2) 内核实现约束（对系统）
- 不物理删除 `org_events` 行；保持 append-only，不破坏 One Door。
- 删除相关写入必须走 DB Kernel 函数，在同一事务完成“审计/撤销标记 + replay + 提交”。
- replay 失败整笔回滚，不能出现“已撤销但未重建”的半成品状态。

### 3) 审计与数据模型
- 第一阶段不新增表。
- 复用 `org_event_corrections_history/current` 记录撤销操作（使用 `replacement_payload` 的结构化字段标识撤销语义）。

## 设计方案（V1）

### A. 删除记录（Delete Record = Rescind Event）
- 定义：撤销指定 `tenant + org_id + effective_date` 的目标事件，使其不再参与重放。
- 前置条件：
  - 目标事件存在；
  - `request_id`、`reason` 必填；
  - 请求具备 OrgUnit 管理权限（沿用现有 Authz）。
- 执行步骤（同事务）：
  1. `assert_current_tenant` + 租户写锁；
  2. 定位目标事件并校验；
  3. 写审计/撤销标记（history + current）；
  4. 执行 `replay_org_unit_versions(...)`；
  5. 成功提交；失败回滚。

### B. 删除组织（Delete Org = Rescind Org Events）
- 定义：批量撤销某 `org_id` 的全部历史事件（错误建档清除）。
- V1 约束：
  - 根组织禁止删除：`ORG_ROOT_DELETE_FORBIDDEN`；
  - 若存在子组织则拒绝：`ORG_HAS_CHILDREN_CANNOT_DELETE`；
  - 若存在下游绑定依赖（如 SetID 绑定）则拒绝：`ORG_HAS_DEPENDENCIES_CANNOT_DELETE`；
  - `request_id`、`reason` 必填。
- 执行步骤（同事务）：
  1. `assert_current_tenant` + 租户写锁；
  2. 校验根组织/子组织/依赖；
  3. 逐事件写审计/撤销标记（同一批次 request_id 前缀 + 序号）；
  4. 执行 `replay_org_unit_versions(...)`；
  5. 成功提交；失败回滚。

## 事件解释规则（必须固定）
- `org_events_effective` 在 V1 中引入“撤销过滤”：被撤销事件不进入 replay 输入流。
- 若同一事件同时存在 correction 与 rescind：**rescind 优先**（即最终不参与重放）。
- 事务顺序固定：`校验 -> 审计/撤销 -> replay -> 提交`。

## 幂等与冲突规则
- 幂等键：`tenant_uuid + request_id`。
- 同 `request_id` 重放：
  - 请求语义完全一致：返回已存在结果（幂等成功）；
  - 请求语义不一致：返回 `ORG_REQUEST_ID_CONFLICT`。
- 已撤销事件再次撤销：返回幂等成功（不再新增业务影响）。

## 行为矩阵（UI 意图 -> 数据行为 -> 校验）
| UI 操作 | 语义 | 数据行为 | 关键校验 | 失败码（建议） |
| --- | --- | --- | --- | --- |
| 停用记录 | 状态变更 | 写 `DISABLE` 事件 | 一天一事件、日期边界、末状态约束 | `EVENT_DATE_CONFLICT` / `ORG_ENABLE_REQUIRED` |
| 启用记录 | 状态变更 | 写 `ENABLE` 事件 | 一天一事件、日期边界 | `EVENT_DATE_CONFLICT` |
| 删除记录 | 错误数据删除 | 写撤销标记 + replay | 目标存在、request_id/reason 必填、replay 成功 | `ORG_EVENT_NOT_FOUND` / `ORG_REQUEST_ID_CONFLICT` / `ORG_REPLAY_FAILED` |
| 删除组织 | 错误组织删除 | 批量写撤销标记 + replay | 非根组织、无子组织、无依赖、request_id/reason 必填、replay 成功 | `ORG_ROOT_DELETE_FORBIDDEN` / `ORG_HAS_CHILDREN_CANNOT_DELETE` / `ORG_HAS_DEPENDENCIES_CANNOT_DELETE` / `ORG_REPLAY_FAILED` |

## 路由与权限
- 继续挂在现有 OrgUnit API 命名空间，不新增独立入口体系。
- Authz 沿用现有 OrgUnit 管理权限；失败必须 fail-closed，统一 403 拒绝语义。
- 保持 routing 契约，不引入新命名空间漂移。

## UI 与文案要求（同入口，不增负担）
- “删除记录”改为：`删除记录（错误数据）`。
- 同一详情面板增加：`删除组织（错误建档）`。
- 删除确认文案统一：
  - `该操作将删除错误数据（通过事件撤销实现），并立即重放版本；操作可审计，不可撤销。`
- 与“停用”文案严格区分，避免继续混用。

## 分阶段实施步骤
1. [x] **P0（契约修订）**：冻结“删除=撤销（非物理删除）”口径，补齐错误码/幂等/优先级规则。
2. [x] **P1（内核：删除记录）**：新增 Kernel 函数 `submit_org_event_rescind(...)`，接入审计复用与 replay。
3. [x] **P2（内核：删除组织）**：新增批量撤销函数，补齐根/子组织/依赖校验。
4. [x] **P3（接口与页面）**：`delete_record` 对接新语义；同面板补 `delete_org` 动作与文案。
5. [x] **P4（测试与门禁）**：补齐删除成功、幂等复放、request_id 冲突、replay 失败回滚、403 契约回归。

## 验收标准
- “删除记录”不再触发 `DISABLE`，且不物理删除 `org_events`。
- 删除记录/删除组织均走 One Door 的 Kernel 写入口。
- 删除后必须 replay 成功才提交，失败时撤销不生效（整笔回滚）。
- 保持“无审批、无新后台、无新表（V1）”。
- 审计可追溯：操作者、请求号、目标事件快照、原因、操作时间齐全。

## 触发器与门禁证据（最小可审计块）
- 本次命中触发器（预期）：
  - Go 代码；
  - DB Schema/迁移（OrgUnit 模块）；
  - Routing/Authz；
  - 文档。
- 本地验证入口（实施 PR 必填）：
  - `go fmt ./... && go vet ./...`
  - `make check lint && make test`
  - `make check routing`
  - `make authz-pack && make authz-test && make authz-lint`
  - `make <module> plan && make <module> lint && make <module> migrate up`
  - `make check doc`

## 非目标（V1 不做）
- 不引入审批人流程。
- 不新增独立审计表（保留后续按性能瓶颈再评估）。
- 不引入 legacy 双链路或物理删事件兜底路径。

## 关联文档
- `AGENTS.md`
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- `docs/dev-plans/073-orgunit-crud-implementation-status.md`
- `docs/dev-plans/074-orgunit-details-update-ui-optimization.md`
- `docs/dev-plans/075a-orgunit-records-ui-and-editing-issues.md`
- `docs/dev-plans/076-orgunit-version-switch-selection-retention.md`

## P1/P2 任务拆解（可直接排期）

### P1：删除记录（Rescind Event）
| 任务 | 目标文件（建议） | 交付定义（DoD） |
| --- | --- | --- |
| DB Kernel 增加撤销函数 | `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` | 提供 `submit_org_event_rescind(...)`，同事务完成“审计/撤销标记 + replay”，并返回稳定错误码 |
| 事件有效视图支持撤销过滤 | `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` | `org_events_effective` 或 replay 输入流可识别 `op=RESCIND_EVENT` 并排除目标事件 |
| 写服务扩展删除记录能力 | `modules/orgunit/services/orgunit_write_service.go`、`modules/orgunit/domain/ports/orgunit_write_store.go` | `OrgUnitWriteService` 增加 `RescindRecord(...)`；校验 `request_id/reason` 必填；请求幂等 |
| Internal API 增加删除记录接口 | `internal/server/orgunit_api.go`、`internal/server/handler.go` | 新增 `POST /org/api/org-units/rescinds`（命名可最终定稿）；状态码与错误码映射稳定 |
| 页面动作改线 | `internal/server/orgunit_nodes.go` | `action=delete_record` 不再调用 disable；文案改为“删除记录（错误数据）”；错误提示与 API 错误码一致 |
| Authz + Routing 对齐 | `internal/server/authz_middleware_test.go`、路由 allowlist 相关文件 | 新接口纳入 allowlist 与授权规则；缺权限统一 403 |

### P2：删除组织（Rescind Org Events）
| 任务 | 目标文件（建议） | 交付定义（DoD） |
| --- | --- | --- |
| DB Kernel 增加批量撤销函数 | `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql` | 提供 `submit_org_rescind(...)`，批量写撤销标记后 replay；失败整体回滚 |
| 依赖校验（根/子组织/下游绑定） | `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`、`modules/orgunit/infrastructure/persistence/schema/00005_orgunit_setid_schema.sql` | 根组织拒绝、存在子组织拒绝、存在 SetID 绑定拒绝，返回稳定错误码 |
| 写服务扩展删除组织能力 | `modules/orgunit/services/orgunit_write_service.go` | `OrgUnitWriteService` 增加 `RescindOrg(...)`；支持批次 request_id 规范 |
| Internal API 增加删除组织接口 | `internal/server/orgunit_api.go`、`internal/server/handler.go` | 新增 `POST /org/api/org-units/rescinds/org`（命名可最终定稿）；错误映射可预测 |
| UI 同面板新增“删除组织”动作 | `internal/server/orgunit_nodes.go` | 不新增独立入口；确认弹窗包含“可审计、不可撤销、立即重放” |

## API 草案（评审用，最终以实现 PR 为准）

### 1) 删除记录（Rescind Event）
- `POST /org/api/org-units/rescinds`
- Request:
```json
{
  "org_code": "ROOT260205A",
  "effective_date": "2026-02-01",
  "request_id": "purge-rec-20260206-001",
  "reason": "误录入名称"
}
```
- Success `200`:
```json
{
  "org_code": "ROOT260205A",
  "effective_date": "2026-02-01",
  "operation": "RESCIND_EVENT",
  "request_id": "purge-rec-20260206-001"
}
```

### 2) 删除组织（Rescind Org Events）
- `POST /org/api/org-units/rescinds/org`
- Request:
```json
{
  "org_code": "ROOT260205A",
  "request_id": "purge-org-20260206-001",
  "reason": "错误建档"
}
```
- Success `200`:
```json
{
  "org_code": "ROOT260205A",
  "operation": "RESCIND_ORG",
  "request_id": "purge-org-20260206-001",
  "rescinded_events": 3
}
```

### 3) 错误码与状态码建议
| 错误码 | HTTP | 含义 |
| --- | --- | --- |
| `ORG_EVENT_NOT_FOUND` | 404 | 目标生效日记录不存在 |
| `ORG_ROOT_DELETE_FORBIDDEN` | 409 | 根组织不可删除 |
| `ORG_HAS_CHILDREN_CANNOT_DELETE` | 409 | 存在子组织，禁止删除 |
| `ORG_HAS_DEPENDENCIES_CANNOT_DELETE` | 409 | 存在下游绑定依赖，禁止删除 |
| `ORG_REQUEST_ID_CONFLICT` | 409 | 同 request_id 但语义不一致 |
| `ORG_REPLAY_FAILED` | 409 | 重放失败，事务已回滚 |
| `EFFECTIVE_DATE_INVALID` | 400 | 日期格式错误 |
| `reason_required`（或稳定 DB 码） | 400 | 缺少删除原因 |

> 响应契约要求：JSON/HTMX/HTML 三类请求均需保持一致的错误语义（特别是 403/409）。

## 测试用例清单（最小回归集）

| 编号 | 场景 | 输入 | 预期 |
| --- | --- | --- | --- |
| T1 | 删除记录成功 | 有效 `org_code + effective_date + request_id + reason` | 200；目标事件不再出现在 replay 结果 |
| T2 | 删除记录幂等重放 | 完全相同请求重复提交 | 200；结果一致；无额外业务副作用 |
| T3 | 删除记录 request_id 冲突 | 同 request_id，不同 target/reason | 409 + `ORG_REQUEST_ID_CONFLICT` |
| T4 | 删除记录目标不存在 | 不存在的 effective_date | 404 + `ORG_EVENT_NOT_FOUND` |
| T5 | 删除记录缺少 reason | reason 为空 | 400 |
| T6 | 删除记录触发 replay 失败 | 构造 replay 失败条件 | 409 + `ORG_REPLAY_FAILED`；事务回滚 |
| T7 | 删除组织成功 | 非根、无子组织、无依赖 | 200；该 org 事件全部被撤销 |
| T8 | 删除组织-根组织 | 目标为 root | 409 + `ORG_ROOT_DELETE_FORBIDDEN` |
| T9 | 删除组织-有子组织 | 目标存在子节点 | 409 + `ORG_HAS_CHILDREN_CANNOT_DELETE` |
| T10 | 删除组织-有下游依赖 | 存在 SetID 绑定等依赖 | 409 + `ORG_HAS_DEPENDENCIES_CANNOT_DELETE` |
| T11 | 权限拒绝 | 无 orgunit admin 权限 | 403（JSON/HTMX/HTML 语义一致） |
| T12 | UI 文案与行为一致 | 点击“删除记录（错误数据）” | 不再触发 disable；提示文案与行为一致 |

## 建议 PR 切分
1. PR-A（P1-DB）：Kernel 函数 + replay 输入调整 + SQL 回归。
2. PR-B（P1-Go）：write service + internal API + 节点页 `delete_record` 改线 + 单测。
3. PR-C（P2-DB/Go）：删除组织能力 + 依赖校验 + 页面动作 + 回归测试。
4. PR-D（收口）：错误码映射、文案收敛、JSON/HTMX/HTML 403/409 契约测试补齐。
