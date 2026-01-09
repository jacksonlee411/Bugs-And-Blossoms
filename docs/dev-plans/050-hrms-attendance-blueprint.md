# DEV-PLAN-050：HRMS 考勤蓝图设计（Time & Attendance / SmartCore）

**状态**: 草拟中（2026-01-09 02:25 UTC）— 由 `docs/dev-records/HRMS考勤蓝图设计.docx` 转写为 Markdown，并完成首轮“仓库不变量”对齐；已进一步收敛“处理状态/可重算”方案（不在事件表上打标，改由读模水位线表达），并拆分为可执行切片计划（`DEV-PLAN-051`～`DEV-PLAN-056`）。

## 转写说明
- 源文档：`docs/dev-records/HRMS考勤蓝图设计.docx`
- 关联路线图：`docs/dev-plans/009-implementation-roadmap.md`

## 0. 范围与对齐（本仓库契约）

### 0.1 本文定位
- 本文是考勤域（Time & Attendance）的方向性蓝图，用于沉淀领域拆分、关键不变量、数据与计算链路的候选方案。
- 文末“引用的著作/第三方包/文章”仅作背景资料，不代表本仓库的依赖承诺；任何新增依赖需另立 dev-plan 评审并对齐门禁。

### 0.2 必须遵守的不变量（SSOT 引用）
- **模块边界与形态**：默认采用模块化单体（Modular Monolith）+ DDD 分层（`docs/dev-plans/015-ddd-layering-framework.md`、`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`），不在早期预设微服务拆分与独立运维。
- **One Door（写入口唯一）**：写入必须走 DB Kernel 的 `submit_*_event(...)`，事件 SoT + 同事务同步投射，避免出现第二写入口（`AGENTS.md`；同构方案见 `docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md` 等）。
- **No Tx, No RLS（fail-closed）**：访问 tenant-scoped 表必须显式事务 + 事务内注入 `app.current_tenant`（`SET LOCAL`/`set_config(..., true)`），policy 使用 `current_setting('app.current_tenant')::uuid`（`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`）。
- **时间语义（Valid Time vs Tx/Audit Time）**：有效期/生效期统一按 `date`（日粒度）表达；秒级时间戳仅用于业务事件时间与审计时间（`AGENTS.md`、`docs/dev-plans/032-effective-date-day-granularity.md`）。
- **授权与路由治理**：RLS 负责圈地、Casbin 负责授权；路由命名/分类/responder 必须对齐门禁（`docs/dev-plans/022-authz-casbin-toolchain.md`、`docs/dev-plans/017-routing-strategy.md`）。
- **工具链闭环**：DB 变更走 Atlas+Goose；查询走 sqlc；不引入未经批准的 ORM/路由框架（`AGENTS.md`、`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`）。
- **早期避免过度运维**：暂不引入 Sidecar/消息队列驱动“权威读模更新”等复杂链路（`AGENTS.md` §3.6）。

### 0.3 处理状态与可重算（选定）
- **不在 `t_time_event` 上维护 `is_processed` 等处理标记**：考勤结果天然可重算（补打卡/补排班/补假日/规则变更/追溯重算），且处理链路常为多阶段/多消费者；单 boolean 语义不足且容易误用。同时，`t_time_event` 为高吞吐分区大表，频繁 UPDATE 会带来膨胀与 VACUUM 压力，破坏“近似 append-only”的最佳形态。
- **幂等与去重**：用“稳定事件 ID/幂等键 + 约束/冲突处理”实现（必要时单独设计去重表以绕开分区表全局 unique 的限制），而不是靠“processed”标记。
- **计算水位线放在权威读模**：在“按人按日”的权威读模表中记录 `computed_at`、`ruleset_version`、`input_watermark_*`（例如已纳入计算的最大 `punch_time`/事件序号），读模由 Kernel 在 `submit_*_event(...)` 同事务维护（写后读强一致）。
- **异步副作用单独建账**：通知/外部回调/补偿重试等如需持久化进度，用独立的 `*_consumer_offsets`/outbox 表按消费者记录，不回写 `t_time_event`。

---

## 1. 执行摘要与战略背景

### 1.1 数字化转型下的劳动力管理新范式

在中国企业数字化转型的浪潮中，人力资源管理系统（HRMS）正经历从“记录型”向“智能型”的根本性转变。考勤模块（Time & Attendance）作为HRMS中数据吞吐量最大、计算逻辑最复杂、合规风险最高的子系统，其架构设计直接决定了整个SaaS平台的性能上限与用户体验。本项目（代号）作为纯粹的“Greenfield”（绿地）实施项目，旨在彻底摒弃传统单体架构的遗留债务，构建一套符合中国2025年复杂劳动法规、适配本土超级应用生态（钉钉/企微）、并具备金融级数据一致性的下一代考勤引擎——内部代号“智核”（SmartCore）。

本报告依据《HRMS考勤模块设计方案研究》及项目路线图（DEV-PLAN-009），结合全球标杆产品（SAP HCM、PeopleSoft）的最佳实践，提出了一份长达15,000字的深度实施蓝图。该方案充分利用 Go 语言的高并发优势与 PostgreSQL 17 的高级数据治理特性，旨在解决中国企业在“996”工时制、法定节假日调休、以及《个人信息保护法》（PIPL）严监管下的核心痛点。

### 1.2 项目的战略约束与原则

根据 项目实施路线图，考勤模块的设计必须严格遵循以下工程原则，这构成了本蓝图的技术基石：

- **单一入口与强制 RLS（One Door / No Tx, No RLS）**：所有对考勤数据的读写操作，必须通过统一的入口，并在事务（Transaction）上下文中强制注入租户上下文（Tenant Context），以触发 PostgreSQL 的行级安全策略（RLS）。这是实现多租户数据强隔离的底线要求 <sup>1</sup>。

- **事件溯源与同步投影（Transactional Event Sourcing）**：核心业务实体（如打卡记录、工时账户）必须采用事件溯源（Event Sourcing）作为唯一事实来源（Single Source of Truth, SSOT），并通过同步投影机制更新查询视图。这确保了在复杂的工时重算场景下，系统具备“时间旅行”般的可追溯性 <sup>1</sup>。

- **工程基线与防漂移（Engineering Baseline SSOT）**：数据库变更必须通过 Atlas 和 Goose 形成的闭环管理，代码生成严格依赖 sqlc，严禁引入未经批准的第三方 ORM 或路由框架，以防止技术栈漂移 <sup>1</sup>。

## 2. 中国考勤业务领域的深度解析与合规挑战（2025版）

在中国设计考勤系统，本质上是在应对世界上最复杂的非线性时间计算逻辑。与西方标准的“朝九晚五”不同，中国的考勤计算是一个由多重工时制度、动态节假日调整和严格隐私法规构成的三维矩阵。

### 2.1 三轨并行的工时制度与计算模型

系统必须在同一套计算引擎中，支持三种完全不同的工时计算逻辑，并能处理员工在不同制度间的动态流转。

#### 2.1.1 标准工时制：分层加班费率的精确计算

适用于大多数办公室职员。其核心难点在于2025年劳动法对加班费率的严格分层：

- **工作日延时（150%）**：基础逻辑，但需结合“容差（Tolerance）”处理。

- **休息日（200%）与“以休代薪”的博弈**：法律规定休息日加班“可以”安排补休，若不补休则支付工资。引擎必须支持“选择权”逻辑，即在生成考勤结果时，允许员工或审批流决定将该时段计入“调休余额（Lieu Quota）”还是“本月薪资（Payroll）”。

- **法定节假日（300%）的强制性**：这是合规红线。法律规定法定节假日加班“不得”以补休代替，必须支付300%工资。系统必须通过“公共假日日历服务”识别此类日期，并强制禁用“转调休”选项，直接生成薪资项 <sup>3</sup>。

#### 2.1.2 综合计算工时制：周期累加器与分桶策略

广泛应用于交通、物流及制造业。其计算逻辑从“日/周”维度跃升至“周期（月/季/年）”维度。

- **周期累加器（Cycle Accumulator）**：系统需维护一个跨日期的累加器对象。例如，实行“年综合工时”的企业，其年度标准工时阈值约为2000小时。引擎只有在累加值超过此阈值时，才触发150%的加班计算。

- **法定节假日的独立性（The Holiday Trap）**：这是系统设计的最大陷阱。即使在综合工时制下，若员工在法定节假日工作（如春节初一），该工时**不能**计入综合工时的普通累计池，必须**单独提取**（Split Bucket），立即按300%结算。SmartCore 引擎必须具备“工时分拆（Time Splitting）”能力，将单一物理班次拆解归入不同的逻辑账户 <sup>3</sup>。

#### 2.1.3 不定时工时制：合规的“静默”模式

适用于高管与外勤销售。系统的核心需求是“记录但不考评”。

- 系统必须记录打卡数据用于安全审计（如轨迹追踪、最后已知位置），但必须在计算层屏蔽“迟到/早退/旷工”异常的生成，避免因系统逻辑错误导致法律层面的“事实劳动合同”纠纷 <sup>3</sup>。

### 2.2 2025年法定节假日调整与“调休”逻辑

中国国务院发布的年度节假日安排包含独特的“调休（Swap）”机制，这直接击穿了传统的 DayOfWeek 算法。

- **逻辑日期覆盖（Logical Date Override）**：引擎不能简单判定“周日是休息日”。例如，2025年1月26日（周日）因春节调休被定义为“工作日”。若员工在此日工作，仅支付正常工资（或视为正常出勤），而非200%加班费。反之，某个周五可能因调休变为“休息日”。

- **动态参数化**：2025年全体公民放假日增加2天，导致月计薪天数（21.75天）的基数可能面临政策调整。系统必须将 Monthly_Paid_Days 设计为可配置的全局参数，而非硬编码常量 <sup>3</sup>。

### 2.3 PIPL 下的生物识别与隐私合规

随着《个人信息保护法》的实施，考勤系统处理人脸指纹数据面临严峻挑战。

- **数据最小化原则**：系统设计应避免存储原始人脸图像。建议仅存储**特征向量（Feature Vectors/Hashes）**。

- **被遗忘权（Right to be Forgotten）的工程实现**：当员工离职（状态变为 Terminated）时，系统必须触发自动化流程，物理删除或“加密粉碎（Crypto-shredding）”其生物特征数据，并生成不可篡改的 Data_Destruction_Log 以备监管审计 <sup>3</sup>。

## 3. 标杆竞品架构剖析与 SmartCore 设计哲学

为了设计超越现状的系统，我们需要深入解构 SAP HCM 和 PeopleSoft 的核心逻辑，取其精华，去其糟粕。

### 3.1 SAP HCM：严谨有余，实时不足

SAP 的时间评价（Time Evaluation）模块（Schema/PCR）是行业标杆。

- **优势**：极度灵活的规则脚本（PCR），通过 TIP（Time Input）、TOP（Time Output）表进行精细化的时间处理。

- **劣势**：架构陈旧，依赖夜间批处理作业（RPTIME00）。员工打卡后无法即时获得反馈，且 ABAP 脚本难以调试。

- **SmartCore 策略**：保留 SAP 的“时间类型（Time Type）”和“处理类（Processing Class）”概念，但用 **Go 语言的责任链模式（Chain of Responsibility）** 替换 ABAP 脚本，实现毫秒级的实时计算 <sup>3</sup>。

### 3.2 PeopleSoft：规则引擎的可视化先驱

PeopleSoft Global Payroll 引入了“元素（Element）”和“规则程序（Rule Program）”的概念。

- **优势**：规则的可视化配置能力强，支持通过 SQL 表达式定义逻辑。

- **SmartCore 策略**：借鉴其“规则组（Rule Group）”设计，将不同人群（如“上海研发中心”与“东莞工厂”）绑定到不同的 TimeProfile，实现差异化管理 <sup>6</sup>。

### 3.3 盖雅工场（GaiaWorks）：本土化与移动优先

作为本土 WFM 独角兽，盖雅展现了对中国移动互联网生态的极致适配。

- **优势**：深度集成钉钉/企微，支持复杂的排班与实时计算。

- **SmartCore 策略**：全盘吸纳其“移动端优先”的设计理念，将考勤异常处理、加班申请等流程完全嵌入 IM 工具中，而非强制用户登录 PC 端 <sup>3</sup>。

## 4. SmartCore 系统架构蓝图：基于 Go 与 DDD

SmartCore 以领域驱动设计（DDD）理念将考勤域拆分为多个界限上下文（Bounded Contexts）。在本仓库落地时默认采用**模块化单体**形态（对齐 `DEV-PLAN-015/016`），不预设“独立微服务部署”，以降低早期运维复杂度与边界漂移风险。

### 4.1 核心界限上下文划分

| **界限上下文 (Bounded Context)**   | **核心职责**                                                                         | **关键聚合根 (Aggregates)**                           | **技术模式**                          |
|------------------------------------|--------------------------------------------------------------------------------------|-------------------------------------------------------|---------------------------------------|
| **时间采集域 (Time Collection)**   | 负责从各种终端（钉钉、企微、生物考勤机）摄入原始打卡数据，进行清洗、去重和标准化。   | RawPunch (原始打卡), Device (考勤设备)                | 高吞吐写入、事件溯源 (Event Sourcing) |
| **时间评价域 (Time Evaluation)**   | 核心计算引擎。基于排班和规则，将原始打卡对（Pairs）转化为合规的工时结果（Results）。 | TimePair (打卡对), DailyResult (日结果), Shift (班次) | 责任链模式、内存计算                  |
| **额度与银行域 (Quota & Banking)** | 管理假期余额、调休池和综合工时累加器。                                               | LeaveAccount (假期账户), CompensatoryBucket (调休桶)  | 强一致性事务、乐观锁                  |
| **主数据配置域 (Master Data)**     | 定义时间档案、假日日历和计算规则参数。                                               | TimeProfile (时间档案), HolidayCalendar (假日日历)    | 读多写少、多级缓存                    |

### 4.2 "单一入口" (One Door) 数据流架构

为了严格遵循 项目的“No Tx, No RLS”原则，所有数据的流入必须经过统一的网关层。

1.  **摄入层 (Ingestion Layer)**：适配器模式。针对钉钉（Stream SDK）和企微（Polling/Callback）的不同协议，提供统一的 ProviderAdapter 接口，将外部 JSON 转换为内部 RawPunch 结构。

2.  **标准化服务 (Normalization Service)**：将外部状态码（如钉钉的 checkin_type: OnDuty）映射为内部枚举（PunchType: IN）。

3.  **事件存储核心（DB Kernel / One Door）**：在显式事务内注入 `app.current_tenant`，调用 DB Kernel 的 `submit_*_event(...)` 作为唯一写入口：写入事件 SoT，并在同一事务内同步投射更新权威读模表（写后读强一致）。

4.  **缓存/副作用（可选、非权威）**：Redis 可用于去重/限流计数/缓存预热等，但不作为权威读模；如需异步任务（例如通知/外部回调重试），不得直接写权威读模表，且需要单独计划评审以避免第二写入口。

## 5. 数据库设计：发挥 PostgreSQL 17 的极致性能

SmartCore 的数据库设计充分利用了 PG 17 的**分区表**、**JSONB** 和 **RLS** 特性，以应对海量数据与灵活规则的双重挑战。

### 5.1 核心表结构设计 (Schema Design)

#### 5.1.1 T_Time_Event (原始打卡流水表 - IT2011)

此表数据量巨大且只增不减，必须采用分区策略。

```sql
CREATE TABLE t_time_event (
  event_uuid UUID NOT NULL,
  tenant_id UUID NOT NULL, -- RLS 核心字段
  person_uuid UUID NOT NULL,
  punch_time TIMESTAMPTZ NOT NULL,
  source_provider VARCHAR(20) NOT NULL, -- 'DINGTALK', 'WECOM', 'IOT'
  source_raw_payload JSONB, -- 存储原始报文以备审计调试 [10]
  device_info JSONB, -- 存储 GPS, Wifi Mac, Device ID
  checkin_type VARCHAR(10), -- 'IN', 'OUT'
  created_at TIMESTAMPTZ DEFAULT NOW(),

  -- 联合主键包含分区键和 RLS 键
  PRIMARY KEY (tenant_id, punch_time, event_uuid)
) PARTITION BY RANGE (punch_time);

-- 按月自动分区，便于冷热数据分离和快速归档 [11]
CREATE TABLE t_time_event_y2025m01 PARTITION OF t_time_event
FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
```

- **设计洞察**：按月分区不仅提升查询性能，更重要的是允许通过 DROP TABLE 秒级删除过期数据，避免大表 DELETE 带来的 VACUUM 风暴。将 tenant_id 纳入主键是 RLS 性能优化的关键，确保查询优化器能利用分区剪枝 <sup>12</sup>。

##### 5.1.1A 处理状态不打标：用读模水位线表达（选定）

本项目采用 “One Door + 同事务同步投射” 口径，因此“事件写入成功”应等价于“权威读模已更新”。对于考勤这种高频、可重算的域，不建议在 `t_time_event` 上通过 `is_processed` 维护处理进度；推荐把“计算版本/输入水位线”记录在权威读模中，以支撑追溯重算与一致性验收。

- **权威读模（示例）**：以“租户 + 人员 + 工作日（`date`）”为主键，存储当日出勤/工时/异常等结果；并记录“本次结果使用的规则版本”和“已纳入计算的输入水位线”。
  ```sql
  CREATE TABLE t_daily_attendance_result (
    tenant_id UUID NOT NULL,
    person_uuid UUID NOT NULL,
    work_date DATE NOT NULL, -- Valid Time（日粒度）

    -- 结果字段（示意）
    status TEXT NOT NULL, -- PRESENT/ABSENT/EXCEPTION...
    worked_minutes INT NOT NULL DEFAULT 0,

    -- 版本与水位线（用于可重算与一致性）
    ruleset_version TEXT NOT NULL,
    input_max_punch_time TIMESTAMPTZ, -- 当次计算纳入的最大打卡时间（示意）
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, person_uuid, work_date)
  );
  ```
- **幂等/去重的现实约束**：若需要对 `source_provider + source_event_id` 做全局唯一，需注意 PostgreSQL 分区表 unique 约束必须包含分区键；可选方案包括（1）把分区键纳入幂等键；（2）单独维护一个非分区的去重/幂等表（仅存键与指向 `event_uuid`），在 `submit_*_event(...)` 同事务内完成校验与写入。
- **身份锚点**：上例中的 `person_uuid` 与人员身份锚点对齐（见 `docs/dev-plans/027-person-minimal-identity-for-staffing.md`），避免在写路径中使用 pernr/外部 userid 作为权威主键。

#### 5.1.2 T_Time_Profile (规则配置表 - JSONB)

考勤规则千变万化，使用传统 EAV 模型会导致表结构极其复杂。我们利用 PG 的 JSONB 存储规则参数，实现“Schema-less”的灵活性，同时保持强类型的查询能力。

```sql
CREATE TABLE t_time_profile (
  profile_id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  profile_name VARCHAR(100),

  -- 灵活规则配置，取代数百个配置字段 [10]
  rules_config JSONB NOT NULL DEFAULT '{}'::jsonb,
  -- 示例内容:
  -- {
  --   "tolerance": {"late_minutes": 5, "early_leave_minutes": 3},
  --   "overtime": {"rounding": "DOWN_30_MIN", "min_threshold": 60},
  --   "deduction": {"tier_1_threshold": 30, "tier_1_amount": 50},
  --   "holiday_logic": {"enable_swap": true}
  -- }

  CONSTRAINT fk_tenant FOREIGN KEY (tenant_id) REFERENCES t_tenants(id)
);

-- 创建 GIN 索引以加速规则查询
CREATE INDEX idx_rules_config ON t_time_profile USING GIN (rules_config);
```

- **设计洞察**：JSONB 的目标是减少“频繁加列/改列”的结构性变更，但不代表绕过 Atlas/Goose 闭环；规则参数的新增/变更仍属于契约变更，应在对应 dev-plan 中声明，并通过迁移/校验/默认值策略闭环。

#### 5.1.3 T_Accumulator (综合工时累加器)

用于存储综合工时制下的各项“桶”数据。

```sql
CREATE TABLE t_accumulator (
  acc_uuid UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  person_uuid UUID NOT NULL,
  cycle_id VARCHAR(20), -- e.g., '2025-Q1'

  -- 高精度工时桶
  standard_hours NUMERIC(10, 2) DEFAULT 0,
  holiday_ot_hours NUMERIC(10, 2) DEFAULT 0, -- 300% 桶，独立累加
  weekend_ot_hours NUMERIC(10, 2) DEFAULT 0, -- 200% 桶

  last_updated TIMESTAMPTZ,
  version INT -- 乐观锁版本号
);
```

### 5.2 行级安全（RLS）的强制实施

根据 文档的“No Tx, No RLS”原则，必须在数据库层面强制实施隔离。

```sql
-- 启用 RLS
ALTER TABLE t_time_event ENABLE ROW LEVEL SECURITY;

-- 创建基于当前租户上下文的策略
CREATE POLICY tenant_isolation_policy ON t_time_event
USING (tenant_id = current_setting('app.current_tenant')::uuid);
```

- **性能优化策略**：在高并发写入场景，应保证“每请求一事务”，并在事务开始时只注入一次租户上下文（`SET LOCAL app.current_tenant = ...` 或 `SELECT set_config('app.current_tenant', ..., true)`）。不要使用 Session 级变量（尤其在 PgBouncer transaction pooling 等场景下容易导致租户上下文泄漏）。如需减少 RTT，可用 pgx pipeline 将租户注入与后续 SQL 批量发送，但仍必须在同一事务内完成。

## 6. SmartCore 计算引擎：Go 语言的高性能实现

SmartCore 引擎的设计目标是替代 SAP 笨重的 Schema/PCR 逻辑。利用 Go 的静态类型、接口多态和 Goroutine 并发能力，构建一个可测试、可扩展的实时计算管道。

### 6.1 管道架构与责任链模式 (Chain of Responsibility)

计算过程被抽象为一个 Context 在一系列 Handler 中流转的过程。

```go
// 核心上下文结构
type Context struct {
	TenantID   uuid.UUID
	EmployeeID uuid.UUID
	Date       civil.Date // 使用 Google Civil Time 库处理日期，避免时区陷阱
	Punches    RawPunch
	Pairs      TimePair
	DailyResult *Result
	Rules      *RuleSet // 从 JSONB 解析出的强类型规则对象
}

// 处理器接口
type Evaluator interface {
	Evaluate(ctx *Context) error
}

// 引擎核心逻辑
func (e *Engine) Run(ctx *Context) {
	// 定义责任链
	handlers := []Evaluator{
		&PairMatchingHandler{},    // 步骤1: 原始打卡配对 (IN/OUT)
		&ShiftIdentification{},    // 步骤2: 班次识别 (固定/弹性/排班)
		&HolidayOverrideHandler{}, // 步骤3: 2025节假日与调休逻辑覆盖
		&ToleranceHandler{},       // 步骤4: 应用容差 (迟到5分钟豁免)
		&OvertimeCalculator{},     // 步骤5: 加班计算 (1.5/2.0/3.0 分桶)
		&ComplianceChecker{},      // 步骤6: 合规校验 (11小时休息规则)
		&PersistenceHandler{},     // 步骤7: 持久化结果与同步投影
	}

	for _, h := range handlers {
		if err := h.Evaluate(ctx); err != nil {
			// 错误处理与日志记录
			e.Logger.Error("evaluation failed", "step", reflect.TypeOf(h), "err", err)
			break
		}
	}
}
```

### 6.2 核心算法：打卡配对与“跨天”处理

这是考勤计算中最棘手的问题。员工可能多次打卡、漏打卡或跨午夜打卡 <sup>16</sup>。

**SmartCore 配对算法逻辑：**

1.  **数据准备**：加载目标日 T 及次日 T+1（至 MaxShiftLength 时间点，如次日 14:00）的所有原始打卡。

2.  **排序与清洗**：按时间戳排序，剔除极短时间内的重复打卡（去抖动）。

3.  **启发式配对**：

    - 遍历打卡流，寻找 IN 类型点。

    - 对于每个 IN，在随后的时间窗口内寻找最近的 OUT。

    - **跨天处理**：若班次定义允许跨天（如 22:00 - 06:00），算法会在 T+1 的数据桶中搜索 OUT 点。若找到，将该对标记为 IsCrossDay=true，并将逻辑工作时间归属到 T 日。

    - **异常标记**：若 IN 后紧跟另一个 IN，则前一个标记为 MISSING_OUT 异常。

### 6.3 规则引擎的选型：参数化策略 vs. 动态脚本

虽然存在 GoRules (ZEN Engine) 等优秀的开源规则引擎 <sup>18</sup>，但考虑到考勤规则的**有限枚举性**（只有迟到、早退、缺勤、加班等有限状态）和**高性能要求**，SmartCore 采用**参数化策略模式（Parameterized Strategy Pattern）**。

- **实现方式**：核心逻辑（如加班计算）是编译好的 Go 代码（Strategy）。t_time_profile 中的 JSON 配置仅作为参数注入（Parameters）。例如，ToleranceHandler 读取 rules.tolerance.late_minutes (如 5) 来决定是否触发迟到逻辑，而不是在运行时解释执行一段脚本。

- **优势**：性能比解释型脚本快 1-2 个数量级，且类型安全，易于单元测试。

- **扩展性**：优先通过“有限枚举 + 参数化策略”扩展规则能力，不引入通用规则引擎/动态脚本/表达式解释器；如确需表达式配置，必须另立 dev-plan 评审（含依赖引入、性能、安全与门禁口径），避免实现期漂移。

## 7. 集成生态：连接钉钉与企业微信的实战策略

连接中国本土的“超级应用”需要处理其特有的协议与限制。

### 7.1 适配器架构 (Adapter Pattern)

定义通用的 AttendanceProvider 接口，屏蔽底层差异。

```go
type AttendanceProvider interface {
	// 拉取历史打卡（用于补数或初始化）
	FetchPunches(ctx context.Context, from, to time.Time) (RawPunch, error)
	// 建立实时流连接（用于实时计算）
	SubscribeToStream(ctx context.Context, ch chan<- RawPunch) error
}
```

### 7.2 钉钉集成：全面拥抱 Stream 模式

传统的 Webhook 模式在企业内网穿透和稳定性上存在劣势。钉钉推出的 **Stream 模式**（基于 WebSocket）是最佳选择 <sup>20</sup>。

- **SDK 选型**：使用官方 github.com/open-dingtalk/dingtalk-stream-sdk-go。

- **架构优势**：

  - **低延迟**：打卡数据毫秒级推送，无需轮询。

  - **内网友好**：仅需出站连接（Outbound），无需配置防火墙入站白名单或公网 IP。

- **数据清洗陷阱**：钉钉推送的 payload 中包含 timeResult（正常/迟到/早退）。**SmartCore 必须忽略此字段**。因为钉钉的内置计算逻辑无法涵盖复杂的 2025 节假日调休或特殊的综合工时规则。我们只取 timestamp 和 userId，完全依赖 SmartCore 进行重算 <sup>3</sup>。

### 7.3 企业微信集成：轮询与回调的混合策略

企微的 API 开放程度略有不同，目前打卡数据主要依赖**拉取（Polling）** <sup>21</sup>。

- **策略**：

  - **主链路**：实现一个高可用的 Poller 服务，按 opencheckin 数据类型，每隔 N 秒（如 30s）批量拉取增量数据。

  - **限流治理**：企微 API 频率限制严格。Go 服务必须实现**令牌桶（Token Bucket）**限流算法，并在 Redis 中维护全局限流计数器，防止多实例并发导致触发 429 Too Many Requests。

- **身份映射（Identity Mapping）**：企微使用 userid，钉钉使用 unionid。HRMS 使用内部 UUID。必须建立 T_Identity_Map 表，在数据存入 Event Store 之前完成 ID 转换 <sup>1</sup>。

## 8. 高性能与高并发设计

面对早高峰（8:55 - 9:05）每秒数万次的打卡请求，系统必须具备削峰填谷的能力。

### 8.1 写入优化：COPY 协议与 UNNEST

- **批量写入**：对于来自 Poller 的批量数据，拒绝逐条 INSERT。使用 pgx.CopyFrom 接口，利用 PostgreSQL 的 **COPY 协议**，性能比普通 INSERT 提升 30 倍以上 <sup>8</sup>。

- **实时流优化**：对于 Stream 模式的单条流数据，使用 Go 的 channel 进行微批次缓冲（Micro-batching），每 100ms 或 满 50 条 刷入数据库。在 SQL 层面，使用 UNNEST 模式一次性解构数组插入多行，减少网络往返（RTT） <sup>22</sup>。

### 8.2 读优化：同步投射与读模表

- **同步投射（权威）**：根据 路线图 <sup>1</sup>，核心读模（如 `t_daily_attendance_result`）应在事务内同步更新，以保证写后读强一致。

- **读模与聚合（优先简单）**：“团队考勤看板”等聚合查询优先采用专用读模/聚合表（由 Kernel 同事务维护）+ 索引策略实现；暂不引入 Sidecar + NOTIFY/LISTEN + 物化视图刷新链路。

- **未来可选项**：若确需物化视图/异步刷新/更多缓存层，必须另立 dev-plan，评估运维复杂度与一致性边界。

## 9. 实施路线图与阶段规划（对齐 DEV-PLAN-009 的颗粒度）

> 本节只定义“阶段与出口条件”，不复制门禁命令细节；门禁入口以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。

### 9.1 前置条件（对齐 DEV-PLAN-009 Phase 0-3）

> 说明：`DEV-PLAN-009` 已完成，但“新增业务域/模块”仍必须复用其门禁与出口条件，不得在新域里重新发明第二套工作流。

- UI：AHA Shell + en/zh i18n + 路由治理已就绪（`DEV-PLAN-018/020/017/012`）。
- 平台：Tenancy/AuthN + RLS（fail-closed）+ Authz（Casbin）已就绪（`DEV-PLAN-019/021/022/023`）。
- 工具链：Atlas+Goose 闭环、sqlc 规范与生成物门禁已就绪（`DEV-PLAN-024/025/012`）。
- 模块边界：需明确“考勤能力落点”（建议先落在 `modules/staffing` 内的子域，避免过早扩展模块数量）；若确需新增模块或变更模块边界，必须先更新 `DEV-PLAN-016` 并通过架构门禁（`DEV-PLAN-015/016`）。

### 9.2 Phase 4：考勤业务垂直切片（业务 + UI 同步交付）

> 每个切片的 done 定义（对齐 `DEV-PLAN-009`）：Kernel 写入口（`submit_*_event`）+ 同事务读模 + UI 可操作链路 + RLS/Authz/routing 门禁全绿。

#### Slice 4A：打卡流水闭环（手工/导入，先可见）
> 对应：`docs/dev-plans/051-attendance-slice-4a-punch-ledger.md`
- 目标：在不接入外部平台的前提下，完成“写入→列表/详情可见”的最小闭环，作为后续重算与合规的输入底座。
- 交付物：
  - DB Kernel：定义“打卡事件”的唯一写入口（命名待定，但必须是 `submit_*_event(...)` 形态），并同步投射出可查询的流水读模。
  - UI：考勤入口页；支持选择人员 + 日期范围查看流水；支持手工补打卡（IN/OUT）作为端到端验收入口。
  - 验收：RLS 未注入时 fail-closed；跨租户不可见；Authz 拒绝未授权写入。

#### Slice 4B：日结果计算闭环（标准班次）
> 对应：`docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`
- 目标：把流水变成可解释的“工作日结果”（`date` 粒度），并支持稳定重算。
- 交付物：
  - 读模：以 `t_daily_attendance_result` 为代表的“按人按日”权威读模（含 `ruleset_version + input_watermark`），由 Kernel 同事务维护。
  - 计算：最小配对（含跨天）+ 基本异常（缺勤/迟到/早退）标记；对齐 0.3 的“处理状态不打标”策略。
  - UI：日结果列表/详情；展示异常原因与构成（来自读模，而非临时计算）。

#### Slice 4C：主数据与合规闭环（时间档案/假日日历）
> 对应：`docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`
- 目标：把可变的规则/日历显式化，并通过有效期（`date`）管理，支撑追溯重算与合规解释。
- 交付物：
  - 主数据：TimeProfile/Shift/HolidayCalendar（均 effective-dated，`date` 粒度）。
  - 逻辑：调休覆盖、法定假日识别、加班分桶（1.5/2.0/3.0）；保持“参数化策略”，不引入通用脚本/规则引擎。
  - UI：最小主数据配置页（新增/启用/禁用/查看生效期），并能驱动日结果变化。

#### Slice 4D：额度与银行闭环（调休/综合工时累加器）
> 对应：`docs/dev-plans/054-attendance-slice-4d-time-banking-and-accumulators.md`
- 目标：把“以休代薪/调休余额/综合工时累加器”等结果显式化为权威读模，并与日结果同事务一致更新，支撑后续 payroll/结算衔接。
- 交付物：
  - 读模：以 `t_accumulator` 等为代表的额度/累加器读模；其更新必须由 Kernel 同事务维护（避免第二写入口）。
  - 逻辑：在日结果计算中同步维护“调休余额/各类加班桶/综合工时周期累计”等派生状态；并支持追溯重算（更正事件会回滚并重算额度）。
  - UI：最小余额/累加器可视化（按人员/周期查看），并能解释“余额从何而来”（关联到当日结果）。
- 备注：与 Payroll 的正式结算接口/工资项映射不在本计划内，需与 Payroll 系列计划（`DEV-PLAN-039` 及后续）对齐后再落地。

#### Slice 4E：纠错与审计闭环（更正事件 + 重算）
> 对应：`docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`
- 目标：覆盖考勤的高频现实：补卡/更正/撤销/申诉导致重算，并且需要可追溯与可对账。
- 交付物：
  - 事件类型：更正/作废/覆盖结果等事件统一走 `submit_*_event(...)`；明确重算边界（以 `work_date` 为主要定界）。
  - 读模：同一 `work_date` 的结果可被重算覆盖，但事件序列可追溯；UI 可展示该日事件链与最后计算时间。
  - UI：在日结果页发起一次更正并触发重算（至少一条链路）作为验收入口。

#### Slice 4F：生态集成闭环（钉钉/企微，最后接真实数据）
> 对应：`docs/dev-plans/056-attendance-slice-4f-dingtalk-wecom-integration.md`
- 目标：在内核/读模稳定后接入外部来源，避免“平台差异污染内核”，并确保外部数据与手工数据同口径可重算。
- 交付物：
  - 接入：DingTalk Stream / WeCom Poller 的适配层；身份映射配置（外部 userId/unionId → `person_uuid`）。
  - 验收：外部事件进入后与手工事件同口径；外部字段不作为权威计算输入（仅作为原始报文留存）。
- 备注：限流/重试/回执等属于副作用链路，需明确“不写权威读模”。

### 9.3 Phase 5：质量收口（对齐 DEV-PLAN-009 Phase 5）

- 覆盖：最小 E2E（从补打卡到日结果）、RLS/Authz 的负例测试、生成物漂移检查、routing/authz 门禁稳定产出结论。
- 性能：先建立可重复的小规模基线（避免过早追求 10k TPS 指标），并把压测与诊断入口固化为可复现流程。
- 合规：PIPL 相关（敏感数据最小化、删除/加密粉碎、审计日志）建议拆为独立切片或另立 dev-plan 后落地，避免在早期阶段引入过度运维。

## 10. 结论

本设计蓝图并未止步于构建一个简单的“考勤记录工具”，而是致力于打造一个**金融级的劳动力资产管理引擎**。通过严格遵循 项目的“Greenfield”技术规范，结合 Go 语言的高并发特性与 PostgreSQL 17 的数据治理能力，SmartCore 不仅能从容应对中国市场 2025 年的复杂合规要求，更为企业提供了一个透明、实时、可信的数据底座。

这套架构将数据的所有权和计算规则的定义权从第三方平台（钉钉/企微）收回至企业内部，确保了在多变的商业与法规环境中，企业始终掌握着劳动力管理的主动权。

**附录：关键数据结构定义 (Go)**

```go
// TimePair: 考勤计算的原子单位
type TimePair struct {
	PairID  uuid.UUID
	Date    civil.Date // 使用 Google Civil 库保证日期处理的安全性
	InEvent *RawPunch
	OutEvent *RawPunch
	ShiftID uuid.UUID

	// 计算字段
	Duration  time.Duration
	IsCrossDay bool   // 是否跨天
	Tags      string // "Late", "Early", "NoShow"
}

// RuleResult: 财务维度的计算结果
type RuleResult struct {
	Category   string // "Standard", "OT_1.5", "OT_2.0", "OT_3.0"
	Hours      decimal.Decimal
	CostCenter string
}
```

#### 引用的著作

1.  009-implementation-roadmap.md

2.  eventsourcing package - github.com/thefabric-io/eventsourcing - Go Packages, 访问时间为 一月 9, 2026， [<u>https://pkg.go.dev/github.com/thefabric-io/eventsourcing</u>](https://pkg.go.dev/github.com/thefabric-io/eventsourcing)

3.  HRMS考勤模块设计方案研究

4.  Your Guide to the 2025 Holiday Calendar in China - Oreate AI Blog, 访问时间为 一月 9, 2026， [<u>https://www.oreateai.com/blog/your-guide-to-the-2025-holiday-calendar-in-china/f01508a5dc8f1c61559baac220234fa3</u>](https://www.oreateai.com/blog/your-guide-to-the-2025-holiday-calendar-in-china/f01508a5dc8f1c61559baac220234fa3)

5.  Master the Art of Writing Time PCRs - SAP Community, 访问时间为 一月 9, 2026， [<u>https://community.sap.com/t5/enterprise-resource-planning-blog-posts-by-members/master-the-art-of-writing-time-pcrs/ba-p/13291279</u>](https://community.sap.com/t5/enterprise-resource-planning-blog-posts-by-members/master-the-art-of-writing-time-pcrs/ba-p/13291279)

6.  Documentation: 18: 8.14. JSON Types - PostgreSQL, 访问时间为 一月 9, 2026， [<u>https://www.postgresql.org/docs/current/datatype-json.html</u>](https://www.postgresql.org/docs/current/datatype-json.html)

7.  dingtalk package - github.com/hopeio/utils/sdk/dingtalk - Go Packages, 访问时间为 一月 9, 2026， [<u>https://pkg.go.dev/github.com/hopeio/utils/sdk/dingtalk</u>](https://pkg.go.dev/github.com/hopeio/utils/sdk/dingtalk)

8.  The fastest Postgres inserts - Hatchet Documentation, 访问时间为 一月 9, 2026， [<u>https://docs.hatchet.run/blog/fastest-postgres-inserts</u>](https://docs.hatchet.run/blog/fastest-postgres-inserts)

9.  Golang for High-Performance Real-Time Analytics: From WebSockets to Kafka Explained, 访问时间为 一月 9, 2026， [<u>https://medium.com/@jealousgx/golang-for-high-performance-real-time-analytics-from-websockets-to-kafka-explained-5cd7eb824484</u>](https://medium.com/@jealousgx/golang-for-high-performance-real-time-analytics-from-websockets-to-kafka-explained-5cd7eb824484)

10. Documentation: 18: 5.9. Row Security Policies - PostgreSQL, 访问时间为 一月 9, 2026， [<u>https://www.postgresql.org/docs/current/ddl-rowsecurity.html</u>](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)

11. Performance tips for partitioned tables : r/PostgreSQL - Reddit, 访问时间为 一月 9, 2026， [<u>https://www.reddit.com/r/PostgreSQL/comments/1or4yid/performance_tips_for_partitioned_tables/</u>](https://www.reddit.com/r/PostgreSQL/comments/1or4yid/performance_tips_for_partitioned_tables/)

12. Exploring Row Level Security In PostgreSQL - pgDash, 访问时间为 一月 9, 2026， [<u>https://pgdash.io/blog/exploring-row-level-security-in-postgres.html</u>](https://pgdash.io/blog/exploring-row-level-security-in-postgres.html)

13. Faster Is Not Always Better: Choosing the Right PostgreSQL Insert Strategy in Python (+Benchmarks) \| Towards Data Science, 访问时间为 一月 9, 2026， [<u>https://towardsdatascience.com/faster-is-not-always-better-choosing-the-right-postgresql-insert-strategy-in-python-benchmarks/</u>](https://towardsdatascience.com/faster-is-not-always-better-choosing-the-right-postgresql-insert-strategy-in-python-benchmarks/)

14. c# - Attendance punch/shift pairing algorithm - Stack Overflow, 访问时间为 一月 9, 2026， [<u>https://stackoverflow.com/questions/48825472/attendance-punch-shift-pairing-algorithm</u>](https://stackoverflow.com/questions/48825472/attendance-punch-shift-pairing-algorithm)

15. Matching user time logs to day shift in an attendance system - Stack Overflow, 访问时间为 一月 9, 2026， [<u>https://stackoverflow.com/questions/42921782/matching-user-time-logs-to-day-shift-in-an-attendance-system</u>](https://stackoverflow.com/questions/42921782/matching-user-time-logs-to-day-shift-in-an-attendance-system)

16. GoRules: Open-source Business Rules Engine, 访问时间为 一月 9, 2026， [<u>https://gorules.io/</u>](https://gorules.io/)

17. logical Expression evaluate with variables - golang - Reddit, 访问时间为 一月 9, 2026， [<u>https://www.reddit.com/r/golang/comments/1arewg6/logical_expression_evaluate_with_variables/</u>](https://www.reddit.com/r/golang/comments/1arewg6/logical_expression_evaluate_with_variables/)

18. open-dingtalk/dingtalk-stream-sdk-go: Go SDK for DingTalk Stream Mode API, Compared with the webhook mode, it is easier to access the DingTalk chatbot - GitHub, 访问时间为 一月 9, 2026， [<u>https://github.com/open-dingtalk/dingtalk-stream-sdk-go</u>](https://github.com/open-dingtalk/dingtalk-stream-sdk-go)

19. wecom package - github.com/eryajf/glactl/api/wecom - Go Packages, 访问时间为 一月 9, 2026， [<u>https://pkg.go.dev/github.com/eryajf/glactl/api/wecom</u>](https://pkg.go.dev/github.com/eryajf/glactl/api/wecom)

20. Boosting Postgres INSERT Performance by 2x With UNNEST - Tiger Data, 访问时间为 一月 9, 2026， [<u>https://www.tigerdata.com/blog/boosting-postgres-insert-performance</u>](https://www.tigerdata.com/blog/boosting-postgres-insert-performance)
