package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20231129DDL(m *migrate.Manager) {
	m.Schema("20231129-ddl").Raw("alipay_history", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS alipay_history
(
    id               INT AUTO_INCREMENT                  PRIMARY KEY,
    user_id          INT                                 NOT NULL,
    payment_id       VARCHAR(50)                         NOT NULL,
    product_id       VARCHAR(30)                         NULL,
    buyer_id         VARCHAR(32)                         NULL,
    invoice_amount   INT UNSIGNED                        NULL,
    receipt_amount   INT UNSIGNED                        NULL,
    buyer_pay_amount INT UNSIGNED                        NULL,
    total_amount     INT UNSIGNED                        NULL,
    point_amount     INT UNSIGNED                        NULL,
    trade_no         VARCHAR(64)                         NULL,
    buyer_logon_id   VARCHAR(50)                         NULL,
    status           TINYINT                             NULL,
    purchase_at      TIMESTAMP                           NULL,
    note             VARCHAR(255)                        NULL,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("apple_pay_history", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS apple_pay_history
(
    id                 INT AUTO_INCREMENT                  PRIMARY KEY,
    user_id            INT                                 NOT NULL,
    payment_id         VARCHAR(50)                         NOT NULL,
    purchase_id        VARCHAR(30)                         NULL,
    transaction_id     VARCHAR(30)                         NULL,
    product_id         VARCHAR(30)                         NULL,
    source             VARCHAR(15)                         NULL,
    status             TINYINT                             NULL,
    environment        VARCHAR(10)                         NULL,
    purchase_at        TIMESTAMP                           NULL,
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    server_verify_data TEXT                                NULL,
    note               TEXT                                NULL
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX apple_pay_payment_id ON apple_pay_history (user_id, payment_id)`,
			`CREATE INDEX apple_pay_status ON apple_pay_history (user_id, status);`,
		}
	})

	m.Schema("20231129-ddl").Raw("cache", func() []string {
		return []string{
			"CREATE TABLE IF NOT EXISTS cache\n(\n    id INT AUTO_INCREMENT PRIMARY KEY,\n    `key`       VARCHAR(255)                        NOT NULL,\n    value       TEXT                                NOT NULL,\n    valid_until TIMESTAMP                           NULL,\n    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,\n    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP\n)\n    CHARSET = utf8mb4",
			"CREATE INDEX cache_key ON cache (`key`)",
			"CREATE INDEX cache_ttl ON cache (valid_until)",
		}
	})

	m.Schema("20231129-ddl").Raw("chat_group_member", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS chat_group_member
(
    id         INT AUTO_INCREMENT                      PRIMARY KEY,
    user_id    INT                                     NOT NULL,
    group_id   INT                                     NOT NULL,
    model_id   VARCHAR(255)                            NOT NULL,
    model_name VARCHAR(255) COLLATE utf8mb4_general_ci NULL,
    status     INT       DEFAULT 1                     NOT NULL COMMENT '状态：1-正常 2-已删除',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP     NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP     NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX chat_group_member_group_user_idx ON chat_group_member (group_id, user_id)`,
		}
	})

	m.Schema("20231129-ddl").Raw("chat_group_message", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS chat_group_message
(
    id             INT AUTO_INCREMENT                  PRIMARY KEY,
    user_id        INT                                 NOT NULL,
    group_id       INT                                 NOT NULL,
    message        TEXT COLLATE utf8mb4_general_ci     NULL COMMENT '消息内容',
    role           TINYINT                             NULL COMMENT '角色：1-用户 2-机器人',
    token_consumed INT                                 NULL COMMENT '消耗的 Token',
    quota_consumed INT                                 NULL COMMENT '消耗的配额',
    pid            INT                                 NULL COMMENT '父消息 ID',
    member_id      INT                                 NULL COMMENT '发送消息的成员 ID',
    status         INT       DEFAULT 1                 NOT NULL COMMENT '状态：0-待处理 1-成功 2-失败',
    created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    error          TEXT                                NULL
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX chat_group_message_group_user_idx ON chat_group_message (group_id, user_id)`,
		}
	})

	m.Schema("20231129-ddl").Raw("chat_messages", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS chat_messages
(
    id             INT AUTO_INCREMENT                     PRIMARY KEY,
    user_id        INT                                    NOT NULL,
    room_id        INT                                    NULL,
    message        TEXT COLLATE utf8mb4_general_ci        NULL COMMENT '消息内容',
    role           TINYINT                                NULL COMMENT '角色：1-用户 2-机器人',
    token_consumed INT                                    NULL COMMENT '消耗的 Token',
    quota_consumed INT                                    NULL COMMENT '消耗的配额',
    created_at     TIMESTAMP    DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at     TIMESTAMP    DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    pid            INT UNSIGNED                           NULL COMMENT '父记录 ID（问题 ID）',
    model          VARCHAR(32)                            NULL COMMENT '聊天模型',
    status         INT UNSIGNED DEFAULT '1'               NULL COMMENT '消息状态：1-成功 2-失败',
    error          TEXT                                   NULL COMMENT '错误详情'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX chat_messages_user_pid_idx ON chat_messages (user_id, pid)`,
		}
	})

	m.Schema("20231129-ddl").Raw("chat_sys_prompt_example", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS chat_sys_prompt_example
(
    id         BIGINT UNSIGNED AUTO_INCREMENT      PRIMARY KEY,
    title      VARCHAR(255)                        NOT NULL COMMENT '标题',
    content    TEXT                                NOT NULL COMMENT '内容',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("creative_gallery", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS creative_gallery
(
    id                  BIGINT UNSIGNED AUTO_INCREMENT      PRIMARY KEY,
    user_id             INT                                 NULL COMMENT '关联的用户 ID',
    username            VARCHAR(100)                        NULL COMMENT '用户名',
    hot_value           BIGINT    DEFAULT 0                 NULL COMMENT '热度值',
    creative_history_id BIGINT UNSIGNED                     NULL COMMENT '关联的创作历史 ID',
    creative_type       TINYINT                             NOT NULL COMMENT '类型：1-文本生成，2-图片生成，3-视频生成，4-音频生成',
    meta                JSON                                NULL COMMENT '元信息',
    prompt              TEXT                                NULL COMMENT '提示语',
    negative_prompt     TEXT                                NULL COMMENT '反向提示语',
    answer              TEXT                                NULL COMMENT '结果',
    tags                JSON                                NULL COMMENT '标签',
    ref_count           BIGINT UNSIGNED                     NULL COMMENT '引用次数',
    star_level          TINYINT                             NULL COMMENT '星级',
    status              TINYINT                             NOT NULL COMMENT '0-待审核 1-审核通过 2-审核不通过',
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX idx_hot_value ON creative_gallery (hot_value DESC)`,
		}
	})

	m.Schema("20231129-ddl").Raw("creative_gallery_random", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS creative_gallery_random
(
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    gallery_id BIGINT UNSIGNED NOT NULL
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("creative_history", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS creative_history
(
    id           BIGINT UNSIGNED AUTO_INCREMENT      PRIMARY KEY,
    user_id      INT                                 NOT NULL,
    island_id    VARCHAR(100)                        NOT NULL COMMENT '创意岛 ID',
    island_type  TINYINT                             NOT NULL COMMENT '类型：1-文本生成，2-图片生成，3-视频生成，4-音频生成',
    island_model VARCHAR(50)                         NULL COMMENT '模型',
    arguments    TEXT                                NULL COMMENT '命令参数，JSON 格式',
    prompt       TEXT                                NULL COMMENT '提示语',
    answer       TEXT                                NULL COMMENT '结果',
    task_id      VARCHAR(255)                        NULL COMMENT '异步任务 ID',
    status       TINYINT                             NOT NULL COMMENT '1-pending, 2-processing, 3-success, 4-failed',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    quota_used   INT                                 NULL COMMENT '消耗配额',
    shared       TINYINT   DEFAULT 0                 NOT NULL COMMENT '是否分享，0-否 1-是'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX creative_history_user_island_id ON creative_history (user_id, island_id)`,
			`CREATE INDEX creative_history_user_task_id ON creative_history (user_id, task_id)`,
		}
	})

	m.Schema("20231129-ddl").Raw("creative_island", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS creative_island
(
    id                        INT AUTO_INCREMENT                  PRIMARY KEY,
    island_id                 VARCHAR(100)                        NOT NULL COMMENT '创作岛 ID',
    title                     VARCHAR(255)                        NOT NULL COMMENT '创作岛标题',
    title_color               VARCHAR(10)                         NULL COMMENT '标题颜色',
    description               VARCHAR(255)                        NULL COMMENT '创作岛描述',
    category                  VARCHAR(50)                         NULL COMMENT '创作岛分类',
    model_type                VARCHAR(20)                         NOT NULL COMMENT '模型类型',
    model                     VARCHAR(50)                         NOT NULL COMMENT '模型',
    vendor                    VARCHAR(50)                         NULL COMMENT '模型服务商',
    word_count                INT                                 NULL COMMENT '输入上下文最大字数限制',
    hint                      VARCHAR(255)                        NULL COMMENT '提示语输入提示信息',
    prompt                    TEXT                                NULL COMMENT '文本模型提示语',
    bg_image                  VARCHAR(255)                        NULL COMMENT '背景图',
    label                     VARCHAR(255)                        NULL COMMENT '提示语输入框标题',
    label_color               VARCHAR(10)                         NULL COMMENT '提示语输入框标题颜色',
    priority                  INT                                 NULL COMMENT '优显示先级，值越大，越靠前',
    created_at                TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at                TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    style_preset              VARCHAR(50)                         NULL COMMENT '风格预设',
    submit_btn_text           VARCHAR(50)                         NULL COMMENT '提交按钮文案',
    status                    TINYINT   DEFAULT 1                 NOT NULL COMMENT '状态：0-禁用，1-启用',
    prompt_input_title        VARCHAR(50)                         NULL COMMENT '提示语输入框标题',
    wait_seconds              INT                                 NULL COMMENT '估计等待时间',
    show_image_style_selector TINYINT   DEFAULT 0                 NULL COMMENT '是否显示风格选择器 0-否 1-是',
    no_prompt                 TINYINT   DEFAULT 0                 NULL COMMENT '是否不需要输入提示语',
    version_min               VARCHAR(10)                         NULL COMMENT '最小版本',
    version_max               VARCHAR(10)                         NULL COMMENT '最大版本',
    ext                       JSON                                NULL COMMENT '扩展信息',
    bg_embedded_image         VARCHAR(255)                        NULL COMMENT '背景图嵌入图',
    CONSTRAINT creative_island_id UNIQUE (island_id)
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("debt", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS debt
(
    id         INT AUTO_INCREMENT PRIMARY KEY,
    user_id    INT                                 NOT NULL,
    used       INT                                 NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("events", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS events
(
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_type VARCHAR(50)                         NOT NULL,
    payload    TEXT                                NULL,
    status     VARCHAR(20)                         NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX events_event_type ON events (status, event_type)`,
		}
	})

	m.Schema("20231129-ddl").Raw("image_filter", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS image_filter
(
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name          VARCHAR(255)                        NOT NULL COMMENT '滤镜名称',
    model_id      VARCHAR(100)                        NOT NULL COMMENT '统一模型 ID',
    meta          JSON                                NULL COMMENT '模型元信息',
    preview_image VARCHAR(255)                        NULL COMMENT '模型预览图地址',
    description   TEXT                                NULL COMMENT '模型描述',
    status        TINYINT   DEFAULT 1                 NOT NULL COMMENT '状态：0-禁用，1-启用',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    star          INT                                 NULL COMMENT '星级'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("image_model", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS image_model
(
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    model_id      VARCHAR(100)                        NOT NULL COMMENT '统一模型 ID',
    model_name    VARCHAR(100)                        NOT NULL COMMENT '统一模型名称',
    vendor        VARCHAR(100)                        NOT NULL COMMENT '模型服务商',
    real_model    VARCHAR(100)                        NOT NULL COMMENT '真实模型 ID',
    meta          JSON                                NULL COMMENT '模型元信息',
    preview_image VARCHAR(255)                        NULL COMMENT '模型预览图地址',
    description   TEXT                                NULL COMMENT '模型描述',
    status        TINYINT   DEFAULT 1                 NOT NULL COMMENT '状态：0-禁用，1-启用',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    star_level    INT                                 NULL COMMENT '星级'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("payment_history", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS payment_history
(
    id           INT AUTO_INCREMENT PRIMARY KEY,
    user_id      INT                                 NOT NULL,
    payment_id   VARCHAR(50)                         NOT NULL,
    source       VARCHAR(15)                         NULL,
    source_id    INT                                 NULL,
    quantity     INT                                 NULL,
    valid_until  TIMESTAMP                           NULL,
    status       TINYINT                             NULL,
    environment  VARCHAR(10)                         NULL,
    purchase_at  TIMESTAMP                           NULL,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    retail_price INT                                 NULL COMMENT '销售价格'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX payment_payment_id ON payment_history (user_id, payment_id)`,
			`CREATE INDEX payment_status ON payment_history (user_id, status)`,
		}
	})

	m.Schema("20231129-ddl").Raw("prompt_example", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS prompt_example
(
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    title      VARCHAR(255)                        NOT NULL COMMENT '标题',
    content    TEXT                                NOT NULL COMMENT '内容',
    models     JSON                                NULL COMMENT '支持的模型，字符串数组格式',
    tags       JSON                                NULL COMMENT '支持的标签，字符串数组格式',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("prompt_tags", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS prompt_tags
(
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tag_name     VARCHAR(255)                        NOT NULL COMMENT '标签名称',
    tag_value    VARCHAR(255)                        NULL COMMENT '标签值',
    description  VARCHAR(255)                        NULL COMMENT '标签描述',
    category     VARCHAR(255)                        NULL COMMENT '标签分类',
    category_sub VARCHAR(255)                        NULL COMMENT '标签子分类',
    tag_type     TINYINT   DEFAULT 1                 NOT NULL COMMENT '标签类型：0-通用 1-提示语 2-反向提示语',
    meta         JSON                                NULL COMMENT '标签元信息',
    status       TINYINT   DEFAULT 1                 NOT NULL COMMENT '状态：0-禁用，1-启用',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("queue_tasks", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS queue_tasks
(
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    title      VARCHAR(255)                        NOT NULL,
    uid        INT                                 NOT NULL,
    task_id    VARCHAR(255) CHARSET utf8mb3        NOT NULL,
    task_type  VARCHAR(255) CHARSET utf8mb3        NOT NULL,
    queue_name VARCHAR(255) CHARSET utf8mb3        NOT NULL,
    payload    TEXT                                NOT NULL,
    result     TEXT                                NULL,
    status     VARCHAR(20) CHARSET utf8mb3         NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX queue_tasks_task_id ON queue_tasks (task_id)`,
			`CREATE INDEX queue_tasks_task_type ON queue_tasks (uid, task_type)`,
		}
	})

	m.Schema("20231129-ddl").Raw("queue_tasks_pending", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS queue_tasks_pending
(
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    payload         TEXT                                NULL COMMENT '任务负载',
    next_execute_at TIMESTAMP                           NULL COMMENT '下次执行时间',
    task_id         VARCHAR(255)                        NOT NULL,
    task_type       VARCHAR(255)                        NOT NULL,
    status          TINYINT                             NOT NULL COMMENT '1-处理中 2-执行成功 3-执行失败',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    deadline_at     TIMESTAMP                           NULL COMMENT '执行截止时间',
    execute_times   INT       DEFAULT 0                 NOT NULL COMMENT '当前执行次数'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX idx_queue_tasks_pending_status_next_execute_at ON queue_tasks_pending (status, next_execute_at)`,
		}
	})

	m.Schema("20231129-ddl").Raw("quota", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS quota
(
    id              INT AUTO_INCREMENT PRIMARY KEY,
    user_id         INT                                 NOT NULL,
    quota           INT                                 NOT NULL,
    rest            INT                                 NOT NULL,
    period_start_at TIMESTAMP                           NULL,
    period_end_at   TIMESTAMP                           NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    note            VARCHAR(255)                        NULL,
    payment_id      VARCHAR(50)                         NULL
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX quota_user_id ON quota (user_id, period_end_at)`,
		}
	})

	m.Schema("20231129-ddl").Raw("quota_statistics", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS quota_statistics
(
    id         INT AUTO_INCREMENT PRIMARY KEY,
    user_id    INT                                 NOT NULL,
    used       INT                                 NULL COMMENT '当日总用量',
    cal_date   DATE                                NULL COMMENT '统计日期',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("quota_usage", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS quota_usage
(
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id    INT                                 NOT NULL,
    used       INT                                 NULL,
    quota_ids  VARCHAR(255)                        NULL,
    debt       INT                                 NULL,
    meta       TEXT                                NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX quota_usage_created ON quota_usage (created_at)`,
			`CREATE INDEX quota_usage_user_id_created ON quota_usage (user_id, created_at)`,
		}
	})

	m.Schema("20231129-ddl").Raw("room_gallery", func() []string {
		return []string{`CREATE TABLE IF NOT EXISTS room_gallery
(
    id           INT AUTO_INCREMENT PRIMARY KEY,
    user_id      INT                                   NULL,
    avatar_id    INT                                   NULL COMMENT '头像 ID',
    name         VARCHAR(255)                          NULL COMMENT '房间名称',
    model        VARCHAR(50)                           NULL COMMENT '模型',
    vendor       VARCHAR(50)                           NULL COMMENT '模型服务商',
    prompt       TEXT                                  NULL COMMENT '系统提示语',
    max_context  TINYINT                               NULL COMMENT '最大上下文数',
    init_message TEXT                                  NULL COMMENT '初始化消息',
    created_at   TIMESTAMP   DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP   DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    avatar_url   VARCHAR(255)                          NULL COMMENT ' 头像地址',
    tags         JSON                                  NULL COMMENT '分类标签',
    description  TEXT                                  NULL COMMENT '房间描述',
    version_min  VARCHAR(10)                           NULL COMMENT '最小版本',
    version_max  VARCHAR(10)                           NULL COMMENT '最大版本',
    room_type    VARCHAR(20) DEFAULT 'default'         NULL COMMENT '房间类型：default/system'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`}
	})

	m.Schema("20231129-ddl").Raw("rooms", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS rooms
(
    id               INT AUTO_INCREMENT PRIMARY KEY,
    user_id          INT                                 NOT NULL,
    avatar_id        INT                                 NULL COMMENT '头像 ID',
    name             VARCHAR(255)                        NULL COMMENT '房间名称',
    description      VARCHAR(255)                        NULL COMMENT '房间描述',
    priority         INT                                 NULL COMMENT '房间优先级',
    model            VARCHAR(50)                         NULL COMMENT '房间模型',
    vendor           VARCHAR(50)                         NULL COMMENT '房间模型服务商',
    system_prompt    TEXT                                NULL COMMENT '系统提示语',
    last_active_time TIMESTAMP                           NULL COMMENT '最后活跃时间',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    max_context      TINYINT                             NULL COMMENT '最大上下文数',
    room_type        TINYINT                             NULL COMMENT '房间类型：1-系统预设 2-自定义',
    init_message     TEXT                                NULL COMMENT '初始化消息',
    avatar_url       VARCHAR(255)                        NULL COMMENT ' 头像地址'
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX rooms_user_id ON rooms (user_id)`,
		}
	})

	m.Schema("20231129-ddl").Raw("storage_file", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS storage_file
(
    id         INT AUTO_INCREMENT PRIMARY KEY,
    user_id    INT                                 NOT NULL,
    file_key   VARCHAR(255)                        NULL,
    hash       VARCHAR(255)                        NULL,
    file_size  INT                                 NULL,
    bucket     VARCHAR(100)                        NULL,
    name       VARCHAR(255)                        NULL,
    status     TINYINT   DEFAULT 1                 NOT NULL COMMENT '状态：1-正常 2-禁用 3-REVIEW',
    note       VARCHAR(255)                        NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    channel    VARCHAR(20)                         NULL COMMENT '上传渠道',
    CONSTRAINT idx_storage_file_key UNIQUE (file_key)
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
			`CREATE INDEX idx_storage_file_user ON storage_file (user_id)`,
		}
	})

	m.Schema("20231129-ddl").Raw("user_api_key", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS user_api_key
(
    id           INT AUTO_INCREMENT PRIMARY KEY,
    user_id      INT                                 NOT NULL,
    name         VARCHAR(255)                        NULL,
    token        VARCHAR(255)                        NOT NULL,
    status       TINYINT   DEFAULT 1                 NOT NULL COMMENT '状态：1-正常 2-禁用',
    valid_before TIMESTAMP                           NULL COMMENT '有效期',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT idx_api_token UNIQUE (token)
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci
`,
			`CREATE INDEX idx_api_user ON user_api_key (user_id)`,
		}
	})

	m.Schema("20231129-ddl").Raw("user_custom", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS user_custom
(
    id         INT AUTO_INCREMENT PRIMARY KEY,
    user_id    INT                                 NOT NULL,
    config     JSON                                NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT user_custom_user_idx UNIQUE (user_id)
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
		}
	})

	m.Schema("20231129-ddl").Raw("users", func() []string {
		return []string{
			`CREATE TABLE IF NOT EXISTS users
(
    id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_type            TINYINT   DEFAULT 0                 NULL COMMENT '用户类型：0-普通用户 1-内部用户',
    phone                VARCHAR(20)                         NULL,
    email                VARCHAR(40)                         NULL,
    apple_uid            VARCHAR(255)                        NULL,
    password             VARCHAR(255)                        NULL,
    realname             VARCHAR(255)                        NULL,
    avatar               VARCHAR(255)                        NULL,
    status               VARCHAR(20)                         NULL,
    created_at           TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at           TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    invited_by           INT                                 NULL,
    invite_code          VARCHAR(20) COLLATE utf8mb4_bin     NULL,
    prefer_signin_method VARCHAR(10)                         NULL COMMENT '优先选择的登录方式：sms_code, password',
    CONSTRAINT users_apple_uid UNIQUE (apple_uid),
    CONSTRAINT users_email UNIQUE (email),
    CONSTRAINT users_invite_code UNIQUE (invite_code),
    CONSTRAINT users_phone UNIQUE (phone)
) CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci`,
		}
	})

	m.Schema("20231129-ddl").Create("notifications", func(builder *migrate.Builder) {
		builder.Increments("id")
		builder.String("title", 255).Nullable(false).Comment("标题")
		builder.String("content", 255).Nullable(false).Comment("简介")
		builder.Integer("article_id", false, true).Nullable(false).Comment("文章 ID")
		builder.String("type", 255).Nullable(true).Comment("类型")
		builder.Timestamps(0)
		builder.Charset("utf8mb4")
		builder.Collation("utf8mb4_general_ci")
	})

	m.Schema("20231129-ddl").Create("articles", func(builder *migrate.Builder) {
		builder.Increments("id")
		builder.String("title", 255).Nullable(false).Comment("标题")
		builder.Text("content").Nullable(false).Comment("文章内容")
		builder.String("author", 255).Nullable(true).Comment("作者")
		builder.Timestamps(0)
		builder.Charset("utf8mb4")
		builder.Collation("utf8mb4_general_ci")
	})
}
