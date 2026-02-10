# DEV-PLAN-080A：OrgUnit before_snapshot/after_snapshot 机制调查与收敛修复方案

**状态**: 草拟中（2026-02-10 01:10 UTC）

## 背景
DEV-PLAN-080 将 OrgUnit 的审计事实源收敛到 `orgunit.org_events`，并在 OrgUnit Details 的「变更日志」页签中以 `before_snapshot/after_snapshot` 生成字段级 diff。

近期发现一个典型问题：对 `RENAME`（组织更名）事件，变更日志详情页无法显示“变更前”的旧值，只能看到类似：

- 字段：`new_name`
- 变更前：`-`
- 变更后：`财务部2`

该现象不是纯 UI 漏渲染，而是 `before_snapshot/after_snapshot` 的整体语义与落库形态不一致导致的“机制性问题”。本文件专门对该机制做调查与批判，并提出收敛修复方案。

## 目标
- 解释并冻结 **before_snapshot/after_snapshot 的语义**：它们分别代表什么、面向谁、在什么时点产生。
- 盘点现状（DB Kernel / 迁移 / UI 渲染），明确为什么会出现 `RENAME` 看不到变更前值。
- 形成可落地的收敛方案，使「变更日志」稳定展示：**同一业务字段同名对齐（例如 name）**，并能可靠给出“变更前/后”。

## 非目标
- 不引入第二写入口（遵循 One Door）。
- 不引入 legacy/双链路回退。
- 不做“为补齐快照而引入 replay/离线重建”的历史回填工具链（与 080 的非目标一致；历史缺口先暴露，再按窗口策略逐步收口）。
- 不在本文件内复制工具链/门禁矩阵；以 `AGENTS.md` 为准。
- 不追求通用 JSON 深度 diff 引擎（本轮优先解决“字段口径与快照形态”）。

## 现状调查（What is happening today）

### 1) 数据结构现状：orgunit.org_events
`orgunit.org_events` 具备：
- `payload`：事件 payload（目前更接近“操作 patch”）。
- `before_snapshot/after_snapshot`：jsonb，可空。

但目前缺少“快照形态”层面的强约束：并未强制其为 object、也未强制哪些事件类型必填。

### 2) 写入现状：不同事件类型的快照来源不一致
当前写入路径大致呈现三类形态：

1. **基础事件（CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT）**
   - 仅写 `after_snapshot = payload`（等价于把 patch 当作 after_snapshot）。
   - 通常不写 `before_snapshot`。

2. **纠错/撤销事件（CORRECT_EVENT/CORRECT_STATUS/RESCIND_*）**
   - 会写 `before_snapshot`，但其内容往往是“目标事件行（org_events rowtype）序列化后的 JSON”，而不是业务字段快照。
   - `after_snapshot` 往往是“纠错/撤销自身的 payload（含 op/target_event_uuid 等）”。

3. **触发器（fill_org_event_audit_snapshot）**
   - 仅补齐 `tx_time` 与 actor 元数据（initiator_*），不负责 before/after 的业务快照生成。

### 3) 读与渲染现状：UI diff 的假设过强
变更日志详情页的核心逻辑是：
- 把 `before_snapshot/after_snapshot` 各自 JSON 解析成 `map[string]any`。
- 取 keys 的并集，逐 key 生成“字段/变更前/变更后”。
- 如果 `after_snapshot` 为空，视为快照缺失并显式暴露，不做 payload 回退。

这个算法隐含了一个强假设：
> before_snapshot 与 after_snapshot 应该是“同构的业务字段集合（canonical shape）”。

但现状恰好相反：
- 基础事件：after_snapshot 是 patch（例如 `new_name`），before_snapshot 常为空。
- 纠错/撤销：before_snapshot 可能是“整行事件对象”，after_snapshot 是 patch/op。

因此 UI diff 只能得到“技术上成立但业务上不可读”的结果。

## 机制批判（Why this is a design smell）

### 批判 1：把 payload（patch）当作 after_snapshot（state）是概念错误
- `payload` 的职责是驱动内核 apply_* 逻辑；它自然倾向于使用 `new_name/new_parent_id` 这类“动作型字段名”。
- `after_snapshot` 在审计语义上更接近“变更后的业务状态片段（state snapshot）”，应该使用 `name/parent_id/status/...` 这类“状态型字段名”。

把 patch 直接写进 after_snapshot 会导致：
- 业务字段名漂移（name vs new_name）。
- before/after 无法对齐同一字段，diff 失真。
- 变更前值无法展示（因为 before 常为空）。

### 批判 2：before_snapshot 写成“目标事件整行”会把审计层的噪声注入 diff
对 CORRECT/RESCIND：`before_snapshot = to_jsonb(v_target)` 这种策略会把 event row 的字段（id/tenant_uuid/request_code/tx_time/initiator_uuid/…）混入 before_snapshot。

在 UI diff 算法下，这些 key 将和 after_snapshot（通常是 patch/op）发生大量无意义差异，导致：
- “字段变更”表格充满元数据差异，掩盖真正业务变更。
- 变更日志可读性下降，背离 DEV-PLAN-080 的目标（可追溯、可理解、可排障）。

### 批判 3：缺少 shape 约束与生成策略，使得快照字段名无法冻结
DEV-PLAN-080 以“快照字段”承载审计展示，但目前并没有把快照 shape 升格为契约：
- 未明确哪些 key 是 canonical。
- 未明确各事件类型 before/after 的必填与来源。
- 未通过 DB 约束/测试把契约钉死。

结果就是：任何一个 payload 设计决定（比如 rename 用 new_name）都会外溢到审计 UI。

## 证据索引（代码与文档）
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`：080 对 diff 的规则与目标。
- `docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`：rename payload 使用 `new_name` 的历史约定。
- `internal/sqlc/schema.sql`：
  - `org_events` 字段与约束定义；
  - `submit_org_event`、`submit_org_event_correction`、`submit_org_event_rescind`、`submit_org_status_correction` 的 before/after 写入形态；
  - `fill_org_event_audit_snapshot` 触发器职责边界。
- `internal/server/orgunit_nodes.go`：变更日志详情 diff 渲染逻辑（当前已移除 `after_snapshot` 为空时 fallback，改为显式暴露快照缺失）。
- `modules/orgunit/services/orgunit_write_service.go`：rename 事件写入 payload 使用 `new_name`。

## 结论：我们需要的“快照契约”（Proposed Contract）

### 1) 明确语义：snapshot 是业务状态，不是 patch
- `payload`：操作意图/patch（面向内核 apply_*）。
- `before_snapshot/after_snapshot`：面向审计展示的 **业务状态快照（canonical shape）**。

### 2) 定义 OrgUnit 审计快照 canonical shape（可扩展版）
本计划不采用“固定几个字段”的白名单方案，而采用 **开放式业务字段快照**：

- `before_snapshot/after_snapshot` 只承载“业务状态（state）”，并且默认包含 **所有可能被编辑的业务字段**。
- 元数据（例如 `event_uuid/request_code/initiator_uuid/tx_time/...`）不进入快照，继续由 `org_events` 的列承载，并在 UI 摘要/原始数据区展示。

> 核心原则：**宁可多存业务字段，也不要因为字段遗漏而把问题藏起来**。快照完整性优先于“字段选择的精致化”。

#### 2.1 快照对象结构（非版本化，当前方案）
当前阶段不引入快照版本号，快照对象直接使用业务字段对象：

```json
{
  "...business_state_fields": "..."
}
```

- 顶层对象就是用于 diff 的业务字段集合（默认全量）。
- 若未来确实出现结构演进需求，再以独立计划引入版本化（不在 080A 当前范围内预埋）。

#### 2.2 字段来源策略：从“业务 SoT”抽取，而不是从 payload 拼装
为了可扩展性与一致性，快照的字段来源必须“来源驱动”而不是“手写字段列表”：

1) **基础字段（当前已建模）**：统一从 `orgunit.org_unit_versions` 抽取业务状态。
- 这样新增字段（新增列）只要进入 versions，就会自动进入快照（无需改 UI/Go）。

2) **未来可配置字段**：必须有一个统一落点，才能自动进入快照。
- 推荐：在 `org_unit_versions` 增加 `custom_fields/attrs`（jsonb object）作为扩展容器。
- 当“用户配置启用新字段”时，只允许通过该容器（或新增列）承载；否则无法保证审计与 diff 的完整性。

3) **元数据剔除**：采用 denylist（拒绝清单）而不是 allowlist（允许清单）。
- allowlist 的问题：每次加新字段都要改代码/改列表，极易漏；漏了就等于审计缺口。
- denylist 的思路：只排除明确的元数据，其余默认保留，天然前向兼容。

#### 2.3 具体生成算法（推荐形态）
推荐在 DB Kernel 提供一个“快照抽取函数”，返回 canonical snapshot（业务字段对象）：

- 输入：`tenant_uuid + org_id + as_of_effective_date`
- 输出：`{...business_state_fields...}`

快照对象建议基于 `to_jsonb(org_unit_versions_row)` 自动包含所有列，再剔除明确元数据/噪声列：

- 明确元数据（建议剔除）：`tenant_uuid`、`id`、`last_event_id`
- 强噪声/派生列（可选剔除，视展示需要）：`path_ids`

> 注意：`validity/node_path/full_name_path/status/...` 是否剔除要基于“是否属于业务状态”而非“是否好看”。本计划倾向于保留业务状态；展示层再做分组/排序/默认折叠。

额外建议：把 `org_code` 一并并入快照（来自 `org_unit_codes`），以便 UI 展示可读性（若未来 org_code 可变，则应明确其审计语义并纳入 versions）。

#### 2.4 元数据判定规则（新增，强约束）
为保证“元数据剔除”可执行、可审计，采用以下判定规则（按优先级）：

1) **系统身份字段优先判定为元数据**
- 用于唯一标识事件/请求/租户/操作者的字段，一律判定为元数据。
- 例：`event_uuid/request_code/tenant_uuid/initiator_*`。

2) **系统时序字段优先判定为元数据**
- 用于记录写入时间、事务时间、创建时间的字段，一律判定为元数据。
- 例：`tx_time/transaction_time/created_at`。

3) **业务状态可重建性判定**
- 若移除某字段后仍能完整表达“该组织在该生效日的业务状态”，该字段判定为元数据。
- 例：`last_event_id/id`。

4) **业务可编辑性判定**
- 能被业务动作或用户配置直接改变、并影响业务状态解释的字段，判定为业务字段，必须入快照。
- 例：`name/parent_id/status/is_business_unit/manager_*/custom_fields`。

5) **冲突处理规则（兜底）**
- 同一字段若存在歧义，默认按“业务字段”处理（即保留进快照），避免误剔除造成审计缺口。

#### 2.5 元数据剔除清单（初始，作用域=快照对象）
本清单仅作用于“快照抽取函数”生成的业务快照对象（来源：`orgunit.org_unit_versions` + 必要的补充 join）。

以下字段在快照对象中默认剔除（denylist）：

- `id`（versions 行 id，非业务语义）
- `tenant_uuid`（隔离上下文，不属于业务状态）
- `last_event_id`（技术锚点，不属于业务状态）
- `path_ids`（派生列，可选；若用于展示路径可保留）

> 说明：事件层元数据（如 `event_uuid/request_code/initiator_* / tx_time`）不属于快照来源，原则上不应进入快照对象；它们应继续由 `org_events` 列承载，并在“摘要/原始数据”区展示。

> 说明：当前暂不引入“字段目录/registry”，也暂不引入快照版本号。展示顺序与标签先采用稳定默认规则（业务高频字段优先 + 其余按稳定序）；后续若引入 registry，仅用于展示治理，不改变“是否入快照”的判定。

#### 2.6 denylist 落地机制（执行细则）
为把“采用 denylist”从原则落到实现，冻结以下执行规则：

1) **唯一执行入口（Single Entry）**
- 仅允许在 DB Kernel 的快照抽取函数中执行剔除逻辑（建议签名：`orgunit.extract_orgunit_snapshot(p_tenant_uuid uuid, p_org_id int, p_as_of date) RETURNS jsonb`）。
- 其它层（handler/service/UI）不得再次做字段剔除，以避免口径漂移。

2) **单一事实源（Single Source）**
- denylist 初始清单仅维护在该快照抽取函数内（SQL 常量段），不在多处复制。
- 若后续确有需要再抽成独立配置，但在 080A 范围内保持“函数内单源”。

3) **剔除实现形态（SQL 形态）**
- 先以 `to_jsonb(org_unit_versions_row)` 取全量业务状态。
- 再以连续减键方式剔除 denylist（示意）：
  - `v_snapshot := to_jsonb(v_row) - 'id' - 'tenant_uuid' - 'last_event_id' - 'path_ids';`
- 对扩展容器（如 `custom_fields/attrs`）不做二次白名单过滤，默认保留。

4) **变更治理（新增字段默认保留）**
- 新增字段时，必须在 PR 描述中声明“是否元数据”。
- 若未声明，默认按业务字段处理并进入快照。
- 若新增可编辑字段没有落到 `org_unit_versions` 列或 `custom_fields/attrs`，该变更不得上线；必须先完成模型接入，再谈审计展示。
- 仅当字段满足 2.4 判定规则并评审通过，方可加入 denylist。

5) **回归门禁（可验证）**
- 必须新增/保留至少 2 类自动化断言：
  - `denylist` 字段不出现在快照 diff；
  - 非 denylist 新字段（含未来配置字段）无需改 UI diff 代码即可出现在快照 diff。

### 3) before/after 的时间口径
对基础事件（CREATE/MOVE/RENAME/DISABLE/ENABLE/SET_BUSINESS_UNIT）：
- `before_snapshot`：该事件生效前的业务状态（建议以“apply 前的 as-of effective_date 状态”作为定义）。
- `after_snapshot`：该事件生效后的业务状态（建议以“apply 后的 as-of effective_date 状态”作为定义）。

对纠错/撤销（CORRECT_*/RESCIND_*）：
- `before_snapshot`：纠错/撤销前该 effective_date 的业务状态（以 effective view/versions 为准）。
- `after_snapshot`：纠错/撤销后该 effective_date 的业务状态。

这样，UI diff 才能稳定展示“同一字段的前后对比”。

## 修复方案（Design-to-Implementation Plan）

### Phase 0：最小可用修复（止血）
1. [ ] **UI 侧移除 after_snapshot 回退逻辑（立刻暴露数据缺口）**
   - `after_snapshot` 为空时，不再 fallback 到 payload。
   - 页面显式提示快照缺失（before/after 哪侧缺失）。

> 评价：这是“先暴露问题再修机制”的必要动作，防止错误数据被 UI 掩盖。

### Phase 1：内核生成 canonical before/after（根治）
2. [ ] **DB Kernel：在写入 org_events 时生成 before_snapshot/after_snapshot（来源统一）**
   - 对基础事件：在 `submit_org_event` 内，apply 前读取一次 canonical snapshot，apply 后再读取一次，分别写入 before/after。
   - 对纠错/撤销：以“变更前后业务状态”生成 before/after，禁止 `to_jsonb(v_target_row)` 直写到 before。
   - 快照抽取函数若返回 `NULL`、非 object 或字段来源越界（非 versions/custom_fields），写入必须 fail-closed。

3. [ ] **停止把 patch 写进 after_snapshot**
   - `after_snapshot` 不再等于 `payload`。
   - `payload` 继续保留为 patch（保持 apply_* 逻辑稳定）。

4. [ ] **扩展字段统一接入策略（面向未来配置）**
   - 确认“未来用户配置字段”的唯一落点（强约束：仅允许 `org_unit_versions.custom_fields/attrs`）。
   - 快照抽取函数默认并入该扩展容器，保证新字段启用后自动进入审计快照。

5. [ ] **denylist 单点落地（按 2.6 执行）**
   - 在 DB Kernel 快照抽取函数中实现 denylist 剔除（Single Entry + Single Source）。
   - handler/service/UI 不再重复做字段剔除。

> 注意：本阶段不引入第二写入口；所有写入仍通过内核函数，符合 One Door。

#### Phase 1.5：迁移与历史缺口处理策略（前向收口，无 replay）
由于 080/080A 明确不引入 replay/离线重建工具链，历史事件要补齐“变更前/后业务状态”在技术上通常需要重建状态序列，属于高风险与高成本。

因此本方案采用“先前向正确，再逐步收口历史”的策略：

- **前向写入（强制）**：自部署切换点起，所有新写入事件必须生成 `before_snapshot/after_snapshot`（按 3) 时间口径），否则写入失败（fail-closed）。
- **历史缺口（允许暴露）**：切换点之前的历史事件若缺快照，不做 UI 回退；页面显式提示“快照缺失”。
- **约束落地（避免阻断历史）**：DB 约束采用“切换点豁免”写法，而不是要求全表历史立刻满足。

约束示例（示意）：
- 对基础事件：`(created_at < :cutover) OR (before_snapshot IS NOT NULL AND after_snapshot IS NOT NULL)`
- 对 CREATE：允许 `before_snapshot IS NULL` 表示“对象不存在”（但必须有 `after_snapshot`）。

> Stopline：除 CREATE 的 before 允许 NULL 外，其它基础事件出现 NULL 视为写链路缺陷，必须 fail-closed。


### Phase 2：约束与回归（把契约钉死）
6. [ ] **增加 DB 约束（shape 优先，presence 先走内核 fail-closed）**
- `before_snapshot/after_snapshot` 若非空必须是 JSON object（可用 CHECK 约束固定）。
- **presence（必填）**：由于快照是在 `submit_*` 中以同事务 UPDATE 回写，直接用表级 CHECK 约束会在 INSERT 阶段提前触发；本阶段先用内核 `assert_org_event_snapshots(...)` fail-closed 固定必填口径，后续若要上升到表级约束，需要把快照写入改为 INSERT 即写齐（或引入可延迟校验机制）。

7. [ ] **补齐自动化回归（含扩展字段）**
   - RENAME：断言 diff 输出 `name | <old> | <new>`，旧值不是 `-`。
   - CORRECT/RESCIND：断言默认 diff 不混入元数据噪声。
   - denylist：断言剔除字段不会出现在快照 diff。
   - 扩展字段：模拟启用新字段后，断言无需改 diff 代码即可在变更日志中显示该字段。

## 验收标准
- [ ] 对 RENAME 事件：变更日志显示 `name` 的前后值，不出现 `new_name` 作为业务 diff 字段。
- [ ] 除元数据外，所有可编辑业务字段（含未来配置字段）都会进入快照并可被 diff。
- [ ] 新字段启用后（通过新增列或扩展容器），无需改 UI diff 白名单即可被审计页看见。
- [ ] denylist 仅在 DB Kernel 快照抽取函数维护，且新增字段默认保留（除非评审通过加入 denylist）。
- [ ] 默认 diff 不混入元数据字段（event_uuid/tenant_uuid/request_code/tx_time 等），原始数据区仍可完整查看。
- [ ] before_snapshot/after_snapshot 形态由 DB 约束 + 自动化测试固定，不随 payload 结构漂移。

## 工具链与门禁（SSOT 引用）
- 触发器矩阵与本地必跑：`AGENTS.md`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 命令入口：`Makefile`
- Atlas/Goose：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc：`docs/dev-plans/025-sqlc-guidelines.md`

## 关联文档
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
- `docs/dev-records/dev-plan-080-execution-log.md`
- `docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- `AGENTS.md`
