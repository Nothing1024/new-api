package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAuditImportTestDB(t *testing.T) {
	t.Helper()
	prevDB, prevLogDB := model.DB, model.LOG_DB
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.AuditWatchlistRule{}, &model.AuditWatchlistMeta{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = prevDB, prevLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
}

func doImportRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST("/api/audit/watchlist/import", ImportWatchlistRules)
	req := httptest.NewRequest(http.MethodPost, "/api/audit/watchlist/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// BR-117: 含非法 kind 的导入整批拒绝且不写入任何规则。
func TestImportWatchlistRulesRejectsInvalidEntry(t *testing.T) {
	setupAuditImportTestDB(t)

	bad := `{"rules":[
		{"kind":"keyword","pattern":"ok","severity":"high","enabled":true},
		{"kind":"bogus-kind","pattern":"x","severity":"high","enabled":true}
	]}`
	w := doImportRequest(t, bad)
	assert.Equal(t, http.StatusOK, w.Code) // ApiErrorMsg returns HTTP 200 with success:false

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))

	// Nothing written (ALL-or-nothing).
	var count int64
	require.NoError(t, model.DB.Model(&model.AuditWatchlistRule{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

// 超过条数上限整批拒绝。
func TestImportWatchlistRulesRejectsTooMany(t *testing.T) {
	setupAuditImportTestDB(t)

	items := make([]map[string]any, 0, 101)
	for i := 0; i < 101; i++ {
		items = append(items, map[string]any{
			"kind": "keyword", "pattern": "p", "severity": "low", "enabled": true,
		})
	}
	payload := map[string]any{"rules": items}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	w := doImportRequest(t, string(body))
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Contains(t, resp["message"].(string), "max 100")

	var count int64
	require.NoError(t, model.DB.Model(&model.AuditWatchlistRule{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

// 合法导入写入全部规则。
func TestImportWatchlistRulesValid(t *testing.T) {
	setupAuditImportTestDB(t)

	body := `{"rules":[
		{"kind":"keyword","pattern":"a","severity":"high","enabled":true},
		{"kind":"domain","pattern":"example.com","severity":"medium","enabled":false}
	]}`
	w := doImportRequest(t, body)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["success"].(bool))
	data := resp["data"].(map[string]any)
	assert.Equal(t, float64(2), data["imported"])

	var count int64
	require.NoError(t, model.DB.Model(&model.AuditWatchlistRule{}).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

// 空规则列表提示没有可导入的规则。
func TestImportWatchlistRulesEmpty(t *testing.T) {
	setupAuditImportTestDB(t)

	w := doImportRequest(t, `{"rules":[]}`)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp["success"].(bool))
	assert.Contains(t, resp["message"].(string), "no rules")
}
