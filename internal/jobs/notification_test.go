package jobs_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/internal/jobs"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
	"github.com/redis/go-redis/v9"
)

func TestUserSignupNotificationJob(t *testing.T) {
	db := must.Must(sql.Open("mysql", os.Getenv("AISERVER_DB_URI")))
	defer db.Close()

	rds := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("AISERVER_REDIS_URI"),
		Password: os.Getenv("AISERVER_REDIS_PASSWORD"),
	})

	assert.NoError(t, jobs.UserSignupNotificationJob(context.TODO(), db, rds, nil))
}
