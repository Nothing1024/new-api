# Phase 3 Summary — Claude / Gemini 多格式 Walker

## 完成任务
- Task 19: audit/segment.go Claude walker ✓
- Task 20: audit/segment.go Gemini walker ✓
- Task 21: audit_sink.go OnInput 路由多格式 ✓
- Task 22: Phase 3 回归验证 ✓

## 实现要点
- `BuildClaudeSegments`：system（字符串 + 结构化 ParseSystem）、text/image/tool_use/tool_result/thinking
  分块；tool_result 块按 KindToolResult 处理（即使嵌在 user 消息内）。
- `BuildGeminiSegments`：SystemInstructions → system；role=user/model/function → user/assistant/tool_result；
  InlineData → omitted；FunctionCall → tool_call + derive；FunctionResponse → tool_result drop。
- `buildAuditInputSnapshot`（controller/relay.go）按 request 类型路由 OpenAI/Claude/Gemini，
  其余 opaque（fidelity=structured for 前三者）。

## 验证
| 命令 | 结果 |
|---|---|
| `make test` | 无 FAIL |
| `GOWORK=off go test ./audit/...` | ok（含 TestBuildClaudeSegments_* / TestBuildGeminiSegments_* 5 个）|
| `GOWORK=off go build ./...` + relaykit | BUILD_OK |

## 真实场景（curl 端到端）
- Claude `/v1/messages`（channel type 14）→ `fidelity=structured`，segments: system(preview 'claude 系统提示') + user(full) ✓
- Gemini `/v1beta/models/...:generateContent`（channel type 24）→ `fidelity=structured`，segments: user(full) ✓
- 注：PaLM type=11 通道有既有 convert 限制（`convert_request_failed`），改用 type=24 正常。

## 剩余风险
- Gemini/Claude 的 response 采集（OnOutput）未接入（spec 仅 P2 覆盖 OpenAI 响应）。
