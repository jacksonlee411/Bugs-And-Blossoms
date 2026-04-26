# DEV-PLAN-467 Readiness

## 说明

- 本文件用于登记 `DEV-PLAN-467` 的专项调查证据、真实页面复验、网络抓包与修复后复验。

## 2026-04-25 修复记录

### 修复范围

- 已修订 `DEV-PLAN-467` 文档状态、章节编号与 `P0` 完成定义。
- 已补充 `modules/orgunit/presentation/cubebox/*.md` 中的代词继承、查询域 fail-closed 与“压缩摘要不是查询锚点”规则和示例。
- 已在查询主链中复用 canonical events 回放最近已确认 `orgunit` 查询实体，并显式注入 planner。
- 已增加 `NO_QUERY` 通用 fail-closed stopline：planner 未产出合法 plan 时不落回普通聊天链，且不在 server 中硬编码组织架构业务澄清选项。

### 自动化验证

- 命令：`go test ./internal/server ./modules/cubebox`
- 结果：通过。
- 覆盖夹具：`100000 -> 查该组织的下级组织` 的最近确认实体注入；`NO_QUERY` 不再回落普通 Gateway 对话链。

### 真实页面复验

- 状态：通过（2026-04-25 11:26 CST）。
- 环境：`make dev-up`、`make dev-server`、`make dev-kratos-stub`；默认身份通过 `tools/codex/skills/bugs-and-blossoms-dev-login/scripts/seed_kratosstub_identity.sh` 写入 `admin@localhost / admin123`。
- 待验路径：主应用壳层右侧 `CubeBox` 抽屉 -> 新建/恢复对话 -> 输入 `查一下 100000 在 2026-04-25 的组织详情` -> 输入 `查该组织的下级组织` -> 输入 `就是你哦最开始说的。那个。组织啊`。
- 通过标准：第二轮能继承 `org_code=100000` 与 `as_of=2026-04-25`；若 planner 无法形成合法 plan，应返回通用安全 stopline，不得输出“没有查询接口/工具权限”。
- 复验会话：`conv_ff07b16ef1e6403182c4f8a6b5b7e33b`。
- 第一轮结果：系统成功回答 `100000` 在 `2026-04-25` 的组织详情，名称为“飞虫与鲜花”。
- 第二轮结果：用户输入 `查该组织的下级组织` 后，系统继承 `org_code=100000` 与 `as_of=2026-04-25`，返回 2 个直接下级组织：`200000「飞虫公司」`、`300000「鲜花公司」`。
- 第三轮结果：用户输入 `就是你哦最开始说的。那个。组织啊` 后，系统没有输出“没有查询接口/工具权限”。该次复验发生在旧 stopline 口径下；当前代码已收敛为通用 `NO_QUERY` stopline，业务澄清继续由 planner/知识包负责。
- 网络证据：`docs/dev-records/assets/dev-plan-467/network.txt` 记录三次 `/internal/cubebox/turns:stream` 请求均返回 `200 OK`，请求体依次为三轮复验输入。
- 截图证据：`docs/dev-records/assets/dev-plan-467/cubebox-revalidation.png`。
