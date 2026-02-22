# DEV-PLAN-098：组织架构模块架构评估——多类型宽表预留字段 + 元数据驱动（V2.0）

**状态**: 草拟中（2026-02-12 23:34 UTC）

## 0. 变更说明（V2.0）

- 新增第 7 章：补齐“字典数据 vs 业务实体数据”的区分逻辑，明确 Select 类字段的存储规范（存 ID vs 存 Value）。
- 对第 7 章之后的章节进行了重新编号，以保证逻辑连续性。

---

## 1. 执行摘要与架构综述

在多租户 SaaS（软件即服务）应用开发中，组织架构（Organization Structure）模块由于其核心地位与高度定制化需求，往往成为系统设计的深水区。用户提出的架构方案—— **“多类型宽表预留字段 + 元数据驱动”** ——旨在解决关系型数据库（RDBMS）严格的模式约束（Schema Rigidity）与 SaaS 业务对灵活性需求之间的根本矛盾。该方案避开了传统的实体-属性-值（EAV）模式带来的查询复杂性，也试图修正纯 JSONB 文档存储模式在排序与聚合性能上的短板。

本报告基于 PostgreSQL 数据库内核原理、存储机制、索引算法及并发控制机制，对该方案进行了详尽的解构与评估。分析表明，该架构利用 PostgreSQL 特有的**空值位图（NULL Bitmap）**存储优化机制，在存储效率与查询性能之间取得了显著优于 JSONB 的平衡，特别是在涉及大规模数据的排序（Sorting）、范围扫描（Range Scan）及聚合（Aggregation）场景下，原生列（Native Column）展现出了数量级的性能优势。

为了进一步完善该方案，本报告 V2.0 版新增了对于**业务实体与字典数据**的建模区分策略，并结合 **ltree** 扩展处理层级关系、**部分索引**解决多租户索引膨胀问题，形成一套完整的“混合核心-扩展架构（Hybrid Core-Extension）”。

---

## 2. 核心架构深度解析：宽表模式的物理存储机制

要评估“宽表预留字段”方案的可行性，必须深入 PostgreSQL 的元组（Tuple）存储层。对于 SaaS 应用而言，最核心的担忧在于：预留大量（如 100 个）字段是否会导致存储空间的浪费？以及这些空闲字段是否会拖累查询性能？

### 2.1 元组头部与空值位图的存储物理学

PostgreSQL 的堆元组（Heap Tuple）由 `HeapTupleHeader` 和后续的数据区域组成。在 `HeapTupleHeaderData` 结构中，包含了一个至关重要的组件：`t_bits`（空值位图）。这是一个变长的位数组，用于标记哪些列是 NULL。

根据 PostgreSQL 的存储机制，如果在行数据中某一列的值为 NULL，数据库 **不会** 在数据区域为该列预留任何存储空间，也不存储长度信息。相反，它仅仅在 `t_bits` 数组中设置对应的一位（1 bit）。这意味着，如果一个宽表预留了 100 个列，而某一行数据仅填充了前 5 个列，其余 95 个列为 NULL，那么这 95 个 NULL 值在磁盘上的总开销仅为 `ceiling(95 / 8) = 12` 字节，加上少量的对齐填充。

相比之下，若采用 JSONB 方案存储稀疏数据，即使是存储一个简单的 null 值，也需要在 JSON 文档结构中维护键名（Key String）及相应的结构开销。因此，从存储密度的角度分析，利用 PostgreSQL 的原生 NULL 位图特性来处理 SaaS 场景下的稀疏数据，在理论上是极其高效的。

### 2.2 数据对齐与 CPU 访问效率

在宽表设计中，建议严格按照数据类型的宽度进行物理排序，而非逻辑用途排序，以利用内存对齐（MAXALIGN）减少 padding 浪费。最优的物理定义顺序应为：

1. 定长 8 字节类型（bigint, timestamptz, double precision）
2. 定长 4 字节类型（integer, real, date）
3. 定长 2 字节类型（smallint）
4. 定长 1 字节类型（boolean）
5. 变长类型（text, varchar, jsonb）

### 2.3 变长字段与 TOAST 机制的边界

必须警惕将“备注”、“简介”等长文本字段混入预留字段。如果多个预留字段同时存储长文本，会导致频繁的 TOAST 访问。架构中必须明确：**预留字段仅用于短文本与结构化数据，长文档数据应剥离至 JSONB 或专用文本列**。

---

## 3. 性能基准对比：原生宽表 vs. JSONB

### 3.1 排序操作的算法复杂度分析

在 SaaS 列表中，“按自定义字段排序”是一个高频且计算昂贵的操作。

- **原生列（Native Column）：** 当执行 `ORDER BY int_01` 时，PostgreSQL 能够直接利用列的统计信息选择最优排序算法。如果存在 B-Tree 索引，时间复杂度接近 O(1)（分页场景）。
- **JSONB：** 当执行 `ORDER BY (data->>'years_exp')::int` 时，必须在运行时进行“提取-转换”，不仅消耗 CPU，且优化器难以准确评估成本。
  研究表明，在数百万行级别的数据集上，原生列的排序性能通常比无索引的 JSONB 提取排序快一个数量级以上。

### 3.2 聚合分析与列式统计

对于 `SELECT avg(int_01)...` 这类聚合查询，原生列享有 SIMD 优化潜力。而 JSONB 列缺乏单一属性的统计分布信息（Statistics），容易引发磁盘溢出（Disk Spill）。

### 3.3 写入放大（Write Amplification）风险

JSONB 的更新涉及整份文档的重写，产生巨大的 WAL 日志流量。而原生列（特别是未索引列）的更新可以利用 HOT（Heap Only Tuple）特性进行页内更新，大幅减少索引维护开销。

---

## 4. 层次结构数据的终极方案：ltree 深度集成

组织架构的核心是树状关系。传统的 `parent_id` 邻接表模式在深层级查询时性能较差。

### 4.1 引入 ltree 扩展

强烈建议集成 `ltree` 扩展处理 `dept_path`（部门路径，如 `Root.Eng.Backend`）。

| 操作类型 | 递归 CTE (parent_id) | ltree 路径枚举 (path) | 性能差异 |
| --- | --- | --- | --- |
| 查询所有后代 | 慢 (多次 Join/递归) | **极快 (索引扫描)** | ltree 胜出 (数量级优势) |
| 节点移动 | 快 (更新 1 行) | 慢 (更新所有子孙节点) | CTE 胜出 |

### 4.2 读写分离的权衡

鉴于 SaaS 组织架构 **“极高频读取，低频写入”** 的特征，`ltree` 的读取优势无可比拟。配合 GiST 索引，查询“研发部及其下属所有团队”只需：

```sql
SELECT * FROM org_employees WHERE dept_path <@ 'Global.Eng';
```

---

## 5. 多租户环境下的索引策略优化

### 5.1 多租户索引策略（建议口径：避免 per-tenant index）

在多租户系统里，“为每个租户单独建索引（per-tenant index）”虽然在单租户压测里看起来很美，但一旦租户数量上来，会带来：

```sql
-- 1) DDL/迁移与回滚复杂度急剧上升（索引数量爆炸）
-- 2) 索引维护与 bloat 管理成本不可控
-- 3) 生产变更窗口被大量索引操作占满
```

因此更推荐“通用复合索引 + 稀疏列的 NOT NULL 部分索引”的组合口径（按真实热点字段逐步加）：

```sql
-- 通用：tenant 复合索引（绝大多数查询天然带 tenant 过滤）
CREATE INDEX idx_org_units_tenant_attr_str_01
ON org_units (tenant_uuid, attr_str_01);

-- 可选：稀疏列用 NOT NULL 过滤，显著缩小索引体积（适用于宽表扩展列）
CREATE INDEX idx_org_units_tenant_attr_str_01_not_null
ON org_units (tenant_uuid, attr_str_01)
WHERE attr_str_01 IS NOT NULL;
```

补充建议：

- 预留字段（宽表扩展列）**不应默认全部建索引**；只对“明确需要排序/筛选且频率高”的少量字段加索引，并以压测/慢查询证据驱动。
- 如果扩展字段需要 `ILIKE/contains` 模糊匹配，优先考虑 `pg_trgm` + GIN（同样遵循“热点字段少量启用”的原则）。

### 5.2 BRIN 索引与时序数据

对于 `created_at` 等时序字段，建议使用 BRIN 索引，其大小仅为 B-Tree 的千分之一，适合大表范围查询。

---

## 6. 模式演进与锁机制管理

### 6.1 锁风险与无锁迁移

添加列（ADD COLUMN）虽然通常快，但需获取排他锁，可能阻塞生产环境。

### 6.2 解决方案

建议集成 **pg-osc** 或 **reshape** 等工具，通过“影子表 + 触发器同步 + 瞬间切换”的机制，实现零停机的字段扩容。

---

## 7. 数据模型精细化设计：字典与业务实体的架构分层

**（本章节为 V2.0 新增核心内容）**

在前端交互层面，“选择离职原因”与“选择职级”通常都表现为下拉菜单（Select Widget）。然而，在后端架构与数据模型层面，二者存在本质区别。为了保证系统的可扩展性与数据一致性，必须在元数据配置层对**“字典数据（Dictionary）”**与**“业务实体数据（Business Entity）”**进行严格区分。

### 7.1 核心差异对比

| 特性 | 字典数据 (如：离职原因、性别) | 业务实体数据 (如：职级、职位、部门) |
| --- | --- | --- |
| **数据本质** | **KV 键值对** (Code-Label) | **结构化对象** (Object / Row) |
| **属性复杂度** | 仅包含 Label 和 Value，通常无额外业务逻辑 | **属性丰富**（如职级包含：薪资范围、能力模型；职位包含：编制数） |
| **数据量级** | 较小（通常 < 100 个选项） | 可能较大（成百上千，随业务增长） |
| **生命周期** | 系统预设为主，偶尔微调 | **租户完全自定义**，频繁增删改 |
| **宽表存储策略** | **存 Value (String)** (如 'reason_code_01') | **存 ID (BigInt)** (即 Foreign Key) |

### 7.2 元数据层配置设计

在租户字段配置表（Metadata Table）中，需增加 `data_source_type` 字段以驱动后端逻辑。

**Schema 示例 (`tenant_field_config`)：**

```sql
CREATE TABLE tenant_field_config (
    id               BIGSERIAL PRIMARY KEY,
    tenant_uuid      uuid,
    field_key        VARCHAR(50),      -- 业务键，如 'rank_id' 或 'leaving_reason'
    physical_col     VARCHAR(50),      -- 物理列映射，如 'int_01' (实体ID) 或 'str_01' (字典Code)

    -- 【核心区分字段】
    widget_type      VARCHAR(20),      -- 前端控件类型，均为 'SELECT_SINGLE'
    data_source_type VARCHAR(20),      -- 区分关键：'DICT' vs 'ENTITY'

    -- 【数据源详细配置】
    -- DICT 存字典编码；ENTITY 存实体表名及关联字段
    data_source_config JSONB
);
```

**配置场景示例：**

1. **离职原因（字典）：**
   - `data_source_type`: `DICT`
   - `data_source_config`: `{"dict_code": "reason_leaving"}`
   - `physical_col`: 映射至 **`str_xx`**。

2. **职级（实体）：**
   - `data_source_type`: `ENTITY`
   - `data_source_config`: `{"entity_name": "tenant_rank", "label_field": "rank_name", "value_field": "id"}`
   - `physical_col`: 映射至 **`int_xx`**（存储 `tenant_ranks` 表的主键）。

### 7.3 实体数据的独立存储设计

对于职级、职位等实体数据，决不能混入通用字典表，必须建立独立的业务表以支撑复杂的业务属性。

**职级定义表 (`tenant_ranks`)：**

```sql
CREATE TABLE tenant_ranks (
    id           BIGSERIAL PRIMARY KEY,
    tenant_uuid  uuid,
    rank_code    VARCHAR(50),  -- 租户自定义编码，如 P5
    rank_name    VARCHAR(100), -- 租户自定义名称，如 资深专家
    salary_min   DECIMAL,      -- 关联业务属性：薪资下限
    salary_max   DECIMAL,      -- 关联业务属性：薪资上限
    is_active    BOOLEAN
);
```

**架构收益：**

- **引用完整性：** 宽表存储 ID，当租户修改 `rank_name` 时，员工档案自动展示新名称，无需批量更新宽表。
- **业务扩展性：** 未来若需统计“所有薪资范围在 20k 以上的员工”，可通过 JOIN `tenant_ranks` 轻松实现，而字典模式无法支持此需求。

### 7.4 决策模型

建议开发团队遵循以下决策树：

1. 该选项是否包含除 Name/Value 外的额外业务属性？（是 -> **Entity**）
2. 该选项是否会被其他模块（如审批流、薪酬计算）作为逻辑判断依据？（是 -> **Entity**）
3. 租户是否需要对该数据进行高频的增删改管理？（是 -> **Entity**）
4. 否则 -> **Dict**。

---

## 8. 应用层集成与元数据缓存

### 8.1 元数据驱动的 SQL 生成

应用程序不应使用 `SELECT *`。应用层应根据 `tenant_field_config`，动态生成精确的 SQL。对于 **Entity** 类型的字段，应用层需在组装数据时决定是仅返回 ID，还是执行 Join/Look-up 操作以返回实体名称。

### 8.2 高性能行扫描（Row Scanning）

在 Go 语言实现中，建议使用 **pgx** 驱动配合 **scany** 库或自定义的代码生成器，以避免使用 `reflect` 带来的运行时开销。对于宽表结构，应根据元数据生成对应的 Go Struct 或 Map 结构，确保数据映射的高效性。

---

## 9. 结论与优化建议

综合评估，“多类型宽表预留字段”方案在 PostgreSQL 环境下是构建高性能 SaaS 组织架构模块的最优解之一。

**最终架构推荐配置：**

1. **分层存储策略：**
   - **Tier 1（核心字段）：** `id`, `tenant_uuid`, `node_path (ltree)`，原生列，强制非空。
   - **Tier 2（高频扩展字段）：** 预留 `str_01~30`, `int_01~10` 等。其中 `int` 类型重点用于存储 **业务实体 ID**（如职级 ID），`str` 类型用于存储 **字典 Code**（如离职原因）。
   - **Tier 3（长尾字段）：** `custom_data (JSONB)`，用于存储无搜索需求的纯展示型数据。

2. **数据模型规范：** 严格区分字典与实体，禁止将具有业务属性的实体数据降维存储为字典字符串。
3. **智能索引管理：** 利用部分索引（Partial Index）解决多租户索引隔离问题。
4. **运维自动化：** 集成 pg-osc 以应对未来的字段扩容需求。

通过上述优化，该架构不仅能满足 SaaS 业务当前的灵活性需求，更能支撑未来复杂业务逻辑（如薪酬计算、审批流转）对数据模型的高要求。

---

## 10. 实施可行性评估（面向本仓库落地）

> 目的：把“理论可行”收敛到“在本仓库约束下可交付”。本节强调边界、不变量与最小闭环路径，避免把 098 变成引入第二写入口/弱一致性的借口。

**实施承接**：落地实施步骤与路线图见 `DEV-PLAN-100`：`docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`。

### 10.1 现有基座（可直接复用）

- **树结构与查询基座已存在**：OrgUnit 已采用 `ltree`（路径列为 `node_path`）表达层级结构，并配套 GiST/Gin 索引支撑子树/路径查询（见 `modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql:178`）。
- **有效期/生效日口径已对齐**：仓库对“业务生效”统一使用 `date` 与 `daterange`（Valid Time day 粒度），非常适合“Entity 存 ID + as-of 查询拿 label”的方式（对齐 `docs/dev-plans/032-effective-date-day-granularity.md`）。
- **UI Select 的交互约定已具备**：下拉类字段统一走 options endpoint + 输入搜索，天然适配 DICT/ENTITY 两类数据源的分流（见 `docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`）。

### 10.2 关键缺口与高风险点（决定交付难度的部分）

1) **元数据与槽位映射的“不可变性”**  
如果允许把同一个 `field_key` 重新映射到另一个 `physical_col`（或复用旧槽位），历史数据会发生语义漂移：同一列里的旧值突然代表另一个字段含义。  
建议契约冻结：`(tenant_uuid, field_key) -> physical_col` 一经启用即不可改；字段“删除”只能停用，不允许复用槽位。

2) **ENTITY 存 ID 的引用完整性问题**  
若把多个不同实体的 ID 复用到同一个物理列（例如 `int_01` 既可能存职级也可能存职位），数据库无法建立稳定外键。  
可选路径：
- 保守方案：按实体类型划分槽位（例如 `rank_uuid_01` 只允许引用 ranks），从设计上避免“一列多义”。  
- 工程方案：允许复用槽位，但把引用校验下沉到 Kernel/Service（写入时强校验存在性/可见性），并通过测试/门禁保证 fail-closed。

3) **不能引入第二写入口（One Door）**  
即便引入宽表扩展列，也不能允许绕过事件提交直接更新 versions 表；否则会与“事件 SoT + 同事务同步投射”的不变量冲突。  
建议：扩展字段的写入仍通过既有 `submit_*_event(...)` 写入口；投射到 versions 时同事务写齐（对齐 026/030/029/031 的 One Door 原则）。

4) **动态 SQL 与安全/可观测性**  
元数据驱动“动态选择列/排序”很容易把 SQL 注入、不可审计查询带入系统。  
建议：所有可选列名必须来自服务端 allowlist（由 `physical_col` 映射得到），值参数全部走 prepared/参数化；并在服务端打点记录“使用了哪些字段/排序条件”，否则很难排障。

### 10.3 推荐落地路径（最小闭环，风险可控）

1. **先限定范围**：仅针对 OrgUnit 的少量扩展字段落地（2-5 个），并且只支持最常见类型（`text/uuid/int/bool/date`）。  
2. **落点优先选读模型**：扩展列优先落在 OrgUnit 的 versions（读模型）上，避免污染事件模型与跨模块契约；事件 payload 仅作为审计/回显的补充。  
3. **元数据先行**：先把 `tenant_field_config` 的生命周期规则写清（启用/停用、不可变映射、槽位耗尽策略），再进入代码实现，避免后期返工。  
4. **DICT/ENTITY 分流先做“读取闭环”**：先实现 options endpoint（DICT 读字典、ENTITY 读实体表 + as-of），再做写入与列表排序/筛选。  
5. **索引按证据逐步加**：只对“明确需要排序/筛选且频率高”的字段建立 `(tenant_uuid, ext_col)` 索引（必要时加 `WHERE ext_col IS NOT NULL`），避免一次性铺满导致维护负担。

### 10.4 DICT/ENTITY 的历史一致性（避免“改字典=改历史”）

- 业务上需要“历史报表可复现”时，DICT 的 label 变更不应隐式改变历史展示。这里至少要做一个明确选择：  
  - 要么把 DICT 也做成 effective-dated（Valid Time 参与查询）；  
  - 要么在写入时对关键 DICT 字段做 label 快照（读时优先展示快照）。  
- 对于 ENTITY（例如 JobCatalog 一类有效期主数据），推荐“存 ID + as-of join 拿 label”，天然符合历史一致性（动机可对齐 `docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md` 中的相关讨论）。

### 10.5 与仓库不变量/SSOT 的对齐点（实施前必须逐条确认）

- **Valid Time**：业务生效一律 day 粒度（`docs/dev-plans/032-effective-date-day-granularity.md`）。  
- **One Door**：写入必须走 Kernel `submit_*_event(...)`，同事务同步投射（对齐 `docs/archive/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`）。  
- **No Tx, No RLS**：访问 Greenfield 表必须显式事务 + 租户注入且 fail-closed（对齐 `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`）。  
- **UI Select**：options endpoint + 搜索（`docs/archive/dev-plans/018-astro-aha-ui-shell-for-hrms.md`）。  
- **门禁入口**：触发器与命令以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准（避免在本文件复制脚本细节）。
