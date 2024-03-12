## 常用表结构

### 用户表 users

| 常用字段    | 备注                                  |
|-------------|---------------------------------------|
| phone       | 手机号                                |
| email       | 邮箱                                  |
| password    | 密码（加密存储）                        |
| realname    | 昵称                                  |
| status      | 用户状态： active-正常， deleted-已注销 |
| created_at  | 账号注册时间                          |
| invited_by  | 邀请人 ID                             |
| invite_code | 用户的邀请码                          |

### 系统默认数字人 room_gallery

| 常用字段 | 备注       |
|----------|------------|
| name | 数字人名称 | 
| model | 数字人模型：gpt-3.5-turbo、gpt-4 | 
| vendor | 厂商，默认全部 openai | 
| prompt | 提示语 | 
| max_context | 最大保持的上下文长度，默认全部写 6 | 
| init_message | 初次进入数字人时，默认显示的欢迎信息 | 
| avatar_url |  数字人头像 URL 地址 | 
| tags |  数字人分类 | 
| root_type | 数字人类型：system/default-默认数字人 | 

### 用户智慧果余额 quota

| 常用字段 | 备注       |
|----------|------------|
| user_id | 用户 ID | 
| quota | 总额度 | 
| rest |  剩余额度 | 
| period_end_at | 有效期截止时间 | 
| note | 备注，可不填写 | 

### 用户聊天历史记录 chat_messages

| 常用字段 | 备注       |
|----------|------------|
| user_id | 用户 ID | 
| room_id | 数字人 ID，首页聊一聊发起的，这里为 0 |
| message | 聊天消息内容 | 
| role | 角色：1-用户，2-机器人 | 
| created_at | 创建时间 | 