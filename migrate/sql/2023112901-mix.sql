CREATE TABLE `migrations`
(
    `id`        int unsigned                            NOT NULL AUTO_INCREMENT,
    `table`     varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
    `version`   varchar(20) COLLATE utf8mb4_unicode_ci  NOT NULL,
    `migration` longtext COLLATE utf8mb4_unicode_ci     NOT NULL,
    `batch`     int                                     NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB AUTO_INCREMENT = 39 DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (1,'alipay_history','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (2,'apple_pay_history','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (3,'cache','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (4,'chat_group_member','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (5,'chat_group_message','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (6,'chat_messages','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (7,'chat_sys_prompt_example','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (8,'creative_gallery','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (9,'creative_gallery_random','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (10,'creative_history','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (11,'creative_island','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (12,'debt','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (13,'events','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (14,'image_filter','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (15,'image_model','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (16,'payment_history','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (17,'prompt_example','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (18,'prompt_tags','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (19,'queue_tasks','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (20,'queue_tasks_pending','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (21,'quota','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (22,'quota_statistics','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (23,'quota_usage','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (24,'room_gallery','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (25,'rooms','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (26,'storage_file','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (27,'user_api_key','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (28,'user_custom','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (29,'users','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (30,'notifications','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (31,'articles','20231129-ddl','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (32,'room_gallery','20231129-dml','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (33,'creative_gallery','20231129-dml','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (34,'image_filter','20231129-dml','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (35,'image_model','20231129-dml','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (36,'prompt_tags','20231129-dml','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (37,'prompt_example','20231129-dml','',1701230025);
INSERT INTO `migrations` (`id`, `table`, `version`, `migration`, `batch`) VALUES (38,'chat_sys_prompt_example','20231129-dml','',1701230025);