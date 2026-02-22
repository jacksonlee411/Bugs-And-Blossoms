# DEV-PLAN-070A：全局共享租户模式 vs 天然租户隔离模式专项调查（SetID/Scope Package）

**状态**: 草拟中（2026-02-22 16:40 UTC）

## 1. 背景与上下文 (Context)
- **来源**：基于 `DEV-PLAN-070/071/071A` 已落地方案，在评审中识别出中长期架构风险：当前“`global_tenant` + `SHARE`”模式可用，但与更严格的“每租户天然隔离”相比，未来在合规、扩展与治理上存在潜在压力。
- **当前口径（摘要）**：
  - `DEV-PLAN-070`：共享层与租户层物理隔离；共享读需显式开关；禁止 OR 合并读取。
  - `DEV-PLAN-071`：引入 `scope_code + package` 订阅层，并在 shared-only 场景通过受控函数读取共享包。
  - `DEV-PLAN-071A`：引入 `owner_setid` 以明确包编辑归属，业务编辑与订阅治理分离。
- **触发问题**：在多租户长期演进下，是否应继续以“共享租户运行时读取”为主，还是转向“共享数据发布到租户（天然隔离）”的主路径。

## 2. 调查目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 明确“全局共享租户”与“天然租户隔离”在合规、扩展、排障、审计上的差异与边界。
- [ ] 形成可比选项（至少 2-3 个）并给出推荐路线、前置条件与迁移成本。
- [ ] 输出对 `DEV-PLAN-070/071/071A` 的修订建议（含需要冻结/调整的契约条款）。
- [ ] 保持仓库不变量：One Door、No Tx No RLS、Valid Time（date）、No Legacy。

### 2.2 非目标
- 不在本计划内直接执行大规模 schema 重构或业务迁移。
- 不在本计划内引入新的业务 scope 或 UI 大改。
- 不替代 `DEV-PLAN-071B` 等后续功能计划，仅提供架构层调查与决策输入。

### 2.3 工具链与门禁（SSOT 引用）
- 文档阶段：`make check doc`。
- 若调查结论触发后续代码/迁移实施，按 `AGENTS.md` 触发器矩阵执行对应门禁。
- SSOT：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile`。

## 3. 调查范围与边界 (Scope)
### 3.1 in-scope
- SetID 解析链：`org_unit -> setid -> scope -> package`。
- shared-only 配置域的读取与订阅机制。
- `global_tenant` 模式下的 RLS、权限、审计与运营影响。
- `owner_setid` 在包所有权与编辑权限中的治理能力。

### 3.2 out-of-scope
- 非 SetID 控制域的独立业务模型。
- HR 业务规则本身（例如 Job Catalog 字段语义）正确性评估。

## 4. 现状基线（待核验清单）
1. [ ] 共享数据读路径是否全部通过受控函数/专用入口，无旁路 SQL。
2. [ ] shared-only 场景中 `app.current_tenant` 切换与恢复是否在异常路径可证明安全。
3. [ ] 关键接口是否存在 `current_date` 默认导致的回放口径漂移。
4. [ ] `owner_setid` 回填/变更流程是否可制度化，避免人工决策成为常态。
5. [ ] shared-only 与 tenant-only 的权限矩阵是否与 Casbin 对象一致且可审计。

## 5. 外部对标假设（Workday 公开资料）
> 说明：以下为公开资料可见口径，用于对标调查；不代表对 Workday 私有实现细节的断言。

- [ ] 验证“租户为主隔离单位”的公开描述与我们当前模式差异。
- [ ] 验证“上下文化安全（Contextual Security）”在 API 层的表现，并映射到本仓库权限模型。
- [ ] 验证“审计可追溯（谁在何时访问过什么）”能力与我们当前日志/事件口径差距。

## 6. 关键调查问题 (Research Questions)
### 6.1 合规与数据主权
1. 跨租户运行时共享读取（即使受控）是否会在审计/合规问卷中提高解释成本？
2. shared-only 数据是否存在“按地区/法域”进一步分区需求？若有，`global_tenant` 是否会成为瓶颈？

### 6.2 安全与权限
3. 当前 `RLS + app.allow_share_read + SECURITY DEFINER` 组合是否存在“上下文污染”与误配风险窗口？
4. `owner_setid` 权限是否应从“租户级角色”进化到“组织上下文 + 角色”双维控制？

### 6.3 扩展与性能
5. 共享表/函数是否会形成热点（高并发租户集中读共享包）？
6. 新增 stable scope 的回填与 bootstrap 是否可在大租户数量下保持可控时延？

### 6.4 可运维性与可审计
7. 现有证据链是否可清晰回答“某次读取为何命中共享包、由谁触发、在何 as_of 生效”？
8. 故障场景下，是否可在不引入 legacy 双链路的前提下快速止损与恢复？

## 7. 候选架构选项（调查对象，不是最终决策）
### 7.1 选项 A：维持 global_tenant 主模式（加强治理）
- 核心：保留现有共享读取路径，补强权限、审计、默认参数与异常恢复。
- 优点：迁移成本最低、对现有链路冲击小。
- 风险：中长期合规解释成本与共享热点风险仍在。

### 7.2 选项 B：共享数据“发布到租户”主模式（天然隔离优先）
- 核心：共享端只做发布源，租户运行时只读本租户数据；共享读取作为运营/治理路径而非业务主路径。
- 优点：隔离与合规表达最清晰，运行时边界简单。
- 风险：发布同步与版本治理复杂度上升，需要明确发布/回滚契约。

### 7.3 选项 C：混合模式（按 scope 分层）
- 核心：极少数公共字典保留共享运行时读取，其余 scope 采用发布到租户。
- 优点：兼顾迁移成本与隔离收益。
- 风险：模型复杂，需严格防止“策略漂移”。

## 8. 评估维度与打分准则
- **隔离强度**：租户边界是否天然成立，是否依赖运行时开关。
- **合规可解释性**：外部审计/客户问卷是否易解释。
- **安全稳健性**：误配、越权、上下文污染的潜在面。
- **可扩展性**：租户规模、scope 数量增长下的可持续性。
- **运维复杂度**：发布、回填、回放、排障路径复杂度。
- **迁移成本**：对现有 `070/071/071A` 代码与数据的改动量。

> 输出时采用 1-5 分（5 为最好），并记录评分依据与证据链接。

## 9. 调查方法与证据来源
### 9.1 内部证据
- 计划与执行记录：
  - `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
  - `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
  - `docs/dev-plans/071a-package-selection-ownership-and-subscription.md`
  - `docs/dev-records/dev-plan-070-execution-log.md`
  - `docs/dev-records/dev-plan-071-execution-log.md`
- 代码/Schema/权限核查（按需抽样）：`modules/orgunit/**`、`config/access/policy.csv`、`config/routing/allowlist.yaml`。

### 9.2 外部证据（公开资料）
- Workday SOAP API Reference（多租户 API 入口与版本）。
- Workday API 操作文档中 `Contextual Security` 字段（如 Staffing 相关操作）。
- Workday 官方安全与审计公开说明（Trust & Security / 官方事件通告）。

## 10. 里程碑与交付物
1. [ ] **M1（基线核对）**：完成当前实现与风险清单核验，形成“问题-证据”矩阵。
2. [ ] **M2（外部对标）**：完成 Workday 公开机制映射，形成“相同点/差异点/不可比点”。
3. [ ] **M3（选项评估）**：完成 A/B/C 三方案评分、成本估算、风险排序。
4. [ ] **M4（决策建议）**：输出推荐方案与分阶段落地建议（含对 070/071/071A 的修订条目）。

**交付物**：
- 调查结论文档（本文件持续更新）。
- 风险矩阵与评分表（可附录）。
- 若进入实施：新增后续计划（建议编号 `070B`）与执行记录文档。

## 11. 验收标准 (Acceptance Criteria)
- [ ] 每个关键风险均有证据来源（内部或外部），且可追溯。
- [ ] 至少 2 个候选架构被完整评估并给出明确取舍理由。
- [ ] 推荐方案与仓库不变量无冲突（One Door/No Tx No RLS/No Legacy/Valid Time）。
- [ ] 明确迁移边界：不引入双链路回退，回滚策略仍为环境级停写 + 修复后重试。
- [ ] 明确对 `070/071/071A` 的具体修订点（条目级别）。

## 12. 风险登记（调查阶段）
- **R1：证据不足风险** —— 公开资料粒度有限，可能无法覆盖私有实现细节。缓解：明确“可证/不可证”边界，避免过度推断。
- **R2：范围蔓延风险** —— 调查阶段混入实施细节。缓解：严格按本计划 out-of-scope 执行。
- **R3：策略摇摆风险** —— 未形成量化评估导致反复。缓解：统一评分维度与门槛，先证据后结论。

## 13. 关联文档
- `AGENTS.md`
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-records/dev-plan-070-execution-log.md`
- `docs/dev-records/dev-plan-071-execution-log.md`
