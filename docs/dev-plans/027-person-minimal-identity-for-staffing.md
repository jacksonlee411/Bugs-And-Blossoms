# DEV-PLAN-027：Person 最小身份锚点（Pernr 1-8 位数字字符串，前导 0 同值）以支撑 Staffing 落地

**状态**: 草拟中（2026-01-05 04:26 UTC）

## 1. 背景与上下文 (Context)

`DEV-PLAN-031` 选定：任职记录（Assignment/Job Data）归属 `modules/staffing`，写路径以 `person_uuid` 作为唯一身份锚点；`pernr`（工号）仅作为 UI 查询/筛选与展示用途（不进入 write-side 合同）。

为避免 `staffing` 与 `persons` 表形成隐式耦合（跨模块直接查表/解析），`DEV-PLAN-016/031` 明确要求：**pernr → person_uuid 的解析由 `modules/person` 提供 options/read API**，并把“人员身份”收敛为一个最小、稳定、可复用的契约。

当前仓库已存在 `modules/person` 的模块骨架，但 **Person 的 schema/table 尚未作为 SSOT 落盘**（`modules/person/infrastructure/persistence/schema/*.sql` 仍为空）。
为使“Person Identity”从文档合同变为可执行合同，本计划将把最小 `person.persons` 表与其约束、RLS、以及对外 read API 一并落地，作为 `modules/staffing` 落地的前置契约。

## 2. 目标与非目标 (Goals & Non-Goals)

### 2.1 核心目标
- [ ] 冻结 Person Identity 的最小合同：`person_uuid` + `pernr`（**1-8 位数字字符串**）+ `display_name` + `status`。
- [ ] 约束 `pernr` 的格式与唯一性：同租户唯一，且必须为 **1-8 位数字字符串**（最多 8 位；**`00001234` 与 `1234` 视为同一工号**）。
- [ ] 提供 `staffing`/前端可直接复用的 pernr→uuid 解析能力（options/read API），避免 `staffing` 在 Go/SQL 层直接读 `persons` 表做解析。
- [ ] 保持与 `DEV-PLAN-015/016/031` 的边界一致：Person 仅负责身份锚点，不承载任职/组织/职位等跨域逻辑。

### 2.2 非目标（明确不做）
- 不在本计划内引入“人员主数据全量模型”（证件/地址/雇佣信息等）。
- 不在本计划内把 Person 强制改造成事件 SoT + versions（`DEV-PLAN-016` 已明确：Person 可选是否事件化，本计划默认不强制）。
- 不在本计划内实现任职/组织/职位等跨域逻辑；Person 仅负责身份锚点与解析。
- 不在本计划内引入跨模块 FK（默认仅存 ID；对齐 `DEV-PLAN-024` 的 stopline）。

> 说明：为使合同可执行，本计划将新增 `person.persons` 表（最小字段），并按 `AGENTS.md` 红线执行（`CREATE TABLE` 需用户手工确认）。

### 2.3 工具链与门禁（SSOT 引用）
- DDD 分层与共享策略：`docs/dev-plans/015-ddd-layering-framework.md`
- 4 模块边界与跨模块契约：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`
- Staffing 方案（对 Person 的依赖点）：`docs/dev-plans/031-greenfield-assignment-job-data.md`
- Kernel 风格与 One Door Policy：`docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- RLS（本计划将 Person 纳入强租户隔离）：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- 命令入口与触发器矩阵：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`

## 3. 对 `DEV-PLAN-031` 的架构分析：Person 的最小责任面

`DEV-PLAN-031` 对 Person 的需求可以收敛为 3 点：
1. **身份锚点**：给任职时间线的 write-side 提供稳定的 `person_uuid`（跨模块统一 ID）。
2. **解析能力**：把用户输入/筛选用的 `pernr` 映射到 `person_uuid`（options/read API），由 Person 模块对外提供。
3. **展示数据**：为 UI 展示提供最小字段（`pernr`、`display_name`、`status`），其余主数据不在本阶段强行纳入。

因此，本计划的边界是：**Person=Identity Anchor**，而不是“人员全量档案系统”。

## 4. 方案：Person 最小身份锚点（Minimal Identity Anchor）

### 4.0 关键决策（冻结）

1) **Pernr 形状与同值规则**：
- 输入形状：选定 `pernr` 为 **1-8 位数字字符串**（`^[0-9]{1,8}$`），类型为 `text/string`。
- 同值规则：**忽略前导 0**，即 `00001234` 与 `1234` 为同一个工号。
- 规范化（canonical）：
  - `normalize_pernr(v)`：`v=btrim(v)`；校验 `v ~ '^[0-9]{1,8}$'`；将 `v` 的前导 0 去掉；若结果为空则置为 `'0'`。
  - DB 存储与跨模块传递使用 `normalize_pernr` 的结果（避免同一个概念出现两套权威表达）。
  - 展示层如需 8 位显示，可在渲染时对 canonical 值做左侧补 0（presentation concern），但不得把“展示形态”作为唯一性/查找的权威输入。

理由：本仓库语境已使用 SAP 术语 `pernr`，且 `DEV-PLAN-031` 的 UI/筛选会把它视为“工号”；将其收敛为“最多 8 位数字 + 前导 0 同值”有利于：一致性（避免多种格式并存）、可索引、可解释（少歧义），且允许逐步从短工号过渡到更长工号。  
若未来必须支持非数字工号，应以**显式的破坏性变更**升级本合同（放宽 regex + 回填/迁移策略），而不是在实现期隐式兼容。

2) **解析契约单一来源**：`pernr → person_uuid` 的 **精确解析**必须由 `modules/person` 提供稳定 API（`persons:by-pernr`），`persons:options` 仅用于 UI 联想/选择，不作为精确解析的替代路径。

### 4.1 数据模型与不变量（Domain Contract）

**实体：Person（Identity）**
- `person_uuid`：UUID，跨模块唯一身份锚点（write-side 使用）
- `pernr`：**1-8 位数字字符串（canonical）**（例如输入 `00001234` 将规范化为 `1234`；输入全 0 将规范化为 `0`），同租户唯一
- `display_name`：展示名（非空，trim 后存储）
- `status`：`active|inactive`
- `created_at/updated_at`：审计时间（`timestamptz`；对齐 032 的“审计时间”语义）

**不变量（必须）**
- `pernr` 必须满足：
  - `btrim(pernr) = pernr`（无前后空格）
  - `pernr ~ '^[0-9]{1,8}$'`（严格 1-8 位数字字符串）
  - canonical：`pernr = '0' OR pernr !~ '^0'`（禁止前导 0；前导 0 在写入时必须被规范化掉）
- `(tenant_id, pernr)` 唯一
- `display_name` 非空且 trim 后存储

### 4.2 DB 约束（最小）与兼容策略

> 实施阶段的 SSOT：`modules/person/infrastructure/persistence/schema/*.sql`（对齐 `DEV-PLAN-024/025` 的目录约定）。

- 本仓库落盘策略：直接以“canonical pernr”作为存储权威表达，并用 DB check 约束兜底：
  - digits：`CHECK (pernr ~ '^[0-9]{1,8}$')`
  - canonical：`CHECK (pernr = '0' OR pernr !~ '^0')`
  - trim：`CHECK (btrim(pernr) = pernr)`、`CHECK (btrim(display_name) = display_name)`

**实施前置检查（避免盲目加约束）**
- [ ] 若未来引入存量数据导入：必须在导入前先跑审计（非数字/超长/前导 0 同值冲突），并给出修复策略后再导入（避免把“清洗逻辑”塞进运行时分支）。

### 4.3 对外接口契约（供 Staffing/前端复用）

> 目标：让 `staffing` 的 UI/API 在不 import `modules/person/**` 的前提下完成 pernr→uuid 解析（对齐 016 的“跨模块以 HTTP/JSON API 组合”）。

**必须提供（最小集）**
- `GET /person/api/persons:options?q=<pernr_or_name>&limit=...`
  - 返回：`items[]`，每项含 `person_uuid/pernr/display_name`
  - 用途：表单选择器/搜索联想（Staffing create/edit）
  - 约束：若 `q` 满足 `^[0-9]{1,8}$`，则服务端应先执行 `normalize_pernr(q)` 再用于查询（保证 `00001234` 与 `1234` 的搜索体验一致）。

- `GET /person/api/persons:by-pernr?pernr=<digits_max8>`
  - 用途：**精确解析** pernr→person_uuid；Staffing/前端在以 pernr 作为筛选参数时，应先解析为 `person_uuid` 再查询 Staffing（避免 Staffing 自己做解析）

**错误契约（稳定错误码）**
- `persons:by-pernr`：
  - 400 `PERSON_PERNR_INVALID`：缺少/非法 `pernr`（不匹配 `^[0-9]{1,8}$`）
  - 404 `PERSON_NOT_FOUND`：该租户下不存在该 `pernr`
  - 500 `PERSON_INTERNAL`：内部错误

> 说明：创建 Person 的 body 校验与冲突错误码沿用现有约定（`PERSON_VALIDATION_FAILED` / `PERSON_PERNR_CONFLICT`），本计划不强制引入新的 create-side 错误码，避免无谓扩散。

### 4.4 跨模块边界（与 016 对齐）

- `modules/staffing` 的 write-side 合同只接收 `person_uuid`，不接收 `pernr`（对齐 031）。
- `modules/staffing` 不应在 Go 层 import `modules/person/**`；需要 pernr→uuid 时，走 Person 的 options/read API（或由前端先解析）。
- DB 层如需强一致性（可选）：`staffing` 表可对 `(tenant_id, person_uuid)` 建立外键引用 `persons(tenant_id, person_uuid)`，以拒绝孤儿引用；但这不改变“解析责任属于 person 模块”的边界。

### 4.5 与 RLS（021）的关系（本计划强制）

本计划将 `person.persons` 纳入“强租户隔离（fail-closed）”，与 OrgUnit/JobCatalog 同口径：
- 事务内注入 `app.current_tenant`
- `persons` 启用 `ENABLE/FORCE ROW LEVEL SECURITY` + `tenant_isolation` policy
- Go 访问路径必须满足 “No Tx, No RLS” 契约（否则会出现 fail-closed）

**实现提醒（避免脚枪）**
- `persons:options`/`persons:by-pernr` 都是读路径，但由于表启用 RLS，读路径同样必须在事务内注入 tenant（fail-closed）。

## 5. 实施步骤（Plan → Implement）

1. [ ] 冻结 Person Identity 合同（本文档）并在 `AGENTS.md` Doc Map 登记。
2. [ ] DB：落盘 `person` schema 与 `person.persons`（最小字段 + check constraints + RLS policy），并接入 Atlas+Goose 闭环（`make person plan/lint/migrate up`）。
3. [ ] Go：实现 `pernr` 规范化与校验（1-8 位数字 + 前导 0 同值），并用于 API 入参校验与查询路径。
4. [ ] API：补齐 `persons:by-pernr`（精确解析）与 `persons:options`（联想），并提供稳定错误码（对齐 §4.3）。
5. [ ] UI（可选但推荐用于 M2 演示）：提供最小 Person 列表/创建入口，便于 end-to-end 验收。
6. [ ] 测试：新增最小测试覆盖（pernr 校验、按 pernr 解析不存在/存在、RLS fail-closed）。
7. [ ] 门禁对齐：命中项按 `AGENTS.md` 触发器矩阵执行（Go + schema/migrations + authz + routing + doc）。

## 6. 验收标准 (Acceptance Criteria)
- [ ] `pernr` 不满足 `^[0-9]{1,8}$` 时：创建 Person 必须失败（Go 校验 + DB 双保险）。
- [ ] 同租户重复 `pernr`：必须以稳定错误码失败（409 / `PERSON_PERNR_CONFLICT` 或等价契约）。
- [ ] `persons:options` 返回可用于 staffing 选人的最小字段：`person_uuid/pernr/display_name`。
- [ ] `persons:by-pernr`：非法 pernr 必须 400 `PERSON_PERNR_INVALID`；存在则返回稳定结构；不存在则 404 `PERSON_NOT_FOUND`。

## 7. 风险与缓解
- **存量数据不满足“最多 8 位数字”约束**：先做审计与修复策略，再落 DB constraint（必要时 `NOT VALID`）。
- **前导 0 同值导致的冲突**：实施前先跑重复检测（按 `normalize_pernr(pernr)` 分组），若存在冲突需先决策“保留哪个 person_uuid/如何合并引用”。
- **边界漂移**：避免在 Person 中引入任职/组织/职位字段；Staffing 只依赖 `person_uuid`，展示数据通过 read API 或读侧 join（由 Staffing 计划明确）。

## 8. Simple > Easy Review（DEV-PLAN-003）
- 通过：将 Person 收敛为“身份锚点”，不把 Staffing 的复杂度搬到 Person（避免“万能模块”）。
- 通过：用明确的不变量（pernr 最多 8 位数字 + 前导 0 同值）+ 稳定 API 合同替代隐式解析与跨模块查表。
- 需警惕：为“省事”让 Staffing 直接依赖 `persons` 表做解析/写入会形成双权威与边界漂移，应在实现评审中阻断。
