package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20240709DDL(m *migrate.Manager) {
	m.Schema("20240709-ddl").Create("chat_messages_share.yaml", func(builder *migrate.Builder) {
		builder.Increments("id")
		builder.Timestamps(0)
		builder.Integer("user_id", false, true).Comment("User ID")
		builder.Json("data").Nullable(true).Comment("Shared data")
		builder.String("code", 64).Unique().Comment("Share code")
	})
}
