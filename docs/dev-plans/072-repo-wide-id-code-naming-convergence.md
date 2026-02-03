# DEV-PLAN-072：全仓 ID/Code 命名与对外标识收敛

**状态**: 草拟中（2026-02-02 14:34 UTC）

## 1. 背景
- 现状：多个模块在字段命名与对外标识上存在不一致（例如 `tenant_id/request_id/event_id` 与 026A 的 `_uuid/_code` 规则不一致；对外仍暴露 `org_unit_id` 与 026B 冲突）。
- 目标：以 026A/026B 为 SSOT，将全仓命名与边界契约收敛到一致规则，避免跨模块“混用/双轨”。

## 2. 目标与非目标
### 2.1 目标
- [ ] 以 `DEV-PLAN-026A/026B` 为全仓命名与对外标识的唯一事实源；完成跨模块字段命名与边界契约收敛。
- [ ] 统一“外部标识 vs 内部结构标识”的边界：对外仅暴露 `org_code`，内部仅使用 `org_id`（按 026B）。
- [ ] 消除 `tenant_id/request_id/event_id/initiator_id` 等与 026A 命名规则冲突的字段命名歧义。
- [ ] 建立可执行的收敛验收清单与门禁（避免回潮）。

### 2.2 非目标
- 不引入新的业务功能或新模块。
- 不引入双轨兼容/回退通道（遵循 No Legacy）。
- 不在本计划内改变业务语义（仅做命名/边界契约收敛与必要的接口适配）。

## 3. 依赖与前置条件
- [x] `DEV-PLAN-026B` 作为对外契约采纳；OrgUnit 侧改造由 026B 负责，不在本计划实施范围。
- [ ] 新增数据库表/迁移必须取得用户确认（仓库红线）。
- [ ] 涉及字段重命名/DDL 变更，按模块执行 Atlas+Goose 闭环（入口引用 `DEV-PLAN-024` 与 `AGENTS.md`）。
- [x] 外部调用方范围确认为 UI/HTMX 与 Internal API。

## 4. 事实源与对齐原则
- 命名与类型规则：`DEV-PLAN-026A`（字段后缀与类型规则、UUID v7、request_code）。
- 对外标识规则：`DEV-PLAN-026B`（外部仅 `org_code`、内部仅 `org_id`、边界解析）。
- 路由门禁：`DEV-PLAN-017`；CI/本地门禁入口：`AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md`。
- 例外：`setid` 属于专有名词，**豁免** `_code` 后缀要求，保持 `setid` 命名。
- `org_code` 命名与字符限制沿用 026B：`trim + upper` 归一化；允许 `A-Z/a-z/0-9/-/_`，长度 1~16；存储正则 `^[A-Z0-9_-]{1,16}$`；对外回显统一大写；错误码使用 `org_code_invalid/org_code_not_found/org_code_conflict`。

## 5. 收敛范围（模块级）
- OrgUnit：由 `DEV-PLAN-026B` 承接，不在本计划实施范围（避免与 026B 重复交付；本计划仅对齐依赖与边界契约）。
- Staffing（Position/Assignment）：边界层字段命名与 UI/API 输入输出需从 `org_unit_id` 收敛为 `org_code`，内部仍使用 `org_id`。
- Job Catalog：保持“外部 code / 内部 id”的模型，但字段命名需按 026A 规则收敛（`tenant_uuid/request_code/event_uuid/initiator_uuid`）。
- 其它涉及 OrgUnit/SetID/Scope 的模块：按 026A/026B 统一命名与边界规则。

> 说明：是否新增表需用户确认，遵循仓库红线。

### 5.1 Staffing（Position/Assignment）改造清单（边界与字段）
- UI/HTMX：
  - 表单与列表字段从 `org_unit_id` 改为 `org_code`（仅展示 `org_code`；如需内部 ID，仅限 debug/内部视图）。
  - 查询参数与重定向参数由 `org_unit_id` 收敛为 `org_code`。
- Internal API：
  - 入参/出参统一使用 `org_code`，禁止对外暴露 `org_id`。
  - 错误码补齐 `org_code_invalid/org_code_not_found` 等与 026B 对齐的语义。
- 服务层：
  - 复用 026B 解析器进行 `org_code -> org_id`，内部继续使用 `org_id`。
  - DB payload 仍传 `org_unit_id`（内部字段），但禁止对外直接透出。
- 测试：
  - 现有 Position/Assignment 用例的参数与断言改为 `org_code`。
  - 负例覆盖：非法/不存在 `org_code`。

### 5.2 Job Catalog（职位分类）改造清单（命名收敛）
- Schema/SQL：
  - `tenant_id` → `tenant_uuid`；`event_id` → `event_uuid`；`request_id` → `request_code`；`initiator_id` → `initiator_uuid`。
  - UUID 字段统一后缀为 `_uuid`（例如 `package_id/job_profile_id/job_family_id` → `package_uuid/job_profile_uuid/job_family_uuid`）。
  - code 字段统一为 `_code`（例如 `code` → `job_family_code/job_profile_code`）。
  - 函数签名、约束命名与索引命名同步更新，避免 026A/029/030 口径漂移。
- Go 层与查询：
  - 参数与 JSON 字段名同步改名，保持对外语义不变。
  - 适配历史测试与错误映射（以稳定 code 为准）。

### 5.3 Staffing（职位管理/任职）改造清单（命名收敛）
- Schema/SQL：
  - `tenant_id` → `tenant_uuid`；`event_id` → `event_uuid`；`request_id` → `request_code`；`initiator_id` → `initiator_uuid`。
  - UUID 字段统一后缀为 `_uuid`（例如 `position_id/assignment_id/reports_to_position_id/job_profile_id` → `position_uuid/assignment_uuid/reports_to_position_uuid/job_profile_uuid`）。
  - `jobcatalog_setid` 作为专有名词保留原名（不加 `_code` 后缀）；`jobcatalog_setid_as_of` 保持为日期语义字段。
  - `org_unit_id` 继续保持 8 位 `int` 结构标识（按 026A，不改为 `_uuid`）。
  - 函数签名、约束命名与索引命名同步更新。
- Go 层与查询：
  - 参数与 JSON 字段名同步改名（内部字段与 payload/SQL 形状一致）。
  - 测试与错误映射同步更新，避免字符串字段名漂移。

### 5.4 对外契约变更清单（UI/HTMX/Internal API）
> 仅覆盖非 OrgUnit 模块；OrgUnit 对外契约由 026B 承接。

**Staffing（/org/positions, /org/assignments, /org/api/positions, /org/api/assignments）**
- 字段命名收敛（对外）：
  - `org_unit_id` → `org_code`（按 026B 规则校验与回显大写）。
  - `position_id` → `position_uuid`。
  - `assignment_id` → `assignment_uuid`。
  - `reports_to_position_id` → `reports_to_position_uuid`。
  - `job_profile_id` → `job_profile_uuid`。
- UI 展示列名同步改为 `*_uuid`（仍显示实际 UUID 值）。
- 内部 payload/SQL 继续使用 `org_unit_id`（8 位结构标识），不对外暴露。

**Job Catalog（/org/job-catalog UI）**
- UI 表单已使用 `*_code` 字段名（保持不变）。
- 若 UI 展示 `id` 列，列名改为 `<entity>_uuid`（值不变）。

## 6. 关键设计决策（摘要）
- **决策 1**：026A/026B 为全仓命名与边界契约 SSOT；任何偏差必须通过 dev-plan 明确说明并获批准。
- **决策 2**：对外只暴露 `org_code`，内部仅使用 `org_id`；进入服务边界即解析（不允许外部同时携带 `org_id`）。
- **决策 3**：拒绝双轨字段与兼容层（No Legacy）；切换以“模块级原子收敛”方式进行。

## 7. 实施步骤
1. [ ] 盘点差异清单（字段/接口/SQL/测试）——草拟映射表已生成，待冻结：
   - 记录见 `docs/dev-records/dev-plan-072-naming-convergence-mapping.md`。
2. [ ] Staffing 边界收敛（Position/Assignment）：
   - UI 表单与 Internal API 入参/出参改为 `org_code`；
   - 服务层复用 026B 的解析器（不在本计划重复建设），并保留内部 `org_id` 流程；
   - 更新错误码与测试断言。
3. [ ] Staffing 命名收敛：
   - Schema/SQL/Go 层按 026A 规则统一命名（详见 5.3）。
   - 更新迁移与函数签名。
4. [ ] Job Catalog 命名收敛：
   - DB/SQL/Go 层字段命名按 026A 规则统一（`tenant_uuid/event_uuid/request_code/initiator_uuid` 等）；
   - 更新迁移与函数签名。
5. [ ] 横切门禁与回归：
   - 新增或完善命名检查/契约检查（避免 `*_id`/`*_uuid`/`*_code` 混用回潮）；
   - 更新文档与测试证据（`docs/dev-records/`）。

> 门禁与命令入口引用 `AGENTS.md` 与 CI 质量门禁文档；本计划不重复列出具体命令。

## 8. 风险与缓解
- **风险：大范围字段重命名导致迁移成本高**
  - 缓解：模块级原子切换；严格禁止双轨；在每个模块完成后补充记录与测试证据。
- **风险：外部接口改名导致调用方破坏**
  - 缓解：在切换前提供明确迁移说明与端到端验证；保持路由不变，仅字段名收敛。
- **风险：新增表或迁移触发红线**
  - 缓解：任何新增表/迁移执行前获得用户确认（适用于非 OrgUnit 模块变更）。

## 9. 验收标准
- [ ] 非 OrgUnit 模块的外部接口/页面不再出现 `org_id`，仅使用 `org_code`（OrgUnit 范围由 026B 验收）。
- [ ] 非 OrgUnit 模块的字段命名满足 026A 规则（`*_uuid`/`*_id`/`*_code`）。
- [ ] 业务逻辑内部不出现 `org_code` 参与结构性关系或索引计算。
- [ ] 关键路径测试与门禁通过，并在 `docs/dev-records/` 留证。

## 10. 交付物
- DEV-PLAN-072（本文件）。
- 差异清单与收敛映射表（`docs/dev-records/`）。
- 相关模块迁移/代码/测试/文档更新。

## 参考
- `docs/dev-plans/026a-orgunit-id-uuid-code-naming.md`
- `docs/dev-plans/026b-orgunit-external-id-code-mapping.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
