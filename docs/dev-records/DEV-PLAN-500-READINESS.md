# DEV-PLAN-500 Readiness

## 说明

- 本文件记录 `DEV-PLAN-500/501` 两次会话专项调查的直接证据命令、环境与结果摘要。
- 本文件只承载取证事实，不重复承载结论性分析；正式结论以 `DEV-PLAN-501` 为准。

## 2026-05-04 取证记录

### 环境

- 仓库：`/home/lee/Projects/Bugs-And-Blossoms`
- 时间：`2026-05-04 22:26 CST`
- 数据库容器：`bugs-and-blossoms-dev-postgres-1`
- 数据库：`bugs_and_blossoms`
- 用户：`app`

### 关键会话

1. `conv_0fc7637be99c47538e311860ffe972b2`
2. `conv_2d934635cd6449e48eb45ff4b9a0dddb`

### 已执行命令与结果

1. 会话元信息取证

```bash
docker exec bugs-and-blossoms-dev-postgres-1 psql -U app -d bugs_and_blossoms -P pager=off -c \
"select conversation_id, tenant_uuid, principal_id, title, status, archived \
from iam.cubebox_conversations \
where conversation_id in ('conv_2d934635cd6449e48eb45ff4b9a0dddb','conv_0fc7637be99c47538e311860ffe972b2') \
order by conversation_id;"
```

结果摘要：

- 两会话 `tenant_uuid` 相同：`00000000-0000-0000-0000-000000000001`
- 两会话 `principal_id` 相同：`33e5e61a-2734-474e-9621-6dfa031e34bc`
- 两会话标题均为 `新对话`

2. 会话事件序列取证

```bash
docker exec bugs-and-blossoms-dev-postgres-1 psql -U app -d bugs_and_blossoms -P pager=off -c \
"select conversation_id, sequence, turn_id, event_type, payload::text as payload, created_at \
from iam.cubebox_conversation_events \
where conversation_id in ('conv_2d934635cd6449e48eb45ff4b9a0dddb','conv_0fc7637be99c47538e311860ffe972b2') \
order by conversation_id, sequence;"
```

结果摘要：

- `conv_2d934635cd6449e48eb45ff4b9a0dddb`
  - `conversation.loaded`
  - `turn.started`
  - `turn.user_message.accepted`
  - `turn.error(code=ai_plan_boundary_violation)`
  - `turn.completed(status=failed)`
- `conv_0fc7637be99c47538e311860ffe972b2`
  - 第一次仅有 `turn.started` + `turn.user_message.accepted`
  - 第二次同句 turn 成功写出：
    - `turn.started`
    - `turn.user_message.accepted`
    - `turn.query_entity.confirmed`
    - `turn.agent_message.delta`
    - `turn.agent_message.completed`
    - `turn.completed(status=completed)`

3. conversations 表结构核对

```bash
docker exec bugs-and-blossoms-dev-postgres-1 psql -U app -d bugs_and_blossoms -P pager=off -c \
"select column_name \
from information_schema.columns \
where table_schema='iam' and table_name='cubebox_conversations' \
order by ordinal_position;"
```

结果摘要：

- `iam.cubebox_conversations` 不含 `provider_id/provider_type/model_slug` 列
- 模型链路证据应以 `turn.started` / `turn.error` / `turn.completed` payload 为准

### 关键代码路径核对

1. query flow 主链：
   - `internal/server/cubebox_query_flow.go`
2. planner 消息组装：
   - `internal/server/cubebox_query_flow.go`
3. narrator 消息组装：
   - `internal/server/cubebox_query_flow.go`
4. prompt view / 历史重建：
   - `modules/cubebox/turn_prep.go`
   - `modules/cubebox/store.go`
   - `modules/cubebox/compaction.go`
5. query context / evidence window：
   - `modules/cubebox/query_entity.go`
6. budget / repeat plan：
   - `modules/cubebox/query_working_results.go`
7. tool boundary：
   - `modules/cubebox/api_call_plan.go`
   - `internal/server/cubebox_api_tool_runner.go`
8. 前端 terminal event 要求：
   - `apps/web/src/pages/cubebox/api.ts`
   - `apps/web/src/pages/cubebox/CubeBoxProvider.tsx`
   - `apps/web/src/pages/cubebox/reducer.ts`

### 当前证据边界

以下内容当前没有直接证据：

1. planner raw JSON 输出
2. narrator raw 流式输出
3. 成功会话第二次同句发送的操作者归因

正式结论见：`docs/dev-plans/501-cubebox-two-conversations-boundary-investigation-report.md`
