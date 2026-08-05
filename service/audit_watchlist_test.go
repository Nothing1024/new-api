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
