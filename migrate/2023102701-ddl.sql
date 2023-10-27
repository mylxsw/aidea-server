
ALTER TABLE chat_messages
    ADD status INT UNSIGNED DEFAULT 1 COMMENT '消息状态：1-成功 2-失败';