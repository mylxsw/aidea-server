package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20240221DDL(m *migrate.Manager) {
	m.Schema("20240221-ddl").Table("chat_messages", func(builder *migrate.Builder) {
		builder.String("model", 64).Nullable(true).Comment("聊天模型").Change()
	})
}
