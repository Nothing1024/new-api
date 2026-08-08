package model

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func logContentTestDB(t *testing.T) {
	t.Helper()
	prevDB, prevLogDB := DB, LOG_DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&LogContent{}))
	t.Cleanup(func() {
		DB, LOG_DB = prevDB, prevLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
}

func seedLogContents(t *testing.T) {
	t.Helper()
	rows := []*LogContent{
		{RequestId: "r1", UserId: 1, ModelName: "gpt-4", CreatedAt: 1000, HitSeverity: "high", HitCount: 3},
		{RequestId: "r2", UserId: 1, ModelName: "gpt-4", CreatedAt: 2000, HitSeverity: "low", HitCount: 1},
		{RequestId: "r3", UserId: 2, ModelName: "claude", CreatedAt: 3000, HitSeverity: "high", HitCount: 5},
		{RequestId: "r4", UserId: 2, ModelName: "claude", CreatedAt: 9000, HitSeverity: "high", HitCount: 2},
	}
	for _, r := range rows {
		require.NoError(t, LOG_DB.Create(r).Error)
	}
}

func TestListLogContentsFilters(t *testing.T) {
	logContentTestDB(t)
	seedLogContents(t)

	// Severity filter.
	got, total, err := ListLogContents(ListLogContentsParams{Severity: "high"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, got, 3)

	// MinHit filter.
	got, total, err = ListLogContents(ListLogContentsParams{MinHit: 3})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	for _, r := range got {
		require.GreaterOrEqual(t, r.HitCount, 3)
	}

	// Time range (created_at between 1500 and 4000).
	got, total, err = ListLogContents(ListLogContentsParams{StartTime: 1500, EndTime: 4000})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// User + model combined.
	got, total, err = ListLogContents(ListLogContentsParams{UserId: 2, ModelName: "claude"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// Combined severity + user.
	got, total, err = ListLogContents(ListLogContentsParams{Severity: "high", UserId: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// Pagination: page 2, size 2 -> all 4 items across pages.
	got, total, err = ListLogContents(ListLogContentsParams{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	require.Len(t, got, 2)

	// Order newest-first on page 1.
	got, _, err = ListLogContents(ListLogContentsParams{Page: 1, PageSize: 4})
	require.NoError(t, err)
	assert.Equal(t, "r4", got[0].RequestId)
}

func TestDeleteOldLogContentBatchCutoff(t *testing.T) {
	logContentTestDB(t)
	seedLogContents(t)

	// Delete rows strictly older than cutoff=2500 -> r1(1000), r2(2000).
	deleted, err := DeleteOldLogContentBatch(context.Background(), 2500, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	var remaining int64
	require.NoError(t, LOG_DB.Model(&LogContent{}).Count(&remaining).Error)
	assert.Equal(t, int64(2), remaining)

	// A second run at a later cutoff removes the rest.
	deleted, err = DeleteOldLogContentBatch(context.Background(), 10000, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	require.NoError(t, LOG_DB.Model(&LogContent{}).Count(&remaining).Error)
	assert.Equal(t, int64(0), remaining)
}

func TestCountOldLogContent(t *testing.T) {
	logContentTestDB(t)
	seedLogContents(t)

	total, err := CountOldLogContent(context.Background(), 2500)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
}
