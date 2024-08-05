package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20240805DDL(m *migrate.Manager) {
	m.Schema("20240805-ddl").Table("chat_messages", func(builder *migrate.Builder) {
		builder.Json("meta").Nullable(true).Comment("Meta data")
	})
}
