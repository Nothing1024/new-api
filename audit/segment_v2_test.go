package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BR-102：ScanText 标 json:"-"，序列化结果不含扫描全文（落库面与匹配面分离）。
func TestScanTextNotMarshal(t *testing.T) {
	b, err := common.Marshal(Segment{Kind: KindUser, Text: "keep", ScanText: "SECRET_FULL_TEXT"})
	require.NoError(t, err)
	js := string(b)
	assert.NotContains(t, js, "SECRET_FULL_TEXT")
	assert.Contains(t, js, "keep")
}

// BR-120：defaultKindPolicy 各 kind 值与 F-101 一致，tool_def=preview/1024。
func TestDefaultKindPolicy(t *testing.T) {
	assert.Equal(t, kindPolicy{ModePreview, DefaultSystemPreviewBytes}, defaultKindPolicy[KindSystem])
	assert.Equal(t, kindPolicy{ModeFull, DefaultUserFullBytes}, defaultKindPolicy[KindUser])
	assert.Equal(t, kindPolicy{ModeFull, DefaultAssistantFullBytes}, defaultKindPolicy[KindAssistant])
	assert.Equal(t, kindPolicy{ModeDerive, DefaultToolCallDeriveBytes}, defaultKindPolicy[KindToolCall])
	assert.Equal(t, kindPolicy{ModeDrop, 0}, defaultKindPolicy[KindToolResult])
	assert.Equal(t, kindPolicy{ModePreview, DefaultToolDefPreviewBytes}, defaultKindPolicy[KindToolDef])
	assert.Equal(t, kindPolicy{ModeOmitted, 0}, defaultKindPolicy[KindImage])
	assert.Equal(t, kindPolicy{ModeOmitted, 0}, defaultKindPolicy[KindAudio])
}

// BR-106：downgradeOrder 次序 tool_result < tool_def < tool_call < system < assistant < user。
func TestDowngradeOrder(t *testing.T) {
	expected := []string{KindToolResult, KindToolDef, KindToolCall, KindSystem, KindAssistant, KindUser}
	assert.Equal(t, expected, downgradeOrder)
}

// BR-103：OpenAI tool_call 参数全文 derive URLs/Domains + 填 ScanText。
func TestToolCallDerivesFacts(t *testing.T) {
	args := `{"url":"https://evil.com/x","q":"敏感词A"}`
	call := dto.ToolCallRequest{
		ID:       "c1",
		Type:     "function",
		Function: dto.FunctionRequest{Name: "search", Arguments: args},
	}
	seg := makeToolCallSegment(call, 0)
	assert.Equal(t, ModeDerive, seg.Mode)
	require.NotNil(t, seg.Derived)
	assert.Contains(t, seg.Derived.Domains, "evil.com")
	assert.Contains(t, seg.Derived.Tools, "search")
	assert.Equal(t, args, seg.ScanText)
	assert.NotEmpty(t, seg.Text)
}

// BR-103：Claude tool_use 补齐 Text/ScanText/Derived。
func TestClaudeToolUseComplete(t *testing.T) {
	mc := dto.ClaudeMediaMessage{
		Type:  "tool_use",
		Name:  "search",
		Input: map[string]any{"url": "https://evil.com/x", "q": "敏感词A"},
	}
	seg := makeClaudeToolUseSegment(mc, 0)
	assert.Equal(t, ModeDerive, seg.Mode)
	require.NotNil(t, seg.Derived)
	assert.Contains(t, seg.Derived.Domains, "evil.com")
	assert.Contains(t, seg.Derived.Tools, "search")
	assert.NotEmpty(t, seg.Text)
	assert.NotEmpty(t, seg.ScanText)
	assert.Contains(t, seg.ScanText, "evil.com")
}

// BR-103：Gemini function_call 补齐 Text/ScanText/Derived。
func TestGeminiFunctionCallComplete(t *testing.T) {
	fc := &dto.FunctionCall{
		FunctionName: "search",
		Arguments:    map[string]any{"url": "https://evil.com/x", "q": "敏感词A"},
	}
	seg := makeGeminiFunctionCallSegment(fc, 0)
	assert.Equal(t, ModeDerive, seg.Mode)
	require.NotNil(t, seg.Derived)
	assert.Contains(t, seg.Derived.Domains, "evil.com")
	assert.Contains(t, seg.Derived.Tools, "search")
	assert.NotEmpty(t, seg.Text)
	assert.NotEmpty(t, seg.ScanText)
	assert.Contains(t, seg.ScanText, "evil.com")
}

// BR-104：drop 段 Text 为空但 ScanText 保留全文。
func TestDropSegmentScanText(t *testing.T) {
	text := "包含 敏感词A 的 tool 结果全文"
	seg := makeDropSegment(KindToolResult, text, 0, "tool_result")
	assert.Equal(t, "", seg.Text)
	assert.Equal(t, text, seg.ScanText)
	assert.Equal(t, ModeDrop, seg.Mode)
}

// BR-105：OpenAI 请求带 3 个 tools → 3 条 tool_def 段。
func TestBuildOpenAIInputSegmentsToolDef(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "a", Description: "tool a"}},
			{Type: "function", Function: dto.FunctionRequest{Name: "b", Description: "tool b"}},
			{Type: "function", Function: dto.FunctionRequest{Name: "c", Description: "tool c"}},
		},
	}
	segs := BuildOpenAIInputSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	toolDefs := 0
	for _, s := range segs {
		if s.Kind == KindToolDef {
			toolDefs++
			assert.Equal(t, ModePreview, s.Mode)
			assert.NotEmpty(t, s.Text)
			assert.NotEmpty(t, s.ScanText)
		}
	}
	assert.Equal(t, 3, toolDefs)
	// 无 tools 时不产生 tool_def 段。
	req.Tools = nil
	segs = BuildOpenAIInputSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	for _, s := range segs {
		assert.NotEqual(t, KindToolDef, s.Kind)
	}
}

// BR-105：Claude 请求带 tools → tool_def 段。
func TestBuildClaudeInputSegmentsToolDef(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model: "claude-3",
		Tools: []any{
			&dto.Tool{Name: "search", Description: "search web", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}
	segs := BuildClaudeInputSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 1)
	assert.Equal(t, KindToolDef, segs[0].Kind)
	assert.Equal(t, ModePreview, segs[0].Mode)
}

// BR-105：Gemini 请求带 tools → tool_def 段（复用 GetTools 解析器，F-137）。
func TestBuildGeminiInputSegmentsToolDef(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Tools: json.RawMessage(`[{"functionDeclarations":[{"name":"search","description":"web search","parameters":{"type":"object"}}]}]`),
	}
	segs := BuildGeminiInputSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 1)
	assert.Equal(t, KindToolDef, segs[0].Kind)
	assert.Equal(t, ModePreview, segs[0].Mode)
}

// BR-105 + BR-106：tool_def 参与降级（tool_result 之后第二个被砍）。
func TestBudgetDowngradesToolDefBeforeToolCall(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "user", Content: strings.Repeat("u", 16*1024)},
			{Role: "assistant", Content: strings.Repeat("a", 16*1024)},
		},
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: strings.Repeat("t", 2000)}},
		},
	}
	segs := BuildOpenAIInputSegments(req, SegmentConfig{PerRequestMaxBytes: 1024})
	byKind := map[string]Segment{}
	for _, s := range segs {
		if _, ok := byKind[s.Kind]; !ok {
			byKind[s.Kind] = s
		}
	}
	// BR-106：预算极小时 tool_def 先于 user 被砍（tool_def 降为 drop，user 保持 preview 而非 drop）。
	td, ok := byKind[KindToolDef]
	require.True(t, ok, "tool_def segment must exist")
	assert.Equal(t, ModeDrop, td.Mode, "tool_def must be dropped first under tight budget")
	u, ok := byKind[KindUser]
	require.True(t, ok, "user segment must exist")
	assert.NotEqual(t, ModeDrop, u.Mode, "user is last in downgradeOrder, must not be dropped")
	assert.Equal(t, ModePreview, u.Mode, "user degrades to preview, never drop")
}

// BR-107：输出侧 tool_calls 独立成段（BuildOutputToolCallSegments 复用 makeToolCallSegment）。
func TestBuildOutputToolCallSegments(t *testing.T) {
	tcs := []dto.ToolCallRequest{
		{ID: "c1", Type: "function", Function: dto.FunctionRequest{Name: "search", Arguments: `{"url":"https://evil.com/x","q":"敏感词B"}`}},
		{ID: "c2", Type: "function", Function: dto.FunctionRequest{Name: "weather", Arguments: `{"city":"beijing"}`}},
	}
	segs := BuildOutputToolCallSegments(tcs, 65536)
	require.Len(t, segs, 2)
	for _, s := range segs {
		assert.Equal(t, KindToolCall, s.Kind)
		assert.Equal(t, ModeDerive, s.Mode)
		assert.NotEmpty(t, s.ScanText) // BR-103：输出侧同样填全文
	}
	// 参数含 URL → domain 命中（BR-103）。
	assert.Contains(t, segs[0].Derived.Domains, "evil.com")
	// 无 tool_calls → 空。
	assert.Nil(t, BuildOutputToolCallSegments(nil, 65536))
}
