# DEV-PLAN-380B Readiness Record

**记录日期**: 2026-04-15  
**对应计划**: `docs/dev-plans/380b-cubebox-backend-formal-implementation-cutover-plan.md`  
**结论**: `380B` 后端正式实现面切换已完成收尾；可作为 `380C` API/DTO 收口与 `/internal/assistant/*` 退役前置。

## 1. 收尾范围

本次 readiness 对齐以下事实：

- `/internal/cubebox/*` 不再直接代理 `handleAssistant*API`。
- conversations / turns / tasks / models / runtime-status / files 已经通过 `modules/cubebox` facade 暴露。
- create / append / confirm 已具备 bounded authoritative result -> `cubebox_*` formal snapshot write。
- `commit turn` 已优先由 `cubebox_conversations` / `cubebox_turns` formal snapshot 生成 `TaskSubmitRequest` 并写入 `iam.cubebox_tasks`。
- direct task lifecycle 的 submit / detail / cancel / dispatch / retry / deadline / dead-letter / terminal transition 已迁入 `modules/cubebox/services/facade.go`。
- workflow dispatch 入口已由 formal conversation snapshot 驱动，bounded adapter 不再自行回读 legacy conversation/turn。
- execute/apply result 后的 committed / manual_takeover conversation/turn terminal snapshot 已同步回写 `cubebox_*`。
- snapshot 回写失败不会触发重复 workflow execution，会收敛为 task `manual_takeover_required`。
- `/internal/cubebox/files` 已通过 `modules/cubebox` 组合根与 facade 暴露，不再由 `internal/server` 直连 new local store。

## 2. Bridge 清单

`380B` 收尾后仍保留的 bounded bridge：

- `internal/server/cubebox_links.go` 中的 `cubeboxLegacyFacade.ExecuteTaskWorkflow(...)` 仍调用 `assistantConversationService.executeCommitCoreTx(...)`。
- 该 bridge 只承接 commit executor / commit adapter 执行能力；formal conversation snapshot 由 `modules/cubebox/services/facade.go` 提供，执行结果 terminal snapshot 会回写 `cubebox_*`。
- create / append / confirm 当前仍由 bounded helper 产出 authoritative conversation snapshot，再由 cubebox facade 同步到 formal read/write 面。
- `CommitTurn(...)` 在 formal conversation 缺失时仍保留 bounded fallback，用于切换期保护；后续应在 `380C` 或后续收口批次删除。

不再保留的 bridge：

- `/internal/cubebox/*` 直接调用 `handleAssistant*API`。
- task `poll_uri` 响应后字符串改写。
- `DELETE /internal/cubebox/conversations/{conversation_id}` 的 `501` 占位实现。
- files API 由 `internal/server` 直接装配本地文件服务。
- workflow dispatch adapter 自行从 legacy conversation/turn 表回读 authoritative snapshot。

## 3. Stopline 结论

- Stopline 1 `/internal/cubebox/*` 仍主要通过 `handleAssistant*API` 代理实现：已清零。
- Stopline 2 `poll_uri` 仍依赖响应字符串改写：已清零。
- Stopline 3 conversation delete 仍返回 `501`：已清零。
- Stopline 4 `internal/server` 仍承载 task 状态机、dispatch 重试、snapshot compatibility 或 formal snapshot persistence 主逻辑：已收敛。当前只剩 bounded commit executor/adapter bridge。
- Stopline 5 `modules/cubebox` 缺少稳定 domain/ports/errors：已清零。
- Stopline 6 files 主链仍由 `internal/server` 直接 new local store：已清零。

## 4. 验证记录

已执行并通过：

```bash
go test ./modules/cubebox/... ./internal/server -run 'TestFacadeDispatchTask|TestFacadeGetTaskDispatchesPendingFormalTask|TestCubeBox|TestCubeBoxTurnActionMapsFormalCommitErrors'
```

结果：

```text
ok github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services
ok github.com/jacksonlee411/Bugs-And-Blossoms/internal/server
```

已执行并通过：

```bash
go test ./modules/cubebox/... ./internal/server -run 'TestCubeBox|TestAssistantNamespaceSegment|TestFacade|TestPGStore'
```

结果：

```text
ok github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox
ok github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure
ok github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/persistence
ok github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen
ok github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services
ok github.com/jacksonlee411/Bugs-And-Blossoms/internal/server
```

已执行并通过：

```bash
make check ddd-layering-p0
make check ddd-layering-p2
make check no-legacy
```

结果：

```text
[ddd-layering-p0] OK
[ddd-layering-p2] OK
[no-legacy] OK
```

## 5. Tenant Tx / RLS

- `modules/cubebox/infrastructure/persistence.PGStore` 继续通过模块级 store 访问 `iam.cubebox_*`。
- handler 侧仍由既有 server bootstrap 注入 pool / transaction 边界，PGStore 侧保持 tenantID 参数显式传递。
- 本批次未新增数据库表或迁移，未改变 `380A` 数据面 contract。
- 当前验证覆盖的是 Go facade / server focused tests 与 DDD/no-legacy 门禁；完整 DB 迁移与 RLS 回归仍由 `380A` readiness 与后续全量 preflight 承接。

## 6. 下一步

- 进入 `380C`：收口 `/internal/cubebox/*` API/DTO，并开始 `/internal/assistant/*` 退役批次。
- 删除或进一步缩小 `CommitTurn(...)` formal conversation missing fallback。
- 将 `internal/server/cubebox_links.go -> executeCommitCoreTx(...)` 的 bounded executor bridge 拆到可删除的 commit adapter 边界。
- 保持 `380D` 文件 metadata contract 与当前 files facade/组合根实现同步。
