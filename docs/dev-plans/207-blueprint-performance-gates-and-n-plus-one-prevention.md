# DEV-PLAN-207：200蓝图 Phase 2 性能停止线与反 N+1 门禁收口

**状态**: 已完成（2026-02-28 18:55 UTC）

## 1. 背景与上下文
200 已定义性能停止线（事务<=3、SQL<=10、禁止字段级 N+1）；本计划将其转成可持续回归门禁。

## 2. 目标与非目标
### 2.1 核心目标
1. [X] 建立组合快照查询/事务计数基线与回归测试。
2. [X] 实现策略决议批量化接口，禁止按字段逐次调用。
3. [X] 固化压测证据格式（P50/P95/QPS/查询数/事务数）。
4. [X] 停止线不满足时触发阻断与整改流程。

### 2.2 非目标
1. [X] 不引入外部缓存（Redis/Ristretto/BigCache）。
2. [X] 不做过度运维化指标扩张。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 2；里程碑映射：M8。
- 输入依赖：DEV-PLAN-206。
- 后续输出依赖：DEV-PLAN-212（评测门禁）。

### 3.1 标准对齐（DEV-PLAN-005）
- [X] `STD-004`：性能问题处置采用前向修复，不回退旧实现。
- [X] `STD-008`（门禁可执行，承接 DEV-PLAN-012）：性能停止线进入 CI/`make preflight`。
- [X] `STD-011`：性能退化与阻断提示使用明确错误码/原因。

## 4. 关键设计（Simple > Easy）
1. [X] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [X] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [X] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [X] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [X] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [X] 在关键接口增加查询计数/事务计数采样与测试断言。
2. [X] 改造策略决议为 batch 输入输出（`field_key IN (...)`）。
3. [X] 搭建基准压测脚本并落盘结果到 `docs/dev-records/`。
4. [X] 把性能回归接入 `make preflight`（或对应 CI required checks）。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [X] `make test`
  - [X] `make preflight`
  - [X] `make check doc`

## 7. 验收标准
1. [X] 主加载链路不存在字段级 N+1 回流。
2. [X] 停止线指标在回归样本下稳定达标。
3. [X] 压测与回归证据可审计。

## 8. 风险与缓解
1. [X] 测试环境数据规模不足。缓解：固定规模数据集与采样基线。
2. [X] 为过线引入隐式缓存副作用。缓解：坚持 request-scope/短TTL 原生方案。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m8-performance-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。
- 本次执行记录：见 `docs/dev-records/dev-plan-200-m8-performance-evidence.md` 的 2026-02-28 条目。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `AGENTS.md`
