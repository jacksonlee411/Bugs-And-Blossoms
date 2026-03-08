# DEV-PLAN-292：LibreChat 正式入口 vendored UI 认证/启动最小兼容层专项

**状态**: 已实施（2026-03-08 CST；正式入口 vendored UI 的 `sid` 认证/启动最小兼容层已落位，`/assets/librechat-web/api/**` façade、路由治理、结构化错误与 fail-closed 校验均已接线完成，并已解除 `288/290` 的最近实现阻塞）

## 1. 背景
1. [X] 当前正式入口 `/app/assistant/librechat` 已切到 vendored LibreChat Web UI，但其前端启动时仍按 LibreChat 约定拉起 `/api/auth/*`、`/api/user`、`/api/config`、`/api/roles/*`、`/api/endpoints`、`/api/models` 等接口。
2. [X] 本仓当前正式 tenant 登录事实源只有 `/app/login` + `POST /iam/api/sessions` + `sid` cookie；尚未提供 vendored UI 启动所需的兼容读接口。
3. [X] 调查已确认当前 `<base href>` 被改写为 `/assets/librechat-web/`，因此 vendored UI 实际命中的并非根路径 `/api/*`，而是正式静态前缀下的相对路径；这意味着问题不仅是“缺少 `/api/auth/*`”，还包括“API 前缀落点、handler 注册顺序与现有服务端路由未对齐”。
4. [X] 调查已确认 `ChatRoute` 在认证恢复后还会阻塞等待 `GET /api/endpoints` 与 `GET /api/models`；若这两类启动查询不返回前端可识别 DTO，页面会停留在 loading/空白态，无法把问题简化成“只补 auth refresh”。
5. [X] `DEV-PLAN-288/290` 的最近阻塞已明确收敛为“正式入口 vendored UI 与 `sid` 会话的认证/启动闭环缺口”；若不先把该底座补齐，`266/260` 的真实入口证据无法稳定复跑。

## 2. 问题定义
1. [X] vendored UI 的 `AuthContextProvider` 在挂载后会优先执行 silent refresh；当 token 缺失或未认证时，会调用 refresh，并在失败后跳转到 vendored `/login`。
2. [X] vendored UI 期望从前端可访问路径获得 `TLoginResponse` / `TRefreshTokenResponse` / `TUser` / `TRole` / startup config / endpoints / models 等 LibreChat DTO；而本仓当前仅暴露 `204 No Content` 风格的 `POST /iam/api/sessions`。
3. [X] 现状下，`/assets/librechat-web/**` 主要承载正式静态资源，`/assets/librechat-web/api/**` 尚未被定义为受保护 API façade；若直接落到静态文件服务或被分类为 `RouteClassStatic`，将绕过本专项所需的会话保护与结构化错误输出。
4. [X] 若直接补一套新的 LibreChat 登录/会话体系，会违反仓库既有的“单事实源 / No Legacy / No Double Door”约束；因此需要的是**最小兼容层**，而不是第二套 AuthN/AuthZ 体系。

## 3. 目标与非目标
### 3.1 目标
1. [X] 为正式入口 vendored UI 提供一组**最小、只读优先、以现有 `sid` 为唯一事实源**的认证/启动兼容接口。
2. [X] 使“先经 `/app/login` 建立 `sid`，再进入 `/app/assistant/librechat`”成为稳定可复跑的正式产品路径。
3. [X] 明确 vendored UI 认证 façade 的路径冻结、basename/formal-entry 兼容决策、DTO 映射规则、失败语义与 stopline，避免后续补丁式扩张。
4. [X] 为 `288/290` 提供可复用的最近阻塞修复口径，为 `285` 封板前的真实入口回归扫清前置缺口。

### 3.2 非目标
1. [X] 不在本计划内引入第二套 session store、第二套 principal 映射、第二套租户解析或任何并行登录事实源。
2. [X] 不默认开放 vendored LibreChat 自带 `/login` 作为第二正式登录入口；正式 tenant 登录入口仍以 `/app/login` + `POST /iam/api/sessions` 为准。
3. [X] 不在本计划内承担 `260` 的业务 Case 验收，也不承担 `266` 的 UI 气泡/单通道业务语义收口。
4. [X] 不通过恢复 `/assistant-ui`、iframe、bridge.js、HTML 注入或任何 legacy 旁路来规避当前缺口。
5. [X] 不把 basename/path 兼容问题偷换成“新增 `/assets/librechat-web` 作为第二正式页面入口”；正式用户可见入口仍冻结为 `/app/assistant/librechat`。

## 4. 方案边界与核心决策
### 4.1 路径边界冻结
1. [X] 正式用户可见入口继续冻结为 `/app/assistant/librechat`；本专项不得通过 302/重定向把用户正式入口切换为 `/assets/librechat-web/*`。
2. [X] vendored UI 的静态资源与相对 API 解析继续以 `<base href="/assets/librechat-web/">` 为准；因此认证/启动兼容层的正式目标路径冻结为 `/assets/librechat-web/api/**`。
3. [X] 是否同时暴露根路径 `/api/**` 别名，不作为本专项默认目标；若后续确需增加，必须单独证明不会形成第二正式 API 面或第二入口语义。
4. [X] basename/formal-entry 兼容的当前决策是：**formal entry 负责页面承载，static prefix 负责静态资源与相对 API 落点**；本专项只解决其 auth/startup 闭环，不把路径拓扑扩张成第二正式入口。

### 4.2 会话事实源冻结
1. [X] 兼容层所有认证态判断均只读取现有 `sid` cookie 与 `sessions.Lookup(...)` / `principals.GetByID(...)` 结果。
2. [X] vendored UI 所需 `token` 仅作为前端兼容 DTO 字段存在，不得被引入为新的后端权威会话事实源。
3. [X] 兼容层的失败语义必须与现有 fail-closed 行为一致：无 `sid`、租户不匹配、session 失效、principal 失效时统一拒绝，不得为照顾 vendored UI 启动链而放宽校验。

### 4.3 路由治理与注册顺序冻结
1. [X] `/assets/librechat-web/api/**` 必须在服务端显式注册为**受保护 façade**，且注册顺序必须位于 `/assets/librechat-web/**` 静态文件服务之前，避免请求落入 `http.FileServer(...)`。
2. [X] classifier / allowlist / responder 口径必须同步更新：`/assets/librechat-web/api/**` 不得再被判定为静态资源路径，也不得复用“匿名静态资源”的例外放行。
3. [X] route_class 必须显式落到受保护 UI/API 类，而不是 `RouteClassStatic`；无论最终采用现有哪一类受保护 route_class，都必须保证进入 `tenant -> sid session -> principal` 校验链与结构化错误输出链。
4. [X] 若某个 façade 路径未实现或 DTO 映射缺失，必须返回明确的 fail-closed 错误，而不是回退到静态 404、HTML 页面或模糊 `Connection error`。

### 4.4 启动关键性与降级口径冻结
1. [X] 启动关键查询冻结为：`auth/refresh`、`user`、`config`、`endpoints`、`models`；这些接口缺任一项都不得靠伪造空成功结果蒙混过关。
2. [X] `roles/user` 视为正式入口基础权限查询，必须返回最小可运行 `USER` 权限 DTO，不得直接省略。
3. [X] 非关键启动查询只允许限缩在 `roles/admin` 与 admin-only 配置叶子项；其降级口径必须是**显式无能力/不可用**，而不是返回超出承诺范围的伪权限。
4. [X] 若本仓当前不支持 admin 语义，可对 `roles/admin` 返回明确的“无 admin 能力”兼容结果，前提是不会阻断普通 tenant 用户进入正式聊天入口。

### 4.5 登录入口冻结
1. [X] 本阶段默认不补 vendored `login`，而是先保证“已有 `sid` 的正式入口启动闭环”。
2. [X] `login` 接口是否补齐，必须作为显式后续决策项单独评审；若补齐，也只能桥接到现有 `/iam/api/sessions` 与 `sid` 体系，不得自造 LibreChat 本地账户体系。
3. [X] 未经新的计划更新与明确批准，不得把 vendored `/login` 页面宣称为正式用户入口。

## 5. 最小接口矩阵（本计划目标口径）
1. [X] `POST /assets/librechat-web/api/auth/refresh`
   - 作用：用现有 `sid` 恢复 vendored UI 所需认证上下文。
   - 成功：返回 `TRefreshTokenResponse` 兼容 DTO（`token` + `user`）。
   - 失败：返回明确未认证/租户失配/session 失效/principal 失效语义；不得创建新会话。
2. [X] `GET /assets/librechat-web/api/user`
   - 作用：返回基于现有 principal/session 映射的 vendored `TUser`。
3. [X] `GET /assets/librechat-web/api/roles/user`
   - 作用：返回 vendored UI 最小运行所需的 `USER` 角色权限 DTO。
4. [X] `GET /assets/librechat-web/api/roles/admin`
   - 作用：仅当映射角色需要时返回 `ADMIN` 角色权限 DTO；若本仓 tenant principal 不支持该语义，需返回显式“无 admin 能力”的兼容结果或明确定义的非阻断降级语义。
5. [X] `GET /assets/librechat-web/api/config`
   - 作用：返回 vendored UI 首屏与导航所需最小 startup config，且与本仓正式入口路径、可用模型、禁用项保持一致。
6. [X] `GET /assets/librechat-web/api/endpoints`
   - 作用：返回正式入口当前允许启用的 endpoint 列表，口径必须与本仓实际可用 Assistant/runtime 配置一致。
   - 失败：若无法给出可信列表，应显式报“startup config missing / endpoints unavailable”，不得伪造空壳成功结果让前端停在不可解释状态。
7. [X] `GET /assets/librechat-web/api/models`
   - 作用：返回与 `endpoints` 对齐的模型清单，满足 `ChatRoute` 首屏启动所需最小 DTO。
   - 失败：若当前 tenant/运行时无可用模型，必须明确报错，不得用随意拼装的占位模型欺骗前端进入不可提交状态。
8. [X] `POST /assets/librechat-web/api/auth/logout`
   - 作用：桥接现有 `sid` 失效与前端登出动作，保持单事实源。
9. [X] `POST /assets/librechat-web/api/auth/login`
   - 当前状态：**不列为本计划默认必做项**；只有在确认“必须支持 vendored 登录页本身”后，才可在本计划后续修订中纳入。

## 6. DTO 映射、basename 兼容与失败语义
1. [X] 输出 DTO 必须是 vendored UI 当前源码所需的最小子集，不追求实现 LibreChat 全量账户能力。
2. [X] `TUser` 字段映射应来源于本仓 `tenant/principal/session` 可稳定提供的数据；对于缺省字段，采用明确常量/默认值，而不是运行时猜测。
3. [X] `TRole` 应返回 vendored UI 启动/基础导航所必需的最小权限集；不得顺手扩张到本仓未承诺支持的管理能力。
4. [X] startup config、`endpoints`、`models` 必须共同对齐当前正式入口配置：页面从 `/app/assistant/librechat` 进入，但其静态/相对 API 基准仍落在 `/assets/librechat-web/`；不得让配置中再暴露第二正式页面入口。
5. [X] 非关键启动查询的降级策略必须预先写清：普通 tenant 用户路径允许 `roles/admin` 返回显式“disabled / unsupported / no capability”兼容结果；但不得把关键查询降级为无含义空对象。
6. [X] 错误码与文案必须明确区分：未认证、租户不匹配、session 失效、principal 失效、DTO 映射缺失、配置缺失、endpoints/models 不可用；不得一律退化为模糊 `Connection error`。

## 7. 顺序与依赖
1. [X] 本计划是 `288/290` 当前战术阻塞的直接前置专项；推荐默认顺序冻结为 `292 -> 288 -> 290 -> 285`。
2. [X] 本计划依赖 `235/283/284` 已冻结的正式入口、会话边界与主链承载面，不得回退其既有约束。
3. [X] 本计划完成后，`288` 应先复跑正式入口 live E2E 与证据闭环，再由 `290` 继续真实 Case 1~4 验收。
4. [X] 本计划与 `291` 可并行准备，但不得因为“升级兼容仍未完成”而延迟解决当前正式入口认证/启动阻塞。

## 8. 实施步骤
1. [X] 冻结 formal-entry / static-prefix / basename 决策：明确正式入口继续是 `/app/assistant/librechat`，而 vendored 相对 API 只走 `/assets/librechat-web/api/**`。
2. [X] 落位 handler 与路由治理：为 `refresh / user / roles / config / endpoints / models / logout` 建立受保护 façade，调整 handler 注册顺序，并补齐 classifier / allowlist / responder 口径。
3. [X] 完成 DTO 映射：定义 principal/session -> `TUser`、角色 -> `TRole`、本仓配置 -> startup config / endpoints / models 的最小映射规则。
4. [X] 明确非关键查询降级：把 `roles/admin` 等非关键启动查询的兼容返回与失败码冻结成文档与代码契约，避免运行时临时兜底。
5. [X] 校验首屏：确保 vendored UI 在已有 `sid` 场景下能完成 silent refresh、用户上下文恢复、Root 渲染与正式聊天入口进入，不再误跳 vendored `/login` 或内部落回 `/app/login` 空白页。
6. [X] 负测补齐：覆盖无 `sid`、session 失效、跨租户、principal 失效、配置缺失、`endpoints/models` 缺失等 fail-closed 场景。
7. [X] 交接到 `288/290`：复跑真实入口证据链，并将“最近战术阻塞”从笼统描述更新为已关闭专项。

## 9. 验收标准
1. [X] 已有 `sid` 的用户进入 `/app/assistant/librechat` 时，vendored UI 可稳定完成认证恢复与首屏启动，不再误跳 vendored `/login`、不再内部落回 `/app/login` 空白页。
2. [X] `/assets/librechat-web/api/**` 全部仍受租户/session/principal 校验保护；未登录、跨租户、principal 失效时保持 fail-closed。
3. [X] 无新增第二登录入口、第二会话事实源、第二 principal 事实源、第二正式页面入口或 legacy 兼容窗口。
4. [X] `roles/admin` 等非关键查询的降级结果可解释、可复核，且不会伪造 admin 能力或掩盖真实配置缺口。
5. [X] `288` 能基于本计划产物完成正式入口复跑与证据固化；`290` 能在其后继续真实 Case 验收。若仍无法复跑，必须明确指出剩余缺口，不得将本计划标记为“已完成”。

## 10. 测试与门禁（SSOT 引用）
1. [X] 触发器与命中门禁以 `AGENTS.md`、`Makefile`、`docs/dev-plans/012-ci-quality-gates.md` 为准。
2. [X] 文档改动至少通过 `make check doc`。
3. [X] 若命中 Go 代码改动，按仓库级触发器执行 `go fmt ./... && go vet ./... && make check lint && make test`。

## 11. 交付物
1. [X] 本计划文档：`docs/dev-plans/292-librechat-vendored-ui-auth-startup-compat-plan.md`。
2. [X] 正式入口 vendored auth/startup façade 路径冻结说明（含 formal-entry / static-prefix / basename 决策）。
3. [X] 兼容 DTO 映射说明、关键/非关键启动查询矩阵与失败语义矩阵。
4. [X] 面向 `288/290` 的阻塞关闭记录与复跑入口说明（见本计划状态、`271/288/290` 更新与本次代码变更）。

## 12. 关联文档
- `docs/dev-plans/235-librechat-auth-session-and-tenant-boundary-hardening-plan.md`
- `docs/dev-plans/266-librechat-official-ui-single-dialog-channel-and-in-bubble-gpt52-plan.md`
- `docs/dev-plans/271-assistant-librechat-cross-plan-sequenced-delivery-plan.md`
- `docs/dev-plans/280-librechat-web-ui-vendoring-and-runtime-layered-reuse-plan.md`
- `docs/dev-plans/283-librechat-formal-entry-cutover-plan.md`
- `docs/dev-plans/288-librechat-266-live-e2e-and-evidence-closure-plan.md`
- `docs/dev-plans/290-librechat-260-m5-real-case-validation-and-evidence-plan.md`
- `AGENTS.md`
