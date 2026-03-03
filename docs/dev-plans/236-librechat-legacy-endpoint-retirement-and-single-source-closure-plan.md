# DEV-PLAN-236：LibreChat 旧入口退役与单主源封板实施计划

**状态**: 草拟中（2026-03-03 13:45 UTC）

## 1. 背景
- 承接 `DEV-PLAN-230` 的 PR-230-05。
- 核心问题是 `model-providers:apply` 等旧入口若长期存在，会形成“名义单主源、实际双主源”。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 按 stopline 执行旧入口 A/B/C 三阶段退役。
2. [ ] 清理路由、handler、前端调用与文档残留。
3. [ ] 确保只读/校验接口不回流为第二写入口。

### 2.2 非目标
1. [ ] 不新增替代写入口。
2. [ ] 不以临时 legacy 分支回避退役。

## 3. 输入与输出契约
### 3.1 输入
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `internal/server/handler.go`
- `config/routing/allowlist.yaml`

### 3.2 输出
1. [ ] 阶段 A（2026-03-20）：迁移入口化 + `Deprecation` 响应头。
2. [ ] 阶段 B（2026-04-10）：CI/Prod 返回 `410 Gone`。
3. [ ] 阶段 C（2026-04-24）：代码/路由/文档彻底删除。
4. [ ] `docs/dev-records/` 退役执行证据。

## 4. 实施步骤（直接落地）
1. [ ] 阶段 A：降级
   - [ ] `:apply` 仅写 LibreChat 主源，输出弃用提示。
2. [ ] 阶段 B：停用
   - [ ] 生产默认禁用；测试环境保留只读回放开关（有审计）。
3. [ ] 阶段 C：下线
   - [ ] 删除 handler、allowlist、前端入口、文档引用。
4. [ ] 封板
   - [ ] gate 阻断任何恢复旧写入口的变更。

## 5. 验收与门禁
1. [ ] A/B/C 时间点执行记录完整。
2. [ ] `make check no-legacy` 通过。
3. [ ] `make check assistant-config-single-source` 通过。
4. [ ] `make check routing` 与 `make e2e` 通过。

## 6. 风险与缓解
1. [ ] 风险：下线后出现紧急回写需求。  
   缓解：仅允许通过 LibreChat 主源修复，不恢复旧入口。
2. [ ] 风险：文档与代码退役不同步。  
   缓解：退役 PR 必须绑定文档与门禁改动一并提交。

## 7. SSOT 引用
- `AGENTS.md`
- `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `internal/server/handler.go`
- `config/routing/allowlist.yaml`
