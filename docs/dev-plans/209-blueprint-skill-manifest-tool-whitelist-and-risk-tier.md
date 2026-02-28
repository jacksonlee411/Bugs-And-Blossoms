# DEV-PLAN-209：200蓝图 Phase 3 Skill 契约化与工具白名单治理

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
承接 DEV-PLAN-208，将 AI 作业从 prompt 驱动收敛为 Skill registry 驱动，限制工具权限与输出结构。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 落地 `SkillManifest/SkillExecutionPlan/SkillExecutionResult/SkillValidationReport` 契约。
2. [ ] 建立 Skill Registry 生命周期（draft/validated/published/deprecated）。
3. [ ] 将工具调用绑定 `allowed_tools` 与 `risk_tier` 策略。
4. [ ] 建立 Skill 样本回归与发布前校验流程。

### 2.2 非目标
1. [ ] 不实现业务写提交。
2. [ ] 不允许未注册 Skill 参与高风险作业。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 3；里程碑映射：M9B。
- 输入依赖：DEV-PLAN-208。
- 后续输出依赖：DEV-PLAN-210/212。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-004`：未注册 Skill 不得绕过门禁直接执行。
[ ] `STD-008`：Skill 校验与工具白名单纳入 CI 可执行门禁。
[ ] `STD-011`：`skill_*` 错误码与拒绝提示契约一致。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 定义 Skill manifest schema 与 registry 存储模型。
2. [ ] 实现执行前白名单检查与 `skill_tool_not_allowed` 拒绝路径。
3. [ ] 实现输入/输出 strict schema 校验与证据哈希。
4. [ ] 建立 Skill 发布前回归流程与准入阈值。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make test`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] 高风险作业未注册 Skill 直接拒绝。
2. [ ] Skill I/O schema 违约不进入提交链路。
3. [ ] M9B 证据文档可回放。

## 8. 风险与缓解
1. [ ] Skill 文档膨胀。缓解：SKILL 骨架 + references 渐进披露。
2. [ ] 工具权限漂移。缓解：manifest 审核与自动校验双重门禁。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m9b-skill-schema-evidence.md`、`docs/dev-records/dev-plan-200-m9b-skill-tool-matrix-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `AGENTS.md`
