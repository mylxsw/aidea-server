

CREATE TABLE chat_group_member
(
    id         INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id    INT             NOT NULL,
    group_id   INT             NOT NULL,
    model_id   VARCHAR(255)    NOT NULL,
    model_name VARCHAR(255)    COLLATE utf8mb4_general_ci NULL,
    `status`   INT             NOT NULL DEFAULT 1 COMMENT '状态：1-正常 2-已删除',
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE chat_group_message
(
    id               INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id          INT             NOT NULL,
    group_id         INT             NOT NULL,
    message          TEXT            COLLATE utf8mb4_general_ci NULL COMMENT '消息内容',
    `role`           TINYINT         NULL COMMENT '角色：1-用户 2-机器人',
    token_consumed   INT             NULL COMMENT '消耗的 Token',
    quota_consumed   INT             NULL COMMENT '消耗的配额',
    pid              INT             NULL COMMENT '父消息 ID',
    member_id        INT             NULL COMMENT '发送消息的成员 ID',
    `status`         INT             NOT NULL DEFAULT 1 COMMENT '状态：0-待处理 1-成功 2-失败',
    created_at       TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE INDEX chat_group_member_group_user_idx ON chat_group_member (group_id, user_id);

CREATE INDEX chat_group_message_group_user_idx ON chat_group_message (group_id, user_id);