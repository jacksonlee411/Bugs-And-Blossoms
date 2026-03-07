# DEV-PLAN-281 执行日志（LibreChat Web UI 源码纳管与新主链路冻结）

## 1. 执行范围
- 冻结 `third_party/librechat-web/` 单一来源目录与 `UPSTREAM.yaml` 来源元数据。
- 落地 `patches/series` 与首个补丁 `0001-use-node-fs-in-post-build.patch`。
- 落地 `scripts/librechat-web/{verify,build}.sh` 与 `Makefile` 入口。
- 冻结静态产物出口 `internal/server/assets/librechat-web/`。

## 2. 实施结果
- [X] `third_party/librechat-web/source/` 已导入上游前端源码快照。
- [X] `make librechat-web-verify` 通过（`state=source_imported`，来源字段可审计）。
- [X] `make librechat-web-build` 通过，产物输出到 `internal/server/assets/librechat-web/`。
- [X] 连续两次构建并对比全量文件 `sha256`，结果 `reproducible=exact-match`。

## 3. 验证命令
```bash
make librechat-web-verify
make librechat-web-build
find internal/server/assets/librechat-web -type f -print0 | sort -z | xargs -0 sha256sum > /tmp/librechat-web-build-1.sha256
make librechat-web-build
find internal/server/assets/librechat-web -type f -print0 | sort -z | xargs -0 sha256sum > /tmp/librechat-web-build-2.sha256
cmp -s /tmp/librechat-web-build-1.sha256 /tmp/librechat-web-build-2.sha256 && echo reproducible=exact-match
```

## 4. 已知告警（不阻断 281 完成）
- 前端构建阶段存在 Browserslist 过期提示（`caniuse-lite is outdated`）。
- Vite 输出大 chunk 警告（`Some chunks are larger than 1500 kB`）。
- PWA 预缓存 glob 警告（若干 `assets/favicon*.png` 匹配为空）。

上述告警均不影响“来源冻结 + patch 回放 + 可重复构建”这一 281 目标达成；后续优化由 `DEV-PLAN-237/285` 统筹。

## 5. 后续承接
- `DEV-PLAN-282`：删除旧桥接正式职责。
- `DEV-PLAN-283`：正式入口切换。
- `DEV-PLAN-284`：发送/渲染链路源码级接管。
- `DEV-PLAN-285`：切换回归与封板。
