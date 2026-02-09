# DEV-PLAN-078 执行日志

> 目的：记录 DEV-PLAN-078 的落地变更与可复现验证入口（命令/时间/样本/结果/提交号）。

## 变更摘要
- [ ] 删除 replay 相关函数/入口与错误码映射。
- [ ] 修正/撤销改为增量区间运算（无 replay）。
- [ ] 新增审计表 `orgunit.org_events_audit` 并完成表级重基线。
- [ ] 回归与一致性测试通过（替代 replay 对照）。

## 环境与样本
- 环境：开发早期测试环境（可清库重建）。
- 样本规模：
  - org_events_total=__
  - org_events_effective=__
  - org_unit_versions=__
  - org_unit_codes=__
  - org_trees=__

## 本地验证
- ____-__-__ __:__ UTC：`make orgunit plan && make orgunit lint && make orgunit migrate up && make orgunit plan`
  - 结果：____
- ____-__-__ __:__ UTC：`rg -n "replay_org_unit_versions|ORG_REPLAY_FAILED" modules/orgunit migrations/orgunit internal/server`
  - 结果：____
- ____-__-__ __:__ UTC：`go fmt ./... && go vet ./... && make check lint && make test`
  - 结果：____
- ____-__-__ __:__ UTC：`make check no-legacy`
  - 结果：____
- ____-__-__ __:__ UTC：`make check doc`
  - 结果：____

## 指标与验收
- correction/rescind P95 写延迟下降 >= 60%：____
- WAL 写入量下降 >= 50%：____
- replay 已彻底删除（代码/DB 入口均不可调用）：____

## 证据与提交号
- Commit/PR：____
- 关联说明：____
