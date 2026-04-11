# DEV-PLAN-320 Stopline Explain Report

- captured_at: `2026-04-11T01:02:36Z`
- as_of_date: `2026-04-11`

## Samples

- heavy tenant: `00000000-0000-0000-0000-000000000001` (`Local Tenant`, `localhost`)
- heavy root: `1` / `AAAAAAAB`
- heavy details target: `2` / `AAAAAAAC`
- heavy subtree filter: `3` / `AAAAAAAB.AAAAAAAD`
- heavy move: `3` -> `2` at `2026-04-12`
- chain tenant: `00d01cf4-7633-4ac6-93c7-748b2a6d6678` (`TP060-02 Tenant 1775779599776`, `t-tp060-02-1775779599776.localhost`)
- chain business unit: `RND9776` setid=`S2601`

## source-real

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `org-ancestor-chain` | 0.202 | 0.844 | 11 | 0 | 0 | 0 | `source-real-org-ancestor-chain.explain.json` |
| `org-children` | 0.227 | 0.669 | 21 | 0 | 0 | 0 | `source-real-org-children.explain.json` |
| `org-details` | 0.222 | 0.741 | 12 | 0 | 0 | 0 | `source-real-org-details.explain.json` |
| `org-full-name-rebuild` | 0.162 | 0.478 | 1 | 0 | 0 | 0 | `source-real-org-full-name-rebuild.explain.json` |
| `org-move` | 2.313 | 3.308 | 295 | 0 | 2 | 0 | `source-real-org-move.explain.json` |
| `org-roots` | 0.243 | 1.034 | 19 | 0 | 0 | 0 | `source-real-org-roots.explain.json` |
| `org-search` | 0.174 | 0.515 | 23 | 0 | 0 | 0 | `source-real-org-search.explain.json` |
| `org-subtree-filter` | 0.054 | 0.316 | 1 | 0 | 0 | 0 | `source-real-org-subtree-filter.explain.json` |
| `setid-resolve` | 0.184 | 0.633 | 14 | 0 | 0 | 0 | `source-real-setid-resolve.explain.json` |
| `staffing-by-org` | 0.247 | 0.241 | 7 | 0 | 0 | 0 | `source-real-staffing-by-org.explain.json` |

## target-real

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `org-ancestor-chain` | 0.160 | 1.246 | 9 | 0 | 0 | 0 | `target-real-org-ancestor-chain.explain.json` |
| `org-children` | 0.165 | 0.402 | 33 | 0 | 0 | 0 | `target-real-org-children.explain.json` |

说明：target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表

| `org-details` | 0.159 | 0.742 | 12 | 0 | 0 | 0 | `target-real-org-details.explain.json` |

说明：manager/person 关联未纳入当前 target bootstrap

| `org-full-name-rebuild` | 1.051 | 0.303 | 114 | 7 | 6 | 0 | `target-real-org-full-name-rebuild.explain.json` |
| `org-move` | 0.240 | 1.110 | 1 | 0 | 0 | 0 | `target-real-org-move.explain.json` |
| `org-roots` | 0.181 | 0.492 | 16 | 1 | 0 | 0 | `target-real-org-roots.explain.json` |

说明：target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表

| `org-search` | 0.186 | 0.803 | 18 | 0 | 0 | 0 | `target-real-org-search.explain.json` |
| `org-subtree-filter` | 0.168 | 0.546 | 18 | 0 | 1 | 0 | `target-real-org-subtree-filter.explain.json` |
| `setid-resolve` | 0.393 | 0.767 | 33 | 3 | 0 | 0 | `target-real-setid-resolve.explain.json` |

说明：在 dedicated target 的 `orgunit.setid_binding_versions` 内导入当前态样本；该 explain 已不再依赖 `stopline` shadow 表，但这仍不等于 P3 正式 runtime 切主

| `staffing-by-org` | 0.107 | 0.272 | 23 | 0 | 0 | 0 | `target-real-staffing-by-org.explain.json` |

说明：使用 committed staffing target schema（`staffing.position_versions`）采集 explain；当前态样本通过 org_code -> org_node_key 映射导入 dedicated target，但不再走 shadow 表


## Notes

1. `target-real` 当前覆盖 `orgunit` 新 schema、`orgunit.setid_binding_versions` 与 committed `staffing.position_versions`；当前态样本均通过 `org_code -> org_node_key` 映射导入 dedicated target。
2. `SetID` 的 explain 已不再依赖 `stopline` shadow 表，但这仍不等于 P3 正式 runtime 切主。
3. `org-move` 与 `org-full-name-rebuild` 均在事务内执行 `EXPLAIN (ANALYZE, BUFFERS)`，采集后由调用侧回滚。
4. 原始 explain JSON 见同目录 `*.explain.json`。
