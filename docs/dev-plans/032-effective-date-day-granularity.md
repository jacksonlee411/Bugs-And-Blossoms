# DEV-PLAN-032：Valid Time（日粒度 Effective Date）与 Audit/Tx Time（时间戳）口径

**状态**: 草拟中（2026-01-06 15:10 UTC）

> 本文为仓库内“时间语义”的单一事实源（SSOT）：把**业务有效期（Valid Time）**统一收敛为 **date（日粒度）**；把**操作/审计/事务时间（Audit/Tx Time）**统一收敛为 **timestamptz（秒/微秒级）**。  
> 目标：对齐 HR 领域习惯（SAP HCM `BEGDA/ENDDA`、PeopleSoft `EFFDT`），并减少实现期各模块各自发明“时间字段含义/边界规则”的漂移。

## 1. 核心定义（必须遵守）

### 1.1 Valid Time（业务有效期）
- **定义**：业务语义上的“生效日期/失效日期/有效区间”，粒度为 **day**。
- **数据类型**：必须使用 `date`（不得用 `timestamptz` 表达业务有效期）。
- **用途**：effective-dated 主数据/组织结构/任职记录等“随日期演化”的业务事实。

### 1.2 Audit/Tx Time（操作/审计/事务时间）
- **定义**：记录“何时被写入/修改/提交到系统”的时间戳（与业务有效期无关）。
- **数据类型**：使用 `timestamptz`（例如 `created_at` / `updated_at` / `transaction_time`）。
- **用途**：审计、排障、重放、幂等与一致性验证。

## 2. 标准建模模式（推荐）

### 2.1 Effective-Dated：`versions` + `daterange`（推荐模式）
对采用“版本表（versions）”的实体：
- `effective_date` 作为输入/事件字段，类型为 `date`。
- DB 中推荐使用 `validity daterange` 存储有效区间，统一采用 **半开区间 `[start, end)`**（day-range）。

**约束建议（用于门禁/一致性）**
- **no-overlap**：同一 `(tenant_id, setid?, business_key...)` 的 `validity` 不得重叠。
- **gapless（若该实体要求连续）**：相邻版本必须首尾相接（上一段 `upper(validity)` 等于下一段 `lower(validity)`）。
- **last infinite**：最后一段版本 `upper_inf(validity) = true`（若业务要求“当前版本开放式有效”）。

> 是否要求 gapless/last-infinite 取决于具体实体的契约（由对应 DEV-PLAN 冻结）；本计划只冻结“Valid Time 的表示法与边界规则”。

### 2.2 若必须存 `end_date`（不推荐但可兼容）
若某些场景更适合存储 `effective_date` + `end_date`（闭区间语义，均为 `date`）：
- 业务语义可采用闭区间 `[effective_date, end_date]`（含 end）。
- DB 做不重叠约束时，应转换为半开区间：`daterange(effective_date, end_date + 1, '[)')`。

## 3. API/UI 契约（输入输出）
- 外部接口（JSON/Form）传递的有效日期必须是 `YYYY-MM-DD`。
- 不允许在接口层引入时分秒来表达业务有效期（避免“时区/午夜边界”歧义）。

## 4. 与 SetID/多租户的关系
- `tenant_id` 是硬隔离边界（RLS/租户注入），与 Valid Time 无关。
- `setid`（若该表受 SetID 控制）参与“共享/隔离”与唯一性约束，但不改变 Valid Time 语义。

## 5. 停止线（避免 Easy 式漂移）
- 在业务有效期字段上使用 `timestamptz`、或引入时区换算逻辑。
- 同一实体在不同模块/表里使用不同的区间边界规则（有人用闭区间、有人用半开区间）且无 SSOT 说明。
- 为了兼容/偷懒在读路径引入“时间回退/默认今天/取最近一条”等隐式规则，而不在契约中冻结。
