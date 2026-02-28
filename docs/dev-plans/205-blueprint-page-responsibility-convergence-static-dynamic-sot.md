# DEV-PLAN-205：200蓝图 Phase 1 页面职责收敛（Static Metadata × Dynamic Policy）

**状态**: 规划中（2026-02-28 16:50 UTC）

## 1. 背景与上下文
承接 DEV-PLAN-165/184，目标是完成“字段页静态主写、策略页动态主写、字典页候选池主写”收口。

## 2. 目标与非目标
### 2.1 核心目标
1. [ ] 删除同语义双写入口，页面职责与 SoT 分层一一对应。
2. [ ] 字段配置页保留动态镜像只读能力并提供跳转策略页入口。
3. [ ] 策略页成为动态策略唯一写入口。
4. [ ] 字典页只维护候选池事实源，不承载业务行为策略。

### 2.2 非目标
1. [ ] 不扩展新模块页面，仅聚焦 Org 现有入口。
2. [ ] 不重写运行时决策器算法。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 1；里程碑映射：M4。
- 输入依赖：DEV-PLAN-203/204、DEV-PLAN-165/184。
- 后续输出依赖：DEV-PLAN-206。

### 3.1 标准对齐（DEV-PLAN-005）
[ ] `STD-004`：页面收敛不保留 legacy 双写入口。
[ ] `STD-005/STD-006`（MUI 与前端单链路，承接 DEV-PLAN-103）：页面职责收敛保持单链路交付。
[ ] `STD-011`：页面提示明确区分 Static/Dynamic 来源与失败原因。

## 4. 关键设计（Simple > Easy）
1. [ ] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [ ] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [ ] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [ ] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [ ] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [ ] 梳理字段页/策略页/字典页字段级权限与可写项矩阵。
2. [ ] 移除或禁用双写按钮，改为只读镜像 + 来源标签。
3. [ ] 补齐页面联动：从字段页带 `capability_key+field_key+as_of` 跳转策略页。
4. [ ] 补齐 E2E：改静态来源 + 改动态策略 + 表单提交结果一致。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [ ] `make check routing`
  - [ ] `make check capability-route-map`
  - [ ] `make e2e`
  - [ ] `make check doc`

## 7. 验收标准
1. [ ] 任一语义仅有一个主写页面入口。
2. [ ] 用户可见来源标签，不再出现“改了但不生效”。
3. [ ] M4 收敛证据可追溯。

## 8. 风险与缓解
1. [ ] 用户迁移成本。缓解：保留镜像展示与跳转，不做硬删除。
2. [ ] 隐藏旧入口后 API 仍可写。缓解：API 层同步封禁并加门禁。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m4-page-convergence-evidence.md`（新增）
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- `docs/dev-plans/184-field-metadata-and-runtime-policy-sot-convergence.md`
