# DEV-PLAN-026C：OrgUnit 外部ID兼容（org_code 映射）评审与修订方案

**状态**: 部分完成（2026-02-03，迁移样本统计豁免：无样本数据，审批人：我）

## 1. 背景与目的
- 本文为 DEV-PLAN-026B 的评审与修订计划，聚焦“契约一致性、可迁移性、可重放性、边界清晰”。
- 不改变 026B 的核心目标：**对外只见 org_code，对内只用 org_id**。

## 2. 范围与非范围
### 2.1 范围
- 澄清 026B 的语义歧义与边界条件，并给出修订建议。
- 明确前置事实与依赖机制（投射重建、输入校验链路、迁移样本）。
- 形成可执行的修订清单与验收口径。

### 2.2 非范围
- 不引入“多对多外部ID/改码/缓存”等新特性。
- 不新增独立运维工具链；仅在必要处补齐“重放/修复策略”描述。

### 2.3 实施顺序
- 先完成 026C（契约修订与对齐），再推进 026D（增量投射实施）。
- 026D 的实施以 026B/026C 口径稳定为前置条件，避免在不稳定契约上改造投射路径。

## 3. 需核实的前置事实（Facts to Verify）
- [X] 现有投射重建是否有“清表重放/幂等 upsert”约定与实现（作为 026B 重放策略的前提）。
  - 结论：采用同事务 delete+replay（清表重放），非 upsert。
  - 证据：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`；`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`。
- [X] API 输入校验是否保留原始输入（用于空格/\\t/全角空白允许语义与“非全空白”判定）。
  - 结论：多数入口保留原始输入；SetID UI 路径在 Normalize 前先 Trim，导致语义不一致。
  - 证据：`pkg/orgunit/resolve.go`；`internal/server/orgunit_nodes.go`；`internal/server/staffing_handlers.go`；`internal/server/orgunit_api.go`；`internal/server/setid.go`。
- [~] 迁移样本统计：历史 org_code 的长度/字符集分布是否覆盖 026B 约束（**豁免：无样本数据，审批人：我**）。
  - 现状：未在 `docs/dev-plans/` 或 `docs/dev-records/` 找到迁移样本统计记录；本次按豁免处理。
- [X] "ROOT" 是否为保留 org_code，若不是需修订示例。
  - 结论：未发现 “ROOT 为保留 org_code” 的规则或校验；root 语义由 parent_id 为空与 root_org_id 约束定义。
  - 证据：`docs/dev-plans/026b-orgunit-external-id-code-mapping.md`；`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`。

## 4. 方案内可直接修订的问题
### 4.1 文档状态与完成记录混用
- 026B 目前处于“草拟中”，但步骤含大量“已完成+已验证”记录。
- 修订建议：完成记录迁移到 `docs/dev-records/`，计划文档仅保留可执行步骤与状态。

### 4.2 org_code 校验语义不一致
- 旧版 026B 使用“trim + upper”，与“允许空格/\\t/全角字符”存在冲突风险。
- 修订建议：统一为 **仅 upper（不 trim）** + 白名单校验 + 长度 1~64 + 禁止全空白（空格/\\t/全角空白）；DB 约束改为 `org_code = upper(org_code)` + 白名单正则，避免语义冲突。

### 4.3 org_id 分配器边界与返回语义不一致
- `next_org_id` 上限与分配函数条件不一致；“返回值 + 自增”语义与实现对齐不足。
- 修订建议：统一边界为 `10000000~99999999`，并明确“返回分配值，next_org_id = 分配值 + 1”。

### 4.4 占位 org_code 缺少来源标记
- 026B 允许用格式化 org_id 作为占位，但无来源标记机制。
- 修订建议：补充“占位策略”与“来源标识”（字段或保留前缀），防止语义污染。

## 5. 风险评估（需证据支持）
- 字符集/长度限制可能与迁移数据不兼容（需样本证据）。
- "不可改码" 的长期现实性需评估（若存在，则需规划后续方案边界）。
- 投射失败“阻断但可重放”的机制已确认采用 delete+replay；风险聚焦在写放大与锁持有时长（见 7.8/026D）。
- Resolver 仅单体解析可能引发 N+1（需确认现有查询模式）。
- **全量回放写放大**：当前每次写入都触发租户级 delete+replay，会随事件规模线性增加锁持有时长与写入延迟。
  - 证据：`orgunit.submit_org_event` 写入后调用 `replay_org_unit_versions`；replay 内部删除 `org_unit_versions/org_trees/org_unit_codes` 并全量重放事件。
  - 参考：`modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql`；`modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`。

## 6. 修订原则
- 不改变 026B 核心边界：外部只见 org_code，内部只用 org_id。
- 文档与实现语义必须一致；不以实现猜测替代契约。
- 任何“可回放/可修复”承诺需具备明确流程或机制描述。
- 门禁与验证入口统一引用 SSOT（`AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md`）。

## 7. 修订方案（Proposed Changes）
### 7.1 文档治理
- 将 026B 的“完成记录”迁移至 `docs/dev-records/`，计划文档改为“待办 + 验收”格式。

### 7.2 校验与归一化链路
- 明确校验顺序：原始输入校验（白名单/长度/控制字符/非全空白）→ 归一化（upper，不 trim）→ 存储一致性校验。
- DB 约束仅保证存储值合法，不作为原始输入校验替代。
- 必改入口：SetID UI 不得在 Normalize 前先 Trim（需保留空格/\\t 的允许语义）。

### 7.3 org_id 分配器一致性修订
- 统一 `next_org_id` 取值边界并修正文档语义（返回值/自增规则一致）。
- 在 026B 中加入“示例：分配前后值变化”以避免误解。

### 7.4 投射重放策略（选定）
- 选定方案：**投射前清空 `org_unit_codes`，然后全量重放**（delete+replay，当前实现一致）。
- 不采用幂等 upsert（避免引入双轨逻辑与语义分叉）。
- 明确投射时间字段来源：**不将 `org_unit_codes.created_at/updated_at` 作为业务/审计时间依据**（保持技术字段语义；如需审计时间以事件时间为准）。

### 7.5 迁移占位策略（选定）
- 允许占位 org_code，但**必须**明确“占位标记”与“退出/纠偏机制”（避免语义污染）。
  - 若无法提供来源标记，则不得使用占位 org_code。
- 占位标记方案（草案）：使用固定前缀 `ZZ-`（仍需满足格式约束），并在迁移清单中记录 `source=placeholder`；当拿到真实外部 ID 时，按专项迁移清单替换并重放。

### 7.6 示例与保留字
- 若 `ROOT` 为保留码，写入“保留字规则”；若不是，调整示例避免误导。

### 7.7 批量解析支持
- 给出批量解析接口或联表查询建议，避免 UI/列表场景的 N+1 风险。

### 7.8 增量投射方案（已拆分为 026D）
> 目标：在不破坏 One Door 与事务一致性的前提下，减少“每次写入全量回放”的写放大与锁时长。

- 已拆分为独立计划：`DEV-PLAN-026D`（`docs/dev-plans/026d-orgunit-incremental-projection-plan.md`）。  
- 026D 已补齐以下可执行内容（本计划仅引用，不重复细节）：  
  1) **前置依赖**：明确 026B/026C 对齐与 `hierarchy_type` 状态。  
  2) **full_name_path 增量算法草案**：CREATE/RENAME/MOVE 的最小可执行策略。  
  3) **局部不变量校验 SQL 模板**：gapless 与末段 infinity 的局部校验。  
  4) **锁策略与收益说明**：默认保持租户级锁，收益来自写放大减少。  
  5) **回放权限收敛草案**：replay 仅 `orgunit_kernel` 可执行，并给出权限 SQL 草案。  
  6) **测试矩阵**：增量写入 vs 全量 replay 对照（结构/validity/full_name_path）。  
- **实施与验证仅在 026D 执行**，026C 不承载实现细节。  

## 8. 验收标准
- [x] 026B 已按本方案完成修订（语义一致、边界清晰）。
- [x] 关键歧义点（校验链路/分配语义/重放策略）有明确、可执行描述。
- [x] 迁移占位策略有明确规则或被显式禁止。
- [ ] 示例与保留字规则一致，无隐性假设。
- [ ] 相关实现与测试对齐修订后的契约（验证入口引用 SSOT）。
- [~] 迁移样本统计已完成并有结论；若无样本数据则按豁免记录（审批人：我）。

## 9. 实施步骤
1. [ ] 核实前置事实（投射重建、输入校验、迁移样本、ROOT 语义）。
2. [X] 修订 026B 文档：校验链路、分配语义、重放策略、占位策略、示例。
3. [X] 形成对应 `dev-records` 证据记录（如涉及已完成事项）。
4. [X] 修复 SetID UI 输入链路（避免 Normalize 前 Trim，确保空格/\\t 允许语义一致）。
5. [X] 评估并记录 N+1 风险（批量解析/联表方案是否必要）。
6. [X] 若修订涉及实现差异，提交最小代码变更与测试用例。
7. [X] 依据 SSOT 门禁进行验证并记录结果（见 `AGENTS.md`）。
8. [ ] 移交并推进 `DEV-PLAN-026D`（增量投射方案）的实施与验证（实现细节详见 026D）。

## 10. 交付物
- DEV-PLAN-026C（本文件）。
- DEV-PLAN-026B 修订稿（语义一致、边界清晰）。
- 相关 dev-records 证据记录（如涉及已完成事项）。
- 如需：最小实现与测试修订。
