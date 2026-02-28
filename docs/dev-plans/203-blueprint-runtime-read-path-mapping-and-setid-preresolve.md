# DEV-PLAN-203：200蓝图 Phase 1 运行时读路径（映射注册表 + SetID 硬前置）

**状态**: 已完成（2026-02-28 17:00 UTC）

## 1. 背景与上下文
承接 201/202。该计划先打通 6.1 读路径前半段：映射唯一命中、SetID 前置、候选读取边界。

## 2. 目标与非目标
### 2.1 核心目标
1. [X] 接入 `SurfaceIntentCapabilityRegistry` 并在运行时强制唯一命中。
2. [X] 把 `ResolveSetID` 设为候选读取硬前置，禁止“查完再猜 setid”。
3. [X] 候选接口回显 `resolved_setid + setid_source`，并保留 explain 追踪字段。
4. [X] 为后续组合 DTO 输出准备稳定上游数据结构。

### 2.2 非目标
1. [X] 不实现写入提交链路。
2. [X] 不在本计划引入 AI 编排能力。

## 3. 对齐关系（与 DEV-PLAN-200）
- 对应 DEV-PLAN-200 的 Phase 1；里程碑映射：M2/M3。
- 输入依赖：DEV-PLAN-201/202。
- 后续输出依赖：DEV-PLAN-204/206。

### 3.1 标准对齐（DEV-PLAN-005）
- [X] `STD-002`：先 `ResolveSetID` 再取数，`as_of` 必须入参显式传递。
- [X] `STD-004`：映射缺失/歧义不回退旧路径。
- [X] `STD-010`（Capability 映射治理，承接 DEV-PLAN-156）：`surface+intent` 映射完整性需可门禁。

## 4. 关键设计（Simple > Easy）
1. [X] 单一事实源：同一语义仅一个主写层，不新增平行事实源。
2. [X] 显式不变量：边界、失败路径、状态转换可在 5 分钟内解释清楚。
3. [X] Fail-Closed：缺上下文/缺策略/版本冲突/权限不满足一律拒绝。
4. [X] No Legacy：不引入双链路、回退通道、兼容别名窗口。
5. [X] 规格先行：实现偏离本计划时，先更新计划再改代码。

## 5. 实施步骤
1. [X] 实现映射决议查询与 fail-closed 错误返回（`mapping_missing/mapping_ambiguous`）。
2. [X] 实现 SetID 决议中间层与缓存策略（request-scope 优先，不引入外部缓存）。
3. [X] 改造候选读取接口，统一 tenant-only 边界并回显 `resolved_setid/setid_source`。
4. [X] 补充集成测试：未先 ResolveSetID 直接失败；global mapping 仍 tenant-only 读取。

## 6. 门禁与验证（SSOT 引用）
- 触发器与本地必跑矩阵：`AGENTS.md`
- 命令入口：`Makefile`
- CI 门禁：`.github/workflows/quality-gates.yml`
- 本计划预计命中门禁：
  - [X] `make check capability-route-map`
  - [X] `make check routing`
  - [X] `make test`
  - [X] `make check doc`

## 7. 验收标准
1. [X] 所有组合候选接口均通过映射决议 + SetID 决议后才可取数。
2. [X] `resolved_setid` 回显在响应与 explain 中一致。
3. [X] M2/M3 对应证据文档可一次性回填。

## 8. 风险与缓解
1. [X] 多入口绕过映射注册表。缓解：统一入口并加门禁扫描。
2. [X] SetID 决议链路引入额外延迟。缓解：在 Phase 2 纳入查询/事务预算回归。

## 9. 交付物与证据
- 证据归档：`docs/dev-records/dev-plan-200-m2-setid-pre-resolve-evidence.md`、`docs/dev-records/dev-plan-200-m3-mapping-registry-evidence.md`
- 交付物最小集：契约文档更新、自动化测试/门禁项、Readiness 证据记录。
- 本次执行记录：见 `docs/dev-records/dev-plan-200-m2-setid-pre-resolve-evidence.md`、`docs/dev-records/dev-plan-200-m3-mapping-registry-evidence.md` 的 2026-02-28 条目。

## 10. 文档完整性与 DEV-PLAN-003 对齐自检
1. [X] 已覆盖对应阶段目标、边界、不变量与失败路径。
2. [X] 已声明 Goals/Non-Goals、依赖关系、实施步骤、标准对齐与验收标准。
3. [X] 已包含门禁入口与证据归档路径（避免仅“能跑”不可审计）。
4. [X] 已落实 Simple > Easy：不新增多事实源、不引入 legacy 双链路。

## 11. 关联文档
- `docs/dev-plans/200-composable-building-block-architecture-blueprint.md`
- `docs/dev-plans/185-field-config-dict-values-setid-column-and-master-data-fetch-control.md`
