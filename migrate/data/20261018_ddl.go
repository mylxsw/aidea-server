package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20261018DDL(m *migrate.Manager) {
	m.Schema("20261018-ddl").Raw("external_pay_clients", func() []string {
		return []string{`
CREATE TABLE IF NOT EXISTS external_pay_clients (
  client_id VARCHAR(64) PRIMARY KEY,
  public_key_pem TEXT NOT NULL,
  enabled TINYINT NOT NULL DEFAULT 1,
  allowed_channels VARCHAR(100) NOT NULL DEFAULT 'alipay_page,wechat_native',
  min_amount_fen BIGINT NOT NULL DEFAULT 1,
  max_amount_fen BIGINT NOT NULL DEFAULT 10000000,
  max_daily_orders INT NOT NULL DEFAULT 1000,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`}
	})
	m.Schema("20261018-ddl").Raw("external_pay_orders", func() []string {
		return []string{`
CREATE TABLE IF NOT EXISTS external_pay_orders (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  payment_id VARCHAR(32) NOT NULL UNIQUE,
  client_id VARCHAR(64) NOT NULL,
  merchant_order_id VARCHAR(100) NOT NULL,
  channel VARCHAR(32) NOT NULL,
  amount_fen BIGINT NOT NULL,
  currency CHAR(3) NOT NULL DEFAULT 'CNY',
  subject VARCHAR(127) NOT NULL,
  callback_url VARCHAR(2048) NOT NULL,
  status VARCHAR(24) NOT NULL,
  pay_url TEXT NULL,
  provider_trade_id VARCHAR(64) NULL,
  provider_payload JSON NULL,
  failure_reason VARCHAR(255) NULL,
  paid_at DATETIME(6) NULL,
  checked_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_external_merchant_order (client_id, merchant_order_id),
  UNIQUE KEY uq_external_provider_trade (channel, provider_trade_id),
  KEY idx_external_client_updates (client_id, updated_at, id),
  KEY idx_external_client_created (client_id, created_at),
  KEY idx_external_pending (status, checked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`}
	})
	m.Schema("20261018-ddl").Raw("external_pay_events", func() []string {
		return []string{`
CREATE TABLE IF NOT EXISTS external_pay_events (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  event_id VARCHAR(40) NOT NULL UNIQUE,
  payment_id VARCHAR(32) NOT NULL,
  event_type VARCHAR(32) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  locked_until DATETIME(6) NULL,
  last_error VARCHAR(500) NULL,
  delivered_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_external_payment_event (payment_id, event_type),
  KEY idx_external_event_due (status, next_attempt_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`}
	})
	m.Schema("20261018-ddl").Raw("external_pay_nonces", func() []string {
		return []string{`
CREATE TABLE IF NOT EXISTS external_pay_nonces (
  client_id VARCHAR(64) NOT NULL,
  nonce VARCHAR(80) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  PRIMARY KEY (client_id, nonce),
  KEY idx_external_nonce_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`}
	})
}
