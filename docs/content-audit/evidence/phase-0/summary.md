# Phase 0 Summary — 勘察校准

## 完成任务
- Task 1: 校准 AutoMigrate 运行时建表疑点 ✓（F-35 闭环：正常行为，LogContent 未注册）
- Task 2: 验证 audit/service/relay 分层不成环 ✓（import cycle 复现确认，BR-004 强制）
- Task 3: 执行 Phase 0 回归验证 ✓

## 验证命令
| 命令 | 结果 |
|---|---|
| `make test` | 无 FAIL，exit 0（INV-003 基线确认）|
| `GOWORK=off go build ./...` | BUILD_OK（需先 `bun run build` 生成 web/dist）|
| `cd relaykit && GOWORK=off go build ./...` | BUILD_OK（INV-004）|
| `GOWORK=off go build ./audit/...` | BUILD_OK（audit 包可独立构建）|
| 成环模拟（audit→model→relay/common→audit）| `import cycle not allowed`（F-34 复现）|

## 环境准备记录
- 本机无 Go 工具链；已安装 go1.26.5 到 `~/opt/go`（spec F-19 记录的 go1.26.5 一致）
- 本机无 sqlite3 CLI；后续 DB 查询用 `python3 -c "import sqlite3..."` 替代
- `web/node_modules` 缺失 → `bun install`（1118 packages）→ `bun run build` 成功生成 web/dist

## 剩余风险
- 无。两个 P0 疑点均已闭环，P1 可安心开发。
