# DEV-PLAN-235：LibreChat 身份/会话/租户边界硬化实施计划

**状态**: 草拟中（2026-03-03 13:40 UTC）

## 1. 背景
- 承接 `DEV-PLAN-230` 的 PR-230-04。
- 当前已识别 `/assistant-ui/*` 在现有 UI 路由判定下存在会话绕过风险，需要与项目统一身份体系对齐。

## 2. 目标与非目标
### 2.1 目标
1. [ ] `/assistant-ui/*` 全量纳入会话校验，不得因非 `/app/**` 路径跳过。
2. [ ] 明确并固化 AuthN/AuthZ/Tenant 注入归属。
3. [ ] 强化代理边界：路径、方法、头透传最小化。

### 2.2 非目标
1. [ ] 不引入 LibreChat 自管身份体系。
2. [ ] 不改变 One Door 业务提交边界。

## 3. 输入与输出契约
### 3.1 输入
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `config/routing/allowlist.yaml`
- `internal/server/handler.go`

### 3.2 输出
1. [ ] 路由与中间件规则修订：assistant-ui 不再绕过 session。
2. [ ] proxy 透传白名单规则（header/path/method）。
3. [ ] e2e 负测：未登录、跨租户、旁路写路由。

## 4. 实施步骤（直接落地）
1. [ ] 中间件修复
   - [ ] 去除 assistant-ui 路径在 UI 快速放行中的特例风险。
   - [ ] 确保 tenant/principal 注入一致。
2. [ ] 路由收口
   - [ ] allowlist 与 handler 分类一致。
   - [ ] 无分类漂移与未注册路由。
3. [ ] 代理边界硬化
   - [ ] 限定允许方法（默认 GET）。
   - [ ] 清理危险头透传。
4. [ ] 测试加固
   - [ ] 未登录访问 assistant-ui 返回 302/401。
   - [ ] 跨租户 cookie 不可复用。
   - [ ] assistant-ui 不可触发业务写路由。

## 5. 验收与门禁
1. [ ] `make check routing` 通过。
2. [ ] `make check capability-route-map` 通过。
3. [ ] `make e2e` 中 assistant-ui 三类负测通过。
4. [ ] `make check error-message` 通过（错误码稳定）。

## 6. 风险与缓解
1. [ ] 风险：边界修复影响现有 UI 可访问性。  
   缓解：分阶段灰度开关（仅本地/测试先启），确认后全量启用。
2. [ ] 风险：代理头清理导致上游功能缺失。  
   缓解：建立白名单清单并逐项补证据。

## 7. SSOT 引用
- `AGENTS.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `config/routing/allowlist.yaml`
- `internal/server/handler.go`
