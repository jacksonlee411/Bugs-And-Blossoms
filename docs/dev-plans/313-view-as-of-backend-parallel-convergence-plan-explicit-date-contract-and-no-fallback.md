# DEV-PLAN-313：View As Of 后端并行收口计划——显式日期契约、无 fallback、统一错误语义

**状态**: 草拟中（2026-04-08 02:10 UTC）

## 背景

`DEV-PLAN-310` 已冻结仓库级时间语义的最小收敛方向：

- 产品/UI 层：`current-by-default`
- 服务/集成层：`explicit-by-contract`
- 写链路：`effective_date` 显式必填，且不得与 `as_of` 混用

`DEV-PLAN-311` 已明确 `DEV-PLAN-313` 的定位应是**后端并行收口计划**；`DEV-PLAN-312` 则负责前端详情页单历史锚点与 A 类页面读写解耦。

因此，`DEV-PLAN-313` 的职责不是继续讨论页面 IA，也不是重做前端状态模型，而是把后端当前仍然存在的时间默认化和校验漂移收平，为 `DEV-PLAN-312` 的前端减法提供稳定契约。

当前后端至少存在三类残余问题：

1. Org 写链路仍存在 `effective_date` 缺失时回填当天的 fail-open 逻辑，例如 `internal/server/orgunit_api.go` 中的 `orgUnitDefaultDate(...)` 及其调用点。
2. 多个读接口都要求显式 `as_of`，但缺少统一的解析与错误口径，`missing` / `invalid` 的表达并不一致。
3. 前端已经开始收敛为“URL 可省略 `as_of` 的 current 模式”，但后端尚未明确：**current 只属于产品层心智，浏览器到 API 的实际读请求仍必须是显式日期合同**。

本计划用于冻结这一层后端边界，避免前端一边删 page-local fallback，后端一边继续保留静默推断或口径漂移。

## 与 `DEV-PLAN-310/311/312` 的关系

- `DEV-PLAN-310` 是仓库级时间语义原则 SSOT；本计划不得重定义“两层规则”。
- `DEV-PLAN-311` 是页面矩阵与专项拆分 SSOT；本计划承接其对 `DEV-PLAN-313` 的预留主题。
- `DEV-PLAN-312` 负责前端页面减法与状态解耦；本计划与其**并行推进**，但不处理页面布局、URL IA 和样板页交互。
- 若后续发现需要改变“current-by-default / explicit-by-contract / 写链路显式必填”这三条上位原则，必须先更新 `DEV-PLAN-310/311`，再回写本计划。

## 目标

1. [ ] 冻结后端时间合同边界：UI 可以 default current，但浏览器发往后端的读请求必须在边界形成显式日期参数；后端不得自行推断 `current=today`。
2. [ ] 移除后端残余日期 fallback，尤其是 Org 写链路对 `effective_date` 的默认当天回填。
3. [ ] 统一 `as_of` / `effective_date` 的解析、缺失处理、非法处理与错误语义，避免各 handler 各写一套。
4. [ ] 冻结后端实施顺序、测试要点与 stopline，供后续 PR 直接承接。

## 非目标

- 不在本计划内重做前端页面、详情页 IA 或 URL 状态模型；这些由 `DEV-PLAN-312` 承接。
- 不在本计划内引入新的全局时间框架、`view + asOf` 对外新协议、`timeAnchor` 或 continuation envelope。
- 不在本计划内把所有页面请求改成新的 BFF/Facade；当前阶段优先在既有 JSON API 上完成收口。
- 不在本计划内改变业务 day 粒度语义；时间粒度继续以 `DEV-PLAN-032/102B` 为 SSOT。

## 现状问题

### 1. Org 写链路仍存在 `effective_date -> today` 的 fail-open

当前 `internal/server/orgunit_api.go` 仍通过 `orgUnitDefaultDate(...)` 在以下写接口中补当天：

- `handleOrgUnitsAPI` 的创建分支
- `handleOrgUnitsRenameAPI`
- `handleOrgUnitsMoveAPI`
- `handleOrgUnitsDisableAPI`
- `handleOrgUnitsEnableAPI`

这与 `DEV-PLAN-310`、`STD-002` 以及仓库“写链路显式日期”原则冲突，也会迫使前端继续保留防御性逻辑，难以判断“这是用户明确输入的生效日”还是“服务端偷偷补的当天”。

### 2. 读链路缺少统一的 day 参数解析入口

当前后端至少存在下列并存模式：

- 直接手写 `strings.TrimSpace(...Get(\"as_of\")) + time.Parse(...)`
- `orgUnitAPIAsOf(...)`
- `requiredAsOf(...)`
- 依赖下层 store 返回 `"invalid as_of"` 字符串再反推错误码

这导致：

- 同样是缺 `as_of`，有的 handler 返回 `as_of required`，有的统一落成 `invalid as_of`；
- 同样是非法日期，有的在入口 fail-fast，有的把错误延后到 store；
- 后续想批量收口或加门禁时，很难确认哪些 handler 还在绕开统一规则。

### 3. “产品层 current” 与 “服务层 explicit date” 的边界尚未冻结

`DEV-PLAN-312` 已开始将页面 URL 收敛为：

- current 模式：URL 不携带 `as_of`
- history 模式：URL 显式携带 `as_of`

但后端层当前还没有把边界写清楚：

- current 只是一种**产品层浏览语义**；
- 一旦浏览器真正发出会命中时间切片的 API 请求，应在边界形成显式日期；
- 后端只接收显式日期，不负责把“未传”解释成 current，更不负责回填 today。

如果这条边界不冻结，后续前端很容易再次为了兼容旧接口而恢复 page-local `todayISO()`、`fallbackAsOf` 一类逻辑。

### 4. 错误语义与回归保护不足

当前已有若干测试覆盖了 `as_of required` / `invalid as_of` 场景，但缺少仓库级统一口径：

- 哪些接口必须区分 missing 与 malformed；
- 哪些错误码必须稳定；
- 哪些写接口要明确禁止 `effective_date` 缺失自动补当天；
- 哪些内部/工具态接口可以复用统一 helper，而不是继续自带解析逻辑。

## 关键设计决策

### 决策 1：`current-by-default` 只属于产品层，不属于后端推断逻辑

选定规则：

- 页面可以在 URL/本地状态层表达 `current`；
- 浏览器一旦要调用“受时间切片影响”的后端读接口，必须在边界形成显式日期参数；
- 后端不再承担“没传就代表 current”的解释责任，更不允许把它实现成 `today` fallback。

说明：

- 这条规则允许前端继续做“URL current 模式不带 `as_of`”的产品减法；
- 也允许后端继续保持读接口的显式日期合同；
- 真正禁止的是“服务端看见缺参后自行推断 current/today”。

### 决策 2：后端彻底移除请求日期 fallback

冻结规则：

- `as_of` 缺失：直接 fail-closed，不得补今天。
- `effective_date` 缺失：直接 fail-closed，不得补今天。
- 非法 day 字符串：直接 fail-closed，不得吞掉错误后退回默认值。
- 响应中的 `as_of` / `effective_date` 只回显**显式输入或业务结果**，不得回显由 handler 默认生成的日期。

### 决策 3：写链路的 `effective_date` 必须显式必填

冻结规则：

- Org 写接口移除 `orgUnitDefaultDate(...)` 及等价逻辑。
- 后端以 `invalid_effective_date` 区分：
  - 缺失：`effective_date required`
  - 非法：`invalid effective_date`
- 写链路禁止从 `as_of`、URL、当前查看版本或服务端 `today` 自动派生写入日期。

### 决策 4：允许抽取后端最小 day parser，但禁止演化为新时间框架

允许的收口方式：

- 统一的 query day 解析 helper
- 统一的 body day 字段校验 helper
- 统一的 typed error 或稳定错误码映射

禁止的方式：

- 新增全局时间上下文对象
- 新增 `view + asOf` 对外合同层
- 新增跨 handler 的“大一统时间状态机”
- 继续把缺失/非法语义藏在 store 或字符串匹配里

### 决策 5：错误合同以“稳定 code + 明确 message”收敛

冻结规则：

- `as_of` 缺失：`400 + invalid_as_of + "as_of required"`
- `as_of` 非法：`400 + invalid_as_of + "invalid as_of"`
- `effective_date` 缺失：`400 + invalid_effective_date + "effective_date required"`
- `effective_date` 非法：`400 + invalid_effective_date + "invalid effective_date"`

说明：

- 当前不强求新增更多错误码维度，优先保持与现有调用方兼容；
- 但必须先把“缺失”和“非法”的 message 语义统一，避免不同 handler 各写各的。

## 实施范围

### P0：必须优先收口

1. `internal/server/orgunit_api.go`
2. `internal/server/orgunit_field_metadata_api.go`

目标：

- 移除 Org 写链路 `effective_date` 默认当天回填。
- 统一 Org 读链路及 Org 字段配置/字段选项相关接口的 `as_of` 错误语义。

### P1：公共读接口统一接线

1. `internal/server/dicts_api.go`
2. `internal/server/jobcatalog_api.go`
3. `internal/server/staffing_handlers.go`
4. `internal/server/setid_api.go`
5. `internal/server/setid_scope_api.go`

目标：

- 统一改为共享 day parser 或等价统一规则。
- 禁止继续在 handler 内重复手写日期校验模板。

### P2：工具态/内部接口收尾

1. `internal/server/setid_strategy_registry_api.go`
2. `internal/server/setid_explain_api.go`
3. `internal/server/internal_rules_evaluate_api.go`

目标：

- 工具态接口继续保持显式时间输入；
- 但校验方式与错误语义与公共接口对齐，不再依赖字符串错误反推。

## 实施步骤

1. [ ] 冻结本计划，确认“UI current / API explicit”的后端边界作为 `DEV-PLAN-312` 的并行前提。
2. [ ] 为后端 day 粒度参数建立最小共享解析入口，至少覆盖 query `as_of` 与 body `effective_date` 两类场景。
3. [ ] 在 Org 写链路移除 `orgUnitDefaultDate(...)`，将缺失/非法 `effective_date` 改为显式 400 失败。
4. [ ] 在 Org 读链路和 Org 字段配置相关接口统一 `as_of` 缺失/非法口径，避免 `missing` 被折叠成泛化 `invalid as_of`。
5. [ ] 按 P1/P2 顺序迁移 Dict / JobCatalog / Staffing / SetID / 工具态接口，逐步去掉重复的日期解析模板。
6. [ ] 为后端时间合同补齐回归测试，锁定：
   - `as_of` 缺失与非法的差异
   - `effective_date` 缺失与非法的差异
   - 不再存在请求日期默认当天回填
7. [ ] 视需要补充轻量 stopline，阻断 handler 层新增 `time.Now().UTC().Format("2006-01-02")` 用于请求日期默认化。

## 配套 stopline

后续承接代码改造时，应阻断以下模式继续回流：

- 在 handler 中新增 `if asOf == \"\" { asOf = time.Now()... }`
- 在 handler 中新增 `if req.EffectiveDate == \"\" { req.EffectiveDate = time.Now()... }`
- 通过 store 返回 `"invalid as_of"` 字符串，再由 handler 反向猜测缺失/非法
- 新增 `as_of -> effective_date` 或 `effective_date -> as_of` 的服务端自动回填

说明：

- stopline 可以是测试、lint 规则、grep 门禁或现有质量门禁扩展；
- 具体接线以 `AGENTS.md`、`Makefile` 与 CI 为 SSOT，本计划只冻结需要阻断的反模式。

## 测试与覆盖率

覆盖率与门禁口径以仓库 SSOT 为准：

- 入口：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`
- Go 测试分层：`DEV-PLAN-300`、`DEV-PLAN-301`

本计划要求的测试重点：

- `internal/server` 中受影响 handler 的黑盒 HTTP 测试，覆盖缺失/非法/合法三类日期输入。
- 若最小共享 day parser / body date validator 可脱离 HTTP，应优先落在 `pkg/**` 并采用 `package xxx_test` 黑盒测试；`internal/server` 只保留 handler 协议解析、错误映射与组合调用测试。
- 对最小共享 parser / validator 增加纯函数测试，避免重复通过 handler 间接覆盖。
- 对 Org 写接口新增“缺失 `effective_date` 不再自动补当天”的回归测试。

测试组织要求：

- 新增日期相关测试默认使用表驱动 + `t.Run(...)`，并优先并入现有正式测试入口或主测试簇。
- 不得为本计划新增 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 一类补洞式测试文件。
- 若共享 day parser / validator 形成开放输入空间，应按 `DEV-PLAN-301` 评估最小 fuzz；若不补 fuzz，需在执行记录中登记“不适用”的理由。

本计划不通过降低覆盖率阈值或扩大排除项达成收口；若存在难测分支，应优先通过职责拆小与 helper 纯函数化提升可测性。

## 交付物

1. [ ] 一份后端时间合同边界说明：明确 `UI current / API explicit` 的职责分层。
2. [ ] 一套最小共享 day parser / 校验入口设计说明。
3. [ ] 一份 Org 写链路 `effective_date` fallback 清理清单。
4. [ ] 一份公共读接口与工具态接口的迁移清单（P0/P1/P2）。
5. [ ] 一组后端回归测试与 stopline 约束说明。

## 验收标准

- [ ] `internal/server` 不再存在用 `time.Now().UTC().Format("2006-01-02")` 为请求日期默认值兜底的 handler 逻辑。
- [ ] Org 写接口在 `effective_date` 缺失时返回显式 400，而不是自动补当天继续执行。
- [ ] 公共读接口对 `as_of` 缺失/非法的 code/message 语义一致。
- [ ] 工具态/内部接口不再依赖字符串匹配推断 `invalid as_of`。
- [ ] `DEV-PLAN-312` 的前端减法不再需要为了兼容服务端 fallback 而保留额外防御逻辑。
- [ ] 文档门禁通过，且 `AGENTS.md` 文档地图已挂接本计划。

## 关联文档

- `docs/dev-plans/310-project-wide-view-as-of-semantics-review-and-minimal-convergence-plan.md`
- `docs/dev-plans/311-view-as-of-page-cutover-matrix-and-orgunit-details-sample-plan.md`
- `docs/dev-plans/312-view-as-of-implementation-plan-details-single-history-anchor-and-a-pages-read-write-decoupling.md`
- `docs/dev-plans/005-project-standards-and-spec-adoption.md`
- `docs/dev-plans/032-effective-date-day-granularity.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/300-test-system-investigation-report.md`
- `docs/dev-plans/301-go-test-layering-and-best-practices-remediation-plan.md`
- `AGENTS.md`
