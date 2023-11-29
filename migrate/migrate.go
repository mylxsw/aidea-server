package migrate

import (
	"context"
	"database/sql"
	"github.com/mylxsw/aidea-server/migrate/data"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/eloquent/migrate"
	"time"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	log.Debugf("正在执行数据库迁移")
	startTs := time.Now()
	defer func() {
		log.Debugf("数据库迁移执行完成，耗时 %s", time.Since(startTs).String())
	}()

	m := migrate.NewManager(db).Init(ctx)

	data.Migrate20231129DDL(m)
	data.Migrate20231129DML(m)

	return m.Run(ctx)
}
