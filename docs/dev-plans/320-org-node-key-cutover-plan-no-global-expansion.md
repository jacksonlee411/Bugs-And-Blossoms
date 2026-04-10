# DEV-PLAN-320：Org 域 8 位非纯数字 `org_node_key` 一步切换方案（不扩大到全对象）

**状态**: 实施中（截至 2026-04-11：`P0/P1/P2` 与 `7.4A` rehearsal 工具链已完成；`P4` 中 Assistant / `internal/server` façade / Staffing Position 的 `org_node_key` 收口已完成；`P3/P5/P6` 仍未完成）

## 1. 背景

当前仓库中，Org 域是少数把内部结构标识深度嵌入树结构内核的模块：

- `org_id int` 既是内部结构主键，也是 `ltree` 路径标签、父子关系键、SetID 绑定键与多处读模型 join 键。
- Org 域存在 `node_path ltree`、`parent_id`、`path_ids int[]`、subtree move 等树结构热点路径。
- `org_id` 与用户常见的 8 位 `org_code` 视觉上高度相似，容易误读。

相比之下，其他业务对象当前并没有出现与 Org 同级别的树结构压力：

- Person 主要使用 `person_uuid`
- Staffing 中 Position / Assignment 主要使用 `position_uuid` / `assignment_uuid`
- JobCatalog 主要使用 `*_uuid`

因此，本计划只处理 Org 域：引入固定 8 位、非纯数字、无业务语义的 `org_node_key`，并一次性替换当前运行期 `org_id` 结构主键。

本计划明确不把 Org 的结构主键方案扩大到全对象。若未来其他域要采用类似方案，必须另起新计划并提供独立性能与结构证据。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 为 Org 域定义 `org_node_key` 标准：固定 8 位、非纯数字、无业务语义、唯一、自增分配。
- [ ] Org 域一步切换为 `org_node_key` / `parent_org_node_key`，不保留运行期双轨。
- [ ] 将 Org 内核，以及 SetID / Staffing / Assistant / `internal/server` 中**引用 Org 的链路**同步对齐到 `org_node_key`。
- [ ] 保持对外契约继续只暴露业务编码 `org_code`，不得回流暴露内部结构标识。

### 2.2 非目标

- 不在本计划内建立“全对象统一 8 位内部编码”。
- 不在本计划内把 Person、Position、Assignment、JobCatalog 等对象改为 `char(8)` 主键。
- 不在本计划内为其他模块新增 `*_node_key`。
- 不在本计划内推动“全仓外部标识收敛”或一并治理非 Org 对象的 `*_uuid` 对外暴露问题。
- 不在本计划内改变 Person / Staffing / JobCatalog / Assistant 各自对象的主键策略、历史策略或发布策略；这些模块只处理“引用 Org 时如何解析与传递 Org 标识”。
- 不保留切换前 Org 的历史事件账本、历史版本链与历史审计记录。
- 不通过兼容别名、双写双读或 legacy fallback 来实现平滑过渡。
- 不把内部编码设计成新的业务编码、可读编码或人工输入编码。

### 2.3 扩大化边界

本计划采纳以下边界判断：

1. Org 的 8 位内部结构键方案首先是为了解决树路径、父子结构、索引和 subtree move 的综合问题。
2. 其他业务对象当前没有看到与 Org 等价的树结构性能压力。
3. SetID / Staffing / Assistant / `internal/server` 在 320 中只允许修改“消费 Org 引用”的适配层，不得借机引入这些域自己的 `*_node_key`、新历史账本策略或额外 surrogate key 迁移。
4. 因此，Org 的结构主键方案不应直接扩大到其他业务对象。
5. 未来若其他域要引入类似方案，必须单独提交新的调查与实施计划，不得直接引用 320 外推。

### 2.4 与现行标准的关系（STD-003 已对齐）

现行标准 `STD-003` 已对齐为：

- 对外契约仅使用 `org_code`
- 内部结构关系仅使用 `org_node_key`

见：

- `docs/dev-plans/005-project-standards-and-spec-adoption.md`

因此，`DEV-PLAN-320` 的标识边界前置条件已在标准层完成同步；后续实施不得再回退到 `org_id` 口径。

本计划明确要求：

1. 320 的实施分支必须包含已修订的 `STD-003`
2. 标准口径要求 Org 内部结构键使用 `org_node_key`
3. 不得以“先改代码、后补标准”的方式推进

当前标准目标口径为：

- 对外契约仅使用 `org_code`
- Org 内部结构关系仅使用 `org_node_key`
- 请求进入服务边界时必须先做 `org_code -> org_node_key` 解析
- 对外响应回写标识时必须使用 `org_code`

## 2.5 工具链与门禁（SSOT 引用）

- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口与脚本实现：`Makefile`
- CI 门禁定义：`.github/workflows/quality-gates.yml`
- DB / DDD / No Legacy / Routing / 测试分层口径：
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/015-ddd-layering-framework.md`
  - `docs/dev-plans/015a-ddd-layering-framework-implementation-gap-assessment.md`
  - `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
  - `docs/dev-plans/300-test-system-investigation-report.md`
  - `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`

## 2.6 实施阶段冻结（7 阶段）

320 不按“普通重构”铺开，必须按以下 7 阶段推进；后续代码、工具、测试与发布准备均以本节为冻结口径：

1. `P0 契约冻结`
   - 唯一实施口径为本文。
   - 必须先锁定边界、历史不保留、发布 choreography、reopen 条件。
   - 退出条件：范围只剩 Org 域与“引用 Org 的链路”，不存在“顺手扩到其他对象”的开放项。
2. `P1 DB 基础设施`
   - 仅落 `org_node_key_seq`、`org_node_key_registry`、编码/分配函数、RLS/权限约束。
   - 本阶段不得切主、不得导流、不得保留运行期双轨。
3. `P2 当前态导出/导入工具链`
   - 必须提供“当前有效 Org 树快照”的导出器、核对器、导入器、结构校验器。
   - Dry-run 必须能反复执行并产出稳定核对结果。
4. `P3 Org 内核切换`
   - 在同一实施分支内完成 Org Schema、事件 payload、解析器、读写服务、审计适配的整域切主。
   - Org 内部运行时只允许保留 `org_node_key`。
5. `P4 消费方收口`
   - 只收口 SetID、Staffing、Assistant、`internal/server` 中对 Org 的引用边界。
   - 禁止借机修改这些模块自己的主键、历史模型、发布策略。
6. `P5 验收与门禁收口`
   - 必须按第 11.3 节对齐四大 Gate，并同步更新 `DEV-PLAN-060` 主链路测试。
7. `P6 预演与正式切换`
   - 必须先完成至少 1 次全流程 rehearsal，再进入正式维护窗口。
   - 正式窗口顺序固定为：`停写 -> 快照 -> 导入/核对 -> 后端 -> 前端 -> smoke -> reopen`。
   - 任一步失败，只允许数据库快照回滚，禁止临时开启 legacy。

## 2.7 P0 冻结产物

### 2.7.1 引用清单（实施范围）

- Org Kernel：
  - `modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql`
  - `modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql`
  - `modules/orgunit/infrastructure/persistence/schema/00004_orgunit_read.sql`
  - `modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql`
  - `modules/orgunit/infrastructure/persistence/orgunit_pg_store.go`
  - `modules/orgunit/services/orgunit_write_service.go`
  - `pkg/orgunit/resolve.go`
- SetID 引用 Org：
  - `modules/orgunit/infrastructure/persistence/schema/00005_orgunit_setid_schema.sql`
  - `modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`
  - `modules/orgunit/infrastructure/persistence/schema/00020_orgunit_setid_strategy_registry_schema.sql`
  - `modules/orgunit/infrastructure/persistence/setid_pg_store.go`
  - `pkg/setid/setid.go`
  - `internal/server/setid*.go`
  - `migrations/orgunit/20260222193000_orgunit_setid_strategy_registry_schema.sql`
  - `migrations/orgunit/20260225120000_orgunit_setid_strategy_org_applicability.sql`
  - `migrations/orgunit/20260410103000_orgunit_org_node_key_runtime_compat.sql`
- Staffing 引用 Org：
  - `modules/staffing/infrastructure/persistence/schema/00002_staffing_tables.sql`
  - `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`
  - `modules/staffing/infrastructure/persistence/schema/00015_staffing_read.sql`
  - `modules/staffing/infrastructure/persistence/position_pg_store.go`
  - `modules/staffing/domain/types/position.go`
  - `internal/server/staffing*.go`
- `internal/server` / Assistant / Presentation：
  - `internal/server/orgunit_nodes.go`
  - `internal/server/orgunit_api.go`
  - `internal/server/orgunit_field_metadata_api.go`
  - `internal/server/orgunit_mutation_capabilities_api.go`
  - `internal/server/assistant_*`
  - `internal/server/setid_explain_api.go`
- 工具链 / 验证：
  - `cmd/dbtool/**`
  - `docs/dev-plans/060-business-e2e-test-suite.md`
  - `e2e/tests/tp060-02-master-data.spec.js`
  - `e2e/tests/tp060-03-person-and-assignments.spec.js`

### 2.7.2 停写窗口方案

- 停写范围：
  - Org 一切写流；
  - SetID 绑定写流；
  - Staffing Position/Assignment 中依赖 Org 解析的写流；
  - Assistant 中一切会触发 Org 写入或读取待提交 Org 候选的写流。
- 停写实现要求：
  - 通过维护窗口或路由层显式阻断；
  - 停写期间允许只读核对、快照导出、导入和 smoke；
  - 停写解除条件只能是第 6.5 节 choreography 完整通过。

### 2.7.3 回滚负责人

- 数据库快照回滚负责人：Org 模块实施人。
- reopen 决策负责人：同一实施责任人，不得在失败后把回滚责任转移给消费方模块。
- 责任语义冻结：
  - 若导出核对失败、导入结构核对失败、smoke 失败或 stopline 失败，负责人必须执行数据库快照恢复并保持停写；
  - 不得以“先恢复服务、后补修复”的方式绕过整窗回滚语义。

### 2.7.4 Stopline 清单

- 未包含已修订 `STD-003` 的分支不得进入实施。
- 存在未收口的 `org_id` / `org_unit_id` 外部协议字段不得进入正式切换。
- 当前态导出总数、根数量、父子关系、`org_code` 唯一性任一不闭环，不得进入导入。
- 导入后 `node_path` / `path_node_keys` 不一致，不得 reopen。
- 任一 Gate-1~Gate-4 不通过，不得 reopen。
- 任一主读链路或 `move` 链路触发第 9.5 节 stopline，不得进入正式窗口。

## 2.8 当前完成度评估（2026-04-10）

本节用于区分三类状态，避免把“代码/工具链已落地”“本地 rehearsal 已跑通”“正式切主已执行”混为一谈：

- `已完成`：仓库代码、门禁或本地实库证据已经闭环，可由现有记录直接证明
- `部分完成`：已有部分实现或证据，但仍存在明确缺口，尚不能视为验收完成
- `未完成`：正式实现、正式验证或正式切换尚未发生

### 2.8.1 阶段总览

1. `P0 契约冻结`：已完成
   - 本计划已作为 SSOT 生效，且 `STD-003` 对齐口径已写入本计划与标准引用。
2. `P1 DB 基础设施`：已完成
   - `org_node_key` 目标态 bootstrap、分配/校验函数、`org_node_key_registry` 与相关约束已在 target bootstrap 中落地。
3. `P2 当前态导出/导入工具链`：已完成
   - `cmd/dbtool` 与 `scripts/db/orgunit-node-key-rehearsal.sh` 已形成 committed `export -> check -> bootstrap -> import -> verify` 闭环。
4. `P3 Org 内核切换`：未完成
   - 当前 source-real / 运行主链仍是旧 `org_id` 内核加 compat；正式 target-real 切主尚未执行。
5. `P4 消费方收口`：部分完成
   - SetID strategy registry 的 schema cutover / rehearsal 链路已完成。
   - Assistant 候选/响应、OrgUnit details 扩展字段快照 compat bridge、Staffing Position 的运行时/schema/DTO 已完成 `org_node_key` 收口，并继续只对外暴露 `org_code`。
   - consumer runtime 的真实 `target-real` explain、`DEV-PLAN-060` 业务链路套件与正式 Gate 闭环仍未完成。
6. `P5 验收与门禁收口`：部分完成
   - 已有 `make check org-node-key-backflow`、本地 stopline explain 与 rehearsal 证据。
   - `DEV-PLAN-060` 全链路业务测试、consumer runtime target-real explain、完整 Gate 收口仍未完成。
7. `P6 预演与正式切换`：部分完成
   - 已完成至少 2 次本地 source/target rehearsal。
   - 正式维护窗口、停写、发布 choreography 与 reopen 尚未执行。

### 2.8.2 已完成项

1. Org target bootstrap 已落地
   - 证据：`cmd/dbtool orgunit-snapshot-bootstrap-target`
   - 现状：支持 fresh target 自动应用 `00023-00025`，并在命中 SetID rehearsal/validate 时一并补齐 `00020-00022`
2. 当前态导出 / 导入 / 结构核对工具链已落地
   - 证据：`cmd/dbtool orgunit-snapshot-export/check/import/verify`
   - 现状：本地 source/target committed rehearsal 已闭环，记录见 `docs/dev-records/dev-plan-320-rehearsal-log.md`
3. SetID strategy registry 的 target schema 收口与 rehearsal 子链路已落地
   - 证据：`cmd/dbtool orgunit-setid-strategy-registry-export/check/import/verify/validate`
   - 现状：target stopline 已 fail-closed；fresh target-only 约束已生效；本地 committed rehearsal 已闭环
4. 反回流门禁已部分落地
   - 证据：`make check org-node-key-backflow`
5. 本地 stopline explain 证据已完成一轮采集
   - 证据：`docs/dev-records/dev-plan-320-stopline-log.md`
   - 归档：`docs/dev-records/assets/dev-plan-320-stopline/`
6. Assistant / `internal/server` façade / Staffing Position 的 committed 收口已落地
   - 证据：`go test ./modules/staffing/... ./internal/server -count=1`、`make check org-node-key-backflow`、`scripts/sqlc/verify-schema-consistency.sh`（在 dev postgres 容器 shim 下完成）
   - 现状：Position 持久化 schema / replay / snapshot 已切到 `org_node_key`，外部请求/响应继续仅使用 `org_code`

### 2.8.3 部分完成项

1. SetID / Staffing 消费方的性能与运行期收口
   - 已完成：Staffing committed schema/runtime 已切到 `org_node_key`，并补齐本地 `target-shadow` explain 证据
   - 未完成：consumer runtime 的真实 `target-real` explain 与正式 runtime 切主
2. SetID strategy registry 的 `business_unit` 真实数据分支验证
   - 已完成：代码、门禁、validate、tenant-only 实库 rehearsal，以及独立 `rehearsal/source + rehearsal/target` 的 `pass / unresolved / ambiguous` 三分支受控 rehearsal
   - 未完成：真实 source 数据当前仍为 `business_unit_rows=0`，尚未出现“source-real 自然携带 business_unit 当前态”的生产样本
3. 四大 Gate 收口
   - 已完成：局部门禁、dbtool 单测、文档与 stopline 证据
   - 未完成：`DEV-PLAN-060` 业务套件、跨模块验收与正式 Gate 对齐闭环

### 2.8.4 未完成项

1. Org source-real / runtime 主链的正式切主
2. consumer runtime 的真实 `target-real` explain 与最终契约验收
3. 正式维护窗口、停写、后端/前端发布与 reopen

### 2.8.5 当前结论

截至 2026-04-10，320 的真实完成状态应判断为：

1. 工具链与 rehearsal readiness 已明显前进，已具备进入更深一步 consumer/runtime 收口的条件。
2. 320 仍不能判定为“已完成”或“可正式切主”。
3. 当前最主要缺口不再是 Org target bootstrap 本身，而是：
   - Org source-real 到 target-real 的正式内核切主
   - consumer runtime 的真实 target-real 收口
   - 四大 Gate 与用户可见主链路的正式验收闭环

### 2.8.6 从当前状态到“可正式切主”的短清单

后续执行顺序压缩为以下 4 步；只有前一步完成并留痕后，才进入下一步：

1. 完成 Org source-real 到 target-real 的正式内核切主准备
   - 收口 Org kernel 仍残留的 `org_id` 运行路径
   - 确认 target-real 形态与 `00023-00025` / `path_node_keys` / 新账本初始化口径一致
   - 重新跑 source/target committed rehearsal，并把结果补入 `docs/dev-records/`
2. 完成 consumer/runtime 收口
   - SetID、Staffing、Assistant、`internal/server` 只保留 `org_node_key` 内部语义
   - 对外协议、前端状态、页面链路继续只暴露 `org_code`
   - 补齐真实 `target-real` explain，而不再只依赖 `target-shadow`
3. 完成 Gate 与业务验收收口
   - 补齐 11.2 与 11.3 所要求的测试面
   - 同步 `DEV-PLAN-060` 的用户可见主链路
   - 确认反回流门禁足以阻断 `org_id` 回流和 `org_node_key` 外露
4. 进入正式维护窗口前的最终 readiness review
   - 明确停写、数据库快照、发布顺序、smoke、reopen 条件与回滚负责人
   - 只有在正式切主 rehearsal、consumer/runtime、四大 Gate 都完成后，320 才能从“实施中”升级到“可正式切主”

## 3. 关键设计决策

### 3.1 决策 A：仅在 Org 域引入 `org_node_key`

选定方案：

- Org 结构键改为 `org_node_key`
- Org 父节点结构键改为 `parent_org_node_key`
- 其他模块不在本计划内跟进 `*_node_key`

理由：

- `org_node_key` 能明确表达“Org 内部结构键”，避免继续用 `org_id` 这种默认联想到数字型 ID 的命名。
- 收窄范围能显著降低切换风险，符合“只为有明确证据的域付出主键迁移成本”原则。

### 3.2 决策 B：不使用对象类型前缀

选定方案：**不使用对象类型前缀**。

本计划采纳用户判断：内部编码不承接业务语义，因此不应把 Org 类型语义编码进内部键本身。

因此：

- `org_node_key` 值本身不表达对象类型、租户、层级或时间语义。
- 键的归属由字段名、表名和约束表达，而不是由 key 字符串前缀表达。

### 3.3 决策 C：采用“非语义首字母 + 7 位 Base32 体”的 8 位编码

选定格式：

```text
[A-Z]{1}[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{7}
```

更精确地说：

- 第 1 位：`alpha_guard`
  - 字符集：`ABCDEFGHJKLMNPQRSTUVWXYZ`
  - 作用：保证 key 永远不是纯数字
  - 语义：无业务语义
- 第 2~8 位：`base32_body`
  - 字符集：`ABCDEFGHJKLMNPQRSTUVWXYZ23456789`
  - 作用：承载自增序列的编码体

示例：

- `AAAAAAAB`
- `AAAAAAAC`
- `AAAAAAAD`

### 3.4 决策 D：使用 Org 专属单序列分配

选定方案：

- 引入 `orgunit.org_node_key_seq`
- 新建 Org 时，从该 sequence 申请下一个 `seq`
- 由 Org 专属 DB 函数编码为 8 位 `org_node_key`
- 并写入 `orgunit.org_node_key_registry`

这意味着：

- `org_node_key` 仅保证 Org 域内唯一
- 该编码空间不与其他业务对象共享
- “自增”语义体现在 Org 专属 sequence，而不是字符串前缀

### 3.5 决策 E：不做双轨，Org 域一次性切换

选定方案：

- 不保留运行期 `org_id + org_node_key` 双轨写读
- 不保留 legacy alias、fallback、compat adapter
- 切换窗口内完成当前态导入、Schema 替换、Go 代码替换、跨模块联动与验证
- 回滚只允许数据库快照恢复与重新发起切换，不允许回到双链路常驻状态

### 3.6 决策 F：不保留历史数据，只保留切换时的当前有效组织树

选定方案：

- 320 **不迁移**切换前的 `org_events` 历史账本、`org_unit_versions` 历史版本链与历史审计记录
- 320 的唯一导入源是“停写窗口内导出的当前有效 Org 树快照”
- 切换后重新初始化符合 `org_node_key` 口径的新 Org 账本；旧账本不再参与 replay
- 切换后的 replay 仅针对新账本产生的事件成立，不承担旧 `org_id` 账本兼容

这意味着：

- 本计划不要求“旧 `org_events` 在新内核上继续可回放”
- 本计划不要求“旧 payload 中的 `parent_id/new_parent_id` 在新内核上兼容解释”
- 若未来需要保留或迁移历史账本，必须另起专门计划，不得在 320 实施阶段临时追加

## 4. 编码规范

### 4.1 字符集

- `alpha_guard`：`ABCDEFGHJKLMNPQRSTUVWXYZ`
- `base32_body`：`ABCDEFGHJKLMNPQRSTUVWXYZ23456789`

禁止字符：

- `0`
- `1`
- `I`
- `L`
- `O`

### 4.2 语法规则

- 长度固定 8
- 全大写
- 不允许空格、下划线、连字符、点号
- 正则：

```regex
^[ABCDEFGHJKLMNPQRSTUVWXYZ][ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{7}$
```

### 4.3 分配规则

设：

- `S` 为 `orgunit.org_node_key_seq` 返回的正整数
- `A = 24`
- `B = 32`

编码规则：

1. `guard_index = floor(S / B^7) mod A`
2. `body_value = S mod B^7`
3. `body_value` 以固定 7 位 Base32 编码，不足左补首字符
4. `org_node_key = alpha_guard[guard_index] || body[7]`

容量：

- `24 * 32^7 = 824,633,720,832`

对 Org 域容量来说足够大。

### 4.4 禁止事项

1. 禁止在应用层自行拼接/计算 `org_node_key`
2. 禁止把层级、租户、年份、环境等业务语义编码到 `org_node_key`
3. 禁止把 `org_node_key` 当作对外业务编码返回给用户
4. 禁止在运行时同时保留 `org_id` 与 `org_node_key` 双事实源

## 5. 数据模型与约束

### 5.1 Org 专属新增

```sql
orgunit.org_node_key_registry
- org_node_key char(8) primary key
- seq bigint not null unique
- tenant_uuid uuid not null
- created_at timestamptz not null default now()
```

另新增：

- `orgunit.org_node_key_seq`
- `encode_org_node_key(seq bigint)`
- `decode_org_node_key(org_node_key char(8))`
- `allocate_org_node_key(tenant_uuid uuid)`
- `org_path_node_keys(p_path ltree)`

约束要求：

- `org_node_key_registry` 只承担“分配登记 / 当前态导入核对 / 审计追踪”职责，**不得**承载 `org_code` 映射
- `org_node_key_registry` 必须启用并强制 RLS：`ENABLE ROW LEVEL SECURITY` + `FORCE ROW LEVEL SECURITY`
- `org_node_key_registry` 必须定义 `tenant_isolation` policy，遵循 `DEV-PLAN-021` 的 fail-closed 口径
- `allocate_org_node_key(tenant_uuid uuid)` 必须先执行 `assert_current_tenant(...)`
- `org_node_key_registry` 与 `allocate_org_node_key(...)` 的 owner / grant / `SECURITY DEFINER` / `search_path` 约束必须与现有 `org_id_allocators` 口径一致
- 若 `org_node_key_registry` 发生写入，写入口必须限定为 Org kernel；不得引入第二写入口

### 5.2 Org 域替换目标

当前 Org 域以 `org_id int` 为结构主键，并深嵌：

- `orgunit.org_trees.root_org_id`
- `orgunit.org_events.org_id`
- `orgunit.org_unit_versions.org_id`
- `orgunit.org_unit_versions.parent_id`
- `node_path ltree`
- `path_ids int[]`
- SetID 绑定与解析函数

本计划要求替换为：

- `root_org_node_key char(8)`
- `org_node_key char(8)`
- `parent_org_node_key char(8)`
- `path_node_keys text[] generated always as (orgunit.org_path_node_keys(node_path)) stored`
- `node_path ltree` 保留，但标签从“8 位数字”改为“8 位 `org_node_key`”

事实源约束：

- `node_path` 仍然是 Org 树结构的**唯一事实源**
- `path_node_keys` 仅作为由 `node_path` 派生出的祖先链辅助列，**不得**作为独立维护列直接写入
- `move` / `replay` / `rebuild` / 审计快照重建等路径只允许更新 `node_path`，不得同时手工维护第二份祖先链

### 5.3 Org Code 映射

`org_code` 仍是对外业务标识；其权威映射关系改为：

- `tenant_uuid + org_node_key -> org_code`
- `tenant_uuid + org_code -> org_node_key`

权威承载表要求：

- 以上映射必须由现有 `orgunit.org_unit_codes` 的演进版本承载，不得在 `org_node_key_registry` 中重复存储
- `orgunit.org_unit_codes` 应从当前 `(tenant_uuid, org_id)` 主键迁移为 `(tenant_uuid, org_node_key)` 主键
- `org_code` 唯一约束与现有 kernel-only write trigger 必须继续保留
- `org_node_key_registry` 不是 `org_code` 映射表，只用于分配登记与当前态导入核对

不得再以 `org_id` 作为对外映射桥梁。

### 5.4 跨模块引用替换

受 Org 影响的跨模块结构列同步改为 `org_node_key`：

- `staffing.position_versions.org_unit_id` -> `org_node_key`
- 与 SetID 绑定相关的 `org_id` 列 / 入参 / payload
- `internal/server` 中的 `ResolveOrgID/ResolveOrgCode` 风格接口

范围澄清：

- Staffing 不只是“内部联动”；当前 `Position` 外部输入/输出与 DTO 已直接暴露 `org_unit_id`
- 若 320 继续坚持“内部结构键完全不对外暴露”，则 Staffing 外部契约必须同步迁移为 `org_code`
- 迁移后的边界规则应为：Staffing 对外请求/响应使用 `org_code`，服务边界内解析为 `org_node_key`
- 不允许把 `org_node_key` 重新包装成名为 `org_unit_id` 的外部兼容别名继续长期存在；这会形成新的语义漂移
- 本节只处理这些链路中“对 Org 的引用字段 / 解析函数 / DTO / 页面状态”；不改变 Position / Assignment / Person / JobCatalog 自身主键、内部历史模型或独立发布边界

### 5.4A SetID Strategy Registry Schema Cutover

`orgunit.setid_strategy_registry` 是 320 当前仍未完成 schema 收口的关键 consumer：

- 表列名仍为 `business_unit_id`
- `business_unit` 作用域仍沿用 `^[0-9]{8}$` 旧约束
- 但运行态主链已经把该列当成 `business_unit_node_key` / `org_node_key` 语义在读写

这意味着当前状态仅是“运行时先切语义”，并未完成 DB SoT 收口；若继续维持，会形成：

- 列名表达旧语义、数据内容表达新语义的双重漂移
- schema 约束与应用校验不一致
- sqlc / explain / 运维核对仍会被误导为“8 位数字 BU 标识”

本计划对该表的目标态冻结如下。

#### 5.4A.1 目标列定义

- 列名：`business_unit_id` 一次性更名为 `business_unit_node_key`
- 类型：本轮保持 `text NOT NULL DEFAULT ''`
- 原因：当前租户级作用域以空串 `''` 作为哨兵值；320 本轮只收口“内部结构键语义”，**不**同时引入 `NULL` 化语义改造，避免把一次 schema cutover 扩成第二个数据模型重构

#### 5.4A.2 目标约束

- `org_applicability` 枚举继续保持：`tenant` / `business_unit`
- 旧约束 `setid_strategy_registry_business_unit_applicability_check` 必须重命名并重写为 `setid_strategy_registry_business_unit_node_key_applicability_check`
- 新约束逻辑冻结为：
  - `org_applicability = 'tenant'` 时，`business_unit_node_key = ''`
  - `org_applicability = 'business_unit'` 时，`orgunit.is_valid_org_node_key(btrim(business_unit_node_key)) = true`
- 不再允许使用 `^[0-9]{8}$` 作为本表的 BU 作用域校验

明确不做的事：

- 本轮**不**在该表上增加到 `orgunit.org_node_key_registry` 或 `orgunit.org_unit_codes` 的外键
- 原因：320 的 source/target choreography 中，SetID registry 需要在 target bootstrap 期间独立导入；完整一致性改由导入核对与 stopline 保证，而不是把 bootstrap 顺序耦合成运行期外键依赖

#### 5.4A.3 索引 / conflict key / 查询键

- 唯一键语义保持不变，但列参与项改为：
  - `(tenant_uuid, capability_key, field_key, org_applicability, business_unit_node_key, effective_date)`
- `ON CONFLICT` 键必须同步改为 `business_unit_node_key`
- `setid_strategy_registry_key_unique_idx` 可保留原索引名，但其定义必须切换到新列
- `setid_strategy_registry_lookup_idx` 的列集合保持不变；本轮不为 list 查询额外引入第二索引，避免在未完成正式 explain 基线前过早扩张索引面

#### 5.4A.4 数据迁移来源与回填原则

该表存量数据分两类处理：

1. `tenant` 作用域行：
   - 目标值固定为 `business_unit_node_key = ''`
2. `business_unit` 作用域行：
   - 目标值必须是合法 `org_node_key`

对存量 `business_unit` 行，320 冻结以下原则：

- **不允许**在 target DB 内根据旧 `business_unit_id` 文本做“猜测式转换”
- **不允许**新增一条长期存在的 compat 列 / compat 视图 / compat 触发器来保留旧语义
- source 导出物若仍只能提供旧 `business_unit_id`，则必须在导出阶段补齐“每条策略所对应的 `business_unit_org_code` 或已解析 `business_unit_node_key`”
- target 导入阶段统一解析 / 校验为 `business_unit_node_key`
- 若任一 `business_unit` 行无法无歧义落到唯一 `org_node_key`，则视为 stopline，整窗不得 reopen

换言之，正式 cutover 的权威迁移输入不再是“旧列原值原样搬运”，而是“已补足 Org 语义后的策略快照”。

#### 5.4A.5 Source / Target Choreography

`setid_strategy_registry` 的 schema cutover 必须服从 320 已冻结的 source / target choreography：

- `source-real`
  - 允许暂时保留旧列名 `business_unit_id`
  - 仅承担当前态导出、只读核对与 rehearsal 数据源职责
  - 不得因为 source 仍有旧列名，就把 target 也设计成继续保留旧名
- `target-real`
  - 作为正式切主库，schema bootstrap 后必须只存在 `business_unit_node_key`
  - 不得保留 `business_unit_id` 列、别名视图、同步触发器或双写适配

这与 320 “No Legacy / Big-Bang / target 切主后运行期单事实源”原则一致。

#### 5.4A.6 与 `20260410103000_orgunit_org_node_key_runtime_compat.sql` 的关系

`20260410103000_orgunit_org_node_key_runtime_compat.sql` 的职责边界冻结为：

- 为旧 `org_id` Org kernel 提供 `org_node_key` 运行时兼容函数
- 让应用主链在正式 Org DB cutover 前，能够先按 `org_code/org_node_key` 语义运行

它**不是**以下任一事项的依据：

- 不是 `setid_strategy_registry` 长期继续保留 `business_unit_id` 列名的依据
- 不是为 SetID registry 新增 DB 级 compat 视图/函数/触发器的依据
- 不是 target-real schema 保留旧正则约束的依据

正式 schema cutover 后：

- source-real 上的 compat migration 仅可作为 rehearsal/source 导出辅助
- target-real 上的 SetID registry 必须直接使用 `business_unit_node_key`

#### 5.4A.7 RLS / grant / ownership

- RLS policy、`ENABLE/FORCE ROW LEVEL SECURITY`、grant、sequence 权限保持不变
- 本轮只改“列语义 + 约束 + 索引定义”，不改变该表的租户隔离模型

#### 5.4A.8 文档收口说明

以下文档仍含旧 `business_unit_id` 口径，正式实现前必须按 320 的 schema 冻结口径逐步刷新：

- `docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md`
- `docs/dev-plans/161-org-create-dynamic-field-policy-on-capability-registry.md`
- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- `docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md`

在这些文档完成逐篇更新前，凡涉及 `setid_strategy_registry` schema 目标态的争议，以本节为 override SSOT。

### 5.5 非 Org 对象保持现状

以下对象不在本计划内变更其结构主键策略：

- `person.persons.person_uuid`
- `staffing.positions.position_uuid`
- `staffing.assignments.assignment_uuid`
- `jobcatalog.job_profiles.job_profile_uuid`
- `jobcatalog.job_families.job_family_uuid`
- `jobcatalog.job_family_groups.job_family_group_uuid`
- `jobcatalog.job_levels.job_level_uuid`

理由：

- 当前没有证据表明这些对象存在与 Org 等价的树结构性能需求。
- 不应为了“一致”而引入不必要的主键迁移成本。

## 6. 接口与边界契约

### 6.1 对外契约

对外继续维持现行规则：

- Org 只暴露 `org_code`
- 凡跨模块对外协议引用 Org，也只允许使用 `org_code`

禁止事项：

- 禁止外部 JSON/query/form 字段出现 `org_id`
- 禁止外部 JSON/query/form 字段出现 `org_node_key`
- 禁止继续把 `staffing.org_unit_id` 作为外部协议字段承载内部结构键
- 禁止 Assistant、内部 API、页面数据模型向前端回写 `org_id` 或 `org_node_key`

### 6.2 服务边界

Org 域新边界规则：

- 请求进入服务边界时：`org_code -> org_node_key`
- 领域与内核内部：只使用 `org_node_key`
- 响应回写时：`org_node_key -> org_code`

不得继续保留：

- `ResolveOrgID`
- `ResolveOrgCode(... orgID int)`
- `parent_id`
- `new_parent_id`

应统一替换为：

- `ResolveOrgNodeKeyByCode`
- `ResolveOrgCodeByNodeKey`
- `parent_org_node_key`
- `new_parent_org_node_key`

### 6.3 内部键完全对外隐藏

本计划要求不仅完成 `org_id -> org_node_key` 的内部切换，还必须做到：

- `org_id` 对外完全不可见
- `org_node_key` 对外完全不可见

这里的“对外”包括：

- HTTP JSON response
- query / form / path 参数
- Assistant 候选项与会话回包
- 前端页面状态、路由参数、隐藏字段与 API client 类型定义

#### 6.3.1 外部协议要求

1. 对外请求只允许使用业务标识 `org_code`
2. 对外响应只允许返回 `org_code`，不得返回 `org_id` 或 `org_node_key`
3. 同时携带 `org_code` 与任一内部键视为契约违规，必须 fail-closed

#### 6.3.2 分层要求

1. Handler / Controller：
   - 只接收和返回 `org_code`
   - 不得把 `org_id` / `org_node_key` 写入 response DTO
2. Service：
   - 仅在服务边界内做一次 `org_code -> org_node_key` 解析
   - 进入服务内部后统一使用 `org_node_key`
3. Repository / Store：
   - 只认内部结构键，不认对外业务标识
4. Frontend / Presentation：
   - 不得在页面状态、URL、隐藏字段中持有 `org_id` / `org_node_key`

### 6.4 历史数据与账本切换契约

320 在历史数据上的明确口径如下：

1. 切换前的 Org 历史事件、历史版本链、历史审计记录**不保留**
2. 切换窗口内仅导出“当前有效组织树快照”作为导入源
3. 导入源至少包含：
   - 当前有效节点集合
   - 父子关系
   - `org_code`
   - `name`
   - `status`
   - `is_business_unit`
   - 其余切换后运行所必需的当前态字段
4. 切换后新账本从导入基线重新开始；旧 `org_id` 事件账本不再 replay
5. 若当前态快照校验失败、导入后树结构校验失败或链路验收失败，则不得 reopen 写流

### 6.5 发布 choreography 冻结契约

320 的发布 choreography 不是“建议顺序”，而是冻结后的实施契约：

1. 进入维护窗口并停写 Org 写流，以及一切引用 Org 的写流
2. 固化切换前数据库快照，仅作为整窗回滚介质，不作为运行期兼容双轨的数据源
3. 在停写状态下导出“当前有效 Org 树快照”，并完成只读核对
4. 完成 `org_node_key` 分配、导入、新账本初始化与结构核对
5. 完成 SetID strategy registry 快照导出、`business_unit_node_key` 回填/校验与 target schema bootstrap
6. 先发布后端，再发布前端；维护窗口内禁止新旧代码与新旧 Schema 长时间混跑
7. 完成 smoke、主链路验收、性能 stopline 后，才允许 reopen 写流
8. 任一步失败都只能回到“数据库快照恢复 + 保持停写 + 修复后整窗重来”，不得临时启用 legacy、双写双读、旧账本 replay 或兼容别名窗口

冻结要求：

- 以上顺序、导出源、reopen 条件、失败恢复语义均视为 320 的冻结口径
- 若实施前需要修改 choreography，必须先更新 320 并重新评审；不得在维护窗口内临时改口径

## 7. 实施步骤（Big-Bang，一次发布窗口）

### 7.1 前置冻结

1. [ ] 确认实施分支已包含修订后的 `STD-003`，并与 320 口径一致。
2. [ ] 冻结 `STD-003` 的 Org 专项补充口径，不得在实施阶段再次回退到 `org_id` 标准。
3. [ ] 新增 `DEV-PLAN-320` 并评审通过。
4. [ ] 明确停写窗口、当前态导出策略、回滚负责人、reopen 写流条件和 stopline。

### 7.2 Org 专属基础设施

1. [ ] 新增 `orgunit.org_node_key_seq`
2. [ ] 新增 `orgunit.org_node_key_registry`
3. [ ] 新增 Org 专属 DB 函数：
   - `allocate_org_node_key(...)`
   - `encode_org_node_key(seq bigint)`
   - `decode_org_node_key(org_node_key char(8))`
   - `org_path_node_keys(p_path ltree)`
4. [ ] 为 `org_node_key_registry` 增加 RLS / policy / owner / grant / `SECURITY DEFINER` / `search_path` 约束，并与 `DEV-PLAN-021/025` 对齐
5. [ ] 增加 DB 级唯一约束与格式校验

### 7.3 当前态导出与停写

补充冻结说明：

- rehearsal/source 库导出必须使用 owner / bypass-RLS 级连接；当前 dev runtime 的 `app_runtime` 连接无法跨租户执行全量快照导出。

1. [ ] 进入维护窗口，阻断 Org 写流与引用 Org 的写流
2. [ ] 在停写状态下导出每个租户的“当前有效 Org 树快照”
3. [ ] 对导出结果执行只读核对：
   - 总数一致
   - 空值为 0
   - 重复为 0
   - 根节点数量符合约束
   - 父子关系无悬挂
4. [ ] 仅在导出核对通过后允许进入下一阶段

### 7.4 基线导入与新账本初始化

补充冻结说明：

- rehearsal 必须采用 source / target 双库 choreography：
  - source：当前旧 `org_id` 运行库（允许带 compat 适配，只负责导出当前态）
  - target：专用 `org_node_key` 目标库（负责 schema bootstrap、导入与 verify）
- 不得在 source 运行库原地执行 import / verify。
- 2026-04-10 本地 rehearsal 记录：`docs/dev-records/dev-plan-320-rehearsal-log.md`
- 2026-04-10 本地 stopline explain 记录：`docs/dev-records/dev-plan-320-stopline-log.md`
- 2026-04-10 explain 证据归档：`docs/dev-records/assets/dev-plan-320-stopline/`

1. [ ] 基于当前态快照为所有当前有效 Org 分配 `org_node_key`
2. [ ] 将分配结果写入 `orgunit.org_node_key_registry`
3. [ ] 清理旧 Org 运行表中的历史数据，不保留旧 `org_id` 账本
4. [ ] 以当前态快照重建新 `org_trees`、`org_unit_versions`、`org_unit_codes`
5. [ ] 初始化切换后的新 Org 账本；旧 `org_events` 不再参与 replay
6. [ ] 对导入结果执行结构核对：
   - 节点总数一致
   - `org_code` 唯一约束成立
   - `node_path` / `path_node_keys` 一致
   - 根/父子关系一致

### 7.4A SetID Strategy Registry Target Bootstrap

1. [ ] 从 source-real 导出 SetID strategy registry 当前有效快照，导出物必须显式包含：
   - `tenant_uuid`
   - `capability_key`
   - `field_key`
   - `org_applicability`
   - `effective_date/end_date`
   - `business_unit_org_code` 或已解析的 `business_unit_node_key`
2. [ ] target-real 的 `orgunit.setid_strategy_registry` schema 直接以 `business_unit_node_key` 建表；不得先建旧列再长期保留 compat
3. [ ] 若迁移链必须覆盖“已有表升级”场景，则新 goose migration 只能执行：
   - `RENAME COLUMN business_unit_id TO business_unit_node_key`
   - 删除旧数字正则约束
   - 重建为 `orgunit.is_valid_org_node_key(...)` 约束
   - 以新列重建唯一键/冲突键定义
4. [ ] target 导入前执行数据预检：
   - `tenant` 行必须全部为空串
   - `business_unit` 行必须全部是合法 `org_node_key`
   - `business_unit_node_key` 必须能在 target 的 Org 当前态映射中解析到唯一节点
   - 本地执行入口可使用：`scripts/db/orgunit-setid-strategy-registry-validate.sh --url <target-url> --as-of <YYYY-MM-DD>`
5. [ ] 预检失败即 stopline；不得通过临时恢复旧列名/旧正则/旧接口绕过
6. [ ] 导入完成后执行 target verify：
   - 行数与 source 快照一致
   - 唯一键无冲突
   - 不存在非法 `business_unit_node_key`
   - 关键 explain 与 upsert/disable/list 主查询命中新 schema 键
   - 本地 rehearsal 可通过 `scripts/db/orgunit-node-key-rehearsal.sh --source-url <source> --target-url <target> --as-of <YYYY-MM-DD> --rehearse-setid-strategy-registry --validate-setid-strategy-registry` 串行执行 `source export -> snapshot check -> target import -> target verify -> stopline validate`
   - 在未传 `--skip-bootstrap` 的 fresh target 路径上，脚本必须自动补齐 320 target 预置：`00023-00025` Org node-key bootstrap，并在启用 SetID registry rehearsal/validate 时一并补齐 `00020-00022` 的 registry target schema

### 7.5 Org 域切主

1. [ ] 用 `org_node_key` 替换 Org Schema 中所有结构主键与父子键
2. [ ] 将 `ltree` 标签函数从 `org_id` 版改为 `org_node_key` 版
3. [ ] 将 `path_ids int[]` 改为由 `node_path` 派生的 `path_node_keys text[]`
4. [ ] 将 `org_unit_codes` 迁移为 `(tenant_uuid, org_node_key) <-> org_code` 的唯一权威映射表
5. [ ] 将事件 payload 中的 `parent_id/new_parent_id` 全部改为 `parent_org_node_key/new_parent_org_node_key`
6. [ ] 重写 Org 解析器、写服务、读模型、搜索与审计适配

### 7.6 跨模块联动与发布 choreography

1. [ ] SetID 绑定和解析全量改为 `org_node_key`
2. [ ] SetID strategy registry 的 Repository / Store SQL 全量切到 `business_unit_node_key`
3. [ ] SetID strategy registry API / DTO 对外继续只暴露 `business_unit_org_code`
4. [ ] Staffing 内部对 Org 的引用全量改为 `org_node_key`
5. [ ] Staffing 外部协议从 `org_unit_id` 收敛为 `org_code`
6. [ ] `internal/server` 删除 `org_id` 中心 DTO 与解析接口
7. [ ] Assistant 候选对象去除 `org_id` 暴露与依赖
8. [ ] 对外 DTO、前端 API 类型与页面状态中彻底删除 `org_id` / `org_node_key` 暴露
9. [ ] 发布顺序固定为：DB schema/数据导入完成 -> 后端发布 -> 前端发布 -> smoke 验收 -> reopen 写流
10. [ ] 维护窗口内不允许新旧后端与新旧 DB 长时间重叠运行；若版本不一致，必须保持停写
11. [ ] 只有在 smoke、主链路验收、性能 stopline 均通过后，才允许 reopen 写流
12. [ ] 切换前数据库快照只作为整窗回滚介质，不得被接回运行期双轨或旧账本 replay
13. [ ] 若发布顺序、导出源或 reopen 条件需要变更，必须先更新 320 并重新评审

### 7.7 失败语义与恢复

1. [ ] 任一导出核对失败：停止切换，保持停写，修复后重新导出
2. [ ] 任一导入结构核对失败：恢复数据库快照，不 reopen 写流
3. [ ] 任一 smoke / 主链路验收 / stopline 失败：恢复数据库快照，不 reopen 写流
4. [ ] 恢复后只允许在修复并重新完成全量验收后再次发起切换

### 7.8 切换后清理

1. [ ] 删除 `org_id` allocator、旧解析函数、旧 JSON 字段
2. [ ] 删除残留 `ResolveOrgID` / `ResolveOrgCode(orgID int)` 接口
3. [X] 新增反回流门禁 `make check org-node-key-backflow`，阻断 `org_id` 再进入运行期路径

## 8. 风险与策略

### 8.1 高风险

1. **Org Kernel 改动面极大**
   - 风险：`org_id` 深嵌 Org SQL 函数、投射与 SetID 逻辑，一次切换容易漏点。
   - 策略：必须用停写窗口 + 当前态快照导出 + 映射核对 + 端到端回归来执行。

2. **跨模块联动点多**
   - 风险：Staffing / Assistant / `internal/server` / SetID 任一链路遗漏，都会造成运行期断链。
   - 策略：实施前必须先产出全仓引用清单；切换后按链路逐项验证。

3. **不保留历史数据意味着切换失败只能整窗回退**
   - 风险：一旦导出、导入或链路验收失败，不能依赖旧账本 replay 进行局部修复。
   - 策略：回滚仅允许数据库快照恢复；上线窗口必须足够长，且切换前完成 dry-run 与当前态导出演练。

4. **现行标准未先修订会导致计划与 SSOT 冲突**
   - 风险：若实施分支未包含已修订的 `STD-003`，或代码仍按 `org_id` 口径推进，320 的实现将与现行标准直接冲突。
   - 策略：把“实施分支必须包含已修订 `STD-003`”设为硬前置条件；未对齐不得进入实施。

5. **SetID registry 的旧列名与旧正则可能掩盖脏数据**
   - 风险：source-real 中若仍混有旧 `business_unit_id` 存量值，直接 rename + 放行会把错误数据带入 target-real。
   - 策略：正式 cutover 前必须通过快照导出把每条策略补齐到 `business_unit_org_code` 或 `business_unit_node_key`；无法无歧义解析的记录一律 stopline。

### 8.2 中风险

1. **无前缀方案在日志里不如“类型前缀”直观**
   - 风险：排障时单看 key 无法直接知道对象类型。
   - 策略：通过字段名、表名与日志上下文字段表达类型；不得把类型再编码回 key 本身。

2. **8 位空间终有上界**
   - 风险：虽空间充足，但不是无限。
   - 策略：在 Org registry 中保存 `seq` 原值，并预留未来扩容到 9/10 位的单独计划，不在 320 内提前引入 v2 双制式。

3. **路径数组从 `int[]` 切到 `text[]` 后索引特性变化**
   - 风险：树路径查询、祖先匹配与审计 join 计划可能退化。
   - 策略：切换前完成 explain 基线；必要时为 `path_node_keys` 建新索引并补专项压测。

## 9. 性能专项与 Stopline

### 9.1 当前性能事实

当前 Org 树路径的核心结构不是“整数树”，而是：

- `node_path ltree`：每层 label 为固定 8 位数字字符串
- `path_ids int[]`：由 `node_path` 解析出的祖先数组
- `GIN(path_ids)`：用于祖先链辅助查询

因此，切换到 `org_node_key` 后的性能影响应分开评估：

1. **`ltree` 主路径能力**
   - 当前 label 已是固定 8 字符，只是内容为数字。
   - 切换后若仍保持固定 8 字符，`<@`、`nlevel()`、prefix 拼接等主路径能力预计接近持平。

2. **`path_ids int[]` 辅助链路**
   - 该链路会从 `int[]` 改为 `text[]`
   - `GIN(text[])` 体积、缓存命中、比较开销与 `unnest -> join` 代价预计都会高于当前 `int[]`
   - 这是本计划最主要的性能风险点

3. **普通等值 join**
   - `org_id = ...` / `parent_id = ...` 将变为 `org_node_key = ...` / `parent_org_node_key = ...`
   - 固定 8 字符等值比较通常仅带来小幅退化，不预计成为首要瓶颈

### 9.2 必测链路

切换前后必须对以下链路分别采集基线：

1. [X] 树根列表查询
2. [X] 指定父节点下的 children 查询
3. [X] 节点详情查询
4. [X] 搜索候选查询
5. [X] 子树 move
6. [X] `full_name_path` 重建
7. [ ] SetID 基于组织祖先链的解析
8. [ ] Staffing 通过组织引用联查 position

补充说明：

- 2026-04-10 已完成本地 `source-real` + `target-real` 的 Org 主链路 stopline 采集，证据见 `docs/dev-records/dev-plan-320-stopline-log.md`。
- `SetID` / `Staffing` 已补齐 `target-shadow` explain：
  - dedicated target 内使用 shadow 表承载当前态样本
  - 样本通过 `org_code -> org_node_key` 映射导入
  - 该证据只用于 stopline 对比，不等于 consumer runtime 已完成 cutover
- 因此，`SetID` / `Staffing` 的“真实 target-real explain”仍待对应 schema/runtime 切主后补齐。

### 9.3 Explain / Analyze 基线清单

实施前后都必须保存 `EXPLAIN (ANALYZE, BUFFERS)` 证据，至少覆盖：

1. [X] `node_path <@ ...` 子树过滤查询
2. [X] `path_ids` / `path_node_keys` 祖先链展开查询
3. [X] `full_name_path` 更新 SQL
4. [X] `move` 对整棵子树的版本切分与重写 SQL
5. [X] 详情页主查询
6. [X] 搜索候选主查询

补充说明：

- 本地 explain 证据已归档至 `docs/dev-records/assets/dev-plan-320-stopline/`。
- 当前样本未出现“大范围 seq scan 且无法修复”的 stopline 信号，但这仍不替代正式切窗前后的完整环境复测。

### 9.4 建议的性能补偿策略

1. [ ] 保留 `ltree`，不要因切换 `org_node_key` 而退回邻接表递归
2. [ ] `org_node_key` 保持固定 8 位，避免路径 label 变长
3. [ ] 为 `path_node_keys text[]` 建立替代索引，并与当前 `GIN(path_ids)` 做对比验证
4. [ ] 若祖先链展开显著退化，优先评估“缓存祖先显示链/增量维护 `full_name_path`”，而不是回退到 legacy 双链路
5. [ ] 对 `move` 场景单独做大子树压测，确认写放大仍在可接受范围内

### 9.5 Stopline

以下任一条件触发，320 不得进入执行或必须暂停切换：

1. [ ] 树根列表、children、详情、搜索任一主读链路的 P95 延迟较基线退化超过 20%
2. [ ] `move` 大子树场景的总执行时间较基线退化超过 30%
3. [ ] `full_name_path` 重建 SQL 的总耗时或 shared buffers 读写量明显失控，无法通过索引/SQL 收敛
4. [ ] `path_node_keys` 索引体积或 vacuum 成本显著超出当前容量预算
5. [ ] Explain 结果显示关键查询不再命中 `ltree` gist / 预期数组索引，转为大范围 seq scan 且无法在计划内修复
6. [ ] 实施分支未包含已修订的 `STD-003`，或仍按 `org_id` 口径编码
7. [ ] 当前态导出结果与导入后结构核对无法闭环
8. [ ] `setid_strategy_registry` 任一 `business_unit` 作用域记录无法校验为合法且可解析的 `business_unit_node_key`
9. [ ] target-real 仍保留 `business_unit_id` 列、数字正则约束，或依赖 compat 视图/触发器维持旧语义

## 10. 反回流门禁

实施完成后，至少新增以下门禁：

- 已落地入口：`make check org-node-key-backflow`

1. [X] 禁止对外 DTO 出现 `json:"org_id"` / `json:"org_node_key"`
2. [X] 禁止新增 `ResolveOrgID` / `ResolveOrgCode(...int)` 风格接口
3. [X] 禁止 Org 域运行时再写入 `parent_id` / `new_parent_id`
4. [ ] 禁止应用层自行生成 `org_node_key`
5. [ ] 禁止 `internal/server` 再持有模块内 PG store 的 `org_id` 中心实现
6. [ ] 禁止前端页面状态、路由参数和 API client 类型引入 `org_id` / `org_node_key`
7. [ ] 禁止在 Org 域之外新增 `*_node_key`，或把 320 作为“全对象统一内部编码”的先例直接外推

## 11. 测试与覆盖率

### 11.1 覆盖率口径

- 覆盖率口径与测试分层基线以：
  - `docs/dev-plans/300-test-system-investigation-report.md`
  - `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
  为准。
- 涉及覆盖率统计范围、测试触发条件、paths-filter 触发范围的任何调整，必须遵循 `docs/dev-plans/226-test-guide-tg004-gate-caliber-change-approval.md`；不得通过改门禁口径替代补测试或删死分支。

### 11.2 本计划必须覆盖的测试面

1. [ ] `org_node_key` 编码/解码纯函数测试
2. [ ] Org sequence 分配唯一性测试
3. [ ] Org registry 一致性与当前态导入核对测试
4. [ ] Org tree create/move/rename/disable/enable/correct/rescind 集成测试
5. [ ] SetID 基于 `org_node_key` 的解析集成测试
6. [ ] SetID strategy registry schema cutover 测试：
   - 表升级迁移测试
   - 非法 `business_unit_id` 存量 stopline 测试
   - `business_unit_node_key` upsert / disable / list / resolve 查询测试
7. [ ] Staffing 引用 `org_node_key` 的联动测试
8. [ ] Assistant / internal API 不暴露 `org_id` / `org_node_key` 的响应契约测试
9. [ ] 当前态导出 / 导入 / 结构核对测试
10. [ ] 路由、lint、DDD layering、No Legacy 与相关反回流门禁测试
11. [ ] `DEV-PLAN-060` 对应全链路业务测试套件同步更新，至少覆盖 Org / SetID / Staffing / Assistant 一条用户可见、仅暴露 `org_code` 的端到端主链路

### 11.3 与 DEV-PLAN-012 四大 Gate 的对齐

- Gate-1 `Code Quality & Formatting`：承载文档更新、No Legacy、DDD layering、错误提示、反回流脚本与生成物一致性检查。
- Gate-2 `Unit & Integration Tests`：承载 `org_node_key` 编解码、当前态导入核对、Org/SetID/Staffing/Assistant 集成测试与 100% coverage 单主源执行。
- Gate-3 `Routing Gates`：承载路由 allowlist / 分类 / responder 契约收敛，确保对外不再接受或回写 `org_id` / `org_node_key`。
- Gate-4 `E2E Tests`：承载 `DEV-PLAN-060` 同步后的用户可见主链路验证，确认外部入口只使用 `org_code`，且切换后主路径可操作。

## 12. 验收标准

1. [ ] 任意新建 Org 都能拿到唯一 8 位 `org_node_key`
2. [ ] Org 域运行时不再依赖 `org_id`
3. [ ] `internal/server` 与对外响应不再暴露 `org_id`
4. [ ] `org_node_key` 也未暴露到任何外部协议、前端状态或 Assistant 回包中
5. [ ] 旧 Org 历史数据已按计划丢弃，切换后仅保留当前态重建出的新账本
6. [ ] Org 主链路、SetID、Staffing、Assistant 回归通过
7. [ ] `orgunit.setid_strategy_registry` 已完成 schema SoT 收口：目标库中只存在 `business_unit_node_key`，不存在 `business_unit_id` 旧列
8. [ ] 无 legacy 双轨、无 fallback、无兼容别名窗口
9. [ ] 边界保持收敛：320 只影响 Org 及其他模块中的 Org 引用面，未把 `*_node_key` 扩大到 Person / Position / Assignment / JobCatalog 等非 Org 对象
10. [ ] 发布 choreography 已按 6.5 冻结执行：仅允许“停写 -> 当前态导出 -> 导入/核对 -> 后端 -> 前端 -> smoke -> reopen”，失败仅允许整窗回滚
11. [ ] 门禁与验收已对齐 `DEV-PLAN-012` 四大 Gate 口径；相关变更未通过调整覆盖率/触发范围规避问题，符合 `TG-004`
12. [ ] 文档地图、`DEV-PLAN-060` 套件与相关反回流门禁已更新；后续新增代码无法把 `org_id` 回流到运行期主路径，也无法把 `org_node_key` 暴露到外部协议

## 13. 最终结论

本计划采纳以下最终口径：

- 仅在 Org 域引入新的内部 surrogate key：`org_node_key`
- 编码规则为“8 位、非纯数字、无业务语义、Org 域内唯一、自增分配”
- **不使用对象类型前缀**
- 通过“非语义首字母保护位 + 7 位 Base32 编码体”满足“8 位非纯数字”要求
- Org 域一次性切换到 `org_node_key`
- Person / Position / Assignment / JobCatalog 保持现状，不因 320 被扩大化改造
