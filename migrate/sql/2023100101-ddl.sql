
ALTER TABLE chat_messages
    ADD pid INT UNSIGNED NULL COMMENT '父记录 ID（问题 ID）',
    ADD model VARCHAR(32) NULL COMMENT '聊天模型';

CREATE INDEX chat_messages_user_pid_idx ON chat_messages (user_id, pid);

