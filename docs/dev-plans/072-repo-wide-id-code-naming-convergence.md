# DEV-PLAN-072：全仓 ID/Code 命名与对外标识收敛

**状态**: 草拟中（2026-02-02 14:34 UTC）；已复评（2026-02-03）

## 0. 复评结论（基于 DEV-PLAN-026B/026C/026D）
- 026B 已完成并落地，对外契约以 `org_code` 为唯一事实源：**可作为 072 的边界基准**。
- 026C 仍为“部分完成”（迁移样本统计与示例/保留字一致性待补）：**外部契约改名需等 026C 收口**，避免约束变更导致返工。
- 026D 已实施并留证，OrgUnit 写路径与权限边界稳定：**对 072 无新增阻断**，但不改变 072 的边界约束。
- 结论：072 继续推进“非 OrgUnit 模块的命名收敛”，但**涉及对外 `org_code` 的边界改造应以 026C 完成作为前置条件**。

## 1. 背景与上下文 (Context)
- **需求来源**：`DEV-PLAN-026A/026B/026C/026D` 与全仓命名收敛要求。
- **当前痛点**：跨模块字段命名不一致（`*_id`/`*_uuid`/`*_code` 混用），对外仍暴露 `org_unit_id`，与 026B 产生冲突。
- **业务价值**：统一“外部标识 vs 内部结构标识”边界，降低调用方困惑与后续协作成本，避免双轨与回退通道（No Legacy）。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 目标
- [ ] 以 `DEV-PLAN-026A/026B` 为全仓命名与对外标识的唯一事实源；完成跨模块字段命名与边界契约收敛。
- [ ] 统一“外部标识 vs 内部结构标识”的边界：对外仅暴露 `org_code`，内部仅使用 `org_id`（按 026B）。
- [ ] 消除 `tenant_id/request_id/event_id/initiator_id` 等与 026A 命名规则冲突的字段命名歧义。
- [ ] 建立可执行的收敛验收清单与门禁（避免回潮）。

### 2.2 非目标
- 不引入新的业务功能或新模块。
- 不引入双轨兼容/回退通道（遵循 No Legacy）。
- 不在本计划内改变业务语义（仅做命名/边界契约收敛与必要的接口适配）。

### 2.3 工具链与门禁（SSOT 引用）
> 仅声明本计划命中的触发器；命令入口与门禁以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。

- **触发器清单（勾选本计划命中的项）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] `.templ` / Tailwind（若 UI 模板变更触发：`make generate && make css`）
  - [x] DB 迁移 / Schema（模块级 Atlas+Goose 闭环）
  - [x] sqlc（`make sqlc-generate`）
  - [ ] 路由治理（路由不变则不触发；如需改 allowlist 再触发）
  - [ ] Authz（策略不变则不触发）
  - [x] 文档（`make check doc`）

### 2.4 阻断条件（Blocking Conditions）
- 未完成 `DEV-PLAN-026C` 的收口项时，**禁止合并任何涉及对外 `org_code` 的契约改名**（避免约束变更返工）。
- 未冻结差异映射表时，**禁止进入模块级改名实施**（避免执行期反复漂移）。

## 3. 依赖与前置条件
- [ ] `DEV-PLAN-026B` 需先更新对外 `org_code` 约束（长度/字符集放宽）；本计划仅消费更新后的 SSOT。
- [ ] `DEV-PLAN-026C` 未完成项需收口：迁移样本统计 + 示例/保留字一致性；基于放宽后的约束完成验证结论。
- [x] `DEV-PLAN-026D` 已完成实施并留证，OrgUnit 写路径稳定。
- [ ] 新增数据库表/迁移必须取得用户确认（仓库红线）。
- [ ] 涉及字段重命名/DDL 变更，按模块执行 Atlas+Goose 闭环（入口引用 `DEV-PLAN-024` 与 `AGENTS.md`）。
- [x] 外部调用方范围确认为 UI/HTMX 与 Internal API。

## 4. 架构与关键决策 (Architecture & Decisions)
### 4.1 架构图（边界解析与命名收敛）
```mermaid
graph TD
  A[UI/HTMX] --> B[Handler]
  B --> C[Service]
  C --> D[OrgCode Resolver]
  C --> E[DB (org_id)]
  D --> E
```

### 4.2 关键设计决策（摘要）
- **决策 1**：026A/026B 为全仓命名与边界契约 SSOT；任何偏差必须通过 dev-plan 明确说明并获批准。
- **决策 2**：对外只暴露 `org_code`，内部仅使用 `org_id`；进入服务边界即解析（不允许外部同时携带 `org_id`）。
- **决策 3**：拒绝双轨字段与兼容层（No Legacy）；切换以“模块级原子收敛”方式进行。
- **决策 4**：列表/批量场景避免 N+1，优先批量解析或联表查询（对齐 026C 风险评估结论）。

### 4.3 收敛范围（模块级）
- OrgUnit：由 `DEV-PLAN-026B` 承接，不在本计划实施范围（避免与 026B 重复交付；本计划仅对齐依赖与边界契约）。
- Staffing（Position/Assignment）：边界层字段命名与 UI/API 输入输出需从 `org_unit_id` 收敛为 `org_code`，内部仍使用 `org_id`。
- Job Catalog：保持“外部 code / 内部 id”的模型，但字段命名需按 026A 规则收敛（`tenant_uuid/request_code/event_uuid/initiator_uuid`）。
- IAM：纳入命名收敛（`tenant_uuid/event_uuid/request_code/initiator_uuid` 等），对外路由参数名暂不调整（避免契约漂移）。
- Person：纳入命名收敛（`tenant_uuid` 等），对外契约如涉及需另行补充。

## 5. 数据模型与约束 (Data Model & Constraints)
### 5.1 命名与约束规则（SSOT）
- 命名与类型规则：`DEV-PLAN-026A`（字段后缀与类型规则、UUID v7、request_code）。
- 对外标识规则：`DEV-PLAN-026B`（外部仅 `org_code`、内部仅 `org_id`、边界解析）。
- 校验/占位与输入语义修订：`DEV-PLAN-026C`（校验链路、占位规则、示例一致性）。
- 例外：`setid` 属于专有名词，**豁免** `_code` 后缀要求，保持 `setid` 命名。
- `org_code` 约束（**放宽版**，以 026B/026C 更新为准）：仅做 `upper` 归一化（不 trim）；长度 **1~64**；白名单字符为 **ASCII 可打印字符**（含空格）、`\\t`、中文标点与全角空白（CJK Symbols and Punctuation：`\\u3000-\\u303F`）与全角字符（Fullwidth Forms：`\\uFF01-\\uFF60`、`\\uFFE0-\\uFFEE`）；**禁止全空白**（空格/\\t/全角空白）。更严格的约束交由租户配置执行；对外回显统一大写；错误码仍为 `org_code_invalid/org_code_not_found/org_code_conflict`。推荐正则：`^[\\t\\x20-\\x7E\\u3000-\\u303F\\uFF01-\\uFF60\\uFFE0-\\uFFEE]{1,64}$`。

### 5.2 Staffing（Position/Assignment）数据模型变更
- 仅命名收敛，不引入新表；如需新增表，必须先获用户确认。
- `tenant_id` → `tenant_uuid`；`event_id` → `event_uuid`；`request_id` → `request_code`；`initiator_id` → `initiator_uuid`。
- UUID 字段统一后缀为 `_uuid`（如 `position_id/assignment_id/reports_to_position_id/job_profile_id` → `position_uuid/assignment_uuid/reports_to_position_uuid/job_profile_uuid`）。
- `org_unit_id` 继续保持 8 位 `int` 结构标识（按 026A，不改为 `_uuid`）。
- 函数签名、约束命名与索引命名同步更新。

### 5.3 Job Catalog（职位分类）数据模型变更
- 仅命名收敛，不引入新表；如需新增表，必须先获用户确认。
- `tenant_id` → `tenant_uuid`；`event_id` → `event_uuid`；`request_id` → `request_code`；`initiator_id` → `initiator_uuid`。
- UUID 字段统一后缀为 `_uuid`（如 `package_id/job_profile_id/job_family_id` → `package_uuid/job_profile_uuid/job_family_uuid`）。
- code 字段统一为 `_code`（如 `code` → `job_family_code/job_profile_code`）。
- 函数签名、约束命名与索引命名同步更新，避免 026A/029/030 口径漂移。

### 5.4 迁移策略
- 模块级“原子收敛”：同一模块内字段重命名与引用更新一次性完成，禁止双轨字段并存。
- 迁移顺序：DDL 重命名 → SQL/函数更新 → sqlc 生成 → Go 层重命名与测试对齐。
- 差异清单与文件级替换映射见 `docs/dev-records/dev-plan-072-naming-convergence-mapping.md`。
- **DDL 策略**：优先使用 `ALTER TABLE ... RENAME COLUMN`（保数据），避免 drop+add 造成数据丢失。
- **回滚限制**：不提供自动回滚；如需回滚，仅允许对等的“反向 rename 迁移”，不得引入双轨字段。

### 5.5 差异映射冻结机制
- 在实施前，将 `docs/dev-records/dev-plan-072-naming-convergence-mapping.md` 标记为 **冻结版**（添加时间戳/提交号）。
- 冻结后如需调整，必须先更新映射表并在本计划记录变更原因与影响范围。
- 冻结版需补充以下元数据（写入 dev-records 头部）：冻结时间、冻结提交号、覆盖模块范围、变更审批人。

## 6. 接口契约 (API/HTMX Contracts)
> 路由不变，仅字段命名与 payload 字段收敛。

### 6.1 Internal API（Staffing）
- **位置**：`/org/api/positions`、`/org/api/assignments`
- **变更**：对外字段 `org_unit_id` → `org_code`，其余 id 字段后缀统一 `_uuid`。
- **错误码**：`org_code_invalid/org_code_not_found`（与 026B 对齐）。
- **示例（请求）**：
  ```json
  {
    "org_code": "HQ-001",
    "position_uuid": "...",
    "assignment_uuid": "..."
  }
  ```

### 6.2 UI/HTMX（Staffing 表单）
- **位置**：`/org/positions`、`/org/assignments`
- **变更**：表单字段由 `org_unit_id` 改为 `org_code`；列表展示列名同步更新。
- **响应**：HTML 片段结构不变，仅字段名与错误提示文案对齐 026B 语义。

### 6.3 旧字段拒绝策略（No Legacy）
- 外部请求 **不得** 再使用 `org_unit_id` 或旧的 `*_id` 字段名作为对外契约字段。
- 如检测到旧字段：返回 **400 + invalid_request**（沿用现有错误形态），并提示改用新字段名（例如 `org_code`、`*_uuid`）。
- 响应 payload **不得** 回填旧字段，避免隐性兼容。
- 冲突规则：若新旧字段同时出现（如 `org_code` 与 `org_unit_id` 并存，或 `position_uuid` 与 `position_id` 并存），一律按 **invalid_request** 拒绝。
- 校验位置：**handler 解析层**（进入 service 前完成），避免不同层重复/分叉校验。
- 说明：`org_code` 的严格字符约束由租户配置执行；标品侧只执行放宽后的最小校验（白名单 + 长度 + 非全空白 + upper 归一化）。

### 6.4 Job Catalog（/org/job-catalog）UI 合约
- **GET** `/org/job-catalog`：查询参数 `as_of`、`package_code`、`setid`（`package_code` 与 `setid` 互斥，违者 `invalid_request`）。
- **POST** `/org/job-catalog`：表单字段包含 `action`、`package_code`、`setid`、`effective_date` 及各子表单字段：
  - `job_family_group_code/name/description`
  - `job_family_code/name/description` + `job_family_group_code`
  - `job_level_code/name/description`
  - `job_profile_code/name/description` + `job_profile_family_codes` + `job_profile_primary_family_code`
- **UI 命名收敛**：列表列名中的 `id` 显示改为 `<entity>_uuid`（值不变），与 026A 规则一致。

### 6.5 调用方清单（必须全量覆盖）
- UI：`/org/positions`、`/org/assignments`、`/org/job-catalog`。
- Internal API：`/org/api/positions`、`/org/api/assignments`。

## 7. 核心逻辑与算法 (Business Logic & Algorithms)
- **边界解析**：进入服务边界即执行 `org_code -> org_id` 解析，解析失败直接返回 `org_code_invalid/org_code_not_found`。
- **列表/批量（必做项，方案 A）**：
  - 先在列表/批量接口收集去重后的 `org_unit_id` 列表。
  - 新增批量解析入口（例如 `ResolveOrgCodesBatch(tenant_uuid, org_unit_ids[])`），一次性查询映射：
    ```sql
    SELECT org_unit_id, org_code
    FROM orgunit.org_unit_codes
    WHERE tenant_uuid = $1 AND org_unit_id = ANY($2);
    ```
  - 将结果映射为 `map[int]string`，在组装响应时直接查表，**禁止**循环内逐条解析。
  - 实现与证据需记录在 `docs/dev-records/`（查询形态 + 调用次数）。
- **内部逻辑**：内部仍以 `org_id` 参与结构性关系与索引计算；`org_code` 不参与内部关系计算。

## 8. 安全与鉴权 (Security & Authz)
- Casbin 策略与路由权限不变；仅字段命名变更。
- RLS 与租户注入保持不变（No Tx, No RLS）。
- 对外严格隐藏 `org_id`，避免形成新的外部契约。

## 9. 实施步骤与里程碑 (Dependencies & Milestones)
1. [ ] 更新 026B/026C：明确放宽后的 `org_code` 长度/字符集约束，并完成迁移样本统计与示例/保留字一致性对齐；若无样本数据则记录豁免（审批人：我）。
2. [ ] 冻结差异清单（字段/接口/SQL/测试）：`docs/dev-records/dev-plan-072-naming-convergence-mapping.md`（记录冻结时间/提交号/覆盖范围/审批人）。
3. [ ] Staffing 边界收敛：UI 表单与 Internal API 入参/出参改为 `org_code`；旧字段拒绝；冲突规则落地；错误码与测试更新。
4. [ ] Staffing 命名收敛：Schema/SQL/Go 按 026A 规则统一命名。
5. [ ] Job Catalog 命名收敛：Schema/SQL/Go 按 026A 规则统一命名。
6. [ ] 批量解析方案落地（方案 A）：新增批量解析入口并在列表/批量接口调用；记录 SQL 形态与调用次数证据到 `docs/dev-records/`。
7. [ ] 横切门禁与回归：命名检查/契约检查、测试证据与文档留档（`docs/dev-records/`）。

## 10. 测试与验收标准 (Acceptance Criteria)
- [ ] 026B/026C 已更新并收口，`org_code` 放宽约束稳定且无需回滚/再放宽。
- [ ] 非 OrgUnit 模块的外部接口/页面不再出现 `org_id`，仅使用 `org_code`（OrgUnit 范围由 026B 验收）。
- [ ] 非 OrgUnit 模块的字段命名满足 026A 规则（`*_uuid`/`*_id`/`*_code`）。
- [ ] 业务逻辑内部不出现 `org_code` 参与结构性关系或索引计算。
- [ ] 关键路径测试与门禁通过，并在 `docs/dev-records/` 留证。
- [ ] 旧字段（如 `org_unit_id`、旧 `*_id`）作为对外字段时被明确拒绝，并有测试覆盖（400 + invalid_request）。
- [ ] 新旧字段同时出现时明确拒绝（冲突规则测试覆盖）。
- [ ] 列表/批量请求不会触发 N+1（记录批量查询 SQL 形态 + 调用次数证据，例如计数批量解析调用次数）。

## 11. 风险与缓解
- **风险：大范围字段重命名导致迁移成本高**
  - 缓解：模块级原子切换；严格禁止双轨；在每个模块完成后补充记录与测试证据。
- **风险：外部接口改名导致调用方破坏**
  - 缓解：在切换前提供明确迁移说明与端到端验证；保持路由不变，仅字段名收敛。
- **风险：026C 迁移样本统计未完成导致 `org_code` 约束调整**
  - 缓解：先完成 026C 样本统计并收口约束；再冻结 072 的外部契约改名清单。
- **风险：新增表或迁移触发红线**
  - 缓解：任何新增表/迁移执行前获得用户确认（适用于非 OrgUnit 模块变更）。

## 12. 运维与监控 (Ops & Monitoring)
- 本计划不新增 Feature Flag、监控指标或运维开关（对齐仓库“早期阶段”约束）。
- 回滚策略：仅通过正常代码回滚与迁移回退（如适用）；禁止引入双链路或 legacy 回退通道。

## 13. 交付物
- DEV-PLAN-072（本文件）。
- 差异清单与收敛映射表（`docs/dev-records/`）。
- 相关模块迁移/代码/测试/文档更新。

## 参考
- `docs/dev-plans/026a-orgunit-id-uuid-code-naming.md`
- `docs/dev-plans/026b-orgunit-external-id-code-mapping.md`
- `docs/dev-plans/026c-orgunit-external-id-code-mapping-review-and-revision.md`
- `docs/dev-plans/026d-orgunit-incremental-projection-plan.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
