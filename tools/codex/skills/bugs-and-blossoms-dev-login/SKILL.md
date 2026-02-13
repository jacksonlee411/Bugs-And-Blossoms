---
name: bugs-and-blossoms-dev-login
description: Start the Bugs-And-Blossoms local dev stack and bring up the tenant login page at http://localhost:8080/login (Postgres+Redis via make dev-up, IAM migrations, KratosStub, and the Go server). Use when you need a repeatable workflow to get a working login on port 8080, seed a test identity, and verify /login redirecting to /app (cookie sid) locally. 适用于“启动8080登录页/本地联调登录/起dev-up+kratosstub+dev-server”的场景。
---

# Bugs-And-Blossoms：本地 8080 登录页启动（dev-up + kratosstub + server）

目标：在本机把 `http://localhost:8080/login` 跑通（含 `POST /login` 成功设置 `sid` 并跳转 `/app?as_of=...`）。

本技能默认只做“启动/迁移/seed/验证”，不会执行 `make dev-reset`，不会轻易清库删数据。

前置：已安装并可用 `docker compose`、Go（能 `go run`）、`make`、`curl`。

## 工作流（推荐顺序）

1) 启动基础依赖（Postgres/Redis；不会删除数据）

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-up
```

2) 确保 IAM 迁移已执行（会插入 `localhost` 租户域名与 Local Tenant）

```bash
cd "$(git rev-parse --show-toplevel)"
make iam migrate up
```

（可选）若你希望登录后能直接打开各业务模块页面（如 `/org/*`、`/person/*`），需要把对应模块的 schema/table 也迁移到本地 dev DB：

```bash
cd "$(git rev-parse --show-toplevel)"
make orgunit migrate up
make jobcatalog migrate up
make person migrate up
make staffing migrate up
```

3) 启动 KratosStub（本地认证 stub）

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-kratos-stub
```

默认监听：public `127.0.0.1:4433` / admin `127.0.0.1:4434`。

4) 创建/确保一个可登录账号（固定账号/密码）

```bash
cd "$(git rev-parse --show-toplevel)"
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000001 \
  --email admin@localhost \
  --password admin123 \
  --role-slug tenant-admin
```

说明：
- identifier 格式固定为 `tenant_id:email`（服务端会这样拼接）。
- 固定账号：`admin@localhost` / `admin123`。
- KratosStub 是**内存存储**：每次重启 KratosStub 后，需要重新跑一次 seed（同一个 `tenant_id:email` 会生成同一个 identity id）。

5) 启动 Web 服务（Go server，默认 `:8080`）

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-server
```

## 一键启动 + seed + 验证登录（推荐）

说明：适合本地临时启动（包含 dev-up、IAM 迁移、kratosstub、seed、server 启动与登录验证）。

- 下面脚本会把 `kratosstub` 与 `dev-server` 放到后台运行（使用 `&`）。
- 脚本结束后服务仍会继续占用端口；停止方式见本文的“关闭”小节。

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-up
make iam migrate up
make dev-kratos-stub &
sleep 0.5
./tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh \
  --tenant-id 00000000-0000-0000-0000-000000000001 \
  --email admin@localhost \
  --password admin123 \
  --role-slug tenant-admin
make dev-server &
sleep 0.5
curl -i -X POST -H 'Host: localhost:8080' \
  -d 'email=admin@localhost&password=admin123' \
  http://127.0.0.1:8080/login
```

## 验证（必须用 localhost）

1) 打开登录页：`http://localhost:8080/login`
2) 用账号登录：`admin@localhost` / `admin123`
3) 预期：302 到 `/app?as_of=...`，并设置 `sid` cookie。
4) 登录后可直接访问：`http://localhost:8080/app/org/units`（未登录时访问 `/app/*` 会 302 到 `/login`，属正常行为）。

（注意）不要用 `http://127.0.0.1:8080/login`：租户解析基于 Host，IAM 默认只插入了 `localhost` 域名，`127.0.0.1` 会 404（tenant not found）。

## 常见排障

- 浏览器提示“无法访问此网站 / 连接被拒绝”：先确认服务是否在监听 8080（`curl -fsS http://localhost:8080/healthz` 预期输出 `ok`）。若连接失败，重新执行 `make dev-server` 并查看其输出。
- 404 tenant not found：确认用的是 `localhost`；并确认 `make iam migrate up` 已执行。
- 登录一直 invalid credentials：确认 KratosStub 在跑；并确认已按 `tenant_id:email` seed 过同一密码。
- 登录显示 identity error：确认 KratosStub 在跑（4433/4434）；未设置时默认 `KRATOS_PUBLIC_URL=http://127.0.0.1:4433`；并确认已执行 seed。
- 登录显示 principal error：通常表示数据库里 `iam.principals(tenant_id,email)` 已绑定了**不同的** `kratos_identity_id`（历史数据与当前 KratosStub 的 identity id 算法不一致会触发该保护）。处理方式：
  - A) 清库重建（最省心，会丢 dev 数据）：执行一次 `make dev-reset`，然后从本技能第 1 步重新跑起。
  - B) 不清库：换一个新邮箱重新 seed；或手工把该 principal 的 `kratos_identity_id` 清空后再登录（这会改动数据库数据，需你自行确认风险）。
- 8080 端口占用：用 `HTTP_ADDR=:8080` 或换端口后相应调整访问地址；但本技能的目标是跑通 8080。

## 关闭（默认保留数据）

说明：
- 如果你在前台运行了 `make dev-server` / `make dev-kratos-stub`：用 Ctrl+C 结束即可。
- 如果你使用了 `&` 放到后台：`make dev-down` 只会停 Docker（Postgres/Redis），不会自动停止本机上的 Go 进程；请先手工停止它们。

（Linux）查看监听端口与 PID（然后 `kill <pid>`）：

```bash
ss -ltnp | grep -E ':(8080|8081|4433|4434)'
```

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-down
```

`make dev-down` 会停止容器但保留 Postgres volume（数据不丢）。

## 危险操作（会清空数据，需明确确认）

`make dev-reset` 会 `down -v` 删除 Postgres volume（数据全清空），仅在用户明确要求“重置环境/清库重来”时使用：

```bash
cd "$(git rev-parse --show-toplevel)"
make dev-reset
```
