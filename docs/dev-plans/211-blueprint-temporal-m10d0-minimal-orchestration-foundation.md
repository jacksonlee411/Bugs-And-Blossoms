# DEV-PLAN-211：200蓝图 Phase 5 自建 Temporal M10D0 最小化落地

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
按 200 的“先最小化、后平台化”决策，仅建设编排必需能力，不提前引入生产级 HA/灾备负担。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 落地编排最小运行基线：Namespace 隔离、checkpoint/retry、dead-letter 人工接管。
2. [ ] 落地最小观测：队列积压、失败率、超时率、重试耗尽率。
3. [ ] 保持业务授权与 One Door 裁决边界不变。
4. [ ] 形成进入 M10D1 的可量化触发条件输入。

### 2.2 非目标
1. [ ] 不在本阶段要求生产级 HA 与灾备演练。
2. [ ] 不迁移 6.1 运行时组合链路到 Temporal。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 5；里程碑映射：M10D0。
- 输入依赖：DEV-PLAN-210。
- 后续输出依赖：DEV-PLAN-212（触发式平台化验收）。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-004`：M10D0 异常处置仅人工接管/前向修复，不引入旧链路兜底。
[ ] `STD-008`：最小化编排能力纳入可执行测试与门禁。
[ ] `STD-012`：Temporal 仅承载编排，不越权承载授权裁决。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 部署最小 Temporal 环境（dev/staging 隔离）。
2. [ ] 实现 workflow 主键 `conversation_id+turn_id+request_id` 幂等约束。
3. [ ] 接入 checkpoint 恢复与 dead-letter 人工接管流程。
4. [ ] 补齐运行测试与故障演练（最小范围）。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make test`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] M10D0 证据可证明最小编排能力已闭环。
2. [ ] 异常流程不会自动绕过到提交。
3. [ ] 与 AGENTS 早期阶段约束一致。

## 8. 风险与缓解
1. [ ] 最小化边界不清导致范围膨胀。缓解：严格限定 M10D0 范围并 gate review。
2. [ ] 队列积压不可见。缓解：最小观测指标必达。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m10d0-self-host-temporal-minimal-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `AGENTS.md`
