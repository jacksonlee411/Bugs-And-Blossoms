# DEV-PLAN-105：全模块字典配置模块（DICT 值配置 + 生效日期 + 变更记录）

**状态**: 已完成（2026-02-17）

> 本计划承接 `DEV-PLAN-100/100D/100D2/100E/101`，把各模块的 DICT 字段从“代码内静态字典”收敛为一个**平台级**“可配置字典值模块”，并提供用户可见的管理闭环（UI+API+存储）。

## 1. 背景

当前各模块 DICT 值来源仍包含代码内静态注册（例如 Org 的 `fieldmeta.dictOptionsRegistry`），导致：

1. 业务要求“编码变更/新增”时需要改代码与发版，不符合配置化目标。
2. 详情页在 label 缺失/值不一致时容易出现 `unresolved` 警告，排障成本高。
3. 缺少“字典值的生效日与变更时间”用户可见入口，不利于审计与追溯。

本计划冻结一个新模块：**全模块字典配置模块**（Dictionary Config Module），作为“所有模块 DICT 字段”的统一事实源（SSOT）。  
为降低风险，Phase 0 先以 `org_type` 作为首个可见样板闭环；后续模块的 DICT 字段按同一机制接入。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. 新增“字典配置”平台模块（UI+API+存储），支持 **dict_code -> values** 的配置与审计，用于服务全部模块的 DICT 字段。
2. 首个落地 dict_code：`org_type`，默认值冻结为：
   - `10`：部门
   - `20`：单位
3. UI 形态对齐 Org 模块：**左侧列表 + 右侧列表 + 详情**（两栏/分区布局）；详情支持：
   - 基本信息
   - 生效日期（Valid Time/day 粒度）
   - 变更记录（含变更时间 `tx_time`）
4. `fields:options` 与写入 label 快照统一改为查询此字典模块（替代静态 registry），避免漂移。
5. 保持 One Door / No Tx, No RLS / fail-closed / No Legacy。
6. 使用**独立权限模型**（不复用 `orgunit.admin`）：
   - `dict.read`：查看字典与值列表（只读）
   - `dict.admin`：新增/停用/更正字典值（写）

### 2.2 非目标（Stopline）

1. 不在本计划引入业务数据多语言（仍遵循 `DEV-PLAN-020`）。
2. 不引入 legacy 双链路（不保留“旧字典注册 + 新模块”并行长期运行）。
3. 不改动 Org 主体事件模型（仍走 `submit_*_event` 写入口）。
4. 不扩展 ENTITY 字段管理能力（仅 DICT）。

## 3. 需求初衷与“简单性”冻结点（对齐 DEV-PLAN-003）

> 需求初衷（必须始终成立）：
> 1) DICT 值可以配置，不依赖发版；2) 写入时严格校验（fail-closed）并写入 label 快照，避免运行期漂移；
> 3) 字典值本身支持 Valid Time（日粒度）与可追溯审计（tx_time）；4) 对外行为可验证，门禁可阻断漂移。

本计划的“简单性冻结点”（进入实现前必须明确）：
1. **字典模块自身也必须 One Door**：字典值的写入同样走 `submit_*_event(...)`（事件 SoT + 同事务同步投射），而不是“controller 直接写表”。
2. **Valid Time 与 Audit/Tx Time 分离**：有效期用 date（日粒度），审计用 timestamp；查询口径必须能用一句话解释清楚。
3. **不引入双事实源**：运行期不允许“静态 registry + 字典模块”长期并行；对存量缺快照的展示兜底若存在，必须时间盒并有退出条件。

## 4. 100 系列对齐原则（冻结）

1. **One Door**：
   - 各业务模块写入仍走各自 Kernel 的 `submit_*_event(...)`（不绕过 Kernel）。
   - 字典模块自身写入走 iam 模块的 `submit_*_event(...)`（事件 SoT + 同事务同步投射）。
   - 其他模块对字典模块只做“读（校验/解析）”，不形成第二写入口。
2. **No Tx, No RLS**：字典模块数据访问必须显式事务 + 注入 `app.current_tenant`（缺失即 fail-closed）。
3. **Valid Time（日粒度）**：字典值有效期使用 `enabled_on/disabled_on`（date），审计时间使用 `tx_time`（timestamp）。
4. **Fail-Closed**：
   - DICT 值不存在/未生效/已停用 => 拒绝写入（稳定错误码）。
   - options 请求字段非 DICT/配置非法 => 拒绝。
5. **No Legacy**：完成切换后，运行态不再依赖代码内静态 dict registry；不保留长期双链路。

> 纠偏说明：本计划将统一采用“有效期窗口”术语，避免 `effective_date/enabled_on` 混用。
> - `enabled_on`：生效日（date，含当天）
> - `disabled_on`：失效日（date，不含当天）；为空表示无结束
> - `as_of`：查询视图日期（date）；口径为 `enabled_on <= as_of < disabled_on(or +inf)`

## 5. 模块 IA 与路由（UI 冻结）

### 5.1 导航与路由

- 新增导航：`字典配置`
- 路由：
  - 列表页：`/app/dicts`（平台能力，不归属单一业务模块）
  - 值详情页：`/app/dicts/:dictCode/values/:code`
- 权限：`dict.admin`（独立权限；无权限 fail-closed）

> 路由治理要求（对齐 `DEV-PLAN-017`）：
> - `/app/dicts` 必须登记到 `config/routing/allowlist.yaml`，`route_class=ui`
> - `/app/dicts/{dict_code}/values/{code}` 必须登记到 `config/routing/allowlist.yaml`，`route_class=ui`
> - `/iam/api/dicts*` 必须登记到 allowlist，`route_class=internal_api`
> - 新增/变更路由必须跑 `make check routing`

### 5.2 页面布局（与 Org 模块对齐）

1. **分屏 1：`/app/dicts`（字典总览）**
   - 左侧字典列表（Dict List）：
     - 数据来源：字典模块内的 `dict_codes`（或等价概念）；展示 dict_code + 名称（名称仅为“字典本身的展示名”，可用 i18n key；不引入业务数据多语言）。
     - 支持按“模块/关键字”过滤（便于规模扩展时定位）。
     - Phase 0 最小集：仅 `org_type`。
   - 右侧值列表（Value Grid）：
     - 列：`code`、`label`、`status`、`enabled_on`、`disabled_on`、`updated_at`（updated_at 为审计/排障辅助）。
     - 支持按 `as_of` 过滤。
     - 点击行进入分屏 2（值详情页）。
2. **分屏 2：`/app/dicts/:dictCode/values/:code`（值详情）**
   - Tabs：`基本信息` / `变更日志`。
   - 基本信息：左侧生效日期时间轴（enabled_on/disabled_on），右侧展示当前版本详情与操作。
   - 变更日志：左侧修改时间时间轴（tx_time），右侧展示事件详情（request_code/initiator/event_type）。

## 6. 数据与领域模型（冻结）

> 说明：以下为目标模型。若进入实现涉及新增表/迁移，按仓库红线需先获用户手工确认后再落库。

### 6.1 核心实体（概念冻结）

- `dict_code`：字典代码（例如 `org_type`），跨模块稳定枚举键（SSOT）。
  - 仅描述“这是哪个字典”，不承载业务数据多语言。
- `dict_value_segment`（投射/查询面向）：字典值“有效期窗口”记录（Valid Time）。
  - 关键字段：`dict_code`、`code`、`label`、`enabled_on`、`disabled_on`、`status`
  - 负责支撑 `as_of` 查询与 options 列表。
- `dict_value_event`（事件 SoT / 审计面向）：字典值变更事件（Audit/Tx Time）。
  - 事件类型（冻结）：`DICT_VALUE_CREATED`、`DICT_VALUE_LABEL_CORRECTED`、`DICT_VALUE_DISABLED`、`DICT_VALUE_REENABLED`、`DICT_VALUE_RESCINDED`
  - 关键字段：`request_code`、`tx_time`、`initiator`、`before_snapshot/after_snapshot`（快照字段命名以实现阶段收敛，但语义必须可审计）

### 6.2 状态机与失败模式（冻结）

1. 有效性判定（`as_of`）：
   - **有效**：`enabled_on <= as_of` 且（`disabled_on` 为空或 `as_of < disabled_on`）
   - **无效**：不满足上式
2. 写入 fail-closed：
   - dict_code 不存在 => 拒绝（稳定错误码）
   - code 不存在或在 `as_of`（通常由业务实体写入的 `effective_date` 提供）下无有效窗口 => 拒绝
   - code 被停用（落入无效窗口）=> 拒绝
3. 幂等与冲突：
   - 同一 `(tenant, request_code)` 幂等
   - 同一 request_code 不允许复用不同 payload（冲突拒绝）

### 6.3 关键约束（冻结）

1. code 基本约束：
   - code 不允许空白
   - Phase 0：`org_type` 的 code 冻结为两位数字字符串（`10`,`20`），不允许其他值进入“有效窗口”
2. Valid Time 约束：
   - 生效区间半开：`[enabled_on, disabled_on)`
   - 同一 `(tenant_uuid, dict_code, code)` 的有效期窗口 **不得重叠**
3. 唯一性（投射面向）：
   - 建议唯一键冻结为 `(tenant_uuid, dict_code, code, enabled_on)`（用于允许同一 code 的多段有效期窗口）
4. 事件幂等（事件 SoT）：
   - 同一 `(tenant_uuid, request_code)` 幂等；不同 payload 复用同 request_code 必须冲突拒绝

> 备注：是否采用 exclusion constraint / daterange 等 DB 级强约束属于实现细节，但“不可重叠”必须能被 DB 强制，而不是仅靠应用层判断。

## 7. API 合约（草案冻结）

1. `GET /iam/api/dicts?as_of=...`
   - 返回 dict_code 列表（Phase 0：仅 org_type）；`as_of` 必填（`YYYY-MM-DD`）。
2. `GET /iam/api/dicts/values?dict_code=org_type&as_of=...&status=...`
   - 返回选中字典的值列表。
3. `POST /iam/api/dicts/values`
   - 创建/启用字典值（含 `enabled_on`、`request_code`）。
4. `POST /iam/api/dicts/values:disable`
   - 停用字典值（含 `disabled_on`、`request_code`）；停用语义为设置窗口结束日（半开区间）。
5. `POST /iam/api/dicts/values:correct`
   - 更正 label（MVP 冻结为“同日更正 label”，不改变 enabled_on/disabled_on；跨日变更有效期需另起子计划或在 Phase A 显式补齐能力矩阵与约束）。
6. `GET /iam/api/dicts/values/audit?dict_code=...&code=...`
   - 返回变更记录（含 `tx_time`）。

### 7.1 查询参数与检索能力（Phase A 必补齐）

为对齐现有 Org 的 `fields:options` 使用方式（支持 `q/limit/as_of`），本计划冻结：字典值列表查询必须支持 **keyword + limit**，否则 UI 将被迫把“全量字典”拉回前端过滤。

- `GET /iam/api/dicts`
  - Query:
    - `as_of`（必填，`YYYY-MM-DD`）
  - 说明：Phase 0 仅返回 `org_type`；后续扩展按接入模块逐步追加。
- `GET /iam/api/dicts/values`
  - Query（冻结）：
    - `dict_code`（必填）
    - `as_of`（必填，`YYYY-MM-DD`）
    - `q`（可选，keyword；对 `code/label` 做 contains 匹配；大小写不敏感）
    - `limit`（可选，默认 10；最大 50）
    - `status`（可选；MVP 先支持 `active|inactive|all`，或显式说明不支持）
- `GET /iam/api/dicts/values/audit`
  - Query（冻结）：
    - `dict_code`（必填）
    - `code`（必填）
    - `limit`（可选，默认 50；最大 200）

> 冻结结论：`GET /iam/api/dicts` 的 `as_of` **必须提供**；不提供视为 `invalid_request`，避免隐式“今天”带来的时区与版本歧义。

### 7.2 稳定错误码（Phase A 必补齐）

为保持 fail-closed 且可排障，本计划要求字典模块与接入方在关键失败路径上具备**稳定错误码**。Phase A 需冻结最小集合（建议）：

- `invalid_as_of`：`as_of` 格式非法
- `dict_code_required`：缺少 `dict_code`
- `dict_not_found`：dict_code 不存在（Phase 0 仅 org_type）
- `dict_value_code_required`：缺少/空白 code
- `dict_value_label_required`：缺少/空白 label（写接口）
- `dict_value_not_found_as_of`：指定 as_of 下无有效窗口
- `dict_value_conflict`：窗口冲突/幂等冲突（同 request_code 不同 payload）
- `forbidden`：权限不足（dict.read / dict.admin）

权限口径（冻结）：
- 列表/详情读取接口：`dict.read`（或 `dict.admin` 继承可读）。
- 写接口（create/disable/correct）：`dict.admin`。
- UI 页面访问：`dict.admin`（Phase 0 先聚焦管理页；若后续新增只读页再放开 `dict.read`）。

> 各模块的 `fields:options` 在 DICT 分支改为查询此模块，不再直接使用代码静态 registry。

## 8. 写入与读取链路改造（冻结）

### 8.1 写入链路（各业务模块）

各模块写入（create/rename/move/disable/enable/correct 等）中 DICT 字段处理改为：

1. 按业务实体写入目标 `effective_date`（date）作为 `as_of`，查询字典模块中的有效值（tenant + dict_code + code）。
2. 不存在/无效则拒绝（例如：`DICT_VALUE_NOT_FOUND_AS_OF`；错误码以实现阶段收敛为准，但必须稳定且可解释）。
3. 存在则写入 `payload.ext` 的 code，并写 `payload.ext_labels_snapshot` 的 canonical label（label 快照是对外可解释的“写入时事实”，用于避免运行期漂移）。

### 8.2 读取链路（详情展示/排障）

详情页 display 值优先级保持 100D 口径（冻结）：

`versions 快照 -> events 快照 -> （可选）字典模块诊断查询`

补充规则：
- 当 value 为 `null` 时不展示“已显示原始值”类 warning，避免误导。
- “字典模块诊断查询”仅用于减少存量数据的 `unresolved`，必须满足：
  1) UI 明确标识 `source=dict_diagnostic`（不等同于快照事实）；2) 有时间盒与退出条件；
  3) 退出方式为：通过 One Door 更正事件回填 label 快照或数据修复，使运行期不再依赖诊断查询。

### 8.3 集成方式（模块接入，冻结）

为避免跨模块 import 与“到处拼 dict 查库逻辑”，冻结以下接入方式：
1. 业务模块侧只依赖 `pkg/**` 提供的“DICT 解析/校验门面”（例如 `pkg/dict`），不直接 import `modules/iam/**`。
2. 门面仅暴露必要能力：
   - `ResolveValueLabel(as_of, dict_code, code) -> label/err`
   - `ListOptions(as_of, dict_code, q, limit) -> []{code,label,status,...}`
3. `pkg/dict` 的实现由 iam 模块提供，并在服务启动时注册；缺失实现视为配置不可用（fail-fast），禁止静默降级到静态 registry（对齐 No Legacy）。

## 9. 权限模型（Authz 合同冻结）

对齐 `DEV-PLAN-022`（object/action 集中注册、action 仅 `read|admin|debug` 的约束），本计划冻结：
1. permissionKey：
   - `dict.read`
   - `dict.admin`
2. Casbin object/action（MVP 冻结）：
   - object：`iam.dicts`
   - action：`read`（list/values/audit 等读取）
   - action：`admin`（create/disable/correct 等写入）
3. 约束：
   - 禁止在模块内手写 object/action 字符串；必须走 `pkg/authz` 的集中 registry/helper。
   - 无权限一律 403（internal_api）/ NoAccessPage（ui），fail-closed。

### 9.1 permissionKey 与角色映射（Phase A 必补齐）

本仓库当前 UI 权限判断依赖 `permissionKey`（例如 `orgunit.read/orgunit.admin`），而服务端 Casbin 使用 object/action（例如 `orgunit.orgunits/read|admin`）。因此 Phase A 必须冻结：

1. `dict.read`/`dict.admin` 在 UI 侧按本计划命名执行（冻结）。
2. `iam.dicts` object 常量注册到 `pkg/authz/registry.go`（冻结）。
3. 角色策略（冻结）：
   - `role:tenant-admin`：`iam.dicts read + admin`
   - `role:tenant-viewer`：`iam.dicts read`

## 10. `org_type` 编码收敛策略（10/20）

### 10.1 冻结值

- `10`：部门
- `20`：单位

### 10.2 存量数据处理策略（冻结）

冻结结论：**不存在需要自动映射的旧值迁移问题**，不执行“旧 code -> 新 code 自动映射”。

执行口径：
1. 运行态只接受并返回 `10/20`。
2. 若发现非 `10/20` 的异常数据，按 fail-closed 进入人工处理清单；禁止自动猜测映射。
3. 通过 One Door 更正事件进行修复，确保 `ext_labels_snapshot` 与 code 一致。

## 11. 验收标准（DoD）

1. [x] `/app/dicts` 可见、可访问、可操作（`dict.admin`）。
2. [x] 分屏 1（左字典列表/右值列表）与分屏 2（基本信息+生效日期+变更记录）完整可用。
3. [x] `org_type` 默认值 `10/20` 可配置并生效。
4. [x] 各模块写入时 DICT 校验来源可切换为模块化字典（至少先完成 Org 样板），静态 registry 不再作为运行态事实源。
5. [x] details/options 无“静态字典漂移”导致的错误展示。
6. [x] 门禁证据齐全（对齐 `AGENTS.md` 触发器矩阵）：
   - 路由：`make check routing`
   - 权限：`make authz-pack && make authz-test && make authz-lint`
   - Go：`go fmt ./... && go vet ./... && make check lint && make test`
   - UI/生成物（若改动前端/样式）：`make generate && make css`，且生成后 `git status --short` 为空
   - E2E（当 UI 闭环可用后）：`make e2e`

## 12. 分阶段执行清单（建议）

1. [x] Phase A（契约冻结）：补齐本计划 API/错误码/状态机与权限矩阵。
2. [x] Phase B（后端）：字典值存储 + API + options/read/write 接口接入。
3. [x] Phase C（前端）：模块页面（分屏 1：左字段/右值；分屏 2：详情+变更记录）。
4. [x] Phase D（数据核验）：核验是否存在非 `10/20` 异常值；若存在则人工修复并生成执行记录。
5. [x] Phase E（收口）：门禁/E2E/执行日志，更新路线图状态。

### 12.1 Phase A 输出物清单（执行时作为 checklist）

- [x] 冻结 API：补齐 `q/limit/status/as_of` 等口径（见 §7.1），并给出请求/响应 JSON 示例（不必实现细节）。
- [x] 冻结错误码：最小集合（见 §7.2），并明确“何时返回 400/403/404/409/500”。
- [x] 冻结权限：permissionKey（UI）与 object/action（Casbin）的映射与角色策略（见 §9.1）。
- [x] 冻结 `org_type` 存量处理策略（不做自动映射；异常值人工处理，见 §10.2）。
- [x] 冻结“去静态 registry”的切换策略：允许的短期诊断兜底、时间盒、以及 No Legacy 的退出门禁。

## 13. 关联文档

- `docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
- `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- `docs/dev-plans/100h-org-metadata-wide-table-phase5-stability-performance-ops-closure.md`
- `docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- `docs/dev-plans/105a-dict-config-validation-issues-investigation.md`
- `docs/dev-plans/105b-dict-code-management-and-governance.md`
- `docs/dev-records/dev-plan-105-execution-log.md`
- `docs/dev-records/dev-plan-105a-execution-log.md`
- `docs/dev-records/dev-plan-105b-execution-log.md`
