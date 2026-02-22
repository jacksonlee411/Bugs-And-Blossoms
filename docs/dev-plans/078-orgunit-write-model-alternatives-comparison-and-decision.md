# DEV-PLAN-078：OrgUnit 写模型替代方案对比与决策建议

**状态**: 评审中（2026-02-09 — 对齐 DEV-PLAN-080 审计链收敛方向）

## 0. 背景与问题定义
- DEV-PLAN-075C/075D/075E 已收敛“删除标记、正常停用/启用、同日状态修正”的语义边界，但修正类写入仍会触发 replay。
- DEV-PLAN-026D 已让常规 `submit_org_event(...)` 走增量 apply，但 075 系列新增入口仍有全量或近全量重放风险。
- DEV-PLAN-077 基线显示 replay 在中高历史深度下写放大明显，且会带来锁等待、WAL 增长和 vacuum 压力。

> 核心矛盾：当前模型将“修复工具（replay）”长期放在“业务热路径”，导致可扩展性与可维护性持续对冲。

## 1. 当前方案批判（Baseline）

### 1.1 当前方案摘要
- 事件为 SoT：`org_events[_effective]`。
- 投影为派生：`org_unit_versions/org_trees/org_unit_codes`。
- correction/rescind/status-correction 在同事务中触发 replay 以保持强一致。

### 1.2 当前方案优点
1. 强审计与可重建：事件链天然可追溯。
2. fail-closed 边界清晰：失败可整事务回滚。
3. 与既有 075 错误码和对外契约高度兼容。

### 1.3 当前方案结构性问题
1. **写放大不可忽视**：历史修正可能引发大量派生重算。
2. **锁竞争扩散**：租户级串行化在热点租户下明显排队。
3. **业务路径与修复路径耦合**：日常写入绑定 replay，复杂度偏高。
4. **容量上限受限**：事件规模到 10^6 后，延迟抖动风险升高。
5. **运维成本上升**：WAL + autovacuum 压力使故障恢复窗口变窄。

## 2. 替代方案横向比较

### 2.1 方案定义
- 方案 A：状态 SoT（`org_unit_versions`）+ 事件仅审计。
- 方案 B：事件 SoT 保持，但禁止改历史，改为前向补偿。
- 方案 C：冷热分层（热窗口强一致、冷区异步重算）。
- 方案 D：CQRS 异步投影（最终一致）。

### 2.2 对比矩阵

| 维度 | 当前方案 | 方案A 状态SoT | 方案B 前向补偿 | 方案C 冷热分层 | 方案D CQRS |
| --- | --- | --- | --- | --- | --- |
| 写延迟上界 | 高且波动 | 低-中 | 中 | 中 | 低 |
| 百万事件弹性 | 弱 | 强 | 中-强 | 中 | 强 |
| 同事务强一致 | 强 | 强 | 强 | 局部强 | 弱 |
| 审计可追溯 | 强 | 中-强（审计链） | 强 | 强 | 强 |
| 业务改造量 | 低 | 高 | 中 | 中-高 | 高 |
| 运维复杂度 | 中 | 中 | 中 | 高 | 高 |
| 与 075 契约兼容 | 高 | 中（需映射） | 中-高 | 中 | 中-低 |
| 长尾故障面 | 中-高 | 中 | 中 | 中 | 中 |

### 2.3 决策结论（本阶段）
1. 仅做局部护栏（077-P1）不足以根治成本结构问题。
2. 若坚持同步强一致，不建议直接进入方案 D。
3. **在“未上线 + 测试数据可重建”前提下，直接落地方案 A（一次性重基线）；方案 B 仅保留为备选，不进入当前实施路径**。

## 3. 重点展开：方案 A（状态 SoT + 事件审计）

### 3.1 目标边界
- 目标：彻底删除 replay（函数与调用点），让业务写路径仅保留增量区间运算。
- 保留：One Door（单写入口）、No Legacy（无双链路）、Fail-Closed。
- 非目标：不改变 075 已冻结的业务语义与错误码名称。

### 3.2 数据模型草案
1. 主状态表（SoT）：`org_unit_versions`
   - 唯一约束建议：`(tenant_id, org_unit_id, effective_date)`（其中 `effective_date = lower(validity)`）。
   - 关键列：`status, name, parent_id, validity, row_version`。
   - 时间语义：按 DEV-PLAN-032，写入口使用 `effective_date(date)`，区间落库统一为 `validity daterange [start, end)`。
2. 审计事件表（Append-only）：`org_events`
   - 记录“谁在何时做了什么修正”，用于审计与回放分析（**唯一审计链**，对齐 DEV-PLAN-080）。
   - 关键列：`event_type, request_code, actor, reason, before_snapshot, after_snapshot, tx_time`。
   - 约束建议：`request_code`（或对外 `request_id` 的映射字段）在 tenant 级唯一。
3. 幂等键
   - 在写入口强制 `request_code/request_id` 在 **tenant 级唯一**，防重复提交（对齐 DEV-PLAN-080 的 `request_code` 口径）。

### 3.3 写路径（同事务）
> 统一模板：前置条件 -> 影响集合 -> 区间变换 -> 审计写入 -> 后置不变量 -> 幂等语义/错误码。

1. 新增/正常启用（CREATE/ENABLE）
   - 前置条件：`effective_date=D` 合法；新增时该 `org_unit_id` 尚无版本；启用时目标日存在快照且可被启用。
   - 影响集合：从 `D` 起到“下一次状态切换日”前的连续段。
   - 区间变换：在 `D` 处分裂区间；新增时插入 `[D, +inf)`；启用时将受影响段 `status=active`。
   - 审计写入：写入 `org_events`（event_type=CREATE/ENABLE，记录 request_code/request_id、actor、reason、before/after）。
   - 后置不变量：保持 `no-overlap`/`gapless`/末段一致性；同日仅一条有效快照。
   - 幂等/错误码：同 `request_id` 同语义幂等成功；异语义返回 `ORG_REQUEST_ID_CONFLICT`；目标不存在返回 `ORG_NOT_FOUND_AS_OF`。

2. 正常停用（DISABLE）
   - 前置条件：目标日存在快照，且允许停用。
   - 影响集合：从 `D` 起到“下一次状态切换日”前的连续段。
   - 区间变换：在 `D` 处分裂；将受影响段 `status=disabled`；前一段上界截断为 `D`（半开区间，不使用 `D-1`）。
   - 审计写入：写入 `org_events`（event_type=DISABLE）。
   - 后置不变量：保持 `no-overlap`/`gapless`/末段一致性。
   - 幂等/错误码：同 `request_id` 同语义幂等成功；异语义返回 `ORG_REQUEST_ID_CONFLICT`。

3. 同日状态修正（CORRECT_STATUS）
   - 前置条件：目标日存在快照；对应审计事件语义为 ENABLE/DISABLE；目标事件未被撤销（rescind）。
   - 影响集合：从 `D` 起到“下一次状态切换日”前的连续段。
   - 区间变换：仅修正该连续段的 `status`，**不改 `effective_date`**，不触碰非状态字段。
   - 审计写入：写入 `org_events`（event_type=CORRECT_STATUS），保留 before/after。
   - 后置不变量：保持 `no-overlap`/`gapless`/末段一致性；同日仍只有一条有效快照。
   - 幂等/错误码：同 `request_id` 同语义幂等成功；异语义返回 `ORG_REQUEST_ID_CONFLICT`；目标不存在返回 `ORG_EVENT_NOT_FOUND`；目标已撤销返回 `ORG_EVENT_RESCINDED`。

4. 删除记录（语义删除 = Rescind Event）
   - 说明：**删除记录即 rescind**，只是用户语义别名；实现必须走同一内核函数与审计通道，不得出现双入口。
   - 前置条件：目标事件存在；`request_id`/`reason` 必填。
   - 影响集合：目标事件对应的业务可见区间，以及相邻可合并区间。
   - 区间变换：撤销目标事件的业务可见性，回补/合并相邻区间（不物理删除审计与事件）。
   - 审计写入：写入 `org_events`（event_type=RESCIND/DELETE_RECORD，记录 before/after）。
   - 后置不变量：保持 `no-overlap`/`gapless`/末段一致性。
   - 幂等/错误码：同 `request_id` 同语义幂等成功；异语义返回 `ORG_REQUEST_ID_CONFLICT`；目标不存在返回 `ORG_EVENT_NOT_FOUND`。

5. 边界用例（日期示例）
   - 首段停用：已有 `active [2025-01-01, +inf)`，在 `2026-01-01` 停用 -> `active [2025-01-01, 2026-01-01)` + `disabled [2026-01-01, +inf)`。
   - 末段停用：已有 `active [2025-01-01, 2026-02-01)`，在 `2026-02-01` 停用 -> `disabled [2026-02-01, +inf)`。
   - 同日冲突：`2026-03-01` 已有状态事件，重复写 ENABLE/DISABLE 走 `EVENT_DATE_CONFLICT`，需引导 CORRECT_STATUS。
   - rescind 后再修正：`2026-01-15` 事件已 rescind，再发 CORRECT_STATUS -> `ORG_EVENT_RESCINDED`。
   - 重复 request_id（同语义）：同 `request_id` 重放同一操作 -> 幂等成功。
   - 重复 request_id（异语义）：同 `request_id` 但不同操作/参数 -> `ORG_REQUEST_ID_CONFLICT`。

### 3.4 必须满足的不变量
1. 同一 `org_unit_id` 的有效期区间 `no-overlap`（不可重叠）。
2. 时间轴 `gapless`（除显式停用段外不留隐式空洞）。
3. 末段边界一致（最后一段满足 `upper_inf(validity)=true` 或明确终止上界）。
4. 同日只允许一条业务有效状态快照。
5. API 幂等：同一 `tenant_id` 下相同 `request_id` 重试不得产生额外版本。

### 3.5 并发与锁策略
- 由租户级串行化收敛为“组织/子树级锁”：
  - 优先 `pg_advisory_xact_lock(hash(tenant_id, org_unit_id))`；
  - move/merge 场景锁定最小必要子树。
- 冲突策略：fail-fast + 重试指引，避免长事务阻塞。

### 3.6 与 075 语义映射
- 075C 删除语义：继续区分“删除记录”与“停用”；**删除记录=Rescind 事件**（语义别名，同一内核函数）。
- 075D 显式状态切换：仍保持有效/无效显式入口。
- 075E 同日修正：映射为“同日快照改写”，不再依赖 replay。
- 077-P1 高风险重排禁令：在方案 A 中保留为防呆护栏。

### 3.7 风险与回滚
1. 风险：状态 SoT 改造触及写模型核心，需严密对账。
2. 回滚：不引入读写双链路；仅允许“环境级保护 + 只读/停写 + 修复后重试”。
3. 可观测：保持最小排障证据（错误码、request_id、关键 SQL 路径），不引入迁移期开关体系。
4. 派生表损坏处置：本阶段**不引入**离线重建工具；仅允许“环境级保护 + 清库重灌 + 重建最小 seed”恢复。

## 4. 迁移策略（开发早期：一次性重基线）

### 4.1 阶段 0：契约冻结与切换窗口（必做）
- 冻结唯一目标模型：`org_unit_versions`（状态 SoT）+ `org_events`（append-only 审计，唯一审计链）。
- 冻结时间语义与不变量：`effective_date + validity [start,end)`、同日唯一、no-overlap、gapless。
- 约定一次性切换窗口：窗口内允许清空/重建测试数据，不做在线灰度。

### 4.2 阶段 1：数据库重基线（一次完成）
- 直接以目标 schema 重建 OrgUnit 写模型相关对象（表/约束/函数/索引）。
- 删除迁移期对象与命名（如 `*_v2`、影子表、临时对账对象），不保留兼容层。
- 删除 `orgunit.replay_org_unit_versions(...)` 及其相关权限配置、错误码映射与维护入口。
- 清空旧测试数据并按新模型重灌最小可复现 seed（仅保留验证所需样本）。

### 4.3 阶段 2：应用层一次性切换
- 一次性切换到新写模型入口；删除旧路径分支与迁移期开关代码。
- 明确禁止 `read_from_v2` / `write_to_v2` / 按租户白名单回退等 legacy 形态。
- 保持 One Door：写入统一走 DB Kernel，不新增第二写入口。
- correction/rescind/status-correction 全量改为增量路径；不得再调用 replay。

### 4.4 阶段 3：一次性验收（无灰度）
- DB 闭环：`make orgunit plan && make orgunit lint && make orgunit migrate up`。
- 质量门禁：按触发器执行 `go fmt ./... && go vet ./... && make check lint && make test`。
- 单链路门禁：`make check no-legacy` 必须通过；任何迁移期开关残留视为阻断项。
- 若失败：进入只读/停写保护，修复后重跑，不回退到旧读写链路。

### 4.5 不采纳项（明确拒绝）
- 说明：开发早期已确认“测试数据可重建”，以下迁移策略统一不采纳。
- 结论：**不采纳**。
- 理由：
  1. 影子写 + 双写对账 + 读写灰度（会引入双链路复杂度，与 No Legacy 冲突）。
  2. `read_from_v2/write_to_v2` 开关化迁移（不符合早期阶段“避免过度运维”）。
  3. 保留 replay 作为长期 break-glass 工具（会固化第二实现路径与维护成本）。
  4. 历史事件物理裁剪/删除（破坏审计可解释性；且对当前问题收益有限）。
  5. 派生表离线重建工具/脚本（避免引入第二条修复路径与运维复杂度；本阶段仅允许清库重灌）。

### 4.6 稳态收敛
- `org_unit_versions` 成为唯一业务读写 SoT。
- `org_events` 成为唯一 append-only 审计链。
- `org_events_audit` 与 `org_event_corrections_*` 被移除。
- 系统中不再保留 replay 函数、调用链与运行时入口。

### 4.7 受影响 API 分组（实施范围）

> 原则：优先保持对外 URL 与请求/响应形状稳定；本次主要调整写入内核与错误码映射（删除 `ORG_REPLAY_FAILED`）。

#### A 组：OrgUnit 常规写入 API（中等影响，回归必跑）
- `POST /org/api/org-units`（创建）
- `POST /org/api/org-units/rename`（改名）
- `POST /org/api/org-units/move`（调整上级）
- `POST /org/api/org-units/disable`（停用）
- `POST /org/api/org-units/enable`（启用）
- `POST /org/api/org-units/set-business-unit`（设置业务单元）
- 影响说明：维持现有接口契约；需验证在“删除 replay 后”写入不变量（同日唯一/no-overlap/gapless）仍稳定。

#### B 组：OrgUnit 修正/撤销 API（高影响，重点改造）
- `POST /org/api/org-units/corrections`（记录更正）
- `POST /org/api/org-units/status-corrections`（同日状态纠错）
- `POST /org/api/org-units/rescinds`（删除记录）
- `POST /org/api/org-units/rescinds/org`（删除组织）
- 影响说明：本组原先直接/间接依赖 replay；本次改为增量区间运算实现，且需移除 `ORG_REPLAY_FAILED` 相关返回与前端提示映射。

#### C 组：OrgUnit UI 写入口（高影响，用户可见）
- `POST /org/nodes`（表单动作：`create`、`rename`、`move`、`change_status`、`add_record`、`insert_record`、`delete_record`、`delete_org`、`set_business_unit`）
- 影响说明：入口不变，但所有修正/删除类动作需改走“无 replay”新路径；错误提示文案与前端错误码映射需同步。

#### D 组：OrgUnit 读 API/读页面（低代码改动，高回归优先级）
- `GET /org/api/org-units`
- `GET /org/nodes`
- `GET /org/nodes/children`
- `GET /org/nodes/details`
- `GET /org/nodes/view`
- `GET /org/nodes/search`
- 影响说明：通常不改接口实现，但必须验证“写后即读”一致性，避免出现版本链断裂或状态回显错误。

#### E 组：跨模块依赖 API（联调回归）
- `GET/POST /org/api/positions`
- `GET/POST /org/api/assignments`
- `GET/POST /org/positions`
- `GET/POST /org/assignments`
- 影响说明：Staffing 依赖 OrgUnit 有效状态（as_of）校验；需验证 OrgUnit 写模型改造后，岗位/任职链路无回归。

### 4.8 实施子计划（表级清单：新增/改造/删除）

> 注意：本次不新增表；如后续新增表必须先获得用户手工确认。所有 DDL 变更通过 Atlas+Goose 闭环。

#### 新增表（New）
- 无（对齐 DEV-PLAN-080：审计链收敛到 `org_events`，不新增审计表）。

#### 改造表（Alter/Behavioral）
- `orgunit.org_unit_versions`
  - DDL 要点：维持 `validity daterange` 与 no-overlap 约束；补齐/强化 `gapless` 与 `upper_inf` 校验入口（函数/触发器/校验查询）。
  - 语义变化：由投影表提升为 SoT；写路径改为增量区间运算（禁止全量 replay）。

- `orgunit.org_unit_codes`
  - DDL 要点：结构不变；需要支持增量维护（由 apply_* 逻辑生成/更新）。
  - 语义变化：移除“全量重放后重建”的依赖。

- `orgunit.org_trees`
  - DDL 要点：结构不变；需要支持增量维护（move/rename/create 时局部更新）。
  - 语义变化：移除“全量重放后重建”的依赖。

- `orgunit.org_events`
  - DDL 要点：作为唯一审计链；补齐 `request_code`/`reason`/`before_snapshot`/`after_snapshot`/`tx_time`；`request_code` 施加 tenant 级唯一约束。
  - 语义变化：不再触发 replay；修正/撤销通过区间增量逻辑直接更新状态表；审计写入统一落在 `org_events`。

#### 删除对象（Drop）
- `orgunit.org_events_audit`（若已存在，需在迁移中删除）
- `orgunit.org_event_corrections_current`
- `orgunit.org_event_corrections_history`
- `orgunit.replay_org_unit_versions(...)` 及其权限配置/入口
  - DDL 要点：`DROP FUNCTION` + 清理 `GRANT/REVOKE` 相关脚本。
  - 同步删除：`ORG_REPLAY_FAILED` 错误码映射与测试用例。

#### 迁移期对象清理（Cleanup）
- 任意 `*_v2`、影子表、临时对账表（若存在）必须删除。
  - DDL 要点：`DROP TABLE` / `DROP VIEW`，并从迁移与文档中移除引用。

### 4.9 生效日期字段命名合规性调查（专项）

#### 合规基准（SSOT）
- 有效期输入字段统一使用 `effective_date`（类型 `date`）。
- 有效期区间落库统一使用 `validity daterange [start,end)`。
- 禁止 `valid_from/valid_to` 作为业务有效期字段名（避免口径漂移）。
- 允许 `end_date` 仅作为兼容表达，且必须明确闭区间与 `[start,end)` 的换算。

#### 本计划内调查结论（OrgUnit 写模型）
- 方案 A 设计已统一为 `effective_date + validity`，不再使用 `valid_from/valid_to`。
- 迁移后不允许再引入 `valid_from/valid_to` 或 `end_date` 作为新表字段名。

#### 审查动作（实施前后必须执行）
- 代码/迁移静态扫描：
  - `rg -n "\\bvalid_from\\b|\\bvalid_to\\b" modules/orgunit migrations/orgunit`
  - `rg -n "\\beffective_date\\b|\\bvalidity\\b" modules/orgunit migrations/orgunit`
- 若发现新增 `valid_from/valid_to` 字段，必须阻断合并并回退为 `effective_date/validity` 口径。

### 4.10 实施子计划拆分（内嵌版）

> 说明：按你的要求，子计划不另建文档，直接内嵌在 078 中；后续如需独立追踪再拆分为独立文件。

#### 078A：去 replay + 修正/撤销增量化（DB Kernel）
- 背景与上下文：修正/撤销仍调用 replay，形成双实现路径与写放大。
- 目标：删除 `replay_org_unit_versions` 调用；让 correction/rescind/status-correction 全量走增量区间运算。
- 非目标：不改对外 API 形状、不引入双链路、不新增表。
- 触发器与门禁：DB 迁移/Schema；Go 代码与测试（见 `AGENTS.md` 触发器矩阵）。
- 数据模型与约束：不新增表；强化 `no-overlap/gapless/upper_inf` 校验入口与 fail-closed 行为。
- 核心逻辑：
  - 计算前后相邻有效期，做区间拆分/合并。
  - 变更节点及子树的派生字段（如 `full_name_path`）只做局部更新。
  - 租户级锁保持，避免并发破坏区间。
- 失败路径：冲突/越界/父级无效/同日重复均 fail-closed，不允许部分成功。
- 验收：不变量检查通过；`rg -n "replay_org_unit_versions"` 结果为空；对应测试通过。
- 依赖：075/075B/075E 口径；026D 增量投影逻辑。
- 回滚：进入只读/停写保护并修复后重试，禁止回退旧链路。

#### 078B：审计链收敛（`org_events`）+ 纠错表清理 + 表级重基线
- 背景与上下文：DEV-PLAN-080 要求单一审计链；去 replay 后需统一审计口径。
- 目标：将 `org_events` 扩展为唯一审计链；删除 `org_events_audit` 与 `org_event_corrections_*`；完成表级 DDL 变更与重基线。
- 非目标：不引入历史兼容表或影子表，不引入双链路。
- 触发器与门禁：DB 迁移/Schema（Atlas+Goose 闭环）。
- 数据模型与约束：
  - `org_events` 为 append-only；`tx_time` 默认 `now()`；`tenant_uuid + tx_time` 索引。
  - `request_code` tenant 级唯一；补齐 `reason/before_snapshot/after_snapshot`。
  - 不与业务读写链路耦合（仅审计）。
- 核心逻辑：审计写入与业务写同事务提交；写失败应 fail-closed；纠错/撤销不再写入纠错表。
- 验收：`make orgunit plan` 无漂移；`org_events_audit` 与 `org_event_corrections_*` 不再存在；表结构与索引满足计划口径。
- 依赖：078A 完成删除 replay 后再收口审计口径；对齐 DEV-PLAN-080。
- 回滚：早期可清库重灌并重基线（需记录证据），禁止回退双链路。

#### 078C：API/错误码/UI 文案清理（删除 ORG_REPLAY_FAILED）
- 背景与上下文：replay 移除后该错误码不应继续暴露。
- 目标：彻底移除 `ORG_REPLAY_FAILED` 相关映射与测试。
- 非目标：不改变其他错误码语义。
- 触发器与门禁：Go 代码与测试；文档门禁。
- 接口契约：对外错误码集合中移除 `ORG_REPLAY_FAILED`。
- 失败路径：删除映射后，相关错误需被其他稳定错误码覆盖或 fail-closed。
- 验收：`rg -n "ORG_REPLAY_FAILED"` 结果为空；相关测试更新通过。
- 依赖：078A 删除 replay 调用；078B 完成审计链收敛。
- 回滚：回滚代码与迁移即可恢复旧映射（仅限早期）。

#### 078D：回归与一致性测试
- 背景与上下文：replay 对照测试被删除，需要新的确定性证据。
- 目标：用“不变量 + 读写一致性”替代 replay 对照测试。
- 非目标：不新增大规模性能基建。
- 触发器与门禁：Go 代码与测试；如触发 sqlc/UI 生成按 SSOT 执行。
- 测试范围：覆盖 A/B/C/D/E 组 API 与 UI 写入口，重点覆盖更正/撤销/插入记录。
- 验收：`make check lint && make test` 通过；关键场景具备最小证据记录。
- 依赖：078A/078B/078C 完成后执行。
- 回滚：测试失败即阻断合并，修复后重跑。
- 最小回归用例草案（日期固定，可直接转测试用例）：
  1. 首段停用：前置已有 `active [2025-01-01, +inf)`；操作 `DISABLE@2026-01-01`；期望 `active [2025-01-01, 2026-01-01)` + `disabled [2026-01-01, +inf)`，不变量通过。
  2. 末段停用：前置已有 `active [2025-01-01, 2026-02-01)`；操作 `DISABLE@2026-02-01`；期望 `disabled [2026-02-01, +inf)`，不变量通过。
  3. 同日冲突：`2026-03-01` 已有状态事件；操作 `ENABLE@2026-03-01`；期望 `EVENT_DATE_CONFLICT`，引导走 `CORRECT_STATUS`。
  4. rescind 后再修正：`2026-01-15` 事件已 rescind；操作 `CORRECT_STATUS@2026-01-15`；期望 `ORG_EVENT_RESCINDED`。
  5. 重复 request_id（同语义）：同 `request_id` 重放同一操作；期望幂等成功（结果一致）。
  6. 重复 request_id（异语义）：同 `request_id` 但不同操作/参数；期望 `ORG_REQUEST_ID_CONFLICT`。

#### 078E：测试数据重灌 + 最小 E2E 样板
- 背景与上下文：开发早期数据可重建，需要可重复的最小样板。
- 目标：一次性重基线后，提供可重复的最小 seed 与可见 E2E 流程。
- 非目标：不引入生产级数据导入工具。
- 触发器与门禁：E2E/文档门禁按 SSOT 执行。
- 工作清单：
  - 最小 seed 可重复执行（覆盖 OrgUnit 关键路径）。
  - 至少 1 条 UI 可见端到端操作（新增/插入/纠错）走通。
- 验收：seed 可重复执行；E2E 流程可复现并记录证据。
- 依赖：078A~078D 完成后执行。
- 回滚：重灌失败可清库重做（测试环境）。

## 5. 里程碑与验收标准
> 证据统一写入：`docs/dev-records/dev-plan-078-execution-log.md`（记录命令、时间、样本规模、结果、提交号）。

1. M1（契约冻结）
   - 文档冻结“一次性重基线 + 无双链路 + 无迁移期开关”。
   - 不变量映射为可执行检查：`no-overlap`、`gapless`、同日唯一、`request_code/request_id` 幂等。
   - 验收方式：评审清单签字 + 执行日志中记录冻结日期与 commit hash。

2. M2（数据库重基线完成）
   - 必跑命令：`make orgunit plan && make orgunit lint && make orgunit migrate up && make orgunit plan`。
   - 通过标准：最后一次 `make orgunit plan` 为 No Changes；迁移期对象（`*_v2`/影子表/临时对账对象）清理完成。
  - 删除标准（以最终 Schema/SSOT 与代码为准，历史迁移文本不纳入扫描范围）：
     - `rg -n "org_events_audit|org_event_corrections_" modules/orgunit/infrastructure/persistence/schema internal/sqlc/schema.sql internal/server` 结果为空。
     - `rg -n "replay_org_unit_versions|ORG_REPLAY_FAILED" modules/orgunit/infrastructure/persistence/schema internal/sqlc/schema.sql internal/server` 结果为空。
   - 数据标准：测试数据重灌成功（最小可复现 seed 可重复执行）。

3. M3（功能与门禁验收）
   - 必跑命令：`go fmt ./... && go vet ./... && make check lint && make test`。
   - 单链路门禁：`make check no-legacy` 必须通过。
   - 触发器补充：若命中 sqlc/UI 生成链路，额外执行 `make sqlc-generate` 或 `make generate && make css`，且 `git status --short` 无生成物漂移。
   - 业务验收：075C/075D/075E 语义不回归；至少 1 条用户可见端到端操作可完成。

4. M4（性能与稳态验收，取消）
   - 说明：按当前指令取消，不作为 078/080 的前置要求。

## 5A. 实施步骤与路径（执行清单）
> 按 SSOT 与 078 子计划拆分执行；保持“单链路 + 无 replay + 无离线重建工具”。

1. 契约冻结与门槛确认（M1）
   - 明确“状态 SoT + 事件审计”已替代 026，路线图与入口一致。
   - 幂等口径：`request_code/request_id` 仅在 tenant 级唯一（建议唯一约束：`(tenant_id, request_code)`）。
   - 明确“本阶段不引入派生表离线重建工具”。
   - 记录冻结日期与 commit hash 到执行日志。

2. DB Kernel 改造（078A）
   - 删除 `replay_org_unit_versions(...)` 调用链。
   - correction/rescind/status-correction 全量切换为增量区间写入。
   - 保持 One Door：写入仅走 DB Kernel 入口。

3. 数据库重基线（078B）
   - 扩展 `orgunit.org_events` 为唯一审计链（append-only）。
   - 删除 `orgunit.org_events_audit` 与 `org_event_corrections_*`（若已存在，迁移中清理）。
   - `org_unit_versions` 升格为 SoT；强化 `no-overlap/gapless/upper_inf` 校验入口。
   - 清理 `*_v2`/影子表/临时对账对象（若存在）。
   - 删除 `replay_org_unit_versions` 及相关权限脚本与错误码映射。

4. 应用层切换（078C）
   - API/UI 修正与撤销动作改走“无 replay”路径。
   - 彻底移除 `ORG_REPLAY_FAILED`（代码、文案、测试）。

5. 回归与一致性测试（078D）
   - 覆盖 A/B/C/D/E 组 API 与 UI 写入口。
   - 执行不变量验证：no-overlap/gapless/同日唯一/request_id 幂等。
   - 运行质量门禁与触发器命令（按 AGENTS.md）。

6. 测试数据重灌 + 最小 E2E（078E）
   - 清库重灌最小 seed（覆盖 OrgUnit 关键路径）。
   - 至少 1 条 UI 可见端到端操作（新增/插入/纠错）走通并记录证据。

7. 验收与 Go/No-Go
   - DB 闭环：`make orgunit plan && make orgunit lint && make orgunit migrate up && make orgunit plan`。
   - 删除标准：`rg -n "replay_org_unit_versions|ORG_REPLAY_FAILED" modules/orgunit migrations/orgunit internal/server` 结果为空。
   - 性能门槛：correction/rescind P95 延迟下降 >= 60%，WAL 写入量下降 >= 50%。
   - 证据记录完整，否则 No-Go（进入只读/停写保护并修复后重试）。

## 6. Go/No-Go 门槛
- Go：M1~M4 全部通过，且执行日志证据完整（命令、结果、样本、提交号可追溯）。
- No-Go：任一硬门禁失败（DB 闭环/No Legacy/测试不变量/性能指标/未删除 replay）即冻结合并；进入只读/停写保护修复，禁止回退旧链路。

## 7. 评审待确认
1. [x] 是否同意当前阶段直接落地方案 A（一次性重基线），不启用方案 B 过渡？
2. [x] 是否同意“开发早期一次性重基线”，不采用影子写/灰度/双开关迁移？
3. [x] 是否确认立即删除 replay（函数、调用点、错误码、入口）并纳入硬门禁？
4. [x] 是否批准进入 DEV-PLAN-078 的实施子计划拆分？

## 8. 关联文档
- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/archive/dev-plans/026d-orgunit-incremental-projection-plan.md`
- `docs/dev-plans/075c-orgunit-delete-disable-semantics-alignment.md`
- `docs/archive/dev-plans/075e-orgunit-same-day-correction-status-conflict-investigation.md`
- `docs/dev-plans/077-orgunit-replay-write-amplification-assessment-and-mitigation.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`
- `docs/dev-records/dev-plan-077-write-amplification-baseline.md`
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`

## 9. 变更登记
- 2026-02-09：PR #312 合并（merge commit `1164bce78c0684862172ec39e19dc2ec73d6685f`）
  - 影响：确立本计划为当前实施口径；明确 `request_code/request_id` tenant 级唯一；明确本阶段不引入派生表离线重建工具；新增“实施步骤与路径（执行清单）”。
- 2026-02-09：078A 完成（PR #314）
  - 影响：去 replay + 修正/撤销增量化；删除 replay 调用链，correction/rescind/status-correction 全量改为增量区间运算。
- 2026-02-09：078B 完成（PR #315）
  - 影响：新增 `orgunit.org_events_audit` 审计表与写入逻辑；补齐权限与迁移；同步 sqlc 产物并修正相关测试。
- 2026-02-09：方向调整（对齐 DEV-PLAN-080）
  - 影响：审计链收敛到 `org_events`；`org_events_audit`/`org_event_corrections_*` 路线标记为需回收；更新 078B/表级清单/里程碑与验收口径。
