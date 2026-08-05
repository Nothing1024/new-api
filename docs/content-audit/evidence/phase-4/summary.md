# Phase 4 Summary — Watchlist + 重扫

## 完成任务
- Task 23: model/audit_watchlist_rule.go + meta ✓
- Task 24: 更新 InitDB 注册 watchlist 表 ✓
- Task 25: service/audit_watchlist.go 扫描逻辑 ✓
- Task 26: audit_sink.go OnSettled 集成 watchlist 扫描 ✓
- Task 27: controller/audit_content.go watchlist CRUD + log content API ✓
- Task 28: controller/audit_content.go rescan + 重扫逻辑 ✓
- Task 29: router/api-router.go 注册审计路由 ✓
- Task 30: Phase 4 回归验证 ✓

## 实现要点
- `audit_watchlist_rules` / `audit_watchlist_meta`（主库）；增删改均 version++（BR-011）。
- `ScanSegments`：domain（Derived.Domains map）/ keyword（AC 自动机复用 getOrBuildAC）/
  regex（≤8 条 enabled，BR-010）三档扫描，命中存 rule_id + pattern 快照（BR-012）。
- 规则缓存 version 驱动（TTL 5s），sink worker 每请求读取。
- 重扫：500/批 + 100ms sleep，进度写 option `AuditRescanStatus`，只处理 TTL 内 wl_version 落后行（BR-013）。
- OnSettled 钩子传 `model.GetWatchlistVersion()` 作为 WLVersion。

## 重要修正
- `controller/audit.go` 是**既有**操作审计基础设施文件（recordManageAudit 等），
  我的新 handler 放到 `controller/audit_content.go`，避免覆盖受保护代码。

## 验证命令
| 命令 | 结果 |
|---|---|
| `make test` | 无 FAIL |
| `GOWORK=off go build ./...` + relaykit | BUILD_OK |
| `GOWORK=off go test ./service/... -run TestScanSegments` | ok |

## 真实场景
- 创建 domain+keyword 规则 → version 2；含"https://phishing.example.com"+"敏感词"请求 →
  hit_count=2, severity=high, flags 含两条 pattern 快照 ✓
- 第 9 条 enabled regex → 400（BR-010）✓
- 删除规则 → version++，规则消失（BR-011）✓
- 重扫 → processed 1/1, status=done, wl_version 更新 ✓
- 隔离：admin GET /api/log/content → 200；无/无效 token → 401；普通用户 token → 403 ✓
  （spec 预期 401，实际 AdminAuth 对已认证非管理员返回 403——隔离已强制，状态码差异记录于此）

## 剩余风险
- 无。
