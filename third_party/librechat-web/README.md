# LibreChat Web UI vendoring

本目录是 `DEV-PLAN-281` 冻结后的 LibreChat Web UI 单一来源骨架。

- 来源元数据：`third_party/librechat-web/UPSTREAM.yaml`
- 源码根目录：`third_party/librechat-web/source/`
- patch stack：`third_party/librechat-web/patches/`
- 构建脚本：`scripts/librechat-web/verify.sh`、`scripts/librechat-web/build.sh`
- 静态产物出口：`internal/server/assets/librechat-web/`

边界冻结：

- 仅纳管 LibreChat Web UI 所需源码与构建资产。
- 不纳管上游 Node backend runtime 作为本仓正式实现面。
- 不允许绕过 `UPSTREAM.yaml` 与 `patches/` 直接散改来源口径。
- `DEV-PLAN-281` 完成后，旧 `iframe` / `bridge.js` / HTML 注入链路不再作为新增功能承载面。


源码快照状态：

- 当前已导入上游 `refs/tags/v0.8.0` 的最小前端源码集。
- 当前已验证 `make librechat-web-build` 可成功产出 `internal/server/assets/librechat-web/`。
- 构建在临时目录进行，并按 `patches/series` 顺序回放 patch，避免直接污染 `source/`。
