package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20240131DDL(m *migrate.Manager) {
	m.Schema("20240131-ddl").Table("users", func(builder *migrate.Builder) {
		builder.String("union_id", 128).Nullable(true).Unique().Comment("微信登录 unionid")
	})
}
