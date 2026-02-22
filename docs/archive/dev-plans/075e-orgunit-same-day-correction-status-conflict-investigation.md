# [Archived] DEV-PLAN-075E：OrgUnit 同日状态修正（生效日不变）方案

> 2026-02-18 补充：自 `DEV-PLAN-108` 起，状态更正不再要求独立 `CORRECT_STATUS` 主路径；
> 更正链路允许 `status + 其他字段` 同次提交（仍为单 `CORRECT_EVENT` 审计链，replay 视角解释为 `UPDATE`）。
> 因此本文件作为“108 前的专项纠错方案与实现记录”保留，但 UI/契约应以 108 为准。

**状态**: 已归档（2026-02-22，模块标准条款已并入 `DEV-PLAN-108`；本文仅保留 108 前专项实现记录）

> 实施与修复已合并：PR #307（功能落地）+ PR #308（补齐迁移闭环，修复 `submit_org_status_correction` 缺失）。

## 背景
- 用户在 OrgUnit 页面存在“状态误维护”场景：目标是把某天（例如 `2026-01-01`）的状态从错误值修正为正确值，且**生效日期不变**。
- 当前 075D 的“状态变更”路径是写 `ENABLE/DISABLE` 新事件；当同一组织同一天已存在事件时，会触发唯一约束冲突（`org_events_one_per_day_unique`）。
- 团队已确认：不应让用户通过“改日期”绕过错误数据，应提供“同日状态纠错”的显式能力。

## 调查结论（承接 073/075~075D）
1. `org_events` 存在硬约束：`(tenant_uuid, org_id, effective_date)` 同日唯一。
2. `rescind` 当前是“撤销标记 + replay”，不物理删除 `org_events` 行，不释放同日占位。
3. 因此“同事务先 rescind 再同日写 ENABLE/DISABLE 新事件”在当前模型下仍可能撞唯一约束。
4. correction 现有能力用于字段与生效日纠错，但 `status` 被明确排除在 correction 外（状态必须走显式事件）。

> 结论：
> - 直接采用“rescind + insert 同日事件”的方案不稳妥；
> - 推荐方案应为“**状态纠错（不新增同日事件）**”，在 One Door 下原子执行“校验 -> 写纠错审计 -> replay -> 提交”。

## 目标
1. 支持用户在页面进行“同日状态修正”（`effective_date` 不变）。
2. 保持“一天一事件”不变量，不引入同日多事件（不引入 `effseq`）。
3. 保持 One Door 与可审计：所有写入仍通过 DB Kernel，且有完整历史。
4. 错误语义稳定：避免向用户暴露 SQLSTATE/constraint name。

## 非目标
- 不放开“同日多事件”。
- 不改变 075C 的删除语义（`delete_record` 仍是 rescind，不等价状态修正）。
- 不修改 Valid Time 粒度（仍为 day）。

## 推荐方案（冻结）

### 1) 语义定义
新增“状态纠错（correct_status）”语义：
- 输入：`org_code + target_effective_date + target_status(active|disabled) + request_id`。
- 约束：仅允许修正“目标日对应的状态事件语义”；`target_effective_date` 不变。
- 行为：不新增 `org_events` 新行；通过纠错叠加改变该目标事件在 replay 中的“有效状态语义”。

### 2) DB Kernel（One Door）
冻结函数名：
- `orgunit.submit_org_status_correction(...)`

事务内步骤：
1. `assert_current_tenant` + advisory lock。
2. 定位目标记录（按 `org_id + target_effective_date`，以 effective 视图定位实际目标）。
3. 校验目标事件类型仅允许 `ENABLE/DISABLE`，否则 fail-closed。
4. 校验请求幂等（`request_id`）。
5. 写入 `org_event_corrections_history/current`（审计保留，current 覆盖生效语义）。
6. `replay_org_unit_versions(...)`。
7. 成功提交；失败回滚。

### 3) replay 语义调整（冻结实现口径）
- 为 replay 增加“状态纠错投影”解释规则：
  - 当 `replacement_payload.op = 'CORRECT_STATUS'` 时，effective 视图把该事件投影为 `ENABLE` 或 `DISABLE`（由 `replacement_payload.target_status` 决定）。
  - replay 仍按 `event_type` 分支执行，不新增 replay 第二逻辑链。
  - `replacement_payload` 至少包含：`op`、`target_status`、`target_event_uuid`、`target_effective_date`。
- 不改变 `org_events` 主表“同日唯一”约束。

### 4) 服务/API/UI 契约
- Service：新增 `CorrectStatus(...)`。
- Internal API：冻结路径 `POST /org/api/org-units/status-corrections`。
- UI：在“状态变更”区新增“修正同日状态”入口；当检测到同日冲突且目标为同日修正场景时，引导走该入口。
- 错误映射：统一返回稳定码（例如 `EVENT_DATE_CONFLICT`、`ORG_EVENT_NOT_FOUND`、`ORG_REQUEST_ID_CONFLICT`），不透出 SQLSTATE 文本。

## 关键冻结决策（本轮评审结论）
1. **事件语义边界冻结**：status correction 仅允许目标事件类型为 `ENABLE/DISABLE`；不允许把 `CREATE/MOVE/RENAME/SET_BUSINESS_UNIT` 纠错为状态语义。
2. **幂等冲突码冻结**：同 `request_id` 不同语义统一返回 `ORG_REQUEST_ID_CONFLICT`；同语义重复提交返回幂等成功。
3. **已撤销事件优先级冻结**：目标事件若已是 rescind 态，status correction fail-closed，返回 `ORG_EVENT_RESCINDED`。
4. **请求编号规范冻结**：UI 端 request_id 前缀统一 `ui:orgunit:status-correction:`，避免与 correction/rescind 复用导致冲突。

## API 契约（冻结）

### Request
`POST /org/api/org-units/status-corrections`

```json
{
  "org_code": "A001",
  "effective_date": "2026-01-01",
  "target_status": "active",
  "request_id": "ui:orgunit:status-correction:20260208:001"
}
```

字段约束：
- `org_code`：必填，沿用现有 OrgCode 校验规则。
- `effective_date`：必填，`YYYY-MM-DD`。
- `target_status`：必填，枚举 `active|disabled`。
- `request_id`：必填，建议由 UI 端自动生成并可重放。

### Success Response（200）

```json
{
  "org_code": "A001",
  "effective_date": "2026-01-01",
  "target_status": "active",
  "request_id": "ui:orgunit:status-correction:20260208:001",
  "operation": "CORRECT_STATUS"
}
```

### Error Matrix（冻结）
| code | HTTP | 说明 |
| --- | --- | --- |
| `EFFECTIVE_DATE_INVALID` | 400 | 日期非法 |
| `ORG_CODE_INVALID` | 400 | 组织编码非法 |
| `ORG_CODE_NOT_FOUND` | 404 | 组织不存在 |
| `ORG_EVENT_NOT_FOUND` | 404 | 目标日事件不存在 |
| `ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET` | 409 | 目标事件不是 `ENABLE/DISABLE` |
| `ORG_EVENT_RESCINDED` | 409 | 目标事件已被撤销 |
| `EVENT_DATE_CONFLICT` | 409 | 普通 `change_status` 同日冲突（旧路径） |
| `ORG_REQUEST_ID_CONFLICT` | 409 | 同 `request_id` 不同语义 |
| `ORG_REPLAY_FAILED` | 409 | replay 失败，事务回滚 |
| `forbidden` | 403 | Authz 拒绝（JSON/页面 语义一致） |

## 分阶段拆分（评审版）

### P0：契约冻结与评审准备
**目标**：先冻结边界，避免实现期漂移。  
**范围**：文档/契约，不改业务代码。  
**交付**：
1. [x] 冻结 `status correction` 请求/响应契约（字段、错误码、幂等语义）。
2. [x] 冻结 Authz 口径（与 `change_status` 保持同级权限）。
3. [x] 冻结“旧路径冲突时的引导策略”（继续报冲突 vs UI 提供跳转纠错入口）。
4. [x] 输出评审决策记录（函数名、API 路径、冲突码统一策略）。
**DoD**：
- 评审会上对“是否新增同日事件”达成一致：明确**不新增**；
- 错误码矩阵可直接用于测试用例编写；
- 事件类型边界（仅 `ENABLE/DISABLE`）与已撤销优先级达成一致。

### P1：Kernel + Service + API（能力落地）
**目标**：先打通后端原子能力，确保可测试。  
**范围**：DB Kernel、Go Service、Internal API。  
**交付**：
1. [x] DB：新增 `submit_org_status_correction(...)` 及 effective-view 投影规则。
2. [x] DB：迁移与 schema 同步，保持 Atlas/Goose 闭环。
3. [x] Service：新增 `CorrectStatus(...)`，含参数校验与幂等透传。
4. [x] API：新增 `POST /org/api/org-units/status-corrections`，错误码映射稳定。
5. [x] API：将 `23505 + org_events_one_per_day_unique` 映射为业务冲突语义（`EVENT_DATE_CONFLICT`）。
**DoD**：
- 单测覆盖成功路径、目标不存在、幂等冲突、权限拒绝、已撤销拒绝、非状态事件拒绝；
- API 层不再向上透出 SQLSTATE 原文；
- 对外冲突码统一为 `ORG_REQUEST_ID_CONFLICT`；
- `REQUEST_DUPLICATE` 仅作为内部兼容映射保留在 P1~P2 过渡窗口，P2 收口时删除（删除条件：外部调用与前端文案全部切换完成，回归通过）。

### P2：UI 接入 + 回归收口
**目标**：让用户可发现、可操作，并与 075D 协同。  
**范围**：OrgUnit 页面交互、提示文案、回归测试与执行记录。  
**交付**：
1. [x] UI：新增“修正同日状态”入口（与“状态变更”并列但语义区分）。
2. [x] UI：状态冲突时给出可操作提示（引导用户走同日修正入口）。
3. [x] UI：提交结果/失败提示与 API 稳定错误码一致。
4. [x] 测试：补齐 UI/Handler/E2E 回归（含 JSON/页面 的 403/409 一致性）。
5. [x] 记录：在 `docs/dev-records/` 写入门禁与关键场景证据。
**DoD**：
- 用户无需改日期即可完成状态纠错；
- UI 文案与行为无歧义（“状态变更” vs “状态纠错”）；
- 已撤销/非状态目标场景提示可操作；
- 兼容映射 `REQUEST_DUPLICATE -> ORG_REQUEST_ID_CONFLICT` 已删除。

## 回滚与迁移策略（冻结）
1. DB 变更以单迁移提交，若 P1 验证失败：执行对应 `goose down` 回滚该迁移，恢复旧的 correction 解释规则。
2. API 层保持 fail-closed：若新函数不可用，`status-corrections` 返回 `ORG_REPLAY_FAILED` 或内部错误，不回退到 legacy 路径。
3. UI 不走双链路：新入口失败时只提示稳定错误码，不自动改走“改日期”或“新增事件”兜底。
4. 回滚验收：回滚后 `change_status` 路径行为保持 075D 既有契约，且不引入新数据污染。

## 评审准备清单
1. [x] 命名最终确认：沿用冻结命名（`submit_org_status_correction` / `status-corrections`）。
2. [x] 错误码最终确认：统一使用 `ORG_REQUEST_ID_CONFLICT`，并设置兼容映射退出条件。
3. [x] 交互最终确认：同日冲突后不自动弹出向导；显示可操作提示并提供“一键进入状态纠错入口”。
4. [x] 可观测性确认：日志字段最小集（tenant/org_code/effective_date/request_id/target_status）。
5. [x] 测试边界确认：根组织、disabled 可见性、并发提交、重复提交、已撤销事件。

## 评审决策记录（2026-02-08）
- 命名冻结：`orgunit.submit_org_status_correction(...)` + `POST /org/api/org-units/status-corrections`。
- 交互冻结：`change_status` 返回 `EVENT_DATE_CONFLICT` 时，UI 不自动弹窗；展示引导文案并允许用户主动进入“同日状态纠错”。
- 错误码冻结：外部冲突码统一 `ORG_REQUEST_ID_CONFLICT`；`REQUEST_DUPLICATE` 仅内部兼容到 P2 收口。
- 可观测冻结：最小日志字段固定为 `tenant/org_code/effective_date/request_id/target_status`。
- 边界冻结：status correction 仅面向 `ENABLE/DISABLE` 目标事件；已 rescind 目标返回 `ORG_EVENT_RESCINDED`。

## 与现有方案关系
- 与 075D 并不冲突：
  - 075D 的 `change_status` 继续用于“新增状态事件”（常规前进式状态流转）；
  - 075E 新增“同日纠错”用于“错误维护修正”。
- 与 075C 不冲突：
  - 075C 的 rescind 仍用于“删除错误数据”；
  - 075E 是“保留该日版本、仅纠正其状态语义”；
  - 对已 rescind 事件，075E fail-closed（保持 rescind 优先）。

## 最小验收标准
- 用户可在不改变 `effective_date` 的前提下，修正目标日状态。
- 同日唯一约束保持有效，且同日状态纠错不再依赖“新增事件”。
- replay 后快照状态与纠错目标一致。
- 错误提示使用稳定业务语义，不出现 SQLSTATE 原文。
- 已撤销事件与非状态事件目标场景返回稳定业务码。

## 回归测试清单（最小）
- T1：目标日状态错误 -> 提交 status correction -> 200，状态修正成功。
- T2：重复相同 `request_id` 与相同语义 -> 幂等成功。
- T3：重复 `request_id` 但语义不同 -> `ORG_REQUEST_ID_CONFLICT`。
- T4：目标日不存在 -> `ORG_EVENT_NOT_FOUND`。
- T5：普通 `change_status` 同日冲突 -> 返回稳定 `EVENT_DATE_CONFLICT`（不透 SQLSTATE）。
- T6：403 权限拒绝在 JSON/页面 两类入口语义一致。
- T7：目标事件不是 `ENABLE/DISABLE` -> `ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET`。
- T8：目标事件已 rescind -> `ORG_EVENT_RESCINDED`。

## 触发器与门禁（实现阶段）
- DB Schema/迁移（orgunit 模块）：
  - `make orgunit plan && make orgunit lint && make orgunit migrate up`
- Go/Router/Authz/UI 变更：
  - `go fmt ./... && go vet ./...`
  - `make check lint && make test`
  - `make check routing`
  - `make authz-pack && make authz-test && make authz-lint`
- 文档收敛：
  - `make check doc`

## 证据记录要求（对齐 DEV-PLAN-003）
- 在 PR 或执行记录中必须明确：
  1. 本次命中的触发器；
  2. 实际执行命令与结果；
  3. 错误码矩阵回归结果（特别是 403/409 的 JSON/页面 一致性）；
  4. 兼容映射退出（若仍保留映射，说明原因与退场日期）。

## 关联文档
- `docs/dev-plans/073-orgunit-crud-implementation-status.md`
- `docs/dev-plans/075-orgunit-effective-date-backdating-assessment.md`
- `docs/archive/dev-plans/075a-orgunit-records-ui-and-editing-issues.md`
- `docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- `docs/archive/dev-plans/075d-orgunit-status-field-active-inactive-selector.md`
- `docs/dev-records/dev-plan-075d-execution-log.md`
