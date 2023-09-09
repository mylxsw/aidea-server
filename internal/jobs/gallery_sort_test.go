package jobs_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/mylxsw/aidea-server/internal/jobs"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
)

func TestGallerySortJob(t *testing.T) {
	db := must.Must(sql.Open("mysql", os.Getenv("AISERVER_DB_URI")))
	defer db.Close()

	assert.NoError(t, jobs.GallerySortJob(context.TODO(), db, nil))
}
