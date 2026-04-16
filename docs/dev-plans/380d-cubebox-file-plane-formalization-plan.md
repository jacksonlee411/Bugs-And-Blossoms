# DEV-PLAN-380D：CubeBox 文件面正式化

**状态**: 草拟中（2026-04-16 14:38 CST；已按 `DEV-PLAN-001` 颗粒度重写，冻结文件面边界、删除/引用契约、存储适配器策略与切换阶段；实现与 readiness 证据待后续批次关闭）

> 本文从 `DEV-PLAN-380` 拆分而来，作为 `CubeBox` 文件元数据、引用关系、对象存储适配器与删除回收语义的实施 SSOT。  
> `DEV-PLAN-380A` 持有 `cubebox_files / cubebox_file_links` 的 PostgreSQL schema contract 与历史索引导入规则；`DEV-PLAN-380B` 持有后端组合根与 facade 主链；`DEV-PLAN-380C` 持有 `/internal/cubebox/files` 的对外 DTO/API 收口；`DEV-PLAN-380E` 持有 `apps/web` 文件页与交互收口。  
> 本文只裁决“文件面本身的正式事实源、对象存储、删除保护、引用一致性、过渡兼容与验收方式”，不越权裁决相邻面的 API/页面细节。

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：把 `CubeBox` 文件能力从“`localfs + index.json + 过渡 DTO` 最小闭环”收敛为“`PostgreSQL 元数据/links + 可替换对象存储 + 明确删除/引用保护 + 明确兼容窗口`”的正式文件面。
- **关联模块/目录**：
  - `modules/cubebox/domain`
  - `modules/cubebox/services`
  - `modules/cubebox/infrastructure`
  - `modules/cubebox/infrastructure/sqlc/**`
  - `internal/server/cubebox_files_api.go`
  - `apps/web/src/api/cubebox.ts`
  - `apps/web/src/pages/cubebox/**`
- **关联计划/标准**：
  - `docs/dev-plans/001-technical-design-template.md`
  - `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`
  - `docs/dev-plans/015-ddd-layering-framework.md`
  - `docs/dev-plans/017-routing-strategy.md`
  - `docs/dev-plans/019-tenant-and-authn.md`
  - `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
  - `docs/dev-plans/025-sqlc-guidelines.md`
  - `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
  - `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
  - `docs/dev-plans/380a-cubebox-postgresql-data-plane-and-migration-contract.md`
  - `docs/dev-plans/380b-cubebox-backend-formal-implementation-cutover-plan.md`
  - `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
- **用户入口/触点**：
  - `GET /internal/cubebox/files`
  - `POST /internal/cubebox/files`
  - `DELETE /internal/cubebox/files/{file_id}`
  - `/app/cubebox`
  - `/app/cubebox/files`

### 0.1 Simple > Easy 三问

1. **边界**：`PostgreSQL` 只持有文件元数据与引用关系；对象正文只在对象存储；对外 DTO/API 由 `380C` 持有；前端状态与页面 UX 由 `380E` 持有。
2. **不变量**：文件删除必须先经过引用保护与租户隔离；不得把 `index.json`、页面状态或旧 DTO 字段回流为正式事实源；不得通过“物理删对象但保留 link”或“删 metadata 但残留强引用”制造悬空状态。
3. **可解释**：作者必须能在 5 分钟内讲清上传、列出、删除、引用判定、对象回收、失败 stopline 与 `index.json -> PostgreSQL` 的前向迁移语义。

### 0.2 现状研究摘要

- **现状实现**：
  - `380A` 已冻结 `iam.cubebox_files / iam.cubebox_file_links` 表结构、RLS 与本地索引导入规则。
  - `380B` 已建立 `modules/cubebox/services/files.go`、`modules/cubebox/infrastructure/local_file_store.go` 与 facade 接线。
  - 当前运行态仍存在 `LocalFileStore(root/.local/cubebox/files)`，以 `index.json + objects/` 保存对象与最小元数据。
  - `modules/cubebox/infrastructure/persistence/store.go` 目前只具备 `ListFiles / GetFile / ListConversationFileLinks` 等只读 PG 能力，尚未形成正式的文件写入、引用写入、删除判定与对象回收主链。
  - `internal/server/cubebox_files_api.go` 当前对外仍返回带单值 `conversation_id` 的过渡 DTO。
- **现状约束**：
  - 文件大小上限已收敛为 `20 MiB`。
  - `storage_provider` 允许 `localfs / s3_compat`，但当前仓内正式运行实现只有 `localfs`。
  - `cubebox_file_links` 已支持 `conversation_attachment / turn_input / turn_output`，但当前对外与领域对象仍基本停留在 conversation 级附件。
  - `No Tx, No RLS` 适用于 `cubebox_files / cubebox_file_links`，所有 PG 访问必须显式事务并注入 `app.current_tenant`。
- **最容易出错的位置**：
  - “元数据已落 PG，但对象仍只存在 index.json/localfs” 的双事实源漂移。
  - 删除时只删对象或只删 metadata 造成悬空。
  - `apps/web` 继续长期消费单值 `conversation_id`，阻碍 `links[]` 完成态收口。
  - conversation 删除与 file 删除交叉时的 link 回收与 orphan object 回收时序。
- **本次不沿用的“容易做法”**：
  - 长期保留 `index.json` 作为正式 SoT。
  - 把 `conversation_id` 永久塞在 `cubebox_files` 主表和 DTO 主字段中。
  - 直接在 `internal/server` 里拼接删除策略、对象回收与 link 判定。
  - 为了兼容前端而长期保留双 DTO 或双删除语义。

## 1. 背景与上下文（Context）

- **需求来源**：
  - `docs/dev-plans/380-cubebox-first-party-ownership-and-librechat-retirement-plan.md`
  - `docs/dev-plans/380a-cubebox-postgresql-data-plane-and-migration-contract.md`
  - `docs/dev-plans/380b-cubebox-backend-formal-implementation-cutover-plan.md`
  - `docs/dev-plans/380c-cubebox-api-dto-convergence-and-assistant-retirement-plan.md`
  - `docs/dev-plans/380e-cubebox-apps-web-frontend-convergence-plan.md`
- **当前痛点**：
  - 文件面已经有 `cubebox_files / cubebox_file_links` schema contract，但运行实现还没完全切到这套 contract。
  - 当前 `LocalFileStore` 仍把 `conversation_id` 嵌在单条记录里，并以 `index.json` 维护列表，这和 `380A` 已冻结的 “metadata / links 分离” 模型不一致。
  - 前端和 server response 仍消费/输出单值 `conversation_id`，与 `380C` 已写明的 `links[]` 完成态存在差距。
  - 删除语义仍不够明确：是“有引用即阻断”，还是“先 detach link 再删对象”，以及谁负责 orphan 回收，尚未冻结。
- **业务价值**：
  - 用户能稳定上传、查看、删除附件，而不会碰到“列表有文件但对象不存在”或“对象删了但历史引用还在”的伪成功。
  - 后续 `380C/380E` 可以围绕稳定文件 contract 收口 API 与前端，而不必继续围绕临时 `conversation_id` DTO 打补丁。
  - `CubeBox` 文件能力能在保留本仓一方默认实现的同时，为未来切到 `s3_compat` 留出正式适配器边界。
- **仓库级约束**：
  - 单链路：正式文件事实源只能有一套，切换后禁止 `index.json` 与 PG 双主源并存。
  - `No Tx, No RLS`：读写 `cubebox_files / cubebox_file_links` 必须显式事务 + 租户注入 + fail-closed。
  - 模块边界：文件业务规则必须下沉到 `modules/cubebox`，`internal/server` 只做 API delivery。
  - 用户可见性：文件能力必须继续通过 `/app/cubebox` / `/app/cubebox/files` 暴露，而不是只存在后端。

## 2. 目标与非目标（Goals & Non-Goals）

### 2.1 核心目标

- [ ] 冻结文件面的唯一正式事实源：`PostgreSQL metadata + links` 为 SoT，对象正文由单一对象存储适配器承载，`index.json` 仅允许存在于迁移期。
- [ ] 冻结上传、列出、删除、引用保护、orphan 回收与对象存储切换的正式契约。
- [ ] 明确当前过渡 DTO/领域模型的退出条件：`conversation_id` 从长期主字段收敛为兼容窗口，完成态改为 `links[]`。
- [ ] 明确 `localfs` 作为开发默认实现、`s3_compat` 作为未来可替换实现的边界与不变量。
- [ ] 为 `380C`、`380E` 提供稳定文件 contract，使 API/前端收口不再依赖 `index.json` 或过渡 DTO。

### 2.2 非目标（Out of Scope）

- 不在本文实现 RAG、File Search、向量索引、OCR、全文解析或内容抽取。
- 不在本文扩张文件权限模型到“单文件 ACL / 跨会话共享 / 外链分享”。
- 不在本文重做 `/internal/cubebox/files` 的最终 DTO 字段命名与前端页面 IA；这些分别由 `380C` 与 `380E` 持有。
- 不在本文把大文件正文写入 PostgreSQL。
- 不在本文通过 legacy 双写或长期兼容窗口保留 `index.json` 作为正式读写链。

### 2.3 用户可见性交付

- **用户可见入口**：
  - `/app/cubebox` 会话页中的文件上传/展示入口
  - `/app/cubebox/files` 文件列表页
  - `/internal/cubebox/files` JSON API
- **最小可操作闭环**：
  - 用户上传文件 -> 文件出现在会话附件或租户文件列表中 -> 用户删除未被引用的文件 -> 列表与对象存储同步收敛。
- **若短期为后端先行**：
  - 本计划允许后端先把 `links[]` / 删除保护 / metadata SoT 收口，再由 `380E` 调整前端展示。
  - 但后端先行期间，仍必须保持现有 `/app/cubebox` / `/app/cubebox/files` 能完成至少一条上传与删除路径，避免“只有 schema 没有入口”的僵尸功能。

## 2.4 工具链与门禁（SSOT 引用）

- **命中触发器（勾选）**：
  - [x] Go 代码
  - [x] `apps/web/**` / presentation assets / 生成物
  - [ ] i18n（仅 `en/zh`）
  - [x] DB Schema / Migration / Backfill / Correction
  - [x] sqlc
  - [x] Routing / allowlist / responder / capability-route-map
  - [x] AuthN / Tenancy / RLS
  - [ ] Authz（若文件能力复用既有 capability，则由 `380C/380E` 同批承接）
  - [ ] E2E（最终由 `380G` 汇总验收）
  - [x] 文档 / readiness / 证据记录
  - [x] 其他专项门禁：`no-legacy`、`error-message`、`ddd-layering-p0/p2`
- **本次引用的 SSOT**：
  - `AGENTS.md`
  - `docs/dev-plans/012-ci-quality-gates.md`
  - `docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
  - `docs/dev-plans/025-sqlc-guidelines.md`
  - `Makefile`
  - `.github/workflows/quality-gates.yml`

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 代表对象/文件 | 说明 |
| --- | --- | --- | --- |
| `pkg/**` | 文件名规范化、media type 默认值、对象 key 生成、删除判定 helper、link shape validator | 新增纯函数测试文件 | 纯函数优先黑盒表驱动 |
| `modules/cubebox/services` | 上传/删除/引用保护/orphan 判定/兼容窗口选择 | `modules/cubebox/services/*_test.go` | 业务规则不继续堆在 `internal/server` |
| `modules/cubebox/infrastructure` | PG repository、对象存储 adapter、tenant tx / RLS fail-closed | `modules/cubebox/infrastructure/**` | 真实 PG/本地文件系统集成优先放这里 |
| `internal/server` | multipart 解析、path 参数、错误码映射、`routing.ErrorEnvelope`、tenant/principal 适配 | `internal/server/cubebox_files_api_test.go` | 只验证 delivery 适配层 |
| `apps/web/src/**` | API client DTO 适配、页面最小交互与兼容期字段消费 | `apps/web/src/api/cubebox*.test.ts`、页面测试 | 先测 adapter，再测页面 |
| `E2E` | 上传/显示/删除端到端闭环 | 后续 `380G` | 不替代单元/集成测试 |

- **黑盒 / 白盒策略**：
  - `sanitizeFileName`、media type fallback、link validator、删除判定默认黑盒。
  - 若必须保留对白盒 localfs 实现的测试，应只限于“对象文件是否落对路径、orphan 回收是否真正删除物理对象”这类内部状态验证。
- **并行 / 全局状态策略**：
  - 触碰本地文件系统、临时目录、环境变量、共享 PG 的测试不得并行。
  - 纯 validator / helper 可并行。
- **fuzz / benchmark 适用性**：
  - 文件名规范化、path/key sanitizer、multipart 元数据校验应评估最小 fuzz。
  - benchmark 不是当前重点；除非文件名/key helper 成为热路径，否则可登记“不适用”。
- **前端测试原则**：
  - 先测 `apps/web/src/api/cubebox.ts` 的文件 DTO adapter。
  - 页面只验证“列表加载、上传成功、删除失败/阻断提示”等关键用户行为，不为 coverage 补洞。

## 3. 架构与关键决策（Architecture & Decisions）

### 3.1 5 分钟主流程

```mermaid
flowchart LR
  U[User] --> WEB[/app/cubebox or /app/cubebox/files]
  WEB --> API[/internal/cubebox/files]
  API --> FACADE[modules/cubebox/services facade]
  FACADE --> META[PostgreSQL cubebox_files + cubebox_file_links]
  FACADE --> STORE[Object Storage Adapter localfs/s3_compat]
  META --> RLS[RLS + explicit tx]
```

- **主流程叙事**：
  - 上传时，service 先校验 tenant / actor / filename / media type / size，再写对象存储，随后在显式事务内写 `cubebox_files` 与必要的 `cubebox_file_links`，事务成功后才返回。
  - 列表时，以 `cubebox_files` 为主表，必要时通过 `cubebox_file_links` 按 conversation 过滤，并在兼容窗口内把 `links[]` 映射为过渡 DTO。
  - 删除时，service 先判断文件是否仍被引用；如仍有活跃引用则 fail-closed；如允许删除，则在事务内删 metadata / links，并在提交成功后回收对象文件。
- **失败路径叙事**：
  - 对象写入失败：不得先写 metadata。
  - metadata/link 写入失败：必须回滚事务，并清理刚写入的对象或标记为待清理 orphan。
  - 删除时发现仍被引用：返回正式阻断错误码，不做隐式 detach。
  - 缺 tenant / 缺 principal / RLS 注入失败：立即 fail-closed。
- **恢复叙事**：
  - 迁移失败：保持写入口停用或只读，修复后重跑前向迁移，不回退到 `index.json` 正式读写。
  - 对象写成功但 metadata 失败：由补偿清理或 orphan 扫描回收。
  - metadata 已删但对象回收失败：记录可重试 orphan cleanup，不把删除静默视为完全成功。

### 3.2 模块归属与职责边界

- **owner module**：`modules/cubebox`
- **交付面**：
  - `internal/server`：multipart/path/error 适配
  - `modules/cubebox/services`：上传、列出、删除、引用保护、对象回收编排
  - `modules/cubebox/infrastructure`：PG repository、localfs / future `s3_compat` adapter
  - `apps/web`：文件列表与上传删除交互
- **跨模块交互方式**：
  - 文件面不跨业务模块直接 import 其他模块领域逻辑；若要校验 conversation 存在性，优先通过 `cubebox` 自己的 PG/repository 能力完成。
- **组合根落点**：
  - `modules/cubebox/module.go` / `links.go` 负责注入 file metadata repo 与 object store，不允许 `internal/server` 继续直接 `new LocalFileStore(...)` 作为长期主链。

### 3.3 落地形态决策

- **形态选择**：
  - [x] `A. Go DDD`
  - [ ] `B. DB Kernel + Go Facade`
- **选择理由**：
  - 文件面不是 `submit_*_event(...)` 型 One Door kernel 写链，更适合由 `Go facade + PG metadata + object store adapter` 管理。
  - 但仍必须遵守“metadata 权威只有一套、对象存储只有一套、删除语义只有一套”的单主链原则。

### 3.4 ADR 摘要

- **决策 1**：正式事实源是 `cubebox_files + cubebox_file_links`，不是 `index.json`
  - **备选 A**：长期保留 `index.json` 为文件列表 SoT。缺点：与 `380A` contract 冲突，形成双主源。
  - **备选 B（选定）**：`index.json` 仅允许出现在迁移期。完成态所有列表、删除、引用判定都以 PG 为准。
- **决策 2**：`cubebox_files` 不长期承载单值 `conversation_id`
  - **备选 A**：继续在文件主表和 DTO 中保留 `conversation_id`。缺点：无法表达 turn 级 link，也会压扁 `links` 模型。
  - **备选 B（选定）**：正式数据模型使用 `cubebox_file_links`；单值 `conversation_id` 仅作为过渡 DTO 映射存在，并有删除批次。
- **决策 3**：删除默认是“有引用即阻断”，不是隐式 detach
  - **备选 A**：用户删文件时自动删除所有 links。缺点：会把用户以为删“副本”变成删“共享资源”，风险高。
  - **备选 B（选定）**：只允许删除无引用文件；若未来需要 detach 单条 link，另开明确 API/计划。
- **决策 4**：对象存储写入和 metadata 事务采用“对象先写，metadata 后写，失败补偿回收”
  - **备选 A**：先写 metadata 再写对象。缺点：更容易出现“有记录无对象”。
  - **备选 B（选定）**：先写对象、再事务写 metadata，失败则补偿删除对象或标记 orphan cleanup。
- **决策 5**：开发默认 `localfs`，但 contract 先冻结成可替换 adapter
  - **备选 A**：把 `localfs` 路径与 `index.json` 格式写死进业务层。缺点：未来切 `s3_compat` 需要拆业务逻辑。
  - **备选 B（选定）**：把对象存储抽成端口，默认实现仍是一方本地文件系统。

### 3.5 Simple > Easy 自评

- **这次保持简单的关键点**：
  - metadata 与 links 一套正式事实源
  - 删除语义一套
  - 对象存储一套活跃实现
  - 兼容窗口有明确结束条件
- **明确拒绝的“容易做法”**：
  - [x] legacy alias / 双链路 / fallback
  - [x] 第二写入口 / controller 直写表
  - [x] 页面内自造第二套 link / conversation 关联语义
  - [x] 为过测临时加死分支或兼容层
  - [x] 复制一份旧 `FileRecord` / 旧 DTO 长期并存

## 4. 数据模型、状态模型与约束（Data / State Model & Constraints）

### 4.1 数据结构定义

- **实体 / 表 / 视图 / DTO 列表**：
  - `iam.cubebox_files`
    - owner：`380A/380D`
    - 用途：文件元数据、对象定位、扫描状态
  - `iam.cubebox_file_links`
    - owner：`380A/380D`
    - 用途：conversation/turn 与 file 的引用关系
  - `modules/cubebox/domain.FileRecord`
    - 当前：过渡态，仍含单值 `ConversationID`
    - 完成态：应扩为 `links[]` 或配套 link 读模型，`ConversationID` 只保留兼容窗口
  - `/internal/cubebox/files` response DTO
    - 当前：单值 `conversation_id`
    - 完成态：由 `380C` 收口为 `links[]`

- **精确字段约束（引用 `380A`，这里只写本计划新增解释）**：
  - `cubebox_files.storage_provider`：完成态只允许一个活跃 provider 实例；不能同一 tenant 同时写两套 provider 作为双主链。
  - `cubebox_files.storage_key`：tenant 内唯一，必须能稳定定位对象。
  - `cubebox_files.scan_status`：当前冻结 `pending / ready / failed`；`380D` 不在本批次扩展内容扫描状态机。
  - `cubebox_file_links.link_role`：当前冻结为 `conversation_attachment / turn_input / turn_output`。
  - `cubebox_file_links_shape_check`：禁止 conversation-only 和 turn-scoped 形状混用。

### 4.2 时间语义与标识语义

- **Valid Time**：文件面不引入业务 `effective_date`。
- **Audit / Tx Time**：
  - `uploaded_at / created_at / updated_at` 使用 `timestamptz`
  - 对象存储的写入时间只做审计，不参与业务有效期判断
- **ID / Code 命名**：
  - `file_id` 采用 `file_<uuid>` 形式
  - `storage_key` 是对象定位键，不是用户可见 code
  - 请求追踪仍使用统一 `request_id / trace_id`
- **有效期不变量**：
  - 文件链接当前不建 valid-time；删除/存在性为离散状态，不做日粒度版本切片

### 4.3 RLS / 显式事务契约

- **tenant-scoped 表**：
  - `iam.cubebox_files`
  - `iam.cubebox_file_links`
- **事务要求**：
  - 所有访问这两张表的 repository 必须在显式事务内执行
  - 事务内必须先 `set_config('app.current_tenant', ...)`
  - 缺 tenant 上下文必须 fail-closed
- **RLS 与应用职责边界**：
  - RLS 负责 tenant 隔离
  - 应用负责 multipart 解析、conversation/turn 存在性判定、引用阻断、对象回收补偿
  - 不允许用“手写 where tenant_uuid=...” 替代 tenant tx + RLS 契约
- **例外表**：
  - 无；文件 metadata / links 均是 tenant-scoped，不能豁免

### 4.4 迁移 / backfill / correction 策略

- **Up**：
  - 由 `380A` 持有 schema / migration / sqlc contract
  - `380D` 补齐 repository、对象存储 adapter、导入器与删除/引用判定逻辑
- **Backfill / correction**：
  - `.local/cubebox/files/index.json + objects/` 的导入规则以 `380A 4.6.2` 为准
  - `380D` 只补充运行切换要求：
    - 导入验证通过前，不得把 PG 文件面宣布为正式完成态
    - 导入完成后，所有正式列表/删除都必须只读写 PG metadata + object store，不再读 `index.json`
- **Down / rollback**：
  - 不允许把正式文件面回退为 `index.json` 事实源
  - 失败处置是停写、修复、重跑前向导入/回放
- **停止线**：
  - 不通过双写 `index.json + PG` 长期兜底
  - 不通过“metadata 从 PG 读、对象从 index 推断”维持半收口状态

## 5. 路由、UI 与 API 契约（Route / UI / API Contracts）

### 5.1 交付面与路由对齐表

| 交付面 | Canonical Path / Route | `route_class` | owner module | Authz object/action | capability / route-map | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| UI 页面 | `/app/cubebox/files` | `ui_authn` | `cubebox` | 由 `380E` 对齐现有 capability | N/A | 文件列表页 |
| UI 页面 | `/app/cubebox` | `ui_authn` | `cubebox` | 同上 | N/A | 会话页附件入口 |
| internal API | `/internal/cubebox/files` | `internal_api` | `cubebox` | 由 `380C` 统一冻结 | 已注册 | list / upload |
| internal API | `/internal/cubebox/files/{file_id}` | `internal_api` | `cubebox` | 由 `380C` 统一冻结 | 已注册 | delete |

- **要求**：
  - 不新增第二文件 API 命名空间
  - delete 路由完成态只能是 `DELETE /internal/cubebox/files/{file_id}`
  - 若未来新增 detach link API，必须单独立计划，不能复用 delete 语义偷渡

### 5.2 `apps/web` 交互契约

- **页面/组件入口**：
  - `CubeBoxPage.tsx`
  - `CubeBoxFilesPage.tsx`
- **数据来源**：
  - `apps/web/src/api/cubebox.ts`
  - 当前 `CubeBoxFile` 仍消费 `conversation_id`
- **状态要求**：
  - `loading / empty / error / success / delete-blocked / unavailable`
  - 删除阻断必须显式提示“文件仍被引用”，不能泛化成 `delete failed`
- **i18n**：
  - 如新增文件阻断、迁移提示、兼容窗口文案，必须对齐 `en/zh`
- **视觉与交互约束**：
  - 前端如何展示 `links[]`、是否在文件页显示“所属 conversation / turn”，由 `380E` 冻结
- **禁止**：
  - 前端自行把 `conversation_id` 反推为正式 link 模型
  - 页面静默吞掉 `file_delete_blocked` 之类正式错误码

### 5.3 JSON API 契约

#### 5.3.1 `GET /internal/cubebox/files`

- **用途**：按 tenant 列出文件；若带 `conversation_id`，则按 link 过滤
- **owner module**：`cubebox`
- **route_class**：`internal_api`
- **Request**：

```json
{
  "conversation_id": "conv_123"
}
```

- **Response (完成态上界，由 `380C` 收口)**：

```json
{
  "items": [
    {
      "file_id": "file_123",
      "file_name": "design.txt",
      "media_type": "text/plain",
      "size_bytes": 128,
      "scan_status": "ready",
      "uploaded_at": "2026-04-16T06:00:00Z",
      "links": [
        {
          "link_role": "conversation_attachment",
          "conversation_id": "conv_123",
          "turn_id": null
        }
      ]
    }
  ]
}
```

- **兼容窗口**：
  - 当前允许继续输出 `conversation_id` 单值字段
  - 但必须在 readiness 中登记删除批次，且不得新增依赖它的新页面逻辑

#### 5.3.2 `POST /internal/cubebox/files`

- **用途**：上传对象并写 metadata / links
- **Request**：multipart `file`，可选 `conversation_id`
- **完成态语义**：
  - `conversation_id` 只表示“立即创建一条 `conversation_attachment` link”
  - 不表示文件天然属于单一 conversation
- **错误返回（最小契约）**：
  - `invalid_request`
  - `cubebox_files_unavailable`
  - `cubebox_file_upload_failed`
  - `cubebox_conversation_not_found` 或等价正式错误码（若 `conversation_id` 无法映射）

#### 5.3.3 `DELETE /internal/cubebox/files/{file_id}`

- **用途**：删除一个无引用文件
- **完成态语义**：
  - 若文件仍存在任一 `cubebox_file_links`，返回阻断错误
  - 若文件无任何 link，则删除 metadata 并回收对象
  - 删除成功返回 `204 No Content`
- **禁止**：
  - delete 自动 detach 全部 link
  - delete 成功但对象保留为长期悬空垃圾，且无补偿回收

### 5.4 失败语义 / stopline

| 失败场景 | 正式错误码 | 是否允许 fallback | explain 最低输出 | 是否 stopline |
| --- | --- | --- | --- | --- |
| 文件存储未装配 | `cubebox_files_unavailable` | 否 | `path` `method` | 否 |
| multipart/filename/size 非法 | `invalid_request` | 否 | `field` | 否 |
| `conversation_id` 映射缺失 | `cubebox_conversation_not_found` 或正式等价码 | 否 | `conversation_id` | 否 |
| 文件仍被引用不可删 | `cubebox_file_delete_blocked` | 否 | `file_id` `link_count` | 否 |
| metadata/object 导入不一致 | `cubebox_file_import_mismatch` | 否 | mismatch 摘要 | 是 |
| `index.json` 仍被正式路径读取 | `cubebox_file_legacy_source_detected` | 否 | source | 是 |

- **错误码约束**：
  - 文件删除阻断必须有独立稳定错误码，不能继续只有 `cubebox_file_delete_failed`
  - 所有 Internal API 错误仍统一使用 `routing.ErrorEnvelope`

## 6. 核心流程与算法（Business Flow & Algorithms）

### 6.1 写路径主算法：上传

1. 解析 tenant / principal / multipart 输入
2. 校验 `filename / mediaType / size / conversation_id`
3. 若带 `conversation_id`，先验证目标 conversation 存在且属于同 tenant
4. 将对象写入当前活跃 object store
5. 开启显式事务并注入 tenant
6. 写入 `cubebox_files`
7. 如带 `conversation_id`，写入一条 `cubebox_file_links(link_role='conversation_attachment')`
8. 提交事务
9. 返回正式 file DTO

**失败补偿**：

1. 对象写失败：直接返回，不写 metadata
2. metadata/link 写失败：补偿删除刚写入对象；若删除补偿失败，记录 orphan cleanup 任务/日志
3. tenant/RLS 失败：直接失败，不允许回退写 `index.json`

### 6.2 读路径主算法：列表

1. 解析 tenant 与可选 `conversation_id`
2. 开启显式事务 + tenant 注入
3. 不带 `conversation_id`：
   - 读取 `cubebox_files`
4. 带 `conversation_id`：
   - 通过 `cubebox_file_links` 过滤引用集合
5. 组装正式读模型：
   - 完成态：`file + links[]`
   - 兼容窗口：可同时映射单值 `conversation_id`
6. 返回 JSON DTO

### 6.3 删除算法

1. 解析 tenant 与 `file_id`
2. 开启显式事务并注入 tenant
3. 查询 `cubebox_files` 是否存在
4. 查询 `cubebox_file_links` 数量
5. 若 `link_count > 0`：
   - 返回 `cubebox_file_delete_blocked`
   - 不删除对象，不删除 metadata
6. 若 `link_count == 0`：
   - 记录对象定位信息
   - 删除 `cubebox_files`
   - 提交事务
7. 事务成功后回收对象文件
8. 若对象回收失败：
   - 记录 orphan cleanup 信号
   - 不恢复 metadata，不回滚已提交事务

### 6.4 幂等、回放与恢复

- **幂等键**：
  - 上传暂不提供业务幂等键；如未来需要，另行冻结 request_id 复用规则
- **回放 / replay**：
  - `index.json -> PostgreSQL` 导入与 orphan cleanup 允许重跑
- **恢复策略**：
  - metadata 缺失但对象存在：通过 orphan scanner / import verifier 收敛
  - metadata 存在但对象缺失：视为 stopline，禁止宣布文件面正式完成

## 7. 安全、租户、授权与运行保护（Security / Tenancy / Authz / Recovery）

### 7.1 AuthN / Tenancy

- **tenant 解析事实源**：HTTP context / session
- **未登录或串租户行为**：
  - 未登录：`401 unauthorized`
  - tenant 缺失：fail-closed
- **会话 / principal**：
  - `uploaded_by` 必须来自可信 principal
  - 不允许客户端自填 `uploaded_by`

### 7.2 Authz

- 文件面的 capability/object/action 由 `380C` 统一冻结。
- `380D` 只要求：
  - 删除阻断不能绕过 authz
  - 不允许把“有删除权限”误实现为“可隐式 detach 任意 link”

### 7.3 运行保护

- 不引入文件面 feature flag 双主链
- 故障处置优先：
  - 停写
  - 修复 metadata/object mismatch
  - 重跑导入/cleanup
  - 恢复
- 不在早期阶段引入复杂对象存储监控平台；只保留最小健康探针与结构化日志

## 8. 依赖、切片与里程碑（Dependencies & Milestones）

### 8.1 前置依赖

- `380A`：schema、迁移、sqlc、index 导入规则
- `380B`：facade、模块组合根、server 接线
- `380C`：API/DTO 收口
- `380E`：前端 `links[]` / 错误提示收口

### 8.2 建议实施切片

1. [ ] **Contract Slice**：冻结删除保护、兼容窗口、`links[]` 完成态、object store 端口
2. [ ] **Persistence Slice**：补齐 PG file metadata/link repository 的写入、删除、存在性判定
3. [ ] **Storage Slice**：把 `localfs` 适配器从 `index.json` 风格改成对象存储实现；`index.json` 只保留导入器
4. [ ] **Service Slice**：收口 upload/list/delete/orphan cleanup 业务规则
5. [ ] **Delivery Slice**：`internal/server` 错误码与 DTO 适配；`apps/web` 消费兼容窗口/完成态
6. [ ] **Readiness Slice**：导入验证、门禁、文档与 dev-record 证据

### 8.3 每个切片的完成定义

- **Contract Slice**
  - **输入**：`380A/380C` 相邻 contract 已可引用
  - **输出**：本文档冻结
  - **阻断条件**：若文件 delete / detach / links 模型仍存在多种解释，必须暂停继续对齐
- **Persistence Slice**
  - **输入**：`cubebox_files / cubebox_file_links` schema 已存在
  - **输出**：正式 repository 可写 metadata / links / 删除判定
  - **阻断条件**：若 tenant tx/RLS 无法覆盖新查询
- **Storage Slice**
  - **输入**：对象存储端口已定义
  - **输出**：`localfs` 完成态不再依赖 `index.json` 作为正式列表
  - **阻断条件**：对象写入与 metadata 提交之间补偿语义不清
- **Service Slice**
  - **输入**：repo + object store 已可组合
  - **输出**：upload/list/delete/orphan cleanup 主链
  - **阻断条件**：删除语义仍不清晰
- **Delivery Slice**
  - **输入**：service 完成态稳定
  - **输出**：server/API/前端适配完成
  - **阻断条件**：DTO 兼容窗口与删除批次未登记
- **Readiness Slice**
  - **输入**：上述切片全部完成
  - **输出**：`DEV-PLAN-380D-READINESS.md`
  - **阻断条件**：仍检测到 `index.json` 被正式路径依赖

## 9. 测试、验收与 Readiness（Acceptance & Evidence）

### 9.1 验收标准

- **边界验收**：
  - [ ] metadata / links / object store 职责清晰，无第二套正式事实源
  - [ ] `internal/server` 只做 API delivery，不再持有文件业务判定

- **用户可见性验收**：
  - [ ] `/app/cubebox` 或 `/app/cubebox/files` 至少保留一条完整上传/删除闭环
  - [ ] 文件删除阻断对用户有明确提示，不是泛化失败

- **数据 / 租户验收**：
  - [ ] 所有 PG 文件访问都显式事务 + RLS
  - [ ] `index.json` 不再被正式 list/delete 路径读取
  - [ ] metadata 与对象存储之间不存在已知 mismatch

- **UI / API 验收**：
  - [ ] `/internal/cubebox/files` 只走正式单链路
  - [ ] DTO 兼容窗口与完成态 `links[]` 的边界已登记
  - [ ] 若继续保留 `conversation_id`，已写明删除批次

- **测试与门禁验收**：
  - [ ] 已按第 `2.5` 节补齐分层测试
  - [ ] 命中的 `sqlc / routing / error-message / no-legacy / ddd-layering / Go tests` 已通过
  - [ ] 没有通过降低覆盖率、保留假 fallback 或长期双 DTO 来规避收口

### 9.2 Readiness 记录

- [ ] 新建或更新 `docs/dev-records/DEV-PLAN-380D-READINESS.md`
- [ ] 在 readiness 中记录：
  - 时间戳
  - 命中的切片与执行入口
  - 导入验证结果
  - object store / metadata mismatch 结果
  - DTO 兼容窗口仍保留的字段与删除批次
  - 关键截图 / API 响应 / 日志摘要

### 9.3 例外登记

- **白盒保留理由**：若 localfs 物理对象删除需验证真实文件路径，可保留最小白盒测试
- **暂不并行理由**：文件系统与共享 PG 测试涉及全局状态
- **不补 benchmark 理由**：当前重点是契约收口与一致性，不是性能瓶颈
- **暂留页面级测试理由**：`links[]` UI 展示由 `380E` 接棒，`380D` 只保留最小消费验证
- **暂不能下沉的理由**：若某些 path 提取或 multipart 错误映射仍在 server 层，只保留 delivery 适配断言
- **若删除死分支/旧链路**：必须说明 `index.json` 已不再是正式 SoT，删除不改变对外 API 契约，只改变内部事实源

## 10. 附：作者自检清单

- [ ] 我已经写清文件 metadata、links、对象存储、删除保护与兼容窗口边界
- [ ] 我已经写清 `index.json` 的退出条件，而不是默认它会一直留着
- [ ] 我没有把 `conversation_id` 单值 DTO 当作完成态继续固化
- [ ] 我已经写清文件删除到底是阻断、detach 还是回收对象，不留多重解释
- [ ] 我已经说明与 `380A/380B/380C/380E` 的边界，没有重复裁决别人的范围
- [ ] 我已经给 reviewer 提供 5 分钟可复述的上传、列出、删除与恢复路径
