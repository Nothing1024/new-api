package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildOpenAISegments_UserFull(t *testing.T) {
	big := strings.Repeat("a", 16*1024-30)
	text := big + " https://derived.example.com/x"
	msgs := []dto.Message{{Role: "user", Content: text}}
	segs := BuildOpenAISegments(msgs, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 1)
	assert.Equal(t, KindUser, segs[0].Kind)
	assert.Equal(t, ModeFull, segs[0].Mode)
	assert.Equal(t, text, segs[0].Text)
	assert.False(t, segs[0].Truncated)
	assert.NotEmpty(t, segs[0].SHA256)
	// BR-007：full 段也保留 derived facts（domain watchlist 匹配用）
	require.NotNil(t, segs[0].Derived)
	assert.Contains(t, segs[0].Derived.Domains, "derived.example.com")
}

func TestBuildOpenAISegments_KindMapping(t *testing.T) {
	msgs := []dto.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	segs := BuildOpenAISegments(msgs, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 3)
	assert.Equal(t, KindSystem, segs[0].Kind)
	assert.Equal(t, ModePreview, segs[0].Mode)
	assert.Equal(t, KindUser, segs[1].Kind)
	assert.Equal(t, KindAssistant, segs[2].Kind)
}

func TestBuildOpenAISegments_ToolResultDropDerived(t *testing.T) {
	msgs := []dto.Message{
		{Role: "tool", ToolCallId: "call_1", Content: "https://example.com/result 敏感内容"},
	}
	segs := BuildOpenAISegments(msgs, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 1)
	assert.Equal(t, KindToolResult, segs[0].Kind)
	assert.Equal(t, ModeDrop, segs[0].Mode)
	require.NotNil(t, segs[0].Derived)
	assert.Contains(t, segs[0].Derived.Domains, "example.com")
	assert.NotEmpty(t, segs[0].SHA256)
}

func TestBuildOpenAISegments_ToolCallDerive(t *testing.T) {
	msgs := []dto.Message{
		{
			Role: "assistant",
			Content: "calling tool",
			ToolCalls: json.RawMessage(`[{"id":"c1","type":"function","function":{"name":"search","arguments":"{\"q\":\"audit\"}"}}]`),
		},
	}
	segs := BuildOpenAISegments(msgs, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 2)
	var toolSeg *Segment
	for i := range segs {
		if segs[i].Kind == KindToolCall {
			toolSeg = &segs[i]
			break
		}
	}
	require.NotNil(t, toolSeg)
	assert.Equal(t, ModeDerive, toolSeg.Mode)
	require.NotNil(t, toolSeg.Derived)
	assert.Equal(t, []string{"search"}, toolSeg.Derived.Tools)
	assert.Equal(t, []string{"q"}, toolSeg.Derived.ArgsKeys)
}

func TestBuildOpenAISegments_BudgetDowngradeOrder(t *testing.T) {
	// 预算极小（1KB），user 16KB + assistant 16KB + system 1KB。
	// 降级顺序：system → assistant → user。user 最后被砍，但最终只降到 preview 而非 drop。
	msgs := []dto.Message{
		{Role: "system", Content: strings.Repeat("s", 1024)},
		{Role: "user", Content: strings.Repeat("u", 16*1024)},
		{Role: "assistant", Content: strings.Repeat("a", 16*1024)},
	}
	segs := BuildOpenAISegments(msgs, SegmentConfig{PerRequestMaxBytes: 1024})
	byKind := segmentsByKind(segs)
	require.NotNil(t, byKind[KindSystem])
	require.NotNil(t, byKind[KindAssistant])
	require.NotNil(t, byKind[KindUser])
	assert.Equal(t, ModeDrop, byKind[KindSystem].Mode)
	// assistant 被降为 preview（预算 1KB 下 system 先 drop，assistant 仅降级）
	assert.Equal(t, ModePreview, byKind[KindAssistant].Mode)
	// user 最后砍：预算 1KB 下最多到 preview，绝不为 drop。
	assert.Equal(t, ModePreview, byKind[KindUser].Mode)
	assert.True(t, byKind[KindUser].Truncated)
}

func TestBuildOpenAISegments_BudgetKeepsUserFull(t *testing.T) {
	// 预算 17KB：user 16KB(full) + assistant 16KB + system 1KB。
	// 先降 system 与 assistant，user prompt 保持 full（BR-009）。
	msgs := []dto.Message{
		{Role: "system", Content: strings.Repeat("s", 1024)},
		{Role: "user", Content: strings.Repeat("u", 16*1024)},
		{Role: "assistant", Content: strings.Repeat("a", 16*1024)},
	}
	segs := BuildOpenAISegments(msgs, SegmentConfig{PerRequestMaxBytes: 17 * 1024})
	byKind := segmentsByKind(segs)
	require.NotNil(t, byKind[KindSystem])
	require.NotNil(t, byKind[KindAssistant])
	require.NotNil(t, byKind[KindUser])
	assert.Equal(t, ModeDrop, byKind[KindSystem].Mode)
	assert.Equal(t, ModePreview, byKind[KindAssistant].Mode)
	// user 保持 full
	assert.Equal(t, ModeFull, byKind[KindUser].Mode)
	assert.False(t, byKind[KindUser].Truncated)
	assert.Equal(t, 16*1024, len(byKind[KindUser].Text))
}

func segmentsByKind(segs []Segment) map[string]*Segment {
	out := make(map[string]*Segment, len(segs))
	for i := range segs {
		if _, exists := out[segs[i].Kind]; !exists {
			out[segs[i].Kind] = &segs[i]
		}
	}
	return out
}

func TestBuildOpenAISegments_ImageOmitted(t *testing.T) {
	msgs := []dto.Message{
		{
			Role: "user",
			Content: []any{
				map[string]any{"type": "text", "text": "看图"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://img.example.com/a.png"}},
			},
		},
	}
	segs := BuildOpenAISegments(msgs, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 2)
	assert.Equal(t, ModeFull, segs[0].Mode)
	assert.Equal(t, KindImage, segs[1].Kind)
	assert.Equal(t, ModeOmitted, segs[1].Mode)
}

func TestBuildOpaqueSegment(t *testing.T) {
	body := []byte("post https://api.example.com/v1/chat 敏感词")
	segs := BuildOpaqueSegment(body, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 1)
	assert.Equal(t, ModePreview, segs[0].Mode)
	assert.NotEmpty(t, segs[0].SHA256)
	require.NotNil(t, segs[0].Derived)
	assert.Contains(t, segs[0].Derived.Domains, "api.example.com")
}

func TestBuildOpaqueSegment_Truncated(t *testing.T) {
	body := []byte(strings.Repeat("x", 2000))
	segs := BuildOpaqueSegment(body, SegmentConfig{PerRequestMaxBytes: 65536})
	require.Len(t, segs, 1)
	assert.True(t, segs[0].Truncated)
	assert.LessOrEqual(t, len(segs[0].Text), DefaultPreviewBytes)
}

func TestBuildMetaSegment(t *testing.T) {
	assert.Nil(t, BuildMetaSegment())
}

func TestDeriveFacts_UrlsAndDomains(t *testing.T) {
	text := "see https://a.com/x and http://b.org/y for details"
	facts := deriveFacts(text)
	require.NotNil(t, facts)
	assert.Contains(t, facts.URLs, "https://a.com/x")
	assert.Contains(t, facts.Domains, "a.com")
	assert.Contains(t, facts.Domains, "b.org")
}
