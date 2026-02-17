# DEV-PLAN-100H：Org 模块宽表元数据落地 Phase 5：稳定性/性能/异常与运维收口

**状态**: 评审中（2026-02-16 12:10 UTC）

> 本文承接 `DEV-PLAN-100` 的 Phase 5，作为 Phase 5 的 SSOT；`DEV-PLAN-100` 保持总路线图。

## 1. 背景与上下文

截至 2026-02-16，100 系列已完成 Phase 0~4（100A/100B/100C/100D/100D2/100E/100E1/100G/101），并已形成“字段配置 -> 写入 -> 列表筛选/排序 -> 详情回显”的用户可见闭环。  
当前缺口集中在 Phase 5：缺少统一冻结的性能阈值、可复跑基准脚本、异常场景全量清单与运维手册，导致后续扩展（新增字段/新增筛选场景）缺乏统一收口标准。

## 2. 目标与非目标

### 2.1 核心目标（本计划冻结）

1. 冻结 OrgUnit 扩展字段能力的性能阈值（P95/P99/错误率），并给出统一统计口径。  
2. 冻结可复跑的基准脚本与证据产物规范（输入数据、执行命令、结果文件位置）。  
3. 冻结异常场景清单（功能/安全/语义），并映射到稳定错误码与测试类型。  
4. 冻结运维手册最小目录（启停流程、回滚策略、排障步骤）。  
5. 定义 Phase 5 出口条件（DoD）与里程碑。

### 2.2 非目标（Stopline）

- 不新增 legacy/双链路，不改变 One Door 写入原则。  
- 不在本计划内新增业务能力（例如多条件 DSL、跨模块联动字段）。  
- 不默认引入大规模预防性索引；仅允许“证据驱动”补索引。  
- 若需新增数据库表（例如专用压测明细表），必须先获用户手工确认（遵循 `AGENTS.md` 红线）。

## 3. 性能阈值来源与冻结口径（SLO/SLA）

### 3.1 统计口径（冻结）

- 统计窗口：单场景连续 3 轮，每轮 N=50；取总体 P95/P99 与错误率。  
- 错误率定义：非预期状态码（非业务预期 4xx）/总请求数。  
- 环境口径：本地单节点 baseline（PostgreSQL + apps/web + internal/server），用于回归对比；生产阈值另行评审。

### 3.2 继承阈值（不在 100H 重定义）

以下阈值已在 `DEV-PLAN-083B` 冻结并有执行证据；100H 直接继承，不重复定义第二套口径：

- 列表 ext filter/sort（`mode=grid`，含 `parent_org_code`）：见 `DEV-PLAN-083B` §4.3.2 条目 3。  
- 写入负例 fail-closed（预期 4xx）：见 `DEV-PLAN-083B` §4.3.2 条目 4。  

> 规则：若 100H 与 083B 文案出现冲突，以 083B 为准；如需改阈值，必须先改 083B 再改 100H 引用。

### 3.3 100H 新增阈值（冻结）

| 场景 | 阈值 |
| --- | --- |
| 详情读取（含 `ext_fields[]` 展开） | P95 <= 350ms；P99 <= 700ms；错误率 <= 0.5% |
| 字段配置启用/停用（field-configs enable/disable） | P95 <= 450ms；P99 <= 900ms；错误率 <= 0.5% |
| 更正写入（`corrections` + `patch.ext`） | P95 <= 600ms；P99 <= 1200ms；错误率 <= 0.5% |

## 4. 基准脚本与证据规范（冻结）

### 4.1 必须脚本与目录（冻结）

- 基准脚本目录：`scripts/perf/orgunit-ext/`
- 结果目录：`docs/dev-records/perf/100h/`
- 必须脚本（缺一不可）：
  - `scripts/perf/orgunit-ext/prepare_dataset.sh`（准备租户与样本数据）
  - `scripts/perf/orgunit-ext/bench_list_ext_query.sh`
  - `scripts/perf/orgunit-ext/bench_details_ext_fields.sh`
  - `scripts/perf/orgunit-ext/bench_field_configs_write.sh`
  - `scripts/perf/orgunit-ext/bench_corrections_ext_patch.sh`
  - `scripts/perf/orgunit-ext/aggregate_results.py`（汇总 P95/P99/错误率）

> Stopline：若未提供上述脚本或脚本无法复跑，则本计划不得标记“已完成”。

### 4.2 基线数据规模（冻结）

- 租户：1 个（独立租户基准）。  
- 组织单元：不少于 500 条。  
- 已启用扩展字段：不少于 2 个（至少包含 `org_type` + 1 个 PLAIN 字段）。  
- corrections 样本：不少于 200 条（含正例与负例）。  
- 列表查询样本：每轮至少覆盖 `parent_org_code` + ext filter/sort 组合。

### 4.3 最小证据产物（每次基准必须产出）

- 原始请求结果：`raw-<scenario>-<timestamp>.jsonl`
- 汇总报表：`summary-<timestamp>.md`
- 关键 SQL 与 explain（如有索引调整）：`explain-<scenario>-<timestamp>.txt`
- 结论记录：追加到 `docs/dev-records/dev-plan-100h-execution-log.md`

### 4.4 必须执行命令（冻结）

每次 Phase 5 基准至少执行以下命令（顺序固定）：

```bash
bash scripts/perf/orgunit-ext/prepare_dataset.sh
bash scripts/perf/orgunit-ext/bench_list_ext_query.sh
bash scripts/perf/orgunit-ext/bench_details_ext_fields.sh
bash scripts/perf/orgunit-ext/bench_field_configs_write.sh
bash scripts/perf/orgunit-ext/bench_corrections_ext_patch.sh
python3 scripts/perf/orgunit-ext/aggregate_results.py
```

> 命令输出与聚合结果必须落盘到 `docs/dev-records/perf/100h/`，并在执行日志中给出时间戳与文件名索引。

## 5. 异常场景清单（冻结）

| 类别 | 场景 | 期望错误码/行为 | 覆盖方式 |
| --- | --- | --- | --- |
| 槽位与映射 | 启用字段时槽位耗尽 | `ORG_FIELD_CONFIG_SLOT_EXHAUSTED` | API 契约测试 + E2E |
| 幂等冲突 | 同 `request_code` 不同输入 | `ORG_REQUEST_ID_CONFLICT` | API 契约测试 |
| 映射不可变 | 修改 `physical_col/value_type/...` | `ORG_FIELD_CONFIG_MAPPING_IMMUTABLE` | DB/Store 测试 |
| 访问防线 | 非 kernel 直写 metadata 表 | `ORGUNIT_FIELD_CONFIGS_WRITE_FORBIDDEN` | DB/Schema token 测试 |
| Query allowlist | 列表请求 ext 字段不在 allowlist | `ORG_EXT_QUERY_FIELD_NOT_ALLOWED` | API + 前端回归 |
| Options 能力 | PLAIN 调 options | `ORG_FIELD_OPTIONS_NOT_SUPPORTED` | API 测试 |
| 生效窗口 | 字段未 enabled-as-of 却查询 options | `ORG_FIELD_OPTIONS_FIELD_NOT_ENABLED_AS_OF` | API 测试 |
| 写入策略一致性 | 非 allowed_fields 提交 `patch.ext` | `PATCH_FIELD_NOT_ALLOWED` | 服务层测试 |
| 快照一致性 | DICT 缺失 label 快照 | `ORG_EXT_LABEL_SNAPSHOT_REQUIRED` | Kernel/集成测试 |
| 实体引用 | ENTITY 目标不存在/不可解析 | fail-closed（稳定错误码） | API/服务测试 |

## 6. 运维手册目录（冻结）

手册文件（计划）：`docs/dev-records/runbook-orgunit-ext-fields.md`

最小章节：
1. 启用字段操作手册（前置检查、request_code 规则、成功判据）  
2. 停用与延期停用手册（day 粒度、不可回滚规则）  
3. 回滚策略（仅环境级保护 + 只读/停写 + 修复后重试；禁止 legacy 回退）  
4. 常见故障排查（错误码 -> 原因 -> SQL/API 检查点）  
5. 索引调整流程（慢查询证据 -> 变更 -> 回归基准 -> 记录）

## 7. 里程碑与交付物

- H0：阈值与口径冻结（本文）。  
- H1：基准脚本落地 + 首轮基线报告（`docs/dev-records/perf/100h/`）。  
- H2：异常场景自动化覆盖补齐（API/服务/E2E）。  
- H3：运维手册落地（`runbook-orgunit-ext-fields.md`）。  
- H4：Phase 5 收口执行日志（`dev-plan-100h-execution-log.md`）与 `DEV-PLAN-100` 状态更新为“已完成”。

## 8. 验收标准（DoD）

- [ ] 阈值冻结并被评审确认（本文件为唯一 SSOT）。  
- [ ] 4 类核心场景（列表/详情/字段配置写/更正写）均有可复跑脚本与结果产物。  
- [ ] 异常场景清单全部有自动化覆盖（至少 API 或服务层；关键路径含 E2E）。  
- [ ] 运维手册落地并可按手册完成一次演练。  
- [ ] 若发生索引变更，必须附带“慢查询证据 + 基准前后对比”。

## 9. 门禁与触发器（SSOT 引用）

- 门禁入口与触发器：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`  
- 本计划通常命中：
  - 文档：`make check doc`
  - Go：`go fmt ./... && go vet ./... && make check lint && make test`
  - E2E：`make e2e`
  - 路由/Authz（若触及）：`make check routing`、`make authz-pack && make authz-test && make authz-lint`

## 10. 关联文档

- 总路线图：`docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- 既有阈值与基线证据：`docs/dev-plans/083b-org-mutation-capabilities-post-083a-closure-plan.md`
- Phase 1：`docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md`
- Phase 2：`docs/dev-plans/100c-org-metadata-wide-table-phase2-kernel-projection-extension-one-door.md`
- Phase 3：`docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- Phase 4A：`docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- Phase 4C：`docs/dev-plans/100g-org-metadata-wide-table-phase4c-orgunits-list-ext-query-i18n-closure.md`
- 字段配置页：`docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- 质量门禁：`docs/dev-plans/012-ci-quality-gates.md`
- 文档治理：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
- 仓库总规则：`AGENTS.md`
