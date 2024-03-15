package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20240315Mix(m *migrate.Manager) {
	m.Schema("20240315-ddl").Create("models", func(builder *migrate.Builder) {
		builder.Increments("id")
		builder.Timestamps(0)

		builder.String("model_id", 100).Nullable(false).Comment("模型ID")
		builder.String("name", 100).Nullable(false).Comment("模型名称")
		builder.String("short_name", 100).Nullable(true).Comment("模型简称")
		builder.String("description", 255).Nullable(true).Comment("模型描述")
		builder.String("avatar_url", 255).Nullable(true).Comment("模型头像")
		builder.TinyInteger("status", false, true).Nullable(false).Comment("模型状态：0-禁用，1-启用")
		builder.String("version_min", 20).Nullable(true).Comment("最低版本")
		builder.String("version_max", 20).Nullable(true).Comment("最高版本")
		builder.Json("meta_json").Nullable(true).Comment("模型元信息，JSON 格式")
		builder.Json("providers_json").Nullable(false).Comment("模型提供商，JSON 格式")
	})

	m.Schema("20240315-dml").Raw("models", func() []string {
		return []string{}
	})
}
