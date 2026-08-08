package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func logContentCleanupTestDB(t *testing.T) {
	t.Helper()
	prevDB, prevLogDB := model.DB, model.LOG_DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}, &model.LogContent{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = prevDB, prevLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
}

// BR-112: TTL=0 时 StartLogContentCleanupTask 为 no-op，不产生任务。
func TestStartLogContentCleanupTaskNoOpOnZeroTTL(t *testing.T) {
	logContentCleanupTestDB(t)

	task, err := StartLogContentCleanupTask(0)
	require.NoError(t, err)
	assert.Nil(t, task)

	var count int64
	require.NoError(t, model.DB.Model(&model.SystemTask{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

// TTL>0 enqueues a pending log_content_cleanup task with a valid cutoff.
func TestStartLogContentCleanupTaskCreates(t *testing.T) {
	logContentCleanupTestDB(t)

	task, err := StartLogContentCleanupTask(7)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, model.SystemTaskTypeLogContentCleanup, task.Type)

	// Enqueuing again while active returns the same task (dedup).
	again, err := StartLogContentCleanupTask(7)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, task.TaskID, again.TaskID)
}
