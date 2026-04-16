# DEV-PLAN-380C Readiness Record

**记录日期**: 2026-04-16  
**对应计划**: `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`  
**前置计划**: `docs/dev-records/DEV-PLAN-380B-READINESS.md`  
**结论**: `380C` 已完成主 API 面收口；`/internal/cubebox/*` 现为仓内唯一正式 API 命名空间。旧 `/internal/assistant/*` 已被清晰区分为 `compat window only` 或稳定 `410 Gone`，不再把 route registry 的 `status=active` 误读为“仍是正式产品入口”。

## 1. 收口范围

本次 readiness 对齐以下事实：

- `/internal/cubebox/*` 成为 conversations / turns / tasks / models / runtime-status / formal entry 的唯一正式命名空间。
- formal entry successor 已补齐并稳定暴露：
  - `GET /internal/cubebox/ui-bootstrap`
  - `GET /internal/cubebox/session`
  - `POST /internal/cubebox/session/refresh`
  - `POST /internal/cubebox/session/logout`
- turns / tasks 的完成态 path 已从泛化 action placeholder 收口为显式 literal：
  - `POST /internal/cubebox/conversations/{conversation_id}/turns/{turn_id}:confirm`
  - `POST /internal/cubebox/conversations/{conversation_id}/turns/{turn_id}:commit`
  - `POST /internal/cubebox/conversations/{conversation_id}/turns/{turn_id}:reply`
  - `POST /internal/cubebox/tasks/{task_id}:cancel`
- routing / authz template matcher / capability route registry / route-capability-map 均已同步为显式完成态 path literal，不再保留 `{turn_action}` / `{task_action}` 这类漂移占位。
- `apps/web` 正式 consumer 已切到 `/internal/cubebox/*`，且正式 IA 已收口到 `/app/cubebox*`。
- 旧 `/internal/assistant/ui-bootstrap`、`/session*`、`model-providers*` 已进入稳定 `410 Gone`，统一返回 `routing.ErrorEnvelope` 与错误码 `assistant_api_gone`。
- task receipt / link builder 已直接输出 `/internal/cubebox/tasks/{task_id}`，不再泄露旧 assistant namespace。

## 2. 旧 Assistant API 退役矩阵

### 2.1 successor in cubebox

- `GET /internal/assistant/ui-bootstrap` -> successor: `GET /internal/cubebox/ui-bootstrap`
- `GET /internal/assistant/session` -> successor: `GET /internal/cubebox/session`
- `POST /internal/assistant/session/refresh` -> successor: `POST /internal/cubebox/session/refresh`
- `POST /internal/assistant/session/logout` -> successor: `POST /internal/cubebox/session/logout`

说明：
- 上述路由虽然存在 successor，但当前运行时状态是稳定 `410 Gone`，不属于 compat window。

### 2.2 gone without successor

- `GET /internal/assistant/model-providers`
- `POST /internal/assistant/model-providers:validate`

### 2.3 compat window only

- `conversations / turns / tasks / models / runtime-status` 旧 assistant 命名空间仍保留兼容窗口，但不再作为仓内正式 consumer 主入口。
- 兼容窗口内不允许继续新增能力，也不允许再作为文档主入口。

## 3. 仓内 Consumer 收口

- `apps/web/src/api/cubebox.ts` 已成为唯一正式 client，并承接 conversations / turns / tasks / models / runtime-status / formal entry。
- `apps/web` 已删除 `AssistantModelProvidersPage`、`LibreChatPage`、`AssistantPage`、`assistantUiState` 与对应测试，不再保留会直接命中 `410` 的前端死页面。
- `apps/web` 路由仍把 `/assistant`、`/assistant/models` 导向 `/cubebox` / `/cubebox/models`，但这些仅是 redirect alias，不对应任何 assistant 页面组件。

## 4. 路由与能力映射结论

- `config/routing/allowlist.yaml` 已登记 cubebox successor formal entry，并保持旧 assistant retired routes 仍可分类为 internal API。
- `internal/server/capability_route_registry.go` 与 `config/capability/route-capability-map.v1.json` 已完成同批 literal 对齐。
- `internal/routing/pattern.go` 与 `internal/server/authz_middleware.go` 已收紧普通 `{id}` 段匹配，避免 detail route 吞掉 `:confirm/:commit/:reply/:cancel` action suffix。
- 上述治理配置中的 route `status=active` 仅表示“路由仍被注册并纳入 routing/authz/capability gate”，不表示“业务能力仍是正式产品入口”。

## 5. 错误语义结论

- retirement code 已收口为单一 `assistant_api_gone` -> `410`
- 旧 assistant retired internal API 继续复用统一 `routing.ErrorEnvelope`：
  - `code`
  - `message`
  - `trace_id`
  - `meta.path`
  - `meta.method`
- `apps/web` 错误文案已加入 `assistant_api_gone` 映射，避免回退为裸错误码。

## 6. 验证记录

已执行并通过：

```bash
go test ./internal/routing
```

结果：

```text
ok github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing
```

已执行并通过：

```bash
cd apps/web && npx vitest run src/api/cubebox.test.ts src/errors/presentApiError.test.ts
```

结果：

```text
2 passed
18 passed
```

本轮已执行：

```bash
go test ./internal/server
```

结果：

```text
ok github.com/jacksonlee411/Bugs-And-Blossoms/internal/server
```

已执行并通过：

```bash
make check routing
make check capability-route-map
```

结果：

```text
[routing] OK
[capability-route-map] OK
```

## 7. 下一步

- 在后续批次结束 assistant `conversations / turns / tasks / models / runtime-status` 的兼容窗口，并推进物理删除。
- 进入 `380F/380G` 前，复核旧 assistant runtime / deploy / vendored 资产是否已不再承载正式 API 入口。
