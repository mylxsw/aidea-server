
ALTER TABLE chat_messages
    ADD status INT UNSIGNED DEFAULT 1 COMMENT '消息状态：1-成功 2-失败';
ALTER TABLE chat_messages
    ADD error TEXT NULL COMMENT '错误详情';