CREATE TABLE storage_file
(
    id         INT PRIMARY KEY NOT NULL AUTO_INCREMENT,
    user_id    INT             NOT NULL,
    file_key   VARCHAR(255)    NULL,
    hash       VARCHAR(255)    NULL,
    file_size  INT             NULL,
    bucket     VARCHAR(100)    NULL,
    name       VARCHAR(255)    NULL,
    status     TINYINT         NOT NULL DEFAULT 1 COMMENT '状态：1-正常 2-禁用 3-REVIEW',
    note       VARCHAR(255)    NULL COMMENT '备注',
    channel    VARCHAR(20)     NULL COMMENT '上传渠道',
    created_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE INDEX idx_storage_file_user ON storage_file (user_id);
CREATE UNIQUE INDEX idx_storage_file_key ON storage_file (file_key);