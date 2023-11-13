
CREATE TABLE user_api_key
(
    id           INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id      INT             NOT NULL,
    name         VARCHAR(255)    NULL,
    token        VARCHAR(255)    NOT NULL,
    status       TINYINT         NOT NULL DEFAULT 1 COMMENT '状态：1-正常 2-禁用',
    valid_before TIMESTAMP       NULL COMMENT '有效期',
    created_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_api_token ON user_api_key (token);
CREATE INDEX idx_api_user ON user_api_key (user_id);
