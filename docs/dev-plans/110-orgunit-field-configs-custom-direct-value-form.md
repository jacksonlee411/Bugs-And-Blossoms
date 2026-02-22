# DEV-PLAN-110：启用字段表单增强：自定义（直接值）+ 值类型选择 + 自定义字段名称

**状态**: 已完成（2026-02-18 21:00 UTC）

## 1. 背景（Research 结论）

`DEV-PLAN-101/106/106A` 已打通字段配置最小闭环，但与本计划目标仍有明确差距：

1. UI 文案仍是“自定义（PLAIN 文本）”，不符合用户心智。
2. `x_...` 自定义字段在启用时仍被固定为 `value_type='text'`，无法选择 `int/uuid/bool/date/numeric`。
3. 自定义字段未打通“启用时填写字段名称（display label）”的链路。

同时，底层能力已具备：

- DB Kernel `enable_tenant_field_config(...)` 已支持 `text/int/uuid/bool/date/numeric` 的槽位分配；
- `tenant_field_configs.display_label` 已存在并可持久化。

因此本计划采用“**收敛现有能力，不新增抽象**”策略，避免补丁式分叉（对齐 `DEV-PLAN-003`）。

## 2. 目标与非目标

### 2.1 目标（冻结）

1. 将启用字段表单文案“自定义（PLAIN 文本）”更名为 **自定义（直接值）**。
2. `x_...` 启用时必须可选全部预留槽位类型：`text/int/uuid/bool/date/numeric`。
3. `x_...` 启用时可选填写字段名称（display label），并在列表/详情展示优先使用。
4. 保持 One Door：写入仍只走 DB Kernel，不新增第二写入口，不引入 legacy 分支。

### 2.2 非目标（Stopline）

1. 不引入“业务数据多语言 label 存储”（仍为单一 canonical string）。
2. 不改变已启用映射不可变规则（`physical_col/value_type/data_source_type/data_source_config/enabled_on`）。
3. 不开放 `x_...` 的 `allow_filter/allow_sort`（保持 `false/false`）。

## 3. 按 DEV-PLAN-003 的方案约束（Simple > Easy）

### 3.1 边界（Boundary）

- 只改四个边界：`field-configs enable API`、`enable-candidates API`、`字段配置页启用对话框`、`展示 label 选择规则`。
- 不新增 service2/adapter2；复用现有 handler/store/kernel 路径。

### 3.2 不变量（Invariants）

1. `x_...` 必须是 `PLAIN`，且 `data_source_config={}`。
2. `x_...` 的 `value_type` 必须显式给定，且只允许六种枚举值。
3. `display_label` 仅影响展示，不参与权限/查询能力/数据源判定。
4. 任意非法输入必须 fail-closed，错误码稳定可排障。

### 3.3 五分钟可解释主流程

1. UI 选择“自定义（直接值）” -> 输入 `field_key`、选择 `value_type`、可选 `label`。
2. API 校验 `x_... + value_type + {}`，规范化为 `PLAIN` 写入参数。
3. Kernel 按 `value_type` 分配对应 `ext_*` 槽位并写事件。
4. 列表/详情读取时优先展示 `display_label`，否则回退 `field_key`。

## 4. 方案设计（冻结）

### 4.1 API 契约：`POST /org/api/org-units/field-configs`

对 `x_...` 请求冻结以下语义：

- `value_type`：必填，枚举 `text/int/uuid/bool/date/numeric`。
- `label`：可选，写入 `display_label`；空值归一化为 `NULL`。
- `data_source_type`：服务端固定为 `PLAIN`（客户端不作为权威来源）。
- `data_source_config`：服务端要求 `{}`（缺省补 `{}`，非 `{}` 直接拒绝）。

对非 `x_...` 路径保持现有契约（built-in、`d_<dict_code>`）不扩散复杂度。

### 4.2 API 契约：`GET /org/api/org-units/field-configs:enable-candidates`

将 `plain_custom_hint` 从“单一 text”扩展为“完整可选集”，建议结构：

- `pattern: "^x_[a-z0-9_]{1,60}$"`
- `value_types: ["text","int","uuid","bool","date","numeric"]`
- `default_value_type: "text"`

### 4.3 展示契约：`field-configs/details`

`x_...` 字段展示规则冻结为：

1. `display_label` 非空 -> `label=display_label`
2. 否则 -> `label=field_key`
3. `label_i18n_key=null`

### 4.4 UI IA（字段配置页启用对话框）

仅调整 custom 分支：

1. 文案改为“自定义（直接值）”。
2. 新增“字段名称（可选）”输入。
3. 新增“值类型（必选）”下拉（六种枚举）。
4. 保留 `x_...` 格式校验与 request_code 变更规则。

## 5. 失败模式与错误语义（冻结）

1. `value_type` 缺失或非法 -> `400 invalid_request`（或既有稳定业务错误码）。
2. `x_...` + 非 `{}` `data_source_config` -> `ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG`。
3. `x_...` 与现有 field_key 冲突 -> `ORG_FIELD_CONFIG_ALREADY_ENABLED`。
4. 槽位耗尽 -> `ORG_FIELD_CONFIG_SLOT_EXHAUSTED`。
5. 同 `request_code` 不同 payload -> `ORG_REQUEST_ID_CONFLICT`。

> 错误码命名遵循现有 SSOT，不在本计划引入新错误码体系。

## 6. 实施步骤（Contract First）

1. [X] 先完成“文档同步台账”（见 §6.1），再进入代码实现。
2. [X] 后端 API：`x_...` 路径接收/校验 `value_type`，写入 `display_label`。
3. [X] enable-candidates：返回 custom 可选 `value_types` 集合与默认值。
4. [X] 展示层：`x_...` 按 `display_label -> field_key` 回退链输出 `label`。
5. [X] 前端：更新文案与表单控件（字段名称 + 值类型），并接入新请求字段。
6. [X] 测试补齐（见 §7）并通过触发器门禁（见 §8）。

### 6.1 需同步更新文档计划（Contract First 台账）

> 规则：以下文档在对应状态变为“已完成”前，不得推进“实现完成”状态；避免代码先行导致契约漂移。

| 状态 | 文档 | 需同步更新内容（冻结） |
| --- | --- | --- |
| 已完成 | `docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md` | 将 `x_...` 从“固定 text”更新为“可选 `text/int/uuid/bool/date/numeric`”；补充 enable 请求中的 `value_type` 与 `label(display_label)` 语义。 |
| 已完成 | `docs/archive/dev-plans/101-orgunit-field-config-management-ui-ia.md` | 把“自定义（PLAIN 文本）”改为“自定义（直接值）”；新增 custom 分支的“值类型”与“字段名称（可选）”表单项。 |
| 已完成 | `docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md` | 在 106 文档中补充“被 110 扩展”的说明：自定义 PLAIN 不再固定 text，而是按槽位类型可选。 |
| 已完成 | `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md` | 对齐 110：确认 106A 中关于 `x_...` 的描述不再暗含“仅 text”，并保持 DICT 路径不受 110 影响。 |
| 已完成 | `docs/dev-plans/107-orgunit-ext-field-slots-expand-to-100.md` | 补充“110 直接消费 107 六种槽位能力”说明，作为容量与类型来源引用，避免两份枚举漂移。 |
| 已完成 | `AGENTS.md` | Doc Map 增补 `DEV-PLAN-110` 链接（若尚未登记）；若已登记，核对标题与文件名保持一致。 |
| 已完成 | `docs/dev-records/dev-plan-110-execution-log.md` | 新增执行日志文件，记录文档同步与实现门禁证据（时间戳、命令、结果）。 |

建议执行顺序（Simple 路径）：

1. 先更 `100d2`、`101`（直接契约与 IA）。  
2. 再更 `106`、`106A`、`107`（上游/旁路口径对齐）。  
3. 最后写 `dev-plan-110-execution-log.md` 作为收口证据。  

## 7. 测试策略（最小可审计）

1. Handler/API 测试：
   - `x_... + int/date/numeric` 启用成功；
   - `x_... + 非法 value_type` 失败；
   - `x_... + data_source_config!= {}` 失败。
2. 展示测试：
   - `display_label` 存在时展示 label；
   - 缺失时回退 field_key。
3. 前端测试：
   - custom 模式出现“值类型”“字段名称”控件；
   - 提交 payload 含 `value_type` 与可选 `label`。
4. E2E（至少一条）：
   - 创建 `x_cost_grade`（`value_type=int`）-> 详情可编辑并回显。

## 8. 门禁与证据（按 AGENTS 触发器）

本计划落地通常命中：

- 文档：`make check doc`
- Go：`go fmt ./... && go vet ./... && make check lint && make test`
- E2E：`make e2e`（若更新用例）
- 预检建议：`make preflight`

执行证据统一登记到 `docs/dev-records/dev-plan-110-execution-log.md`（实现阶段新增）。

## 9. 验收标准（DoD）

1. 启用表单文案已更名为“自定义（直接值）”。
2. `x_...` 可选择六种 `value_type`，且槽位分配类型正确。
3. `x_...` 可填写字段名称，列表与详情优先展示该名称。
4. 非法输入均 fail-closed，错误语义可解释且稳定。
5. 不新增 legacy 分支，不新增第二写入口。

## 10. 关联文档

- `docs/dev-plans/003-simple-not-easy-review-guide.md`
- `docs/dev-plans/100d2-org-metadata-wide-table-phase3-contract-alignment-and-hardening.md`
- `docs/archive/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- `docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md`
- `docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md`
- `docs/dev-plans/107-orgunit-ext-field-slots-expand-to-100.md`
- `AGENTS.md`
