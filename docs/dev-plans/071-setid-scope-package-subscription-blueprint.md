# DEV-PLAN-071：SetID Scope Package 订阅蓝图

**状态**: 草拟中（2026-01-29 03:17 UTC）

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

### 3.1 首批 scope_code 清单（建议）
| scope_code | 归属模块 | 范围说明 | 共享层可见 |
| --- | --- | --- | --- |
| `jobcatalog` | `jobcatalog` | 职位目录主数据（Job Family Group/Family/Level/Profile） | 否 |
| `orgunit_geo_admin` | `orgunit` | 国家/省市等行政区划（共享白名单） | 是 |
| `orgunit_location` | `orgunit` | 地点与地点类型（共享白名单） | 是 |
| `orgunit_school` | `orgunit` | 学校（共享白名单） | 是 |
| `orgunit_education_type` | `orgunit` | 学历类型（共享白名单） | 是 |
| `orgunit_credential_type` | `orgunit` | 资格证书/证照类型（共享白名单） | 是 |

### 3.2 归属规则
- 单一归属：每个 `scope_code` 必须明确唯一归属模块，归属模块负责 schema、kernel 写入口、管理 UI 与测试门禁。
- 命名规则：`scope_code` 使用 `lower_snake_case`；跨域或通用参考数据应以归属模块前缀命名，避免歧义。
- 共享层边界：仅标注“共享白名单”的 `scope_code` 允许读取 `SHARE`，其余默认 tenant-only。
- 跨模块使用：非归属模块只能通过共享的解析/读接口使用该 `scope_code`，禁止私自定义重复的配置域。
- 新增流程：新增或调整 `scope_code` 必须更新本清单并补齐门禁与验收项。

## 4. 架构概览（高阶）
1. `ResolveSetID(tenant_id, org_unit_id, as_of_date)` 仍由 `DEV-PLAN-070` 定义。
2. 新增 `ResolveScopePackage(tenant_id, setid, scope_code, as_of_date)`。
3. 配置数据读取/写入使用 `package_id`（或 `scope_code + package_id`），不再直接用 `setid` 过滤。

## 5. 数据模型（概念蓝图）
> 具体建表需用户确认（对齐 `AGENTS.md` 新增表确认要求）。
- `orgunit.setid_scope_packages`
  - `tenant_id`, `scope_code`, `package_id`, `package_code`, `name`, `status`, `created_at`, `updated_at`
  - 约束：`tenant_id + scope_code + package_code` 唯一，`package_code` 仅在其 `scope_code` 内唯一
- `orgunit.setid_scope_package_events`
  - 包创建/停用事件（One Door）
- `orgunit.setid_scope_subscriptions`
  - `tenant_id`, `setid`, `scope_code`, `package_id`, `validity`, `last_event_id`
- `orgunit.setid_scope_subscription_events`
  - 订阅变更事件（One Door）

## 6. 核心规则与不变量
- 单一有效订阅：同一 `tenant_id + setid + scope_code` 在同一日期仅允许一个有效 `package_id`。
- 包归属明确：`package_id` 只能绑定一个 `scope_code`，禁止跨域复用。
- 包编码唯一性：`package_code` 在同一 `tenant_id + scope_code` 内唯一。
- 解析 fail-closed：缺少订阅或订阅无效即拒绝。
- 订阅与包均需走事件入口，禁止直接写表。
- 订阅有效期与 `as_of_date` 对齐（Valid Time 语义）。
- 默认包规则：每个 `scope_code` 必须存在 `package_code=DEFLT` 的默认包，且 `DEFLT` 为保留字。
- 默认订阅规则：`setid=DEFLT` 必须显式订阅每个 `scope_code` 的 `DEFLT` 包（通过事件入口写入）。
- `DEFLT` 包可变：允许在包内更新配置内容，变更必须记录事件并可审计回放。

## 7. 迁移与切换（蓝图）
- 盘点所有 setid-controlled 配置域，形成 `scope_code` 清单。
- 为每个域创建 `DEFLT` 默认包，并显式订阅到 `setid=DEFLT`。
- 迁移窗口内完成：
  1) 生成包与订阅（含 `DEFLT` 默认包与默认订阅）；
  2) 将存量配置数据绑定至对应 `package_id`；
  3) 切换读写到 `package_id`；
  4) 移除旧 `setid` 直连路径（避免 legacy 双链路）。
- 迁移过程遵循停写切换与审计留痕，具体门禁以 `AGENTS.md` 与 `DEV-PLAN-012` 为准。

## 8. API / UI（概要）
- 新增包管理入口：创建/停用 `scope_package`（按 scope_code）。
- 新增订阅入口：为 SetID 在某个 scope 选择包（含有效期）。
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
- [ ] 迁移后无旧 `setid` 直连路径残留。

## 11. 关联文档
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/028-setid-management.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
