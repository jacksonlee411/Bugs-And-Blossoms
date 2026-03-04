# DEV-PLAN-238：LibreChat MongoDB 运行异常专项修复与防回归计划

**状态**: 草拟中（2026-03-03 23:39 UTC）

## 1. 背景
- 承接 `DEV-PLAN-232`（运行基线）与 `DEV-PLAN-237`（升级回归闭环）。
- 2026-03-04（Asia/Shanghai）现场出现 LibreChat 依赖异常：`mongodb` 容器退出，运行状态降级为 `unavailable`。
- 现场证据显示：
  1. MongoDB 日志在 `2026-03-03T23:10:09Z` 报 `FileNotOpen`（`/data/db/diagnostic.data/metrics.interim.temp`）与 `WT_PANIC`。
  2. Docker 启动错误为 `/data/db` 绑定源路径不存在（mount `no such file or directory`）。
  3. 运行路径与清理/文档口径存在偏差，导致数据目录治理存在漂移风险。

## 2. 目标与非目标
### 2.1 目标
1. [ ] 固化 LibreChat 数据目录单一口径，消除“运行路径 vs 清理路径 vs 文档路径”不一致。
2. [ ] 完成 MongoDB 异常根因修复，恢复 `assistant-runtime-up/status` 可重复通过。
3. [ ] 增加防回归校验：目录存在性、compose 解析结果与健康检查输出一致。
4. [ ] 补齐证据与操作手册，保证后续排障可审计。

### 2.2 非目标
1. [ ] 不在本计划引入新中间件或替换 MongoDB。
2. [ ] 不扩展业务能力（仅处理运行时稳定性与运维脚本收口）。
3. [ ] 不引入 legacy 双链路或旁路启动方式。

## 3. 输入与输出契约
### 3.1 输入
- `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
- `deploy/librechat/docker-compose.upstream.yaml`
- `deploy/librechat/docker-compose.overlay.yaml`
- `deploy/librechat/.env.example`
- `scripts/librechat/up.sh`
- `scripts/librechat/down.sh`
- `scripts/librechat/status.sh`
- `scripts/librechat/clean.sh`

### 3.2 输出
1. [ ] 路径口径收敛后的运行脚本与环境模板（单主路径、可预测、可重放）。
2. [ ] 运行健康检查增强（目录/挂载异常可快速识别）。
3. [ ] 文档与故障处置 Runbook 更新（含恢复流程与禁止项）。
4. [ ] `docs/dev-records/` 对应执行证据（时间、命令、结果）。

## 4. 实施步骤（直接落地）
1. [ ] 根因封板与证据固化
   - [ ] 固化日志证据：MongoDB fatal、Docker mount error、runtime-status 失败快照。
   - [ ] 固化“最小复现步骤”与“最小恢复步骤”。
2. [ ] 数据路径单一口径改造
   - [ ] 统一 `compose/env/scripts/README` 的数据根目录表达（禁止相对路径歧义）。
   - [ ] `clean.sh` 改为读取与运行时同源的目录配置，避免清理偏移。
3. [ ] 健康检查与启动前置校验增强
   - [ ] 启动前检查必需目录与挂载源可用性，不满足时 fail-fast 并给出明确错误。
   - [ ] `status.sh` 输出明确区分：容器未运行、挂载缺失、上游不可达。
4. [ ] 回归验证
   - [ ] 执行 `down -> clean -> up -> status` 闭环验证（至少 3 轮）。
   - [ ] 验证 MongoDB 重启后数据目录与容器状态一致。
5. [ ] 文档与门禁收口
   - [ ] 更新 `deploy/librechat/README.md` 与相关 dev-record 证据。
   - [ ] 执行 `make check doc`，确保文档门禁通过。

## 5. 验收与门禁
1. [ ] `make assistant-runtime-up` 可稳定拉起 `api/mongodb/meilisearch/rag_api/vectordb`。
2. [ ] `make assistant-runtime-status` 输出 `status=healthy` 且 MongoDB 为 `healthy`。
3. [ ] MongoDB 容器重启后不再出现 mount 源路径缺失导致的 `Exited (14)`。
4. [ ] 文档口径与脚本行为一致，不再出现路径定义分叉。
5. [ ] `make check doc` 通过。

## 6. 风险与缓解
1. [ ] 风险：历史残留容器仍持有旧挂载定义。  
   缓解：提供“停机+移除旧容器+按新配置重建”的标准流程。
2. [ ] 风险：开发环境（WSL/Docker Desktop）路径语义差异导致偶发失败。  
   缓解：启动脚本增加绝对路径归一与可读错误提示。
3. [ ] 风险：后续改动再次引入口径漂移。  
   缓解：在回归脚本中加入路径一致性断言，并纳入升级回归清单（承接 `DEV-PLAN-237`）。

## 7. SSOT 引用
- `AGENTS.md`
- `Makefile`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/230-librechat-project-level-integration-plan.md`
- `docs/dev-plans/232-librechat-official-runtime-baseline-plan.md`
- `docs/dev-plans/237-librechat-upgrade-and-regression-closure-plan.md`
