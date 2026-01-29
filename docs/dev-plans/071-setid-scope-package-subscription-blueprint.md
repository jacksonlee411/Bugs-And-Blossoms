# DEV-PLAN-071：SetID Scope Package 订阅蓝图

**状态**: 草拟中（2026-01-29 05:52 UTC）

## 1. 背景与问题
- `DEV-PLAN-070` 将 SetID 作为配置主数据的显式入口，但同一 SetID 只能指向一套“全域配置组合”，难以表达“同一 SetID 在不同配置域选择不同方案”的需求。
- 为避免“新增 SetID 仅用于组合差异”的膨胀，需要引入可复用、可版本化的配置方案层。

## 2. 目标与非目标
### 2.1 目标
- 保留“租户内全局唯一 SetID”的既有口径。
- 引入 `scope_code`（配置域标识）与 `scope_package`（域内配置方案）。
- 支持 `setid -> scope_code -> scope_package` 订阅关系，并以 `as_of_date` 生效。
- 解析链路 fail-closed：缺失订阅即拒绝读取/写入配置。
- 继续满足 One Door、No Tx/No RLS、Valid Time（date）等不变量。

### 2.2 非目标
- 不做跨域运行时合并/覆盖规则。
- 不引入“旧 SetID 直连配置表”的兼容分支。
- 不改变 `DEV-PLAN-070` 的“org_unit 解析 SetID”主流程。
- 不定义共享层包的混合读取规则（如需共享包，另起计划）。

## 3. 术语与边界
- **scope_code**：配置域标识（稳定枚举），例如 `location`、`education_type`、`jobcatalog`。
- **scope_package**：某个配置域的一套完整方案（可复用/可版本化）。
- **package_code**：包编码（稳定标识），保留字 `DEFLT` 表示默认包。
- **订阅关系**：`(tenant_id, setid, scope_code) -> package_id`，具备有效期。

### 3.1 scope_code 划分原则（强制）
- 配置域优先：以“可独立选择的一套配置方案”为边界，不以单表/字段为边界。
- 强耦合同域：存在硬不变量或强引用关系的配置必须归同一 scope（避免跨域语义混合）。
- 生命周期独立才拆分：维护责任、变更频率、审计口径可独立时才允许拆分 scope。
- 单一归属：每个 scope_code 只能归属一个模块，归属模块对其配置数据负全责。
- 共享边界明确：仅白名单 scope 允许订阅共享包，其他默认 tenant-only。
- 唯一映射：任一配置数据只能映射到一个 scope_code，禁止多套权威表达。
- 前缀规则：scope_code 前缀仅表达归属模块，不表达共享/开通状态；共享属性通过规则与列标注。

### 3.2 首批 scope_code 清单（建议）
| scope_code | 归属模块 | 范围说明 | 共享层可见 |
| --- | --- | --- | --- |
| `jobcatalog` | `jobcatalog` | 职位目录主数据（Job Family Group/Family/Level/Profile） | 否 |
| `orgunit_geo_admin` | `orgunit` | 国家/省市等行政区划（共享白名单） | 是 |
| `orgunit_location` | `orgunit` | 地点与地点类型（共享白名单） | 是 |
| `person_school` | `person` | 学校（共享白名单） | 是 |
| `person_education_type` | `person` | 学历类型（共享白名单） | 是 |
| `person_credential_type` | `person` | 资格证书/证照类型（共享白名单） | 是 |

### 3.3 归属规则
- 单一归属：每个 `scope_code` 必须明确唯一归属模块，归属模块负责 schema、kernel 写入口、管理 UI 与测试门禁。
- 命名规则：`scope_code` 使用 `lower_snake_case`；跨域或通用参考数据应以归属模块前缀命名，避免歧义。
- 共享层边界：仅标注“共享白名单”的 `scope_code` 允许读取 `SHARE`，其余默认 tenant-only。
- 跨模块使用：非归属模块只能通过共享的解析/读接口使用该 `scope_code`，禁止私自定义重复的配置域。
- 新增流程：新增或调整 `scope_code` 必须更新本清单并补齐门禁与验收项。
- 归属拆分：治理元数据（scope/包/订阅）归 `orgunit`，业务配置数据归各自模块；`orgunit` 仅提供解析入口，不承载业务配置表。

### 3.4 配置数据登记清单（当前已知）
| scope_code | 归属模块 | 配置表/入口 | 包字段 | 共享层 |
| --- | --- | --- | --- | --- |
| `jobcatalog` | `jobcatalog` | DB: `jobcatalog.job_family_groups/job_families/job_levels/job_profiles`（含 `*_events/*_versions`、`job_profile_version_job_families`）；UI: `/org/job-catalog` | `package_id` | 否 |
| `orgunit_geo_admin` | `orgunit` | 未落地（共享白名单，表/入口待定义） | `package_id` | 是 |
| `orgunit_location` | `orgunit` | 未落地（共享白名单，表/入口待定义） | `package_id` | 是 |
| `person_school` | `person` | 未落地（共享白名单，表/入口待定义） | `package_id` | 是 |
| `person_education_type` | `person` | 未落地（共享白名单，表/入口待定义） | `package_id` | 是 |
| `person_credential_type` | `person` | 未落地（共享白名单，表/入口待定义） | `package_id` | 是 |

### 3.5 避免遗漏的流程约束
- 新增配置数据必须在对应 dev-plan 中声明其 `scope_code` 并更新本清单。
- 新增配置表若引入 `package_id`，必须补齐清单项与解析入口说明。
- PR 评审必须检查：是否新增/调整配置数据、是否需要包选择、是否已登记 scope_code。

## 4. 架构概览（高阶）
1. `ResolveSetID(tenant_id, org_unit_id, as_of_date)` 仍由 `DEV-PLAN-070` 定义。
2. 新增 `ResolveScopePackage(tenant_id, setid, scope_code, as_of_date)`。
3. 配置数据读取/写入使用 `package_id`（或 `scope_code + package_id`），不再直接用 `setid` 过滤。

### 4.1 归属与调用链（示例）
- 治理元数据（`orgunit`）：维护 `setid_scope_packages` / `setid_scope_subscriptions`，提供 `orgunit.resolve_scope_package(...)`。
- 业务模块（示例 `jobcatalog`）：写入口先解析 `package_id`，再把 `package_id` 写入 `jobcatalog` 配置表。
- 跨模块调用：通过 `pkg/**` 封装解析函数，禁止 Go 代码跨模块 import。

## 5. 数据模型（概念蓝图）
> 具体建表需用户确认（对齐 `AGENTS.md` 新增表确认要求）。
- `orgunit.setid_scope_packages`
  - `tenant_id`, `scope_code`, `package_id`, `package_code`, `name`, `status`, `created_at`, `updated_at`
  - 约束：`tenant_id + scope_code + package_code` 唯一，`package_code` 仅在其 `scope_code` 内唯一
- `orgunit.global_setid_scope_packages`
  - `tenant_id`（固定为 `orgunit.global_tenant_id()`）, `scope_code`, `package_id`, `package_code`, `name`, `status`, `created_at`, `updated_at`
  - 约束：`tenant_id = orgunit.global_tenant_id()`；`tenant_id + scope_code + package_code` 唯一
- `orgunit.setid_scope_package_events`
  - 包创建/停用事件（One Door）
- `orgunit.global_setid_scope_package_events`
  - 共享包创建/停用事件（仅 SaaS）
- `orgunit.setid_scope_subscriptions`
  - `tenant_id`, `setid`, `scope_code`, `package_id`, `package_owner_tenant_id`, `validity`, `last_event_id`
  - 约束：同一 `tenant_id + setid + scope_code` 有效期不得重叠；`package_owner_tenant_id` 仅允许为 `tenant_id` 或 `orgunit.global_tenant_id()`
- `orgunit.setid_scope_subscription_events`
  - 订阅变更事件（One Door）

## 6. 核心规则与不变量
- 单一有效订阅：同一 `tenant_id + setid + scope_code` 在同一日期仅允许一个有效 `package_id`。
- 包归属明确：`package_id` 只能绑定一个 `scope_code`，禁止跨域复用。
- 包编码唯一性：`package_code` 在同一 `tenant_id + scope_code` 内唯一。
- 配置数据 `code` 唯一性：同一 `tenant_id + scope_code + package_id` 内唯一；不同 package 允许同码。
- 解析 fail-closed：缺少订阅或订阅无效即拒绝。
- 订阅与包均需走事件入口，禁止直接写表。
- 订阅有效期与 `as_of_date` 对齐（Valid Time 语义）。
- 默认包规则：每个 `scope_code` 必须存在 `package_code=DEFLT` 的默认包，且 `DEFLT` 为保留字。
- 默认订阅规则：新建任意 `setid` 时，必须通过事件入口显式写入对所有稳定 `scope_code` 的 `DEFLT` 订阅；`setid=DEFLT` 同样适用。
- `DEFLT` 包可变：允许在包内更新配置内容；配置数据必须具备有效期/事件化可回放语义，保证 `as_of_date` 审计可复现。
- 共享包订阅：订阅记录包含 `package_owner_tenant_id`；若为 `orgunit.global_tenant_id()`，仅允许白名单 `scope_code`，且解析时必须显式开启共享读开关（不允许 OR 读混用）。
- 订阅来源一致性：`package_owner_tenant_id = tenant_id` 时仅可指向租户包；`package_owner_tenant_id = orgunit.global_tenant_id()` 时仅可指向共享包。

## 7. 迁移与切换（蓝图）
- 盘点所有 setid-controlled 配置域，形成 `scope_code` 清单。
- 为每个域创建 `DEFLT` 默认包，并显式订阅到 `setid=DEFLT`。
- SetID 创建时自动写入“默认订阅事件”（所有稳定 `scope_code` -> `DEFLT` 包）。
- 迁移窗口内完成：
  1) 生成包与订阅（含 `DEFLT` 默认包与默认订阅）；
  2) 将存量配置数据绑定至对应 `package_id`；
  3) 切换读写到 `package_id`；
  4) 移除旧 `setid` 直连路径（避免 legacy 双链路）。
- 迁移过程遵循停写切换与审计留痕，具体门禁以 `AGENTS.md` 与 `DEV-PLAN-012` 为准。

## 8. API / UI（概要）
- 新增包管理入口：创建/停用 `scope_package`（按 scope_code）。
- 新增订阅入口：为 SetID 在某个 scope 选择包（含有效期）。
- 订阅入口允许选择共享包（仅白名单 scope），需明确标识“共享/只读”。
- SetID 管理页展示“当前 SetID 的 scope 订阅表”，并允许切换包。

## 9. 实施步骤（草案）
1. [ ] 定义 `scope_code` 清单与治理规则（归属、权限点、命名约束）。
2. [ ] 设计并确认事件表/版本表结构（需用户确认新增表）。
3. [ ] 在解析链路引入 `ResolveScopePackage` 并更新配置读写口径。
4. [ ] 制定并演练迁移切换方案（停写切换、无双链路）。
5. [ ] 补齐 UI、Authz、测试与门禁证据。

## 10. 验收标准（草案）
- [ ] 任一 SetID 可在不同 `scope_code` 订阅不同 `scope_package` 并生效。
- [ ] 缺失订阅时配置读取/写入 fail-closed。
- [ ] 订阅与包均可审计回放（事件 + validity）。
- [ ] `package_code` 在同一 `tenant_id + scope_code` 内唯一，冲突必须被拒绝。
- [ ] 每个 `scope_code` 存在 `package_code=DEFLT` 的默认包，且 `setid=DEFLT` 默认订阅已落库。
- [ ] `DEFLT` 包更新可生效且可审计（无隐式回退路径）。
- [ ] 共享包订阅仅限白名单 `scope_code`，解析时必须显式共享读开关且无 OR 读混用。
- [ ] 迁移后无旧 `setid` 直连路径残留。

## 11. 关联文档
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/028-setid-management.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
