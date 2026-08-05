# Phase 1 Summary — OnInput + OnSettled 采集骨架

## 完成任务
- Task 4: audit/ 包接口与快照类型 ✓
- Task 5: audit/segment.go OpenAI+opaque ✓
- Task 6: model/log_content.go DDL+CRUD ✓
- Task 7: migrateLOGDB 注册 LogContent ✓（+ 关键修复：migrateDB 主库也注册，见下）
- Task 8: service/audit_sink.go LogContentSink ✓
- Task 9: relay_info.go 注入 ContentSink 字段 ✓
- Task 10: controller/relay.go OnInput 钩子 ✓
- Task 11: service/text_quota.go OnSettled 钩子 ✓
- Task 12: model/option.go AuditEnabled 等 option ✓
- Task 13: Phase 1 回归验证 ✓

## 关键发现（超出 spec 预期）
1. **migrateLOGDB 在 LOG_SQL_DSN 为空时不会被调用**：`InitLogDB` 的 LOG_SQL_DSN 空分支
   （LOG_DB=DB，SQLite 默认）直接 return，不执行 migrateLOGDB。因此 `logs_content` 必须
   同时注册到主库 `migrateDB()` 的 AutoMigrate 列表（已追加 `&LogContent{}`），
   否则默认 SQLite 配置下表不存在。这是 F-35 的更深层根因。
2. 本机无 sqlite3 CLI；DB 查询用 python3 sqlite3 替代。
3. :3000 被其他项目（phy-sci next-server）占用；后端以 PORT=3456 运行。

## 验证命令
| 命令 | 结果 |
|---|---|
| `make test` | 无 FAIL（INV-003）|
| `GOWORK=off go build ./...` | BUILD_OK |
| `cd relaykit && GOWORK=off go build ./...` | BUILD_OK（INV-004）|
| `GOWORK=off go test ./audit/...` | ok（segment 单测 9 个）|
| sqlite3 .tables | 含 `log_contents`（EVD-001）|

## 真实场景验证（curl 完整路径）
- 启用 AuditEnabled 后发 OpenAI 请求 → `log_contents` 新增 1 行：
  `request_id, user_id=1, channel_id=1, model_name=gpt-3.5-turbo, fidelity=structured, hit_count=0`
- segments 正确：system=preview + user=full（含 derived.urls/domains）
- `logs.other.admin_info.audit` = `{"hit_count":0,"request_id":"..."}`（BR-002 指针，约 60B）
- 普通用户 alice 调 GET /api/log/self → `other` 无 admin_info（INV-005）✓

## 用户路径 / API 验证
| UF/API | 结果 | Evidence |
|---|---|---|
| EVD-001 | log_contents 表存在 | evidence/phase-1/automigrate.txt |
| EVD-002 | curl 请求 → log_contents 有记录 | evidence/phase-1/sink-invoke.json |
| INV-005 | 普通用户无 admin_info | 实测 alice /api/log/self ✓ |

## 剩余风险
- 无。P1 骨架闭环，可进入 P2 Response 采集。
