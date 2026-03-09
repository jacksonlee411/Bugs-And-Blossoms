---
name: bugs-and-blossoms-dev-login
description: Start the Bugs-And-Blossoms local login stack and bring up the tenant login page at http://localhost:8080/app/login (Postgres+Redis via make dev-up, IAM migrations, KratosStub, and the Go server). Optionally boot LibreChat runtime (`make assistant-runtime-up`) for `/app/assistant/librechat` formal-entry verification and `/assistant-ui/*` boundary checks. Use when you need a repeatable workflow to get a working login on port 8080, seed test identities, verify session creation (cookie sid), and check assistant-ui tenant/session boundaries locally. 适用于“启动8080登录页/本地联调登录/起dev-up+kratosstub+dev-server（可选 assistant runtime）”的场景。
---

# Bugs-And-Blossoms：本地 8080 登录页启动（dev-up + kratosstub + server，可选 assistant runtime）

目标 A：在本机把 `http://localhost:8080/app/login` 跑通（含 `POST /iam/api/sessions` 成功设置 `sid` cookie，并进入 `/app`）。

目标 B（可选）：在已登录前提下，跑通 `/app/assistant/librechat` 正式入口与 `/assistant-ui/*` 受保护代理链路，并验证 LibreChat runtime 可用性。

本技能默认只做“启动/迁移/seed/验证”，不会执行 `make dev-reset`，不会轻易清库删数据。

> 重要提醒（防遗忘）：`kratosstub` 使用内存存储。**每次重启 `make dev-kratos-stub`（或重启全部服务）后，都必须重新执行 seed 步骤**，否则会出现 `invalid credentials`。

前置：已安装并可用 `docker compose`、Go（能 `go run`）、`make`、`curl`、`jq`，并完成以下本地环境初始化（对齐当前 Ubuntu/.env 口径）：

```bash
cd "$(git rev-parse --show-toplevel)"
cp -n .env.example .env
```

确保 `.env` 至少包含：
- `DB_USER=app_runtime`
- `RLS_ENFORCE=enforce`
- `AUTHZ_MODE=enforce`
- `TRUST_PROXY=1`（当你通过 `X-Forwarded-Host` 做租户联调/E2E 时必须开启；否则会退回 `localhost` 租户导致登录串租户并出现 `invalid_credentials`）
- `ASSISTANT_MODEL_CONFIG_JSON="{\"provider_routing\":{\"strategy\":\"priority_failover\",\"fallback_enabled\":true},\"providers\":[{\"name\":\"openai\",\"enabled\":true,\"model\":\"gpt-5-codex\",\"endpoint\":\"https://api.openai.com/v1\",\"timeout_ms\":8000,\"retries\":1,\"priority\":10,\"key_ref\":\"OPENAI_API_KEY\"}]}"`（避免 `assistant_runtime_config_missing/invalid`）

## 工作流（推荐顺序）

0) 启动前先做“单实例清理”（强烈建议）

> 目的：避免同机残留多个 `server/superadmin/kratosstub` 进程导致“seed 在 A 实例、登录走 B 实例”的错位，出现看似随机的 `invalid_credentials`。

```bash
cd "$(git rev-parse --show-toplevel)"

# 先停容器依赖（不删数据）
DEV_INFRA_ENV_FILE=.env make dev-down || true

# 清理本机 Go 服务残留进程（前台/后台都覆盖）
pkill -f 'cmd/server|/exe/server' || true
pkill -f 'cmd/superadmin|/exe/superadmin' || true
pkill -f 'cmd/kratosstub|/exe/kratosstub' || true

# 端口基线检查：预期无监听
ss -ltnp | grep -E ':(8080|8081|4433|4434|5438|6379)' || true
```

1) 启动基础依赖（Postgres/Redis；不会删除数据）

```bash
cd "$(git rev-parse --show-toplevel)"
DEV_INFRA_ENV_FILE=.env make dev-up
```

2) 确保 IAM 迁移已执行（执行后请校验租户/域名映射；若缺失按后文 SQL 幂等补齐）

```bash
cd "$(git rev-parse --show-toplevel)"
admin_url="postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable"
DATABASE_URL="$admin_url" make iam migrate up
```

（可选）若你希望登录后能直接打开各业务模块页面（如 `/org/*`、`/person/*`），需要把对应模块的 schema/table 也迁移到本地 dev DB：

```bash
cd "$(git rev-parse --show-toplevel)"
admin_url="postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable"
DATABASE_URL="$admin_url" make orgunit migrate up
DATABASE_URL="$admin_url" make jobcatalog migrate up
DATABASE_URL="$admin_url" make person migrate up
DATABASE_URL="$admin_url" make staffing migrate up
```

3) 启动 KratosStub（本地认证 stub）

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-kratos-stub
```

默认监听：public `127.0.0.1:4433` / admin `127.0.0.1:4434`。

⚠️ 每次 KratosStub 重启后，下一步的 seed 必须重跑（见步骤 4）。

4) 创建/确保可登录账号（多租户验证，统一密码）

```bash
cd "$(git rev-parse --show-toplevel)"
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000000 \
  --email admin0@localhost \
  --password admin123 \
  --role-slug tenant-admin
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000001 \
  --email admin@localhost \
  --password admin123 \
  --role-slug tenant-admin
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000002 \
  --email admin2@localhost \
  --password admin123 \
  --role-slug tenant-admin
```

说明：
- identifier 格式固定为 `tenant_id:email`（服务端会这样拼接）。
- 多租户测试账号（密码统一 `admin123`）：
  - `00000000-0000-0000-0000-000000000000`（SAAS 厂商租户）：`admin0@localhost`
  - `00000000-0000-0000-0000-000000000001`（Local Tenant）：`admin@localhost`
  - `00000000-0000-0000-0000-000000000002`（普通租户）：`admin2@localhost`
- KratosStub 是**内存存储**：每次重启 KratosStub 后，需要重新跑一次 seed（同一个 `tenant_id:email` 会生成同一个 identity id）。

5) 启动 Web 服务（Go server，默认 `:8080`）

```bash
cd "$(git rev-parse --show-toplevel)"
DEV_SERVER_ENV_FILE=.env make dev-server
```

（可选）6) 启动 LibreChat Runtime（用于 `/app/assistant/librechat` 正式入口）

先确认当前 shell 已导出 `OPENAI_API_KEY`（`make assistant-runtime-up` 会做非空校验；为空会直接失败并提示缺少该环境变量）：

```bash
export OPENAI_API_KEY='<your-openai-api-key>'
```

```bash
cd "$(git rev-parse --show-toplevel)"
make assistant-runtime-up
make assistant-runtime-status
```

预期：
- `make assistant-runtime-status` 输出 `status=healthy` 时，可通过 `/app/assistant/librechat` 进入正式对话闭环（`/app/assistant` 页面仅用于运行态与日志查看）。
- 若输出 `status=unavailable`，先执行文末“关闭”小节中的 runtime 恢复流程再重试。
- 注意：正式入口 `/app/assistant/librechat` 复用 `/app/login` 的 `sid` 会话，不需要单独 LibreChat 登录；仅当你直连 runtime upstream（如 `http://127.0.0.1:3080`）时，才会使用 LibreChat 本地账号体系。

## 一键启动 + seed + 验证登录（推荐）

说明：适合本地临时启动（包含 dev-up、IAM 迁移、kratosstub、seed、server 启动与登录验证）。

- 下面脚本会把 `kratosstub` 与 `dev-server` 放到后台运行（使用 `&`）。
- 脚本结束后服务仍会继续占用端口；停止方式见本文的“关闭”小节。

```bash
cd "$(git rev-parse --show-toplevel)"
admin_url="postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable"
DEV_INFRA_ENV_FILE=.env make dev-up
DATABASE_URL="$admin_url" make iam migrate up
make dev-kratos-stub &
sleep 0.5
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000000 \
  --email admin0@localhost \
  --password admin123 \
  --role-slug tenant-admin
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000001 \
  --email admin@localhost \
  --password admin123 \
  --role-slug tenant-admin
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000002 \
  --email admin2@localhost \
  --password admin123 \
  --role-slug tenant-admin
DEV_SERVER_ENV_FILE=.env make dev-server &
sleep 0.5
curl -i -X POST -H 'Host: localhost:8080' -H 'Content-Type: application/json' \
  --data-binary '{"email":"admin@localhost","password":"admin123"}' \
  http://127.0.0.1:8080/iam/api/sessions
```

## 验证（多租户请使用不同 Host）

1) 打开登录页（任选其一）：
   - `http://saas.localhost:8080/app/login`（tenant `00000000-0000-0000-0000-000000000000`）
   - `http://localhost:8080/app/login`（tenant `00000000-0000-0000-0000-000000000001`）
   - `http://tenant2.localhost:8080/app/login`（tenant `00000000-0000-0000-0000-000000000002`）
2) 用账号登录（任选其一，密码都为 `admin123`）：
   - `admin0@localhost`（tenant `00000000-0000-0000-0000-000000000000`）
   - `admin@localhost`（tenant `00000000-0000-0000-0000-000000000001`）
   - `admin2@localhost`（tenant `00000000-0000-0000-0000-000000000002`）
3) 预期：前端调用 `POST /iam/api/sessions` 成功（204），并设置 `sid` cookie，然后进入 `/app`。
4) 登录后可直接访问：`http://<对应租户域名>:8080/app/org/units`（未登录时访问 `/app/*` 会 302 到 `/app/login`，属正常行为）。
5) （可选）访问：
   - `http://<对应租户域名>:8080/app/assistant`（运行态与会话记录页）
   - `http://<对应租户域名>:8080/app/assistant/librechat`（正式聊天入口）
   - 若 runtime healthy：可进入正式聊天页面并发起对话。
   - 若 runtime 未启动：通过会话校验后可能返回 `502`（表示边界已生效，但上游不可用）。
   - 验收口径优先看“路由与会话行为”（如是否命中 `/app/assistant/librechat`、是否复用 `sid`、是否出现错误码）；页面标题/文案可随产品调整，不应作为唯一通过条件。

（注意）不要用 `http://127.0.0.1:8080/app/login`：租户解析基于 Host，`127.0.0.1` 默认无租户映射，会 404（tenant not found）。

## 快速验证清单（多租户，curl 可直接复制）

> 目标：快速确认 3 个账号都能登录，且 session 不能跨租户复用。

1) （仅首次）补齐租户与域名映射（幂等）

```bash
cd "$(git rev-parse --show-toplevel)"
psql "postgres://app:app@127.0.0.1:5438/bugs_and_blossoms?sslmode=disable" <<'SQL'
INSERT INTO iam.tenants (id, name, is_active) VALUES
  ('00000000-0000-0000-0000-000000000000', 'GLOBAL', true),
  ('00000000-0000-0000-0000-000000000001', 'Local Tenant', true),
  ('00000000-0000-0000-0000-000000000002', 'Tenant 2', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO iam.tenant_domains (tenant_uuid, hostname, is_primary) VALUES
  ('00000000-0000-0000-0000-000000000000', 'saas.localhost', true),
  ('00000000-0000-0000-0000-000000000001', 'localhost', true),
  ('00000000-0000-0000-0000-000000000002', 'tenant2.localhost', true)
ON CONFLICT DO NOTHING;
SQL
```

2) 三个租户分别登录（预期都返回 `204` 且有 `Set-Cookie: sid=...`）

```bash
curl -i -c /tmp/sid-saas.txt -X POST \
  -H 'Host: saas.localhost:8080' \
  -H 'Content-Type: application/json' \
  --data-binary '{"email":"admin0@localhost","password":"admin123"}' \
  http://127.0.0.1:8080/iam/api/sessions

curl -i -c /tmp/sid-local.txt -X POST \
  -H 'Host: localhost:8080' \
  -H 'Content-Type: application/json' \
  --data-binary '{"email":"admin@localhost","password":"admin123"}' \
  http://127.0.0.1:8080/iam/api/sessions

curl -i -c /tmp/sid-t2.txt -X POST \
  -H 'Host: tenant2.localhost:8080' \
  -H 'Content-Type: application/json' \
  --data-binary '{"email":"admin2@localhost","password":"admin123"}' \
  http://127.0.0.1:8080/iam/api/sessions
```

3) 同租户访问鉴权接口（预期 `200`）

```bash
curl -i -b /tmp/sid-saas.txt -H 'Host: saas.localhost:8080' \
  'http://127.0.0.1:8080/iam/api/dicts?as_of=2026-01-01'
```

4) 跨租户复用 sid（预期 `401 unauthorized`，证明租户隔离生效）

```bash
curl -i -b /tmp/sid-saas.txt -H 'Host: tenant2.localhost:8080' \
  'http://127.0.0.1:8080/iam/api/dicts?as_of=2026-01-01'
```

5) （可选）assistant-ui 边界快速验证（与 DEV-PLAN-235 对齐）

- 未登录访问 assistant-ui（预期 `302` 到 `/app/login`）：

```bash
curl -i -H 'Host: localhost:8080' \
  http://127.0.0.1:8080/assistant-ui
```

- 登录后同租户访问 assistant-ui（预期 `200` 或 `502`；`502` 表示上游 runtime 未就绪）：

```bash
curl -i -b /tmp/sid-local.txt -H 'Host: localhost:8080' \
  http://127.0.0.1:8080/assistant-ui
```

- 跨租户复用 sid 访问 assistant-ui（预期 `302` 到 `/app/login`）：

```bash
curl -i -b /tmp/sid-saas.txt -H 'Host: tenant2.localhost:8080' \
  http://127.0.0.1:8080/assistant-ui
```

## 1 分钟手工验证（浏览器三窗口并行）

> 目标：不用 curl，快速做一次“3 租户并行登录 + 隔离”肉眼验收。

1) 打开 3 个无痕窗口（建议分别命名为 A/B/C）：
   - A：`http://saas.localhost:8080/app/login`
   - B：`http://localhost:8080/app/login`
   - C：`http://tenant2.localhost:8080/app/login`

2) 在 3 个窗口分别登录（密码都为 `admin123`）：
   - A：`admin0@localhost`
   - B：`admin@localhost`
   - C：`admin2@localhost`

3) 每个窗口登录后都访问：`/app/org/units`
   - 预期：3 个窗口都可进入应用页，不跳回登录页。

4) 在 A 窗口地址栏把域名改成 `tenant2.localhost`（保留路径 `/app/org/units`）回车
   - 预期：由于是不同租户域名，A 窗口不会复用 C 的登录态，应跳到登录页（或相关未授权响应）。

5) 在 C 窗口改成 `saas.localhost` 重复一次
   - 预期：同样不能复用 SAAS 租户登录态，证明跨租户隔离成立。

## 常见排障

- 浏览器提示“无法访问此网站 / 连接被拒绝”：先确认服务是否在监听 8080（`curl -fsS http://localhost:8080/health` 预期返回 200）。若连接失败，重新执行 `DEV_SERVER_ENV_FILE=.env make dev-server` 并查看其输出。
- 登录页直接显示 `tenant resolve error`（HTTP 500）：通常是 DB 未启动或服务端无法连接 DB（常见于未执行 `DEV_INFRA_ENV_FILE=.env make dev-up`）。先启动 infra，再重试 `http://localhost:8080/app/login`。
- 404 tenant not found：确认访问 Host 是否存在于 `iam.tenant_domains`；并确认 `make iam migrate up` 已执行，必要时按“快速验证清单”第 1 步补齐域名映射。
- 登录一直 invalid credentials：确认 KratosStub 在跑；并确认已按 `tenant_id:email` seed 过同一密码。
- 访问 `/app/assistant/librechat` 却出现 LibreChat 登录页：优先确认是否误连到了 runtime upstream（如 `:3080`）。正式入口应走 `:8080` 同域并复用 `sid`，通常不应再次要求登录。
- 若你确实在调试“直连 runtime upstream”（非正式入口）且遇到 LibreChat 邮箱/注册问题：`admin@localhost` 会因邮箱格式被拒；可改用 `admin@localhost.local`，并按需在 `deploy/librechat/.env` 设置 `ALLOW_REGISTRATION=true` 后重启 runtime。
- seed 脚本提示 409 但你仍然 invalid credentials：说明 **KratosStub 当前进程**里该 identifier 已存在，seed 不会更新密码；处理方式是重启 KratosStub（它是内存存储）后重新 seed，或换一个新邮箱 seed。
- 登录显示 identity error：确认 KratosStub 在跑（4433/4434）；未设置时默认 `KRATOS_PUBLIC_URL=http://127.0.0.1:4433`；并确认已执行 seed。
- `POST /iam/api/sessions` 返回 `invalid_json`：确认 `Content-Type: application/json`，并传入合法 JSON（例如 `{"email":"admin@localhost","password":"admin123"}`）。
- 登录显示 principal error：通常表示数据库里 `iam.principals(tenant_id,email)` 已绑定了**不同的** `kratos_identity_id`（历史数据与当前 KratosStub 的 identity id 算法不一致会触发该保护）。处理方式：
  - A) 清库重建（最省心，会丢 dev 数据）：执行一次 `make dev-reset`，然后从本技能第 1 步重新跑起。
  - B) 不清库：换一个新邮箱重新 seed；或手工把该 principal 的 `kratos_identity_id` 清空后再登录（这会改动数据库数据，需你自行确认风险）。
- 未登录访问 `/assistant-ui` 时应为 `302 /app/login`；若不是，优先检查 `withTenantAndSession` 边界链路是否被本地改动破坏。
- 登录后访问 `/assistant-ui` 返回 `502`：通常是 LibreChat 上游未启动或不可达，执行 `make assistant-runtime-up && make assistant-runtime-status`。
- `make assistant-runtime-status` 显示 `unavailable`：按顺序执行 `make assistant-runtime-down && make assistant-runtime-clean && make assistant-runtime-up && make assistant-runtime-status`。
- 8080 端口占用：用 `HTTP_ADDR=:8080` 或换端口后相应调整访问地址；但本技能的目标是跑通 8080。

## 关闭（默认保留数据）

说明：
- 如果你在前台运行了 `make dev-server` / `make dev-kratos-stub`：用 Ctrl+C 结束即可。
- 如果你使用了 `&` 放到后台：`make dev-down` 只会停 Docker（Postgres/Redis），不会自动停止本机上的 Go 进程；请先手工停止它们。
- 若你还启动了 LibreChat runtime：执行 `make assistant-runtime-down` 停止运行基线容器。

（Linux）查看监听端口与 PID（然后 `kill <pid>`）：

```bash
ss -ltnp | grep -E ':(8080|8081|3080|4433|4434)'
```

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-down
```

`make dev-down` 会停止容器但保留 Postgres volume（数据不丢）。

（可选）如果需要连同 LibreChat runtime 一并停止：

```bash
cd "$(git rev-parse --show-toplevel)"
make assistant-runtime-down
```

（可选）仅当你明确要清理 LibreChat 本地运行数据时：

```bash
cd "$(git rev-parse --show-toplevel)"
make assistant-runtime-clean
```

## 危险操作（会清空数据，需明确确认）

`make dev-reset` 会 `down -v` 删除 Postgres volume（数据全清空），仅在用户明确要求“重置环境/清库重来”时使用：

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-reset
```
