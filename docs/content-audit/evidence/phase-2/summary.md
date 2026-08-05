# Phase 2 Summary — Response 采集

## 完成任务
- Task 14: audit/types.go 补充 OutputSnapshot ✓（Task 4 已定义，字段覆盖 RequestId + Segments）
- Task 15: OaiStreamHandler 添加 OnOutput 钩子 ✓
- Task 16: OpenaiHandler 添加 OnOutput 钩子 ✓
- Task 17: audit_sink.go OnOutput 合并写入 logs_content ✓（含 late-arrival 合并）
- Task 18: Phase 2 回归验证 ✓

## 实现要点
- 新增 `audit.BuildAssistantOutputSegment(text, maxBytes)`：assistant 输出段，上限 = min(16KB, per_request_max_bytes)。
- 流式：OaiStreamHandler 末尾用 `responseTextBuilder.String()`（全文已累积）→ OnOutput。
- 非流式：OpenaiHandler 末尾从 `simpleResponse.Choices[].Message.StringContent()` 拼装 → OnOutput。
- **竞态修复**：OnOutput 可能在 OnSettled 落库后才到达（异步 goroutine 顺序不保证）。
  sink 的 applyOutput 在记录已移除时调用 `mergeOutputToDB` 把输出段合并进现有 logs_content 行，
  保证 BR-001 1:1 且不丢数据。

## 验证命令
| 命令 | 结果 |
|---|---|
| `make test` | 无 FAIL |
| `GOWORK=off go build ./...` | BUILD_OK |

## 真实场景验证（curl 流式 + 非流式）
```
非流式: user(full '非流式测试 https://ns.example.com') + assistant(full '非流式响应文本')
流式:   user(full '流式测试 https://s.example.com') + assistant(full '流式响应')
```
两个请求 logs_content.segments 均含 user + assistant kind（EVD-003）✓

## 剩余风险
- 无。
