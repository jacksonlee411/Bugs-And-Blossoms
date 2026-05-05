# DEV-PLAN-521：开发计划文档目的分类与元数据治理方案

**状态**: 规划中（2026-05-05 15:34 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T1`
- **范围一句话**：为 `docs/dev-plans/` 建立统一的目的分类、状态枚举和最小元数据约定，让活体计划、调查报告、治理方案、设计契约和路线图可被快速识别与索引。
- **关联模块/目录**：`docs/dev-plans/`、`docs/archive/dev-plans/`、`docs/dev-records/`、`AGENTS.md`
- **关联计划/标准**：`AGENTS.md`、`docs/dev-plans/000-docs-format.md`、`docs/dev-plans/003-simple-not-easy-review-guide.md`、`docs/dev-plans/013-docs-creation-and-governance-guide.md`、`docs/dev-plans/520-periodic-repository-noise-cleanup-and-search-surface-governance-plan.md`

### 0.1 Simple > Easy 三问

1. **边界**：本计划只定义文档分类与元数据，不触碰业务接口、数据库、鉴权、路由或 UI 行为。
2. **不变量**：每份活体开发计划都应能回答“它是什么类型”“现在是否有效”“谁是 owner”“它依赖什么入口”。
3. **可解释**：reviewer 应能仅凭文档标题、头部元数据和索引判断文档用途，而不需要先阅读全文猜测。

## 1. 背景与问题陈述

`docs/dev-plans/` 目前同时承载规范、契约、实施方案、调查、报告、路线图、治理文档与设计对齐文档。若没有统一分类，后续检索会出现三类问题：

1. 同名词条混读，无法快速分辨“标准”“计划”“调查”“报告”。
2. 活体 owner 与历史来源混在一起，容易把 archive 文档误认为当前 PoR。
3. 目录和文件名只能部分表达语义，缺少机器可读的最小元数据。

`DEV-PLAN-521` 的目标是把“文档到底是干什么的”固定下来，而不是要求所有文档写成同一种结构。

## 2. 目标与非目标

### 2.1 目标

1. [ ] 定义 `docs/dev-plans/` 的统一 `doc_type` 分类。
2. [ ] 定义 `docs/dev-plans/` 的统一 `status` 枚举。
3. [ ] 定义新增或修改活体文档的最小元数据字段。
4. [ ] 给 `AGENTS.md` 和未来 `docs/dev-plans/README.md` 提供可筛选索引口径。
5. [ ] 让调查、报告、路线图、契约、治理和实施计划不再互相冒用职责。

### 2.2 非目标

- 不要求全量改写历史文档。
- 不要求目录物理拆分成很多子目录。
- 不要求每份 archive 文档都补齐完整元数据。
- 不在本计划中实际移动、删除或重命名既有文档。
- 不引入外部文档系统或复杂标签平台。

## 3. 分类体系

### 3.1 `doc_type` 建议

| `doc_type` | 用途 | 典型文档 |
| --- | --- | --- |
| `standard` | 长期规则、规范、统一门禁口径 | `000`、`003`、`012`、`013`、`304` |
| `contract` | 当前系统契约、稳定接口、架构约束 | `019`、`022`、`431a`、`437a`、`461` |
| `implementation-plan` | 具体特性或重构的实施方案 | `440`、`450`、`493`、`502` |
| `roadmap` | 多计划排序、阶段安排、系列推进图 | `437`、`480a`、`391` |
| `investigation` | 调查方案、复现与采样口径 | `500` |
| `report` | 调查结果、事实总结、证据结论 | `501`、`463` |
| `remediation` | 修复、清理、收口、反回流 | `436`、`441`、`438b` |
| `design-contract` | UI / `.pen` / 页面设计对齐契约 | `431a`、`493` 相关设计引用 |
| `readiness-plan` | 准备就绪、验收前证据与命令记录 | `440` 的 readiness 记录、`437` 相关准备项 |
| `governance` | 长期治理、索引、归档、清理机制 | `520`、`521` |

### 3.2 `status` 建议

| `status` | 含义 |
| --- | --- |
| `draft` | 草拟中，结构未定 |
| `active` | 当前有效 owner，可作为实现依据 |
| `approved` | 边界已冻结，可进入实施 |
| `in-progress` | 已开始实施 |
| `completed` | 已完成，仍可作为当前参考 |
| `superseded` | 已被替代，不再作为当前依据 |
| `archived` | 仅历史来源 |
| `stopped` | 已明确停止，保留作阻断回流参考 |

### 3.3 最小元数据

建议新建或修改的活体 `docs/dev-plans/*.md` 至少具备：

```yaml
doc_type: implementation-plan
status: active
owner: DEV-PLAN-493
domain: orgunit
supersedes: []
superseded_by: null
implementation_entrypoints: []
design_refs: []
archive_refs: []
```

最小集合的含义：

- `doc_type`：文档用途。
- `status`：当前有效性。
- `owner`：当前主责编号。
- `domain`：主要业务域或治理域。
- `supersedes` / `superseded_by`：历史和替代关系。
- `implementation_entrypoints`：实现入口。
- `design_refs`：设计源。
- `archive_refs`：补充历史来源。

## 4. 索引与导航建议

1. 建议新增 `docs/dev-plans/README.md` 作为当前索引入口，按 `doc_type + status + domain` 过滤。
2. `AGENTS.md` 只保留活体高价值入口，不再承担完整目录镜像。
3. `archive` 文档只保留仍有追溯价值的内容，并明确标记为 `archived` 或 `superseded`。
4. 新增文档时，文件名、标题、元数据三者应一致，不让文件名单独承担全部语义。

## 5. 落地节奏

### 5.1 新增文档

1. [ ] 新增 dev-plan 时先确定 `doc_type` 和 `status`。
2. [ ] 补最小元数据，再写正文。
3. [ ] 若是活体计划，及时进入 `AGENTS.md` Doc Map。

### 5.2 修改文档

1. [ ] 修改活体文档时补齐或修正元数据。
2. [ ] 若文档已经被替代，更新 `status` 并写明 `superseded_by`。
3. [ ] 若只是历史来源，保持最小必要信息，不反复扩写过程细节。

### 5.3 历史文档

1. [ ] 对明显属于历史调查或已停止路线的文档，优先标记为 `archived` / `stopped` / `superseded`。
2. [ ] 需要保留的 archive 文档只保留追溯价值，不继续承担当前 owner 职责。

## 6. 验收标准

1. [ ] 新增或修改的 dev-plan 可以被稳定归入一个 `doc_type`。
2. [ ] reviewer 能凭头部元数据判断文档是否仍是当前 PoR。
3. [ ] `AGENTS.md` 和未来索引可以按 `status` 过滤出 active 文档。
4. [ ] 调查、报告、计划、规范、治理文档之间不再靠语感区分。
5. [ ] 分类规则足够轻量，不会逼迫历史文档一次性大改。

## 7. 风险与停止线

1. 若分类要求变成“先改完全部历史文档才能继续开发”，立即停止，回收为首发 metadata 规则。
2. 若分类体系增加太多层级，导致人反而更难判断文档用途，必须收敛回最小集合。
3. 若将分类规则用于业务门禁或代码实现，必须另起计划，不得在本计划中扩张。

## 8. 关联文档

1. 文档格式：`docs/dev-plans/000-docs-format.md`
2. Simple > Easy：`docs/dev-plans/003-simple-not-easy-review-guide.md`
3. Docs 治理：`docs/dev-plans/013-docs-creation-and-governance-guide.md`
4. 仓库定期清理：`docs/dev-plans/520-periodic-repository-noise-cleanup-and-search-surface-governance-plan.md`
