package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/audit"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// auditSinkTestDB swaps DB/LOG_DB for a fresh in-memory sqlite and restores the
// previous globals afterward.
func auditSinkTestDB(t *testing.T) {
	t.Helper()
	prevDB, prevLogDB := model.DB, model.LOG_DB
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.LogContent{}, &model.AuditWatchlistRule{}, &model.AuditWatchlistMeta{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = prevDB, prevLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
}

// BR-101/BR-102（INV-103）：flush 落库前清空 ScanText —— 序列化进 log_contents.segments
// 的 JSON 不含扫描全文，且 drop 段 text 为空（BR-104 留存面不变）。
func TestSinkFlushClearsScanText(t *testing.T) {
	auditSinkTestDB(t)

	sink := &LogContentSink{
		pending: make(map[string]*pendingRecord),
		flushed: make(map[string]time.Time),
	}
	rec := &pendingRecord{
		requestId: "req-flush-1",
		modelName: "gpt-4",
		segments: []audit.Segment{
			{Kind: audit.KindUser, Text: "user text", ScanText: "SECRET_FULL_USER_TEXT"},
			{Kind: audit.KindToolResult, Mode: audit.ModeDrop, ScanText: "SECRET_FULL_TOOL_RESULT"},
		},
		createdAt: time.Now(),
	}
	sink.flush(rec)

	lc, err := model.GetLogContent("req-flush-1")
	require.NoError(t, err)
	assert.NotContains(t, lc.Segments, "SECRET_FULL_USER_TEXT")
	assert.NotContains(t, lc.Segments, "SECRET_FULL_TOOL_RESULT")
	assert.Contains(t, lc.Segments, "user text")
	// drop 段 text 仍为空（BR-104 留存面）：全文只用于扫描，不落库。
	assert.NotContains(t, lc.Segments, "SECRET_FULL_TOOL_RESULT")
}

// BR-101 回归：watchlist 扫描必须发生在清空 ScanText 之前 —— drop 段（Text 空）的
// ScanText 全文中的关键词必须通过 flush 落库命中（此前清空在先导致全文匹配永不执行）。
func TestSinkFlushScansFullTextBeforeClear(t *testing.T) {
	auditSinkTestDB(t)

	// 清掉前序测试可能填充的规则缓存，强制 flush 重新加载种子规则。
	watchlistCacheMu.Lock()
	watchlistCacheRules = nil
	watchlistCacheAt = time.Time{}
	watchlistCacheMu.Unlock()

	// 种子 keyword 规则：命中 tool_result 全文中的关键词。
	require.NoError(t, model.CreateWatchlistRule(&model.AuditWatchlistRule{
		Kind:     model.WatchlistKindKeyword,
		Pattern:  "SECRET_KEYWORD_IN_FULL",
		Severity: "high",
		Enabled:  true,
	}))

	sink := &LogContentSink{
		pending: make(map[string]*pendingRecord),
		flushed: make(map[string]time.Time),
	}
	rec := &pendingRecord{
		requestId: "req-flush-scan-1",
		modelName: "gpt-4",
		// drop 段：Text 为空（截断/丢弃），ScanText 保留全文。
		segments: []audit.Segment{
			{Kind: audit.KindToolResult, Mode: audit.ModeDrop, ScanText: "tool result contains SECRET_KEYWORD_IN_FULL beyond truncation"},
		},
		createdAt: time.Now(),
	}
	sink.flush(rec)

	lc, err := model.GetLogContent("req-flush-scan-1")
	require.NoError(t, err)
	assert.Equal(t, 1, lc.HitCount, "keyword in ScanText must produce a hit through the sink")
	assert.Equal(t, "high", lc.HitSeverity)
	assert.Contains(t, lc.Flags, "SECRET_KEYWORD_IN_FULL")
	// 落库 segments 不含全文（BR-102）。
	assert.NotContains(t, lc.Segments, "SECRET_KEYWORD_IN_FULL")
	assert.NotContains(t, lc.Segments, "tool result contains")
}
