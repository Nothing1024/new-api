package service

import (
	"testing"

	"github.com/QuantumNous/new-api/audit"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enabledRule(id uint, kind, pattern, severity string) model.AuditWatchlistRule {
	return model.AuditWatchlistRule{Id: id, Kind: kind, Pattern: pattern, Severity: severity, Enabled: true}
}

func TestScanSegments_Domain(t *testing.T) {
	segs := []audit.Segment{{
		Kind: audit.KindUser,
		Text: "https://evil.example.com/x",
		Derived: &audit.DerivedFacts{Domains: []string{"evil.example.com"}},
	}}
	rules := []model.AuditWatchlistRule{
		enabledRule(1, model.WatchlistKindDomain, "evil.example.com", "high"),
	}
	flags := ScanSegments(segs, rules)
	require.Len(t, flags, 1)
	assert.Equal(t, uint(1), flags[0].RuleId)
	assert.Equal(t, "evil.example.com", flags[0].PatternSnapshot)
	assert.Equal(t, "high", flags[0].Severity)
	assert.Equal(t, 0, flags[0].SegIdx)
}

func TestScanSegments_Keyword(t *testing.T) {
	segs := []audit.Segment{{Kind: audit.KindUser, Text: "这条消息包含 敏感词A 和 敏感词B"}}
	rules := []model.AuditWatchlistRule{
		enabledRule(2, model.WatchlistKindKeyword, "敏感词A", "low"),
		enabledRule(3, model.WatchlistKindKeyword, "敏感词B", "medium"),
	}
	flags := ScanSegments(segs, rules)
	require.Len(t, flags, 2)
	assert.Equal(t, uint(2), flags[0].RuleId)
	assert.Equal(t, uint(3), flags[1].RuleId)
}

func TestScanSegments_Regex(t *testing.T) {
	segs := []audit.Segment{{Kind: audit.KindAssistant, Text: "API key: sk-abc12345xyz"}}
	rules := []model.AuditWatchlistRule{
		enabledRule(4, model.WatchlistKindRegex, `sk-[a-zA-Z0-9]{8,}`, "high"),
	}
	flags := ScanSegments(segs, rules)
	require.Len(t, flags, 1)
	assert.Equal(t, "high", flags[0].Severity)
}

func TestScanSegments_DisabledRuleSkipped(t *testing.T) {
	segs := []audit.Segment{{Kind: audit.KindUser, Text: "命中 敏感词C"}}
	rules := []model.AuditWatchlistRule{
		{Id: 5, Kind: model.WatchlistKindKeyword, Pattern: "敏感词C", Severity: "high", Enabled: false},
	}
	assert.Empty(t, ScanSegments(segs, rules))
}

func TestScanSegments_MaxSeverity(t *testing.T) {
	flags := []audit.HitFlag{
		{Severity: "low"},
		{Severity: "high"},
		{Severity: "medium"},
	}
	assert.Equal(t, "high", MaxSeverity(flags))
	assert.Equal(t, "", MaxSeverity(nil))
}

// BR-104：drop 段 Text 为空但 ScanText 有全文 → keyword 档命中（BR-101 扫描面）。
func TestScanSegments_KeywordScanText(t *testing.T) {
	segs := []audit.Segment{{
		Kind:     audit.KindToolResult,
		Mode:     audit.ModeDrop,
		Text:     "",
		ScanText: "联网结果包含 敏感词A 的全文（超过1KB，Text 为空）",
	}}
	rules := []model.AuditWatchlistRule{
		enabledRule(6, model.WatchlistKindKeyword, "敏感词A", "medium"),
	}
	flags := ScanSegments(segs, rules)
	require.Len(t, flags, 1)
	assert.Equal(t, uint(6), flags[0].RuleId)
	assert.Equal(t, "medium", flags[0].Severity)
}

// BR-104：drop 段 Text 为空但 ScanText 有全文 → regex 档命中。
func TestScanSegments_RegexScanText(t *testing.T) {
	segs := []audit.Segment{{
		Kind:     audit.KindToolResult,
		Mode:     audit.ModeDrop,
		Text:     "",
		ScanText: "key=sk-abc12345xyz",
	}}
	rules := []model.AuditWatchlistRule{
		enabledRule(7, model.WatchlistKindRegex, `sk-[a-zA-Z0-9]{8,}`, "high"),
	}
	flags := ScanSegments(segs, rules)
	require.Len(t, flags, 1)
	assert.Equal(t, uint(7), flags[0].RuleId)
}

// BR-101：ScanText 优先于 Text（两者都非空时扫描全文而非截断文）。
func TestScanSegments_ScanTextPrecedence(t *testing.T) {
	segs := []audit.Segment{{
		Kind:     audit.KindUser,
		Text:     "clean visible text",
		ScanText: "full text contains 敏感词B beyond truncation",
	}}
	rules := []model.AuditWatchlistRule{
		enabledRule(8, model.WatchlistKindKeyword, "敏感词B", "low"),
	}
	flags := ScanSegments(segs, rules)
	require.Len(t, flags, 1)
	assert.Equal(t, uint(8), flags[0].RuleId)
}
