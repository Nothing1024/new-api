package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// auditTemplateTestDB swaps DB/LOG_DB for a fresh in-memory sqlite DB preloaded
// with the tables under test and restores the previous globals afterward.
func auditTemplateTestDB(t *testing.T) {
	t.Helper()
	prevDB, prevLogDB := DB, LOG_DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&AuditWatchlistRule{}, &AuditWatchlistMeta{}, &LogContent{}))
	t.Cleanup(func() {
		DB, LOG_DB = prevDB, prevLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
}

func sampleTemplateRules(templateID string) []AuditWatchlistRule {
	return []AuditWatchlistRule{
		{Kind: WatchlistKindKeyword, Pattern: "ignore previous instructions", Severity: "high", Enabled: true, Note: "injection"},
		{Kind: WatchlistKindRegex, Pattern: `sk-[A-Za-z0-9]{48}`, Severity: "high", Enabled: false, Note: "api key"},
	}
}

// BR-114: 同一模板重复应用不产生重复规则。
func TestApplyTemplateRulesIdempotent(t *testing.T) {
	auditTemplateTestDB(t)

	applied, skipped, err := ApplyTemplateRules("basic-security", sampleTemplateRules("basic-security"))
	require.NoError(t, err)
	assert.Equal(t, 2, applied)
	assert.Equal(t, 0, skipped)

	// Second apply with identical rules must be a no-op.
	applied2, skipped2, err := ApplyTemplateRules("basic-security", sampleTemplateRules("basic-security"))
	require.NoError(t, err)
	assert.Equal(t, 0, applied2)
	assert.Equal(t, 2, skipped2)

	n, err := CountWatchlistRulesByTemplate("basic-security", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	rules, err := ListWatchlistRulesByTemplate("basic-security")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	for _, r := range rules {
		assert.Equal(t, "template", r.Source)
		assert.Equal(t, "basic-security", r.TemplateId)
	}
}

// BR-115: 整包启用只影响模板规则，整包停用把 enabled 置 false。
func TestEnableDisableTemplateRules(t *testing.T) {
	auditTemplateTestDB(t)

	_, _, err := ApplyTemplateRules("privacy-pii", sampleTemplateRules("privacy-pii"))
	require.NoError(t, err)

	// Disable: all template rules turned off.
	_, err = DisableTemplateRules("privacy-pii")
	require.NoError(t, err)
	enabledCount, err := CountWatchlistRulesByTemplate("privacy-pii", boolPtr(true))
	require.NoError(t, err)
	assert.Equal(t, int64(0), enabledCount)

	// Enable: non-regex enabled, regex stays disabled (BR-116).
	affected, err := EnableTemplateRules("privacy-pii")
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
	enabledCount, err = CountWatchlistRulesByTemplate("privacy-pii", boolPtr(true))
	require.NoError(t, err)
	assert.Equal(t, int64(1), enabledCount)

	// Regex remains disabled by design.
	var regexCnt int64
	require.NoError(t, DB.Model(&AuditWatchlistRule{}).
		Where("template_id = ? AND kind = ? AND enabled = ?", "privacy-pii", WatchlistKindRegex, true).
		Count(&regexCnt).Error)
	assert.Equal(t, int64(0), regexCnt)
}

// 移除模板只删模板规则。
func TestDeleteTemplateRulesScopedToTemplate(t *testing.T) {
	auditTemplateTestDB(t)

	_, _, err := ApplyTemplateRules("api-key-leak", sampleTemplateRules("api-key-leak"))
	require.NoError(t, err)
	// A manual rule outside the template must survive.
	require.NoError(t, CreateWatchlistRule(&AuditWatchlistRule{Kind: WatchlistKindKeyword, Pattern: "manual rule", Severity: "low", Enabled: true}))

	removed, err := DeleteTemplateRules("api-key-leak")
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)

	manual, err := ListWatchlistRules(nil, "")
	require.NoError(t, err)
	require.Len(t, manual, 1)
	assert.Equal(t, "manual", manual[0].Source)

	left, err := CountWatchlistRulesByTemplate("api-key-leak", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), left)
}

// CreateWatchlistRule 规范化 Source 空字符串 -> manual。
func TestCreateWatchlistRuleNormalizesSource(t *testing.T) {
	auditTemplateTestDB(t)

	r := &AuditWatchlistRule{Kind: WatchlistKindKeyword, Pattern: "p", Severity: "medium", Enabled: true}
	require.NoError(t, CreateWatchlistRule(r))
	got, err := GetWatchlistRule(r.Id)
	require.NoError(t, err)
	assert.Equal(t, "manual", got.Source)
}

func boolPtr(v bool) *bool { return &v }

// EVD-106 / INV-108: AutoMigrate 在三库兼容的 GORM 语句下为规则表建出
// source/template_id 两列（SQLite 实测；MySQL/PG 同为 ADD COLUMN 路径）。
func TestAuditWatchlistRuleSchemaColumns(t *testing.T) {
	auditTemplateTestDB(t)

	var cols []string
	require.NoError(t, DB.Raw("SELECT name FROM pragma_table_info('audit_watchlist_rules')").Scan(&cols).Error)
	joined := strings.Join(cols, ",")
	assert.Contains(t, joined, "source")
	assert.Contains(t, joined, "template_id")

	// v1 历史规则 source 为空 → 按 manual 解释（不迁移旧数据）。
	r := AuditWatchlistRule{Kind: WatchlistKindKeyword, Pattern: "legacy", Severity: "low", Enabled: true}
	require.NoError(t, CreateWatchlistRule(&r))
	assert.Equal(t, "manual", r.Source)
}
