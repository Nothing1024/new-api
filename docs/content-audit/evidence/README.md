# Evidence Directory — content-audit

本目录保存「内容监控 / API 输入输出安全审计」功能的执行证据。没有 evidence，不视为完成。

## 目录结构

```text
evidence/
  phase-0/
    automigrate-diagnosis.txt   # Task 1：AutoMigrate 疑点结论 + go run 日志摘要
    import-cycle-proof.txt      # Task 2：audit 包分层验证命令输出
  phase-1/
    automigrate.txt             # Task 7：sqlite3 .tables 含 log_contents
    build-check.txt             # Task 4/6/8/9：GOWORK=off go build 输出
    sink-invoke.json            # Task 10/11：curl 请求后 sqlite3 query 结果
    segment-test.txt            # Task 5：go test ./audit/... 输出
    options-sync.txt            # Task 12：option 写入验证
    server.log                  # Task 11：相关 server log 片段（含 admin_info.audit）
  phase-2/
    response-capture.json       # Task 15/16/17：logs_content.segments 含 assistant kind
  phase-3/
    claude-segments.txt         # Task 19：Claude walker 单测结果
    gemini-segments.txt         # Task 20：Gemini walker 单测结果
  phase-4/
    tables.txt                  # Task 24：sqlite3 .tables 含 audit_watchlist_rules
    watchlist-crud.json         # Task 27：watchlist CRUD API response 样例
    scan-test.txt               # Task 25/26：扫描单测 + curl 含命中词 logs_content
    rescan.txt                  # Task 28：POST /api/audit/rescan + GET status 进度输出
  phase-5/
    i18n-check.txt              # Task 40：bun run i18n:sync 无缺失 key
    build-final.txt             # Task 43：make test + bun typecheck + relaykit build 全绿
  UF-001/
    badge.png                   # 命中行显示审计徽章截图
    no-hit.png                  # 无命中行无徽章截图
    disabled.png                # AuditEnabled=false 时无徽章截图
  UF-002/
    detail-tab.png              # 审计 Tab 展示 segments + flags 截图
    empty.png                   # 无审计记录空态截图
    error.png                   # 网络错误态截图
  UF-003/
    copy.png                    # 复制按钮状态变化截图
    no-copy.png                 # mode=drop segment 无复制按钮截图
  UF-004/
    user-response.json          # 普通用户 GET /api/log/self 响应（无 admin_info）
  UF-005/
    settings.png                # 审计配置区块截图 + 保存 toast
    validation.png              # 参数越界前端校验提示截图
  UF-006/
    crud.png                    # 规则列表 + 新增弹窗截图
    regex-limit.png             # 第 9 条 regex 被拒绝 toast 截图
    delete.png                  # 删除规则后列表截图
  UF-007/
    rescan-progress.png         # 重扫进度条更新截图
    no-op.png                   # 无需重扫 toast 截图
    in-progress.png             # 重扫中再次点击被拒绝截图
```

## EVD 对应表

| EVD ID | 期望证据 | 保存路径 |
|---|---|---|
| EVD-001 | `sqlite3 .tables` 含 `log_contents` | `evidence/phase-1/automigrate.txt` |
| EVD-002 | curl OpenAI 请求 → sqlite3 logs_content 有记录 | `evidence/phase-1/sink-invoke.json` |
| EVD-003 | 流式请求 → logs_content.segments 含 assistant | `evidence/phase-2/response-capture.json` |
| EVD-004 | watchlist CRUD API response + version 递增 | `evidence/phase-4/watchlist-crud.json` |
| EVD-005 | 审计徽章截图 + 详情弹窗审计 Tab 截图 | `evidence/UF-001/badge.png`, `evidence/UF-002/detail-tab.png` |
| EVD-006 | 普通用户响应无 admin_info 字段 | `evidence/UF-004/user-response.json` |
| EVD-007 | make test 全绿输出 | `evidence/phase-5/build-final.txt` |
| EVD-008 | go build ./... + relaykit build 输出 | `evidence/phase-1/build-check.txt` |
| EVD-009 | 重扫进度 option 更新截图 + server log | `evidence/UF-007/rescan-progress.png` |
| EVD-010 | 审计配置设置页截图 + 保存 toast | `evidence/UF-005/settings.png` |

## Phase Summary 模板

执行每个 Phase 后，在对应 `phase-N/` 目录创建 `summary.md`：

```markdown
# Phase N Summary — {phase_name}

## 完成任务
- Task X: {标题} ✓

## 验证命令
| 命令 | 结果 |
|---|---|
| `make test` | 无 FAIL |

## 用户路径 / API 验证
| UF/API | 结果 | Evidence |
|---|---|---|

## 剩余风险
- （无 / 具体描述）
```

## 命名约定

- 截图文件名含 UF 编号和状态：`UF-001-badge.png`（简写 `badge.png` 亦可）
- API 样例：`{endpoint}-{scenario}.json`
- 命令输出：保存完整命令 + 执行时间 + 结果摘要（复制终端输出即可）
- 任何 `已完成` 状态的真实场景任务，对应 `evidence/` 路径必须存在——否则 `validate_package.py` 二次运行会 FAIL
