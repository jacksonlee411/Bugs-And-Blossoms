# DEV-PLAN-100：Org 模块宽表预留字段 + 元数据驱动落地实施计划与路线图（承接 DEV-PLAN-098）

**状态**: 规划中（2026-02-13 07:37 UTC）

## 1. 背景与目标

`DEV-PLAN-098` 已完成“架构评估”，结论是：在 PostgreSQL + 多租户 + 有效期（day 粒度）场景下，采用“原生宽表预留字段 + 元数据驱动 + PLAIN/DICT/ENTITY 分流（PLAIN 无 options）”具备可行性与性能优势。  
本计划（DEV-PLAN-100）把评估结论转化为**可执行的分阶段实施路线图**，并严格对齐仓库不变量（One Door / No Tx, No RLS / Valid Time / No Legacy）。

本计划的目标不是一次性做“全量动态表单平台”，而是先在 OrgUnit 做一个风险可控、可验证、可回滚的最小闭环（MVP）。

## 2. 蓝图分析结论（对 DEV-PLAN-098 的工程化收敛）

### 2.1 可以直接采用的结论

1. 宽表稀疏列在 PostgreSQL 的 NULL Bitmap 下存储成本可控，优先于纯 JSONB 做排序/筛选。  
2. 组织树继续以 `ltree` 为主，不改变现有层级查询主路径。  
3. 扩展字段按热点逐步建索引，默认不全量建索引。  
4. Select 字段必须区分 `DICT` 与 `ENTITY` 两类数据源。

### 2.2 必须补齐的仓库级约束

1. **One Door**：扩展字段写入仍必须走 `orgunit.submit_org_event(...)` / correction / rescind 体系，禁止绕过事件写 `org_unit_versions`。  
2. **映射不可变**：`(tenant_uuid, field_key) -> physical_col` 启用后不可改，不允许槽位复用。  
3. **历史一致性必须显式选择**：DICT 采用“写入时 label 快照 + code 存储”；ENTITY 采用“存 ID + as-of join”。  
4. **动态 SQL 必须 allowlist + 参数化**：仅允许元数据表中登记的物理列参与查询构造。
5. **字段配置写入口唯一**：`tenant_field_configs` 的新增/启停/停用必须走单一管理入口（Kernel 函数），禁止应用角色直接 DML。  
6. **No Tx, No RLS**：新增元数据表必须启用并强制 RLS，且所有访问都要求显式事务 + 租户注入，fail-closed。

### 2.3 本期落地边界（MVP）

- 仅覆盖 OrgUnit 扩展字段（先 2~5 个字段）。
- 类型仅支持：`text / int / uuid / bool / date`。
- 仅支持单值字段，不支持多值集合、公式字段、跨模块写时联动。
- 仅在 OrgUnit 详情页/列表页开放读取闭环；高级报表与跨模块复用后置。

## 3. 范围与非目标

### 3.1 范围（In Scope）

- OrgUnit 扩展字段元数据模型（字段定义、槽位映射、数据源配置、生命周期）。
- 宽表扩展列在 `orgunit.org_unit_versions` 的最小增量。
- 写入链路：Create/Correct 事件支持扩展字段 payload 并同步投射。
- 读取链路：详情读取、列表筛选/排序（仅白名单字段）。
- 能力外显：mutation capabilities（承接 `DEV-PLAN-083`，含 `deny_reasons`）与扩展字段 `filter/sort/options` allowlist。
- Options 接口：DICT 与 ENTITY 双通道读取（PLAIN 无 options）。
- 字段配置管理：管理端字段启用/停用的 API + UI（UI IA 见 `DEV-PLAN-101`）。

### 3.2 非目标（Out of Scope）

- 不改写 OrgUnit 事件模型主语义（CREATE/MOVE/RENAME/…）。
- 不引入第二写入口，不引入 legacy 兼容双链路。
- 不在本计划实现“任意模块通用元数据平台”。
- 不做 per-tenant index、也不做自动索引生成器。

## 4. 关键设计决策（冻结项）

### D1. 扩展列落点

- 扩展列落在 `orgunit.org_unit_versions`（读模型）。
- `orgunit.org_events.payload` 保留扩展字段原始提交值，用于审计与重放。

### D2. 槽位与映射规则

- 采用固定槽位命名（示例）：`ext_str_01..ext_str_30`、`ext_int_01..ext_int_10`、`ext_uuid_01..ext_uuid_10`、`ext_bool_01..ext_bool_10`、`ext_date_01..ext_date_10`。
- 元数据启用后不可变更 `physical_col`；停用不等于可复用。
- `field_key` 形状冻结（用于 `payload.ext` 的 key；必须可枚举、可校验、可防注入）：
  - 仅允许 `^[a-z][a-z0-9_]{0,62}$`；
  - 禁止包含 `.`（保留给 `ext.<field_key>` dot-path 语义）；
  - 字段定义清单不得包含与基础字段冲突的 key（例如 `name/effective_date/org_code/...`），避免 UI/服务端出现路径歧义；冲突必须在启用阶段 fail-closed 拒绝（防线与错误码口径以 Phase 1/3 SSOT 为准）。

### D3. PLAIN/DICT/ENTITY（数据源与展示策略）

- `PLAIN`：无 options；versions 直接存值；事件 payload 不需要也不接受该字段的 label 快照（如出现则视为非法输入）。  
- `DICT`：versions 存 `code`（通常进入 `ext_str_xx`），事件 payload 同时写 `ext_labels_snapshot`（按字段 key 存 label）。  
  - `label` 口径冻结为 **canonical label（非本地化展示名）**：用于 `ext_labels_snapshot` 与 options 返回值；不随 UI locale 变化（避免引入“业务数据多语言”，边界见 `DEV-PLAN-020`）。  
- `ENTITY`：versions 存主键 ID（`ext_int_xx` 或 `ext_uuid_xx`），展示时按 `as_of` join 实体表拿 label。

### D4. 历史一致性策略

- DICT：`orgunit.org_events.payload` 写入 `ext_labels_snapshot`，投射时同步写入 `orgunit.org_unit_versions.ext_labels_snapshot`；读取优先级固定为 `versions 快照 -> events 快照 -> 当前字典 label（兼容兜底并打审计标记）`。
  - “审计标记”冻结为对外可见的来源标识（例如 details 返回 `display_value_source`），用于 UI 显式提示兜底路径；禁止静默“当前名称覆盖历史”（SSOT：Phase 3 的 `DEV-PLAN-100D`）。
- ENTITY：严格 `as_of` 查询，禁止“当前名称覆盖历史”。

### D5. 安全与可观测

- 所有扩展字段查询都必须经过服务端 capability/allowlist 校验。  
- 记录审计日志字段：`tenant_uuid`、`field_key`、`physical_col`、`query_mode(filter/sort/options)`。

### D6. 字段配置生命周期与时间语义

- 字段配置的业务生效时间使用 **day 粒度**（`enabled_on/disabled_on` 为 `date`）；`created_at/updated_at/disabled_at` 仅用于审计时间（`timestamptz`）。
- 写入校验按 `effective_date` 解释字段配置是否生效；读取（详情/列表/options）按 `as_of` 解释字段可见性。

### D7. allowlist 单一事实源（SSOT）

- 扩展字段 `filter/sort/options` 的能力判断统一由服务层能力解析器产出（承接 `DEV-PLAN-083` “策略单点/能力外显”原则），禁止在 API/SQL 层重复维护第二套白名单。
- `data_source_config` 不允许透传任意表名/列名；仅允许枚举化实体标识 + 预定义 SQL 模板映射。

### D8. Mutation capabilities 与 Query allowlist 统一（承接 DEV-PLAN-083）

- OrgUnit 的写入动作能力（mutation：create/event_update/correct*/rescind*）与查询能力（query：filter/sort/options）必须通过同一“能力解析器/策略矩阵”产出；UI/API/SQL 不得各自维护第二套白名单、字段映射或组合约束。
- 对扩展字段，能力解析器必须可证明地产出并解释：`field_key -> payload_key`（写入）与 `field_key -> physical_col`（查询）映射，且两者均 fail-closed。
- `GET /org/api/org-units/mutation-capabilities`（详见 `DEV-PLAN-083`）必须覆盖扩展字段：返回 `allowed_fields`、`field_payload_keys` 与 `deny_reasons`，用于 UI 禁用/隐藏与原因解释。
- API 不可用或解析失败时，UI 必须 fail-closed（只读/禁用），不做乐观放行。

## 5. 分阶段实施路线图

## Phase 0：契约冻结与就绪检查（先文档后代码）

已拆分为独立实施计划（SSOT）：`docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`。

## Phase 1：Schema 与元数据骨架（最小数据库闭环）

已拆分为独立实施计划（SSOT）：`docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`。

## Phase 2：Kernel/Projection 扩展（保持 One Door）

已拆分为独立实施计划（SSOT）：`docs/dev-plans/100c-org-metadata-wide-table-phase2-kernel-projection-extension-one-door.md`。

## Phase 3：服务层与 API（读写可用）

已拆分为独立实施计划（SSOT）：`docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`。

## Phase 4：UI 集成（用户可见闭环）

> 实施优先级建议：先完成 **4A（详情页编辑态能力外显，消除“可输必败”）**，再补齐 **4B（字段配置管理页可发现性）**，最后收口 **4C（列表能力 + i18n）**。  
> 为支持 4A 开发，字段配置可临时通过 Phase 3 的管理接口创建；但 Phase 4 出口必须包含 4B 页面以满足“配置字段”端到端路径可发现。

### 4A：详情页编辑态能力外显（承接 DEV-PLAN-083）

已拆分为独立实施计划（SSOT）：`docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`。

1. [ ] OrgUnit 详情页展示扩展字段（按字段配置动态渲染）。
2. [ ] 详情页编辑态严格按 mutation capabilities 控制字段可编辑性、动作可用性与原因解释（`deny_reasons`）；接口不可用时 fail-closed（只读/禁用）。
3. [ ] Select 字段接入 options endpoint（DICT/ENTITY 双通道）。

### 4B：字段配置管理页（由DEV-PLAN-101承接）

4. [ ] 字段配置管理页（UI）可发现且可操作（仅管理员可见/可达；启用/停用；映射槽位只读），IA/页面结构以 `DEV-PLAN-101` 为准。

### 4C：列表能力与 i18n（闭环收口）

5. [ ] OrgUnit 列表页开放 1~2 个扩展字段筛选/排序入口。
6. [ ] i18n（en/zh）补齐扩展字段标签与错误提示。

**出口条件**：
- 用户可在页面完成“字段配置管理（启用字段）-> 写入值 -> 列表筛选/排序 -> 详情回显”至少一条端到端路径。
- 页面在字段未生效/已停用时给出明确禁用原因（不允许静默放行）。
- 字段配置管理页在租户 App 内可发现（导航/页面入口）且权限 fail-closed（无权限统一拒绝）。

## Phase 5：稳定化与性能收口

1. [ ] 基于慢查询与真实访问证据补充索引，不做预防性全量索引。  
2. [ ] 增加基准对比（至少：列表排序、筛选、详情读取）。  
3. [ ] 补齐异常场景：槽位耗尽、映射冲突、ENTITY 不存在、DICT 快照缺失。  
4. [ ] 形成运维手册：字段启停、回滚策略、故障排查步骤。

**出口条件**：P95/P99 延迟与错误率达到团队可接受阈值（阈值由评审会冻结后补充）。

## 6. 里程碑交付物（按阶段）

- M0（Phase 0）：契约冻结版文档（SSOT：`DEV-PLAN-100A`）+ 字段清单。  
- M1（Phase 1）：迁移脚本（SSOT：`DEV-PLAN-100B`）+ 元数据表 + 扩展列 + 基础约束。  
- M2（Phase 2）：Kernel/Projection 支持扩展字段写入与回放（SSOT：`DEV-PLAN-100C`）。  
- M3（Phase 3）：API 可读写 + options + mutation-capabilities + 列表筛选/排序（SSOT：`DEV-PLAN-100D`）。  
- M4（Phase 4）：UI 可操作闭环（字段配置管理页 + 详情页 capabilities 驱动编辑 + 列表筛选/排序）。  
- M5（Phase 5）：性能与稳定性报告。

## 7. 验收标准（DoD）

1. [ ] 不新增第二写入口，所有写操作可追溯到 `submit_*` 内核函数。  
2. [ ] 扩展字段在有效期切换下读写一致，历史可复现。  
3. [ ] DICT/ENTITY 语义不混用，存储策略与展示策略一致。  
4. [ ] 列表筛选/排序命中至少 1 个扩展字段，且性能可接受。  
5. [ ] 字段配置写入可追溯到单一管理入口（无直写元数据表绕行）。  
6. [ ] 门禁通过：`make check doc` + 命中的代码/迁移/测试门禁。
7. [ ] 无效 action/event/target 组合必拒绝（fail-closed），且服务层策略与 Kernel 防守性校验口径一致。  
8. [ ] mutation capabilities 可稳定返回“可用性 + 字段映射 + 拒绝原因”，扩展字段与核心字段同口径可解释。  
9. [ ] `PATCH_FIELD_NOT_ALLOWED` 等稳定错误码不因 UI/Service/Kernel 漂移。
10. [ ] 字段配置管理页可发现、可操作且权限正确：仅管理员可启用/停用字段；`physical_col` 等映射信息只读可审计；无权限访问 fail-closed。

## 8. 测试与门禁计划（引用 SSOT）

- 触发器与命令入口以 `AGENTS.md` 和 `docs/dev-plans/012-ci-quality-gates.md` 为准。  
- 本计划新增代码后，至少补齐：
  - Kernel SQL 回归测试（写入/重放/快照）；
  - 元数据管理入口测试（字段新增/启停/不可复用/权限拒绝）；
  - Service 单测（策略矩阵组合、字段映射、类型校验、allowlist）；
  - API 契约测试（mutation-capabilities/options/details/list，含 `deny_reasons`）；
  - E2E 用例（字段配置管理页可用 + 详情页可见/可写/禁用原因可解释 + 列表筛选/排序）。
- 若命中相应触发器，补跑并记录结果：
  - 路由：`make check routing`
  - Authz：`make authz-pack && make authz-test && make authz-lint`
  - sqlc：`make sqlc-generate`（并验证 `git status --short` 为空）

## 9. 风险清单与缓解

1. **槽位耗尽**：字段增长快于预留列。  
   - 缓解：Phase 1 预留容量评估 + Phase 5 扩容流程（Atlas/Goose 闭环）。
2. **历史语义漂移**：映射被误改或槽位复用。  
   - 缓解：DB trigger 禁改 + 服务层禁止复用 + 审计告警。
3. **动态 SQL 漏洞**：拼接列名导致注入风险。  
   - 缓解：列名仅来自元数据 allowlist，参数全部占位符绑定。
4. **重放写放大加剧**：correction/rescind 导致版本重建成本上升。  
   - 缓解：先小范围字段、再按压测结果迭代优化重放路径。

## 10. 依赖关系与并行策略

- 强依赖：`098 -> 100A(Phase 0) -> 100B(Phase 1) -> 100C(Phase 2) -> 100D(Phase 3) -> 100 Phase 4`。
- 可并行：Phase 3（API）与 Phase 4（UI）可部分并行，但必须在 Phase 2 出口后合并。
- 质量收口：Phase 5 在 Phase 3/4 功能闭环后统一执行。

## 11. 代码落点（预分配）

- Schema/函数：`modules/orgunit/infrastructure/persistence/schema/`  
- 写服务：`modules/orgunit/services/orgunit_write_service.go`（及新建 metadata 辅助文件）  
- API：`internal/server/orgunit_api.go`、`internal/server/orgunit_nodes.go`  
- 测试：
  - `internal/server/orgunit_api_test.go`
  - `internal/server/orgunit_nodes_test.go`
  - `internal/server/orgunit_nodes_read_test.go`
  - `internal/server/orgunit_audit_snapshot_schema_test.go`

## 12. 关联文档

- `docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`
- `docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`
- `docs/dev-plans/100c-org-metadata-wide-table-phase2-kernel-projection-extension-one-door.md`
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- `docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- `docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- `docs/dev-plans/098-org-module-wide-table-metadata-driven-architecture-assessment.md`
- `docs/dev-plans/083-org-whitelist-extensibility-capability-matrix-plan.md`
- `docs/dev-plans/080c-orgunit-audit-snapshot-presence-table-constraint-plan.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`
- `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`
