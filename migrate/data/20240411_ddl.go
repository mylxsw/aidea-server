package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20240411DDL(m *migrate.Manager) {
	m.Schema("20240411-ddl").Create("wechat_pay_history", func(builder *migrate.Builder) {
		builder.Increments("id")
		builder.Timestamps(0)
		builder.Integer("user_id", false, true).Comment("用户ID")
		builder.String("payment_id", 50).Unique().Nullable(false).Comment("支付ID")
		builder.String("product_id", 30).Nullable(false).Comment("产品ID")
		builder.Integer("amount", false, true).Nullable(true).Comment("支付金额")

		builder.Json("extra").Nullable(true).Comment("额外信息")

		builder.String("environment", 10).Nullable(true).Comment("支付环境")
		builder.Integer("status", false, true).Nullable(true).Comment("支付状态")
		builder.Timestamp("purchase_at", 0).Nullable(true).Comment("支付时间")
		builder.String("note", 255).Nullable(true).Comment("备注")

		builder.Index("idx_user_id", "user_id")
	})
}
