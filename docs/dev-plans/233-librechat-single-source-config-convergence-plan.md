# DEV-PLAN-233：LibreChat 模型配置单主源收口实施计划

**状态**: 草拟中（2026-03-03 13:30 UTC）

## 1. 背景
- 承接 `DEV-PLAN-230` 的 PR-230-02。
- 当前风险是“LibreChat 主源 + 本仓写入口”双主源并存，导致配置与异步任务契约漂移。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 模型/Provider 配置主写源收敛到 LibreChat。
2. [ ] `assistantModelGateway` 收口为只读适配与边界校验层。
3. [ ] 建立迁移脚本与一致性比对，支持平滑切换。

### 2.2 非目标
1. [ ] 不处理旧接口最终下线（由 DEV-PLAN-236 承接）。
2. [ ] 不处理 MCP/Actions 能力复用（由 DEV-PLAN-234 承接）。

## 3. 输入与输出契约
### 3.1 输入
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`

### 3.2 输出
1. [ ] 配置读路径：仅读取 LibreChat 主源。
2. [ ] 配置比对工具：旧源 vs 新源 canonical 对比报告。
3. [ ] 迁移窗口策略：只读观察期 -> 单主源切换 -> 双源残留清理。

## 4. 实施步骤（直接落地）
1. [ ] 读路径改造
   - [ ] `assistantModelGateway` 去除主写逻辑。
   - [ ] 返回结构保留 224/225 所需路由元数据。
2. [ ] 迁移脚本
   - [ ] 一次性迁移：旧配置导入 LibreChat。
   - [ ] 增量校验：每日/每 PR 比对报告。
3. [ ] 契约一致性
   - [ ] 任何配置变化不得直接改写 `intent_hash/plan_hash`。
   - [ ] 异步执行仅消费快照，不消费“当前可变配置”。

## 5. 验收与门禁
1. [ ] `assistant-config-single-source` gate 通过。
2. [ ] 同一输入在迁移前后产出等价路由决策（可审计）。
3. [ ] 双源差异报告为空（或仅剩已批准豁免项）。
4. [ ] `make e2e` 与 `make check doc` 通过。

## 6. 风险与缓解
1. [ ] 风险：迁移脚本遗漏边界字段。  
   缓解：以 canonical JSON 差异为唯一验收依据。
2. [ ] 风险：切换后出现不可逆配置问题。  
   缓解：切换前保留快照与回滚脚本（只回滚版本，不回滚架构）。

## 7. SSOT 引用
- `AGENTS.md`
- `docs/dev-plans/224-assistant-multi-model-and-llm-intent-governance-plan.md`
- `docs/dev-plans/225-assistant-tasks-temporal-p2-implementation-plan.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `Makefile`
