package jobs_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mylxsw/aidea-server/internal/jobs"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
)

func TestQuotaUsageStatistics(t *testing.T) {
	db := must.Must(sql.Open("mysql", os.Getenv("AISERVER_DB_URI")))
	defer db.Close()

	for i := 0; i < 30; i++ {
		assert.NoError(t, jobs.QuotaUsageStatistics(context.TODO(), db, time.Now().AddDate(0, 0, -i-1)))
	}
}
