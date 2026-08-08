package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// logTestDB swaps DB/LOG_DB for a fresh in-memory sqlite DB preloaded with the
// logs + log_contents tables and restores the previous globals afterward.
func logTestDB(t *testing.T) {
	t.Helper()
	prevDB, prevLogDB := DB, LOG_DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&Log{}, &LogContent{}))
	t.Cleanup(func() {
		DB, LOG_DB = prevDB, prevLogDB
		sqlDB, _ := db.DB()
		require.NoError(t, sqlDB.Close())
	})
}

// TestUpdateLogAuditFieldsDualWrite verifies BR-302: the helper writes both the
// legacy logs.other.admin_info.audit JSON pointer and the three new columns.
func TestUpdateLogAuditFieldsDualWrite(t *testing.T) {
	logTestDB(t)
	require.NoError(t, LOG_DB.Create(&Log{RequestId: "req-1", Other: "{}"}).Error)

	require.NoError(t, UpdateLogAuditFields("req-1", 3, "high", 7))

	var got Log
	require.NoError(t, LOG_DB.Where("request_id = ?", "req-1").First(&got).Error)
	assert.Equal(t, 3, got.AuditHitCount)
	assert.Equal(t, "high", got.AuditHitSeverity)
	assert.Equal(t, 7, got.AuditWLVersion)

	otherMap, err := common.StrToMap(got.Other)
	require.NoError(t, err)
	adminInfo, ok := otherMap["admin_info"].(map[string]interface{})
	require.True(t, ok, "admin_info must remain for backward compat (INV-302)")
	audit, ok := adminInfo["audit"].(map[string]interface{})
	require.True(t, ok, "admin_info.audit must remain for backward compat (INV-302)")
	assert.Equal(t, "req-1", audit["request_id"])
	assert.Equal(t, float64(3), audit["hit_count"])
}

// TestUpdateLogAuditFieldsZeroValues verifies the map-based Updates writes zero
// values (struct Updates would skip them), covering no-hit log rows.
func TestUpdateLogAuditFieldsZeroValues(t *testing.T) {
	logTestDB(t)
	require.NoError(t, LOG_DB.Create(&Log{RequestId: "req-2", Other: "{}"}).Error)

	require.NoError(t, UpdateLogAuditFields("req-2", 0, "", 0))

	var got Log
	require.NoError(t, LOG_DB.Where("request_id = ?", "req-2").First(&got).Error)
	assert.Zero(t, got.AuditHitCount)
	assert.Empty(t, got.AuditHitSeverity)
	assert.Zero(t, got.AuditWLVersion)
}

// TestGetAllLogsAuditSeverityFilter verifies BR-304: a non-empty
// auditHitSeverity filters rows, an empty value returns everything.
func TestGetAllLogsAuditSeverityFilter(t *testing.T) {
	logTestDB(t)
	for _, tc := range []struct {
		reqID    string
		severity string
		count    int
	}{
		{"r-high-1", "high", 2},
		{"r-high-2", "high", 1},
		{"r-low-1", "low", 1},
		{"r-none", "", 0},
	} {
		require.NoError(t, LOG_DB.Create(&Log{
			RequestId:       tc.reqID,
			Type:            LogTypeConsume,
			AuditHitSeverity: tc.severity,
			AuditHitCount:    tc.count,
		}).Error)
	}

	got, total, err := GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 0, 10, 0, "", "", "", "high")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, got, 2)
	for _, l := range got {
		assert.Equal(t, "high", l.AuditHitSeverity)
	}

	got, total, err = GetAllLogs(LogTypeUnknown, 0, 0, "", "", "", 0, 10, 0, "", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	require.Len(t, got, 4)
}

// TestGetUserLogsAuditSeverityFilter verifies BR-304 on the user-scoped query.
func TestGetUserLogsAuditSeverityFilter(t *testing.T) {
	logTestDB(t)
	for _, tc := range []struct {
		reqID    string
		userID   int
		severity string
	}{
		{"u-high-1", 10, "high"},
		{"u-high-2", 10, "high"},
		{"u-low-1", 10, "low"},
		{"other-user", 11, "high"},
	} {
		require.NoError(t, LOG_DB.Create(&Log{
			RequestId:        tc.reqID,
			UserId:           tc.userID,
			Type:             LogTypeConsume,
			AuditHitSeverity: tc.severity,
			AuditHitCount:    1,
		}).Error)
	}

	got, total, err := GetUserLogs(10, LogTypeUnknown, 0, 0, "", "", 0, 10, "", "", "", "high")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, got, 2)
	for _, l := range got {
		assert.Equal(t, 10, l.UserId)
		// SQL 过滤生效，但返回行经 formatUserLogs 剥离 audit 列（ASM-301，admin-only）。
		assert.Empty(t, l.AuditHitSeverity)
		assert.Zero(t, l.AuditHitCount)
	}
}

// TestFormatUserLogsStripsAuditColumns verifies ASM-301: audit columns are
// removed for non-admin log views, matching the admin_info JSON stripping.
func TestFormatUserLogsStripsAuditColumns(t *testing.T) {
	logs := []*Log{{
		Other:             `{"model_price":0.004,"admin_info":{"audit":{"request_id":"r1","hit_count":3}}}`,
		AuditHitSeverity:  "high",
		AuditHitCount:     3,
		AuditWLVersion:    5,
	}}
	formatUserLogs(logs, 0)

	assert.Empty(t, logs[0].AuditHitSeverity)
	assert.Zero(t, logs[0].AuditHitCount)
	assert.Zero(t, logs[0].AuditWLVersion)
	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	assert.False(t, hasAdminInfo, "admin_info (and nested audit pointer) must be stripped for non-admin views")
	assert.Contains(t, parsed, "model_price")
}
