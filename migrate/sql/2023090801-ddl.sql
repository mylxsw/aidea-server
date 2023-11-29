-- 用户智慧果配额表
CREATE TABLE quota
(
    id              BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id         INT             NOT NULL,
    quota           INT             NOT NULL,
    rest            INT             NOT NULL,
    period_start_at TIMESTAMP       NULL     DEFAULT NULL,
    period_end_at   TIMESTAMP       NULL     DEFAULT NULL,
    note            VARCHAR(255)    NULL,
    payment_id      VARCHAR(50)     NULL,
    created_at      TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 用户智慧果债务表
CREATE TABLE debt
(
    id         INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id    INT             NOT NULL,
    used       INT             NOT NULL,
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 用户智慧果使用情况表
CREATE TABLE quota_usage
(
    id         BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id    INT             NOT NULL,
    used       INT             NULL,
    quota_ids  VARCHAR(255)    NULL,
    debt       INT             NULL,
    meta       TEXT            NULL,
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 用户智慧果用量统计表
CREATE TABLE quota_statistics
(
    id         INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id    INT             NOT NULL,
    used       INT             NULL COMMENT '当日总用量',
    cal_date   DATE            NULL COMMENT '统计日期',
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 缓存表
CREATE TABLE cache
(
    id          INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    `key`       VARCHAR(255)    NOT NULL,
    `value`     TEXT            NOT NULL,
    valid_until TIMESTAMP       NULL     DEFAULT NULL,
    created_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 任务队列任务（用于任务执行时查询，实际的任务队列存储在 Redis）
CREATE TABLE queue_tasks
(
    id         BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    title      VARCHAR(255)    NOT NULL,
    uid        INT             NOT NULL,
    task_id    VARCHAR(255)    NOT NULL,
    task_type  VARCHAR(255)    NOT NULL,
    queue_name VARCHAR(255)    NOT NULL,
    payload    TEXT            COLLATE utf8mb4_general_ci NOT NULL,
    result     TEXT            COLLATE utf8mb4_general_ci NULL,
    status     VARCHAR(20)     NOT NULL,
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) COLLATE = utf8mb4_general_ci;

-- 任务队列生成的待处理任务（类似于子任务）
CREATE TABLE queue_tasks_pending
(
    id              BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    payload         TEXT            NULL COMMENT '任务负载',
    next_execute_at TIMESTAMP       NULL COMMENT '下次执行时间',
    deadline_at     TIMESTAMP       NULL COMMENT '执行截止时间',
    execute_times   INT             NOT NULL DEFAULT 0 COMMENT '当前执行次数',
    task_id         VARCHAR(255)    NOT NULL,
    task_type       VARCHAR(255)    NOT NULL,
    status          TINYINT         NOT NULL COMMENT '1-处理中 2-执行成功 3-执行失败',
    created_at      TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 用户
CREATE TABLE users
(
    id                   INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    email                VARCHAR(40)     NULL,
    phone                VARCHAR(20)     NULL,
    password             VARCHAR(255)    NULL,
    realname             VARCHAR(255)    NULL,
    avatar               VARCHAR(255)    NULL,
    apple_uid            VARCHAR(255)    NULL,
    status               VARCHAR(20)     NULL,
    invited_by           INT             NULL,
    invite_code          VARCHAR(20)     COLLATE utf8mb4_bin NULL,
    prefer_signin_method VARCHAR(10)     NULL COMMENT '优先选择的登录方式：sms_code, password',
    user_type            TINYINT         NULL     DEFAULT 0 COMMENT '用户类型：0-普通用户 1-内部用户',
    created_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 系统事件
CREATE TABLE events
(
    id         BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    event_type VARCHAR(50)     NOT NULL,
    payload    TEXT            NULL,
    status     VARCHAR(20)     NULL,
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 支付历史纪录
CREATE TABLE payment_history
(
    id          INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id     INT             NOT NULL,
    payment_id  VARCHAR(50)     NOT NULL,
    source      VARCHAR(15)     NULL,
    source_id   INT             NULL,
    quantity    INT             NULL,
    valid_until TIMESTAMP       NULL,
    status      TINYINT         NULL,
    environment VARCHAR(10)     NULL,
    purchase_at TIMESTAMP       NULL,
    created_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Apple 应用内支付记录
CREATE TABLE apple_pay_history
(
    id                 INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id            INT             NOT NULL,
    payment_id         VARCHAR(50)     NOT NULL,
    purchase_id        VARCHAR(30)     NULL,
    transaction_id     VARCHAR(30)     NULL,
    product_id         VARCHAR(30)     NULL,
    source             VARCHAR(15)     NULL,
    status             TINYINT         NULL,
    server_verify_data TEXT            NULL,
    environment        VARCHAR(10)     NULL,
    purchase_at        TIMESTAMP       NULL,
    note               TEXT            NULL,
    created_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

--  支付宝支付历史纪录
CREATE TABLE alipay_history
(
    id                 INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id            INT             NOT NULL,
    payment_id         VARCHAR(50)     NOT NULL,
    product_id         VARCHAR(30)     NULL,
    buyer_id           VARCHAR(32)     NULL,
    invoice_amount     INT UNSIGNED    NULL,
    receipt_amount     INT UNSIGNED    NULL,
    buyer_pay_amount   INT UNSIGNED    NULL,
    total_amount       INT UNSIGNED    NULL,
    point_amount       INT UNSIGNED    NULL,
    trade_no           VARCHAR(64)     NULL,
    buyer_logon_id     VARCHAR(50)     NULL,
    status             TINYINT         NULL,
    purchase_at        TIMESTAMP       NULL,
    note               VARCHAR(255)    NULL,
    created_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 用户数字人列表
CREATE TABLE rooms
(
    id               INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id          INT             NOT NULL,
    avatar_id        INT             NULL COMMENT '头像 ID',
    avatar_url       VARCHAR(255)    NULL COMMENT '头像地址',
    name             VARCHAR(255)    NULL COMMENT '房间名称',
    description      VARCHAR(255)    NULL COMMENT '房间描述',
    priority         INT             NULL COMMENT '房间优先级',
    model            VARCHAR(50)     NULL COMMENT '房间模型',
    vendor           VARCHAR(50)     NULL COMMENT '房间模型服务商',
    system_prompt    TEXT            NULL COMMENT '系统提示语',
    max_context      TINYINT         NULL COMMENT '最大上下文数',
    room_type        TINYINT         NULL COMMENT '房间类型：1-系统预设 2-自定义',
    init_message     TEXT            NULL COMMENT '初始化消息',
    last_active_time TIMESTAMP       NULL COMMENT '最后活跃时间',
    created_at       TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) AUTO_INCREMENT = 1000;

-- 用户聊天消息存储（用于后续扩展以实现历史记录的同步）
CREATE TABLE chat_messages (
    id               BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id          INT             NOT NULL,
    room_id          INT             NULL,
    message          TEXT            COLLATE utf8mb4_general_ci NULL COMMENT '消息内容',
    `role`           TINYINT         NULL COMMENT '角色：1-用户 2-机器人',
    token_consumed   INT             NULL COMMENT '消耗的 Token',
    quota_consumed   INT             NULL COMMENT '消耗的配额',
    created_at       TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP 
);

-- 系统内置的数字人列表
CREATE TABLE room_gallery
(
    id           INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id      INT             NULL,
    avatar_id    INT             NULL COMMENT '头像 ID',
    avatar_url   VARCHAR(255)    NULL COMMENT '头像地址',
    name         VARCHAR(255)    NULL COMMENT '房间名称',
    model        VARCHAR(50)     NULL COMMENT '模型',
    vendor       VARCHAR(50)     NULL COMMENT '模型服务商',
    prompt       TEXT            NULL COMMENT '系统提示语',
    description  TEXT            NULL COMMENT '房间描述',
    max_context  TINYINT         NULL COMMENT '最大上下文数',
    init_message TEXT            NULL COMMENT '初始化消息',
    tags         JSON            NULL COMMENT'分类标签',
    version_min  VARCHAR(10)     NULL COMMENT '最小版本',
    version_max  VARCHAR(10)     NULL COMMENT '最大版本',
    room_type    VARCHAR(20)     NULL DEFAULT 'default' COMMENT '房间类型：default/system',
    created_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 创作岛项目列表（已废弃，对应 App 版本 1.0.1）
CREATE TABLE creative_island
(
    id                 BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    island_id          VARCHAR(100)    NOT NULL COMMENT '创作岛 ID',
    title              VARCHAR(255)    NOT NULL COMMENT '创作岛标题',
    title_color        VARCHAR(10)     NULL COMMENT '标题颜色',
    description        VARCHAR(255)    NULL COMMENT '创作岛描述',
    category           VARCHAR(50)     NULL COMMENT '创作岛分类',
    model_type         VARCHAR(20)     NOT NULL COMMENT '模型类型',
    model              VARCHAR(50)     NOT NULL COMMENT '模型',
    vendor             VARCHAR(50)     NULL COMMENT '模型服务商',
    style_preset       VARCHAR(50)     NULL COMMENT '风格预设',
    word_count         INT             NULL COMMENT '输入上下文最大字数限制',
    hint               VARCHAR(255)    NULL COMMENT '提示语输入提示信息',
    prompt             TEXT            NULL COMMENT '文本模型提示语',
    bg_image           VARCHAR(255)    NULL COMMENT '背景图',
    bg_embedded_image  VARCHAR(255)    NULL COMMENT '背景图嵌入图',
    label              VARCHAR(255)    NULL COMMENT '提示语输入框标题',
    label_color        VARCHAR(10)     NULL COMMENT '提示语输入框标题颜色',
    submit_btn_text    VARCHAR(50)     NULL COMMENT '提交按钮文案',
    prompt_input_title VARCHAR(50)     NULL COMMENT '提示语输入框标题',
    wait_seconds       INT             NULL COMMENT '估计等待时间',
    show_image_style_selector TINYINT  NULL DEFAULT 0  COMMENT '是否显示风格选择器 0-否 1-是',
    no_prompt          TINYINT         NULL DEFAULT 0 COMMENT '是否不需要输入提示语',
    priority           INT             NULL COMMENT '优显示先级，值越大，越靠前',
    status             TINYINT         NOT NULL DEFAULT 1 COMMENT '状态：0-禁用，1-启用',
    ext                JSON            NULL COMMENT '扩展信息',
    version_min        VARCHAR(10) NULL COMMENT '最小版本',
    version_max        VARCHAR(10) NULL COMMENT '最大版本',
    created_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 创作岛创作历史纪录
CREATE TABLE creative_history
(
    id           BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id      INT             NOT NULL,
    island_id    VARCHAR(100)    NOT NULL COMMENT '创作岛 ID',
    island_type  TINYINT         NOT NULL COMMENT '类型：1-文本生成，2-图片生成，3-视频生成，4-音频生成',
    island_model VARCHAR(50)     NULL COMMENT '模型',
    arguments    TEXT            NULL COMMENT '命令参数，JSON 格式',
    prompt       TEXT            NULL COMMENT '提示语',
    answer       TEXT            NULL COMMENT '结果',
    task_id      VARCHAR(255)    NULL COMMENT '异步任务 ID',
    shared       TINYINT         DEFAULT 0 NOT NULL COMMENT '是否分享，0-否 1-是',
    quota_used   INT             NULL COMMENT '消耗配额',
    `status`     TINYINT         NOT NULL COMMENT '0-pending, 1-processing, 2-success, 3-failed',
    created_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 创作岛公开分享记录（用于绘玩图片展示）
CREATE TABLE creative_gallery
(
    id                   BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id              INT             NULL COMMENT '关联的用户 ID',
    username             VARCHAR(100)    NULL COMMENT '用户名',
    creative_history_id  BIGINT UNSIGNED NULL COMMENT '关联的创作历史 ID',
    creative_type        TINYINT         NOT NULL COMMENT '类型：1-文本生成，2-图片生成，3-视频生成，4-音频生成',
    meta                 JSON            NULL COMMENT '元信息',
    prompt               TEXT            NULL COMMENT '提示语',
    negative_prompt      TEXT            NULL COMMENT '反向提示语',
    answer               TEXT            NULL COMMENT '结果',
    tags                 JSON            NULL COMMENT '标签',
    ref_count            BIGINT UNSIGNED NULL COMMENT '引用次数',
    star_level           TINYINT         NULL COMMENT '星级',
    hot_value            BIGINT          DEFAULT 0 NULL COMMENT '热度值',
    `status`             TINYINT         NOT NULL COMMENT '0-待审核 1-审核通过 2-审核不通过',
    created_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 绘玩图片随机展示记录
CREATE TABLE creative_gallery_random (
    id                   BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    gallery_id           BIGINT UNSIGNED              NOT NULL
);

-- 支持的图像生成模型
CREATE TABLE image_model
(
    id                   BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    model_id             VARCHAR(100) NOT NULL COMMENT '统一模型 ID',
    model_name           VARCHAR(100) NOT NULL COMMENT '统一模型名称',
    vendor               VARCHAR(100) NOT NULL COMMENT '模型服务商',
    real_model           VARCHAR(100) NOT NULL COMMENT '真实模型 ID',
    meta                 JSON         NULL COMMENT '模型元信息',
    preview_image        VARCHAR(255) NULL COMMENT '模型预览图地址',
    description          TEXT         NULL COMMENT '模型描述',
    status               TINYINT      NOT NULL DEFAULT 1 COMMENT '状态：0-禁用，1-启用',
    star_level           INT          NULL COMMENT '星级',
    created_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 支持的图片风格
CREATE TABLE image_filter
(
    id                   BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    name                 VARCHAR(255) NOT NULL COMMENT '滤镜名称',
    model_id             VARCHAR(100) NOT NULL COMMENT '统一模型 ID',
    meta                 JSON         NULL COMMENT '模型元信息',
    preview_image        VARCHAR(255) NULL COMMENT '模型预览图地址',
    description          TEXT         NULL COMMENT '模型描述',
    status               TINYINT      NOT NULL DEFAULT 1 COMMENT '状态：0-禁用，1-启用',
    created_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 图像生成提示语标签库
CREATE TABLE prompt_tags
(
    id                   BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    tag_name             VARCHAR(255) NOT NULL COMMENT '标签名称',
    tag_value            VARCHAR(255) NULL COMMENT '标签值',
    description          VARCHAR(255) NULL COMMENT '标签描述',
    category             VARCHAR(255) NULL COMMENT '标签分类',
    category_sub         VARCHAR(255) NULL COMMENT '标签子分类',
    tag_type             TINYINT NOT NULL DEFAULT 1 COMMENT '标签类型：0-通用 1-提示语 2-反向提示语',
    meta                 JSON         NULL COMMENT '标签元信息',
    status               TINYINT      NOT NULL DEFAULT 1 COMMENT '状态：0-禁用，1-启用',
    created_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 聊天模型系统提示语示例
CREATE TABLE chat_sys_prompt_example
(
    id                   BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    title                VARCHAR(255) NOT NULL COMMENT '标题',
    content              TEXT NOT NULL COMMENT '内容',
    created_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) COLLATE utf8mb4_general_ci;

-- 通用的提示语示例
CREATE TABLE prompt_example
(
    id                   BIGINT UNSIGNED PRIMARY KEY NOT NULL AUTO_INCREMENT,
    title                VARCHAR(255) NOT NULL COMMENT '标题',
    content              TEXT NOT NULL COMMENT '内容',
    models               JSON NULL COMMENT '支持的模型，字符串数组格式',
    tags                 JSON NULL COMMENT '支持的标签，字符串数组格式',
    created_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) COLLATE utf8mb4_general_ci;

CREATE UNIQUE INDEX creative_island_id ON creative_island (island_id);

CREATE INDEX creative_history_user_task_id ON creative_history (user_id, task_id);
CREATE INDEX creative_history_user_island_id ON creative_history (user_id, island_id);

CREATE INDEX rooms_user_id ON rooms (user_id);

CREATE INDEX quota_user_id ON quota (user_id, period_end_at);
CREATE INDEX quota_usage_user_id_created ON quota_usage (user_id, created_at);
CREATE INDEX quota_usage_created ON quota_usage (created_at);

CREATE INDEX cache_key ON cache (`key`);
CREATE INDEX cache_ttl ON cache (`valid_until`);

CREATE INDEX queue_tasks_task_id ON queue_tasks (task_id);
CREATE INDEX queue_tasks_task_type ON queue_tasks (uid, task_type);
CREATE INDEX idx_queue_tasks_pending_status_next_execute_at ON queue_tasks_pending (status, next_execute_at);

CREATE UNIQUE INDEX users_email ON users (email);
CREATE UNIQUE INDEX users_phone ON users (phone);
CREATE UNIQUE INDEX users_apple_uid ON users (apple_uid);
CREATE UNIQUE INDEX users_invite_code ON users (invite_code);

CREATE INDEX events_event_type ON events (status, event_type);

CREATE INDEX payment_payment_id ON payment_history (user_id, payment_id);
CREATE INDEX payment_status ON payment_history (user_id, status);

CREATE INDEX apple_pay_payment_id ON apple_pay_history (user_id, payment_id);
CREATE INDEX apple_pay_status ON apple_pay_history (user_id, status);
