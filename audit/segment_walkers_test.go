package audit

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeSegments_SimpleText(t *testing.T) {
	req := &dto.ClaudeRequest{
		System: "你是助手",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "你好 https://claude.example.com"},
			{Role: "assistant", Content: "我很好"},
		},
	}
	segs := BuildClaudeSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	byKind := segmentsByKind(segs)
	require.NotNil(t, byKind[KindSystem])
	assert.Equal(t, "你是助手", byKind[KindSystem].Text)
	require.NotNil(t, byKind[KindUser])
	assert.Equal(t, ModeFull, byKind[KindUser].Mode)
	require.NotNil(t, byKind[KindUser].Derived)
	assert.Contains(t, byKind[KindUser].Derived.Domains, "claude.example.com")
	require.NotNil(t, byKind[KindAssistant])
	assert.Equal(t, "我很好", byKind[KindAssistant].Text)
}

func TestBuildClaudeSegments_ToolUseAndResult(t *testing.T) {
	req := &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "tool_result", "tool_use_id": "tu_1", "content": "https://tool.example.com/data"},
				},
			},
			{
				Role: "assistant",
				Content: []any{
					map[string]any{"type": "text", "text": "calling"},
					map[string]any{"type": "tool_use", "id": "tu_1", "name": "search", "input": map[string]any{"q": "audit"}},
				},
			},
		},
	}
	segs := BuildClaudeSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	byKind := segmentsByKind(segs)
	require.NotNil(t, byKind[KindToolResult])
	assert.Equal(t, ModeDrop, byKind[KindToolResult].Mode)
	require.NotNil(t, byKind[KindToolResult].Derived)
	assert.Contains(t, byKind[KindToolResult].Derived.Domains, "tool.example.com")

	require.NotNil(t, byKind[KindToolCall])
	assert.Equal(t, ModeDerive, byKind[KindToolCall].Mode)
	require.NotNil(t, byKind[KindToolCall].Derived)
	assert.Equal(t, []string{"search"}, byKind[KindToolCall].Derived.Tools)
	assert.Equal(t, []string{"q"}, byKind[KindToolCall].Derived.ArgsKeys)
}

func TestBuildClaudeSegments_ImageOmitted(t *testing.T) {
	req := &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: []any{map[string]any{"type": "image", "source": map[string]any{"type": "base64"}}}},
		},
	}
	segs := BuildClaudeSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 1)
	assert.Equal(t, KindImage, segs[0].Kind)
	assert.Equal(t, ModeOmitted, segs[0].Mode)
}

func TestBuildGeminiSegments_SimpleText(t *testing.T) {
	req := &dto.GeminiChatRequest{
		SystemInstructions: &dto.GeminiChatContent{Parts: []dto.GeminiPart{{Text: "sys prompt"}}},
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "你好 https://gemini.example.com"}}},
			{Role: "model", Parts: []dto.GeminiPart{{Text: "hi there"}}},
		},
	}
	segs := BuildGeminiSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	byKind := segmentsByKind(segs)
	require.NotNil(t, byKind[KindSystem])
	assert.Equal(t, "sys prompt", byKind[KindSystem].Text)
	require.NotNil(t, byKind[KindUser])
	assert.Contains(t, byKind[KindUser].Derived.Domains, "gemini.example.com")
	require.NotNil(t, byKind[KindAssistant])
	assert.Equal(t, "hi there", byKind[KindAssistant].Text)
}

func TestBuildGeminiSegments_FunctionCallAndResponse(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "model", Parts: []dto.GeminiPart{
				{FunctionCall: &dto.FunctionCall{FunctionName: "get_weather", Arguments: map[string]any{"city": "beijing"}}},
			}},
			{Role: "function", Parts: []dto.GeminiPart{
				{FunctionResponse: &dto.GeminiFunctionResponse{Name: "get_weather", Response: map[string]interface{}{"temp": "20"}}},
			}},
		},
	}
	segs := BuildGeminiSegments(req, SegmentConfig{PerRequestMaxBytes: 65536})
	byKind := segmentsByKind(segs)
	require.NotNil(t, byKind[KindToolCall])
	assert.Equal(t, ModeDerive, byKind[KindToolCall].Mode)
	assert.Equal(t, []string{"get_weather"}, byKind[KindToolCall].Derived.Tools)
	assert.Equal(t, []string{"city"}, byKind[KindToolCall].Derived.ArgsKeys)

	require.NotNil(t, byKind[KindToolResult])
	assert.Equal(t, ModeDrop, byKind[KindToolResult].Mode)
}
